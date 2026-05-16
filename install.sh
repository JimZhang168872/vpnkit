#!/usr/bin/env bash
set -euo pipefail

# vpnkit installer
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
#   VERSION=v0.8.0 INSTALL_DIR=$HOME/bin bash <(curl -sSL .../install.sh)
#
# Env:
#   VERSION             pin a tag (default: latest non-prerelease release)
#   INSTALL_DIR         binary target (default: $HOME/.local/bin)
#   INSTALL_FORCE       1 = reinstall even when same version is already present
#   INSTALL_TAKEOVER    1 = overwrite ~/.config/mihomo/ if it was made by another clash tool

log()  { printf '%s\n' "$*"; }
warn() { printf '⚠️  %s\n' "$*" >&2; }
fail() { printf '❌ %s\n' "$*" >&2; exit 1; }

command -v curl       >/dev/null || fail "curl is required"
command -v sha256sum  >/dev/null || fail "sha256sum is required (coreutils)"
command -v tar        >/dev/null || fail "tar is required"

REPO="JimZhang168872/vpnkit"
DEST="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
VPNKIT_CFG="$CONFIG_HOME/vpnkit/config.toml"
MIHOMO_CFG="$CONFIG_HOME/mihomo/config.yaml"

# ───────── arch detect ─────────
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail "unsupported arch $arch (only amd64 / arm64 are released)" ;;
esac

# ───────── version resolve ─────────
if [ -z "${VERSION:-}" ]; then
  log "🔎 resolving latest release …"
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -oP '"tag_name":\s*"\K[^"]+' || true)
fi
[ -n "${VERSION:-}" ] || fail "cannot resolve latest version (set VERSION=v…)"

# ───────── pre-flight: existing install detection ─────────
backup_file=""
if [ -x "$DEST/vpnkit" ]; then
  current="$("$DEST/vpnkit" --version 2>/dev/null | head -1 | awk '{print $2}' || true)"
  if [ "v${current}" = "$VERSION" ] && [ -z "${INSTALL_FORCE:-}" ]; then
    log "✅ vpnkit $VERSION already installed at $DEST/vpnkit"
    log "   set INSTALL_FORCE=1 to reinstall anyway"
    exit 0
  fi
  log "🧹 found existing vpnkit ${current:-?} — running uninstall first"
  uninstall_out="$("$DEST/vpnkit" uninstall --yes --keep-profiles 2>&1 || true)"
  printf '%s\n' "$uninstall_out"
  backup_file="$(printf '%s' "$uninstall_out" | sed -n 's/^BACKUP=//p' | head -1)"
fi

# Refuse to overwrite a foreign ~/.config/mihomo/ unless explicitly allowed.
if [ -e "$MIHOMO_CFG" ] && [ ! -e "$VPNKIT_CFG" ]; then
  if [ -z "${INSTALL_TAKEOVER:-}" ]; then
    warn "$MIHOMO_CFG exists but no vpnkit config — likely from another clash tool"
    fail "set INSTALL_TAKEOVER=1 to overwrite, or move/remove it first"
  fi
  log "⚠️  taking over existing $MIHOMO_CFG (INSTALL_TAKEOVER=1)"
fi

# ───────── download ─────────
VER_NUM="${VERSION#v}"
TARBALL="vpnkit_${VER_NUM}_linux_${arch}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

log "⬇️  downloading $TARBALL …"
curl -fsSL -o "$tmp/$TARBALL" "$BASE/$TARBALL" || fail "download failed"
curl -fsSL -o "$tmp/SHA256SUMS" "$BASE/SHA256SUMS" || fail "checksum download failed"

if ( cd "$tmp" && grep " $TARBALL\$" SHA256SUMS | sha256sum -c - >/dev/null ); then
  log "✅ checksum verified"
else
  fail "checksum mismatch"
fi

tar -xzf "$tmp/$TARBALL" -C "$tmp"
mkdir -p "$DEST"
install -m 0755 "$tmp/vpnkit" "$DEST/vpnkit"
log "📦 installed $VERSION → $DEST/vpnkit"

# ───────── init config ─────────
log "🛠️  initializing config …"
if [ -n "$backup_file" ] && [ -f "$backup_file" ]; then
  "$DEST/vpnkit" init --restore "$backup_file" || warn "init with restore returned non-zero"
else
  "$DEST/vpnkit" init || warn "init returned non-zero"
fi

# ───────── done ─────────
log ""
log "🎉 vpnkit $VERSION ready"
log "   • $VPNKIT_CFG"
log "   • $MIHOMO_CFG"
log ""
log "next:"
log "  \$ vpnkit              # open TUI, add a subscription"
log "  \$ vpnkit status       # quick state check"
log "  \$ eval \"\$(vpnkit env --shell zsh)\"   # wire shell proxy env"
