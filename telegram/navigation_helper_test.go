package telegram

import (
	"testing"
)

func TestNavigationHelper_CreateMainMenuKeyboard(t *testing.T) {
	nh := NewNavigationHelper()
	keyboard := nh.CreateMainMenuKeyboard()

	if keyboard == nil {
		t.Fatal("Expected keyboard to be created, got nil")
	}

	if len(keyboard.InlineKeyboard) == 0 {
		t.Fatal("Expected keyboard to have buttons, got empty")
	}

	// Check that main menu has the expected buttons
	firstRow := keyboard.InlineKeyboard[0]
	if len(firstRow) < 2 {
		t.Fatal("Expected at least 2 buttons in first row")
	}

	// Check for Server List button
	found := false
	for _, button := range firstRow {
		if button.Text == "📋 Server List" && button.CallbackData == "refresh" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'Server List' button with 'refresh' callback")
	}

	// Check for Ping Test button
	found = false
	for _, button := range firstRow {
		if button.Text == "📊 Ping Test" && button.CallbackData == "ping_test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'Ping Test' button with 'ping_test' callback")
	}
}

func TestNavigationHelper_CreateErrorNavigationKeyboard(t *testing.T) {
	nh := NewNavigationHelper()
	keyboard := nh.CreateErrorNavigationKeyboard("ping_test", "ping_test")

	if keyboard == nil {
		t.Fatal("Expected keyboard to be created, got nil")
	}

	if len(keyboard.InlineKeyboard) == 0 {
		t.Fatal("Expected keyboard to have buttons, got empty")
	}

	// Check that error navigation has retry button
	hasRetryButton := false
	hasMainMenuButton := false

	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if button.Text == "🔄 Retry Test" && button.CallbackData == "ping_test" {
				hasRetryButton = true
			}
			if button.Text == "🏠 Main Menu" && button.CallbackData == "main_menu" {
				hasMainMenuButton = true
			}
		}
	}

	if !hasRetryButton {
		t.Error("Expected to find retry button for ping test error")
	}

	if !hasMainMenuButton {
		t.Error("Expected to find main menu button")
	}
}

func TestNavigationHelper_CreateBreadcrumbNavigation(t *testing.T) {
	nh := NewNavigationHelper()

	tests := []struct {
		page     string
		expected int
	}{
		{"server_list", 2},
		{"ping_test", 2},
		{"server_status", 3},
		{"update", 2},
		{"unknown", 0},
	}

	for _, test := range tests {
		breadcrumbs := nh.CreateBreadcrumbNavigation(test.page)
		if len(breadcrumbs) != test.expected {
			t.Errorf("For page '%s', expected %d breadcrumbs, got %d",
				test.page, test.expected, len(breadcrumbs))
		}
	}
}

func TestNavigationHelper_CreateContextualBackButton(t *testing.T) {
	nh := NewNavigationHelper()

	tests := []struct {
		context      string
		expectedText string
		expectedData string
	}{
		{"server_selection", "⬅️ Back to List", "refresh"},
		{"ping_results", "⬅️ Back to Servers", "refresh"},
		{"server_status", "⬅️ Back to List", "refresh"},
		{"update_process", "⬅️ Back to Main", "main_menu"},
		{"error_recovery", "⬅️ Go Back", "main_menu"},
		{"unknown", "🏠 Main Menu", "main_menu"},
	}

	for _, test := range tests {
		button := nh.CreateContextualBackButton(test.context)
		if button.Text != test.expectedText {
			t.Errorf("For context '%s', expected text '%s', got '%s'",
				test.context, test.expectedText, button.Text)
		}
		if button.CallbackData != test.expectedData {
			t.Errorf("For context '%s', expected callback '%s', got '%s'",
				test.context, test.expectedData, button.CallbackData)
		}
	}
}

func TestNavigationHelper_CreateNextActionSuggestions(t *testing.T) {
	nh := NewNavigationHelper()

	// Test with next actions enabled
	suggestions := nh.CreateNextActionSuggestions("server_list_loaded", true)
	if len(suggestions) == 0 {
		t.Error("Expected next action suggestions for server_list_loaded, got none")
	}

	// Test with next actions disabled
	nh.enableNextActions = false
	suggestions = nh.CreateNextActionSuggestions("server_list_loaded", true)
	if len(suggestions) != 0 {
		t.Error("Expected no next action suggestions when disabled, got some")
	}
}

func TestNavigationHelper_CreateRetryButton(t *testing.T) {
	nh := NewNavigationHelper()

	tests := []struct {
		context      string
		action       string
		expectedText string
	}{
		{"server_load_failed", "refresh", "🔄 Retry Loading"},
		{"ping_test_failed", "ping_test", "🔄 Retry Test"},
		{"server_switch_failed", "confirm_switch", "🔄 Try Switch Again"},
		{"update_failed", "confirm_update", "🔄 Retry Update"},
		{"connection_failed", "ping_test", "🔄 Retry Connection"},
		{"unknown", "action", "🔄 Try Again"},
	}

	for _, test := range tests {
		button := nh.CreateRetryButton(test.context, test.action)
		if button.Text != test.expectedText {
			t.Errorf("For context '%s', expected text '%s', got '%s'",
				test.context, test.expectedText, button.Text)
		}
		if button.CallbackData != test.action {
			t.Errorf("For context '%s', expected callback '%s', got '%s'",
				test.context, test.action, button.CallbackData)
		}
	}
}

func TestNavigationHelper_CreateQuickSelectKeyboard(t *testing.T) {
	nh := NewNavigationHelper()

	servers := []QuickSelectServer{
		{ID: "server1", ButtonText: "Server 1 (50ms)"},
		{ID: "server2", ButtonText: "Server 2 (75ms)"},
	}

	keyboard := nh.CreateQuickSelectKeyboard(servers)

	if len(keyboard) == 0 {
		t.Fatal("Expected quick select keyboard to have rows, got none")
	}

	// Should have header row + server rows + separator
	expectedRows := 1 + len(servers) + 1 // header + servers + separator
	if len(keyboard) != expectedRows {
		t.Errorf("Expected %d rows, got %d", expectedRows, len(keyboard))
	}

	// Check header row
	headerRow := keyboard[0]
	if len(headerRow) != 1 || headerRow[0].Text != "⚡ Quick Select:" {
		t.Error("Expected header row with 'Quick Select:' text")
	}

	// Check server rows
	for i, server := range servers {
		serverRow := keyboard[i+1] // +1 to skip header
		if len(serverRow) != 1 {
			t.Errorf("Expected server row %d to have 1 button, got %d", i, len(serverRow))
			continue
		}

		button := serverRow[0]
		if button.Text != server.ButtonText {
			t.Errorf("Expected button text '%s', got '%s'", server.ButtonText, button.Text)
		}

		expectedCallback := "server_" + server.ID
		if button.CallbackData != expectedCallback {
			t.Errorf("Expected callback '%s', got '%s'", expectedCallback, button.CallbackData)
		}
	}
}

func TestNavigationHelper_CreateConfirmationKeyboard(t *testing.T) {
	nh := NewNavigationHelper()

	keyboard := nh.CreateConfirmationKeyboard("confirm_action", "cancel_action", "✅ Confirm", "❌ Cancel")

	if keyboard == nil {
		t.Fatal("Expected keyboard to be created, got nil")
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(keyboard.InlineKeyboard))
	}

	// Check confirm button
	confirmRow := keyboard.InlineKeyboard[0]
	if len(confirmRow) != 1 {
		t.Fatal("Expected 1 button in confirm row")
	}

	confirmButton := confirmRow[0]
	if confirmButton.Text != "✅ Confirm" || confirmButton.CallbackData != "confirm_action" {
		t.Error("Confirm button has incorrect text or callback")
	}

	// Check cancel button
	cancelRow := keyboard.InlineKeyboard[1]
	if len(cancelRow) != 1 {
		t.Fatal("Expected 1 button in cancel row")
	}

	cancelButton := cancelRow[0]
	if cancelButton.Text != "❌ Cancel" || cancelButton.CallbackData != "cancel_action" {
		t.Error("Cancel button has incorrect text or callback")
	}
}

func TestNavigationHelper_CreateLoadingKeyboard(t *testing.T) {
	nh := NewNavigationHelper()

	keyboard := nh.CreateLoadingKeyboard()

	if keyboard == nil {
		t.Fatal("Expected keyboard to be created, got nil")
	}

	// Should have main menu button when back buttons are enabled
	hasMainMenuButton := false
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if button.Text == "🏠 Main Menu" && button.CallbackData == "main_menu" {
				hasMainMenuButton = true
			}
		}
	}

	if !hasMainMenuButton {
		t.Error("Expected loading keyboard to have main menu button")
	}

	// Test with back buttons disabled
	nh.enableBackButtons = false
	keyboard = nh.CreateLoadingKeyboard()

	if len(keyboard.InlineKeyboard) != 0 {
		t.Error("Expected no buttons when back buttons are disabled")
	}
}
