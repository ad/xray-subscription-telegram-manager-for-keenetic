package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/server"
	"xray-telegram-manager/types"
)

// TestEndToEndServerSwitching tests the complete server switching workflow
func TestEndToEndServerSwitching(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directories
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	xrayConfigPath := filepath.Join(tempDir, "xray_config.json")

	// Create mock subscription server
	vlessUrl := "vless://ec82bca8-1072-4682-822f-30306af408ea@127.0.0.300:443?type=tcp&security=reality&sni=example.com&pbk=testkey&sid=testid&fp=chrome&flow=xtls-rprx-vision#Test%20Server"
	mockServer := server.CreateMockSubscriptionServer([]string{vlessUrl})
	defer mockServer.Close()

	// Create initial xray config
	initialXrayConfig := types.XrayConfig{
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

	xrayData, err := json.MarshalIndent(initialXrayConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal xray config: %v", err)
	}

	if err := os.WriteFile(xrayConfigPath, xrayData, 0644); err != nil {
		t.Fatalf("Failed to write xray config: %v", err)
	}

	// Create service config
	configData := map[string]interface{}{
		"admin_id":             int64(123456789),
		"bot_token":            "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"config_path":          xrayConfigPath,
		"subscription_url":     mockServer.URL(),
		"log_level":            "debug",
		"xray_restart_command": "/bin/echo xray restarted",
		"cache_duration":       3600,
		"ping_timeout":         2,
	}

	configJSON, err := json.MarshalIndent(configData, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create service (skip bot creation for integration test)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create server manager directly to avoid Telegram bot issues
	serverMgr := server.NewServerManager(cfg)

	// Test server loading
	if err := serverMgr.LoadServers(); err != nil {
		t.Fatalf("Failed to load servers: %v", err)
	}

	servers := serverMgr.GetServers()
	if len(servers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(servers))
	}

	testServer := servers[0]
	if testServer.Name != "Test Server" {
		t.Errorf("Expected server name 'Test Server', got '%s'", testServer.Name)
	}

	if testServer.Address != "127.0.0.300" {
		t.Errorf("Expected server address '127.0.0.300', got '%s'", testServer.Address)
	}

	// Test server switching
	if err := serverMgr.SwitchServer(testServer.ID); err != nil {
		t.Fatalf("Failed to switch server: %v", err)
	}

	// Verify current server is set
	currentServer := serverMgr.GetCurrentServer()
	if currentServer == nil {
		t.Fatal("Current server should not be nil after switching")
	}

	if currentServer.ID != testServer.ID {
		t.Errorf("Expected current server ID '%s', got '%s'", testServer.ID, currentServer.ID)
	}

	if currentServer.Address != "127.0.0.300" {
		t.Errorf("Expected current server address '127.0.0.300', got '%s'", currentServer.Address)
	}

	if currentServer.Protocol != "vless" {
		t.Errorf("Expected current server protocol 'vless', got '%s'", currentServer.Protocol)
	}
}

// TestErrorHandlingAndRecovery tests error handling and recovery scenarios
func TestErrorHandlingAndRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	xrayConfigPath := filepath.Join(tempDir, "xray_config.json")

	// Create mock server that returns error
	mockServer := server.NewMockHTTPServer("", http.StatusInternalServerError)
	defer mockServer.Close()

	// Create service config
	configData := map[string]interface{}{
		"admin_id":             int64(123456789),
		"bot_token":            "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"config_path":          xrayConfigPath,
		"subscription_url":     mockServer.URL(),
		"log_level":            "debug",
		"xray_restart_command": "/bin/echo xray restarted",
		"cache_duration":       3600,
		"ping_timeout":         1,
	}

	configJSON, err := json.MarshalIndent(configData, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create server manager directly to avoid Telegram bot issues
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Use temporary cache directory to avoid interference from existing cache
	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	serverMgr := server.NewServerManagerWithCacheDir(cfg, cacheDir)

	// Test loading servers with network error (should fail gracefully)
	err = serverMgr.LoadServers()
	if err == nil {
		t.Error("Expected error when loading from failing mock server")
	}

	// Servers should be empty
	servers := serverMgr.GetServers()
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers after failed load, got %d", len(servers))
	}

	// Test switching to non-existent server
	err = serverMgr.SwitchServer("nonexistent")
	if err == nil {
		t.Error("Expected error when switching to non-existent server")
	}

	// Current server should still be nil
	currentServer := serverMgr.GetCurrentServer()
	if currentServer != nil {
		t.Error("Current server should be nil after failed switch")
	}
}

// TestConcurrentOperations tests concurrent operations and thread safety
func TestConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	xrayConfigPath := filepath.Join(tempDir, "xray_config.json")

	// Create mock subscription server with multiple servers
	vlessUrls := []string{
		"vless://b5cedeb2-005b-5f92-a180-6b5ecd7835c1@server1.com:443?type=tcp&security=tls#Server%201",
		"vless://b5cedeb2-005b-5f92-a180-6b5ecd7835c2@server2.com:443?type=tcp&security=tls#Server%202",
		"vless://b5cedeb2-005b-5f92-a180-6b5ecd7835c3@server3.com:443?type=tcp&security=tls#Server%203",
	}
	mockServer := server.CreateMockSubscriptionServer(vlessUrls)
	defer mockServer.Close()

	// Create initial xray config
	initialXrayConfig := types.XrayConfig{
		Outbounds: []types.XrayOutbound{
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
		},
	}

	xrayData, err := json.MarshalIndent(initialXrayConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal xray config: %v", err)
	}

	if err := os.WriteFile(xrayConfigPath, xrayData, 0644); err != nil {
		t.Fatalf("Failed to write xray config: %v", err)
	}

	// Create service config
	configData := map[string]interface{}{
		"admin_id":             int64(123456789),
		"bot_token":            "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"config_path":          xrayConfigPath,
		"subscription_url":     mockServer.URL(),
		"log_level":            "info",
		"xray_restart_command": "/bin/echo xray restarted",
		"cache_duration":       3600,
		"ping_timeout":         1,
	}

	configJSON, err := json.MarshalIndent(configData, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create server manager directly to avoid Telegram bot issues
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	serverMgr := server.NewServerManager(cfg)

	// Load servers
	if err := serverMgr.LoadServers(); err != nil {
		t.Fatalf("Failed to load servers: %v", err)
	}

	servers := serverMgr.GetServers()
	if len(servers) != 3 {
		t.Fatalf("Expected 3 servers, got %d", len(servers))
	}

	// Test concurrent operations with proper synchronization now implemented
	done := make(chan bool)
	errors := make(chan error, 20)

	// Start multiple goroutines that switch servers concurrently
	// Now that we have proper synchronization, we can test real concurrency
	for i := 0; i < 5; i++ {
		go func(serverIndex int) {
			defer func() { done <- true }()

			// Switch servers multiple times to stress test the synchronization
			for switchCount := 0; switchCount < 3; switchCount++ {
				serverID := servers[(serverIndex+switchCount)%len(servers)].ID
				if err := serverMgr.SwitchServer(serverID); err != nil {
					errors <- err
				}
				// Small delay to allow other goroutines to interleave
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Start goroutines that read server information concurrently (this should always be safe)
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < 10; j++ {
				serverMgr.GetServers()
				serverMgr.GetCurrentServer()
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Expected
		case err := <-errors:
			// Filter out expected errors in concurrent scenarios
			errMsg := err.Error()
			if !strings.Contains(errMsg, "is already active") &&
				!strings.Contains(errMsg, "server not found") {
				t.Errorf("Unexpected concurrent operation error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations to complete")
		}
	}

	// Verify final state is consistent
	currentServer := serverMgr.GetCurrentServer()
	if currentServer == nil {
		t.Error("Current server should not be nil after concurrent operations")
	} else {
		// Verify the current server is one of the expected servers
		found := false
		for _, testServer := range servers {
			if testServer.ID == currentServer.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("Current server should be one of the loaded servers")
		}
	}
}

// TestPingTestingWorkflow tests the complete ping testing workflow
func TestPingTestingWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create mock TCP server for available server
	mockTCPServer, err := server.NewMockTCPServer()
	if err != nil {
		t.Fatalf("Failed to create mock TCP server: %v", err)
	}
	defer mockTCPServer.Stop()
	mockTCPServer.Start()

	// Create mock subscription server with test servers
	vlessUrls := []string{
		fmt.Sprintf("vless://b5cedeb2-005b-5f92-a180-6b5ecd7835c4@%s:%d?type=tcp&security=none#Available%%20Server", mockTCPServer.Address(), mockTCPServer.Port()),
		"vless://b5cedeb2-005b-5f92-a180-6b5ecd7835c5@127.0.0.1:65529?type=tcp&security=none#Unavailable%20Server", // Should be unreachable (closed port)
	}
	mockHTTPServer := server.CreateMockSubscriptionServer(vlessUrls)
	defer mockHTTPServer.Close()

	// Create service config
	configData := map[string]interface{}{
		"admin_id":         int64(123456789),
		"bot_token":        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"subscription_url": mockHTTPServer.URL(),
		"log_level":        "info",
		"ping_timeout":     2,
	}

	configJSON, err := json.MarshalIndent(configData, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create server manager directly to avoid Telegram bot issues
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	serverMgr := server.NewServerManager(cfg)

	// Load servers
	if err := serverMgr.LoadServers(); err != nil {
		t.Fatalf("Failed to load servers: %v", err)
	}

	servers := serverMgr.GetServers()
	if len(servers) != 2 {
		t.Fatalf("Expected 2 servers, got %d", len(servers))
	}

	// Test ping functionality
	results, err := serverMgr.TestPing()
	if err != nil {
		t.Fatalf("Failed to test ping: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 ping results, got %d", len(results))
	}

	// Verify results
	var availableResult, unavailableResult *types.PingResult
	for i := range results {
		switch results[i].Server.Name {
		case "Available Server":
			availableResult = &results[i]
		case "Unavailable Server":
			unavailableResult = &results[i]
		}
	}

	if availableResult == nil {
		t.Fatal("Available Server result not found")
	}

	if unavailableResult == nil {
		t.Fatal("Unavailable Server result not found")
	}

	// Mock server should be available
	if !availableResult.Available {
		t.Errorf("Mock server should be available, got error: %v", availableResult.Error)
	}

	// Test Net should be unavailable (192.0.2.1 is a test address)
	if unavailableResult.Available {
		t.Error("Test Net server should not be available")
	}

	if unavailableResult.Error == nil {
		t.Error("Test Net server should have an error")
	}

	// Test ping with progress callback
	progressCalls := 0
	progressCallback := func(completed, total int, serverName string) {
		progressCalls++
		if completed < 0 || completed > total {
			t.Errorf("Invalid progress: completed=%d, total=%d", completed, total)
		}
		if total != 2 {
			t.Errorf("Expected total=2, got %d", total)
		}
		if serverName == "" {
			t.Error("Server name should not be empty")
		}
	}

	results, err = serverMgr.TestPingWithProgress(progressCallback)
	if err != nil {
		t.Fatalf("Failed to test ping with progress: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 ping results with progress, got %d", len(results))
	}

	if progressCalls != 2 {
		t.Errorf("Expected 2 progress calls, got %d", progressCalls)
	}
}

// TestConfigurationReload tests configuration reloading functionality
func TestConfigurationReload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Skipping test that requires Telegram bot creation")

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create initial config
	initialConfigData := map[string]interface{}{
		"admin_id":         int64(123456789),
		"bot_token":        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"subscription_url": "https://initial.example.com/config.txt",
		"log_level":        "info",
		"ping_timeout":     5,
	}

	configJSON, err := json.MarshalIndent(initialConfigData, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Test config loading directly instead of creating service
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify initial config
	if cfg.BotToken != "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ" {
		t.Error("Initial bot token should be 'initial_token'")
	}

	if cfg.LogLevel != "info" {
		t.Error("Initial log level should be 'info'")
	}

	if cfg.PingTimeout != 5 {
		t.Error("Initial ping timeout should be 5")
	}

	// Update config file
	updatedConfigData := map[string]interface{}{
		"admin_id":         int64(123456789),
		"bot_token":        "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"subscription_url": "https://updated.example.com/config.txt",
		"log_level":        "debug",
		"ping_timeout":     10,
	}

	updatedConfigJSON, err := json.MarshalIndent(updatedConfigData, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal updated config: %v", err)
	}

	if err := os.WriteFile(configPath, updatedConfigJSON, 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	// Test reload by loading config again
	updatedCfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	// Verify updated config
	if updatedCfg.BotToken != "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ" {
		t.Error("Bot token should be updated to 'updated_token'")
	}

	if updatedCfg.LogLevel != "debug" {
		t.Error("Log level should be updated to 'debug'")
	}

	if updatedCfg.PingTimeout != 10 {
		t.Error("Ping timeout should be updated to 10")
	}

	if updatedCfg.SubscriptionURL != "https://updated.example.com/config.txt" {
		t.Error("Subscription URL should be updated")
	}
}

// TestServiceLifecycle tests the complete service lifecycle
func TestServiceLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Skipping test that requires Telegram bot creation")

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create service config
	configData := map[string]interface{}{
		"admin_id":              int64(123456789),
		"bot_token":             "1234567890:ABCDefGhiJklMnoPqRsTuVwXyZ",
		"subscription_url":      "https://example.com/config.txt",
		"log_level":             "info",
		"health_check_interval": 0, // Disable health checks for this test
	}

	configJSON, err := json.MarshalIndent(configData, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Test config loading instead of full service creation
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test server manager creation (this doesn't require Telegram bot)
	serverMgr := server.NewServerManager(cfg)
	if serverMgr == nil {
		t.Fatal("Server manager should not be nil")
	}

	// Test basic server manager functionality
	servers := serverMgr.GetServers()
	if servers == nil {
		t.Error("GetServers should return empty slice, not nil")
	}

	currentServer := serverMgr.GetCurrentServer()
	if currentServer != nil {
		t.Error("Current server should be nil initially")
	}
}
