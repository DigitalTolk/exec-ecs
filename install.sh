#!/bin/bash

set -e  # Exit on any error
set -u  # Exit on undefined variable

# Enable debug mode if requested
if [[ "${DEBUG:-}" == "true" ]]; then
    set -x  # Print commands before execution
fi

# Function to log messages
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $1"
}

# Function to log errors
error() {
    echo "[ERROR] $1" >&2
}

# Function to check command availability
check_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        error "Required command '$1' not found. Please install it first."
        exit 1
    fi
}

# Check for required commands
check_command curl
check_command tar
check_command chmod
check_command sudo

# Configuration
REPO="DigitalTolk/exec-ecs"

# Parse arguments
SPECIFIC_VERSION=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            SPECIFIC_VERSION="$2"
            shift 2
            ;;
        *)
            error "Unknown argument: $1"
            exit 1
            ;;
    esac
done

if [[ -n "$SPECIFIC_VERSION" ]]; then
    log "Specific version requested: $SPECIFIC_VERSION"
    VERSION="v$SPECIFIC_VERSION"  # Ensure it includes the 'v' prefix if needed
else
    log "Fetching latest release version..."
    VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$VERSION" ]]; then
        error "Failed to fetch latest version"
        exit 1
    fi
    log "Latest version: $VERSION"
fi

# Determine the platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

log "Detected OS: $OS"
log "Detected architecture: $ARCH"

# Map uname output to GoReleaser naming conventions
case "$OS" in
    linux)
        PLATFORM="Linux"
        ;;
    darwin)
        PLATFORM="Darwin"
        ;;
    *)
        error "Unsupported OS: $OS. Use the Windows .bat file for Windows support."
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH="x86_64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    armv6l|armv7l)
        ARCH="armv6"
        ;;
    i386)
        ARCH="i386"
        ;;
    *)
        error "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Construct the filename and download URL
FILENAME="exec-ecs_${PLATFORM}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

# Create temporary directory
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

# Download the archive
log "Downloading $FILENAME from $URL..."
if ! curl -Lo "${TEMP_DIR}/${FILENAME}" "$URL"; then
    error "Download failed"
    exit 1
fi

# Verify download
if [ ! -f "${TEMP_DIR}/${FILENAME}" ]; then
    error "Downloaded file not found"
    exit 1
fi

# Extract the archive
log "Extracting $FILENAME..."
cd "$TEMP_DIR"
if ! tar -xzf "$FILENAME"; then
    error "Extraction failed"
    exit 1
fi

# Install the binary
if [ ! -f "exec-ecs" ]; then
    error "Binary not found after extraction"
    exit 1
fi

log "Installing exec-ecs..."
chmod +x exec-ecs
if ! sudo mv exec-ecs /usr/local/bin/exec-ecs; then
    error "Installation failed"
    exit 1
fi

# Verify installation
if command -v exec-ecs >/dev/null 2>&1; then
    log "exec-ecs installed successfully! Version: $(exec-ecs --version)"
    log "Run 'exec-ecs --help' to get started."
else
    error "Installation verification failed"
    exit 1
fi

log "Installation completed successfully!"
