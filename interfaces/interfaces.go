package interfaces
import (
	"context"
	"time"
	"xray-telegram-manager/types"
)
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}
type ConfigProvider interface {
	GetAdminID() int64
	GetBotToken() string
	GetConfigPath() string
	GetSubscriptionURL() string
	GetCacheDuration() int
	GetPingTimeout() int
	GetXrayRestartCommand() string
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
	GetServerStatus() (map[string]interface{}, error)
	SetCurrentServer(serverID string) error
	DetectCurrentServer() error
}
type SubscriptionLoader interface {
	LoadFromURL() ([]types.Server, error)
	GetCachedServers() []types.Server
	InvalidateCache()
	DecodeBase64Config(data string) ([]types.Server, error)
	ParseVlessUrls(urls []string) ([]types.Server, error)
	ParseVlessUrl(vlessUrl string) (types.Server, error)
}
type PingTester interface {
	TestServers(servers []types.Server) ([]types.PingResult, error)
	TestServersWithProgress(servers []types.Server, progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error)
	TestServer(server types.Server) types.PingResult
	SortByLatency(results []types.PingResult) []types.PingResult
	FormatResultsForTelegram(results []types.PingResult) string
	GetAvailableServers(results []types.PingResult) []types.Server
	GetFastestServer(results []types.PingResult) (*types.Server, error)
}
type XrayController interface {
	UpdateConfig(server types.Server) error
	RestartService() error
	GetCurrentConfig() (*types.XrayConfig, error)
	BackupConfig() error
	RestoreConfig() error
	ReplaceProxyOutbound(server types.Server) error
}
type TelegramBot interface {
	Start(ctx context.Context) error
	Stop()
}
type HealthChecker interface {
	PerformHealthCheck() map[string]interface{}
	GetHealthStatus() map[string]interface{}
	GetLastHealthCheck() time.Time
	IsHealthy() bool
}
type Service interface {
	Start() error
	Stop() error
	Restart() error
	Reload() error
	IsRunning() bool
	GetStatus() map[string]interface{}
}
