package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"xray-telegram-manager/types"
)

// XrayController manages xray configuration and service
type XrayController struct {
	config ConfigProvider
	mutex  sync.Mutex // Protects file operations
}

// ConfigProvider interface for accessing configuration
type ConfigProvider interface {
	GetConfigPath() string
	GetXrayRestartCommand() string
}

// NewXrayController creates a new xray controller instance
func NewXrayController(config ConfigProvider) *XrayController {
	return &XrayController{
		config: config,
		mutex:  sync.Mutex{},
	}
}

// UpdateConfig updates xray configuration with the specified server
func (xc *XrayController) UpdateConfig(server types.Server) error {
	// Protect the entire update operation with mutex to prevent race conditions
	xc.mutex.Lock()
	defer xc.mutex.Unlock()

	// Create backup before making changes
	if err := xc.backupConfigUnsafe(); err != nil {
		return fmt.Errorf("failed to create backup before update: %w", err)
	}

	// Get current configuration
	config, err := xc.getCurrentConfigUnsafe()
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}

	// Replace proxy outbound with new server
	if err := xc.replaceProxyOutbound(config, server); err != nil {
		// Attempt to restore backup on failure
		if restoreErr := xc.restoreConfigUnsafe(); restoreErr != nil {
			return fmt.Errorf("failed to replace proxy outbound: %w, and failed to restore backup: %v", err, restoreErr)
		}
		return fmt.Errorf("failed to replace proxy outbound (backup restored): %w", err)
	}

	// Write updated configuration
	if err := xc.writeConfigUnsafe(config); err != nil {
		// Attempt to restore backup on failure
		if restoreErr := xc.restoreConfigUnsafe(); restoreErr != nil {
			return fmt.Errorf("failed to write config: %w, and failed to restore backup: %v", err, restoreErr)
		}
		return fmt.Errorf("failed to write config (backup restored): %w", err)
	}

	return nil
}

// RestartService restarts the xray service
func (xc *XrayController) RestartService() error {
	cmd := exec.Command("sh", "-c", xc.config.GetXrayRestartCommand())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart xray service: %w", err)
	}
	return nil
}

// GetCurrentConfig reads and parses the current xray configuration (thread-safe)
func (xc *XrayController) GetCurrentConfig() (*types.XrayConfig, error) {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()
	return xc.getCurrentConfigUnsafe()
}

// getCurrentConfigUnsafe reads and parses the current xray configuration (not thread-safe)
func (xc *XrayController) getCurrentConfigUnsafe() (*types.XrayConfig, error) {
	data, err := os.ReadFile(xc.config.GetConfigPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config types.XrayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// BackupConfig creates a backup of the current configuration (thread-safe)
func (xc *XrayController) BackupConfig() error {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()
	return xc.backupConfigUnsafe()
}

// backupConfigUnsafe creates a backup of the current configuration (not thread-safe)
func (xc *XrayController) backupConfigUnsafe() error {
	configPath := xc.config.GetConfigPath()

	// Read current config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file for backup: %w", err)
	}

	// Create backup filename with timestamp and process ID for uniqueness
	backupPath := fmt.Sprintf("%s.backup.%s.%d", configPath, time.Now().Format("20060102-150405"), os.Getpid())

	// Write backup file
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}

	return nil
}

// RestoreConfig restores configuration from the most recent backup (thread-safe)
func (xc *XrayController) RestoreConfig() error {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()
	return xc.restoreConfigUnsafe()
}

// restoreConfigUnsafe restores configuration from the most recent backup (not thread-safe)
func (xc *XrayController) restoreConfigUnsafe() error {
	configPath := xc.config.GetConfigPath()

	// Find the most recent backup file
	backupPattern := configPath + ".backup.*"
	matches, err := filepath.Glob(backupPattern)
	if err != nil {
		return fmt.Errorf("failed to search for backup files: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("no backup files found")
	}

	// Get the most recent backup (last in sorted order)
	var mostRecentBackup string
	var mostRecentTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.ModTime().After(mostRecentTime) {
			mostRecentTime = info.ModTime()
			mostRecentBackup = match
		}
	}

	if mostRecentBackup == "" {
		return fmt.Errorf("no valid backup files found")
	}

	// Read backup data
	backupData, err := os.ReadFile(mostRecentBackup)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Restore the backup using atomic write
	if err := xc.writeFileAtomicUnsafe(configPath, backupData); err != nil {
		return fmt.Errorf("failed to restore config from backup: %w", err)
	}

	return nil
}

// writeConfigUnsafe writes xray configuration to file with proper formatting (not thread-safe)
func (xc *XrayController) writeConfigUnsafe(config *types.XrayConfig) error {
	// Marshal config with proper indentation
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write using atomic operation
	return xc.writeFileAtomicUnsafe(xc.config.GetConfigPath(), data)
}

// writeFileAtomicUnsafe writes data to file atomically using unique temporary file (not thread-safe)
func (xc *XrayController) writeFileAtomicUnsafe(filePath string, data []byte) error {
	// Create unique temporary file name using timestamp and process ID
	tempPath := fmt.Sprintf("%s.tmp.%d.%d", filePath, time.Now().UnixNano(), os.Getpid())

	// Write to temporary file first for atomic operation
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomically replace the original file
	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

// replaceProxyOutbound replaces the proxy outbound in the configuration while preserving non-proxy outbounds
func (xc *XrayController) replaceProxyOutbound(config *types.XrayConfig, server types.Server) error {
	// Convert server to xray outbound format
	newOutbound := types.XrayOutbound{
		Tag:            server.Tag,
		Protocol:       server.Protocol,
		Settings:       server.Settings,
		StreamSettings: server.StreamSettings,
	}

	// Find and replace existing proxy outbound or add new one
	proxyFound := false
	for i, outbound := range config.Outbounds {
		// Replace if this is a proxy outbound (not direct or block)
		if outbound.Protocol != "freedom" && outbound.Protocol != "blackhole" {
			config.Outbounds[i] = newOutbound
			proxyFound = true
			break
		}
	}

	// If no proxy outbound found, add it at the beginning
	if !proxyFound {
		config.Outbounds = append([]types.XrayOutbound{newOutbound}, config.Outbounds...)
	}

	return nil
}

// ReplaceProxyOutbound is a public method to replace proxy outbound (for external use)
func (xc *XrayController) ReplaceProxyOutbound(server types.Server) error {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()

	config, err := xc.getCurrentConfigUnsafe()
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}

	if err := xc.replaceProxyOutbound(config, server); err != nil {
		return err
	}

	return xc.writeConfigUnsafe(config)
}
