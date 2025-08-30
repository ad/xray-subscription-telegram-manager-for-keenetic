package telegram

import (
	"testing"
	"xray-telegram-manager/types"
)

func TestCreateProgressBar(t *testing.T) {
	// Create a MessageFormatter instance for testing
	mf := NewMessageFormatter()

	tests := []struct {
		name       string
		percentage int
		length     int
		expected   string
	}{
		{"0 percent", 0, 10, "[░░░░░░░░░░] 0%"},
		{"50 percent", 50, 10, "[█████░░░░░] 50%"},
		{"100 percent", 100, 10, "[██████████] 100%"},
		{"Over 100", 150, 10, "[██████████] 100%"},
		{"Negative", -10, 10, "[░░░░░░░░░░] 0%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mf.createProgressBar(tt.percentage, tt.length)
			if result != tt.expected {
				t.Errorf("createProgressBar(%d, %d) = %s, want %s", tt.percentage, tt.length, result, tt.expected)
			}
		})
	}
}

func TestTestPingWithProgress(t *testing.T) {
	serverMgr := &MockServerManager{
		servers: []types.Server{
			{ID: "1", Name: "Server 1", Address: "1.1.1.1", Port: 443},
			{ID: "2", Name: "Server 2", Address: "2.2.2.2", Port: 443},
			{ID: "3", Name: "Server 3", Address: "3.3.3.3", Port: 443},
		},
	}

	progressUpdates := []struct {
		completed  int
		total      int
		serverName string
	}{}

	progressCallback := func(completed, total int, serverName string) {
		progressUpdates = append(progressUpdates, struct {
			completed  int
			total      int
			serverName string
		}{completed, total, serverName})
	}

	results, err := serverMgr.TestPingWithProgress(progressCallback)
	if err != nil {
		t.Fatalf("TestPingWithProgress failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	if len(progressUpdates) != 3 {
		t.Errorf("Expected 3 progress updates, got %d", len(progressUpdates))
	}

	// Verify progress updates are correct
	for i, update := range progressUpdates {
		expectedCompleted := i + 1
		expectedTotal := 3
		expectedServerName := serverMgr.servers[i].Name

		if update.completed != expectedCompleted {
			t.Errorf("Progress update %d: expected completed=%d, got %d", i, expectedCompleted, update.completed)
		}
		if update.total != expectedTotal {
			t.Errorf("Progress update %d: expected total=%d, got %d", i, expectedTotal, update.total)
		}
		if update.serverName != expectedServerName {
			t.Errorf("Progress update %d: expected serverName=%s, got %s", i, expectedServerName, update.serverName)
		}
	}
}
