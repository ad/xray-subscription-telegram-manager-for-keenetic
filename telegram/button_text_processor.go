package telegram

// ButtonTextProcessor handles emoji-aware text processing for Telegram buttons
type ButtonTextProcessor struct {
	maxLength int
	emojiMap  map[string]int // emoji -> display width
}

// ButtonTextProcessorInterface defines the interface for button text processing
type ButtonTextProcessorInterface interface {
	// ProcessButtonText processes text for button display with emoji awareness
	ProcessButtonText(text string, maxLength int) string

	// CalculateTextLength calculates the real display length of text with emojis
	CalculateTextLength(text string) int

	// TruncateWithEmoji truncates text while preserving emoji integrity
	TruncateWithEmoji(text string, maxLength int) string
}

// NewButtonTextProcessor creates a new ButtonTextProcessor instance
func NewButtonTextProcessor(maxLength int) *ButtonTextProcessor {
	processor := &ButtonTextProcessor{
		maxLength: maxLength,
		emojiMap:  make(map[string]int),
	}

	// Initialize common emoji mappings with their display widths
	processor.initializeEmojiMap()

	return processor
}

// ProcessButtonText processes button text with emoji awareness and smart truncation
func (btp *ButtonTextProcessor) ProcessButtonText(text string, maxLength int) string {
	if text == "" {
		return text
	}

	// Use provided maxLength or default
	targetLength := maxLength
	if targetLength <= 0 {
		targetLength = btp.maxLength
	}

	// Calculate actual display length
	displayLength := btp.CalculateTextLength(text)

	// If text fits, return as-is
	if displayLength <= targetLength {
		return text
	}

	// Truncate with emoji preservation
	return btp.TruncateWithEmoji(text, targetLength)
}

// CalculateTextLength calculates the real display length considering emojis
func (btp *ButtonTextProcessor) CalculateTextLength(text string) int {
	if text == "" {
		return 0
	}

	length := 0
	runes := []rune(text)

	for i := 0; i < len(runes); {
		// Check for multi-rune emoji sequences
		emojiLength := btp.getEmojiLength(runes, i)
		if emojiLength > 0 {
			// This is an emoji sequence, count as 2 display units
			length += 2
			i += emojiLength
		} else {
			// Regular character, count as 1
			length++
			i++
		}
	}

	return length
}

// TruncateWithEmoji truncates text while preserving emoji integrity
func (btp *ButtonTextProcessor) TruncateWithEmoji(text string, maxLength int) string {
	if text == "" || maxLength <= 0 {
		return ""
	}

	// Check if text already fits
	currentDisplayLength := btp.CalculateTextLength(text)
	if currentDisplayLength <= maxLength {
		return text
	}

	// Reserve space for ellipsis if needed
	ellipsis := "..."
	ellipsisLength := 3
	targetLength := maxLength - ellipsisLength

	if targetLength <= 0 {
		return ellipsis
	}

	runes := []rune(text)
	result := make([]rune, 0, len(runes))
	currentLength := 0

	for i := 0; i < len(runes); {
		// Check for emoji sequence
		emojiLength := btp.getEmojiLength(runes, i)

		if emojiLength > 0 {
			// This is an emoji sequence
			emojiDisplayLength := 2

			// Check if adding this emoji would exceed the limit
			if currentLength+emojiDisplayLength > targetLength {
				break
			}

			// Add the entire emoji sequence
			for j := 0; j < emojiLength; j++ {
				result = append(result, runes[i+j])
			}
			currentLength += emojiDisplayLength
			i += emojiLength
		} else {
			// Regular character
			if currentLength+1 > targetLength {
				break
			}

			result = append(result, runes[i])
			currentLength++
			i++
		}
	}

	// Only add ellipsis if we actually truncated something
	if len(result) < len(runes) {
		return string(result) + ellipsis
	}

	return string(result)
}

// getEmojiLength determines if the rune sequence starting at index i is an emoji
// and returns the length of the emoji sequence in runes
func (btp *ButtonTextProcessor) getEmojiLength(runes []rune, startIndex int) int {
	if startIndex >= len(runes) {
		return 0
	}

	// Check for common emoji patterns
	r := runes[startIndex]

	// Single emoji characters
	if btp.isEmojiRune(r) {
		length := 1

		// Check for variation selectors and modifiers
		for i := startIndex + 1; i < len(runes) && i < startIndex+4; i++ {
			next := runes[i]
			if btp.isEmojiModifier(next) || btp.isVariationSelector(next) {
				length++
			} else if next == 0x200D { // Zero Width Joiner
				length++
				// ZWJ sequences can be complex, look for the next emoji
				if i+1 < len(runes) && btp.isEmojiRune(runes[i+1]) {
					length++
					i++ // Skip the next emoji as we counted it
				}
			} else {
				break
			}
		}

		return length
	}

	return 0
}

// isEmojiRune checks if a rune is an emoji character
func (btp *ButtonTextProcessor) isEmojiRune(r rune) bool {
	// Common emoji ranges
	return (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
		(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols and Pictographs
		(r >= 0x1F680 && r <= 0x1F6FF) || // Transport and Map
		(r >= 0x1F1E6 && r <= 0x1F1FF) || // Regional indicators (flags)
		(r >= 0x2600 && r <= 0x26FF) || // Misc symbols
		(r >= 0x2700 && r <= 0x27BF) || // Dingbats
		(r >= 0xFE00 && r <= 0xFE0F) || // Variation Selectors
		r == 0x2764 || // Heavy Black Heart
		r == 0x2665 || // Black Heart Suit
		r == 0x2663 || // Black Club Suit
		r == 0x2666 || // Black Diamond Suit
		r == 0x2660 || // Black Spade Suit
		r == 0x2B50 || // White Medium Star
		r == 0x2705 || // White Heavy Check Mark
		r == 0x274C || // Cross Mark
		r == 0x274E || // Negative Squared Cross Mark
		r == 0x2753 || // Black Question Mark Ornament
		r == 0x2757 || // Heavy Exclamation Mark Symbol
		r == 0x203C || // Double Exclamation Mark
		r == 0x2049 // Exclamation Question Mark
}

// isEmojiModifier checks if a rune is an emoji modifier
func (btp *ButtonTextProcessor) isEmojiModifier(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF // Skin tone modifiers
}

// isVariationSelector checks if a rune is a variation selector
func (btp *ButtonTextProcessor) isVariationSelector(r rune) bool {
	return r >= 0xFE00 && r <= 0xFE0F
}

// initializeEmojiMap initializes the emoji mapping with common emojis and their display widths
func (btp *ButtonTextProcessor) initializeEmojiMap() {
	// Common status emojis used in the bot
	commonEmojis := map[string]int{
		"✅":  2, // Check mark
		"❌":  2, // Cross mark
		"🌐":  2, // Globe
		"📋":  2, // Clipboard
		"📊":  2, // Bar chart
		"🔄":  2, // Counterclockwise arrows
		"🏠":  2, // House
		"⚡":  2, // High voltage
		"🔴":  2, // Red circle
		"🟢":  2, // Green circle
		"🟡":  2, // Yellow circle
		"🟠":  2, // Orange circle
		"⬅️": 2, // Left arrow
		"➡️": 2, // Right arrow
		"🏓":  2, // Ping pong
		"🚀":  2, // Rocket
		"⏳":  2, // Hourglass
		"💡":  2, // Light bulb
		"🎯":  2, // Direct hit
		"🔗":  2, // Link
		"🏷️": 2, // Label
		"📄":  2, // Page facing up
		"🎉":  2, // Party popper
		"⚠️": 2, // Warning sign
	}

	for emoji, width := range commonEmojis {
		btp.emojiMap[emoji] = width
	}
}

// GetEmojiDisplayWidth returns the display width of a specific emoji
func (btp *ButtonTextProcessor) GetEmojiDisplayWidth(emoji string) int {
	if width, exists := btp.emojiMap[emoji]; exists {
		return width
	}

	// Default emoji width
	return 2
}

// ProcessServerButtonText specifically processes server button text with status emojis
func (btp *ButtonTextProcessor) ProcessServerButtonText(serverName string, statusEmoji string, maxLength int) string {
	if serverName == "" {
		return statusEmoji
	}

	// Calculate space taken by status emoji and space
	statusLength := btp.CalculateTextLength(statusEmoji + " ")

	// Calculate available space for server name
	availableLength := maxLength - statusLength

	if availableLength <= 0 {
		return statusEmoji
	}

	// Process the server name to fit in available space
	processedName := btp.ProcessButtonText(serverName, availableLength)

	// Combine status emoji with processed name
	return statusEmoji + " " + processedName
}
