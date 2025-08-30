package telegram

import (
	"github.com/go-telegram/bot/models"
)

// NavigationHelper provides consistent navigation patterns and button layouts
type NavigationHelper struct {
	// Configuration for navigation
	enableBackButtons  bool
	enableRetryButtons bool
	enableBreadcrumbs  bool
	enableNextActions  bool
}

// NewNavigationHelper creates a new navigation helper with default settings
func NewNavigationHelper() *NavigationHelper {
	return &NavigationHelper{
		enableBackButtons:  true,
		enableRetryButtons: true,
		enableBreadcrumbs:  true,
		enableNextActions:  true,
	}
}

// CreateMainMenuKeyboard creates the main menu keyboard with consistent styling
func (nh *NavigationHelper) CreateMainMenuKeyboard() *models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton

	// Primary actions
	keyboard = append(keyboard, []models.InlineKeyboardButton{
		{Text: "📋 Server List", CallbackData: "refresh"},
		{Text: "📊 Ping Test", CallbackData: "ping_test"},
	})

	// Additional helpful actions if enabled
	if nh.enableNextActions {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📊 Server Status", CallbackData: "status"},
			{Text: "🔄 Update Bot", CallbackData: "update_menu"},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// CreateServerListNavigationKeyboard creates navigation for server list with pagination
func (nh *NavigationHelper) CreateServerListNavigationKeyboard(page, totalPages int) [][]models.InlineKeyboardButton {
	var keyboard [][]models.InlineKeyboardButton

	// Pagination row if needed
	if totalPages > 1 {
		var paginationRow []models.InlineKeyboardButton

		if page > 0 {
			paginationRow = append(paginationRow, models.InlineKeyboardButton{
				Text: "⬅️ Previous", CallbackData: "page_" + string(rune(page-1+'0')),
			})
		}

		paginationRow = append(paginationRow, models.InlineKeyboardButton{
			Text:         "📄 " + string(rune(page+1+'0')) + "/" + string(rune(totalPages+'0')),
			CallbackData: "noop",
		})

		if page < totalPages-1 {
			paginationRow = append(paginationRow, models.InlineKeyboardButton{
				Text: "Next ➡️", CallbackData: "page_" + string(rune(page+1+'0')),
			})
		}

		keyboard = append(keyboard, paginationRow)
	}

	// Primary action buttons
	keyboard = append(keyboard, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh List", CallbackData: "refresh"},
		{Text: "📊 Test Servers", CallbackData: "ping_test"},
	})

	// Next logical actions if enabled
	if nh.enableNextActions {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📊 Current Status", CallbackData: "status"},
		})
	}

	// Navigation breadcrumb if enabled
	if nh.enableBreadcrumbs {
		breadcrumbs := nh.CreateBreadcrumbNavigation("server_list")
		if len(breadcrumbs) > 0 {
			keyboard = append(keyboard, breadcrumbs)
		}
	} else if nh.enableBackButtons {
		// Fallback to simple back button
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})
	}

	return keyboard
}

// CreatePingTestNavigationKeyboard creates navigation for ping test results
func (nh *NavigationHelper) CreatePingTestNavigationKeyboard(hasResults bool) *models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton

	if hasResults {
		// Primary actions for successful results
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📋 View All Servers", CallbackData: "refresh"},
			{Text: "🔄 Test Again", CallbackData: "ping_test"},
		})

		// Next logical actions if enabled
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "� Current Status", CallbackData: "status"},
			})
		}
	} else {
		// Retry and alternative actions for failed results
		if nh.enableRetryButtons {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "🔄 Retry Test", CallbackData: "ping_test"},
			})
		}

		// Alternative actions
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📋 Server List", CallbackData: "refresh"},
		})

		// Helpful next actions for troubleshooting
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "📊 Check Status", CallbackData: "status"},
				{Text: "🔄 Refresh Servers", CallbackData: "refresh"},
			})
		}
	}

	// Navigation breadcrumb if enabled
	if nh.enableBreadcrumbs {
		breadcrumbs := nh.CreateBreadcrumbNavigation("ping_test")
		if len(breadcrumbs) > 0 {
			keyboard = append(keyboard, breadcrumbs)
		}
	} else if nh.enableBackButtons {
		// Fallback to simple back button
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// CreateServerStatusNavigationKeyboard creates navigation for server status display
func (nh *NavigationHelper) CreateServerStatusNavigationKeyboard(isCurrentServer bool) *models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton

	if isCurrentServer {
		// Actions for current server
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📊 Test Connection", CallbackData: "ping_test"},
			{Text: "📋 Switch Server", CallbackData: "refresh"},
		})

		// Next logical actions for current server
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "� Refresh Status", CallbackData: "status"},
			})
		}
	} else {
		// Actions for non-current server
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "✅ Select Server", CallbackData: "confirm_switch"},
			{Text: "📊 Test Connection", CallbackData: "ping_test"},
		})

		// Next logical actions for non-current server
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "📋 Compare Servers", CallbackData: "ping_test"},
			})
		}

		// Back to server list
		if nh.enableBackButtons {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "⬅️ Back to List", CallbackData: "refresh"},
			})
		}
	}

	// Navigation breadcrumb if enabled
	if nh.enableBreadcrumbs {
		breadcrumbs := nh.CreateBreadcrumbNavigation("server_status")
		if len(breadcrumbs) > 0 {
			keyboard = append(keyboard, breadcrumbs)
		}
	} else if nh.enableBackButtons {
		// Fallback to simple back button
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// CreateErrorNavigationKeyboard creates navigation for error messages
func (nh *NavigationHelper) CreateErrorNavigationKeyboard(errorType string, retryAction string) *models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton

	// Primary retry button if enabled and action provided
	if nh.enableRetryButtons && retryAction != "" {
		var retryText string
		switch errorType {
		case "server_load":
			retryText = "🔄 Retry Loading"
		case "ping_test":
			retryText = "🔄 Retry Test"
		case "server_switch":
			retryText = "🔄 Try Again"
		case "update":
			retryText = "🔄 Retry Update"
		case "general":
			retryText = "🔄 Try Again"
		default:
			retryText = "🔄 Retry"
		}

		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: retryText, CallbackData: retryAction},
		})
	}

	// Alternative actions based on error type
	switch errorType {
	case "server_load", "no_servers":
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🔄 Refresh", CallbackData: "refresh"},
		})
		// Next logical actions for server loading errors
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "📊 Check Status", CallbackData: "status"},
			})
		}

	case "ping_test":
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📋 Server List", CallbackData: "refresh"},
		})
		// Next logical actions for ping test errors
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "📊 Check Status", CallbackData: "status"},
				{Text: "🔄 Refresh Servers", CallbackData: "refresh"},
			})
		}

	case "server_switch":
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📋 Choose Different", CallbackData: "refresh"},
		})
		// Next logical actions for server switch errors
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "📊 Test Servers", CallbackData: "ping_test"},
				{Text: "📊 Current Status", CallbackData: "status"},
			})
		}

	case "update":
		// Next logical actions for update errors
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "ℹ️ Check Status", CallbackData: "update_status"},
				{Text: "📊 Test Bot", CallbackData: "ping_test"},
			})
		}

	case "general":
		// General error recovery options
		if nh.enableNextActions {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "📋 Server List", CallbackData: "refresh"},
				{Text: "📊 Test Connection", CallbackData: "ping_test"},
			})
		}
	}

	// Back navigation
	if nh.enableBackButtons {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// CreateUpdateNavigationKeyboard creates navigation for update-related messages
func (nh *NavigationHelper) CreateUpdateNavigationKeyboard(updateState string) *models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton

	switch updateState {
	case "confirmation":
		// Update confirmation
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "✅ Yes, Update Bot", CallbackData: "confirm_update"},
		})
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "❌ Cancel", CallbackData: "main_menu"},
			{Text: "ℹ️ Check Status", CallbackData: "update_status"},
		})

	case "in_progress":
		// Update in progress
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🔄 Refresh Status", CallbackData: "update_status"},
		})
		if nh.enableBackButtons {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "🏠 Main Menu", CallbackData: "main_menu"},
			})
		}

	case "completed":
		// Update completed successfully
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "📋 Server List", CallbackData: "refresh"},
			{Text: "📊 Test Servers", CallbackData: "ping_test"},
		})
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})

	case "failed":
		// Update failed
		if nh.enableRetryButtons {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: "🔄 Try Again", CallbackData: "confirm_update"},
				{Text: "ℹ️ Check Status", CallbackData: "update_status"},
			})
		}
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})

	case "status":
		// Update status check
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🔄 Start Update", CallbackData: "confirm_update"},
		})
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})

	default:
		// Default update navigation
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// CreateQuickSelectKeyboard creates keyboard for quick server selection
func (nh *NavigationHelper) CreateQuickSelectKeyboard(servers []QuickSelectServer) [][]models.InlineKeyboardButton {
	var keyboard [][]models.InlineKeyboardButton

	if len(servers) > 0 {
		// Header row
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "⚡ Quick Select:", CallbackData: "noop"},
		})

		// Server buttons (each on its own row for better readability)
		for _, server := range servers {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{
					Text:         server.ButtonText,
					CallbackData: "server_" + server.ID,
				},
			})
		}

		// Separator
		keyboard = append(keyboard, []models.InlineKeyboardButton{})
	}

	return keyboard
}

// CreateConfirmationKeyboard creates a confirmation dialog keyboard
func (nh *NavigationHelper) CreateConfirmationKeyboard(confirmAction, cancelAction string, confirmText, cancelText string) *models.InlineKeyboardMarkup {
	if confirmText == "" {
		confirmText = "✅ Confirm"
	}
	if cancelText == "" {
		cancelText = "❌ Cancel"
	}
	if cancelAction == "" {
		cancelAction = "main_menu"
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: confirmText, CallbackData: confirmAction},
			},
			{
				{Text: cancelText, CallbackData: cancelAction},
			},
		},
	}
}

// CreateLoadingKeyboard creates a minimal keyboard for loading states
func (nh *NavigationHelper) CreateLoadingKeyboard() *models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton

	// Only show main menu during loading to avoid confusion
	if nh.enableBackButtons {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "🏠 Main Menu", CallbackData: "main_menu"},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

// Helper types for quick select
type QuickSelectServer struct {
	ID         string
	ButtonText string
}

// CreateBreadcrumbNavigation creates breadcrumb-style navigation
func (nh *NavigationHelper) CreateBreadcrumbNavigation(currentPage string) []models.InlineKeyboardButton {
	var breadcrumbs []models.InlineKeyboardButton

	switch currentPage {
	case "server_list":
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "🏠 Main", CallbackData: "main_menu",
		})
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "📋 Servers", CallbackData: "noop",
		})

	case "ping_test":
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "🏠 Main", CallbackData: "main_menu",
		})
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "📊 Ping Test", CallbackData: "noop",
		})

	case "server_status":
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "🏠 Main", CallbackData: "main_menu",
		})
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "📋 Servers", CallbackData: "refresh",
		})
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "📊 Status", CallbackData: "noop",
		})

	case "update":
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "🏠 Main", CallbackData: "main_menu",
		})
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "🔄 Update", CallbackData: "noop",
		})

	case "confirmation":
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "🏠 Main", CallbackData: "main_menu",
		})
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "⬅️ Back", CallbackData: "refresh",
		})

	case "error":
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "🏠 Main", CallbackData: "main_menu",
		})
		breadcrumbs = append(breadcrumbs, models.InlineKeyboardButton{
			Text: "⬅️ Back", CallbackData: "refresh",
		})
	}

	return breadcrumbs
}

// CreateContextualBackButton creates a context-aware back button
func (nh *NavigationHelper) CreateContextualBackButton(context string) models.InlineKeyboardButton {
	switch context {
	case "server_selection":
		return models.InlineKeyboardButton{
			Text: "⬅️ Back to List", CallbackData: "refresh",
		}
	case "ping_results":
		return models.InlineKeyboardButton{
			Text: "⬅️ Back to Servers", CallbackData: "refresh",
		}
	case "server_status":
		return models.InlineKeyboardButton{
			Text: "⬅️ Back to List", CallbackData: "refresh",
		}
	case "update_process":
		return models.InlineKeyboardButton{
			Text: "⬅️ Back to Main", CallbackData: "main_menu",
		}
	case "error_recovery":
		return models.InlineKeyboardButton{
			Text: "⬅️ Go Back", CallbackData: "main_menu",
		}
	default:
		return models.InlineKeyboardButton{
			Text: "🏠 Main Menu", CallbackData: "main_menu",
		}
	}
}

// CreateNextActionSuggestions creates logical next action buttons based on current context
func (nh *NavigationHelper) CreateNextActionSuggestions(context string, hasResults bool) []models.InlineKeyboardButton {
	var suggestions []models.InlineKeyboardButton

	if !nh.enableNextActions {
		return suggestions
	}

	switch context {
	case "server_list_loaded":
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📊 Test All Servers", CallbackData: "ping_test",
		})
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📊 Check Current", CallbackData: "status",
		})

	case "ping_test_completed":
		if hasResults {
			suggestions = append(suggestions, models.InlineKeyboardButton{
				Text: "📋 Browse All", CallbackData: "refresh",
			})
			suggestions = append(suggestions, models.InlineKeyboardButton{
				Text: "📊 Check Status", CallbackData: "status",
			})
		} else {
			suggestions = append(suggestions, models.InlineKeyboardButton{
				Text: "🔄 Refresh Servers", CallbackData: "refresh",
			})
			suggestions = append(suggestions, models.InlineKeyboardButton{
				Text: "📊 Check Current", CallbackData: "status",
			})
		}

	case "server_switched":
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📊 Test New Server", CallbackData: "ping_test",
		})
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📊 Check Status", CallbackData: "status",
		})

	case "status_checked":
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📊 Test Connection", CallbackData: "ping_test",
		})
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📋 Switch Server", CallbackData: "refresh",
		})

	case "update_completed":
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📊 Test Bot", CallbackData: "ping_test",
		})
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📋 Check Servers", CallbackData: "refresh",
		})

	case "error_occurred":
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "📊 Check Status", CallbackData: "status",
		})
		suggestions = append(suggestions, models.InlineKeyboardButton{
			Text: "🔄 Refresh", CallbackData: "refresh",
		})
	}

	return suggestions
}

// CreateRetryButton creates a context-aware retry button
func (nh *NavigationHelper) CreateRetryButton(context string, action string) models.InlineKeyboardButton {
	if !nh.enableRetryButtons {
		return models.InlineKeyboardButton{
			Text: "🔄 Try Again", CallbackData: action,
		}
	}

	switch context {
	case "server_load_failed":
		return models.InlineKeyboardButton{
			Text: "🔄 Retry Loading", CallbackData: action,
		}
	case "ping_test_failed":
		return models.InlineKeyboardButton{
			Text: "🔄 Retry Test", CallbackData: action,
		}
	case "server_switch_failed":
		return models.InlineKeyboardButton{
			Text: "🔄 Try Switch Again", CallbackData: action,
		}
	case "update_failed":
		return models.InlineKeyboardButton{
			Text: "🔄 Retry Update", CallbackData: action,
		}
	case "connection_failed":
		return models.InlineKeyboardButton{
			Text: "🔄 Retry Connection", CallbackData: action,
		}
	default:
		return models.InlineKeyboardButton{
			Text: "🔄 Try Again", CallbackData: action,
		}
	}
}
