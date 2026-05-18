# 节点延迟测试

vpnkit 提供两条入口主动触发对节点 / 节点组的连通性与延迟测量，对接 mihomo
controller 的 `/proxies/<name>/delay` 与 `/group/<name>/delay`。

## TL;DR

| 入口 | 命令 / 按键 | 范围 |
|---|---|---|
| TUI | Groups tab → 进入右面板 → `t` | 当前 selected group 的全部成员 |
| CLI | `vpnkit test <group>` | 整组测试，按名字母序输出 |
| CLI | `vpnkit test <group> <node>` | 单节点测试 |

## 工作机制

### mihomo 端

mihomo 接到 `/proxies/<name>/delay?url=<url>&timeout=<ms>` 时：

1. 通过该 proxy 拨号到 `url`
2. 收到 HTTP 响应后停表
3. 返回 `{"delay": <ms>}` JSON

`/group/<name>/delay` 是同一逻辑的并发批处理：对组里所有 select-eligible
成员同时跑一遍，返回 `{<member>: <ms>, ...}`。

**超时编码**：mihomo 用 `delay: 0` 表示超时（拨号失败 / TLS 握手失败 / HTTP
响应超过 timeout）。0 不是合法的 RTT 值，所以可以无歧义地复用为 sentinel。

### vpnkit 默认参数

| 参数 | 值 | 出处 |
|---|---|---|
| Test URL | `https://www.gstatic.com/generate_204` | mihomo 标准 health-check URL，204 响应体几乎为空，测的是代理通路而非上游网页加载耗时 |
| Timeout | 5000 ms | mihomo `url-test` 组默认 |
| Concurrent test (group) | 由 mihomo 决定 | controller 内部并发，vpnkit 不参与控制 |

CLI 两个 flag 可覆盖：

```bash
vpnkit test doge --url https://www.cloudflare.com --timeout-ms 3000
```

TUI 当前用默认值，没有暴露覆盖入口（如果需要再加）。

## TUI 显示约定

Groups tab 右面板每节点行尾追加测得延迟（颜色分级）：

| 延迟 | 颜色 | 含义 |
|---|---|---|
| `< 200 ms` | 绿 | 优 |
| `200–500 ms` | 黄 | 可用 |
| `> 500 ms` | 红 | 慢 |
| `timeout` | 红 | mihomo 返回 0，拨号失败 |
| *(无显示)* | – | 本次会话从未测过该节点 |

延迟只在本次进程的内存里（`groupsTab.Model.delayByNode` map）；
TUI 重启后清空。要持久化得自己跑 CLI 写到文件，或等以后版本加 cache。

## CLI 输出

### 文本（默认）

```
$ vpnkit test doge
  HK-01                     234 ms
  JP-02                     567 ms
  US-03                     timeout
```

### JSON (`--json`)

```bash
$ vpnkit test doge --json
{
  "group": "doge",
  "url": "https://www.gstatic.com/generate_204",
  "timeout_ms": 5000,
  "results": {
    "HK-01": 234,
    "JP-02": 567,
    "US-03": 0
  }
}
```

`results` 的 `0` 表示 timeout — JSON 故意保留原始值，调用方自己决定怎么
呈现给最终用户（CLI 文本路径把它翻译为 `timeout`）。

单节点版：

```json
{
  "node":       "HK-01",
  "delay_ms":   234,
  "url":        "https://www.gstatic.com/generate_204",
  "timeout_ms": 5000
}
```

## 退出码

| 码 | 场景 |
|---|---|
| 0 | 测试完成（即使所有节点都 timeout 也是 0 — 这是合法的测试结果，不是错误） |
| 1 | 用户错（缺参 / group 名不存在 / 参数不合法） |
| 2 | 运行时错（mihomo controller 不可达 / 鉴权失败 / 网络问题） |

## API 端点参考

vpnkit 调用的 mihomo controller endpoint：

```
GET /proxies/<name>/delay?url=<url>&timeout=<ms>
    → {"delay": <ms>}                          0 = timeout

GET /group/<name>/delay?url=<url>&timeout=<ms>
    → {"<member1>": <ms>, "<member2>": <ms>, ...}
```

vpnkit 端封装在 `internal/api/proxies.go` 的 `Client.Delay` /
`Client.GroupDelay`。

## 与 `vpnkit nodes` 的区别

| 命令 | 数据来源 |
|---|---|
| `vpnkit nodes <group>` | `/proxies` snapshot 里的 history 字段 — mihomo 自己 url-test 组定期跑出来缓存的，**只读** |
| `vpnkit test <group>` | 主动调用 `/group/<name>/delay`，**立即测**当前网络情况 |

如果一个节点最近 url-test 还没轮到 / 节点刚被禁用过 / 当前网络环境变了
（VPN 上线下线、Wi-Fi 切到 4G），`vpnkit nodes` 会显示过期数据，
`vpnkit test` 是新鲜数据。

## 故障排查

**全部 `timeout`**

- mihomo 进程没起：`systemctl --user status mihomo`
- mihomo controller 不可达：`vpnkit status` 看 controller port 是否 listen
- 节点本身挂了：换组测一次，看是普遍问题还是局部
- 自己的网络出了：直接 `curl -x http://127.0.0.1:<mixed-port> https://www.gstatic.com/generate_204 -I` 看返回

**`vpnkit test: ... connection refused`**

mihomo controller port 在 store.toml 里 (`controller_port`)，跟实际监听
的端口不一致 — 通常是上次端口冲突后 vpnkit 自动找了新端口但客户端
缓存了旧值。重启 TUI 或重新跑 `vpnkit env` 都会刷新。

**`vpnkit test: 401 Unauthorized`**

controller secret 漂移。`grep secret ~/.config/vpnkit/config.toml` 和
mihomo 进程内的 secret 应该一致。如不一致 vpnkit 重启会自动 reload
config，把 store 的 secret 推进 mihomo。

## 设计取舍

- **为什么默认 5 秒超时**：mihomo url-test 默认 5 秒，对齐避免用户认知
  分裂；网络好的节点 234 ms 就响应，不会真等满。
- **为什么 timeout 用 0 不用 -1 / null**：跟 mihomo wire format 一致，
  避免转换出错。代价是 JSON 调用方要知道 0 = timeout 这个约定，文档
  写明了。
- **为什么 TUI 用本地内存 map 不持久化**：节点延迟随网络环境变化大
  （wifi → 4G、跨时区漫游），缓存反而误导用户。要持久化的话以后加
  `vpnkit history` 一类命令更合适。
- **为什么不并发限速**：mihomo controller 已经在服务端做了并发拨号
  控制，vpnkit 这一层重复限制只会让结果更慢。
