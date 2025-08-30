package telegram

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"xray-telegram-manager/types"
)

// MessageFormatter provides consistent message formatting with proper emoji usage and visual hierarchy
type MessageFormatter struct {
	// Configuration for formatting
	maxServerNameLength int
	maxErrorLength      int
}

// NewMessageFormatter creates a new message formatter with default settings
func NewMessageFormatter() *MessageFormatter {
	return &MessageFormatter{
		maxServerNameLength: 30,
		maxErrorLength:      100,
	}
}

// FormatWelcomeMessage creates a consistently formatted welcome message
func (mf *MessageFormatter) FormatWelcomeMessage(serverCount int) string {
	return fmt.Sprintf("🚀 Xray Telegram Manager\n\n"+
		"Welcome! I can help you manage your xray proxy servers.\n\n"+
		"📊 Server Status\n"+
		"└ Available servers: %d\n\n"+
		"💡 Quick Actions\n"+
		"Use the buttons below to get started:",
		serverCount)
}

// FormatServerListMessage creates a formatted server list with visual hierarchy
func (mf *MessageFormatter) FormatServerListMessage(servers []types.Server, currentServerID string, page, totalPages int) string {
	var builder strings.Builder

	// Header with pagination info
	if totalPages > 1 {
		builder.WriteString(fmt.Sprintf("📋 Server List (Page %d/%d)\n\n", page+1, totalPages))
	} else {
		builder.WriteString("📋 Server List\n\n")
	}

	// Server count summary
	builder.WriteString(fmt.Sprintf("📊 Summary\n"+
		"└ Total servers: %d\n\n", len(servers)))

	// Servers grouped by status
	builder.WriteString("🌐 Available Servers\n")

	const serversPerPage = 32
	start := page * serversPerPage
	end := start + serversPerPage
	if end > len(servers) {
		end = len(servers)
	}

	for i := start; i < end; i++ {
		server := servers[i]
		var statusIcon, statusText string

		if server.ID == currentServerID {
			statusIcon = "✅"
			statusText = " (Current)"
		} else {
			statusIcon = "🌐"
			statusText = ""
		}

		// Truncate server name if too long
		displayName := server.Name
		if len(displayName) > mf.maxServerNameLength {
			displayName = displayName[:mf.maxServerNameLength-3] + "..."
		}

		builder.WriteString(fmt.Sprintf("%s %s%s\n", statusIcon, displayName, statusText))
	}

	return builder.String()
}

// FormatPingTestProgress creates a formatted ping test progress message
func (mf *MessageFormatter) FormatPingTestProgress(completed, total int, currentServer string) string {
	percentage := (completed * 100) / total
	progressBar := mf.createProgressBar(percentage, 20)

	// Truncate current server name
	displayName := currentServer
	if len(displayName) > 25 {
		displayName = displayName[:22] + "..."
	}

	return fmt.Sprintf("🏓 Ping Test in Progress\n\n"+
		"📊 Progress Overview\n"+
		"└ Completed: %d/%d servers (%d%%)\n\n"+
		"%s\n\n"+
		"🔄 Currently Testing\n"+
		"└ %s\n\n"+
		"⏳ Please wait while testing continues...",
		completed, total, percentage, progressBar, displayName)
}

// FormatPingTestResults creates a formatted ping test results message
func (mf *MessageFormatter) FormatPingTestResults(results []types.PingResult, currentServerID string) string {
	var builder strings.Builder

	// Count available servers
	availableCount := 0
	for _, result := range results {
		if result.Available {
			availableCount++
		}
	}

	// Header and summary
	builder.WriteString("🏓 Ping Test Complete\n\n")
	builder.WriteString(fmt.Sprintf("📊 Test Summary\n"+
		"└ Available: %d/%d servers\n"+
		"└ Success rate: %.1f%%\n\n",
		availableCount, len(results), float64(availableCount)/float64(len(results))*100))

	// Fast servers section
	if availableCount > 0 {
		builder.WriteString("⚡ Fastest Servers\n")

		count := 0
		maxFastest := 10
		for _, result := range results {
			if result.Available && count < maxFastest {
				var statusIcon, statusText string
				if result.Server.ID == currentServerID {
					statusIcon = "✅"
					statusText = " (Current)"
				} else {
					statusIcon = "🟢"
					statusText = ""
				}

				// Format latency with quality indicator
				qualityEmoji := mf.getLatencyQualityEmoji(result.Latency)

				displayName := result.Server.Name
				if len(displayName) > 20 {
					displayName = displayName[:17] + "..."
				}

				builder.WriteString(fmt.Sprintf("%s %s %s %dms%s\n",
					statusIcon, displayName, qualityEmoji, result.Latency, statusText))
				count++
			}
		}
		builder.WriteString("\n")
	}

	// Unavailable servers section
	unavailableCount := len(results) - availableCount
	if unavailableCount > 0 {
		builder.WriteString(fmt.Sprintf("❌ Unavailable Servers\n"+
			"└ %d servers are currently unreachable\n\n", unavailableCount))
	}

	// Recommendations
	builder.WriteString("💡 Recommendations\n")
	if availableCount > 0 {
		builder.WriteString("└ Select a fast server from the quick-select buttons\n")
		builder.WriteString("└ Servers with 🟢 quality are recommended\n")
	} else {
		builder.WriteString("└ Check your internet connection\n")
		builder.WriteString("└ Try refreshing the server list\n")
	}

	return builder.String()
}

// FormatServerStatusMessage creates a formatted server status message
func (mf *MessageFormatter) FormatServerStatusMessage(server *types.Server, result *types.PingResult) string {
	var builder strings.Builder

	builder.WriteString("📊 Current Server Status\n\n")

	// Server information section
	builder.WriteString("🏷️ Server Information\n")
	builder.WriteString(fmt.Sprintf("└ Name: %s\n", server.Name))
	builder.WriteString(fmt.Sprintf("└ Address: %s:%d\n", server.Address, server.Port))
	builder.WriteString(fmt.Sprintf("└ Protocol: %s\n", server.Protocol))
	builder.WriteString(fmt.Sprintf("└ Tag: %s\n\n", server.Tag))

	// Connection status section
	builder.WriteString("🔗 Connection Status\n")

	if result != nil {
		if result.Available {
			qualityEmoji := mf.getLatencyQualityEmoji(result.Latency)
			qualityText := mf.getLatencyQualityText(result.Latency)

			builder.WriteString("└ Status: ✅ Connected\n")
			builder.WriteString(fmt.Sprintf("└ Latency: ⚡ %dms\n", result.Latency))
			builder.WriteString(fmt.Sprintf("└ Quality: %s %s\n", qualityEmoji, qualityText))
		} else {
			errorMsg := result.Error.Error()
			if len(errorMsg) > mf.maxErrorLength {
				errorMsg = errorMsg[:mf.maxErrorLength-3] + "..."
			}

			builder.WriteString("└ Status: ❌ Disconnected\n")
			builder.WriteString(fmt.Sprintf("└ Error: %s\n", errorMsg))
			builder.WriteString("└ Quality: 🔴 Unavailable\n")
		}
	} else {
		builder.WriteString("└ Status: ⏳ Testing connection...\n")
	}

	// Timestamp
	builder.WriteString("\n🕐 Last Updated\n")
	builder.WriteString(fmt.Sprintf("└ %s\n", time.Now().Format("15:04:05")))

	return builder.String()
}

// FormatErrorMessage creates a consistently formatted error message
func (mf *MessageFormatter) FormatErrorMessage(title, description string, suggestions []string) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("❌ %s\n\n", title))

	// Error details
	builder.WriteString("🔴 Error Details\n")

	errorMsg := description
	if len(errorMsg) > mf.maxErrorLength {
		errorMsg = errorMsg[:mf.maxErrorLength-3] + "..."
	}
	builder.WriteString(fmt.Sprintf("└ %s\n\n", errorMsg))

	// Suggestions if provided
	if len(suggestions) > 0 {
		builder.WriteString("💡 Suggested Actions\n")
		for _, suggestion := range suggestions {
			builder.WriteString(fmt.Sprintf("└ %s\n", suggestion))
		}
	}

	return builder.String()
}

// FormatUpdateProgressMessage creates a formatted update progress message
func (mf *MessageFormatter) FormatUpdateProgressMessage(progress int, stage, message string) string {
	var builder strings.Builder

	stageEmoji := mf.getUpdateStageEmoji(stage)
	progressBar := mf.createProgressBar(progress, 20)

	builder.WriteString("🔄 Bot Update in Progress\n\n")

	// Progress section
	builder.WriteString("📊 Update Progress\n")
	builder.WriteString(fmt.Sprintf("└ Completion: %d%%\n", progress))
	builder.WriteString(fmt.Sprintf("└ %s\n\n", progressBar))

	// Current stage section
	builder.WriteString("⚙️ Current Stage\n")
	builder.WriteString(fmt.Sprintf("└ %s %s\n", stageEmoji, toTitle(stage)))
	if message != "" {
		builder.WriteString(fmt.Sprintf("└ %s\n", message))
	}
	builder.WriteString("\n")

	// Status message
	builder.WriteString("⏳ Please Wait\n")
	builder.WriteString("└ The update process is running\n")
	builder.WriteString("└ Do not close the application\n")

	return builder.String()
}

// FormatNoServersMessage creates a formatted "no servers" message
func (mf *MessageFormatter) FormatNoServersMessage() string {
	return "❌ No Servers Available\n\n" +
		"🔴 Issue\n" +
		"└ No servers were found in your configuration\n\n" +
		"💡 Possible Solutions\n" +
		"└ Check your subscription configuration\n" +
		"└ Verify your internet connection\n" +
		"└ Try refreshing the server list\n\n" +
		"🔄 Use the refresh button to try again"
}

// FormatUnauthorizedMessage creates a formatted unauthorized access message
func (mf *MessageFormatter) FormatUnauthorizedMessage() string {
	return "❌ Unauthorized Access\n\n" +
		"🔒 Access Denied\n" +
		"└ This bot is restricted to authorized users only\n\n" +
		"💡 Information\n" +
		"└ Contact the administrator for access\n" +
		"└ Ensure you're using the correct account"
}

// FormatRateLimitMessage creates a formatted rate limit message
func (mf *MessageFormatter) FormatRateLimitMessage() string {
	return "⚠️ Rate Limit Exceeded\n\n" +
		"🚫 Request Limit\n" +
		"└ You are sending requests too quickly\n\n" +
		"💡 Next Steps\n" +
		"└ Please wait a moment before trying again\n" +
		"└ This helps maintain system stability"
}

// Helper methods

func (mf *MessageFormatter) createProgressBar(progress int, length int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	filled := (progress * length) / 100
	empty := length - filled

	var bar strings.Builder
	for i := 0; i < filled; i++ {
		bar.WriteString("█")
	}
	for i := 0; i < empty; i++ {
		bar.WriteString("░")
	}

	return fmt.Sprintf("[%s] %d%%", bar.String(), progress)
}

func (mf *MessageFormatter) getLatencyQualityEmoji(latency int64) string {
	if latency < 100 {
		return "🟢" // Excellent
	} else if latency < 300 {
		return "🟡" // Good
	} else if latency < 500 {
		return "🟠" // Fair
	} else {
		return "🔴" // Poor
	}
}

func (mf *MessageFormatter) getLatencyQualityText(latency int64) string {
	if latency < 100 {
		return "Excellent"
	} else if latency < 300 {
		return "Good"
	} else if latency < 500 {
		return "Fair"
	} else {
		return "Poor"
	}
}

func (mf *MessageFormatter) getUpdateStageEmoji(stage string) string {
	switch strings.ToLower(stage) {
	case "downloading":
		return "📥"
	case "backing_up":
		return "💾"
	case "installing":
		return "⚙️"
	case "completing":
		return "✅"
	case "initializing":
		return "🔄"
	default:
		return "🔄"
	}
}

// toTitle capitalizes the first letter of each word, replacing deprecated strings.Title
func toTitle(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	inWord := false

	for i, r := range runes {
		if unicode.IsLetter(r) {
			if !inWord {
				runes[i] = unicode.ToUpper(r)
				inWord = true
			} else {
				runes[i] = unicode.ToLower(r)
			}
		} else {
			inWord = false
		}
	}

	return string(runes)
}
