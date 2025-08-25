package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the application configuration
type Config struct {
	AdminID             int64  `json:"admin_id"`
	BotToken            string `json:"bot_token"`
	ConfigPath          string `json:"config_path"`
	SubscriptionURL     string `json:"subscription_url"`
	LogLevel            string `json:"log_level"`
	XrayRestartCommand  string `json:"xray_restart_command"`
	CacheDuration       int    `json:"cache_duration"`
	HealthCheckInterval int    `json:"health_check_interval"`
	PingTimeout         int    `json:"ping_timeout"`
}

// LoadConfig loads configuration from the specified file path
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	config.SetDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// SetDefaults sets default values for optional configuration fields
func (c *Config) SetDefaults() {
	if c.ConfigPath == "" {
		c.ConfigPath = "/opt/etc/xray/configs/04_outbounds.json"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.XrayRestartCommand == "" {
		c.XrayRestartCommand = "/opt/etc/init.d/S24xray restart"
	}
	if c.CacheDuration == 0 {
		c.CacheDuration = 3600 // 1 hour
	}
	if c.HealthCheckInterval == 0 {
		c.HealthCheckInterval = 300 // 5 minutes
	}
	if c.PingTimeout == 0 {
		c.PingTimeout = 5 // 5 seconds
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.AdminID == 0 {
		return fmt.Errorf("admin_id is required")
	}
	if c.BotToken == "" {
		return fmt.Errorf("bot_token is required")
	}
	if c.SubscriptionURL == "" {
		return fmt.Errorf("subscription_url is required")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log_level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate timeout values
	if c.CacheDuration < 0 {
		return fmt.Errorf("cache_duration must be non-negative")
	}
	if c.HealthCheckInterval < 0 {
		return fmt.Errorf("health_check_interval must be non-negative")
	}
	if c.PingTimeout <= 0 {
		return fmt.Errorf("ping_timeout must be positive")
	}

	return nil
}

// CreateTemplate creates a template configuration file at the specified path
func CreateTemplate(path string) error {
	template := Config{
		AdminID:             0, // User must fill this
		BotToken:            "your_bot_token_here",
		ConfigPath:          "/opt/etc/xray/configs/04_outbounds.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "info",
		XrayRestartCommand:  "/opt/etc/init.d/S24xray restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         5,
	}

	data, err := json.MarshalIndent(template, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal template config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write template config file: %w", err)
	}

	return nil
}

// LoadConfigOrCreateTemplate loads configuration from file, or creates a template if file doesn't exist
func LoadConfigOrCreateTemplate(path string) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create template config file
		if err := CreateTemplate(path); err != nil {
			return nil, fmt.Errorf("failed to create template config: %w", err)
		}
		return nil, fmt.Errorf("config file not found, created template at %s - please fill in required fields (admin_id, bot_token, subscription_url)", path)
	}

	// Load existing config
	return LoadConfig(path)
}

// GetAdminID returns the admin ID for the ConfigProvider interface
func (c *Config) GetAdminID() int64 {
	return c.AdminID
}

// GetBotToken returns the bot token for the ConfigProvider interface
func (c *Config) GetBotToken() string {
	return c.BotToken
}
