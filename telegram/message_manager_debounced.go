package telegram

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// MessageType represents the type of message being managed
type MessageType string

const (
	MessageTypeMenu       MessageType = "menu"
	MessageTypeServerList MessageType = "server_list"
	MessageTypePingTest   MessageType = "ping_test"
	MessageTypeStatus     MessageType = "status"
)

// ActiveMessage represents an active message that can be edited
type ActiveMessage struct {
	ChatID    int64
	MessageID int
	Type      MessageType
	CreatedAt time.Time
}

// MessageContent represents the content to be sent or edited
type MessageContent struct {
	Text        string
	ReplyMarkup *models.InlineKeyboardMarkup
	ParseMode   models.ParseMode
	Type        MessageType
}

// PendingUpdate represents a pending message update for debouncing
type PendingUpdate struct {
	UserID    int64
	Content   MessageContent
	UpdatedAt time.Time
	timer     *time.Timer
}

// MessageManagerInterface defines the interface for message management
type MessageManagerInterface interface {
	// SendOrEdit sends a new message or edits an existing one
	SendOrEdit(ctx context.Context, userID int64, content MessageContent) error

	// SendNew forces sending a new message
	SendNew(ctx context.Context, userID int64, content MessageContent) error

	// ClearActiveMessage clears the active message for a user
	ClearActiveMessage(userID int64)

	// GetActiveMessage gets the active message for a user
	GetActiveMessage(userID int64) *ActiveMessage
}

// BotInterface defines the interface for bot operations needed by MessageManager
type BotInterface interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error)
	DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error)
}

// MessageManager handles message editing and fallbacks with debouncing
type MessageManager struct {
	bot              BotInterface
	logger           Logger
	activeMessages   map[int64]*ActiveMessage
	mutex            sync.RWMutex
	messageTimeout   time.Duration
	operationTimeout time.Duration
	maxRetries       int
	retryDelay       time.Duration

	// Debouncing fields
	pendingUpdates map[int64]*PendingUpdate
	debounceMutex  sync.RWMutex
	debounceDelay  time.Duration
}

// NewMessageManager creates a new MessageManager instance
func NewMessageManager(b BotInterface, logger Logger) *MessageManager {
	return &MessageManager{
		bot:              b,
		logger:           logger,
		activeMessages:   make(map[int64]*ActiveMessage),
		pendingUpdates:   make(map[int64]*PendingUpdate),
		messageTimeout:   60 * time.Minute, // Default timeout of 60 minutes
		operationTimeout: 30 * time.Second, // Default operation timeout of 30 seconds
		maxRetries:       3,                // Default max retries
		retryDelay:       1 * time.Second,  // Default retry delay
		debounceDelay:    time.Second,      // Default debounce delay of 1 second
	}
}

// SendOrEdit sends a new message or edits an existing one with timeout and retry handling
func (mm *MessageManager) SendOrEdit(ctx context.Context, userID int64, content MessageContent) error {
	// Use debounced version for frequent updates like ping progress
	if content.Type == MessageTypePingTest {
		return mm.SendOrEditDebounced(ctx, userID, content)
	}

	// Direct send for other message types
	return mm.sendOrEditImmediate(ctx, userID, content)
}

// SendOrEditDebounced sends a message with debouncing to prevent rate limiting
func (mm *MessageManager) SendOrEditDebounced(ctx context.Context, userID int64, content MessageContent) error {
	// Ensure content text is valid UTF-8
	if !utf8.ValidString(content.Text) {
		content.Text = strings.ToValidUTF8(content.Text, "")
		mm.logger.Warn("Fixed invalid UTF-8 in message content for user %d", userID)
	}

	mm.debounceMutex.Lock()
	defer mm.debounceMutex.Unlock()

	// Cancel existing timer if any
	if existing := mm.pendingUpdates[userID]; existing != nil {
		if existing.timer != nil {
			existing.timer.Stop()
		}
	}

	// Create new pending update
	pending := &PendingUpdate{
		UserID:    userID,
		Content:   content,
		UpdatedAt: time.Now(),
	}

	// Set up timer to send the update after debounce delay
	pending.timer = time.AfterFunc(mm.debounceDelay, func() {
		mm.executePendingUpdate(ctx, userID)
	})

	mm.pendingUpdates[userID] = pending
	mm.logger.Debug("Scheduled debounced update for user %d (delay: %v)", userID, mm.debounceDelay)

	return nil
}

// sendOrEditImmediate sends a message immediately without debouncing
func (mm *MessageManager) sendOrEditImmediate(ctx context.Context, userID int64, content MessageContent) error {
	// Ensure content text is valid UTF-8
	if !utf8.ValidString(content.Text) {
		content.Text = strings.ToValidUTF8(content.Text, "")
		mm.logger.Warn("Fixed invalid UTF-8 in message content for user %d", userID)
	}

	// Create a context with timeout for the operation
	opCtx, cancel := context.WithTimeout(ctx, mm.operationTimeout)
	defer cancel()

	mm.mutex.Lock()
	activeMsg := mm.activeMessages[userID]
	mm.mutex.Unlock()

	// If no active message or message is too old, send new message
	if activeMsg == nil || mm.isMessageExpired(activeMsg) {
		mm.logger.Debug("No active message or expired message for user %d, sending new message", userID)
		return mm.sendNewWithRetry(opCtx, userID, content)
	}

	// Try to edit the existing message with retry logic
	mm.logger.Debug("Attempting to edit message %d for user %d", activeMsg.MessageID, userID)

	editParams := &bot.EditMessageTextParams{
		ChatID:      activeMsg.ChatID,
		MessageID:   activeMsg.MessageID,
		Text:        content.Text,
		ReplyMarkup: mm.ensureValidReplyMarkup(content.ReplyMarkup),
		ParseMode:   content.ParseMode,
	}

	err := mm.editMessageWithRetry(opCtx, editParams)
	if err != nil {
		mm.logger.Warn("Failed to edit message %d for user %d after retries: %v, falling back to new message",
			activeMsg.MessageID, userID, err)

		// Fallback: try to delete old message and send new one
		mm.deleteMessageWithTimeout(opCtx, activeMsg.ChatID, activeMsg.MessageID)
		mm.ClearActiveMessage(userID)

		return mm.sendNewWithRetry(opCtx, userID, content)
	}

	// Update the message type and timestamp
	mm.mutex.Lock()
	activeMsg.Type = content.Type
	activeMsg.CreatedAt = time.Now()
	mm.mutex.Unlock()

	mm.logger.Debug("Successfully edited message %d for user %d", activeMsg.MessageID, userID)
	return nil
}

// executePendingUpdate executes a pending debounced update
func (mm *MessageManager) executePendingUpdate(ctx context.Context, userID int64) {
	mm.debounceMutex.Lock()
	pending := mm.pendingUpdates[userID]
	delete(mm.pendingUpdates, userID)
	mm.debounceMutex.Unlock()

	if pending == nil {
		return
	}

	mm.logger.Debug("Executing debounced update for user %d", userID)

	// Execute the actual update
	if err := mm.sendOrEditImmediate(ctx, userID, pending.Content); err != nil {
		mm.logger.Error("Failed to execute debounced update for user %d: %v", userID, err)
	}
}

// SendNew forces sending a new message
func (mm *MessageManager) SendNew(ctx context.Context, userID int64, content MessageContent) error {
	// Create a context with timeout for the operation
	opCtx, cancel := context.WithTimeout(ctx, mm.operationTimeout)
	defer cancel()

	return mm.sendNewWithRetry(opCtx, userID, content)
}

// ensureValidReplyMarkup ensures that ReplyMarkup is valid or returns an empty keyboard
func (mm *MessageManager) ensureValidReplyMarkup(markup *models.InlineKeyboardMarkup) *models.InlineKeyboardMarkup {
	if markup == nil {
		return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}}
	}
	return markup
}

// sendNewWithRetry sends a new message with retry logic
func (mm *MessageManager) sendNewWithRetry(ctx context.Context, userID int64, content MessageContent) error {
	mm.logger.Debug("Sending new message to user %d", userID)

	sendParams := &bot.SendMessageParams{
		ChatID:      userID,
		Text:        content.Text,
		ReplyMarkup: mm.ensureValidReplyMarkup(content.ReplyMarkup),
		ParseMode:   content.ParseMode,
	}

	var sentMsg *models.Message
	var err error

	for attempt := 0; attempt < mm.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(mm.retryDelay):
				// Wait before retry, but respect context cancellation
			}
		}

		sentMsg, err = mm.bot.SendMessage(ctx, sendParams)
		if err == nil {
			break // Success
		}

		mm.logger.Debug("Attempt %d failed to send message to user %d: %v", attempt+1, userID, err)

		// Check if we should retry based on error type
		if !mm.shouldRetry(err) {
			break
		}
	}

	if err != nil {
		mm.logger.Error("Failed to send new message to user %d after %d attempts: %v", userID, mm.maxRetries, err)
		return err
	}

	// Store the new active message
	mm.mutex.Lock()
	mm.activeMessages[userID] = &ActiveMessage{
		ChatID:    sentMsg.Chat.ID,
		MessageID: sentMsg.ID,
		Type:      content.Type,
		CreatedAt: time.Now(),
	}
	mm.mutex.Unlock()

	mm.logger.Debug("Successfully sent new message %d to user %d", sentMsg.ID, userID)
	return nil
}

// ClearActiveMessage clears the active message for a user
func (mm *MessageManager) ClearActiveMessage(userID int64) {
	mm.mutex.Lock()
	delete(mm.activeMessages, userID)
	mm.mutex.Unlock()

	mm.logger.Debug("Cleared active message for user %d", userID)
}

// GetActiveMessage gets the active message for a user
func (mm *MessageManager) GetActiveMessage(userID int64) *ActiveMessage {
	mm.mutex.RLock()
	activeMsg := mm.activeMessages[userID]
	mm.mutex.RUnlock()

	return activeMsg
}

// isMessageExpired checks if a message is too old to be edited
func (mm *MessageManager) isMessageExpired(msg *ActiveMessage) bool {
	return time.Since(msg.CreatedAt) > mm.messageTimeout
}

// editMessageWithRetry attempts to edit a message with retry logic
func (mm *MessageManager) editMessageWithRetry(ctx context.Context, params *bot.EditMessageTextParams) error {
	var err error

	for attempt := 0; attempt < mm.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(mm.retryDelay):
				// Wait before retry, but respect context cancellation
			}
		}

		_, err = mm.bot.EditMessageText(ctx, params)
		if err == nil {
			return nil // Success
		}

		mm.logger.Debug("Attempt %d failed to edit message %d: %v", attempt+1, params.MessageID, err)

		// Check if we should retry based on error type
		if !mm.shouldRetry(err) {
			break
		}
	}

	return err
}

// deleteMessageWithTimeout attempts to delete a message with timeout (best effort)
func (mm *MessageManager) deleteMessageWithTimeout(ctx context.Context, chatID int64, messageID int) {
	deleteParams := &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	}

	_, err := mm.bot.DeleteMessage(ctx, deleteParams)
	if err != nil {
		mm.logger.Debug("Could not delete message %d from chat %d: %v", messageID, chatID, err)
	} else {
		mm.logger.Debug("Successfully deleted message %d from chat %d", messageID, chatID)
	}
}

// shouldRetry determines if an error is retryable
func (mm *MessageManager) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retry on network errors, timeouts, and rate limits
	retryableErrors := []string{
		"timeout",
		"connection",
		"network",
		"rate limit",
		"too many requests",
		"internal server error",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}

	return false
}

// CleanupExpiredMessages removes expired messages from memory with enhanced logging
func (mm *MessageManager) CleanupExpiredMessages() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	now := time.Now()
	expiredUsers := make([]int64, 0)
	totalMessages := len(mm.activeMessages)

	for userID, msg := range mm.activeMessages {
		if now.Sub(msg.CreatedAt) > mm.messageTimeout {
			expiredUsers = append(expiredUsers, userID)
		}
	}

	for _, userID := range expiredUsers {
		delete(mm.activeMessages, userID)
		mm.logger.Debug("Cleaned up expired message for user %d", userID)
	}

	if len(expiredUsers) > 0 {
		mm.logger.Info("Cleaned up %d expired messages (total active: %d -> %d)",
			len(expiredUsers), totalMessages, len(mm.activeMessages))
	}
}

// CleanupExpiredPendingUpdates cleans up expired pending updates
func (mm *MessageManager) CleanupExpiredPendingUpdates() {
	mm.debounceMutex.Lock()
	defer mm.debounceMutex.Unlock()

	now := time.Now()
	expiredUsers := make([]int64, 0)

	for userID, pending := range mm.pendingUpdates {
		// Clean up pending updates older than 1 minute
		if now.Sub(pending.UpdatedAt) > time.Minute {
			if pending.timer != nil {
				pending.timer.Stop()
			}
			expiredUsers = append(expiredUsers, userID)
		}
	}

	for _, userID := range expiredUsers {
		delete(mm.pendingUpdates, userID)
		mm.logger.Debug("Cleaned up expired pending update for user %d", userID)
	}

	if len(expiredUsers) > 0 {
		mm.logger.Debug("Cleaned up %d expired pending updates", len(expiredUsers))
	}
}

// SetDebounceDelay allows configuring debounce delay after creation
func (mm *MessageManager) SetDebounceDelay(delay time.Duration) {
	mm.debounceMutex.Lock()
	defer mm.debounceMutex.Unlock()

	mm.debounceDelay = delay
	mm.logger.Info("Updated debounce delay to %v", delay)
}

// CancelPendingUpdate cancels any pending update for a user
func (mm *MessageManager) CancelPendingUpdate(userID int64) {
	mm.debounceMutex.Lock()
	defer mm.debounceMutex.Unlock()

	if pending := mm.pendingUpdates[userID]; pending != nil {
		if pending.timer != nil {
			pending.timer.Stop()
		}
		delete(mm.pendingUpdates, userID)
		mm.logger.Debug("Cancelled pending update for user %d", userID)
	}
}

// GetPendingUpdateCount returns the number of pending updates (for monitoring)
func (mm *MessageManager) GetPendingUpdateCount() int {
	mm.debounceMutex.RLock()
	defer mm.debounceMutex.RUnlock()
	return len(mm.pendingUpdates)
}
