package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// MockBot implements a mock bot for testing
type MockBot struct {
	sentMessages     []MockSentMessage
	editedMessages   []MockEditedMessage
	deletedMessages  []MockDeletedMessage
	shouldFailEdit   bool
	shouldFailSend   bool
	shouldFailDelete bool
}

type MockSentMessage struct {
	ChatID      int64
	Text        string
	ReplyMarkup *models.InlineKeyboardMarkup
	ParseMode   models.ParseMode
}

type MockEditedMessage struct {
	ChatID      int64
	MessageID   int
	Text        string
	ReplyMarkup *models.InlineKeyboardMarkup
	ParseMode   models.ParseMode
}

type MockDeletedMessage struct {
	ChatID    int64
	MessageID int
}

func (mb *MockBot) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	if mb.shouldFailSend {
		return nil, errors.New("Bad Request")
	}

	chatID, ok := params.ChatID.(int64)
	if !ok {
		return nil, errors.New("Invalid ChatID type")
	}

	var replyMarkup *models.InlineKeyboardMarkup
	if params.ReplyMarkup != nil {
		if km, ok := params.ReplyMarkup.(*models.InlineKeyboardMarkup); ok {
			replyMarkup = km
		}
	}

	mb.sentMessages = append(mb.sentMessages, MockSentMessage{
		ChatID:      chatID,
		Text:        params.Text,
		ReplyMarkup: replyMarkup,
		ParseMode:   params.ParseMode,
	})

	// Return a mock message
	return &models.Message{
		ID: len(mb.sentMessages), // Use length as message ID
		Chat: models.Chat{
			ID: chatID,
		},
	}, nil
}

func (mb *MockBot) EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
	chatID, ok := params.ChatID.(int64)
	if !ok {
		return nil, errors.New("Invalid ChatID type")
	}

	var replyMarkup *models.InlineKeyboardMarkup
	if params.ReplyMarkup != nil {
		if km, ok := params.ReplyMarkup.(*models.InlineKeyboardMarkup); ok {
			replyMarkup = km
		}
	}

	// Always record the attempt
	mb.editedMessages = append(mb.editedMessages, MockEditedMessage{
		ChatID:      chatID,
		MessageID:   params.MessageID,
		Text:        params.Text,
		ReplyMarkup: replyMarkup,
		ParseMode:   params.ParseMode,
	})

	// Then check if we should fail
	if mb.shouldFailEdit {
		return nil, errors.New("Bad Request: message is not modified")
	}

	return &models.Message{
		ID: params.MessageID,
		Chat: models.Chat{
			ID: chatID,
		},
	}, nil
}

func (mb *MockBot) DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
	if mb.shouldFailDelete {
		return false, errors.New("Bad Request: message can't be deleted")
	}

	chatID, ok := params.ChatID.(int64)
	if !ok {
		return false, errors.New("Invalid ChatID type")
	}

	mb.deletedMessages = append(mb.deletedMessages, MockDeletedMessage{
		ChatID:    chatID,
		MessageID: params.MessageID,
	})

	return true, nil
}

// MockLogger implements a mock logger for testing
type MockLogger struct {
	logs []string
}

func (ml *MockLogger) Debug(format string, args ...interface{}) {
	ml.logs = append(ml.logs, fmt.Sprintf("DEBUG: "+format, args...))
}

func (ml *MockLogger) Info(format string, args ...interface{}) {
	ml.logs = append(ml.logs, fmt.Sprintf("INFO: "+format, args...))
}

func (ml *MockLogger) Warn(format string, args ...interface{}) {
	ml.logs = append(ml.logs, fmt.Sprintf("WARN: "+format, args...))
}

func (ml *MockLogger) Error(format string, args ...interface{}) {
	ml.logs = append(ml.logs, fmt.Sprintf("ERROR: "+format, args...))
}

func (ml *MockLogger) HasLog(substring string) bool {
	for _, log := range ml.logs {
		if strings.Contains(log, substring) {
			return true
		}
	}
	return false
}

func TestNewMessageManager(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}

	mm := NewMessageManager(mockBot, mockLogger)

	if mm == nil {
		t.Fatal("NewMessageManager returned nil")
	}

	if mm.bot == nil {
		t.Error("MessageManager bot not set correctly")
	}

	if mm.logger == nil {
		t.Error("MessageManager logger not set correctly")
	}

	if mm.activeMessages == nil {
		t.Error("MessageManager activeMessages not initialized")
	}

	if mm.messageTimeout != 60*time.Minute {
		t.Error("MessageManager messageTimeout not set to default value")
	}
}

func TestSendNew(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)
	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Fatalf("SendNew failed: %v", err)
	}

	// Check that message was sent
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

	// Check that active message was stored
	activeMsg := mm.GetActiveMessage(userID)
	if activeMsg == nil {
		t.Fatal("Active message not stored")
	}

	if activeMsg.ChatID != userID {
		t.Errorf("Expected active message ChatID %d, got %d", userID, activeMsg.ChatID)
	}

	if activeMsg.Type != content.Type {
		t.Errorf("Expected active message type %s, got %s", content.Type, activeMsg.Type)
	}
}

func TestSendOrEdit_NoActiveMessage(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)
	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	err := mm.SendOrEdit(ctx, userID, content)
	if err != nil {
		t.Fatalf("SendOrEdit failed: %v", err)
	}

	// Should send new message since no active message exists
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 0 {
		t.Fatalf("Expected 0 edited messages, got %d", len(mockBot.editedMessages))
	}
}

func TestSendOrEdit_WithActiveMessage(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)

	// First, send a message to create an active message
	content1 := MessageContent{
		Text: "First message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content1)
	if err != nil {
		t.Fatalf("SendNew failed: %v", err)
	}

	// Now try to send/edit with new content
	content2 := MessageContent{
		Text: "Second message",
		Type: MessageTypeServerList,
	}

	err = mm.SendOrEdit(ctx, userID, content2)
	if err != nil {
		t.Fatalf("SendOrEdit failed: %v", err)
	}

	// Should edit existing message
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 1 {
		t.Fatalf("Expected 1 edited message, got %d", len(mockBot.editedMessages))
	}

	editedMsg := mockBot.editedMessages[0]
	if editedMsg.Text != content2.Text {
		t.Errorf("Expected edited text '%s', got '%s'", content2.Text, editedMsg.Text)
	}
}

func TestSendOrEdit_EditFailsFallback(t *testing.T) {
	mockBot := &MockBot{shouldFailEdit: true}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)

	// First, send a message to create an active message
	content1 := MessageContent{
		Text: "First message",
		Type: MessageTypeMenu,
	}

	err := mm.SendNew(ctx, userID, content1)
	if err != nil {
		t.Fatalf("SendNew failed: %v", err)
	}

	// Now try to send/edit with new content (edit will fail)
	content2 := MessageContent{
		Text: "Second message",
		Type: MessageTypeServerList,
	}

	err = mm.SendOrEdit(ctx, userID, content2)
	if err != nil {
		t.Fatalf("SendOrEdit failed: %v", err)
	}

	// Should attempt to edit, fail, then send new message
	if len(mockBot.sentMessages) != 2 {
		t.Fatalf("Expected 2 sent messages, got %d", len(mockBot.sentMessages))
	}

	if len(mockBot.editedMessages) != 1 {
		t.Fatalf("Expected 1 edit attempt, got %d", len(mockBot.editedMessages))
	}

	// Should also attempt to delete the old message
	if len(mockBot.deletedMessages) != 1 {
		t.Fatalf("Expected 1 deleted message, got %d", len(mockBot.deletedMessages))
	}
}

func TestClearActiveMessage(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	ctx := context.Background()
	userID := int64(123)
	content := MessageContent{
		Text: "Test message",
		Type: MessageTypeMenu,
	}

	// Send a message to create an active message
	err := mm.SendNew(ctx, userID, content)
	if err != nil {
		t.Fatalf("SendNew failed: %v", err)
	}

	// Verify active message exists
	if mm.GetActiveMessage(userID) == nil {
		t.Fatal("Active message should exist")
	}

	// Clear active message
	mm.ClearActiveMessage(userID)

	// Verify active message is cleared
	if mm.GetActiveMessage(userID) != nil {
		t.Fatal("Active message should be cleared")
	}
}

func TestIsMessageExpired(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	// Test non-expired message
	recentMsg := &ActiveMessage{
		ChatID:    123,
		MessageID: 1,
		Type:      MessageTypeMenu,
		CreatedAt: time.Now().Add(-30 * time.Minute), // 30 minutes ago
	}

	if mm.isMessageExpired(recentMsg) {
		t.Error("Recent message should not be expired")
	}

	// Test expired message
	oldMsg := &ActiveMessage{
		ChatID:    123,
		MessageID: 1,
		Type:      MessageTypeMenu,
		CreatedAt: time.Now().Add(-90 * time.Minute), // 90 minutes ago
	}

	if !mm.isMessageExpired(oldMsg) {
		t.Error("Old message should be expired")
	}
}

func TestCleanupExpiredMessages(t *testing.T) {
	mockBot := &MockBot{}
	mockLogger := &MockLogger{}
	mm := NewMessageManager(mockBot, mockLogger)

	// Add some messages manually
	userID1 := int64(123)
	userID2 := int64(456)
	userID3 := int64(789)

	// Recent message (should not be cleaned up)
	mm.activeMessages[userID1] = &ActiveMessage{
		ChatID:    userID1,
		MessageID: 1,
		Type:      MessageTypeMenu,
		CreatedAt: time.Now().Add(-30 * time.Minute),
	}

	// Expired message (should be cleaned up)
	mm.activeMessages[userID2] = &ActiveMessage{
		ChatID:    userID2,
		MessageID: 2,
		Type:      MessageTypeServerList,
		CreatedAt: time.Now().Add(-90 * time.Minute),
	}

	// Another expired message (should be cleaned up)
	mm.activeMessages[userID3] = &ActiveMessage{
		ChatID:    userID3,
		MessageID: 3,
		Type:      MessageTypePingTest,
		CreatedAt: time.Now().Add(-120 * time.Minute),
	}

	// Verify initial state
	if len(mm.activeMessages) != 3 {
		t.Fatalf("Expected 3 active messages, got %d", len(mm.activeMessages))
	}

	// Run cleanup
	mm.CleanupExpiredMessages()

	// Verify cleanup results
	if len(mm.activeMessages) != 1 {
		t.Fatalf("Expected 1 active message after cleanup, got %d", len(mm.activeMessages))
	}

	// Verify the correct message remains
	if mm.GetActiveMessage(userID1) == nil {
		t.Error("Recent message should not be cleaned up")
	}

	if mm.GetActiveMessage(userID2) != nil {
		t.Error("Expired message should be cleaned up")
	}

	if mm.GetActiveMessage(userID3) != nil {
		t.Error("Expired message should be cleaned up")
	}
}
