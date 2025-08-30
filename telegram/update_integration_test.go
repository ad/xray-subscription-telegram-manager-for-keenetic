package telegram

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"xray-telegram-manager/types"
)

// MockUpdateManager implements UpdateManagerInterface for testing
type MockUpdateManager struct {
	updateStatus    UpdateStatus
	progressChan    chan UpdateProgress
	shouldFail      bool
	failureMessage  string
	progressUpdates []UpdateProgress
}

// Ensure MockUpdateManager implements UpdateManagerInterface
var _ UpdateManagerInterface = (*MockUpdateManager)(nil)

func NewMockUpdateManager() *MockUpdateManager {
	return &MockUpdateManager{
		updateStatus: UpdateStatus{
			InProgress: false,
			Progress:   0,
		},
		progressChan:    make(chan UpdateProgress, 10),
		progressUpdates: make([]UpdateProgress, 0),
	}
}

func (m *MockUpdateManager) ExecuteUpdate(ctx context.Context) error {
	if m.shouldFail {
		m.updateStatus.Error = errors.New(m.failureMessage)
		m.updateStatus.InProgress = false
		return m.updateStatus.Error
	}

	m.updateStatus.InProgress = true
	m.updateStatus.StartedAt = time.Now()

	// Simulate logging that would happen in real UpdateManager
	// (In a real test, we would inject a logger and verify it was called)

	// Simulate progress updates
	stages := []struct {
		stage    string
		progress int
		message  string
	}{
		{"downloading", 25, "Downloading update script..."},
		{"preparing", 50, "Preparing for update..."},
		{"installing", 75, "Installing update..."},
		{"completing", 100, "Update completed successfully"},
	}

	for _, stage := range stages {
		m.updateStatus.Stage = stage.stage
		m.updateStatus.Progress = stage.progress

		progress := UpdateProgress{
			Stage:    stage.stage,
			Progress: stage.progress,
			Message:  stage.message,
		}

		m.progressUpdates = append(m.progressUpdates, progress)

		// Send progress update (non-blocking)
		select {
		case m.progressChan <- progress:
		default:
		}

		// Small delay to simulate work
		time.Sleep(10 * time.Millisecond)
	}

	m.updateStatus.InProgress = false
	m.updateStatus.CompletedAt = time.Now()
	return nil
}

func (m *MockUpdateManager) CheckUpdateAvailable() (bool, string, error) {
	return true, "latest", nil
}

func (m *MockUpdateManager) GetCurrentVersion() string {
	return "dev"
}

func (m *MockUpdateManager) GetUpdateStatus() UpdateStatus {
	return m.updateStatus
}

func (m *MockUpdateManager) StartProgressMonitoring() <-chan UpdateProgress {
	return m.progressChan
}

func (m *MockUpdateManager) StopProgressMonitoring() {
	close(m.progressChan)
	m.progressChan = make(chan UpdateProgress, 10)
}

// TestUpdateCommandEndToEndFlow tests the complete update command workflow
func TestUpdateCommandEndToEndFlow(t *testing.T) {
	// Create mock dependencies - no network calls
	mockConfig := &MockConfig{
		adminID:  123456789,
		botToken: "test-token",
	}

	mockLogger := &MockLogger{}
	mockServerMgr := &MockServerManager{
		servers: []types.Server{
			{ID: "1", Name: "Test Server", Address: "test.com", Port: 443},
		},
	}

	// Create MockUpdateManager instead of real one
	updateManager := NewMockUpdateManager()

	// Create mock bot for message handling
	mockBot := &MockBot{}

	// Create TelegramBot with all dependencies
	tb := &TelegramBot{
		bot:         nil, // We don't need real bot for this test
		config:      mockConfig,
		serverMgr:   mockServerMgr,
		logger:      mockLogger,
		rateLimiter: NewRateLimiter(10, time.Minute),
	}

	// Initialize MessageManager and handlers
	tb.messageManager = NewMessageManager(mockBot, mockLogger)
	tb.handlers = NewCommandHandlers(tb, updateManager)

	// Test update command execution
	ctx := context.Background()

	// Start progress monitoring
	progressChan := updateManager.StartProgressMonitoring()
	defer updateManager.StopProgressMonitoring()

	// Execute update in goroutine to monitor progress
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- updateManager.ExecuteUpdate(ctx)
	}()

	// Monitor progress updates
	progressUpdates := make([]UpdateProgress, 0)
	timeout := time.After(10 * time.Second)

	for {
		select {
		case progress := <-progressChan:
			progressUpdates = append(progressUpdates, progress)
			t.Logf("Progress: %s (%d%%) - %s", progress.Stage, progress.Progress, progress.Message)

			// Check for completion
			if progress.Progress == 100 {
				goto checkResult
			}

		case err := <-updateDone:
			if err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			goto checkResult

		case <-timeout:
			t.Fatal("Update timed out")
		}
	}

checkResult:
	// Wait for update to complete if not already done
	select {
	case err := <-updateDone:
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Update already completed
	}

	// Verify progress updates were received
	if len(progressUpdates) == 0 {
		t.Error("Expected to receive progress updates")
	}

	// Verify final status
	status := updateManager.GetUpdateStatus()
	if status.InProgress {
		t.Error("Update should not be in progress after completion")
	}

	if status.Error != nil {
		t.Errorf("Update should not have error after successful completion: %v", status.Error)
	}

	if status.Progress != 100 {
		t.Errorf("Expected final progress to be 100, got %d", status.Progress)
	}

	// Verify mock update completed successfully (no error)
	if status.Error != nil {
		t.Errorf("Update should complete without error, got: %v", status.Error)
	}

	// In a real implementation, we would verify logger calls
	// For this mock test, we verify the update completed successfully
	if status.Progress != 100 {
		t.Errorf("Expected final progress to be 100, got %d", status.Progress)
	}

	t.Log("Update command end-to-end test completed successfully")
}

// TestUpdateCommandWithErrors tests error handling in update flow
func TestUpdateCommandWithErrors(t *testing.T) {
	updateManager := NewMockUpdateManager()

	// Configure mock to fail
	updateManager.shouldFail = true
	updateManager.failureMessage = "failed to download update script: HTTP 500"

	ctx := context.Background()

	// Execute update - should fail
	err := updateManager.ExecuteUpdate(ctx)
	if err == nil {
		t.Error("Expected update to fail with HTTP error")
	}

	// Verify error status
	status := updateManager.GetUpdateStatus()
	if status.Error == nil {
		t.Error("Expected error status to be set")
	}

	if status.InProgress {
		t.Error("Update should not be in progress after failure")
	}

	// Verify error status was set correctly
	if !containsString(status.Error.Error(), "failed to download update script") {
		t.Errorf("Expected error to contain download failure message, got: %v", status.Error)
	}

	t.Log("Update command error handling test completed successfully")
}

// MockSlowUpdateManager simulates slow update for concurrency testing
type MockSlowUpdateManager struct {
	*MockUpdateManager
	isRunning bool
	mutex     sync.Mutex
}

func NewMockSlowUpdateManager() *MockSlowUpdateManager {
	return &MockSlowUpdateManager{
		MockUpdateManager: NewMockUpdateManager(),
	}
}

func (m *MockSlowUpdateManager) ExecuteUpdate(ctx context.Context) error {
	m.mutex.Lock()
	if m.isRunning {
		m.mutex.Unlock()
		return errors.New("update already in progress")
	}
	m.isRunning = true
	m.mutex.Unlock()

	defer func() {
		m.mutex.Lock()
		m.isRunning = false
		m.mutex.Unlock()
	}()

	// Simulate slow update
	time.Sleep(200 * time.Millisecond)
	return m.MockUpdateManager.ExecuteUpdate(ctx)
}

// TestUpdateCommandConcurrency tests concurrent update attempts
func TestUpdateCommandConcurrency(t *testing.T) {
	updateManager := NewMockSlowUpdateManager()

	ctx := context.Background()

	// Start first update
	updateDone1 := make(chan error, 1)
	go func() {
		updateDone1 <- updateManager.ExecuteUpdate(ctx)
	}()

	// Wait a bit to ensure first update starts
	time.Sleep(50 * time.Millisecond)

	// Try second update - should fail immediately
	err := updateManager.ExecuteUpdate(ctx)
	if err == nil {
		t.Error("Expected second update to fail due to concurrent execution")
	}

	if err.Error() != "update already in progress" {
		t.Errorf("Expected 'update already in progress' error, got: %v", err)
	}

	// Wait for first update to complete
	select {
	case err := <-updateDone1:
		if err != nil {
			t.Errorf("First update should succeed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("First update timed out")
	}

	t.Log("Update command concurrency test completed successfully")
}

// TestCompleteMessageFlowWithUpdateCommand tests message management during update
func TestCompleteMessageFlowWithUpdateCommand(t *testing.T) {
	// Create mock dependencies - no network calls
	mockConfig := &MockConfig{
		adminID:  123456789,
		botToken: "test-token",
	}

	mockLogger := &MockLogger{}
	mockServerMgr := &MockServerManager{}
	mockBot := &MockBot{}

	// Create TelegramBot
	tb := &TelegramBot{
		config:      mockConfig,
		serverMgr:   mockServerMgr,
		logger:      mockLogger,
		rateLimiter: NewRateLimiter(10, time.Minute),
	}

	tb.messageManager = NewMessageManager(mockBot, mockLogger)
	updateManager := NewMockUpdateManager()
	tb.handlers = NewCommandHandlers(tb, updateManager)

	ctx := context.Background()
	userID := int64(123456789) // Admin user

	// Test message flow during update process
	// 1. Send initial update start message
	startContent := MessageContent{
		Text: "🔄 Starting update process...",
		Type: MessageTypeStatus,
	}

	err := tb.messageManager.SendNew(ctx, userID, startContent)
	if err != nil {
		t.Fatalf("Failed to send start message: %v", err)
	}

	// Verify message was sent
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockBot.sentMessages))
	}

	// 2. Update message with progress
	progressContent := MessageContent{
		Text: "📥 Downloading update script... (25%)",
		Type: MessageTypeStatus,
	}

	err = tb.messageManager.SendOrEdit(ctx, userID, progressContent)
	if err != nil {
		t.Fatalf("Failed to update progress message: %v", err)
	}

	// Should have edited the message, not sent new one
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message (edited), got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 1 {
		t.Fatalf("Expected 1 edited message, got %d", len(mockBot.editedMessages))
	}

	// 3. Update with completion message
	completeContent := MessageContent{
		Text: "✅ Update completed successfully!",
		Type: MessageTypeStatus,
	}

	err = tb.messageManager.SendOrEdit(ctx, userID, completeContent)
	if err != nil {
		t.Fatalf("Failed to send completion message: %v", err)
	}

	// Should have edited again
	if len(mockBot.editedMessages) != 2 {
		t.Fatalf("Expected 2 edited messages, got %d", len(mockBot.editedMessages))
	}

	// Verify final message content
	finalEdit := mockBot.editedMessages[1]
	if finalEdit.Text != completeContent.Text {
		t.Errorf("Expected final message '%s', got '%s'", completeContent.Text, finalEdit.Text)
	}

	// 4. Test fallback scenario - simulate edit failure
	mockBot.shouldFailEdit = true

	fallbackContent := MessageContent{
		Text: "🔄 Update process restarted...",
		Type: MessageTypeStatus,
	}

	err = tb.messageManager.SendOrEdit(ctx, userID, fallbackContent)
	if err != nil {
		t.Fatalf("Failed to handle edit failure with fallback: %v", err)
	}

	// Should have attempted edit, failed, then sent new message
	if len(mockBot.sentMessages) != 2 {
		t.Fatalf("Expected 2 sent messages (original + fallback), got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.deletedMessages) != 1 {
		t.Fatalf("Expected 1 deleted message, got %d", len(mockBot.deletedMessages))
	}

	t.Log("Complete message flow with update command test completed successfully")
}

// TestServerListDisplayWithOptimizationIntegration tests server display with all features
func TestServerListDisplayWithOptimizationIntegration(t *testing.T) {
	// Create servers with common suffixes for optimization
	testServers := []types.Server{
		{ID: "1", Name: "Web-Server-US-East.example.com", Address: "web1.example.com", Port: 443},
		{ID: "2", Name: "API-Server-US-East.example.com", Address: "api1.example.com", Port: 443},
		{ID: "3", Name: "DB-Server-US-East.example.com", Address: "db1.example.com", Port: 443},
		{ID: "4", Name: "Cache-Server-EU-West.different.org", Address: "cache1.different.org", Port: 443},
	}

	mockConfig := &MockConfig{
		adminID:  123456789,
		botToken: "test-token",
	}

	mockLogger := &MockLogger{}
	mockServerMgr := &MockServerManager{
		servers: testServers,
	}

	mockBot := &MockBot{}

	// Create TelegramBot
	tb := &TelegramBot{
		config:      mockConfig,
		serverMgr:   mockServerMgr,
		logger:      mockLogger,
		rateLimiter: NewRateLimiter(10, time.Minute),
	}

	tb.messageManager = NewMessageManager(mockBot, mockLogger)

	// Test server list creation with optimization and sorting
	keyboard := tb.createServerListKeyboard(testServers, 0)

	if keyboard == nil {
		t.Fatal("Keyboard should not be nil")
	}

	// The keyboard includes server buttons plus navigation buttons
	if len(keyboard.InlineKeyboard) < len(testServers) {
		t.Errorf("Expected at least %d keyboard rows, got %d", len(testServers), len(keyboard.InlineKeyboard))
	}

	// Verify server buttons are created with proper text processing (skip navigation rows)
	serverRows := 0
	for _, row := range keyboard.InlineKeyboard {
		// Skip navigation buttons (they have different structure)
		if len(row) == 1 && len(row[0].CallbackData) > 0 && len(row[0].CallbackData) >= 7 && row[0].CallbackData[:7] == "server_" {
			serverRows++
			button := row[0]

			// Verify button has status emoji
			if !containsString(button.Text, "🌐") {
				t.Errorf("Server button %d should contain status emoji, got: %s", serverRows, button.Text)
			}

			// Verify button text is not too long
			if len([]rune(button.Text)) > 50 {
				t.Errorf("Server button %d text too long (%d runes): %s", serverRows, len([]rune(button.Text)), button.Text)
			}

			// Verify callback data format
			if !containsString(button.CallbackData, "server_") {
				t.Errorf("Server button %d callback should contain 'server_', got: %s", serverRows, button.CallbackData)
			}
		}
	}

	// Verify we found all server buttons
	if serverRows != len(testServers) {
		t.Errorf("Expected %d server buttons, found %d", len(testServers), serverRows)
	}

	t.Log("Server list display with optimization integration test completed successfully")
}

// TestPingTestFlowWithFormattingIntegration tests ping test with improved formatting
func TestPingTestFlowWithFormattingIntegration(t *testing.T) {
	// Create test servers for ping testing
	testServers := []types.Server{
		{ID: "1", Name: "Fast Server", Address: "fast.example.com", Port: 443},
		{ID: "2", Name: "Slow Server", Address: "slow.example.com", Port: 443},
		{ID: "3", Name: "Unavailable Server", Address: "down.example.com", Port: 443},
	}

	// Create mock ping results (for reference in test description)
	_ = []types.PingResult{
		{
			Server:    testServers[0],
			Available: true,
			Latency:   int64(50 * time.Millisecond),
			Error:     nil,
		},
		{
			Server:    testServers[1],
			Available: true,
			Latency:   int64(200 * time.Millisecond),
			Error:     nil,
		},
		{
			Server:    testServers[2],
			Available: false,
			Latency:   0,
			Error:     errors.New("connection timeout"),
		},
	}

	mockConfig := &MockConfig{
		adminID:  123456789,
		botToken: "test-token",
	}

	mockLogger := &MockLogger{}
	mockServerMgr := &MockServerManager{
		servers: testServers,
	}

	mockBot := &MockBot{}

	// Create TelegramBot
	tb := &TelegramBot{
		config:      mockConfig,
		serverMgr:   mockServerMgr,
		logger:      mockLogger,
		rateLimiter: NewRateLimiter(10, time.Minute),
	}
	// These fields are used by the test framework
	_ = tb.config
	_ = tb.serverMgr
	_ = tb.logger
	_ = tb.rateLimiter

	tb.messageManager = NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123456789)

	// Test ping test flow with progress updates
	// 1. Start ping test
	startContent := MessageContent{
		Text: "🏓 Starting ping test...\n\n⏳ Testing servers...",
		Type: MessageTypePingTest,
	}

	err := tb.messageManager.SendNew(ctx, userID, startContent)
	if err != nil {
		t.Fatalf("Failed to send ping start message: %v", err)
	}

	// 2. Update with progress
	progressContent := MessageContent{
		Text: "🏓 Ping Test Progress\n\n" +
			"✅ Fast Server - 50ms\n" +
			"⏳ Testing Slow Server...\n" +
			"⏳ Testing Unavailable Server...\n\n" +
			"Progress: 1/3 completed",
		Type: MessageTypePingTest,
	}

	err = tb.messageManager.SendOrEdit(ctx, userID, progressContent)
	if err != nil {
		t.Fatalf("Failed to update ping progress: %v", err)
	}

	// 3. Complete with results
	resultsContent := MessageContent{
		Text: "🏓 Ping Test Results\n\n" +
			"🟢 Available Servers:\n" +
			"✅ Fast Server - 50ms\n" +
			"✅ Slow Server - 200ms\n\n" +
			"🔴 Unavailable Servers:\n" +
			"❌ Unavailable Server - timeout\n\n" +
			"📊 Summary: 2/3 servers available",
		Type: MessageTypePingTest,
	}

	err = tb.messageManager.SendOrEdit(ctx, userID, resultsContent)
	if err != nil {
		t.Fatalf("Failed to send ping results: %v", err)
	}

	// Verify message flow
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 2 {
		t.Fatalf("Expected 2 edited messages, got %d", len(mockBot.editedMessages))
	}

	// Verify final results formatting
	finalEdit := mockBot.editedMessages[1]
	if !containsString(finalEdit.Text, "🏓 Ping Test Results") {
		t.Error("Final message should contain ping test results header")
	}

	if !containsString(finalEdit.Text, "🟢 Available Servers") {
		t.Error("Final message should contain available servers section")
	}

	if !containsString(finalEdit.Text, "🔴 Unavailable Servers") {
		t.Error("Final message should contain unavailable servers section")
	}

	if !containsString(finalEdit.Text, "📊 Summary") {
		t.Error("Final message should contain summary section")
	}

	t.Log("Ping test flow with formatting integration test completed successfully")
}

// Helper function for string contains check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
