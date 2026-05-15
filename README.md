# vpnkit

TUI for managing the [mihomo](https://github.com/MetaCubeX/mihomo) proxy core on Linux, non-root.

Inspired by [Clash Verge](https://github.com/clash-verge-rev/clash-verge-rev). Single Go binary, lives in `~/.local/bin`, manages mihomo as a `systemd --user` service (or PID-managed fallback).

## Status

Under development. See `docs/superpowers/specs/2026-05-15-vpnkit-tui-design.md` for design, `docs/superpowers/plans/` for implementation plans.

## Build

```bash
go build -o ~/.local/bin/vpnkit ./cmd/vpnkit
```

## Usage

```bash
vpnkit              # launch TUI
eval "$(vpnkit env --shell zsh)"   # export HTTP_PROXY etc. for current shell
```

## Quickstart (Phase 1)

```bash
make install        # builds and installs to ~/.local/bin/vpnkit
vpnkit              # launches TUI; first run silently downloads mihomo and starts the service
                    # press 1-6 to switch tabs, q to quit (mihomo keeps running)
eval "$(vpnkit env --shell zsh)"   # export proxy env vars for current shell
curl https://www.google.com         # traffic now goes through mihomo
```

Stop the managed mihomo:

- systemd mode: `systemctl --user stop mihomo`
- PID mode:     `kill $(cat ~/.local/state/vpnkit/mihomo.pid)`

Phase 1 ships the installer, service manager, env helper, and a working Dashboard tab streaming live traffic from mihomo's external-controller API.

## Phase 2 features

Profiles tab:
- `a` add subscription (popup form: name + URL)
- `u` update selected subscription (fetches, parses, writes `config.yaml`, reloads mihomo)
- `d` delete; `Enter` activate; `↑↓` / `j k` navigate

Proxies tab:
- Live group/node view from mihomo's `/proxies` (polled every 5 s)
- `Enter` to expand a group; `t` to run a delay test against the highlighted group
- `↑↓` / `j k` navigate

Supported subscription formats: Clash YAML, SIP008 JSON, Base64-encoded URI list,
and single-URI variants of `vmess://`, `ss://`, `ssr://`, `trojan://`, `vless://`,
`hysteria://`, `hysteria2://`, `tuic://`.

Default rule template: Loyalsoldier (changeable via `~/.config/vpnkit/config.toml`).
User overlay: `~/.config/mihomo/patch.yaml` is deep-merged on every update.

Connections / Rules / Logs / Settings (full editor + theme + cache management)
land in Phase 3 and Phase 4.
