package telegram

import (
	"context"
	"fmt"
	"time"
	"xray-telegram-manager/types"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type CommandHandlers struct {
	bot              *TelegramBot
	updateManager    UpdateManagerInterface
	messageFormatter *MessageFormatter
	navigationHelper *NavigationHelper
}

func NewCommandHandlers(tb *TelegramBot, updateManager UpdateManagerInterface) *CommandHandlers {
	return &CommandHandlers{
		bot:              tb,
		updateManager:    updateManager,
		messageFormatter: NewMessageFormatter(),
		navigationHelper: NewNavigationHelper(),
	}
}

func (ch *CommandHandlers) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	username := getUsername(update.Message.From)
	ch.bot.logger.Info("Received /start command from user %d (%s)", userID, username)

	if !ch.bot.isAuthorized(userID) {
		ch.bot.logger.Warn("Unauthorized access attempt from user %d (%s)", userID, username)
		ch.bot.sendUnauthorizedMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	if !ch.bot.rateLimiter.IsAllowed(userID) {
		ch.bot.logger.Warn("Rate limit exceeded for user %d (%s)", userID, username)
		ch.sendRateLimitMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	ch.bot.logger.Debug("User %d is authorized, processing /start command", userID)

	ch.bot.logger.Debug("Loading servers for /start command...")
	if err := ch.bot.serverMgr.LoadServers(); err != nil {
		ch.bot.logger.Error("Failed to load servers for /start command: %v", err)
		ch.sendErrorMessage(ctx, b, update.Message.Chat.ID, "Failed to load servers", err.Error(), "refresh")
		return
	}

	servers := ch.bot.serverMgr.GetServers()
	ch.bot.logger.Debug("Loaded %d servers for /start command", len(servers))

	if len(servers) == 0 {
		ch.bot.logger.Warn("No servers available for /start command")
		ch.sendNoServersMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	ch.bot.logger.Debug("Sending welcome message with %d servers", len(servers))
	message := ch.messageFormatter.FormatWelcomeMessage(len(servers))

	keyboard := ch.navigationHelper.CreateMainMenuKeyboard()
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send welcome message: %v", err)
	} else {
		ch.bot.logger.Info("Successfully sent welcome message to user %d", userID)
	}
}

func (ch *CommandHandlers) handleStatus(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	username := getUsername(update.Message.From)
	ch.bot.logger.Info("Received /status command from user %d (%s)", userID, username)

	if !ch.bot.isAuthorized(userID) {
		ch.bot.logger.Warn("Unauthorized access attempt from user %d (%s) for /status command", userID, username)
		ch.bot.sendUnauthorizedMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	if !ch.bot.rateLimiter.IsAllowed(userID) {
		ch.bot.logger.Warn("Rate limit exceeded for user %d (%s)", userID, username)
		ch.sendRateLimitMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	ch.bot.logger.Debug("User %d is authorized, processing /status command", userID)

	currentServer := ch.bot.serverMgr.GetCurrentServer()
	if currentServer == nil {
		ch.bot.logger.Debug("No active server found for /status command")
		ch.sendNoActiveServerMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	ch.bot.logger.Debug("Found active server: %s (%s:%d) for /status command",
		currentServer.Name, currentServer.Address, currentServer.Port)

	message := ch.messageFormatter.FormatServerStatusMessage(currentServer, nil)

	sentMsg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   message,
	})
	if err != nil {
		ch.bot.logger.Error("Failed to send initial status message: %v", err)
		return
	}

	ch.bot.logger.Debug("Sent initial status message, starting ping test for server %s", currentServer.Name)

	results, err := ch.bot.serverMgr.TestPing()
	if err != nil {
		ch.bot.logger.Error("Ping test failed for /status command: %v", err)
		ch.updateStatusMessageWithError(ctx, b, sentMsg, currentServer, err)
		return
	}

	var currentResult *ServerPingResult
	for _, result := range results {
		if result.Server.ID == currentServer.ID {
			currentResult = &result
			break
		}
	}

	if currentResult == nil {
		ch.bot.logger.Warn("Current server not found in ping results for /status command")
		ch.updateStatusMessageWithWarning(ctx, b, sentMsg, currentServer)
		return
	}

	ch.updateStatusMessageWithResult(ctx, b, sentMsg, currentServer, currentResult)
}

func (ch *CommandHandlers) sendNoActiveServerMessage(ctx context.Context, b *bot.Bot, chatID int64) {
	suggestions := []string{
		"Use `/start` to view available servers",
		"Select a server to activate",
		"Test server connections with `/ping`",
	}
	message := ch.messageFormatter.FormatErrorMessage("No Active Server",
		"No server is currently selected or active", suggestions)

	keyboard := ch.navigationHelper.CreateErrorNavigationKeyboard("no_servers", "refresh")

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send 'no active server' message: %v", err)
	} else {
		ch.bot.logger.Info("Successfully sent 'no active server' message to user %d", chatID)
	}
}

func (ch *CommandHandlers) updateStatusMessageWithError(ctx context.Context, b *bot.Bot, sentMsg *models.Message, server *Server, testErr error) {
	// Create a mock ping result with error for formatting
	mockResult := &types.PingResult{
		Server:    *server,
		Available: false,
		Error:     testErr,
	}

	updatedMessage := ch.messageFormatter.FormatServerStatusMessage(server, mockResult)

	// Add suggestions
	updatedMessage += "\n💡 Suggestions\n" +
		"└ Check your internet connection\n" +
		"└ Try a different server\n" +
		"└ Refresh server list"

	keyboard := ch.navigationHelper.CreateErrorNavigationKeyboard("server_switch", "ping_test")

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      sentMsg.Chat.ID,
		MessageID:   sentMsg.ID,
		Text:        updatedMessage,
		ReplyMarkup: keyboard,
	})
}

func (ch *CommandHandlers) updateStatusMessageWithWarning(ctx context.Context, b *bot.Bot, sentMsg *models.Message, server *Server) {
	updatedMessage := ch.messageFormatter.FormatServerStatusMessage(server, nil)

	// Add warning section
	updatedMessage += "\n⚠️ Warning\n" +
		"└ Server not found in available servers\n" +
		"└ Configuration may have changed"

	keyboard := ch.navigationHelper.CreateErrorNavigationKeyboard("server_load", "refresh")

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      sentMsg.Chat.ID,
		MessageID:   sentMsg.ID,
		Text:        updatedMessage,
		ReplyMarkup: keyboard,
	})
}

func (ch *CommandHandlers) updateStatusMessageWithResult(ctx context.Context, b *bot.Bot, sentMsg *models.Message, server *Server, result *ServerPingResult) {
	// Convert ServerPingResult to types.PingResult for formatting
	pingResult := &types.PingResult{
		Server:    *server,
		Available: result.Available,
		Latency:   result.Latency,
		Error:     result.Error,
	}

	if result.Available {
		ch.bot.logger.Debug("Server %s is available with latency %dms", server.Name, result.Latency)
	} else {
		ch.bot.logger.Debug("Server %s is not available, error: %v", server.Name, result.Error)
	}

	updatedMessage := ch.messageFormatter.FormatServerStatusMessage(server, pingResult)

	keyboard := ch.navigationHelper.CreateServerStatusNavigationKeyboard(true)

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      sentMsg.Chat.ID,
		MessageID:   sentMsg.ID,
		Text:        updatedMessage,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to edit final status message: %v", err)
	} else {
		ch.bot.logger.Info("Successfully sent complete status information to user %d", sentMsg.Chat.ID)
	}
}

func (ch *CommandHandlers) sendRateLimitMessage(ctx context.Context, b *bot.Bot, chatID int64) {
	message := ch.messageFormatter.FormatRateLimitMessage()

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send rate limit message: %v", err)
	}
}

func (ch *CommandHandlers) sendErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, title, description, retryAction string) {
	suggestions := []string{
		"Try the retry button below",
		"Check your connection and try again",
	}
	message := ch.messageFormatter.FormatErrorMessage(title, description, suggestions)

	keyboard := ch.navigationHelper.CreateErrorNavigationKeyboard("general", retryAction)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send error message '%s': %v", title, err)
	}
}

func (ch *CommandHandlers) sendNoServersMessage(ctx context.Context, b *bot.Bot, chatID int64) {
	message := ch.messageFormatter.FormatNoServersMessage()

	keyboard := ch.navigationHelper.CreateErrorNavigationKeyboard("no_servers", "refresh")

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send no servers message: %v", err)
	}
}

func (ch *CommandHandlers) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	username := getUsername(update.Message.From)
	ch.bot.logger.Info("Received /update command from user %d (%s)", userID, username)

	if !ch.bot.isAuthorized(userID) {
		ch.bot.logger.Warn("Unauthorized access attempt from user %d (%s) for /update command", userID, username)
		ch.bot.sendUnauthorizedMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	if !ch.bot.rateLimiter.IsAllowed(userID) {
		ch.bot.logger.Warn("Rate limit exceeded for user %d (%s)", userID, username)
		ch.sendRateLimitMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	ch.bot.logger.Debug("User %d is authorized, processing /update command", userID)

	// Check if update is already in progress
	status := ch.updateManager.GetUpdateStatus()
	if status.InProgress {
		ch.bot.logger.Debug("Update already in progress for user %d", userID)
		ch.sendUpdateInProgressMessage(ctx, b, update.Message.Chat.ID, status)
		return
	}

	// Send initial update message
	message := "🔄 Bot Update\n\n" +
		"⚠️ Warning: This will update the bot to the latest version and restart the service.\n\n" +
		"📋 What will happen:\n" +
		"• Download latest update script\n" +
		"• Create configuration backup (if enabled)\n" +
		"• Install updates\n" +
		"• Restart bot service\n\n" +
		"⏱️ Estimated time: 2-5 minutes\n" +
		"🔌 Connection: Will be briefly interrupted\n\n" +
		"Are you sure you want to proceed?"

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Yes, Update Bot", CallbackData: "confirm_update"},
			},
			{
				{Text: "❌ Cancel", CallbackData: "main_menu"},
				{Text: "ℹ️ Check Status", CallbackData: "update_status"},
			},
		},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send update confirmation message: %v", err)
	} else {
		ch.bot.logger.Info("Successfully sent update confirmation to user %d", userID)
	}
}

func (ch *CommandHandlers) handleUpdateConfirm(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	ch.bot.logger.Info("Processing update confirmation for user %d", chatID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Starting update...",
	})

	// Check if update is already in progress
	status := ch.updateManager.GetUpdateStatus()
	if status.InProgress {
		ch.bot.logger.Debug("Update already in progress for user %d", chatID)
		ch.sendUpdateInProgressMessage(ctx, b, chatID, status)
		return
	}

	// Send initial progress message
	message := "🔄 Bot Update Started\n\n" +
		"📊 Progress: 0%\n" +
		"📋 Stage: Initializing...\n\n" +
		"⏳ Please wait while the update is being processed.\n" +
		"🔔 You will be notified when the update is complete."

	content := MessageContent{
		Text: message,
		Type: MessageTypeStatus,
	}

	if err := ch.bot.messageManager.SendOrEdit(ctx, chatID, content); err != nil {
		ch.bot.logger.Error("Failed to send initial update progress message: %v", err)
		return
	}

	var messageID int
	if activeMsg := ch.bot.messageManager.GetActiveMessage(chatID); activeMsg != nil {
		messageID = activeMsg.MessageID
	} else {
		ch.bot.logger.Error("Could not retrieve active message ID for update progress")
		return
	}

	// Start monitoring progress updates
	progressChan := ch.updateManager.StartProgressMonitoring()
	defer ch.updateManager.StopProgressMonitoring()

	// Start the update process in a goroutine
	go func() {
		updateErr := ch.updateManager.ExecuteUpdate(ctx)
		if updateErr != nil {
			ch.bot.logger.Error("Update failed: %v", updateErr)
			ch.sendUpdateErrorMessage(ctx, b, chatID, messageID, updateErr)
		}
	}()

	// Monitor progress updates
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute) // Maximum update time

	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				// Channel closed, update completed
				ch.sendUpdateCompleteMessage(ctx, b, chatID, messageID)
				return
			}

			if progress.Error != nil {
				ch.sendUpdateErrorMessage(ctx, b, chatID, messageID, progress.Error)
				return
			}

			// Update progress message
			ch.updateProgressMessage(ctx, b, chatID, messageID, progress)

		case <-ticker.C:
			// Check if update completed
			status := ch.updateManager.GetUpdateStatus()
			if !status.InProgress {
				if status.Error != nil {
					ch.sendUpdateErrorMessage(ctx, b, chatID, messageID, status.Error)
				} else {
					ch.sendUpdateCompleteMessage(ctx, b, chatID, messageID)
				}
				return
			}

		case <-timeout:
			ch.bot.logger.Error("Update timeout for user %d", chatID)
			ch.sendUpdateTimeoutMessage(ctx, b, chatID, messageID)
			return

		case <-ctx.Done():
			ch.bot.logger.Info("Update cancelled due to context cancellation for user %d", chatID)
			return
		}
	}
}

func (ch *CommandHandlers) sendUpdateInProgressMessage(ctx context.Context, b *bot.Bot, chatID int64, status UpdateStatus) {
	elapsed := time.Since(status.StartedAt)
	message := fmt.Sprintf("🔄 Update Already in Progress\n\n"+
		"📊 Progress: %d%%\n"+
		"📋 Stage: %s\n"+
		"⏱️ Running for: %s\n\n"+
		"⏳ Please wait for the current update to complete.",
		status.Progress,
		status.Stage,
		elapsed.Round(time.Second))

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Refresh Status", CallbackData: "update_status"},
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	content := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeStatus,
	}

	if err := ch.bot.messageManager.SendOrEdit(ctx, chatID, content); err != nil {
		ch.bot.logger.Error("Failed to send update in progress message: %v", err)
	}
}

func (ch *CommandHandlers) updateProgressMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, progress UpdateProgress) {
	var stageEmoji string
	switch progress.Stage {
	case "downloading":
		stageEmoji = "📥"
	case "backing_up":
		stageEmoji = "💾"
	case "installing":
		stageEmoji = "⚙️"
	case "completing":
		stageEmoji = "✅"
	default:
		stageEmoji = "🔄"
	}

	progressBar := ch.messageFormatter.createProgressBar(progress.Progress, 20)

	message := fmt.Sprintf("🔄 Bot Update in Progress\n\n"+
		"📊 Progress: %d%%\n"+
		"%s\n\n"+
		"%s Stage: %s\n"+
		"💬 %s\n\n"+
		"⏳ Please wait...",
		progress.Progress,
		progressBar,
		stageEmoji,
		progress.Stage,
		progress.Message)

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      message,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to update progress message: %v", err)
	}
}

func (ch *CommandHandlers) sendUpdateCompleteMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	message := "✅ Bot Update Complete\n\n" +
		"🎉 Success! The bot has been updated to the latest version.\n\n" +
		"📋 What was done:\n" +
		"• ✅ Downloaded latest update script\n" +
		"• ✅ Created configuration backup\n" +
		"• ✅ Installed updates\n" +
		"• ✅ Restarted bot service\n\n" +
		"🟢 Status: Bot is now running the latest version\n" +
		"🔄 Service: Fully operational\n\n" +
		"💡 You can now continue using the bot normally."

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 Server List", CallbackData: "refresh"},
				{Text: "📊 Test Servers", CallbackData: "ping_test"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send update complete message: %v", err)
	} else {
		ch.bot.logger.Info("Successfully sent update complete message to user %d", chatID)
	}
}

func (ch *CommandHandlers) sendUpdateErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, updateErr error) {
	message := fmt.Sprintf("❌ Bot Update Failed\n\n"+
		"🔴 Error: %s\n\n"+
		"📋 Possible causes:\n"+
		"• Network connectivity issues\n"+
		"• Server maintenance\n"+
		"• Insufficient permissions\n"+
		"• Script execution failure\n\n"+
		"💡 Next steps:\n"+
		"• Check your internet connection\n"+
		"• Try again in a few minutes\n"+
		"• Contact support if the issue persists",
		updateErr.Error())

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Try Again", CallbackData: "confirm_update"},
				{Text: "ℹ️ Check Status", CallbackData: "update_status"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send update error message: %v", err)
	} else {
		ch.bot.logger.Info("Successfully sent update error message to user %d", chatID)
	}
}

func (ch *CommandHandlers) sendUpdateTimeoutMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	message := "⏰ Update Timeout\n\n" +
		"🔴 Timeout: The update process took longer than expected.\n\n" +
		"📋 What happened:\n" +
		"• Update process exceeded maximum time limit\n" +
		"• The update may still be running in the background\n" +
		"• Bot service status is uncertain\n\n" +
		"💡 Next steps:\n" +
		"• Wait a few minutes and check status\n" +
		"• Try testing bot functionality\n" +
		"• Contact support if issues persist"

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "ℹ️ Check Status", CallbackData: "update_status"},
				{Text: "📊 Test Bot", CallbackData: "ping_test"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send update timeout message: %v", err)
	}
}

func (ch *CommandHandlers) handleUpdateStatus(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	ch.bot.logger.Info("Processing update status request for user %d", chatID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "ℹ️ Checking status...",
	})

	status := ch.updateManager.GetUpdateStatus()
	currentVersion := ch.updateManager.GetCurrentVersion()

	var message string
	var keyboard *models.InlineKeyboardMarkup

	if status.InProgress {
		elapsed := time.Since(status.StartedAt)
		message = fmt.Sprintf("🔄 Update Status: In Progress\n\n"+
			"📊 Progress: %d%%\n"+
			"📋 Stage: %s\n"+
			"⏱️ Running for: %s\n"+
			"🏷️ Current version: %s\n\n"+
			"⏳ Please wait for the update to complete.",
			status.Progress,
			status.Stage,
			elapsed.Round(time.Second),
			currentVersion)

		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🔄 Refresh", CallbackData: "update_status"},
				},
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}
	} else if status.Error != nil {
		elapsed := status.CompletedAt.Sub(status.StartedAt)
		message = fmt.Sprintf("❌ Update Status: Failed\n\n"+
			"🔴 Error: %s\n"+
			"⏱️ Duration: %s\n"+
			"🏷️ Current version: %s\n\n"+
			"💡 The bot is still running on the previous version.",
			status.Error.Error(),
			elapsed.Round(time.Second),
			currentVersion)

		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🔄 Try Update", CallbackData: "confirm_update"},
					{Text: "📊 Test Bot", CallbackData: "ping_test"},
				},
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}
	} else if !status.StartedAt.IsZero() {
		elapsed := status.CompletedAt.Sub(status.StartedAt)
		message = fmt.Sprintf("✅ Update Status: Completed\n\n"+
			"🎉 Last update: Successful\n"+
			"⏱️ Duration: %s\n"+
			"🕐 Completed: %s\n"+
			"🏷️ Current version: %s\n\n"+
			"🟢 Bot is running the latest version.",
			elapsed.Round(time.Second),
			status.CompletedAt.Format("15:04:05"),
			currentVersion)

		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "📋 Server List", CallbackData: "refresh"},
					{Text: "📊 Test Servers", CallbackData: "ping_test"},
				},
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}
	} else {
		message = fmt.Sprintf("ℹ️ Update Status: Ready\n\n"+
			"🏷️ Current version: %s\n"+
			"📋 Status: No recent updates\n\n"+
			"💡 You can start an update anytime using the button below.",
			currentVersion)

		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🔄 Start Update", CallbackData: "confirm_update"},
				},
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}
	}

	content := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeMenu,
	}

	if err := ch.bot.messageManager.SendOrEdit(ctx, chatID, content); err != nil {
		ch.bot.logger.Error("Failed to send update status message: %v", err)
	} else {
		ch.bot.logger.Info("Successfully sent update status to user %d", chatID)
	}
}
