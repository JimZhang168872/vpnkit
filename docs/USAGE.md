# vpnkit Technical Reference

> 中文版 → [USAGE_zh.md](USAGE_zh.md)

The single most detailed source for using vpnkit — every CLI command, every
TUI tab, every keyboard shortcut, every config file, every JSON output
schema. Read top-to-bottom for a guided tour, or jump to a section.

- [Quick start](#quick-start)
- [Key concepts](#key-concepts)
- [CLI reference](#cli-reference)
- [TUI reference](#tui-reference)
- [File layout](#file-layout)
- [Configuration schema](#configuration-schema)
- [Active delay test (deep dive)](#active-delay-test-deep-dive)
- [Multi-user same-host installs](#multi-user-same-host-installs)
- [Troubleshooting](#troubleshooting)

---

## Quick start

```bash
# install (place binary on PATH and seed config)
mv vpnkit ~/.local/bin/
vpnkit init                                # creates ~/.config/vpnkit/config.toml

# subscribe to a feed
vpnkit subs add main "https://provider.example.com/sub?token=..."
vpnkit subs update                         # fetch nodes for all enabled subs

# get shell proxy env
eval "$(vpnkit env --functions)"           # adds proxy_on / proxy_off
proxy_on                                   # exports HTTPS_PROXY etc.

# open TUI
vpnkit                                     # 7-tab interactive UI
```

If `~/.local/bin` is not on `$PATH`, run `vpnkit env` once to print the
recommended `export PATH=...` snippet.

---

## Key concepts

| Term | Meaning |
|---|---|
| **mihomo** | the underlying proxy core ([MetaCubeX/mihomo](https://github.com/MetaCubeX/mihomo)). vpnkit assembles its config.yaml, launches it, and reads its controller API. |
| **store** | `~/.config/vpnkit/config.toml`. Single source of truth for subscriptions, local nodes/groups, local rules, ports, credentials, active source. Schema v2. |
| **source** | Generic term for "thing that holds proxies." A source is **either** a subscription **or** a local-nodes-group. Both emit a `<name>` select group + `<name>-auto` url-test group in mihomo — symmetric. |
| **subscription** | A remote URL returning a base64'd / clash-yaml node list. Default User-Agent is `mihomo/v1.19.25` (some providers gate on UA — see `vpnkit subs add --ua=...` for overrides). |
| **local node group** | User-named container for hand-entered nodes (e.g., `home`, `office`). |
| **Local node `Via`** | First-class `dialer-proxy` target on the node itself. Set to any proxy/group name to make mihomo dial through that hop first (Shadowrocket-style inline chaining). |
| **active source** | The one source whose rules drive routing AND whose nodes back `🚀 Proxy`. Set via `vpnkit active <name>` or Settings → Active Source. If the active source ships no rules (local-nodes groups never do), the loyalsoldier template fills in. The Groups tab marks it with `★`. |
| **Routing mode** | `rule` (default) / `global` / `direct`. Stored in `store.Cfg.Mode`. `rule` follows the assembled rule list; `global` sends everything to `🚀 Proxy`; `direct` bypasses the proxy entirely. |
| **Global target** | Which member of the `🚀 Proxy` Selector mihomo picks by default. Derived from the active source (`<active>-auto`); the `vpnkit target` CLI lets advanced users override it. |
| **Controller secret** | Random token in store, written into both mihomo's `secret:` and vpnkit's API client. Rotated on `init --force` or Settings → External Controller `r`. |
| **Service mode** | `systemd-user` (default on Linux with systemd) or `pid` (fallback, manages a pidfile + child process directly). Stored in `store.Cfg.ServiceMode`. |
| **Embedded rulesets** | The 13 loyalsoldier `.txt.gz` files are bundled into the vpnkit binary (~2 MB gz / ~8 MB uncompressed). Bootstrap unpacks them into `~/.config/mihomo/ruleset/` before mihomo's first launch, so RULE-SET rules work without a CDN round-trip. mihomo's own `interval: 86400` refresh keeps them current. |
| **Name validation** | Source names (subs + local-groups) must be ≤64 chars, no shell metacharacters (`$`, backtick, `;`, etc.), no `:` `/` whitespace, no leading `-`, no case-insensitive match against mihomo built-ins (DIRECT, REJECT, GLOBAL, …). Local-node names allow emoji + space (for parity with feed names) but block the same shell metas. |

---

## CLI reference

Conventions:
- `[ ]` = optional. `< >` = required positional.
- `--json` is supported on all read commands; output is single-document JSON
  (not NDJSON). Schema for each is shown below.
- Exit codes:
  - `0` = success (or "got the answer", e.g. node returned `timeout` is not an error)
  - `1` = user error (bad args, missing entity)
  - `2` = runtime error (mihomo unreachable, file IO failed, save failed)

### `vpnkit` *(no args)*

Opens the 7-tab TUI. Equivalent to running with no subcommand. See
[TUI reference](#tui-reference).

### `vpnkit version` (alias `--version`, `-v`)

Prints vpnkit semver + commit + build date + mihomo binary path & size.
Always exits 0.

### `vpnkit env [--shell bash|zsh|fish] [--unset] [--no-proxy CSV] [--functions] [--no-netrc]`

Prints shell-export snippet for proxy env vars and optionally writes
`~/.netrc` for basic-auth.

| Flag | Default | Meaning |
|---|---|---|
| `--shell` | from `$SHELL` | output dialect (bash/zsh/fish) |
| `--unset` | false | emit `unset`/`erase` instead of `export`/`set` |
| `--no-proxy` | `localhost,127.0.0.1,::1` | list assigned to `no_proxy` |
| `--functions` | false | emit `proxy_on` / `proxy_off` function defs (one-shot for `~/.zshrc`) |
| `--no-netrc` | false | skip writing `~/.netrc` even if creds present |

Always exits 0. Reads `mixed_port` / `proxy_user` / `proxy_pass` from store.

### `vpnkit status [--json]`

Snapshot: mihomo version, service running, ports, mode, # subscriptions,
# local nodes, # local rules, controller URL.

JSON shape (abridged):
```json
{
  "vpnkit_version": "v1.0.0-rc.7",
  "mihomo": {"version": "v1.19.25", "running": true, "pid": 12345},
  "ports": {"mixed": 7890, "controller": 9090},
  "mode": "rule",
  "active_source": "doge",
  "global_target": "doge-auto",
  "subscriptions": 2,
  "local_nodes": 3,
  "local_rules": 4
}
```

### `vpnkit ip [--json]`

Fetches `https://ipinfo.io/json` **through** mihomo's mixed-port so the
returned IP / country / region / org is the exit IP, not your real one.
Also reports which proxy group the request matched.

JSON shape:
```json
{"ip": "203.0.113.45", "country": "HK", "region": "Hong Kong",
 "city": "Hong Kong", "org": "AS12345 Example", "via": "🚀 Proxy → HK-01"}
```

Exit 2 if mihomo unreachable or ipinfo timed out.

### `vpnkit mode [rule|global|direct] [--json]`

No arg → print current. With arg → save into store, reassemble
config.yaml, hot-reload mihomo.

JSON (set): `{"from": "rule", "to": "global"}`
JSON (get): `{"mode": "rule"}`

### `vpnkit target [<member>] [--json]`

Get or set `global_target` — the `🚀 Proxy` Selector's default member.
Setting active source via `vpnkit active` automatically sets this to
`<active>-auto`, so most users never need `target` directly.

Validation: accepts `DIRECT`, `REJECT`, an enabled source name,
`<source>-auto`, or `<source>:<node>`. Disabled sources, garbage, and
empty strings are rejected at set time (no more silently-broken assemble
states).

### `vpnkit active [<name>] [--json]`

The single most important routing knob. Picks **which source** (one
subscription OR one local-nodes-group) drives:
- the emitted rule list (active's own rules, or loyalsoldier template
  if active has none)
- the `🚀 Proxy` Selector membership

No-arg form prints the current active source plus its kind
(`subscription` / `local`).

```bash
vpnkit active                 # show
vpnkit active doge            # switch to subscription `doge`
vpnkit active home            # switch to local-nodes-group `home`
vpnkit active --json          # → {"active_source":"doge","kind":"subscription"}
```

Setting active triggers an immediate reassemble + mihomo reload, so the
flip is live within seconds.

### `vpnkit subs <verb> ...`

| Verb | Synopsis | Action |
|---|---|---|
| `list` | `vpnkit subs list [--json]` | tabular list of subs + node count + URL |
| `add` | `vpnkit subs add <name> <url> [--ua USER_AGENT]` | append; fails on dup name |
| `rm` | `vpnkit subs rm <name>` | remove from store + drop cached result |
| `enable` | `vpnkit subs enable <name>` | flip `enabled` to true |
| `disable` | `vpnkit subs disable <name>` | flip `enabled` to false |
| `update` | `vpnkit subs update [<name>...]` | fetch + parse + cache. If no names → all enabled. Times out at 60s per sub. |

`list` JSON entry: `{"name": "main", "url": "...", "user_agent": "",
"enabled": true, "node_count": 50}`.

### `vpnkit local-groups <verb> ...`

| Verb | Synopsis | Action |
|---|---|---|
| `list` | `vpnkit local-groups list [--json]` | name + enabled |
| `add` | `vpnkit local-groups add <name>` | create empty group |
| `rm` | `vpnkit local-groups rm <name> [--force]` | delete; `--force` cascades nodes |
| `enable` | `vpnkit local-groups enable <name>` | flip enabled |
| `disable` | `vpnkit local-groups disable <name>` | flip enabled |
| `rename` | `vpnkit local-groups rename <old> <new>` | also moves member nodes (Group field rewrites in place) |

### `vpnkit local-nodes <verb> ...`

| Verb | Synopsis |
|---|---|
| `list` | `vpnkit local-nodes list [--json]` |
| `add` | `vpnkit local-nodes add <uri> [--group=NAME] [--via=PROXY]` |
| `rm` | `vpnkit local-nodes rm <ref>` |
| `mv` | `vpnkit local-nodes mv <ref> <new-group>` |
| `edit` | `vpnkit local-nodes edit <ref> key=val [key=val ...]` |

**Node ref** is either a short name (e.g. `JP-A`) or namespaced
(`group:JP-A`). Short refs error out as ambiguous if multiple groups have
a node by that name — always namespace when scripting.

**`add`** parses a proxy URI in any of the six supported schemes (ss /
vmess / vless / trojan / hysteria2 / tuic). `--group` defaults to `local`
and auto-creates the group if it doesn't exist. `--via` sets the
[Local node Via](#key-concepts) field, written through to mihomo as
`dialer-proxy`.

**`edit`** recognized keys: `name`, `group`, `via`, `server`, `port`, and
`proto`. Anything else (e.g. `password=...`, `cipher=...`) is written into
the `Fields` blob — proto-specific keys end up under whichever mihomo
field name they map to. Port is parsed as int; rest stay strings.

**`mv`** auto-creates the destination group.

`list` JSON entry: `{"name": "JP-A", "group": "home", "via": "doge-auto",
"proto": "hysteria2", "server": "jp.example.com", "port": 443, "fields":
{"password": "...", ...}}`.

### `vpnkit local-rules <verb> ...`

| Verb | Synopsis | Notes |
|---|---|---|
| `list` | `vpnkit local-rules list [--json]` | shows index + type + payload + target |
| `add` | `vpnkit local-rules add <type> <payload> <target>` | appended at end |
| `rm` | `vpnkit local-rules rm <idx>` | 0-based |
| `move` | `vpnkit local-rules move <from> <to>` | reorder (earlier = matches first) |

Common rule types: `DOMAIN`, `DOMAIN-SUFFIX`, `DOMAIN-KEYWORD`, `IP-CIDR`,
`PROCESS-NAME`, `MATCH`. Local rules are inserted **before** subscription-
supplied rules in the assembled mihomo config.

`list` JSON entry: `{"type": "DOMAIN-SUFFIX", "payload": "github.com",
"target": "🚀 Proxy"}`.

### `vpnkit groups [--json]`

Live `/proxies` snapshot from mihomo controller. Filters out built-in
GLOBAL/DIRECT/REJECT and non-user-selectable types.

JSON: array of `{"name": "doge", "type": "Selector", "now": "doge:HK-01",
"members": 12}`.

### `vpnkit nodes <group> [--json]`

Lists members of `<group>` with mihomo's **cached** delay (from its own
url-test loop). Use [`vpnkit test`](#vpnkit-test-group-node--url-url---timeout-ms-ms---json)
for fresh measurements.

JSON: `{"group": "doge", "current": "doge:HK-01", "nodes": [{"name":
"doge:HK-01", "delay": 234}, {"name": "doge:JP-02", "delay": null}]}`.
`delay: null` means "never tested".

### `vpnkit test <group> [<node>] [--url URL] [--timeout-ms MS] [--json]`

Active delay test — see [deep-dive section](#active-delay-test-deep-dive).
Single node if `<node>` given, otherwise group-wide concurrent test.
Defaults: `--url=https://www.gstatic.com/generate_204 --timeout-ms=5000`.

### `vpnkit use <group> <node> [--json]`

Calls mihomo's `PUT /proxies/<group>` to switch the active member. Node
name must be the mihomo-side name (namespaced as `<group>:<original>`
for subscription / local nodes — same as what shows in `vpnkit nodes`).

### `vpnkit init [--force]`

Without args: creates `~/.config/vpnkit/config.toml` if missing, picks
free TCP ports for mixed-port + controller-port, generates a random
controller secret + proxy basic-auth creds. No-op on existing store.

With `--force`: backs up the existing store to `config.toml.bak.<ts>` and
regenerates everything. Use to recover from a corrupt store or to rotate
all secrets at once.

### `vpnkit uninstall [--yes] [--purge] [--keep-mihomo] [--keep-profiles=true|false] [--backup-dir DIR]`

Best-effort removal: stops mihomo service, deletes systemd unit, all four
XDG dirs (`vpnkit` + `mihomo` + state + cache), and both binaries.

| Flag | Default | Effect |
|---|---|---|
| `--yes` | false | skip the interactive confirmation prompt |
| `--purge` | false | delete profiles (no backup) — implies `--keep-profiles=false` |
| `--keep-mihomo` | false | don't delete `~/.local/bin/mihomo` |
| `--keep-profiles` | true | back up profiles to `--backup-dir` (set false to drop) |
| `--backup-dir` | `/tmp` | where the profile backup tarball lands |

Exits 2 if `HOME` is unset/relative (refuses to operate to avoid wiping
cwd). Emits a `BACKUP=<path>` line on stdout when a backup was created —
install/uninstall scripts can grep for it.

### `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]`

Checks GitHub releases for newer vpnkit + mihomo, prompts (unless
`--yes`), downloads, swaps binaries, re-execs vpnkit on self-update.

| Flag | Effect |
|---|---|
| `--check` | print plan, don't install |
| `--yes` | skip confirmation |
| `--vpnkit-only` | only update vpnkit |
| `--mihomo-only` | only update mihomo |

---

## TUI reference

Launch with `vpnkit` (no args). Two-level focus model:
- **MainSidebar focus** — top tab list owns ↑/↓ to cycle tabs.
- **TabBody focus** — active tab owns ↑/↓ for its own navigation.

`←` always moves focus toward MainSidebar (back). `→` always moves focus
into the tab body / sub-page content. `1`–`7` jump to tab. `Tab`/`Shift+Tab`
cycle. `q` / `Ctrl+C` quit.

When a text-input overlay is open (Sources Add form, Connections filter,
Rules filter) **every** key — including digits and Tab — is delivered to
the input. No global hijack.

### Tab 1: Dashboard

Single pane. Shows:
- service status (●/○ running/stopped) + mode + PID
- mihomo version
- ports (mixed-port + external-controller)
- live traffic (↑ up, ↓ down — auto-formatted B/s, KiB/s, MiB/s)
- update badge if newer vpnkit/mihomo on GitHub

Read-only. No tab-specific keys.

### Tab 2: Groups

Two-pane.

Left pane lists every proxy group:
```
▶ doge (12)         → doge:HK-01
  boost (8)         → boost:relay-1
  home (3)          → home:JP-A
```
The `→ <name>` suffix is mihomo's current `now` (active member).

Right pane lists members of the highlighted group:
```
▶ ● doge:HK-01      hysteria2  hk.example.com:443      234 ms
    doge:JP-02      vmess      jp.example.com:443      567 ms
    doge:SG-03      trojan     sg.example.com:443      timeout
```
`● ` marks current `now`. Trailing `XXX ms` / `timeout` appears after
running a delay test in this session (colored: green <200ms, yellow
200-500, red >500 or timeout). No measurement → no suffix.

| Key | Action |
|---|---|
| `←` / `→` | focus left ↔ right pane |
| `↑` / `↓` (left) | move group cursor; reset right cursor to 0 |
| `↑` / `↓` (right) | move node cursor within current group |
| `r` | refresh group list from store |
| `t` | active delay test for current group ([details](#active-delay-test-deep-dive)) |
| `Enter` (right pane) | `PUT /proxies/<group>` switch to highlighted node |
| `Enter` (left pane) | shows hint to focus right pane first |

### Tab 3: Sources

Two sub-pages: **Subscriptions** and **Local Nodes**. `↑`/`↓` on the
left sub-sidebar switches sub-pages.

#### Subscriptions sub-page

List view. Each row: `[✓] <name>  nodes=N  <URL>`.

| Key | Action |
|---|---|
| `a` | open Add Subscription form (Name / URL / User-Agent) |
| `d` | delete highlighted |
| `u` | fetch + parse this subscription now (60s timeout) |
| `e` | toggle enabled state (✓ ↔ ✗) |

Add form: `Tab`/`↑↓` cycle fields, `Enter` confirms (or moves to next
field, then submits on last), `Esc` cancels.

#### Local Nodes sub-page

Group tab bar at top + nodes list. Group bar shows `▶ home  office
(disabled)  [+ new group]`. Tabs are switched with `←/→` (when no form
open).

| Key | Action |
|---|---|
| `a` | open Add Local Node form (proto-driven, defaults to hysteria2) |
| `e` | edit highlighted (form pre-filled with current values) |
| `d` | delete highlighted |
| `u` | open URI paste form (one-shot from clipboard) |
| `N` | new local group form |
| `D` | delete current group (errors if non-empty; hint shown) |
| `E` | rename current group |
| `T` | toggle group enabled |
| `←` / `→` | cycle to previous / next group |

#### Add/Edit Node form

The form fields depend on the chosen proto. Common fields first
(name / group / server / port), proto-specific fields next (e.g. cipher
+ password for ss; uuid + alterId + cipher + network + ws-opts.host/path
+ tls + servername for vmess; etc.), and Via last.

| Key | Action |
|---|---|
| `Tab` / `↑↓` | navigate fields |
| `←` / `→` (on Proto field, focused=0) | cycle ss / vmess / vless / trojan / hysteria2 / tuic. Common fields are preserved across cycles. |
| `Enter` | save (Add mode → `Manager.Add`; Edit mode → Remove+Add with rollback on name collision) |
| `Esc` | cancel |

### Tab 4: Rules

Two sub-pages: **Live** and **Local Rules**.

#### Live sub-page

Live `/rules` + `/providers/rules` view. Read-only.

| Key | Action |
|---|---|
| `/` | enter filter mode (substring match on type+payload+proxy) |
| `Esc` | exit filter |
| `↑` / `↓` / `PgUp` / `PgDown` | navigate |
| `u` | refresh rule providers |
| `T` (Shift+t) | switch to Local Rules sub-page |

`Tab` and `Shift+Tab` are reserved for the global tab cycler (move
between top-level tabs), so the Rules sub-page toggle uses `T` instead.

#### Local Rules sub-page

CRUD over `store.Cfg.LocalRules`. Local rules are emitted **before**
the active source's rules in the assembled config and apply regardless
of which source is active — they're the user-defined override layer.

| Key | Action |
|---|---|
| `d` | delete highlighted rule |
| `K` | move highlighted up |
| `J` | move highlighted down |
| `T` (Shift+t) | switch back to Live |

(Add is currently CLI-only — `vpnkit local-rules add <type> <payload>
<target>`.)

### Tab 5: Connections

Live `/connections` (WebSocket stream). Columns: host, port, network,
upload, download, rule, chain.

| Key | Action |
|---|---|
| `/` | enter filter mode (substring on host or chain) |
| `Esc` | exit filter |
| `↑` / `↓` / `PgUp` / `PgDown` | navigate |
| `x` | close highlighted connection via `DELETE /connections/<id>` |

### Tab 6: Logs

Tail of mihomo log (`~/.local/state/vpnkit/mihomo.log` in PID mode, or
journalctl in systemd-user mode). Ring buffer ≈ 1000 lines.

| Key | Action |
|---|---|
| `p` | pause/resume tailing. `[PAUSED]` appears in the header while paused. Paused lines are dropped at the source (no replay on resume). |

Lines truncate-on-overflow so they never wrap into the tab bar.

### Tab 7: Settings

Sub-sidebar lists 8 sub-pages. ↑/↓ on the sub-sidebar cycles; ← goes
back to MainSidebar; → drills into content (only meaningful on the
sub-pages that own arrows: Routing, Active Source, Rule Template).

| Sub-page | What it shows |
|---|---|
| **Mihomo Core** | binary path, version, mixed-port, controller-port, secret (masked), proxy basic-auth user (masked). `u` upgrades the binary asynchronously. |
| **Service** | systemd-user vs pid mode, running state, log path. `s`/`S`/`r`/`u` start/stop/restart/uninstall the service asynchronously (won't freeze the TUI). |
| **External Controller** | URL + secret (masked). `r` rotates the secret; mihomo restart required for it to take effect. |
| **Routing** | mode selector (rule / global / direct) — `↑↓ Enter` to pick. Apply is async; flash reports outcome. |
| **Active Source** | radio list of every enabled source (subs first, then local-groups). `↑↓ Enter` picks; the Groups tab's `★` marker updates accordingly. Async apply. |
| **Rule Template** | embedded mihomo rule templates (loyalsoldier / minimal) — `↑↓ Enter` to apply. |
| **Cache** | mihomo cache dir + size + last-modified |
| **About** | vpnkit version + commit + license + repo URL |

Most sub-pages are display-only; Routing, Active Source, Rule Template,
and the action keys on Service / External Controller / Mihomo Core are
the mutating ones. All mutations that touch mihomo (Routing, Active
Source, Service ops, Core upgrade) run on a goroutine so the TUI stays
responsive even during 30s+ operations.

---

## File layout

| Path | Owner | Purpose |
|---|---|---|
| `~/.local/bin/vpnkit` | user | this binary |
| `~/.local/bin/mihomo` | user | managed mihomo core (auto-installed) |
| `~/.config/vpnkit/config.toml` | vpnkit | **store** (schema v2): subs, local nodes/groups/rules, ports, creds, mode, active_source, service mode |
| `~/.config/vpnkit/config.toml.lock` | vpnkit | POSIX flock target — serializes concurrent CLI mutations |
| `~/.config/mihomo/config.yaml` | vpnkit | assembled mihomo config (regenerated on every mutation) |
| `~/.config/mihomo/cache.db` | mihomo | mihomo session cache |
| `~/.config/mihomo/ruleset/*.txt` | vpnkit | loyalsoldier ruleset snapshot (unpacked from embedded `*.txt.gz` on first launch) |
| `~/.config/mihomo/*.mmdb`, `*.dat` | bootstrap | GeoIP / GeoSite (pre-seeded once) |
| `~/.config/systemd/user/mihomo.service` | vpnkit | systemd unit (mode 0600, forwards `HTTPS_PROXY`) |
| `~/.netrc` | `vpnkit env` | proxy basic-auth entry (mode 0600) |
| `~/.local/state/vpnkit/` | runtime | mihomo log + PID file (PID mode only) |
| `~/.cache/vpnkit/` | runtime | downloaded archives, rule templates |

Resolution lives in `internal/paths/paths.go` and honors `$XDG_CONFIG_HOME`,
`$XDG_STATE_HOME`, `$XDG_CACHE_HOME`, `$XDG_RUNTIME_DIR` when set —
so isolation in tests / sandboxes just works.

---

## Configuration schema

```toml
schema_version = 2
mode = "rule"               # "rule" | "global" | "direct"
active_source = "doge"      # which source drives rules + 🚀 Proxy (added in rc.7)
global_target = "doge-auto" # 🚀 Proxy's default member; derived from active_source
service_mode = "systemd-user"  # "systemd-user" | "pid"
mixed_port = 7890
controller_port = 9090
controller_secret = "hex-token-32-chars"
proxy_user = "vpnkit"
proxy_pass = "random-hex"

[[subscriptions]]
name = "doge"
url = "https://doge.example.com/sub?token=..."
user_agent = "mihomo/v1.19.25"           # optional; default ships mihomo UA
enabled = true
node_count = 52                          # cached, updated by `subs update`

[[local_node_groups]]
name = "home"
enabled = true

[[local_nodes]]
name = "JP-A"
group = "home"                           # references a local_node_groups entry
via = "doge-auto"                        # optional dialer-proxy target
proto = "hysteria2"
server = "jp.example.com"
port = 443
[local_nodes.fields]
password = "..."
up = "100 Mbps"
down = "1000 Mbps"
sni = "..."

[[local_rules]]
type = "DOMAIN-SUFFIX"
payload = "github.com"
target = "🚀 Proxy"
```

vpnkit lazy-migrates rc.2 stores (no `local_node_groups` block) to rc.3+
on first launch by backfilling a default `local` group for any ungrouped
nodes. No `vpnkit init --force` required.

---

## Active delay test (deep dive)

vpnkit provides two entry points to trigger fresh connectivity probes via
mihomo's `/proxies/<name>/delay` and `/group/<name>/delay`.

| Entry | Trigger | Range |
|---|---|---|
| TUI | Groups tab → focus right pane → `t` | all members of selected group |
| CLI | `vpnkit test <group>` | all members of group |
| CLI | `vpnkit test <group> <node>` | single node |

### Defaults

| Param | Value | Why |
|---|---|---|
| Test URL | `https://www.gstatic.com/generate_204` | mihomo standard. 204 No Content keeps payload near-zero so the timing reflects the proxy hop, not page load |
| Timeout | 5000 ms | mihomo `url-test` default; tested-good nodes return in 100-500 ms, no real wait |
| Concurrency | mihomo-controlled or vpnkit fan-out | depends on which endpoint succeeds (see below) |

### Group endpoint resolution

mihomo's `/group/<name>/delay` endpoint only accepts **url-test / fallback /
load-balance** group types. Selectors (which is what `🚀 Proxy`, `doge`,
`local`, and every user-facing vpnkit group actually is) return `404
Resource not found`.

vpnkit's assembler emits **two** mihomo groups per user-facing group: the
Selector `<name>` and the companion url-test `<name>-auto`. So the
`api.MeasureGroup` helper used by both the TUI and CLI runs this cascade:

1. Try `/group/<name>-auto/delay` — works for every vpnkit-assembled
   subscription / local-nodes group. **One round-trip.**
2. On 404, try `/group/<name>/delay` — covers custom user-defined
   url-test groups (e.g. from a hand-edited `~/.config/mihomo/config.yaml`).
3. On 404 again, read members from `/proxies/<name>.all` and call
   `/proxies/<member>/delay` in parallel goroutines. Slower but works for
   any group regardless of type, including bare Selectors that have no
   companion url-test.

Non-404 errors (`401`, `500`, network timeout) surface as-is rather than
triggering the cascade — they indicate a real problem, not "wrong endpoint."

CLI overrides: `--url https://...` and `--timeout-ms 3000`. TUI uses
defaults (no in-UI override).

### Timeout encoding

mihomo encodes a failed measurement as `{"delay": 0}` — zero ms is not a
valid RTT, so it's an unambiguous sentinel. vpnkit:
- **Text output** translates 0 → `timeout`
- **JSON output** preserves 0 verbatim. Machine consumers decide.

### Color grading (TUI only)

| Range | Color | Meaning |
|---|---|---|
| < 200 ms | green (46) | good |
| 200–500 | yellow (214) | usable |
| > 500 | red (196) | slow |
| `timeout` (0) | red (196) | failed |
| (no measurement) | — | never tested in this session |

### CLI output

Text (default), sorted by node name:
```
$ vpnkit test doge
  HK-01                     234 ms
  JP-02                     567 ms
  US-03                     timeout
```

JSON:
```bash
$ vpnkit test doge --json
{
  "group": "doge",
  "url": "https://www.gstatic.com/generate_204",
  "timeout_ms": 5000,
  "results": {"HK-01": 234, "JP-02": 567, "US-03": 0}
}
```

Single node:
```bash
$ vpnkit test doge HK-01 --json
{"node": "HK-01", "delay_ms": 234,
 "url": "...", "timeout_ms": 5000}
```

### `vpnkit test` vs `vpnkit nodes`

| | `vpnkit nodes` | `vpnkit test` |
|---|---|---|
| Source | mihomo's url-test history (cached) | live `/group/.../delay` call |
| Freshness | as old as the last url-test cycle | now |
| Network conditions | reflects past | reflects current |

Use `nodes` for casual checks, `test` after switching Wi-Fi / VPN /
suspecting a node is down.

### Persistence

TUI delays live in `groupsTab.Model.delayByNode` map — **not persisted**.
TUI restart wipes them. The justification: per-node delay correlates with
current network conditions (wifi vs 4G, time of day, BGP path), so
caching tends to mislead users more than it helps.

For persistent history, use CLI `vpnkit test ... --json` and pipe to your
own logger.

---

## Multi-user same-host installs

vpnkit picks free TCP ports for `mixed-port` and `external-controller`
on first launch via `internal/portutil` (random within 10000–60000,
crypto-rand'd to avoid pattern collisions). Saved into store. Multiple
users on the same machine can each run vpnkit independently — they get
non-overlapping port pairs.

If a saved port becomes busy (another tool grabbed it after a reboot),
vpnkit auto-finds the next free pair at startup and force-rewrites
`config.yaml` to match — silently. Service is then restarted to pick up
the new ports.

systemd-user units are mode 0600 specifically because `Environment=`
lines may contain proxy basic-auth credentials and you don't want
neighbors reading them via `/etc/systemd/user/`.

---

## Troubleshooting

### TUI shows `❌ mihomo not reachable`

1. `vpnkit status` — is the service running? If not: `systemctl --user
   start mihomo` (systemd-user mode) or check `~/.local/state/vpnkit/
   mihomo.log` (pid mode).
2. Controller port drift: `grep controller_port ~/.config/vpnkit/
   config.toml` should match what's in `~/.config/mihomo/config.yaml`'s
   `external-controller:` line. If they diverge, vpnkit's port
   reconciler did the right thing but mihomo was already running with
   the old port. Restart the service.
3. Secret drift: same idea — vpnkit hot-reloads or restarts on secret
   change but if mihomo died mid-flight a manual restart fixes it.

### `vpnkit subs update <name>` hangs / times out

- Test URL access: `curl -I -H "User-Agent: ClashforWindows/0.20.39"
  "<sub-url>"` should return 200 + a base64 body or yaml document.
- Proxy required to reach feed: subscriptions are fetched **directly**,
  not through mihomo. Set `HTTPS_PROXY=http://127.0.0.1:7890` in the
  shell launching vpnkit if the feed itself needs the proxy (chicken-
  and-egg case, but solvable).

### China network — first launch deadlock

mihomo's bootstrap pulls GeoIP MMDB from GitHub; if the user hasn't
plumbed a proxy yet, mihomo deadlocks waiting on it. vpnkit avoids this
two ways:
1. systemd-user unit injects `HTTPS_PROXY` / `HTTP_PROXY` from the
   user's environment so mihomo can use them.
2. `vpnkit init` pre-seeds the GeoIP/GeoSite files into
   `~/.config/mihomo/` from the embedded bootstrap data.

If you're hitting this, run `vpnkit init --force` to re-seed.

### Local node form: `port must be int`

Port field expects a plain integer. Common mistake: pasting `:443`
including the leading colon, or `443/udp` with a protocol suffix.
Strip to `443`.

### Local node form: name collision on edit

Editing a node and changing its name to one that already exists in any
local group triggers a rollback (the original node is re-inserted) and
shows `save: localnodes: duplicate name X` flash. Either pick a new name
or first delete the conflicting node.

### Delay test → all timeout

- mihomo not running (`vpnkit status`)
- Controller port closed (`ss -tln | grep <controller_port>`)
- The probe URL is itself unreachable for the proxy egress — try
  `vpnkit test <group> --url https://1.1.1.1` for a sanity check that
  doesn't require GFW-bypass.
- Auth misconfig: `curl -i http://127.0.0.1:<port>/proxies` should
  return 200 with the secret in `Authorization: Bearer <token>` header.

### `delay <group>: mihomo GET /group/<group>/delay: 404 Resource not found`

This used to bite anyone trying to test a vpnkit-managed group before
rc.6 because vpnkit's user-facing groups are mihomo Selectors, and
`/group/<name>/delay` only works on url-test / fallback / load-balance
types. Fixed by the
[Group endpoint resolution](#group-endpoint-resolution) cascade — now
upgrade to rc.6+ or rebuild from `main`.

### "TUI tabs don't show subscriptions / local nodes after `vpnkit subs add` from another shell"

The TUI doesn't auto-watch store mutations from external processes.
Re-open the TUI, or use the TUI's own Sources tab forms for CRUD so
it can refresh in-place.

---

## Design notes & tradeoffs

- **Why mihomo and not native Go proxy code**: mihomo is battle-tested,
  supports ss/vmess/vless/trojan/hysteria2/tuic out of the box, and
  exposes a stable controller API. vpnkit wraps it instead of
  reimplementing 6 protocols.
- **Why TOML for store**: comments survive round-trips, hand-edit
  friendly, no whitespace pitfalls. YAML is reserved for mihomo's own
  config (its native format).
- **Why no `vpnkit chain` / `vpnkit group` / `vpnkit ext` anymore**:
  rc.4 absorbed dialer-proxy into the local node's `Via` field and
  the custom-group builder UI was dropped. Power users can still hand-
  edit `~/.config/mihomo/config.yaml` after assemble — vpnkit only
  rewrites the security-owned keys (`mixed-port`, `external-controller`,
  `secret`, `authentication`, `bind-address`, `allow-lan`).
- **Why two routing-mode concepts**: vpnkit's `mode` (in store) and
  mihomo's `mode:` (in config.yaml) — vpnkit always emits `mode: rule`
  to mihomo and emulates `global` / `direct` via the assembled `rules:`
  list. That way Mihomo's mode flag is never the source of truth and
  can't drift.
- **Why store is single-file**: easier backup, lazy migration, atomic
  rewrite via tmp+rename. Locking is sub-process-coarse (the binary
  doesn't run concurrently against itself).
