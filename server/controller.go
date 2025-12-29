package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"xray-telegram-manager/types"
)

type XrayController struct {
	config ConfigProvider
	mutex  sync.Mutex // Protects file operations
}

type ConfigProvider interface {
	GetConfigPath() string
	GetXrayRestartCommand() string
}

func NewXrayController(config ConfigProvider) *XrayController {
	return &XrayController{
		config: config,
		mutex:  sync.Mutex{},
	}
}

func (xc *XrayController) UpdateConfig(server types.Server) error {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()
	if err := xc.backupConfigUnsafe(); err != nil {
		return fmt.Errorf("failed to create backup before update: %w", err)
	}
	config, err := xc.getCurrentConfigUnsafe()
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}
	if err := xc.replaceProxyOutbound(config, server); err != nil {
		if restoreErr := xc.restoreConfigUnsafe(); restoreErr != nil {
			return fmt.Errorf("failed to replace proxy outbound: %w, and failed to restore backup: %v", err, restoreErr)
		}
		return fmt.Errorf("failed to replace proxy outbound (backup restored): %w", err)
	}
	if err := xc.writeConfigUnsafe(config); err != nil {
		if restoreErr := xc.restoreConfigUnsafe(); restoreErr != nil {
			return fmt.Errorf("failed to write config: %w, and failed to restore backup: %v", err, restoreErr)
		}
		return fmt.Errorf("failed to write config (backup restored): %w", err)
	}
	return nil
}

func (xc *XrayController) RestartService() error {
	restartCmd := "/opt/etc/init.d/S24xray restart"
	cmd := exec.Command("/bin/sh", "-c", restartCmd)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start xray restart command: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				// Process kill failed, but we continue anyway - this is expected
				_ = err
			}
		}
		return fmt.Errorf("xray restart command timed out after 30 seconds")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to restart xray service: %w", err)
		}
	}
	return nil
}

func (xc *XrayController) StopService() error {
	stopCmd := "/opt/etc/init.d/S24xray stop"
	cmd := exec.Command("/bin/sh", "-c", stopCmd)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start xray stop command: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				_ = err
			}
		}
		return fmt.Errorf("xray stop command timed out after 30 seconds")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to stop xray service: %w", err)
		}
	}
	return nil
}

func (xc *XrayController) StartService() error {
	startCmd := "/opt/etc/init.d/S24xray start"
	cmd := exec.Command("/bin/sh", "-c", startCmd)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start xray start command: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				_ = err
			}
		}
		return fmt.Errorf("xray start command timed out after 30 seconds")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to start xray service: %w", err)
		}
	}
	return nil
}

func (xc *XrayController) IsServiceRunning() (bool, error) {
	statusCmd := "/opt/etc/init.d/S24xray status"
	cmd := exec.Command("/bin/sh", "-c", statusCmd)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return false, fmt.Errorf("xray status check timed out")
	}

	// Exit code 0 means running
	if err == nil {
		return true, nil
	}

	// Check for exit codes that mean "stopped"
	if exitErr, ok := err.(*exec.ExitError); ok {
		// 1: Generic error (often used for dead/not running)
		// 3: Program is not running
		if exitErr.ExitCode() == 1 || exitErr.ExitCode() == 3 {
			return false, nil
		}
	}

	return false, err
}

// GetCurrentConfig reads and parses the current xray configuration (thread-safe)
func (xc *XrayController) GetCurrentConfig() (*types.XrayConfig, error) {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()
	return xc.getCurrentConfigUnsafe()
}

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

func (xc *XrayController) BackupConfig() error {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()
	return xc.backupConfigUnsafe()
}

func (xc *XrayController) backupConfigUnsafe() error {
	configPath := xc.config.GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file for backup: %w", err)
	}
	backupPath := fmt.Sprintf("%s.backup.%s.%d", configPath, time.Now().Format("20060102-150405"), os.Getpid())
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	return nil
}

func (xc *XrayController) RestoreConfig() error {
	xc.mutex.Lock()
	defer xc.mutex.Unlock()
	return xc.restoreConfigUnsafe()
}

func (xc *XrayController) restoreConfigUnsafe() error {
	configPath := xc.config.GetConfigPath()
	backupPattern := configPath + ".backup.*"
	matches, err := filepath.Glob(backupPattern)
	if err != nil {
		return fmt.Errorf("failed to search for backup files: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no backup files found")
	}
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
	backupData, err := os.ReadFile(mostRecentBackup)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}
	if err := xc.writeFileAtomicUnsafe(configPath, backupData); err != nil {
		return fmt.Errorf("failed to restore config from backup: %w", err)
	}
	return nil
}

func (xc *XrayController) writeConfigUnsafe(config *types.XrayConfig) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return xc.writeFileAtomicUnsafe(xc.config.GetConfigPath(), data)
}

func (xc *XrayController) writeFileAtomicUnsafe(filePath string, data []byte) error {
	tempPath := fmt.Sprintf("%s.tmp.%d.%d", filePath, time.Now().UnixNano(), os.Getpid())
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		if err := os.Remove(tempPath); err != nil {
			// Failed to remove temp file, but we continue anyway - this is expected
			_ = err
		}
		return fmt.Errorf("failed to replace config file: %w", err)
	}
	return nil
}

func (xc *XrayController) replaceProxyOutbound(config *types.XrayConfig, server types.Server) error {
	newOutbound := types.XrayOutbound{
		Tag:            server.Tag,
		Protocol:       server.Protocol,
		Settings:       server.Settings,
		StreamSettings: server.StreamSettings,
	}
	proxyFound := false
	for i, outbound := range config.Outbounds {
		if outbound.Protocol != "freedom" && outbound.Protocol != "blackhole" {
			config.Outbounds[i] = newOutbound
			proxyFound = true
			break
		}
	}
	if !proxyFound {
		config.Outbounds = append([]types.XrayOutbound{newOutbound}, config.Outbounds...)
	}
	return nil
}

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
