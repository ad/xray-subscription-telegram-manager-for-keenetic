# Deployment Scripts

This directory contains scripts for building, deploying, and managing xray-telegram-manager on Keenetic routers.

## Scripts Overview

### Build Scripts
- **`build.sh`** - Advanced build script with cross-compilation support
- **`Makefile`** - Make-based build system for easy compilation

### Deployment Scripts
- **`deploy.sh`** - Comprehensive deployment script with remote deployment support
- **`install.sh`** - Installation script for Keenetic routers
- **`uninstall.sh`** - Uninstallation script
- **`update.sh`** - Update script with backup and rollback support

### Configuration Scripts
- **`generate-config.sh`** - Configuration template generator

### Service Files
- **`xray-telegram-manager.service`** - Systemd service file

## Quick Start

### 1. Build for MIPS (Keenetic)
```bash
# Using Makefile (recommended)
make mips

# Or using build script
./scripts/build.sh -t mips
```

### 2. Deploy to Router
```bash
# Create deployment package
./scripts/deploy.sh

# Or deploy directly to router
./scripts/deploy.sh -t 192.168.1.1 -u root
```

### 3. Install on Router
```bash
# Copy package to router first, then:
./scripts/install.sh
```

## Detailed Usage

### Build Script (`build.sh`)

Cross-compilation build script with optimization and compression support.

```bash
# Build all MIPS variants
./scripts/build.sh

# Build specific target
./scripts/build.sh -t mips-softfloat

# Build with compression
./scripts/build.sh -t mips --compress

# Clean build
./scripts/build.sh --clean -t mips
```

**Options:**
- `-t, --target` - Build target (mips-softfloat, mips-hardfloat, linux-amd64, all)
- `-o, --output` - Output directory (default: ./dist)
- `-c, --clean` - Clean before build
- `--no-compress` - Skip UPX compression

### Makefile Targets

```bash
# Build for MIPS (both variants)
make mips

# Build specific variant
make mips-softfloat
make mips-hardfloat

# Build for testing
make linux-amd64

# Full release build
make release

# Run tests
make test

# Format and lint
make check
```

### Deployment Script (`deploy.sh`)

Comprehensive deployment with packaging and remote deployment support.

```bash
# Build and package
./scripts/deploy.sh

# Deploy to remote router
./scripts/deploy.sh -t 192.168.1.1 -u root -p 22

# Build only
./scripts/deploy.sh --build-only

# Package only (skip build)
./scripts/deploy.sh --package-only
```

**Options:**
- `-t, --target HOST` - Deploy to remote host
- `-u, --user USER` - SSH user (default: root)
- `-p, --port PORT` - SSH port (default: 22)
- `-k, --key FILE` - SSH private key
- `--build-only` - Only build binaries
- `--package-only` - Only create package
- `--clean` - Clean before build

### Installation Script (`install.sh`)

Installs xray-telegram-manager on Keenetic routers.

```bash
# Standard installation
./scripts/install.sh

# Force reinstallation
./scripts/install.sh --force

# Skip service installation
./scripts/install.sh --no-service

# Only create configuration
./scripts/install.sh --config-only
```

**Features:**
- Automatic system detection (OpenWrt/systemd)
- Service installation and configuration
- Directory structure creation
- Configuration template generation
- Security settings application

### Update Script (`update.sh`)

Updates existing installation with backup and rollback support.

```bash
# Interactive update
./scripts/update.sh

# Force update
./scripts/update.sh --force

# Update without restart
./scripts/update.sh --no-restart

# Check current version
./scripts/update.sh --check

# Rollback to previous version
./scripts/update.sh --rollback
```

**Features:**
- Automatic backup creation
- Service management
- Version checking
- Rollback capability
- Configuration preservation

### Uninstall Script (`uninstall.sh`)

Removes xray-telegram-manager from the system.

```bash
# Interactive uninstallation
./scripts/uninstall.sh

# Force uninstallation
./scripts/uninstall.sh --force

# Keep configuration and logs
./scripts/uninstall.sh --keep-data
```

### Configuration Generator (`generate-config.sh`)

Generates configuration templates for different environments.

```bash
# Basic template
./scripts/generate-config.sh

# Interactive configuration
./scripts/generate-config.sh -i

# Keenetic-specific config
./scripts/generate-config.sh --keenetic

# OpenWrt-specific config
./scripts/generate-config.sh --openwrt

# Custom output path
./scripts/generate-config.sh -o /path/to/config.json
```

## Directory Structure After Installation

```
/opt/etc/xray-manager/
├── xray-telegram-manager          # Main binary
├── config.json                    # Configuration file
├── scripts/                       # Management scripts
│   ├── install.sh
│   ├── uninstall.sh
│   └── update.sh
├── logs/                          # Log files
│   └── app.log
├── cache/                         # Cache directory
│   └── servers.json
└── backup/                        # Backup directory
    ├── backup_20240825_120000_binary
    └── backup_20240825_120000_config.json
```

## Service Management

### Systemd (if available)
```bash
# Start service
systemctl start xray-telegram-manager

# Stop service
systemctl stop xray-telegram-manager

# Enable auto-start
systemctl enable xray-telegram-manager

# Check status
systemctl status xray-telegram-manager

# View logs
journalctl -u xray-telegram-manager -f
```

### OpenWrt/Keenetic
```bash
# Start service
/opt/etc/init.d/S99xray-telegram-manager start

# Stop service
/opt/etc/init.d/S99xray-telegram-manager stop

# Restart service
/opt/etc/init.d/S99xray-telegram-manager restart

# Check status
/opt/etc/init.d/S99xray-telegram-manager status
```

## Configuration

### Required Settings
```json
{
    "admin_id": 123456789,              // Your Telegram user ID
    "bot_token": "123:ABC-DEF...",      // Bot token from @BotFather
    "subscription_url": "https://...",   // VLESS subscription URL
    "config_path": "/opt/etc/xray/configs/04_outbounds.json"
}
```

### Optional Settings
```json
{
    "log_level": "info",                           // debug, info, warn, error
    "xray_restart_command": "/opt/etc/init.d/S24xray restart",
    "cache_duration": 3600,                        // Cache duration in seconds
    "health_check_interval": 300,                  // Health check interval
    "ping_timeout": 5                              // Ping timeout in seconds
}
```

## Troubleshooting

### Build Issues
```bash
# Check Go installation
go version

# Test compilation
make test-compile

# Clean and rebuild
make clean && make mips
```

### Installation Issues
```bash
# Check system compatibility
uname -a

# Check available space
df -h /opt

# Check permissions
ls -la /opt/etc/
```

### Runtime Issues
```bash
# Check logs
tail -f /opt/etc/xray-manager/logs/app.log

# Check service status
/opt/etc/init.d/S99xray-telegram-manager status

# Test configuration
/opt/etc/xray-manager/xray-telegram-manager --check-config
```

### Network Issues
```bash
# Test connectivity
ping google.com

# Check xray status
/opt/etc/init.d/S24xray status

# Check firewall
iptables -L
```

## Security Considerations

1. **Configuration File Permissions**: Config files are set to 600 (owner read/write only)
2. **Service User**: Runs as root (required for xray management)
3. **Network Access**: Only outbound connections required
4. **Telegram Bot**: Only responds to configured admin ID
5. **File System**: Uses protected directories under /opt/etc/

## Development

### Adding New Features
1. Modify source code
2. Update build configuration if needed
3. Test with `make test`
4. Build with `make mips`
5. Test deployment with `./scripts/deploy.sh --build-only`

### Testing Scripts
```bash
# Test build script
./scripts/build.sh --help

# Test installation (dry run)
./scripts/install.sh --config-only

# Test configuration generator
./scripts/generate-config.sh --template
```

## Support

For issues and questions:
1. Check logs: `/opt/etc/xray-manager/logs/app.log`
2. Verify configuration: `/opt/etc/xray-manager/config.json`
3. Test connectivity and xray status
4. Check GitHub issues and documentation