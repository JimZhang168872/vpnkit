<h1 align="center">vpnkit</h1>

<p align="center">
  Terminal UI for the <a href="https://github.com/MetaCubeX/mihomo">mihomo</a> proxy core. Inspired by <a href="https://github.com/clash-verge-rev/clash-verge-rev">Clash Verge</a>. Single Go binary, fully non-root.
  <br>
  给 <a href="https://github.com/MetaCubeX/mihomo">mihomo</a> 内核做的终端管理 UI。模仿 <a href="https://github.com/clash-verge-rev/clash-verge-rev">Clash Verge</a>，单一 Go 二进制，完全非 root。
</p>

<p align="center">
  <a href="https://github.com/JimZhang168872/vpnkit/releases"><img alt="Tag" src="https://img.shields.io/github/v/tag/JimZhang168872/vpnkit"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/JimZhang168872/vpnkit/actions"><img alt="CI" src="https://github.com/JimZhang168872/vpnkit/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://go.dev/"><img alt="Go" src="https://img.shields.io/badge/go-1.22%2B-00ADD8.svg"></a>
</p>

<p align="center">
  <a href="#english">English</a> · <a href="#%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87">简体中文</a>
</p>

---

<a id="english"></a>

## English

vpnkit is a terminal-based management UI for [mihomo](https://github.com/MetaCubeX/mihomo)
(the actively-maintained Clash.Meta core). It runs entirely in user space — no
root, no TUN — and gives you the same proxy switching, delay testing,
connection inspection, and rule management you'd get from a desktop GUI like
Clash Verge, but from a terminal that fits an SSH session.

### Features

- **Six-tab TUI** (bubbletea): Dashboard · Proxies · Profiles · Connections · Rules · Settings
- **Settings sub-menu**: Mihomo Core · Service · External Controller · Default Rules · Patch Editor · Logs · Cache · About
- **Multi-format subscriptions**: Clash YAML, SIP008 JSON, Base64-encoded URI list, single-URI variants of `vmess` `ss` `ssr` `trojan` `vless` `hysteria(2)` `tuic`
- **Default rule template** (Loyalsoldier rule-providers, ~13 categories)
- **Patch overlay** — `~/.config/mihomo/patch.yaml` is deep-merged into every regenerated config
- **Service management** — `systemd --user` first, falls back to internal PID-file mode
- **Real-time data** — traffic (SSE), connections (WebSocket), proxies + rules (polled)
- **Delay testing** for proxy groups
- **In-TUI mihomo upgrade**, cache management, controller secret rotation
- **Profile persistence** to `~/.config/vpnkit/config.toml`
- **Release mirror support** that covers both the mihomo binary download and its runtime geox-url downloads (helpful in restricted networks)
- **`vpnkit env`** prints shell snippets that export `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY`

### Not in scope

TUN mode, Windows / macOS, command palette, theme switcher, GUI.

### Install

#### From source (only path today)

Requires Go 1.22+ (the toolchain directive auto-fetches 1.23 if needed).

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
cd vpnkit
make install   # builds and installs to ~/.local/bin/vpnkit
```

Add `~/.local/bin` to your `PATH` if it isn't already.

Pre-built releases are planned but not yet published.

### First launch

```bash
vpnkit
```

vpnkit on first launch:

1. downloads the latest mihomo release to `~/.local/bin/mihomo`,
2. generates `~/.config/mihomo/config.yaml` from a sane default skeleton,
3. installs `~/.config/systemd/user/mihomo.service` and starts mihomo,
4. opens the TUI.

After that:

```bash
eval "$(vpnkit env --shell zsh)"   # export proxy env vars for current shell
curl https://www.google.com         # traffic now goes through mihomo
```

Stop the managed mihomo: `systemctl --user stop mihomo` (or `kill $(cat ~/.local/state/vpnkit/mihomo.pid)` in PID mode).

> 📖 **Step-by-step walkthrough**: see [`docs/USAGE.md`](docs/USAGE.md) for a
> zero-to-first-proxy guide, every TUI page explained, and a primer on
> `systemctl --user` / `loginctl enable-linger` / `HTTP_PROXY` / `PATH`.

### First launch behind the GFW

mihomo's first start needs to download `geoip.metadb` and other geo data from
GitHub. If that fails, mihomo crashes and systemd marks the service failed
after 3 retries (vpnkit surfaces the failure in the TUI flash bar).

Two ways to fix it:

**(a) Existing proxy** — if you already have any reachable HTTP proxy (e.g. a
running Clash Verge on `127.0.0.1:7897`), add a systemd override:

```ini
# ~/.config/systemd/user/mihomo.service.d/proxy.conf
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7897"
Environment="HTTPS_PROXY=http://127.0.0.1:7897"
```

Then `systemctl --user daemon-reload && systemctl --user restart mihomo`.

**(b) Release mirror** — set in `~/.config/vpnkit/config.toml`:

```toml
release_mirror = "https://ghproxy.com/"
```

This rewrites both the mihomo-binary download URL and the `geox-url` block in
the generated mihomo config to go through the mirror.

### Layout

| Path | Purpose |
|---|---|
| `~/.local/bin/vpnkit` | this binary |
| `~/.local/bin/mihomo` | managed mihomo core |
| `~/.config/vpnkit/config.toml` | profiles, controller secret, mirror, theme |
| `~/.config/mihomo/config.yaml` | generated mihomo config (regenerated on each subscription update) |
| `~/.config/mihomo/patch.yaml` | user-editable overlay (preserved across updates) |
| `~/.config/systemd/user/mihomo.service` | systemd unit |
| `~/.local/state/vpnkit/` | logs, PID file (PID mode) |
| `~/.cache/vpnkit/` | downloaded mihomo archives |

### Key bindings

| Key | Action |
|---|---|
| `1`–`6` | jump to tab |
| `Tab` / `Shift+Tab` | cycle tabs |
| `q` / `Ctrl+C` | quit (mihomo keeps running) |
| `↑` `↓` `j` `k` | navigate within a tab |
| `Enter` | activate / expand |

Per-tab actions are shown in the footer of each tab. Examples:

- Profiles — `a` add, `u` update, `d` delete
- Proxies — `t` delay-test highlighted group
- Connections — `x` close highlighted connection, `/` filter
- Settings → Patch Editor — `Ctrl+S` save
- Settings → Logs — `p` pause / resume
- Settings → Mihomo Core — `u` upgrade

### Build & test

```bash
make build      # ./bin/vpnkit
make test       # go test -race -cover ./...
make lint       # golangci-lint run
```

### Documentation

Design doc and per-phase implementation plans live under
[`docs/superpowers/`](docs/superpowers/):

- `specs/2026-05-15-vpnkit-tui-design.md` — design doc
- `plans/2026-05-15-vpnkit-phase1.md` — installer + service mgmt + TUI shell
- `plans/2026-05-15-vpnkit-phase2.md` — subscription pipeline + Profiles/Proxies tabs
- `plans/2026-05-16-vpnkit-phase3.md` — Connections, Rules, Logs
- `plans/2026-05-16-vpnkit-phase4.md` — Settings sub-menu polish

### License

[MIT](LICENSE).

### Acknowledgments

- [mihomo](https://github.com/MetaCubeX/mihomo) — the proxy core this TUI manages
- [Clash Verge](https://github.com/clash-verge-rev/clash-verge-rev) — desktop GUI this TUI is patterned after
- [Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules) — default rule providers
- [bubbletea](https://github.com/charmbracelet/bubbletea), [lipgloss](https://github.com/charmbracelet/lipgloss), [bubbles](https://github.com/charmbracelet/bubbles) — TUI stack

---

<a id="简体中文"></a>

## 简体中文

vpnkit 是给 [mihomo](https://github.com/MetaCubeX/mihomo)（仍在维护的 Clash.Meta 内核）写的终端管理 UI。完全 user-space 跑 — 不需要 root、不依赖 TUN — 提供和 Clash Verge 桌面端相当的节点切换、延迟测试、连接观察、规则管理能力，但塞得进 SSH 会话。

### 特性

- **6 个 tab**（bubbletea）：Dashboard · Proxies · Profiles · Connections · Rules · Settings
- **Settings 子菜单**：Mihomo Core · Service · External Controller · Default Rules · Patch Editor · Logs · Cache · About
- **多格式订阅**：Clash YAML、SIP008 JSON、Base64 URI 列表，以及 `vmess` `ss` `ssr` `trojan` `vless` `hysteria(2)` `tuic` 单 URI
- **默认规则模板**（Loyalsoldier rule-providers，13 类）
- **本地 patch 覆盖** — 每次重生成 config 时把 `~/.config/mihomo/patch.yaml` deep-merge 进去
- **服务管理** — 优先 `systemd --user`，不可用时回退到内置 PID 文件模式
- **实时数据** — 流量 (SSE)、连接 (WebSocket)、proxies + rules（轮询）
- **代理组延迟测试**
- **TUI 内升级 mihomo**、清缓存、轮换 controller secret
- **profile 持久化**到 `~/.config/vpnkit/config.toml`
- **Release 镜像** 同时影响 mihomo 二进制下载和它运行时的 geox-url 下载（被墙环境救命）
- **`vpnkit env`** 输出 shell 片段，导出 `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY`

### 不做

TUN 模式、Windows/macOS、命令面板、主题切换、GUI。

### 安装

#### 源码编译（目前唯一方式）

需要 Go 1.22+（toolchain 会按需自动下载 1.23）。

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
cd vpnkit
make install   # 编译并装到 ~/.local/bin/vpnkit
```

把 `~/.local/bin` 加入 `PATH`（如果还没加过）。

预编译 release 在计划中但还没发布。

### 首次启动

```bash
vpnkit
```

首次启动会：

1. 下载最新 mihomo 到 `~/.local/bin/mihomo`，
2. 生成 `~/.config/mihomo/config.yaml`（默认骨架），
3. 安装 `~/.config/systemd/user/mihomo.service` 并启动,
4. 打开 TUI。

之后：

```bash
eval "$(vpnkit env --shell zsh)"   # 导出代理环境变量
curl https://www.google.com         # 流量已走 mihomo
```

停掉服务：`systemctl --user stop mihomo`（PID 模式下用 `kill $(cat ~/.local/state/vpnkit/mihomo.pid)`）。

> 📖 **完整教程**：[`docs/USAGE.md`](docs/USAGE.md) 有从零到第一个代理的全流程、
> 每个 TUI 页面详解，以及 `systemctl --user` / `loginctl enable-linger` /
> `HTTP_PROXY` / `PATH` 的入门讲解。

### 被墙环境第一次启动

mihomo 第一次启动需要从 GitHub 下 `geoip.metadb` 等 geo 数据。下载失败 → mihomo fatal → systemd 重试 3 次后放弃（vpnkit 会把失败信息显示在 TUI 状态栏）。

两种解决方案：

**(a) 已有代理** — 如果你已经有任何能用的 HTTP 代理（比如已经运行的 Clash Verge 在 `127.0.0.1:7897`），加 systemd override：

```ini
# ~/.config/systemd/user/mihomo.service.d/proxy.conf
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7897"
Environment="HTTPS_PROXY=http://127.0.0.1:7897"
```

再 `systemctl --user daemon-reload && systemctl --user restart mihomo`。

**(b) Release 镜像** — 在 `~/.config/vpnkit/config.toml` 里设：

```toml
release_mirror = "https://ghproxy.com/"
```

这会把 mihomo 二进制下载链接 + 生成的 mihomo config 里的 `geox-url` 都走镜像。

### 目录布局

| 路径 | 用途 |
|---|---|
| `~/.local/bin/vpnkit` | 本程序 |
| `~/.local/bin/mihomo` | 受管的 mihomo 核心 |
| `~/.config/vpnkit/config.toml` | 订阅列表、controller secret、镜像、主题 |
| `~/.config/mihomo/config.yaml` | 生成的 mihomo 配置（每次订阅更新重写） |
| `~/.config/mihomo/patch.yaml` | 用户编辑的覆盖层（更新后保留） |
| `~/.config/systemd/user/mihomo.service` | systemd 单元 |
| `~/.local/state/vpnkit/` | 日志、PID 文件（PID 模式下） |
| `~/.cache/vpnkit/` | 下载的 mihomo 压缩包 |

### 快捷键

| 键 | 动作 |
|---|---|
| `1`–`6` | 跳转到对应 tab |
| `Tab` / `Shift+Tab` | 循环切 tab |
| `q` / `Ctrl+C` | 退出（mihomo 继续跑） |
| `↑` `↓` `j` `k` | tab 内导航 |
| `Enter` | 激活 / 展开 |

每个 tab 的具体动作显示在该 tab 底部，例如：

- Profiles — `a` 添加、`u` 更新、`d` 删除
- Proxies — `t` 对当前组跑延迟测试
- Connections — `x` 关闭选中连接、`/` 过滤
- Settings → Patch Editor — `Ctrl+S` 保存
- Settings → Logs — `p` 暂停/恢复
- Settings → Mihomo Core — `u` 升级

### 编译 & 测试

```bash
make build      # ./bin/vpnkit
make test       # go test -race -cover ./...
make lint       # golangci-lint run
```

### 设计文档

设计文档和每个 phase 的实现计划在 [`docs/superpowers/`](docs/superpowers/) 下：

- `specs/2026-05-15-vpnkit-tui-design.md` — 设计文档
- `plans/2026-05-15-vpnkit-phase1.md` — 安装器 + 服务管理 + TUI 骨架
- `plans/2026-05-15-vpnkit-phase2.md` — 订阅 pipeline + Profiles/Proxies tab
- `plans/2026-05-16-vpnkit-phase3.md` — Connections / Rules / Logs
- `plans/2026-05-16-vpnkit-phase4.md` — Settings 子菜单 polish

### License

[MIT](LICENSE)。

### 致谢

- [mihomo](https://github.com/MetaCubeX/mihomo) — 这个 TUI 管理的核心
- [Clash Verge](https://github.com/clash-verge-rev/clash-verge-rev) — UI 模式参考
- [Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules) — 默认规则源
- [bubbletea](https://github.com/charmbracelet/bubbletea)、[lipgloss](https://github.com/charmbracelet/lipgloss)、[bubbles](https://github.com/charmbracelet/bubbles) — TUI 技术栈
