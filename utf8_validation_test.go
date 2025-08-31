package main

import (
	"testing"
	"unicode/utf8"
	"xray-telegram-manager/telegram"
	"xray-telegram-manager/types"
)

func TestUTF8Handling(t *testing.T) {
	// Test MessageFormatter with UTF-8 server names
	t.Run("MessageFormatter UTF-8 server names", func(t *testing.T) {
		formatter := telegram.NewMessageFormatter()

		servers := []types.Server{
			{ID: "1", Name: "🌟 Российский сервер №1 с очень длинным названием которое должно быть обрезано"},
			{ID: "2", Name: "🚀 中文服务器名称"},
			{ID: "3", Name: "💯 العربية خادم اسم"},
			{ID: "4", Name: "🎯 Normal ASCII server name"},
		}

		result := formatter.FormatServerListMessage(servers, "1", 0, 1)

		// The result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("Formatted server list should be valid UTF-8")
		}

		// Should contain references to all servers
		if !contains(result, "🌟") || !contains(result, "🚀") || !contains(result, "💯") || !contains(result, "🎯") {
			t.Errorf("Result should contain server icons: %s", result)
		}

		t.Logf("Formatted message: %s", result)
	})

	// Test ping progress with UTF-8 server name
	t.Run("Ping progress with UTF-8", func(t *testing.T) {
		formatter := telegram.NewMessageFormatter()

		// Long UTF-8 server name that should be truncated
		longUTF8Name := "🌟 Очень длинное название российского сервера которое должно быть обрезано"

		result := formatter.FormatPingTestProgress(5, 10, longUTF8Name)

		// The result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("Ping progress message should be valid UTF-8")
		}

		// Should contain progress information
		if !contains(result, "5/10") {
			t.Errorf("Result should contain progress information: %s", result)
		}

		t.Logf("Ping progress message: %s", result)
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
