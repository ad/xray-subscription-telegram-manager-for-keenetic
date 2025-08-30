package server

import (
	"sort"
	"strings"
	"xray-telegram-manager/logger"
	"xray-telegram-manager/types"
)

// ServerNameOptimizerInterface defines the interface for server name optimization
type ServerNameOptimizerInterface interface {
	// OptimizeNames optimizes server names by removing common suffixes
	OptimizeNames(servers []types.Server) OptimizationResult

	// FindCommonSuffixes finds common suffixes in server names
	FindCommonSuffixes(names []string) []string

	// ApplyOptimization applies optimization to servers with given suffix
	ApplyOptimization(servers []types.Server, suffix string) []types.Server
}

// ServerNameOptimizer handles server name optimization
type ServerNameOptimizer struct {
	threshold float64 // threshold for applying optimization (e.g., 0.7 = 70%)
	logger    *logger.Logger
}

// OptimizationResult contains the results of name optimization
type OptimizationResult struct {
	OriginalNames  []string
	OptimizedNames []string
	RemovedSuffix  string
	AppliedCount   int
	TotalCount     int
}

// NewServerNameOptimizer creates a new ServerNameOptimizer
func NewServerNameOptimizer(threshold float64, logger *logger.Logger) *ServerNameOptimizer {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.7 // default to 70%
	}

	return &ServerNameOptimizer{
		threshold: threshold,
		logger:    logger,
	}
}

// OptimizeNames optimizes server names by removing common suffixes
func (sno *ServerNameOptimizer) OptimizeNames(servers []types.Server) OptimizationResult {
	if len(servers) == 0 {
		return OptimizationResult{}
	}

	// Extract server names
	names := make([]string, len(servers))
	for i, server := range servers {
		names[i] = server.Name
	}

	result := OptimizationResult{
		OriginalNames: make([]string, len(names)),
		TotalCount:    len(names),
	}
	copy(result.OriginalNames, names)

	// Find common suffixes
	suffixes := sno.FindCommonSuffixes(names)
	if len(suffixes) == 0 {
		if sno.logger != nil {
			sno.logger.Debug("No common suffixes found in server names")
		}
		result.OptimizedNames = make([]string, len(names))
		copy(result.OptimizedNames, names)
		return result
	}

	// Find the best suffix to remove (longest one that meets threshold)
	var bestSuffix string
	var bestCoverage float64

	for _, suffix := range suffixes {
		coverage := sno.calculateCoverage(names, suffix)
		if coverage >= sno.threshold && coverage > bestCoverage {
			bestSuffix = suffix
			bestCoverage = coverage
		}
	}

	if bestSuffix == "" {
		if sno.logger != nil {
			sno.logger.Debug("No suffix meets the threshold of %.1f%%", sno.threshold*100)
		}
		result.OptimizedNames = make([]string, len(names))
		copy(result.OptimizedNames, names)
		return result
	}

	// Apply optimization
	optimizedNames := make([]string, len(names))
	appliedCount := 0

	for i, name := range names {
		if strings.HasSuffix(name, bestSuffix) {
			optimized := strings.TrimSuffix(name, bestSuffix)
			optimized = strings.TrimSpace(optimized)

			// Validate the optimized name
			if sno.isValidOptimizedName(optimized) {
				optimizedNames[i] = optimized
				appliedCount++
			} else {
				optimizedNames[i] = name // keep original if optimized is invalid
			}
		} else {
			optimizedNames[i] = name
		}
	}

	result.OptimizedNames = optimizedNames
	result.RemovedSuffix = bestSuffix
	result.AppliedCount = appliedCount

	if sno.logger != nil {
		sno.logger.Info("Server name optimization completed: removed suffix '%s' from %d/%d servers (%.1f%% coverage)",
			bestSuffix, appliedCount, len(names), float64(appliedCount)/float64(len(names))*100)
	}

	return result
}

// FindCommonSuffixes finds common suffixes in server names
func (sno *ServerNameOptimizer) FindCommonSuffixes(names []string) []string {
	if len(names) < 2 {
		return []string{}
	}

	suffixMap := make(map[string]int)

	// Generate all possible suffixes for each name
	for _, name := range names {
		suffixes := sno.generateSuffixes(name)
		for _, suffix := range suffixes {
			suffixMap[suffix]++
		}
	}

	// Filter suffixes that appear in multiple names
	var commonSuffixes []string
	for suffix, count := range suffixMap {
		if count >= 2 && len(suffix) >= 3 { // minimum 3 characters and appears in at least 2 names
			commonSuffixes = append(commonSuffixes, suffix)
		}
	}

	// Sort by length (longest first) to prioritize longer suffixes
	sort.Slice(commonSuffixes, func(i, j int) bool {
		return len(commonSuffixes[i]) > len(commonSuffixes[j])
	})

	return commonSuffixes
}

// ApplyOptimization applies optimization to servers with given suffix
func (sno *ServerNameOptimizer) ApplyOptimization(servers []types.Server, suffix string) []types.Server {
	if suffix == "" {
		return servers
	}

	optimizedServers := make([]types.Server, len(servers))

	for i, server := range servers {
		optimizedServer := server // copy the server

		if strings.HasSuffix(server.Name, suffix) {
			optimized := strings.TrimSuffix(server.Name, suffix)
			optimized = strings.TrimSpace(optimized)

			if sno.isValidOptimizedName(optimized) {
				optimizedServer.Name = optimized
			}
		}

		optimizedServers[i] = optimizedServer
	}

	return optimizedServers
}

// generateSuffixes generates all meaningful suffixes for a given name
func (sno *ServerNameOptimizer) generateSuffixes(name string) []string {
	if len(name) < 3 {
		return []string{}
	}

	var suffixes []string

	// Generate suffixes of different lengths, starting from 3 characters
	for i := 3; i <= len(name); i++ {
		suffix := name[len(name)-i:]

		// Skip suffixes that are just numbers or single characters repeated
		if sno.isMeaningfulSuffix(suffix) {
			suffixes = append(suffixes, suffix)
		}
	}

	return suffixes
}

// isMeaningfulSuffix checks if a suffix is meaningful for optimization
func (sno *ServerNameOptimizer) isMeaningfulSuffix(suffix string) bool {
	if len(suffix) < 3 {
		return false
	}

	// Skip suffixes that are just numbers
	if sno.isOnlyNumbers(suffix) {
		return false
	}

	// Skip suffixes that are just repeated characters
	if sno.isRepeatedChar(suffix) {
		return false
	}

	// Common domain extensions and meaningful words
	commonMeaningfulSuffixes := []string{
		"com", "org", "net", "edu", "gov", "mil", "int",
		"east", "west", "north", "south", "prod", "test", "dev", "staging",
	}

	for _, meaningful := range commonMeaningfulSuffixes {
		if suffix == meaningful {
			return true
		}
	}

	// Skip suffixes that don't contain meaningful separators or patterns
	// Common patterns: .domain.com, -region, _suffix, etc.
	meaningfulChars := []string{".", "-", "_", " "}
	for _, char := range meaningfulChars {
		if strings.Contains(suffix, char) {
			return true
		}
	}

	// If suffix is long enough (>= 5 chars) and contains letters, consider it meaningful
	if len(suffix) >= 5 && sno.containsLetters(suffix) {
		return true
	}

	return false
}

// calculateCoverage calculates what percentage of names have the given suffix
func (sno *ServerNameOptimizer) calculateCoverage(names []string, suffix string) float64 {
	if len(names) == 0 {
		return 0
	}

	count := 0
	for _, name := range names {
		if strings.HasSuffix(name, suffix) {
			count++
		}
	}

	return float64(count) / float64(len(names))
}

// isValidOptimizedName validates that the optimized name is acceptable
func (sno *ServerNameOptimizer) isValidOptimizedName(name string) bool {
	// Name must not be empty
	if name == "" {
		return false
	}

	// Name must be at least 3 characters long
	if len(name) < 3 {
		return false
	}

	// Name should contain at least one letter or number
	return sno.containsAlphanumeric(name)
}

// Helper functions

func (sno *ServerNameOptimizer) isOnlyNumbers(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (sno *ServerNameOptimizer) isRepeatedChar(s string) bool {
	if len(s) == 0 {
		return false
	}

	first := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] != first {
			return false
		}
	}
	return true
}

func (sno *ServerNameOptimizer) containsLetters(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func (sno *ServerNameOptimizer) containsAlphanumeric(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return true
		}
	}
	return false
}
