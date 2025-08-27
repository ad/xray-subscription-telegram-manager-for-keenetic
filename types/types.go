package types
type Server struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	VlessUrl       string                 `json:"vlessUrl"`
	Tag            string                 `json:"tag"`
	Protocol       string                 `json:"protocol"`
	Address        string                 `json:"address"`
	Port           int                    `json:"port"`
	Settings       map[string]interface{} `json:"settings"`
	StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
}
type PingResult struct {
	Server    Server
	Latency   int64 // in milliseconds
	Available bool
	Error     error
}
type XrayConfig struct {
	Outbounds []XrayOutbound `json:"outbounds"`
}
type XrayOutbound struct {
	Tag            string                 `json:"tag"`
	Protocol       string                 `json:"protocol"`
	Settings       map[string]interface{} `json:"settings,omitempty"`
	StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
}
