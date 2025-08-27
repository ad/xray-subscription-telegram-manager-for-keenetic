package server
import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"xray-telegram-manager/types"
)
type VlessParser struct{}
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
func NewVlessParser() *VlessParser {
	return &VlessParser{}
}
func (vp *VlessParser) ParseUrl(vlessUrl string) (VlessConfig, error) {
	config := VlessConfig{}
	if vlessUrl == "" {
		return config, fmt.Errorf("VLESS URL is empty")
	}
	const maxURLLength = 2048
	if len(vlessUrl) > maxURLLength {
		return config, fmt.Errorf("VLESS URL too long (max %d characters)", maxURLLength)
	}
	parsedUrl, err := url.Parse(vlessUrl)
	if err != nil {
		return config, fmt.Errorf("failed to parse VLESS URL: %w", err)
	}
	if parsedUrl.Scheme != "vless" {
		return config, fmt.Errorf("invalid protocol: expected 'vless', got '%s'", parsedUrl.Scheme)
	}
	config.UUID = parsedUrl.User.Username()
	if config.UUID == "" {
		return config, fmt.Errorf("UUID not found in VLESS URL")
	}
	if err := vp.validateUUID(config.UUID); err != nil {
		return config, fmt.Errorf("invalid UUID format: %w", err)
	}
	config.Address = parsedUrl.Hostname()
	if config.Address == "" {
		return config, fmt.Errorf("address not found in VLESS URL")
	}
	if err := vp.validateAddress(config.Address); err != nil {
		return config, fmt.Errorf("invalid address: %w", err)
	}
	portStr := parsedUrl.Port()
	if portStr == "" {
		config.Port = 443 // Default port for VLESS
	} else {
		config.Port, err = strconv.Atoi(portStr)
		if err != nil {
			return config, fmt.Errorf("invalid port: %w", err)
		}
		if config.Port < 1 || config.Port > 65535 {
			return config, fmt.Errorf("port out of range: %d (must be 1-65535)", config.Port)
		}
	}
	queryParams, err := vp.ExtractQueryParams(parsedUrl.RawQuery)
	if err != nil {
		return config, fmt.Errorf("failed to extract query parameters: %w", err)
	}
	config.Type = vp.sanitizeString(queryParams["type"], 32)
	config.Security = vp.sanitizeString(queryParams["security"], 32)
	config.SNI = vp.sanitizeString(queryParams["sni"], 256)
	config.PublicKey = vp.sanitizeString(queryParams["pbk"], 256)
	config.ShortID = vp.sanitizeString(queryParams["sid"], 32)
	config.Fingerprint = vp.sanitizeString(queryParams["fp"], 32)
	config.Flow = vp.sanitizeString(queryParams["flow"], 32)
	if parsedUrl.Fragment != "" {
		config.Name = vp.sanitizeString(parsedUrl.Fragment, 256)
	}
	if config.Name == "" {
		config.Name = fmt.Sprintf("%s:%d", config.Address, config.Port)
	}
	return config, nil
}
func (vp *VlessParser) ExtractQueryParams(rawQuery string) (map[string]string, error) {
	params := make(map[string]string)
	if rawQuery == "" {
		return params, nil
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query parameters: %w", err)
	}
	for key, valueList := range values {
		if len(valueList) > 0 {
			params[key] = valueList[0]
		}
	}
	return params, nil
}
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
	if config.Flow != "" {
		vnext := settings["vnext"].([]map[string]interface{})
		users := vnext[0]["users"].([]map[string]interface{})
		users[0]["flow"] = config.Flow
	}
	server.Settings = settings
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
func generateServerID(config VlessConfig) string {
	id := strings.ReplaceAll(config.Address, ".", "_")
	id = strings.ReplaceAll(id, ":", "_")
	return fmt.Sprintf("%s_%d", id, config.Port)
}
func (vp *VlessParser) validateUUID(uuid string) error {
	if uuid == "" {
		return fmt.Errorf("UUID is empty")
	}
	if len(uuid) != 36 && len(uuid) != 32 {
		return fmt.Errorf("UUID has invalid length: %d (expected 32 or 36)", len(uuid))
	}
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)
	if !uuidRegex.MatchString(uuid) {
		return fmt.Errorf("UUID has invalid format")
	}
	return nil
}
func (vp *VlessParser) validateAddress(address string) error {
	if address == "" {
		return fmt.Errorf("address is empty")
	}
	if ip := net.ParseIP(address); ip != nil {
		return nil
	}
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	if !hostnameRegex.MatchString(address) {
		return fmt.Errorf("invalid hostname/IP address format")
	}
	return nil
}
func (vp *VlessParser) sanitizeString(s string, maxLen int) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\x0c", "")
	s = strings.ReplaceAll(s, "\x0b", "")
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "&", "")
	s = strings.ReplaceAll(s, "|", "")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}
