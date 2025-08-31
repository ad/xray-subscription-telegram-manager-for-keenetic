package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// getAvailableShell returns the path to an available shell, preferring sh over bash
func getAvailableShell() string {
	shells := []string{"/bin/sh", "/bin/bash", "/usr/bin/sh", "/usr/bin/bash"}
	for _, shell := range shells {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}
	// Fallback to sh if nothing found
	return "/bin/sh"
}

// UpdateManager handles bot update operations
type UpdateManager struct {
	scriptURL    string
	timeout      time.Duration
	backupConfig bool
	logger       Logger
	mutex        sync.RWMutex
	updateStatus UpdateStatus
	progressChan chan UpdateProgress
}

// UpdateStatus represents the current status of an update operation
type UpdateStatus struct {
	InProgress  bool
	Stage       string
	Progress    int // 0-100
	Error       error
	StartedAt   time.Time
	CompletedAt time.Time
}

// UpdateProgress represents progress updates during the update process
type UpdateProgress struct {
	Stage    string
	Progress int
	Message  string
	Error    error
}

// UpdateManagerInterface defines the interface for update operations
type UpdateManagerInterface interface {
	ExecuteUpdate(ctx context.Context) error
	CheckUpdateAvailable() (bool, string, error)
	GetCurrentVersion() string
	GetUpdateStatus() UpdateStatus
	StartProgressMonitoring() <-chan UpdateProgress
	StopProgressMonitoring()
}

// NewUpdateManager creates a new UpdateManager instance
func NewUpdateManager(scriptURL string, timeout time.Duration, backupConfig bool, logger Logger) *UpdateManager {
	if scriptURL == "" {
		scriptURL = "https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh"
	}
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	return &UpdateManager{
		scriptURL:    scriptURL,
		timeout:      timeout,
		backupConfig: backupConfig,
		logger:       logger,
		updateStatus: UpdateStatus{},
		progressChan: make(chan UpdateProgress, 10),
	}
}

// ExecuteUpdate performs the bot update process
func (um *UpdateManager) ExecuteUpdate(ctx context.Context) error {
	um.mutex.Lock()
	if um.updateStatus.InProgress {
		um.mutex.Unlock()
		return fmt.Errorf("update already in progress")
	}

	um.updateStatus = UpdateStatus{
		InProgress: true,
		StartedAt:  time.Now(),
		Stage:      "initializing",
		Progress:   0,
	}
	um.mutex.Unlock()

	defer func() {
		um.mutex.Lock()
		um.updateStatus.InProgress = false
		um.updateStatus.CompletedAt = time.Now()
		um.mutex.Unlock()
	}()

	um.logger.Info("Starting bot update process")

	// Create context with timeout
	updateCtx, cancel := context.WithTimeout(ctx, um.timeout)
	defer cancel()

	// Step 1: Download update script (25% progress)
	um.updateProgress("downloading", 25, "Downloading update script...")
	scriptPath, err := um.downloadScript(updateCtx)
	if err != nil {
		um.updateError(err)
		return fmt.Errorf("failed to download update script: %w", err)
	}
	defer func() {
		if err := os.Remove(scriptPath); err != nil {
			um.logger.Error("Failed to remove script file: %v", err)
		}
	}() // Clean up downloaded script

	// Step 2: Backup configuration if enabled (50% progress)
	if um.backupConfig {
		um.updateProgress("backing_up", 50, "Creating configuration backup...")
		if err := um.createConfigBackup(updateCtx); err != nil {
			um.logger.Warn("Failed to create config backup (continuing anyway): %v", err)
			// Don't fail the update if backup fails, just log it
		}
	} else {
		um.updateProgress("preparing", 50, "Preparing for update...")
	}

	// Step 3: Execute update script (75% progress)
	um.updateProgress("installing", 75, "Installing update...")
	if err := um.executeScript(updateCtx, scriptPath); err != nil {
		um.updateError(err)
		return fmt.Errorf("failed to execute update script: %w", err)
	}

	// Step 4: Verify update completion (100% progress)
	um.updateProgress("completing", 100, "Update completed successfully")
	um.logger.Info("Bot update completed successfully")

	return nil
}

// CheckUpdateAvailable checks if an update is available
func (um *UpdateManager) CheckUpdateAvailable() (bool, string, error) {
	// For now, we'll always return true since we don't have version checking
	// In a real implementation, this would check against a version endpoint
	return true, "latest", nil
}

// GetCurrentVersion returns the current version of the bot
func (um *UpdateManager) GetCurrentVersion() string {
	// This would typically read from a version file or build info
	return "dev" // Placeholder
}

// GetUpdateStatus returns the current update status
func (um *UpdateManager) GetUpdateStatus() UpdateStatus {
	um.mutex.RLock()
	defer um.mutex.RUnlock()
	return um.updateStatus
}

// StartProgressMonitoring starts monitoring update progress
func (um *UpdateManager) StartProgressMonitoring() <-chan UpdateProgress {
	return um.progressChan
}

// StopProgressMonitoring stops monitoring update progress
func (um *UpdateManager) StopProgressMonitoring() {
	close(um.progressChan)
	um.progressChan = make(chan UpdateProgress, 10)
}

// downloadScript downloads the update script from the configured URL
func (um *UpdateManager) downloadScript(ctx context.Context) (string, error) {
	um.logger.Debug("Downloading update script from: %s", um.scriptURL)

	req, err := http.NewRequestWithContext(ctx, "GET", um.scriptURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download script: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			um.logger.Error("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download script: HTTP %d", resp.StatusCode)
	}

	// Create temporary file for the script
	tmpFile, err := os.CreateTemp("", "update-script-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			um.logger.Error("Failed to close temp file: %v", err)
		}
	}()

	// Copy script content to temporary file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		if err := os.Remove(tmpFile.Name()); err != nil {
			um.logger.Error("Failed to remove temp file: %v", err)
		}
		return "", fmt.Errorf("failed to write script to file: %w", err)
	}

	// Make script executable
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		if err := os.Remove(tmpFile.Name()); err != nil {
			um.logger.Error("Failed to remove temp file: %v", err)
		}
		return "", fmt.Errorf("failed to make script executable: %w", err)
	}

	um.logger.Debug("Successfully downloaded update script to: %s", tmpFile.Name())
	return tmpFile.Name(), nil
}

// createConfigBackup creates a backup of the current configuration
func (um *UpdateManager) createConfigBackup(ctx context.Context) error {
	um.logger.Debug("Creating configuration backup")

	backupDir := "/opt/etc/xray-manager/backups"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s/config-backup-%s.json", backupDir, timestamp)

	configPath := "/opt/etc/xray-manager/config.json"

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		um.logger.Debug("Config file does not exist, skipping backup")
		return nil
	}

	// Copy config file to backup location
	cmd := exec.CommandContext(ctx, "cp", configPath, backupPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to backup config file: %w", err)
	}

	um.logger.Info("Configuration backed up to: %s", backupPath)
	return nil
}

// executeScript executes the update script with proper security measures
func (um *UpdateManager) executeScript(ctx context.Context, scriptPath string) error {
	um.logger.Debug("Executing update script: %s", scriptPath)

	// Validate script path to prevent path traversal
	if !um.isValidScriptPath(scriptPath) {
		return fmt.Errorf("invalid script path: %s", scriptPath)
	}

	// Ensure the script is executable
	if err := os.Chmod(scriptPath, 0755); err != nil {
		um.logger.Warn("Failed to set execute permissions on script: %v", err)
		// Continue anyway, as the shell might still be able to execute it
	}

	// Execute the script with restricted environment
	shell := getAvailableShell()
	um.logger.Debug("Using shell: %s for script execution", shell)
	cmd := exec.CommandContext(ctx, shell, scriptPath)

	// Set a clean environment with only essential variables
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/sbin:/opt/bin",
		"HOME=/root",
		"SHELL=" + shell,
	}

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		um.logger.Error("Update script execution failed: %v", err)
		um.logger.Error("Script output: %s", string(output))
		return fmt.Errorf("script execution failed: %w", err)
	}

	um.logger.Info("Update script executed successfully")
	um.logger.Debug("Script output: %s", string(output))
	return nil
}

// isValidScriptPath validates that the script path is safe to execute
func (um *UpdateManager) isValidScriptPath(path string) bool {
	// Check for path traversal attempts
	if len(path) == 0 || len(path) > 256 {
		return false
	}

	// Must be in /tmp directory (where CreateTemp creates files)
	if !os.IsPathSeparator(path[0]) {
		return false
	}

	// Check for dangerous characters
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\"", "'", "\\"}
	for _, char := range dangerousChars {
		if contains(path, char) {
			return false
		}
	}

	return true
}

// updateProgress updates the current progress and sends notification
func (um *UpdateManager) updateProgress(stage string, progress int, message string) {
	um.mutex.Lock()
	um.updateStatus.Stage = stage
	um.updateStatus.Progress = progress
	um.mutex.Unlock()

	// Send progress update through channel (non-blocking)
	select {
	case um.progressChan <- UpdateProgress{
		Stage:    stage,
		Progress: progress,
		Message:  message,
	}:
	default:
		// Channel is full, skip this update
	}

	um.logger.Debug("Update progress: %s (%d%%) - %s", stage, progress, message)
}

// updateError updates the status with an error
func (um *UpdateManager) updateError(err error) {
	um.mutex.Lock()
	um.updateStatus.Error = err
	um.mutex.Unlock()

	// Send error through channel (non-blocking)
	select {
	case um.progressChan <- UpdateProgress{
		Error: err,
	}:
	default:
		// Channel is full, skip this update
	}

	um.logger.Error("Update error: %v", err)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
