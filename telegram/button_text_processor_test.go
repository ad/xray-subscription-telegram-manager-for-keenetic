package telegram

import (
	"testing"
)

func TestNewButtonTextProcessor(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	if processor.maxLength != 50 {
		t.Errorf("Expected maxLength to be 50, got %d", processor.maxLength)
	}

	if processor.emojiMap == nil {
		t.Error("Expected emojiMap to be initialized")
	}

	// Test that common emojis are initialized
	if width := processor.GetEmojiDisplayWidth("✅"); width != 2 {
		t.Errorf("Expected ✅ to have width 2, got %d", width)
	}
}

func TestCalculateTextLength(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "simple text",
			input:    "Hello",
			expected: 5,
		},
		{
			name:     "text with single emoji",
			input:    "Hello ✅",
			expected: 8, // 6 chars + 2 for emoji
		},
		{
			name:     "text with multiple emojis",
			input:    "✅ Server 🌐",
			expected: 12, // 2 + 8 + 2 = 12
		},
		{
			name:     "only emojis",
			input:    "✅❌🌐",
			expected: 6, // 3 emojis * 2 each
		},
		{
			name:     "complex server name",
			input:    "🌐 US-East-1 ✅",
			expected: 15, // 2 + 1 + 9 + 1 + 2 = 15
		},
		{
			name:     "unicode text",
			input:    "Сервер-1",
			expected: 8,
		},
		{
			name:     "mixed unicode and emoji",
			input:    "Сервер ✅ Test",
			expected: 14, // 6 + 1 + 2 + 1 + 4 = 14
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.CalculateTextLength(tt.input)
			if result != tt.expected {
				t.Errorf("CalculateTextLength(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateWithEmoji(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{
			name:      "no truncation needed",
			input:     "Short",
			maxLength: 10,
			expected:  "Short",
		},
		{
			name:      "simple truncation",
			input:     "This is a very long text that needs truncation",
			maxLength: 20,
			expected:  "This is a very lo...",
		},
		{
			name:      "truncation with emoji at end",
			input:     "Server Name ✅",
			maxLength: 10,
			expected:  "Server ...",
		},
		{
			name:      "truncation preserving emoji",
			input:     "✅ Server Name",
			maxLength: 10,
			expected:  "✅ Serv...",
		},
		{
			name:      "multiple emojis",
			input:     "✅ 🌐 Server Name",
			maxLength: 12,
			expected:  "✅ 🌐 Ser...",
		},
		{
			name:      "only emoji fits",
			input:     "✅ Very Long Server Name",
			maxLength: 5,
			expected:  "✅...",
		},
		{
			name:      "empty string",
			input:     "",
			maxLength: 10,
			expected:  "",
		},
		{
			name:      "zero max length",
			input:     "Test",
			maxLength: 0,
			expected:  "",
		},
		{
			name:      "max length too small for ellipsis",
			input:     "Test",
			maxLength: 2,
			expected:  "...",
		},
		{
			name:      "exact fit with emoji",
			input:     "✅ Test",
			maxLength: 7,
			expected:  "✅ Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.TruncateWithEmoji(tt.input, tt.maxLength)
			if result != tt.expected {
				t.Errorf("TruncateWithEmoji(%q, %d) = %q, expected %q", tt.input, tt.maxLength, result, tt.expected)
			}
		})
	}
}

func TestProcessButtonText(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{
			name:      "short text no processing",
			input:     "Short",
			maxLength: 20,
			expected:  "Short",
		},
		{
			name:      "long text gets truncated",
			input:     "This is a very long server name that exceeds the limit",
			maxLength: 20,
			expected:  "This is a very lo...",
		},
		{
			name:      "text with emoji fits",
			input:     "✅ Server",
			maxLength: 15,
			expected:  "✅ Server",
		},
		{
			name:      "text with emoji needs truncation",
			input:     "✅ Very Long Server Name That Exceeds Limit",
			maxLength: 20,
			expected:  "✅ Very Long Serv...",
		},
		{
			name:      "use default max length",
			input:     "Test",
			maxLength: 0,
			expected:  "Test",
		},
		{
			name:      "empty input",
			input:     "",
			maxLength: 20,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ProcessButtonText(tt.input, tt.maxLength)
			if result != tt.expected {
				t.Errorf("ProcessButtonText(%q, %d) = %q, expected %q", tt.input, tt.maxLength, result, tt.expected)
			}
		})
	}
}

func TestProcessServerButtonText(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	tests := []struct {
		name        string
		serverName  string
		statusEmoji string
		maxLength   int
		expected    string
	}{
		{
			name:        "normal server with current status",
			serverName:  "US-East-1",
			statusEmoji: "✅",
			maxLength:   20,
			expected:    "✅ US-East-1",
		},
		{
			name:        "normal server with available status",
			serverName:  "EU-West-1",
			statusEmoji: "🌐",
			maxLength:   20,
			expected:    "🌐 EU-West-1",
		},
		{
			name:        "long server name gets truncated",
			serverName:  "Very-Long-Server-Name-That-Exceeds-The-Limit",
			statusEmoji: "✅",
			maxLength:   20,
			expected:    "✅ Very-Long-Serv...",
		},
		{
			name:        "server name with emoji",
			serverName:  "🇺🇸 US Server",
			statusEmoji: "✅",
			maxLength:   20,
			expected:    "✅ 🇺🇸 US Server",
		},
		{
			name:        "empty server name",
			serverName:  "",
			statusEmoji: "✅",
			maxLength:   20,
			expected:    "✅",
		},
		{
			name:        "max length too small",
			serverName:  "Server",
			statusEmoji: "✅",
			maxLength:   2,
			expected:    "✅",
		},
		{
			name:        "exact fit",
			serverName:  "Test",
			statusEmoji: "✅",
			maxLength:   7,
			expected:    "✅ Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ProcessServerButtonText(tt.serverName, tt.statusEmoji, tt.maxLength)
			if result != tt.expected {
				t.Errorf("ProcessServerButtonText(%q, %q, %d) = %q, expected %q",
					tt.serverName, tt.statusEmoji, tt.maxLength, result, tt.expected)
			}
		})
	}
}

func TestIsEmojiRune(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	tests := []struct {
		name     string
		input    rune
		expected bool
	}{
		{
			name:     "check mark emoji",
			input:    '✅',
			expected: true,
		},
		{
			name:     "cross mark emoji",
			input:    '❌',
			expected: true,
		},
		{
			name:     "globe emoji",
			input:    '🌐',
			expected: true,
		},
		{
			name:     "regular letter",
			input:    'A',
			expected: false,
		},
		{
			name:     "number",
			input:    '1',
			expected: false,
		},
		{
			name:     "space",
			input:    ' ',
			expected: false,
		},
		{
			name:     "unicode letter",
			input:    'А', // Cyrillic A
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isEmojiRune(tt.input)
			if result != tt.expected {
				t.Errorf("isEmojiRune(%c) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetEmojiLength(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	tests := []struct {
		name       string
		input      string
		startIndex int
		expected   int
	}{
		{
			name:       "simple emoji",
			input:      "✅",
			startIndex: 0,
			expected:   1,
		},
		{
			name:       "emoji in text",
			input:      "Test ✅ More",
			startIndex: 5,
			expected:   1,
		},
		{
			name:       "non-emoji character",
			input:      "Test",
			startIndex: 0,
			expected:   0,
		},
		{
			name:       "out of bounds",
			input:      "Test",
			startIndex: 10,
			expected:   0,
		},
		{
			name:       "complex emoji",
			input:      "🌐",
			startIndex: 0,
			expected:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.input)
			result := processor.getEmojiLength(runes, tt.startIndex)
			if result != tt.expected {
				t.Errorf("getEmojiLength(%q, %d) = %d, expected %d", tt.input, tt.startIndex, result, tt.expected)
			}
		})
	}
}

func TestGetEmojiDisplayWidth(t *testing.T) {
	processor := NewButtonTextProcessor(50)

	tests := []struct {
		name     string
		emoji    string
		expected int
	}{
		{
			name:     "known emoji",
			emoji:    "✅",
			expected: 2,
		},
		{
			name:     "another known emoji",
			emoji:    "🌐",
			expected: 2,
		},
		{
			name:     "unknown emoji",
			emoji:    "🦄", // Not in our map
			expected: 2,   // Default width
		},
		{
			name:     "empty string",
			emoji:    "",
			expected: 2, // Default width
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.GetEmojiDisplayWidth(tt.emoji)
			if result != tt.expected {
				t.Errorf("GetEmojiDisplayWidth(%q) = %d, expected %d", tt.emoji, result, tt.expected)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkCalculateTextLength(b *testing.B) {
	processor := NewButtonTextProcessor(50)
	text := "✅ This is a test server name with emoji 🌐"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.CalculateTextLength(text)
	}
}

func BenchmarkProcessButtonText(b *testing.B) {
	processor := NewButtonTextProcessor(50)
	text := "✅ This is a very long server name that will need truncation 🌐"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.ProcessButtonText(text, 20)
	}
}

func BenchmarkProcessServerButtonText(b *testing.B) {
	processor := NewButtonTextProcessor(50)
	serverName := "Very-Long-Server-Name-That-Needs-Processing"
	statusEmoji := "✅"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.ProcessServerButtonText(serverName, statusEmoji, 20)
	}
}
