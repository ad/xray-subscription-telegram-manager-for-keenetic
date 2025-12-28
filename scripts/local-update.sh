#!/bin/sh

# Local update script for development
# This script updates the bot with locally built binary

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root"
    exit 1
fi

# Check if binary exists
if [ ! -f "./xray-telegram-manager" ]; then
    print_error "Binary not found. Run 'go build' first"
    exit 1
fi

# Find current binary location
BINARY_PATH=""
if [ -f "/opt/etc/xray-manager/xray-telegram-manager" ]; then
    BINARY_PATH="/opt/etc/xray-manager/xray-telegram-manager"
elif [ -f "/opt/bin/xray-telegram-manager" ]; then
    BINARY_PATH="/opt/bin/xray-telegram-manager"
else
    print_error "Installed binary not found"
    exit 1
fi

print_info "Found installed binary: $BINARY_PATH"

# Stop service
print_info "Stopping service..."
if [ -f "/opt/etc/init.d/S99xray-telegram-manager" ]; then
    /opt/etc/init.d/S99xray-telegram-manager stop
else
    pkill -f xray-telegram-manager || true
fi

# Wait for process to stop
sleep 2

# Update binary
print_info "Updating binary..."
cp ./xray-telegram-manager "$BINARY_PATH"
chmod 755 "$BINARY_PATH"

# Start service
print_info "Starting service..."
if [ -f "/opt/etc/init.d/S99xray-telegram-manager" ]; then
    /opt/etc/init.d/S99xray-telegram-manager start
else
    print_warn "Service file not found, please start manually"
fi

# Wait and check
sleep 3
if pgrep -f xray-telegram-manager >/dev/null; then
    print_info "✓ Service started successfully"
else
    print_error "Service failed to start"
    exit 1
fi

print_info "✓ Local update completed successfully!"