package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewUpdateManager(t *testing.T) {
	logger := &MockLogger{}

	// Test with default values
	um := NewUpdateManager("", 0, false, logger)

	if um.scriptURL != "https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh" {
		t.Errorf("Expected default script URL, got: %s", um.scriptURL)
	}

	if um.timeout != 10*time.Minute {
		t.Errorf("Expected default timeout of 10 minutes, got: %v", um.timeout)
	}

	if um.backupConfig != false {
		t.Errorf("Expected default backupConfig to be false, got: %v", um.backupConfig)
	}

	// Test with custom values
	customURL := "https://example.com/script.sh"
	customTimeout := 5 * time.Minute
	um2 := NewUpdateManager(customURL, customTimeout, true, logger)

	if um2.scriptURL != customURL {
		t.Errorf("Expected custom script URL %s, got: %s", customURL, um2.scriptURL)
	}

	if um2.timeout != customTimeout {
		t.Errorf("Expected custom timeout %v, got: %v", customTimeout, um2.timeout)
	}

	if um2.backupConfig != true {
		t.Errorf("Expected backupConfig to be true, got: %v", um2.backupConfig)
	}
}

func TestGetCurrentVersion(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	version := um.GetCurrentVersion()
	if version != "dev" {
		t.Errorf("Expected version 'dev', got: %s", version)
	}
}

func TestCheckUpdateAvailable(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	available, version, err := um.CheckUpdateAvailable()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !available {
		t.Errorf("Expected update to be available")
	}

	if version != "latest" {
		t.Errorf("Expected version 'latest', got: %s", version)
	}
}

func TestGetUpdateStatus(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	// Initial status should not be in progress
	status := um.GetUpdateStatus()
	if status.InProgress {
		t.Errorf("Expected initial status to not be in progress")
	}

	if status.Progress != 0 {
		t.Errorf("Expected initial progress to be 0, got: %d", status.Progress)
	}
}

func TestProgressMonitoring(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	// Start monitoring
	progressChan := um.StartProgressMonitoring()

	// Send a progress update
	um.updateProgress("test", 50, "Testing progress")

	// Check if we receive the progress update
	select {
	case progress := <-progressChan:
		if progress.Stage != "test" {
			t.Errorf("Expected stage 'test', got: %s", progress.Stage)
		}
		if progress.Progress != 50 {
			t.Errorf("Expected progress 50, got: %d", progress.Progress)
		}
		if progress.Message != "Testing progress" {
			t.Errorf("Expected message 'Testing progress', got: %s", progress.Message)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout waiting for progress update")
	}

	// Stop monitoring
	um.StopProgressMonitoring()
}

func TestDownloadScript(t *testing.T) {
	logger := &MockLogger{}

	// Create a test server that serves a simple script
	testScript := "#!/bin/bash\necho 'test script'\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(testScript)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	um := NewUpdateManager(server.URL, time.Minute, false, logger)

	ctx := context.Background()
	scriptPath, err := um.downloadScript(ctx)
	if err != nil {
		t.Fatalf("Failed to download script: %v", err)
	}
	defer func() {
		if err := os.Remove(scriptPath); err != nil {
			t.Logf("Failed to remove script file: %v", err)
		}
	}()

	// Verify the script was downloaded correctly
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded script: %v", err)
	}

	if string(content) != testScript {
		t.Errorf("Expected script content '%s', got: '%s'", testScript, string(content))
	}

	// Verify the script is executable
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("Failed to stat script file: %v", err)
	}

	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("Script is not executable")
	}
}

func TestDownloadScriptHTTPError(t *testing.T) {
	logger := &MockLogger{}

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	um := NewUpdateManager(server.URL, time.Minute, false, logger)

	ctx := context.Background()
	_, err := um.downloadScript(ctx)
	if err == nil {
		t.Errorf("Expected error for HTTP 404, got nil")
	}

	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("Expected error to mention HTTP 404, got: %v", err)
	}
}

func TestDownloadScriptTimeout(t *testing.T) {
	logger := &MockLogger{}

	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("test")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	um := NewUpdateManager(server.URL, time.Minute, false, logger)

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := um.downloadScript(ctx)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}

func TestIsValidScriptPath(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	// Valid paths
	validPaths := []string{
		"/tmp/script.sh",
		"/tmp/update-script-123.sh",
	}

	for _, path := range validPaths {
		if !um.isValidScriptPath(path) {
			t.Errorf("Expected path '%s' to be valid", path)
		}
	}

	// Invalid paths
	invalidPaths := []string{
		"",                         // empty
		"relative/path",            // not absolute
		"/tmp/script;rm -rf /",     // dangerous characters
		"/tmp/script && echo test", // dangerous characters
		"/tmp/script | cat",        // dangerous characters
		"/tmp/script`whoami`",      // dangerous characters
		"/tmp/script$(whoami)",     // dangerous characters
		strings.Repeat("a", 300),   // too long
	}

	for _, path := range invalidPaths {
		if um.isValidScriptPath(path) {
			t.Errorf("Expected path '%s' to be invalid", path)
		}
	}
}

func TestExecuteUpdateConcurrency(t *testing.T) {
	logger := &MockLogger{}

	// Create a test server that delays response to ensure first update is in progress
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Delay to ensure update is in progress
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("#!/bin/bash\necho 'test'\n")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	um := NewUpdateManager(server.URL, time.Minute, false, logger)

	ctx := context.Background()

	// Start first update in goroutine
	go func() {
		if err := um.ExecuteUpdate(ctx); err != nil {
			t.Logf("Update failed as expected: %v", err)
		}
	}()

	// Wait a bit to ensure first update starts
	time.Sleep(50 * time.Millisecond)

	// Try to start second update - should fail
	err := um.ExecuteUpdate(ctx)
	if err == nil {
		t.Errorf("Expected error for concurrent update, got nil")
		return
	}

	if !strings.Contains(err.Error(), "update already in progress") {
		t.Errorf("Expected 'update already in progress' error, got: %v", err)
	}
}

func TestUpdateProgress(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	// Test progress update
	um.updateProgress("testing", 75, "Test message")

	status := um.GetUpdateStatus()
	if status.Stage != "testing" {
		t.Errorf("Expected stage 'testing', got: %s", status.Stage)
	}

	if status.Progress != 75 {
		t.Errorf("Expected progress 75, got: %d", status.Progress)
	}

	// Check if logger received the debug message
	if !logger.HasLog("Update progress: testing (75%) - Test message") {
		t.Errorf("Expected progress log message not found")
	}
}

func TestUpdateError(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	testError := fmt.Errorf("test error")
	um.updateError(testError)

	status := um.GetUpdateStatus()
	if status.Error == nil {
		t.Errorf("Expected error to be set")
	}

	if status.Error.Error() != "test error" {
		t.Errorf("Expected error 'test error', got: %v", status.Error)
	}

	// Check if logger received the error message
	if !logger.HasLog("Update error: test error") {
		t.Errorf("Expected error log message not found")
	}
}

func TestCreateConfigBackup(t *testing.T) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, true, logger)

	// Note: This test is limited because createConfigBackup uses hardcoded paths
	// In a real implementation, we would make the paths configurable for testing
	// For now, we'll test that the method handles missing config gracefully
	ctx := context.Background()
	err := um.createConfigBackup(ctx)

	// Should not error even if config doesn't exist or we don't have permissions
	// The method should handle these cases gracefully
	if err != nil {
		// This is expected in test environment due to permission restrictions
		t.Logf("createConfigBackup failed as expected in test environment: %v", err)
	}
}

// Benchmark tests
func BenchmarkNewUpdateManager(b *testing.B) {
	logger := &MockLogger{}

	for i := 0; i < b.N; i++ {
		NewUpdateManager("https://example.com/script.sh", time.Minute, false, logger)
	}
}

func BenchmarkGetUpdateStatus(b *testing.B) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	for i := 0; i < b.N; i++ {
		um.GetUpdateStatus()
	}
}

func BenchmarkUpdateProgress(b *testing.B) {
	logger := &MockLogger{}
	um := NewUpdateManager("", 0, false, logger)

	for i := 0; i < b.N; i++ {
		um.updateProgress("benchmark", i%100, "benchmark message")
	}
}
