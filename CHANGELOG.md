# Changelog

## v1.0.0-rc.5 — 2026-05-18

### Documentation

- **[`docs/USAGE.md`](docs/USAGE.md)** — new comprehensive technical
  reference: every CLI command (synopsis + flags + exit codes + JSON
  schema), every TUI tab (layout + keybindings + behavior contracts),
  config schema, file layout, troubleshooting, design tradeoffs. README
  is now the elevator pitch; USAGE.md is the source of truth.
- `docs/delay-test.md` removed — content absorbed into USAGE.md ›
  Active delay test (deep dive).

### Added

- **Active delay test** via two entry points (full details in
  [USAGE.md › Active delay test](docs/USAGE.md#active-delay-test-deep-dive)):
  - **CLI**: `vpnkit test <group> [<node>]` with `--url` / `--timeout-ms`
    / `--json` flags. Defaults to mihomo's standard
    `https://www.gstatic.com/generate_204` + 5000 ms.
  - **TUI**: Groups tab → drill into right pane → `t` triggers
    `GroupDelay` on the selected group. Each node row shows the measured
    delay color-coded (green < 200 ms / yellow 200-500 / red > 500 or
    timeout).
  - Result coding: mihomo returns `0` for timeout — CLI text path renders
    `timeout`; CLI JSON path keeps `0` so machine consumers decide.

### Fixed

- README claim "🌐 Groups (read-only + delay test)" now matches reality
  (rc.4 advertised it but never wired the trigger).

---

## v1.0.0-rc.4 — 2026-05-18

### Fixed

- **Local Nodes `[e]` edit**: rc.3 advertised `[e] edit` in the list help
  but the Update handler had no case for it, so pressing `e` did nothing.
  Now opens the proto-driven form pre-filled with the highlighted node's
  current values; save updates the node in place (rename supported, name
  collision rolls back).

### Changed

- **Add/Edit Local Node form shortcuts** moved off `Ctrl+P` / `Ctrl+U`
  (potential terminal conflicts). New scheme:
  - Proto field has focus → `←/→` cycles through ss / vmess / vless /
    trojan / hysteria2 / tuic. Common fields (name, group, server, port,
    via) carry over across proto changes.
  - URI paste mode is entered from the list with `u` (unchanged).

### Removed

- **Settings → Extensions sub-page** and the `internal/extensions` package:
  chain proxies are now first-class on local nodes via the `Via` field, and
  the user opted out of the custom-`proxy-group` builder UI.
  `~/.config/vpnkit/extensions.toml` on disk is silently ignored; safe to
  delete manually.
- **CLI**: `vpnkit chain`, `vpnkit group`, `vpnkit ext` subcommands.
- **API**: `app.NewPipeline` signature is now `(st, configYAMLPath)` — the
  trailing `extensionsPath` argument is gone.

---

## v1.0.0-rc.3 — 2026-05-18

### Added

- **Multi local-nodes groups**: hand-entered nodes now belong to user-named
  groups (`home`, `office`, …) symmetric with subscriptions. Each enabled
  local group emits its own `<group>` (select) + `<group>-auto` (url-test).
  `vpnkit local-groups list/add/rm/enable/disable/rename` CLI.
- **`Via` field on local nodes** (first-class `LocalNode.Via`): writes through
  to mihomo's `dialer-proxy` so chains can be set inline at node creation
  time, no extensions overlay needed. Editable via `vpnkit local-nodes edit
  <node> via=<target>` or the TUI Add Node form.
- **Proto-driven Add Node form**: Sources › Local Nodes → `a` opens a
  multi-field form whose fields adapt to the chosen protocol (ss / vmess /
  vless / trojan / hysteria2 / tuic), including hy2/tuic `up`/`down` QoS
  limits. URI mode preserved as `[u]` from the list or `Ctrl+U` from the form.
- **`vpnkit local-nodes` extensions**: `--group/--via` flags, `mv` verb,
  `<group>:<name>` namespaced node references.
- **tmux TUI integration tests** (`test/tui/`): harness builds the binary
  once per run, drives a detached tmux session, captures pane output for
  assertions. Skipped gracefully when tmux is unavailable. `make test-tui`
  or `make test-all`.

### Changed

- TUI Local Nodes sub-page sprouts a group tab bar at the top with
  `←/→` switch, `N` new group, `D` delete group, `E` rename, `T` toggle
  enabled.
- Groups tab now lists every enabled local group as its own row (previously
  a single hardcoded `local`).

### Migrated automatically

- rc.2 stores with `[[local_nodes]]` but no `[[local_node_groups]]` are
  lazy-backfilled at first launch: a default `local` group is created and
  every node without an explicit `group` field is assigned to it. No
  `vpnkit init --force` required.

### Removed

Nothing (additive release).

---

## v1.0.0-rc.1 — 2026-05-17

**Pre-release.** Major architecture change. v0.10.x store files are
incompatible — see `docs/UPGRADE-v1.md`.

### Added

- **Multi-source subscription groups**: multiple subscriptions coexist; mihomo's
  `config.yaml` is now assembled by `internal/assembler` from all enabled
  subscriptions plus local nodes plus local rules.
- **Local nodes**: hand-entered proxy nodes (`ss / vmess / trojan / vless /
  hysteria2 / tuic`) managed via `vpnkit local-nodes` CLI and the TUI
  Sources tab.
- **Local rules CRUD**: structured rule entries via `vpnkit local-rules` and
  the TUI Rules tab's new Local Rules sub-page.
- **Routing knobs**: explicit `mode` (`rule | global | direct`) and
  `global_target` settings via `vpnkit mode` / `vpnkit target` and the TUI
  Settings → Routing sub-page.
- **TUI: Groups tab** — single view of all subscription groups + the local
  nodes group; selecting a node calls mihomo's controller to make it active.
- **TUI: Sources tab** — Subscriptions + Local Nodes sub-pages with add /
  delete / refresh / toggle-enable / edit.
- **TUI: Logs tab** — promoted from Settings sub-page to a top-level tab.
- **`vpnkit init --force`** — backs up an existing config.toml (any schema)
  and regenerates a fresh v2 store.

### Changed

- **Store schema bumped v1 → v2.** `active_profile` and `[[profiles]]` are
  replaced by `[[subscriptions]]`, `[[local_nodes]]`, `[[local_rules]]`,
  `mode`, `global_target`. Old store files are rejected with a remediation
  hint to run `vpnkit init --force`.
- **Default ports randomized** in [30000, 60000] (see v0.10.1) — carries over.
- **systemd-user unit** injects shell-level `HTTPS_PROXY` / `HTTP_PROXY` /
  `ALL_PROXY` / `NO_PROXY` env (carries over from v0.10.2).
- **GeoIP pre-seed at bootstrap** (carries over from v0.10.2).
- **TUI layout** now has 7 tabs: Dashboard, Groups, Sources, Rules,
  Connections, Logs, Settings. Old Proxies and Profiles tabs deleted.

### Removed

- `internal/profiles/` package entirely.
- `internal/subscription/assemble.go` (top-level path; protocol parsers in
  `internal/subscription/proto/` remain).
- `--restore` flag on `vpnkit init` (the v0.10 profile-restore concept is gone).
- `internal/tabs/profiles/` and `internal/tabs/proxies/` packages.

### Migration

See `docs/UPGRADE-v1.md` for the step-by-step path from v0.10.x.

---

## v0.10.2 — 2026-05-17

- `fix(service,installer): Bug P` — inject proxy env into mihomo systemd unit
  + bootstrap pre-downloads GeoIP/GeoSite files via SmartClient, sidestepping
  mihomo's hardcoded-90s downloader that ignores `HTTPS_PROXY`.

## v0.10.1 — 2026-05-17

- `fix(store): Bug O` — randomize default mihomo ports in [30000, 60000]
  using crypto/rand to avoid multi-user same-host collisions.

## v0.10.0 — 2026-05-17

Earlier releases tracked via git history. See `git log v0.9.3..v0.10.0` for
the direct-network + extensions overlay landing.
