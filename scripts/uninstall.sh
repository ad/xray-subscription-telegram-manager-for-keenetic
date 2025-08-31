#!/bin/sh

# Uninstallation script for xray-telegram-manager

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
SERVICE_FILE="/opt/etc/init.d/S99xray-telegram-manager"
SYSTEMD_SERVICE_FILE="/etc/systemd/system/xray-telegram-manager.service"
BINARY_NAME="xray-telegram-manager"

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

# Function to check if systemd is available
has_systemd() {
    command -v systemctl >/dev/null 2>&1 && [ -d /etc/systemd/system ]
}

# Function to stop and remove systemd service
remove_systemd_service() {
    if [ -f "$SYSTEMD_SERVICE_FILE" ]; then
        print_step "Removing systemd service..."
        
        # Stop service
        systemctl stop xray-telegram-manager.service 2>/dev/null || true
        print_info "✓ Service stopped"
        
        # Disable service
        systemctl disable xray-telegram-manager.service 2>/dev/null || true
        print_info "✓ Service disabled"
        
        # Remove service file
        rm -f "$SYSTEMD_SERVICE_FILE"
        print_info "✓ Service file removed"
        
        # Reload systemd
        systemctl daemon-reload
        print_info "✓ Systemd reloaded"
    else
        print_info "Systemd service not found, skipping"
    fi
}

# Function to stop and remove OpenWrt service
remove_openwrt_service() {
    if [ -f "$SERVICE_FILE" ]; then
        print_step "Removing OpenWrt service..."
        
        # Stop service
        "$SERVICE_FILE" stop 2>/dev/null || true
        print_info "✓ Service stopped"
        
        # Remove service file
        rm -f "$SERVICE_FILE"
        print_info "✓ Service file removed"
    else
        print_info "OpenWrt service not found, skipping"
    fi
}

# Function to remove binary and directories
remove_files() {
    print_step "Removing files..."
    
    # Stop any running processes
    pkill -f "$BINARY_NAME" 2>/dev/null || true
    sleep 2
    
    # Remove binary from found location
    local binary_path=$(find_binary)
    if [ -n "$binary_path" ] && [ -f "$binary_path" ]; then
        rm -f "$binary_path"
        print_info "✓ Binary removed from $binary_path"
    fi
    
    if [ -d "$INSTALL_DIR" ]; then
        # Ask about configuration and logs
        echo ""
        print_warn "The following directories contain configuration and logs:"
        print_warn "  $INSTALL_DIR"
        echo ""
        printf "Do you want to remove all data including config and logs? [y/N]: "
        read -r reply
        echo ""
        
        case $reply in
            [Yy]|[Yy][Ee][Ss])
                rm -rf "$INSTALL_DIR"
                print_info "✓ All data removed"
                ;;
            *)
                print_info "Configuration and logs preserved in: $INSTALL_DIR"
                print_info "To remove manually: rm -rf $INSTALL_DIR"
                ;;
        esac
    else
        print_info "Installation directory not found, skipping"
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -f, --force             Force removal without confirmation"
    echo "  --keep-data             Keep configuration and log files"
    echo ""
    echo "Examples:"
    echo "  $0                      # Interactive uninstallation"
    echo "  $0 --force              # Force uninstallation"
    echo "  $0 --keep-data          # Uninstall but keep data"
}

# Main uninstallation function
main() {
    local force=false
    local keep_data=false
    
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
            --keep-data)
                keep_data=true
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
    
    # Confirmation
    if [ "$force" = false ]; then
        echo ""
        print_warn "This will uninstall xray-telegram-manager from your system"
        printf "Are you sure you want to continue? [y/N]: "
        read -r reply
        echo ""
        case $reply in
            [Yy]|[Yy][Ee][Ss])
                ;;
            *)
                print_info "Uninstallation cancelled"
                exit 0
                ;;
        esac
    fi
    
    print_step "Starting uninstallation..."
    
    # Remove services
    if has_systemd; then
        remove_systemd_service
    fi
    
    remove_openwrt_service
    
    # Remove files
    if [ "$keep_data" = true ]; then
        print_step "Removing binary only..."
        local binary_path=$(find_binary)
        if [ -n "$binary_path" ] && [ -f "$binary_path" ]; then
            rm -f "$binary_path"
            print_info "✓ Binary removed"
        fi
        print_info "Configuration and logs preserved in: $INSTALL_DIR"
    else
        remove_files
    fi
    
    print_step "Uninstallation completed!"
    print_info "xray-telegram-manager has been removed from your system"
}

# Run main function
main "$@"