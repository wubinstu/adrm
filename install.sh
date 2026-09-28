#!/usr/bin/env bash
# adrm installer / uninstaller
#
#   curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
#   bash install.sh --uninstall            # clean removal
#
# Environment overrides:
#   ADRM_VERSION      release tag to install (default: latest)
#   ADRM_BASE_URL    where to fetch binaries from (default: GitHub releases)
#   ADRM_INSTALL_DIR binary install directory (default: ~/.local/bin)
#   ADRM_HOME        adrm home directory   (default: ~/.adrm)
#   ADRM_ALIAS       create the rm alias?  (default: 1)
#   ADRM_SHELLS      bash,zsh,fish         (default: detected from $SHELL)
set -euo pipefail

REPO="wubinstu/adrm"
BASE_URL="${ADRM_BASE_URL:-https://github.com/${REPO}/releases}"
VERSION="${ADRM_VERSION:-latest}"
INSTALL_DIR="${ADRM_INSTALL_DIR:-}"
ADRM_HOME_DIR="${ADRM_HOME:-}"
DO_ALIAS="${ADRM_ALIAS:-1}"
SHELLS="${ADRM_SHELLS:-}"
UNINSTALL=0
PURGE_HOME=0
ASSUME_YES=0
MARK_BEGIN="# >>> adrm >>>"
MARK_END="# <<< adrm <<<"

while [ $# -gt 0 ]; do
    case "$1" in
        --uninstall)   UNINSTALL=1 ;;
        --purge-home)  PURGE_HOME=1 ;;
        -y|--yes)      ASSUME_YES=1 ;;
        --version)     VERSION="$2"; shift ;;
        --install-dir) INSTALL_DIR="$2"; shift ;;
        --home)        ADRM_HOME_DIR="$2"; shift ;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \?//' | head -20
            exit 0 ;;
        *) echo "install.sh: unknown option: $1" >&2; exit 2 ;;
    esac
    shift
done

say()  { printf '%s\n' "$*"; }
warn() { printf 'install.sh: %s\n' "$*" >&2; }
die()  { warn "$*"; exit 1; }

expand_tilde() {
    case "$1" in
        "~")   echo "$HOME" ;;
        "~/"*) echo "$HOME/${1#\~/}" ;;
        *)     echo "$1" ;;
    esac
}

ask() { # ask <prompt> <default>
    if [ "$ASSUME_YES" = "1" ]; then
        printf '%s\n' "$2"
        return
    fi
    if [ ! -t 0 ]; then
        printf '%s\n' "$2"
        return
    fi
    local reply=""
    read -r -p "$1 [$2]: " reply || true
    if [ -z "$reply" ]; then printf '%s\n' "$2"; else printf '%s\n' "$reply"; fi
}

# ---------------------------------------------------------------- uninstall
if [ "$UNINSTALL" = "1" ]; then
    BIN="${INSTALL_DIR:-$HOME/.local/bin}/adrm"
    [ -n "${ADRM_HOME_DIR:-}" ] || ADRM_HOME_DIR="$HOME/.adrm"
    ADRM_HOME_DIR="$(expand_tilde "$ADRM_HOME_DIR")"

    say "== adrm uninstall =="
    # 1. shell integration in rc files (pure shell, works without the binary)
    rc_files="$HOME/.bashrc $HOME/.zshrc $HOME/.config/fish/config.fish"
    for rc in $rc_files; do
        [ -f "$rc" ] || continue
        grep -qF "$MARK_BEGIN" "$rc" || continue
        # delete the managed block, markers included
        awk -v b="$MARK_BEGIN" -v e="$MARK_END" '
            $0 == b { inblock=1; next }
            $0 == e { inblock=0; next }
            !inblock { print }
        ' "$rc" > "$rc.adrm.tmp"
        # collapse triple newlines left behind
        awk 'NF{blank=0;print;next} {blank++; if(blank<=1) print}' "$rc.adrm.tmp" > "$rc"
        rm -f "$rc.adrm.tmp"
        say "cleaned shell integration in $rc"
    done
    # 2. generated files inside the adrm home
    if [ -d "$ADRM_HOME_DIR" ]; then
        rm -rf "${ADRM_HOME_DIR%/}/completions" "${ADRM_HOME_DIR%/}/adrm-init.sh"
        say "removed shell integration files under $ADRM_HOME_DIR"
        if [ "$PURGE_HOME" = "1" ]; then
            if [ "$ASSUME_YES" != "1" ]; then
                read -r -p "delete $ADRM_HOME_DIR including everything in the trash? [y/N]: " reply || true
                case "$reply" in y|Y|yes) ;; *) say "kept $ADRM_HOME_DIR"; PURGE_HOME=0 ;; esac
            fi
            if [ "$PURGE_HOME" = "1" ]; then
                rm -rf "$ADRM_HOME_DIR"
                say "removed $ADRM_HOME_DIR"
            fi
        fi
    fi
    # 3. the binary itself
    if [ -e "$BIN" ]; then
        rm -f "$BIN"
        say "removed $BIN"
    else
        say "no binary at $BIN (already removed?)"
    fi
    say "uninstall complete."
    exit 0
fi

# ------------------------------------------------------------------- install
OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS" in
    Linux)  OS_NAME="linux" ;;
    Darwin) OS_NAME="macos" ;;
    *) die "unsupported OS: $OS (adrm supports Linux and macOS)" ;;
esac
case "$ARCH" in
    x86_64|amd64)  ARCH_NAME="x86_64" ;;
    aarch64|arm64) ARCH_NAME="arm64" ;;
    *) die "unsupported architecture: $ARCH" ;;
esac
ARTIFACT="adrm-${OS_NAME}-${ARCH_NAME}"

[ -n "$INSTALL_DIR" ] || INSTALL_DIR="$(ask 'Binary install location' "$HOME/.local/bin")"
[ -n "$ADRM_HOME_DIR" ] || ADRM_HOME_DIR="$(ask 'adrm home directory' "$HOME/.adrm")"
if [ -z "$SHELLS" ]; then
    DO_ALIAS="$(ask "Create the 'rm' alias (interactive shells only)" "$DO_ALIAS")"
    case "$(basename "${SHELL:-bash}")" in
        zsh|fish) SHELLS="$(basename "$SHELL")" ;;
        *)        SHELLS="bash" ;;
    esac
fi
INSTALL_DIR="$(expand_tilde "$INSTALL_DIR")"
ADRM_HOME_DIR="$(expand_tilde "$ADRM_HOME_DIR")"

# GitHub has no ".../download/latest/..." path: the "latest" channel lives at
# ".../releases/latest/download/..." (it redirects to the newest release),
# while an explicit version uses ".../releases/download/<tag>/...".
if [ "$VERSION" = "latest" ]; then
    URL="${BASE_URL%/}/latest/download/${ARTIFACT}"
    SUMS_URL="${BASE_URL%/}/latest/download/SHA256SUMS"
else
    URL="${BASE_URL%/}/download/${VERSION}/${ARTIFACT}"
    SUMS_URL="${BASE_URL%/}/download/${VERSION}/SHA256SUMS"
fi
TMPDIR_INSTALL="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_INSTALL"' EXIT

say ""
say "== adrm installer =="
say "  platform : $OS_NAME/$ARCH_NAME"
say "  binary   : $INSTALL_DIR/adrm"
say "  home     : $ADRM_HOME_DIR"
say "  shells   : $SHELLS"
say "  rm alias : $DO_ALIAS"
say ""

mkdir -p "$INSTALL_DIR"

say "downloading $URL ..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$URL" -o "$TMPDIR_INSTALL/adrm" || die "download failed"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMPDIR_INSTALL/adrm" "$URL" || die "download failed"
else
    die "neither curl nor wget found"
fi

# checksum verification when a SHA256SUMS file ships with the release
fetch() { # fetch <url> <out>
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$1" -o "$2"
    else
        wget -qO "$2" "$1"
    fi
}
if fetch "$SUMS_URL" "$TMPDIR_INSTALL/SHA256SUMS" >/dev/null 2>&1; then
    want="$(awk -v f="$ARTIFACT" '$2==f || $2=="*"f {print $1; exit}' "$TMPDIR_INSTALL/SHA256SUMS")"
    got="$(sha256sum "$TMPDIR_INSTALL/adrm" | awk '{print $1}')"
    if [ -n "$want" ] && [ "$want" = "$got" ]; then
        say "checksum verified"
    else
        warn "checksum verification failed (continuing; expected $want, got $got)"
    fi
else
    warn "no SHA256SUMS in the release, skipping checksum verification"
fi

install -m 0755 "$TMPDIR_INSTALL/adrm" "$INSTALL_DIR/adrm"
say "installed $INSTALL_DIR/adrm"

# shell integration + config via the binary itself (single source of truth)
mkdir -p "$ADRM_HOME_DIR"
export ADRM_HOME="$ADRM_HOME_DIR"
alias_flag="--no-alias"
[ "$DO_ALIAS" = "1" ] && alias_flag="--alias"
"$INSTALL_DIR/adrm" setup --install --shells "$SHELLS" "$alias_flag"

if [ ! -f "$ADRM_HOME_DIR/config" ]; then
    "$INSTALL_DIR/adrm" config --init
fi

"$INSTALL_DIR/adrm" doctor || true

say ""
say "done. open a new shell (or run: source ~/.bashrc) and try:"
say "  adrm --help"
say "  adrm ls"
