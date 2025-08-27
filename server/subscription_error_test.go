package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"xray-telegram-manager/config"
)

func TestSubscriptionLoader_RetryLogic(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dmxlc3M6Ly9lYzgyYmNhOC0xMDcyLTQ2ODItODIyZi0zMDMwNmFmNDA4ZWFAMTI3LjAuMDM6ODA4MD90eXBlPXRjcCZzZWN1cml0eT1ub25lI1Rlc3QlMjBTZXJ2ZXI=")) // base64 encoded VLESS URL
	}))
	defer server.Close()

	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")

	cfg := &config.Config{
		SubscriptionURL: server.URL,
		CacheDuration:   3600,
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// Should succeed after retries
	servers, err := loader.LoadFromURL()
	if err != nil {
		t.Fatalf("LoadFromURL should succeed after retries: %v", err)
	}

	if len(servers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(servers))
	}

	// Verify that 3 attempts were made
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestSubscriptionLoader_MaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always fail
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create cache file with test data for fallback
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")
	testServers := `[{"id":"test","name":"Test Server","address":"127.0.0.3","port":8080,"protocol":"vless"}]`
	err := os.WriteFile(cacheFile, []byte(testServers), 0644)
	if err != nil {
		t.Fatalf("Failed to create cache file: %v", err)
	}

	cfg := &config.Config{
		SubscriptionURL: server.URL,
		CacheDuration:   3600,
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// Should fallback to cache after max retries
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

func TestSubscriptionLoader_NetworkErrorWithoutCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always fail
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")

	cfg := &config.Config{
		SubscriptionURL: server.URL,
		CacheDuration:   3600,
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// Should fail when no cache is available
	_, err := loader.LoadFromURL()
	if err == nil {
		t.Fatal("LoadFromURL should fail when no cache is available")
	}

	// Error message should mention retries and no cache
	expectedSubstrings := []string{"failed to fetch from URL after", "retries", "no valid cache"}
	for _, substr := range expectedSubstrings {
		if !contains(err.Error(), substr) {
			t.Errorf("Error message should contain '%s', got: %s", substr, err.Error())
		}
	}
}

func TestSubscriptionLoader_InvalidBase64WithCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid-base64-data!!!"))
	}))
	defer server.Close()

	// Create cache file with test data for fallback
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")
	testServers := `[{"id":"test","name":"Test Server","address":"127.0.0.3","port":8080,"protocol":"vless"}]`
	err := os.WriteFile(cacheFile, []byte(testServers), 0644)
	if err != nil {
		t.Fatalf("Failed to create cache file: %v", err)
	}

	cfg := &config.Config{
		SubscriptionURL: server.URL,
		CacheDuration:   3600,
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// Should fallback to cache when decoding fails
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

func TestSubscriptionLoader_CachePersistence(t *testing.T) {
	// Test that cache is properly saved and loaded from file system
	base64Data := "dmxlc3M6Ly9lYzgyYmNhOC0xMDcyLTQ2ODItODIyZi0zMDMwNmFmNDA4ZWFAMTI3LjAuMDM6ODA4MD90eXBlPXRjcCZzZWN1cml0eT1ub25lI1Rlc3QlMjBTZXJ2ZXI=" // base64 encoded VLESS URL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(base64Data))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")

	cfg := &config.Config{
		SubscriptionURL: server.URL,
		CacheDuration:   3600,
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// First load - should fetch from URL and save to cache
	servers1, err := loader.LoadFromURL()
	if err != nil {
		t.Fatalf("First LoadFromURL failed: %v", err)
	}

	// Verify cache file exists
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Fatal("Cache file should exist after first load")
	}

	// Create new loader instance to simulate restart
	loader2 := NewSubscriptionLoader(cfg)
	loader2.cacheFile = cacheFile

	// Stop the server to force cache usage
	server.Close()

	// Second load - should use cache file
	servers2, err := loader2.LoadFromURL()
	if err != nil {
		t.Fatalf("Second LoadFromURL should succeed with cache: %v", err)
	}

	// Verify both loads return same data
	if len(servers1) != len(servers2) {
		t.Fatalf("Server count mismatch: %d vs %d", len(servers1), len(servers2))
	}

	if servers1[0].Name != servers2[0].Name {
		t.Errorf("Server name mismatch: %s vs %s", servers1[0].Name, servers2[0].Name)
	}
}

func TestSubscriptionLoader_CacheExpiration(t *testing.T) {
	base64Data := "dmxlc3M6Ly9lYzgyYmNhOC0xMDcyLTQ2ODItODIyZi0zMDMwNmFmNDA4ZWFAMTI3LjAuMDM6ODA4MD90eXBlPXRjcCZzZWN1cml0eT1ub25lI1Rlc3QlMjBTZXJ2ZXI="

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(base64Data))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "servers.json")

	cfg := &config.Config{
		SubscriptionURL: server.URL,
		CacheDuration:   1, // 1 second cache duration
		PingTimeout:     1,
	}
	loader := NewSubscriptionLoader(cfg)
	loader.cacheFile = cacheFile

	// First load
	_, err := loader.LoadFromURL()
	if err != nil {
		t.Fatalf("First LoadFromURL failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	// Second load immediately - should use cache
	_, err = loader.LoadFromURL()
	if err != nil {
		t.Fatalf("Second LoadFromURL failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected still 1 request (cache hit), got %d", requestCount)
	}

	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond)

	// Third load - should fetch from URL again
	_, err = loader.LoadFromURL()
	if err != nil {
		t.Fatalf("Third LoadFromURL failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests (cache expired), got %d", requestCount)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr))))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
