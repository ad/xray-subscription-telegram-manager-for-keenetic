package telegram

import (
	"context"
	"strings"
	"sync"
	"time"

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

// MessageManager handles message editing and fallbacks
type MessageManager struct {
	bot              BotInterface
	logger           Logger
	activeMessages   map[int64]*ActiveMessage
	mutex            sync.RWMutex
	messageTimeout   time.Duration
	operationTimeout time.Duration
	maxRetries       int
	retryDelay       time.Duration
}

// NewMessageManager creates a new MessageManager instance
func NewMessageManager(b BotInterface, logger Logger) *MessageManager {
	return &MessageManager{
		bot:              b,
		logger:           logger,
		activeMessages:   make(map[int64]*ActiveMessage),
		messageTimeout:   60 * time.Minute, // Default timeout of 60 minutes
		operationTimeout: 30 * time.Second, // Default operation timeout of 30 seconds
		maxRetries:       3,                // Default max retries
		retryDelay:       1 * time.Second,  // Default retry delay
	}
}

// SendOrEdit sends a new message or edits an existing one with timeout and retry handling
func (mm *MessageManager) SendOrEdit(ctx context.Context, userID int64, content MessageContent) error {
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
	// activeMsg is already checked to be non-nil above, so this condition is always true
	activeMsg.Type = content.Type
	activeMsg.CreatedAt = time.Now()
	mm.mutex.Unlock()

	mm.logger.Debug("Successfully edited message %d for user %d", activeMsg.MessageID, userID)
	return nil
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
			mm.logger.Debug("Retry attempt %d/%d for sending message to user %d", attempt+1, mm.maxRetries, userID)

			// Wait before retry, but respect context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(mm.retryDelay * time.Duration(attempt)):
				// Continue with retry
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
			mm.logger.Debug("Retry attempt %d/%d for editing message %d", attempt+1, mm.maxRetries, params.MessageID)

			// Wait before retry, but respect context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(mm.retryDelay * time.Duration(attempt)):
				// Continue with retry
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
			mm.logger.Debug("Message for user %d expired (age: %v, type: %s)",
				userID, now.Sub(msg.CreatedAt), msg.Type)
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

// CleanupExpiredMessagesWithContext removes expired messages with context cancellation support
func (mm *MessageManager) CleanupExpiredMessagesWithContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		mm.CleanupExpiredMessages()
		return nil
	}
}

// GetActiveMessageCount returns the number of active messages (for monitoring)
func (mm *MessageManager) GetActiveMessageCount() int {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return len(mm.activeMessages)
}

// GetActiveMessagesByType returns the count of active messages by type (for monitoring)
func (mm *MessageManager) GetActiveMessagesByType() map[MessageType]int {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	counts := make(map[MessageType]int)
	for _, msg := range mm.activeMessages {
		counts[msg.Type]++
	}
	return counts
}

// StartCleanupRoutine starts a background routine to clean up expired messages with enhanced monitoring
func (mm *MessageManager) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute) // Cleanup every 10 minutes
	defer ticker.Stop()

	mm.logger.Info("Started message cleanup routine (interval: 10 minutes, timeout: %v)", mm.messageTimeout)

	for {
		select {
		case <-ctx.Done():
			mm.logger.Info("Message cleanup routine stopped due to context cancellation")
			return
		case <-ticker.C:
			mm.performCleanupWithRecovery(ctx)
		}
	}
}

// performCleanupWithRecovery performs cleanup with panic recovery
func (mm *MessageManager) performCleanupWithRecovery(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			mm.logger.Error("Panic during message cleanup: %v", r)
		}
	}()

	// Create a timeout context for cleanup operation
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := mm.CleanupExpiredMessagesWithContext(cleanupCtx); err != nil {
		mm.logger.Error("Cleanup operation failed: %v", err)
		return
	}

	// Log statistics periodically
	activeCount := mm.GetActiveMessageCount()
	if activeCount > 0 {
		typeStats := mm.GetActiveMessagesByType()
		mm.logger.Debug("Active messages: %d (types: %+v)", activeCount, typeStats)
	}
}

// ForceCleanupUser removes active message for a specific user (for error recovery)
func (mm *MessageManager) ForceCleanupUser(userID int64, reason string) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if msg, exists := mm.activeMessages[userID]; exists {
		delete(mm.activeMessages, userID)
		mm.logger.Info("Force cleaned up message for user %d (reason: %s, type: %s, age: %v)",
			userID, reason, msg.Type, time.Since(msg.CreatedAt))
	}
}

// SetTimeouts allows configuring timeouts after creation
func (mm *MessageManager) SetTimeouts(messageTimeout, operationTimeout time.Duration) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.messageTimeout = messageTimeout
	mm.operationTimeout = operationTimeout
	mm.logger.Info("Updated timeouts: message=%v, operation=%v", messageTimeout, operationTimeout)
}

// SetRetryConfig allows configuring retry behavior after creation
func (mm *MessageManager) SetRetryConfig(maxRetries int, retryDelay time.Duration) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.maxRetries = maxRetries
	mm.retryDelay = retryDelay
	mm.logger.Info("Updated retry config: maxRetries=%d, retryDelay=%v", maxRetries, retryDelay)
}
