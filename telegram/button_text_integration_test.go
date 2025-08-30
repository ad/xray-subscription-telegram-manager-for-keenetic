package telegram

import (
	"testing"
	"xray-telegram-manager/types"
)

// TestButtonTextProcessingIntegration tests the integration of ButtonTextProcessor
// with keyboard creation methods using various emoji combinations and text lengths
func TestButtonTextProcessingIntegration(t *testing.T) {
	// Create a mock bot with ButtonTextProcessor
	config := &MockConfig{
		adminID:  123456789,
		botToken: "test_token",
	}

	// Create test servers with various name patterns and lengths
	testServers := []types.Server{
		{
			ID:   "1",
			Name: "Short",
		},
		{
			ID:   "2",
			Name: "Medium Length Server Name",
		},
		{
			ID:   "3",
			Name: "Very Long Server Name That Should Be Truncated Because It Exceeds The Maximum Length",
		},
		{
			ID:   "4",
			Name: "🇺🇸 US East Server",
		},
		{
			ID:   "5",
			Name: "🌟 Premium ⚡ Fast Server 🚀",
		},
		{
			ID:   "6",
			Name: "Сервер в России 🇷🇺",
		},
		{
			ID:   "7",
			Name: "🔥 Super Long Server Name With Multiple Emojis That Should Be Properly Truncated 🌐⚡🚀",
		},
		{
			ID:   "8",
			Name: "✅❌🌐📊🔄🏠⚡🔴🟢🟡", // Only emojis
		},
	}

	serverMgr := &MockServerManager{
		servers: testServers,
	}

	bot := NewMockTelegramBot(config, serverMgr)

	// Test createServerListKeyboard with various server names
	keyboard := bot.createServerListKeyboard(testServers, 0)

	if keyboard == nil {
		t.Fatal("Keyboard should not be nil")
	}

	if len(keyboard.InlineKeyboard) != len(testServers) {
		t.Errorf("Expected %d keyboard rows, got %d", len(testServers), len(keyboard.InlineKeyboard))
	}

	// Test each button text to ensure proper processing
	for i, row := range keyboard.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("Row %d should have exactly 1 button, got %d", i, len(row))
			continue
		}

		button := row[0]
		serverName := testServers[i].Name

		t.Logf("Server %d: Original='%s', Processed='%s'", i+1, serverName, button.Text)

		// Verify button text is not empty
		if button.Text == "" {
			t.Errorf("Button text for server %d should not be empty", i+1)
		}

		// Verify button text starts with status emoji
		if !contains(button.Text, "🌐") {
			t.Errorf("Button text for server %d should contain status emoji '🌐', got: '%s'", i+1, button.Text)
		}

		// Verify button text length is reasonable (not exceeding 35 characters for this test)
		if len([]rune(button.Text)) > 35 {
			t.Errorf("Button text for server %d is too long (%d runes): '%s'", i+1, len([]rune(button.Text)), button.Text)
		}

		// Verify callback data is correct
		expectedCallback := "server_" + testServers[i].ID
		if button.CallbackData != expectedCallback {
			t.Errorf("Button callback for server %d should be '%s', got '%s'", i+1, expectedCallback, button.CallbackData)
		}

		// Test specific cases
		switch i {
		case 0: // Short name
			if button.Text != "🌐 Short" {
				t.Errorf("Short server name should be '🌐 Short', got '%s'", button.Text)
			}
		case 2: // Very long name - should be truncated
			if !contains(button.Text, "...") {
				t.Errorf("Very long server name should be truncated with '...', got '%s'", button.Text)
			}
		case 3: // Name with flag emoji
			if !contains(button.Text, "🇺🇸") {
				t.Errorf("Server name with flag emoji should preserve the flag, got '%s'", button.Text)
			}
		case 4: // Multiple emojis
			// Should preserve some emojis and handle truncation properly
			if !contains(button.Text, "🌟") {
				t.Errorf("Server name with multiple emojis should preserve at least the first emoji, got '%s'", button.Text)
			}
		case 7: // Only emojis
			// Should handle emoji-only names properly
			if len(button.Text) < 5 { // At least status emoji + space + some content
				t.Errorf("Emoji-only server name should be processed properly, got '%s'", button.Text)
			}
		}
	}
}

// TestButtonTextProcessorWithDifferentLengths tests the processor with various length limits
func TestButtonTextProcessorWithDifferentLengths(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	testCases := []struct {
		name      string
		input     string
		maxLength int
		shouldFit bool
	}{
		{
			name:      "short text with small limit",
			input:     "Test",
			maxLength: 10,
			shouldFit: true,
		},
		{
			name:      "emoji text with small limit",
			input:     "✅ Test Server",
			maxLength: 15,
			shouldFit: true,
		},
		{
			name:      "long text with small limit",
			input:     "Very Long Server Name That Exceeds Limit",
			maxLength: 20,
			shouldFit: false,
		},
		{
			name:      "multiple emojis with medium limit",
			input:     "🌟 ⚡ 🚀 Premium Server",
			maxLength: 25,
			shouldFit: true,
		},
		{
			name:      "unicode with emojis",
			input:     "🇷🇺 Российский сервер",
			maxLength: 30,
			shouldFit: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := processor.ProcessButtonText(tc.input, tc.maxLength)

			// Calculate display length of result
			resultLength := processor.CalculateTextLength(result)

			if resultLength > tc.maxLength {
				t.Errorf("Processed text length (%d) exceeds max length (%d): '%s'",
					resultLength, tc.maxLength, result)
			}

			if tc.shouldFit {
				// If it should fit, it shouldn't be truncated
				if contains(result, "...") && processor.CalculateTextLength(tc.input) <= tc.maxLength {
					t.Errorf("Text that should fit was truncated: input='%s', result='%s'",
						tc.input, result)
				}
			} else {
				// If it shouldn't fit, it should be truncated
				if !contains(result, "...") && processor.CalculateTextLength(tc.input) > tc.maxLength {
					t.Errorf("Text that should be truncated was not: input='%s', result='%s'",
						tc.input, result)
				}
			}

			t.Logf("Input: '%s' (len=%d) -> Output: '%s' (len=%d)",
				tc.input, processor.CalculateTextLength(tc.input),
				result, resultLength)
		})
	}
}

// TestEmojiPreservation tests that emojis are properly preserved during truncation
func TestEmojiPreservation(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	testCases := []struct {
		name             string
		input            string
		maxLength        int
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:             "preserve leading emoji",
			input:            "✅ This is a long server name that needs truncation",
			maxLength:        20,
			shouldContain:    []string{"✅"},
			shouldNotContain: []string{},
		},
		{
			name:             "preserve multiple leading emojis",
			input:            "🌟 ⚡ Server Name That Is Too Long",
			maxLength:        15,
			shouldContain:    []string{"🌟", "⚡"},
			shouldNotContain: []string{},
		},
		{
			name:             "truncate but preserve important emojis",
			input:            "🇺🇸 United States East Coast Server",
			maxLength:        20,
			shouldContain:    []string{"🇺🇸"},
			shouldNotContain: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := processor.ProcessButtonText(tc.input, tc.maxLength)

			for _, emoji := range tc.shouldContain {
				if !contains(result, emoji) {
					t.Errorf("Result should contain emoji '%s': input='%s', result='%s'",
						emoji, tc.input, result)
				}
			}

			for _, text := range tc.shouldNotContain {
				if contains(result, text) {
					t.Errorf("Result should not contain '%s': input='%s', result='%s'",
						text, tc.input, result)
				}
			}

			// Verify length constraint
			if processor.CalculateTextLength(result) > tc.maxLength {
				t.Errorf("Result exceeds max length: input='%s', result='%s', length=%d, max=%d",
					tc.input, result, processor.CalculateTextLength(result), tc.maxLength)
			}

			t.Logf("Input: '%s' -> Output: '%s'", tc.input, result)
		})
	}
}
