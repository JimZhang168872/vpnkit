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

vpnkit 完全 user-space 跑 mihomo（仍在维护的 Clash.Meta 内核）— 不需要 root、不依赖 TUN。提供和桌面端相当的节点切换、延迟测试、连接观察、规则管理，但塞得进 SSH 会话。

## 安装

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

自动识别 amd64/arm64、SHA256 校验、装到 `~/.local/bin/vpnkit`，生成默认配置骨架，
升级时先清理旧版再装新版。锁版本用 `VERSION=v0.9.0 ./install.sh`。确认
`~/.local/bin` 在 `PATH` 上。

源码编译：`git clone … && cd vpnkit && make install`（需要 Go 1.22+）。

### 墙内安装

vpnkit 默认让 mihomo 从 `cdn.jsdelivr.net` 拉 geoip/geosite 数据，**国内首次启动通常无需额外配置**。如果 `github.com` 直连也太慢，用公开 GitHub 镜像加速 — 一个环境变量同时覆盖安装脚本**和**后续 mihomo 自己下数据：

```bash
MIRROR="https://ghproxy.com/"           # 选一个当前能用的
VERSION="v0.9.0"                         # pin：大多镜像不代理 api.github.com

curl -sSL "${MIRROR}https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh" \
  | INSTALL_MIRROR="$MIRROR" VERSION="$VERSION" bash
```

挑镜像前先 ping 一下：

```bash
curl -fsSL --max-time 5 -o /dev/null \
  "${MIRROR}https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/README.md" \
  && echo OK || echo "挂了，换一个"
```

备选：`https://mirror.ghproxy.com/`、`https://ghp.ci/`、`https://gh.api.99988866.xyz/`。
`INSTALL_MIRROR` 会持久化到 `~/.config/vpnkit/config.toml` 的 `release_mirror`，
之后所有 GitHub 下载——mihomo 升级、geo 数据更新——都走同一镜像。

## 3 分钟上手

```bash
vpnkit
```

首次启动会下 mihomo、生成 `~/.config/mihomo/config.yaml`、装 systemd unit
并启动、打开 TUI。

加订阅：

1. `3` 进 Profiles → `a` 弹表单
2. 输入名字 + 粘贴订阅 URL → `Enter`
3. `u` → 拉订阅 + 解析 + 重生成 config + reload mihomo

选节点：

1. `2` 进 Proxies → 高亮 `🚀 Proxy` → `t` 对组内所有节点跑延迟测试
2. `Enter` 展开组 → `↓` 移到具体节点 → `Enter` 切到该节点

订阅 URL 支持：Clash YAML 链接、Base64 文本列表、单个协议 URI
（`vmess://`、`hysteria2://`、`trojan://`、`vless://`、`ss://`、`tuic://`）。

### 在终端里走代理

```bash
eval "$(vpnkit env --shell zsh)"        # 或 bash / fish
curl https://www.google.com              # 走 mihomo
eval "$(vpnkit env --unset)"             # 关掉
```

输出同时设小写和大写两组（`http_proxy`、`HTTP_PROXY`…）。Go 程序和只读大写
的库都能识别。同时写一份 `~/.netrc`（权限 0600）让 curl/git 读 netrc 也能
拿到凭据，`--no-netrc` 跳过。

想要在所有 shell 都用，把函数追加进 rc 文件一次：

```bash
vpnkit env --shell zsh --functions >> ~/.zshrc
exec zsh
# 之后任何新 shell：
proxy_on    # 🟢 proxy on
proxy_off   # 🔴 proxy off
```

## 升级

vpnkit 启动 2 秒后会去查 GitHub 最新 release，发现新版就在状态栏显示一个
低优先级的 `⚡` badge。装：

```bash
vpnkit update                            # 检查 + 计划 + 交互确认
vpnkit update --yes                      # 跳确认
vpnkit update --check                    # 只看 plan，不装
vpnkit update --vpnkit-only              # 只升 vpnkit
vpnkit update --mihomo-only              # 只升 mihomo
```

升级走 `release_mirror`，原子替换 binary（POSIX 允许覆盖运行中的可执行文件），
然后 `syscall.Exec` 重跑当前 TUI。mihomo 在替换期间会重启，代理短暂中断 ~1 s。

## 多用户 / 多实例

vpnkit 自动挑空闲端口。默认 `7890` / `9090` 被占（同机其他用户、其他代理）
时会向上扫描并把最终端口写入 `~/.config/vpnkit/config.toml`。

mihomo 的代理端口加了三道锁：

- `allow-lan: false`
- `bind-address: 127.0.0.1`
- `authentication: [user:pass]`（首次启动随机生成，存 0600 权限的 toml）

没这对用户名密码，**本机其他用户连不上你的代理**（即使端口绑在共享 loopback 上）。

## 命令行

| 命令 | 说明 |
|---|---|
| `vpnkit` | 打开 TUI |
| `vpnkit status` | mihomo 状态、模式、端口、组、当前订阅 |
| `vpnkit ip` | 经 mihomo 代理查出口 IP（用 ipinfo.io） |
| `vpnkit mode [rule\|global\|direct]` | 显示/切换模式 |
| `vpnkit groups` | 列用户可选 proxy 组 |
| `vpnkit nodes '<组>'` | 列某组成员 + 缓存延迟 |
| `vpnkit use '<组>' '<节点>'` | 切换某组的选中节点 |
| `vpnkit env [--shell zsh] [--unset] [--functions] [--no-netrc]` | 输出 shell snippet |
| `vpnkit update [--check] [--yes] [--vpnkit-only] [--mihomo-only]` | 升级 vpnkit + mihomo |
| `vpnkit init [--restore <path>] [--release-mirror <url>]` | 重建配置骨架 |
| `vpnkit uninstall [--yes] [--purge] [--keep-mihomo]` | 停服务，删 vpnkit 全部文件 |

只读命令接受 `--json`。退出码：`0` 成功、`1` 用户错、`2` 运行时错。

## TUI 快捷键

- `1`–`6` 跳 tab · `Tab`/`Shift+Tab` 循环 · `q` 退出（mihomo 继续跑）
- `↑` `↓` `j` `k` 移动 · `Enter` 激活/展开
- **Profiles**：`a` 添加 · `u` 更新 · `d` 删除 · `Enter` 设为 active
- **Proxies**：`Enter` 在组上展开/收起 · `Enter` 在节点上切换 · `t` 延迟测试
- **Connections**：`x` 关连接 · `/` 过滤
- **Rules**：`/` 过滤 · `u` 刷新 rule-providers
- **Settings**：`↑`/`↓` 子页循环（Mihomo Core、Service、External Controller、Default Rules、Patch Editor、Logs、Cache、About）

## 目录布局

| 路径 | 用途 |
|---|---|
| `~/.local/bin/vpnkit` | 本程序 |
| `~/.local/bin/mihomo` | 受管 mihomo |
| `~/.config/vpnkit/config.toml` | 订阅、端口、代理凭据、controller secret、release_mirror |
| `~/.config/mihomo/config.yaml` | 生成的 mihomo 配置（订阅更新时重写） |
| `~/.config/mihomo/patch.yaml` | 用户覆盖层，每次 deep-merge 到生成的 config |
| `~/.config/systemd/user/mihomo.service` | systemd-user 单元 |
| `~/.netrc` | proxy basic-auth 条目（`vpnkit env` 写入，权限 0600） |
| `~/.local/state/vpnkit/` | 日志、PID 文件（PID 模式） |
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
