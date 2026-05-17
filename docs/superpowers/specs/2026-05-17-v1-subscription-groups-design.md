# v1.0.0-rc.1 设计：订阅组管理 + 本地节点 + 本地规则

**日期**：2026-05-17
**目标版本**：v1.0.0-rc.1（预发布）
**作者**：Jim + Claude (brainstorming session)

---

## 1. 背景与目标

当前 v0.10.x 是"单激活订阅"模型——store 里可有多个 profile，但只有 `ActiveProfile` 一份真正生成 `config.yaml`，切换 profile 等于整体替换。

v1.0.0 改为 Shadowrocket-like **多源共存**：
- 多个订阅组**同时存在**，节点可跨组选
- 一类"本地节点"组，用户手动表单/URI 录入
- 规则分三层：本地规则（最高优）→ 各订阅自带规则 → MATCH 兜底走顶层 Global Target
- 顶层两个旋钮：Mode（rule/global/direct）+ Global Target（任一组或节点）

破坏性变更：v0.10.x 的 `active_profile` / `profiles[]` / `rule_template` 字段被替换，老 store.toml 不兼容，启动时 fatal 提示 `vpnkit init --force`。

---

## 2. 数据模型（store.toml schema v2）

```toml
schema_version = 2

# 未变：controller_secret, controller_port, mixed_port,
#       proxy_user, proxy_pass, ui_theme, service_mode
# 删除：active_profile, profiles[], rule_template

mode = "rule"                       # rule | global | direct
global_target = "doge-auto"         # 任一 group/node name；assembler 用作顶层 select 默认当选项

[[subscriptions]]
name = "doge"
url = "https://..."
user_agent = ""
enabled = true
last_updated = 2026-05-17T13:50:57Z
node_count = 30                     # cached, refreshed on update

[[local_nodes]]
name = "HK-Manual"
proto = "hysteria2"                 # ss | vmess | vless | trojan | hysteria2 | tuic
server = "1.2.3.4"
port = 443
# proto-specific 字段以 inline TOML 表保存，避免 schema 锁死：
fields = { password = "...", up = "100 Mbps", down = "200 Mbps", sni = "example.com" }

[[local_rules]]
type = "DOMAIN-SUFFIX"
payload = "baidu.com"
target = "🎯 Direct"
# 优先级按数组下标，第一条最高优
```

**关键决策**：
- 节点全名空间用 `<group>:<node-name>` 防跨组冲突；TUI 显示时 strip 前缀
- `extensions.toml`（chains + custom groups）作为高阶 overlay 不变
- store 检测 v1 → fatal 退出，提示 `vpnkit init --force`（会先备份旧文件再重生）

---

## 3. 模块拆分

```
internal/
├── groups/           NEW   — Group 抽象（Subscription / LocalNode 两类）
├── localnodes/       NEW   — 本地节点 CRUD + URI parse（6 协议）
├── localrules/       NEW   — 本地规则 CRUD + 序列化为 mihomo rule line
├── assembler/        NEW   — 多源 → 一份 config.yaml（替代 subscription/assemble.go 顶层逻辑）
│
├── subscription/     CHANGED — 保留 fetch/convert/detect/proto；删 assemble.go 顶层路径
├── store/            CHANGED — schema v2 + Load 检测 v1 fatal
├── profiles/         REMOVED — 整个包删
└── extensions/       UNCHANGED
```

### 3.1 `groups/`

```go
type Kind int
const (
    KindSubscription Kind = iota + 1
    KindLocalNodes
)

type Group interface {
    Name() string
    Kind() Kind
    Enabled() bool
    Proxies() []proto.Proxy
    Rules() []localrules.Rule  // 订阅自带 rules；本地组返回 nil
}
```

具体实现：`SubscriptionGroup`（包装 fetch+convert 结果）和 `LocalNodesGroup`（包装 localnodes.Manager）。

### 3.2 `localnodes/`

```go
type Node struct {
    Name   string
    Proto  string             // ss/vmess/vless/trojan/hysteria2/tuic
    Server string
    Port   int
    Fields map[string]any     // proto-specific
}

type Manager struct{ ... }    // CRUD + 持久化到 store.toml

func ParseURI(uri string) (Node, error)
```

URI parse 支持：`ss://`, `vmess://`, `vless://`, `trojan://`, `hysteria2://`, `tuic://`。

**字段集对齐 mihomo 文档**：`Fields` 接受的 key 集合直接复用 `internal/subscription/proto/*.go` 里各协议已定义的字段（例如 hysteria2 的 `password`/`up`/`down`/`sni`/`obfs`/`skip-cert-verify`，trojan 的 `password`/`sni`/`alpn`，vmess 的 `uuid`/`alterId`/`cipher`/`network`/`ws-opts` 等）。本期不引入新字段，只暴露已有 proto package 解析过的子集。

### 3.3 `localrules/`

```go
type Rule struct {
    Type    string  // DOMAIN-SUFFIX, DOMAIN-KEYWORD, IP-CIDR, RULE-SET, GEOIP, MATCH, ...
    Payload string
    Target  string  // group/node name 或 🎯 Direct / 🛑 Reject
}

type Manager struct{ ... }    // CRUD + reorder
func (r Rule) Render() string  // "DOMAIN-SUFFIX,baidu.com,🎯 Direct"
```

类型白名单按 mihomo 文档枚举（30+ 种）。

### 3.4 `assembler/`

```go
type Input struct {
    Mode             Mode                    // rule | global | direct
    GlobalTarget     string                  // group/node name
    Subscriptions    []groups.Group          // 已 fetch+convert
    LocalNodes       groups.Group            // 单一组（可能为空）
    LocalRules       []localrules.Rule
    Extensions       extensions.Extensions
    MixedPort        int
    ControllerPort   int
    ControllerSecret string
    ProxyUser        string
    ProxyPass        string
}

func Assemble(in Input) ([]byte, error)
```

合并规则见 §5。

---

## 4. CLI（ctl）兼容性

### 4.1 现有命令变更

| 命令 | 状态 | 说明 |
|---|---|---|
| `vpnkit`（TUI） | 改 | 新 Groups / Sources / Local Rules tab |
| `vpnkit init` | 改 | schema v2 重写；新增 `--force` 检测 v1 时备份 |
| `vpnkit update` | 不变 | binary 自更新独立 |
| `vpnkit uninstall` | 不变 | 文件清理 |
| `vpnkit status` | 改 | 输出 subscriptions 数 + local nodes 数 + mode + global_target |
| `vpnkit ip` | 不变 | 走 mihomo controller |
| `vpnkit mode rule\|global\|direct` | 改 | 写 store.Cfg.Mode + 重生成 config.yaml + reload |
| `vpnkit groups` / `nodes` / `use` | 不变 | 走 mihomo controller |
| `vpnkit chain / group / ext` | 不变 | extensions overlay |

### 4.2 新增命令

```bash
vpnkit subs list
vpnkit subs add <name> <url> [--ua=...]
vpnkit subs rm <name>
vpnkit subs enable <name>
vpnkit subs disable <name>
vpnkit subs update [<name>]

vpnkit local-nodes list
vpnkit local-nodes add <uri>
vpnkit local-nodes add-form     # 进 TUI form
vpnkit local-nodes rm <name>
vpnkit local-nodes edit <name> <key=val>...

vpnkit local-rules list
vpnkit local-rules add <type> <payload> <target>
vpnkit local-rules rm <idx>
vpnkit local-rules move <idx> <new-idx>

vpnkit target <group-or-node>   # 设 global_target
vpnkit target                    # show
```

`--json` 输出：新增字段，不删旧字段（旧字段以空数组兼容老脚本）。

---

## 5. Assembler 算法 + 输出样板

### 5.1 输入示例

```
Subscriptions:
  doge      [enabled, 3 nodes, has-rules]
  boost-net [enabled, 2 nodes, no-rules]
LocalNodes:
  HK-Manual (hysteria2)
LocalRules:
  1. DOMAIN-SUFFIX,baidu.com,🎯 Direct
  2. DOMAIN-KEYWORD,internal,🎯 Direct
Mode: rule
GlobalTarget: doge-auto
```

### 5.2 输出 config.yaml

```yaml
mixed-port: 50595
external-controller: 127.0.0.1:32645
secret: <hex>
mode: rule
authentication: [vpnkit-xxxx:yyyy]
bind-address: 127.0.0.1
allow-lan: false

proxies:
  - {name: "doge:HK-A",   type: ss,        server: ...}
  - {name: "doge:JP-B",   type: vmess,     server: ...}
  - {name: "doge:US-C",   type: trojan,    server: ...}
  - {name: "boost:SG-1",  type: vless,     server: ...}
  - {name: "boost:DE-2",  type: vless,     server: ...}
  - {name: "local:HK-Manual", type: hysteria2, server: ..., up: 100Mbps, down: 200Mbps}

proxy-groups:
  - {name: "doge",        type: select,   proxies: ["doge-auto", "doge:HK-A", "doge:JP-B", "doge:US-C"]}
  - {name: "doge-auto",   type: url-test, proxies: ["doge:HK-A", "doge:JP-B", "doge:US-C"], url: http://www.gstatic.com/generate_204, interval: 300}
  - {name: "boost",       type: select,   proxies: ["boost-auto", "boost:SG-1", "boost:DE-2"]}
  - {name: "boost-auto",  type: url-test, proxies: ["boost:SG-1", "boost:DE-2"], url: http://www.gstatic.com/generate_204, interval: 300}
  - {name: "local",       type: select,   proxies: ["local:HK-Manual", DIRECT]}

  - {name: "🚀 Proxy",    type: select,   proxies: ["doge-auto", "doge", "boost-auto", "boost", "local", DIRECT]}
  - {name: "🎯 Direct",   type: select,   proxies: [DIRECT]}
  - {name: "🛑 Reject",   type: select,   proxies: [REJECT, DIRECT]}

rules:
  # local_rules 最高
  - DOMAIN-SUFFIX,baidu.com,🎯 Direct
  - DOMAIN-KEYWORD,internal,🎯 Direct

  # doge 自带 rules (target 已重写)
  - DOMAIN-SUFFIX,youtube.com,doge
  - DOMAIN-SUFFIX,netflix.com,doge

  # boost-net 无 rules → 跳过

  - MATCH,🚀 Proxy
```

### 5.3 关键决策

1. **命名空间**：所有节点 `<group>:<original-name>`。重名跨组无影响。UI 显示时 strip prefix
2. **订阅自带 rules 重写**：
   - target 是 `🚀 Proxy / 🎯 Direct / 🛑 Reject` → 保留
   - target 是订阅内部 group 名 → 重写成 `<group-name>`（整组）
   - target 是订阅内部 node 名 → 重写成 `<group>:<node>`
3. **GlobalTarget 写入**：`🚀 Proxy` 这个 select 的 proxies 数组第一项 = `GlobalTarget`（mihomo select 取第一项为默认）
4. **mode=global**：rules 段写单一 `MATCH,🚀 Proxy`，**mihomo 的 `mode` 字段始终保持 `rule`**（vpnkit 的 mode 仅是用户视角概念，全靠改写 rules 段实现，避免 mihomo 在不同 mode 下行为不一致）
5. **mode=direct**：rules 段写单一 `MATCH,🎯 Direct`，同样不动 mihomo mode 字段
6. **空订阅 group**：disabled 或 fetch 失败 → 整组不 emit，store 元数据保留
7. **url-test 默认 URL**：`http://www.gstatic.com/generate_204`，interval 300s
8. **错误路径**：
   - 节点 URI parse 失败 → 不 emit，TUI 红色标注
   - 订阅 fetch 失败 → 上次 cached yaml 仍用，TUI 黄色 `(last update X minutes ago, fetch failed)`
   - assembler 写 config.yaml 失败 → atomic write，错误浮现到 TUI status bar

---

## 6. TUI 结构

主菜单 7 个 tab：

```
[1] 🏠 Dashboard
[2] 🌐 Groups
[3] 📚 Sources
[4] 📜 Rules
[5] 🔗 Connections
[6] 📓 Logs
[7] ⚙  Settings
```

### 6.1 Groups（主操作面）

左侧 list 所有 group（订阅 + 本地）。右侧展开当前 group 节点列表 + 节点 detail。`[Enter]` 切节点（走 mihomo controller PUT /proxies），`[t]` 单节点延迟测试，`[e]` 编辑（仅本地节点）。

### 6.2 Sources（配置面，两个 sub-page）

- **Subscriptions**：列表 + add/del/update/toggle enable
- **Local Nodes**：列表 + add（URI 或 form）/ del / edit form。Form 字段按 proto 动态显示（hy2/tuic 出现 up/down 字段）。

### 6.3 Rules（两个 sub-page）

- **Local Rules**：CRUD + reorder
- **Live (mihomo view)**：现有 Rules tab 实时数据迁过来

### 6.4 Settings（多一个 Routing sub-page）

```
Settings sub-pages:
  Routing       ← 新加：Mode + Global Target
  Extensions    ← 现有
  Update        ← 现有
  Reset         ← 现有
```

Routing 内容：Mode 三选一 + Global Target select（候选 = 所有 group + 所有 node）。

### 6.5 删除的 tab

- 旧 Proxies tab：合并进 Groups（看 mihomo 视角的延迟测试等还在）
- 旧 Profiles tab：变成 Sources tab 的 Subscriptions sub-page

---

## 7. 测试策略

### 7.1 包级 unit test

| 包 | 关键 test |
|---|---|
| `localnodes/` | URI parse 6 协议（含 ss-2022 / vmess base64 / hy2 query / vless reality / tuic）；CRUD + dup name；hy2/tuic up/down 往返序列化 |
| `localrules/` | 类型白名单 table-driven；reorder；Render 字符串与 mihomo 语法对齐；非法 target 拒绝 |
| `groups/` | Group 接口契约；Disabled() 跳过 emit |
| `assembler/` | 8 个 golden file：仅订阅 / 仅本地 / 混合 / 订阅 rules 重写 / mode=global / mode=direct / fetch 失败 cached / 节点重名跨组 |
| `store/` | schema v2 round-trip；v1 → fatal；空 store 合法 |
| `cmd/vpnkit/cmd_subs_*` | subs add/rm/enable/disable/update；golden output |
| `cmd/vpnkit/cmd_local_nodes_*` | URI add；form add；rm；edit；JSON |
| `cmd/vpnkit/cmd_local_rules_*` | add/rm/move；列表带 idx |
| `cmd/vpnkit/cmd_target_*` | target set/show；指向不存在的组报错 |

### 7.2 集成 test

```
internal/app/integration_test.go (新):
  TestE2EBootstrapWithMultiSubs:
    1. httptest.Server 仿造 doge + boost subscription endpoints
    2. tempdir 当 HOME，跑 cmd_init
    3. CLI: subs add * 2 + local-nodes add + local-rules add
    4. CLI: subs update
    5. 校验 config.yaml 包含三组 proxy-group + rules 顺序
    6. 校验 mihomo 启动成功（fake binary stub）
```

### 7.3 覆盖率门槛

- 新包（groups/localnodes/localrules/assembler）**≥ 85%**
- 改包（store/cmd/subscription）**≥ 80%**
- TUI 包不要求（bubbletea View 难测）

### 7.4 回归保护

- 保留 v0.10.x 所有现有 *_test.go
- 老 schema 测试改成 "expect fatal"
- extensions/ 测试不动
- 加 integration_test.go 在 CI matrix 跑 `-race`

---

## 8. Release 拆步

9 个可独立 commit 的工作单元。每单元跑通 `go vet ./... && go test ./... -race` 才进下一单元。

| # | 单元 | 关键交付 | 阻塞下游 |
|---|---|---|---|
| 1 | **store schema v2** | `Config` 重写、`Load` 检测 v1 fatal、`cmd init --force` 备份重生 | 所有 |
| 2 | **localnodes 包** | `Node` + `Manager` + 6 协议 URI parse + 单测 ≥85% | 3, 6, 8 |
| 3 | **localrules 包** | `Rule` + `Manager` + `Render` + 单测 ≥85% | 6, 8 |
| 4 | **groups 包** | `Group` 接口 + 两实现 + 单测 | 6 |
| 5 | **assembler 包** | `Assemble(Input)` + 8 golden file 测试 | 6 |
| 6 | **app/run.go 接管** | 删 profiles 包、删 subscription/assemble 顶层；新 pipeline；整库 build 绿 | 7, 8, 9 |
| 7 | **CLI 子命令** | subs/local-nodes/local-rules/target 全套；改 status/mode/init | 9 |
| 8 | **TUI 重构** | Groups/Sources/Rules-local/Settings-routing 4 个新面 | 9 |
| 9 | **release 收尾** | README/docs/CHANGELOG + migration note + tag v1.0.0-rc.1 push | — |

依赖图：

```
#1 ────► #2 ─┐
        #3 ─┤
        #4 ─┴► #5 ────► #6 ─┬─► #7 ─┬─► #9
                            └─► #8 ─┘
```

`#2/#3/#4` 互不依赖，可并行（worktree 或 subagent-driven）。
`#7/#8` 在 #6 之后可并行。
`#9` 收尾，必须 #7 + #8 都完成。

---

## 9. 显式不做

- 节点限速 / token-bucket throttling（mihomo 不支持，用户已确认 hy2/tuic 的 up/down 是协议 hint 即可）
- 节点 Speed Test（实测带宽）—— 留到后续版本，本期只暴露 hy2/tuic up/down 字段
- mihomo proxy-providers 原生路线（评估为方案 A，已舍弃）
- 任何 GeoIP 自带规则改动（保持 v0.10.2 行为）
- mirror_wrap 复活（v0.10.0 主动删除，不回退）
