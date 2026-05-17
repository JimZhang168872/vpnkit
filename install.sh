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
#
# Network: this installer reaches github.com directly. If you're behind a
# restrictive network, configure HTTPS_PROXY in your shell before running
# (or use another box to download the tarball and run the binary's
# `vpnkit init` locally).

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
if [ -z "${VERSION:-}" ]; then
  warn "could not resolve latest version automatically"
  fail "set VERSION=v… and re-run"
fi

# ───────── pre-flight: existing install detection ─────────
#
# We do NOT call `vpnkit uninstall` here because pre-v0.8.0 binaries do not
# implement that subcommand (they fall through to the TUI which then dies on
# stdin-not-a-tty under `curl | bash`). We perform the cleanup ourselves in
# shell — it's straightforward and avoids the bootstrap-paradox.
backup_file=""
if [ -x "$DEST/vpnkit" ]; then
  current="$("$DEST/vpnkit" --version 2>/dev/null | head -1 | awk '{print $2}' || true)"
  if [ "v${current}" = "$VERSION" ] && [ -z "${INSTALL_FORCE:-}" ]; then
    log "✅ vpnkit $VERSION already installed at $DEST/vpnkit"
    log "   set INSTALL_FORCE=1 to reinstall anyway"
    exit 0
  fi
  log "🧹 found existing vpnkit ${current:-?} — cleaning up before reinstall"

  if [ -f "$VPNKIT_CFG" ] && grep -q '^\[\[profiles\]\]' "$VPNKIT_CFG"; then
    backup_file="/tmp/vpnkit-profiles-$(date +%Y%m%d-%H%M%S).toml"
    awk '/^\[\[profiles\]\]/{p=1} p' "$VPNKIT_CFG" > "$backup_file"
    chmod 600 "$backup_file"
    log "📦 backed up profiles → $backup_file"
  fi

  if [ -f "$CONFIG_HOME/systemd/user/mihomo.service" ]; then
    systemctl --user stop mihomo 2>/dev/null || true
    systemctl --user disable mihomo 2>/dev/null || true
    rm -f "$CONFIG_HOME/systemd/user/mihomo.service"
    systemctl --user daemon-reload 2>/dev/null || true
    log "🧹 removed systemd unit"
  fi

  STATE_HOME="${XDG_STATE_HOME:-$HOME/.local/state}"
  CACHE_HOME="${XDG_CACHE_HOME:-$HOME/.cache}"
  rm -rf \
    "$CONFIG_HOME/mihomo" \
    "$CONFIG_HOME/vpnkit" \
    "$STATE_HOME/vpnkit" \
    "$CACHE_HOME/vpnkit"

  rm -f "$DEST/vpnkit" "$DEST/mihomo"
  log "🧹 removed old binaries + config dirs"
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
curl -fsSL -o "$tmp/$TARBALL" "$BASE/$TARBALL" \
  || fail "download failed (configure HTTPS_PROXY if you're behind a restricted network)"
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
init_args=()
[ -n "$backup_file" ] && [ -f "$backup_file" ] && init_args+=(--restore "$backup_file")
"$DEST/vpnkit" init "${init_args[@]}" || warn "init returned non-zero"

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
