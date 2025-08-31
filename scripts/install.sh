#!/bin/sh

# Installation script for xray-telegram-manager
# Supports Keenetic routers and other OpenWrt-based systems

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/etc/xray-manager"
ALT_INSTALL_DIR="/opt/bin"
CONFIG_FILE="$INSTALL_DIR/config.json"
SERVICE_FILE="/opt/etc/init.d/S99xray-telegram-manager"
SYSTEMD_SERVICE_FILE="/etc/systemd/system/xray-telegram-manager.service"
BINARY_NAME="xray-telegram-manager"
LOG_DIR="$INSTALL_DIR/logs"
CACHE_DIR="$INSTALL_DIR/cache"

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Function to check if running as root
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

# Function to find binary location
find_binary() {
    # Check primary location
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        echo "$INSTALL_DIR/$BINARY_NAME"
        return 0
    fi
    
    # Check alternative location
    if [ -f "$ALT_INSTALL_DIR/$BINARY_NAME" ]; then
        echo "$ALT_INSTALL_DIR/$BINARY_NAME"
        return 0
    fi
    
    # Check if it's in PATH
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        command -v "$BINARY_NAME"
        return 0
    fi
    
    return 1
}

# Function to detect system type
detect_system() {
    if [ -f /etc/openwrt_release ]; then
        echo "openwrt"
    elif [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "$ID"
    else
        echo "unknown"
    fi
}

# Function to check if systemd is available
has_systemd() {
    command -v systemctl >/dev/null 2>&1 && [ -d /etc/systemd/system ]
}

# Function to create directories
create_directories() {
    print_step "Creating directories..."
    
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$CACHE_DIR"
    
    # Set permissions
    chmod 755 "$INSTALL_DIR"
    chmod 755 "$LOG_DIR"
    chmod 755 "$CACHE_DIR"
    
    print_info "✓ Directories created"
}

# Function to install binary
install_binary() {
    print_step "Installing binary..."
    
    local binary_path=""
    
    # Try to find the appropriate binary
    if [ -f "./dist/${BINARY_NAME}-mips-softfloat" ]; then
        binary_path="./dist/${BINARY_NAME}-mips-softfloat"
    elif [ -f "./dist/${BINARY_NAME}-mips-hardfloat" ]; then
        binary_path="./dist/${BINARY_NAME}-mips-hardfloat"
    elif [ -f "./${BINARY_NAME}" ]; then
        binary_path="./${BINARY_NAME}"
    else
        print_error "Binary not found. Please build the project first."
        print_info "Run: make mips"
        exit 1
    fi
    
    # Copy binary
    cp "$binary_path" "$INSTALL_DIR/$BINARY_NAME"
    chmod 755 "$INSTALL_DIR/$BINARY_NAME"
    
    print_info "✓ Binary installed: $INSTALL_DIR/$BINARY_NAME"
}

# Function to create configuration template
create_config_template() {
    print_step "Creating configuration template..."
    
    if [ -f "$CONFIG_FILE" ]; then
        print_warn "Configuration file already exists: $CONFIG_FILE"
        print_info "Creating backup: ${CONFIG_FILE}.backup"
        cp "$CONFIG_FILE" "${CONFIG_FILE}.backup"
    fi
    
    cat > "$CONFIG_FILE" << 'EOF'
{
    "admin_id": 0,
    "bot_token": "YOUR_BOT_TOKEN_HERE",
    "config_path": "/opt/etc/xray/configs/04_outbounds.json",
    "subscription_url": "YOUR_SUBSCRIPTION_URL_HERE",
    "log_level": "info",
    "xray_restart_command": "/opt/etc/init.d/S24xray restart",
    "cache_duration": 3600,
    "health_check_interval": 300,
    "ping_timeout": 5
}
EOF
    
    chmod 600 "$CONFIG_FILE"
    
    print_info "✓ Configuration template created: $CONFIG_FILE"
    print_warn "Please edit the configuration file and set your bot token and admin ID"
}

# Function to install systemd service
install_systemd_service() {
    print_step "Installing systemd service..."
    
    if [ -f "./scripts/xray-telegram-manager.service" ]; then
        cp "./scripts/xray-telegram-manager.service" "$SYSTEMD_SERVICE_FILE"
        chmod 644 "$SYSTEMD_SERVICE_FILE"
        
        systemctl daemon-reload
        systemctl enable xray-telegram-manager.service
        
        print_info "✓ Systemd service installed and enabled"
    else
        print_error "Systemd service file not found: ./scripts/xray-telegram-manager.service"
        return 1
    fi
}

# Function to install OpenWrt init script
install_openwrt_service() {
    print_step "Installing OpenWrt init script..."
    
    cat > "$SERVICE_FILE" << EOF
#!/bin/sh /etc/rc.common

START=99
STOP=10

USE_PROCD=1
PROG="$INSTALL_DIR/$BINARY_NAME"
CONF="$CONFIG_FILE"

start_service() {
    procd_open_instance
    procd_set_param command "\$PROG"
    procd_set_param respawn \${respawn_threshold:-3600} \${respawn_timeout:-5} \${respawn_retry:-5}
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param user root
    procd_set_param pidfile /var/run/xray-telegram-manager.pid
    procd_close_instance
}

stop_service() {
    killall -9 $BINARY_NAME 2>/dev/null || true
}

reload_service() {
    stop
    start
}
EOF
    
    chmod 755 "$SERVICE_FILE"
    
    print_info "✓ OpenWrt init script installed: $SERVICE_FILE"
}

# Function to show post-installation instructions
show_instructions() {
    print_step "Installation completed!"
    echo ""
    print_info "Next steps:"
    echo "1. Edit the configuration file: $CONFIG_FILE"
    echo "   - Set your Telegram bot token"
    echo "   - Set your Telegram admin ID"
    echo "   - Configure subscription URL"
    echo ""
    
    local system_type=$(detect_system)
    
    if has_systemd; then
        echo "2. Start the service:"
        echo "   systemctl start xray-telegram-manager"
        echo ""
        echo "3. Check service status:"
        echo "   systemctl status xray-telegram-manager"
        echo ""
        echo "4. View logs:"
        echo "   journalctl -u xray-telegram-manager -f"
    elif [ "$system_type" = "openwrt" ]; then
        echo "2. Start the service:"
        echo "   $SERVICE_FILE start"
        echo ""
        echo "3. Enable auto-start:"
        echo "   $SERVICE_FILE enable"
        echo ""
        echo "4. Check status:"
        echo "   $SERVICE_FILE status"
    else
        echo "2. Start the service manually:"
        echo "   cd $INSTALL_DIR && ./$BINARY_NAME"
    fi
    
    echo ""
    print_info "Configuration file location: $CONFIG_FILE"
    print_info "Log directory: $LOG_DIR"
    print_info "Cache directory: $CACHE_DIR"
    echo ""
    print_warn "Remember to configure your firewall to allow outbound connections"
    print_warn "Make sure xray is properly installed and configured"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -f, --force             Force installation (overwrite existing files)"
    echo "  --no-service            Skip service installation"
    echo "  --config-only           Only create configuration template"
    echo "  --uninstall             Uninstall the service"
    echo ""
    echo "Examples:"
    echo "  $0                      # Standard installation"
    echo "  $0 --force              # Force reinstallation"
    echo "  $0 --config-only        # Only create config template"
    echo "  $0 --uninstall          # Uninstall the service"
}

# Function to uninstall
uninstall() {
    print_step "Uninstalling xray-telegram-manager..."
    
    # Stop service
    if has_systemd && [ -f "$SYSTEMD_SERVICE_FILE" ]; then
        systemctl stop xray-telegram-manager.service 2>/dev/null || true
        systemctl disable xray-telegram-manager.service 2>/dev/null || true
        rm -f "$SYSTEMD_SERVICE_FILE"
        systemctl daemon-reload
        print_info "✓ Systemd service removed"
    fi
    
    if [ -f "$SERVICE_FILE" ]; then
        "$SERVICE_FILE" stop 2>/dev/null || true
        rm -f "$SERVICE_FILE"
        print_info "✓ OpenWrt init script removed"
    fi
    
    # Remove files (but keep config and logs)
    local binary_path=$(find_binary)
    if [ -n "$binary_path" ] && [ -f "$binary_path" ]; then
        rm -f "$binary_path"
        print_info "✓ Binary removed"
    fi
    
    print_info "Uninstallation completed"
    print_warn "Configuration and logs preserved in: $INSTALL_DIR"
    print_info "To completely remove: rm -rf $INSTALL_DIR"
}

# Main installation function
main() {
    local force=false
    local no_service=false
    local config_only=false
    local uninstall_mode=false
    
    # Parse arguments
    while [ $# -gt 0 ]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -f|--force)
                force=true
                shift
                ;;
            --no-service)
                no_service=true
                shift
                ;;
            --config-only)
                config_only=true
                shift
                ;;
            --uninstall)
                uninstall_mode=true
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Check root privileges
    check_root
    
    # Handle uninstall
    if [ "$uninstall_mode" = true ]; then
        uninstall
        exit 0
    fi
    
    # Detect system
    local system_type=$(detect_system)
    print_info "Detected system: $system_type"
    
    # Check if already installed
    local binary_path=$(find_binary)
    if [ -n "$binary_path" ] && [ -f "$binary_path" ] && [ "$force" = false ]; then
        print_warn "xray-telegram-manager is already installed"
        print_info "Use --force to reinstall or --uninstall to remove"
        exit 1
    fi
    
    # Create directories
    create_directories
    
    # Config only mode
    if [ "$config_only" = true ]; then
        create_config_template
        exit 0
    fi
    
    # Install binary
    install_binary
    
    # Create configuration
    create_config_template
    
    # Install service
    if [ "$no_service" = false ]; then
        if has_systemd; then
            install_systemd_service
        elif [ "$system_type" = "openwrt" ]; then
            install_openwrt_service
        else
            print_warn "No supported init system found, skipping service installation"
        fi
    fi
    
    # Show instructions
    show_instructions
}

# Run main function
main "$@"