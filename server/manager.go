package server

import (
	"fmt"
	"sync"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
	"xray-telegram-manager/xray"
)

// ServerManager coordinates all server-related operations
type ServerManager struct {
	config             *config.Config
	servers            []types.Server
	currentServer      *types.Server
	subscriptionLoader *SubscriptionLoaderImpl
	pingTester         *PingTesterImpl
	xrayController     *xray.XrayController
	mutex              sync.RWMutex
}

// NewServerManager creates a new server manager instance
func NewServerManager(cfg *config.Config) *ServerManager {
	return &ServerManager{
		config:             cfg,
		servers:            make([]types.Server, 0),
		currentServer:      nil,
		subscriptionLoader: NewSubscriptionLoader(cfg),
		pingTester:         NewPingTester(cfg),
		xrayController:     xray.NewXrayController(&configAdapter{cfg}),
		mutex:              sync.RWMutex{},
	}
}

// configAdapter adapts config.Config to implement xray.ConfigProvider interface
type configAdapter struct {
	*config.Config
}

func (ca *configAdapter) GetConfigPath() string {
	return ca.ConfigPath
}

func (ca *configAdapter) GetXrayRestartCommand() string {
	return ca.XrayRestartCommand
}

// LoadServers loads servers from subscription URL using SubscriptionLoader
func (sm *ServerManager) LoadServers() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	servers, err := sm.subscriptionLoader.LoadFromURL()
	if err != nil {
		return fmt.Errorf("failed to load servers from subscription: %w", err)
	}

	if len(servers) == 0 {
		return fmt.Errorf("no servers found in subscription")
	}

	sm.servers = servers
	return nil
}

// GetServers returns a copy of all available servers (thread-safe)
func (sm *ServerManager) GetServers() []types.Server {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make([]types.Server, len(sm.servers))
	copy(result, sm.servers)
	return result
}

// GetCurrentServer returns the currently active server (thread-safe)
func (sm *ServerManager) GetCurrentServer() *types.Server {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if sm.currentServer == nil {
		return nil
	}

	// Return a copy to prevent external modification
	serverCopy := *sm.currentServer
	return &serverCopy
}

// GetServerByID finds a server by its ID
func (sm *ServerManager) GetServerByID(serverID string) (*types.Server, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for _, server := range sm.servers {
		if server.ID == serverID {
			serverCopy := server
			return &serverCopy, nil
		}
	}

	return nil, fmt.Errorf("server with ID %s not found", serverID)
}

// RefreshServers forces a refresh of the server list from subscription
func (sm *ServerManager) RefreshServers() error {
	// Invalidate cache to force fresh load
	sm.subscriptionLoader.InvalidateCache()
	return sm.LoadServers()
}

// SwitchServer switches to the specified server by coordinating xray updates
func (sm *ServerManager) SwitchServer(serverID string) error {
	// Find the server by ID
	server, err := sm.GetServerByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Create backup before switching
	if err := sm.xrayController.BackupConfig(); err != nil {
		return fmt.Errorf("failed to create backup before switching: %w", err)
	}

	// Update xray configuration
	if err := sm.xrayController.UpdateConfig(*server); err != nil {
		return fmt.Errorf("failed to update xray configuration: %w", err)
	}

	// Restart xray service
	if err := sm.xrayController.RestartService(); err != nil {
		// Attempt to restore backup on restart failure
		if restoreErr := sm.xrayController.RestoreConfig(); restoreErr != nil {
			return fmt.Errorf("failed to restart xray service: %w, and failed to restore backup: %v", err, restoreErr)
		}

		// Try to restart again after restore
		if restartErr := sm.xrayController.RestartService(); restartErr != nil {
			return fmt.Errorf("failed to restart xray service after restore: %w (original error: %v)", restartErr, err)
		}

		return fmt.Errorf("xray service restart failed but backup was restored and service restarted: %w", err)
	}

	// Update current server state (thread-safe)
	sm.mutex.Lock()
	sm.currentServer = server
	sm.mutex.Unlock()

	return nil
}

// TestPing tests ping latency for all servers using PingTester
func (sm *ServerManager) TestPing() ([]types.PingResult, error) {
	return sm.TestPingWithProgress(nil)
}

// TestPingWithProgress tests ping latency for all servers with progress callback
func (sm *ServerManager) TestPingWithProgress(progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error) {
	servers := sm.GetServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers available for ping testing")
	}

	results, err := sm.pingTester.TestServersWithProgress(servers, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("failed to test server pings: %w", err)
	}

	// Sort results by latency for better user experience
	sortedResults := sm.pingTester.SortByLatency(results)
	return sortedResults, nil
}

// GetServerStatus returns status information about the current server and connection
func (sm *ServerManager) GetServerStatus() (map[string]interface{}, error) {
	sm.mutex.RLock()
	currentServer := sm.currentServer
	sm.mutex.RUnlock()

	status := make(map[string]interface{})

	if currentServer == nil {
		status["current_server"] = nil
		status["status"] = "no_server_selected"
		status["message"] = "No server is currently selected"
		return status, nil
	}

	status["current_server"] = map[string]interface{}{
		"id":      currentServer.ID,
		"name":    currentServer.Name,
		"address": currentServer.Address,
		"port":    currentServer.Port,
		"tag":     currentServer.Tag,
	}

	// Test current server connectivity
	pingResult := sm.pingTester.TestServer(*currentServer)

	if pingResult.Available {
		status["status"] = "connected"
		status["latency"] = pingResult.Latency
		status["message"] = fmt.Sprintf("Connected to %s (latency: %dms)", currentServer.Name, pingResult.Latency)
	} else {
		status["status"] = "disconnected"
		status["latency"] = 0
		status["error"] = pingResult.Error.Error()
		status["message"] = fmt.Sprintf("Connection to %s failed: %s", currentServer.Name, pingResult.Error.Error())
	}

	return status, nil
}

// SetCurrentServer manually sets the current server (for initialization purposes)
func (sm *ServerManager) SetCurrentServer(serverID string) error {
	server, err := sm.GetServerByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	sm.mutex.Lock()
	sm.currentServer = server
	sm.mutex.Unlock()

	return nil
}

// DetectCurrentServer attempts to detect the currently active server from xray config
func (sm *ServerManager) DetectCurrentServer() error {
	// Get current xray configuration
	xrayConfig, err := sm.xrayController.GetCurrentConfig()
	if err != nil {
		return fmt.Errorf("failed to get current xray config: %w", err)
	}

	// Find the proxy outbound (not direct or blackhole)
	var proxyOutbound *types.XrayOutbound
	for _, outbound := range xrayConfig.Outbounds {
		if outbound.Protocol != "freedom" && outbound.Protocol != "blackhole" {
			proxyOutbound = &outbound
			break
		}
	}

	if proxyOutbound == nil {
		// No proxy outbound found, clear current server
		sm.mutex.Lock()
		sm.currentServer = nil
		sm.mutex.Unlock()
		return nil
	}

	// Try to match with available servers
	servers := sm.GetServers()
	for _, server := range servers {
		if sm.serverMatchesOutbound(server, *proxyOutbound) {
			sm.mutex.Lock()
			sm.currentServer = &server
			sm.mutex.Unlock()
			return nil
		}
	}

	// No matching server found, but there is a proxy outbound
	// This could happen if the config was manually modified
	sm.mutex.Lock()
	sm.currentServer = nil
	sm.mutex.Unlock()

	return fmt.Errorf("current xray configuration does not match any available servers")
}

// serverMatchesOutbound checks if a server configuration matches an xray outbound
func (sm *ServerManager) serverMatchesOutbound(server types.Server, outbound types.XrayOutbound) bool {
	// Compare basic properties
	if server.Tag != outbound.Tag || server.Protocol != outbound.Protocol {
		return false
	}

	// For more detailed comparison, we could check settings and stream settings
	// but for now, tag and protocol matching should be sufficient
	return true
}
