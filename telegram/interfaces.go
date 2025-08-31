package telegram

import (
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)

// Local interfaces for telegram package
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
	GetCurrentServer() *types.Server
	SwitchServer(serverID string) error
	GetServerByID(serverID string) (*types.Server, error)
	RefreshServers() error
	TestPing() ([]types.PingResult, error)
	TestPingWithProgress(progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error)
	GetQuickSelectServers(results []types.PingResult, limit int) []types.PingResult
	GetServerStatus() (map[string]interface{}, error)
	SetCurrentServer(serverID string) error
	DetectCurrentServer() error
}
