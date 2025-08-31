package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Config struct {
	AdminID             int64        `json:"admin_id"`
	BotToken            string       `json:"bot_token"`
	ConfigPath          string       `json:"config_path"`
	SubscriptionURL     string       `json:"subscription_url"`
	LogLevel            string       `json:"log_level"`
	XrayRestartCommand  string       `json:"xray_restart_command"`
	CacheDuration       int          `json:"cache_duration"`
	HealthCheckInterval int          `json:"health_check_interval"`
	PingTimeout         int          `json:"ping_timeout"`
	UI                  UIConfig     `json:"ui"`
	Update              UpdateConfig `json:"update"`
}

type UIConfig struct {
	MaxButtonTextLength       int     `json:"max_button_text_length"`
	ServersPerPage            int     `json:"servers_per_page"`
	MaxQuickSelectServers     int     `json:"max_quick_select_servers"`
	MessageTimeoutMinutes     int     `json:"message_timeout_minutes"`
	EnableNameOptimization    bool    `json:"enable_name_optimization"`
	NameOptimizationThreshold float64 `json:"name_optimization_threshold"`
}

type UpdateConfig struct {
	ScriptURL      string `json:"script_url"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	BackupConfig   bool   `json:"backup_config"`
}

func LoadConfig(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

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
		c.CacheDuration = 3600
	}
	if c.HealthCheckInterval == 0 {
		c.HealthCheckInterval = 300
	}
	if c.PingTimeout == 0 {
		c.PingTimeout = 5
	}

	// UI defaults
	if c.UI.MaxButtonTextLength == 0 {
		c.UI.MaxButtonTextLength = 50
	}
	if c.UI.ServersPerPage == 0 {
		c.UI.ServersPerPage = 32
	}
	if c.UI.MaxQuickSelectServers == 0 {
		c.UI.MaxQuickSelectServers = 10
	}
	if c.UI.MessageTimeoutMinutes == 0 {
		c.UI.MessageTimeoutMinutes = 60
	}
	if c.UI.NameOptimizationThreshold == 0 {
		c.UI.NameOptimizationThreshold = 0.7
		c.UI.EnableNameOptimization = true
	}

	// Update defaults
	if c.Update.ScriptURL == "" {
		c.Update.ScriptURL = "https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/update.sh"
	}
	if c.Update.TimeoutMinutes == 0 {
		c.Update.TimeoutMinutes = 10
	}
	// BackupConfig defaults to false (zero value)
}

func (c *Config) Validate() error {
	if c.AdminID == 0 {
		return fmt.Errorf("admin_id is required and must be non-zero")
	}

	if c.AdminID < 0 {
		return fmt.Errorf("admin_id must be positive")
	}

	if err := c.validateBotToken(); err != nil {
		return fmt.Errorf("invalid bot_token: %w", err)
	}

	if err := c.validateSubscriptionURL(); err != nil {
		return fmt.Errorf("invalid subscription_url: %w", err)
	}

	if err := c.validateConfigPath(); err != nil {
		return fmt.Errorf("invalid config_path: %w", err)
	}

	if err := c.validateLogLevel(); err != nil {
		return fmt.Errorf("invalid log_level: %w", err)
	}

	if err := c.validateTimeouts(); err != nil {
		return fmt.Errorf("invalid timeout values: %w", err)
	}

	if err := c.validateCommand(); err != nil {
		return fmt.Errorf("invalid xray_restart_command: %w", err)
	}

	if err := c.validateUI(); err != nil {
		return fmt.Errorf("invalid UI configuration: %w", err)
	}

	if err := c.validateUpdate(); err != nil {
		return fmt.Errorf("invalid Update configuration: %w", err)
	}

	return nil
}

func (c *Config) validateBotToken() error {
	if c.BotToken == "" {
		return fmt.Errorf("bot_token is required")
	}

	botTokenRegex := regexp.MustCompile(`^\d{8,10}:[A-Za-z0-9_-]{20,}$`)
	if !botTokenRegex.MatchString(c.BotToken) {
		return fmt.Errorf("bot_token has invalid format")
	}

	return nil
}

func (c *Config) validateSubscriptionURL() error {
	if c.SubscriptionURL == "" {
		return fmt.Errorf("subscription_url is required")
	}

	parsedURL, err := url.Parse(c.SubscriptionURL)
	if err != nil {
		return fmt.Errorf("subscription_url is not a valid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("subscription_url must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("subscription_url must have a valid host")
	}

	return nil
}

func (c *Config) validateConfigPath() error {
	if c.ConfigPath == "" {
		c.ConfigPath = "/opt/etc/xray/configs/04_outbounds.json"
		return nil
	}

	if !strings.HasPrefix(c.ConfigPath, "/") {
		return fmt.Errorf("config_path must be an absolute path")
	}

	if strings.Contains(c.ConfigPath, "..") {
		return fmt.Errorf("config_path cannot contain '..' path components")
	}

	return nil
}

func (c *Config) validateLogLevel() error {
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLogLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("log_level must be one of: debug, info, warn, error")
	}

	c.LogLevel = strings.ToLower(c.LogLevel)
	return nil
}

func (c *Config) validateTimeouts() error {
	if c.CacheDuration < 0 {
		return fmt.Errorf("cache_duration must be non-negative")
	}

	if c.HealthCheckInterval < 0 {
		return fmt.Errorf("health_check_interval must be non-negative")
	}

	if c.PingTimeout <= 0 {
		return fmt.Errorf("ping_timeout must be positive")
	}

	if c.PingTimeout > 60 {
		return fmt.Errorf("ping_timeout cannot exceed 60 seconds")
	}

	if c.CacheDuration > 86400 {
		return fmt.Errorf("cache_duration cannot exceed 24 hours (86400 seconds)")
	}

	if c.HealthCheckInterval > 3600 {
		return fmt.Errorf("health_check_interval cannot exceed 1 hour (3600 seconds)")
	}

	return nil
}

func (c *Config) validateCommand() error {
	if c.XrayRestartCommand == "" {
		c.XrayRestartCommand = "/opt/etc/init.d/S24xray restart"
		return nil
	}

	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\"", "'", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(c.XrayRestartCommand, char) {
			return fmt.Errorf("xray_restart_command contains potentially dangerous character: %s", char)
		}
	}

	parts := strings.Fields(c.XrayRestartCommand)
	if len(parts) == 0 {
		return fmt.Errorf("xray_restart_command cannot be empty")
	}

	if !strings.HasPrefix(parts[0], "/") {
		return fmt.Errorf("xray_restart_command must start with an absolute path")
	}

	if len(c.XrayRestartCommand) > 256 {
		return fmt.Errorf("xray_restart_command too long (max 256 characters)")
	}

	allowedCommands := []string{
		"/opt/etc/init.d/S24xray",
		"/bin/systemctl",
		"/usr/bin/systemctl",
		"/sbin/service",
		"/usr/sbin/service",
		"/etc/init.d/xray",
		"/bin/echo",
		"/usr/bin/echo",
	}

	commandAllowed := false
	for _, allowed := range allowedCommands {
		if strings.HasPrefix(parts[0], allowed) {
			commandAllowed = true
			break
		}
	}

	if !commandAllowed {
		return fmt.Errorf("xray_restart_command uses non-whitelisted command: %s", parts[0])
	}

	return nil
}

func CreateTemplate(path string) error {
	template := Config{
		AdminID:             0,
		BotToken:            "your_bot_token_here",
		ConfigPath:          "/opt/etc/xray/configs/04_outbounds.json",
		SubscriptionURL:     "https://example.com/config.txt",
		LogLevel:            "info",
		XrayRestartCommand:  "/opt/etc/init.d/S24xray restart",
		CacheDuration:       3600,
		HealthCheckInterval: 300,
		PingTimeout:         5,
		UI: UIConfig{
			MaxButtonTextLength:       50,
			ServersPerPage:            32,
			MaxQuickSelectServers:     10,
			MessageTimeoutMinutes:     60,
			EnableNameOptimization:    true,
			NameOptimizationThreshold: 0.7,
		},
		Update: UpdateConfig{
			ScriptURL:      "https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/update.sh",
			TimeoutMinutes: 10,
			BackupConfig:   false,
		},
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

func LoadConfigOrCreateTemplate(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := CreateTemplate(path); err != nil {
			return nil, fmt.Errorf("failed to create template config: %w", err)
		}
		return nil, fmt.Errorf("config file not found, created template at %s - please fill in required fields (admin_id, bot_token, subscription_url)", path)
	}

	return LoadConfig(path)
}

func (c *Config) GetAdminID() int64 {
	return c.AdminID
}

func (c *Config) GetBotToken() string {
	return c.BotToken
}

func (c *Config) GetUpdateConfig() UpdateConfig {
	return c.Update
}

func (c *Config) GetUIConfig() UIConfig {
	return c.UI
}

func (c *Config) GetMaxButtonTextLength() int {
	return c.UI.MaxButtonTextLength
}

func (c *Config) GetServersPerPage() int {
	return c.UI.ServersPerPage
}

func (c *Config) GetMaxQuickSelectServers() int {
	return c.UI.MaxQuickSelectServers
}

func (c *Config) GetMessageTimeoutMinutes() int {
	return c.UI.MessageTimeoutMinutes
}

func (c *Config) IsNameOptimizationEnabled() bool {
	return c.UI.EnableNameOptimization
}

func (c *Config) GetNameOptimizationThreshold() float64 {
	return c.UI.NameOptimizationThreshold
}

func (c *Config) validateUI() error {
	if c.UI.MaxButtonTextLength <= 0 {
		return fmt.Errorf("max_button_text_length must be positive")
	}
	if c.UI.MaxButtonTextLength > 200 {
		return fmt.Errorf("max_button_text_length cannot exceed 200 characters")
	}

	if c.UI.ServersPerPage <= 0 {
		return fmt.Errorf("servers_per_page must be positive")
	}
	if c.UI.ServersPerPage > 100 {
		return fmt.Errorf("servers_per_page cannot exceed 100")
	}

	if c.UI.MaxQuickSelectServers <= 0 {
		return fmt.Errorf("max_quick_select_servers must be positive")
	}
	if c.UI.MaxQuickSelectServers > 50 {
		return fmt.Errorf("max_quick_select_servers cannot exceed 50")
	}

	if c.UI.MessageTimeoutMinutes <= 0 {
		return fmt.Errorf("message_timeout_minutes must be positive")
	}
	if c.UI.MessageTimeoutMinutes > 1440 { // 24 hours
		return fmt.Errorf("message_timeout_minutes cannot exceed 1440 minutes (24 hours)")
	}

	if c.UI.NameOptimizationThreshold < 0 || c.UI.NameOptimizationThreshold > 1 {
		return fmt.Errorf("name_optimization_threshold must be between 0 and 1")
	}

	return nil
}

func (c *Config) validateUpdate() error {
	if c.Update.ScriptURL == "" {
		return fmt.Errorf("update script_url is required")
	}

	parsedURL, err := url.Parse(c.Update.ScriptURL)
	if err != nil {
		return fmt.Errorf("update script_url is not a valid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("update script_url must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("update script_url must have a valid host")
	}

	if c.Update.TimeoutMinutes <= 0 {
		return fmt.Errorf("update timeout_minutes must be positive")
	}

	if c.Update.TimeoutMinutes > 60 {
		return fmt.Errorf("update timeout_minutes cannot exceed 60 minutes")
	}

	return nil
}
