package telegram

import (
	"context"
	"testing"
	"time"
	"xray-telegram-manager/types"
)

func TestMessageManagerIntegration(t *testing.T) {
	// Create mock dependencies - no network calls
	mockLogger := &MockLogger{}

	// Create mock bot instead of real TelegramBot
	mockBot := &MockBot{}

	// Create MessageManager directly with mocks
	mm := NewMessageManager(mockBot, mockLogger)

	// Test that MessageManager is properly initialized
	if mm == nil {
		t.Fatal("MessageManager should be initialized")
	}

	// Test basic MessageManager functionality without network calls
	ctx := context.Background()
	userID := int64(123)
	content := MessageContent{
		Text: "Test integration message",
		Type: MessageTypeMenu,
	}

	// This should work with mock bot
	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Fatalf("SendNew should work with mock bot: %v", err)
	}

	// Verify message was sent to mock bot
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockBot.sentMessages))
	}

	sentMsg := mockBot.sentMessages[0]
	if sentMsg.ChatID != userID {
		t.Errorf("Expected ChatID %d, got %d", userID, sentMsg.ChatID)
	}

	if sentMsg.Text != content.Text {
		t.Errorf("Expected text '%s', got '%s'", content.Text, sentMsg.Text)
	}

	// Test that active message was stored
	activeMsg := mm.GetActiveMessage(userID)
	if activeMsg == nil {
		t.Fatal("Active message should be stored")
	}

	if activeMsg.Type != content.Type {
		t.Errorf("Expected message type %s, got %s", content.Type, activeMsg.Type)
	}

	// Test SendOrEdit functionality
	content2 := MessageContent{
		Text: "Updated message",
		Type: MessageTypeServerList,
	}

	err = mm.SendOrEdit(ctx, userID, content2)
	if err != nil {
		t.Fatalf("SendOrEdit should work with mock bot: %v", err)
	}

	// Should have edited the message, not sent a new one
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message (no new messages), got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 1 {
		t.Fatalf("Expected 1 edited message, got %d", len(mockBot.editedMessages))
	}

	editedMsg := mockBot.editedMessages[0]
	if editedMsg.Text != content2.Text {
		t.Errorf("Expected edited text '%s', got '%s'", content2.Text, editedMsg.Text)
	}

	// Test cleanup functionality
	mm.CleanupExpiredMessages() // Should not panic

	t.Log("MessageManager integration test completed successfully")
}

func TestTelegramBotWithMessageManagerIntegration(t *testing.T) {
	// Create mock dependencies
	mockConfig := &MockConfig{
		adminID:  123456789,
		botToken: "mock-token",
	}

	mockLogger := &MockLogger{}
	mockServerMgr := &MockServerManager{
		servers: []types.Server{
			{
				ID:       "server1",
				Name:     "Test Server 1",
				Address:  "test1.example.com",
				Port:     443,
				Protocol: "vless",
				Tag:      "proxy",
			},
		},
	}

	// Create a mock TelegramBot structure manually to avoid network calls
	mockBot := &MockBot{}
	rateLimiter := NewRateLimiter(10, time.Minute)

	tb := &TelegramBot{
		bot:         nil, // We don't need real bot for this test
		config:      mockConfig,
		serverMgr:   mockServerMgr,
		logger:      mockLogger,
		rateLimiter: rateLimiter,
	}

	// Initialize MessageManager manually
	tb.messageManager = NewMessageManager(mockBot, mockLogger)
	updateManager := NewUpdateManager("", 0, false, mockLogger)
	tb.handlers = NewCommandHandlers(tb, updateManager)

	// Test that MessageManager is properly integrated
	if tb.messageManager == nil {
		t.Fatal("MessageManager should be initialized in TelegramBot")
	}

	// Test GetMessageManager method
	mm := tb.GetMessageManager()
	if mm == nil {
		t.Fatal("GetMessageManager should return a valid MessageManager")
	}

	if mm != tb.messageManager {
		t.Error("GetMessageManager should return the same instance as tb.messageManager")
	}

	// Test that handlers have access to the bot (and thus MessageManager)
	if tb.handlers == nil {
		t.Fatal("Handlers should be initialized")
	}

	if tb.handlers.bot != tb {
		t.Error("Handlers should have reference to TelegramBot")
	}

	// Test MessageManager functionality through the bot
	ctx := context.Background()
	userID := int64(123)
	content := MessageContent{
		Text: "Test bot integration message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Fatalf("SendNew should work through bot integration: %v", err)
	}

	// Verify message was sent
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockBot.sentMessages))
	}

	// Test that active message tracking works
	activeMsg := mm.GetActiveMessage(userID)
	if activeMsg == nil {
		t.Fatal("Active message should be tracked")
	}

	// Test edit functionality
	content2 := MessageContent{
		Text: "Updated bot integration message",
		Type: MessageTypeServerList,
	}

	err = mm.SendOrEdit(ctx, userID, content2)
	if err != nil {
		t.Fatalf("SendOrEdit should work through bot integration: %v", err)
	}

	// Should have edited, not sent new
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message (no new messages), got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 1 {
		t.Fatalf("Expected 1 edited message, got %d", len(mockBot.editedMessages))
	}

	t.Log("TelegramBot with MessageManager integration test completed successfully")
}

func TestMessageManagerErrorHandling(t *testing.T) {
	// Test error handling with failing mock bot
	mockBot := &MockBot{
		shouldFailSend: true,
		shouldFailEdit: true,
	}
	mockLogger := &MockLogger{}

	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)
	content := MessageContent{
		Text: "Test error handling",
		Type: MessageTypeMenu,
	}

	// Test SendNew with failing bot
	err := mm.SendNew(ctx, userID, content)
	if err == nil {
		t.Error("Expected error when bot fails to send message")
	}

	// Verify no active message was stored due to send failure
	activeMsg := mm.GetActiveMessage(userID)
	if activeMsg != nil {
		t.Error("No active message should be stored when send fails")
	}

	// Test with bot that can send but can't edit
	mockBot2 := &MockBot{
		shouldFailEdit: true,
	}
	mm2 := NewMessageManager(mockBot2, mockLogger)

	// First send a message successfully
	err = mm2.SendNew(ctx, userID, content)
	if err != nil {
		t.Fatalf("SendNew should succeed: %v", err)
	}

	// Verify message was sent and tracked
	if len(mockBot2.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockBot2.sentMessages))
	}

	activeMsg = mm2.GetActiveMessage(userID)
	if activeMsg == nil {
		t.Fatal("Active message should be tracked after successful send")
	}

	// Now try to edit (should fail and fallback to new message)
	content2 := MessageContent{
		Text: "Updated message",
		Type: MessageTypeServerList,
	}

	err = mm2.SendOrEdit(ctx, userID, content2)
	if err != nil {
		t.Fatalf("SendOrEdit should succeed with fallback: %v", err)
	}

	// Should have attempted edit, failed, then sent new message
	if len(mockBot2.editedMessages) != 1 {
		t.Fatalf("Expected 1 edit attempt, got %d", len(mockBot2.editedMessages))
	}

	if len(mockBot2.sentMessages) != 2 {
		t.Fatalf("Expected 2 sent messages (original + fallback), got %d", len(mockBot2.sentMessages))
	}

	// Should have attempted to delete old message
	if len(mockBot2.deletedMessages) != 1 {
		t.Fatalf("Expected 1 delete attempt, got %d", len(mockBot2.deletedMessages))
	}

	t.Log("MessageManager error handling test completed successfully")
}

func TestMessageManagerCleanupRoutine(t *testing.T) {
	// Create a mock bot and logger
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}

	mm := NewMessageManager(mockBot, mockLogger)

	// Add an expired message manually
	userID := int64(123)
	mm.activeMessages[userID] = &ActiveMessage{
		ChatID:    userID,
		MessageID: 1,
		Type:      MessageTypeMenu,
		CreatedAt: time.Now().Add(-2 * time.Hour), // 2 hours ago (expired)
	}

	// Verify message exists
	if len(mm.activeMessages) != 1 {
		t.Fatalf("Expected 1 active message, got %d", len(mm.activeMessages))
	}

	// Run cleanup
	mm.CleanupExpiredMessages()

	// Verify message was cleaned up
	if len(mm.activeMessages) != 0 {
		t.Fatalf("Expected 0 active messages after cleanup, got %d", len(mm.activeMessages))
	}

	t.Log("MessageManager cleanup routine test completed successfully")
}
