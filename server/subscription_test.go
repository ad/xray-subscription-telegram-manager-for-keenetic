package server

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)

func TestSubscriptionLoader_DecodeBase64Config(t *testing.T) {
	cfg := &config.Config{
		CacheDuration: 3600,
		PingTimeout:   1,
	}
	loader := NewSubscriptionLoader(cfg)

	// Test data with VLESS URLs
	vlessUrls := []string{
		"vless://ec82bca8-1072-4682-822f-30306af408ea@127.0.0.1:443?type=tcp&security=reality&sni=outlook.office.com&pbk=TEST&sid=test&fp=chrome&flow=xtls-rprx-vision#Netherlands",
		"vless://ec82bca8-1072-4682-822f-30306af408ea@127.0.0.3:8080?type=tcp&security=none#Test%20Server",
	}

	// Create base64 encoded data
	data := base64.StdEncoding.EncodeToString([]byte(vlessUrls[0] + "\n" + vlessUrls[1]))

	servers, err := loader.DecodeBase64Config(data)
	if err != nil {
		t.Fatalf("DecodeBase64Config failed: %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("Expected 2 servers, got %d", len(servers))
	}

	// Verify first server
	server1 := servers[0]
	if server1.Address != "127.0.0.1" {
		t.Errorf("Expected address '127.0.0.1', got '%s'", server1.Address)
	}
	if server1.Port != 443 {
		t.Errorf("Expected port 443, got %d", server1.Port)
	}
	if server1.Protocol != "vless" {
		t.Errorf("Expected protocol 'vless', got '%s'", server1.Protocol)
	}

	// Verify second server
	server2 := servers[1]
	if server2.Address != "127.0.0.3" {
		t.Errorf("Expected address '127.0.0.3', got '%s'", server2.Address)
	}
	if server2.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", server2.Port)
	}
}

func TestSubscriptionLoader_LoadFromURL(t *testing.T) {
	// Create mock HTTP server
	vlessUrl := "vless://ec82bca8-1072-4682-822f-30306af408ea@127.0.0.3:8080?type=tcp&security=none#Test%20Server"
	mockServer := CreateMockSubscriptionServer([]string{vlessUrl})
	defer mockServer.Close()

	// Create temporary cache directory
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")

	cfg := &config.Config{
		SubscriptionURL: mockServer.URL(),
		CacheDuration:   3600,
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// Test loading from URL
	servers, err := loader.LoadFromURL()
	if err != nil {
		t.Fatalf("LoadFromURL failed: %v", err)
	}

	if len(servers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(servers))
	}

	server1 := servers[0]
	if server1.Address != "127.0.0.3" {
		t.Errorf("Expected address '127.0.0.3', got '%s'", server1.Address)
	}
	if server1.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", server1.Port)
	}

	// Verify cache file was created
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}
}

func TestSubscriptionLoader_CacheValidation(t *testing.T) {
	cfg := &config.Config{
		CacheDuration: 1, // 1 second for quick testing
		PingTimeout:   1,
	}
	loader := NewSubscriptionLoader(cfg)

	// Set cache with current time
	loader.cache = []types.Server{{ID: "test", Name: "Test Server"}}
	loader.lastUpdate = time.Now()

	// Cache should be valid immediately
	if !loader.isCacheValid() {
		t.Error("Cache should be valid immediately after setting")
	}

	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond)

	// Cache should now be invalid
	if loader.isCacheValid() {
		t.Error("Cache should be invalid after expiration")
	}
}

func TestSubscriptionLoader_FallbackToCache(t *testing.T) {
	// Create mock HTTP server that returns error
	mockServer := NewMockHTTPServer("", http.StatusInternalServerError)
	defer mockServer.Close()

	// Create temporary cache directory and file
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")

	// Create cache file with test data
	testServers := `[{"id":"test","name":"Test Server","address":"127.0.0.3","port":8080,"protocol":"vless"}]`
	err := os.WriteFile(cacheFile, []byte(testServers), 0644)
	if err != nil {
		t.Fatalf("Failed to create cache file: %v", err)
	}

	cfg := &config.Config{
		SubscriptionURL: mockServer.URL(),
		CacheDuration:   3600,
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// Should fallback to cache when URL fails
	servers, err := loader.LoadFromURL()
	if err != nil {
		t.Fatalf("LoadFromURL should succeed with cache fallback: %v", err)
	}

	if len(servers) != 1 {
		t.Fatalf("Expected 1 server from cache, got %d", len(servers))
	}

	if servers[0].Name != "Test Server" {
		t.Errorf("Expected 'Test Server', got '%s'", servers[0].Name)
	}
}

func TestSubscriptionLoader_InvalidateCache(t *testing.T) {
	cfg := &config.Config{
		CacheDuration: 3600,
		PingTimeout:   1,
	}
	loader := NewSubscriptionLoader(cfg)

	// Set cache
	loader.cache = []types.Server{{ID: "test", Name: "Test Server"}}
	loader.lastUpdate = time.Now()

	// Cache should be valid
	if !loader.isCacheValid() {
		t.Error("Cache should be valid")
	}

	// Invalidate cache
	loader.InvalidateCache()

	// Cache should now be invalid
	if loader.isCacheValid() {
		t.Error("Cache should be invalid after invalidation")
	}

	// Cache should be empty
	cached := loader.GetCachedServers()
	if len(cached) != 0 {
		t.Errorf("Expected empty cache, got %d servers", len(cached))
	}
}
