package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Version information - will be set by build flags
var (
	CurrentVersion = "dev"
	BuildTime      = "unknown"
	GoVersion      = "unknown"
)

// GitHub API response for releases
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Draft       bool   `json:"draft"`
	PreRelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
}

// VersionInfo contains version comparison information
type VersionInfo struct {
	Current         string
	Latest          string
	UpdateAvailable bool
	ReleaseNotes    string
	PublishedAt     string
}

// getAvailableShell returns the path to an available shell, preferring bash over sh
func getAvailableShell() string {
	shells := []string{"/bin/bash", "/usr/bin/bash", "/bin/sh", "/usr/bin/sh"}
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
	GetVersionInfo() (*VersionInfo, error)
	GetCurrentVersion() string
	GetUpdateStatus() UpdateStatus
	StartProgressMonitoring() <-chan UpdateProgress
	StopProgressMonitoring()
}

// NewUpdateManager creates a new UpdateManager instance
func NewUpdateManager(scriptURL string, timeout time.Duration, backupConfig bool, logger Logger) *UpdateManager {
	if scriptURL == "" {
		scriptURL = "https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/update.sh"
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
	um.updateProgress("installing", 75, "Installing update and restarting service...")
	if err := um.executeScript(updateCtx, scriptPath); err != nil {
		um.updateError(err)
		return fmt.Errorf("failed to execute update script: %w", err)
	}

	// Step 4: Verify update completion (100% progress)
	um.updateProgress("completing", 100, "Update completed successfully")
	um.logger.Info("Bot update completed successfully - service should be restarted automatically")

	return nil
}

// CheckUpdateAvailable checks if an update is available by querying GitHub releases
func (um *UpdateManager) CheckUpdateAvailable() (bool, string, error) {
	versionInfo, err := um.GetVersionInfo()
	if err != nil {
		return false, "", err
	}
	return versionInfo.UpdateAvailable, versionInfo.Latest, nil
}

// GetVersionInfo gets detailed version information including release notes
func (um *UpdateManager) GetVersionInfo() (*VersionInfo, error) {
	current := um.GetCurrentVersion()

	// Get latest release from GitHub
	latest, releaseNotes, publishedAt, err := um.getLatestReleaseFromGitHub()
	if err != nil {
		return &VersionInfo{
			Current:         current,
			Latest:          "unknown",
			UpdateAvailable: false,
			ReleaseNotes:    "",
			PublishedAt:     "",
		}, err
	}

	// Compare versions
	updateAvailable := um.compareVersions(current, latest)

	return &VersionInfo{
		Current:         current,
		Latest:          latest,
		UpdateAvailable: updateAvailable,
		ReleaseNotes:    releaseNotes,
		PublishedAt:     publishedAt,
	}, nil
}

// getLatestReleaseFromGitHub fetches the latest release from GitHub API
func (um *UpdateManager) getLatestReleaseFromGitHub() (string, string, string, error) {
	// GitHub API URL for the latest release
	url := "https://api.github.com/repos/ad/xray-subscription-telegram-manager-for-keenetic/releases/latest"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", "", fmt.Errorf("failed to parse release info: %w", err)
	}

	// Skip draft and pre-release versions
	if release.Draft || release.PreRelease {
		return "", "", "", fmt.Errorf("latest release is draft or pre-release")
	}

	// Clean up release notes (limit length)
	releaseNotes := strings.TrimSpace(release.Body)
	if len(releaseNotes) > 500 {
		releaseNotes = releaseNotes[:497] + "..."
	}

	return release.TagName, releaseNotes, release.PublishedAt, nil
}

// compareVersions compares two version strings
func (um *UpdateManager) compareVersions(current, latest string) bool {
	// Simple version comparison
	// If current is "dev", always consider update available
	if current == "dev" {
		return true
	}

	// If we can't determine, assume no update to be safe
	if latest == "" || latest == "unknown" {
		return false
	}

	// Simple string comparison (could be improved with semantic versioning)
	return current != latest
}

// GetCurrentVersion returns the current version of the bot
func (um *UpdateManager) GetCurrentVersion() string {
	return CurrentVersion
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

	shell := getAvailableShell()
	um.logger.Debug("Using shell: %s for script execution", shell)

	// If systemd-run is available, execute as a transient unit so it survives service stop
	if hasSystemdRun() {
		args := []string{
			"--unit", "xray-telegram-manager-update",
			"--quiet",
			shell, scriptPath, "--force",
		}
		cmd := exec.CommandContext(ctx, "systemd-run", args...)
		// Minimal env
		cmd.Env = []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/sbin:/opt/bin",
			"HOME=/root",
			"SHELL=" + shell,
		}
		if err := cmd.Start(); err != nil {
			um.logger.Error("Failed to start update via systemd-run: %v", err)
			return fmt.Errorf("failed to start detached updater: %w", err)
		}
		um.logger.Info("Update launched as transient systemd unit; service will be restarted by updater")
		return nil
	}

	// Fallback: nohup in background (OpenWrt/BusyBox etc.)
	// Use sh -c to run nohup and background the process so that stop script doesn't kill it
	cmd := exec.CommandContext(ctx, shell, "-c", fmt.Sprintf("nohup %s '%s' --force >/tmp/xray-tg-update.log 2>&1 &", shell, scriptPath))
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/sbin:/opt/bin",
		"HOME=/root",
		"SHELL=" + shell,
	}
	if err := cmd.Start(); err != nil {
		um.logger.Error("Failed to start detached update script: %v", err)
		return fmt.Errorf("failed to start detached updater: %w", err)
	}
	um.logger.Info("Update script started in background; service will be restarted by updater")
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

// hasSystemdRun checks if systemd-run is available on the system
func hasSystemdRun() bool {
	if _, err := os.Stat("/bin/systemd-run"); err == nil {
		return true
	}
	if _, err := os.Stat("/usr/bin/systemd-run"); err == nil {
		return true
	}
	return false
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
