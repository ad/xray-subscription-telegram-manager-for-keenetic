#!/bin/bash

# Configuration template generator for xray-telegram-manager

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -o, --output FILE       Output file path (default: config.json)"
    echo "  -i, --interactive       Interactive configuration"
    echo "  --keenetic              Generate Keenetic-specific configuration"
    echo "  --openwrt               Generate OpenWrt-specific configuration"
    echo "  --template              Generate template with placeholders"
    echo ""
    echo "Examples:"
    echo "  $0                      # Generate basic template"
    echo "  $0 -i                   # Interactive configuration"
    echo "  $0 --keenetic           # Keenetic-specific config"
    echo "  $0 -o /path/config.json # Custom output path"
}

# Function to generate basic template
generate_template() {
    local output_file="$1"
    
    cat > "$output_file" << 'EOF'
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
}

# Function to generate Keenetic-specific configuration
generate_keenetic_config() {
    local output_file="$1"
    
    cat > "$output_file" << 'EOF'
{
    "admin_id": 0,
    "bot_token": "YOUR_BOT_TOKEN_HERE",
    "config_path": "/opt/etc/xray/configs/04_outbounds.json",
    "subscription_url": "YOUR_SUBSCRIPTION_URL_HERE",
    "log_level": "info",
    "xray_restart_command": "/opt/etc/init.d/S24xray restart",
    "cache_duration": 3600,
    "health_check_interval": 300,
    "ping_timeout": 5,
    "keenetic_specific": {
        "ndm_config_path": "/opt/etc/ndm/fs.d/100-xray.conf",
        "backup_path": "/opt/etc/xray-manager/backup",
        "log_path": "/opt/etc/xray-manager/logs/app.log",
        "pid_file": "/var/run/xray-telegram-manager.pid"
    }
}
EOF
}

# Function to generate OpenWrt-specific configuration
generate_openwrt_config() {
    local output_file="$1"
    
    cat > "$output_file" << 'EOF'
{
    "admin_id": 0,
    "bot_token": "YOUR_BOT_TOKEN_HERE",
    "config_path": "/etc/xray/config.json",
    "subscription_url": "YOUR_SUBSCRIPTION_URL_HERE",
    "log_level": "info",
    "xray_restart_command": "/etc/init.d/xray restart",
    "cache_duration": 3600,
    "health_check_interval": 300,
    "ping_timeout": 5,
    "openwrt_specific": {
        "uci_config": true,
        "backup_path": "/etc/xray-manager/backup",
        "log_path": "/var/log/xray-telegram-manager.log",
        "pid_file": "/var/run/xray-telegram-manager.pid"
    }
}
EOF
}

# Function for interactive configuration
interactive_config() {
    local output_file="$1"
    
    print_step "Interactive Configuration Setup"
    echo ""
    
    # Get admin ID
    while true; do
        read -p "Enter your Telegram admin ID: " admin_id
        if [[ "$admin_id" =~ ^[0-9]+$ ]]; then
            break
        else
            print_error "Please enter a valid numeric ID"
        fi
    done
    
    # Get bot token
    while true; do
        read -p "Enter your Telegram bot token: " bot_token
        if [[ "$bot_token" =~ ^[0-9]+:[a-zA-Z0-9_-]+$ ]]; then
            break
        else
            print_error "Please enter a valid bot token (format: 123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11)"
        fi
    done
    
    # Get subscription URL
    read -p "Enter your subscription URL: " subscription_url
    
    # Get xray config path
    read -p "Enter xray config path [/opt/etc/xray/configs/04_outbounds.json]: " config_path
    config_path=${config_path:-"/opt/etc/xray/configs/04_outbounds.json"}
    
    # Get restart command
    read -p "Enter xray restart command [/opt/etc/init.d/S24xray restart]: " restart_command
    restart_command=${restart_command:-"/opt/etc/init.d/S24xray restart"}
    
    # Get log level
    echo "Select log level:"
    echo "1) debug"
    echo "2) info (default)"
    echo "3) warn"
    echo "4) error"
    read -p "Choice [2]: " log_choice
    log_choice=${log_choice:-2}
    
    case $log_choice in
        1) log_level="debug" ;;
        2) log_level="info" ;;
        3) log_level="warn" ;;
        4) log_level="error" ;;
        *) log_level="info" ;;
    esac
    
    # Get cache duration
    read -p "Enter cache duration in seconds [3600]: " cache_duration
    cache_duration=${cache_duration:-3600}
    
    # Get health check interval
    read -p "Enter health check interval in seconds [300]: " health_check_interval
    health_check_interval=${health_check_interval:-300}
    
    # Get ping timeout
    read -p "Enter ping timeout in seconds [5]: " ping_timeout
    ping_timeout=${ping_timeout:-5}
    
    # Generate configuration
    cat > "$output_file" << EOF
{
    "admin_id": $admin_id,
    "bot_token": "$bot_token",
    "config_path": "$config_path",
    "subscription_url": "$subscription_url",
    "log_level": "$log_level",
    "xray_restart_command": "$restart_command",
    "cache_duration": $cache_duration,
    "health_check_interval": $health_check_interval,
    "ping_timeout": $ping_timeout
}
EOF
}

# Main function
main() {
    local output_file="config.json"
    local interactive=false
    local config_type="template"
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -o|--output)
                output_file="$2"
                shift 2
                ;;
            -i|--interactive)
                interactive=true
                shift
                ;;
            --keenetic)
                config_type="keenetic"
                shift
                ;;
            --openwrt)
                config_type="openwrt"
                shift
                ;;
            --template)
                config_type="template"
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Check if output file exists
    if [ -f "$output_file" ]; then
        print_warn "Configuration file already exists: $output_file"
        read -p "Do you want to overwrite it? [y/N]: " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Configuration generation cancelled"
            exit 0
        fi
    fi
    
    print_step "Generating configuration..."
    
    # Generate configuration based on type
    if [ "$interactive" = true ]; then
        interactive_config "$output_file"
    else
        case $config_type in
            "keenetic")
                generate_keenetic_config "$output_file"
                ;;
            "openwrt")
                generate_openwrt_config "$output_file"
                ;;
            "template"|*)
                generate_template "$output_file"
                ;;
        esac
    fi
    
    # Set appropriate permissions
    chmod 600 "$output_file"
    
    print_info "✓ Configuration generated: $output_file"
    
    if [ "$interactive" = false ]; then
        print_warn "Please edit the configuration file and set your bot token and admin ID"
        print_info "Required fields to configure:"
        print_info "  - admin_id: Your Telegram user ID"
        print_info "  - bot_token: Your Telegram bot token"
        print_info "  - subscription_url: Your VLESS subscription URL"
    fi
    
    print_info "Configuration file permissions set to 600 (owner read/write only)"
}

# Run main function
main "$@"