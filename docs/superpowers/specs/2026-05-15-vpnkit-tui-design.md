# vpnkit — TUI Clash Verge for mihomo (Design)

- **Date:** 2026-05-15
- **Author:** Jim (with Claude)
- **Status:** Draft for review
- **Scope:** A single Go binary that provides a Clash-Verge-style TUI for managing the mihomo proxy core on Linux, fully non-root.

---

## 1. Goals and Non-Goals

### Goals

- Provide a Clash-Verge-equivalent management experience for `mihomo` in a terminal UI.
- Run **fully non-root**: binary lives in `~/.local/bin`, config in `~/.config`, state in `~/.local/state`, cache in `~/.cache` (XDG-compliant).
- Treat mihomo installation as transparent plumbing — the user opens `vpnkit` and it works. No separate `install` step in normal usage.
- Support multi-format subscription conversion (vmess / ss / ssr / trojan / vless / hysteria(2) / tuic / SIP008 / Base64 list / Clash YAML) into mihomo-compatible YAML.
- Ship a sensible default rule set (Loyalsoldier rule-providers) so users don't have to hand-write rules.
- Manage mihomo as a long-running background service via `systemd --user` (preferred) or an internal PID-managed process (fallback).

### Non-Goals

- TUN / transparent proxy. We are **HTTP/SOCKS proxy only** (proxy via `HTTP_PROXY`/`SOCKS_PROXY` env). No `setcap`, no routing-table edits.
- GUI / web UI. Terminal only.
- Windows / macOS support (linux/amd64 + linux/arm64 only).
- Bundling mihomo itself. We download upstream releases from MetaCubeX/mihomo on demand.
- Acting as the mihomo replacement. We are a wrapper, not a fork.

---

## 2. Entry Points

```
$ vpnkit                          # Launch TUI (primary usage)
$ vpnkit env [--shell zsh|bash|fish]
                                  # Print shell export statements for HTTP_PROXY etc.
                                  # Kept as a non-TUI command because it must write to stdout
                                  # for `eval "$(vpnkit env)"` to work.
$ vpnkit --version                # Print vpnkit + mihomo version, exit
```

No other subcommands. Install, service, subscription, proxy switching, mode change — all live inside the TUI.

---

## 3. First-Run Bootstrap

When `vpnkit` starts and detects `~/.local/bin/mihomo` is missing:

1. Enter TUI on **Dashboard** tab; show inline progress: `Preparing mihomo core…`
2. Background tasks (silent, no wizard prompts):
   - Query GitHub Releases API for latest `mihomo` tag
   - Detect arch (`runtime.GOARCH`) and CPU compatibility (parse `/proc/cpuinfo` for `popcnt` / `sse4_2`; if missing → use `-compatible` build)
   - Download `mihomo-linux-<arch>[-compatible]-v<VER>.gz` to `~/.cache/vpnkit/downloads/`
   - SHA256-verify against release asset checksums
   - Gunzip → `chmod +x` → atomic rename to `~/.local/bin/mihomo`
3. Generate initial `~/.config/mihomo/config.yaml`:
   - Random `external-controller` secret (stored in `~/.config/vpnkit/config.toml`)
   - Default port `7890` (mixed-port)
   - Default rule template = `loyalsoldier` (see §6)
   - Empty proxies (until first subscription)
4. Install systemd unit at `~/.config/systemd/user/mihomo.service`; run `systemctl --user daemon-reload` + `enable --now`. If systemd-user is not available (some containers/WSL), fall back to internal PID mode and start mihomo as a managed background child.
5. Switch TUI focus to **Profiles** tab and show inline hint: `No subscription yet — press 'a' to add`.

Subsequent runs skip all of the above. They just attach to the running mihomo via its REST API and render.

Failure handling during bootstrap:
- Any step failing leaves TUI in a usable degraded state with a red banner: e.g. `mihomo core unavailable — Settings → Mihomo Core → Retry install`.
- Specifically, GitHub rate-limit / network failure suggests setting a mirror (Settings has a `Release mirror URL` field, defaults to direct GitHub).

---

## 4. TUI Architecture

### 4.1 Stack

- **Framework:** `github.com/charmbracelet/bubbletea` (Elm-style model/update/view)
- **Styling:** `github.com/charmbracelet/lipgloss`
- **Components:** `github.com/charmbracelet/bubbles` (table, viewport, textinput, spinner, progress)
- **YAML:** `gopkg.in/yaml.v3` (Node-level access preserves key order and comments)
- **HTTP / SSE / WS:** standard `net/http` + `github.com/coder/websocket` (for `/connections`)
- **TOML:** `github.com/BurntSushi/toml` (for vpnkit's own config)
- Go 1.22+, zero CGO. One binary.

### 4.2 Top-level Model

```go
type Model struct {
    activeTab     Tab
    tabs          [6]TabModel        // Dashboard, Proxies, Profiles, Connections, Rules, Settings
    statusBar     StatusBarModel
    helpOverlay   HelpModel
    cmdPalette    PaletteModel
    api           *api.Client         // mihomo REST client
    services      *service.Manager    // systemd-or-pid abstraction
    store         *store.Store        // ~/.config/vpnkit/config.toml accessor
    paths         paths.XDG
    flash         FlashStack          // transient toast messages
}
```

Each tab is its own `tea.Model` with isolated state; the top-level model dispatches messages and global keys.

### 4.3 Async Data Flow

Three long-lived `tea.Cmd`s on startup:

- `tickTraffic` — connects to `GET /traffic` SSE and emits `TrafficMsg{up, down}` per event. Restarts on disconnect with backoff.
- `tickConnections` — connects to `/connections` (WS); emits `ConnectionsMsg{snapshot}`.
- `tickProxies` — 5-second polling of `GET /proxies`; emits `ProxiesMsg{groups}`.

Tab models subscribe to relevant messages. When TUI exits, contexts are cancelled but mihomo is **not** killed (it lives in systemd / PID-mode as an independent process).

### 4.4 Layout

```
┌─ vpnkit ─────────────────────────────────────────────────────┐
│ [1] Dashboard │                                               │
│ [2] Proxies   │   <main area: current tab's view>             │
│ [3] Profiles  │                                               │
│ [4] Conn'ns   │                                               │
│ [5] Rules     │                                               │
│ [6] Settings  │                                               │
├───────────────┴───────────────────────────────────────────────┤
│ ● running  mode:rule  ↑1.2MB/s ↓0.3MB/s  sub:airport-A  ?:help│
└───────────────────────────────────────────────────────────────┘
```

Global keybindings:

| Key | Action |
|---|---|
| `1`-`6` | Jump to tab |
| `Tab` / `Shift+Tab` | Cycle tabs |
| `?` | Show / hide help overlay |
| `:` | Open command palette (see commands below) |
| `q` / `Ctrl+C` | Quit TUI (mihomo keeps running) |
| `m` | Cycle mihomo mode: rule → global → direct |
| `r` | Restart mihomo service |

Command palette (typed after `:`):

| Command | Action |
|---|---|
| `:quit` | Same as `q` |
| `:restart` | Restart mihomo service |
| `:stop` / `:start` | Stop / start service |
| `:reload` | Trigger `Profile.update` on active subscription and reload config |
| `:upgrade` | Run mihomo core upgrade flow |
| `:mode rule\|global\|direct` | Set mode directly |
| `:profile <name>` | Activate subscription by name (tab-completed) |

### 4.5 Tabs

| Tab | View | Key bindings |
|---|---|---|
| **Dashboard** | Sparkline of up/down traffic (last 60s), service status card, active subscription name, currently-selected nodes per group, total connections | `r` restart, `m` mode, `s` switch subscription (popup picker) |
| **Proxies** | Tree: group → nodes; columns: name, current selection, delay (ms), proxies-count | `↑↓` navigate, `Enter` switch node, `t` delay-test current group, `T` delay-test all groups |
| **Profiles** | List of subscriptions: name, last-updated, node count, ★ marks active | `a` add (popup: name + URL), `u` update selected, `U` update all, `Enter` activate (triggers update + restart), `d` delete, `e` edit URL/name |
| **Connections** | Live table: source → destination, rule matched, up, down, duration | `f` filter (textinput), `/` search, `k` close selected connection (`DELETE /connections/{id}`), `s` cycle sort |
| **Rules** | Current rules listing (paginated), rule-providers status block (last-updated per provider) | `u` refresh rule-providers, `/` search |
| **Settings** | Stacked sub-pages selectable from a left sub-sidebar: *Mihomo Core*, *Service*, *External Controller*, *Default Rules*, *Patch Editor*, *Logs Viewer*, *Cache*, *About* | `Enter` enter field, `Ctrl+S` save, `Esc` back |

Settings sub-pages:

- **Mihomo Core** — installed version, latest available, `Upgrade` action, toggle `-compatible` build, change release mirror URL.
- **Service** — current mode (`systemd-user` / `pid`), unit file path, start/stop/restart/uninstall buttons, `linger` status (with one-shot copy-paste `sudo loginctl enable-linger $USER` hint).
- **External Controller** — port, secret (regenerate button), allow-lan toggle.
- **Default Rules** — radio: `loyalsoldier` / `minimal` / `custom`. Custom opens a file picker for user-supplied `rules.yaml`.
- **Patch Editor** — full-screen textarea over `~/.config/mihomo/patch.yaml`; `Ctrl+T` validates with `mihomo -t`; `Ctrl+S` saves + triggers config rebuild.
- **Logs Viewer** — tail mihomo log (`journalctl --user -u mihomo -f` or `~/.local/state/vpnkit/mihomo.log`) and vpnkit log (`~/.local/state/vpnkit/vpnkit.log`).
- **Cache** — show sizes of `~/.cache/vpnkit/`, button to clear.
- **About** — versions, license, links.

---

## 5. Subscription Conversion

### 5.1 Auto-detection order

For each subscription URL, the response body is matched in this order:

| # | Format | Detection | Parser |
|---|---|---|---|
| 1 | Already Clash/mihomo YAML | Body parses as YAML and contains key `proxies` or `proxy-groups` | passthrough |
| 2 | SIP008 JSON | `Content-Type: application/json` and JSON has `version` + `servers` | `internal/subscription/sip008` |
| 3 | Base64 plaintext list | Whole body is valid base64; after decode contains `://` | decode → recursively dispatch each line |
| 4..N | Single-link parsers per protocol scheme | URL begins with `vmess://`, `ss://`, `ssr://`, `trojan://`, `vless://`, `hysteria://`, `hysteria2://`, `tuic://` | per-protocol parser in `internal/subscription/proto/<name>.go` |

Parsing is **best-effort per line**. Failures collected into a `[]ParseError`, reported in status bar and Logs Viewer; valid nodes proceed to YAML emission.

### 5.2 Protocol → mihomo field mapping (summary)

Each protocol parser returns a `mihomo.Proxy` (typed struct mirroring mihomo's YAML schema). Examples:

- `vmess://<base64-json>` → `{type: vmess, server, port, uuid, alterId, cipher, network, ws-opts/grpc-opts/h2-opts, tls}` per the V2RayN VMess spec.
- `ss://method:pass@host:port#name` → `{type: ss, cipher, password, server, port, plugin/plugin-opts if SIP003}`.
- `trojan://pass@host:port?sni=&allowInsecure=#name` → `{type: trojan, password, server, port, sni, skip-cert-verify}`.
- `vless://uuid@host:port?security=reality&pbk=&fp=...#name` → `{type: vless, uuid, network, tls, reality-opts, client-fingerprint, flow}`.
- `hysteria2://pass@host:port?obfs=salamander&obfs-password=&sni=#name` → `{type: hysteria2, password, server, port, obfs, obfs-password, sni}`.

Each parser has a table of fixture URLs in tests (≥5 per protocol: 3 valid variants, 1 edge, 1 malformed → error).

### 5.3 Group synthesis

If parsed result has no `proxy-groups`, synthesize the default template:

```yaml
proxy-groups:
  - { name: "🚀 Proxy",  type: select,   proxies: ["♻️ Auto", "🎯 Direct", <all-nodes>] }
  - { name: "♻️ Auto",   type: url-test, proxies: [<all-nodes>],
      url: "https://www.gstatic.com/generate_204", interval: 300, tolerance: 50 }
  - { name: "🎯 Direct", type: select,   proxies: [DIRECT] }
  - { name: "🛑 Reject", type: select,   proxies: [REJECT, DIRECT] }
```

If subscription already provides groups, leave them as-is.

### 5.4 Final config assembly

`config.yaml` is assembled fresh on every `Profile.update`:

```
final = merge(
    base_template,         // port/log-level/external-controller skeleton
    rule_template,         // default rule set (loyalsoldier|minimal|custom)
    subscription_data,     // proxies + (possibly) proxy-groups
    user_patch,            // ~/.config/mihomo/patch.yaml deep-merge LAST
)
```

Merge semantics: deep merge maps; arrays in patch **replace** (not append) to keep semantics predictable. Patch wins on conflict. Written atomically (`config.yaml.tmp` → `rename`).

---

## 6. Default Rule Set

### 6.1 `loyalsoldier` template (default)

Uses `rule-providers` to reference https://github.com/Loyalsoldier/clash-rules (well-maintained, GFW-list + China-IP + private-IP based):

```yaml
rule-providers:
  reject:    {type: http, behavior: domain, format: text,
              url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/reject.txt",
              path: ./ruleset/reject.yaml, interval: 86400}
  icloud:    {type: http, behavior: domain, url: "...icloud.txt", path: ./ruleset/icloud.yaml, interval: 86400}
  apple:     {type: http, behavior: domain, url: "...apple.txt", path: ./ruleset/apple.yaml, interval: 86400}
  google:    {type: http, behavior: domain, url: "...google.txt", path: ./ruleset/google.yaml, interval: 86400}
  proxy:     {type: http, behavior: domain, url: "...proxy.txt", path: ./ruleset/proxy.yaml, interval: 86400}
  direct:    {type: http, behavior: domain, url: "...direct.txt", path: ./ruleset/direct.yaml, interval: 86400}
  private:   {type: http, behavior: domain, url: "...private.txt", path: ./ruleset/private.yaml, interval: 86400}
  gfw:       {type: http, behavior: domain, url: "...gfw.txt", path: ./ruleset/gfw.yaml, interval: 86400}
  greatfire: {type: http, behavior: domain, url: "...greatfire.txt", path: ./ruleset/greatfire.yaml, interval: 86400}
  tld-not-cn:{type: http, behavior: domain, url: "...tld-not-cn.txt", path: ./ruleset/tld-not-cn.yaml, interval: 86400}
  telegramcidr: {type: http, behavior: ipcidr, url: "...telegramcidr.txt", path: ./ruleset/telegramcidr.yaml, interval: 86400}
  cncidr:    {type: http, behavior: ipcidr, url: "...cncidr.txt", path: ./ruleset/cncidr.yaml, interval: 86400}
  lancidr:   {type: http, behavior: ipcidr, url: "...lancidr.txt", path: ./ruleset/lancidr.yaml, interval: 86400}

rules:
  - RULE-SET,reject,🛑 Reject
  - RULE-SET,private,🎯 Direct
  - RULE-SET,direct,🎯 Direct
  - RULE-SET,lancidr,🎯 Direct
  - RULE-SET,cncidr,🎯 Direct
  - GEOIP,CN,🎯 Direct
  - RULE-SET,proxy,🚀 Proxy
  - RULE-SET,gfw,🚀 Proxy
  - RULE-SET,greatfire,🚀 Proxy
  - RULE-SET,tld-not-cn,🚀 Proxy
  - RULE-SET,telegramcidr,🚀 Proxy
  - MATCH,🚀 Proxy
```

### 6.2 `minimal` template

```yaml
rules:
  - GEOIP,CN,🎯 Direct
  - GEOIP,LAN,🎯 Direct
  - MATCH,🚀 Proxy
```

### 6.3 `custom`

User supplies their own `rules.yaml`. vpnkit reads it and merges (rules array fully replaces).

Templates ship embedded in the binary via `embed.FS` under `internal/rules/templates/`.

---

## 7. File System Layout

| Path | Owner | Purpose |
|---|---|---|
| `~/.local/bin/vpnkit` | user | CLI binary |
| `~/.local/bin/mihomo` | user (vpnkit-managed) | mihomo core binary |
| `~/.config/vpnkit/config.toml` | vpnkit | Subscription list, active sub name, controller secret, UI theme, mirrors |
| `~/.config/mihomo/config.yaml` | vpnkit-generated | Final config — regenerated on every Profile.update; never hand-edit |
| `~/.config/mihomo/patch.yaml` | user | Local overlay (hand-editable through Settings → Patch Editor) |
| `~/.config/mihomo/profiles/<name>.yaml` | vpnkit | Per-subscription cache (converted to Clash YAML form, source of truth for re-merge) |
| `~/.config/mihomo/ruleset/*.yaml` | mihomo-managed | Rule-provider files downloaded by mihomo itself |
| `~/.config/systemd/user/mihomo.service` | vpnkit-installed | systemd unit |
| `~/.local/state/vpnkit/mihomo.pid` | vpnkit (PID mode only) | PID file |
| `~/.local/state/vpnkit/mihomo.log` | vpnkit (PID mode only) | Combined stdout/stderr |
| `~/.local/state/vpnkit/vpnkit.log` | vpnkit | Self log (debug, async errors) |
| `~/.cache/vpnkit/downloads/` | vpnkit | mihomo `.gz` archives, by version |

`config.toml` example:

```toml
controller_secret = "abc123…"
controller_port = 9090
release_mirror = ""              # empty = direct github
active_profile = "airport-A"
rule_template = "loyalsoldier"   # loyalsoldier|minimal|custom
service_mode = "systemd-user"    # systemd-user|pid (auto-detected on first run)
ui_theme = "default"             # default|dark|light

[[profiles]]
name = "airport-A"
url = "https://example.com/sub?token=…"
user_agent = "clash-verge/v1.4.0"
last_updated = "2026-05-15T20:30:00Z"
```

---

## 8. mihomo REST API Usage

The mihomo `external-controller` listens at `127.0.0.1:9090`. vpnkit speaks to it with the bearer secret from `config.toml`.

| Endpoint | Verb | Used by |
|---|---|---|
| `/version` | GET | Service status card |
| `/configs` | GET | Settings page initial load |
| `/configs` | PATCH | Mode switching (`{"mode":"rule"}`) |
| `/configs` | PUT | Hot-reload config after `Profile.update` (`{"path":"~/.config/mihomo/config.yaml"}`) |
| `/proxies` | GET | Proxies tab (polled every 5s) |
| `/proxies/{group}` | PUT | Switch selection (`{"name":"HongKong-01"}`) |
| `/proxies/{name}/delay` | GET | Single-node delay test |
| `/group/{group}/delay` | GET | Group-wide delay test |
| `/connections` | WS | Connections tab |
| `/connections/{id}` | DELETE | Close connection |
| `/connections` | DELETE | Close all |
| `/traffic` | SSE | Dashboard sparkline |
| `/rules` | GET | Rules tab |
| `/providers/rules` | GET | Rule-providers status |
| `/providers/rules/{name}` | PUT | Refresh a rule-provider |

Client lives in `internal/api/`. All calls have a 5-second timeout (long-lived SSE/WS use context cancellation).

---

## 9. Service Management Abstraction

`internal/service/manager.go` exposes an interface:

```go
type Manager interface {
    Mode() Mode        // SystemdUser | PIDFile
    Install(ctx) error
    Uninstall(ctx) error
    Start(ctx) error
    Stop(ctx) error
    Restart(ctx) error
    Status(ctx) (Status, error)  // running, pid, since
    Logs(ctx, follow bool) (io.ReadCloser, error)
}
```

Two implementations:

- **`systemdUser`** — shells out to `systemctl --user`. Renders unit file from embedded template. On install: `daemon-reload` + `enable --now`. Logs via `journalctl --user -u mihomo`.
- **`pidFile`** — direct `os/exec.Cmd` with `SysProcAttr{Setsid: true}` to detach; PID written to `~/.local/state/vpnkit/mihomo.pid`; stdout+stderr appended to `mihomo.log`. Stop = `SIGTERM` then `SIGKILL` after 5s grace. Status reads PID file and `/proc/<pid>/comm`.

Detection on first run, in order:

1. `$XDG_RUNTIME_DIR/systemd/private` socket exists → systemd-user available.
2. Else, `systemctl --user show-environment` exits 0 → also fine.
3. Else, PID mode.

Result stored in `config.toml.service_mode`. User can force a mode via Settings → Service.

---

## 10. Error Handling

| Situation | Behavior |
|---|---|
| Download (release / subscription) fails | Retry 3× with exponential backoff (1s/2s/4s). Final failure → red flash + retry button. Never crash TUI. |
| GitHub rate-limit (HTTP 403 with `X-RateLimit-Remaining: 0`) | Banner: `GitHub rate-limited. Settings → Mihomo Core → Release Mirror`. |
| mihomo binary launch fails (port-in-use, bad config) | Service sub-page shows red error + last 30 lines of mihomo log. Suggest `:reload` or `Settings → Patch Editor`. |
| Subscription parse, partial failure | Skip failed nodes, accept the rest, status-bar toast `N nodes failed`. Detail in Logs Viewer. |
| Subscription parse, total failure | Profile entry shows error icon; activating it is disabled until fixed. |
| systemd-user unavailable (containers, WSL without systemd) | Silent fallback to PID mode. Settings shows current mode badge. |
| `external-controller` unreachable | Banner: `Lost connection to core`. Reconnect with backoff. Other TUI panes remain navigable (showing last-known data, greyed). |
| Atomic write fails mid-rename | Write to `<file>.tmp.NNN`, fsync, rename. If rename fails, leave previous good file untouched. |
| vpnkit's own config.toml unparseable | TUI enters recovery screen with the parse error, offering `[Backup & Reset]` or `[Quit]`. |

Internal code does not defensively wrap errors that "shouldn't happen"; it lets them surface to a top-level `tea.Cmd` boundary that logs and shows a flash. System boundaries (HTTP, file IO, subprocess) get explicit error returns.

---

## 11. Testing Strategy

Target: ≥ 80% coverage on `internal/`. TUI rendering is exempt (covered manually + via `teatest` interaction tests).

| Layer | Test approach |
|---|---|
| Protocol parsers (`internal/subscription/proto/*`) | Table-driven; ≥5 fixtures per protocol (valid variants + 1 edge + 1 malformed). Fixtures stored in `testdata/`. |
| Subscription fetch (`internal/subscription/fetch.go`) | `httptest.Server` returning each Content-Type + Base64 + redirect chain. |
| Config merger (`internal/config/merge.go`) | Golden-file: input triplet (base + rules + patch) → expected YAML output (byte-equal). |
| Default-rules templates | Snapshot test of embedded YAML files (catch accidental edits). |
| Installer (`internal/installer`) | Mock GitHub Releases server; verify version detection, arch selection, SHA mismatch error path. One real-CI integration test downloading a known small artifact. |
| API client (`internal/api`) | Mock mihomo server (`httptest.Server`) covering all endpoints in §8, including SSE / WS edge cases (early close, malformed event). |
| Service manager (PID mode) | Real fork of `/usr/bin/sleep 60` to test start/stop/status/restart semantics. Skipped if `/usr/bin/sleep` absent. |
| Service manager (systemd mode) | `systemctl --user` is wrapped in an interface and replaced with a fake in tests. One CI job on a systemd-enabled runner runs the real path. |
| TUI components | `github.com/charmbracelet/x/exp/teatest` — script keystrokes, snapshot final view. Cover: tab navigation, mode toggle, profile add flow, error flash on bad input. |
| End-to-end smoke | Single integration test in CI: install real mihomo (vendored small build), start, hit `/version`, stop. Tagged `// +build integration`. |

CI: GitHub Actions matrix `linux-amd64`, `linux-arm64`. Pipeline: `golangci-lint run` → `go test -race -cover ./...` → upload coverage.

---

## 12. Phase Plan

Implementation split into 4 phases, each independently usable. Each phase ends with: full tests green, ≥80% coverage on touched packages, manual smoke check, one PR.

- **Phase 1 — TUI skeleton + silent bootstrap.** bubbletea framework, 6 empty tabs (Dashboard renders status, others "Not implemented yet"), installer, service manager (both modes), basic config generation with `loyalsoldier` template, status bar live data via `/traffic`. Outcome: `vpnkit` launches, downloads mihomo, starts service, shows traffic graph. No subscriptions yet.
- **Phase 2 — Profiles + Proxies.** Subscription CRUD UI, all protocol parsers, conversion pipeline, group synthesis, patch overlay merge, Proxies tab (list/switch/delay-test). Outcome: vpnkit becomes daily-usable.
- **Phase 3 — Connections, Rules, Logs.** Connections tab (WS streaming, filter, close), Rules tab (list + provider refresh), Logs sub-page in Settings.
- **Phase 4 — Settings polish + extras.** Mihomo Core upgrade UI, Patch Editor with validate-on-save, theme switching, cache management, command palette, About page.

---

## 13. Module / Directory Layout

Go module name: `vpnkit`. Files relative to repo root (currently `/home/zhangjunming/workchain/vpn/`).

```
vpnkit/  (= repo root)
├── cmd/vpnkit/main.go          # entry: dispatch `env` / `--version` / TUI
├── internal/
│   ├── app/                    # top-level bubbletea Model / Update / View
│   ├── tabs/
│   │   ├── dashboard/
│   │   ├── proxies/
│   │   ├── profiles/
│   │   ├── connections/
│   │   ├── rules/
│   │   └── settings/           # sub-pages: core, service, controller, rules, patch, logs, cache, about
│   ├── api/                    # mihomo REST/SSE/WS client
│   ├── installer/              # GitHub release fetch, SHA check, unpack
│   ├── service/                # Manager interface + systemdUser + pidFile
│   ├── subscription/
│   │   ├── fetch.go            # HTTP fetch + content-type / base64 detect
│   │   ├── convert.go          # dispatch to per-protocol parsers
│   │   └── proto/              # vmess.go, ss.go, ssr.go, trojan.go, vless.go, hy2.go, tuic.go, sip008.go
│   ├── config/                 # merge.go (base+rules+sub+patch), assemble.go, atomic.go
│   ├── rules/                  # embedded templates + provider URL refresh helper
│   ├── store/                  # config.toml read/write
│   ├── paths/                  # XDG resolver
│   ├── env/                    # `vpnkit env` shell-snippet generator
│   └── log/                    # vpnkit's own logger
└── docs/superpowers/specs/2026-05-15-vpnkit-tui-design.md   # this file
```

---

## 14. Open Questions

None blocking. Items to revisit during implementation:

1. Should `m` (mode toggle) also be available as a global hotkey, or only within Dashboard? — current plan: global.
2. Subscription `User-Agent`: default to `clash-verge/v1.4.0` for max compatibility, override per-profile in Settings.
3. `linger` setup needs one-shot `sudo loginctl enable-linger $USER`. We can't avoid sudo here; the Service page surfaces the exact command for copy-paste rather than running it ourselves.

---

## 15. Out of Scope (Explicit)

- TUN mode
- Multi-user shared service
- Windows / macOS
- HTTPS MITM / sniffing
- DNS hijacking config
- Built-in node provider / paid-subscription integration
- Auto-update of vpnkit itself (manual `go install` or download from Releases)

End of design.
