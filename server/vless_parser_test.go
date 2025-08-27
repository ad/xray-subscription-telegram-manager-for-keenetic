package server

import (
	"testing"
)

func TestVlessParser_ParseUrl(t *testing.T) {
	parser := NewVlessParser()

	// Test VLESS URL from the design document
	vlessUrl := "vless://ec82bca8-1072-4682-822f-30306af408ea@127.0.0.1:443?type=tcp&security=reality&sni=outlook.office.com&pbk=TEST&sid=testid&fp=chrome&flow=xtls-rprx-vision&mux=true&concurrency=8#Netherlands"

	config, err := parser.ParseUrl(vlessUrl)
	if err != nil {
		t.Fatalf("Failed to parse VLESS URL: %v", err)
	}

	// Verify parsed values
	if config.UUID != "ec82bca8-1072-4682-822f-30306af408ea" {
		t.Errorf("Expected UUID 'ec82bca8-1072-4682-822f-30306af408ea', got '%s'", config.UUID)
	}

	if config.Address != "127.0.0.1" {
		t.Errorf("Expected address '127.0.0.1', got '%s'", config.Address)
	}

	if config.Port != 443 {
		t.Errorf("Expected port 443, got %d", config.Port)
	}

	if config.Type != "tcp" {
		t.Errorf("Expected type 'tcp', got '%s'", config.Type)
	}

	if config.Security != "reality" {
		t.Errorf("Expected security 'reality', got '%s'", config.Security)
	}

	if config.SNI != "outlook.office.com" {
		t.Errorf("Expected SNI 'outlook.office.com', got '%s'", config.SNI)
	}

	if config.PublicKey != "TEST" {
		t.Errorf("Expected PublicKey 'TEST', got '%s'", config.PublicKey)
	}

	if config.ShortID != "testid" {
		t.Errorf("Expected ShortID 'testid', got '%s'", config.ShortID)
	}

	if config.Fingerprint != "chrome" {
		t.Errorf("Expected Fingerprint 'chrome', got '%s'", config.Fingerprint)
	}

	if config.Flow != "xtls-rprx-vision" {
		t.Errorf("Expected Flow 'xtls-rprx-vision', got '%s'", config.Flow)
	}

	if config.Name != "Netherlands" {
		t.Errorf("Expected Name 'Netherlands', got '%s'", config.Name)
	}
}

func TestVlessParser_ToXrayOutbound(t *testing.T) {
	parser := NewVlessParser()

	config := VlessConfig{
		UUID:        "ec82bca8-1072-4682-822f-30306af408ea",
		Address:     "127.0.0.1",
		Port:        443,
		Type:        "tcp",
		Security:    "reality",
		SNI:         "outlook.office.com",
		PublicKey:   "TEST",
		ShortID:     "testid",
		Fingerprint: "chrome",
		Flow:        "xtls-rprx-vision",
		Name:        "Netherlands",
	}

	server, err := parser.ToXrayOutbound(config)
	if err != nil {
		t.Fatalf("Failed to convert to xray outbound: %v", err)
	}

	// Verify server fields
	if server.Name != "Netherlands" {
		t.Errorf("Expected Name 'Netherlands', got '%s'", server.Name)
	}

	if server.Protocol != "vless" {
		t.Errorf("Expected Protocol 'vless', got '%s'", server.Protocol)
	}

	if server.Address != "127.0.0.1" {
		t.Errorf("Expected Address '127.0.0.1', got '%s'", server.Address)
	}

	if server.Port != 443 {
		t.Errorf("Expected Port 443, got %d", server.Port)
	}

	// Verify settings structure
	if server.Settings == nil {
		t.Fatal("Settings should not be nil")
	}

	vnext, ok := server.Settings["vnext"].([]map[string]interface{})
	if !ok || len(vnext) == 0 {
		t.Fatal("vnext should be a non-empty slice")
	}

	users, ok := vnext[0]["users"].([]map[string]interface{})
	if !ok || len(users) == 0 {
		t.Fatal("users should be a non-empty slice")
	}

	if users[0]["id"] != "ec82bca8-1072-4682-822f-30306af408ea" {
		t.Errorf("Expected user ID 'ec82bca8-1072-4682-822f-30306af408ea', got '%v'", users[0]["id"])
	}

	if users[0]["flow"] != "xtls-rprx-vision" {
		t.Errorf("Expected flow 'xtls-rprx-vision', got '%v'", users[0]["flow"])
	}

	// Verify stream settings
	if server.StreamSettings == nil {
		t.Fatal("StreamSettings should not be nil")
	}

	if server.StreamSettings["network"] != "tcp" {
		t.Errorf("Expected network 'tcp', got '%v'", server.StreamSettings["network"])
	}

	if server.StreamSettings["security"] != "reality" {
		t.Errorf("Expected security 'reality', got '%v'", server.StreamSettings["security"])
	}

	realitySettings, ok := server.StreamSettings["realitySettings"].(map[string]interface{})
	if !ok {
		t.Fatal("realitySettings should be a map")
	}

	if realitySettings["publicKey"] != "TEST" {
		t.Errorf("Expected publicKey 'TEST', got '%v'", realitySettings["publicKey"])
	}

	if realitySettings["serverName"] != "outlook.office.com" {
		t.Errorf("Expected serverName 'outlook.office.com', got '%v'", realitySettings["serverName"])
	}
}

// TestVlessParser_ParseUrl_EdgeCases tests various edge cases for VLESS URL parsing
func TestVlessParser_ParseUrl_EdgeCases(t *testing.T) {
	parser := NewVlessParser()

	tests := []struct {
		name        string
		vlessUrl    string
		expectError bool
		expected    VlessConfig
	}{
		{
			name:        "Invalid protocol",
			vlessUrl:    "http://uuid@host:443",
			expectError: true,
		},
		{
			name:        "Missing UUID",
			vlessUrl:    "vless://@host:443",
			expectError: true,
		},
		{
			name:        "Invalid port",
			vlessUrl:    "vless://uuid@host:invalid",
			expectError: true,
		},
		{
			name:        "Missing host",
			vlessUrl:    "vless://uuid@:443",
			expectError: true,
		},
		{
			name:     "Minimal valid URL",
			vlessUrl: "vless://ec82bca8-1072-4682-822f-30306af408ea@example.com:443#Test",
			expected: VlessConfig{
				UUID:    "ec82bca8-1072-4682-822f-30306af408ea",
				Address: "example.com",
				Port:    443,
				Name:    "Test",
			},
		},
		{
			name:     "URL with special characters in name",
			vlessUrl: "vless://ec82bca8-1072-4682-822f-30306af408ea@example.com:443#Test%20Server%20🇺🇸",
			expected: VlessConfig{
				UUID:    "ec82bca8-1072-4682-822f-30306af408ea",
				Address: "example.com",
				Port:    443,
				Name:    "Test Server 🇺🇸",
			},
		},
		{
			name:     "URL with IPv6 address",
			vlessUrl: "vless://ec82bca8-1072-4682-822f-30306af408ea@[2001:db8::1]:443#IPv6%20Server",
			expected: VlessConfig{
				UUID:    "ec82bca8-1072-4682-822f-30306af408ea",
				Address: "2001:db8::1", // Go's url.Parse strips the brackets
				Port:    443,
				Name:    "IPv6 Server",
			},
		},
		{
			name:     "URL with TLS security",
			vlessUrl: "vless://ec82bca8-1072-4682-822f-30306af408ea@example.com:443?type=tcp&security=tls&sni=example.com#TLS%20Server",
			expected: VlessConfig{
				UUID:     "ec82bca8-1072-4682-822f-30306af408ea",
				Address:  "example.com",
				Port:     443,
				Type:     "tcp",
				Security: "tls",
				SNI:      "example.com",
				Name:     "TLS Server",
			},
		},
		{
			name:     "URL with WebSocket transport",
			vlessUrl: "vless://ec82bca8-1072-4682-822f-30306af408ea@example.com:443?type=ws&path=/path&host=example.com#WebSocket%20Server",
			expected: VlessConfig{
				UUID:    "ec82bca8-1072-4682-822f-30306af408ea",
				Address: "example.com",
				Port:    443,
				Type:    "ws",
				Name:    "WebSocket Server",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := parser.ParseUrl(tt.vlessUrl)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for URL: %s", tt.vlessUrl)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if config.UUID != tt.expected.UUID {
				t.Errorf("Expected UUID '%s', got '%s'", tt.expected.UUID, config.UUID)
			}

			if config.Address != tt.expected.Address {
				t.Errorf("Expected Address '%s', got '%s'", tt.expected.Address, config.Address)
			}

			if config.Port != tt.expected.Port {
				t.Errorf("Expected Port %d, got %d", tt.expected.Port, config.Port)
			}

			if config.Name != tt.expected.Name {
				t.Errorf("Expected Name '%s', got '%s'", tt.expected.Name, config.Name)
			}

			if tt.expected.Type != "" && config.Type != tt.expected.Type {
				t.Errorf("Expected Type '%s', got '%s'", tt.expected.Type, config.Type)
			}

			if tt.expected.Security != "" && config.Security != tt.expected.Security {
				t.Errorf("Expected Security '%s', got '%s'", tt.expected.Security, config.Security)
			}

			if tt.expected.SNI != "" && config.SNI != tt.expected.SNI {
				t.Errorf("Expected SNI '%s', got '%s'", tt.expected.SNI, config.SNI)
			}
		})
	}
}

// TestVlessParser_ToXrayOutbound_EdgeCases tests edge cases for xray outbound conversion
func TestVlessParser_ToXrayOutbound_EdgeCases(t *testing.T) {
	parser := NewVlessParser()

	tests := []struct {
		name     string
		config   VlessConfig
		expected map[string]interface{}
	}{
		{
			name: "TLS configuration",
			config: VlessConfig{
				UUID:     "ec82bca8-1072-4682-822f-30306af408ea",
				Address:  "example.com",
				Port:     443,
				Type:     "tcp",
				Security: "tls",
				SNI:      "example.com",
				Name:     "TLS Server",
			},
			expected: map[string]interface{}{
				"network":  "tcp",
				"security": "tls",
				"tlsSettings": map[string]interface{}{
					"serverName": "example.com",
				},
			},
		},
		{
			name: "None security configuration",
			config: VlessConfig{
				UUID:     "ec82bca8-1072-4682-822f-30306af408ea",
				Address:  "example.com",
				Port:     80,
				Type:     "tcp",
				Security: "none",
				Name:     "Plain Server",
			},
			expected: map[string]interface{}{
				"network": "tcp",
				// Note: "none" security doesn't create stream settings in the implementation
			},
		},
		{
			name: "WebSocket configuration without security",
			config: VlessConfig{
				UUID:    "ec82bca8-1072-4682-822f-30306af408ea",
				Address: "example.com",
				Port:    443,
				Type:    "ws",
				Name:    "WebSocket Server",
			},
			expected: map[string]interface{}{
				// Note: Without security, no stream settings are created
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := parser.ToXrayOutbound(tt.config)
			if err != nil {
				t.Fatalf("Failed to convert to xray outbound: %v", err)
			}

			// Check if we expect stream settings
			if len(tt.expected) == 0 {
				// No stream settings expected
				if server.StreamSettings != nil {
					t.Errorf("Expected no StreamSettings, but got: %v", server.StreamSettings)
				}
				return
			}

			// Verify stream settings exist when expected
			if server.StreamSettings == nil {
				t.Fatal("StreamSettings should not be nil")
			}

			for key, expectedValue := range tt.expected {
				actualValue, exists := server.StreamSettings[key]
				if !exists {
					t.Errorf("Expected key '%s' not found in StreamSettings", key)
					continue
				}

				// Handle nested maps
				if expectedMap, ok := expectedValue.(map[string]interface{}); ok {
					actualMap, ok := actualValue.(map[string]interface{})
					if !ok {
						t.Errorf("Expected '%s' to be a map, got %T", key, actualValue)
						continue
					}

					for nestedKey, nestedExpected := range expectedMap {
						nestedActual, exists := actualMap[nestedKey]
						if !exists {
							t.Errorf("Expected nested key '%s.%s' not found", key, nestedKey)
							continue
						}

						if nestedActual != nestedExpected {
							t.Errorf("Expected %s.%s '%v', got '%v'", key, nestedKey, nestedExpected, nestedActual)
						}
					}
				} else {
					if actualValue != expectedValue {
						t.Errorf("Expected %s '%v', got '%v'", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

// TestVlessParser_ExtractQueryParams tests query parameter extraction
func TestVlessParser_ExtractQueryParams(t *testing.T) {
	parser := NewVlessParser()

	tests := []struct {
		name        string
		rawQuery    string
		expected    map[string]string
		expectError bool
	}{
		{
			name:     "Empty query",
			rawQuery: "",
			expected: map[string]string{},
		},
		{
			name:     "Single parameter",
			rawQuery: "type=tcp",
			expected: map[string]string{"type": "tcp"},
		},
		{
			name:     "Multiple parameters",
			rawQuery: "type=tcp&security=reality&sni=example.com",
			expected: map[string]string{
				"type":     "tcp",
				"security": "reality",
				"sni":      "example.com",
			},
		},
		{
			name:     "URL encoded values",
			rawQuery: "sni=example.com&path=%2Fpath%2Fto%2Fws",
			expected: map[string]string{
				"sni":  "example.com",
				"path": "/path/to/ws",
			},
		},
		{
			name:     "Empty values",
			rawQuery: "type=tcp&empty=&security=tls",
			expected: map[string]string{
				"type":     "tcp",
				"empty":    "",
				"security": "tls",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ExtractQueryParams(tt.rawQuery)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parameters, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				actualValue, exists := result[key]
				if !exists {
					t.Errorf("Expected key '%s' not found", key)
					continue
				}

				if actualValue != expectedValue {
					t.Errorf("Expected %s='%s', got '%s'", key, expectedValue, actualValue)
				}
			}
		})
	}
}
