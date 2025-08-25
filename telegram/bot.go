package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"
	"xray-telegram-manager/types"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TelegramBot represents the Telegram bot interface
type TelegramBot struct {
	bot       *bot.Bot
	config    ConfigProvider
	serverMgr ServerManager
	logger    Logger
}

// Logger interface for logging
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// ConfigProvider interface for accessing configuration
type ConfigProvider interface {
	GetAdminID() int64
	GetBotToken() string
}

// LoggerProvider interface for accessing logger
type LoggerProvider interface {
	GetLogger() Logger
}

// ServerManager interface for server management operations
type ServerManager interface {
	LoadServers() error
	GetServers() []types.Server
	SwitchServer(serverID string) error
	GetCurrentServer() *types.Server
	TestPing() ([]types.PingResult, error)
	TestPingWithProgress(progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error)
}

// NewTelegramBot creates a new Telegram bot instance
func NewTelegramBot(config ConfigProvider, serverMgr ServerManager, logger Logger) (*TelegramBot, error) {
	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Log unhandled messages for debugging
			if update.Message != nil {
				logger.Debug("Unhandled message from user %d: %s", update.Message.From.ID, update.Message.Text)
			} else if update.CallbackQuery != nil {
				logger.Debug("Unhandled callback query from user %d: %s", update.CallbackQuery.From.ID, update.CallbackQuery.Data)
			} else {
				logger.Debug("Unhandled update type: %+v", update)
			}
		}),
	}

	b, err := bot.New(config.GetBotToken(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	logger.Info("Telegram bot created successfully for admin ID: %d", config.GetAdminID())

	return &TelegramBot{
		bot:       b,
		config:    config,
		serverMgr: serverMgr,
		logger:    logger,
	}, nil
}

// Start starts the Telegram bot
func (tb *TelegramBot) Start(ctx context.Context) error {
	// Register command handlers
	tb.registerHandlers()

	tb.logger.Info("Starting Telegram bot...")

	// Start the bot
	tb.bot.Start(ctx)
	tb.logger.Info("Telegram bot started and listening for messages")
	return nil
}

// Stop stops the Telegram bot
func (tb *TelegramBot) Stop() {
	// Bot will be stopped when context is cancelled
}

// registerHandlers registers all command handlers
func (tb *TelegramBot) registerHandlers() {
	tb.logger.Debug("Registering Telegram bot handlers...")

	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, tb.handleStart)
	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/list", bot.MatchTypeExact, tb.handleList)
	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/status", bot.MatchTypeExact, tb.handleStatus)
	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/ping", bot.MatchTypeExact, tb.handlePing)
	tb.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, tb.handleCallback)

	tb.logger.Info("Registered handlers for commands: /start, /list, /status, /ping and callback queries")
}

// isAuthorized checks if the user is authorized to use the bot
func (tb *TelegramBot) isAuthorized(userID int64) bool {
	return userID == tb.config.GetAdminID()
}

// sendUnauthorizedMessage sends an unauthorized access message
func (tb *TelegramBot) sendUnauthorizedMessage(ctx context.Context, b *bot.Bot, chatID int64) {
	tb.logger.Debug("Sending unauthorized access message to user %d", chatID)
	message := "❌ Unauthorized access. This bot is restricted to the configured admin user."
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,
	})

	if err != nil {
		tb.logger.Error("Failed to send unauthorized message: %v", err)
	} else {
		tb.logger.Debug("Successfully sent unauthorized message to user %d", chatID)
	}
}

// handleStart handles the /start command
func (tb *TelegramBot) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.Username
	tb.logger.Info("Received /start command from user %d (@%s)", userID, username)

	if !tb.isAuthorized(userID) {
		tb.logger.Warn("Unauthorized access attempt from user %d (@%s)", userID, username)
		tb.sendUnauthorizedMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	tb.logger.Debug("User %d is authorized, processing /start command", userID)

	// Load servers to ensure we have the latest list
	tb.logger.Debug("Loading servers for /start command...")
	if err := tb.serverMgr.LoadServers(); err != nil {
		tb.logger.Error("Failed to load servers for /start command: %v", err)
		message := fmt.Sprintf("❌ Failed to load servers: %v", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   message,
		})
		return
	}

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Loaded %d servers for /start command", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for /start command")
		message := "❌ No servers available. Please check your subscription configuration."
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   message,
		})
		return
	}

	tb.logger.Debug("Sending welcome message with %d servers", len(servers))
	message := fmt.Sprintf("🚀 Xray Telegram Manager\n\nWelcome! I can help you manage your xray proxy servers.\n\n📊 Available servers: %d\n\nUse the buttons below to interact with the system:", len(servers))

	keyboard := tb.createMainMenuKeyboard()
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to send welcome message: %v", err)
	} else {
		tb.logger.Info("Successfully sent welcome message to user %d", userID)
	}
}

// handleList handles the /list command
func (tb *TelegramBot) handleList(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.Username
	tb.logger.Info("Received /list command from user %d (@%s)", userID, username)

	if !tb.isAuthorized(userID) {
		tb.logger.Warn("Unauthorized access attempt from user %d (@%s) for /list command", userID, username)
		tb.sendUnauthorizedMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	tb.logger.Debug("User %d is authorized, processing /list command", userID)

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Retrieved %d servers for /list command", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for /list command")
		message := "❌ No servers available. Use /start to refresh the server list."
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   message,
		})
		return
	}

	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	message := "📋 Available Servers:\n\n"
	for i, server := range servers {
		status := ""
		if server.ID == currentServerID {
			status = " ✅ Current"
		}
		message += fmt.Sprintf("%d. %s%s\n", i+1, server.Name, status)
	}

	keyboard := tb.createServerListKeyboard(servers, 0)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to send server list message: %v", err)
	} else {
		tb.logger.Info("Successfully sent server list to user %d", userID)
	}
}

// handleStatus handles the /status command
func (tb *TelegramBot) handleStatus(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.Username
	tb.logger.Info("Received /status command from user %d (@%s)", userID, username)

	if !tb.isAuthorized(userID) {
		tb.logger.Warn("Unauthorized access attempt from user %d (@%s) for /status command", userID, username)
		tb.sendUnauthorizedMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	tb.logger.Debug("User %d is authorized, processing /status command", userID)

	currentServer := tb.serverMgr.GetCurrentServer()
	if currentServer == nil {
		tb.logger.Debug("No active server found for /status command")
		message := "❌ No Active Server\n\n🔴 No server is currently selected or active.\n\nNext Steps:\n• Use /start to view available servers\n• Select a server to activate\n• Test server connections with /ping"

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
			ChatID: update.Message.Chat.ID,
			Text:   message,

			ReplyMarkup: keyboard,
		})

		if err != nil {
			tb.logger.Error("Failed to send 'no active server' message: %v", err)
		} else {
			tb.logger.Info("Successfully sent 'no active server' message to user %d", userID)
		}
		return
	}

	tb.logger.Debug("Found active server: %s (%s:%d) for /status command", currentServer.Name, currentServer.Address, currentServer.Port)

	// Send initial status message with progress indicator
	message := fmt.Sprintf("📊 Current Server Status\n\n"+
		"🏷️ Name: %s\n"+
		"🌐 Address: %s:%d\n"+
		"🔗 Protocol: %s\n"+
		"🏷️ Tag: %s\n\n"+
		"⏳ Testing connection...",
		currentServer.Name,
		currentServer.Address,
		currentServer.Port,
		currentServer.Protocol,
		currentServer.Tag)

	// Send initial message
	sentMsg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   message,
	})
	if err != nil {
		tb.logger.Error("Failed to send initial status message: %v", err)
		return
	}

	tb.logger.Debug("Sent initial status message, starting ping test for server %s", currentServer.Name)

	// Test ping to current server
	results, err := tb.serverMgr.TestPing()
	if err != nil {
		tb.logger.Error("Ping test failed for /status command: %v", err)
		updatedMessage := fmt.Sprintf("📊 Current Server Status\n\n"+
			"🏷️ Name: %s\n"+
			"🌐 Address: %s:%d\n"+
			"🔗 Protocol: %s\n"+
			"🏷️ Tag: %s\n\n"+
			"❌ Status: Connection test failed\n"+
			"🔴 Error: %v\n\n"+
			"💡 Suggestions:\n• Check your internet connection\n• Try a different server\n• Refresh server list",
			currentServer.Name,
			currentServer.Address,
			currentServer.Port,
			currentServer.Protocol,
			currentServer.Tag,
			err)

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
			ChatID:    sentMsg.Chat.ID,
			MessageID: sentMsg.ID,
			Text:      updatedMessage,

			ReplyMarkup: keyboard,
		})
		return
	}

	// Find result for current server
	var currentResult *types.PingResult
	for _, result := range results {
		if result.Server.ID == currentServer.ID {
			currentResult = &result
			break
		}
	}

	if currentResult == nil {
		tb.logger.Warn("Current server not found in ping results for /status command")
		updatedMessage := fmt.Sprintf("📊 Current Server Status\n\n"+
			"🏷️ Name: %s\n"+
			"🌐 Address: %s:%d\n"+
			"🔗 Protocol: %s\n"+
			"🏷️ Tag: %s\n\n"+
			"⚠️ Status: Server not found in available servers\n\n"+
			"💡 This may indicate the server configuration has changed.",
			currentServer.Name,
			currentServer.Address,
			currentServer.Port,
			currentServer.Protocol,
			currentServer.Tag)

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
			ChatID:    sentMsg.Chat.ID,
			MessageID: sentMsg.ID,
			Text:      updatedMessage,

			ReplyMarkup: keyboard,
		})
		return
	}

	var statusIcon, statusText, latencyText, healthStatus string
	if currentResult.Available {
		tb.logger.Debug("Server %s is available with latency %dms", currentServer.Name, currentResult.Latency)
		statusIcon = "✅"
		statusText = "Connected"
		latencyText = fmt.Sprintf("⚡ Latency: %dms", currentResult.Latency)

		// Determine connection quality based on latency
		if currentResult.Latency < 100 {
			healthStatus = "🟢 Quality: Excellent"
		} else if currentResult.Latency < 300 {
			healthStatus = "🟡 Quality: Good"
		} else {
			healthStatus = "🟠 Quality: Fair"
		}
	} else {
		tb.logger.Debug("Server %s is not available, error: %v", currentServer.Name, currentResult.Error)
		statusIcon = "❌"
		statusText = "Disconnected"
		latencyText = fmt.Sprintf("🔴 Error: %v", currentResult.Error)
		healthStatus = "🔴 Quality: Unavailable"
	}

	updatedMessage := fmt.Sprintf("📊 Current Server Status\n\n"+
		"🏷️ Name: %s\n"+
		"🌐 Address: %s:%d\n"+
		"🔗 Protocol: %s\n"+
		"🏷️ Tag: %s\n\n"+
		"%s Status: %s\n"+
		"%s\n"+
		"%s\n\n"+
		"🕐 Last checked: %s",
		currentServer.Name,
		currentServer.Address,
		currentServer.Port,
		currentServer.Protocol,
		currentServer.Tag,
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

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    sentMsg.Chat.ID,
		MessageID: sentMsg.ID,
		Text:      updatedMessage,

		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to edit final status message: %v", err)
	} else {
		tb.logger.Info("Successfully sent complete status information to user %d", userID)
	}
}

// handlePing handles the /ping command
func (tb *TelegramBot) handlePing(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.Username
	tb.logger.Info("Received /ping command from user %d (@%s)", userID, username)

	if !tb.isAuthorized(userID) {
		tb.logger.Warn("Unauthorized access attempt from user %d (@%s) for /ping command", userID, username)
		tb.sendUnauthorizedMessage(ctx, b, update.Message.Chat.ID)
		return
	}

	tb.logger.Debug("User %d is authorized, processing /ping command", userID)
	// Use the same logic as the callback handler
	tb.handlePingTestCallback(ctx, b, update.Message.Chat.ID, "")
}

// handleCallback handles callback queries from inline keyboards
func (tb *TelegramBot) handleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.CallbackQuery.From.ID
	username := update.CallbackQuery.From.Username
	data := update.CallbackQuery.Data
	tb.logger.Info("Received callback query from user %d (@%s): %s", userID, username, data)

	if !tb.isAuthorized(userID) {
		tb.logger.Warn("Unauthorized callback query attempt from user %d (@%s): %s", userID, username, data)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "❌ Unauthorized access",
			ShowAlert:       true,
		})
		return
	}

	tb.logger.Debug("User %d is authorized, processing callback: %s", userID, data)

	// For callback queries, we'll send new messages instead of editing
	// This avoids the complexity of handling MaybeInaccessibleMessage
	chatID := update.CallbackQuery.From.ID

	// Handle different callback types
	switch {
	case data == "refresh":
		tb.logger.Debug("Processing refresh callback for user %d", userID)
		tb.handleRefreshCallback(ctx, b, chatID, update.CallbackQuery.ID)
	case data == "ping_test":
		tb.logger.Debug("Processing ping_test callback for user %d", userID)
		tb.handlePingTestCallback(ctx, b, chatID, update.CallbackQuery.ID)
	case data == "main_menu":
		tb.logger.Debug("Processing main_menu callback for user %d", userID)
		tb.handleMainMenuCallback(ctx, b, chatID, update.CallbackQuery.ID)
	case len(data) > 5 && data[:5] == "page_":
		// Handle pagination callbacks
		tb.logger.Debug("Processing pagination callback for user %d: %s", userID, data)
		tb.handlePaginationCallback(ctx, b, chatID, update.CallbackQuery.ID, data)
	case len(data) > 8 && data[:8] == "confirm_":
		// Handle server switch confirmation
		serverID := data[8:]
		tb.logger.Debug("Processing confirm_switch callback for user %d, server: %s", userID, serverID)
		tb.handleConfirmSwitchCallback(ctx, b, chatID, update.CallbackQuery.ID, serverID)
	case len(data) > 7 && data[:7] == "server_":
		serverID := data[7:]
		tb.logger.Debug("Processing server_select callback for user %d, server: %s", userID, serverID)
		tb.handleServerSelectCallback(ctx, b, chatID, update.CallbackQuery.ID, serverID)
	case data == "noop":
		// No-op callback for page indicator button
		tb.logger.Debug("Processing noop callback for user %d", userID)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})
	default:
		tb.logger.Warn("Unknown callback query from user %d: %s", userID, data)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "❌ Unknown command",
		})
	}
}

// createMainMenuKeyboard creates the main menu inline keyboard
func (tb *TelegramBot) createMainMenuKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 Server List", CallbackData: "refresh"},
				{Text: "📊 Ping Test", CallbackData: "ping_test"},
			},
		},
	}
}

// createServerListKeyboard creates an inline keyboard for server selection
func (tb *TelegramBot) createServerListKeyboard(servers []types.Server, page int) *models.InlineKeyboardMarkup {
	const serversPerPage = 32
	start := page * serversPerPage
	end := start + serversPerPage
	if end > len(servers) {
		end = len(servers)
	}

	var keyboard [][]models.InlineKeyboardButton
	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	// Add server buttons (1 per row for better readability)
	for i := start; i < end; i++ {
		server := servers[i]
		buttonText := server.Name
		if len(buttonText) > 50 {
			buttonText = buttonText[:47] + "..."
		}

		// Add current server indicator
		if server.ID == currentServerID {
			buttonText = "✅ " + buttonText
		} else {
			buttonText = "🌐 " + buttonText
		}

		row := []models.InlineKeyboardButton{
			{
				Text:         buttonText,
				CallbackData: fmt.Sprintf("server_%s", server.ID),
			},
		}

		keyboard = append(keyboard, row)
	}

	// Add pagination buttons if needed
	totalPages := (len(servers) + serversPerPage - 1) / serversPerPage
	if totalPages > 1 {
		var paginationRow []models.InlineKeyboardButton

		if page > 0 {
			paginationRow = append(paginationRow, models.InlineKeyboardButton{
				Text: "⬅️ Prev", CallbackData: fmt.Sprintf("page_%d", page-1),
			})
		}

		paginationRow = append(paginationRow, models.InlineKeyboardButton{
			Text: fmt.Sprintf("📄 %d/%d", page+1, totalPages), CallbackData: "noop",
		})

		if page < totalPages-1 {
			paginationRow = append(paginationRow, models.InlineKeyboardButton{
				Text: "Next ➡️", CallbackData: fmt.Sprintf("page_%d", page+1),
			})
		}

		keyboard = append(keyboard, paginationRow)
	}

	// Add control buttons
	keyboard = append(keyboard, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: "refresh"},
		{Text: "📊 Ping Test", CallbackData: "ping_test"},
	})

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// handleRefreshCallback handles the refresh button callback
func (tb *TelegramBot) handleRefreshCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing refresh callback for user %d", chatID)

	// Answer callback query first
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Refreshing server list...",
	})

	// Send loading message
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "🔄 Refreshing server list...\n⏳ Please wait...",
	})

	// Refresh servers
	tb.logger.Debug("Loading servers for refresh callback...")
	if err := tb.serverMgr.LoadServers(); err != nil {
		tb.logger.Error("Failed to load servers for refresh callback: %v", err)
		message := fmt.Sprintf("❌ Failed to refresh servers: %v", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   message,
		})
		return
	}

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Loaded %d servers for refresh callback", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for refresh callback")
		message := "❌ No servers available. Please check your subscription configuration."
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   message,
		})
		return
	}

	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	message := "📋 Available Servers:\n\n"
	for i, server := range servers {
		status := ""
		if server.ID == currentServerID {
			status = " ✅ Current"
		}
		message += fmt.Sprintf("%d. %s%s\n", i+1, server.Name, status)
	}

	keyboard := tb.createServerListKeyboard(servers, 0)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,

		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to send refreshed server list: %v", err)
	} else {
		tb.logger.Info("Successfully sent refreshed server list to user %d", chatID)
	}
}

// handlePingTestCallback handles the ping test button callback
func (tb *TelegramBot) handlePingTestCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing ping test callback for user %d", chatID)

	// Answer callback query first
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🏓 Starting ping test...",
	})

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Retrieved %d servers for ping test", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for ping testing")
		message := "❌ No servers available for ping testing."
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   message,
		})
		return
	}

	// Send initial progress message
	message := fmt.Sprintf("🏓 Ping Test Started\n\n📊 Testing %d servers...\n⏳ Progress: 0/%d\n\n🔄 Initializing...", len(servers), len(servers))
	progressMsg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,
	})
	if err != nil {
		return
	}

	// Create progress callback for live updates
	progressCallback := func(completed, total int, serverName string) {
		// Limit server name length for display
		displayName := serverName
		if len(displayName) > 30 {
			displayName = displayName[:27] + "..."
		}

		// Calculate progress percentage
		percentage := (completed * 100) / total
		progressBar := tb.createProgressBar(percentage, 20)

		updatedMessage := fmt.Sprintf("🏓 Ping Test in Progress\n\n� Testsing %d servers...\n⏳ Progress: %d/%d (%d%%)\n\n%s\n\n🔄 Last tested: %s",
			total, completed, total, percentage, progressBar, displayName)

		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    progressMsg.Chat.ID,
			MessageID: progressMsg.ID,
			Text:      updatedMessage,
		})
	}

	// Perform ping test with progress updates
	tb.logger.Debug("Starting ping test with progress updates for %d servers", len(servers))
	results, err := tb.serverMgr.TestPingWithProgress(progressCallback)
	if err != nil {
		tb.logger.Error("Ping test failed: %v", err)
		updatedMessage := fmt.Sprintf("🏓 Ping Test Failed\n\n❌ Error: %v\n\nPlease try again or check your network connection.", err)

		retryKeyboard := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🔄 Retry Ping Test", CallbackData: "ping_test"},
					{Text: "📋 Server List", CallbackData: "refresh"},
				},
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}

		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    progressMsg.Chat.ID,
			MessageID: progressMsg.ID,
			Text:      updatedMessage,

			ReplyMarkup: retryKeyboard,
		})
		return
	}

	// Format results with enhanced display
	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	// Count available servers
	availableCount := 0
	for _, result := range results {
		if result.Available {
			availableCount++
		}
	}

	tb.logger.Info("Ping test completed: %d/%d servers available", availableCount, len(results))

	message = fmt.Sprintf("🏓 Ping Test Complete\n\n📊 Summary: %d/%d servers available\n\n", availableCount, len(results))

	// Add top 5 fastest servers
	fastestCount := min(10, len(results))
	if availableCount > 0 {
		message += "⚡ Fastest Servers:\n"
		count := 0
		for _, result := range results {
			if result.Available && count < fastestCount {
				status := ""
				if result.Server.ID == currentServerID {
					status = " ✅ Current"
				}
				message += fmt.Sprintf("%d. %s%s ⚡ %dms\n",
					count+1, result.Server.Name, status, result.Latency)
				count++
			}
		}
		message += "\n"
	}

	// Add unavailable servers summary
	unavailableCount := len(results) - availableCount
	if unavailableCount > 0 {
		message += fmt.Sprintf("❌ Unavailable: %d servers\n\n", unavailableCount)
	}

	// Add action buttons
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 View All Servers", CallbackData: "refresh"},
				{Text: "🔄 Test Again", CallbackData: "ping_test"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    progressMsg.Chat.ID,
		MessageID: progressMsg.ID,
		Text:      message,

		ReplyMarkup: keyboard,
	})
}

// handleMainMenuCallback handles the main menu button callback
func (tb *TelegramBot) handleMainMenuCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing main menu callback for user %d", chatID)

	// Answer callback query first
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🏠 Main menu",
	})

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Retrieved %d servers for main menu", len(servers))

	message := fmt.Sprintf("🚀 Xray Telegram Manager\n\nWelcome! I can help you manage your xray proxy servers.\n\n� Availiable servers: %d\n\nUse the buttons below to interact with the system:", len(servers))

	keyboard := tb.createMainMenuKeyboard()
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,

		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to send main menu: %v", err)
	} else {
		tb.logger.Info("Successfully sent main menu to user %d", chatID)
	}
}

// handlePaginationCallback handles pagination button callbacks
func (tb *TelegramBot) handlePaginationCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string, data string) {
	tb.logger.Info("Processing pagination callback for user %d: %s", chatID, data)

	// Extract page number from callback data (format: "page_N")
	var page int
	if _, err := fmt.Sscanf(data, "page_%d", &page); err != nil {
		tb.logger.Error("Invalid page number in pagination callback: %s", data)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackQueryID,
			Text:            "❌ Invalid page number",
		})
		return
	}

	// Answer callback query first
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            fmt.Sprintf("📄 Page %d", page+1),
	})

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Retrieved %d servers for pagination", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for pagination")
		message := "❌ No servers available."
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   message,
		})
		return
	}

	// Validate page number
	const serversPerPage = 32
	totalPages := (len(servers) + serversPerPage - 1) / serversPerPage
	if page < 0 || page >= totalPages {
		tb.logger.Error("Invalid page number %d, total pages: %d", page, totalPages)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Invalid page number",
		})
		return
	}

	tb.logger.Debug("Showing page %d/%d for user %d", page+1, totalPages, chatID)

	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	// Build server list message for the requested page
	start := page * serversPerPage
	end := start + serversPerPage
	if end > len(servers) {
		end = len(servers)
	}

	message := fmt.Sprintf("📋 Available Servers (Page %d/%d):\n\n", page+1, totalPages)
	for i := start; i < end; i++ {
		server := servers[i]
		status := ""
		if server.ID == currentServerID {
			status = " ✅ Current"
		}
		message += fmt.Sprintf("%d. %s%s\n", i+1, server.Name, status)
	}

	keyboard := tb.createServerListKeyboard(servers, page)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,

		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to send pagination page %d: %v", page+1, err)
	} else {
		tb.logger.Info("Successfully sent page %d/%d to user %d", page+1, totalPages, chatID)
	}
}

// handleServerSelectCallback handles server selection callback
func (tb *TelegramBot) handleServerSelectCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string, serverID string) {
	tb.logger.Info("Processing server select callback for user %d, server: %s", chatID, serverID)

	// Find the server
	servers := tb.serverMgr.GetServers()
	var selectedServer *types.Server
	for _, server := range servers {
		if server.ID == serverID {
			selectedServer = &server
			break
		}
	}

	if selectedServer == nil {
		tb.logger.Error("Server not found for selection: %s", serverID)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackQueryID,
			Text:            "❌ Server not found",
			ShowAlert:       true,
		})
		return
	}

	tb.logger.Debug("Found server for selection: %s (%s:%d)", selectedServer.Name, selectedServer.Address, selectedServer.Port)

	// Check if this is already the current server
	currentServer := tb.serverMgr.GetCurrentServer()
	if currentServer != nil && currentServer.ID == serverID {
		tb.logger.Debug("Server %s is already active, showing status", selectedServer.Name)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackQueryID,
			Text:            "✅ This server is already active",
			ShowAlert:       true,
		})

		// Show current server status
		message := fmt.Sprintf("✅ Current Active Server\n\n�️ Name: %s\n*🌐 Address: %s:%d\n🔗 Protocol: %s\n🏷️ Tag: %s\n\n🟢 This server is already active and running.\n\n💡 You can test the connection or choose a different server.",
			selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol, selectedServer.Tag)

		keyboard := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "📊 Test Connection", CallbackData: "ping_test"},
					{Text: "📋 Choose Different", CallbackData: "refresh"},
				},
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}

		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   message,

			ReplyMarkup: keyboard,
		})

		if err != nil {
			tb.logger.Error("Failed to send 'server already active' message: %v", err)
		} else {
			tb.logger.Info("Successfully sent 'server already active' message to user %d", chatID)
		}
		return
	}

	// Show confirmation dialog
	tb.logger.Debug("Showing confirmation dialog for server switch to %s", selectedServer.Name)
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Preparing to switch...",
	})

	// Show current server info if available
	currentServerInfo := ""
	if currentServer != nil {
		currentServerInfo = fmt.Sprintf("\n🔄 Current: %s (%s:%d)\n", currentServer.Name, currentServer.Address, currentServer.Port)
	}

	// Send enhanced confirmation message with server details
	message := fmt.Sprintf("🔄 Confirm Server Switch\n\n"+
		"🎯 Switch to: %s\n"+
		"🌐 Address: %s:%d\n"+
		"🔗 Protocol: %s\n"+
		"🏷️ Tag: %s%s\n"+
		"⚠️ Warning: This will restart the xray service and briefly interrupt your connection.\n\n"+
		"Are you sure you want to proceed?",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol, selectedServer.Tag, currentServerInfo)

	confirmKeyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Yes, Switch Server", CallbackData: fmt.Sprintf("confirm_%s", serverID)},
			},
			{
				{Text: "❌ Cancel", CallbackData: "refresh"},
				{Text: "📊 Test First", CallbackData: "ping_test"},
			},
		},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,

		ReplyMarkup: confirmKeyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to send server switch confirmation: %v", err)
	} else {
		tb.logger.Info("Successfully sent server switch confirmation to user %d", chatID)
	}
}

// handleConfirmSwitchCallback handles the confirmation of server switching
func (tb *TelegramBot) handleConfirmSwitchCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string, serverID string) {
	tb.logger.Info("Processing server switch confirmation for user %d, server: %s", chatID, serverID)

	// Answer callback query first
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Switching server...",
	})

	// Find the server
	servers := tb.serverMgr.GetServers()
	var selectedServer *types.Server
	for _, server := range servers {
		if server.ID == serverID {
			selectedServer = &server
			break
		}
	}

	if selectedServer == nil {
		tb.logger.Error("Server not found for switch confirmation: %s", serverID)
		tb.sendErrorMessage(ctx, b, chatID, "Server not found", "The selected server could not be found. Please refresh the server list and try again.", "refresh")
		return
	}

	tb.logger.Debug("Starting server switch to: %s (%s:%d)", selectedServer.Name, selectedServer.Address, selectedServer.Port)

	// Send initial progress message
	message := fmt.Sprintf("🔄 Switching to Server\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 1/4: Preparing configuration...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	progressMsg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,
	})
	if err != nil {
		return
	}

	// Step 2: Creating backup
	time.Sleep(500 * time.Millisecond) // Brief pause for user experience
	message = fmt.Sprintf("🔄 Switching to Server\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 2/4: Creating backup...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    progressMsg.Chat.ID,
		MessageID: progressMsg.ID,
		Text:      message,
	})

	// Step 3: Updating configuration
	time.Sleep(500 * time.Millisecond)
	message = fmt.Sprintf("🔄 Switching to Server\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 3/4: Updating configuration...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    progressMsg.Chat.ID,
		MessageID: progressMsg.ID,
		Text:      message,
	})

	// Step 4: Restarting service
	time.Sleep(500 * time.Millisecond)
	message = fmt.Sprintf("🔄 Switching to Server\n\n�️ Nam*e: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 4/4: Restarting xray service...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    progressMsg.Chat.ID,
		MessageID: progressMsg.ID,
		Text:      message,
	})

	// Perform the actual server switch
	tb.logger.Debug("Executing server switch to %s", selectedServer.Name)
	if err := tb.serverMgr.SwitchServer(serverID); err != nil {
		tb.logger.Error("Server switch failed for %s: %v", selectedServer.Name, err)
		tb.sendSwitchErrorMessage(ctx, b, chatID, selectedServer, err)
		return
	}

	tb.logger.Info("Server switch successful to %s", selectedServer.Name)

	// Success - show completion message
	message = fmt.Sprintf("✅ Server Switch Successful\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n🏷️ Tag: %s\n\n🟢 Status: Active and ready\n⚡ Service: Xray restarted successfully\n\n🎉 You are now connected to the new server!",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol, selectedServer.Tag)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📊 Test Connection", CallbackData: "ping_test"},
				{Text: "📋 Server List", CallbackData: "refresh"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	// Edit the progress message with the final result
	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    progressMsg.Chat.ID,
		MessageID: progressMsg.ID,
		Text:      message,

		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to edit server switch success message: %v", err)
	} else {
		tb.logger.Info("Successfully completed server switch to %s for user %d", selectedServer.Name, chatID)
	}
}

// createProgressBar creates a visual progress bar
func (tb *TelegramBot) createProgressBar(percentage int, length int) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	filled := (percentage * length) / 100
	empty := length - filled

	bar := "["
	bar += strings.Repeat("█", filled)
	bar += strings.Repeat("░", empty)
	bar += "]"

	return bar
}

// sendErrorMessage sends a formatted error message with retry options
func (tb *TelegramBot) sendErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, title, description, retryAction string) {
	tb.logger.Debug("Sending error message to user %d: %s - %s", chatID, title, description)
	message := fmt.Sprintf("❌ %s\n\n🔴 Error: %s", title, description)

	var keyboard *models.InlineKeyboardMarkup
	switch retryAction {
	case "refresh":
		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🔄 Refresh Servers", CallbackData: "refresh"},
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}
	case "ping_test":
		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🔄 Retry Ping Test", CallbackData: "ping_test"},
					{Text: "📋 Server List", CallbackData: "refresh"},
				},
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}
	default:
		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🏠 Main Menu", CallbackData: "main_menu"},
				},
			},
		}
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,

		ReplyMarkup: keyboard,
	})

	if err != nil {
		tb.logger.Error("Failed to send error message '%s': %v", title, err)
	} else {
		tb.logger.Debug("Successfully sent error message '%s' to user %d", title, chatID)
	}
}

// sendSwitchErrorMessage sends a detailed error message for server switching failures
func (tb *TelegramBot) sendSwitchErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, server *types.Server, err error) {
	tb.logger.Error("Sending server switch error message to user %d for server %s: %v", chatID, server.Name, err)
	message := fmt.Sprintf("❌ Server Switch Failed\n\n🏷️ Server: %s\n🌐 Address: %s:%d\n\n🔴 Error Details:\n%v\n\n💡 Suggestions:\n• Check if the server is accessible\n• Try a different server\n• Refresh the server list\n• Check your network connection",
		server.Name, server.Address, server.Port, err)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Try Different Server", CallbackData: "refresh"},
				{Text: "📊 Test Servers", CallbackData: "ping_test"},
			},
			{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			},
		},
	}

	_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   message,

		ReplyMarkup: keyboard,
	})

	if sendErr != nil {
		tb.logger.Error("Failed to send server switch error message for %s: %v", server.Name, sendErr)
	} else {
		tb.logger.Info("Successfully sent server switch error message for %s to user %d", server.Name, chatID)
	}
}
