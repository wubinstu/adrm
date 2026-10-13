#!/usr/bin/env bash
# adrm installer / uninstaller
#
#   curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
#   bash install.sh --uninstall [--purge-home]   # clean removal
#
# By default only the binaries and the system-wide shell completions are
# installed: adrm (a safer rm) and adrmm (the trash bin manager). Each user
# then gets a private ~/.adrm home automatically on first use.
#
# Optional extras (opt-in):
#   --alias    also alias rm=adrm in your shell rc and take rm's completion
#   --timer    install a systemd --user timer that cleans expired items daily
#
# Environment overrides:
#   ADRM_VERSION      release tag to install (default: latest)
#   ADRM_BASE_URL     where to fetch binaries from (default: GitHub releases)
#   ADRM_INSTALL_DIR  binary directory (default: /usr/bin when writable,
#                     else ~/.local/bin)
#   ADRM_HOME         adrm home directory   (default: ~/.adrm)
#   ADRM_ALIAS        1 = install the rm alias (default: 0)
#   ADRM_TIMER        1 = install the systemd --user timer (default: 0)
#   ADRM_SHELLS       bash,zsh,fish (default: detected from $SHELL)
set -euo pipefail

REPO="wubinstu/adrm"
BASE_URL="${ADRM_BASE_URL:-https://github.com/${REPO}/releases}"
VERSION="${ADRM_VERSION:-latest}"
INSTALL_DIR="${ADRM_INSTALL_DIR:-}"
ADRM_HOME_DIR="${ADRM_HOME:-}"
DO_ALIAS="${ADRM_ALIAS:-0}"
DO_TIMER="${ADRM_TIMER:-0}"
SHELLS="${ADRM_SHELLS:-}"
UNINSTALL=0
PURGE_HOME=0
ASSUME_YES=0
MARK_BEGIN="# >>> adrm >>>"
MARK_END="# <<< adrm <<<"
BINARIES="adrm adrmm"

while [ $# -gt 0 ]; do
    case "$1" in
        --uninstall)   UNINSTALL=1 ;;
        --purge-home)  PURGE_HOME=1 ;;
        --alias)       DO_ALIAS=1 ;;
        --no-alias)    DO_ALIAS=0 ;;
        --timer)       DO_TIMER=1 ;;
        --no-timer)    DO_TIMER=0 ;;
        -y|--yes)      ASSUME_YES=1 ;;
        --version)     VERSION="$2"; shift ;;
        --install-dir) INSTALL_DIR="$2"; shift ;;
        --home)        ADRM_HOME_DIR="$2"; shift ;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \?//' | head -30
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
    if [ "$ASSUME_YES" = "1" ] || [ ! -t 0 ]; then
        printf '%s\n' "$2"
        return
    fi
    local reply=""
    read -r -p "$1 [$2]: " reply || true
    if [ -z "$reply" ]; then printf '%s\n' "$2"; else printf '%s\n' "$reply"; fi
}

# default_install_dir picks /usr/bin for root (or when writable), else the
# per-user ~/.local/bin, so "curl | bash" gives a system-wide adrm.
default_install_dir() {
    if [ "$(id -u)" = "0" ]; then
        echo "/usr/bin"
        return
    fi
    if [ -d /usr/bin ] && [ -w /usr/bin ]; then
        echo "/usr/bin"
        return
    fi
    echo "$HOME/.local/bin"
}

# ---------------------------------------------------------------- uninstall
if [ "$UNINSTALL" = "1" ]; then
    [ -n "$INSTALL_DIR" ] || INSTALL_DIR="$(default_install_dir)"
    [ -n "${ADRM_HOME_DIR:-}" ] || ADRM_HOME_DIR="$HOME/.adrm"
    INSTALL_DIR="$(expand_tilde "$INSTALL_DIR")"
    ADRM_HOME_DIR="$(expand_tilde "$ADRM_HOME_DIR")"

    say "== adrm uninstall =="
    # 0. let the binary remove what it owns (rc blocks, completions, timer)
    if [ -x "$INSTALL_DIR/adrmm" ]; then
        ADRM_HOME="$ADRM_HOME_DIR" "$INSTALL_DIR/adrmm" setup --uninstall ${PURGE_HOME:+--purge-home} || true
    fi
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
        # collapse repeated blank lines left behind
        awk 'NF{blank=0;print;next} {blank++; if(blank<=1) print}' "$rc.adrm.tmp" > "$rc"
        rm -f "$rc.adrm.tmp"
        say "cleaned shell integration in $rc"
    done
    # 2. generated files inside the adrm home
    if [ -d "$ADRM_HOME_DIR" ]; then
        rm -rf "${ADRM_HOME_DIR%/}/completions" "${ADRM_HOME_DIR%/}/adrm-init.sh"
        if [ "$PURGE_HOME" = "1" ]; then
            if [ "$ASSUME_YES" != "1" ]; then
                read -r -p "delete $ADRM_HOME_DIR including everything in the trash? [y/N]: " reply || true
                case "$reply" in y|Y|yes) ;; *) say "kept $ADRM_HOME_DIR"; PURGE_HOME=0 ;; esac
            fi
            if [ "$PURGE_HOME" = "1" ]; then
                rm -rf "$ADRM_HOME_DIR"
                say "removed $ADRM_HOME_DIR"
            fi
        else
            say "kept $ADRM_HOME_DIR (use --purge-home to delete it too)"
        fi
    fi
    # 3. the binaries themselves
    for b in $BINARIES; do
        if [ -e "$INSTALL_DIR/$b" ]; then
            rm -f "$INSTALL_DIR/$b"
            say "removed $INSTALL_DIR/$b"
        fi
    done
    # 4. global completion files, in case the binary was already gone
    for d in /usr/share/bash-completion/completions /etc/bash_completion.d \
             /usr/share/zsh/site-functions /usr/local/share/zsh/site-functions \
             /usr/share/fish/vendor_completions.d /usr/local/share/fish/vendor_completions.d; do
        for b in $BINARIES; do
            for f in "$d/$b.bash" "$d/$b.fish" "$d/_$b"; do
                [ -e "$f" ] && rm -f "$f" && say "removed $f"
            done
        done
    done
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
ARTIFACTS=""
for b in $BINARIES; do ARTIFACTS="$ARTIFACTS ${b}-${OS_NAME}-${ARCH_NAME}"; done
ARTIFACTS="${ARTIFACTS# }"

[ -n "$INSTALL_DIR" ] || INSTALL_DIR="$(ask 'Binary install location' "$(default_install_dir)")"
[ -n "$ADRM_HOME_DIR" ] || ADRM_HOME_DIR="$(ask 'adrm home directory' "$HOME/.adrm")"
if [ -z "$SHELLS" ]; then
    DO_ALIAS="$(ask "Create the 'rm' alias (interactive shells only)" "$DO_ALIAS")"
    DO_TIMER="$(ask "Install a systemd --user cleanup timer" "$DO_TIMER")"
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
    DL_PATH="latest/download"
else
    DL_PATH="download/${VERSION}"
fi
TMPDIR_INSTALL="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_INSTALL"' EXIT

fetch() { # fetch <url> <out>
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$1" -o "$2"
    else
        wget -qO "$2" "$1"
    fi
}

say ""
say "== adrm installer =="
say "  platform : $OS_NAME/$ARCH_NAME"
say "  binaries : $INSTALL_DIR/adrm $INSTALL_DIR/adrmm"
say "  home     : $ADRM_HOME_DIR (created per user on first use)"
say "  shells   : $SHELLS (system-wide completions)"
say "  rm alias : $DO_ALIAS    systemd timer : $DO_TIMER"
say ""

mkdir -p "$INSTALL_DIR"

for art in $ARTIFACTS; do
    url="${BASE_URL%/}/${DL_PATH}/${art}"
    say "downloading $url ..."
    fetch "$url" "$TMPDIR_INSTALL/$art" || die "download failed for $art"
done

# checksum verification when a SHA256SUMS file ships with the release
if fetch "${BASE_URL%/}/${DL_PATH}/SHA256SUMS" "$TMPDIR_INSTALL/SHA256SUMS" >/dev/null 2>&1; then
    ok=1
    for art in $ARTIFACTS; do
        want="$(awk -v f="$art" '$2==f || $2=="*"f {print $1; exit}' "$TMPDIR_INSTALL/SHA256SUMS")"
        got="$(sha256sum "$TMPDIR_INSTALL/$art" | awk '{print $1}')"
        if [ -z "$want" ] || [ "$want" != "$got" ]; then
            warn "checksum verification failed for $art (expected $want, got $got)"
            ok=0
        fi
    done
    [ "$ok" = "1" ] && say "checksums verified"
else
    warn "no SHA256SUMS in the release, skipping checksum verification"
fi

for art in $ARTIFACTS; do
    install -m 0755 "$TMPDIR_INSTALL/$art" "$INSTALL_DIR/${art%%-*}"
done
say "installed $INSTALL_DIR/adrm and $INSTALL_DIR/adrmm"

# shell integration + config via the management binary (single source of
# truth); completions go to the system-wide directories when writable.
mkdir -p "$ADRM_HOME_DIR"
export ADRM_HOME="$ADRM_HOME_DIR"
extra=""
[ "$DO_ALIAS" = "1" ] && extra="$extra --alias"
[ "$DO_TIMER" = "1" ] && extra="$extra --timer"
# shellcheck disable=SC2086
"$INSTALL_DIR/adrmm" setup --install --shells "$SHELLS" $extra

if [ ! -f "$ADRM_HOME_DIR/config" ]; then
    "$INSTALL_DIR/adrmm" config --init
fi

"$INSTALL_DIR/adrmm" doctor || true

say ""
say "done. open a new shell (or run: source ~/.bashrc) and try:"
say "  adrm --help      # delete files (a safer rm)"
say "  adrmm --help     # manage the trash bin"
say "  adrm file.txt    # recycle a file"
say "  adrmm ls         # see what is in the trash"
