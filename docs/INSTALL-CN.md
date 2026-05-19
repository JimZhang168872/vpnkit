# 墙内安装 vpnkit

> 这份文档专门写给国内 (GFW 内) 用户。如果你能直连 GitHub,直接
> [README.md](../README.md#install) 一行命令搞定,不需要看这里。

---

## 0. 一个鸡生蛋的问题

vpnkit 本身是个**代理管理器**,装它需要从 `github.com` 拉两个东西:

1. **vpnkit binary tarball** — 走 `install.sh` 里的 `curl`,**会读** `HTTPS_PROXY` 环境变量
2. **mihomo binary** — 走 `vpnkit init` 的 bootstrap,为了避免 v0.9.x 历史上的
   chicken-and-egg deadlock,**故意不读** env proxy,直连 `github.com`

所以墙内安装的真正难点不在第一步(curl 会用你设的代理),而在第二步
(vpnkit init 那一步会绕过代理直连)。下面三条路径都要解决这个问题。

---

## 1. 决策树:你属于哪种情况?

```
有现成代理客户端 (clash / v2rayN / clash-verge / 小火箭) 在跑?
  ├─ 是 → 路径 A (推荐,最快)
  └─ 否
      ├─ 能从某种渠道临时上 GitHub (镜像 / 公司网络 / 朋友机器)?
      │   ├─ 能 → 路径 B (GitHub 镜像) 或 路径 C (离线)
      │   └─ 不能 → 找朋友帮你下,走路径 C
```

---

## 2. 路径 A:已有现成代理客户端 (推荐)

假设你本机已经有 clash / v2rayN 在 `127.0.0.1:7897` 跑着(或者别的端口)。

### 一行装好

```bash
# 1. 在 shell 里先把代理打开,这样 install.sh 的 curl 会用它
export HTTPS_PROXY=http://127.0.0.1:7897
export HTTP_PROXY=http://127.0.0.1:7897
export ALL_PROXY=socks5://127.0.0.1:7897

# 2. 关键:先用你的代理把 mihomo 二进制下下来放到位,
#    绕过 vpnkit init bootstrap 的 NoProxy 限制
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64;; aarch64) ARCH=arm64;; esac
MIHOMO_VER=$(curl -fsSL https://api.github.com/repos/MetaCubeX/mihomo/releases/latest | grep -oP '"tag_name":\s*"\K[^"]+')
mkdir -p ~/.local/bin
curl -fL "https://github.com/MetaCubeX/mihomo/releases/download/${MIHOMO_VER}/mihomo-linux-${ARCH}-${MIHOMO_VER}.gz" \
  | gunzip > ~/.local/bin/mihomo
chmod +x ~/.local/bin/mihomo

# 3. 现在跑 vpnkit 一键脚本 (它检测到 mihomo 已经在,会跳过下载)
bash <(curl -fsSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh)
```

跑完检查:

```bash
vpnkit --version
vpnkit status
```

如果 `vpnkit --version` 显示 `mihomo binary: /home/<you>/.local/bin/mihomo`,
并且 `vpnkit status` 看到 `🟢 mihomo running`,就算完事。

### 为什么要先手动下 mihomo?

`vpnkit init` 的 bootstrap 流程为了避免历史上的死锁(`HTTPS_PROXY` 指向
vpnkit 自己 mihomo,但 mihomo 还没装),**故意把 mihomo 下载这一步走直连**。
墙内这一步必死。提前把 mihomo 放好,bootstrap 检测到就跳过下载,不再触发
直连。

---

## 3. 路径 B:用 GitHub 镜像加速 (没有代理时)

社区有几个公开的 GitHub 镜像,在国内能直接通。常用的:

- `https://gh-proxy.com/`
- `https://ghproxy.net/`
- `https://kkgithub.com` (raw + release 都支持)
- `https://github.akams.cn/` (raw)

> 镜像服务**经常变**,挂了找[这里](https://github.akams.cn/) 之类的导航
> 站现找一个可用的。

### 一键脚本(用 gh-proxy.com)

```bash
# 1. 选一个能用的镜像 (这里以 gh-proxy.com 为例)
PROXY_URL="https://gh-proxy.com"
# 测一下镜像通不通
curl -fsSL --max-time 8 "${PROXY_URL}/https://api.github.com/repos/JimZhang168872/vpnkit/releases/latest" \
  | head -3 || { echo "镜像挂了,换一个"; exit 1; }

# 2. 通过镜像下 mihomo 二进制
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64;; aarch64) ARCH=arm64;; esac
MIHOMO_VER=$(curl -fsSL "${PROXY_URL}/https://api.github.com/repos/MetaCubeX/mihomo/releases/latest" \
  | grep -oP '"tag_name":\s*"\K[^"]+')
mkdir -p ~/.local/bin
curl -fL "${PROXY_URL}/https://github.com/MetaCubeX/mihomo/releases/download/${MIHOMO_VER}/mihomo-linux-${ARCH}-${MIHOMO_VER}.gz" \
  | gunzip > ~/.local/bin/mihomo
chmod +x ~/.local/bin/mihomo

# 3. 通过镜像下 install.sh 本体 + 改写里面的 GitHub URL
curl -fsSL "${PROXY_URL}/https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh" \
  | sed "s|https://github.com/|${PROXY_URL}/https://github.com/|g; s|https://api.github.com/|${PROXY_URL}/https://api.github.com/|g" \
  > /tmp/install-cn.sh
bash /tmp/install-cn.sh
rm /tmp/install-cn.sh
```

镜像方案的好处:不需要任何代理客户端。坏处:镜像有限速、有时挂、可能被滥用 ban。

---

## 4. 路径 C:完全离线安装 ✅ 已验证

> **本节命令在 Docker `--network=none`(完全无网)容器里跑通过。**
> 验证日期 2026-05-19,vpnkit v1.0.3 + mihomo v1.19.25。

适合:目标机器**完全**上不了外网,但你有另一台能上 GitHub 的机器。

### 关键 finding (避坑)

1. **mihomo 启动需要 GeoIP MMDB 文件** —— 默认 config 含 `GEOIP,CN,...`
   规则,缺 MMDB 就直接 fatal exit。所以 bundle **必须**包含 4 个 geo
   文件,不能指望 vpnkit init 离线时把它们拉下来。
2. **GitHub release 资产下载经常 SSL EOF 中途切断** —— curl 必须带
   `--retry --retry-all-errors`,否则一次 fail 整个 bundle 准备阶段就跪。
3. **mihomo 二进制 → `~/.local/bin/mihomo`** —— vpnkit init 看到就跳过
   下载,这是规避 bootstrap NoProxy 直连 github 的关键。

### 在能上 GitHub 的机器上(准备 bundle)

```bash
set -e
ARCH=amd64   # 或 arm64,跟目标机一致

# 1. 解析版本
VPN_VER=$(curl -fsSL https://api.github.com/repos/JimZhang168872/vpnkit/releases/latest \
  | grep -oP '"tag_name":\s*"\K[^"]+')
MIHOMO_VER=$(curl -fsSL https://api.github.com/repos/MetaCubeX/mihomo/releases/latest \
  | grep -oP '"tag_name":\s*"\K[^"]+')
echo "vpnkit=$VPN_VER  mihomo=$MIHOMO_VER"

# 2. 准备目录
rm -rf /tmp/vpnkit-bundle
mkdir -p /tmp/vpnkit-bundle/geo && cd /tmp/vpnkit-bundle

# 3. 公共下载函数 —— retry 防 GFW 中途 SSL EOF / connect reset
fetch() {
  curl -fL --retry 5 --retry-delay 2 --retry-max-time 180 \
       --retry-all-errors --retry-connrefused \
       --connect-timeout 30 -O "$1"
}

# 4. vpnkit binary + 校验文件
fetch "https://github.com/JimZhang168872/vpnkit/releases/download/${VPN_VER}/vpnkit_${VPN_VER#v}_linux_${ARCH}.tar.gz"
fetch "https://github.com/JimZhang168872/vpnkit/releases/download/${VPN_VER}/SHA256SUMS"

# 5. mihomo binary
fetch "https://github.com/MetaCubeX/mihomo/releases/download/${MIHOMO_VER}/mihomo-linux-${ARCH}-${MIHOMO_VER}.gz"

# 6. ⚠️ 关键:4 个 GeoIP / GeoSite 数据文件,缺一不可
cd geo
for f in country.mmdb geoip.metadb geosite.dat GeoLite2-ASN.mmdb; do
  fetch "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/$f"
done
cd ..

# 7. 完整性自查
gunzip -t mihomo-linux-*.gz && echo "✅ mihomo gz 完整"
grep "vpnkit_${VPN_VER#v}_linux_${ARCH}.tar.gz" SHA256SUMS | sha256sum -c -

# 8. 打包 (大概 40MB)
cd /tmp
tar czf vpnkit-bundle.tgz -C /tmp vpnkit-bundle
du -h vpnkit-bundle.tgz

# 9. 传给目标机
scp vpnkit-bundle.tgz user@target:/tmp/
```

### 在目标机上(离线安装)

```bash
set -e
cd /tmp && tar xzf vpnkit-bundle.tgz && cd vpnkit-bundle

mkdir -p ~/.local/bin ~/.config/mihomo

# 1. mihomo 二进制
gunzip -c mihomo-linux-*.gz > ~/.local/bin/mihomo
chmod +x ~/.local/bin/mihomo

# 2. vpnkit 二进制 (顺便校验 SHA256)
grep "vpnkit_.*_linux_.*\.tar\.gz" SHA256SUMS | sha256sum -c -
tar xzf vpnkit_*_linux_*.tar.gz
install -m 0755 vpnkit ~/.local/bin/vpnkit

# 3. ⚠️ 关键:把 geo 文件放到 mihomo 配置目录,bootstrap 看到就跳过拉取
cp geo/*.mmdb geo/*.metadb geo/*.dat ~/.config/mihomo/

# 4. 跑 vpnkit init —— 不需要 --skip-bootstrap!
#    所有需要的资源 (mihomo binary / geo / ruleset embed) 都已就位,
#    bootstrap 全程不会触发网络。
~/.local/bin/vpnkit init

# 5. 验证
~/.local/bin/vpnkit status
```

### 预期输出 (验证过)

`vpnkit init` 应该看到:

```
🛠️  vpnkit init
✅ ~/.config/vpnkit/config.toml (created)
✅ ~/.config/mihomo/config.yaml (created)
🎉 ready — run `vpnkit` to start
✅ mihomo binary already present (~/.local/bin/mihomo)
✅ geo files seeded
✅ rulesets seeded
🔧 installing pid service backend…              # (或 systemd-user,看你机器)
✅ mihomo running (mode=pid, pid=XX)
```

`vpnkit status` 应该看到:

```
🟢 mihomo  v1.19.25   running
🔧 mode    rule
🚪 ports   mixed=XXXXX   controller=XXXXX
🚀 groups  4 selectable (GLOBAL → DIRECT, ...)
📚 sources   0 subs + 0 local nodes
🔀 routing   mode=rule  target=DIRECT
```

如果看到 mihomo 没起来,看日志:

```bash
tail -30 ~/.local/state/vpnkit/mihomo.log
# 或 systemd 模式下:
journalctl --user -u mihomo -n 30 --no-pager
```

常见离线 + 漏 geo 的报错:

```
level=fatal msg="Parse config error: rules[5] [GEOIP,CN,🎯 Direct] error:
can't download MMDB: ... dial tcp: lookup github.com ... network is unreachable"
```

→ bundle 里没装 geo,补 `cp geo/*.mmdb ... ~/.config/mihomo/` 那一步,然后
`systemctl --user restart mihomo` 或 `pkill mihomo && vpnkit init`。

---

## 5. 装完之后

继续看 [USAGE_zh.md](USAGE_zh.md) 配订阅、节点、规则。常用入口:

```bash
vpnkit                              # 打开 TUI
vpnkit subs add main <你的订阅URL>   # 加订阅
vpnkit subs update main             # 拉节点
vpnkit active main                  # 用这个订阅做活跃源
vpnkit status                       # 查状态
eval "$(vpnkit env --shell zsh)"    # 把代理 env 灌进当前 shell
```

订阅源也可能被 GFW 卡,这时给订阅指定一个能通的 UA:

```bash
vpnkit subs add main "<url>" --ua=clash.meta
```

---

## 6. 常见错误排查

### `OpenSSL SSL_connect: SSL_ERROR_SYSCALL`

```
curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to release-assets.githubusercontent.com:443
```

GFW 中途切断 TLS 握手。**install.sh 已经带 retry 防护** (`--retry 5
--retry-all-errors`),但如果整片都被掐,只能换路径。最稳的是路径 A
(用你自己的代理)。

### `dial tcp 127.0.0.1:XXXXX: connect: connection refused`

```
proxyconnect tcp: dial tcp 127.0.0.1:52697: connect: connection refused
```

`HTTPS_PROXY` 指的代理端口没人在听。可能原因:

- 代理客户端没启动 → 启动你的 clash/v2rayN
- 代理换了端口 → 重新 `vpnkit env --shell zsh` / 看你代理客户端当前端口
- vpnkit-managed mihomo 刚 reload 短暂下线 → 等几秒重试

### `vpnkit init` 卡在 "downloading mihomo"

bootstrap 在直连 `github.com/MetaCubeX/mihomo/releases/...`,被 GFW 掐。
**先按路径 A / 路径 B 手动把 mihomo 放到 `~/.local/bin/mihomo`**,再重跑
`vpnkit init` 即可。

### `mihomo failed to start` / `geo pre-seed had errors`

geo 数据预拉失败一般非致命。看 mihomo systemd 日志:

```bash
systemctl --user status mihomo
journalctl --user -u mihomo -n 50 --no-pager
```

如果是 `Geo data` 类报错,等 mihomo 自己 retry,或重启 mihomo:

```bash
systemctl --user restart mihomo
```

### `vpnkit update` 卡 / 报 connection refused

`vpnkit update` 会**遵守** `HTTPS_PROXY` (从 v1.0.3 开始)。所以装好之后:

```bash
eval "$(vpnkit env --shell zsh)"   # 把 vpnkit 自己的代理灌到 shell
vpnkit update --yes                # 现在走你 vpnkit-managed mihomo 出去
```

如果还报 `connection refused`,说明 vpnkit 的 mihomo 当前下线或换了端口。
看 `vpnkit status` 同时检查 `env | grep -i proxy` 端口对不对。

### 想完全确认 vpnkit init 用了哪种网络路径

```bash
vpnkit init --skip-bootstrap   # 跳过所有网络步骤,只写配置
# 然后看 ~/.local/bin/mihomo 是不是你手动放的那份
ls -lh ~/.local/bin/mihomo
~/.local/bin/mihomo -v
```

---

## 7. 我能帮你做的镜像 / 加速优化

vpnkit 目前**不**内置镜像 fallback,故意保持简单 + 直连。如果你想自己魔改
install.sh 用某个内网镜像,把脚本改两行就行:

```bash
# 例如把 github.com 换成 gh.your-mirror.local
sed -i 's|https://github.com/|https://gh.your-mirror.local/|g' install.sh
sed -i 's|https://api.github.com/|https://gh.your-mirror.local/api/|g' install.sh
sed -i 's|https://raw.githubusercontent.com/|https://gh.your-mirror.local/raw/|g' install.sh
```

mihomo binary 那块儿是 vpnkit Go 代码里 hardcode 的 GitHub URL,要改的话:

- 短期:把 mihomo 手动下好放到 `~/.local/bin/mihomo`,vpnkit 检测到就不下了
- 长期:给我们提个 issue,我们可以加一个 `VPNKIT_MIHOMO_URL` 环境变量做覆盖

---

最后:如果你卡在 GFW 安装的任何一步,**贴具体报错** + `vpnkit --version` +
`env | grep -i proxy` 输出到 issue,我们继续修。
