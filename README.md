<h1 align="center">vpnkit</h1>

<p align="center">
  Terminal UI for the <a href="https://github.com/MetaCubeX/mihomo">mihomo</a> proxy core. Inspired by <a href="https://github.com/clash-verge-rev/clash-verge-rev">Clash Verge</a>. Single Go binary, fully non-root.
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
nodes, and structured local rules**, all editable from a 7-tab TUI or a
matching `vpnkit subs / local-nodes / local-rules / target` CLI surface.

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
vpnkit mode global            # all traffic → global target
vpnkit mode direct            # all traffic bypasses proxy
vpnkit target doge-auto       # set global target (group or node name)
```

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
(url-test) group. The top-level `🚀 Proxy` group lists all subscription
groups + the synthetic `local` group + `DIRECT`; routing's MATCH falls back
to whichever target the user picked. See
[`docs/superpowers/specs/2026-05-17-v1-subscription-groups-design.md`](docs/superpowers/specs/2026-05-17-v1-subscription-groups-design.md)
for the assembler algorithm in detail.

v1.0.0-rc.3 generalizes the previous single `local` group into named
user-managed groups (e.g. `home`, `office`). Each enabled local group emits
its own `<group>` (select) + `<group>-auto` (url-test) pair — exactly
symmetric with subscriptions. Hand-entered nodes carry a `Via` field that
writes through to mihomo's `dialer-proxy`, so you can build per-node
chains directly in the form (Shadowrocket-style) without touching the
extensions overlay.

```
proxies: each node renamed "<group>:<original-name>" so cross-group
         duplicates do not collide
proxy-groups:
  - {name: doge,        type: select,   proxies: [doge-auto, doge:HK-A, ...]}
  - {name: doge-auto,   type: url-test, proxies: [doge:HK-A, ...], interval: 300}
  - {name: boost,       type: select,   ...}
  - {name: local,       type: select,   proxies: [local:HK-manual, DIRECT]}
  - {name: 🚀 Proxy,    type: select,   proxies: [<global-target>, ...rest]}
rules:
  - <local rules first — always win>
  - <each enabled subscription's own rules, with targets rewritten>
  - MATCH,🚀 Proxy
```

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

## Extensions: chains & custom groups

Chain one subscription node through another (multi-hop egress, `dialer-proxy`)
and add your own proxy-groups. Edits persist in
`~/.config/vpnkit/extensions.toml` and survive subscription updates.

```bash
vpnkit chain set "US-1" "JP-Relay"        # US-1 egress now hops through JP-Relay
vpnkit chain unset "US-1"
vpnkit group add "Stream" --type select --proxies "US-1,JP-1,DIRECT"
vpnkit ext apply                          # reassemble + reload mihomo
```

In the TUI: Settings → Extensions. `c` toggles to Chains, `g` to Groups,
`a/e/d` add/edit/delete the highlighted row, `r` reassembles + reloads.

## CLI

| Command | What it does |
|---|---|
| `vpnkit` | open the TUI |
| `vpnkit status` | mihomo state, ports, subscriptions count, local nodes count, mode, global target |
| `vpnkit ip` | exit IP via the proxy |
| `vpnkit mode [rule\|global\|direct]` | show or change routing mode |
| `vpnkit target [<group-or-node>]` | show or set GlobalTarget |
| `vpnkit subs list/add/rm/enable/disable/update [<name>]` | manage subscriptions |
| `vpnkit local-groups list/add/rm/enable/disable/rename` | manage local-nodes groups |
| `vpnkit local-nodes list/add/rm/edit/mv` (with `--group/--via`) | manage hand-entered nodes |
| `vpnkit local-rules list/add/rm/move` | manage local routing rules |
| `vpnkit groups` | live proxy-group list (from mihomo controller) |
| `vpnkit nodes '<group>'` | members + cached delay |
| `vpnkit use '<group>' '<node>'` | switch a group's selection |
| `vpnkit env [--shell zsh] [--unset] [--functions] [--no-netrc]` | shell snippet |
| `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]` | upgrade vpnkit + mihomo |
| `vpnkit init [--force]` | regenerate config skeleton (`--force` backs up existing) |
| `vpnkit uninstall [--yes] [--purge] [--keep-mihomo]` | stop services, remove all vpnkit-owned paths |
| `vpnkit chain ls/set/unset` | manage dialer-proxy chains |
| `vpnkit group ls/add/rm` | manage custom proxy-groups |
| `vpnkit ext apply` | reassemble + reload mihomo with current extensions |

All read commands accept `--json` for scripting. Exit codes: `0` ok,
`1` user error, `2` runtime error.

## TUI layout (v1)

```
[1] 🏠 Dashboard      live mihomo / traffic
[2] 🌐 Groups         all groups + nodes (read-only + delay test)
[3] 📚 Sources        Subscriptions / Local Nodes sub-pages (CRUD)
[4] 📜 Rules          Live (mihomo view) / Local Rules (CRUD) sub-pages
[5] 🔗 Connections    live connections (`x` close, `/` filter)
[6] 📓 Logs           mihomo log tail
[7] ⚙  Settings       Mihomo Core / Service / External Controller / Routing /
                      Rule Template / Extensions / Cache / About sub-pages
```

Keys:
- `↑↓` navigate · `←` back / sidebar focus · `→` content focus / drill-in · `Enter` activate · `q` quit
- `1`–`7` jump to tab · `Tab`/`Shift+Tab` cycle
- **Sources › Subscriptions**: `a` add · `d` delete · `u` update now · `e` toggle enabled
- **Sources › Local Nodes**: `a` add URI · `d` delete
- **Rules › Live**: `/` filter · `u` refresh providers · `Tab` switch to Local Rules
- **Rules › Local Rules**: `d` delete · `K/J` move up/down · `Tab` back to Live
- **Settings › Routing**: `↑↓ Enter` pick mode · global target editable via `vpnkit target`
- **Settings → Extensions**: `c` chains / `g` groups · `a/e/d` add/edit/delete · `r` apply

## Layout

| Path | Purpose |
|---|---|
| `~/.local/bin/vpnkit` | this binary |
| `~/.local/bin/mihomo` | managed mihomo core |
| `~/.config/vpnkit/config.toml` | subscriptions, local nodes, local rules, ports, creds (schema v2) |
| `~/.config/vpnkit/extensions.toml` | chains + custom proxy-groups overlay |
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
