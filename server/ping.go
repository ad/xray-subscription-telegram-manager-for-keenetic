package server

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)

// PingTesterImpl handles server latency testing
type PingTesterImpl struct {
	config *config.Config
}

// NewPingTester creates a new ping tester instance
func NewPingTester(cfg *config.Config) *PingTesterImpl {
	return &PingTesterImpl{
		config: cfg,
	}
}

// TestServers tests ping latency for multiple servers concurrently
func (pt *PingTesterImpl) TestServers(servers []types.Server) ([]types.PingResult, error) {
	return pt.TestServersWithProgress(servers, nil)
}

// TestServersWithProgress tests ping latency for multiple servers concurrently with progress callback
func (pt *PingTesterImpl) TestServersWithProgress(servers []types.Server, progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error) {
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers provided for testing")
	}

	results := make([]types.PingResult, len(servers))
	var wg sync.WaitGroup
	var completedMutex sync.Mutex
	completed := 0

	// Use a semaphore to limit concurrent connections
	semaphore := make(chan struct{}, 10) // Limit to 10 concurrent tests

	for i, server := range servers {
		wg.Add(1)
		go func(index int, srv types.Server) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			results[index] = pt.TestServer(srv)

			// Update progress if callback is provided
			if progressCallback != nil {
				completedMutex.Lock()
				completed++
				currentCompleted := completed
				completedMutex.Unlock()

				progressCallback(currentCompleted, len(servers), srv.Name)
			}
		}(i, server)
	}

	wg.Wait()
	return results, nil
}

// TestServer tests ping latency for a single server
func (pt *PingTesterImpl) TestServer(server types.Server) types.PingResult {
	result := types.PingResult{
		Server:    server,
		Available: false,
		Latency:   0,
		Error:     nil,
	}

	// Create context with timeout
	timeout := time.Duration(pt.config.PingTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Record start time
	startTime := time.Now()

	// Attempt TCP connection to the server
	address := fmt.Sprintf("%s:%d", server.Address, server.Port)

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)

	// Calculate latency
	latency := time.Since(startTime)

	if err != nil {
		result.Error = fmt.Errorf("connection failed: %w", err)
		result.Available = false
		result.Latency = 0
		return result
	}

	// Close connection immediately
	conn.Close()

	// Connection successful
	result.Available = true
	result.Latency = latency.Milliseconds()
	result.Error = nil

	return result
}

// SortByLatency sorts ping results by latency (ascending order)
// Available servers are sorted first, then unavailable servers
func (pt *PingTesterImpl) SortByLatency(results []types.PingResult) []types.PingResult {
	// Create a copy to avoid modifying the original slice
	sorted := make([]types.PingResult, len(results))
	copy(sorted, results)

	sort.Slice(sorted, func(i, j int) bool {
		// Available servers come first
		if sorted[i].Available && !sorted[j].Available {
			return true
		}
		if !sorted[i].Available && sorted[j].Available {
			return false
		}

		// If both are available, sort by latency (ascending)
		if sorted[i].Available && sorted[j].Available {
			return sorted[i].Latency < sorted[j].Latency
		}

		// If both are unavailable, maintain original order
		return false
	})

	return sorted
}

// FormatResultsForTelegram formats ping results for display in Telegram
func (pt *PingTesterImpl) FormatResultsForTelegram(results []types.PingResult) string {
	if len(results) == 0 {
		return "No servers to test"
	}

	// Sort results by latency
	sortedResults := pt.SortByLatency(results)

	var message string
	message += "🏓 *Ping Test Results*\n\n"

	availableCount := 0
	for _, result := range sortedResults {
		if result.Available {
			availableCount++
		}
	}

	message += fmt.Sprintf("📊 *Summary:* %d/%d servers available\n\n", availableCount, len(results))

	for i, result := range sortedResults {
		// Add server number
		message += fmt.Sprintf("%d\\. ", i+1)

		// Add server name (escape special characters for Telegram)
		serverName := result.Server.Name
		message += fmt.Sprintf("*%s*\n", serverName)

		// Add status and latency
		if result.Available {
			message += fmt.Sprintf("   ✅ %dms\n", result.Latency)
		} else {
			message += "   ❌ Unavailable\n"
			if result.Error != nil {
				errorMsg := result.Error.Error()
				message += fmt.Sprintf("   📝 %s\n", errorMsg)
			}
		}

		// Add server address for reference
		address := fmt.Sprintf("%s:%d", result.Server.Address, result.Server.Port)
		message += fmt.Sprintf("   🌐 %s\n", address)

		// Add spacing between servers
		if i < len(sortedResults)-1 {
			message += "\n"
		}
	}

	return message
}

// GetAvailableServers returns only the servers that are available (ping successful)
func (pt *PingTesterImpl) GetAvailableServers(results []types.PingResult) []types.Server {
	var available []types.Server

	for _, result := range results {
		if result.Available {
			available = append(available, result.Server)
		}
	}

	return available
}

// GetFastestServer returns the server with the lowest latency from available servers
func (pt *PingTesterImpl) GetFastestServer(results []types.PingResult) (*types.Server, error) {
	availableResults := make([]types.PingResult, 0)

	for _, result := range results {
		if result.Available {
			availableResults = append(availableResults, result)
		}
	}

	if len(availableResults) == 0 {
		return nil, fmt.Errorf("no available servers found")
	}

	// Sort by latency and return the first (fastest) one
	sortedResults := pt.SortByLatency(availableResults)
	return &sortedResults[0].Server, nil
}
