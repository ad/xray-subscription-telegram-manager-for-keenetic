# Xray Telegram Manager

Менеджер подключений xray с управлением через Telegram Bot для роутеров Keenetic.

## Project Structure

```
├── config/          # Configuration management
│   └── config.go    # Config loading and validation
├── telegram/        # Telegram bot interface
│   └── bot.go       # Bot handlers and commands
├── server/          # Server management
│   └── manager.go   # Server coordination and management
├── xray/            # Xray configuration management
│   └── controller.go # Xray config manipulation
├── logger/          # Logging system
│   └── logger.go    # Structured logging
├── main.go          # Main service entry point
├── go.mod           # Go module definition
└── go.sum           # Go module checksums
```

## Dependencies

- `github.com/go-telegram/bot` - Telegram Bot API client

## Building

```bash
# For local development
go build -o xray-telegram-manager

# For MIPS (Keenetic)
GOOS=linux GOARCH=mips GOMIPS=softfloat go build -ldflags="-s -w" -o xray-telegram-manager
```

## Configuration

The application expects a configuration file at `/opt/etc/xray-manager/config.json`:

```json
{
    "admin_id": 123456789,
    "bot_token": "your_bot_token_here",
    "config_path": "/opt/etc/xray/configs/04_outbounds.json",
    "subscription_url": "https://example.com/config.txt",
    "log_level": "info",
    "xray_restart_command": "/opt/etc/init.d/S24xray restart",
    "cache_duration": 3600,
    "health_check_interval": 300,
    "ping_timeout": 5
}
```

## Usage

```bash
# Run with default config path
./xray-telegram-manager

# Run with custom config path
./xray-telegram-manager /path/to/config.json
```