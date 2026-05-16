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

Auto-detects amd64/arm64, verifies SHA256, installs to `~/.local/bin/vpnkit`.
Pin a version with `VERSION=v0.8.0 ./install.sh`. Make sure `~/.local/bin` is
on your `PATH`.

From source: `git clone … && cd vpnkit && make install` (needs Go 1.22+).

### Installing from inside the GFW

If `github.com` and `api.github.com` are unreachable from your host, route
everything (the install script's downloads **and** mihomo's later geo-data
downloads) through a public GitHub-accelerator mirror. Pick one that currently
works for you — public mirrors come and go:

```bash
# Example mirror (replace with whatever is currently working for you):
MIRROR="https://ghproxy.com/"
VERSION=v0.8.0  # pin: some mirrors don't proxy api.github.com

curl -sSL "${MIRROR}https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh" \
  | INSTALL_MIRROR="$MIRROR" VERSION="$VERSION" bash
```

`INSTALL_MIRROR` is also persisted to `~/.config/vpnkit/config.toml` as
`release_mirror`, so when mihomo later downloads its `geoip.metadb` /
`geosite.dat` it goes through the same mirror — no further configuration
needed.

Other public mirrors people have used: `https://mirror.ghproxy.com/`,
`https://ghp.ci/`, `https://gh.api.99988866.xyz/`. Pick one with a check:

```bash
curl -fsSL --max-time 5 -o /dev/null \
  "${MIRROR}https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/README.md" \
  && echo OK || echo "mirror dead, try another"
```

## First run (3 minutes)

```bash
vpnkit
```

On first launch vpnkit downloads mihomo, generates `~/.config/mihomo/config.yaml`,
installs `~/.config/systemd/user/mihomo.service`, starts it, and opens the TUI.

Add a subscription:

1. Press `3` (Profiles) → `a` to open the form
2. Name + paste subscription URL → `Enter`
3. Press `u` to fetch + parse + write config + reload mihomo

Pick a node:

1. Press `2` (Proxies) → highlight `🚀 Proxy` → `t` to delay-test
2. Highlight the fastest → `Enter` to switch

Use the proxy from your shell:

```bash
eval "$(vpnkit env --shell zsh)"   # or bash / fish
curl https://www.google.com         # now routed through mihomo
eval "$(vpnkit env --unset)"        # turn off
```

`vpnkit env` also writes a `~/.netrc` entry (mode 0600) so tools that prefer
netrc (curl, git) pick up the proxy credentials there. `--no-netrc` to skip.

## Multi-user / multi-instance safety

vpnkit picks a free port automatically. If `7890`/`9090` are already taken
(another user, another proxy), it scans upward and saves the chosen ports to
`~/.config/vpnkit/config.toml`.

Mihomo is configured with `allow-lan: false` + `bind-address: 127.0.0.1` +
`authentication: [user:pass]`. The user/pass is auto-generated on first launch
and stored in `~/.config/vpnkit/config.toml` (mode 0600). Without those
credentials, other local users cannot use your proxy.

## CLI

```bash
vpnkit status                       # mihomo state, mode, ports, groups, profile
vpnkit ip                           # exit IP via mihomo proxy
vpnkit mode [rule|global|direct]    # show or set mode
vpnkit groups                       # list user-selectable proxy groups
vpnkit nodes '🚀 Proxy'              # list members + cached delay
vpnkit use '🚀 Proxy' 'HK-01'        # switch to a specific node
vpnkit env [--shell zsh] [--unset] [--no-netrc]
```

All accept `--json` for scripting. Exit codes: `0` ok, `1` user error, `2`
runtime error.

## Behind the GFW

mihomo's first start downloads geo data from GitHub. If that's blocked, either
set a release mirror in `~/.config/vpnkit/config.toml`:

```toml
release_mirror = "https://ghproxy.com/"
```

…or point mihomo through an existing HTTP proxy via a systemd drop-in at
`~/.config/systemd/user/mihomo.service.d/proxy.conf`:

```ini
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7897"
Environment="HTTPS_PROXY=http://127.0.0.1:7897"
```

Then `systemctl --user daemon-reload && systemctl --user restart mihomo`.

## TUI cheatsheet

- `1`–`6` jump to tab · `Tab`/`Shift+Tab` cycle · `q` quit (mihomo keeps running)
- `↑` `↓` `j` `k` navigate within a tab · `Enter` activate/expand
- Profiles: `a` add · `u` update · `d` delete
- Proxies: `t` delay-test
- Connections: `x` close · `/` filter
- Settings: `↑`/`↓` cycle subpages (Mihomo Core, Service, External Controller, Default Rules, Patch Editor, Logs, Cache, About)

## Layout

| Path | Purpose |
|---|---|
| `~/.local/bin/vpnkit` | this binary |
| `~/.local/bin/mihomo` | managed mihomo core |
| `~/.config/vpnkit/config.toml` | profiles, ports, proxy creds, controller secret |
| `~/.config/mihomo/config.yaml` | generated mihomo config (regenerated on subscription update) |
| `~/.config/mihomo/patch.yaml` | user overlay, deep-merged into generated config |
| `~/.config/systemd/user/mihomo.service` | systemd unit |
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
