package server_test

import (
	"testing"
	"xray-telegram-manager/config"
	"xray-telegram-manager/server"
	"xray-telegram-manager/types"
)

func TestServer_Basic(t *testing.T) {
	srv := types.Server{
		ID:       "test-1",
		Name:     "Test Server",
		Address:  "127.0.0.1",
		Port:     443,
		Protocol: "vless",
	}

	if srv.ID != "test-1" {
		t.Errorf("Expected ID 'test-1', got '%s'", srv.ID)
	}
	if srv.Name != "Test Server" {
		t.Errorf("Expected Name 'Test Server', got '%s'", srv.Name)
	}
}

func TestPingResult_Basic(t *testing.T) {
	srv := types.Server{ID: "test", Name: "Test"}
	result := types.PingResult{
		Server:    srv,
		Latency:   100,
		Available: true,
		Error:     nil,
	}

	if result.Latency != 100 {
		t.Errorf("Expected latency 100, got %d", result.Latency)
	}
	if !result.Available {
		t.Error("Expected server to be available")
	}
}

func TestServerManager_Creation(t *testing.T) {
	cfg := &config.Config{
		SubscriptionURL: "http://test.com",
		CacheDuration:   3600,
		LogLevel:        "info",
	}

	manager := server.NewServerManager(cfg)
	if manager == nil {
		t.Error("Expected manager to be created")
	}

	servers := manager.GetServers()
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers initially, got %d", len(servers))
	}

	current := manager.GetCurrentServer()
	if current != nil {
		t.Error("Expected no current server initially")
	}
}
