package server

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"xray-telegram-manager/types"
)

// VlessParser handles parsing of VLESS URLs
type VlessParser struct{}

// VlessConfig represents parsed VLESS configuration
type VlessConfig struct {
	UUID        string
	Address     string
	Port        int
	Type        string
	Security    string
	SNI         string
	PublicKey   string
	ShortID     string
	Fingerprint string
	Flow        string
	Name        string
}

// NewVlessParser creates a new VLESS parser instance
func NewVlessParser() *VlessParser {
	return &VlessParser{}
}

// ParseUrl parses a VLESS URL and extracts configuration parameters
func (vp *VlessParser) ParseUrl(vlessUrl string) (VlessConfig, error) {
	config := VlessConfig{}

	// Parse the URL
	parsedUrl, err := url.Parse(vlessUrl)
	if err != nil {
		return config, fmt.Errorf("failed to parse VLESS URL: %w", err)
	}

	// Verify protocol
	if parsedUrl.Scheme != "vless" {
		return config, fmt.Errorf("invalid protocol: expected 'vless', got '%s'", parsedUrl.Scheme)
	}

	// Extract UUID (username part)
	config.UUID = parsedUrl.User.Username()
	if config.UUID == "" {
		return config, fmt.Errorf("UUID not found in VLESS URL")
	}

	// Extract address and port
	config.Address = parsedUrl.Hostname()
	if config.Address == "" {
		return config, fmt.Errorf("address not found in VLESS URL")
	}

	portStr := parsedUrl.Port()
	if portStr == "" {
		config.Port = 443 // Default port for VLESS
	} else {
		config.Port, err = strconv.Atoi(portStr)
		if err != nil {
			return config, fmt.Errorf("invalid port: %w", err)
		}
	}

	// Extract query parameters
	queryParams, err := vp.ExtractQueryParams(parsedUrl.RawQuery)
	if err != nil {
		return config, fmt.Errorf("failed to extract query parameters: %w", err)
	}

	// Map query parameters to config fields
	config.Type = queryParams["type"]
	config.Security = queryParams["security"]
	config.SNI = queryParams["sni"]
	config.PublicKey = queryParams["pbk"]
	config.ShortID = queryParams["sid"]
	config.Fingerprint = queryParams["fp"]
	config.Flow = queryParams["flow"]

	// Extract server name from URL fragment
	if parsedUrl.Fragment != "" {
		config.Name = parsedUrl.Fragment
	}

	return config, nil
}

// ExtractQueryParams parses URL query parameters into a map
func (vp *VlessParser) ExtractQueryParams(rawQuery string) (map[string]string, error) {
	params := make(map[string]string)

	if rawQuery == "" {
		return params, nil
	}

	// Parse query parameters
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query parameters: %w", err)
	}

	// Convert to simple string map (take first value for each key)
	for key, valueList := range values {
		if len(valueList) > 0 {
			params[key] = valueList[0]
		}
	}

	return params, nil
}

// ToXrayOutbound converts VlessConfig to Server struct with xray outbound format
func (vp *VlessParser) ToXrayOutbound(config VlessConfig) (types.Server, error) {
	server := types.Server{
		ID:       generateServerID(config),
		Name:     config.Name,
		VlessUrl: "", // Will be set by caller
		Tag:      "vless-reality",
		Protocol: "vless",
		Address:  config.Address,
		Port:     config.Port,
	}

	// Build settings for VLESS protocol
	settings := map[string]interface{}{
		"vnext": []map[string]interface{}{
			{
				"address": config.Address,
				"port":    config.Port,
				"users": []map[string]interface{}{
					{
						"id":         config.UUID,
						"encryption": "none",
						"level":      0,
					},
				},
			},
		},
	}

	// Add flow if specified
	if config.Flow != "" {
		vnext := settings["vnext"].([]map[string]interface{})
		users := vnext[0]["users"].([]map[string]interface{})
		users[0]["flow"] = config.Flow
	}

	server.Settings = settings

	// Build stream settings based on security type
	if config.Security != "" {
		streamSettings := map[string]interface{}{
			"network": config.Type,
		}

		switch config.Security {
		case "reality":
			streamSettings["security"] = "reality"
			realitySettings := map[string]interface{}{
				"spiderX": "/",
			}

			// Map Reality-specific parameters
			if config.PublicKey != "" {
				realitySettings["publicKey"] = config.PublicKey
			}
			if config.SNI != "" {
				realitySettings["serverName"] = config.SNI
			}
			if config.ShortID != "" {
				realitySettings["shortId"] = config.ShortID
			}
			if config.Fingerprint != "" {
				realitySettings["fingerprint"] = config.Fingerprint
			}

			streamSettings["realitySettings"] = realitySettings
		case "tls":
			streamSettings["security"] = "tls"
			tlsSettings := map[string]interface{}{}

			if config.SNI != "" {
				tlsSettings["serverName"] = config.SNI
			}
			if config.Fingerprint != "" {
				tlsSettings["fingerprint"] = config.Fingerprint
			}

			streamSettings["tlsSettings"] = tlsSettings
		}

		server.StreamSettings = streamSettings
	}

	return server, nil
}

// generateServerID creates a unique ID for the server based on address and port
func generateServerID(config VlessConfig) string {
	// Use address:port as base ID, replace dots and colons with underscores
	id := strings.ReplaceAll(config.Address, ".", "_")
	id = strings.ReplaceAll(id, ":", "_")
	return fmt.Sprintf("%s_%d", id, config.Port)
}
