#!/bin/sh

# Update script for xray-telegram-manager

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
BACKUP_DIR="$INSTALL_DIR/backup"

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

# Function to check if service is running
is_service_running() {
    if has_systemd && [ -f "$SYSTEMD_SERVICE_FILE" ]; then
        systemctl is-active --quiet xray-telegram-manager.service
    elif [ -f "$SERVICE_FILE" ]; then
        pgrep -f "$BINARY_NAME" >/dev/null
    else
        pgrep -f "$BINARY_NAME" >/dev/null
    fi
}

# Function to stop service
stop_service() {
    print_step "Stopping service..."
    
    if has_systemd && [ -f "$SYSTEMD_SERVICE_FILE" ]; then
        systemctl stop xray-telegram-manager.service
        print_info "✓ Systemd service stopped"
    elif [ -f "$SERVICE_FILE" ]; then
        "$SERVICE_FILE" stop
        print_info "✓ OpenWrt service stopped"
    else
        pkill -f "$BINARY_NAME" 2>/dev/null || true
        print_info "✓ Process stopped"
    fi
    
    # Wait for process to stop
    sleep 2
}

# Function to start service
start_service() {
    print_step "Starting service..."
    
    if has_systemd && [ -f "$SYSTEMD_SERVICE_FILE" ]; then
        systemctl start xray-telegram-manager.service
        print_info "✓ Systemd service started"
    elif [ -f "$SERVICE_FILE" ]; then
        "$SERVICE_FILE" start
        print_info "✓ OpenWrt service started"
    else
        print_warn "No service configuration found, please start manually"
    fi
}

# Function to create backup
create_backup() {
    print_step "Creating backup..."
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_file="$BACKUP_DIR/backup_$timestamp"
    
    mkdir -p "$BACKUP_DIR"
    
    local binary_path=$(find_binary)
    if [ -n "$binary_path" ] && [ -f "$binary_path" ]; then
        cp "$binary_path" "${backup_file}_binary"
        print_info "✓ Binary backed up: ${backup_file}_binary"
    fi
    
    if [ -f "$CONFIG_FILE" ]; then
        cp "$CONFIG_FILE" "${backup_file}_config.json"
        print_info "✓ Configuration backed up: ${backup_file}_config.json"
    fi
    
    # Keep only last 5 backups
    ls -t "$BACKUP_DIR"/backup_*_binary 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true
    ls -t "$BACKUP_DIR"/backup_*_config.json 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true
}

# Function to update binary
update_binary() {
    print_step "Updating binary..."
    
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
    
    # Find current binary location
    local current_binary=$(find_binary)
    if [ -z "$current_binary" ]; then
        # Fallback to primary location
        current_binary="$INSTALL_DIR/$BINARY_NAME"
    fi
    
    # Copy new binary
    cp "$binary_path" "$current_binary"
    chmod 755 "$current_binary"
    
    print_info "✓ Binary updated: $current_binary"
}

# Function to update service files
update_service() {
    print_step "Updating service files..."
    
    local service_updated=false
    
    # Update systemd service if it exists
    if has_systemd && [ -f "$SYSTEMD_SERVICE_FILE" ] && [ -f "./scripts/xray-telegram-manager.service" ]; then
        if ! cmp -s "./scripts/xray-telegram-manager.service" "$SYSTEMD_SERVICE_FILE"; then
            cp "./scripts/xray-telegram-manager.service" "$SYSTEMD_SERVICE_FILE"
            chmod 644 "$SYSTEMD_SERVICE_FILE"
            systemctl daemon-reload
            print_info "✓ Systemd service updated"
            service_updated=true
        else
            print_info "✓ Systemd service is up to date"
        fi
    fi
    
    return 0
}

# Function to check version
check_version() {
    local binary_path=$(find_binary)
    if [ -n "$binary_path" ] && [ -f "$binary_path" ]; then
        print_info "Current version information:"
        "$binary_path" --version 2>/dev/null || print_warn "Version information not available"
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -f, --force             Force update without confirmation"
    echo "  --no-backup             Skip backup creation"
    echo "  --no-restart            Don't restart service after update"
    echo "  --check                 Check current version and exit"
    echo ""
    echo "Examples:"
    echo "  $0                      # Interactive update"
    echo "  $0 --force              # Force update"
    echo "  $0 --check              # Check current version"
}

# Function to rollback
rollback() {
    print_step "Rolling back to previous version..."
    
    local latest_backup=$(ls -t "$BACKUP_DIR"/backup_*_binary 2>/dev/null | head -n 1)
    
    if [ -n "$latest_backup" ] && [ -f "$latest_backup" ]; then
        local binary_path=$(find_binary)
        if [ -n "$binary_path" ]; then
            cp "$latest_backup" "$binary_path"
            chmod 755 "$binary_path"
            print_info "✓ Rolled back to previous version"
        else
            # Fallback to primary location
            cp "$latest_backup" "$INSTALL_DIR/$BINARY_NAME"
            chmod 755 "$INSTALL_DIR/$BINARY_NAME"
            print_info "✓ Rolled back to previous version (to $INSTALL_DIR)"
        fi
        
        # Restart service
        if is_service_running; then
            start_service
        fi
    else
        print_error "No backup found for rollback"
        exit 1
    fi
}

# Main update function
main() {
    local force=false
    local no_backup=false
    local no_restart=false
    local check_only=false
    local was_running=false
    
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
            --no-backup)
                no_backup=true
                shift
                ;;
            --no-restart)
                no_restart=true
                shift
                ;;
            --check)
                check_only=true
                shift
                ;;
            --rollback)
                check_root
                rollback
                exit 0
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
    
    # Check version only
    if [ "$check_only" = true ]; then
        check_version
        exit 0
    fi
    
    # Check if installed
    local binary_path=$(find_binary)
    if [ -z "$binary_path" ] || [ ! -f "$binary_path" ]; then
        print_error "xray-telegram-manager is not installed"
        print_info "Please run the installation script first"
        exit 1
    fi
    
    # Show current version
    print_info "Current installation:"
    check_version
    
    # Confirmation
    if [ "$force" = false ]; then
        echo ""
        print_warn "This will update xray-telegram-manager"
        printf "Are you sure you want to continue? [y/N]: "
        read -r reply
        echo ""
        case $reply in
            [Yy]|[Yy][Ee][Ss])
                ;;
            *)
                print_info "Update cancelled"
                exit 0
                ;;
        esac
    fi
    
    print_step "Starting update..."
    
    # Check if service is running
    if is_service_running; then
        was_running=true
        print_info "Service is currently running"
    fi
    
    # Create backup
    if [ "$no_backup" = false ]; then
        create_backup
    fi
    
    # Stop service if running
    if [ "$was_running" = true ]; then
        stop_service
    fi
    
    # Update binary
    update_binary
    
    # Update service files
    update_service
    
    # Start service if it was running and not disabled
    if [ "$was_running" = true ] && [ "$no_restart" = false ]; then
        start_service
        
        # Wait a moment and check if service started successfully
        sleep 3
        if is_service_running; then
            print_info "✓ Service is running successfully"
        else
            print_error "Service failed to start, check logs"
            print_info "To rollback: $0 --rollback"
            exit 1
        fi
    fi
    
    print_step "Update completed successfully!"
    
    # Show new version
    print_info "Updated version:"
    check_version
    
    if [ "$no_restart" = true ] && [ "$was_running" = true ]; then
        print_warn "Service was not restarted, please restart manually"
    fi
}

# Run main function
main "$@"