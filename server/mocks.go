package server
import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"xray-telegram-manager/config"
	"xray-telegram-manager/types"
)
type MockHTTPServer struct {
	server   *httptest.Server
	response string
	status   int
}
func NewMockHTTPServer(response string, status int) *MockHTTPServer {
	mock := &MockHTTPServer{
		response: response,
		status:   status,
	}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(mock.status)
		w.Write([]byte(mock.response))
	}))
	return mock
}
func (m *MockHTTPServer) URL() string {
	return m.server.URL
}
func (m *MockHTTPServer) Close() {
	m.server.Close()
}
func (m *MockHTTPServer) SetResponse(response string, status int) {
	m.response = response
	m.status = status
}
func CreateMockSubscriptionServer(vlessUrls []string) *MockHTTPServer {
	data := strings.Join(vlessUrls, "\n")
	base64Data := base64.StdEncoding.EncodeToString([]byte(data))
	return NewMockHTTPServer(base64Data, http.StatusOK)
}
type MockTCPServer struct {
	listener net.Listener
	address  string
	port     int
	running  bool
	delay    time.Duration
}
func NewMockTCPServer() (*MockTCPServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create mock TCP server: %w", err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	mock := &MockTCPServer{
		listener: listener,
		address:  addr.IP.String(),
		port:     addr.Port,
		running:  false,
		delay:    0,
	}
	return mock, nil
}
func (m *MockTCPServer) Start() {
	if m.running {
		return
	}
	m.running = true
	go func() {
		for m.running {
			conn, err := m.listener.Accept()
			if err != nil {
				if m.running {
					continue
				}
				return
			}
			if m.delay > 0 {
				time.Sleep(m.delay)
			}
			conn.Close()
		}
	}()
}
func (m *MockTCPServer) Stop() {
	if !m.running {
		return
	}
	m.running = false
	m.listener.Close()
}
func (m *MockTCPServer) Address() string {
	return m.address
}
func (m *MockTCPServer) Port() int {
	return m.port
}
func (m *MockTCPServer) SetDelay(delay time.Duration) {
	m.delay = delay
}
type MockPingTester struct {
	config      *config.Config
	mockServers map[string]*MockTCPServer
	results     map[string]types.PingResult
}
func NewMockPingTester(cfg *config.Config) *MockPingTester {
	return &MockPingTester{
		config:      cfg,
		mockServers: make(map[string]*MockTCPServer),
		results:     make(map[string]types.PingResult),
	}
}
func (m *MockPingTester) AddMockServer(serverID string, available bool, latency time.Duration) error {
	if available {
		mockServer, err := NewMockTCPServer()
		if err != nil {
			return err
		}
		mockServer.SetDelay(latency)
		mockServer.Start()
		m.mockServers[serverID] = mockServer
	}
	result := types.PingResult{
		Available: available,
		Latency:   int64(latency / time.Millisecond),
	}
	if !available {
		result.Error = fmt.Errorf("mock server unavailable")
	}
	m.results[serverID] = result
	return nil
}
func (m *MockPingTester) TestServer(server types.Server) types.PingResult {
	result, exists := m.results[server.ID]
	if !exists {
		return types.PingResult{
			Server:    server,
			Available: false,
			Latency:   0,
			Error:     fmt.Errorf("server not configured in mock"),
		}
	}
	result.Server = server
	return result
}
func (m *MockPingTester) TestServers(servers []types.Server) ([]types.PingResult, error) {
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers to test")
	}
	results := make([]types.PingResult, len(servers))
	for i, server := range servers {
		results[i] = m.TestServer(server)
	}
	return results, nil
}
func (m *MockPingTester) TestServersWithProgress(servers []types.Server, progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error) {
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers to test")
	}
	results := make([]types.PingResult, len(servers))
	for i, server := range servers {
		results[i] = m.TestServer(server)
		if progressCallback != nil {
			progressCallback(i+1, len(servers), server.Name)
		}
	}
	return results, nil
}
func (m *MockPingTester) SortByLatency(results []types.PingResult) []types.PingResult {
	realTester := NewPingTester(m.config)
	return realTester.SortByLatency(results)
}
func (m *MockPingTester) FormatResultsForTelegram(results []types.PingResult) string {
	realTester := NewPingTester(m.config)
	return realTester.FormatResultsForTelegram(results)
}
func (m *MockPingTester) GetAvailableServers(results []types.PingResult) []types.Server {
	realTester := NewPingTester(m.config)
	return realTester.GetAvailableServers(results)
}
func (m *MockPingTester) GetFastestServer(results []types.PingResult) (*types.Server, error) {
	realTester := NewPingTester(m.config)
	return realTester.GetFastestServer(results)
}
func (m *MockPingTester) Cleanup() {
	for _, mockServer := range m.mockServers {
		mockServer.Stop()
	}
	m.mockServers = make(map[string]*MockTCPServer)
	m.results = make(map[string]types.PingResult)
}
type MockSubscriptionLoader struct {
	config  *config.Config
	servers []types.Server
	error   error
}
func NewMockSubscriptionLoader(cfg *config.Config) *MockSubscriptionLoader {
	return &MockSubscriptionLoader{
		config:  cfg,
		servers: make([]types.Server, 0),
	}
}
func (m *MockSubscriptionLoader) SetServers(servers []types.Server) {
	m.servers = servers
}
func (m *MockSubscriptionLoader) SetError(err error) {
	m.error = err
}
func (m *MockSubscriptionLoader) LoadFromURL() ([]types.Server, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.servers, nil
}
func (m *MockSubscriptionLoader) GetCachedServers() []types.Server {
	return m.servers
}
func (m *MockSubscriptionLoader) InvalidateCache() {
}
func (m *MockSubscriptionLoader) DecodeBase64Config(data string) ([]types.Server, error) {
	realLoader := NewSubscriptionLoader(m.config)
	return realLoader.DecodeBase64Config(data)
}
