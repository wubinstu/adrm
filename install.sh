#!/usr/bin/env bash
set -euo pipefail

REPO="wubinstu/adrm"

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

echo "=== adrm installer ==="
echo ""

# Ask binary install location
read -rp "Binary install location [default: ~/.local/bin]: " INSTALL_DIR
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_DIR="${INSTALL_DIR/#\~/$HOME}"

# Ask home directory
read -rp "adrm home directory [default: ~/.adrm/]: " ADRM_HOME
ADRM_HOME="${ADRM_HOME:-$HOME/.adrm/}"
ADRM_HOME="${ADRM_HOME/#\~/$HOME}"

# Ask to create alias
read -rp "Create 'rm' alias in shell config? [Y/n]: " CREATE_ALIAS
CREATE_ALIAS="${CREATE_ALIAS:-y}"

echo ""
echo "Installing adrm..."

mkdir -p "$INSTALL_DIR"

echo "Downloading ${BINARY_NAME}..."
curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/adrm"
chmod +x "${INSTALL_DIR}/adrm"

# Create adrm home directory
mkdir -p "${ADRM_HOME}"

# Detect shell config file
SHELL_RC=""
if [ -n "${ZSH_VERSION:-}" ]; then
    SHELL_RC="$HOME/.zshrc"
elif [ -n "${BASH_VERSION:-}" ]; then
    SHELL_RC="$HOME/.bashrc"
fi

# Add to PATH if needed
if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
    if [ -n "$SHELL_RC" ]; then
        echo "" >> "$SHELL_RC"
        echo "export PATH=\"\${PATH}:${INSTALL_DIR}\"" >> "$SHELL_RC"
        echo "Added ${INSTALL_DIR} to PATH in ${SHELL_RC}"
    fi
fi

# Set SAFE_RM_HOME in shell config
if [ -n "$SHELL_RC" ]; then
    if ! grep -q "SAFE_RM_HOME" "$SHELL_RC" 2>/dev/null; then
        echo "" >> "$SHELL_RC"
        echo "export SAFE_RM_HOME=\"${ADRM_HOME%/}\"" >> "$SHELL_RC"
        echo "Set SAFE_RM_HOME=${ADRM_HOME%/} in ${SHELL_RC}"
    fi
fi

# Create alias if requested
if [[ "$CREATE_ALIAS" =~ ^[Yy] ]]; then
    if [ -n "$SHELL_RC" ]; then
        # Remove old adrm alias if exists
        if grep -q "alias rm.*adrm" "$SHELL_RC" 2>/dev/null; then
            sed -i '/alias rm.*adrm/d' "$SHELL_RC"
        fi
        echo "" >> "$SHELL_RC"
        echo "alias rm=\"SAFE_RM_HOME=${ADRM_HOME%/} ${INSTALL_DIR}/adrm\"" >> "$SHELL_RC"
        echo "Created 'rm' alias in ${SHELL_RC}"
    fi
fi

echo ""
echo "adrm installed successfully to ${INSTALL_DIR}/adrm"
echo "Run 'source ${SHELL_RC}' or start a new shell to use adrm."
echo ""
echo "To generate default config: adrm --default"
echo "To view help: adrm --help"
