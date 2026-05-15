mkdir -p ~/.local/bin ~/.config/mihomo
cd /tmp

# 看一下最新版本号:https://github.com/MetaCubeX/mihomo/releases
VERSION=1.19.16   # 替换成最新版

# CPU 老一点用 -compatible 后缀更稳;新机器可以去掉 -compatible
curl -L -o mihomo.gz \
  https://github.com/MetaCubeX/mihomo/releases/download/v${VERSION}/mihomo-linux-amd64-compatible-v${VERSION}.gz

gunzip mihomo.gz
chmod +x mihomo
mv mihomo ~/.local/bin/mihomo

# 验证
mihomo -v
