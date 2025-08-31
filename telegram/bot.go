package telegram

import (
	"context"
	"fmt"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type TelegramBot struct {
	bot                 *bot.Bot
	config              ConfigProvider
	serverMgr           ServerManager
	logger              Logger
	rateLimiter         *RateLimiter
	handlers            *CommandHandlers
	messageManager      *MessageManager
	buttonTextProcessor *ButtonTextProcessor
}

type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

type ConfigProvider interface {
	GetAdminID() int64
	GetBotToken() string
	GetUpdateConfig() config.UpdateConfig
}

type ServerManager interface {
	LoadServers() error
	GetServers() []types.Server
	SwitchServer(serverID string) error
	GetCurrentServer() *types.Server
	TestPing() ([]types.PingResult, error)
	TestPingWithProgress(progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error)
	GetQuickSelectServers(results []types.PingResult, limit int) []types.PingResult
}

func NewTelegramBot(config ConfigProvider, serverMgr ServerManager, logger Logger) (*TelegramBot, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if serverMgr == nil {
		return nil, fmt.Errorf("serverMgr cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
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

	rateLimiter := NewRateLimiter(10, time.Minute)

	tb := &TelegramBot{
		bot:         b,
		config:      config,
		serverMgr:   serverMgr,
		logger:      logger,
		rateLimiter: rateLimiter,
	}

	tb.messageManager = NewMessageManager(b, logger)
	tb.buttonTextProcessor = NewButtonTextProcessor(50) // Default max length of 50

	// Create UpdateManager with configuration
	updateCfg := config.GetUpdateConfig()
	timeout := time.Duration(updateCfg.TimeoutMinutes) * time.Minute
	updateManager := NewUpdateManager(updateCfg.ScriptURL, timeout, updateCfg.BackupConfig, logger)
	tb.handlers = NewCommandHandlers(tb, updateManager)

	return tb, nil
}

func (tb *TelegramBot) Start(ctx context.Context) error {
	tb.registerHandlers()

	// Start rate limiter cleanup routine
	go tb.rateLimiter.StartCleanupRoutine(ctx)

	// Start message manager cleanup routine
	go tb.messageManager.StartCleanupRoutine(ctx)

	tb.logger.Info("Starting Telegram bot...")

	// Start the bot
	tb.bot.Start(ctx)
	tb.logger.Info("Telegram bot started and listening for messages")
	return nil
}

func (tb *TelegramBot) Stop() {
}

// GetMessageManager returns the message manager instance
func (tb *TelegramBot) GetMessageManager() *MessageManager {
	return tb.messageManager
}

func (tb *TelegramBot) registerHandlers() {
	tb.logger.Debug("Registering Telegram bot handlers...")

	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, tb.handlers.handleStart)
	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/list", bot.MatchTypeExact, tb.handleList)
	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/status", bot.MatchTypeExact, tb.handlers.handleStatus)
	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/ping", bot.MatchTypeExact, tb.handlePing)
	tb.bot.RegisterHandler(bot.HandlerTypeMessageText, "/update", bot.MatchTypeExact, tb.handlers.handleUpdate)
	tb.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, tb.handleCallback)

	tb.logger.Info("Registered handlers for commands: /start, /list, /status, /ping, /update and callback queries")
}

func (tb *TelegramBot) isAuthorized(userID int64) bool {
	return userID == tb.config.GetAdminID()
}

func (tb *TelegramBot) sendUnauthorizedMessage(ctx context.Context, b *bot.Bot, chatID int64) {
	tb.logger.Debug("Sending unauthorized access message to user %d", chatID)

	messageFormatter := NewMessageFormatter()
	message := messageFormatter.FormatUnauthorizedMessage()

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
		messageFormatter := NewMessageFormatter()
		noServersContent := MessageContent{
			Text:        messageFormatter.FormatNoServersMessage(),
			ReplyMarkup: tb.createEmptyKeyboard(),
			Type:        MessageTypeServerList,
		}
		_ = tb.messageManager.SendNew(ctx, update.Message.Chat.ID, noServersContent)
		return
	}

	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	messageFormatter := NewMessageFormatter()
	message := messageFormatter.FormatServerListMessage(servers, currentServerID, 0, 1)

	keyboard := tb.createServerListKeyboard(servers, 0)
	serverListContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeServerList,
	}

	if err := tb.messageManager.SendNew(ctx, update.Message.Chat.ID, serverListContent); err != nil {
		tb.logger.Error("Failed to send server list message: %v", err)
	} else {
		tb.logger.Info("Successfully sent server list to user %d", userID)
	}
}

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
	tb.handlePingTestCallback(ctx, b, update.Message.Chat.ID, "")
}

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
	case data == "confirm_update":
		tb.logger.Debug("Processing confirm_update callback for user %d", userID)
		tb.handlers.handleUpdateConfirm(ctx, b, chatID, update.CallbackQuery.ID)
	case data == "update_status":
		tb.logger.Debug("Processing update_status callback for user %d", userID)
		tb.handlers.handleUpdateStatus(ctx, b, chatID, update.CallbackQuery.ID)
	case data == "update_menu":
		tb.logger.Debug("Processing update_menu callback for user %d", userID)
		tb.handleUpdateMenuCallback(ctx, b, chatID, update.CallbackQuery.ID)
	case data == "status":
		tb.logger.Debug("Processing status callback for user %d", userID)
		tb.handleStatusCallback(ctx, b, chatID, update.CallbackQuery.ID)
	case len(data) > 5 && data[:5] == "page_":
		tb.logger.Debug("Processing pagination callback for user %d: %s", userID, data)
		tb.handlePaginationCallback(ctx, b, chatID, update.CallbackQuery.ID, data)
	case len(data) > 8 && data[:8] == "confirm_":
		serverID := data[8:]
		tb.logger.Debug("Processing confirm_switch callback for user %d, server: %s", userID, serverID)
		tb.handleConfirmSwitchCallback(ctx, b, chatID, update.CallbackQuery.ID, serverID)
	case len(data) > 7 && data[:7] == "server_":
		serverID := data[7:]
		tb.logger.Debug("Processing server_select callback for user %d, server: %s", userID, serverID)
		tb.handleServerSelectCallback(ctx, b, chatID, update.CallbackQuery.ID, serverID)
	case data == "noop":
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

	for i := start; i < end; i++ {
		server := servers[i]

		// Determine status emoji
		var statusEmoji string
		if server.ID == currentServerID {
			statusEmoji = "✅"
		} else {
			statusEmoji = "🌐"
		}

		// Use ButtonTextProcessor to create properly formatted button text
		buttonText := tb.buttonTextProcessor.ProcessServerButtonText(server.Name, statusEmoji, 50)

		row := []models.InlineKeyboardButton{
			{
				Text:         buttonText,
				CallbackData: fmt.Sprintf("server_%s", server.ID),
			},
		}

		keyboard = append(keyboard, row)
	}

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

	keyboard = append(keyboard, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: "refresh"},
		{Text: "📊 Ping Test", CallbackData: "ping_test"},
	})

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// createEmptyKeyboard creates an empty inline keyboard for messages that don't need buttons
func (tb *TelegramBot) createEmptyKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}}
}

func (tb *TelegramBot) handleRefreshCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing refresh callback for user %d", chatID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Refreshing server list...",
	})

	// Show loading message using MessageManager
	loadingContent := MessageContent{
		Text:        "🔄 Refreshing server list...\n⏳ Please wait...",
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
		Type:        MessageTypeServerList,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, loadingContent); err != nil {
		tb.logger.Error("Failed to send loading message: %v", err)
		return
	}

	tb.logger.Debug("Loading servers for refresh callback...")
	if err := tb.serverMgr.LoadServers(); err != nil {
		tb.logger.Error("Failed to load servers for refresh callback: %v", err)
		messageFormatter := NewMessageFormatter()
		suggestions := []string{
			"Check your internet connection",
			"Verify subscription configuration",
			"Try again in a few moments",
		}
		errorContent := MessageContent{
			Text:        messageFormatter.FormatErrorMessage("Failed to Refresh Servers", err.Error(), suggestions),
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
			Type:        MessageTypeServerList,
		}
		_ = tb.messageManager.SendOrEdit(ctx, chatID, errorContent)
		return
	}

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Loaded %d servers for refresh callback", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for refresh callback")
		messageFormatter := NewMessageFormatter()
		noServersContent := MessageContent{
			Text:        messageFormatter.FormatNoServersMessage(),
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
			Type:        MessageTypeServerList,
		}
		_ = tb.messageManager.SendOrEdit(ctx, chatID, noServersContent)
		return
	}

	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	messageFormatter := NewMessageFormatter()
	message := messageFormatter.FormatServerListMessage(servers, currentServerID, 0, 1)

	keyboard := tb.createServerListKeyboard(servers, 0)
	serverListContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeServerList,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, serverListContent); err != nil {
		tb.logger.Error("Failed to send refreshed server list: %v", err)
	} else {
		tb.logger.Info("Successfully sent refreshed server list to user %d", chatID)
	}
}

func (tb *TelegramBot) handlePingTestCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing ping test callback for user %d", chatID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🏓 Starting ping test...",
	})

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Retrieved %d servers for ping test", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for ping testing")
		messageFormatter := NewMessageFormatter()
		noServersContent := MessageContent{
			Text:        messageFormatter.FormatNoServersMessage(),
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
			Type:        MessageTypePingTest,
		}
		_ = tb.messageManager.SendOrEdit(ctx, chatID, noServersContent)
		return
	}

	// Send initial progress message using MessageManager
	messageFormatter := NewMessageFormatter()
	initialMessage := messageFormatter.FormatPingTestProgress(0, len(servers), "Initializing...")
	initialContent := MessageContent{
		Text:        initialMessage,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
		Type:        MessageTypePingTest,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, initialContent); err != nil {
		tb.logger.Error("Failed to send initial ping test message: %v", err)
		return
	}

	progressCallback := func(completed, total int, serverName string) {
		updatedMessage := messageFormatter.FormatPingTestProgress(completed, total, serverName)

		progressContent := MessageContent{
			Text:        updatedMessage,
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
			Type:        MessageTypePingTest,
		}

		// Use MessageManager for progress updates
		_ = tb.messageManager.SendOrEdit(ctx, chatID, progressContent)
	}

	tb.logger.Debug("Starting ping test with progress updates for %d servers", len(servers))
	results, err := tb.serverMgr.TestPingWithProgress(progressCallback)
	if err != nil {
		tb.logger.Error("Ping test failed: %v", err)
		// Force cleanup the user's active message since the operation failed
		tb.messageManager.ForceCleanupUser(chatID, "ping test failed")

		suggestions := []string{
			"Check your internet connection",
			"Try again in a few moments",
			"Verify server configuration",
		}
		errorMessage := messageFormatter.FormatErrorMessage("Ping Test Failed", err.Error(), suggestions)

		navigationHelper := NewNavigationHelper()
		retryKeyboard := navigationHelper.CreateErrorNavigationKeyboard("ping_test", "ping_test")

		errorContent := MessageContent{
			Text:        errorMessage,
			ReplyMarkup: retryKeyboard,
			Type:        MessageTypePingTest,
		}

		_ = tb.messageManager.SendOrEdit(ctx, chatID, errorContent)
		return
	}

	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	availableCount := 0
	for _, result := range results {
		if result.Available {
			availableCount++
		}
	}

	tb.logger.Info("Ping test completed: %d/%d servers available", availableCount, len(results))

	message := messageFormatter.FormatPingTestResults(results, currentServerID)

	// Create keyboard with quick select buttons for fastest servers
	navigationHelper := NewNavigationHelper()
	var keyboardRows [][]models.InlineKeyboardButton

	// Add quick select buttons for fastest servers using the new sorting
	if availableCount > 0 {
		// Use the server manager's quick select functionality
		quickSelectResults := tb.serverMgr.GetQuickSelectServers(results, 10)

		var quickSelectServers []QuickSelectServer
		for _, result := range quickSelectResults {
			// Process server name with emoji awareness
			processedServerName := tb.buttonTextProcessor.ProcessButtonText(result.Server.Name, 15)

			status := ""
			if result.Server.ID == currentServerID {
				status = "✅"
			} else {
				status = fmt.Sprintf("%dms", result.Latency)
			}

			// Create button text with proper formatting
			buttonText := fmt.Sprintf("%s (%s)", processedServerName, status)

			// Ensure the entire button text fits within reasonable limits
			finalButtonText := tb.buttonTextProcessor.ProcessButtonText(buttonText, 30)

			quickSelectServers = append(quickSelectServers, QuickSelectServer{
				ID:         result.Server.ID,
				ButtonText: finalButtonText,
			})
		}

		quickSelectRows := navigationHelper.CreateQuickSelectKeyboard(quickSelectServers)
		keyboardRows = append(keyboardRows, quickSelectRows...)
	}

	// Add standard navigation buttons
	pingNavKeyboard := navigationHelper.CreatePingTestNavigationKeyboard(availableCount > 0)
	keyboardRows = append(keyboardRows, pingNavKeyboard.InlineKeyboard...)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: keyboardRows,
	}

	// Use MessageManager for final results
	resultsContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypePingTest,
	}

	_ = tb.messageManager.SendOrEdit(ctx, chatID, resultsContent)
}

func (tb *TelegramBot) handleMainMenuCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing main menu callback for user %d", chatID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🏠 Main menu",
	})

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Retrieved %d servers for main menu", len(servers))

	messageFormatter := NewMessageFormatter()
	message := messageFormatter.FormatWelcomeMessage(len(servers))

	navigationHelper := NewNavigationHelper()
	keyboard := navigationHelper.CreateMainMenuKeyboard()
	mainMenuContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeMenu,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, mainMenuContent); err != nil {
		tb.logger.Error("Failed to send main menu: %v", err)
	} else {
		tb.logger.Info("Successfully sent main menu to user %d", chatID)
	}
}

func (tb *TelegramBot) handlePaginationCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string, data string) {
	tb.logger.Info("Processing pagination callback for user %d: %s", chatID, data)

	var page int
	if _, err := fmt.Sscanf(data, "page_%d", &page); err != nil {
		tb.logger.Error("Invalid page number in pagination callback: %s", data)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackQueryID,
			Text:            "❌ Invalid page number",
		})
		return
	}

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            fmt.Sprintf("📄 Page %d", page+1),
	})

	servers := tb.serverMgr.GetServers()
	tb.logger.Debug("Retrieved %d servers for pagination", len(servers))

	if len(servers) == 0 {
		tb.logger.Warn("No servers available for pagination")
		messageFormatter := NewMessageFormatter()
		noServersContent := MessageContent{
			Text: messageFormatter.FormatNoServersMessage(),
			Type: MessageTypeServerList,
		}
		_ = tb.messageManager.SendOrEdit(ctx, chatID, noServersContent)
		return
	}

	const serversPerPage = 32
	totalPages := (len(servers) + serversPerPage - 1) / serversPerPage
	if page < 0 || page >= totalPages {
		tb.logger.Error("Invalid page number %d, total pages: %d", page, totalPages)
		messageFormatter := NewMessageFormatter()
		suggestions := []string{
			"Use the navigation buttons",
			"Return to the first page",
		}
		invalidPageContent := MessageContent{
			Text: messageFormatter.FormatErrorMessage("Invalid Page", "The requested page number is out of range", suggestions),
			Type: MessageTypeServerList,
		}
		_ = tb.messageManager.SendOrEdit(ctx, chatID, invalidPageContent)
		return
	}

	tb.logger.Debug("Showing page %d/%d for user %d", page+1, totalPages, chatID)

	currentServer := tb.serverMgr.GetCurrentServer()
	var currentServerID string
	if currentServer != nil {
		currentServerID = currentServer.ID
	}

	messageFormatter := NewMessageFormatter()
	message := messageFormatter.FormatServerListMessage(servers, currentServerID, page, totalPages)

	keyboard := tb.createServerListKeyboard(servers, page)
	paginationContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeServerList,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, paginationContent); err != nil {
		tb.logger.Error("Failed to send pagination page %d: %v", page+1, err)
	} else {
		tb.logger.Info("Successfully sent page %d/%d to user %d", page+1, totalPages, chatID)
	}
}

func (tb *TelegramBot) handleServerSelectCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string, serverID string) {
	tb.logger.Info("Processing server select callback for user %d, server: %s", chatID, serverID)

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

	currentServer := tb.serverMgr.GetCurrentServer()
	if currentServer != nil && currentServer.ID == serverID {
		tb.logger.Debug("Server %s is already active, showing status", selectedServer.Name)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackQueryID,
			Text:            "✅ This server is already active",
			ShowAlert:       true,
		})

		messageFormatter := NewMessageFormatter()
		message := messageFormatter.FormatServerStatusMessage(selectedServer, nil)
		message += "\n🟢 This server is already active and running.\n\n💡 You can test the connection or choose a different server."

		navigationHelper := NewNavigationHelper()
		keyboard := navigationHelper.CreateServerStatusNavigationKeyboard(true)

		activeServerContent := MessageContent{
			Text:        message,
			ReplyMarkup: keyboard,
			Type:        MessageTypeStatus,
		}

		if err := tb.messageManager.SendOrEdit(ctx, chatID, activeServerContent); err != nil {
			tb.logger.Error("Failed to send 'server already active' message: %v", err)
		} else {
			tb.logger.Info("Successfully sent 'server already active' message to user %d", chatID)
		}
		return
	}

	tb.logger.Debug("Showing confirmation dialog for server switch to %s", selectedServer.Name)
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Preparing to switch...",
	})

	currentServerInfo := ""
	if currentServer != nil {
		currentServerInfo = fmt.Sprintf("\n🔄 Current: %s (%s:%d)\n", currentServer.Name, currentServer.Address, currentServer.Port)
	}

	message := fmt.Sprintf("🔄 Confirm Server Switch\n\n"+
		"🎯 Switch to: %s\n"+
		"🌐 Address: %s:%d\n"+
		"🔗 Protocol: %s\n"+
		"🏷️ Tag: %s%s\n"+
		"⚠️ Warning: This will restart the xray service and briefly interrupt your connection.\n\n"+
		"Are you sure you want to proceed?",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol, selectedServer.Tag, currentServerInfo)

	navigationHelper := NewNavigationHelper()
	confirmKeyboard := navigationHelper.CreateConfirmationKeyboard(
		fmt.Sprintf("confirm_%s", serverID),
		"refresh",
		"✅ Yes, Switch Server",
		"❌ Cancel")

	// Add test first option
	confirmKeyboard.InlineKeyboard = append(confirmKeyboard.InlineKeyboard, []models.InlineKeyboardButton{
		{Text: "📊 Test First", CallbackData: "ping_test"},
	})

	confirmContent := MessageContent{
		Text:        message,
		ReplyMarkup: confirmKeyboard,
		Type:        MessageTypeStatus,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, confirmContent); err != nil {
		tb.logger.Error("Failed to send server switch confirmation: %v", err)
	} else {
		tb.logger.Info("Successfully sent server switch confirmation to user %d", chatID)
	}
}

func (tb *TelegramBot) handleConfirmSwitchCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string, serverID string) {
	tb.logger.Info("Processing server switch confirmation for user %d, server: %s", chatID, serverID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Switching server...",
	})

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
		// Force cleanup the user's active message since we're in an error state
		tb.messageManager.ForceCleanupUser(chatID, "server not found")
		tb.sendErrorMessage(ctx, b, chatID, "Server not found", "The selected server could not be found. Please refresh the server list and try again.", "refresh")
		return
	}

	tb.logger.Debug("Starting server switch to: %s (%s:%d)", selectedServer.Name, selectedServer.Address, selectedServer.Port)

	// Step 1: Preparing configuration
	message := fmt.Sprintf("🔄 Switching to Server\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 1/4: Preparing configuration...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	step1Content := MessageContent{
		Text: message,
		Type: MessageTypeStatus,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, step1Content); err != nil {
		tb.logger.Error("Failed to send step 1 message: %v", err)
		return
	}

	time.Sleep(500 * time.Millisecond)

	// Step 2: Creating backup
	message = fmt.Sprintf("🔄 Switching to Server\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 2/4: Creating backup...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	step2Content := MessageContent{
		Text: message,
		Type: MessageTypeStatus,
	}

	_ = tb.messageManager.SendOrEdit(ctx, chatID, step2Content)

	time.Sleep(500 * time.Millisecond)

	// Step 3: Updating configuration
	message = fmt.Sprintf("🔄 Switching to Server\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 3/4: Updating configuration...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	step3Content := MessageContent{
		Text: message,
		Type: MessageTypeStatus,
	}

	_ = tb.messageManager.SendOrEdit(ctx, chatID, step3Content)

	time.Sleep(500 * time.Millisecond)

	// Step 4: Restarting xray service
	message = fmt.Sprintf("🔄 Switching to Server\n\n🏷️ Name: %s\n🌐 Address: %s:%d\n🔗 Protocol: %s\n\n⏳ Step 4/4: Restarting xray service...",
		selectedServer.Name, selectedServer.Address, selectedServer.Port, selectedServer.Protocol)

	step4Content := MessageContent{
		Text: message,
		Type: MessageTypeStatus,
	}

	_ = tb.messageManager.SendOrEdit(ctx, chatID, step4Content)

	tb.logger.Debug("Executing server switch to %s", selectedServer.Name)
	if err := tb.serverMgr.SwitchServer(serverID); err != nil {
		tb.logger.Error("Server switch failed for %s: %v", selectedServer.Name, err)
		// Force cleanup the user's active message since the operation failed
		tb.messageManager.ForceCleanupUser(chatID, "server switch failed")
		tb.sendSwitchErrorMessage(ctx, b, chatID, selectedServer, err)
		return
	}

	tb.logger.Info("Server switch successful to %s", selectedServer.Name)

	messageFormatter := NewMessageFormatter()
	message = messageFormatter.FormatServerStatusMessage(selectedServer, nil)
	message += "\n🟢 Status: Active and ready\n⚡ Service: Xray restarted successfully\n\n🎉 You are now connected to the new server!"

	navigationHelper := NewNavigationHelper()
	keyboard := navigationHelper.CreateServerStatusNavigationKeyboard(true)

	successContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeStatus,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, successContent); err != nil {
		tb.logger.Error("Failed to send server switch success message: %v", err)
	} else {
		tb.logger.Info("Successfully completed server switch to %s for user %d", selectedServer.Name, chatID)
	}
}

func (tb *TelegramBot) sendErrorMessage(ctx context.Context, _ *bot.Bot, chatID int64, title, description, retryAction string) {
	tb.logger.Debug("Sending error message to user %d: %s - %s", chatID, title, description)

	// Use MessageFormatter for consistent error formatting
	messageFormatter := NewMessageFormatter()
	suggestions := []string{
		"Try the retry button below",
		"Check your connection and try again",
		"Return to main menu if the issue persists",
	}
	message := messageFormatter.FormatErrorMessage(title, description, suggestions)

	// Use NavigationHelper for enhanced error navigation
	navigationHelper := NewNavigationHelper()
	keyboard := navigationHelper.CreateErrorNavigationKeyboard("general", retryAction)

	errorContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeStatus,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, errorContent); err != nil {
		tb.logger.Error("Failed to send error message '%s': %v", title, err)
	} else {
		tb.logger.Debug("Successfully sent error message '%s' to user %d", title, chatID)
	}
}

func (tb *TelegramBot) sendSwitchErrorMessage(ctx context.Context, _ *bot.Bot, chatID int64, server *types.Server, err error) {
	tb.logger.Error("Sending server switch error message to user %d for server %s: %v", chatID, server.Name, err)
	messageFormatter := NewMessageFormatter()
	suggestions := []string{
		"Check if the server is accessible",
		"Try a different server",
		"Refresh the server list",
		"Check your network connection",
	}
	errorMessage := messageFormatter.FormatErrorMessage("Server Switch Failed", err.Error(), suggestions)
	message := fmt.Sprintf("❌ Server Switch Failed\n\n🏷️ Server: %s\n🌐 Address: %s:%d\n\n%s",
		server.Name, server.Address, server.Port, errorMessage)

	navigationHelper := NewNavigationHelper()
	keyboard := navigationHelper.CreateErrorNavigationKeyboard("server_switch", "refresh")

	switchErrorContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeStatus,
	}

	if sendErr := tb.messageManager.SendOrEdit(ctx, chatID, switchErrorContent); sendErr != nil {
		tb.logger.Error("Failed to send server switch error message for %s: %v", server.Name, sendErr)
	} else {
		tb.logger.Info("Successfully sent server switch error message for %s to user %d", server.Name, chatID)
	}
}
func (tb *TelegramBot) handleUpdateMenuCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing update menu callback for user %d", chatID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "🔄 Update options",
	})

	// Show update menu with options
	message := "🔄 Bot Update Options\n\n" +
		"Choose an action:\n\n" +
		"📊 **Check Status**: View current update status\n" +
		"🔄 **Start Update**: Begin bot update process\n" +
		"ℹ️ **Information**: View update details\n\n" +
		"⚠️ **Note**: Updates will briefly interrupt bot service"

	navigationHelper := NewNavigationHelper()
	keyboard := navigationHelper.CreateUpdateNavigationKeyboard("status")

	updateMenuContent := MessageContent{
		Text:        message,
		ReplyMarkup: keyboard,
		Type:        MessageTypeMenu,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, updateMenuContent); err != nil {
		tb.logger.Error("Failed to send update menu: %v", err)
	} else {
		tb.logger.Info("Successfully sent update menu to user %d", chatID)
	}
}

func (tb *TelegramBot) handleStatusCallback(ctx context.Context, b *bot.Bot, chatID int64, callbackQueryID string) {
	tb.logger.Info("Processing status callback for user %d", chatID)

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            "📊 Checking status...",
	})

	// This is similar to the /status command but accessed via callback
	currentServer := tb.serverMgr.GetCurrentServer()
	if currentServer == nil {
		tb.logger.Debug("No active server found for status callback")

		messageFormatter := NewMessageFormatter()
		suggestions := []string{
			"Use server list to select a server",
			"Test server connections",
			"Refresh server configuration",
		}
		message := messageFormatter.FormatErrorMessage("No Active Server",
			"No server is currently selected or active", suggestions)

		navigationHelper := NewNavigationHelper()
		keyboard := navigationHelper.CreateErrorNavigationKeyboard("no_servers", "refresh")

		noServerContent := MessageContent{
			Text:        message,
			ReplyMarkup: keyboard,
			Type:        MessageTypeStatus,
		}

		_ = tb.messageManager.SendOrEdit(ctx, chatID, noServerContent)
		return
	}

	tb.logger.Debug("Found active server: %s (%s:%d) for status callback",
		currentServer.Name, currentServer.Address, currentServer.Port)

	messageFormatter := NewMessageFormatter()
	message := messageFormatter.FormatServerStatusMessage(currentServer, nil)

	// Show loading state first
	loadingContent := MessageContent{
		Text:        message + "\n\n🔄 Testing connection...",
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
		Type:        MessageTypeStatus,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, loadingContent); err != nil {
		tb.logger.Error("Failed to send initial status message: %v", err)
		return
	}

	tb.logger.Debug("Starting ping test for server %s", currentServer.Name)

	results, err := tb.serverMgr.TestPing()
	if err != nil {
		tb.logger.Error("Ping test failed for status callback: %v", err)

		suggestions := []string{
			"Check your internet connection",
			"Try a different server",
			"Refresh server list",
		}
		errorMessage := messageFormatter.FormatErrorMessage("Connection Test Failed", err.Error(), suggestions)

		navigationHelper := NewNavigationHelper()
		keyboard := navigationHelper.CreateErrorNavigationKeyboard("ping_test", "ping_test")

		errorContent := MessageContent{
			Text:        errorMessage,
			ReplyMarkup: keyboard,
			Type:        MessageTypeStatus,
		}

		_ = tb.messageManager.SendOrEdit(ctx, chatID, errorContent)
		return
	}

	var currentResult *types.PingResult
	for _, result := range results {
		if result.Server.ID == currentServer.ID {
			currentResult = &result
			break
		}
	}

	if currentResult == nil {
		tb.logger.Warn("Current server not found in ping results for status callback")

		updatedMessage := messageFormatter.FormatServerStatusMessage(currentServer, nil)
		updatedMessage += "\n⚠️ Warning\n" +
			"└ Server not found in available servers\n" +
			"└ Configuration may have changed"

		navigationHelper := NewNavigationHelper()
		keyboard := navigationHelper.CreateErrorNavigationKeyboard("server_load", "refresh")

		warningContent := MessageContent{
			Text:        updatedMessage,
			ReplyMarkup: keyboard,
			Type:        MessageTypeStatus,
		}

		_ = tb.messageManager.SendOrEdit(ctx, chatID, warningContent)
		return
	}

	// Show final results
	finalMessage := messageFormatter.FormatServerStatusMessage(currentServer, currentResult)

	navigationHelper := NewNavigationHelper()
	keyboard := navigationHelper.CreateServerStatusNavigationKeyboard(true)

	// Add next action suggestions
	nextActions := navigationHelper.CreateNextActionSuggestions("status_checked", currentResult.Available)
	if len(nextActions) > 0 {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, nextActions)
	}

	statusContent := MessageContent{
		Text:        finalMessage,
		ReplyMarkup: keyboard,
		Type:        MessageTypeStatus,
	}

	if err := tb.messageManager.SendOrEdit(ctx, chatID, statusContent); err != nil {
		tb.logger.Error("Failed to send final status message: %v", err)
	} else {
		tb.logger.Info("Successfully sent server status to user %d", chatID)
	}
}
