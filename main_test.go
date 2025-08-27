package main

import (
	"os"
	"path/filepath"
	"testing"
	"xray-telegram-manager/config"
	"xray-telegram-manager/server"
)

// TestConfigLoading tests config loading functionality
func TestConfigLoading(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create valid config
	configData := `{
		"admin_id": 123456789,
		"bot_token": "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"subscription_url": "https://example.com/config.txt",
		"config_path": "/tmp/test_xray_config.json"
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test config loading
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Config should not be nil")
	}

	if cfg.AdminID != 123456789 {
		t.Error("Config AdminID should be 123456789")
	}

	if cfg.BotToken != "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ" {
		t.Error("Config BotToken should match expected format")
	}
}

// TestConfigInvalidCases tests config loading with invalid cases
func TestConfigInvalidCases(t *testing.T) {
	tests := []struct {
		name       string
		configData string
	}{
		{
			name:       "Invalid JSON",
			configData: `{invalid json}`,
		},
		{
			name:       "Missing required fields",
			configData: `{"admin_id": 123}`,
		},
		{
			name:       "Empty config",
			configData: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.json")

			if err := os.WriteFile(configPath, []byte(tt.configData), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err := config.LoadConfig(configPath)
			if err == nil {
				t.Error("Expected error for invalid config")
			}
		})
	}
}

// TestServerManagerCreation tests server manager creation
func TestServerManagerCreation(t *testing.T) {
	// Create a basic config
	cfg := &config.Config{
		AdminID:         123456789,
		BotToken:        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		SubscriptionURL: "https://example.com/config.txt",
		ConfigPath:      "/tmp/test_xray_config.json",
	}

	// Test server manager creation
	serverMgr := server.NewServerManager(cfg)
	if serverMgr == nil {
		t.Fatal("ServerManager should not be nil")
	}

	// Test initial state
	servers := serverMgr.GetServers()
	if len(servers) != 0 {
		t.Error("Initial servers list should be empty")
	}

	currentServer := serverMgr.GetCurrentServer()
	if currentServer != nil {
		t.Error("Initial current server should be nil")
	}
}

// TestNonExistentConfig tests loading non-existent config
func TestNonExistentConfig(t *testing.T) {
	_, err := config.LoadConfig("/nonexistent/config.json")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}
