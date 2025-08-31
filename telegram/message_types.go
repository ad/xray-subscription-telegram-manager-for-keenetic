package telegram

import (
	"time"

	"github.com/go-telegram/bot/models"
)

// MessageType represents the type of message being managed
type MessageType string

const (
	MessageTypeMenu       MessageType = "menu"
	MessageTypeServerList MessageType = "server_list"
	MessageTypePingTest   MessageType = "ping_test"
	MessageTypeStatus     MessageType = "status"
)

// ActiveMessage represents an active message that can be edited
type ActiveMessage struct {
	ChatID    int64
	MessageID int
	Type      MessageType
	CreatedAt time.Time
}

// MessageContent represents the content to be sent or edited
type MessageContent struct {
	Text        string
	ReplyMarkup *models.InlineKeyboardMarkup
	ParseMode   models.ParseMode
	Type        MessageType
}
