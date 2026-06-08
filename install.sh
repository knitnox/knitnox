#!/bin/bash

# Code-Nimai Installer for Linux/macOS
# Usage: curl -sSL https://raw.githubusercontent.com/knitnox/knitnox/main/install.sh | sudo bash

BINARY_NAME="code-nimai"
INSTALL_DIR="/usr/local/bin"
REPO_URL="https://raw.githubusercontent.com/knitnox/knitnox/main/go_mcp-codebase-server"

# Detect Architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" == "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" == "aarch64" ] || [ "$ARCH" == "arm64" ]; then
    ARCH="arm64"
fi

# Construct download URL (assuming your release naming follows this pattern)
DOWNLOAD_URL="${REPO_URL}/${BINARY_NAME}-${OS}-${ARCH}"

echo "🚀 Installing Code-Nimai for ${OS} (${ARCH})..."

# Create temp directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR" || exit

# Download binary
echo "📥 Downloading from ${DOWNLOAD_URL}..."
if curl -sSL -O "$DOWNLOAD_URL"; then
    mv "${BINARY_NAME}-${OS}-${ARCH}" "$BINARY_NAME"
else
    echo "❌ Failed to download binary. Please ensure the URL is correct."
    exit 1
fi

# Set permissions
chmod +x "$BINARY_NAME"

# Move to global bin
echo "📦 Moving to ${INSTALL_DIR} (may require sudo)..."
if sudo mv "$BINARY_NAME" "$INSTALL_DIR/"; then
    echo "✅ Code-Nimai installed successfully!"
    echo "💡 Run it anywhere using the command: code-nimai"
else
    echo "❌ Failed to move binary to ${INSTALL_DIR}. Check your permissions."
    exit 1
fi

# Cleanup
rm -rf "$TMP_DIR"
