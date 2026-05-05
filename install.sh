#!/usr/bin/env bash
set -euo pipefail

REPO="wubinstu/adrm"
INSTALL_DIR="$HOME/.local/bin"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux)  PLATFORM="linux" ;;
    Darwin) PLATFORM="macos" ;;
    *)      echo "Error: Unsupported OS: $OS" >&2; exit 1 ;;
esac

case "$ARCH" in
    x86_64|amd64)  ARCH_NAME="x86_64" ;;
    aarch64|arm64) ARCH_NAME="arm64" ;;
    *)             echo "Error: Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

BINARY_NAME="adrm-${PLATFORM}-${ARCH_NAME}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"

echo "Installing adrm..."

mkdir -p "$INSTALL_DIR"

echo "Downloading ${BINARY_NAME}..."
curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/adrm"
chmod +x "${INSTALL_DIR}/adrm"

if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
    SHELL_RC="$HOME/.bashrc"
    if [ -n "${ZSH_VERSION:-}" ]; then
        SHELL_RC="$HOME/.zshrc"
    fi
    echo "" >> "$SHELL_RC"
    echo "export PATH=\"\${PATH}:${INSTALL_DIR}\"" >> "$SHELL_RC"
    echo "Added ${INSTALL_DIR} to PATH in ${SHELL_RC}"
    echo "Run 'source ${SHELL_RC}' or start a new shell to use adrm."
fi

echo "adrm installed successfully to ${INSTALL_DIR}/adrm"
