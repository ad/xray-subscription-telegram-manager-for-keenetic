package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// MockBotWithRetry extends MockBot to simulate retry scenarios
type MockBotWithRetry struct {
	*MockBot
	sendFailCount   int
	editFailCount   int
	deleteFailCount int
	sendAttempts    int
	editAttempts    int
	deleteAttempts  int
}

func NewMockBotWithRetry() *MockBotWithRetry {
	return &MockBotWithRetry{
		MockBot: &MockBot{},
	}
}

func (m *MockBotWithRetry) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	m.sendAttempts++
	if m.sendAttempts <= m.sendFailCount {
		return nil, errors.New("network timeout")
	}
	return m.MockBot.SendMessage(ctx, params)
}

func (m *MockBotWithRetry) EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
	m.editAttempts++
	if m.editAttempts <= m.editFailCount {
		return nil, errors.New("rate limit exceeded")
	}
	return m.MockBot.EditMessageText(ctx, params)
}

func (m *MockBotWithRetry) DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
	m.deleteAttempts++
	if m.deleteAttempts <= m.deleteFailCount {
		return false, errors.New("message too old")
	}
	return m.MockBot.DeleteMessage(ctx, params)
}

func TestMessageManagerRetryLogic(t *testing.T) {
	mockBot := NewMockBotWithRetry()
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	// Configure for faster testing
	mm.SetRetryConfig(3, 10*time.Millisecond)
	mm.SetTimeouts(1*time.Minute, 5*time.Second)

	ctx := context.Background()
	userID := int64(123)

	// Test retry on send failure
	mockBot.sendFailCount = 2 // Fail first 2 attempts, succeed on 3rd
	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}

	if mockBot.sendAttempts != 3 {
		t.Errorf("Expected 3 send attempts, got %d", mockBot.sendAttempts)
	}

	// Test retry on edit failure with fallback
	mockBot.editFailCount = 5 // Fail all edit attempts
	mockBot.sendAttempts = 0  // Reset counter
	mockBot.sendFailCount = 0 // Don't fail send on fallback

	content.Text = "Updated message"
	err = mm.SendOrEdit(ctx, userID, content)
	if err != nil {
		t.Errorf("Expected success with fallback, got error: %v", err)
	}

	if mockBot.editAttempts != 3 {
		t.Errorf("Expected 3 edit attempts before fallback, got %d", mockBot.editAttempts)
	}
}

func TestMessageManagerTimeoutHandling(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	// Set very short timeout for testing
	mm.SetTimeouts(100*time.Millisecond, 50*time.Millisecond)

	ctx := context.Background()
	userID := int64(123)

	// Send initial message
	content := MessageContent{
		Text: "Initial message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Errorf("Failed to send initial message: %v", err)
	}

	// Wait for message to expire
	time.Sleep(150 * time.Millisecond)

	// Try to edit expired message - should send new instead
	content.Text = "Updated message"
	err = mm.SendOrEdit(ctx, userID, content)
	if err != nil {
		t.Errorf("Failed to handle expired message: %v", err)
	}

	// Should have 2 messages sent (initial + new after expiry)
	if len(mockBot.sentMessages) != 2 {
		t.Errorf("Expected 2 sent messages, got %d", len(mockBot.sentMessages))
	}
}

func TestMessageManagerCleanupWithContext(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	// Set short timeout for testing
	mm.SetTimeouts(50*time.Millisecond, 1*time.Second)

	ctx := context.Background()
	userID := int64(123)

	// Send a message
	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Verify message is active
	if mm.GetActiveMessageCount() != 1 {
		t.Errorf("Expected 1 active message, got %d", mm.GetActiveMessageCount())
	}

	// Wait for message to expire
	time.Sleep(100 * time.Millisecond)

	// Test cleanup with context
	cleanupCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	err = mm.CleanupExpiredMessagesWithContext(cleanupCtx)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Verify message was cleaned up
	if mm.GetActiveMessageCount() != 0 {
		t.Errorf("Expected 0 active messages after cleanup, got %d", mm.GetActiveMessageCount())
	}
}

func TestMessageManagerForceCleanup(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)

	// Send a message
	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Verify message is active
	if mm.GetActiveMessageCount() != 1 {
		t.Errorf("Expected 1 active message, got %d", mm.GetActiveMessageCount())
	}

	// Force cleanup
	mm.ForceCleanupUser(userID, "test cleanup")

	// Verify message was cleaned up
	if mm.GetActiveMessageCount() != 0 {
		t.Errorf("Expected 0 active messages after force cleanup, got %d", mm.GetActiveMessageCount())
	}
}

func TestMessageManagerStatistics(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()

	// Send messages of different types
	messages := []struct {
		userID  int64
		msgType MessageType
	}{
		{123, MessageTypeMenu},
		{124, MessageTypeServerList},
		{125, MessageTypePingTest},
		{126, MessageTypeMenu},
	}

	for _, msg := range messages {
		content := MessageContent{
			Text: "Test message",
			Type: msg.msgType,
		}
		err := mm.SendNew(ctx, msg.userID, content)
		if err != nil {
			t.Errorf("Failed to send message: %v", err)
		}
	}

	// Check total count
	if mm.GetActiveMessageCount() != 4 {
		t.Errorf("Expected 4 active messages, got %d", mm.GetActiveMessageCount())
	}

	// Check type statistics
	typeStats := mm.GetActiveMessagesByType()
	expectedStats := map[MessageType]int{
		MessageTypeMenu:       2,
		MessageTypeServerList: 1,
		MessageTypePingTest:   1,
	}

	for msgType, expectedCount := range expectedStats {
		if typeStats[msgType] != expectedCount {
			t.Errorf("Expected %d messages of type %s, got %d",
				expectedCount, msgType, typeStats[msgType])
		}
	}
}
