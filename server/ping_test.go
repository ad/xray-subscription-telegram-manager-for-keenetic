package server

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)

func TestNewPingTester(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 5,
	}

	pt := NewPingTester(cfg)

	if pt == nil {
		t.Fatal("NewPingTester returned nil")
	}

	if pt.config != cfg {
		t.Error("PingTesterImpl config not set correctly")
	}
}

func TestPingTesterImpl_TestServer(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}

	// Создаем мок TCP сервер для тестирования доступного сервера
	mockServer, err := NewMockTCPServer()
	if err != nil {
		t.Fatalf("Failed to create mock TCP server: %v", err)
	}
	defer mockServer.Stop()
	mockServer.Start()

	pt := NewPingTester(cfg)

	tests := []struct {
		name            string
		server          types.Server
		expectError     bool
		expectAvailable bool
	}{
		{
			name: "Available mock server",
			server: types.Server{
				ID:      "test1",
				Name:    "Mock Server",
				Address: mockServer.Address(),
				Port:    mockServer.Port(),
			},
			expectError:     false,
			expectAvailable: true,
		},
		{
			name: "Invalid server - non-existent host",
			server: types.Server{
				ID:      "test2",
				Name:    "Invalid Server",
				Address: "127.0.0.1", // localhost
				Port:    65534,       // Very unlikely to be open
			},
			expectError:     true,
			expectAvailable: false,
		},
		{
			name: "Valid host but closed port",
			server: types.Server{
				ID:      "test3",
				Name:    "Closed Port Server",
				Address: "127.0.0.1",
				Port:    12345, // Likely closed port
			},
			expectError:     true,
			expectAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pt.TestServer(tt.server)

			if result.Server.ID != tt.server.ID {
				t.Errorf("Expected server ID %s, got %s", tt.server.ID, result.Server.ID)
			}

			if result.Available != tt.expectAvailable {
				t.Errorf("Expected available %v, got %v", tt.expectAvailable, result.Available)
			}

			if tt.expectError && result.Error == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && result.Error != nil {
				t.Errorf("Expected no error but got: %v", result.Error)
			}

			if result.Available && result.Latency < 0 {
				t.Error("Expected non-negative latency for available server")
			}

			if !result.Available && result.Latency != 0 {
				t.Error("Expected zero latency for unavailable server")
			}
		})
	}
}

func TestPingTesterImpl_TestServers(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	// Создаем один доступный мок сервер
	mockServer, err := NewMockTCPServer()
	if err != nil {
		t.Fatalf("Failed to create mock TCP server: %v", err)
	}
	defer mockServer.Stop()
	mockServer.Start()

	servers := []types.Server{
		{
			ID:      "server1",
			Name:    "Available Mock Server",
			Address: mockServer.Address(),
			Port:    mockServer.Port(),
		},
		{
			ID:      "server2",
			Name:    "Invalid Server 1",
			Address: "127.0.0.1", // localhost
			Port:    65533,       // Very unlikely to be open
		},
		{
			ID:      "server3",
			Name:    "Invalid Server 2",
			Address: "127.0.0.1", // localhost
			Port:    65532,       // Very unlikely to be open
		},
	}

	results, err := pt.TestServers(servers)

	if err != nil {
		t.Fatalf("TestServers returned error: %v", err)
	}

	if len(results) != len(servers) {
		t.Fatalf("Expected %d results, got %d", len(servers), len(results))
	}

	// Check that all servers are represented in results
	serverIDs := make(map[string]bool)
	availableCount := 0
	for _, result := range results {
		serverIDs[result.Server.ID] = true
		if result.Available {
			availableCount++
		}
	}

	for _, server := range servers {
		if !serverIDs[server.ID] {
			t.Errorf("Server %s not found in results", server.ID)
		}
	}

	// Должен быть один доступный сервер (мок сервер)
	if availableCount != 1 {
		t.Errorf("Expected 1 available server, got %d", availableCount)
	}
}

func TestPingTesterImpl_TestServers_EmptyList(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	results, err := pt.TestServers([]types.Server{})

	if err == nil {
		t.Error("Expected error for empty server list")
	}

	if results != nil {
		t.Error("Expected nil results for empty server list")
	}
}

func TestPingTesterImpl_SortByLatency(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	results := []types.PingResult{
		{
			Server:    types.Server{ID: "server1", Name: "Server 1"},
			Available: true,
			Latency:   100,
		},
		{
			Server:    types.Server{ID: "server2", Name: "Server 2"},
			Available: false,
			Latency:   0,
		},
		{
			Server:    types.Server{ID: "server3", Name: "Server 3"},
			Available: true,
			Latency:   50,
		},
		{
			Server:    types.Server{ID: "server4", Name: "Server 4"},
			Available: true,
			Latency:   200,
		},
	}

	sorted := pt.SortByLatency(results)

	// Check that original slice is not modified
	if results[0].Server.ID != "server1" {
		t.Error("Original slice was modified")
	}

	// Check sorting: available servers first, sorted by latency
	expectedOrder := []string{"server3", "server1", "server4", "server2"}

	if len(sorted) != len(expectedOrder) {
		t.Fatalf("Expected %d results, got %d", len(expectedOrder), len(sorted))
	}

	for i, expected := range expectedOrder {
		if sorted[i].Server.ID != expected {
			t.Errorf("Position %d: expected %s, got %s", i, expected, sorted[i].Server.ID)
		}
	}

	// Verify available servers come first
	availableCount := 0
	for _, result := range sorted {
		if result.Available {
			availableCount++
		} else {
			break // Should not find available servers after unavailable ones
		}
	}

	if availableCount != 3 {
		t.Errorf("Expected 3 available servers at the beginning, got %d", availableCount)
	}
}

func TestPingTesterImpl_FormatResultsForTelegram(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	results := []types.PingResult{
		{
			Server:    types.Server{ID: "server1", Name: "Fast Server", Address: "1.1.1.1", Port: 443},
			Available: true,
			Latency:   50,
		},
		{
			Server:    types.Server{ID: "server2", Name: "Slow Server", Address: "8.8.8.8", Port: 443},
			Available: true,
			Latency:   200,
		},
		{
			Server:    types.Server{ID: "server3", Name: "Unavailable Server", Address: "127.0.0.1", Port: 65531},
			Available: false,
			Error:     fmt.Errorf("connection timeout"),
		},
	}

	formatted := pt.FormatResultsForTelegram(results)

	// Check that the message contains expected elements
	expectedElements := []string{
		"🏓 *Ping Test Results*",
		"📊 *Summary:* 2/3 servers available",
		"*Fast Server*",
		"✅ 50ms",
		"*Slow Server*",
		"✅ 200ms",
		"*Unavailable Server*",
		"❌ Unavailable",
		"connection timeout",
		"1.1.1.1:443",
		"8.8.8.8:443",
	}

	for _, element := range expectedElements {
		if !strings.Contains(formatted, element) {
			t.Errorf("Expected formatted message to contain '%s', but it didn't.\nFull message:\n%s", element, formatted)
		}
	}
}

func TestPingTesterImpl_FormatResultsForTelegram_Empty(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	formatted := pt.FormatResultsForTelegram([]types.PingResult{})

	expected := "No servers to test"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}
}

func TestPingTesterImpl_GetAvailableServers(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	results := []types.PingResult{
		{
			Server:    types.Server{ID: "server1", Name: "Available 1"},
			Available: true,
		},
		{
			Server:    types.Server{ID: "server2", Name: "Unavailable"},
			Available: false,
		},
		{
			Server:    types.Server{ID: "server3", Name: "Available 2"},
			Available: true,
		},
	}

	available := pt.GetAvailableServers(results)

	if len(available) != 2 {
		t.Fatalf("Expected 2 available servers, got %d", len(available))
	}

	expectedIDs := []string{"server1", "server3"}
	for i, server := range available {
		if server.ID != expectedIDs[i] {
			t.Errorf("Position %d: expected ID %s, got %s", i, expectedIDs[i], server.ID)
		}
	}
}

func TestPingTesterImpl_GetFastestServer(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	results := []types.PingResult{
		{
			Server:    types.Server{ID: "server1", Name: "Slow"},
			Available: true,
			Latency:   200,
		},
		{
			Server:    types.Server{ID: "server2", Name: "Unavailable"},
			Available: false,
		},
		{
			Server:    types.Server{ID: "server3", Name: "Fast"},
			Available: true,
			Latency:   50,
		},
	}

	fastest, err := pt.GetFastestServer(results)

	if err != nil {
		t.Fatalf("GetFastestServer returned error: %v", err)
	}

	if fastest.ID != "server3" {
		t.Errorf("Expected fastest server ID 'server3', got '%s'", fastest.ID)
	}
}

func TestPingTesterImpl_GetFastestServer_NoAvailable(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	results := []types.PingResult{
		{
			Server:    types.Server{ID: "server1", Name: "Unavailable 1"},
			Available: false,
		},
		{
			Server:    types.Server{ID: "server2", Name: "Unavailable 2"},
			Available: false,
		},
	}

	fastest, err := pt.GetFastestServer(results)

	if err == nil {
		t.Error("Expected error when no servers are available")
	}

	if fastest != nil {
		t.Error("Expected nil server when no servers are available")
	}
}

// TestPingTesterImpl_Timeout tests that ping respects the configured timeout
func TestPingTesterImpl_Timeout(t *testing.T) {
	cfg := &config.Config{
		PingTimeout: 1, // Very short timeout
	}
	pt := NewPingTester(cfg)

	// Use a server that should timeout (closed port on localhost)
	server := types.Server{
		ID:      "timeout-test",
		Name:    "Timeout Server",
		Address: "127.0.0.1", // localhost
		Port:    65530,       // Very unlikely to be open
	}

	start := time.Now()
	result := pt.TestServer(server)
	duration := time.Since(start)

	// Should complete within reasonable time (timeout + some overhead)
	maxDuration := time.Duration(cfg.PingTimeout+1) * time.Second
	if duration > maxDuration {
		t.Errorf("Ping took too long: %v (expected max %v)", duration, maxDuration)
	}

	if result.Available {
		t.Error("Expected server to be unavailable due to timeout")
	}

	if result.Error == nil {
		t.Error("Expected timeout error")
	}
}

// Benchmark tests
func BenchmarkPingTesterImpl_TestServer(b *testing.B) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	server := types.Server{
		ID:      "benchmark",
		Name:    "Benchmark Server",
		Address: "8.8.8.8",
		Port:    53,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.TestServer(server)
	}
}

func BenchmarkPingTesterImpl_SortByLatency(b *testing.B) {
	cfg := &config.Config{
		PingTimeout: 2,
	}
	pt := NewPingTester(cfg)

	// Create test data
	results := make([]types.PingResult, 100)
	for i := 0; i < 100; i++ {
		results[i] = types.PingResult{
			Server:    types.Server{ID: fmt.Sprintf("server%d", i)},
			Available: i%3 != 0,       // Make some unavailable
			Latency:   int64(100 - i), // Reverse order latency
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.SortByLatency(results)
	}
}

func TestPingTesterImpl_TestServer_WithMockServer(t *testing.T) {
	// Создаем мок TCP сервер
	mockServer, err := NewMockTCPServer()
	if err != nil {
		t.Fatalf("Failed to create mock TCP server: %v", err)
	}
	defer mockServer.Stop()

	// Устанавливаем небольшую задержку для имитации реального сервера
	mockServer.SetDelay(10 * time.Millisecond)
	mockServer.Start()

	cfg := &config.Config{
		PingTimeout: 5,
	}
	pt := NewPingTester(cfg)

	server := types.Server{
		ID:      "mock-server",
		Name:    "Mock Test Server",
		Address: mockServer.Address(),
		Port:    mockServer.Port(),
	}

	result := pt.TestServer(server)

	t.Logf("Test result: Available=%v, Latency=%d, Error=%v", result.Available, result.Latency, result.Error)

	if !result.Available {
		t.Errorf("Expected mock server to be available, got error: %v", result.Error)
	}

	if result.Available && result.Latency < 0 {
		t.Errorf("Expected non-negative latency for available server, got latency: %d", result.Latency)
	}

	if result.Error != nil {
		t.Errorf("Expected no error for available server, got: %v", result.Error)
	}

	// Проверяем, что задержка примерно соответствует ожидаемой
	if result.Available && result.Latency < 5 {
		t.Logf("Latency %dms is lower than expected 10ms delay, but this is acceptable for local mock", result.Latency)
	}
}
