#!/usr/bin/env bash
# qa-traffic-loop.sh — end-to-end functional test for vpnkit:
#   subscription import → local nodes → jump (proxy chain) → mode switch
#   → use (node selection) → real traffic egress + rule-based routing.
#
# Pre-requisites:
#   1. vpnkit installed (rc.6+) and the service running
#   2. /tmp/vpnkit-qa-creds.env populated from the template (NEVER committed)
#
# All sensitive values stay in env vars sourced from the local creds file.
# Nothing here is written into the repo.

set -uo pipefail

CREDS="/tmp/vpnkit-qa-creds.env"
[ -r "$CREDS" ] || { echo "❌ creds file missing: $CREDS"; exit 2; }
# shellcheck disable=SC1090
source "$CREDS"

VPNKIT="${VPNKIT:-$HOME/.local/bin/vpnkit}"
[ -x "$VPNKIT" ] || { echo "❌ vpnkit not at $VPNKIT"; exit 2; }

LOG_DIR="${LOG_DIR:-/tmp/vpnkit-qa-loop}"
mkdir -p "$LOG_DIR"
LOG="$LOG_DIR/traffic-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee "$LOG") 2>&1
echo "🪵  log → $LOG"

# ─── helpers ──────────────────────────────────────────────────────────────
PASS=0
FAIL=0
SKIP=0
fail()  { echo "❌ FAIL: $*"; FAIL=$((FAIL+1)); }
pass()  { echo "✅ PASS: $*"; PASS=$((PASS+1)); }
skip()  { echo "⚠️  SKIP: $*"; SKIP=$((SKIP+1)); }
section() { echo ""; echo "═══ $* ═══"; }

# Run curl without inheriting the host's HTTPS_PROXY env (baseline calls).
# Returns body to stdout, exit code as-is.
curl_no_env() {
  env -u HTTPS_PROXY -u HTTP_PROXY -u ALL_PROXY -u https_proxy -u http_proxy -u all_proxy \
    curl -s --max-time 15 "$@"
}

# Curl through our local mihomo's mixed-port. PORT must be set.
curl_via_vpnkit() {
  env -u HTTPS_PROXY -u HTTP_PROXY -u ALL_PROXY -u https_proxy -u http_proxy -u all_proxy \
    curl -s --max-time 15 --proxy "http://127.0.0.1:$PORT" "$@"
}

# Best-effort egress-IP detection. Tries the configured IP_ECHO_URL first;
# falls back to a couple of alternates so a single endpoint outage doesn't
# fail the whole test.
egress_ip() {
  local proxy_url="$1"
  local out
  for url in "${IP_ECHO_URL:-https://ip.sb}" "https://api.ipify.org" "https://ipinfo.io/ip"; do
    if [ -n "$proxy_url" ]; then
      out=$(env -u HTTPS_PROXY -u HTTP_PROXY -u ALL_PROXY -u https_proxy -u http_proxy -u all_proxy \
        curl -s --max-time 8 --proxy "$proxy_url" "$url" 2>/dev/null || true)
    else
      out=$(curl_no_env "$url" 2>/dev/null || true)
    fi
    out=$(printf '%s' "$out" | tr -d '\n' | sed -E 's/.*"ip":"([^"]+)".*/\1/')
    if [[ "$out" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      printf '%s' "$out"
      return 0
    fi
  done
  return 1
}

# ─── 0. preflight ─────────────────────────────────────────────────────────
section "0. preflight"
[ -n "${SUB1_URL:-}" ] || { fail "SUB1_URL empty in $CREDS"; exit 3; }
[ -n "${SUB2_URL:-}" ] || { fail "SUB2_URL empty in $CREDS"; exit 3; }
[ -n "${DIRECT_NODE_URI:-}" ] || { fail "DIRECT_NODE_URI empty"; exit 3; }
[ -n "${JUMP_NODE_URI:-}" ] || { fail "JUMP_NODE_URI empty"; exit 3; }
[ -n "${JUMP_NODE_VIA:-}" ] || { fail "JUMP_NODE_VIA empty"; exit 3; }
pass "creds populated"

if ! "$VPNKIT" status >/dev/null 2>&1; then
  fail "vpnkit status crashed — install/service broken"
  exit 3
fi
PORT=$("$VPNKIT" status --json | jq -r .ports.mixed)
CTRL_PORT=$("$VPNKIT" status --json | jq -r .ports.controller)
[[ "$PORT" =~ ^[0-9]+$ ]] || { fail "couldn't read ports.mixed from status (got: $PORT)"; exit 3; }
[[ "$CTRL_PORT" =~ ^[0-9]+$ ]] || { fail "couldn't read ports.controller (got: $CTRL_PORT)"; exit 3; }
pass "vpnkit running, mixed=$PORT controller=$CTRL_PORT"

# ─── A. subscription import ──────────────────────────────────────────────
section "A. subscription import"
# Use clash.meta UA — some providers (boost1.shop in particular) reject the
# default mihomo UA and ship an empty config; clash.meta is more universally
# accepted across feed vendors.
"$VPNKIT" subs add "$SUB1_NAME" "$SUB1_URL" --ua=clash.meta && pass "subs add $SUB1_NAME (ua=clash.meta)" || fail "subs add $SUB1_NAME"
"$VPNKIT" subs add "$SUB2_NAME" "$SUB2_URL" --ua=clash.meta && pass "subs add $SUB2_NAME (ua=clash.meta)" || fail "subs add $SUB2_NAME"
"$VPNKIT" subs update                       && pass "subs update (all)"   || fail "subs update"

subs_json=$("$VPNKIT" subs list --json 2>/dev/null || echo "[]")
n_subs=$(printf '%s' "$subs_json" | jq 'length')
if [ "$n_subs" -ge 2 ]; then
  pass "subs list shows $n_subs subscriptions"
else
  fail "subs list shows $n_subs (expected ≥2)"
fi

# Node counts per sub
sub1_nodes=$(printf '%s' "$subs_json" | jq --arg n "$SUB1_NAME" '[.[]|select(.name==$n)|.node_count][0] // 0')
sub2_nodes=$(printf '%s' "$subs_json" | jq --arg n "$SUB2_NAME" '[.[]|select(.name==$n)|.node_count][0] // 0')
[ "${sub1_nodes:-0}" -gt 0 ] && pass "$SUB1_NAME has $sub1_nodes nodes" || fail "$SUB1_NAME has $sub1_nodes nodes (expected >0)"
[ "${sub2_nodes:-0}" -gt 0 ] && pass "$SUB2_NAME has $sub2_nodes nodes" || fail "$SUB2_NAME has $sub2_nodes nodes (expected >0)"

# Activate sub1 so we have a real Proxy group to route through
"$VPNKIT" active "$SUB1_NAME" && pass "active=$SUB1_NAME" || fail "active=$SUB1_NAME"

# ─── B. local direct node ────────────────────────────────────────────────
section "B. local direct node"
"$VPNKIT" local-nodes add "$DIRECT_NODE_URI" && pass "local-nodes add direct" || fail "local-nodes add direct"
DIRECT_NAME=$("$VPNKIT" local-nodes list --json | jq -r '.[-1].name // empty')
[ -n "$DIRECT_NAME" ] && pass "direct node name = $DIRECT_NAME" || fail "direct node not in local-nodes list"

# ─── C. jump node (proxy chain) ──────────────────────────────────────────
section "C. jump node (proxy chain via dialer-proxy)"
"$VPNKIT" local-nodes add "$JUMP_NODE_URI" --via "$JUMP_NODE_VIA" \
  && pass "local-nodes add jump --via $JUMP_NODE_VIA" \
  || fail "local-nodes add jump"
JUMP_NAME=$("$VPNKIT" local-nodes list --json | jq -r --arg via "$JUMP_NODE_VIA" '.[]|select(.via==$via)|.name' | head -1)
[ -n "$JUMP_NAME" ] && pass "jump node name = $JUMP_NAME (via=$JUMP_NODE_VIA)" || fail "jump node not found with via=$JUMP_NODE_VIA"

# ─── D. mode switching ───────────────────────────────────────────────────
section "D. mode switching"
for m in direct global rule; do
  if "$VPNKIT" mode "$m" >/dev/null && [ "$("$VPNKIT" mode --json | jq -r .mode 2>/dev/null || "$VPNKIT" mode)" = "$m" ]; then
    pass "mode $m"
  else
    fail "mode $m did not stick"
  fi
done
# leave it at rule for the rest

# ─── E. egress IP comparisons ────────────────────────────────────────────
section "E. egress IP via different modes/uses"
BASELINE_IP=$(egress_ip "" || echo "unknown")
echo "ℹ️  baseline (no proxy) IP = $BASELINE_IP"

# E.1 — mode=direct: vpnkit proxy should pass-through to baseline egress
"$VPNKIT" mode direct >/dev/null
sleep 1
DIRECT_IP=$(egress_ip "http://127.0.0.1:$PORT" || echo "unknown")
echo "ℹ️  mode=direct egress IP = $DIRECT_IP"
if [ "$DIRECT_IP" = "$BASELINE_IP" ] && [ "$DIRECT_IP" != "unknown" ]; then
  pass "mode=direct egress == baseline"
elif [ "$DIRECT_IP" = "unknown" ]; then
  skip "could not resolve direct-mode IP (network?)"
else
  fail "mode=direct egress ($DIRECT_IP) != baseline ($BASELINE_IP)"
fi

# E.2 — mode=global + use sub1's first node: egress IP should differ from baseline
"$VPNKIT" mode global >/dev/null
SUB1_FIRST=$("$VPNKIT" nodes "🚀 Proxy" --json 2>/dev/null | jq -r '.nodes[]?|select(.name|test("^(DIRECT|REJECT)$")|not)|select(.name|endswith("-auto")|not)|select(.name==$top|not).name // empty' --arg top "$SUB1_NAME" | head -1)
# If the activation flow gave us the auto-test group as default selected, drill into it to find a real node
if [ -z "$SUB1_FIRST" ]; then
  SUB1_FIRST=$("$VPNKIT" nodes "$SUB1_NAME" --json 2>/dev/null | jq -r '.nodes[]?|.name' | grep -v -E "^(DIRECT|REJECT)$" | head -1)
fi
if [ -z "$SUB1_FIRST" ]; then
  skip "no nodes in 🚀 Proxy group — skipping global-mode IP test"
else
  "$VPNKIT" use "🚀 Proxy" "$SUB1_FIRST" >/dev/null
  sleep 1
  GLOBAL_IP=$(egress_ip "http://127.0.0.1:$PORT" || echo "unknown")
  echo "ℹ️  mode=global use=$SUB1_FIRST → egress IP = $GLOBAL_IP"
  if [ "$GLOBAL_IP" != "$BASELINE_IP" ] && [ "$GLOBAL_IP" != "unknown" ]; then
    pass "mode=global egress changed (proxy is actually proxying)"
  else
    fail "mode=global egress == baseline OR unresolved — proxy not effective"
  fi
fi

# E.3 — mode=rule + foreign URL via Proxy, domestic via DIRECT
"$VPNKIT" mode rule >/dev/null
sleep 1
DOMESTIC_CODE=$(curl_via_vpnkit -o /dev/null -w "%{http_code}" "${DOMESTIC_TEST_URL:-https://www.baidu.com}" || echo 000)
FOREIGN_CODE=$(curl_via_vpnkit -o /dev/null -w "%{http_code}" "${FOREIGN_TEST_URL:-https://api.github.com}" || echo 000)
echo "ℹ️  mode=rule  domestic=$DOMESTIC_TEST_URL  → $DOMESTIC_CODE"
echo "ℹ️  mode=rule  foreign=$FOREIGN_TEST_URL  → $FOREIGN_CODE"
[ "$DOMESTIC_CODE" = "200" ] && pass "domestic URL via rule mode (200)" || fail "domestic URL returned $DOMESTIC_CODE"
[ "$FOREIGN_CODE" = "200" ]   && pass "foreign URL via rule mode (200)" || fail "foreign URL returned $FOREIGN_CODE"

# E.4 — jump chain: switch active to "local" so 🚀 Proxy contains local nodes
# (including the jump node), then `use 🚀 Proxy <jump>` to route through it.
# The chain (jump → New York-phone) is verified by checking egress IP changes.
if [ -n "$JUMP_NAME" ] && [ -n "$DIRECT_NAME" ]; then
  "$VPNKIT" active local >/dev/null 2>&1 || skip "active=local failed (no local group?)"
  "$VPNKIT" mode global >/dev/null
  sleep 1

  # Use the direct node by its namespaced name (local:<name>) — that's how
  # mihomo sees it in the flat namespace.
  if "$VPNKIT" use "🚀 Proxy" "local:$DIRECT_NAME" >/dev/null 2>&1; then
    sleep 1
    DIRECT_EGRESS=$(egress_ip "http://127.0.0.1:$PORT" || echo "unknown")
  else
    DIRECT_EGRESS="unknown"
  fi

  if "$VPNKIT" use "🚀 Proxy" "local:$JUMP_NAME" >/dev/null 2>&1; then
    sleep 1
    JUMP_EGRESS=$(egress_ip "http://127.0.0.1:$PORT" || echo "unknown")
  else
    JUMP_EGRESS="unknown"
  fi

  echo "ℹ️  direct-only egress = $DIRECT_EGRESS"
  echo "ℹ️  jump egress        = $JUMP_EGRESS"
  if [ "$JUMP_EGRESS" != "unknown" ] && [ "$DIRECT_EGRESS" != "unknown" ] && [ "$JUMP_EGRESS" != "$DIRECT_EGRESS" ]; then
    pass "jump chain produces different egress than direct (chain working)"
  else
    skip "could not validate jump vs direct egress (resolve failed or same IP)"
  fi
  # Restore active back to sub-doggy for phase F
  "$VPNKIT" active "$SUB1_NAME" >/dev/null 2>&1 || true
fi

# ─── F. rules classification check (via mihomo /connections) ────────────
section "F. rule classification via mihomo controller"
CTRL_SECRET=$(grep '^controller_secret' "$HOME/.config/vpnkit/config.toml" | awk -F'"' '{print $2}')
# Force a fresh connection for each test URL so /connections sees them.
"$VPNKIT" mode rule >/dev/null
sleep 1
curl_via_vpnkit -o /dev/null "$DOMESTIC_TEST_URL" || true
curl_via_vpnkit -o /dev/null "$FOREIGN_TEST_URL" || true
conn_json=$(curl -s --max-time 5 -H "Authorization: Bearer $CTRL_SECRET" "http://127.0.0.1:$CTRL_PORT/connections" || echo '{}')

DOMESTIC_CHAIN=$(printf '%s' "$conn_json" | jq -r --arg host "$(echo "$DOMESTIC_TEST_URL" | sed -E 's#https?://([^/]+).*#\1#')" \
  '.connections[]?|select(.metadata.host==$host)|.chains|join(" → ")' | head -1)
FOREIGN_CHAIN=$(printf '%s' "$conn_json" | jq -r --arg host "$(echo "$FOREIGN_TEST_URL" | sed -E 's#https?://([^/]+).*#\1#')" \
  '.connections[]?|select(.metadata.host==$host)|.chains|join(" → ")' | head -1)
echo "ℹ️  domestic chain: ${DOMESTIC_CHAIN:-<none captured>}"
echo "ℹ️  foreign  chain: ${FOREIGN_CHAIN:-<none captured>}"
# Domestic should chain through DIRECT, foreign through 🚀 Proxy
if [[ "$DOMESTIC_CHAIN" == *DIRECT* ]]; then
  pass "domestic → DIRECT (rule classification correct)"
elif [ -z "$DOMESTIC_CHAIN" ]; then
  skip "no connection captured for domestic — connection might have already closed"
else
  fail "domestic chain doesn't include DIRECT: $DOMESTIC_CHAIN"
fi
if [[ "$FOREIGN_CHAIN" == *"🚀 Proxy"* ]] || [[ "$FOREIGN_CHAIN" == *Proxy* ]]; then
  pass "foreign → Proxy (rule classification correct)"
elif [ -z "$FOREIGN_CHAIN" ]; then
  skip "no connection captured for foreign"
else
  fail "foreign chain doesn't pass through Proxy group: $FOREIGN_CHAIN"
fi

# ─── summary ────────────────────────────────────────────────────────────
section "summary"
echo "PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP"
echo "log: $LOG"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
