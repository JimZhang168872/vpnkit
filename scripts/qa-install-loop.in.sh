#!/usr/bin/env bash
# qa-install-loop.in.sh — runs inside the container as user `tester`.
# Performs the actual install + assertions. Do not run on host.
set -euxo pipefail

export PATH="$HOME/.local/bin:$PATH"

# VERSION must be exported from outside.
: "${VERSION:?VERSION not set}"

echo "::: running install.sh with VERSION=$VERSION :::"
curl -fsSL "https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh" | bash

echo "::: post-install state :::"
ls -la "$HOME/.local/bin/" || true
ls -la "$HOME/.config/vpnkit/" || true
ls -la "$HOME/.config/mihomo/" || true
ls -la "$HOME/.local/share/vpnkit/" 2>/dev/null || true

echo "::: assertions :::"
command -v vpnkit
vpnkit --version
test -f "$HOME/.config/vpnkit/config.toml" \
  || { echo "FAIL: $HOME/.config/vpnkit/config.toml missing"; exit 1; }
test -f "$HOME/.config/mihomo/config.yaml" \
  || { echo "FAIL: $HOME/.config/mihomo/config.yaml missing"; exit 1; }
test -x "$HOME/.local/bin/mihomo" || test -x "$HOME/.local/share/vpnkit/mihomo" \
  || { echo "FAIL: mihomo binary not found at .local/bin or .local/share/vpnkit"; exit 1; }

vpnkit status >/dev/null || { echo "FAIL: 'vpnkit status' crashed"; exit 1; }

echo "::: idempotent re-init :::"
vpnkit init </dev/null || echo "::: init re-run returned non-zero (acceptable if it warned about existing config) :::"
vpnkit status >/dev/null || { echo "FAIL: 'vpnkit status' crashed after re-init"; exit 1; }

echo "::: uninstall :::"
vpnkit uninstall --yes </dev/null
test ! -f "$HOME/.config/vpnkit/config.toml" || { echo "FAIL: uninstall left vpnkit config"; exit 1; }
test ! -x "$HOME/.local/bin/vpnkit" || { echo "FAIL: uninstall left vpnkit binary"; exit 1; }

echo "✅ ALL ASSERTIONS PASSED"
