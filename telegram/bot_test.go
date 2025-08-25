package telegram

import (
	"testing"
	"xray-telegram-manager/types"

	"github.com/go-telegram/bot/models"
)

// MockConfig implements ConfigProvider for testing
type MockConfig struct {
	adminID  int64
	botToken string
}

func (m *MockConfig) GetAdminID() int64 {
	return m.adminID
}

func (m *MockConfig) GetBotToken() string {
	return m.botToken
}

// MockServerManager implements ServerManager for testing
type MockServerManager struct {
	servers       []types.Server
	currentServer *types.Server
}

func (m *MockServerManager) LoadServers() error {
	return nil
}

func (m *MockServerManager) GetServers() []types.Server {
	return m.servers
}

func (m *MockServerManager) SwitchServer(serverID string) error {
	for _, server := range m.servers {
		if server.ID == serverID {
			m.currentServer = &server
			return nil
		}
	}
	return nil
}

func (m *MockServerManager) GetCurrentServer() *types.Server {
	return m.currentServer
}

func (m *MockServerManager) TestPing() ([]types.PingResult, error) {
	results := make([]types.PingResult, len(m.servers))
	for i, server := range m.servers {
		results[i] = types.PingResult{
			Server:    server,
			Latency:   100,
			Available: true,
			Error:     nil,
		}
	}
	return results, nil
}

func (m *MockServerManager) TestPingWithProgress(progressCallback func(completed, total int, serverName string)) ([]types.PingResult, error) {
	results := make([]types.PingResult, len(m.servers))
	for i, server := range m.servers {
		results[i] = types.PingResult{
			Server:    server,
			Latency:   100,
			Available: true,
			Error:     nil,
		}
		if progressCallback != nil {
			progressCallback(i+1, len(m.servers), server.Name)
		}
	}
	return results, nil
}

// MockTelegramBot for testing without actual API calls
type MockTelegramBot struct {
	config    ConfigProvider
	serverMgr ServerManager
}

func NewMockTelegramBot(config ConfigProvider, serverMgr ServerManager) *MockTelegramBot {
	return &MockTelegramBot{
		config:    config,
		serverMgr: serverMgr,
	}
}

func (tb *MockTelegramBot) isAuthorized(userID int64) bool {
	return userID == tb.config.GetAdminID()
}

func (tb *MockTelegramBot) createMainMenuKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 Server List", CallbackData: "refresh"},
				{Text: "📊 Ping Test", CallbackData: "ping_test"},
			},
		},
	}
}

func (tb *MockTelegramBot) createServerListKeyboard(servers []types.Server, _ int) *models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton

	// Add server buttons
	for _, server := range servers {
		buttonText := "🌐 " + server.Name
		if len(buttonText) > 30 {
			buttonText = buttonText[:27] + "..."
		}

		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: buttonText, CallbackData: "server_" + server.ID},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func TestMockTelegramBot(t *testing.T) {
	config := &MockConfig{
		adminID:  123456789,
		botToken: "test_token",
	}

	serverMgr := &MockServerManager{
		servers: []types.Server{
			{
				ID:       "server1",
				Name:     "Test Server 1",
				Address:  "1.2.3.4",
				Port:     443,
				Protocol: "vless",
				Tag:      "vless-test",
			},
		},
	}

	bot := NewMockTelegramBot(config, serverMgr)
	if bot == nil {
		t.Fatal("Bot should not be nil")
	}

	if bot.config != config {
		t.Error("Bot config should match provided config")
	}

	if bot.serverMgr != serverMgr {
		t.Error("Bot server manager should match provided server manager")
	}
}

func TestIsAuthorized(t *testing.T) {
	config := &MockConfig{
		adminID:  123456789,
		botToken: "test_token",
	}

	serverMgr := &MockServerManager{}

	bot := NewMockTelegramBot(config, serverMgr)

	// Test authorized user
	if !bot.isAuthorized(123456789) {
		t.Error("Admin user should be authorized")
	}

	// Test unauthorized user
	if bot.isAuthorized(987654321) {
		t.Error("Non-admin user should not be authorized")
	}
}

func TestCreateMainMenuKeyboard(t *testing.T) {
	config := &MockConfig{
		adminID:  123456789,
		botToken: "test_token",
	}

	serverMgr := &MockServerManager{}

	bot := NewMockTelegramBot(config, serverMgr)

	keyboard := bot.createMainMenuKeyboard()
	if keyboard == nil {
		t.Fatal("Keyboard should not be nil")
	}

	if len(keyboard.InlineKeyboard) == 0 {
		t.Error("Keyboard should have at least one row")
	}

	// Check if the main menu buttons are present
	found := false
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == "refresh" || button.CallbackData == "ping_test" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("Main menu keyboard should contain refresh or ping_test buttons")
	}
}

func TestCreateServerListKeyboard(t *testing.T) {
	config := &MockConfig{
		adminID:  123456789,
		botToken: "test_token",
	}

	servers := []types.Server{
		{
			ID:       "server1",
			Name:     "Test Server 1",
			Address:  "1.2.3.4",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-test1",
		},
		{
			ID:       "server2",
			Name:     "Test Server 2",
			Address:  "5.6.7.8",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-test2",
		},
	}

	serverMgr := &MockServerManager{servers: servers}

	bot := NewMockTelegramBot(config, serverMgr)

	keyboard := bot.createServerListKeyboard(servers, 0)
	if keyboard == nil {
		t.Fatal("Keyboard should not be nil")
	}

	if len(keyboard.InlineKeyboard) == 0 {
		t.Error("Keyboard should have at least one row")
	}

	// Check if server buttons are present
	serverButtonCount := 0
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if len(button.CallbackData) > 7 && button.CallbackData[:7] == "server_" {
				serverButtonCount++
			}
		}
	}

	if serverButtonCount != len(servers) {
		t.Errorf("Expected %d server buttons, got %d", len(servers), serverButtonCount)
	}
}

func TestConfigProvider(t *testing.T) {
	config := &MockConfig{
		adminID:  123456789,
		botToken: "test_token",
	}

	if config.GetAdminID() != 123456789 {
		t.Error("Config admin ID should be 123456789")
	}

	if config.GetBotToken() != "test_token" {
		t.Error("Config bot token should be test_token")
	}
}

func TestServerManager(t *testing.T) {
	servers := []types.Server{
		{
			ID:       "server1",
			Name:     "Test Server 1",
			Address:  "1.2.3.4",
			Port:     443,
			Protocol: "vless",
			Tag:      "vless-test",
		},
	}

	serverMgr := &MockServerManager{servers: servers}

	if len(serverMgr.GetServers()) != 1 {
		t.Error("Server manager should have 1 server")
	}

	err := serverMgr.SwitchServer("server1")
	if err != nil {
		t.Errorf("Should be able to switch to server1: %v", err)
	}

	currentServer := serverMgr.GetCurrentServer()
	if currentServer == nil {
		t.Fatal("Current server should not be nil after switching")
	}

	if currentServer.ID != "server1" {
		t.Error("Current server ID should be server1")
	}

	results, err := serverMgr.TestPing()
	if err != nil {
		t.Errorf("Ping test should not fail: %v", err)
	}

	if len(results) != 1 {
		t.Error("Ping results should have 1 result")
	}

	if !results[0].Available {
		t.Error("Server should be available in ping test")
	}
}
