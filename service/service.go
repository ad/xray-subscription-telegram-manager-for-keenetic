package service
import (
	"context"
	"fmt"
	"sync"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/interfaces"
	"xray-telegram-manager/server"
	"xray-telegram-manager/telegram"
	"xray-telegram-manager/types"
)
type Service struct {
	config          *config.Config
	logger          interfaces.Logger
	bot             interfaces.TelegramBot
	serverMgr       interfaces.ServerManager
	ctx             context.Context
	cancel          context.CancelFunc
	running         bool
	mutex           sync.RWMutex
	healthTicker    *time.Ticker
	lastHealthCheck time.Time
	healthStatus    map[string]interface{}
}
func NewService(cfg *config.Config, log interfaces.Logger) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	serverMgr := server.NewServerManager(cfg)
	bot, err := telegram.NewTelegramBot(cfg, serverMgr, log)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}
	return &Service{
		config:          cfg,
		logger:          log,
		bot:             bot,
		serverMgr:       serverMgr,
		ctx:             ctx,
		cancel:          cancel,
		running:         false,
		mutex:           sync.RWMutex{},
		healthTicker:    nil,
		lastHealthCheck: time.Time{},
		healthStatus:    make(map[string]interface{}),
	}, nil
}
func (s *Service) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.running {
		return fmt.Errorf("service is already running")
	}
	s.logger.Info("Starting xray-telegram-manager service")
	s.logger.Info("Loading servers from subscription...")
	if err := s.serverMgr.LoadServers(); err != nil {
		s.logger.Warn("Failed to load servers on startup: %v", err)
		s.logger.Info("Service will continue, servers can be loaded later via Telegram commands")
	} else {
		servers := s.serverMgr.GetServers()
		s.logger.Info("Successfully loaded %d servers", len(servers))
		if err := s.serverMgr.DetectCurrentServer(); err != nil {
			s.logger.Debug("Could not detect current server: %v", err)
		} else {
			currentServer := s.serverMgr.GetCurrentServer()
			if currentServer != nil {
				s.logger.Info("Detected current server: %s", currentServer.Name)
			}
		}
	}
	s.logger.Info("Starting Telegram bot...")
	go func() {
		if err := s.bot.Start(s.ctx); err != nil {
			s.logger.Error("Telegram bot error: %v", err)
		}
	}()
	if s.config.HealthCheckInterval > 0 {
		s.logger.Info("Starting health monitoring (interval: %d seconds)", s.config.HealthCheckInterval)
		s.startHealthMonitoring()
	} else {
		s.logger.Info("Health monitoring disabled (interval: 0)")
	}
	s.running = true
	s.logger.Info("Service started successfully")
	return nil
}
func (s *Service) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !s.running {
		return fmt.Errorf("service is not running")
	}
	s.logger.Info("Stopping xray-telegram-manager service")
	if s.healthTicker != nil {
		s.logger.Info("Stopping health monitoring...")
		s.healthTicker.Stop()
		s.healthTicker = nil
	}
	s.cancel()
	s.logger.Info("Stopping Telegram bot...")
	s.bot.Stop()
	time.Sleep(1 * time.Second)
	s.running = false
	s.logger.Info("Service stopped successfully")
	return nil
}
func (s *Service) Restart() error {
	s.logger.Info("Restarting service")
	if s.IsRunning() {
		if err := s.Stop(); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
	}
	time.Sleep(2 * time.Second)
	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	return nil
}
func (s *Service) Reload() error {
	s.mutex.RLock()
	if !s.running {
		s.mutex.RUnlock()
		return fmt.Errorf("service is not running")
	}
	s.mutex.RUnlock()
	s.logger.Info("Reloading service configuration")
	if err := s.serverMgr.RefreshServers(); err != nil {
		s.logger.Warn("Failed to refresh servers: %v", err)
	} else {
		servers := s.serverMgr.GetServers()
		s.logger.Info("Successfully refreshed %d servers", len(servers))
	}
	s.logger.Info("Service configuration reloaded successfully")
	return nil
}
func (s *Service) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}
func (s *Service) GetStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	status := map[string]interface{}{
		"running":     s.running,
		"config_path": s.config.ConfigPath,
		"log_level":   s.config.LogLevel,
		"admin_id":    s.config.AdminID,
	}
	if s.running {
		servers := s.serverMgr.GetServers()
		currentServer := s.serverMgr.GetCurrentServer()
		status["servers_count"] = len(servers)
		if currentServer != nil {
			status["current_server"] = map[string]interface{}{
				"id":   currentServer.ID,
				"name": currentServer.Name,
			}
		}
		if s.config.HealthCheckInterval > 0 {
			status["health_monitoring"] = map[string]interface{}{
				"enabled":          true,
				"interval_seconds": s.config.HealthCheckInterval,
				"last_check":       s.lastHealthCheck.Unix(),
				"last_check_human": s.lastHealthCheck.Format("2006-01-02 15:04:05"),
			}
			if len(s.healthStatus) > 0 {
				status["health_status"] = s.healthStatus["status"]
				status["health_details"] = s.healthStatus
			}
		} else {
			status["health_monitoring"] = map[string]interface{}{
				"enabled": false,
			}
		}
	}
	return status
}
func (s *Service) startHealthMonitoring() {
	interval := time.Duration(s.config.HealthCheckInterval) * time.Second
	s.healthTicker = time.NewTicker(interval)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("Health monitoring goroutine panicked: %v", r)
			}
		}()
		for {
			select {
			case <-s.ctx.Done():
				s.logger.Debug("Health monitoring stopped due to context cancellation")
				return
			case <-s.healthTicker.C:
				s.performHealthCheck()
			}
		}
	}()
	go s.performHealthCheck()
}
func (s *Service) performHealthCheck() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.logger.Debug("Performing health check...")
	s.lastHealthCheck = time.Now()
	healthStatus := map[string]interface{}{
		"timestamp": s.lastHealthCheck.Unix(),
		"status":    "healthy",
		"checks":    make(map[string]interface{}),
	}
	checks := healthStatus["checks"].(map[string]interface{})
	checks["service_running"] = map[string]interface{}{
		"status":  s.running,
		"healthy": s.running,
	}
	serverCheck := s.checkServerManager()
	checks["server_manager"] = serverCheck
	if !serverCheck["healthy"].(bool) {
		healthStatus["status"] = "degraded"
	}
	currentServer := s.serverMgr.GetCurrentServer()
	if currentServer != nil {
		connectivityCheck := s.checkCurrentServerConnectivity(*currentServer)
		checks["current_server_connectivity"] = connectivityCheck
		if !connectivityCheck["healthy"].(bool) {
			healthStatus["status"] = "degraded"
		}
	} else {
		checks["current_server_connectivity"] = map[string]interface{}{
			"status":  "no_server_selected",
			"healthy": true, // Not having a server selected is not unhealthy
		}
	}
	s.healthStatus = healthStatus
	status := healthStatus["status"].(string)
	switch status {
	case "healthy":
		s.logger.Debug("Health check completed: %s", status)
	case "degraded":
		s.logger.Warn("Health check completed: %s", status)
	case "unhealthy":
		s.logger.Error("Health check completed: %s", status)
	}
}
func (s *Service) checkServerManager() map[string]interface{} {
	result := map[string]interface{}{
		"healthy": true,
		"status":  "ok",
	}
	servers := s.serverMgr.GetServers()
	result["servers_count"] = len(servers)
	if len(servers) == 0 {
		result["healthy"] = false
		result["status"] = "no_servers_loaded"
		result["message"] = "No servers are currently loaded"
		return result
	}
	result["message"] = fmt.Sprintf("%d servers loaded", len(servers))
	return result
}
func (s *Service) checkCurrentServerConnectivity(srv types.Server) map[string]interface{} {
	result := map[string]interface{}{
		"server_id":   srv.ID,
		"server_name": srv.Name,
		"healthy":     true,
		"status":      "connected",
	}
	pingTester := server.NewPingTester(s.config)
	pingResult := pingTester.TestServer(srv)
	if pingResult.Available {
		result["latency_ms"] = pingResult.Latency
		result["message"] = fmt.Sprintf("Server responsive (latency: %dms)", pingResult.Latency)
	} else {
		result["healthy"] = false
		result["status"] = "disconnected"
		result["error"] = pingResult.Error.Error()
		result["message"] = fmt.Sprintf("Server not responsive: %s", pingResult.Error.Error())
	}
	return result
}
func (s *Service) GetHealthStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	result := make(map[string]interface{})
	for k, v := range s.healthStatus {
		result[k] = v
	}
	return result
}
func (s *Service) GetLastHealthCheck() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastHealthCheck
}
