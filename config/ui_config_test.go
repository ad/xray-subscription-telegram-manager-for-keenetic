package config

import (
	"testing"
)

func TestUIConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid UI config",
			config: Config{
				AdminID:         123456789,
				BotToken:        "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
				SubscriptionURL: "https://example.com/config.txt",
				UI: UIConfig{
					EnableNameOptimization:    true,
					NameOptimizationThreshold: 0.7,
				},
			},
			expectError: false,
		},
		{
			name: "threshold too low",
			config: Config{
				AdminID:         123456789,
				BotToken:        "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
				SubscriptionURL: "https://example.com/config.txt",
				UI: UIConfig{
					EnableNameOptimization:    true,
					NameOptimizationThreshold: -0.1,
				},
			},
			expectError: true,
			errorMsg:    "name_optimization_threshold must be between 0 and 1",
		},
		{
			name: "threshold too high",
			config: Config{
				AdminID:         123456789,
				BotToken:        "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
				SubscriptionURL: "https://example.com/config.txt",
				UI: UIConfig{
					EnableNameOptimization:    true,
					NameOptimizationThreshold: 1.5,
				},
			},
			expectError: true,
			errorMsg:    "name_optimization_threshold must be between 0 and 1",
		},
		{
			name: "threshold at boundary - 0",
			config: Config{
				AdminID:         123456789,
				BotToken:        "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
				SubscriptionURL: "https://example.com/config.txt",
				UI: UIConfig{
					EnableNameOptimization:    true,
					NameOptimizationThreshold: 0.0,
				},
			},
			expectError: false,
		},
		{
			name: "threshold at boundary - 1",
			config: Config{
				AdminID:         123456789,
				BotToken:        "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
				SubscriptionURL: "https://example.com/config.txt",
				UI: UIConfig{
					EnableNameOptimization:    true,
					NameOptimizationThreshold: 1.0,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestUIConfig_Defaults(t *testing.T) {
	config := Config{
		AdminID:         123456789,
		BotToken:        "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
		SubscriptionURL: "https://example.com/config.txt",
		// UI config not set - should get defaults
	}

	config.SetDefaults()

	if !config.UI.EnableNameOptimization {
		t.Errorf("expected EnableNameOptimization to default to true")
	}

	if config.UI.NameOptimizationThreshold != 0.7 {
		t.Errorf("expected NameOptimizationThreshold to default to 0.7, got %f", config.UI.NameOptimizationThreshold)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
