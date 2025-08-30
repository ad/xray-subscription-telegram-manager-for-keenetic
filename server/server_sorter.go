package server

import (
	"sort"
	"strings"
	"xray-telegram-manager/types"
)

// ServerSorter provides various sorting methods for servers and ping results
type ServerSorter struct{}

// NewServerSorter creates a new ServerSorter instance
func NewServerSorter() *ServerSorter {
	return &ServerSorter{}
}

// SortAlphabetically sorts servers by name in alphabetical order (ascending)
func (ss *ServerSorter) SortAlphabetically(servers []types.Server) []types.Server {
	if len(servers) == 0 {
		return servers
	}

	// Create a copy to avoid modifying the original slice
	sorted := make([]types.Server, len(servers))
	copy(sorted, servers)

	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})

	return sorted
}

// SortPingResults sorts ping results by speed first, then alphabetically
// Available servers are prioritized over unavailable ones
func (ss *ServerSorter) SortPingResults(results []types.PingResult) []types.PingResult {
	if len(results) == 0 {
		return results
	}

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

		// Both available: sort by latency first, then alphabetically
		if sorted[i].Available && sorted[j].Available {
			if sorted[i].Latency != sorted[j].Latency {
				return sorted[i].Latency < sorted[j].Latency
			}
			// Same latency: sort alphabetically
			return strings.ToLower(sorted[i].Server.Name) < strings.ToLower(sorted[j].Server.Name)
		}

		// Both unavailable: sort alphabetically
		return strings.ToLower(sorted[i].Server.Name) < strings.ToLower(sorted[j].Server.Name)
	})

	return sorted
}

// SortForQuickSelect sorts results for quick select functionality
// Returns fastest servers up to the specified limit, sorted by speed then alphabetically
func (ss *ServerSorter) SortForQuickSelect(results []types.PingResult, limit int) []types.PingResult {
	if len(results) == 0 {
		return results
	}

	// First, filter only available servers
	available := make([]types.PingResult, 0)
	for _, result := range results {
		if result.Available {
			available = append(available, result)
		}
	}

	// Sort available servers by latency, then alphabetically
	sorted := ss.SortPingResults(available)

	// Return up to the specified limit
	if limit > 0 && len(sorted) > limit {
		return sorted[:limit]
	}

	return sorted
}
