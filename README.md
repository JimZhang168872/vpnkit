<h1 align="center">vpnkit</h1>

<p align="center">
  Terminal UI for the <a href="https://github.com/MetaCubeX/mihomo">mihomo</a> proxy core. Inspired by <a href="https://github.com/clash-verge-rev/clash-verge-rev">Clash Verge</a>. Single Go binary, fully non-root.
</p>

<p align="center">
  <a href="https://github.com/JimZhang168872/vpnkit/releases"><img alt="Tag" src="https://img.shields.io/github/v/tag/JimZhang168872/vpnkit"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/JimZhang168872/vpnkit/actions"><img alt="CI" src="https://github.com/JimZhang168872/vpnkit/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://go.dev/"><img alt="Go" src="https://img.shields.io/badge/go-1.22%2B-00ADD8.svg"></a>
</p>

<p align="center">中文 → <a href="README_zh.md">README_zh.md</a></p>

---

vpnkit runs mihomo (the maintained Clash.Meta core) entirely in user space — no
root, no TUN. The TUI gives you proxy switching, delay testing, connection
inspection, and rule management from a terminal that fits an SSH session.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

Auto-detects amd64/arm64, verifies SHA256, installs to `~/.local/bin/vpnkit`,
generates a default config skeleton, and (on subsequent runs) cleans up the
old version before installing the new one. Pin a version with
`VERSION=v0.9.0 ./install.sh`. Make sure `~/.local/bin` is on your `PATH`.

From source: `git clone … && cd vpnkit && make install` (needs Go 1.22+).

> **Network:** vpnkit reaches `github.com` directly — no mirror fallback.
> Behind a restrictive network? Configure `HTTPS_PROXY` in your shell
> *before* installing (and before `vpnkit update` runs). SmartClient will
> probe the env proxy at request time and route through it when reachable.

## First run

```bash
vpnkit
```

First launch downloads mihomo, writes `~/.config/mihomo/config.yaml`, installs
`~/.config/systemd/user/mihomo.service`, starts it, and opens the TUI.

Add a subscription:

1. `3` (Profiles) → `a` opens the form
2. Name + paste subscription URL → `Enter`
3. `u` → fetch + parse + write config + reload mihomo

Pick a node:

1. `2` (Proxies) → highlight `🚀 Proxy` → `t` delay-tests the whole group
2. `Enter` to expand → `↓` to a specific node → `Enter` switches to it

Subscription URLs accepted: Clash YAML links, base64-encoded text lists, or a
single protocol URI (`vmess://`, `hysteria2://`, `trojan://`, `vless://`,
`ss://`, `tuic://`).

### Use the proxy from your shell

```bash
eval "$(vpnkit env --shell zsh)"        # or bash / fish
curl https://www.google.com              # routed through mihomo
eval "$(vpnkit env --unset)"             # turn off
```

`vpnkit env` sets both lower- and upper-case variants (`http_proxy`,
`HTTP_PROXY`, …) so Go programs and uppercase-only readers also see it. It
also writes a `~/.netrc` entry at mode 0600 for tools that prefer netrc
(`--no-netrc` to skip).

For a permanent setup, drop named functions into your rc file once:

```bash
vpnkit env --shell zsh --functions >> ~/.zshrc
exec zsh
# any shell after that:
proxy_on    # 🟢 proxy on
proxy_off   # 🔴 proxy off
```

## Update

vpnkit checks for new releases 2 s after launch and shows a dim `⚡` badge
in the status bar when one is available. To install:

```bash
vpnkit update                            # check + plan + interactive confirm
vpnkit update --yes                      # skip the prompt
vpnkit update --check                    # only print the plan, do nothing
vpnkit update --vpnkit-only              # leave mihomo alone
vpnkit update --mihomo-only              # leave vpnkit alone
```

`vpnkit update` upgrades vpnkit and mihomo via direct downloads from
GitHub Releases (through `HTTPS_PROXY` if you have one set), swaps the
binaries atomically (POSIX rename over a running executable is safe),
and `syscall.Exec`'s the new vpnkit so the TUI relaunches with the new
version. Mihomo restarts during the swap, so the proxy is down for ~1 s.

## Extensions: chains & custom groups

Chain one subscription node through another (multi-hop egress, `dialer-proxy`)
and add your own proxy-groups. Edits persist in
`~/.config/vpnkit/extensions.toml` and survive subscription updates.

### CLI

```bash
vpnkit chain ls
vpnkit chain set "US-1" "JP-Relay"        # US-1 egress now hops through JP-Relay
vpnkit chain unset "US-1"

vpnkit group ls
vpnkit group add "Stream" --type select --proxies "US-1,JP-1,DIRECT"
vpnkit group add "Auto-US" --type url-test \
    --proxies "US-1,US-2" \
    --url https://www.gstatic.com/generate_204 \
    --interval 300 --tolerance 50
vpnkit group rm "Stream"

vpnkit ext apply                          # reassemble active profile + reload mihomo
```

### TUI

Settings → Extensions. `c` toggles to Chains, `g` to Groups, `a/e/d`
add/edit/delete the highlighted row, `r` reassembles + reloads. Form has
inline autocomplete hints from the live `/proxies` snapshot.

### Migration from `patch.yaml`

vpnkit no longer reads `~/.config/mihomo/patch.yaml`. The Settings → Patch
Editor sub-page has been replaced by Settings → Extensions. For chain /
proxy-group tweaks, the new structured format is friendlier than free-form
YAML; for arbitrary mihomo config overrides not covered by chains/groups,
edit `~/.config/mihomo/config.yaml` after each subscription update
(or keep an existing `patch.yaml` and merge it manually).

## Multi-user / multi-instance safety

vpnkit picks a free port automatically. If `7890`/`9090` are already taken
(another user, another tool), it scans upward and persists the chosen ports
to `~/.config/vpnkit/config.toml`.

Mihomo is configured with `allow-lan: false` + `bind-address: 127.0.0.1` +
`authentication: [user:pass]`. The user/pass is generated on first launch
and stored at mode 0600 in `~/.config/vpnkit/config.toml`. Without those
credentials, **other local users cannot use your proxy** even though it
listens on the shared loopback.

## CLI

| Command | What it does |
|---|---|
| `vpnkit` | open the TUI |
| `vpnkit status` | mihomo state, mode, ports, groups, active profile |
| `vpnkit ip` | exit IP via the proxy (uses ipinfo.io) |
| `vpnkit mode [rule\|global\|direct]` | show or change rule mode |
| `vpnkit groups` | list user-selectable proxy groups |
| `vpnkit nodes '<group>'` | list members + cached delay |
| `vpnkit use '<group>' '<node>'` | switch a group's selection |
| `vpnkit env [--shell zsh] [--unset] [--functions] [--no-netrc]` | shell snippet |
| `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]` | upgrade vpnkit + mihomo |
| `vpnkit init [--restore <path>]` | regenerate config skeleton |
| `vpnkit uninstall [--yes] [--purge] [--keep-mihomo]` | stop services, remove all vpnkit-owned paths |
| `vpnkit chain ls/set/unset` | manage dialer-proxy chains |
| `vpnkit group ls/add/rm` | manage custom proxy-groups |
| `vpnkit ext apply` | reassemble + reload mihomo with current extensions |

All read commands accept `--json` for scripting. Exit codes: `0` ok,
`1` user error, `2` runtime error.

## TUI cheatsheet

- `1`–`6` jump to tab · `Tab`/`Shift+Tab` cycle · `q` quit (mihomo keeps running)
- `↑` `↓` `j` `k` navigate · `Enter` activate/expand
- **Profiles**: `a` add · `u` update · `d` delete · `Enter` set active
- **Proxies**: `Enter` on group expand/collapse · `Enter` on node switch · `t` delay-test
- **Connections**: `x` close selected · `/` filter
- **Rules**: `/` filter · `u` refresh providers
- **Settings**: `↑`/`↓` cycle subpages (Mihomo Core, Service, External Controller, Default Rules, Extensions, Logs, Cache, About)
- **Settings → Extensions**: `c` chains / `g` groups · `a` add · `e` edit · `d` delete · `r` apply (reassemble + reload)

## Layout

| Path | Purpose |
|---|---|
| `~/.local/bin/vpnkit` | this binary |
| `~/.local/bin/mihomo` | managed mihomo core |
| `~/.config/vpnkit/config.toml` | profiles, ports, proxy creds, controller secret |
| `~/.config/vpnkit/extensions.toml` | chains + custom proxy-groups overlay |
| `~/.config/mihomo/config.yaml` | generated mihomo config (regenerated on each subscription update) |
| `~/.config/systemd/user/mihomo.service` | systemd-user unit |
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
