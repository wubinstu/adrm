#!/usr/bin/env bash

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

# Read from /dev/tty so it works even when piped: curl ... | bash
if ! [ -t 0 ]; then
    exec 0</dev/tty
fi

read -rp "Binary install location [default: ~/.local/bin]: " INSTALL_DIR
if [ -z "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
fi
INSTALL_DIR="${INSTALL_DIR/#\~/$HOME}"

read -rp "adrm home directory [default: ~/.adrm]: " ADRM_HOME
if [ -z "$ADRM_HOME" ]; then
    ADRM_HOME="$HOME/.adrm"
fi
ADRM_HOME="${ADRM_HOME/#\~/$HOME}"
ADRM_HOME="${ADRM_HOME%/}"

read -rp "Create 'rm' alias in shell config? [Y/n]: " CREATE_ALIAS
if [ -z "$CREATE_ALIAS" ]; then
    CREATE_ALIAS="y"
fi

echo ""
echo "Installing adrm..."
echo ""

mkdir -p "$INSTALL_DIR"

echo "Downloading ${BINARY_NAME}..."
curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/adrm"
chmod +x "${INSTALL_DIR}/adrm"
echo "Downloaded to ${INSTALL_DIR}/adrm"

mkdir -p "${ADRM_HOME}"

# Detect shell config file
SHELL_RC=""
if [ -n "${ZSH_VERSION:-}" ]; then
    SHELL_RC="$HOME/.zshrc"
elif [ -n "${BASH_VERSION:-}" ]; then
    SHELL_RC="$HOME/.bashrc"
fi

if [ -z "$SHELL_RC" ]; then
    SHELL_RC="$HOME/.bashrc"
fi

# Remove old adrm entries from shell config
if [ -f "$SHELL_RC" ]; then
    sed -i '/# >>> adrm >>>/,/# <<< adrm <<</d' "$SHELL_RC"
fi

# Write new config block
{
    echo ""
    echo "# >>> adrm >>>"
    echo "export ADRM_HOME=\"${ADRM_HOME}\""
    echo "alias rm=\"${INSTALL_DIR}/adrm\""
    echo "# <<< adrm <<<"
} >> "$SHELL_RC"

echo ""
echo "adrm installed successfully!"
echo ""
echo "  Binary:  ${INSTALL_DIR}/adrm"
echo "  Home:    ${ADRM_HOME}"
echo "  Config:  ${SHELL_RC}"
echo ""
echo "Run 'source ${SHELL_RC}' or start a new shell to use adrm."
echo ""
echo "Quick start:"
echo "  adrm --default     # generate default config"
echo "  adrm --help        # show help"
