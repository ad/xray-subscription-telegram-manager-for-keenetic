package server

import (
	"testing"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)

func TestServerManager_LoadServersWithNameOptimization(t *testing.T) {
	// Create a test config with name optimization enabled
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "debug",
		XrayRestartCommand:  "/bin/echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         5,
		UI: config.UIConfig{
			EnableNameOptimization:    true,
			NameOptimizationThreshold: 0.7,
		},
	}

	// Create a mock subscription loader that returns servers with common suffixes
	mockLoader := NewMockSubscriptionLoader(cfg)
	mockLoader.SetServers([]types.Server{
		{ID: "1", Name: "server1.example.com", VlessUrl: "vless://test1"},
		{ID: "2", Name: "server2.example.com", VlessUrl: "vless://test2"},
		{ID: "3", Name: "server3.example.com", VlessUrl: "vless://test3"},
	})

	// Create server manager
	sm := NewServerManager(cfg)
	sm.subscriptionLoader = mockLoader

	// Load servers
	err := sm.LoadServers()
	if err != nil {
		t.Fatalf("LoadServers failed: %v", err)
	}

	// Get servers and verify optimization was applied
	servers := sm.GetServers()
	if len(servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(servers))
	}

	// Check that names were optimized (should have .example.com removed)
	expectedNames := []string{"server1", "server2", "server3"}
	for i, server := range servers {
		if server.Name != expectedNames[i] {
			t.Errorf("expected server[%d].Name = %s, got %s", i, expectedNames[i], server.Name)
		}
	}
}

func TestServerManager_LoadServersWithNameOptimizationDisabled(t *testing.T) {
	// Create a test config with name optimization disabled
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "debug",
		XrayRestartCommand:  "/bin/echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         5,
		UI: config.UIConfig{
			EnableNameOptimization:    false,
			NameOptimizationThreshold: 0.7,
		},
	}

	// Create a mock subscription loader that returns servers with common suffixes
	mockLoader := NewMockSubscriptionLoader(cfg)
	mockLoader.SetServers([]types.Server{
		{ID: "1", Name: "server1.example.com", VlessUrl: "vless://test1"},
		{ID: "2", Name: "server2.example.com", VlessUrl: "vless://test2"},
		{ID: "3", Name: "server3.example.com", VlessUrl: "vless://test3"},
	})

	// Create server manager
	sm := NewServerManager(cfg)
	sm.subscriptionLoader = mockLoader

	// Load servers
	err := sm.LoadServers()
	if err != nil {
		t.Fatalf("LoadServers failed: %v", err)
	}

	// Get servers and verify optimization was NOT applied
	servers := sm.GetServers()
	if len(servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(servers))
	}

	// Check that names were NOT optimized (should keep original names)
	expectedNames := []string{"server1.example.com", "server2.example.com", "server3.example.com"}
	for i, server := range servers {
		if server.Name != expectedNames[i] {
			t.Errorf("expected server[%d].Name = %s, got %s", i, expectedNames[i], server.Name)
		}
	}
}

func TestServerManager_LoadServersWithInsufficientCoverage(t *testing.T) {
	// Create a test config with name optimization enabled
	cfg := &config.Config{
		AdminID:             123456789,
		BotToken:            "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
		ConfigPath:          "/tmp/test_config.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "debug",
		XrayRestartCommand:  "/bin/echo restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         5,
		UI: config.UIConfig{
			EnableNameOptimization:    true,
			NameOptimizationThreshold: 0.7,
		},
	}

	// Create servers with insufficient common suffix coverage
	mockLoader := NewMockSubscriptionLoader(cfg)
	mockLoader.SetServers([]types.Server{
		{ID: "1", Name: "server1.example.com", VlessUrl: "vless://test1"},
		{ID: "2", Name: "server2.different.org", VlessUrl: "vless://test2"},
		{ID: "3", Name: "server3.another.net", VlessUrl: "vless://test3"},
		{ID: "4", Name: "server4.test.edu", VlessUrl: "vless://test4"},
	})

	// Create server manager
	sm := NewServerManager(cfg)
	sm.subscriptionLoader = mockLoader

	// Load servers
	err := sm.LoadServers()
	if err != nil {
		t.Fatalf("LoadServers failed: %v", err)
	}

	// Get servers and verify optimization was NOT applied due to insufficient coverage
	servers := sm.GetServers()
	if len(servers) != 4 {
		t.Fatalf("expected 4 servers, got %d", len(servers))
	}

	// Check that names were NOT optimized (should keep original names)
	expectedNames := []string{"server1.example.com", "server2.different.org", "server3.another.net", "server4.test.edu"}
	for i, server := range servers {
		if server.Name != expectedNames[i] {
			t.Errorf("expected server[%d].Name = %s, got %s", i, expectedNames[i], server.Name)
		}
	}
}
