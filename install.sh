#!/usr/bin/env sh
set -e

REPO="vuvietnguyenit/gpuxray"

if [ -z "$GITHUB_TOKEN" ]; then
  echo "Error: GITHUB_TOKEN environment variable is not set"
  echo "Example:"
  echo "  export GITHUB_TOKEN=ghp_xxxxxxxxx"
  exit 1
fi

# Detect OS
OS=$(uname | tr '[:upper:]' '[:lower:]')

# Detect ARCH
ARCH=$(uname -m)

# Normalize arch names
case "$ARCH" in
  x86_64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

FILE="gpuxray_${OS}_${ARCH}.tar.gz"

echo "Detecting latest release..."

RELEASE_JSON=$(curl -s \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://api.github.com/repos/$REPO/releases/latest)

VERSION=$(echo "$RELEASE_JSON" | grep tag_name | cut -d '"' -f4)

echo "Latest version: $VERSION"

ASSET_ID=$(echo "$RELEASE_JSON" | grep -B3 "$FILE" | grep '"id":' | head -n1 | tr -dc '0-9')

if [ -z "$ASSET_ID" ]; then
  echo "Could not find asset: $FILE"
  exit 1
fi

echo "Downloading asset $FILE"

TMP_DIR=$(mktemp -d)

curl -L \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Accept: application/octet-stream" \
  https://api.github.com/repos/$REPO/releases/assets/$ASSET_ID \
  -o "$TMP_DIR/$FILE"

tar -xzf "$TMP_DIR/$FILE" -C "$TMP_DIR"

sudo mv "$TMP_DIR/gpuxray" /usr/local/bin/gpuxray

echo "gpuxray installed successfully!"
gpuxray --help