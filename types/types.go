package types

import "time"

// Server represents a proxy server configuration
type Server struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Address        string                 `json:"add"`
	Port           int                    `json:"port"`
	UUID           string                 `json:"uuid"`
	Security       string                 `json:"scy"`
	Network        string                 `json:"net"`
	Type           string                 `json:"type"`
	Path           string                 `json:"path"`
	Host           string                 `json:"host"`
	TLS            string                 `json:"tls"`
	SNI            string                 `json:"sni"`
	ALPN           string                 `json:"alpn"`
	Fp             string                 `json:"fp"`
	Tag            string                 `json:"tag"`
	Protocol       string                 `json:"protocol"`
	Settings       map[string]interface{} `json:"settings,omitempty"`
	StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
	VlessUrl       string                 `json:"vlessUrl,omitempty"`
}

// PingResult represents the result of pinging a server
type PingResult struct {
	Server    Server
	Latency   time.Duration
	Error     error
	Success   bool
	Available bool
	TestTime  time.Time
}

// XrayConfig represents the Xray configuration structure
type XrayConfig struct {
	Inbounds  []XrayInbound  `json:"inbounds"`
	Outbounds []XrayOutbound `json:"outbounds"`
}

// XrayInbound represents an inbound configuration
type XrayInbound struct {
	Tag      string                 `json:"tag"`
	Port     int                    `json:"port"`
	Protocol string                 `json:"protocol"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// XrayOutbound represents an outbound configuration
type XrayOutbound struct {
	Tag            string                 `json:"tag"`
	Protocol       string                 `json:"protocol"`
	Settings       map[string]interface{} `json:"settings"`
	StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
}

// SubscriptionLoader interface for loading servers from subscription
type SubscriptionLoader interface {
	LoadServers() ([]Server, error)
	LoadFromURL() ([]Server, error)
	InvalidateCache()
}

// PingTester interface for testing server latency
type PingTester interface {
	TestServer(server Server) PingResult
	TestServers(servers []Server) []PingResult
	TestServersWithProgress(servers []Server, progressCallback func(int, int)) ([]PingResult, error)
}

// XrayController interface for managing Xray configuration
type XrayControllerInterface interface {
	UpdateConfig(server Server) error
	RestartXray() error
}
