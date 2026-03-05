#!/usr/bin/env sh
set -e

REPO="vuvietnguyenit/gpuxray"

# Detect OS
OS=$(uname | tr '[:upper:]' '[:lower:]')

# Detect ARCH
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Get latest version from GitHub
VERSION=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep tag_name | cut -d '"' -f4)

FILE="gpuxray_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${VERSION}/${FILE}"

echo "Installing gpuxray $VERSION for $OS/$ARCH"

TMP_DIR=$(mktemp -d)

curl -L "$URL" -o "$TMP_DIR/gpuxray.tar.gz"

tar -xzf "$TMP_DIR/gpuxray.tar.gz" -C "$TMP_DIR"

sudo mv "$TMP_DIR/gpuxray" /usr/local/bin/gpuxray

echo "gpuxray installed successfully!"
gpuxray --help