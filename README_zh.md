<h1 align="center">vpnkit</h1>

<p align="center">
  给 <a href="https://github.com/MetaCubeX/mihomo">mihomo</a> 内核做的终端管理 UI。模仿 <a href="https://github.com/clash-verge-rev/clash-verge-rev">Clash Verge</a>。单一 Go 二进制，完全非 root。
</p>

<p align="center">
  <a href="https://github.com/JimZhang168872/vpnkit/releases"><img alt="Tag" src="https://img.shields.io/github/v/tag/JimZhang168872/vpnkit"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/JimZhang168872/vpnkit/actions"><img alt="CI" src="https://github.com/JimZhang168872/vpnkit/actions/workflows/ci.yml/badge.svg"></a>
</p>

<p align="center">English → <a href="README.md">README.md</a></p>

---

vpnkit 完全 user-space 跑 mihomo（仍在维护的 Clash.Meta 内核）— 不需要
root、不依赖 TUN。v1.0.0 新增**多订阅组共存、手动本地节点、结构化本地规则**，
TUI 7 个 tab + 完整 `vpnkit subs / local-nodes / local-rules / target` CLI
都能改。

> **v0.10.x → v1.0.0 是破坏性升级**。store schema v1 → v2。看
> [`docs/UPGRADE-v1.md`](docs/UPGRADE-v1.md) 走迁移流程。

## 安装

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

自动识别 amd64/arm64、SHA256 校验、装到 `~/.local/bin/vpnkit`，生成默认配置骨架，
升级时先清理旧版再装新版。锁版本用 `VERSION=v1.0.0-rc.1 ./install.sh`。确认
`~/.local/bin` 在 `PATH` 上。

源码编译：`git clone … && cd vpnkit && make install`（需要 Go 1.23+）。

> **网络要求：** vpnkit 直连 `github.com`，无 mirror fallback。墙内
> 装请先在 shell 里 `export HTTPS_PROXY=...` 设好已有代理，再跑 install
> 脚本。systemd-user unit 和 bootstrap 会把 shell 代理转发给 mihomo，并
> 在启动前**预下** GeoIP / GeoSite 数据文件，避免 mihomo 首次启动因为拉
> github 而 deadlock。

## 3 分钟上手

```bash
vpnkit
```

首次启动会下 mihomo、生成 `~/.config/mihomo/config.yaml`、装 systemd unit、
预拉 GeoIP 数据、启动 service、打开 TUI。

### 加订阅

```bash
vpnkit subs add doge       https://example.invalid/sub/doge --ua=clash.meta
vpnkit subs add boost-net  https://example.invalid/sub/boost
vpnkit subs update
```

或 TUI：`3` (Sources) → `a` 弹表单 → `Enter`。`u` 拉单个订阅，`e` 启/禁，
`d` 删除。

### 加本地节点（现在支持多组）

```bash
vpnkit local-groups add home
vpnkit local-groups add office
vpnkit local-nodes add 'hysteria2://password@1.2.3.4:443?up=100&down=200#HK-manual' --group=home
vpnkit local-nodes add 'ss://YWVz...@1.2.3.4:8388#JP-rented' --group=office --via=doge-auto
```

`--group` 选节点归属哪个本地组（不存在则自动创建）；`--via` 把节点出口
链式经过某个订阅或本地节点/组（写入 mihomo `dialer-proxy`）。

TUI 里：`3`（Sources）→ `↓` Local Nodes → `N` 创建组 → `←/→` 在组间切换
→ `a` 打开表单（按协议动态字段，含 hy2/tuic up/down 限速 + Via 下拉）。

### 加本地规则（永远胜过订阅规则）

```bash
vpnkit local-rules add DOMAIN-SUFFIX baidu.com '🎯 Direct'
vpnkit local-rules add DOMAIN-KEYWORD internal '🎯 Direct'
vpnkit local-rules list
```

本地规则永远排在所有订阅规则前面，用户意图最高优。

### 顶层路由旋钮

```bash
vpnkit mode rule              # 默认，按规则匹配
vpnkit mode global            # 所有流量走 global target
vpnkit mode direct            # 全直连
vpnkit target doge-auto       # 设 global target（组名或节点名）
```

### 在终端里走代理

```bash
eval "$(vpnkit env --shell zsh)"
curl https://www.google.com
eval "$(vpnkit env --unset)"
```

输出同时设大小写两组（`http_proxy`、`HTTP_PROXY`…）。同时写 `~/.netrc`
（mode 0600）让 curl/git 读 netrc 也能拿凭据。`--no-netrc` 跳过。

想要常驻：

```bash
vpnkit env --shell zsh --functions >> ~/.zshrc
exec zsh
proxy_on    # 🟢 proxy on
proxy_off   # 🔴 proxy off
```

## 升级

vpnkit 启动 2 秒后查 GitHub 最新 release，发现新版就在状态栏显示 `⚡` badge。

```bash
vpnkit update                            # 检查 + 计划 + 交互确认
vpnkit update --yes                      # 跳确认
vpnkit update --check                    # 只看 plan
vpnkit update --vpnkit-only              # 只升 vpnkit
vpnkit update --mihomo-only              # 只升 mihomo
```

## 多源架构

每个订阅都生成自己的 `<name>` (select) + `<name>-auto` (url-test) 组。顶层
`🚀 Proxy` 组列出所有订阅组 + 合成的 `local` 组 + `DIRECT`；rules 的 MATCH
兜底走用户挑的那个 target。详细 assembler 算法见
[`docs/superpowers/specs/2026-05-17-v1-subscription-groups-design.md`](docs/superpowers/specs/2026-05-17-v1-subscription-groups-design.md)。

v1.0.0-rc.3 把之前单一的 `local` 组泛化为用户命名的多组（如 `home`、
`office`）。每个启用的本地组都生成自己的 `<group>` (select) + `<group>-auto`
(url-test) — 跟订阅完全对称。手填节点带 `Via` 字段，直接写入 mihomo
的 `dialer-proxy`，所以可以在表单里 inline 设置每节点链式代理
（Shadowrocket 风格），不用额外动 extensions overlay。

```
proxies: 每个节点重命名 "<group>:<original-name>"，跨组重名不冲突
proxy-groups:
  - {name: doge,        type: select,   proxies: [doge-auto, doge:HK-A, ...]}
  - {name: doge-auto,   type: url-test, proxies: [doge:HK-A, ...], interval: 300}
  - {name: boost,       type: select,   ...}
  - {name: local,       type: select,   proxies: [local:HK-manual, DIRECT]}
  - {name: 🚀 Proxy,    type: select,   proxies: [<global-target>, ...其余]}
rules:
  - <本地规则优先>
  - <每个启用订阅自带的 rules，targets 已重写>
  - MATCH,🚀 Proxy
```

## 多用户 / 多实例

默认端口在 IANA dynamic 区间（30000-60000）用 `crypto/rand` 拒绝采样随机
生成，同机两个用户撞同一端口的概率 < 10⁻⁵。`portutil.FindFree` 兜底再扫
100 个空槽位。

mihomo 三道锁：
- `allow-lan: false`
- `bind-address: 127.0.0.1`
- `authentication: [user:pass]`（首次启动随机生成，存 0600 toml）

systemd-user unit 也是 mode 0600 — 防止 `Environment=` 里的代理凭据
（含密码的 socks5://user:pass@…）被其他本地用户读到。

## Extensions：链式代理 + 自定义代理组

把一个订阅节点串到另一个上游节点（多跳出站，mihomo `dialer-proxy`），或者
新增自己的 proxy-group。配置存 `~/.config/vpnkit/extensions.toml`，**订阅更
新不会丢**。

```bash
vpnkit chain set "US-1" "JP-Relay"        # US-1 出站先经过 JP-Relay
vpnkit chain unset "US-1"
vpnkit group add "Stream" --type select --proxies "US-1,JP-1,DIRECT"
vpnkit ext apply                          # 重 assemble + reload mihomo
```

TUI：Settings → Extensions。`c` chains，`g` groups。`a/e/d` 增/改/删，
`r` 触发 reassemble + reload。

## 命令行

| 命令 | 说明 |
|---|---|
| `vpnkit` | 打开 TUI |
| `vpnkit status` | mihomo 状态、端口、订阅数、本地节点数、模式、global target |
| `vpnkit ip` | 经 mihomo 代理查出口 IP |
| `vpnkit mode [rule\|global\|direct]` | 显示/切换路由模式 |
| `vpnkit target [<组或节点名>]` | 显示/设置 GlobalTarget |
| `vpnkit subs list/add/rm/enable/disable/update [<name>]` | 管理订阅 |
| `vpnkit local-groups list/add/rm/enable/disable/rename` | 管理本地节点组 |
| `vpnkit local-nodes list/add/rm/edit/mv`（含 `--group/--via`） | 管理手动节点 |
| `vpnkit local-rules list/add/rm/move` | 管理本地规则 |
| `vpnkit groups` | 实时 proxy-group 列表（从 mihomo controller 读） |
| `vpnkit nodes '<组>'` | 列某组成员 + 缓存延迟 |
| `vpnkit use '<组>' '<节点>'` | 切换某组的选中节点 |
| `vpnkit env [--shell zsh] [--unset] [--functions] [--no-netrc]` | 输出 shell snippet |
| `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]` | 升级 vpnkit + mihomo |
| `vpnkit init [--force]` | 重建配置骨架（`--force` 会备份旧 store） |
| `vpnkit uninstall [--yes] [--purge] [--keep-mihomo]` | 停服务，删 vpnkit 全部文件 |
| `vpnkit chain ls/set/unset` | 管理 dialer-proxy 链 |
| `vpnkit group ls/add/rm` | 管理自定义代理组 |
| `vpnkit ext apply` | 用当前 extensions 重 assemble + reload mihomo |

只读命令接受 `--json`。退出码：`0` 成功、`1` 用户错、`2` 运行时错。

## TUI 布局 (v1)

```
[1] 🏠 Dashboard      mihomo 状态 / 实时流量
[2] 🌐 Groups         所有组 + 节点（只读 + 延迟测试）
[3] 📚 Sources        Subscriptions / Local Nodes 子页（CRUD）
[4] 📜 Rules          Live (mihomo) / Local Rules 子页
[5] 🔗 Connections    实时连接（`x` 关、`/` 过滤）
[6] 📓 Logs           mihomo 日志
[7] ⚙  Settings       Mihomo Core / Service / External Controller / Routing /
                       Rule Template / Extensions / Cache / About 子页
```

按键：
- `↑↓` 移动 · `←` 退/sidebar focus · `→` content focus / 进 · `Enter` 激活 · `q` 退出
- `1`-`7` 跳 tab · `Tab`/`Shift+Tab` 循环
- **Sources › Subscriptions**：`a` 添加 · `d` 删 · `u` 拉一次 · `e` 启/禁
- **Sources › Local Nodes**：`a` 粘 URI · `d` 删
- **Rules › Live**：`/` 过滤 · `u` 刷新 providers · `Tab` 切 Local Rules
- **Rules › Local Rules**：`d` 删 · `K/J` 上下移 · `Tab` 切回 Live
- **Settings › Routing**：`↑↓ Enter` 选模式；global target 走 CLI 改
- **Settings → Extensions**：`c` chains / `g` groups · `a/e/d` 增/改/删 · `r` 应用

## 目录布局

| 路径 | 用途 |
|---|---|
| `~/.local/bin/vpnkit` | 本程序 |
| `~/.local/bin/mihomo` | 受管 mihomo |
| `~/.config/vpnkit/config.toml` | 订阅、本地节点、本地规则、端口、凭据（schema v2） |
| `~/.config/vpnkit/extensions.toml` | 链式代理 + 自定义代理组覆盖层 |
| `~/.config/mihomo/config.yaml` | 组装的 mihomo 配置（每次 mutation 重写） |
| `~/.config/mihomo/*.mmdb / *.dat` | GeoIP / GeoSite 数据（bootstrap 预下） |
| `~/.config/systemd/user/mihomo.service` | systemd-user unit（mode 0600；转发 HTTPS_PROXY） |
| `~/.netrc` | proxy basic-auth 条目（mode 0600） |
| `~/.local/state/vpnkit/` | 日志、PID 文件 |
| `~/.cache/vpnkit/` | mihomo 压缩包 |

## 不做

TUN 模式、Windows/macOS、命令面板、主题切换、GUI。

## 编译 & 测试

```bash
make build      # ./bin/vpnkit
make test       # go test -race -cover ./...
make lint       # golangci-lint run
```

## License

[MIT](LICENSE)。基于 [mihomo](https://github.com/MetaCubeX/mihomo)、[Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules) 和 [charmbracelet](https://github.com/charmbracelet) TUI 套件。
