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

自动识别 amd64/arm64、SHA256 校验、装到 `~/.local/bin/vpnkit`。
锁版本用 `VERSION=v0.8.0 ./install.sh`。确认 `~/.local/bin` 在 `PATH` 上。

源码编译：`git clone … && cd vpnkit && make install`（需要 Go 1.22+）。

### 墙内安装

如果你的机器直连不了 `github.com` / `api.github.com`，用任意一个公开的
GitHub 加速服务把 install.sh 的下载**和**之后 mihomo 自己下 geo 数据**都**
代理过去。公开镜像经常下线/限流，选一个当前能用的：

```bash
# 例（请换成你当前能用的）：
MIRROR="https://ghproxy.com/"
VERSION=v0.8.0  # pin：部分镜像不代理 api.github.com

curl -sSL "${MIRROR}https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh" \
  | INSTALL_MIRROR="$MIRROR" VERSION="$VERSION" bash
```

`INSTALL_MIRROR` 会同步写入 `~/.config/vpnkit/config.toml` 的 `release_mirror`，
后续 mihomo 下 `geoip.metadb` / `geosite.dat` 都走同一镜像，不用再单独配。

其他公开镜像备选：`https://mirror.ghproxy.com/`、`https://ghp.ci/`、
`https://gh.api.99988866.xyz/`。挑一个先 ping 一下能不能用：

```bash
curl -fsSL --max-time 5 -o /dev/null \
  "${MIRROR}https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/README.md" \
  && echo OK || echo "这个镜像挂了，换一个"
```

## 3 分钟上手

```bash
vpnkit
```

首次启动会下 mihomo、生成 `~/.config/mihomo/config.yaml`、装 systemd unit 并启动、打开 TUI。

加订阅：

1. 按 `3` 进 Profiles → 按 `a` 弹表单
2. 输入名字 + 粘贴订阅 URL → `Enter`
3. 按 `u` 拉订阅 + 解析 + 重生成 config + reload mihomo

选节点：

1. 按 `2` 进 Proxies → 高亮 `🚀 Proxy` → 按 `t` 延迟测试
2. 选最快的 → `Enter` 切过去

终端用代理：

```bash
eval "$(vpnkit env --shell zsh)"   # 或 bash / fish
curl https://www.google.com         # 走 mihomo
eval "$(vpnkit env --unset)"        # 关掉
```

`vpnkit env` 同时会写一份 `~/.netrc`（权限 0600），让 curl/git 等读 netrc 的工具也能拿到代理凭据。`--no-netrc` 可跳过。

## 多用户 / 多实例

vpnkit 自动挑空闲端口。默认 `7890` / `9090` 被占（同机其他用户、其他代理）时会向上扫描并把最终端口写入 `~/.config/vpnkit/config.toml`。

mihomo 的代理端口加了三道锁：

- `allow-lan: false`
- `bind-address: 127.0.0.1`
- `authentication: [user:pass]`（首次启动随机生成，存 `~/.config/vpnkit/config.toml`，权限 0600）

没这对用户名密码，本机其他用户连不上你的代理。

## 命令行

```bash
vpnkit status                       # mihomo 状态、模式、端口、组、订阅
vpnkit ip                           # 经 mihomo 代理查出口 IP
vpnkit mode [rule|global|direct]    # 显示 / 设置模式
vpnkit groups                       # 列用户可选 proxy 组
vpnkit nodes '🚀 Proxy'              # 列组成员 + 缓存延迟
vpnkit use '🚀 Proxy' 'HK-01'        # 切到指定节点
vpnkit env [--shell zsh] [--unset] [--no-netrc]
```

每条都接受 `--json`。退出码：`0` 成功，`1` 用户错，`2` 运行时错。

## 被墙环境第一次启动

mihomo 第一次启动会从 GitHub 下 geo 数据。被墙的话，要么在 `~/.config/vpnkit/config.toml` 设镜像：

```toml
release_mirror = "https://ghproxy.com/"
```

要么让 mihomo 走已有的 HTTP 代理，写 systemd drop-in `~/.config/systemd/user/mihomo.service.d/proxy.conf`：

```ini
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7897"
Environment="HTTPS_PROXY=http://127.0.0.1:7897"
```

然后 `systemctl --user daemon-reload && systemctl --user restart mihomo`。

## TUI 快捷键

- `1`–`6` 跳 tab · `Tab`/`Shift+Tab` 循环 · `q` 退出（mihomo 继续跑）
- `↑` `↓` `j` `k` 移动 · `Enter` 激活/展开
- Profiles：`a` 添加、`u` 更新、`d` 删除
- Proxies：`t` 延迟测试
- Connections：`x` 关连接、`/` 过滤
- Settings：`↑`/`↓` 子页循环（Mihomo Core、Service、External Controller、Default Rules、Patch Editor、Logs、Cache、About）

## 目录布局

| 路径 | 用途 |
|---|---|
| `~/.local/bin/vpnkit` | 本程序 |
| `~/.local/bin/mihomo` | 受管 mihomo |
| `~/.config/vpnkit/config.toml` | 订阅、端口、代理凭据、controller secret |
| `~/.config/mihomo/config.yaml` | 生成的 mihomo 配置（订阅更新时重写） |
| `~/.config/mihomo/patch.yaml` | 用户覆盖层，每次 deep-merge 到生成的 config |
| `~/.config/systemd/user/mihomo.service` | systemd 单元 |
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
