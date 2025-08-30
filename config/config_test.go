package config

import (
	"os"
	"strings"
	"testing"
)

func TestSetDefaults(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	if config.ConfigPath != "/opt/etc/xray/configs/04_outbounds.json" {
		t.Errorf("Expected default ConfigPath, got %s", config.ConfigPath)
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected default LogLevel 'info', got %s", config.LogLevel)
	}
	if config.CacheDuration != 3600 {
		t.Errorf("Expected default CacheDuration 3600, got %d", config.CacheDuration)
	}
	if config.PingTimeout != 5 {
		t.Errorf("Expected default PingTimeout 5, got %d", config.PingTimeout)
	}

	// Test UI defaults
	if config.UI.MaxButtonTextLength != 50 {
		t.Errorf("Expected default MaxButtonTextLength 50, got %d", config.UI.MaxButtonTextLength)
	}
	if config.UI.ServersPerPage != 32 {
		t.Errorf("Expected default ServersPerPage 32, got %d", config.UI.ServersPerPage)
	}
	if config.UI.MaxQuickSelectServers != 10 {
		t.Errorf("Expected default MaxQuickSelectServers 10, got %d", config.UI.MaxQuickSelectServers)
	}
	if config.UI.MessageTimeoutMinutes != 60 {
		t.Errorf("Expected default MessageTimeoutMinutes 60, got %d", config.UI.MessageTimeoutMinutes)
	}
	if !config.UI.EnableNameOptimization {
		t.Error("Expected default EnableNameOptimization to be true")
	}
	if config.UI.NameOptimizationThreshold != 0.7 {
		t.Errorf("Expected default NameOptimizationThreshold 0.7, got %f", config.UI.NameOptimizationThreshold)
	}

	// Test Update defaults
	if config.Update.ScriptURL != "https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh" {
		t.Errorf("Expected default Update ScriptURL, got %s", config.Update.ScriptURL)
	}
	if config.Update.TimeoutMinutes != 10 {
		t.Errorf("Expected default Update TimeoutMinutes 10, got %d", config.Update.TimeoutMinutes)
	}
	if config.Update.BackupConfig {
		t.Error("Expected default Update BackupConfig to be false")
	}
}

func TestValidate(t *testing.T) {
	// Test valid config
	config := &Config{
		AdminID:         123456789,
		BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
		SubscriptionURL: "https://example.com/config.txt",
		LogLevel:        "info",
		PingTimeout:     5,
		UI: UIConfig{
			MaxButtonTextLength:       50,
			ServersPerPage:            32,
			MaxQuickSelectServers:     10,
			MessageTimeoutMinutes:     60,
			EnableNameOptimization:    true,
			NameOptimizationThreshold: 0.7,
		},
		Update: UpdateConfig{
			ScriptURL:      "https://example.com/script.sh",
			TimeoutMinutes: 10,
			BackupConfig:   false,
		},
	}

	if err := config.Validate(); err != nil {
		t.Errorf("Expected valid config to pass validation, got error: %v", err)
	}

	// Test missing required fields
	invalidConfig := &Config{}
	if err := invalidConfig.Validate(); err == nil {
		t.Error("Expected validation to fail for empty config")
	}

	// Test invalid log level
	invalidLogConfig := &Config{
		AdminID:         123456789,
		BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
		SubscriptionURL: "https://example.com/config.txt",
		LogLevel:        "invalid",
		PingTimeout:     5,
		UI: UIConfig{
			MaxButtonTextLength:       50,
			ServersPerPage:            32,
			MaxQuickSelectServers:     10,
			MessageTimeoutMinutes:     60,
			EnableNameOptimization:    true,
			NameOptimizationThreshold: 0.7,
		},
		Update: UpdateConfig{
			ScriptURL:      "https://example.com/script.sh",
			TimeoutMinutes: 10,
			BackupConfig:   false,
		},
	}
	if err := invalidLogConfig.Validate(); err == nil {
		t.Error("Expected validation to fail for invalid log level")
	}
}

func TestCreateTemplate(t *testing.T) {
	tempFile := "/tmp/test_config.json"
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	err := CreateTemplate(tempFile)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Error("Template file was not created")
	}

	// Try to load the template (should fail validation due to missing required fields)
	_, err = LoadConfig(tempFile)
	if err == nil {
		t.Error("Expected template to fail validation due to missing required fields")
	}
}

// TestLoadConfig_EdgeCases tests various edge cases for configuration loading
func TestLoadConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
		setupFunc   func(string) error
	}{
		{
			name:        "Non-existent file",
			expectError: true,
			setupFunc:   func(path string) error { return nil }, // Don't create file
		},
		{
			name:        "Invalid JSON",
			configData:  `{"admin_id": 123, "bot_token": "test", invalid json}`,
			expectError: true,
		},
		{
			name: "Valid minimal config",
			configData: `{
				"admin_id": 123456789,
				"bot_token": "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				"subscription_url": "https://example.com/config.txt",
				"config_path": "/opt/etc/xray/configs/04_outbounds.json"
			}`,
			expectError: false,
		},
		{
			name: "Config with all fields",
			configData: `{
				"admin_id": 123456789,
				"bot_token": "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				"config_path": "/custom/path/config.json",
				"subscription_url": "https://example.com/config.txt",
				"log_level": "debug",
				"xray_restart_command": "/opt/etc/init.d/S24xray restart",
				"cache_duration": 7200,
				"health_check_interval": 600,
				"ping_timeout": 10
			}`,
			expectError: false,
		},
		{
			name: "Config with invalid types",
			configData: `{
				"admin_id": "not_a_number",
				"bot_token": "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				"subscription_url": "https://example.com/config.txt"
			}`,
			expectError: true,
		},
		{
			name:        "Empty file",
			configData:  "",
			expectError: true,
		},
		{
			name:        "Empty JSON object",
			configData:  "{}",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := "/tmp/test_config_" + tt.name + ".json"
			defer func() {
				if err := os.Remove(tempFile); err != nil {
					t.Logf("Failed to remove temp file: %v", err)
				}
			}()

			// Setup file if needed
			if tt.setupFunc != nil {
				if err := tt.setupFunc(tempFile); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			// Write config data if provided
			if tt.configData != "" {
				if err := os.WriteFile(tempFile, []byte(tt.configData), 0644); err != nil {
					t.Fatalf("Failed to write test config: %v", err)
				}
			}

			// Test loading
			config, err := LoadConfig(tempFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify defaults are applied
			if config.ConfigPath == "" {
				t.Error("Expected default ConfigPath to be set")
			}

			if config.LogLevel == "" {
				t.Error("Expected default LogLevel to be set")
			}

			if config.CacheDuration == 0 {
				t.Error("Expected default CacheDuration to be set")
			}

			if config.PingTimeout == 0 {
				t.Error("Expected default PingTimeout to be set")
			}
		})
	}
}

// TestValidate_DetailedCases tests detailed validation scenarios
func TestValidate_DetailedCases(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config",
			config: Config{
				AdminID:         123456789,
				BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "info",
				PingTimeout:     5,
				UI: UIConfig{
					MaxButtonTextLength:       50,
					ServersPerPage:            32,
					MaxQuickSelectServers:     10,
					MessageTimeoutMinutes:     60,
					EnableNameOptimization:    true,
					NameOptimizationThreshold: 0.7,
				},
				Update: UpdateConfig{
					ScriptURL:      "https://example.com/script.sh",
					TimeoutMinutes: 10,
					BackupConfig:   false,
				},
			},
			expectError: false,
		},
		{
			name: "Missing AdminID",
			config: Config{
				BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "info",
				PingTimeout:     5,
			},
			expectError: true,
			errorMsg:    "admin_id",
		},
		{
			name: "Missing BotToken",
			config: Config{
				AdminID:         123456789,
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "info",
				PingTimeout:     5,
			},
			expectError: true,
			errorMsg:    "bot_token",
		},
		{
			name: "Missing SubscriptionURL",
			config: Config{
				AdminID:     123456789,
				BotToken:    "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:  "/opt/etc/xray/configs/04_outbounds.json",
				LogLevel:    "info",
				PingTimeout: 5,
			},
			expectError: true,
			errorMsg:    "subscription_url",
		},
		{
			name: "Invalid LogLevel",
			config: Config{
				AdminID:         123456789,
				BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "invalid_level",
				PingTimeout:     5,
			},
			expectError: true,
			errorMsg:    "log_level",
		},
		{
			name: "Invalid PingTimeout - zero",
			config: Config{
				AdminID:         123456789,
				BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "info",
				PingTimeout:     0,
			},
			expectError: true,
			errorMsg:    "ping_timeout",
		},
		{
			name: "Invalid PingTimeout - negative",
			config: Config{
				AdminID:         123456789,
				BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "info",
				PingTimeout:     -1,
			},
			expectError: true,
			errorMsg:    "ping_timeout",
		},
		{
			name: "Invalid CacheDuration - negative",
			config: Config{
				AdminID:         123456789,
				BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "info",
				PingTimeout:     5,
				CacheDuration:   -1,
			},
			expectError: true,
			errorMsg:    "cache_duration",
		},
		{
			name: "Invalid HealthCheckInterval - negative",
			config: Config{
				AdminID:             123456789,
				BotToken:            "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:          "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL:     "https://example.com/config.txt",
				LogLevel:            "info",
				PingTimeout:         5,
				HealthCheckInterval: -1,
			},
			expectError: true,
			errorMsg:    "health_check_interval",
		},
		{
			name: "Valid config with all optional fields",
			config: Config{
				AdminID:             123456789,
				BotToken:            "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:          "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL:     "https://example.com/config.txt",
				LogLevel:            "debug",
				XrayRestartCommand:  "/opt/etc/init.d/S24xray restart",
				CacheDuration:       7200,
				HealthCheckInterval: 300,
				PingTimeout:         10,
				UI: UIConfig{
					MaxButtonTextLength:       75,
					ServersPerPage:            25,
					MaxQuickSelectServers:     15,
					MessageTimeoutMinutes:     90,
					EnableNameOptimization:    false,
					NameOptimizationThreshold: 0.8,
				},
				Update: UpdateConfig{
					ScriptURL:      "https://example.com/script.sh",
					TimeoutMinutes: 15,
					BackupConfig:   true,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error but got none")
					return
				}

				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

// TestSetDefaults_Comprehensive tests comprehensive default setting
func TestSetDefaults_Comprehensive(t *testing.T) {
	config := &Config{
		// Set some non-default values to ensure they're not overwritten
		AdminID:         123456789,
		BotToken:        "existing_token",
		SubscriptionURL: "https://existing.com/config",
		LogLevel:        "debug", // Non-default
		PingTimeout:     10,      // Non-default
	}

	config.SetDefaults()

	// Check that existing values are preserved
	if config.AdminID != 123456789 {
		t.Error("AdminID should be preserved")
	}

	if config.BotToken != "existing_token" {
		t.Error("BotToken should be preserved")
	}

	if config.SubscriptionURL != "https://existing.com/config" {
		t.Error("SubscriptionURL should be preserved")
	}

	if config.LogLevel != "debug" {
		t.Error("LogLevel should be preserved when already set")
	}

	if config.PingTimeout != 10 {
		t.Error("PingTimeout should be preserved when already set")
	}

	// Check that defaults are set for empty fields
	if config.ConfigPath != "/opt/etc/xray/configs/04_outbounds.json" {
		t.Errorf("Expected default ConfigPath, got %s", config.ConfigPath)
	}

	if config.CacheDuration != 3600 {
		t.Errorf("Expected default CacheDuration 3600, got %d", config.CacheDuration)
	}

	// Test with completely empty config
	emptyConfig := &Config{}
	emptyConfig.SetDefaults()

	if emptyConfig.LogLevel != "info" {
		t.Errorf("Expected default LogLevel 'info', got %s", emptyConfig.LogLevel)
	}

	if emptyConfig.PingTimeout != 5 {
		t.Errorf("Expected default PingTimeout 5, got %d", emptyConfig.PingTimeout)
	}

	if emptyConfig.XrayRestartCommand != "/opt/etc/init.d/S24xray restart" {
		t.Errorf("Expected default XrayRestartCommand, got %s", emptyConfig.XrayRestartCommand)
	}
}

// TestUIConfigValidation tests UI configuration validation
func TestUIConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		uiConfig    UIConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid UI config",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            32,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: false,
		},
		{
			name: "Invalid MaxButtonTextLength - zero",
			uiConfig: UIConfig{
				MaxButtonTextLength:       0,
				ServersPerPage:            32,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "max_button_text_length must be positive",
		},
		{
			name: "Invalid MaxButtonTextLength - too large",
			uiConfig: UIConfig{
				MaxButtonTextLength:       250,
				ServersPerPage:            32,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "max_button_text_length cannot exceed 200",
		},
		{
			name: "Invalid ServersPerPage - zero",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            0,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "servers_per_page must be positive",
		},
		{
			name: "Invalid ServersPerPage - too large",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            150,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "servers_per_page cannot exceed 100",
		},
		{
			name: "Invalid MaxQuickSelectServers - zero",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            32,
				MaxQuickSelectServers:     0,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "max_quick_select_servers must be positive",
		},
		{
			name: "Invalid MaxQuickSelectServers - too large",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            32,
				MaxQuickSelectServers:     60,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "max_quick_select_servers cannot exceed 50",
		},
		{
			name: "Invalid MessageTimeoutMinutes - zero",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            32,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     0,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "message_timeout_minutes must be positive",
		},
		{
			name: "Invalid MessageTimeoutMinutes - too large",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            32,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     1500,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 0.7,
			},
			expectError: true,
			errorMsg:    "message_timeout_minutes cannot exceed 1440",
		},
		{
			name: "Invalid NameOptimizationThreshold - negative",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            32,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: -0.1,
			},
			expectError: true,
			errorMsg:    "name_optimization_threshold must be between 0 and 1",
		},
		{
			name: "Invalid NameOptimizationThreshold - too large",
			uiConfig: UIConfig{
				MaxButtonTextLength:       50,
				ServersPerPage:            32,
				MaxQuickSelectServers:     10,
				MessageTimeoutMinutes:     60,
				EnableNameOptimization:    true,
				NameOptimizationThreshold: 1.5,
			},
			expectError: true,
			errorMsg:    "name_optimization_threshold must be between 0 and 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				AdminID:         123456789,
				BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
				ConfigPath:      "/opt/etc/xray/configs/04_outbounds.json",
				SubscriptionURL: "https://example.com/config.txt",
				LogLevel:        "info",
				PingTimeout:     5,
				UI:              tt.uiConfig,
				Update: UpdateConfig{
					ScriptURL:      "https://example.com/script.sh",
					TimeoutMinutes: 10,
					BackupConfig:   false,
				},
			}

			err := config.Validate()

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error but got none")
					return
				}

				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

// TestConfigGetters tests the new getter methods
func TestConfigGetters(t *testing.T) {
	config := &Config{
		UI: UIConfig{
			MaxButtonTextLength:       75,
			ServersPerPage:            25,
			MaxQuickSelectServers:     15,
			MessageTimeoutMinutes:     90,
			EnableNameOptimization:    false,
			NameOptimizationThreshold: 0.8,
		},
		Update: UpdateConfig{
			ScriptURL:      "https://custom.com/script.sh",
			TimeoutMinutes: 15,
			BackupConfig:   true,
		},
	}

	// Test UI getters
	if config.GetMaxButtonTextLength() != 75 {
		t.Errorf("Expected MaxButtonTextLength 75, got %d", config.GetMaxButtonTextLength())
	}

	if config.GetServersPerPage() != 25 {
		t.Errorf("Expected ServersPerPage 25, got %d", config.GetServersPerPage())
	}

	if config.GetMaxQuickSelectServers() != 15 {
		t.Errorf("Expected MaxQuickSelectServers 15, got %d", config.GetMaxQuickSelectServers())
	}

	if config.GetMessageTimeoutMinutes() != 90 {
		t.Errorf("Expected MessageTimeoutMinutes 90, got %d", config.GetMessageTimeoutMinutes())
	}

	if config.IsNameOptimizationEnabled() {
		t.Error("Expected IsNameOptimizationEnabled to be false")
	}

	if config.GetNameOptimizationThreshold() != 0.8 {
		t.Errorf("Expected NameOptimizationThreshold 0.8, got %f", config.GetNameOptimizationThreshold())
	}

	// Test struct getters
	uiConfig := config.GetUIConfig()
	if uiConfig.MaxButtonTextLength != 75 {
		t.Errorf("Expected UI config MaxButtonTextLength 75, got %d", uiConfig.MaxButtonTextLength)
	}

	updateConfig := config.GetUpdateConfig()
	if updateConfig.ScriptURL != "https://custom.com/script.sh" {
		t.Errorf("Expected Update config ScriptURL, got %s", updateConfig.ScriptURL)
	}
}
