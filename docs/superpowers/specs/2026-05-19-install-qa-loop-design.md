# Install QA Loop — Design

**Date:** 2026-05-19
**Author:** Jim + Claude (Opus 4.7)
**Goal:** Iteratively run the real user install flow (`curl … install.sh | bash`) in a clean Docker container, find bugs, fix them, until two consecutive runs pass without any failure.

## Background

vpnkit v1.0.0 was prepared but its GitHub release was deleted by the user. Only `v1.0.0-rc.1/2/3` pre-releases remain. The install flow has known issues — most recently mihomo refusing to start post-install on the user's host. We need to catch these end-to-end before re-releasing.

## Scope

- **In scope**: `curl install.sh | bash` full path → `vpnkit init` bootstrap → mihomo binary fetch → geo MMDB fetch → default config write → systemd-user unit handling → re-install idempotency → `vpnkit uninstall`.
- **Out of scope**: TUI behavior, subscription import (no real server), profile/group form flows, real mihomo proxy traffic.

## Test environment

| Aspect | Choice | Reason |
|---|---|---|
| Container | `ubuntu:22.04` | Closest to typical user, no systemd init by default |
| systemd | Not running in container | Forces vpnkit to either degrade gracefully OR we mount one — see "Systemd handling" |
| Network | Host network, no special proxy | If user is behind GFW we test that separately (recovery recipe exists) |
| Test user | non-root with `~/.local/bin` on PATH | Matches real user expectations |

### Systemd handling

`ubuntu:22.04` has no PID 1 systemd. `systemctl --user` will fail. The loop treats this as a **feature test**: `vpnkit init` must either
- (a) detect no-systemd environment and skip unit install with a clear warning ("systemd-user unavailable — start mihomo manually with `vpnkit start` / equivalent"), OR
- (b) install the unit file but skip `systemctl --user start`, returning non-zero only on hard errors.

Both are acceptable; silently crashing is not. If current behavior crashes the loop, we fix it in the Go side.

A second-tier verification with `jrei/systemd-ubuntu:22.04` (systemd available) can be added later; not required for v1.0.0 sign-off.

## Smoke check (one iteration)

`scripts/qa-install-loop.sh` runs ONE iteration:

```
1. docker pull ubuntu:22.04 (cached)
2. docker run --rm ubuntu:22.04 sh -c "
     apt-get update -qq && apt-get install -qq -y curl ca-certificates coreutils tar bash
     useradd -m -s /bin/bash tester
     su tester -c '
       set -euxo pipefail
       export VERSION=v1.0.0-rc.4   # or whatever rc the loop is on
       export PATH=\"\$HOME/.local/bin:\$PATH\"
       curl -fsSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
       # --- assertions ---
       command -v vpnkit
       vpnkit --version | grep -E \"^vpnkit v?1\\.0\\.0\"
       test -f \"\$HOME/.config/vpnkit/config.toml\"
       test -f \"\$HOME/.config/mihomo/config.yaml\"
       test -x \"\$HOME/.local/bin/mihomo\" || test -x \"\$HOME/.local/share/vpnkit/mihomo\"
       vpnkit status >/dev/null   # must not crash
       # idempotency: re-run init
       vpnkit init || true        # tolerate non-zero, but not crash
       vpnkit status >/dev/null
       # uninstall must clean up
       vpnkit uninstall --yes
       ! test -f \"\$HOME/.config/vpnkit/config.toml\"
       ! command -v vpnkit
     '
   "
3. Capture exit code + last 100 lines of output
```

Pass/fail = exit 0 of the whole `docker run`.

## Iteration protocol

Loop driven by Claude in this session:

```
N = 1
PASS_STREAK = 0
loop:
  run scripts/qa-install-loop.sh
  if PASS:
    PASS_STREAK += 1
    if PASS_STREAK >= 2: break  # done
    continue
  else:
    PASS_STREAK = 0
    diagnose failure (last 100 lines)
    categorize:
      - install.sh shell bug → fix shell, commit `fix(install-qa-rN): ...`, push main, re-test (~10s)
      - Go-side bug (vpnkit init / bootstrap / systemd / geo / mihomo download) →
          fix Go, commit, bump tag to v1.0.0-rc.(N+4), push tag, wait for goreleaser (~3min), re-test
    N += 1
    report to user: round N, root cause, fix, what was pushed
```

### Per-iteration reporting

After each round Claude reports a single block:
```
Round N — FAIL: <one-line root cause>
Fix: <what changed, files>
Push: <main | tag v1.0.0-rc.X>
Next: re-running smoke
```

After a PASS:
```
Round N — PASS (streak 1/2)
Next: one more confirmation run
```

## Stop conditions

- Two consecutive PASS rounds → done; Claude summarizes total rounds, all fixes, and recommends next step (promote rc.N to v1.0.0 stable, or further manual testing on real host).
- User says stop.
- Repeated identical failure across 3 rounds with no progress → Claude pauses and asks for guidance.

## Authorization scope (user-granted for this task only)

- ✅ Push to `main` directly without per-commit confirmation
- ✅ Create + push `v1.0.0-rc.X` tags (X ≥ 4) without confirmation
- ❌ Push `v1.0.0` (stable)
- ❌ `git push --force` / `--force-with-lease`
- ❌ Open PRs
- ❌ Touch unrelated code ("hongshou clean up")

## Files this design will create/modify

- New: `scripts/qa-install-loop.sh` (smoke runner)
- Modified: `.goreleaser.yaml` (`prerelease: false` → `auto`)
- Modified: any source files needed to fix bugs found
- New tags: `v1.0.0-rc.4`, `rc.5`, … as needed

## Out-of-band: ECC-CI permissions

The release workflow runs goreleaser with `contents: write`. No additional secrets needed.
