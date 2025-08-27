package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type CommandHandlers struct {
	bot *TelegramBot
}

func NewCommandHandlers(tb *TelegramBot) *CommandHandlers {
	return &CommandHandlers{bot: tb}
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
		ch.sendErrorMessage(ctx, b, update.Message.Chat.ID, "Failed to load servers", err.Error())
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
	message := fmt.Sprintf("🚀 *Xray Telegram Manager*\n\n"+
		"Welcome! I can help you manage your xray proxy servers\\.\n\n"+
		"📊 Available servers: %d\n\n"+
		"Use the buttons below to interact with the system:",
		len(servers))

	keyboard := ch.bot.createMainMenuKeyboard()
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

	message := fmt.Sprintf("📊 Current Server Status\n\n"+
		"🏷️ Name: `%s`\n"+
		"🌐 Address: `%s:%d`\n"+
		"🔗 Protocol: `%s`\n"+
		"🏷️ Tag: `%s`\n\n"+
		"⏳ Testing connection...",
		currentServer.Name,
		currentServer.Address,
		currentServer.Port,
		currentServer.Protocol,
		currentServer.Tag)

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
	message := "❌ No Active Server\n\n" +
		"🔴 No server is currently selected or active\\.\n\n" +
		"Next Steps:\n" +
		"• Use `/start` to view available servers\n" +
		"• Select a server to activate\n" +
		"• Test server connections with `/ping`"

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 View Servers", CallbackData: "refresh"},
				{Text: "📊 Test Servers", CallbackData: "ping_test"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

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
	updatedMessage := fmt.Sprintf("📊 Current Server Status\n\n"+
		"🏷️ Name: `%s`\n"+
		"🌐 Address: `%s:%d`\n"+
		"🔗 Protocol: `%s`\n"+
		"🏷️ Tag: `%s`\n\n"+
		"❌ Status: Connection test failed\n"+
		"🔴 Error: `%s`\n\n"+
		"💡 Suggestions:\n"+
		"• Check your internet connection\n"+
		"• Try a different server\n"+
		"• Refresh server list",
		server.Name,
		server.Address,
		server.Port,
		server.Protocol,
		server.Tag,
		testErr.Error())

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Test Again", CallbackData: "ping_test"},
				{Text: "📋 Switch Server", CallbackData: "refresh"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      sentMsg.Chat.ID,
		MessageID:   sentMsg.ID,
		Text:        updatedMessage,
		ReplyMarkup: keyboard,
	})
}

func (ch *CommandHandlers) updateStatusMessageWithWarning(ctx context.Context, b *bot.Bot, sentMsg *models.Message, server *Server) {
	updatedMessage := fmt.Sprintf("📊 Current Server Status\n\n"+
		"🏷️ Name: `%s`\n"+
		"🌐 Address: `%s:%d`\n"+
		"🔗 Protocol: `%s`\n"+
		"🏷️ Tag: `%s`\n\n"+
		"⚠️ Status: Server not found in available servers\n\n"+
		"💡 This may indicate the server configuration has changed.",
		server.Name,
		server.Address,
		server.Port,
		server.Protocol,
		server.Tag)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Refresh Servers", CallbackData: "refresh"},
				{Text: "📊 Test All", CallbackData: "ping_test"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      sentMsg.Chat.ID,
		MessageID:   sentMsg.ID,
		Text:        updatedMessage,
		ReplyMarkup: keyboard,
	})
}

func (ch *CommandHandlers) updateStatusMessageWithResult(ctx context.Context, b *bot.Bot, sentMsg *models.Message, server *Server, result *ServerPingResult) {
	var statusIcon, statusText, latencyText, healthStatus string
	if result.Available {
		ch.bot.logger.Debug("Server %s is available with latency %dms", server.Name, result.Latency)
		statusIcon = "✅"
		statusText = "Connected"
		latencyText = fmt.Sprintf("⚡ Latency: %dms", result.Latency)

		if result.Latency < 100 {
			healthStatus = "🟢 Quality: Excellent"
		} else if result.Latency < 300 {
			healthStatus = "🟡 Quality: Good"
		} else {
			healthStatus = "🟠 Quality: Fair"
		}
	} else {
		ch.bot.logger.Debug("Server %s is not available, error: %v", server.Name, result.Error)
		statusIcon = "❌"
		statusText = "Disconnected"
		latencyText = fmt.Sprintf("🔴 Error: `%s`", result.Error.Error())
		healthStatus = "🔴 Quality: Unavailable"
	}

	updatedMessage := fmt.Sprintf("📊 Current Server Status\n\n"+
		"🏷️ Name: `%s`\n"+
		"🌐 Address: `%s:%d`\n"+
		"🔗 Protocol: `%s`\n"+
		"🏷️ Tag: `%s`\n\n"+
		"%s Status: %s\n"+
		"%s\n"+
		"%s\n\n"+
		"🕐 Last checked: %s",
		server.Name,
		server.Address,
		server.Port,
		server.Protocol,
		server.Tag,
		statusIcon,
		statusText,
		latencyText,
		healthStatus,
		time.Now().Format("15:04:05"))

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Test Again", CallbackData: "ping_test"},
				{Text: "📋 Switch Server", CallbackData: "refresh"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

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
	message := "⚠️ Rate Limit Exceeded\n\n" +
		"You are sending requests too quickly\\. Please wait a moment and try again\\."

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send rate limit message: %v", err)
	}
}

func (ch *CommandHandlers) sendErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, title, description string) {
	message := fmt.Sprintf("❌ %s\n\n🔴 Error: `%s`",
		title, description)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Retry", CallbackData: "refresh"},
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

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
	message := "❌ No Servers Available\n\n" +
		"No servers were found\\. Please check your subscription configuration\\."

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Retry", CallbackData: "refresh"},
			},
		},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		ch.bot.logger.Error("Failed to send no servers message: %v", err)
	}
}
