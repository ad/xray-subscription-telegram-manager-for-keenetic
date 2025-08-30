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

type PingTesterImpl struct {
	config *config.Config
}

func NewPingTester(cfg *config.Config) *PingTesterImpl {
	return &PingTesterImpl{
		config: cfg,
	}
}
func (pt *PingTesterImpl) TestServers(servers []types.Server) ([]types.PingResult, error) {
	return pt.TestServersWithProgress(servers, nil)
}
func (pt *PingTesterImpl) TestServersWithProgress(servers []types.Server, progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error) {
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers provided for testing")
	}
	results := make([]types.PingResult, len(servers))
	var wg sync.WaitGroup
	var completedMutex sync.Mutex
	completed := 0
	semaphore := make(chan struct{}, 10) // Limit to 10 concurrent tests
	for i, server := range servers {
		wg.Add(1)
		go func(index int, srv types.Server) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			results[index] = pt.TestServer(srv)
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
func (pt *PingTesterImpl) TestServer(server types.Server) types.PingResult {
	result := types.PingResult{
		Server:    server,
		Available: false,
		Latency:   0,
		Error:     nil,
	}
	timeout := time.Duration(pt.config.PingTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	startTime := time.Now()
	address := fmt.Sprintf("%s:%d", server.Address, server.Port)
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	latency := time.Since(startTime)
	if err != nil {
		result.Error = fmt.Errorf("connection failed: %w", err)
		result.Available = false
		result.Latency = 0
		return result
	}
	if err := conn.Close(); err != nil {
		// Connection already closed or error occurred - this is expected
		_ = err
	}
	result.Available = true
	result.Latency = latency.Milliseconds()
	result.Error = nil
	return result
}
func (pt *PingTesterImpl) SortByLatency(results []types.PingResult) []types.PingResult {
	sorted := make([]types.PingResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Available && !sorted[j].Available {
			return true
		}
		if !sorted[i].Available && sorted[j].Available {
			return false
		}
		if sorted[i].Available && sorted[j].Available {
			return sorted[i].Latency < sorted[j].Latency
		}
		return false
	})
	return sorted
}
func (pt *PingTesterImpl) FormatResultsForTelegram(results []types.PingResult) string {
	if len(results) == 0 {
		return "No servers to test"
	}
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
		message += fmt.Sprintf("%d\\. ", i+1)
		serverName := result.Server.Name
		message += fmt.Sprintf("*%s*\n", serverName)
		if result.Available {
			message += fmt.Sprintf("   ✅ %dms\n", result.Latency)
		} else {
			message += "   ❌ Unavailable\n"
			if result.Error != nil {
				errorMsg := result.Error.Error()
				message += fmt.Sprintf("   📝 %s\n", errorMsg)
			}
		}
		address := fmt.Sprintf("%s:%d", result.Server.Address, result.Server.Port)
		message += fmt.Sprintf("   🌐 %s\n", address)
		if i < len(sortedResults)-1 {
			message += "\n"
		}
	}
	return message
}
func (pt *PingTesterImpl) GetAvailableServers(results []types.PingResult) []types.Server {
	var available []types.Server
	for _, result := range results {
		if result.Available {
			available = append(available, result.Server)
		}
	}
	return available
}
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
	sortedResults := pt.SortByLatency(availableResults)
	return &sortedResults[0].Server, nil
}
