package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/logger"
	"xray-telegram-manager/server"
	"xray-telegram-manager/telegram"
	"xray-telegram-manager/types"
)

// Service represents the main application service
type Service struct {
	config          *config.Config
	logger          *logger.Logger
	bot             *telegram.TelegramBot
	serverMgr       *server.ServerManager
	ctx             context.Context
	cancel          context.CancelFunc
	running         bool
	mutex           sync.RWMutex
	healthTicker    *time.Ticker
	lastHealthCheck time.Time
	healthStatus    map[string]interface{}
}

// NewService creates a new service instance
func NewService(configPath string) (*Service, error) {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	logLevel := logger.ParseLogLevel(cfg.LogLevel)
	log := logger.NewLogger(logLevel, os.Stdout)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Create server manager (all components are now initialized internally)
	serverMgr := server.NewServerManager(cfg)

	// Initialize Telegram bot
	bot, err := telegram.NewTelegramBot(&configAdapter{cfg}, serverMgr, log)
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

// Start starts the service
func (s *Service) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("service is already running")
	}

	s.logger.Info("Starting xray-telegram-manager service")

	// Initialize server manager by loading servers
	s.logger.Info("Loading servers from subscription...")
	if err := s.serverMgr.LoadServers(); err != nil {
		s.logger.Warn("Failed to load servers on startup: %v", err)
		s.logger.Info("Service will continue, servers can be loaded later via Telegram commands")
	} else {
		servers := s.serverMgr.GetServers()
		s.logger.Info("Successfully loaded %d servers", len(servers))

		// Try to detect current server from xray config
		if err := s.serverMgr.DetectCurrentServer(); err != nil {
			s.logger.Debug("Could not detect current server: %v", err)
		} else {
			currentServer := s.serverMgr.GetCurrentServer()
			if currentServer != nil {
				s.logger.Info("Detected current server: %s", currentServer.Name)
			}
		}
	}

	// Start Telegram bot
	s.logger.Info("Starting Telegram bot...")
	go func() {
		if err := s.bot.Start(s.ctx); err != nil {
			s.logger.Error("Telegram bot error: %v", err)
		}
	}()

	// Start health monitoring if configured
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

// Stop stops the service gracefully
func (s *Service) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("service is not running")
	}

	s.logger.Info("Stopping xray-telegram-manager service")

	// Stop health monitoring
	if s.healthTicker != nil {
		s.logger.Info("Stopping health monitoring...")
		s.healthTicker.Stop()
		s.healthTicker = nil
	}

	// Cancel context to stop all components
	s.cancel()

	// Stop Telegram bot
	s.logger.Info("Stopping Telegram bot...")
	s.bot.Stop()

	// Give components time to shutdown gracefully
	time.Sleep(1 * time.Second)

	s.running = false
	s.logger.Info("Service stopped successfully")
	return nil
}

// Reload reloads the service configuration
func (s *Service) Reload() error {
	s.mutex.RLock()
	if !s.running {
		s.mutex.RUnlock()
		return fmt.Errorf("service is not running")
	}
	s.mutex.RUnlock()

	s.logger.Info("Reloading service configuration")

	// Reload configuration from file
	configPath := "/opt/etc/xray-manager/config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	newConfig, err := config.LoadConfig(configPath)
	if err != nil {
		s.logger.Error("Failed to reload configuration: %v", err)
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	// Update configuration
	s.mutex.Lock()
	s.config = newConfig
	s.mutex.Unlock()

	// Update logger level if changed
	logLevel := logger.ParseLogLevel(newConfig.LogLevel)
	s.logger = logger.NewLogger(logLevel, os.Stdout)

	// Refresh servers with new configuration
	if err := s.serverMgr.LoadServers(); err != nil {
		s.logger.Warn("Failed to reload servers: %v", err)
	} else {
		servers := s.serverMgr.GetServers()
		s.logger.Info("Successfully reloaded %d servers", len(servers))
	}

	s.logger.Info("Service configuration reloaded successfully")
	return nil
}

// IsRunning returns whether the service is currently running
func (s *Service) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// GetStatus returns the current service status
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

		// Include health monitoring information
		if s.config.HealthCheckInterval > 0 {
			status["health_monitoring"] = map[string]interface{}{
				"enabled":          true,
				"interval_seconds": s.config.HealthCheckInterval,
				"last_check":       s.lastHealthCheck.Unix(),
				"last_check_human": s.lastHealthCheck.Format("2006-01-02 15:04:05"),
			}

			// Include latest health status if available
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

// Restart restarts the service
func (s *Service) Restart() error {
	s.logger.Info("Restarting service")

	if s.IsRunning() {
		if err := s.Stop(); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
	}

	// Brief pause before restart
	time.Sleep(2 * time.Second)

	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// startHealthMonitoring starts the health monitoring goroutine
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

	// Perform initial health check
	go s.performHealthCheck()
}

// performHealthCheck performs a comprehensive health check
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

	// Check service running status
	checks["service_running"] = map[string]interface{}{
		"status":  s.running,
		"healthy": s.running,
	}

	// Check server manager
	serverCheck := s.checkServerManager()
	checks["server_manager"] = serverCheck
	if !serverCheck["healthy"].(bool) {
		healthStatus["status"] = "degraded"
	}

	// Check current server connectivity if available
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

	// Check configuration validity
	configCheck := s.checkConfiguration()
	checks["configuration"] = configCheck
	if !configCheck["healthy"].(bool) {
		healthStatus["status"] = "unhealthy"
	}

	s.healthStatus = healthStatus

	// Log health status
	status := healthStatus["status"].(string)
	switch status {
	case "healthy":
		s.logger.Debug("Health check completed: %s", status)
	case "degraded":
		s.logger.Warn("Health check completed: %s", status)
	case "unhealthy":
		s.logger.Error("Health check completed: %s", status)
	}

	// Handle automatic restart if configured and service is unhealthy
	if status == "unhealthy" && s.shouldAutoRestart() {
		s.logger.Warn("Service is unhealthy, attempting automatic restart...")
		go s.attemptAutoRestart()
	}
}

// checkServerManager checks the health of the server manager
func (s *Service) checkServerManager() map[string]interface{} {
	result := map[string]interface{}{
		"healthy": true,
		"status":  "ok",
	}

	// Check if servers are loaded
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

// checkCurrentServerConnectivity checks connectivity to the current server
func (s *Service) checkCurrentServerConnectivity(srv types.Server) map[string]interface{} {
	result := map[string]interface{}{
		"server_id":   srv.ID,
		"server_name": srv.Name,
		"healthy":     true,
		"status":      "connected",
	}

	// Test connectivity using ping tester - we need to create a new instance
	// since we can't access the server manager's internal ping tester
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

// checkConfiguration validates the current configuration
func (s *Service) checkConfiguration() map[string]interface{} {
	result := map[string]interface{}{
		"healthy": true,
		"status":  "valid",
	}

	// Validate configuration
	if err := s.config.Validate(); err != nil {
		result["healthy"] = false
		result["status"] = "invalid"
		result["error"] = err.Error()
		result["message"] = fmt.Sprintf("Configuration validation failed: %s", err.Error())
		return result
	}

	result["message"] = "Configuration is valid"
	return result
}

// shouldAutoRestart determines if the service should attempt an automatic restart
func (s *Service) shouldAutoRestart() bool {
	// For now, we'll be conservative and not auto-restart
	// This can be made configurable in the future
	return false
}

// attemptAutoRestart attempts to restart the service automatically
func (s *Service) attemptAutoRestart() {
	s.logger.Info("Attempting automatic service restart...")

	if err := s.Restart(); err != nil {
		s.logger.Error("Automatic restart failed: %v", err)
	} else {
		s.logger.Info("Automatic restart completed successfully")
	}
}

// GetHealthStatus returns the current health status
func (s *Service) GetHealthStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]interface{})
	for k, v := range s.healthStatus {
		result[k] = v
	}

	return result
}

// GetLastHealthCheck returns the timestamp of the last health check
func (s *Service) GetLastHealthCheck() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastHealthCheck
}

// configAdapter adapts config.Config to various interface requirements
type configAdapter struct {
	*config.Config
}

func (ca *configAdapter) GetAdminID() int64 {
	return ca.AdminID
}

func (ca *configAdapter) GetBotToken() string {
	return ca.BotToken
}

func (ca *configAdapter) GetSubscriptionURL() string {
	return ca.SubscriptionURL
}

func (ca *configAdapter) GetConfigPath() string {
	return ca.ConfigPath
}

func (ca *configAdapter) GetCacheDuration() int {
	return ca.CacheDuration
}

func (ca *configAdapter) GetPingTimeout() int {
	return ca.PingTimeout
}

func (ca *configAdapter) GetXrayRestartCommand() string {
	return ca.XrayRestartCommand
}

func main() {
	// Default config path for Keenetic
	configPath := "/opt/etc/xray-manager/config.json"

	// Allow override via command line argument
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Create service
	service, err := NewService(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service: %v\n", err)
		os.Exit(1)
	}

	// Start service
	if err := service.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start service: %v\n", err)
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Handle signals
	for {
		sig := <-sigChan

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// Graceful shutdown
			service.logger.Info("Received shutdown signal (%v), shutting down gracefully...", sig)
			if err := service.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
				os.Exit(1)
			}
			service.logger.Info("Service shutdown completed")
			return

		case syscall.SIGHUP:
			// Reload configuration
			service.logger.Info("Received reload signal (SIGHUP), reloading configuration...")
			if err := service.Reload(); err != nil {
				service.logger.Error("Failed to reload configuration: %v", err)
			}
		}
	}
}
