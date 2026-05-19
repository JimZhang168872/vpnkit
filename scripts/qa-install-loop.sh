#!/usr/bin/env bash
# qa-install-loop.sh — one iteration of the install smoke test.
#
# Spins a clean ubuntu:22.04 container, runs install.sh inside it as a
# non-root user (`tester`), and asserts the expected post-install state.
#
# Usage:
#   scripts/qa-install-loop.sh [VERSION]
#
# If VERSION is omitted, picks the most-recent release tag from GitHub
# (including pre-releases). Set LOG_DIR=… to override log location.

set -euo pipefail

REPO="JimZhang168872/vpnkit"
VERSION="${1:-}"
LOG_DIR="${LOG_DIR:-/tmp/vpnkit-qa-loop}"

if [ -z "$VERSION" ]; then
  VERSION=$(gh release list --repo "$REPO" --exclude-drafts --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null || true)
fi

if [ -z "$VERSION" ]; then
  echo "❌ could not resolve VERSION (no releases found, or gh not authenticated)" >&2
  exit 2
fi

mkdir -p "$LOG_DIR"
LOG="$LOG_DIR/round-$(date +%Y%m%d-%H%M%S)-${VERSION}.log"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IN_SCRIPT="$SCRIPT_DIR/qa-install-loop.in.sh"

if [ ! -f "$IN_SCRIPT" ]; then
  echo "❌ missing $IN_SCRIPT" >&2
  exit 2
fi

echo "🐳 round running with VERSION=$VERSION"
echo "   log → $LOG"

# If the host has a proxy configured (common behind GFW), forward it into the
# container. We use --network=host so 127.0.0.1:PORT proxies work as-is — this
# mirrors what a real user behind GFW does (run install.sh with their local
# clash/mihomo at 127.0.0.1 listening).
proxy_args=()
if [ -n "${HTTPS_PROXY:-${https_proxy:-}}" ]; then
  px="${HTTPS_PROXY:-${https_proxy:-}}"
  proxy_args+=(--network=host -e "HTTPS_PROXY=$px" -e "HTTP_PROXY=${HTTP_PROXY:-$px}" -e "NO_PROXY=${NO_PROXY:-127.0.0.1,localhost}")
  echo "   proxy: $px (via --network=host)"
fi

set +e
docker run --rm \
  "${proxy_args[@]}" \
  -e VERSION="$VERSION" \
  -v "$IN_SCRIPT:/tmp/qa-install.in.sh:ro" \
  ubuntu:22.04 \
  bash -c '
    set -euo pipefail
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq >/dev/null
    apt-get install -qq -y curl ca-certificates coreutils tar bash sudo >/dev/null
    useradd -m -s /bin/bash tester
    install -m 0755 -o tester -g tester /tmp/qa-install.in.sh /home/tester/qa-install.in.sh
    # Forward proxy env to tester user shell (su drops most env by default).
    if [ -n "${HTTPS_PROXY:-}" ]; then
      printf "export HTTPS_PROXY=%q\nexport HTTP_PROXY=%q\nexport NO_PROXY=%q\n" \
        "$HTTPS_PROXY" "${HTTP_PROXY:-$HTTPS_PROXY}" "${NO_PROXY:-127.0.0.1,localhost}" \
        >> /home/tester/.bashrc
      chown tester:tester /home/tester/.bashrc
    fi
    su - tester -c "VERSION=\"$VERSION\" HTTPS_PROXY=\"${HTTPS_PROXY:-}\" HTTP_PROXY=\"${HTTP_PROXY:-}\" NO_PROXY=\"${NO_PROXY:-}\" /home/tester/qa-install.in.sh"
  ' 2>&1 | tee "$LOG"
exit_code=${PIPESTATUS[0]}
set -e

echo ""
if [ "$exit_code" -eq 0 ]; then
  echo "✅ PASS"
  echo "   log: $LOG"
else
  echo "❌ FAIL (exit $exit_code)"
  echo "   log: $LOG"
  echo ""
  echo "--- last 60 lines ---"
  tail -n 60 "$LOG"
fi

exit "$exit_code"
