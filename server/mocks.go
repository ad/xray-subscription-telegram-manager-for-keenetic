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

// MockHTTPServer создает тестовый HTTP сервер для subscription loader
type MockHTTPServer struct {
	server   *httptest.Server
	response string
	status   int
}

// NewMockHTTPServer создает новый мок HTTP сервер
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

// URL возвращает URL мок сервера
func (m *MockHTTPServer) URL() string {
	return m.server.URL
}

// Close закрывает мок сервер
func (m *MockHTTPServer) Close() {
	m.server.Close()
}

// SetResponse изменяет ответ сервера
func (m *MockHTTPServer) SetResponse(response string, status int) {
	m.response = response
	m.status = status
}

// CreateMockSubscriptionServer создает мок сервер с VLESS URL данными
func CreateMockSubscriptionServer(vlessUrls []string) *MockHTTPServer {
	data := strings.Join(vlessUrls, "\n")
	base64Data := base64.StdEncoding.EncodeToString([]byte(data))
	return NewMockHTTPServer(base64Data, http.StatusOK)
}

// MockTCPServer создает тестовый TCP сервер для ping тестов
type MockTCPServer struct {
	listener net.Listener
	address  string
	port     int
	running  bool
	delay    time.Duration
}

// NewMockTCPServer создает новый мок TCP сервер
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

// Start запускает мок TCP сервер
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

			// Добавляем задержку если настроена
			if m.delay > 0 {
				time.Sleep(m.delay)
			}

			// Сразу закрываем соединение (имитируем успешное подключение)
			conn.Close()
		}
	}()
}

// Stop останавливает мок TCP сервер
func (m *MockTCPServer) Stop() {
	if !m.running {
		return
	}

	m.running = false
	m.listener.Close()
}

// Address возвращает адрес сервера
func (m *MockTCPServer) Address() string {
	return m.address
}

// Port возвращает порт сервера
func (m *MockTCPServer) Port() int {
	return m.port
}

// SetDelay устанавливает задержку для имитации медленного сервера
func (m *MockTCPServer) SetDelay(delay time.Duration) {
	m.delay = delay
}

// MockPingTester создает мок для ping тестирования
type MockPingTester struct {
	config      *config.Config
	mockServers map[string]*MockTCPServer
	results     map[string]types.PingResult
}

// NewMockPingTester создает новый мок ping tester
func NewMockPingTester(cfg *config.Config) *MockPingTester {
	return &MockPingTester{
		config:      cfg,
		mockServers: make(map[string]*MockTCPServer),
		results:     make(map[string]types.PingResult),
	}
}

// AddMockServer добавляет мок сервер для тестирования
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

// TestServer тестирует конкретный сервер (мок версия)
func (m *MockPingTester) TestServer(server types.Server) types.PingResult {
	result, exists := m.results[server.ID]
	if !exists {
		// Если результат не настроен, возвращаем недоступный
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

// TestServers тестирует список серверов (мок версия)
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

// TestServersWithProgress тестирует серверы с прогрессом (мок версия)
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

// SortByLatency сортирует результаты по задержке
func (m *MockPingTester) SortByLatency(results []types.PingResult) []types.PingResult {
	// Используем реальную реализацию из PingTesterImpl
	realTester := NewPingTester(m.config)
	return realTester.SortByLatency(results)
}

// FormatResultsForTelegram форматирует результаты для Telegram
func (m *MockPingTester) FormatResultsForTelegram(results []types.PingResult) string {
	// Используем реальную реализацию из PingTesterImpl
	realTester := NewPingTester(m.config)
	return realTester.FormatResultsForTelegram(results)
}

// GetAvailableServers возвращает доступные серверы
func (m *MockPingTester) GetAvailableServers(results []types.PingResult) []types.Server {
	// Используем реальную реализацию из PingTesterImpl
	realTester := NewPingTester(m.config)
	return realTester.GetAvailableServers(results)
}

// GetFastestServer возвращает самый быстрый сервер
func (m *MockPingTester) GetFastestServer(results []types.PingResult) (*types.Server, error) {
	// Используем реальную реализацию из PingTesterImpl
	realTester := NewPingTester(m.config)
	return realTester.GetFastestServer(results)
}

// Cleanup очищает все мок серверы
func (m *MockPingTester) Cleanup() {
	for _, mockServer := range m.mockServers {
		mockServer.Stop()
	}
	m.mockServers = make(map[string]*MockTCPServer)
	m.results = make(map[string]types.PingResult)
}

// MockSubscriptionLoader создает мок для subscription loader
type MockSubscriptionLoader struct {
	config  *config.Config
	servers []types.Server
	error   error
}

// NewMockSubscriptionLoader создает новый мок subscription loader
func NewMockSubscriptionLoader(cfg *config.Config) *MockSubscriptionLoader {
	return &MockSubscriptionLoader{
		config:  cfg,
		servers: make([]types.Server, 0),
	}
}

// SetServers устанавливает серверы для возврата
func (m *MockSubscriptionLoader) SetServers(servers []types.Server) {
	m.servers = servers
}

// SetError устанавливает ошибку для возврата
func (m *MockSubscriptionLoader) SetError(err error) {
	m.error = err
}

// LoadFromURL загружает серверы (мок версия)
func (m *MockSubscriptionLoader) LoadFromURL() ([]types.Server, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.servers, nil
}

// GetCachedServers возвращает кэшированные серверы
func (m *MockSubscriptionLoader) GetCachedServers() []types.Server {
	return m.servers
}

// InvalidateCache инвалидирует кэш (мок версия)
func (m *MockSubscriptionLoader) InvalidateCache() {
	// В моке ничего не делаем
}

// DecodeBase64Config декодирует base64 конфигурацию (используем реальную реализацию)
func (m *MockSubscriptionLoader) DecodeBase64Config(data string) ([]types.Server, error) {
	realLoader := NewSubscriptionLoader(m.config)
	return realLoader.DecodeBase64Config(data)
}
