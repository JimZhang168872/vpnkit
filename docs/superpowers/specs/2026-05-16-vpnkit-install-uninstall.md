# vpnkit install/uninstall 完整化 + UI 修复 + emoji 美化

**Date**: 2026-05-16
**Status**: APPROVED
**Target release**: v0.8.0

## 背景

v0.7.x 已经把多用户安全（端口避让、proxy auth、secret drift 自动 restart）落地，但
安装/卸载体验仍有几个问题：

1. `install.sh` 只下载二进制，**不创建配置骨架**。用户第一次跑 `vpnkit` 才走
   bootstrap 生成 `~/.config/vpnkit/config.toml` + `~/.config/mihomo/config.yaml`。
   这意味着在 install 阶段无法对旧环境做体检/迁移决策——所有这些必须延迟到
   `vpnkit` 首启动。
2. **没有 `vpnkit uninstall` 子命令**。`Settings → Service → u` 只删 systemd
   unit，留下二进制、~/.config/、~/.cache/、~/.local/state/ 一堆残留。卸载体验差，
   也无法配合 install.sh 做"先卸载再重装"。
3. **Proxies tab 选不了具体节点**。cursor 模型只在 group 行移动，Enter 仅
   expand/collapse，无法把光标移到展开后的某一节点直接 Enter 切换。
4. 界面/输出**缺乏视觉提示**。状态、错误、tab 名、日志全是纯文本，对新用户
   不友好。

## 目标

让一个全新 Linux 用户**`curl … install.sh | bash`** 一行后立刻能跑（含安全配置
骨架），让升级用户**先安全卸载再装**且**保留订阅**，让 TUI 操作直觉化（节点直
选），让 install/uninstall/CLI 输出有适度的 emoji 区分状态。

## 非目标

- TUN 模式（不变）
- Windows / macOS 支持（不变）
- 订阅协议扩展（不变）

---

## 设计

### 1. install.sh 重写

新流程（约 150 行 bash）：

```
┌─ parse env: VERSION / INSTALL_DIR / INSTALL_TAKEOVER / INSTALL_FORCE
├─ detect arch (amd64/arm64) — keep current
├─ resolve VERSION (latest if unset) — keep current
├─ if $DEST/vpnkit exists:
│   ├─ same version + !INSTALL_FORCE → 打印「already installed at $VERSION」+ exit 0
│   └─ different version:
│       ├─ has profiles in ~/.config/vpnkit/config.toml? → 备份到 /tmp/vpnkit-profiles-<ts>.toml
│       ├─ $DEST/vpnkit uninstall --yes --keep-profiles
│       └─ continue
├─ if ~/.config/mihomo/config.yaml 存在 but ~/.config/vpnkit/config.toml 不存在:
│   ├─ 提示「检测到非 vpnkit 的 mihomo 配置在 ~/.config/mihomo/」
│   └─ 除非 INSTALL_TAKEOVER=1，否则 exit 1
├─ download tarball + verify SHA256 (keep current)
├─ install $tmp/vpnkit → $DEST/vpnkit
├─ $DEST/vpnkit init --non-interactive  # 写 toml + yaml 骨架（保留备份的 profiles）
└─ 打印「🎉 vpnkit $VERSION installed」+ 后续指引
```

日志输出加 emoji：`⬇️ downloading`, `✅ verified`, `🧹 removing old install`,
`📦 installing`, `🛠️  initializing config`, `🎉 done`, `⚠️ warning`, `❌ error`。

### 2. `vpnkit init [--non-interactive]`

新子命令。功能：

- 若 `~/.config/vpnkit/config.toml` 不存在 → 调 `store.Load`（已经会生成默认值并 Save）
- 若 `~/.config/mihomo/config.yaml` 不存在 → 调 `config.BuildSkeleton`（带 store 的
  端口/secret/proxy_user/pass）写入
- 若有备份 `/tmp/vpnkit-profiles-<ts>.toml`（最新的） → 合并 profiles 段进
  `~/.config/vpnkit/config.toml`
- `--non-interactive`：不打印交互提示，只打 emoji 进度行

输出：
```
🛠️  vpnkit init
✅ ~/.config/vpnkit/config.toml (created)
✅ ~/.config/mihomo/config.yaml (created)
📋 restored 1 profile from /tmp/vpnkit-profiles-20260516.toml
🎉 ready — run `vpnkit` to start
```

### 3. `vpnkit uninstall [--yes] [--purge] [--keep-profiles]`

新子命令。默认交互式确认：

```
$ vpnkit uninstall
🗑  uninstall plan:
  - stop mihomo service
  - remove ~/.config/systemd/user/mihomo.service
  - remove ~/.config/mihomo/  (config + cache.db + ruleset/)
  - remove ~/.config/vpnkit/  (secrets, ports, profiles)
  - remove ~/.local/state/vpnkit/  (logs, PID)
  - remove ~/.cache/vpnkit/  (mihomo archives)
  - remove ~/.local/bin/vpnkit
  - remove ~/.local/bin/mihomo  [skip with --keep-mihomo]

📦 profiles will be backed up to /tmp/vpnkit-profiles-20260516-201523.toml

continue? [y/N]:
```

flags:
- `--yes`：跳过 prompt
- `--purge`：把 profiles 也一起删，不做备份
- `--keep-profiles`：（默认行为）单独备份 profiles 到 /tmp/
- `--keep-mihomo`：保留 ~/.local/bin/mihomo 二进制（用户可能还想用）

顺序（每步打印进度，用 emoji）：
1. `🛑 stopping mihomo service` → `svc.Stop()`
2. `🧹 uninstalling systemd unit` → `svc.Uninstall()`
3. `📦 backed up profiles → <path>` （除非 --purge）
4. `🗑  removing ~/.config/mihomo/`
5. `🗑  removing ~/.config/vpnkit/`
6. `🗑  removing ~/.local/state/vpnkit/`
7. `🗑  removing ~/.cache/vpnkit/`
8. `🗑  removing ~/.local/bin/vpnkit`
9. `🗑  removing ~/.local/bin/mihomo`（除非 --keep-mihomo）
10. `🎉 uninstalled`

实现：放在 `cmd/vpnkit/cmd_uninstall.go`，复用 `service.Manager.Uninstall`。

### 4. Proxies tab 节点级 cursor

新模型：

```go
type cursorPos struct {
    groupIdx int  // index into m.order
    nodeIdx  int  // -1 = on group row; >=0 = on node row of expanded group
}
```

行为：

| 当前位置 | 按键 | 新位置 |
|---|---|---|
| group, collapsed | `↓` | next group (collapsed view) |
| group, expanded | `↓` | first node of this group |
| node N (not last) | `↓` | node N+1 |
| node N (last) | `↓` | next group |
| group, ? | `↑` | prev group's last node if prev expanded else prev group |
| Enter on group, collapsed | | expand |
| Enter on group, expanded | | collapse |
| Enter on node | | call `client.SelectProxy(group, node)` |

View 中高亮当前光标行（不管是 group 还是 node）。

测试覆盖：collapsed/expanded 下的导航、Enter 行为分发、API 客户端调用。

### 5. emoji 美化全栈

#### 5.1 TUI

- **Tab 名**：sidebar 显示 `🏠 Dashboard` / `🚀 Proxies` / `📋 Profiles` /
  `🔗 Connections` / `📜 Rules` / `⚙️  Settings`
- **状态栏**：mihomo running → `🟢 running`；stopped → `🔴 stopped`；
  bootstrapping → `🟡 bootstrapping`
- **错误信息**：`❌ <err>`；warning → `⚠️ <warn>`；info flash → `ℹ️ <msg>`
- **Profile 列表**：active 标记从 `★` 改成 `⭐`，未激活留空
- **Proxies cursor 指示**：当前行前缀 `▶` 改成 `👉`（可选）

#### 5.2 install.sh / uninstall

如 §1 §3 所述。

#### 5.3 CLI

`vpnkit status` 输出：

```
🟢 mihomo running   pid 12345
🔧 mode    rule
🚪 ports   mixed=7890   controller=9090
📋 active  airport-A
🚀 groups  🚀 Proxy → HK-01 (45ms)
```

`vpnkit ip`：

```
🌍 ip       203.0.113.42
🏳️ country  HK
🏙 city     Hong Kong
🏢 org      AS12345
🛤 via      🚀 Proxy → HK-01
```

`vpnkit env`：保留原样（输出供 eval 用，emoji 会破坏 shell parsing）。

#### 5.4 默认 proxy-groups

继续用现有 `🚀/🎯/🛑/♻️`，**不扩展**（避免和用户 patch.yaml 冲突）。

---

## 数据流 / 模块边界

```
install.sh ────► vpnkit binary
                     │
                     ├── init  ──► store.Load + config.BuildSkeleton + restore profiles
                     ├── uninstall ──► service.Uninstall + os.RemoveAll
                     ├── (default) ──► app.Run TUI
                     └── status / ip / mode / ... ──► api.Client → mihomo
```

不引入新依赖，复用 `internal/store`、`internal/config`、`internal/service`。

## 错误处理

- install.sh：每个 step `set -e`，失败打印 `❌` 并 exit。SHA256 不匹配立即停。
- `vpnkit init`：每个文件单独 try，失败打印路径 + 错误，但不删已写的部分；返回
  非零退出码。
- `vpnkit uninstall`：每步 best-effort，失败打印 `⚠️ failed to remove X: err`
  继续，最后汇总。
- Proxies tab Enter on node：`SelectProxy` 失败 → 状态栏 `❌ switch failed: err`。

## 测试

- `cmd/vpnkit/cmd_init_test.go`：fresh tmpdir → init → 检查 config.toml + config.yaml
  生成；带备份文件 → 验证 profiles 被合并
- `cmd/vpnkit/cmd_uninstall_test.go`：mock paths/svc，验证调用顺序 + --yes/--purge/
  --keep-profiles 行为分支；profiles 备份内容正确
- `internal/api/proxies_test.go`：扩展 SelectProxy 测试（mock HTTP server）
- `internal/tabs/proxies/proxies_test.go`：cursor 导航表驱动测试；Enter 分发测试
- `install.sh`：手动验证流程（CI 跑 shellcheck 即可）

## 兼容性

- 既有用户跑 `vpnkit`：行为不变。新的 `init`/`uninstall` 子命令是新增。
- 既有 install.sh 调用者：新 install.sh **行为变化**——会调 `uninstall + init`。
  需要在 release notes 明确说明。
- 配置文件格式不变（store.Config 不动 schema）。
