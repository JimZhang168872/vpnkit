<h1 align="center">vpnkit</h1>

<p align="center">
  <strong><a href="https://github.com/MetaCubeX/mihomo">mihomo</a> 内核的终端管家</strong> —— 订阅、本地节点、代理链、规则路由,纯 TUI + CLI,不要 Electron,不跑 daemon。模仿 <a href="https://github.com/clash-verge-rev/clash-verge-rev">Clash Verge</a>。单一 Go 二进制,完全非 root。
</p>

<p align="center">
  <a href="https://github.com/JimZhang168872/vpnkit/releases"><img alt="Tag" src="https://img.shields.io/github/v/tag/JimZhang168872/vpnkit"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/JimZhang168872/vpnkit/actions"><img alt="CI" src="https://github.com/JimZhang168872/vpnkit/actions/workflows/ci.yml/badge.svg"></a>
</p>

<p align="center">English → <a href="README.md">README.md</a></p>

---

vpnkit 完全 user-space 跑 mihomo（仍在维护的 Clash.Meta 内核）— 不需要
root、不依赖 TUN。v1.0.0 新增**多订阅组共存、手动本地节点、结构化本地规则、
单一活跃源路由模型**，TUI 7 个 tab + 完整
`vpnkit subs / local-nodes / local-rules / active` CLI 都能改。

默认 loyalsoldier 规则集快照（~2 MB gz）**嵌入在二进制里**，bootstrap
在 mihomo 首次启动前解包出来，所以慢网络 / 国内被劫持 jsdelivr 也能秒
出 RULE-SET 规则。

> 📖 **完整技术参考**：[docs/USAGE_zh.md](docs/USAGE_zh.md)（中文）/
> [docs/USAGE.md](docs/USAGE.md)（English）—— 每个 CLI 命令、每个 TUI tab、
> 每个按键、每份 JSON 输出 schema、配置文件结构、延迟测试深度剖析、故障
> 排查。**README 之外的所有疑问都先看这里。**

> **v0.10.x → v1.0.0 是破坏性升级**。store schema v1 → v2。看
> [`docs/UPGRADE-v1_zh.md`](docs/UPGRADE-v1_zh.md) 走迁移流程。

## 安装

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

自动识别 amd64/arm64、SHA256 校验、装到 `~/.local/bin/vpnkit`，生成默认配置骨架，
升级时先清理旧版再装新版。锁版本用 `VERSION=v1.0.0-rc.1 ./install.sh`。确认
`~/.local/bin` 在 `PATH` 上。

源码编译：`git clone … && cd vpnkit && make install`（需要 Go 1.23+）。

> **网络要求：** vpnkit 直连 `github.com`，无 mirror fallback。
>
> **🇨🇳 在墙内?** 看 [**`docs/INSTALL-CN.md`**](docs/INSTALL-CN.md) —
> 三条安装路径(已有代理 / GitHub 镜像 / 完全离线)+ 常见报错排查
> (`SSL_ERROR_SYSCALL`、`connection refused`、`vpnkit init` 卡在
> mihomo 下载等)。简单情况:`export HTTPS_PROXY=http://127.0.0.1:7897`
> 后再跑上面的 install 脚本即可。

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
vpnkit mode global            # 所有流量走 🚀 Proxy (即 active source)
vpnkit mode direct            # 全直连

# Active source（活跃源）—— 单一路由真相。Groups tab 用 ★ 标
vpnkit active                 # 看当前 active（订阅或本地组）
vpnkit active boost-net       # 切到某个订阅
vpnkit active home            # 切到某个本地组

vpnkit target doge-auto       # 覆盖 🚀 Proxy 默认成员（进阶）
```

**活跃源**是 1 个订阅 OR 1 个本地节点组。它的 `rules:` 段驱动路由；如果
没自带 rules（本地组永远没），就用 loyalsoldier 模板兜底。`🚀 Proxy`
的成员只放活跃源的节点 + `DIRECT`。切换 active 等于同时切规则基线和
兜底代理。

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
（Shadowrocket 风格）。

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

## 本地节点链式出站（Via 字段）

Sources › Local Nodes 的 Add/Edit 表单最后一个字段就是 `Via`：填上任何
mihomo 已知的代理名或代理组名，节点出站时会先走那个上游 —— 等价于 mihomo
的 `dialer-proxy` 字段。Via 写在节点上，订阅更新不丢，节点在本地组之间
移动时跟着走。

```
Via: doge-auto              # 任何订阅/本地节点名 或 组名
```

## 命令行

| 命令 | 说明 |
|---|---|
| `vpnkit` | 打开 TUI |
| `vpnkit version` / `--version` / `-v` | 版本 + commit + mihomo 路径 |
| `vpnkit --help` / `-h` / `help` | 顶层用法（也支持每个子命令 `<verb> --help`） |
| `vpnkit status` | mihomo 状态、端口、订阅数、本地节点数、模式、active source |
| `vpnkit ip` | 经 mihomo 代理查出口 IP |
| `vpnkit mode [rule\|global\|direct]` | 显示/切换路由模式 |
| `vpnkit active [<name>]` | 显示/切换 active source（订阅 或 本地节点组） |
| `vpnkit target [<member>]` | 覆盖 🚀 Proxy 默认成员（进阶——通常用 `active` 就够了） |
| `vpnkit subs list/add/rm/enable/disable/update [<name>]` | 管理订阅 |
| `vpnkit local-groups list/add/rm/enable/disable/rename` | 管理本地节点组 |
| `vpnkit local-nodes list/add/rm/edit/mv`（含 `--group/--via`） | 管理手动节点 |
| `vpnkit local-rules list/add/rm/move` | 管理本地规则（type + payload + target 全校验） |
| `vpnkit groups` | 实时 proxy-group 列表（从 mihomo controller 读） |
| `vpnkit nodes '<组>'` | 列某组成员 + 缓存延迟（被动读，mihomo url-test 缓存） |
| `vpnkit test '<组>' ['<节点>']` | 主动测延迟（见 [USAGE.md › 延迟测试](docs/USAGE_zh.md#延迟测试详解)） |
| `vpnkit use '<组>' '<节点>'` | 切换某组的选中节点 |
| `vpnkit env [--shell bash\|zsh\|fish] [--unset] [--functions] [--no-netrc]` | 输出 shell snippet（shell 类型有校验） |
| `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]` | 升级 vpnkit + mihomo |
| `vpnkit init [--force]` | 重建配置骨架（`--force` 会备份旧 store） |
| `vpnkit uninstall [--yes] [--purge] [--keep-mihomo]` | 停服务，删 vpnkit 全部文件 |

只读命令（`status`、`ip`、`groups`、`nodes`、`test`、`mode`、`target`、
`active`、`... ls`）接受 `--json`；mutation 命令拒绝 `--json` 并报清楚
错误。JSON 模式下运行时失败也写 `{"error":"…"}` 到 stdout，消费脚本仍
能 parse。

退出码：`0` 成功、`1` 用户错、`2` 运行时错。并发 CLI mutation 通过
config 文件的 POSIX flock 序列化，所以 `vpnkit subs add foo url &`
并行也是安全的。每命令 flag 详解 + JSON schema 在
[docs/USAGE_zh.md › CLI 参考](docs/USAGE_zh.md#cli-参考)。

## TUI 布局 (v1)

```
[1] 🏠 Dashboard      mihomo 状态 / 实时流量
[2] 🌐 Groups         所有组 + 节点（★ 标 active source · 延迟测试）
[3] 📚 Sources        Subscriptions / Local Nodes 子页（CRUD）
[4] 📜 Rules          Live (mihomo) / Local Rules 子页
[5] 🔗 Connections    实时连接（`x` 关、`/` 过滤）
[6] 📓 Logs           mihomo 日志（`p` 暂停/继续）
[7] ⚙  Settings       Mihomo Core / Service / External Controller / Routing /
                       Active Source / Rule Template / Cache / About 子页
```

按键：
- `↑↓` 移动 · `←` 退/sidebar focus · `→` content focus / 进 · `Enter` 激活 · `q` 退出 · `Ctrl+C` 退出（表单内也算）
- `1`-`7` 跳 tab · `Tab`/`Shift+Tab` 循环 · `?` keymap 提示 flash
- **Groups**：`r` 刷新 · `t` 测延迟（主动 probe 当前组） · `Enter` 切到高亮节点 · `←/→` 切左右面板 focus · 组名旁的 `★` 表示 active source
- **Sources › Subscriptions**：`a` 添加 · `d` 删 · `u` 拉一次 · `e` 启/禁（长列表自动滚动）
- **Sources › Local Nodes**：`a` 添加（proto 表单）· `e` 编辑 · `d` 删 · `u` 粘 URI ·
  `N`/`D`/`E`/`T` 新建/删/重命名/启停 group · `←/→` 切换 group（无表单时）· 凭据字段（password / uuid）以圆点显示
- **添加/编辑节点表单**：`Tab/↑↓` 切字段 · `Enter` 保存（按协议校验必填字段）· `Esc` 取消 ·
  Proto 字段上按 `←/→` 循环 ss / vmess / vless / trojan / hysteria2 / tuic
- **Rules › Live**：`/` 过滤 · `u` 刷新 providers · `T` (Shift+t) 切 Local Rules
- **Rules › Local Rules**：`d` 删 · `K/J` 上下移 · `T` 切回 Live
- **Logs**：`p` 暂停/继续 tail
- **Settings › Routing**：`↑↓ Enter` 选模式 · 异步 apply (mihomo reload 不阻塞主循环)
- **Settings › Active Source**：`↑↓ Enter` 选 active source —— 决定 🚀 Proxy + rules

最小可用终端尺寸：**60×16**。比这小会显示 "terminal too narrow" 提示
而不是渲染崩坏的 layout。

每个 tab 的按键表 + 行为详解在 [docs/USAGE_zh.md › TUI 参考](docs/USAGE_zh.md#tui-参考)。

## 目录布局

| 路径 | 用途 |
|---|---|
| `~/.local/bin/vpnkit` | 本程序 |
| `~/.local/bin/mihomo` | 受管 mihomo |
| `~/.config/vpnkit/config.toml` | 订阅、本地节点、本地规则、端口、凭据、active_source（schema v2） |
| `~/.config/vpnkit/config.toml.lock` | POSIX flock 锁文件 —— 并发 CLI mutation 串行化 |
| `~/.config/mihomo/config.yaml` | 组装的 mihomo 配置（每次 mutation 重写） |
| `~/.config/mihomo/ruleset/*.txt` | loyalsoldier 规则集快照（启动时从 binary 内嵌的 `.txt.gz` 解压） |
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
