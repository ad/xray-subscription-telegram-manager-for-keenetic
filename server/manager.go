package server

import (
	"fmt"
	"strings"
	"sync"
	"xray-telegram-manager/config"
	"xray-telegram-manager/logger"
	"xray-telegram-manager/types"
)

type ServerManager struct {
	config             *config.Config
	servers            []types.Server
	currentServer      *types.Server
	subscriptionLoader SubscriptionLoader
	pingTester         *PingTesterImpl
	xrayController     *XrayController
	nameOptimizer      *ServerNameOptimizer
	serverSorter       *ServerSorter
	logger             *logger.Logger
	mutex              sync.RWMutex
}

func NewServerManager(cfg *config.Config) *ServerManager {
	logLevel := logger.ParseLogLevel(cfg.LogLevel)
	log := logger.NewLogger(logLevel, nil)

	return &ServerManager{
		config:             cfg,
		servers:            make([]types.Server, 0),
		currentServer:      nil,
		subscriptionLoader: NewSubscriptionLoader(cfg),
		pingTester:         NewPingTester(cfg),
		xrayController:     NewXrayController(&configAdapter{cfg}),
		nameOptimizer:      NewServerNameOptimizer(cfg.UI.NameOptimizationThreshold, log),
		serverSorter:       NewServerSorter(),
		logger:             log,
		mutex:              sync.RWMutex{},
	}
}
func NewServerManagerWithCacheDir(cfg *config.Config, cacheDir string) *ServerManager {
	logLevel := logger.ParseLogLevel(cfg.LogLevel)
	log := logger.NewLogger(logLevel, nil)

	return &ServerManager{
		config:             cfg,
		servers:            make([]types.Server, 0),
		currentServer:      nil,
		subscriptionLoader: NewSubscriptionLoaderWithCacheDir(cfg, cacheDir),
		pingTester:         NewPingTester(cfg),
		xrayController:     NewXrayController(&configAdapter{cfg}),
		nameOptimizer:      NewServerNameOptimizer(cfg.UI.NameOptimizationThreshold, log),
		serverSorter:       NewServerSorter(),
		logger:             log,
		mutex:              sync.RWMutex{},
	}
}

type configAdapter struct {
	*config.Config
}

func (ca *configAdapter) GetConfigPath() string {
	return ca.ConfigPath
}
func (ca *configAdapter) GetXrayRestartCommand() string {
	return ca.XrayRestartCommand
}
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

	// Apply name optimization if enabled
	if sm.config.UI.EnableNameOptimization && sm.nameOptimizer != nil {
		optimizationResult := sm.nameOptimizer.OptimizeNames(servers)
		if optimizationResult.AppliedCount > 0 {
			// Apply the optimization to the servers
			servers = sm.nameOptimizer.ApplyOptimization(servers, optimizationResult.RemovedSuffix)
			sm.logger.Info("Applied server name optimization: removed '%s' from %d/%d servers",
				optimizationResult.RemovedSuffix, optimizationResult.AppliedCount, optimizationResult.TotalCount)
		} else {
			sm.logger.Debug("No server name optimization applied")
		}
	}

	sm.servers = servers
	return nil
}
func (sm *ServerManager) GetServers() []types.Server {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	result := make([]types.Server, len(sm.servers))
	copy(result, sm.servers)

	// Sort servers alphabetically for consistent display
	return sm.serverSorter.SortAlphabetically(result)
}
func (sm *ServerManager) GetCurrentServer() *types.Server {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	if sm.currentServer == nil {
		return nil
	}
	serverCopy := *sm.currentServer
	return &serverCopy
}
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
func (sm *ServerManager) RefreshServers() error {
	sm.subscriptionLoader.InvalidateCache()
	return sm.LoadServers()
}
func (sm *ServerManager) SwitchServer(serverID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	var targetServer *types.Server
	for _, server := range sm.servers {
		if server.ID == serverID {
			serverCopy := server
			targetServer = &serverCopy
			break
		}
	}
	if targetServer == nil {
		return fmt.Errorf("server with ID %s not found", serverID)
	}
	if sm.currentServer != nil && sm.currentServer.ID == serverID {
		return fmt.Errorf("server %s is already active", targetServer.Name)
	}
	if err := sm.xrayController.BackupConfig(); err != nil {
		return fmt.Errorf("failed to create backup before switching: %w", err)
	}
	if err := sm.xrayController.UpdateConfig(*targetServer); err != nil {
		return fmt.Errorf("failed to update xray configuration: %w", err)
	}
	if err := sm.xrayController.RestartService(); err != nil {
		if restoreErr := sm.xrayController.RestoreConfig(); restoreErr != nil {
			return fmt.Errorf("failed to restart xray service: %w, and failed to restore backup: %v", err, restoreErr)
		}
		if restartErr := sm.xrayController.RestartService(); restartErr != nil {
			return fmt.Errorf("failed to restart xray service after restore: %w (original error: %v)", restartErr, err)
		}
		return fmt.Errorf("xray service restart failed but backup was restored and service restarted: %w", err)
	}
	sm.currentServer = targetServer
	return nil
}
func (sm *ServerManager) TestPing() ([]types.PingResult, error) {
	return sm.TestPingWithProgress(nil)
}

// GetQuickSelectServers returns the fastest available servers for quick selection
func (sm *ServerManager) GetQuickSelectServers(results []types.PingResult, limit int) []types.PingResult {
	return sm.serverSorter.SortForQuickSelect(results, limit)
}
func (sm *ServerManager) TestPingWithProgress(progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error) {
	servers := sm.GetServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers available for ping testing")
	}
	results, err := sm.pingTester.TestServersWithProgress(servers, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("failed to test server pings: %w", err)
	}
	// Use the new ServerSorter for combined sorting (speed priority, then alphabetical)
	sortedResults := sm.serverSorter.SortPingResults(results)
	return sortedResults, nil
}
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
func (sm *ServerManager) DetectCurrentServer() error {
	xrayConfig, err := sm.xrayController.GetCurrentConfig()
	if err != nil {
		return fmt.Errorf("failed to get current xray config: %w", err)
	}
	var proxyOutbound *types.XrayOutbound
	for _, outbound := range xrayConfig.Outbounds {
		if outbound.Protocol != "freedom" && outbound.Protocol != "blackhole" {
			proxyOutbound = &outbound
			break
		}
	}
	if proxyOutbound == nil {
		sm.mutex.Lock()
		sm.currentServer = nil
		sm.mutex.Unlock()
		return nil
	}
	servers := sm.GetServers()
	for _, server := range servers {
		if sm.serverMatchesOutbound(server, *proxyOutbound) {
			sm.mutex.Lock()
			sm.currentServer = &server
			sm.mutex.Unlock()
			return nil
		}
	}
	sm.mutex.Lock()
	sm.currentServer = nil
	sm.mutex.Unlock()
	return fmt.Errorf("current xray configuration does not match any available servers")
}
func (sm *ServerManager) serverMatchesOutbound(server types.Server, outbound types.XrayOutbound) bool {
	// Basic protocol check
	if server.Protocol != outbound.Protocol {
		return false
	}
	// Tag may be generic; if both present and different, fail fast
	if server.Tag != "" && outbound.Tag != "" && server.Tag != outbound.Tag {
		return false
	}

	// Helper to get nested map safely
	getMap := func(v interface{}) map[string]interface{} {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
		return nil
	}

	// Extract first vnext entry from outbound
	var obAddr string
	var obPort int
	var obUUID string
	if outbound.Settings != nil {
		if vnextRaw, ok := outbound.Settings["vnext"]; ok {
			if vlist, ok := vnextRaw.([]interface{}); ok && len(vlist) > 0 {
				if vn := getMap(vlist[0]); vn != nil {
					if a, ok := vn["address"].(string); ok {
						obAddr = a
					}
					switch p := vn["port"].(type) {
					case float64:
						obPort = int(p)
					case int:
						obPort = p
					}
					if usersRaw, ok := vn["users"]; ok {
						if ulist, ok := usersRaw.([]interface{}); ok && len(ulist) > 0 {
							if u := getMap(ulist[0]); u != nil {
								if id, ok := u["id"].(string); ok {
									obUUID = id
								}
							}
						}
					}
				}
			}
		}
	}

	// Extract stream settings key fields
	var obSecurity, obSNI, obPBK, obSID, obFP string
	if outbound.StreamSettings != nil {
		if s, ok := outbound.StreamSettings["security"].(string); ok {
			obSecurity = s
		}
		if obSecurity == "reality" {
			if rs := getMap(outbound.StreamSettings["realitySettings"]); rs != nil {
				if v, ok := rs["serverName"].(string); ok {
					obSNI = v
				}
				if v, ok := rs["publicKey"].(string); ok {
					obPBK = v
				}
				if v, ok := rs["shortId"].(string); ok {
					obSID = v
				}
				if v, ok := rs["fingerprint"].(string); ok {
					obFP = v
				}
			}
		} else if obSecurity == "tls" {
			if ts := getMap(outbound.StreamSettings["tlsSettings"]); ts != nil {
				if v, ok := ts["serverName"].(string); ok {
					obSNI = v
				}
				if v, ok := ts["fingerprint"].(string); ok {
					obFP = v
				}
			}
		}
	}

	// Extract same fields from candidate server
	var svSecurity, svSNI, svPBK, svSID, svFP string
	if server.StreamSettings != nil {
		if s, ok := server.StreamSettings["security"].(string); ok {
			svSecurity = s
		}
		if svSecurity == "reality" {
			if rs := getMap(server.StreamSettings["realitySettings"]); rs != nil {
				if v, ok := rs["serverName"].(string); ok {
					svSNI = v
				}
				if v, ok := rs["publicKey"].(string); ok {
					svPBK = v
				}
				if v, ok := rs["shortId"].(string); ok {
					svSID = v
				}
				if v, ok := rs["fingerprint"].(string); ok {
					svFP = v
				}
			}
		} else if svSecurity == "tls" {
			if ts := getMap(server.StreamSettings["tlsSettings"]); ts != nil {
				if v, ok := ts["serverName"].(string); ok {
					svSNI = v
				}
				if v, ok := ts["fingerprint"].(string); ok {
					svFP = v
				}
			}
		}
	}

	// Compare address/port and UUID when available
	addrMatch := (obAddr == "" || server.Address == "" || equalHost(obAddr, server.Address))
	portMatch := (obPort == 0 || obPort == server.Port)
	uuidMatch := (obUUID == "" || server.UUID == "" || obUUID == server.UUID)

	// Compare stream settings where present
	secMatch := (obSecurity == "" || svSecurity == "" || obSecurity == svSecurity)
	sniMatch := (obSNI == "" || svSNI == "" || obSNI == svSNI)
	pbkMatch := (obPBK == "" || svPBK == "" || obPBK == svPBK)
	sidMatch := (obSID == "" || svSID == "" || obSID == svSID)
	fpMatch := (obFP == "" || svFP == "" || obFP == svFP)

	// Strong match: address+port and at least security-specific keys if set
	strongMatch := addrMatch && portMatch && uuidMatch && secMatch
	if obSecurity == "reality" {
		strongMatch = strongMatch && sniMatch && pbkMatch && sidMatch && fpMatch
	}

	// Fallback: if address differs (e.g., IP vs domain), rely on strong identity params and UUID
	fallbackMatch := uuidMatch && secMatch && sniMatch
	if obSecurity == "reality" {
		// For REALITY consider shortId and publicKey as key identifiers; fingerprint if provided
		fallbackMatch = fallbackMatch && pbkMatch && sidMatch
		if obFP != "" && svFP != "" {
			fallbackMatch = fallbackMatch && fpMatch
		}
	}

	return strongMatch || fallbackMatch
}

// equalHost compares two host identifiers allowing for case-insensitive match; no DNS resolution.
func equalHost(a, b string) bool {
	if a == b {
		return true
	}
	// case-insensitive compare for domains
	if strings.EqualFold(a, b) {
		return true
	}
	return false
}
