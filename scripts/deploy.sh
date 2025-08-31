#!/bin/sh

# Comprehensive deployment script for xray-telegram-manager
# Handles building, packaging, and deployment to Keenetic routers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PACKAGE_NAME="xray-telegram-manager"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
DIST_DIR="./dist"
PACKAGE_DIR="./package"

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
    echo "  -t, --target HOST       Deploy to remote host via SSH"
    echo "  -u, --user USER         SSH user (default: root)"
    echo "  -p, --port PORT         SSH port (default: 22)"
    echo "  -k, --key FILE          SSH private key file"
    echo "  --build-only            Only build, don't package"
    echo "  --package-only          Only create package"
    echo "  --no-build              Skip building"
    echo "  --clean                 Clean before build"
    echo ""
    echo "Examples:"
    echo "  $0                      # Build and package"
    echo "  $0 -t 192.168.1.1       # Build, package and deploy"
    echo "  $0 --build-only         # Only build binaries"
    echo "  $0 --package-only       # Only create package"
}

# Function to clean previous builds
clean_build() {
    print_step "Cleaning previous builds..."
    rm -rf "$DIST_DIR"
    rm -rf "$PACKAGE_DIR"
    print_info "✓ Cleaned build directories"
}

# Function to build binaries
build_binaries() {
    print_step "Building binaries..."
    
    if ! command -v make >/dev/null 2>&1; then
        print_error "Make is not installed"
        exit 1
    fi
    
    make mips
    print_info "✓ Binaries built successfully"
}

# Function to create deployment package
create_package() {
    print_step "Creating deployment package..."
    
    # Create package directory structure
    mkdir -p "$PACKAGE_DIR"
    mkdir -p "$PACKAGE_DIR/bin"
    mkdir -p "$PACKAGE_DIR/scripts"
    mkdir -p "$PACKAGE_DIR/config"
    mkdir -p "$PACKAGE_DIR/docs"
    
    # Copy binaries
    if [ -f "$DIST_DIR/${PACKAGE_NAME}-mips-softfloat" ]; then
        cp "$DIST_DIR/${PACKAGE_NAME}-mips-softfloat" "$PACKAGE_DIR/bin/${PACKAGE_NAME}"
        print_info "✓ Copied MIPS softfloat binary"
    elif [ -f "$DIST_DIR/${PACKAGE_NAME}-mips-hardfloat" ]; then
        cp "$DIST_DIR/${PACKAGE_NAME}-mips-hardfloat" "$PACKAGE_DIR/bin/${PACKAGE_NAME}"
        print_info "✓ Copied MIPS hardfloat binary"
    else
        print_error "No MIPS binary found in $DIST_DIR"
        exit 1
    fi
    
    # Copy scripts
    cp scripts/*.sh "$PACKAGE_DIR/scripts/"
    cp scripts/*.service "$PACKAGE_DIR/scripts/" 2>/dev/null || true
    print_info "✓ Copied installation scripts"
    
    # Generate configuration template
    ./scripts/generate-config.sh --keenetic -o "$PACKAGE_DIR/config/config.json.template"
    print_info "✓ Generated configuration template"
    
    # Copy documentation
    cp README.md "$PACKAGE_DIR/docs/" 2>/dev/null || echo "# ${PACKAGE_NAME}" > "$PACKAGE_DIR/docs/README.md"
    
    # Create version info
    cat > "$PACKAGE_DIR/VERSION" << EOF
Package: ${PACKAGE_NAME}
Version: ${VERSION}
Build Date: $(date -u '+%Y-%m-%d %H:%M:%S UTC')
Architecture: MIPS
Target: Keenetic routers
EOF
    
    # Create installation instructions
    cat > "$PACKAGE_DIR/INSTALL.md" << 'EOF'
# Installation Instructions

## Prerequisites
- Keenetic router with Entware installed
- SSH access to the router
- Xray already installed and configured

## Installation Steps

1. Copy the package to your router:
   ```bash
   scp -r package/ root@192.168.1.1:/tmp/xray-telegram-manager/
   ```

2. SSH to your router and run the installation script:
   ```bash
   ssh root@192.168.1.1
   cd /tmp/xray-telegram-manager
   ./scripts/install.sh
   ```

3. Configure the application:
   ```bash
   nano /opt/etc/xray-manager/config.json
   ```
   
   Set your:
   - Telegram bot token
   - Admin user ID
   - Subscription URL

4. Start the service:
   ```bash
   /opt/etc/init.d/S99xray-telegram-manager start
   ```

## Configuration

Edit `/opt/etc/xray-manager/config.json`:

- `admin_id`: Your Telegram user ID
- `bot_token`: Your Telegram bot token from @BotFather
- `subscription_url`: Your VLESS subscription URL
- `config_path`: Path to xray outbound config (default: `/opt/etc/xray/configs/04_outbounds.json`)

## Usage

Send these commands to your Telegram bot:
- `/start` - Show server list
- `/list` - List all servers
- `/status` - Show current server status
- `/ping` - Test all servers

## Troubleshooting

Check logs:
```bash
tail -f /opt/etc/xray-manager/logs/app.log
```

Check service status:
```bash
/opt/etc/init.d/S99xray-telegram-manager status
```

## Uninstallation

```bash
/opt/etc/xray-manager/scripts/uninstall.sh
```
EOF
    
    # Create checksums
    (cd "$PACKAGE_DIR" && find . -type f -exec sha256sum {} \; > checksums.txt)
    
    print_info "✓ Package created in $PACKAGE_DIR"
}

# Function to create archive
create_archive() {
    print_step "Creating archive..."
    
    local archive_name="${PACKAGE_NAME}-${VERSION}-mips.tar.gz"
    
    tar -czf "$archive_name" -C "$PACKAGE_DIR" .
    
    # Create checksum for archive
    sha256sum "$archive_name" > "${archive_name}.sha256"
    
    print_info "✓ Archive created: $archive_name"
    print_info "✓ Checksum created: ${archive_name}.sha256"
}

# Function to deploy to remote host
deploy_remote() {
    local host="$1"
    local user="$2"
    local port="$3"
    local key="$4"
    
    print_step "Deploying to $user@$host:$port..."
    
    # Prepare SSH options
    local ssh_opts="-p $port"
    if [ -n "$key" ]; then
        ssh_opts="$ssh_opts -i $key"
    fi
    
    # Create temporary directory on remote host
    local remote_temp="/tmp/xray-telegram-manager-deploy-$$"
    
    print_info "Creating remote temporary directory..."
    ssh $ssh_opts "$user@$host" "mkdir -p $remote_temp"
    
    # Copy package to remote host
    print_info "Copying package to remote host..."
    scp $ssh_opts -r "$PACKAGE_DIR"/* "$user@$host:$remote_temp/"
    
    # Run installation on remote host
    print_info "Running installation on remote host..."
    ssh $ssh_opts "$user@$host" "cd $remote_temp && ./scripts/install.sh --force"
    
    # Cleanup remote temporary directory
    print_info "Cleaning up remote temporary directory..."
    ssh $ssh_opts "$user@$host" "rm -rf $remote_temp"
    
    print_info "✓ Deployment completed successfully"
    print_warn "Don't forget to configure the application on the remote host:"
    print_warn "  ssh $user@$host 'nano /opt/etc/xray-manager/config.json'"
}

# Function to validate environment
validate_environment() {
    print_step "Validating environment..."
    
    # Check Go installation
    if ! command -v go >/dev/null 2>&1; then
        print_error "Go is not installed"
        exit 1
    fi
    
    # Check if we're in the right directory
    if [ ! -f "go.mod" ]; then
        print_error "Not in a Go project directory"
        exit 1
    fi
    
    # Check if scripts exist
    if [ ! -d "scripts" ]; then
        print_error "Scripts directory not found"
        exit 1
    fi
    
    print_info "✓ Environment validation passed"
}

# Main function
main() {
    local target_host=""
    local ssh_user="root"
    local ssh_port="22"
    local ssh_key=""
    local build_only=false
    local package_only=false
    local no_build=false
    local clean=false
    
    # Parse arguments
    while [ $# -gt 0 ]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -t|--target)
                target_host="$2"
                shift 2
                ;;
            -u|--user)
                ssh_user="$2"
                shift 2
                ;;
            -p|--port)
                ssh_port="$2"
                shift 2
                ;;
            -k|--key)
                ssh_key="$2"
                shift 2
                ;;
            --build-only)
                build_only=true
                shift
                ;;
            --package-only)
                package_only=true
                shift
                ;;
            --no-build)
                no_build=true
                shift
                ;;
            --clean)
                clean=true
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Validate environment
    validate_environment
    
    print_step "Starting deployment process..."
    print_info "Package: $PACKAGE_NAME"
    print_info "Version: $VERSION"
    
    # Clean if requested
    if [ "$clean" = true ]; then
        clean_build
    fi
    
    # Build binaries
    if [ "$no_build" = false ] && [ "$package_only" = false ]; then
        build_binaries
    fi
    
    # Stop here if build-only
    if [ "$build_only" = true ]; then
        print_info "Build completed (build-only mode)"
        exit 0
    fi
    
    # Create package
    if [ "$no_build" = false ] || [ "$package_only" = true ]; then
        create_package
        create_archive
    fi
    
    # Deploy to remote host if specified
    if [ -n "$target_host" ]; then
        deploy_remote "$target_host" "$ssh_user" "$ssh_port" "$ssh_key"
    else
        print_info "Deployment package ready in: $PACKAGE_DIR"
        print_info "Archive created: ${PACKAGE_NAME}-${VERSION}-mips.tar.gz"
        print_info ""
        print_info "To deploy manually:"
        print_info "  1. Copy package to your router"
        print_info "  2. Run: ./scripts/install.sh"
        print_info "  3. Configure: /opt/etc/xray-manager/config.json"
        print_info "  4. Start: /opt/etc/init.d/S99xray-telegram-manager start"
    fi
    
    print_step "Deployment process completed!"
}

# Run main function
main "$@"