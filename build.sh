#!/bin/sh

# Build script for xray-telegram-manager
# Supports cross-compilation for MIPS architecture (Keenetic routers)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
OUTPUT_DIR="./dist"
BINARY_NAME="xray-telegram-manager"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(go version | awk '{print $3}')

# Build flags for size optimization
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GoVersion=${GO_VERSION}"

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

# Function to build for specific architecture
build_target() {
    local goos=$1
    local goarch=$2
    local gomips=$3
    local suffix=$4
    
    local output_name="${BINARY_NAME}${suffix}"
    local output_path="${OUTPUT_DIR}/${output_name}"
    
    print_info "Building ${output_name} (${goos}/${goarch}${gomips:+/${gomips})..."
    
    # Set environment variables
    export GOOS=$goos
    export GOARCH=$goarch
    if [ -n "$gomips" ]; then
        export GOMIPS=$gomips
    fi
    
    # Build the binary
    go build -ldflags="$LDFLAGS" -o "$output_path" .
    
    if [ $? -eq 0 ]; then
        local size=$(du -h "$output_path" | cut -f1)
        print_info "✓ Built ${output_name} (${size})"
        
        # Create checksum
        if command -v sha256sum >/dev/null 2>&1; then
            (cd "$OUTPUT_DIR" && sha256sum "$output_name" >> checksums.txt)
        elif command -v shasum >/dev/null 2>&1; then
            (cd "$OUTPUT_DIR" && shasum -a 256 "$output_name" >> checksums.txt)
        fi
    else
        print_error "✗ Failed to build ${output_name}"
        return 1
    fi
    
    # Unset environment variables
    unset GOOS GOARCH GOMIPS
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -o, --output DIR        Output directory (default: ./dist)"
    echo "  -n, --name NAME         Binary name (default: xray-telegram-manager)"
    echo "  -t, --target TARGET     Build specific target (mips-softfloat, mips-hardfloat, linux-amd64, all)"
    echo "  -c, --clean             Clean output directory before build"
    echo "  --no-compress           Skip UPX compression (if available)"
    echo ""
    echo "Examples:"
    echo "  $0                      # Build all MIPS targets"
    echo "  $0 -t mips-softfloat    # Build only MIPS softfloat"
    echo "  $0 -t all               # Build all supported targets"
    echo "  $0 -c                   # Clean and build"
}

# Parse command line arguments
CLEAN=false
COMPRESS=true
TARGET="mips"

while [ $# -gt 0 ]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -n|--name)
            BINARY_NAME="$2"
            shift 2
            ;;
        -t|--target)
            TARGET="$2"
            shift 2
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        --no-compress)
            COMPRESS=false
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate Go installation
if ! command -v go >/dev/null 2>&1; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_info "Go version: $(go version)"
print_info "Build version: ${VERSION}"
print_info "Build time: ${BUILD_TIME}"

# Clean output directory if requested
if [ "$CLEAN" = true ]; then
    print_info "Cleaning output directory..."
    rm -rf "$OUTPUT_DIR"
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Remove old checksums file
rm -f "${OUTPUT_DIR}/checksums.txt"

# Build targets based on selection
case $TARGET in
    "mips-softfloat")
        build_target "linux" "mips" "softfloat" "-mips-softfloat"
        ;;
    "mips-hardfloat")
        build_target "linux" "mips" "hardfloat" "-mips-hardfloat"
        ;;
    "mips")
        build_target "linux" "mips" "softfloat" "-mips-softfloat"
        build_target "linux" "mips" "hardfloat" "-mips-hardfloat"
        ;;
    "linux-amd64")
        build_target "linux" "amd64" "" "-linux-amd64"
        ;;
    "all")
        build_target "linux" "mips" "softfloat" "-mips-softfloat"
        build_target "linux" "mips" "hardfloat" "-mips-hardfloat"
        build_target "linux" "amd64" "" "-linux-amd64"
        build_target "linux" "arm64" "" "-linux-arm64"
        build_target "darwin" "amd64" "" "-darwin-amd64"
        build_target "darwin" "arm64" "" "-darwin-arm64"
        ;;
    *)
        print_error "Unknown target: $TARGET"
        print_info "Supported targets: mips-softfloat, mips-hardfloat, mips, linux-amd64, all"
        exit 1
        ;;
esac

# Compress binaries with UPX if available and requested
if [ "$COMPRESS" = true ] && command -v upx >/dev/null 2>&1; then
    print_info "Compressing binaries with UPX..."
    for binary in "$OUTPUT_DIR"/*; do
        if [ -f "$binary" ] && [ -x "$binary" ]; then
            case $binary in
                *.txt) ;;  # Skip checksums file
                *)
                    print_info "Compressing $(basename "$binary")..."
                    upx --best --lzma "$binary" 2>/dev/null || print_warn "Failed to compress $(basename "$binary")"
                    ;;
            esac
        fi
    done
else
    if [ "$COMPRESS" = true ]; then
        print_warn "UPX not found, skipping compression"
    fi
fi

print_info "Build completed successfully!"
print_info "Output directory: $OUTPUT_DIR"

# Show build results
if [ -f "${OUTPUT_DIR}/checksums.txt" ]; then
    print_info "Checksums:"
    cat "${OUTPUT_DIR}/checksums.txt"
fi

print_info "Built files:"
ls -lh "$OUTPUT_DIR"