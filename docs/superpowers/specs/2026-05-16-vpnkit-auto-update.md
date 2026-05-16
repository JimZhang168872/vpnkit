# vpnkit 自更新

**Date**: 2026-05-16
**Status**: APPROVED
**Target release**: v0.9.0

## 背景

v0.8.x install.sh 已经能正确处理升级路径（备份 profile、shell-based cleanup、init
还原），但用户必须手动跑 `curl … install.sh | bash` 才能升级。日常打开 vpnkit
没有"新版可用"的提示，也没有一键升级的入口。

mihomo 内核已经在 Settings → Mihomo Core 子页有 `u` 升级按键（复用
`internal/installer`），但只升 mihomo 本身，vpnkit 自己一直靠用户外部触发。

## 目标

让 vpnkit **启动时静默 check 新版本**，状态栏给低优先级提示；**`vpnkit update`
子命令一条命令升 vpnkit + mihomo**；**升完 self-exec 重跑 TUI**。

## 非目标

- 后台 cron / 定时任务（YAGNI；启动期 check 已经足够）
- 自动应用（不问就装）—— 升级 mihomo binary 必须 stop+start 服务，会短暂中断
  代理，应由用户控制
- 跨版本 schema 迁移（v0.8.x → 未来 v1.x 的 store schema break）—— 暂不处理，
  到了那个版本再单独 design

---

## 设计

### 1. 组件

```
internal/updater/
  check.go      — Check(...) (Info, error)
                  比较 vpnkit binary 当前版本 + mihomo 内核当前版本
                  与 GitHub latest tag，返回需要升的有哪些
  download.go   — DownloadVpnkit(url, sha, dst) error
                  vpnkit release 是 .tar.gz；mihomo release 是 .gz，复用 installer.Download
  apply.go      — ApplyVpnkit(tmpBinary, finalPath) error
                  atomic rename，准备 .old 做 rollback
  exec.go       — ExecSelf() error
                  syscall.Exec(/proc/self/exe, os.Args[0:])

cmd/vpnkit/cmd_update.go  — CLI 入口

internal/app/
  run.go        — startup goroutine: go pollUpdate(client, mirror)
                  超时 5s；成功就 prog.Send(UpdateAvailableMsg{...})
  update.go     — case UpdateAvailableMsg: m.updateInfo = msg
  statusbar.go  — 当 m.updateInfo.HasUpdate() 时显示 "⚡ vX.Y.Z (press u)"
```

### 2. 启动期 check

`app.Run()` 起一个 goroutine：

```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    info, err := updater.Check(ctx, updater.Opts{
        VpnkitCurrent: version,                    // ldflags 注入的
        MihomoBinary:  p.MihomoBinary(),           // 从 binary --version 拿
        Mirror:        st.Cfg.ReleaseMirror,
    })
    if err == nil && info.HasUpdate() {
        prog.Send(UpdateAvailableMsg{Info: info})
    }
}()
```

失败静默忽略：网络断 / mirror 不代理 api.github.com / GitHub API rate-limit
都不打扰用户。

如果 `main.version` 是 `"dev"` 或含 `-dev` 后缀（本地 build），跳过 check —
不应自动覆盖本地编译的 binary。

### 3. UI 提示

状态栏右半段（现在只有 `?:help q:quit` / flash）多一个 segment：

```
↑ 1.2 KiB/s ↓ 4.5 MiB/s          ⚡ v0.9.0 (press u in Settings)   ?:help q:quit
```

Settings tab 新增 "Updates" 子页：

```
Updates

  vpnkit   current 0.8.4   →   available 0.9.0   ✓
  mihomo   current v1.19.16 → available v1.19.24  ✓

  [u] update both
  [v] update vpnkit only
  [m] update mihomo only
  [r] re-check now
```

按 `u` 触发 → 列出 plan → 确认 → 装。

### 4. CLI

```bash
vpnkit update                  # both, interactive confirm
vpnkit update --yes            # both, no confirm
vpnkit update --check          # only print what's available, exit 0
vpnkit update --vpnkit-only    # skip mihomo
vpnkit update --mihomo-only    # skip vpnkit
```

退出码：`0` 已是最新或装好；`1` 用户取消；`2` 网络/下载失败；`3` 写盘失败。

### 5. 自更新流程

```
[plan]    vpnkit 0.8.4 → 0.9.0
          mihomo v1.19.16 → v1.19.24

[step 1]  download vpnkit_0.9.0_linux_amd64.tar.gz → /tmp/vpnkit-up.tar.gz
[step 2]  verify SHA256 against SHA256SUMS
[step 3]  tar -xzf → /tmp/vpnkit-up/vpnkit
[step 4]  cp ~/.local/bin/vpnkit ~/.local/bin/vpnkit.old
[step 5]  mv /tmp/vpnkit-up/vpnkit ~/.local/bin/vpnkit
[step 6]  rm /tmp/vpnkit-up.tar.gz

[step 7]  mihomo: svc.Stop()
[step 8]  reuse installer.Install(opts)  → 替换 ~/.local/bin/mihomo
[step 9]  svc.Start()
          代理短暂中断（1-2s）

[step 10] syscall.Exec(/proc/self/exe, [...])
          mihomo 已经在跑新内核，vpnkit 进程被替换，TUI 重新初始化
```

失败回滚：
- step 5 失败 → mv .old 回去
- step 8/9 失败 → svc.Start() 老 binary（.old 没动），打印错误指引重试

### 6. 网络 & mirror

`api.github.com` URL 通过 `installer.ApplyMirror(url, store.ReleaseMirror)` 包装。
release_mirror 已经在 toml 里、`vpnkit init --release-mirror` 已经能写它，
现在只是多一个 caller。

### 7. 版本检测

vpnkit 当前版本：
- 入口拿 `main.version`（ldflags）
- 为空、`"dev"` 或含 `-dev` 后缀 → 视为本地 build，**skip check**

mihomo 当前版本：
- `exec.Command(mihomoBinary, "-v").Output()` 解第一行 `"Mihomo Meta v1.19.16 ..."`
- 失败时降级走 `client.Version(ctx)` 拿 controller 报告的版本
- 都失败 → 视为 "unknown"，不能比对，仍允许强制升

### 8. 错误处理

| 失败点 | 处理 |
|---|---|
| GitHub API 超时/404 | check 静默 fail，UI 无提示；CLI 报 "could not reach github" |
| SHA256 mismatch | 立即停，删 tmp，报错 |
| ~/.local/bin 不可写 | 启动 check 时先 `os.Access(W_OK)` 提前报 |
| mihomo stop 失败 | 中止升级，留 .old，原 mihomo 继续跑 |
| exec.Self 失败 | 打印 "升级完成，请手动 quit 后重跑 vpnkit" |

### 9. 测试

- `internal/updater/check_test.go`：mock GitHub server → 各种 200 / 404 / timeout
- `internal/updater/version_test.go`：版本字符串解析；dev build 检测
- `internal/updater/apply_test.go`：临时目录 sandbox，验证 rename + rollback 逻辑
- `cmd/vpnkit/cmd_update_test.go`：mock updater 接口，验证 flag 行为分支

End-to-end 手动验证：本地 v0.9.0 release 后跑 `vpnkit update --check`，应该
显示 "already at latest"。

### 10. 兼容性

- 既有用户跑 v0.8.x：不变，没看到 update 入口
- 升 v0.9.0 后：第一次启动看到状态栏出现 ⚡ badge
- 跨大版本（v1.x）：spec 不覆盖，到那时单独设计

---

## 范围确认

- LOC 估计：~400 实现 + ~250 测试
- 接触文件：8 个新 + 5 个改
- 可单一 PR 合并
- 单一 release v0.9.0
