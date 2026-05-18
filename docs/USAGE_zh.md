# vpnkit 技术参考手册

> English → [USAGE.md](USAGE.md)

vpnkit 最详细的使用文档：每个 CLI 命令、每个 TUI tab、每个键、每份 JSON
输出 schema、配置文件结构、延迟测试深度剖析、故障排查。可以从头读，也可以
跳到对应章节。

- [快速开始](#快速开始)
- [核心概念](#核心概念)
- [CLI 参考](#cli-参考)
- [TUI 参考](#tui-参考)
- [文件布局](#文件布局)
- [配置 schema](#配置-schema)
- [延迟测试详解](#延迟测试详解)
- [多用户同机部署](#多用户同机部署)
- [故障排查](#故障排查)

---

## 快速开始

```bash
# 装包（放到 PATH 上 + 写默认配置）
mv vpnkit ~/.local/bin/
vpnkit init                                # 生成 ~/.config/vpnkit/config.toml

# 订阅一个 feed
vpnkit subs add main "https://provider.example.com/sub?token=..."
vpnkit subs update                         # 拉取所有 enabled 订阅的节点

# 拿到 shell 代理变量
eval "$(vpnkit env --functions)"           # 装 proxy_on / proxy_off 函数
proxy_on                                   # export HTTPS_PROXY 等

# 打开 TUI
vpnkit                                     # 7-tab 交互界面
```

如果 `~/.local/bin` 不在 `$PATH` 上，跑一次 `vpnkit env` 它会打印推荐的
`export PATH=...` 片段。

---

## 核心概念

| 概念 | 含义 |
|---|---|
| **mihomo** | 底层代理内核 ([MetaCubeX/mihomo](https://github.com/MetaCubeX/mihomo))。vpnkit 拼装它的 config.yaml、启动它、读它的 controller API |
| **store** | `~/.config/vpnkit/config.toml`。订阅、本地节点 / 组、本地规则、端口、凭据的 single source of truth。Schema v2 |
| **subscription** | 远程订阅 URL，返回 base64 / clash-yaml 节点列表。每个 enabled 订阅在 mihomo 里生成 `<name>` select 组 + `<name>-auto` url-test 组 |
| **local node group** | 用户命名的手填节点容器（如 `home`、`office`）。跟订阅完全对称 —— 每个 enabled 组也生成 select + url-test 这一对 |
| **Local node `Via`** | 节点自带的 `dialer-proxy` 目标。设成任何 proxy / 组名就让 mihomo 先经那一跳出去（Shadowrocket 风格的 inline 链式代理） |
| **Routing mode** | `rule` / `global` / `direct`。存在 `store.Cfg.Mode`；rule 模板生成的 mihomo `rules:` 决定行为，跟 mihomo 自己的 mode 设置无关 |
| **Global target** | mode=global 时谁兜底接管。默认 `🚀 Proxy` 的某个具体成员（rc.6 起根据有没有订阅自动选 `<name>-auto` 或 `DIRECT`） |
| **Controller secret** | 随机 token 存在 store 里，写进 mihomo 的 `secret:` 和 vpnkit API client 两边。`init --force` 重置 |
| **Service mode** | `systemd-user`（Linux 默认）或 `pid`（fallback —— 自己管 pidfile + 子进程）。存 `store.Cfg.ServiceMode` |

---

## CLI 参考

约定：
- `[ ]` = 可选；`< >` = 必填位置参数
- `--json` 在所有只读命令上都支持；输出是单 JSON 文档（不是 NDJSON）
- 退出码：
  - `0` 成功（"测出来 timeout" 也是 0 —— 是合法结果，不是错）
  - `1` 用户错（参数错、实体不存在）
  - `2` 运行时错（mihomo 不可达、文件 IO 失败、save 失败）

### `vpnkit` *（无参数）*

打开 7-tab TUI。等同于无 subcommand 启动。详见 [TUI 参考](#tui-参考)。

### `vpnkit version`（别名 `--version`, `-v`）

打印 vpnkit semver + commit + 构建时间 + mihomo 二进制路径 + 大小。永远返回 0。

### `vpnkit env [--shell bash|zsh|fish] [--unset] [--no-proxy CSV] [--functions] [--no-netrc]`

输出 shell export 片段配代理变量；可选写 `~/.netrc` 做 basic-auth。

| Flag | 默认 | 含义 |
|---|---|---|
| `--shell` | 从 `$SHELL` 推断 | 输出方言（bash/zsh/fish） |
| `--unset` | false | 输出 `unset`/`erase` 而不是 `export`/`set` |
| `--no-proxy` | `localhost,127.0.0.1,::1` | 赋给 `no_proxy` 的列表 |
| `--functions` | false | 输出 `proxy_on` / `proxy_off` 函数定义（一次性附加到 `~/.zshrc`） |
| `--no-netrc` | false | 即使有 creds 也不写 `~/.netrc` |

永远退出 0。从 store 读 `mixed_port` / `proxy_user` / `proxy_pass`。

### `vpnkit status [--json]`

snapshot：mihomo 版本、服务运行状态、端口、mode、订阅 / 本地节点 / 本地规则数量、controller URL。

JSON 结构（简版）：
```json
{
  "vpnkit_version": "v1.0.0-rc.6",
  "mihomo": {"version": "v1.16.0", "running": true, "pid": 12345},
  "ports": {"mixed": 7890, "controller": 9090},
  "mode": "rule",
  "global_target": "doge-auto",
  "subscriptions": 2,
  "local_nodes": 3,
  "local_rules": 4
}
```

### `vpnkit ip [--json]`

经 mihomo 的 mixed-port 去 `https://ipinfo.io/json`，所以返回的 IP / 国家 /
ISP 是出口 IP，不是你的本机。还报告匹配到哪个 proxy 组。

JSON 结构：
```json
{"ip": "203.0.113.45", "country": "HK", "region": "Hong Kong",
 "city": "Hong Kong", "org": "AS12345 Example", "via": "🚀 Proxy → HK-01"}
```

mihomo 不可达 / ipinfo 超时退出 2。

### `vpnkit mode [rule|global|direct] [--json]`

无参数 → 打印当前 mode。有参数 → 写到 store + 重新 assemble + hot-reload mihomo。

JSON（set）: `{"from": "rule", "to": "global"}`
JSON（get）: `{"mode": "rule"}`

### `vpnkit target [<group-or-node>]`

查看或设置 `global_target`。`global` 模式下用作兜底成员。名字必须匹配 mihomo
已知的 proxy 或 group（assemble 时验证，不是这里验证）。

### `vpnkit subs <verb> ...`

| Verb | 用法 | 行为 |
|---|---|---|
| `list` | `vpnkit subs list [--json]` | 表格列出所有订阅 + 节点数 + URL |
| `add` | `vpnkit subs add <name> <url> [--ua USER_AGENT]` | 追加；重名报错 |
| `rm` | `vpnkit subs rm <name>` | 从 store 删除 + 清缓存 |
| `enable` | `vpnkit subs enable <name>` | 翻 `enabled` 到 true |
| `disable` | `vpnkit subs disable <name>` | 翻 `enabled` 到 false |
| `update` | `vpnkit subs update [<name>...]` | fetch + 解析 + 缓存。不带 name 就更新所有 enabled。每个超时 60s |

`list` JSON 项：`{"name": "main", "url": "...", "user_agent": "",
"enabled": true, "node_count": 50}`。

### `vpnkit local-groups <verb> ...`

| Verb | 用法 | 行为 |
|---|---|---|
| `list` | `vpnkit local-groups list [--json]` | name + enabled |
| `add` | `vpnkit local-groups add <name>` | 创建空组 |
| `rm` | `vpnkit local-groups rm <name> [--force]` | 删除；`--force` 级联删节点 |
| `enable` | `vpnkit local-groups enable <name>` | 翻 enabled |
| `disable` | `vpnkit local-groups disable <name>` | 翻 enabled |
| `rename` | `vpnkit local-groups rename <old> <new>` | 也移动成员节点（Group 字段就地改写） |

### `vpnkit local-nodes <verb> ...`

| Verb | 用法 |
|---|---|
| `list` | `vpnkit local-nodes list [--json]` |
| `add` | `vpnkit local-nodes add <uri> [--group=NAME] [--via=PROXY]` |
| `rm` | `vpnkit local-nodes rm <ref>` |
| `mv` | `vpnkit local-nodes mv <ref> <new-group>` |
| `edit` | `vpnkit local-nodes edit <ref> key=val [key=val ...]` |

**节点 ref** 可以是短名（如 `JP-A`）或命名空间形式（`group:JP-A`）。短名遇到
多组同名时报错 —— 写脚本时建议总是用命名空间。

**`add`** 解析任意 6 种支持协议的 proxy URI（ss / vmess / vless / trojan /
hysteria2 / tuic）。`--group` 默认 `local`，如果不存在会自动建组。`--via`
就是设 [Local node Via](#核心概念)，写到 mihomo 的 `dialer-proxy`。

**`edit`** 认识的 key：`name`, `group`, `via`, `server`, `port`, `proto`。
其他（如 `password=...`, `cipher=...`）都进 `Fields` blob —— 协议特定字段
映射到 mihomo 对应字段名。Port 解析为 int，其他保留为 string。

**`mv`** 自动建目标组。

`list` JSON 项：`{"name": "JP-A", "group": "home", "via": "doge-auto",
"proto": "hysteria2", "server": "jp.example.com", "port": 443, "fields":
{"password": "...", ...}}`。

### `vpnkit local-rules <verb> ...`

| Verb | 用法 | 备注 |
|---|---|---|
| `list` | `vpnkit local-rules list [--json]` | 显示 index + type + payload + target |
| `add` | `vpnkit local-rules add <type> <payload> <target>` | 追加到末尾 |
| `rm` | `vpnkit local-rules rm <idx>` | 0-indexed |
| `move` | `vpnkit local-rules move <from> <to>` | 重排序（前面的 rule 优先匹配） |

常见 rule type：`DOMAIN`, `DOMAIN-SUFFIX`, `DOMAIN-KEYWORD`, `IP-CIDR`,
`PROCESS-NAME`, `MATCH`。Local rules 在最终 config.yaml 里**先于**订阅规则。

`list` JSON 项：`{"type": "DOMAIN-SUFFIX", "payload": "github.com",
"target": "🚀 Proxy"}`。

### `vpnkit groups [--json]`

mihomo 实时 `/proxies` 快照。过滤掉 GLOBAL/DIRECT/REJECT 等不可选项。

JSON：`{"name": "doge", "type": "Selector", "now": "doge:HK-01",
"members": 12}` 数组。

### `vpnkit nodes <group> [--json]`

列出 `<group>` 的成员 + mihomo **缓存**的延迟（它自己 url-test 跑的）。要
拿到新鲜数据用 [`vpnkit test`](#vpnkit-test-group-node)。

JSON：`{"group": "doge", "current": "doge:HK-01", "nodes": [{"name":
"doge:HK-01", "delay": 234}, {"name": "doge:JP-02", "delay": null}]}`。
`delay: null` 表示"从没测过"。

### `vpnkit test <group> [<node>] [--url URL] [--timeout-ms MS] [--json]`

主动延迟测试 —— 详见 [深度剖析](#延迟测试详解)。带 `<node>` 测单个，否则测整组并发。
默认：`--url=https://www.gstatic.com/generate_204 --timeout-ms=5000`。

### `vpnkit use <group> <node> [--json]`

调 mihomo `PUT /proxies/<group>` 切换当前选中节点。节点名要是 mihomo 那边
的名字（订阅 / 本地节点都是 `<group>:<原名>` 命名空间形式 —— 跟
`vpnkit nodes` 显示的一致）。

### `vpnkit init [--force]`

无参数：缺则生成 `~/.config/vpnkit/config.toml`，挑空闲 TCP 端口给 mixed-port
和 controller-port，生成随机 controller secret + proxy basic-auth creds。
有 store 就不动。

带 `--force`：把现有 store 备份到 `config.toml.bak.<ts>`，重新生成。修复
损坏 store / 一键重置 secret 用。

### `vpnkit uninstall [--yes] [--purge] [--keep-mihomo] [--keep-profiles=true|false] [--backup-dir DIR]`

best-effort 卸载：停 mihomo 服务、删 systemd unit、4 个 XDG 目录
（vpnkit + mihomo + state + cache）、两个二进制。

| Flag | 默认 | 作用 |
|---|---|---|
| `--yes` | false | 跳过交互确认 |
| `--purge` | false | 删 profiles（不备份）—— 隐含 `--keep-profiles=false` |
| `--keep-mihomo` | false | 不删 `~/.local/bin/mihomo` |
| `--keep-profiles` | true | 备份 profiles 到 `--backup-dir`（设 false 直接丢） |
| `--backup-dir` | `/tmp` | profile backup 归档位置 |

如果 `HOME` 未设或非绝对路径，退出 2 拒绝执行（避免乱删 cwd）。备份发生
时 stdout 输出 `BACKUP=<path>` 一行 —— install / uninstall script 可以 grep。

### `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]`

检查 GitHub releases 上是否有新版 vpnkit + mihomo，提示（除非 `--yes`），下载，
换二进制，self-update 时重新 exec vpnkit。

| Flag | 作用 |
|---|---|
| `--check` | 只打印计划，不装 |
| `--yes` | 跳确认 |
| `--vpnkit-only` | 只升级 vpnkit |
| `--mihomo-only` | 只升级 mihomo |

---

## TUI 参考

`vpnkit` 无参数启动。两级 focus 模型：
- **MainSidebar focus** —— 顶部 tab 列表 owns ↑/↓ 切 tab
- **TabBody focus** —— 当前 tab owns ↑/↓ 做自己的导航

`←` 永远把 focus 推回 MainSidebar 方向。`→` 永远把 focus 推到 tab body /
子页面 content。`1`–`7` 跳 tab。`Tab`/`Shift+Tab` 循环。`q` / `Ctrl+C` 退出。

textinput 类 overlay 打开时（Sources Add form、Connections filter、Rules
filter），**所有**键 —— 包括数字和 Tab —— 都交给 input。不被全局劫持。

### Tab 1: Dashboard

单 pane。显示：
- 服务状态（●/○ running/stopped）+ mode + PID
- mihomo 版本
- 端口（mixed-port + external-controller）
- 实时流量（↑ up、↓ down，自动单位 B/s, KiB/s, MiB/s）
- 更新 badge（GitHub 上有新 vpnkit / mihomo 时）

只读。无 tab 特定快捷键。

### Tab 2: Groups

两 pane。

左 pane 列出所有 proxy 组：
```
▶ doge (12)         → doge:HK-01
  boost (8)         → boost:relay-1
  home (3)          → home:JP-A
```
`→ <name>` 后缀是 mihomo 当前 `now`（活跃成员）。

右 pane 列出选中组的成员：
```
▶ ● doge:HK-01      hysteria2  hk.example.com:443      234 ms
    doge:JP-02      vmess      jp.example.com:443      567 ms
    doge:SG-03      trojan     sg.example.com:443      timeout
```
`● ` 标记当前 `now`。尾部 `XXX ms` / `timeout` 是本会话内做过延迟测试后才
显示（颜色：< 200ms 绿 / 200-500 黄 / >500 红 / timeout 红）。没测过 →
不显示。

| 键 | 作用 |
|---|---|
| `←` / `→` | 左 pane ↔ 右 pane 切 focus |
| `↑` / `↓` (左 pane) | 移动组 cursor；右 cursor 重置到 0 |
| `↑` / `↓` (右 pane) | 在当前组的节点列表内移动 |
| `r` | 从 store 刷新组列表 |
| `t` | 对当前组主动测延迟（[详解](#延迟测试详解)） |
| `Enter` (右 pane) | `PUT /proxies/<group>` 切换到高亮节点 |
| `Enter` (左 pane) | 提示先按 → focus 到右 pane |

### Tab 3: Sources

两个子页面：**Subscriptions** 和 **Local Nodes**。`↑`/`↓` 在左侧 sub-sidebar
上切换子页面。

#### Subscriptions 子页

列表视图。每行：`[✓] <name>  nodes=N  <URL>`。

| 键 | 作用 |
|---|---|
| `a` | 打开 Add Subscription 表单（Name / URL / User-Agent） |
| `d` | 删除高亮项 |
| `u` | 立即 fetch + 解析（60s 超时） |
| `e` | 切换 enabled 状态（✓ ↔ ✗） |

Add 表单：`Tab`/`↑↓` 循环字段，`Enter` 确认（或跳下一字段，最后一字段才提交），
`Esc` 取消。

#### Local Nodes 子页

顶部 group tab bar + 节点列表。Tab bar 类似 `▶ home  office (disabled)
[+ new group]`。无 form 打开时 `←/→` 切组。

| 键 | 作用 |
|---|---|
| `a` | 打开 Add Local Node 表单（协议驱动，默认 hysteria2） |
| `e` | 编辑高亮项（表单预填当前值） |
| `d` | 删除高亮项 |
| `u` | URI 粘贴表单（一次性从剪贴板） |
| `N` | 新建组 |
| `D` | 删当前组（非空报错并提示） |
| `E` | 重命名当前组 |
| `T` | 切换组 enabled |
| `←` / `→` | 切到上一个 / 下一个组 |

#### Add/Edit Node 表单

字段随选定协议变。通用字段在前（name / group / server / port），协议特定
字段在后（如 ss 的 cipher + password；vmess 的 uuid + alterId + cipher +
network + ws-opts.host/path + tls + servername 等），Via 最后。

| 键 | 作用 |
|---|---|
| `Tab` / `↑↓` | 循环字段 |
| `←` / `→`（在 Proto 字段，focused=0） | 循环 ss / vmess / vless / trojan / hysteria2 / tuic。通用字段跨循环保留 |
| `Enter` | 保存（Add 模式 → `Manager.Add`；Edit 模式 → Remove+Add，重名 collision 回滚） |
| `Esc` | 取消 |

### Tab 4: Rules

两个子页面：**Live** 和 **Local Rules**。

#### Live 子页

实时 `/rules` + `/providers/rules` 视图。只读。

| 键 | 作用 |
|---|---|
| `/` | 进入 filter 模式（substring 匹配 type+payload+proxy） |
| `Esc` | 退出 filter |
| `↑` / `↓` / `PgUp` / `PgDown` | 导航 |
| `u` | 刷新 rule providers |
| `Tab` | 切到 Local Rules 子页 |

#### Local Rules 子页

`store.Cfg.LocalRules` 的 CRUD。Local rules 在拼装 config 里**先于**订阅规则。

| 键 | 作用 |
|---|---|
| `d` | 删除高亮 rule |
| `K` | 高亮上移 |
| `J` | 高亮下移 |
| `Tab` | 切回 Live |

（Add 目前只走 CLI —— `vpnkit local-rules add <type> <payload> <target>`）

### Tab 5: Connections

实时 `/connections`（WebSocket 流）。列：host, port, network, upload, download,
rule, chain。

| 键 | 作用 |
|---|---|
| `/` | 进入 filter 模式（substring 匹配 host 或 chain） |
| `Esc` | 退出 filter |
| `↑` / `↓` / `PgUp` / `PgDown` | 导航 |
| `x` | `DELETE /connections/<id>` 关闭高亮连接 |

### Tab 6: Logs

mihomo 日志 tail（PID 模式 = `~/.local/state/vpnkit/mihomo.log`；systemd-user
模式 = journalctl）。Ring buffer ≈ 1000 行。

只读。行 truncate 防溢出（保证不会换行顶掉 tab bar）。

### Tab 7: Settings

子 sidebar 列出 7 个子页面。↑/↓ 切换；← 退回 MainSidebar；→ 进入 content
（只对 Routing 和 Rule Template 这两个 own arrows 的子页有意义）。

| 子页 | 显示什么 |
|---|---|
| **Mihomo Core** | binary 路径、版本、mixed-port、controller-port、secret（mask）、proxy basic-auth user（mask） |
| **Service** | systemd-user vs pid 模式、running 状态、log 路径、last error |
| **External Controller** | URL + secret（mask）、复制提示 |
| **Routing** | mode selector（rule / global / direct）+ global target —— `↑↓ Enter` 选；改动后自动 reload mihomo |
| **Rule Template** | mihomo rule 模板（curated 列表）—— `↑↓ Enter` 应用 |
| **Cache** | mihomo cache 目录 + 大小 + 最后修改 |
| **About** | vpnkit 版本 + commit + license + repo URL |

大多数子页只读；Routing 和 Rule Template 才会改 store。

---

## 文件布局

| 路径 | 归属 | 用途 |
|---|---|---|
| `~/.local/bin/vpnkit` | 用户 | 本程序 |
| `~/.local/bin/mihomo` | 用户 | 受管 mihomo 核心（自动装的） |
| `~/.config/vpnkit/config.toml` | vpnkit | **store**（schema v2）：subs、本地节点 / 组 / 规则、端口、creds、mode、service mode |
| `~/.config/mihomo/config.yaml` | vpnkit | 组装的 mihomo 配置（每次 mutation 重写） |
| `~/.config/mihomo/cache.db` | mihomo | mihomo session 缓存 |
| `~/.config/mihomo/ruleset/*.txt` | vpnkit 预置 | loyalsoldier 规则集 snapshot（bootstrap 时落盘，mihomo 后台 refresh 更新） |
| `~/.config/mihomo/*.mmdb`, `*.dat` | bootstrap | GeoIP / GeoSite（预下载） |
| `~/.config/systemd/user/mihomo.service` | vpnkit | systemd unit（mode 0600，转发 `HTTPS_PROXY`） |
| `~/.netrc` | `vpnkit env` | proxy basic-auth 条目（mode 0600） |
| `~/.local/state/vpnkit/` | 运行时 | mihomo log + PID file（仅 PID 模式） |
| `~/.cache/vpnkit/` | 运行时 | 下载的归档、rule 模板 |

路径解析在 `internal/paths/paths.go`，遵循 `$XDG_CONFIG_HOME`、`$XDG_STATE_HOME`、
`$XDG_CACHE_HOME`、`$XDG_RUNTIME_DIR` —— 测试 / sandbox 隔离自动生效。

---

## 配置 schema

```toml
schema_version = 2
mode = "rule"               # "rule" | "global" | "direct"
global_target = "doge-auto"
service_mode = "systemd-user"  # "systemd-user" | "pid"
mixed_port = 7890
controller_port = 9090
controller_secret = "hex-token-32-chars"
proxy_user = "vpnkit"
proxy_pass = "random-hex"

[[subscriptions]]
name = "doge"
url = "https://doge.example.com/sub?token=..."
user_agent = "ClashforWindows/0.20.39"   # 可选
enabled = true
node_count = 52                          # 缓存，`subs update` 时更新

[[local_node_groups]]
name = "home"
enabled = true

[[local_nodes]]
name = "JP-A"
group = "home"                           # 引用一个 local_node_groups 项
via = "doge-auto"                        # 可选 dialer-proxy 目标
proto = "hysteria2"
server = "jp.example.com"
port = 443
[local_nodes.fields]
password = "..."
up = "100 Mbps"
down = "1000 Mbps"
sni = "..."

[[local_rules]]
type = "DOMAIN-SUFFIX"
payload = "github.com"
target = "🚀 Proxy"
```

vpnkit 在第一次启动时自动迁移 rc.2 stores（没有 `local_node_groups`
区块）到 rc.3+：缺组的话建一个默认 `local` 组，把无组节点全归过去。**不需要**
`vpnkit init --force`。

---

## 延迟测试详解

vpnkit 提供两个入口主动调 mihomo 的 `/proxies/<name>/delay` 和
`/group/<name>/delay`。

| 入口 | 触发 | 范围 |
|---|---|---|
| TUI | Groups tab → focus 右 pane → `t` | 当前选中组所有成员 |
| CLI | `vpnkit test <group>` | 整组成员 |
| CLI | `vpnkit test <group> <node>` | 单节点 |

### 默认参数

| 参数 | 值 | 为什么 |
|---|---|---|
| Test URL | `https://www.gstatic.com/generate_204` | mihomo 标准。204 No Content 几乎零 payload，测的是代理通路而不是网页加载耗时 |
| Timeout | 5000 ms | mihomo `url-test` 默认；测通的节点 100-500 ms 就回来，不会真等满 |
| 并发 | mihomo 内部 fan-out | 组 endpoint 在 mihomo 内部并发，vpnkit 不限流 |

CLI 可以覆盖：`--url https://...` 和 `--timeout-ms 3000`。TUI 用默认（没暴露
设置入口）。

### Group endpoint 解析

mihomo 的 `/group/<name>/delay` 只接 **url-test / fallback / load-balance**
类型。Selector（每个 `🚀 Proxy`、订阅组、本地节点组都是 Selector）返回
`404 Resource not found`。

vpnkit assembler 给每个用户可见组都生成**两个** mihomo 组：Selector `<name>`
+ 配套 url-test `<name>-auto`。所以 `api.MeasureGroup` 走这个 cascade：

1. 试 `/group/<name>-auto/delay` —— 覆盖所有 vpnkit 生成的组（一次往返）
2. 404 后试 `/group/<name>/delay` —— 覆盖用户手写 config.yaml 加的 url-test
3. 又 404 → 读 `/proxies/<name>.all` 列出成员，并发调 `/proxies/<member>/delay`

非 404 错误（`401`、`500`、网络超时）原样返回 —— 那是真问题，不该触发 fallback。

### 超时编码

mihomo 把测试失败（timeout / dial error）编码成 `{"delay": 0}` —— 0 ms 不是
合法 RTT，所以是无歧义的 sentinel。vpnkit：
- **文本输出** 把 0 翻译成 `timeout`
- **JSON 输出** 保留 0 原样。机器消费者自己判定。

### 颜色分级（仅 TUI）

| 范围 | 颜色 | 含义 |
|---|---|---|
| < 200 ms | 绿 (46) | 好 |
| 200–500 | 黄 (214) | 可用 |
| > 500 | 红 (196) | 慢 |
| `timeout`（0） | 红 (196) | 失败 |
| (无测量) | — | 本会话没测过 |

### CLI 输出

文本（默认），按节点名排序：
```
$ vpnkit test doge
  HK-01                     234 ms
  JP-02                     567 ms
  US-03                     timeout
```

JSON:
```bash
$ vpnkit test doge --json
{
  "group": "doge",
  "url": "https://www.gstatic.com/generate_204",
  "timeout_ms": 5000,
  "results": {"HK-01": 234, "JP-02": 567, "US-03": 0}
}
```

单节点：
```bash
$ vpnkit test doge HK-01 --json
{"node": "HK-01", "delay_ms": 234,
 "url": "...", "timeout_ms": 5000}
```

### `vpnkit test` vs `vpnkit nodes`

| | `vpnkit nodes` | `vpnkit test` |
|---|---|---|
| 数据源 | mihomo 自己 url-test 跑出来的 history（缓存） | 主动调 `/group/.../delay` |
| 新鲜度 | 跟上次 url-test 周期相关，可能过期 | 现在 |
| 反映当前网络条件？ | 不 | 是 |

随便看用 `nodes`；切了 wifi / VPN / 怀疑节点挂了用 `test`。

### 持久化

TUI 延迟在 `groupsTab.Model.delayByNode` map 里 —— **不持久化**。重启 TUI
就清空。理由：延迟跟当前网络强相关（wifi vs 4G、时段、BGP 路径），缓存反而
误导。

要持久化的话用 CLI `vpnkit test ... --json` 自己存。

---

## 多用户同机部署

vpnkit 在第一次启动时通过 `internal/portutil` 用 crypto-rand 在 10000–60000
范围内挑空闲 TCP 端口给 `mixed-port` 和 `external-controller`，存到 store。
同机多个用户都能各自跑 vpnkit —— 端口对不重复。

如果存的端口被占（重启后别的工具抢了），vpnkit 启动时自动找下一对空闲端口
并强制重写 `config.yaml`。然后重启 service 用新端口。

systemd-user unit 是 mode 0600 因为 `Environment=` 行里可能有 proxy
basic-auth 凭据，不能让邻居通过 `/etc/systemd/user/` 看到。

---

## 故障排查

### TUI 显示 `❌ mihomo not reachable`

1. `vpnkit status` —— 服务跑没？没跑 → systemd-user 模式 `systemctl --user
   start mihomo`；pid 模式 → 看 `~/.local/state/vpnkit/mihomo.log`
2. controller 端口漂移：`grep controller_port ~/.config/vpnkit/config.toml`
   要跟 `~/.config/mihomo/config.yaml` 里 `external-controller:` 一致。如果
   不一致 vpnkit 已经 reconcile 了但 mihomo 还跑在老端口，restart 一下
3. secret 漂移：同上 —— vpnkit 在 secret 变的时候会 hot-reload 或 restart，
   但如果 mihomo 中途死了就要手动 restart

### `vpnkit subs update <name>` 卡 / 超时

- 测 URL 能不能直接访问：`curl -I -H "User-Agent: ClashforWindows/0.20.39"
  "<sub-url>"` 应该返回 200 + base64 body 或 yaml
- feed URL 本身需要走代理：订阅是 **直接** fetch，不经 mihomo。如果 feed
  本身要代理，启动 vpnkit 的 shell 里 `HTTPS_PROXY=http://127.0.0.1:7890`
  set 一下（先有鸡先有蛋的情况，但能解）

### 国内网络 —— 第一次启动死锁

mihomo bootstrap 要从 GitHub 拉 GeoIP MMDB；用户没配代理时 mihomo 死锁
等下载。vpnkit 两种方式规避：
1. systemd-user unit 注入 `HTTPS_PROXY` / `HTTP_PROXY`，让 mihomo 能用
2. `vpnkit init` 把 GeoIP/GeoSite 文件预下到 `~/.config/mihomo/`

如果撞这个，跑 `vpnkit init --force` 重新预下。

### 本地节点表单：`port must be int`

Port 字段要纯整数。常见错：粘了 `:443` 包含前置冒号、或 `443/udp` 带协议
后缀。改成 `443`。

### 本地节点表单：编辑时重名冲突

编辑节点改名为已存在的名字 → 回滚（原节点重新插回）+ flash `save:
localnodes: duplicate name X`。要么换名，要么先删冲突的。

### 延迟测试 → 全 timeout

- mihomo 没跑（`vpnkit status`）
- controller 端口没监听（`ss -tln | grep <controller_port>`）
- 测试 URL 对代理 egress 不可达 —— 试 `vpnkit test <group> --url
  https://1.1.1.1` 做不需要 GFW 绕过的健康检查
- Auth 配错：`curl -i http://127.0.0.1:<port>/proxies` 应该返回 200 +
  secret 写在 `Authorization: Bearer <token>` header

### `delay <group>: mihomo GET /group/<group>/delay: 404 Resource not found`

rc.6 之前对 vpnkit 生成的组测延迟会撞这个 —— vpnkit 用户可见组都是
mihomo Selector，`/group/<name>/delay` 只支持 url-test/fallback/load-balance。
rc.6 起 [Group endpoint resolution](#group-endpoint-解析) cascade 修了。
升级到 rc.6+ 或从 `main` 重 build。

### "TUI tab 没显示我在别的 shell `vpnkit subs add` 加的订阅"

TUI 不监听 store 文件被外部进程改。重开 TUI，或者用 TUI 的 Sources tab
自己的表单做 CRUD（这样能 in-place refresh）。

---

## 设计取舍

- **为什么用 mihomo 不自己 Go 实现 proxy**：mihomo 久经考验、原生支持
  ss/vmess/vless/trojan/hysteria2/tuic 全套、暴露稳定的 controller API。
  vpnkit 包装它而不是重写 6 种协议
- **为什么 store 用 TOML**：注释能 round-trip、手编辑友好、没空白陷阱。
  YAML 留给 mihomo 自己的 config（它的 native 格式）
- **为什么没了 `vpnkit chain` / `vpnkit group` / `vpnkit ext`**：rc.4 把
  dialer-proxy 合到本地节点的 `Via` 字段，自定义组的 builder UI 被移除。
  高级用户仍可在 assemble 之后手编辑 `~/.config/mihomo/config.yaml` ——
  vpnkit 只覆盖安全相关字段（`mixed-port`、`external-controller`、`secret`、
  `authentication`、`bind-address`、`allow-lan`）
- **为什么两套 routing-mode 概念**：vpnkit 的 `mode`（store 里）和 mihomo
  的 `mode:`（config.yaml 里）—— vpnkit 永远给 mihomo 输出 `mode: rule`，
  通过 assembled `rules:` 列表模拟 `global` / `direct`。这样 mihomo 的
  mode flag 永远不是 source of truth，不会漂移
- **为什么 store 是单文件**：备份方便、lazy migration 简单、tmp+rename
  原子重写。锁只到进程粒度（vpnkit 本身不会并发跑自己）
