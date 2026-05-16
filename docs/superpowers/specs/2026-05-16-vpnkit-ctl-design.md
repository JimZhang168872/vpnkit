# vpnkit CLI ctl subcommands (Design)

- **Date:** 2026-05-16
- **Author:** Jim (with Claude)
- **Status:** Draft for review
- **Scope:** Add 6 non-interactive top-level subcommands to vpnkit so common state queries and proxy/mode switching can be done from a shell prompt or a script, without launching the TUI.

---

## 1. Goals & Non-Goals

### Goals

- Read-only inspection commands: `status`, `ip`, `groups`, `nodes`.
- Write commands: `mode`, `use`.
- Human-readable default output, structured `--json` output for scripts.
- Stable, scripting-friendly exit codes.
- Zero new third-party dependencies. Stays consistent with vpnkit's current "no cobra" entry style.

### Non-Goals

- Subscription CRUD via CLI (`profile add` / `profile update`) — TUI-only for now. Out of scope.
- Delay-test command — defer; if needed it can wrap `GroupDelay` later.
- Service control via CLI (`start`/`stop`/`restart`) — `systemctl --user` already covers this; not adding a thin wrapper.
- Backwards-compatibility shims — these are net-new commands.

---

## 2. Entry shape

vpnkit's `cmd/vpnkit/main.go` currently does a tiny `switch os.Args[1]` between `--version`, `env`, and (default) the TUI. We extend that switch with the new subcommands; no cobra, no flag-package re-architecture.

```
$ vpnkit                              ← TUI (unchanged)
$ vpnkit env [--shell …] [--unset]    ← unchanged
$ vpnkit --version                    ← unchanged

$ vpnkit status              [--json]
$ vpnkit ip                  [--json]
$ vpnkit mode [rule|global|direct]  [--json]
$ vpnkit groups              [--json]
$ vpnkit nodes <group>       [--json]
$ vpnkit use <group> <node>  [--json]
```

`--json` is recognised by every new subcommand; everywhere it appears it makes the output a single line of valid JSON suitable for `jq`.

---

## 3. Per-subcommand reference

### 3.1 `vpnkit status`

Snapshot of the running mihomo + the active subscription.

**Human:**
```
mihomo  v1.19.16   ● running
mode    rule
ports   mixed=7890   controller=9090
groups  3 selectable (🚀 Proxy → HK-01, ♻️ Auto → HK-01, 🌍 Streaming → JP-02)
profile airport-A   23 nodes   updated 12m ago
```

If mihomo is unreachable: `mihomo  unreachable on 127.0.0.1:9090`, exit 2.

**JSON:**
```json
{
  "mihomo": {"version": "v1.19.16", "running": true},
  "mode": "rule",
  "ports": {"mixed": 7890, "controller": 9090},
  "groups": [
    {"name": "🚀 Proxy", "type": "Selector", "now": "HK-01", "members": 12},
    {"name": "♻️ Auto", "type": "URLTest", "now": "HK-01", "members": 10},
    {"name": "🌍 Streaming", "type": "Selector", "now": "JP-02", "members": 4}
  ],
  "active_profile": {"name": "airport-A", "node_count": 23, "last_updated": "2026-05-16T01:23:00+08:00"}
}
```

**Sources:** `GET /version` + `GET /configs` (new API method, see §5) + `GET /proxies`. Profile info read from `~/.config/vpnkit/config.toml` via `internal/store`.

---

### 3.2 `vpnkit ip`

Fetches `https://ipinfo.io/json` through mihomo's mixed-port (proves the proxy actually works and shows where the exit lands).

**Human:**
```
ip       203.0.113.42
country  HK
region   Central and Western
city     Hong Kong
org      AS12345 Example Hosting Ltd.
via      🚀 Proxy → HK-01
```

The `via` line names the user-selectable group whose `now` field is the chain `mihomo` is using. If multiple groups feed the route, we pick the first non-builtin Selector/URLTest.

**JSON:** ipinfo.io's response verbatim, plus a synthesised `"via"` key.

**Errors:**
- mihomo not reachable → `vpnkit ip: mihomo not reachable on 127.0.0.1:9090` exit 2.
- ipinfo.io timeout/non-2xx through proxy → `vpnkit ip: ipinfo.io unreachable through proxy` exit 2.

**Implementation note:** `http.Client` with `Transport.Proxy = http.ProxyURL("http://127.0.0.1:<mixed-port>")`. The mixed-port comes from `GET /configs` (mihomo is the source of truth — never re-read `~/.config/mihomo/config.yaml` from disk because it may have been hand-edited via patch.yaml).

---

### 3.3 `vpnkit mode [rule|global|direct]`

No arg: print current mode. With arg: set mode.

```
$ vpnkit mode
rule

$ vpnkit mode global
mode: rule → global

$ vpnkit mode foobar
vpnkit mode: invalid mode "foobar" (allowed: rule, global, direct)        [exit 1]
```

**JSON:**
- No arg: `{"mode": "rule"}`
- With arg: `{"from": "rule", "to": "global"}`

**Sources:** `GET /configs` (current) + `PATCH /configs` (set). Both already supported by `api.Client` (PATCH already exists; GET is the new method in §5).

---

### 3.4 `vpnkit groups`

Lists user-selectable proxy groups (Selector / URLTest / Fallback). Builtin entries — DIRECT, REJECT, REJECT-DROP, GLOBAL, PASS, COMPATIBLE — are filtered out.

```
GROUP            TYPE        CURRENT          MEMBERS
🚀 Proxy         Selector    HK-01            12
♻️ Auto          URLTest     HK-01            10
🌍 Streaming     Selector    JP-02             4
```

**JSON:**
```json
[
  {"name": "🚀 Proxy", "type": "Selector", "now": "HK-01", "members": 12},
  ...
]
```

**Sources:** `GET /proxies`.

---

### 3.5 `vpnkit nodes <group>`

```
$ vpnkit nodes 'Proxy'
✓ HK-01           45 ms
  JP-02           87 ms
  US-03          210 ms
  KR-04         (no test)
```

`✓` marks `proxies.{name}.now`. Delays are read from `proxies.{nodeName}.history[-1].delay` if present (mihomo's last health-check). Nodes without history show `(no test)`.

This command does NOT trigger a delay test. To force one, the user runs the TUI's `t` key on Proxies tab. (A `--test` flag is intentionally deferred — keep this command read-only.)

**JSON:**
```json
{
  "group": "Proxy",
  "current": "HK-01",
  "nodes": [
    {"name": "HK-01", "delay": 45},
    {"name": "JP-02", "delay": 87},
    {"name": "US-03", "delay": 210},
    {"name": "KR-04", "delay": null}
  ]
}
```

**Errors:** group not found → `vpnkit nodes: group "Foo" not found (try 'vpnkit groups')` exit 1. No fuzzy matching (YAGNI for v1).

**Sources:** `GET /proxies` (returns each entry's `history` array; need to extend `ProxyInfo` struct in §5 to include it).

---

### 3.6 `vpnkit use <group> <node>`

```
$ vpnkit use 'Proxy' 'HK-01'
✓ Proxy → HK-01
```

**JSON:** `{"group": "Proxy", "now": "HK-01"}`

**Errors:**
- Group not found → exit 1.
- Node not in group's `all` list → `vpnkit use: node "XX" not in group "Proxy"` exit 1. (Pre-validate locally to give a clean error before mihomo would 404.)
- Group is URLTest / Fallback (mihomo will reject manual selection on these; some versions accept it) → forward mihomo's error and exit 2.
- mihomo unreachable → exit 2.

**Sources:** `GET /proxies` (validate) + `PUT /proxies/{group}` (already in `api.Client`).

---

## 4. Common helpers (`cmd/vpnkit/cmd_common.go`)

Single shared file with utilities every subcommand uses:

```go
// loadClient reads ~/.config/vpnkit/config.toml and constructs an api.Client.
func loadClient() (*api.Client, *store.Store, error)

// parseFlags strips a leading `--json` from args. Returns (jsonOut, rest).
// We only support one flag — keep it dead simple, no flag.FlagSet ceremony.
func parseFlags(args []string) (jsonOut bool, rest []string)

// renderTable formats headers + rows into a fixed-width table on stdout.
func renderTable(out io.Writer, headers []string, rows [][]string)

// writeJSON marshals v compactly + ends with newline.
func writeJSON(out io.Writer, v any) error

// dieUserErr prints to stderr and exits 1.
func dieUserErr(format string, args ...any)

// dieRuntime prints to stderr and exits 2.
func dieRuntime(format string, args ...any)
```

`io.Writer` injection lets tests assert against captured output instead of stdout. Each `runX(out io.Writer, ...)` is the testable function; `cmd_X.go`'s entry function wraps it for the real `os.Stdout` and handles `os.Exit`.

---

## 5. New API methods (`internal/api/configs.go`)

Two additions:

```go
// Configs mirrors the mihomo /configs response (subset we use).
type Configs struct {
    Mode      string `json:"mode"`
    LogLevel  string `json:"log-level"`
    MixedPort int    `json:"mixed-port"`
    AllowLAN  bool   `json:"allow-lan"`
    Secret    string `json:"secret"`
}

// GetConfigs fetches /configs.
func (c *Client) GetConfigs(ctx context.Context) (Configs, error)
```

Plus we extend the existing `ProxyInfo` struct in `internal/api/proxies.go` to include the per-node delay history (currently we only decode `type` / `now` / `all`):

```go
type ProxyInfo struct {
    Type    string         `json:"type"`
    Now     string         `json:"now"`
    All     []string       `json:"all"`
    History []ProxyHistory `json:"history"`   // NEW (per-node delay measurements)
}

type ProxyHistory struct {
    Time  string `json:"time"`
    Delay int    `json:"delay"`
}
```

`History` is empty for groups; per-node entries (which appear in the same `/proxies` map under their node name) carry the most recent measurements.

---

## 6. File-by-file plan

| Path | Status | Purpose |
|---|---|---|
| `cmd/vpnkit/main.go` | MODIFY | extend the `switch os.Args[1]` with the 6 new cases |
| `cmd/vpnkit/cmd_common.go` | NEW | shared helpers from §4 |
| `cmd/vpnkit/cmd_status.go` | NEW | `status` subcommand |
| `cmd/vpnkit/cmd_ip.go` | NEW | `ip` subcommand |
| `cmd/vpnkit/cmd_mode.go` | NEW | `mode` subcommand |
| `cmd/vpnkit/cmd_groups.go` | NEW | `groups` subcommand |
| `cmd/vpnkit/cmd_nodes.go` | NEW | `nodes` subcommand |
| `cmd/vpnkit/cmd_use.go` | NEW | `use` subcommand |
| `cmd/vpnkit/cmd_*_test.go` | NEW (one per subcommand) | unit tests using `httptest.Server` mocked mihomo + captured stdout |
| `cmd/vpnkit/cmd_ip_test.go` | NEW | additionally mocks ipinfo.io |
| `internal/api/configs.go` | NEW | `Configs` struct + `GetConfigs` method + tests |
| `internal/api/proxies.go` | MODIFY | extend `ProxyInfo` with `History` |
| `internal/api/proxies_test.go` | MODIFY | assert `History` decoded |
| `README.md` + `docs/USAGE.md` | MODIFY | document the new subcommands |

---

## 7. Testing strategy

| Function | Approach |
|---|---|
| `runStatus(out, client, store, jsonOut)` | mock `/version` + `/configs` + `/proxies`; assert stdout contains expected lines (or valid JSON if `jsonOut=true`) |
| `runIP(out, client, ipinfoURL, jsonOut)` | inject ipinfo URL (test override); spin two `httptest.Server`s — mock mihomo and mock ipinfo |
| `runMode(out, client, args, jsonOut)` | one test per branch: no-arg / valid set / invalid set |
| `runGroups(out, client, jsonOut)` | assert builtins are filtered, table aligns |
| `runNodes(out, client, group, jsonOut)` | happy + group-not-found + nodes with mixed history presence |
| `runUse(out, client, group, node, jsonOut)` | happy + node-not-in-group + mihomo 4xx |
| `internal/api/configs_test.go` | `TestGetConfigs` |
| `internal/api/proxies_test.go` | extend `TestGetProxies` to also assert `History` decoded |

Coverage target ≥ 80% on `cmd/vpnkit/`. All tests `-race`.

---

## 8. Error handling & exit codes

| Code | Meaning | Examples |
|---|---|---|
| 0 | success | normal completion |
| 1 | user error | bad arg, group not found, node not in group, invalid mode value |
| 2 | runtime error | mihomo unreachable, mihomo 5xx, ipinfo timeout, file IO failure |

All error messages go to **stderr** with prefix `vpnkit <subcmd>: `. Stdout stays clean for piping.

---

## 9. Out of scope (explicit)

- `vpnkit profile <add|update|delete>` — TUI-only.
- `vpnkit service <start|stop|restart>` — `systemctl --user` already covers it.
- `vpnkit dash` (alternative TUI entry) — bare `vpnkit` is already that.
- Fuzzy group/node matching — pure exact-match only.
- Auto-completion (`bash`/`zsh`/`fish`) — defer; if added it's a separate `vpnkit completion <shell>` later.
- `vpnkit purge` — separate Phase 5d candidate.
- A `--no-color` flag — we render plain ASCII tables; no color in the new commands.

End of design.
