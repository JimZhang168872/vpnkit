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
