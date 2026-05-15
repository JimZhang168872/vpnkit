# vpnkit

A lightweight, cross-platform VPN client for Mihomo (Clash.Meta) with TUI management interface.

## Features

- **Lightweight**: Go-based, minimal dependencies
- **Cross-platform**: Linux, macOS, Windows support
- **TUI Dashboard**: Real-time traffic stats, service control via Bubble Tea
- **Seamless Integration**: Automatic shell config (`~/.bashrc`, `~/.zshrc`, etc.)
- **Persistent Config**: TOML-based configuration with atomic writes
- **Auto-update**: Detects and downloads latest Mihomo from GitHub releases

## Quick Start

### Prerequisites

- Go 1.22+ (for building from source)
- systemd (on Linux, for automatic service management)
- macOS 12+ or Linux 4.15+

### Build

```bash
go build -o bin/vpnkit ./cmd/vpnkit
```

### Usage

```bash
# Start the VPN service
./bin/vpnkit start

# Open the dashboard
./bin/vpnkit dashboard

# View current config
./bin/vpnkit config show

# Update Mihomo binary
./bin/vpnkit update-binary

# Source shell config (bash/zsh/fish)
eval "$(./bin/vpnkit env)"
```

## Configuration

Default config location: `~/.config/vpnkit/config.toml`

```toml
[service]
listen_port = 7890
socks5_port = 7891
redir_port = 7892
log_level = "info"

[shell]
auto_inject = true
```

## Development

### Project Structure

```
vpnkit/
  cmd/
    vpnkit/           # Main entry point
  internal/
    api/              # Mihomo REST API client
    app/              # TUI application logic
    config/           # Config management
    env/              # Shell env injection
    installer/        # Binary downloader & installer
    log/              # Structured logging
    paths/            # XDG directory resolution
    rules/            # Firewall rule templates
    service/          # Service manager (systemd/launchd/SC)
    store/            # TOML persistence
    tabs/             # TUI tab modules
  Makefile
  go.mod
  go.sum
```

### Running Tests

```bash
make test
```

### Code Quality

```bash
make lint
make fmt
```

## License

Apache 2.0
