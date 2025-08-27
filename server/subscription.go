package server
import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)
type SubscriptionLoaderImpl struct {
	config     *config.Config
	httpClient *http.Client
	cache      []types.Server
	lastUpdate time.Time
	mutex      sync.RWMutex
	parser     *VlessParser
	cacheFile  string
}
func NewSubscriptionLoader(cfg *config.Config) *SubscriptionLoaderImpl {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.PingTimeout) * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			ResponseHeaderTimeout: 15 * time.Second,
		},
	}
	return &SubscriptionLoaderImpl{
		config:     cfg,
		httpClient: httpClient,
		parser:     NewVlessParser(),
		cacheFile:  "/opt/etc/xray-manager/cache/servers.json",
	}
}
func NewSubscriptionLoaderWithCacheDir(cfg *config.Config, cacheDir string) *SubscriptionLoaderImpl {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.PingTimeout) * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			ResponseHeaderTimeout: 15 * time.Second,
		},
	}
	return &SubscriptionLoaderImpl{
		config:     cfg,
		httpClient: httpClient,
		parser:     NewVlessParser(),
		cacheFile:  filepath.Join(cacheDir, "servers.json"),
	}
}
func (sl *SubscriptionLoaderImpl) LoadFromURL() ([]types.Server, error) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if sl.isCacheValid() && len(sl.cache) > 0 {
		return sl.cache, nil
	}
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
		if cachedServers, cacheErr := sl.loadFromCacheFile(); cacheErr == nil {
			sl.cache = cachedServers
			return cachedServers, nil
		}
		return nil, fmt.Errorf("failed to fetch from URL after %d retries and no valid cache: %w", maxRetries, err)
	}
	servers, err := sl.DecodeBase64Config(data)
	if err != nil {
		if cachedServers, cacheErr := sl.loadFromCacheFile(); cacheErr == nil {
			sl.cache = cachedServers
			return cachedServers, nil
		}
		return nil, fmt.Errorf("failed to decode configuration: %w", err)
	}
	sl.cache = servers
	sl.lastUpdate = time.Now()
	if err := sl.saveToCacheFile(servers); err != nil {
		fmt.Printf("Warning: failed to save cache file: %v\n", err)
	}
	return servers, nil
}
func (sl *SubscriptionLoaderImpl) fetchFromURL() (string, error) {
	if sl.config.SubscriptionURL == "" {
		return "", fmt.Errorf("subscription URL is empty")
	}
	resp, err := sl.httpClient.Get(sl.config.SubscriptionURL)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %d %s", resp.StatusCode, resp.Status)
	}
	const maxResponseSize = 10 * 1024 * 1024 // 10MB
	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("received empty response from subscription URL")
	}
	return string(body), nil
}
func (sl *SubscriptionLoaderImpl) DecodeBase64Config(data string) ([]types.Server, error) {
	data = strings.TrimSpace(data)
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 data: %w", err)
		}
	}
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
	return sl.ParseVlessUrls(vlessUrls)
}
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
	if len(errors) > 0 {
		fmt.Printf("Warning: some VLESS URLs failed to parse: %s\n", strings.Join(errors, "; "))
	}
	return servers, nil
}
func (sl *SubscriptionLoaderImpl) ParseVlessUrl(vlessUrl string) (types.Server, error) {
	vlessConfig, err := sl.parser.ParseUrl(vlessUrl)
	if err != nil {
		return types.Server{}, fmt.Errorf("failed to parse VLESS URL: %w", err)
	}
	server, err := sl.parser.ToXrayOutbound(vlessConfig)
	if err != nil {
		return types.Server{}, fmt.Errorf("failed to convert to xray outbound: %w", err)
	}
	server.VlessUrl = vlessUrl
	return server, nil
}
func (sl *SubscriptionLoaderImpl) GetCachedServers() []types.Server {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()
	result := make([]types.Server, len(sl.cache))
	copy(result, sl.cache)
	return result
}
func (sl *SubscriptionLoaderImpl) isCacheValid() bool {
	if sl.lastUpdate.IsZero() {
		return false
	}
	cacheDuration := time.Duration(sl.config.CacheDuration) * time.Second
	return time.Since(sl.lastUpdate) < cacheDuration
}
func (sl *SubscriptionLoaderImpl) saveToCacheFile(servers []types.Server) error {
	cacheDir := filepath.Dir(sl.cacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal servers: %w", err)
	}
	if err := os.WriteFile(sl.cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	return nil
}
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
func (sl *SubscriptionLoaderImpl) InvalidateCache() {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	sl.lastUpdate = time.Time{}
	sl.cache = nil
}
