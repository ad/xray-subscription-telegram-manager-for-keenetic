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
	}
	if err := invalidLogConfig.Validate(); err == nil {
		t.Error("Expected validation to fail for invalid log level")
	}
}

func TestCreateTemplate(t *testing.T) {
	tempFile := "/tmp/test_config.json"
	defer os.Remove(tempFile)

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
			defer os.Remove(tempFile)

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
