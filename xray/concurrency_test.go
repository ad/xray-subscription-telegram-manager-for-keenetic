package xray

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"
	"xray-telegram-manager/types"
)

// TestXrayController_ConcurrentOperations tests concurrent file operations
func TestXrayController_ConcurrentOperations(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := tempDir + "/test_config.json"

	// Create initial config
	initialConfig := types.XrayConfig{
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

	configData, err := json.MarshalIndent(initialConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Create mock config provider
	mockConfig := &mockConfigProvider{
		configPath:         configPath,
		xrayRestartCommand: "echo 'restarted'",
	}

	// Create controller
	controller := NewXrayController(mockConfig)

	// Test concurrent operations
	const numGoroutines = 10
	const operationsPerGoroutine = 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	// Start multiple goroutines performing concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Create a test server
				server := types.Server{
					ID:       "test-server",
					Name:     "Test Server",
					Address:  "127.0.0.1",
					Port:     8080,
					Protocol: "vless",
					Tag:      "proxy",
					Settings: map[string]interface{}{
						"vnext": []map[string]interface{}{
							{
								"address": "127.0.0.1",
								"port":    8080,
								"users": []map[string]interface{}{
									{
										"id":   "test-uuid",
										"flow": "xtls-rprx-vision",
									},
								},
							},
						},
					},
				}

				// Perform update operation
				if err := controller.UpdateConfig(server); err != nil {
					errors <- err
					return
				}

				// Small delay to allow interleaving
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Found %d errors in concurrent operations", errorCount)
	}

	// Verify final state is consistent
	finalConfig, err := controller.GetCurrentConfig()
	if err != nil {
		t.Fatalf("Failed to get final config: %v", err)
	}

	// Should have at least one proxy outbound
	proxyFound := false
	for _, outbound := range finalConfig.Outbounds {
		if outbound.Protocol != "freedom" && outbound.Protocol != "blackhole" {
			proxyFound = true
			break
		}
	}

	if !proxyFound {
		t.Error("No proxy outbound found in final configuration")
	}
}

// TestXrayController_ConcurrentReadWrite tests concurrent read and write operations
func TestXrayController_ConcurrentReadWrite(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := tempDir + "/test_config.json"

	// Create initial config
	initialConfig := types.XrayConfig{
		Outbounds: []types.XrayOutbound{
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
		},
	}

	configData, err := json.MarshalIndent(initialConfig, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Create mock config provider
	mockConfig := &mockConfigProvider{
		configPath:         configPath,
		xrayRestartCommand: "echo 'restarted'",
	}

	// Create controller
	controller := NewXrayController(mockConfig)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Start writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				server := types.Server{
					ID:       "test-server",
					Name:     "Test Server",
					Address:  "127.0.0.1",
					Port:     8080 + writerID,
					Protocol: "vless",
					Tag:      "proxy",
				}

				if err := controller.UpdateConfig(server); err != nil {
					errors <- err
					return
				}

				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Start readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				_, err := controller.GetCurrentConfig()
				if err != nil {
					errors <- err
					return
				}

				time.Sleep(500 * time.Microsecond)
			}
		}()
	}

	// Wait for all operations to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Errorf("Concurrent read/write error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Found %d errors in concurrent read/write operations", errorCount)
	}
}

// mockConfigProvider implements ConfigProvider for testing
type mockConfigProvider struct {
	configPath         string
	xrayRestartCommand string
}

func (m *mockConfigProvider) GetConfigPath() string {
	return m.configPath
}

func (m *mockConfigProvider) GetXrayRestartCommand() string {
	return m.xrayRestartCommand
}
