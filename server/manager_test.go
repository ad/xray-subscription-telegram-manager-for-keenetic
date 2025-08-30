package server

import (
	"testing"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)

// TestNewServerManager tests the creation of a new ServerManager
func TestNewServerManager(t *testing.T) {
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "test_token",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "info",
		XrayRestartCommand:  "echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         1,
	}

	sm := NewServerManager(cfg)

	if sm == nil {
		t.Fatal("NewServerManager returned nil")
	}

	if sm.config != cfg {
		t.Error("Config not properly set")
	}

	if sm.subscriptionLoader == nil {
		t.Error("SubscriptionLoader not initialized")
	}

	if sm.pingTester == nil {
		t.Error("PingTester not initialized")
	}

	if sm.xrayController == nil {
		t.Error("XrayController not initialized")
	}

	if len(sm.servers) != 0 {
		t.Error("Servers should be empty initially")
	}

	if sm.currentServer != nil {
		t.Error("CurrentServer should be nil initially")
	}
}

// TestGetServers tests the GetServers method
func TestGetServers(t *testing.T) {
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "test_token",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "info",
		XrayRestartCommand:  "echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         1,
	}

	sm := NewServerManager(cfg)

	// Test empty servers
	servers := sm.GetServers()
	if len(servers) != 0 {
		t.Error("Expected empty servers list")
	}

	// Add test servers
	testServers := []types.Server{
		{
			ID:       "server1",
			Name:     "Test Server 1",
			Address:  "1.1.1.1",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-server1",
		},
		{
			ID:       "server2",
			Name:     "Test Server 2",
			Address:  "2.2.2.2",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-server2",
		},
	}

	sm.mutex.Lock()
	sm.servers = testServers
	sm.mutex.Unlock()

	// Test getting servers
	servers = sm.GetServers()
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Verify it returns a copy (modifying returned slice shouldn't affect original)
	servers[0].Name = "Modified Name"
	originalServers := sm.GetServers()
	if originalServers[0].Name == "Modified Name" {
		t.Error("GetServers should return a copy, not the original slice")
	}
}

// TestGetCurrentServer tests the GetCurrentServer method
func TestGetCurrentServer(t *testing.T) {
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "test_token",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "info",
		XrayRestartCommand:  "echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         1,
	}

	sm := NewServerManager(cfg)

	// Test nil current server
	currentServer := sm.GetCurrentServer()
	if currentServer != nil {
		t.Error("Expected nil current server")
	}

	// Set a current server
	testServer := &types.Server{
		ID:       "server1",
		Name:     "Test Server 1",
		Address:  "1.1.1.1",
		Port:     443,
		Protocol: "vless",
		Tag:      "vless-server1",
	}

	sm.mutex.Lock()
	sm.currentServer = testServer
	sm.mutex.Unlock()

	// Test getting current server
	currentServer = sm.GetCurrentServer()
	if currentServer == nil {
		t.Fatal("Expected current server, got nil")
	}

	if currentServer.ID != testServer.ID {
		t.Errorf("Expected server ID %s, got %s", testServer.ID, currentServer.ID)
	}

	// Verify it returns a copy (modifying returned server shouldn't affect original)
	currentServer.Name = "Modified Name"
	originalCurrentServer := sm.GetCurrentServer()
	if originalCurrentServer.Name == "Modified Name" {
		t.Error("GetCurrentServer should return a copy, not the original server")
	}
}

// TestGetServerByID tests the GetServerByID method
func TestGetServerByID(t *testing.T) {
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "test_token",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "info",
		XrayRestartCommand:  "echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         1,
	}

	sm := NewServerManager(cfg)

	// Add test servers
	testServers := []types.Server{
		{
			ID:       "server1",
			Name:     "Test Server 1",
			Address:  "1.1.1.1",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-server1",
		},
		{
			ID:       "server2",
			Name:     "Test Server 2",
			Address:  "2.2.2.2",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-server2",
		},
	}

	sm.mutex.Lock()
	sm.servers = testServers
	sm.mutex.Unlock()

	// Test finding existing server
	server, err := sm.GetServerByID("server1")
	if err != nil {
		t.Fatalf("Expected to find server1, got error: %v", err)
	}

	if server.ID != "server1" {
		t.Errorf("Expected server ID server1, got %s", server.ID)
	}

	if server.Name != "Test Server 1" {
		t.Errorf("Expected server name 'Test Server 1', got %s", server.Name)
	}

	// Test finding non-existent server
	_, err = sm.GetServerByID("nonexistent")
	if err == nil {
		t.Error("Expected error when finding non-existent server")
	}
}

// TestSetCurrentServer tests the SetCurrentServer method
func TestSetCurrentServer(t *testing.T) {
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "test_token",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "info",
		XrayRestartCommand:  "echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         1,
	}

	sm := NewServerManager(cfg)

	// Add test servers
	testServers := []types.Server{
		{
			ID:       "server1",
			Name:     "Test Server 1",
			Address:  "1.1.1.1",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-server1",
		},
	}

	sm.mutex.Lock()
	sm.servers = testServers
	sm.mutex.Unlock()

	// Test setting existing server as current
	err := sm.SetCurrentServer("server1")
	if err != nil {
		t.Fatalf("Expected to set current server, got error: %v", err)
	}

	currentServer := sm.GetCurrentServer()
	if currentServer == nil {
		t.Fatal("Expected current server to be set")
	}

	if currentServer.ID != "server1" {
		t.Errorf("Expected current server ID server1, got %s", currentServer.ID)
	}

	// Test setting non-existent server as current
	err = sm.SetCurrentServer("nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent server as current")
	}
}
func TestServerManager_SortingIntegration(t *testing.T) {
	// Create test servers with names that should be sorted alphabetically
	servers := []types.Server{
		{ID: "3", Name: "Charlie Server", Address: "charlie.example.com", Port: 443},
		{ID: "1", Name: "Alpha Server", Address: "alpha.example.com", Port: 443},
		{ID: "2", Name: "Beta Server", Address: "beta.example.com", Port: 443},
	}

	// Create mock subscription loader that returns unsorted servers
	mockLoader := &MockSubscriptionLoader{
		servers: servers,
	}

	cfg := &config.Config{
		UI: config.UIConfig{
			EnableNameOptimization:    false, // Disable name optimization for this test
			NameOptimizationThreshold: 0.7,
		},
	}

	sm := NewServerManagerWithCacheDir(cfg, "/tmp/test")
	sm.subscriptionLoader = mockLoader

	// Load servers
	err := sm.LoadServers()
	if err != nil {
		t.Fatalf("Failed to load servers: %v", err)
	}

	// Test that GetServers returns alphabetically sorted servers
	sortedServers := sm.GetServers()
	expectedOrder := []string{"Alpha Server", "Beta Server", "Charlie Server"}

	if len(sortedServers) != len(expectedOrder) {
		t.Fatalf("Expected %d servers, got %d", len(expectedOrder), len(sortedServers))
	}

	for i, expected := range expectedOrder {
		if sortedServers[i].Name != expected {
			t.Errorf("Server at position %d: expected %s, got %s", i, expected, sortedServers[i].Name)
		}
	}

	// Test ping results sorting
	// Create mock ping results with different latencies
	mockResults := []types.PingResult{
		{Server: servers[0], Available: true, Latency: 200}, // Charlie - slowest
		{Server: servers[1], Available: true, Latency: 50},  // Alpha - fastest
		{Server: servers[2], Available: false, Latency: 0},  // Beta - unavailable
	}

	// Test GetQuickSelectServers
	quickSelect := sm.GetQuickSelectServers(mockResults, 2)

	// Should return only available servers, sorted by speed
	if len(quickSelect) != 2 {
		t.Fatalf("Expected 2 quick select servers, got %d", len(quickSelect))
	}

	// First should be Alpha (fastest)
	if quickSelect[0].Server.Name != "Alpha Server" {
		t.Errorf("First quick select server should be Alpha Server, got %s", quickSelect[0].Server.Name)
	}

	// Second should be Charlie (slower but available)
	if quickSelect[1].Server.Name != "Charlie Server" {
		t.Errorf("Second quick select server should be Charlie Server, got %s", quickSelect[1].Server.Name)
	}

	// Test that unavailable servers are not included in quick select
	for _, result := range quickSelect {
		if !result.Available {
			t.Error("Quick select should not include unavailable servers")
		}
	}
}
