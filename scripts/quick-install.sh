#!/bin/sh

# Quick installation script for Keenetic routers
# Usage: curl -fsSL https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh | sh

set -e

REPO="ad/xray-subscription-telegram-manager-for-keenetic"
ARCH="mips-softfloat"  # Default for most Keenetic routers
INSTALL_DIR="/opt/bin"
CONFIG_DIR="/opt/etc/xray-manager"

echo "🚀 Installing Xray Telegram Manager for Keenetic..."

# Detect architecture (basic detection)
if [ "$(uname -m)" = "mipsel" ]; then
    ARCH="mipsle-softfloat"
fi

echo "📋 Detected architecture: $ARCH"

# Get latest release info
echo "🔍 Getting latest release info..."
LATEST_URL="https://api.github.com/repos/$REPO/releases/latest"
RELEASE_INFO=$(wget -qO- "$LATEST_URL" 2>/dev/null || curl -fsSL "$LATEST_URL" 2>/dev/null)

if [ -z "$RELEASE_INFO" ]; then
    echo "❌ Failed to get release information"
    exit 1
fi

# Extract version and download URL
VERSION=$(echo "$RELEASE_INFO" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/xray-telegram-manager-$ARCH.tar.gz"

echo "📦 Latest version: $VERSION"
echo "🌐 Download URL: $DOWNLOAD_URL"

# Create directories
echo "📁 Creating directories..."
mkdir -p "$CONFIG_DIR"/{logs,scripts}
mkdir -p "$INSTALL_DIR"

# Download and extract
echo "⬇️  Downloading release..."
cd /tmp

# Try to download the primary architecture
if ! wget -O "xray-telegram-manager-$ARCH.tar.gz" "$DOWNLOAD_URL" 2>/dev/null; then
    echo "❌ Failed to download for architecture: $ARCH"
    echo "🔄 Trying alternative architectures..."
    
    # Try alternative architectures
    for alt_arch in mips-hardfloat mipsle-softfloat mipsle-hardfloat mips-softfloat; do
        if [ "$alt_arch" != "$ARCH" ]; then
            echo "   Trying: $alt_arch"
            ALT_URL="https://github.com/$REPO/releases/latest/download/xray-telegram-manager-$alt_arch.tar.gz"
            if wget -O "xray-telegram-manager-$alt_arch.tar.gz" "$ALT_URL" 2>/dev/null; then
                ARCH="$alt_arch"
                mv "xray-telegram-manager-$alt_arch.tar.gz" "xray-telegram-manager-$ARCH.tar.gz"
                echo "✅ Successfully downloaded: $ARCH"
                break
            fi
        fi
    done
    
    # If all failed
    if [ ! -f "xray-telegram-manager-$ARCH.tar.gz" ]; then
        echo "❌ Failed to download any release archive"
        echo "💡 Available alternatives:"
        echo "   1. Manual download from: https://github.com/$REPO/releases/latest"
        echo "   2. Check your internet connection"
        echo "   3. Try manual installation method"
        exit 1
    fi
fi

echo "📦 Extracting archive..."
tar -xzf "xray-telegram-manager-$ARCH.tar.gz"

# Check if extract worked
if [ ! -f "xray-telegram-manager" ]; then
    echo "❌ Failed to extract binary from archive"
    exit 1
fi

# Install binary
echo "💾 Installing binary..."
cp "xray-telegram-manager" "$INSTALL_DIR/xray-telegram-manager"
chmod +x "$INSTALL_DIR/xray-telegram-manager"

# Install scripts if they exist
if [ -d "scripts" ]; then
    echo "📋 Installing helper scripts..."
    cp -r scripts/* "$CONFIG_DIR/scripts/"
    chmod +x "$CONFIG_DIR/scripts"/*.sh 2>/dev/null || true
fi

# Create init script
echo "🔧 Creating init script..."
cat > /opt/etc/init.d/S99xray-telegram-manager << 'INIT_EOF'
#!/bin/sh

ENABLED=yes
PROCS=xray-telegram-manager
ARGS="-config /opt/etc/xray-manager/config.json"
PREARGS=""
DESC=$PROCS
PATH=/opt/sbin:/opt/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

. /opt/etc/init.d/rc.func
INIT_EOF

chmod +x /opt/etc/init.d/S99xray-telegram-manager

# Create sample config (if doesn't exist)
if [ ! -f "$CONFIG_DIR/config.json" ]; then
    echo "⚙️  Creating sample configuration..."
    if [ -f "config/config.json.sample" ]; then
        cp "config/config.json.sample" "$CONFIG_DIR/config.json"
    else
        cat > "$CONFIG_DIR/config.json" << 'CONFIG_EOF'
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
CONFIG_EOF
    fi
else
    echo "⚙️  Configuration file already exists, skipping..."
fi

# Cleanup
rm -f /tmp/xray-telegram-manager-*.tar.gz /tmp/xray-telegram-manager 2>/dev/null || true
rm -rf /tmp/scripts /tmp/config 2>/dev/null || true

echo "✅ Installation completed!"
echo ""
echo "📝 Next steps:"
echo "1. Edit configuration: nano $CONFIG_DIR/config.json"
echo "2. Set your admin_id, bot_token, and subscription_url"
echo "3. Start the service: /opt/etc/init.d/S99xray-telegram-manager start"
echo ""
echo "🔍 Check status: /opt/etc/init.d/S99xray-telegram-manager status"
echo "📄 View logs: tail -f $CONFIG_DIR/logs/app.log"
echo ""
echo "💡 Get your Telegram ID from @userinfobot"
echo "🤖 Create bot with @BotFather"
