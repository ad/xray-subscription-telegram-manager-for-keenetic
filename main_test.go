package main

import (
	"os"
	"path/filepath"
	"testing"
	"xray-telegram-manager/config"
	"xray-telegram-manager/server"
)

// TestNewService tests the creation of a new service
func TestNewService(t *testing.T) {
	// Skip this test as it requires a valid Telegram bot token
	// In a real environment, this would be tested with integration tests
	t.Skip("Skipping TestNewService as it requires valid Telegram bot token")

	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create valid config
	configData := `{
		"admin_id": 123456789,
		"bot_token": "test_token",
		"subscription_url": "https://example.com/config.txt",
		"config_path": "/tmp/test_xray_config.json"
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test service creation
	service, err := NewService(configPath)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if service == nil {
		t.Fatal("Service should not be nil")
	}

	if service.config == nil {
		t.Error("Service config should not be nil")
	}

	if service.logger == nil {
		t.Error("Service logger should not be nil")
	}

	if service.bot == nil {
		t.Error("Service bot should not be nil")
	}

	if service.serverMgr == nil {
		t.Error("Service serverMgr should not be nil")
	}

	if service.running {
		t.Error("Service should not be running initially")
	}

	// Test cleanup
	service.cancel()
}

// TestNewService_InvalidConfig tests service creation with invalid config
func TestNewService_InvalidConfig(t *testing.T) {
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

			service, err := NewService(configPath)
			if err == nil {
				t.Error("Expected error for invalid config")
				if service != nil {
					service.cancel()
				}
			}
		})
	}
}

// TestServiceComponents tests individual service components that don't require Telegram bot
func TestServiceComponents(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create valid config
	configData := `{
		"admin_id": 123456789,
		"bot_token": "test_token",
		"subscription_url": "https://example.com/config.txt",
		"config_path": "/tmp/test_xray_config.json"
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test config loading
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.AdminID != 123456789 {
		t.Error("Config AdminID should be 123456789")
	}

	if cfg.BotToken != "test_token" {
		t.Error("Config BotToken should be 'test_token'")
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

	// Test config adapter
	adapter := &configAdapter{cfg}
	if adapter.GetAdminID() != 123456789 {
		t.Error("ConfigAdapter should return correct AdminID")
	}

	if adapter.GetBotToken() != "test_token" {
		t.Error("ConfigAdapter should return correct BotToken")
	}
}

// TestNewService_NonExistentConfig tests service creation with non-existent config
func TestNewService_NonExistentConfig(t *testing.T) {
	service, err := NewService("/nonexistent/config.json")
	if err == nil {
		t.Error("Expected error for non-existent config file")
		if service != nil {
			service.cancel()
		}
	}
}

// TestService_IsRunning tests the IsRunning method
func TestService_IsRunning(t *testing.T) {
	t.Skip("Skipping test that requires Telegram bot creation")
}

// TestService_GetStatus tests the GetStatus method
func TestService_GetStatus(t *testing.T) {
	t.Skip("Skipping test that requires Telegram bot creation")
}

// TestService_GetHealthStatus tests the GetHealthStatus method
func TestService_GetHealthStatus(t *testing.T) {
	t.Skip("Skipping test that requires Telegram bot creation")
}

// TestService_GetLastHealthCheck tests the GetLastHealthCheck method
func TestService_GetLastHealthCheck(t *testing.T) {
	t.Skip("Skipping test that requires Telegram bot creation")
}

// TestConfigAdapter tests the configAdapter functionality
func TestConfigAdapter(t *testing.T) {
	cfg := &config.Config{
		AdminID:            123456789,
		BotToken:           "test_token",
		ConfigPath:         "/test/path",
		SubscriptionURL:    "https://example.com/config",
		CacheDuration:      7200,
		PingTimeout:        10,
		XrayRestartCommand: "systemctl restart xray",
	}

	adapter := &configAdapter{cfg}

	if adapter.GetAdminID() != 123456789 {
		t.Error("ConfigAdapter GetAdminID should return correct value")
	}

	if adapter.GetBotToken() != "test_token" {
		t.Error("ConfigAdapter GetBotToken should return correct value")
	}

	if adapter.GetConfigPath() != "/test/path" {
		t.Error("ConfigAdapter GetConfigPath should return correct value")
	}

	if adapter.GetSubscriptionURL() != "https://example.com/config" {
		t.Error("ConfigAdapter GetSubscriptionURL should return correct value")
	}

	if adapter.GetCacheDuration() != 7200 {
		t.Error("ConfigAdapter GetCacheDuration should return correct value")
	}

	if adapter.GetPingTimeout() != 10 {
		t.Error("ConfigAdapter GetPingTimeout should return correct value")
	}

	if adapter.GetXrayRestartCommand() != "systemctl restart xray" {
		t.Error("ConfigAdapter GetXrayRestartCommand should return correct value")
	}
}

// TestService_HealthCheckMethods tests health check related methods
func TestService_HealthCheckMethods(t *testing.T) {
	t.Skip("Skipping test that requires Telegram bot creation")
}

// TestService_ConcurrentAccess tests concurrent access to service methods
func TestService_ConcurrentAccess(t *testing.T) {
	t.Skip("Skipping test that requires Telegram bot creation")
}

// TestService_ContextCancellation tests context cancellation behavior
func TestService_ContextCancellation(t *testing.T) {
	t.Skip("Skipping test that requires Telegram bot creation")
}
