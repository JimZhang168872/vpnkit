# v1.0.0-rc.3 设计：多本地节点组 + Via inline + 6-protocol Form + tmux TUI 测试

**日期**：2026-05-18
**目标版本**：v1.0.0-rc.3（预发布）
**前置**：v1.0.0-rc.2（已 ship）
**作者**：Jim + Claude (brainstorming session)

---

## 1. 背景与目标

rc.2 完成了多源订阅 + 单一 `local` 节点组 + Routing 旋钮的核心模型。三个延续需求：

1. **多本地节点组** — 本地节点跟订阅组完全对称：用户能创建多个组（`home`/`office`/...），每组有 enabled 标记 + 节点列表。
2. **Via inline 编辑** — 创建/编辑本地节点的表单里直接选 Via（链式代理目标），Shadowrocket 风格。Via 是 LocalNode 一等字段，不走 extensions.Chain。
3. **6 协议完整 Form** — Proto select 驱动动态字段，包含 hy2/tuic 的 `up`/`down`（节点限速 QoS）。URI 一键粘贴模式保留。

附带：tmux 驱动的 TUI 集成测试 harness 进项目，把核心 TUI 流程从"人肉跑 tmux + 看截图" 升到自动化。

**兼容性**：schema 仍为 v2，lazy migrate。老 rc.2 store 启动时自动 backfill 一个默认 `local` 组、把无 `Group` 的节点归过去。**不破坏老用户。**

---

## 2. 数据模型（store schema v2 演进）

```toml
schema_version = 2                    # 不变

# NEW: 本地节点组数组 (rc.2 老 store 没有 → Load() lazy backfill)
[[local_node_groups]]
name = "local"                        # 默认组（保留兼容名）
enabled = true

[[local_node_groups]]
name = "home"
enabled = true

[[local_node_groups]]
name = "office"
enabled = false                       # 暂时禁用，不参与 assemble

[[local_nodes]]
name = "HK-manual"
group = "home"                        # NEW: 归属组（空 = "local"）
via = "doge:HK-A"                     # NEW: dialer-proxy 目标（空 = 无 chain）
proto = "hysteria2"
server = "1.2.3.4"
port = 443
fields = { password = "...", up = "100 Mbps", down = "200 Mbps", sni = "example.com" }
```

### Go 结构

```go
type LocalNodeGroup struct {
    Name    string `toml:"name"`
    Enabled bool   `toml:"enabled"`
}

type LocalNode struct {
    Name   string         `toml:"name"`
    Group  string         `toml:"group,omitempty"`   // NEW; "" → "local"
    Via    string         `toml:"via,omitempty"`     // NEW; "" → 无 chain
    Proto  string         `toml:"proto"`
    Server string         `toml:"server"`
    Port   int            `toml:"port"`
    Fields map[string]any `toml:"fields,omitempty"`
}

type Config struct {
    // ... 既有字段不变
    LocalNodeGroups []LocalNodeGroup `toml:"local_node_groups"`
    LocalNodes      []LocalNode      `toml:"local_nodes"`
}
```

### Lazy migrate（在 `store.Load()` zero-value backfill 段加）

```go
// 老 rc.2 store: 有 LocalNodes 但 LocalNodeGroups 缺失 → 自动建 "local" 默认组。
if s.Cfg.LocalNodeGroups == nil {
    s.Cfg.LocalNodeGroups = []LocalNodeGroup{}
    if len(s.Cfg.LocalNodes) > 0 {
        s.Cfg.LocalNodeGroups = []LocalNodeGroup{{Name: "local", Enabled: true}}
    }
    changed = true
}
// 节点无 Group 字段 → 归到默认 "local"（且保证组存在）。
defaultGroup := "local"
hasDefault := false
for _, g := range s.Cfg.LocalNodeGroups {
    if g.Name == defaultGroup {
        hasDefault = true
        break
    }
}
for i, n := range s.Cfg.LocalNodes {
    if n.Group == "" {
        s.Cfg.LocalNodes[i].Group = defaultGroup
        changed = true
    }
}
// 如果有节点归 default 组但组不存在 → 补建
needsDefault := false
for _, n := range s.Cfg.LocalNodes {
    if n.Group == defaultGroup {
        needsDefault = true
        break
    }
}
if needsDefault && !hasDefault {
    s.Cfg.LocalNodeGroups = append(s.Cfg.LocalNodeGroups, LocalNodeGroup{Name: defaultGroup, Enabled: true})
    changed = true
}
```

**Schema 版本不变（仍为 2）**：v2 概念（多源 + 路由旋钮）没变，子结构演进。

---

## 3. Assembler 行为

### 当前（rc.2）

```yaml
# 硬编码 "local" 单组
proxy-groups:
  - {name: "local", type: select, proxies: ["local:HK-manual", DIRECT]}
```

### rc.3 后（多组对称循环）

输入示例：2 sub + 2 local 组 + 1 节点带 Via。

```yaml
proxies:
  - {name: "doge:HK-A",     type: ss,        server: ...}
  - {name: "doge:JP-1",     type: vmess,     server: ...}
  - {name: "boost:SG-1",    type: vless,     server: ...}
  - {name: "home:HK-A",     type: hysteria2, server: ..., password: ..., up: "100 Mbps", down: "200 Mbps",
     dialer-proxy: "doge:JP-1"}                  # ← Via 写为 dialer-proxy 字段
  - {name: "office:WORK-1", type: trojan,    server: ..., password: ...}

proxy-groups:
  # 订阅组（每个一对 select + url-test）
  - {name: "doge",        type: select,   proxies: ["doge-auto", "doge:HK-A", "doge:JP-1"]}
  - {name: "doge-auto",   type: url-test, proxies: ["doge:HK-A", "doge:JP-1"], url: ..., interval: 300}
  - {name: "boost",       type: select,   proxies: ["boost-auto", "boost:SG-1"]}
  - {name: "boost-auto",  type: url-test, proxies: ["boost:SG-1"], interval: 300}

  # 本地组（同样对称一对 select + url-test —— rc.2 只 select，rc.3 加 url-test）
  - {name: "home",        type: select,   proxies: ["home-auto", "home:HK-A"]}
  - {name: "home-auto",   type: url-test, proxies: ["home:HK-A"], interval: 300}
  - {name: "office",      type: select,   proxies: ["office-auto", "office:WORK-1"]}
  - {name: "office-auto", type: url-test, proxies: ["office:WORK-1"], interval: 300}

  # 顶层
  - {name: "🚀 Proxy", type: select, proxies: ["doge-auto", "doge", "boost-auto", "boost", "home-auto", "home", "office-auto", "office", "DIRECT"]}
  - {name: "🎯 Direct", ...}
  - {name: "🛑 Reject", ...}

rules:
  # 本地规则 + 各订阅 rules + MATCH (不变)
```

### 3 个关键决策

1. **本地组也有 url-test**（rc.2 只 select）。跟订阅完全对称。
2. **节点命名空间从 `local:` 改成 `<group>:`**：默认 `local` 组保留原名 `local:HK-manual`，不破坏老用户控制器 state。
3. **Via 写 `dialer-proxy` 字段直接在节点 yaml 里**，不走 extensions.Chain。`extensions.Chain` 仍然在，给订阅节点用。

### 假设

- `Via` 字符串等于 mihomo 视角的 proxy/group 名称（`doge-auto` / `doge:HK-A` / `home:HK-A` / `DIRECT`）。TUI/CLI 表单提供 typeahead 候选避免拼错。
- 防自环：表单不允许 `Via == 节点自己的全名`；assembler 不做二次检测（信任输入层）。
- 禁用组（`enabled=false`）：节点不 emit，对应 proxy-group 也不 emit，但 store 保留。

---

## 4. CLI

### 新动词

```bash
# 本地组管理
vpnkit local-groups list
vpnkit local-groups add <name>                # 创建空组
vpnkit local-groups rm <name>                 # 删空组；非空报错（除非 --force）
vpnkit local-groups enable <name>
vpnkit local-groups disable <name>
vpnkit local-groups rename <old> <new>        # 自动改所有该组节点的 Group 字段

# 本地节点（扩展）
vpnkit local-nodes list                       # 按 group 分段输出
vpnkit local-nodes list --group=home          # 单组
vpnkit local-nodes add <uri> [--group=home] [--via="doge:HK-A"]
vpnkit local-nodes rm <node>                  # node 可写 "HK-manual" 或 "home:HK-manual"
vpnkit local-nodes mv <node> <new-group>      # 跨组移动
vpnkit local-nodes edit <node> key=val ...    # key 含 group/via/proto/server/port/fields.*

# Via 也通过 edit 设置（首选）
vpnkit local-nodes edit HK-manual via="doge-auto"
vpnkit local-nodes edit HK-manual via=""      # 清 Via
```

### 短名 vs 全名

- `<node>` 接受 `HK-manual` 或 `home:HK-manual`
- 跨组重名时短名报错：`vpnkit: ambiguous "HK-A" (in groups: home, office) — use "<group>:<name>"`
- assembler emit / mihomo 视角永远是全名

### 保留不变

- `vpnkit chain ls/set/unset` — 仅给**订阅节点** dialer-proxy chain 用，写 `extensions.toml`。
- `vpnkit subs / local-rules / target / mode` — 不动。
- `vpnkit status` 输出多一行：`📚 sources  N subs + M local groups (K nodes)`。

---

## 5. TUI 重构

### Sources › Local Nodes sub-page

```
○ Local Nodes
─────────────────────────────────────────────────────────
  Group: ▶ [home]    [office]    [+ new group]      ← group tab bar
─────────────────────────────────────────────────────────
  ▶ HK-manual     hysteria2  1.2.3.4:443       via: doge:JP-1
    JP-rented     ss         5.6.7.8:8388

  [a] add node  [d] delete  [e] edit  [u] paste URI
  [N] new group  [D] delete group  [E] rename  [T] toggle enabled
  [←/→] switch group
```

**Group tab bar**：横向显示所有本地组名，当前 group 高亮 `▶`。`←/→` 切换。末尾 `[+ new group]` 占位条目，按 Enter 弹 "Group name:" 单行 form。

**节点列表**：仅显示当前 tab 组内节点，按 `group:name` 字典序。

### Add Node form（proto-driven）

按 Proto 动态出现字段：

```
Add Local Node
─────────────────────────────────────
  Proto:    [hysteria2 ▾]           ← ss/vmess/vless/trojan/hysteria2/tuic
  Group:    [home ▾]                ← typeahead: 当前本地组列表
  Name:     ___________________
  Server:   ___________________
  Port:     _____

  ─── hysteria2 fields ───
  Password:         ___________________
  SNI:              ___________________
  Up:               ___              (int, Mbps)
  Down:             ___              (int, Mbps)
  Obfs:             [none ▾]         (none/salamander)
  Obfs Password:    ___________________  ← 仅 obfs != none 时出现
  Skip-cert-verify: [ ]

  Via (optional):   [— none — ▾]    ← typeahead: 全所有组+节点
  ─────────────────────────────
  [Tab/↑↓] navigate  [Enter] save  [u] URI mode  [Esc] cancel
```

### 字段集（按 proto）

| Proto | 字段 |
|---|---|
| **ss** | Cipher (select)、Password |
| **vmess** | UUID、AlterId (int)、Cipher (select)、Network (select)、Host (ws)、Path (ws)、TLS (bool)、ServerName |
| **vless** | UUID、Network、Flow、TLS (bool)、ServerName、Reality.PublicKey、Reality.ShortID |
| **trojan** | Password、SNI、ALPN (csv)、Skip-cert-verify |
| **hysteria2** | Password、SNI、Up (int)、Down (int)、Obfs (select)、Obfs Password、Skip-cert-verify |
| **tuic** | UUID、Password、SNI、Congestion-controller (select)、UDP-relay-mode (select)、ALPN |

通用字段（每 proto 都有）：Proto select、Group select、Name、Server、Port、Via select。

### URI 一键模式

按 `u` 进 URI 模式：单行输入，解析后回到字段模式（自动填好），用户可改 Group/Via/任意字段再 save。

### Via select 候选构造

`[—none—, doge, doge-auto, doge:HK-A, doge:JP-1, boost, boost-auto, boost:SG-1, home, home-auto, home:HK-A, office, office-auto, office:WORK-1, DIRECT, REJECT]`

- 不含正在编辑的节点自己（防自环）
- typeahead 模糊匹配前缀 + 子串

### Groups tab 也跟着多本地组

之前 Groups tab 显示 `local (3)`。rc.3 起：
```
▶ doge (5)        → doge-auto
  boost (3)       → boost:SG-1
  home (2)        → home-auto
  office (1)      → office:WORK-1
```

每个本地组单独一行。

---

## 6. tmux TUI 集成测试 Harness

### 位置：`test/tui/`

每个 case 独立 tmux session，跑完 kill。Go test 用 `os/exec` 起 tmux + 用 `send-keys` 模拟输入。

### 助手 API（草案）

```go
type isoEnv struct {
    home string
    // XDG_CONFIG_HOME, XDG_STATE_HOME, XDG_CACHE_HOME 也指向 home 子目录
}

func newIsolatedHome(t *testing.T) *isoEnv {
    h := t.TempDir()
    t.Setenv("HOME", h)
    t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
    // ... 其他 XDG
    return &isoEnv{home: h}
}

type tuiSession struct {
    name string  // tmux session name
    iso  *isoEnv
}

func newTUISession(t *testing.T, iso *isoEnv) *tuiSession {
    if _, err := exec.LookPath("tmux"); err != nil {
        t.Skip("tmux not available")
    }
    binary := buildOnce(t)  // sync.Once builds vpnkit binary
    sess := &tuiSession{name: "vpnkit-test-" + randHex(4), iso: iso}
    must(exec.Command("tmux", "new-session", "-d", "-s", sess.name, "-x", "130", "-y", "36",
        sess.envPrefix() + " " + binary).Run())
    time.Sleep(2 * time.Second)  // 等 TUI 渲染
    t.Cleanup(func() { sess.Kill() })
    return sess
}

func (s *tuiSession) SendKeys(keys ...string)     // tmux send-keys
func (s *tuiSession) SendLiteral(text string)     // tmux send-keys -l
func (s *tuiSession) Capture() string             // tmux capture-pane -p
func (s *tuiSession) MustContain(t *testing.T, want string)
func (s *tuiSession) MustNotContain(t *testing.T, want string)
func (s *tuiSession) Kill()
```

### 测试 case（rc.3 范围）

| Case | 验证 |
|---|---|
| `TestTUILaunches` | 启动后 sidebar 列 Dashboard..Settings 7 tab |
| `TestTUILocalNodesAddURI` | URI 粘贴 → 列表显示 → store.toml 写入正确 |
| `TestTUILocalNodesAddViaForm` | proto=hysteria2 → 表单出 Up/Down/SNI → 填 → save |
| `TestTUINewLocalGroup` | `N` 创建 "home" → group tab bar 显示 → 新节点默认归 home |
| `TestTUIViaWritesDialerProxy` | 加节点 Via=doge-auto → 等 assemble → mihomo config.yaml 含 `dialer-proxy` |
| `TestTUISourcesAddSubFormDigits` | 输 URL `https://x:8080/sub?token=12345` 不被切 tab（回归 rc.2 bug） |
| `TestTUIRoutingModeRadioPersists` | Settings › Routing 切 Global → store.toml 写 `mode = "global"` |
| `TestTUIGroupsFocusAndEnter` | → 进右 pane → Enter 调 PutProxy（mihomo 未跑 → flash 含 ❌ 不崩） |

### 框架细节

- **不依赖 mihomo binary**：测试只跑 TUI 本身的逻辑，不验证 mihomo 真启动
- **skip-if-no-tmux**：`exec.LookPath("tmux") != nil` 否则 `t.Skip`，本地无 tmux 仍能跑其他测试
- **输出比较**：`strings.Contains` 容忍渲染抖动，不做像素 diff
- **隔离**：每个 case 用 `t.TempDir()` 单独 HOME，不污染用户真实 `~/.config/vpnkit/`
- **构建一次**：用 `sync.Once + go build` 在第一个 case 把 `/tmp/vpnkit-tui-test` binary 构出来，后续 case 复用

### 集成到 build

- `make test-tui`：单独 target，仅跑 `test/tui/`
- 默认 `make test`：仍跑 `go test ./... -race`，不含 tmux（避免本地无 tmux 时失败）
- `make test-all`：跑所有
- CI：`.github/workflows/ci.yml` matrix 加 `test-tui` job（ubuntu runner 默认有 tmux）

---

## 7. Release 拆步

8 个工作单元。线性依赖，每完成跑 `go vet && go test -race`。

| # | 单元 | 关键交付 | 阻塞下游 |
|---|---|---|---|
| 1 | **store schema 加字段 + lazy migrate** | `LocalNodeGroups[]`、`LocalNode.Group/Via`、`Load()` 自动 backfill 老 store | 所有 |
| 2 | **groups 包多本地组** | `NewLocalNodesGroupForGroup(name, mgr)`；`Pipeline.LocalNodeGroups()` getter | 3 |
| 3 | **assembler 多本地组 emit + via→dialer-proxy** | loop multi local；写 `dialer-proxy`；golden test | 4 |
| 4 | **CLI local-groups + 扩 local-nodes** | 新 dispatcher；`--group/--via/mv`；`<group>:<name>` 引用解析 | 5 |
| 5 | **TUI Sources › Local Nodes 重构** | group tab bar、N/D/E/T、`←/→` 切组 | 6 |
| 6 | **TUI Add Node form (proto-driven)** | Proto select 驱动字段；Group/Via select；保留 URI mode | 7 |
| 7 | **tmux TUI test harness + 8 cases** | `test/tui/` + helper API + Makefile target + CI matrix | 8 |
| 8 | **docs + tag v1.0.0-rc.3** | README 改两段（本地节点 + Via）+ CHANGELOG + push + tag | — |

依赖图：

```
#1 → #2 → #3 → #4 → #5 → #6 ──┐
                              ├─► #7 → #8
                              │
                  (#7 cases 都依赖 #6 form 落地)
```

### 测试覆盖率门槛

- 新代码 ≥80%（CLAUDE.md 工程标准）
- `assembler/` 多组路径必测（golden file）
- TUI 包不计 unit 覆盖率（tmux test 单算）

---

## 8. 显式不做

- 本地节点跨注入到订阅组（你纠正过，不是这意思）
- 订阅节点 inline Via 编辑（订阅节点继续走 `vpnkit chain` CLI；TUI 仅读）
- 节点上贴 tag/label 跨组归属（YAGNI）
- 节点 schema 再升 v3（这次只加字段、lazy migrate，不破坏）
- 节点详情页（detail panel）— Groups tab 当前列表 + 顶部 ●/now 指示器够了
- Add Subscription form 改 proto-driven（订阅是远端 yaml，没必要表单）

---

## 9. 风险与缓解

| 风险 | 缓解 |
|---|---|
| 节点重命名（`local:` → `<group>:`）导致 mihomo controller `now` 状态丢失 | 默认 group="local" 节点保留原名 `local:xxx` 不变；只有用户主动创建新组并 mv 才改名 |
| Via 字段引用的节点被删 → mihomo 启动报错 | assembler 在 emit 前检查 Via 目标存在；不存在则**清空** Via 字段 + stderr 警告（不阻塞 assemble） |
| 表单字段集跟 mihomo 真实需求漂移 | 字段集来源：`internal/subscription/proto/*.go` 已解析过的字段集子集；不引入未在 proto 解析层验证的新字段 |
| tmux test 在不同 terminfo / TERM 下抖动 | 强制 `TERM=xterm-256color`、固定窗口大小 130x36；用 `strings.Contains` 不做像素 diff |
| 老 rc.2 用户 store 已有本地节点 | Load 时 lazy backfill 默认 `local` 组 + 节点 Group="local"；下次 Save 时落盘新字段；老 toml 文件兼容 |
