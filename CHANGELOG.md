# Changelog

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
