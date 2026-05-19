# 升级 vpnkit 到 v1.0.0

> 英文版 → [UPGRADE-v1.md](UPGRADE-v1.md)

> **v1.0.0-rc.1 是预发布版**，所有功能已经齐了。新安装可视为 production-ready；
> 现有 v0.10.x 用户需要走迁移流程。

## 新功能

- **多订阅源** —— 多个订阅并存，所有订阅的节点都可选
- **本地节点** —— 手动录入节点（ss / vmess / trojan / vless / hysteria2 / tuic）
  跟订阅节点同等地位
- **本地规则** —— CLI 和 TUI 都支持结构化 CRUD
- **路由开关** —— 显式 `mode`（rule / global / direct）+ `global_target`
- **新 TUI 布局** —— Groups + Sources (Subscriptions / Local Nodes) + Local Rules + Routing 等 tab

## 破坏性变更：store schema v1 → v2

v0.10.x 单 active profile (`active_profile` + `[[profiles]]`)。v1.0.0 换成
`[[subscriptions]]`, `[[local_nodes]]`, `[[local_rules]]`, `mode`, `global_target`。
旧 store 文件**不会**自动迁移。

第一次用 v1.0.0 启动时，旧 store 会报致命错：

```
store at ~/.config/vpnkit/config.toml uses schema v1 (vpnkit ≤ v0.10.x);
v1.0.0 changed the data model. Back up the file, then run
`vpnkit init --force` to regenerate
```

## 迁移步骤

1. **备份订阅**（可选 —— `init --force` 也会自动 .bak）：

   ```bash
   cp ~/.config/vpnkit/config.toml ~/vpnkit-v0.toml.bak
   ```

2. **升级二进制**：

   ```bash
   vpnkit update          # v0.9+ 可以自动下 v1.0.0-rc.1
   # 或者重新跑 install.sh
   ```

3. **重建 store**：

   ```bash
   vpnkit init --force
   # ↳ 旧 config 改名为 ~/.config/vpnkit/config.toml.bak.<timestamp>
   #   生成全新的 schema v2 文件
   ```

4. **重新添加每个订阅**：

   ```bash
   vpnkit subs add doge       https://example.invalid/sub/doge
   vpnkit subs add boost-net  https://example.invalid/sub/boost
   vpnkit subs update
   ```

5. **（可选）本地节点** 加手动管理的节点：

   ```bash
   vpnkit local-nodes add 'hysteria2://password@1.2.3.4:443?up=100&down=200#HK-manual'
   ```

6. **（可选）本地规则** 覆盖订阅规则：

   ```bash
   vpnkit local-rules add DOMAIN-SUFFIX baidu.com '🎯 Direct'
   vpnkit local-rules add DOMAIN-KEYWORD internal '🎯 Direct'
   ```

7. **选定路由目标**：

   ```bash
   vpnkit target doge-auto   # 把 MATCH 兜底转到 doge 的 url-test 组
   ```

8. **重启 mihomo**：

   ```bash
   systemctl --user restart mihomo.service
   ```

## 新的 CLI 接口

```
vpnkit subs         list | add <name> <url> | rm <name> | enable <name> | disable <name> | update [<name>]
vpnkit local-groups list | add <name> | rm <name> | enable <name> | disable <name> | rename <old> <new>
vpnkit local-nodes  list | add <uri>  | rm <name> | edit <name> <key=val>...        | mv <name> <new-group>
vpnkit local-rules  list | add <type> <payload> <target> | rm <idx> | move <from> <to>
vpnkit active       [<name>]              # 查看 / 切 active source（订阅 OR 本地组）
vpnkit target       [<member>]            # 进阶：覆盖 🚀 Proxy 默认成员
vpnkit mode         rule | global | direct
vpnkit --help / -h / help                 # 顶层 + 每个子命令的 usage
```

`vpnkit status` 现在打印订阅数 / 本地节点数 / mode / **active source** /
global target。Mutation 命令拒绝 `--json` 并报清楚错误，只读命令接受。

### 自动迁移到 rc.7 active-source 模型

`store.Load` 自动升级老 store —— 用户什么都不用做：

| 老字段值 | 新行为 |
|---|---|
| `global_target = "<name>-auto"` | 派生 `active_source = "<name>"` |
| `global_target = "DIRECT"` 且有 ≥1 enabled source | 两个字段都 bump 到第一个 source |
| `global_target = "🚀 Proxy"`（rc.5- 自循环） | 改写后再 bump（同上） |

升级后用 `vpnkit active <name>` 或新的 Settings → Active Source 子页选别的。

## 底层改了什么

- `internal/profiles/` 没了 —— 替换为 `internal/app.Pipeline`
- `internal/subscription/assemble.go` 没了 —— 替换为 `internal/assembler/`
- 4 个新的叶子包：`groups/`, `localnodes/`, `localrules/`, `assembler/`
- Schema v2 在 `internal/store/store.go`；v1 字段保留为 `Legacy*` 别名，仅用于检测旧 store

## 报告问题

迁移过程出问题，把 `~/.config/vpnkit/config.toml.bak.*` 和 `vpnkit status`
输出附在 issue 里。v1.0.0-rc.1 是第一个用这套架构的版本，反馈 rough edges
现在最有用。
