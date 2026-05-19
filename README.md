<h1 align="center">vpnkit</h1>

<p align="center">
  <strong>Terminal-native manager for the <a href="https://github.com/MetaCubeX/mihomo">mihomo</a> proxy core</strong> — subscriptions, local nodes, proxy chains, rule-based routing. TUI + CLI, no Electron, no daemon. Inspired by <a href="https://github.com/clash-verge-rev/clash-verge-rev">Clash Verge</a>. Single Go binary, fully non-root.
</p>

<p align="center">
  <a href="https://github.com/JimZhang168872/vpnkit/releases"><img alt="Tag" src="https://img.shields.io/github/v/tag/JimZhang168872/vpnkit"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/JimZhang168872/vpnkit/actions"><img alt="CI" src="https://github.com/JimZhang168872/vpnkit/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://go.dev/"><img alt="Go" src="https://img.shields.io/badge/go-1.23%2B-00ADD8.svg"></a>
</p>

<p align="center">中文 → <a href="README_zh.md">README_zh.md</a></p>

---

vpnkit runs mihomo (the maintained Clash.Meta core) entirely in user space — no
root, no TUN. v1.0.0 adds **multi-source subscriptions, hand-entered local
nodes, structured local rules, and a single-active-source routing model**,
all editable from a 7-tab TUI or a matching
`vpnkit subs / local-nodes / local-rules / active` CLI surface.

The default loyalsoldier rule-set snapshot ships **embedded in the binary**
(~2 MB gzipped). Bootstrap unpacks it before mihomo's first launch, so
RULE-SET rules work on slow or GFW-restricted networks without waiting on
jsdelivr.

> 📖 **Full reference**: [docs/USAGE.md](docs/USAGE.md) (English) /
> [docs/USAGE_zh.md](docs/USAGE_zh.md) (中文) — every CLI command, every TUI
> tab, every keyboard shortcut, every JSON output schema, config file
> layout, delay-test deep-dive, troubleshooting. **Start there for anything
> beyond the README.**

> **v0.10.x → v1.0.0 is a breaking change.** Store schema bumped v1 → v2.
> See [`docs/UPGRADE-v1.md`](docs/UPGRADE-v1.md) for the migration path.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

Auto-detects amd64/arm64, verifies SHA256, installs to `~/.local/bin/vpnkit`,
generates a default config skeleton, and (on subsequent runs) cleans up the
old version before installing the new one. Pin a version with
`VERSION=v1.0.0-rc.1 ./install.sh`. Make sure `~/.local/bin` is on your `PATH`.

From source: `git clone … && cd vpnkit && make install` (needs Go 1.23+).

> **Network:** vpnkit reaches `github.com` directly — no mirror fallback.
> Behind a restrictive network? Configure `HTTPS_PROXY` in your shell
> *before* installing. The systemd-user unit and bootstrap also pick up your
> shell-level proxy and pre-download the GeoIP/GeoSite data files so
> mihomo's first launch doesn't deadlock waiting on GitHub.
>
> **🇨🇳 Behind the Great Firewall?** See
> [**`docs/INSTALL-CN.md`**](docs/INSTALL-CN.md) — covers three install
> paths (via existing proxy / via GitHub mirror / fully offline) plus
> common errors (`SSL_ERROR_SYSCALL`, `connection refused`, mihomo
> bootstrap failures).

## First run

```bash
vpnkit
```

First launch downloads mihomo, writes `~/.config/mihomo/config.yaml`, installs
`~/.config/systemd/user/mihomo.service`, pre-seeds GeoIP/GeoSite data, starts
the service, and opens the TUI.

### Add a subscription

```bash
vpnkit subs add doge       https://example.invalid/sub/doge --ua=clash.meta
vpnkit subs add boost-net  https://example.invalid/sub/boost
vpnkit subs update
```

Or in the TUI: `3` (Sources) → `a` → form → `Enter`. `u` refreshes a
subscription; `e` toggles enabled; `d` removes one.

### Add a local node (now in multiple groups)

```bash
vpnkit local-groups add home
vpnkit local-groups add office
vpnkit local-nodes add 'hysteria2://password@1.2.3.4:443?up=100&down=200#HK-manual' --group=home
vpnkit local-nodes add 'ss://YWVz...@1.2.3.4:8388#JP-rented' --group=office --via=doge-auto
```

`--group` picks which local-nodes-group the node belongs to (auto-created if
absent); `--via` chains the node's egress through any subscription/local
node or group (mihomo `dialer-proxy` field).

In the TUI: `3` (Sources) → `↓` Local Nodes → `N` create a group → switch
between groups with `←/→` → `a` opens the form (Proto-driven fields including
hy2/tuic up/down limits and a Via select).

### Add a local rule (always wins over subscription rules)

```bash
vpnkit local-rules add DOMAIN-SUFFIX baidu.com '🎯 Direct'
vpnkit local-rules add DOMAIN-KEYWORD internal '🎯 Direct'
vpnkit local-rules list
```

Local rules render before any subscription's own rules so user intent always
takes precedence.

### Routing knobs

```bash
vpnkit mode rule              # default — follow rules
vpnkit mode global            # all traffic → 🚀 Proxy (whatever active source picks)
vpnkit mode direct            # all traffic bypasses proxy

# Active source — single source of routing truth. ★ in Groups tab marks it.
vpnkit active                 # show current active source (sub or local group)
vpnkit active boost-net       # switch to a subscription
vpnkit active home            # switch to a local-nodes group

vpnkit target doge-auto       # override 🚀 Proxy's default member (advanced)
```

The **active source** is one subscription OR one local-nodes-group. Its
`rules:` section drives routing; if it has none (e.g. local-nodes groups
never ship rules), the loyalsoldier template fills in. `🚀 Proxy`'s members
are the active source's nodes + `DIRECT`. Switching active swaps both the
rule baseline and the catch-all proxy in one step.

### Use the proxy from your shell

```bash
eval "$(vpnkit env --shell zsh)"
curl https://www.google.com
eval "$(vpnkit env --unset)"
```

`vpnkit env` sets both lower- and upper-case variants (`http_proxy`,
`HTTP_PROXY`, …). It also writes a `~/.netrc` entry at mode 0600 for tools
that prefer netrc (`--no-netrc` to skip).

For a permanent setup, drop named functions into your rc file once:

```bash
vpnkit env --shell zsh --functions >> ~/.zshrc
exec zsh
proxy_on    # 🟢 proxy on
proxy_off   # 🔴 proxy off
```

## Update

vpnkit checks for new releases 2 s after launch and shows a dim `⚡` badge
in the status bar when one is available.

```bash
vpnkit update                            # check + plan + interactive confirm
vpnkit update --yes                      # skip the prompt
vpnkit update --check                    # only print the plan, do nothing
vpnkit update --vpnkit-only              # leave mihomo alone
vpnkit update --mihomo-only              # leave vpnkit alone
```

## Multi-source architecture

Every subscription becomes its own mihomo `<name>` (select) + `<name>-auto`
(url-test) group. Every named local-nodes-group does the same — exactly
symmetric with subscriptions. Hand-entered nodes carry a `Via` field that
writes through to mihomo's `dialer-proxy`, so you can build per-node
chains directly in the form (Shadowrocket-style).

**One active source drives routing** (`vpnkit active <name>`). Only that
source's rules are emitted; if it doesn't ship rules, the loyalsoldier
template fills in. `🚀 Proxy`'s members are limited to the active source's
nodes + `DIRECT`, so MATCH traffic deterministically routes through it.
Other sources stay loaded in proxy-groups so switching active is a single
command — no re-assemble required from the user's side.

```
proxies: each node renamed "<group>:<original-name>" so cross-group
         duplicates do not collide
proxy-groups:
  - {name: doge,        type: select,   proxies: [doge-auto, doge:HK-A, ...]}
  - {name: doge-auto,   type: url-test, proxies: [doge:HK-A, ...], interval: 300}
  - {name: home,        type: select,   proxies: [home-auto, home:HK-manual, ...]}
  - {name: home-auto,   type: url-test, ...}
  - {name: 🚀 Proxy,    type: select,   proxies: [<active>-auto, <active>, DIRECT]}
rules:
  - <local-rules first — always win, regardless of active source>
  - <active source's own rules, OR loyalsoldier template if it has none>
  - MATCH,🚀 Proxy
```

See [`docs/superpowers/specs/2026-05-17-v1-subscription-groups-design.md`](docs/superpowers/specs/2026-05-17-v1-subscription-groups-design.md)
for the original spec; the active-source model layered on top is described
in [USAGE.md](docs/USAGE.md#core-concepts).

## Multi-user / multi-instance safety

Default ports are randomized into the IANA dynamic range (30000–60000) on
first install via `crypto/rand` rejection sampling, so two users on the same
host almost never collide. `portutil.FindFree` scans the next 100 slots as a
safety net.

Mihomo is configured with `allow-lan: false` + `bind-address: 127.0.0.1` +
`authentication: [user:pass]`. The user/pass is generated on first launch
and stored at mode 0600 in `~/.config/vpnkit/config.toml`. The systemd-user
unit is also mode 0600 to keep proxy credentials in `Environment=` lines off
world-readable disk.

## Chained egress for local nodes

Set `Via` on a local node (Sources › Local Nodes → `a` or `e`, last field of
the form) to make mihomo dial through another proxy/group — equivalent to
mihomo's `dialer-proxy` field. Stored on the node itself, so it survives
subscription updates and travels with the node when you move it between
local groups.

```
Via: doge-auto              # any subscription/local node name OR group name
```

## CLI

| Command | What it does |
|---|---|
| `vpnkit` | open the TUI |
| `vpnkit version` / `--version` / `-v` | binary + commit + mihomo path |
| `vpnkit --help` / `-h` / `help` | top-level usage (also `<verb> --help` per subverb) |
| `vpnkit status` | mihomo state, ports, subscriptions count, local nodes count, mode, active source |
| `vpnkit ip` | exit IP via the proxy |
| `vpnkit mode [rule\|global\|direct]` | show or change routing mode |
| `vpnkit active [<name>]` | show or switch the active source (subscription OR local-nodes-group) |
| `vpnkit target [<member>]` | override 🚀 Proxy's default member (advanced — usually `active` suffices) |
| `vpnkit subs list/add/rm/enable/disable/update [<name>]` | manage subscriptions |
| `vpnkit local-groups list/add/rm/enable/disable/rename` | manage local-nodes groups |
| `vpnkit local-nodes list/add/rm/edit/mv` (with `--group/--via`) | manage hand-entered nodes |
| `vpnkit local-rules list/add/rm/move` | manage local routing rules (type + payload + target all validated) |
| `vpnkit groups` | live proxy-group list (from mihomo controller) |
| `vpnkit nodes '<group>'` | members + cached delay (read-only, from mihomo url-test cache) |
| `vpnkit test '<group>' ['<node>']` | active delay test (see [USAGE.md › Active delay test](docs/USAGE.md#active-delay-test-deep-dive)) |
| `vpnkit use '<group>' '<node>'` | switch a group's selection |
| `vpnkit env [--shell bash\|zsh\|fish] [--unset] [--functions] [--no-netrc]` | shell snippet (validates shell flavor) |
| `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]` | upgrade vpnkit + mihomo |
| `vpnkit init [--force]` | regenerate config skeleton (`--force` backs up existing) |
| `vpnkit uninstall [--yes] [--purge] [--keep-mihomo]` | stop services, remove all vpnkit-owned paths |

Read commands (`status`, `ip`, `groups`, `nodes`, `test`, `mode`, `target`,
`active`, `... ls`) accept `--json` for scripting; mutation verbs reject
`--json` with a clear error. JSON-mode runtime failures emit
`{"error":"…"}` to stdout so consumer scripts still parse cleanly.

Exit codes: `0` ok, `1` user error, `2` runtime error. Concurrent CLI
mutations are serialized via a POSIX flock on the config file, so
`vpnkit subs add foo url &` in parallel is safe. Per-command flags and
JSON schemas are in [docs/USAGE.md › CLI reference](docs/USAGE.md#cli-reference).

## TUI layout (v1)

```
[1] 🏠 Dashboard      live mihomo / traffic
[2] 🌐 Groups         all groups + nodes (★ marks active source · delay test)
[3] 📚 Sources        Subscriptions / Local Nodes sub-pages (CRUD)
[4] 📜 Rules          Live (mihomo view) / Local Rules (CRUD) sub-pages
[5] 🔗 Connections    live connections (`x` close, `/` filter)
[6] 📓 Logs           mihomo log tail (`p` pause/resume)
[7] ⚙  Settings       Mihomo Core / Service / External Controller / Routing /
                      Active Source / Rule Template / Cache / About sub-pages
```

Keys:
- `↑↓` navigate · `←` back / sidebar focus · `→` content focus / drill-in · `Enter` activate · `q` quit · `Ctrl+C` quit (even inside forms)
- `1`–`7` jump to tab · `Tab`/`Shift+Tab` cycle · `?` keymap hint flash
- **Groups**: `r` refresh · `t` test delay (active probe, current group) · `Enter` switch to highlighted node · `←/→` left/right pane focus · `★` after the group name marks the active source
- **Sources › Subscriptions**: `a` add · `d` delete · `u` update now · `e` toggle enabled (lists scroll past the visible window when long)
- **Sources › Local Nodes**: `a` add (proto-driven form) · `e` edit · `d` delete · `u` paste URI ·
  `N`/`D`/`E`/`T` new/delete/rename/toggle group · `←/→` switch group (when no form open) · credential fields (password / uuid) display as bullets
- **Add/Edit Node form**: `Tab/↑↓` navigate fields · `Enter` save (validates proto-specific required fields) · `Esc` cancel ·
  `←/→` on Proto field cycles ss / vmess / vless / trojan / hysteria2 / tuic
- **Rules › Live**: `/` filter · `u` refresh providers · `T` (Shift+t) switch to Local Rules
- **Rules › Local Rules**: `d` delete · `K/J` move up/down · `T` back to Live
- **Logs**: `p` pause/resume tailing
- **Settings › Routing**: `↑↓ Enter` pick mode · async apply (mihomo reload runs off the event loop)
- **Settings › Active Source**: `↑↓ Enter` pick which source drives 🚀 Proxy + rules

Min usable terminal: **60×16**. Below that you get a "terminal too narrow"
gate instead of a corrupted layout.

Per-tab key tables + behavior contracts live in
[docs/USAGE.md › TUI reference](docs/USAGE.md#tui-reference).

## Layout

| Path | Purpose |
|---|---|
| `~/.local/bin/vpnkit` | this binary |
| `~/.local/bin/mihomo` | managed mihomo core |
| `~/.config/vpnkit/config.toml` | subscriptions, local nodes, local rules, ports, creds (schema v2) |
| `~/.config/mihomo/config.yaml` | assembled mihomo config (regenerated on each mutation) |
| `~/.config/mihomo/*.mmdb / *.dat` | GeoIP / GeoSite data files (pre-seeded by bootstrap) |
| `~/.config/systemd/user/mihomo.service` | systemd-user unit (mode 0600; forwards your HTTPS_PROXY) |
| `~/.netrc` | proxy basic-auth entry (written by `vpnkit env`, mode 0600) |
| `~/.local/state/vpnkit/` | logs, PID file (PID mode) |
| `~/.cache/vpnkit/` | mihomo archives |

## Out of scope

TUN mode, Windows / macOS, command palette, theme switcher, GUI.

## Build & test

```bash
make build      # ./bin/vpnkit
make test       # go test -race -cover ./...
make lint       # golangci-lint run
```

## License

[MIT](LICENSE). Builds on [mihomo](https://github.com/MetaCubeX/mihomo),
[Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules), and
the [charmbracelet](https://github.com/charmbracelet) TUI stack.
