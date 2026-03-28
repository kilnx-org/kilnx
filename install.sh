#!/bin/sh
# Install kilnx CLI
# Usage: curl -fsSL https://raw.githubusercontent.com/kilnx-org/kilnx/main/install.sh | sh

set -e

REPO="kilnx-org/kilnx"
BINARY="kilnx"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect arch
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)             echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
  echo "Could not determine latest version"
  exit 1
fi

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

echo "Downloading kilnx v${VERSION} for ${OS}/${ARCH}..."
TMP=$(mktemp -d)
curl -fsSL "$URL" -o "${TMP}/${FILENAME}"
tar -xzf "${TMP}/${FILENAME}" -C "$TMP"

echo "Installing to ${INSTALL_DIR}/kilnx..."
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP}/kilnx" "${INSTALL_DIR}/kilnx"
else
  sudo mv "${TMP}/kilnx" "${INSTALL_DIR}/kilnx"
fi

rm -rf "$TMP"
echo "kilnx v${VERSION} installed successfully"
kilnx version
