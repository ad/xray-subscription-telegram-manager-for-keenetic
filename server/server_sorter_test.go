package server

import (
	"errors"
	"reflect"
	"testing"
	"xray-telegram-manager/types"
)

// comparePingResults compares two slices of PingResult, handling errors properly
func comparePingResults(a, b []types.PingResult) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		// Compare server fields individually (avoiding map comparison)
		if a[i].Server.ID != b[i].Server.ID ||
			a[i].Server.Name != b[i].Server.Name ||
			a[i].Server.VlessUrl != b[i].Server.VlessUrl ||
			a[i].Server.Tag != b[i].Server.Tag ||
			a[i].Server.Protocol != b[i].Server.Protocol ||
			a[i].Server.Address != b[i].Server.Address ||
			a[i].Server.Port != b[i].Server.Port ||
			a[i].Available != b[i].Available ||
			a[i].Latency != b[i].Latency {
			return false
		}

		// Compare errors by their string representation
		if (a[i].Error == nil) != (b[i].Error == nil) {
			return false
		}
		if a[i].Error != nil && b[i].Error != nil {
			if a[i].Error.Error() != b[i].Error.Error() {
				return false
			}
		}
	}

	return true
}

func TestServerSorter_SortAlphabetically(t *testing.T) {
	sorter := NewServerSorter()

	tests := []struct {
		name     string
		servers  []types.Server
		expected []types.Server
	}{
		{
			name:     "empty slice",
			servers:  []types.Server{},
			expected: []types.Server{},
		},
		{
			name: "single server",
			servers: []types.Server{
				{ID: "1", Name: "Server A"},
			},
			expected: []types.Server{
				{ID: "1", Name: "Server A"},
			},
		},
		{
			name: "multiple servers - basic sorting",
			servers: []types.Server{
				{ID: "1", Name: "Charlie"},
				{ID: "2", Name: "Alpha"},
				{ID: "3", Name: "Beta"},
			},
			expected: []types.Server{
				{ID: "2", Name: "Alpha"},
				{ID: "3", Name: "Beta"},
				{ID: "1", Name: "Charlie"},
			},
		},
		{
			name: "case insensitive sorting",
			servers: []types.Server{
				{ID: "1", Name: "charlie"},
				{ID: "2", Name: "Alpha"},
				{ID: "3", Name: "beta"},
			},
			expected: []types.Server{
				{ID: "2", Name: "Alpha"},
				{ID: "3", Name: "beta"},
				{ID: "1", Name: "charlie"},
			},
		},
		{
			name: "servers with special characters",
			servers: []types.Server{
				{ID: "1", Name: "🇺🇸 US Server"},
				{ID: "2", Name: "🇩🇪 DE Server"},
				{ID: "3", Name: "🇬🇧 UK Server"},
			},
			expected: []types.Server{
				{ID: "2", Name: "🇩🇪 DE Server"},
				{ID: "3", Name: "🇬🇧 UK Server"},
				{ID: "1", Name: "🇺🇸 US Server"},
			},
		},
		{
			name: "servers with numbers",
			servers: []types.Server{
				{ID: "1", Name: "Server 10"},
				{ID: "2", Name: "Server 2"},
				{ID: "3", Name: "Server 1"},
			},
			expected: []types.Server{
				{ID: "3", Name: "Server 1"},
				{ID: "1", Name: "Server 10"},
				{ID: "2", Name: "Server 2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sorter.SortAlphabetically(tt.servers)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SortAlphabetically() = %v, want %v", result, tt.expected)
			}

			// Verify original slice is not modified
			if len(tt.servers) > 1 {
				originalFirst := tt.servers[0].Name
				resultFirst := result[0].Name
				if originalFirst == resultFirst && len(tt.servers) > 1 {
					// Only check if the order actually changed
					if tt.servers[0].Name != tt.expected[0].Name {
						t.Error("Original slice should not be modified")
					}
				}
			}
		})
	}
}

func TestServerSorter_SortPingResults(t *testing.T) {
	sorter := NewServerSorter()

	tests := []struct {
		name     string
		results  []types.PingResult
		expected []types.PingResult
	}{
		{
			name:     "empty slice",
			results:  []types.PingResult{},
			expected: []types.PingResult{},
		},
		{
			name: "single result",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Server A"}, Available: true, Latency: 100},
			},
			expected: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Server A"}, Available: true, Latency: 100},
			},
		},
		{
			name: "available servers sorted by latency",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Slow Server"}, Available: true, Latency: 200},
				{Server: types.Server{ID: "2", Name: "Fast Server"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "3", Name: "Medium Server"}, Available: true, Latency: 100},
			},
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Fast Server"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "3", Name: "Medium Server"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "1", Name: "Slow Server"}, Available: true, Latency: 200},
			},
		},
		{
			name: "available servers come before unavailable",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Unavailable"}, Available: false, Latency: 0, Error: errors.New("timeout")},
				{Server: types.Server{ID: "2", Name: "Available"}, Available: true, Latency: 100},
			},
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Available"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "1", Name: "Unavailable"}, Available: false, Latency: 0, Error: errors.New("timeout")},
			},
		},
		{
			name: "same latency sorted alphabetically",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Charlie"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "2", Name: "Alpha"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "3", Name: "Beta"}, Available: true, Latency: 100},
			},
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Alpha"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "3", Name: "Beta"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "1", Name: "Charlie"}, Available: true, Latency: 100},
			},
		},
		{
			name: "unavailable servers sorted alphabetically",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Charlie"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "2", Name: "Alpha"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "3", Name: "Beta"}, Available: false, Latency: 0},
			},
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Alpha"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "3", Name: "Beta"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "1", Name: "Charlie"}, Available: false, Latency: 0},
			},
		},
		{
			name: "mixed available and unavailable",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Unavailable Z"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "2", Name: "Available B"}, Available: true, Latency: 150},
				{Server: types.Server{ID: "3", Name: "Unavailable A"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "4", Name: "Available A"}, Available: true, Latency: 50},
			},
			expected: []types.PingResult{
				{Server: types.Server{ID: "4", Name: "Available A"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "2", Name: "Available B"}, Available: true, Latency: 150},
				{Server: types.Server{ID: "3", Name: "Unavailable A"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "1", Name: "Unavailable Z"}, Available: false, Latency: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sorter.SortPingResults(tt.results)
			if !comparePingResults(result, tt.expected) {
				t.Errorf("SortPingResults() = %v, want %v", result, tt.expected)
			}

			// Verify original slice is not modified
			if len(tt.results) > 1 {
				originalFirst := tt.results[0].Server.Name
				resultFirst := result[0].Server.Name
				if originalFirst == resultFirst && len(tt.results) > 1 {
					// Only check if the order actually changed
					if tt.results[0].Server.Name != tt.expected[0].Server.Name {
						t.Error("Original slice should not be modified")
					}
				}
			}
		})
	}
}

func TestServerSorter_SortForQuickSelect(t *testing.T) {
	sorter := NewServerSorter()

	tests := []struct {
		name     string
		results  []types.PingResult
		limit    int
		expected []types.PingResult
	}{
		{
			name:     "empty slice",
			results:  []types.PingResult{},
			limit:    5,
			expected: []types.PingResult{},
		},
		{
			name: "only unavailable servers",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Unavailable"}, Available: false, Latency: 0},
			},
			limit:    5,
			expected: []types.PingResult{},
		},
		{
			name: "limit larger than available servers",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Server A"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "2", Name: "Server B"}, Available: true, Latency: 50},
			},
			limit: 5,
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Server B"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "1", Name: "Server A"}, Available: true, Latency: 100},
			},
		},
		{
			name: "limit smaller than available servers",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Server A"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "2", Name: "Server B"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "3", Name: "Server C"}, Available: true, Latency: 200},
				{Server: types.Server{ID: "4", Name: "Server D"}, Available: true, Latency: 75},
			},
			limit: 2,
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Server B"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "4", Name: "Server D"}, Available: true, Latency: 75},
			},
		},
		{
			name: "mixed available and unavailable with limit",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Unavailable"}, Available: false, Latency: 0},
				{Server: types.Server{ID: "2", Name: "Fast"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "3", Name: "Slow"}, Available: true, Latency: 200},
				{Server: types.Server{ID: "4", Name: "Medium"}, Available: true, Latency: 100},
			},
			limit: 2,
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Fast"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "4", Name: "Medium"}, Available: true, Latency: 100},
			},
		},
		{
			name: "zero limit returns all available",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Server A"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "2", Name: "Server B"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "3", Name: "Unavailable"}, Available: false, Latency: 0},
			},
			limit: 0,
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Server B"}, Available: true, Latency: 50},
				{Server: types.Server{ID: "1", Name: "Server A"}, Available: true, Latency: 100},
			},
		},
		{
			name: "same latency sorted alphabetically with limit",
			results: []types.PingResult{
				{Server: types.Server{ID: "1", Name: "Charlie"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "2", Name: "Alpha"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "3", Name: "Beta"}, Available: true, Latency: 100},
			},
			limit: 2,
			expected: []types.PingResult{
				{Server: types.Server{ID: "2", Name: "Alpha"}, Available: true, Latency: 100},
				{Server: types.Server{ID: "3", Name: "Beta"}, Available: true, Latency: 100},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sorter.SortForQuickSelect(tt.results, tt.limit)
			if !comparePingResults(result, tt.expected) {
				t.Errorf("SortForQuickSelect() = %v, want %v", result, tt.expected)
			}

			// Verify the limit is respected
			if tt.limit > 0 && len(result) > tt.limit {
				t.Errorf("Result length %d exceeds limit %d", len(result), tt.limit)
			}

			// Verify only available servers are returned
			for _, res := range result {
				if !res.Available {
					t.Error("SortForQuickSelect should only return available servers")
				}
			}
		})
	}
}

func TestServerSorter_Integration(t *testing.T) {
	sorter := NewServerSorter()

	// Test that all methods work together and don't interfere with each other
	servers := []types.Server{
		{ID: "1", Name: "🇺🇸 US East"},
		{ID: "2", Name: "🇩🇪 Germany"},
		{ID: "3", Name: "🇬🇧 UK London"},
	}

	results := []types.PingResult{
		{Server: servers[0], Available: true, Latency: 150},
		{Server: servers[1], Available: false, Latency: 0, Error: errors.New("timeout")},
		{Server: servers[2], Available: true, Latency: 80},
	}

	// Test alphabetical sorting
	sortedServers := sorter.SortAlphabetically(servers)
	expectedServerOrder := []string{"🇩🇪 Germany", "🇬🇧 UK London", "🇺🇸 US East"}
	for i, server := range sortedServers {
		if server.Name != expectedServerOrder[i] {
			t.Errorf("Alphabetical sort failed: got %s, want %s", server.Name, expectedServerOrder[i])
		}
	}

	// Test ping result sorting
	sortedResults := sorter.SortPingResults(results)
	if !sortedResults[0].Available || sortedResults[0].Server.Name != "🇬🇧 UK London" {
		t.Error("Ping result sort failed: fastest available server should be first")
	}

	// Test quick select
	quickSelect := sorter.SortForQuickSelect(results, 1)
	if len(quickSelect) != 1 || quickSelect[0].Server.Name != "🇬🇧 UK London" {
		t.Error("Quick select failed: should return fastest server")
	}
}
