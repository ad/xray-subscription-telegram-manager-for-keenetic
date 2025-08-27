package telegram

import (
	"strings"
	"xray-telegram-manager/types"

	"github.com/go-telegram/bot/models"
)

type Server = types.Server
type ServerPingResult = types.PingResult

func getUsername(user *models.User) string {
	if user == nil {
		return "unknown"
	}

	if user.Username != "" {
		return "@" + user.Username
	}

	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if name == "" {
		return "unknown"
	}

	return name
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
