# vpnkit Usage Guide / 使用指南

<p align="center">
  <a href="#english">English</a> · <a href="#%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87">简体中文</a>
</p>

---

<a id="english"></a>

## English

This guide walks you from a fresh Linux user account to a working
proxy, plus an inventory of every TUI screen and the system-level
plumbing (`systemctl --user`, `loginctl enable-linger`, env vars).

### Table of contents

1. [Zero to your first working proxy](#1-zero-to-your-first-working-proxy)
2. [TUI page-by-page reference](#2-tui-page-by-page-reference)
3. [System plumbing — systemctl, linger, env vars, PATH](#3-system-plumbing)

---

### 1. Zero to your first working proxy

#### 1.1 Prerequisites

You need:

- Linux (anything modern; tested on Ubuntu 22.04+)
- `git`, `make`, `go` ≥ 1.22 (Go's toolchain directive will fetch 1.23 transparently if needed)
- `systemd --user` (almost always present on a normal desktop / server install — WSL without `--systemd` is the main exception)

Install if missing (Ubuntu/Debian):

```bash
sudo apt update
sudo apt install -y git make
# Go: download tarball to ~/.local/go (no sudo needed)
curl -sSL https://go.dev/dl/go1.22.10.linux-amd64.tar.gz | tar -C ~/.local -xz
echo 'export PATH="$HOME/.local/go/bin:$PATH"' >> ~/.bashrc   # or ~/.zshrc
```

#### 1.2 Build and install vpnkit

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
cd vpnkit
make install      # → ~/.local/bin/vpnkit
```

Make sure `~/.local/bin` is on `PATH` (more on this in §3.4):

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc   # or ~/.bashrc
exec $SHELL                                                # reload
which vpnkit                                               # /home/you/.local/bin/vpnkit
```

#### 1.3 If you're behind the GFW: pre-configure a mirror

vpnkit's first launch downloads mihomo and then mihomo itself downloads `geoip.metadb` etc. from `github.com`. If GitHub is blocked from your machine, both downloads will fail.

Pick one of these BEFORE first launch:

**Option A — already have an HTTP proxy** (e.g. Clash Verge already running on `127.0.0.1:7897`):

```bash
mkdir -p ~/.config/systemd/user/mihomo.service.d/
cat > ~/.config/systemd/user/mihomo.service.d/proxy.conf <<'EOF'
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7897"
Environment="HTTPS_PROXY=http://127.0.0.1:7897"
Environment="NO_PROXY=127.0.0.1,localhost"
EOF
```

**Option B — use a release mirror**:

```bash
mkdir -p ~/.config/vpnkit
cat > ~/.config/vpnkit/config.toml <<'EOF'
release_mirror = "https://ghproxy.com/"
EOF
```

(vpnkit will rewrite both the mihomo-binary URL and the runtime `geox-url` block to go through this mirror.)

#### 1.4 First launch

```bash
vpnkit
```

What you should see in the next 5–30 seconds:

1. Status bar: `bootstrapping: downloading…` then `mihomo ready`.
2. Sidebar with 6 tabs, Dashboard active, Mihomo header, version, mode, ↑0 B/s ↓0 B/s.
3. Press `q` to quit. mihomo keeps running in the background.

If the status bar shows `bootstrap: …` red error, see §3.5 troubleshooting.

#### 1.5 Add your first subscription

Get a subscription URL from your VPN provider. It looks like one of:

- A Clash YAML URL (e.g. `https://example.com/sub?token=...`)
- A Base64-encoded text URL (older Shadowsocks-style)
- A single `vmess://` / `ss://` / `trojan://` / `vless://` etc. link

In vpnkit:

1. Press `3` — switch to **Profiles** tab. It shows: `No subscriptions yet — press 'a' to add`.
2. Press `a` — popup form opens.
3. Type a name (e.g. `airport-A`), press `Tab`.
4. Paste the URL, press `Enter`.
5. Status bar: `added airport-A`. Profile shows up in the list, marked `★` (active).
6. Press `u` — fetch + parse + write config.yaml + reload mihomo.
7. Status bar: `airport-A: 23 nodes` (or whatever).

#### 1.6 Pick a node and test it

1. Press `2` — switch to **Proxies** tab. You'll see proxy groups (e.g. `🚀 Proxy`, `♻️ Auto`, `🎯 Direct`).
2. Use `↑`/`↓` (or `j`/`k`) to highlight `🚀 Proxy`. Press `Enter` to expand — you'll see all nodes under it.
3. Press `t` — runs a delay test against every node in the group. Numbers (e.g. `45 ms`, `120 ms`) appear next to each node within ~5 s.
4. Highlight the fastest node. Press `Enter` — switches to it. The current selection marker `✓` moves.
5. Press `q` to quit the TUI.

#### 1.7 Use the proxy from your terminal

vpnkit doesn't auto-set proxy env vars — its mihomo just listens on `127.0.0.1:7890` (HTTP+SOCKS mixed). You opt in per shell:

```bash
eval "$(vpnkit env --shell zsh)"   # or --shell bash / --shell fish
echo $http_proxy                   # http://127.0.0.1:7890
curl -I https://www.google.com     # 200 OK
```

To unset:

```bash
eval "$(vpnkit env --shell zsh --unset)"
```

To make this stick across new shells, add it to `~/.zshrc` (NOT recommended if you toggle proxy on/off often — set up a shell function instead):

```bash
proxy-on()  { eval "$(vpnkit env --shell zsh)"; }
proxy-off() { eval "$(vpnkit env --shell zsh --unset)"; }
```

For browsers / GUI apps, configure the system proxy separately (GNOME Settings → Network → Network Proxy → Manual → HTTP/HTTPS proxy = `127.0.0.1:7890`).

---

### 2. TUI page-by-page reference

The TUI has 6 tabs. Switch with number keys `1`–`6`, `Tab`/`Shift+Tab` cycle, or click in supported terminals. Keys shown here are valid only when that tab is active (unless marked GLOBAL).

#### 2.1 Dashboard (tab `1`)

```
Mihomo

  Status : ● running
  Version: v1.19.16
  Mode   : rule

  ↑ 1.2 KiB/s
  ↓ 4.5 MiB/s
```

**Shows**: mihomo running state, version, rule mode (rule/global/direct), real-time up/down rates streamed via `/traffic` SSE.

**Keys** (all GLOBAL — same on every tab):

| Key | Action |
|---|---|
| `q` / `Ctrl+C` | quit (mihomo keeps running) |
| `1`–`6` | jump to tab |
| `Tab` / `Shift+Tab` | cycle tabs |

#### 2.2 Proxies (tab `2`)

```
Proxies

▶ 🚀 Proxy             Selector      → HK-01
    ✓ HK-01                            45 ms
      JP-02                            87 ms
      US-03                            210 ms
  ♻️ Auto               URLTest       → HK-01
  🛑 Reject             Selector      → REJECT
```

**Shows**: every proxy group (Selector / URLTest / Fallback) and the currently-selected member. Updated every 5 s by polling `/proxies`.

**Keys**:

| Key | Action |
|---|---|
| `↑` `↓` `j` `k` | navigate groups |
| `Enter` | expand group (show all nodes) / switch active node |
| `t` | delay-test highlighted group (each node tested, results show in ms) |

#### 2.3 Profiles (tab `3`)

```
Profiles

★ airport-A   https://example.com/sub        nodes=23
  airport-B   https://other.example.com/sub  nodes=11

[a] add  [u] update  [Enter] activate  [d] delete  [↑↓] navigate
```

**Shows**: subscription list, last-known node count, `★` marks the active subscription whose nodes feed the current `~/.config/mihomo/config.yaml`.

**Keys**:

| Key | Action |
|---|---|
| `a` | popup form: name + URL |
| `u` | re-fetch the highlighted subscription + regenerate config + reload mihomo |
| `Enter` | mark highlighted as active |
| `d` | delete highlighted |
| `↑` `↓` `j` `k` | navigate |

When the form is open: `Tab` switches between Name / URL fields, `Enter` saves, `Esc` cancels.

#### 2.4 Connections (tab `4`)

```
Connections
  ↑ 12 MiB    ↓ 340 MiB    47 active

  HOST                            PORT    UP            DOWN          RULE
▶ www.google.com                  443     2.1 KiB       45 KiB        Match
  api.github.com                  443     800 B         12 KiB        DOMAIN-SUFFIX
  ...

[/] filter  [x] close selected  [↑↓] navigate
```

**Shows**: every active TCP/UDP connection mihomo knows about, streamed live via `/connections` WebSocket.

**Keys**:

| Key | Action |
|---|---|
| `↑` `↓` `j` `k` | navigate |
| `x` | close highlighted connection (mihomo terminates it) |
| `/` | filter (substring match against host or rule) — Phase 4 limitation: filter input is currently set programmatically only |

#### 2.5 Rules (tab `5`)

```
Rules

Rule Providers
  reject               Domain    count=12345  updated=2026-05-15T08:00:00Z
  proxy                Domain    count=8765   updated=2026-05-15T08:00:00Z
  cncidr               IPCIDR    count=21000  updated=2026-05-15T08:00:00Z
  ...

Rules
  RULE-SET       reject                          → 🛑 Reject
  RULE-SET       direct                          → 🎯 Direct
  GEOIP          CN                              → 🎯 Direct
  RULE-SET       proxy                           → 🚀 Proxy
  MATCH                                          → 🚀 Proxy

[/] filter  [u] refresh providers
```

**Shows**: full active rule list and rule-providers state, polled every 30 s.

**Keys**: `[/]` and `[u]` are listed in the footer but UI for filter input + provider refresh dispatch are Phase 4 placeholders.

#### 2.6 Settings (tab `6`)

The Settings tab itself is a sub-menu:

```
Settings        │  ▶ Mihomo Core
                │  ──────────────
▶ Mihomo Core   │     Binary : ~/.local/bin/mihomo
  Service       │     Size   : 33886356 bytes
  External Cont │     Mirror : (direct GitHub)
  Default Rules │
  Patch Editor  │     [u] upgrade to latest release
  Logs          │
  Cache         │
  About         │

[↑↓] navigate
```

**Sub-menu navigation** (only when Settings tab is active): `↑` / `↓` to switch sub-pages.

Each sub-page has its own keys, listed below.

##### 2.6.1 Mihomo Core
- Shows installed mihomo path/size + currently configured release mirror.
- `u` — runs the installer for the latest mihomo release (replaces `~/.local/bin/mihomo`).

##### 2.6.2 Service
- Shows service mode (`systemd-user` or `pid`), running state, PID.
- `s` start · `S` stop · `r` restart · `u` uninstall (removes systemd unit).

##### 2.6.3 External Controller
- Shows controller port (default 9090) and a masked secret.
- `r` — regenerate secret (writes to `~/.config/vpnkit/config.toml`; mihomo restart needed).

##### 2.6.4 Default Rules
- Pick the default rule template (`loyalsoldier` or `minimal`).
- `j` / `k` cycle, `Enter` save (writes to `config.toml`; takes effect at next subscription update).

##### 2.6.5 Patch Editor
- Full textarea over `~/.config/mihomo/patch.yaml`.
- This file is deep-merged onto the generated mihomo config every time a subscription updates, so put your manual overrides here (port changes, custom DNS, extra rules, etc.).
- `Ctrl+S` save.

##### 2.6.6 Logs
- Live tail of mihomo's log (via `journalctl --user -u mihomo` or PID-mode log file).
- `p` pause / resume.

##### 2.6.7 Cache
- Shows `~/.cache/vpnkit/` size (downloaded mihomo archives).
- `c` clear.

##### 2.6.8 About
- vpnkit version, Go version, license info, source links.

---

### 3. System plumbing

#### 3.1 systemctl --user basics

`systemd --user` is "systemd, but for one user, no root needed". It runs services as you, with files in `~/.config/systemd/user/`. vpnkit installs `mihomo.service` there.

Common commands:

```bash
systemctl --user status mihomo       # is it running? what was the last log line?
systemctl --user start mihomo        # start once
systemctl --user stop mihomo         # stop
systemctl --user restart mihomo
systemctl --user enable mihomo       # auto-start on login (vpnkit installs with --now so this is set already)
systemctl --user disable mihomo      # don't auto-start anymore
systemctl --user daemon-reload       # re-read unit file after editing it
journalctl --user -u mihomo -f       # follow mihomo's log live
journalctl --user -u mihomo -n 50    # last 50 lines
```

The unit file is at `~/.config/systemd/user/mihomo.service`. To extend it without editing the file vpnkit owns, drop overrides in `~/.config/systemd/user/mihomo.service.d/*.conf` (see §1.3 example).

#### 3.2 loginctl enable-linger

By default, `systemd --user` services exit when you log out. That means:

- SSH-based: mihomo dies when you disconnect the SSH session.
- Desktop: mihomo dies when you log out of GNOME/KDE.

To allow your user services to survive past logout, run **once** (one-time `sudo`):

```bash
sudo loginctl enable-linger $USER
```

Verify:

```bash
loginctl show-user $USER -p Linger
# Linger=yes
```

After this, mihomo keeps running across SSH disconnects and desktop logouts. To revert: `sudo loginctl disable-linger $USER`.

#### 3.3 Environment variables (HTTP_PROXY etc.)

Most CLI tools (curl, wget, git, pip, npm, apt, …) read these env vars:

| Variable | Used for |
|---|---|
| `http_proxy` | HTTP requests |
| `https_proxy` | HTTPS requests |
| `all_proxy` | Anything else (SOCKS-aware tools use this) |
| `no_proxy` | Comma-separated list of hosts to bypass |

`vpnkit env` outputs `export` statements for these, pointing at mihomo's mixed-port (default `127.0.0.1:7890`). Use it like:

```bash
eval "$(vpnkit env --shell zsh)"     # set
eval "$(vpnkit env --unset)"          # unset
```

Tools that **don't** respect these env vars (need separate config):

- Web browsers (use system / browser proxy settings instead).
- Some Java apps (use `-Dhttp.proxyHost=...`).
- Docker (separate `~/.docker/config.json` proxy block).
- Snap-installed apps (sandboxed; `snap set system proxy.http=...`).

#### 3.4 PATH — getting `vpnkit` and `mihomo` on your shell PATH

vpnkit installs binaries to `~/.local/bin/`. Many distros put that on PATH by default; some don't.

Check:

```bash
echo $PATH | tr ':' '\n' | grep -F "$HOME/.local/bin"
```

If empty, add to your shell rc:

```bash
# bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc

# zsh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc

# fish
fish_add_path -U $HOME/.local/bin
```

Then `exec $SHELL` (or open a new terminal).

#### 3.5 Quick troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| TUI status bar shows red `bootstrap: ...` | Read the message. Common: GitHub blocked → set release mirror or HTTP_PROXY override (§1.3). |
| `systemctl --user status mihomo` shows `failed` | Check `journalctl --user -u mihomo -n 50`. If it's "can't download MMDB", same fix as above. |
| TUI Dashboard shows "Lost connection to core" | mihomo not running OR controller secret mismatch. Check `cat ~/.config/vpnkit/config.toml` controller_port + secret, then `curl -H "Authorization: Bearer $SECRET" http://127.0.0.1:9090/version`. |
| `vpnkit: command not found` | `~/.local/bin` not on PATH (see §3.4). |
| Nodes disappear / subscription shows 0 nodes after `u` | Check `~/.local/state/vpnkit/vpnkit.log` for parser errors. Provider may use a format vpnkit doesn't yet support — file an issue. |
| Mihomo restarted itself and lost state | systemd auto-restarts on crash. Check if `Restart=on-failure` was triggered: `systemctl --user status mihomo` shows `Restart counter`. |
| After `sudo apt install systemd-resolved` you can't reach 127.0.0.1:7890 | DNS conflict; mihomo uses port 53 in some configs. Check `mixed-port` in your config.yaml didn't accidentally collide. |

---

<a id="简体中文"></a>

## 简体中文

这份指南带你从零（一台干净的 Linux 用户账户）到一个能跑的代理，加上每个 TUI 页面和系统层（`systemctl --user`、`loginctl enable-linger`、环境变量）的详细说明。

### 目录

1. [从零到第一个能用的代理](#1-从零到第一个能用的代理)
2. [TUI 各页面详解](#2-tui-各页面详解)
3. [系统层 — systemctl、linger、环境变量、PATH](#3-系统层)

---

### 1. 从零到第一个能用的代理

#### 1.1 前置依赖

需要：

- Linux（任何主流发行版；在 Ubuntu 22.04+ 上测过）
- `git`、`make`、`go ≥ 1.22`（Go 的 toolchain 指令会按需自动下 1.23）
- `systemd --user`（普通桌面 / 服务器都有；WSL 不开 `--systemd` 是主要例外）

缺的话装一下 (Ubuntu/Debian)：

```bash
sudo apt update
sudo apt install -y git make
# Go: 下载 tarball 到 ~/.local/go (不需要 sudo)
curl -sSL https://go.dev/dl/go1.22.10.linux-amd64.tar.gz | tar -C ~/.local -xz
echo 'export PATH="$HOME/.local/go/bin:$PATH"' >> ~/.bashrc   # 或 ~/.zshrc
```

#### 1.2 编译并安装 vpnkit

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
cd vpnkit
make install      # → ~/.local/bin/vpnkit
```

确认 `~/.local/bin` 在 `PATH` 上（详见 §3.4）：

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc   # 或 ~/.bashrc
exec $SHELL                                                # 重载 shell
which vpnkit                                               # /home/you/.local/bin/vpnkit
```

#### 1.3 被墙环境：先配镜像再启动

vpnkit 第一次启动会下 mihomo，然后 mihomo 自己会从 `github.com` 下 `geoip.metadb` 等 geo 数据。GitHub 被墙的话两步都会失败。

**首启动之前**择一执行：

**方案 A — 已经有可用 HTTP 代理**（比如 Clash Verge 已经在 `127.0.0.1:7897` 上跑）：

```bash
mkdir -p ~/.config/systemd/user/mihomo.service.d/
cat > ~/.config/systemd/user/mihomo.service.d/proxy.conf <<'EOF'
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7897"
Environment="HTTPS_PROXY=http://127.0.0.1:7897"
Environment="NO_PROXY=127.0.0.1,localhost"
EOF
```

**方案 B — 用 release 镜像**：

```bash
mkdir -p ~/.config/vpnkit
cat > ~/.config/vpnkit/config.toml <<'EOF'
release_mirror = "https://ghproxy.com/"
EOF
```

（vpnkit 会把 mihomo 二进制下载链接和它运行时的 `geox-url` 都改走镜像。）

#### 1.4 第一次启动

```bash
vpnkit
```

接下来 5–30 秒内你应该看到：

1. 状态栏：`bootstrapping: downloading…` 然后 `mihomo ready`。
2. 左边 6 个 tab 的 sidebar，Dashboard 高亮，主区是 Mihomo header、版本号、模式、`↑0 B/s ↓0 B/s`。
3. 按 `q` 退出。mihomo 在后台继续跑。

如果状态栏出现红色 `bootstrap: ...` 错误信息，看 §3.5 排错。

#### 1.5 加第一个订阅

从你的机场 / VPN 服务商拿一个订阅链接，长这样之一：

- Clash YAML 链接（`https://example.com/sub?token=...`）
- Base64 编码的文本链接（旧式 Shadowsocks）
- 单个 `vmess://` / `ss://` / `trojan://` / `vless://` 等

在 vpnkit 里：

1. 按 `3` — 进 **Profiles** tab，会显示 `No subscriptions yet — press 'a' to add`。
2. 按 `a` — 弹出表单。
3. 输入名字（比如 `airport-A`），按 `Tab`。
4. 粘贴 URL，按 `Enter`。
5. 状态栏：`added airport-A`。Profile 出现在列表，前面有 `★`（active 标记）。
6. 按 `u` — 拉订阅 + 解析 + 写 config.yaml + reload mihomo。
7. 状态栏：`airport-A: 23 nodes`（具体数字按你订阅）。

#### 1.6 选个节点测一下

1. 按 `2` — 进 **Proxies** tab。会看到 proxy groups（如 `🚀 Proxy`、`♻️ Auto`、`🎯 Direct`）。
2. 用 `↑`/`↓`（或 `j`/`k`）高亮 `🚀 Proxy`。按 `Enter` 展开 — 显示组内所有节点。
3. 按 `t` — 对组内每个节点跑延迟测试。5 秒内每个节点旁边出现毫秒数（如 `45 ms`、`120 ms`）。
4. 高亮最快的节点，按 `Enter` 切到它。当前选中的 `✓` 标记移动。
5. 按 `q` 退出 TUI。

#### 1.7 在终端里使用代理

vpnkit 不会自动设代理环境变量 — 它的 mihomo 只是在 `127.0.0.1:7890`（HTTP+SOCKS 混合端口）监听。你按需在每个 shell 里 opt-in：

```bash
eval "$(vpnkit env --shell zsh)"   # 或 --shell bash / --shell fish
echo $http_proxy                   # http://127.0.0.1:7890
curl -I https://www.google.com     # 200 OK
```

取消：

```bash
eval "$(vpnkit env --shell zsh --unset)"
```

如果想常驻每个新 shell，加到 `~/.zshrc`（不推荐，因为有时候你不想走代理；建议做个 shell 函数）：

```bash
proxy-on()  { eval "$(vpnkit env --shell zsh)"; }
proxy-off() { eval "$(vpnkit env --shell zsh --unset)"; }
```

浏览器 / GUI 应用要单独配系统代理（GNOME 设置 → 网络 → 网络代理 → 手动 → HTTP/HTTPS = `127.0.0.1:7890`）。

---

### 2. TUI 各页面详解

TUI 有 6 个 tab。用数字键 `1`–`6` 直跳，`Tab`/`Shift+Tab` 循环切。这里列的快捷键只在该 tab active 时有效（标 GLOBAL 的除外）。

#### 2.1 Dashboard（tab `1`）

```
Mihomo

  Status : ● running
  Version: v1.19.16
  Mode   : rule

  ↑ 1.2 KiB/s
  ↓ 4.5 MiB/s
```

**展示**：mihomo 运行状态、版本号、规则模式（rule/global/direct）、实时上下行速率（通过 `/traffic` SSE 流）。

**快捷键**（GLOBAL — 每个 tab 都有效）：

| 键 | 动作 |
|---|---|
| `q` / `Ctrl+C` | 退出（mihomo 继续跑） |
| `1`–`6` | 跳到对应 tab |
| `Tab` / `Shift+Tab` | 循环切 tab |

#### 2.2 Proxies（tab `2`）

```
Proxies

▶ 🚀 Proxy             Selector      → HK-01
    ✓ HK-01                            45 ms
      JP-02                            87 ms
      US-03                            210 ms
  ♻️ Auto               URLTest       → HK-01
  🛑 Reject             Selector      → REJECT
```

**展示**：所有 proxy group（Selector / URLTest / Fallback）和当前选中的成员。每 5 秒轮询 `/proxies` 刷新。

**快捷键**：

| 键 | 动作 |
|---|---|
| `↑` `↓` `j` `k` | 上下移动 |
| `Enter` | 展开组 / 切节点 |
| `t` | 对当前组跑延迟测试，结果以毫秒显示 |

#### 2.3 Profiles（tab `3`）

```
Profiles

★ airport-A   https://example.com/sub        nodes=23
  airport-B   https://other.example.com/sub  nodes=11

[a] add  [u] update  [Enter] activate  [d] delete  [↑↓] navigate
```

**展示**：订阅列表、已知节点数、`★` 标记当前 active 的订阅（其节点构成当前 `~/.config/mihomo/config.yaml`）。

**快捷键**：

| 键 | 动作 |
|---|---|
| `a` | 弹出表单：name + URL |
| `u` | 重新拉当前订阅 + 重生成 config + reload mihomo |
| `Enter` | 把当前条目设为 active |
| `d` | 删除当前条目 |
| `↑` `↓` `j` `k` | 上下移动 |

表单弹出时：`Tab` 切换 Name/URL 字段，`Enter` 保存，`Esc` 取消。

#### 2.4 Connections（tab `4`）

```
Connections
  ↑ 12 MiB    ↓ 340 MiB    47 active

  HOST                            PORT    UP            DOWN          RULE
▶ www.google.com                  443     2.1 KiB       45 KiB        Match
  api.github.com                  443     800 B         12 KiB        DOMAIN-SUFFIX
  ...

[/] filter  [x] close selected  [↑↓] navigate
```

**展示**：mihomo 知道的所有活跃 TCP/UDP 连接，通过 `/connections` WebSocket 实时流。

**快捷键**：

| 键 | 动作 |
|---|---|
| `↑` `↓` `j` `k` | 上下移动 |
| `x` | 关闭选中连接（mihomo 切断） |
| `/` | 过滤（按 host/rule 子串匹配）— Phase 4 当前限制：filter 输入只能程序化设置 |

#### 2.5 Rules（tab `5`）

```
Rules

Rule Providers
  reject               Domain    count=12345  updated=2026-05-15T08:00:00Z
  proxy                Domain    count=8765   updated=2026-05-15T08:00:00Z
  cncidr               IPCIDR    count=21000  updated=2026-05-15T08:00:00Z
  ...

Rules
  RULE-SET       reject                          → 🛑 Reject
  RULE-SET       direct                          → 🎯 Direct
  GEOIP          CN                              → 🎯 Direct
  RULE-SET       proxy                           → 🚀 Proxy
  MATCH                                          → 🚀 Proxy

[/] filter  [u] refresh providers
```

**展示**：当前生效的全部规则 + rule-providers 状态，每 30 秒轮询。

**快捷键**：底部 `[/]` 和 `[u]` 是占位 — Phase 4 的 filter 输入和 provider 刷新分发还没做完整 UI。

#### 2.6 Settings（tab `6`）

Settings 本身是一个子菜单：

```
Settings        │  ▶ Mihomo Core
                │  ──────────────
▶ Mihomo Core   │     Binary : ~/.local/bin/mihomo
  Service       │     Size   : 33886356 bytes
  External Cont │     Mirror : (direct GitHub)
  Default Rules │
  Patch Editor  │     [u] upgrade to latest release
  Logs          │
  Cache         │
  About         │

[↑↓] navigate
```

**子菜单导航**（只在 Settings tab active 时）：`↑` / `↓` 切换子页。

每个子页有自己的快捷键，下面分别说。

##### 2.6.1 Mihomo Core
- 显示已装的 mihomo 路径/大小 + 当前配置的 release 镜像。
- `u` — 跑 installer 升级 mihomo 到最新 release。

##### 2.6.2 Service
- 显示服务模式（`systemd-user` 或 `pid`）、运行状态、PID。
- `s` start · `S` stop · `r` restart · `u` uninstall（删 systemd unit）。

##### 2.6.3 External Controller
- 显示 controller 端口（默认 9090）和 mask 后的 secret。
- `r` — 重新生成 secret（写到 `~/.config/vpnkit/config.toml`，需要重启 mihomo 生效）。

##### 2.6.4 Default Rules
- 选默认规则模板（`loyalsoldier` 或 `minimal`）。
- `j` / `k` 循环，`Enter` 保存（写到 `config.toml`，下次订阅更新时生效）。

##### 2.6.5 Patch Editor
- 全屏 textarea 编辑 `~/.config/mihomo/patch.yaml`。
- 这个文件每次订阅更新时会被 deep-merge 到生成的 mihomo config 上 — 你的所有手工 override（端口改动、自定义 DNS、额外规则等）都放这里。
- `Ctrl+S` 保存。

##### 2.6.6 Logs
- mihomo 日志的实时 tail（`journalctl --user -u mihomo` 或 PID 模式下的 log 文件）。
- `p` 暂停/恢复。

##### 2.6.7 Cache
- 显示 `~/.cache/vpnkit/` 大小（已下载的 mihomo 压缩包）。
- `c` 清空。

##### 2.6.8 About
- vpnkit 版本、Go 版本、license 信息、源码链接。

---

### 3. 系统层

#### 3.1 systemctl --user 基础

`systemd --user` 就是"systemd，但每个用户一份，不需要 root"。它以你的身份跑服务，单元文件在 `~/.config/systemd/user/`。vpnkit 在这里装了 `mihomo.service`。

常用命令：

```bash
systemctl --user status mihomo       # 在跑吗？最后一行 log 是什么？
systemctl --user start mihomo        # 启动一次
systemctl --user stop mihomo         # 停
systemctl --user restart mihomo
systemctl --user enable mihomo       # 开机/登录自启（vpnkit 装的时候带 --now，这步已做）
systemctl --user disable mihomo      # 不再自启
systemctl --user daemon-reload       # 改了 unit 文件后重新读
journalctl --user -u mihomo -f       # 实时 follow mihomo 日志
journalctl --user -u mihomo -n 50    # 最后 50 行
```

unit 文件在 `~/.config/systemd/user/mihomo.service`。要在 vpnkit 拥有的文件上加东西又不直接改它，往 `~/.config/systemd/user/mihomo.service.d/*.conf` 丢 override 文件（参考 §1.3 的例子）。

#### 3.2 loginctl enable-linger

默认情况下 `systemd --user` 服务在你登出后会退出。意思是：

- SSH 模式：你 SSH 断开 → mihomo 死。
- 桌面：你登出 GNOME/KDE → mihomo 死。

要让你的 user 服务在登出后继续跑，**一次性**执行（需要一次 `sudo`）：

```bash
sudo loginctl enable-linger $USER
```

验证：

```bash
loginctl show-user $USER -p Linger
# Linger=yes
```

之后 mihomo 就能跨 SSH 断开和桌面登出继续跑。要回退：`sudo loginctl disable-linger $USER`。

#### 3.3 环境变量（HTTP_PROXY 等）

大多数 CLI 工具（curl、wget、git、pip、npm、apt、…）都读这些环境变量：

| 变量 | 用于 |
|---|---|
| `http_proxy` | HTTP 请求 |
| `https_proxy` | HTTPS 请求 |
| `all_proxy` | 其他（SOCKS-aware 的工具读这个） |
| `no_proxy` | 逗号分隔的不走代理的 host 列表 |

`vpnkit env` 输出指向 mihomo 混合端口（默认 `127.0.0.1:7890`）的 `export` 语句。用法：

```bash
eval "$(vpnkit env --shell zsh)"     # 设
eval "$(vpnkit env --unset)"          # 取消
```

**不**遵循这些环境变量、需要单独配置的工具：

- 浏览器（用系统/浏览器代理设置）。
- 部分 Java 应用（用 `-Dhttp.proxyHost=...`）。
- Docker（要改 `~/.docker/config.json` 的 proxy 块）。
- Snap 装的应用（沙箱，`snap set system proxy.http=...`）。

#### 3.4 PATH — 让 shell 能找到 `vpnkit` 和 `mihomo`

vpnkit 把二进制装到 `~/.local/bin/`。多数发行版默认会把它放进 PATH，但有些不会。

检查：

```bash
echo $PATH | tr ':' '\n' | grep -F "$HOME/.local/bin"
```

空的话，加到你 shell 的 rc 文件：

```bash
# bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc

# zsh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc

# fish
fish_add_path -U $HOME/.local/bin
```

然后 `exec $SHELL`（或开新终端）。

#### 3.5 快速排错

| 症状 | 可能原因 / 修法 |
|---|---|
| TUI 状态栏出现红色 `bootstrap: ...` | 读那条信息。常见：GitHub 被墙 → 设 release mirror 或 HTTP_PROXY override（§1.3）。 |
| `systemctl --user status mihomo` 显示 `failed` | 看 `journalctl --user -u mihomo -n 50`。如果是"can't download MMDB"，跟上面同样修法。 |
| TUI Dashboard 显示 "Lost connection to core" | mihomo 没跑 或 controller secret 不匹配。检查 `cat ~/.config/vpnkit/config.toml` 的 controller_port + secret，然后 `curl -H "Authorization: Bearer $SECRET" http://127.0.0.1:9090/version`。 |
| `vpnkit: command not found` | `~/.local/bin` 不在 PATH（看 §3.4）。 |
| 节点消失 / 订阅 `u` 后显示 0 nodes | 看 `~/.local/state/vpnkit/vpnkit.log` 里的解析错误。可能机场用了 vpnkit 还不支持的格式 — 提 issue。 |
| Mihomo 自己重启了，状态丢失 | systemd 在 crash 时会自动重启。`systemctl --user status mihomo` 看 `Restart counter`。 |
| 装了 `systemd-resolved` 后连不上 127.0.0.1:7890 | DNS 端口冲突；mihomo 在某些配置下也用 53。检查 `mixed-port` 没意外撞到。 |
