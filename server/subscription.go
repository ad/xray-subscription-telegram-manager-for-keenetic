package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)

// SubscriptionLoaderImpl handles loading and caching of server configurations
type SubscriptionLoaderImpl struct {
	config     *config.Config
	httpClient *http.Client
	cache      []types.Server
	lastUpdate time.Time
	mutex      sync.RWMutex
	parser     *VlessParser
	cacheFile  string
}

// NewSubscriptionLoader creates a new subscription loader instance
func NewSubscriptionLoader(cfg *config.Config) *SubscriptionLoaderImpl {
	return &SubscriptionLoaderImpl{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.PingTimeout) * time.Second,
		},
		parser:    NewVlessParser(),
		cacheFile: "/opt/etc/xray-manager/cache/servers.json",
	}
}

// NewSubscriptionLoaderWithCacheDir creates a new subscription loader with custom cache directory
func NewSubscriptionLoaderWithCacheDir(cfg *config.Config, cacheDir string) *SubscriptionLoaderImpl {
	return &SubscriptionLoaderImpl{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.PingTimeout) * time.Second,
		},
		parser:    NewVlessParser(),
		cacheFile: filepath.Join(cacheDir, "servers.json"),
	}
}

// LoadFromURL fetches server configuration from the subscription URL
func (sl *SubscriptionLoaderImpl) LoadFromURL() ([]types.Server, error) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// Check if cache is still valid
	if sl.isCacheValid() && len(sl.cache) > 0 {
		return sl.cache, nil
	}

	// Fetch data from URL with retry logic
	var data string
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		data, err = sl.fetchFromURL()
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
		}
	}

	if err != nil {
		// Try to load from cache file as fallback
		if cachedServers, cacheErr := sl.loadFromCacheFile(); cacheErr == nil {
			sl.cache = cachedServers
			return cachedServers, nil
		}
		return nil, fmt.Errorf("failed to fetch from URL after %d retries and no valid cache: %w", maxRetries, err)
	}

	// Decode and parse the configuration
	servers, err := sl.DecodeBase64Config(data)
	if err != nil {
		// Try to load from cache file as fallback
		if cachedServers, cacheErr := sl.loadFromCacheFile(); cacheErr == nil {
			sl.cache = cachedServers
			return cachedServers, nil
		}
		return nil, fmt.Errorf("failed to decode configuration: %w", err)
	}

	// Update cache
	sl.cache = servers
	sl.lastUpdate = time.Now()

	// Save to cache file
	if err := sl.saveToCacheFile(servers); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to save cache file: %v\n", err)
	}

	return servers, nil
}

// fetchFromURL performs the HTTP request to fetch subscription data
func (sl *SubscriptionLoaderImpl) fetchFromURL() (string, error) {
	resp, err := sl.httpClient.Get(sl.config.SubscriptionURL)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// DecodeBase64Config decodes base64 data and extracts VLESS URLs
func (sl *SubscriptionLoaderImpl) DecodeBase64Config(data string) ([]types.Server, error) {
	// Clean the data (remove whitespace)
	data = strings.TrimSpace(data)

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		// Try URL-safe base64 decoding
		decoded, err = base64.URLEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 data: %w", err)
		}
	}

	// Split into lines and extract VLESS URLs
	lines := strings.Split(string(decoded), "\n")
	var vlessUrls []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "vless://") {
			vlessUrls = append(vlessUrls, line)
		}
	}

	if len(vlessUrls) == 0 {
		return nil, fmt.Errorf("no VLESS URLs found in decoded data")
	}

	// Parse VLESS URLs into Server structs
	return sl.ParseVlessUrls(vlessUrls)
}

// ParseVlessUrls converts VLESS URL strings to Server structs
func (sl *SubscriptionLoaderImpl) ParseVlessUrls(urls []string) ([]types.Server, error) {
	var servers []types.Server
	var errors []string

	for i, vlessUrl := range urls {
		server, err := sl.ParseVlessUrl(vlessUrl)
		if err != nil {
			errors = append(errors, fmt.Sprintf("URL %d: %v", i+1, err))
			continue
		}
		servers = append(servers, server)
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("failed to parse any VLESS URLs: %s", strings.Join(errors, "; "))
	}

	// Log parsing errors but don't fail if we have some valid servers
	if len(errors) > 0 {
		fmt.Printf("Warning: some VLESS URLs failed to parse: %s\n", strings.Join(errors, "; "))
	}

	return servers, nil
}

// ParseVlessUrl parses a single VLESS URL into a Server struct
func (sl *SubscriptionLoaderImpl) ParseVlessUrl(vlessUrl string) (types.Server, error) {
	// Parse VLESS URL using the parser
	vlessConfig, err := sl.parser.ParseUrl(vlessUrl)
	if err != nil {
		return types.Server{}, fmt.Errorf("failed to parse VLESS URL: %w", err)
	}

	// Convert to Server struct
	server, err := sl.parser.ToXrayOutbound(vlessConfig)
	if err != nil {
		return types.Server{}, fmt.Errorf("failed to convert to xray outbound: %w", err)
	}

	// Set the original VLESS URL
	server.VlessUrl = vlessUrl

	return server, nil
}

// GetCachedServers returns the cached servers without fetching from URL
func (sl *SubscriptionLoaderImpl) GetCachedServers() []types.Server {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make([]types.Server, len(sl.cache))
	copy(result, sl.cache)
	return result
}

// isCacheValid checks if the current cache is still valid based on cache duration
func (sl *SubscriptionLoaderImpl) isCacheValid() bool {
	if sl.lastUpdate.IsZero() {
		return false
	}

	cacheDuration := time.Duration(sl.config.CacheDuration) * time.Second
	return time.Since(sl.lastUpdate) < cacheDuration
}

// saveToCacheFile saves servers to the cache file
func (sl *SubscriptionLoaderImpl) saveToCacheFile(servers []types.Server) error {
	// Create cache directory if it doesn't exist
	cacheDir := filepath.Dir(sl.cacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Marshal servers to JSON
	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal servers: %w", err)
	}

	// Write to file
	if err := os.WriteFile(sl.cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// loadFromCacheFile loads servers from the cache file
func (sl *SubscriptionLoaderImpl) loadFromCacheFile() ([]types.Server, error) {
	data, err := os.ReadFile(sl.cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var servers []types.Server
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache file: %w", err)
	}

	return servers, nil
}

// InvalidateCache forces the cache to be refreshed on next load
func (sl *SubscriptionLoaderImpl) InvalidateCache() {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	sl.lastUpdate = time.Time{}
	sl.cache = nil
}
