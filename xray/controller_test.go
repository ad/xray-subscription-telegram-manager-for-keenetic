package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"xray-telegram-manager/types"
)

// MockConfigProvider implements ConfigProvider for testing
type MockConfigProvider struct {
	configPath         string
	xrayRestartCommand string
}

func (m *MockConfigProvider) GetConfigPath() string {
	return m.configPath
}

func (m *MockConfigProvider) GetXrayRestartCommand() string {
	return m.xrayRestartCommand
}

func TestXrayController_GetCurrentConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create test config
	testConfig := types.XrayConfig{
		Outbounds: []types.XrayOutbound{
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
			{
				Tag:      "block",
				Protocol: "blackhole",
			},
		},
	}

	// Write test config to file
	data, err := json.MarshalIndent(testConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create controller
	mockConfig := &MockConfigProvider{
		configPath:         configPath,
		xrayRestartCommand: "echo restart",
	}
	controller := NewXrayController(mockConfig)

	// Test GetCurrentConfig
	config, err := controller.GetCurrentConfig()
	if err != nil {
		t.Fatalf("GetCurrentConfig failed: %v", err)
	}

	if len(config.Outbounds) != 2 {
		t.Errorf("Expected 2 outbounds, got %d", len(config.Outbounds))
	}

	if config.Outbounds[0].Tag != "direct" {
		t.Errorf("Expected first outbound tag 'direct', got '%s'", config.Outbounds[0].Tag)
	}
}

func TestXrayController_BackupAndRestore(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create test config
	originalData := `{"outbounds":[{"tag":"direct","protocol":"freedom"}]}`
	if err := os.WriteFile(configPath, []byte(originalData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create controller
	mockConfig := &MockConfigProvider{
		configPath:         configPath,
		xrayRestartCommand: "echo restart",
	}
	controller := NewXrayController(mockConfig)

	// Test backup
	if err := controller.BackupConfig(); err != nil {
		t.Fatalf("BackupConfig failed: %v", err)
	}

	// Modify original file
	modifiedData := `{"outbounds":[{"tag":"modified","protocol":"freedom"}]}`
	if err := os.WriteFile(configPath, []byte(modifiedData), 0644); err != nil {
		t.Fatalf("Failed to modify config: %v", err)
	}

	// Test restore
	if err := controller.RestoreConfig(); err != nil {
		t.Fatalf("RestoreConfig failed: %v", err)
	}

	// Verify restoration
	restoredData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read restored config: %v", err)
	}

	if string(restoredData) != originalData {
		t.Errorf("Config not properly restored. Expected: %s, Got: %s", originalData, string(restoredData))
	}
}

func TestXrayController_ReplaceProxyOutbound(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create test config with existing proxy
	testConfig := types.XrayConfig{
		Outbounds: []types.XrayOutbound{
			{
				Tag:      "old-proxy",
				Protocol: "vless",
				Settings: map[string]interface{}{
					"vnext": []interface{}{
						map[string]interface{}{
							"address": "old.server.com",
							"port":    443,
						},
					},
				},
			},
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
			{
				Tag:      "block",
				Protocol: "blackhole",
			},
		},
	}

	// Write test config to file
	data, err := json.MarshalIndent(testConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create controller
	mockConfig := &MockConfigProvider{
		configPath:         configPath,
		xrayRestartCommand: "echo restart",
	}
	controller := NewXrayController(mockConfig)

	// Create new server
	newServer := types.Server{
		Tag:      "new-proxy",
		Protocol: "vless",
		Settings: map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": "new.server.com",
					"port":    443,
				},
			},
		},
	}

	// Test replace proxy outbound
	if err := controller.ReplaceProxyOutbound(newServer); err != nil {
		t.Fatalf("ReplaceProxyOutbound failed: %v", err)
	}

	// Verify the replacement
	updatedConfig, err := controller.GetCurrentConfig()
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	// Should still have 3 outbounds
	if len(updatedConfig.Outbounds) != 3 {
		t.Errorf("Expected 3 outbounds, got %d", len(updatedConfig.Outbounds))
	}

	// First outbound should be the new proxy
	if updatedConfig.Outbounds[0].Tag != "new-proxy" {
		t.Errorf("Expected first outbound tag 'new-proxy', got '%s'", updatedConfig.Outbounds[0].Tag)
	}

	// Direct and block should be preserved
	foundDirect := false
	foundBlock := false
	for _, outbound := range updatedConfig.Outbounds {
		if outbound.Tag == "direct" && outbound.Protocol == "freedom" {
			foundDirect = true
		}
		if outbound.Tag == "block" && outbound.Protocol == "blackhole" {
			foundBlock = true
		}
	}

	if !foundDirect {
		t.Error("Direct outbound not preserved")
	}
	if !foundBlock {
		t.Error("Block outbound not preserved")
	}
}

// TestXrayController_EdgeCases tests various edge cases for xray controller
func TestXrayController_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(string) error
		testFunc    func(*XrayController) error
		expectError bool
	}{
		{
			name: "Non-existent config file",
			setupFunc: func(path string) error {
				// Don't create the file
				return nil
			},
			testFunc: func(controller *XrayController) error {
				_, err := controller.GetCurrentConfig()
				return err
			},
			expectError: true,
		},
		{
			name: "Invalid JSON config",
			setupFunc: func(path string) error {
				return os.WriteFile(path, []byte(`{invalid json}`), 0644)
			},
			testFunc: func(controller *XrayController) error {
				_, err := controller.GetCurrentConfig()
				return err
			},
			expectError: true,
		},
		{
			name: "Empty config file",
			setupFunc: func(path string) error {
				return os.WriteFile(path, []byte(``), 0644)
			},
			testFunc: func(controller *XrayController) error {
				_, err := controller.GetCurrentConfig()
				return err
			},
			expectError: true,
		},
		{
			name: "Config without outbounds",
			setupFunc: func(path string) error {
				config := `{"log": {"level": "info"}}`
				return os.WriteFile(path, []byte(config), 0644)
			},
			testFunc: func(controller *XrayController) error {
				_, err := controller.GetCurrentConfig()
				return err
			},
			expectError: false, // Should succeed with empty outbounds
		},
		{
			name: "Backup non-existent file",
			setupFunc: func(path string) error {
				// Don't create the file
				return nil
			},
			testFunc: func(controller *XrayController) error {
				return controller.BackupConfig()
			},
			expectError: true,
		},
		{
			name: "Restore without backup",
			setupFunc: func(path string) error {
				// Create main config but no backup
				config := `{"outbounds": []}`
				return os.WriteFile(path, []byte(config), 0644)
			},
			testFunc: func(controller *XrayController) error {
				return controller.RestoreConfig()
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.json")

			// Setup
			if err := tt.setupFunc(configPath); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Create controller
			mockConfig := &MockConfigProvider{
				configPath:         configPath,
				xrayRestartCommand: "echo restart",
			}
			controller := NewXrayController(mockConfig)

			// Test
			err := tt.testFunc(controller)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestXrayController_UpdateConfig tests the UpdateConfig method
func TestXrayController_UpdateConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create initial config
	initialConfig := types.XrayConfig{
		Outbounds: []types.XrayOutbound{
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
		},
	}

	data, err := json.MarshalIndent(initialConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Create controller
	mockConfig := &MockConfigProvider{
		configPath:         configPath,
		xrayRestartCommand: "echo restart",
	}
	controller := NewXrayController(mockConfig)

	// Create server to add
	server := types.Server{
		Tag:      "test-proxy",
		Protocol: "vless",
		Settings: map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": "test.server.com",
					"port":    443,
				},
			},
		},
		StreamSettings: map[string]interface{}{
			"network":  "tcp",
			"security": "tls",
		},
	}

	// Test UpdateConfig
	if err := controller.UpdateConfig(server); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the update
	updatedConfig, err := controller.GetCurrentConfig()
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	// Should have 2 outbounds now (proxy + direct)
	if len(updatedConfig.Outbounds) != 2 {
		t.Errorf("Expected 2 outbounds, got %d", len(updatedConfig.Outbounds))
	}

	// First outbound should be the proxy
	if updatedConfig.Outbounds[0].Tag != "test-proxy" {
		t.Errorf("Expected first outbound tag 'test-proxy', got '%s'", updatedConfig.Outbounds[0].Tag)
	}

	// Verify stream settings are preserved
	if updatedConfig.Outbounds[0].StreamSettings == nil {
		t.Error("StreamSettings should be preserved")
	}

	if updatedConfig.Outbounds[0].StreamSettings["network"] != "tcp" {
		t.Error("Network setting should be preserved")
	}
}

// TestXrayController_RestartService tests the RestartService method
func TestXrayController_RestartService(t *testing.T) {
	tests := []struct {
		name           string
		restartCommand string
		expectError    bool
	}{
		{
			name:           "Valid echo command",
			restartCommand: "echo 'restart successful'",
			expectError:    false,
		},
		{
			name:           "Invalid command",
			restartCommand: "nonexistent_command_12345",
			expectError:    true,
		},
		{
			name:           "Empty command",
			restartCommand: "",
			expectError:    false, // Empty command runs successfully as "sh -c ''"
		},
		{
			name:           "Command that fails",
			restartCommand: "false", // Command that always returns exit code 1
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := &MockConfigProvider{
				configPath:         "/tmp/dummy.json",
				xrayRestartCommand: tt.restartCommand,
			}
			controller := NewXrayController(mockConfig)

			err := controller.RestartService()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestXrayController_ConfigManipulation tests complex config manipulation scenarios
func TestXrayController_ConfigManipulation(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create complex initial config
	initialConfig := types.XrayConfig{
		Outbounds: []types.XrayOutbound{
			{
				Tag:      "proxy1",
				Protocol: "vless",
				Settings: map[string]interface{}{
					"vnext": []interface{}{
						map[string]interface{}{
							"address": "server1.com",
							"port":    443,
						},
					},
				},
			},
			{
				Tag:      "proxy2",
				Protocol: "vmess",
				Settings: map[string]interface{}{
					"vnext": []interface{}{
						map[string]interface{}{
							"address": "server2.com",
							"port":    443,
						},
					},
				},
			},
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
			{
				Tag:      "block",
				Protocol: "blackhole",
			},
		},
	}

	data, err := json.MarshalIndent(initialConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	mockConfig := &MockConfigProvider{
		configPath:         configPath,
		xrayRestartCommand: "echo restart",
	}
	controller := NewXrayController(mockConfig)

	// Test replacing multiple proxy outbounds
	newServer := types.Server{
		Tag:      "new-proxy",
		Protocol: "vless",
		Settings: map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": "newserver.com",
					"port":    443,
				},
			},
		},
	}

	if err := controller.ReplaceProxyOutbound(newServer); err != nil {
		t.Fatalf("ReplaceProxyOutbound failed: %v", err)
	}

	// Verify replacement
	updatedConfig, err := controller.GetCurrentConfig()
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	// Should have 4 outbounds (new proxy replaces proxy1, proxy2 remains + direct + block)
	if len(updatedConfig.Outbounds) != 4 {
		t.Errorf("Expected 4 outbounds, got %d", len(updatedConfig.Outbounds))
	}

	// First outbound should be the new proxy (replaced proxy1)
	if updatedConfig.Outbounds[0].Tag != "new-proxy" {
		t.Errorf("Expected first outbound tag 'new-proxy', got '%s'", updatedConfig.Outbounds[0].Tag)
	}

	// Verify proxy1 is replaced but proxy2 remains (only first proxy is replaced)
	foundProxy1 := false
	foundProxy2 := false
	for _, outbound := range updatedConfig.Outbounds {
		if outbound.Tag == "proxy1" {
			foundProxy1 = true
		}
		if outbound.Tag == "proxy2" {
			foundProxy2 = true
		}
	}

	if foundProxy1 {
		t.Error("proxy1 should have been replaced")
	}
	if !foundProxy2 {
		t.Error("proxy2 should still be present (only first proxy is replaced)")
	}

	// Verify direct and block are preserved
	foundDirect := false
	foundBlock := false
	for _, outbound := range updatedConfig.Outbounds {
		if outbound.Tag == "direct" {
			foundDirect = true
		}
		if outbound.Tag == "block" {
			foundBlock = true
		}
	}

	if !foundDirect {
		t.Error("Direct outbound should be preserved")
	}
	if !foundBlock {
		t.Error("Block outbound should be preserved")
	}
}
