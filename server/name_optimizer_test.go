package server

import (
	"testing"
	"xray-telegram-manager/logger"
	"xray-telegram-manager/types"
)

func TestNewServerNameOptimizer(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
		expected  float64
	}{
		{
			name:      "valid threshold",
			threshold: 0.7,
			expected:  0.7,
		},
		{
			name:      "zero threshold defaults to 0.7",
			threshold: 0,
			expected:  0.7,
		},
		{
			name:      "negative threshold defaults to 0.7",
			threshold: -0.5,
			expected:  0.7,
		},
		{
			name:      "threshold > 1 defaults to 0.7",
			threshold: 1.5,
			expected:  0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimizer := NewServerNameOptimizer(tt.threshold, nil)
			if optimizer.threshold != tt.expected {
				t.Errorf("expected threshold %f, got %f", tt.expected, optimizer.threshold)
			}
		})
	}
}

func TestFindCommonSuffixes(t *testing.T) {
	optimizer := NewServerNameOptimizer(0.7, nil)

	tests := []struct {
		name     string
		names    []string
		expected []string
	}{
		{
			name:     "empty names",
			names:    []string{},
			expected: []string{},
		},
		{
			name:     "single name",
			names:    []string{"server1"},
			expected: []string{},
		},
		{
			name: "common domain suffix",
			names: []string{
				"server1.example.com",
				"server2.example.com",
				"server3.example.com",
			},
			expected: []string{".example.com", "example.com", ".com", "com"}, // Only meaningful suffixes
		},
		{
			name: "common region suffix",
			names: []string{
				"server1-us-east",
				"server2-us-east",
				"server3-us-east",
			},
			expected: []string{"-us-east", "us-east", "-east", "east"}, // Only meaningful suffixes
		},
		{
			name: "no common suffixes",
			names: []string{
				"server1",
				"database2",
				"cache3",
			},
			expected: []string{},
		},
		{
			name: "mixed suffixes",
			names: []string{
				"web1.prod.com",
				"web2.prod.com",
				"api1.test.com",
				"api2.test.com",
			},
			expected: []string{".com", "com"}, // Only .com and com are common to all
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.FindCommonSuffixes(tt.names)

			// Check if all expected suffixes are present (algorithm might find more)
			expectedMap := make(map[string]bool)
			for _, suffix := range tt.expected {
				expectedMap[suffix] = true
			}

			foundExpected := make(map[string]bool)
			for _, suffix := range result {
				if expectedMap[suffix] {
					foundExpected[suffix] = true
				}
			}

			// Ensure all expected suffixes were found
			for _, expected := range tt.expected {
				if !foundExpected[expected] {
					t.Errorf("expected suffix not found: %s", expected)
				}
			}
		})
	}
}

func TestOptimizeNames(t *testing.T) {
	logger := logger.NewLogger(logger.DEBUG, nil)
	optimizer := NewServerNameOptimizer(0.7, logger)

	tests := []struct {
		name           string
		servers        []types.Server
		expectedNames  []string
		expectedSuffix string
		expectedCount  int
	}{
		{
			name:           "empty servers",
			servers:        []types.Server{},
			expectedNames:  []string{},
			expectedSuffix: "",
			expectedCount:  0,
		},
		{
			name: "common domain suffix - should optimize",
			servers: []types.Server{
				{ID: "1", Name: "server1.example.com"},
				{ID: "2", Name: "server2.example.com"},
				{ID: "3", Name: "server3.example.com"},
			},
			expectedNames:  []string{"server1", "server2", "server3"},
			expectedSuffix: ".example.com",
			expectedCount:  3,
		},
		{
			name: "insufficient coverage - should not optimize",
			servers: []types.Server{
				{ID: "1", Name: "server1.example.com"},
				{ID: "2", Name: "server2.different.org"},
				{ID: "3", Name: "server3.another.net"},
				{ID: "4", Name: "server4.test.edu"},
			},
			expectedNames:  []string{"server1.example.com", "server2.different.org", "server3.another.net", "server4.test.edu"},
			expectedSuffix: "",
			expectedCount:  0,
		},
		{
			name: "optimization would create invalid names",
			servers: []types.Server{
				{ID: "1", Name: "ab.example.com"}, // would become "ab" - too short
				{ID: "2", Name: "cd.example.com"}, // would become "cd" - too short
				{ID: "3", Name: "server3.example.com"},
			},
			expectedNames:  []string{"ab.example.com", "cd.example.com", "server3"},
			expectedSuffix: ".example.com",
			expectedCount:  1, // only server3 gets optimized
		},
		{
			name: "mixed case with region suffix",
			servers: []types.Server{
				{ID: "1", Name: "Web-Server-US-East"},
				{ID: "2", Name: "API-Server-US-East"},
				{ID: "3", Name: "DB-Server-US-East"},
			},
			expectedNames:  []string{"Web", "API", "DB-Server-US-East"}, // "DB" would be too short (2 chars), so kept original
			expectedSuffix: "-Server-US-East",
			expectedCount:  2, // Only Web and API get optimized (3 chars each)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.OptimizeNames(tt.servers)

			if len(result.OptimizedNames) != len(tt.expectedNames) {
				t.Errorf("expected %d optimized names, got %d", len(tt.expectedNames), len(result.OptimizedNames))
				return
			}

			for i, expected := range tt.expectedNames {
				if result.OptimizedNames[i] != expected {
					t.Errorf("expected optimized name[%d] = %s, got %s", i, expected, result.OptimizedNames[i])
				}
			}

			if result.RemovedSuffix != tt.expectedSuffix {
				t.Errorf("expected removed suffix = %s, got %s", tt.expectedSuffix, result.RemovedSuffix)
			}

			if result.AppliedCount != tt.expectedCount {
				t.Errorf("expected applied count = %d, got %d", tt.expectedCount, result.AppliedCount)
			}

			if result.TotalCount != len(tt.servers) {
				t.Errorf("expected total count = %d, got %d", len(tt.servers), result.TotalCount)
			}
		})
	}
}

func TestApplyOptimization(t *testing.T) {
	optimizer := NewServerNameOptimizer(0.7, nil)

	tests := []struct {
		name          string
		servers       []types.Server
		suffix        string
		expectedNames []string
	}{
		{
			name:          "empty suffix",
			servers:       []types.Server{{ID: "1", Name: "server1"}},
			suffix:        "",
			expectedNames: []string{"server1"},
		},
		{
			name: "apply domain suffix removal",
			servers: []types.Server{
				{ID: "1", Name: "server1.example.com"},
				{ID: "2", Name: "server2.example.com"},
				{ID: "3", Name: "server3.different.com"},
			},
			suffix:        ".example.com",
			expectedNames: []string{"server1", "server2", "server3.different.com"},
		},
		{
			name: "prevent invalid optimization",
			servers: []types.Server{
				{ID: "1", Name: "ab.example.com"}, // would become "ab" - too short
				{ID: "2", Name: "server2.example.com"},
			},
			suffix:        ".example.com",
			expectedNames: []string{"ab.example.com", "server2"}, // first one kept original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.ApplyOptimization(tt.servers, tt.suffix)

			if len(result) != len(tt.expectedNames) {
				t.Errorf("expected %d servers, got %d", len(tt.expectedNames), len(result))
				return
			}

			for i, expected := range tt.expectedNames {
				if result[i].Name != expected {
					t.Errorf("expected server[%d].Name = %s, got %s", i, expected, result[i].Name)
				}

				// Ensure other fields are preserved
				if result[i].ID != tt.servers[i].ID {
					t.Errorf("expected server[%d].ID = %s, got %s", i, tt.servers[i].ID, result[i].ID)
				}
			}
		})
	}
}

func TestIsValidOptimizedName(t *testing.T) {
	optimizer := NewServerNameOptimizer(0.7, nil)

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "empty name",
			input:    "",
			expected: false,
		},
		{
			name:     "too short",
			input:    "ab",
			expected: false,
		},
		{
			name:     "valid short name",
			input:    "abc",
			expected: true,
		},
		{
			name:     "valid name with numbers",
			input:    "server1",
			expected: true,
		},
		{
			name:     "valid name with special chars",
			input:    "web-server",
			expected: true,
		},
		{
			name:     "only special characters",
			input:    "---",
			expected: false,
		},
		{
			name:     "mixed valid",
			input:    "api-v2",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.isValidOptimizedName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %t, got %t for input: %s", tt.expected, result, tt.input)
			}
		})
	}
}

func TestIsMeaningfulSuffix(t *testing.T) {
	optimizer := NewServerNameOptimizer(0.7, nil)

	tests := []struct {
		name     string
		suffix   string
		expected bool
	}{
		{
			name:     "too short",
			suffix:   "ab",
			expected: false,
		},
		{
			name:     "only numbers",
			suffix:   "123",
			expected: false,
		},
		{
			name:     "repeated characters",
			suffix:   "aaa",
			expected: false,
		},
		{
			name:     "domain suffix",
			suffix:   ".com",
			expected: true,
		},
		{
			name:     "region suffix",
			suffix:   "-east",
			expected: true,
		},
		{
			name:     "underscore suffix",
			suffix:   "_prod",
			expected: true,
		},
		{
			name:     "space suffix",
			suffix:   " server",
			expected: true,
		},
		{
			name:     "long meaningful suffix",
			suffix:   "production",
			expected: true,
		},
		{
			name:     "short without separators",
			suffix:   "abc",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.isMeaningfulSuffix(tt.suffix)
			if result != tt.expected {
				t.Errorf("expected %t, got %t for suffix: %s", tt.expected, result, tt.suffix)
			}
		})
	}
}

func TestCalculateCoverage(t *testing.T) {
	optimizer := NewServerNameOptimizer(0.7, nil)

	tests := []struct {
		name     string
		names    []string
		suffix   string
		expected float64
	}{
		{
			name:     "empty names",
			names:    []string{},
			suffix:   ".com",
			expected: 0,
		},
		{
			name:     "full coverage",
			names:    []string{"server1.com", "server2.com", "server3.com"},
			suffix:   ".com",
			expected: 1.0,
		},
		{
			name:     "partial coverage",
			names:    []string{"server1.com", "server2.com", "server3.org"},
			suffix:   ".com",
			expected: 2.0 / 3.0,
		},
		{
			name:     "no coverage",
			names:    []string{"server1", "server2", "server3"},
			suffix:   ".com",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.calculateCoverage(tt.names, tt.suffix)
			if result != tt.expected {
				t.Errorf("expected coverage %f, got %f", tt.expected, result)
			}
		})
	}
}

// Benchmark tests
func BenchmarkOptimizeNames(b *testing.B) {
	optimizer := NewServerNameOptimizer(0.7, nil)

	// Create a large set of servers for benchmarking
	servers := make([]types.Server, 1000)
	for i := 0; i < 1000; i++ {
		servers[i] = types.Server{
			ID:   string(rune(i)),
			Name: "server" + string(rune(i)) + ".example.com",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizer.OptimizeNames(servers)
	}
}

func BenchmarkFindCommonSuffixes(b *testing.B) {
	optimizer := NewServerNameOptimizer(0.7, nil)

	names := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		names[i] = "server" + string(rune(i)) + ".example.com"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizer.FindCommonSuffixes(names)
	}
}
