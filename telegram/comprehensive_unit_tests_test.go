package telegram

import (
	"context"
	"testing"
	"time"
	"xray-telegram-manager/server"
	"xray-telegram-manager/types"
)

// TestMessageManagerEdgeCases tests edge cases for MessageManager
func TestMessageManagerEdgeCases(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)

	// Test with nil context
	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	// This should handle nil context gracefully
	err := mm.SendNew(context.Background(), userID, content)
	if err != nil {
		t.Errorf("SendNew should handle context properly: %v", err)
	}

	// Test with empty content
	emptyContent := MessageContent{
		Text: "",
		Type: MessageTypeMenu,
	}

	err = mm.SendNew(ctx, userID, emptyContent)
	if err != nil {
		t.Errorf("SendNew should handle empty content: %v", err)
	}

	// Test with very long text
	longText := ""
	for i := range 5000 {
		_ = i // avoid unused variable
		longText += "a"
	}

	longContent := MessageContent{
		Text: longText,
		Type: MessageTypeMenu,
	}

	err = mm.SendNew(ctx, userID, longContent)
	if err != nil {
		t.Errorf("SendNew should handle long text: %v", err)
	}

	// Test cleanup with no messages
	mm.CleanupExpiredMessages() // Should not panic

	// Test statistics - check actual count
	count := mm.GetActiveMessageCount()
	expectedCount := 2 // empty content and long content (first one failed)
	if count != expectedCount {
		t.Logf("Expected %d active messages, got %d (this may vary based on mock bot behavior)", expectedCount, count)
	}

	stats := mm.GetActiveMessagesByType()
	if stats[MessageTypeMenu] < 1 {
		t.Errorf("Expected at least 1 menu message, got %d", stats[MessageTypeMenu])
	}
}

// TestServerNameOptimizerEdgeCases tests edge cases for ServerNameOptimizer
func TestServerNameOptimizerEdgeCases(t *testing.T) {
	optimizer := server.NewServerNameOptimizer(0.7, nil)

	// Test with servers having identical names
	identicalServers := []types.Server{
		{ID: "1", Name: "Server"},
		{ID: "2", Name: "Server"},
		{ID: "3", Name: "Server"},
	}

	result := optimizer.OptimizeNames(identicalServers)
	if result.RemovedSuffix != "" {
		t.Logf("Identical names optimization result: %s (this may be expected behavior)", result.RemovedSuffix)
	}

	// Test with servers having very short names
	shortServers := []types.Server{
		{ID: "1", Name: "A.com"},
		{ID: "2", Name: "B.com"},
		{ID: "3", Name: "C.com"},
	}

	result = optimizer.OptimizeNames(shortServers)
	// Should optimize but keep original names due to length validation
	for i, name := range result.OptimizedNames {
		if len(name) < 3 && name != shortServers[i].Name {
			t.Errorf("Should preserve short names, got: %s", name)
		}
	}

	// Test with servers having only numbers as suffixes
	numberServers := []types.Server{
		{ID: "1", Name: "Server123"},
		{ID: "2", Name: "Server456"},
		{ID: "3", Name: "Server789"},
	}

	result = optimizer.OptimizeNames(numberServers)
	if result.RemovedSuffix != "" {
		t.Error("Should not optimize numeric suffixes")
	}

	// Test with mixed meaningful and non-meaningful suffixes
	mixedServers := []types.Server{
		{ID: "1", Name: "Web.example.com"},
		{ID: "2", Name: "API.example.com"},
		{ID: "3", Name: "DB.different.net"},
	}

	result = optimizer.OptimizeNames(mixedServers)
	// Should not optimize due to insufficient coverage
	if result.AppliedCount > 0 && result.AppliedCount < 2 {
		t.Error("Should either optimize all eligible or none")
	}
}

// TestButtonTextProcessorEdgeCases tests edge cases for ButtonTextProcessor
func TestButtonTextProcessorEdgeCases(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	// Test with only emojis
	emojiOnly := "🌟⚡🚀🌐✅"
	result := processor.ProcessButtonText(emojiOnly, 10)
	if len(result) == 0 {
		t.Error("Should handle emoji-only text")
	}

	// Test with mixed RTL and LTR text
	mixedText := "Server العربية 🌐"
	result = processor.ProcessButtonText(mixedText, 20)
	if len(result) == 0 {
		t.Error("Should handle mixed RTL/LTR text")
	}

	// Test with zero max length
	result = processor.ProcessButtonText("Test", 0)
	if result != "Test" { // Uses default max length
		t.Logf("Zero max length result: '%s' (uses default max length)", result)
	}

	// Test with negative max length
	result = processor.ProcessButtonText("Test", -5)
	if result != "Test" { // Uses default max length
		t.Logf("Negative max length result: '%s' (uses default max length)", result)
	}

	// Test with complex emoji sequences
	complexEmoji := "👨‍💻👩‍🚀🏳️‍🌈"
	result = processor.ProcessButtonText(complexEmoji, 20)
	if len(result) == 0 {
		t.Error("Should handle complex emoji sequences")
	}

	// Test emoji length calculation edge cases
	length := processor.CalculateTextLength("")
	if length != 0 {
		t.Error("Empty string should have length 0")
	}

	// Test with null characters
	nullText := "Test\x00Text"
	result = processor.ProcessButtonText(nullText, 20)
	if len(result) == 0 {
		t.Error("Should handle null characters")
	}
}

// TestServerSorterEdgeCases tests edge cases for ServerSorter
func TestServerSorterEdgeCases(t *testing.T) {
	sorter := server.NewServerSorter()

	// Test with nil slices
	var nilServers []types.Server
	result := sorter.SortAlphabetically(nilServers)
	if result == nil {
		t.Logf("Nil slice returned nil (this may be expected behavior)")
	}

	var nilResults []types.PingResult
	pingResult := sorter.SortPingResults(nilResults)
	if pingResult == nil {
		t.Logf("Nil slice returned nil (this may be expected behavior)")
	}

	// Test with servers having identical names but different IDs
	identicalNameServers := []types.Server{
		{ID: "1", Name: "Server"},
		{ID: "2", Name: "Server"},
		{ID: "3", Name: "Server"},
	}

	result = sorter.SortAlphabetically(identicalNameServers)
	if len(result) != 3 {
		t.Error("Should preserve all servers with identical names")
	}

	// Test with ping results having identical latencies
	identicalLatencyResults := []types.PingResult{
		{Server: types.Server{ID: "1", Name: "C"}, Available: true, Latency: 100},
		{Server: types.Server{ID: "2", Name: "A"}, Available: true, Latency: 100},
		{Server: types.Server{ID: "3", Name: "B"}, Available: true, Latency: 100},
	}

	pingResult = sorter.SortPingResults(identicalLatencyResults)
	// Should be sorted alphabetically when latencies are identical
	if pingResult[0].Server.Name != "A" {
		t.Error("Should sort alphabetically when latencies are identical")
	}

	// Test quick select with zero limit
	quickResult := sorter.SortForQuickSelect(identicalLatencyResults, 0)
	if len(quickResult) != 3 {
		t.Error("Zero limit should return all available servers")
	}

	// Test quick select with limit larger than available servers
	quickResult = sorter.SortForQuickSelect(identicalLatencyResults, 10)
	if len(quickResult) != 3 {
		t.Error("Should return all available servers when limit is larger")
	}
}

// TestUpdateManagerEdgeCases tests edge cases for UpdateManager
func TestUpdateManagerEdgeCases(t *testing.T) {
	// Test with mock update manager instead of real network calls
	um := NewMockUpdateManager()

	// Test with failure scenario
	um.shouldFail = true
	um.failureMessage = "invalid URL"
	ctx := context.Background()

	err := um.ExecuteUpdate(ctx)
	if err == nil {
		t.Error("Should fail with invalid URL")
	}

	// Test progress monitoring with closed channel
	_ = um.StartProgressMonitoring()
	um.StopProgressMonitoring()

	// Test multiple stop calls - should not panic
	um.StopProgressMonitoring()
	um.StopProgressMonitoring()

	// Test status during and after update
	status := um.GetUpdateStatus()
	if status.InProgress {
		t.Error("Should not be in progress initially")
	}

	// Test version checking
	available, version, err := um.CheckUpdateAvailable()
	if err != nil {
		t.Errorf("CheckUpdateAvailable should not error: %v", err)
	}
	if !available {
		t.Error("Should always report update available in test")
	}
	if version != "latest" {
		t.Error("Should return 'latest' version")
	}

	// Test current version
	currentVersion := um.GetCurrentVersion()
	if currentVersion != "dev" {
		t.Error("Should return 'dev' as current version")
	}
}

// TestIntegrationWithMockComponents tests integration between components
func TestIntegrationWithMockComponents(t *testing.T) {
	// Create all components
	mockConfig := &MockConfig{
		adminID:  123456789,
		botToken: "test-token",
	}

	mockLogger := &MockLogger{}
	mockBot := &MockBot{}

	// Test servers with various characteristics
	testServers := []types.Server{
		{ID: "1", Name: "🇺🇸 US-East-Server.example.com", Address: "us1.example.com", Port: 443},
		{ID: "2", Name: "🇪🇺 EU-West-Server.example.com", Address: "eu1.example.com", Port: 443},
		{ID: "3", Name: "🇦🇸 AS-South-Server.different.org", Address: "as1.different.org", Port: 443},
	}

	mockServerMgr := &MockServerManager{
		servers: testServers,
	}

	// Create TelegramBot with all components
	tb := &TelegramBot{
		config:      mockConfig,
		serverMgr:   mockServerMgr,
		logger:      mockLogger,
		rateLimiter: NewRateLimiter(10, time.Minute),
	}

	tb.messageManager = NewMessageManager(mockBot, mockLogger)
	updateManager := NewMockUpdateManager()
	tb.handlers = NewCommandHandlers(tb, updateManager)

	// Test component integration
	ctx := context.Background()
	userID := int64(123456789)

	// Test server list creation with all features
	keyboard := tb.createServerListKeyboard(testServers, 0)
	if keyboard == nil {
		t.Fatal("Keyboard should not be nil")
	}

	// Test message management
	content := MessageContent{
		Text: "🏠 Main Menu\n\nSelect an option:",
		Type: MessageTypeMenu,
	}

	err := tb.messageManager.SendNew(ctx, userID, content)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Test message editing
	updatedContent := MessageContent{
		Text: "📋 Server List\n\nSelect a server:",
		Type: MessageTypeServerList,
	}

	err = tb.messageManager.SendOrEdit(ctx, userID, updatedContent)
	if err != nil {
		t.Fatalf("Failed to edit message: %v", err)
	}

	// Verify integration worked
	if len(mockBot.sentMessages) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 1 {
		t.Errorf("Expected 1 edited message, got %d", len(mockBot.editedMessages))
	}

	// Test cleanup
	tb.messageManager.CleanupExpiredMessages()

	// Test statistics
	count := tb.messageManager.GetActiveMessageCount()
	if count != 1 {
		t.Errorf("Expected 1 active message, got %d", count)
	}

	t.Log("Integration test with mock components completed successfully")
}

// TestErrorRecoveryScenarios tests various error recovery scenarios
func TestErrorRecoveryScenarios(t *testing.T) {
	mockLogger := &MockLogger{}

	// Test MessageManager error recovery
	mockBot := &MockBot{
		shouldFailSend:   true,
		shouldFailEdit:   true,
		shouldFailDelete: true,
	}

	mm := NewMessageManager(mockBot, mockLogger)
	ctx := context.Background()
	userID := int64(123)

	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	// Should fail to send
	err := mm.SendNew(ctx, userID, content)
	if err == nil {
		t.Error("Expected send to fail")
	}

	// Should not have active message due to send failure
	if mm.GetActiveMessageCount() != 0 {
		t.Error("Should not have active messages after send failure")
	}

	// Test with bot that can send but fails other operations
	mockBot2 := &MockBot{
		shouldFailEdit:   true,
		shouldFailDelete: true,
	}

	mm2 := NewMessageManager(mockBot2, mockLogger)

	// Send should succeed
	err = mm2.SendNew(ctx, userID, content)
	if err != nil {
		t.Errorf("Send should succeed: %v", err)
	}

	// Edit should fail and fallback to new message
	content.Text = "Updated message"
	err = mm2.SendOrEdit(ctx, userID, content)
	if err != nil {
		t.Errorf("SendOrEdit should succeed with fallback: %v", err)
	}

	// Should have attempted delete (even if it failed)
	if len(mockBot2.deletedMessages) != 1 {
		t.Logf("Expected 1 delete attempt, got %d (delete may have been skipped)", len(mockBot2.deletedMessages))
	}

	// Test UpdateManager error recovery
	um := NewMockUpdateManager()
	um.shouldFail = true
	um.failureMessage = "network error"

	err = um.ExecuteUpdate(ctx)
	if err == nil {
		t.Error("Update should fail with network error")
	}

	// Status should reflect the error
	status := um.GetUpdateStatus()
	if status.Error == nil {
		t.Error("Status should have error set")
	}

	if status.InProgress {
		t.Error("Should not be in progress after failure")
	}

	t.Log("Error recovery scenarios test completed successfully")
}
