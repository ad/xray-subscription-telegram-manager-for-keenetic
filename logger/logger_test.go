package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
	}

	for _, test := range tests {
		if got := test.level.String(); got != test.expected {
			t.Errorf("LogLevel.String() = %v, want %v", got, test.expected)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", DEBUG},
		{"debug", DEBUG},
		{"INFO", INFO},
		{"info", INFO},
		{"WARN", WARN},
		{"warn", WARN},
		{"WARNING", WARN},
		{"ERROR", ERROR},
		{"error", ERROR},
		{"invalid", INFO}, // default
	}

	for _, test := range tests {
		if got := ParseLogLevel(test.input); got != test.expected {
			t.Errorf("ParseLogLevel(%q) = %v, want %v", test.input, got, test.expected)
		}
	}
}

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(INFO, &buf)

	// Test that DEBUG messages are filtered out when level is INFO
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Error("DEBUG message should be filtered out when level is INFO")
	}

	// Test that INFO messages are logged
	logger.Info("info message")
	if buf.Len() == 0 {
		t.Error("INFO message should be logged when level is INFO")
	}

	// Reset buffer
	buf.Reset()

	// Test that WARN messages are logged
	logger.Warn("warn message")
	if buf.Len() == 0 {
		t.Error("WARN message should be logged when level is INFO")
	}

	// Reset buffer
	buf.Reset()

	// Test that ERROR messages are logged
	logger.Error("error message")
	if buf.Len() == 0 {
		t.Error("ERROR message should be logged when level is INFO")
	}
}

func TestLogger_Formatting(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(INFO, &buf)

	logger.Info("test message with %s and %d", "string", 42)

	output := buf.String()

	// Check that timestamp is present (format: 2006-01-02 15:04:05)
	if !strings.Contains(output, time.Now().Format("2006-01-02")) {
		t.Error("Log output should contain timestamp")
	}

	// Check that log level is present
	if !strings.Contains(output, "INFO") {
		t.Error("Log output should contain log level")
	}

	// Check that formatted message is present
	if !strings.Contains(output, "test message with string and 42") {
		t.Error("Log output should contain formatted message")
	}
}

func TestNewFileLogger(t *testing.T) {
	// Create a temporary file
	tmpFile := "/tmp/test_logger.log"
	defer os.Remove(tmpFile)

	logger, err := NewFileLogger(INFO, tmpFile)
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}

	logger.Info("test file logging")

	// Check that file was created and contains the log message
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test file logging") {
		t.Error("Log file should contain the logged message")
	}
}
