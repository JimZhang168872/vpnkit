# Changelog

## v1.0.0 — 2026-05-19

First stable release of the v1.0 architecture. The active-source routing
model, embedded rule-set snapshot, bilingual docs, and ~80 stability
fixes from the rc.4 → rc.7 cycle are all in.

### Major features (since v1.0.0-rc.3)

- **Active source routing model.** One subscription OR one local-nodes
  group drives both the emitted rule list and `🚀 Proxy`'s membership.
  Switch with `vpnkit active <name>` or in Settings → Active Source.
  When the active source ships no rules of its own (every local-nodes
  group, plus subs that fetch as a single URI), the embedded
  loyalsoldier template fills in. The Groups tab marks the active
  source with `★`.
- **Node delay testing.** `vpnkit test <group> [<node>]` (CLI) and `t`
  in the Groups tab (TUI) actively probe each member's latency in
  parallel. Falls back through `/group/<name>-auto/delay`,
  `/group/<name>/delay`, then per-member `/proxies/<member>/delay`, so
  it works against vpnkit-generated Selectors AND user-authored
  url-test groups. See `docs/USAGE.md` → Active delay test for the deep
  dive.
- **Per-node `Via` chained egress.** Set on any local node (form's last
  field or `vpnkit local-nodes add --via=<proxy>`). Writes through to
  mihomo's `dialer-proxy` so the node dials out through another
  proxy/group first. Survives subscription refresh because it lives on
  the node, not on a separate `extensions/` table. The pre-v1
  `Extensions` subsystem is removed.
- **Embedded loyalsoldier rule-set snapshot.** vpnkit ships 13
  pre-fetched `.txt.gz` files in `internal/rules/rulesets/` (~2 MB gz
  in-binary; ~8 MB unpacked on disk). Bootstrap writes them to
  `~/.config/mihomo/ruleset/` before mihomo's first launch — RULE-SET
  rules work immediately on slow / GFW'd networks, no waiting on
  jsdelivr. mihomo's own `interval: 86400` refresh keeps the on-disk
  copies current.
- **Bilingual technical reference.** [`docs/USAGE.md`](docs/USAGE.md)
  and [`docs/USAGE_zh.md`](docs/USAGE_zh.md) are the comprehensive
  reference (every command, every TUI key, every JSON schema, store
  TOML layout, troubleshooting). The v0 → v1 migration guide is
  bilingual too.
- **Top-level + per-subverb `--help`.** `vpnkit --help` / `-h` / `help`
  prints the verb summary. Each category (`vpnkit subs --help`,
  `vpnkit local-nodes --help`, …) prints its own usage.

### CLI improvements

- `vpnkit active [<name>] [--json]` — the new routing primitive.
  Validates that `<name>` is an enabled subscription or local group.
- `vpnkit target` validates against the known source set; garbage like
  `"PROXY"` or `"../../etc/passwd"` is rejected at set time.
- Concurrent mutations are serialized via a POSIX flock on
  `~/.config/vpnkit/config.toml.lock`. 50 parallel `vpnkit subs add &`
  workers all land cleanly; read verbs bypass the lock and never starve.
- Mutation verbs reject `--json` with a clear error listing which verbs
  DO support JSON. Read-verb runtime failures emit
  `{"error":"…"}` JSON to stdout (parseable by consumer scripts).
- `vpnkit env --shell` validates `bash` / `zsh` / `fish`; unknown
  shells get a loud error instead of silent bash fallback. Output
  values are single-quoted so passwords with `$` / backtick survive
  `eval`.
- Default subscription User-Agent is `mihomo/v1.19.25` (replaces the
  pre-v1 `clash-verge` UA that some providers gate on).
- `subs update <name>` warns when a feed returns 0 nodes (almost always
  a sign of UA gating, malformed YAML, or an error page).
- Name validation across the board: subs + local-groups names reject
  shell metacharacters (`$ \` ; | & …`), reserved names (`DIRECT`,
  `REJECT`, `GLOBAL` — case-insensitive), and exceed 64 runes.
- `local-nodes mv` requires the destination group to exist (no more
  typo-creates-phantom-group). `local-rules add` validates the target
  + per-type payload (CIDR / regex / port).
- Unknown top-level verbs report a clear "unknown command" instead of
  cryptic `/dev/tty` errors.

### TUI improvements

- **Settings → Active Source** sub-page picks active in 2 keys.
- **Settings → Routing** + **Active Source** apply asynchronously — a
  long mihomo reload doesn't freeze the bubbletea event loop.
- **Settings → Service / Mihomo Core** action keys (`s`/`S`/`r`/`u`)
  also run async; UI stays responsive during systemctl calls and
  binary downloads.
- **Groups tab** right pane scrolls instead of truncating — a 50-node
  subscription's cursor stays visible past row 22. Indicator shows
  `[N-M/total]`. Same scrolling lands on **Sources** sub-pages.
- **Rules tab** sub-page toggle key is `T` (capital). `Tab` and
  `Shift+Tab` are reserved for the global tab cycler.
- **Logs tab** `p` toggles pause; header shows `[PAUSED]` when frozen.
- **`?`** flashes a keymap hint; `r` / `m` / `:` flash navigation
  hints (previously dead bindings).
- **Sources Local Nodes form** masks `password` / `uuid` /
  `obfs-password` as bullets so over-the-shoulder reading doesn't leak
  secrets.
- **Validation** in the form: ports must be 1-65535, protocol-specific
  required fields are enforced before save.
- **`Ctrl+C`** always quits, even from inside a form input.
- **Minimum terminal**: 60×16. Below that a "terminal too narrow"
  gate replaces a broken layout.
- **About page** shows the vpnkit version + commit prominently.

### Compatibility

- Schema unchanged (still v2). Stores from v1.0.0-rc.\* upgrade in
  place; on first launch `vpnkit` automatically derives `active_source`
  from the legacy `global_target = "<name>-auto"` field. No user action
  required.
- v0.10.x → v1.0.0 is still the same breaking jump documented in
  [`docs/UPGRADE-v1.md`](docs/UPGRADE-v1.md).

---

## v1.0.0-rc.6 — 2026-05-18

### Added

- **Embedded loyalsoldier rule-set snapshot.** vpnkit now ships 13
  pre-fetched `.txt.gz` files in `internal/rules/rulesets/` (gzipped
  2.1 MB; +2.6 MB on the stripped binary). Bootstrap writes them to
  `~/.config/mihomo/ruleset/` before mihomo's first start, so RULE-SET
  rules are usable immediately — no more waiting for jsdelivr.net on
  slow / GFW'd networks. mihomo's `interval: 86400` keeps the local
  copies up-to-date over time.
- **Bilingual technical reference.** [`docs/USAGE.md`](docs/USAGE.md)
  now has a Chinese twin at [`docs/USAGE_zh.md`](docs/USAGE_zh.md); the
  v0 → v1 migration guide gets [`docs/UPGRADE-v1_zh.md`](docs/UPGRADE-v1_zh.md).
  Top-of-doc cross-links keep the two in sync.

### Fixed

- **`vpnkit subs update`/Sources `u` doesn't reach mihomo: delay test
  immediately after refresh fails with `group "<name>" not found in
  /proxies`.** The Sources tab's `u` returned a private
  `refreshDoneMsg` from its tea.Cmd, but the app's top-level Update
  switch had no case for it — the message dropped into the implicit
  default and never reached `sourcesTab.Update`, so the chained
  `emitPipelineMutated()` never fired and `applyCfg` never ran.
  Config.yaml stayed at whatever it was before the refresh, mihomo's
  `/proxies` had no new group, and the subsequent delay test fell into
  the per-member fallback that finally surfaced the missing group.
  Exported `RefreshDoneMsg` + `RefreshErrMsg` from `tabsources` and
  added explicit forwarding in `app/update.go`.
- **狗狗加速 (doggygosubs) imports only 4 fake "client too old" nodes.**
  Their backend gates the response on `User-Agent`: `clash-verge/*` and
  `ClashforWindows/0.20.*` get 4 dummy `❗您的客户端版本太老❗` SS nodes
  instead of the real proxy list. Default UA changed from
  `clash-verge/v1.4.0` to `mihomo/v1.19.25` — also accepted by every
  other tested provider (boostnet, etc.) and matches the core we
  actually run. Existing users: `vpnkit subs update <name>` re-fetches
  with the new UA; no re-add needed.
- **Groups tab right pane truncated long node lists with no way to
  scroll.** A 50-node subscription rendered the first ~22 plus
  `… and 28 more`, hiding the rest. Now uses `viewport.Window` against
  `rightCursor`, so the cursor stays visible as it moves and the right
  header shows a `[start-end/total]` indicator when the list overflows.
- **Existing rc.5- users stuck on `GlobalTarget = "DIRECT"` after upgrade.**
  rc.6's first store migration bumped the self-loop default
  `"🚀 Proxy"` → `"DIRECT"`. The follow-up first-source-auto nudge
  (rc.6) only fires when `AddSubscription`/`AddLocalGroup` runs — so
  upgrading users whose subscriptions already existed never got the
  bump, and `MATCH,🚀 Proxy` resolved to direct connections forever.
  Now `store.Load` ALSO bumps: when `GlobalTarget == "DIRECT"` AND
  there's at least one enabled proxy source on disk, set it to that
  source's `-auto`. Persists once, then no-op on subsequent loads.
  Migration is ordered AFTER the lazy `local_node_groups` synthesis so
  rc.2 stores converge in a single Load (no second-load churn).
- **Default rules disappear after first edit + unmatched traffic goes
  direct instead of through proxy.** Two related bugs:
  - The rule template (loyalsoldier by default — its rule-providers +
    GEOIP/RULE-SET baseline) was only baked into config.yaml by
    `config.BuildSkeleton` at bootstrap time. The very first
    reassemble (any Sources mutation) replaced the file with a stripped
    version that had only the user's local rules + `MATCH,🚀 Proxy`.
    mihomo lost every RULE-SET reference, the user perceived "default
    rules took forever / never came back."
    Fix: `assembler.Input` now carries `RuleTemplate`; `Assemble`
    re-loads the embedded template every emit and merges
    `rule-providers` + baseline rules into the doc. Order is
    `local → subscription → template → MATCH,🚀 Proxy`.
  - When the user's first proxy source was added, vpnkit left
    `GlobalTarget = "DIRECT"` (rc.6 safe default). That made
    `MATCH,🚀 Proxy` resolve to DIRECT for unmatched traffic. Fix:
    `Pipeline.AddSubscription` / `Pipeline.AddLocalGroup` auto-nudge
    `GlobalTarget` to `<new-source>-auto` when it's still "DIRECT" and
    this is the very first proxy source. Subsequent adds and explicit
    user choices via `vpnkit target` are preserved.
- **`🚀 Proxy` self-loop crash at mihomo startup.** Old vpnkit
  (≤ rc.5) defaulted `store.Cfg.GlobalTarget` to `"🚀 Proxy"` — the
  name of the top-level Selector group itself. The assembler then
  emitted that name into its own member list, producing:
  ```
  - name: 🚀 Proxy
    type: select
    proxies: [🚀 Proxy, doge-auto, doge, Local-auto, Local, DIRECT]
                ↑ self-reference
  ```
  mihomo refuses to load this config with `Parse config error: loop
  is detected in ProxyGroup, please check following ProxyGroups: [🚀
  Proxy]` and crashes in a tight restart loop until `Start request
  repeated too quickly` hits.
  - **Three layers of defense:**
    1. `assembler.withTargetFirst` refuses to inject `"🚀 Proxy"` into
       any list — the choke point regardless of input source.
    2. `store.Load` migrates persisted `"🚀 Proxy"` → `"DIRECT"` and
       persists the change. New stores default to `"DIRECT"`.
    3. `app.Run` force-reassembles `config.yaml` on every launch so a
       drifted on-disk YAML can't outlive the store migration.
- **Delay test → `connection refused` doesn't tell user mihomo is down.**
  TUI Groups tab `[t]` now detects transport-layer failures (connection
  refused / EOF / Client.Timeout / no route to host) separately from real
  HTTP responses (`api.IsUnreachable`). On unreachable, it automatically
  calls `applyCfg` (reassemble + restart-via-service) and retries once.
  If still unreachable, the flash points the user to Settings → Service
  `[r]` and `journalctl --user -u mihomo`.
- **Sources mutations didn't reload mihomo** (the bug behind `❌ delay
  Local: group "Local" not found in /proxies`). TUI Sources tab CRUD
  (subscription add/rm/enable/disable/update, local-node add/rm/edit/mv,
  local-group add/rm/enable/disable/rename) wrote the store to disk but
  never regenerated `~/.config/mihomo/config.yaml` and never reloaded
  the running mihomo. Result: every subsequent `vpnkit use` /
  `vpnkit test` / TUI `t` got a 404 from the mihomo controller because
  the new group/node didn't exist in mihomo's runtime state.
  - TUI `PipelineMutatedMsg` handler now invokes `applyCfg` async after
    every Sources mutation, with `⏳ reloading mihomo…` → `✓ mihomo
    reloaded` / `❌ apply: …` flash.
  - CLI `vpnkit subs / local-groups / local-nodes / local-rules` each
    call the new `applyMutation` helper at the end of mutating verbs.
    Best-effort reload — if mihomo isn't running, the new config.yaml
    is still written to disk and picked up on next launch (with a
    stderr note so the user knows the reload was skipped).
- **Proxy URI parse failure when password contains literal `/`**. Real-world
  hysteria2 / trojan URIs from gulujili.xyz and other providers ship the
  password with unescaped `/` (RFC 3986 says `%2F`, but lenient form is
  accepted by Shadowrocket / Clash / NekoBox). Go's `net/url.Parse` was
  treating the `/` as the start of the URL path, so the password got
  truncated, host became gibberish, and `ParseURI` returned `parse(hy2):
  missing password (userinfo)`. Now `ParseURI` percent-encodes any `/`
  inside the userinfo segment before handing to `net/url.Parse`.
  Reported case:
  `hysteria2://CBAI0bv97b21KRjXw3fDArlnW/ymWTur@jim.gulujili.xyz:8443?...`
- **Delay test 404 on user-facing groups**. mihomo's `/group/<name>/delay`
  only accepts url-test / fallback / load-balance types, but vpnkit's
  Selectors (every `🚀 Proxy`, every subscription group, every local-
  nodes group) returned `404 Resource not found`. Both `vpnkit test
  <group>` and the TUI Groups tab `[t]` would error instead of measuring
  anything.
  - New `api.Client.MeasureGroup` runs a 3-step cascade: try
    `<name>-auto` (vpnkit's url-test companion) → `<name>` direct →
    parallel per-member `/proxies/<member>/delay`. Details in
    [USAGE.md › Group endpoint resolution](docs/USAGE.md#group-endpoint-resolution).
- **TUI double-namespacing of measured delays**. The Groups tab handler
  re-prefixed mihomo-returned keys with `<group>:` even though mihomo
  already namespaces them, so `delayByNode["doge:doge:HK-A"]` never
  matched the View's `"doge:HK-A"` lookup. Delays were measured but
  never rendered. Removed the prefix step.

---

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
