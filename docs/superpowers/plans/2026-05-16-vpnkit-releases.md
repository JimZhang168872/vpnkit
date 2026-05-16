# vpnkit GitHub Releases — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a tag-driven release pipeline (GoReleaser + GitHub Actions) that produces pre-built `vpnkit` binaries for `linux/amd64` and `linux/arm64` as `tar.gz` artifacts on GitHub Releases, plus a `curl | bash` install script.

**Architecture:** GoReleaser v2 invoked from a GitHub Actions workflow on `push: tags: ['v*']`. GoReleaser handles cross-compile + tar.gz + SHA256SUMS + GitHub Release creation in one shot. `vpnkit --version` gains commit + date fields injected via ldflags. A standalone `install.sh` at the repo root downloads the right tarball, verifies the SHA, and installs to `~/.local/bin`. First release is tagged `v0.6.0-rc1` so we can validate the pipeline before promoting to `v0.6.0`.

**Tech Stack:** GoReleaser v2 · GitHub Actions · Bash (install script) · Go (existing). Zero new Go deps.

**Spec reference:** [`docs/superpowers/specs/2026-05-16-vpnkit-releases-design.md`](../specs/2026-05-16-vpnkit-releases-design.md).

---

## File Map

| Path | Status | Purpose |
|---|---|---|
| `cmd/vpnkit/main.go` | MODIFY | extend `version` var with `commit` + `date`; richer `--version` output |
| `.goreleaser.yaml` | NEW | declarative build / archive / release config |
| `.github/workflows/release.yml` | NEW | tag-trigger workflow that runs goreleaser |
| `install.sh` | NEW | repo-root `curl | bash` installer |
| `README.md` | MODIFY | Install section — pre-built binaries first (EN + 中文) |
| `docs/USAGE.md` | MODIFY | §1.2 — alternative one-line install (EN + 中文) |

Total: 6 files (3 new, 3 modify). Single, focused concern (releases).

---

## Task 1: Extend `--version` output with commit + date

This goes first because the ldflags in `.goreleaser.yaml` (Task 2) inject into these vars.

**Files:**
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Read the current main.go to see existing version block**

```bash
export PATH="$HOME/.local/go/bin:$PATH"
grep -n "^var version\|func runVersion" cmd/vpnkit/main.go
```

You'll see `var version = "dev"` and `func runVersion()`.

- [ ] **Step 2: Replace `var version = "dev"` with the multi-var block**

Find:
```go
// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"
```

Replace with:
```go
// version, commit, date are overridden at build time via -ldflags
//   -X main.version=... -X main.commit=... -X main.date=...
// (set by GoReleaser; defaults below for source builds).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)
```

- [ ] **Step 3: Replace `runVersion()` to print commit + date**

Find the existing function:
```go
func runVersion() {
	fmt.Printf("vpnkit %s\n", version)
	p := paths.Resolve()
	if info, err := os.Stat(p.MihomoBinary()); err == nil {
		fmt.Printf("mihomo binary: %s (%d bytes)\n", p.MihomoBinary(), info.Size())
	} else {
		fmt.Println("mihomo binary: not installed")
	}
}
```

Replace with:
```go
func runVersion() {
	short := commit
	if len(short) > 7 {
		short = short[:7]
	}
	fmt.Printf("vpnkit %s  (commit %s, built %s)\n", version, short, date)
	p := paths.Resolve()
	if info, err := os.Stat(p.MihomoBinary()); err == nil {
		fmt.Printf("mihomo binary: %s (%d bytes)\n", p.MihomoBinary(), info.Size())
	} else {
		fmt.Println("mihomo binary: not installed")
	}
}
```

- [ ] **Step 4: Build + smoke**

```bash
go build -o ./bin/vpnkit ./cmd/vpnkit
./bin/vpnkit --version
```

Expected output:
```
vpnkit dev  (commit none, built unknown)
mihomo binary: /home/zhangjunming/.local/bin/mihomo (33886356 bytes)
```

- [ ] **Step 5: Verify ldflags actually inject**

```bash
go build -ldflags "-X main.version=v0.6.0-test -X main.commit=abc1234567 -X main.date=2026-05-16T12:00:00Z" \
  -o ./bin/vpnkit ./cmd/vpnkit
./bin/vpnkit --version
```

Expected:
```
vpnkit v0.6.0-test  (commit abc1234, built 2026-05-16T12:00:00Z)
```

(commit short-form is the first 7 characters of `abc1234567`.)

- [ ] **Step 6: Run all existing tests to make sure nothing regressed**

```bash
go test -race ./...
```

Expected: all packages pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/vpnkit/main.go
git commit -m "feat(cmd): expose commit + build date via --version (ldflags-driven)"
```

---

## Task 2: GoReleaser config

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Write the file**

`.goreleaser.yaml` (exact contents):

```yaml
version: 2
project_name: vpnkit

builds:
  - id: vpnkit
    main: ./cmd/vpnkit
    binary: vpnkit
    env: [CGO_ENABLED=0]
    goos: [linux]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - id: vpnkit
    name_template: "vpnkit_{{.Version}}_{{.Os}}_{{.Arch}}"
    formats: [tar.gz]
    files:
      - LICENSE
      - README.md
      - docs/USAGE.md

checksum:
  name_template: "SHA256SUMS"
  algorithm: sha256

release:
  github:
    owner: JimZhang168872
    name: vpnkit
  draft: false
  prerelease: auto
  mode: replace
```

Notes:
- `version: 2` is required by GoReleaser v2.
- `prerelease: auto` flags any tag that contains a `-` (e.g. `v0.6.0-rc1`) as a pre-release. Pure semver (`v0.6.0`) becomes a full release.
- `mode: replace` lets us re-run goreleaser against the same tag if we need to re-push (overwrites prior assets).
- `formats: [tar.gz]` is the v2 spelling; v1 used `format: tar.gz` (singular).

- [ ] **Step 2: (Optional, if you have GoReleaser locally) snapshot dry-run**

```bash
which goreleaser || go install github.com/goreleaser/goreleaser/v2@latest
goreleaser release --snapshot --clean
ls dist/
```

Expected: 2 tarballs + a SHA256SUMS file appear under `dist/`. If goreleaser isn't installed and you don't want to install it, **skip this step** — Task 4 will validate via real CI.

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yaml
git commit -m "build: add GoReleaser config (linux amd64/arm64 tar.gz)"
```

---

## Task 3: Release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write the file**

`.github/workflows/release.yml`:

```yaml
name: release

on:
  push:
    tags: ['v*']

permissions:
  contents: write   # GoReleaser needs this to create the GitHub release

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0    # required: goreleaser walks commits to compute changelog
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Notes:
- `permissions: contents: write` is required at the top of the file because the default `GITHUB_TOKEN` is read-only since GitHub's 2023 security update.
- `fetch-depth: 0` fetches the full history; GoReleaser needs it to compute "commits since previous tag".
- We pin `actions/checkout@v4`, `actions/setup-go@v5`, `goreleaser/goreleaser-action@v6` to the current major versions.

- [ ] **Step 2: Validate YAML syntax locally**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"
```

Expected: no output (success). Errors would print here.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: tag-triggered release workflow (GoReleaser)"
```

---

## Task 4: install.sh at repo root

**Files:**
- Create: `install.sh`

- [ ] **Step 1: Write the script**

`install.sh` (executable):

```bash
#!/usr/bin/env bash
set -euo pipefail

# vpnkit installer
# Usage:
#   curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
#   VERSION=v0.6.0 INSTALL_DIR=$HOME/bin bash <(curl -sSL .../install.sh)
#
# Env:
#   VERSION       pin a tag (default: latest non-prerelease release on GitHub)
#   INSTALL_DIR   target dir (default: $HOME/.local/bin)

REPO="JimZhang168872/vpnkit"
DEST="${INSTALL_DIR:-$HOME/.local/bin}"

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "vpnkit: unsupported arch $arch (only amd64 / arm64 are released)" >&2; exit 1 ;;
esac

if [ -z "${VERSION:-}" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -oP '"tag_name":\s*"\K[^"]+' || true)
fi
[ -n "${VERSION:-}" ] || { echo "vpnkit: cannot resolve latest version (set VERSION=v…)" >&2; exit 1; }

VER_NUM="${VERSION#v}"
TARBALL="vpnkit_${VER_NUM}_linux_${arch}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "vpnkit: downloading $TARBALL …"
curl -fsSL -o "$tmp/$TARBALL" "$BASE/$TARBALL"
curl -fsSL -o "$tmp/SHA256SUMS" "$BASE/SHA256SUMS"

( cd "$tmp" && grep " $TARBALL\$" SHA256SUMS | sha256sum -c - >/dev/null )
tar -xzf "$tmp/$TARBALL" -C "$tmp"
mkdir -p "$DEST"
install -m 0755 "$tmp/vpnkit" "$DEST/vpnkit"

echo "vpnkit: installed $VERSION → $DEST/vpnkit"
"$DEST/vpnkit" --version
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x install.sh
ls -l install.sh
```

Expected: mode shows `-rwxr-xr-x …`.

- [ ] **Step 3: Lint with shellcheck (if installed)**

```bash
which shellcheck && shellcheck install.sh || echo "shellcheck not installed — skipping"
```

Expected: no errors. If shellcheck isn't installed, skip — script is short and the `set -euo pipefail` + `curl -fsSL` give us safety.

- [ ] **Step 4: Local sanity check (no actual install — there's no release yet)**

```bash
bash -n install.sh
```

Expected: no output. (`bash -n` parses without executing.)

- [ ] **Step 5: Commit**

```bash
git add install.sh
git commit -m "build: add install.sh (curl | bash) for one-line install"
```

---

## Task 5: README updates (both languages)

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Read current Install section to find anchor points**

```bash
grep -n "^#### From source\|^#### 从源码编译\|^### Install\|^### 安装" README.md
```

Note the line numbers — you'll insert above each "From source" subsection.

- [ ] **Step 2: Insert "Pre-built binaries" subsection in English half**

Find this in `README.md`:

```markdown
#### From source (only path today)
```

Replace with:

```markdown
#### Pre-built binaries (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

The script auto-detects amd64 / arm64, verifies SHA256, and installs to
`~/.local/bin/vpnkit` (override with `INSTALL_DIR=…`). Pin a version with
`VERSION=v0.6.0` env var. See the [releases page](https://github.com/JimZhang168872/vpnkit/releases) for tags.

#### From source

```

(Note: the existing "(only path today)" suffix is dropped.)

- [ ] **Step 3: Same in 中文 half**

Find:

```markdown
#### 源码编译（目前唯一方式）
```

Replace with:

```markdown
#### 预编译二进制（推荐）

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

脚本自动识别 amd64 / arm64、SHA256 校验、装到 `~/.local/bin/vpnkit`
（用 `INSTALL_DIR=…` 改目标）。锁版本用 `VERSION=v0.6.0`。
[Releases 页面](https://github.com/JimZhang168872/vpnkit/releases) 看所有 tag。

#### 源码编译

```

- [ ] **Step 4: Verify both halves render**

```bash
grep -n "Pre-built binaries\|预编译二进制" README.md
```

Expected: 2 matches (1 each language).

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs(readme): pre-built binaries section (EN + 中文)"
```

---

## Task 6: USAGE.md updates (both languages)

**Files:**
- Modify: `docs/USAGE.md`

- [ ] **Step 1: Find §1.2 anchors**

```bash
grep -n "^#### 1.2 " docs/USAGE.md
```

Expected: 2 lines (English then 中文).

- [ ] **Step 2: Insert one-liner in English §1.2**

Find:

```markdown
#### 1.2 Build and install vpnkit

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
```

Replace with:

```markdown
#### 1.2 Build and install vpnkit

The fastest way (no Go toolchain needed):

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

Or build from source:

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
```

- [ ] **Step 3: Same in 中文 §1.2**

Find:

```markdown
#### 1.2 编译并安装 vpnkit

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
```

Replace with:

```markdown
#### 1.2 编译并安装 vpnkit

最快的方式（不需要 Go 环境）：

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

或者源码编译：

```bash
git clone https://github.com/JimZhang168872/vpnkit.git
```

- [ ] **Step 4: Commit**

```bash
git add docs/USAGE.md
git commit -m "docs(usage): one-line install via install.sh (EN + 中文)"
```

---

## Task 7: Push docs/CI changes + first release-candidate tag

This task is the live validation of the whole pipeline. You're pushing all the prep commits, then tagging `v0.6.0-rc1` — which triggers the new workflow on GitHub.

- [ ] **Step 1: Push commits to main**

```bash
git push origin main
```

Watch the existing `ci.yml` workflow on https://github.com/JimZhang168872/vpnkit/actions for green. The `release.yml` doesn't trigger here (no tag), only `ci.yml` does.

- [ ] **Step 2: Tag the release candidate**

```bash
git tag v0.6.0-rc1 -m "First pre-built release (pipeline verification)"
git push origin v0.6.0-rc1
```

- [ ] **Step 3: Watch the release workflow**

Open https://github.com/JimZhang168872/vpnkit/actions and look for the "release" workflow run triggered by the tag. It should:

- Check out at full depth
- Set up Go 1.22
- Run `goreleaser release --clean`
- Upload artifacts to https://github.com/JimZhang168872/vpnkit/releases/tag/v0.6.0-rc1

Expected total time: 2–4 minutes.

- [ ] **Step 4: Verify the release page**

Open https://github.com/JimZhang168872/vpnkit/releases/tag/v0.6.0-rc1.

Confirm:
- Page exists, marked **"Pre-release"**
- Three assets: `vpnkit_0.6.0-rc1_linux_amd64.tar.gz`, `vpnkit_0.6.0-rc1_linux_arm64.tar.gz`, `SHA256SUMS`
- Auto-generated release notes show commits since the previous tag

If anything is wrong, fix locally, then force-push the tag (GoReleaser's `mode: replace` re-uploads):

```bash
git tag -d v0.6.0-rc1
git push origin :refs/tags/v0.6.0-rc1
git tag v0.6.0-rc1 -m "First pre-built release (pipeline verification)"
git push origin v0.6.0-rc1
```

- [ ] **Step 5: Live install test**

```bash
# Use VERSION=... because the install.sh script's "latest" endpoint skips pre-releases.
VERSION=v0.6.0-rc1 INSTALL_DIR=/tmp/vpnkit-install-test bash <(curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh)
/tmp/vpnkit-install-test/vpnkit --version
```

Expected:
```
vpnkit: downloading vpnkit_0.6.0-rc1_linux_amd64.tar.gz …
vpnkit: installed v0.6.0-rc1 → /tmp/vpnkit-install-test/vpnkit
vpnkit v0.6.0-rc1  (commit <7-char>, built 2026-05-16T...)
mihomo binary: ...
```

If install fails, read the error — most likely SHA mismatch (tarball name template wrong) or download 404 (asset name mismatch). Fix `.goreleaser.yaml` `name_template`, or fix `install.sh`'s `TARBALL` line, then re-tag rc.

- [ ] **Step 6: Cleanup the test install**

```bash
rm -rf /tmp/vpnkit-install-test
```

---

## Task 8: Promote rc to canonical v0.6.0

Only do this once Task 7 verified everything end-to-end.

- [ ] **Step 1: Tag the real release**

```bash
git tag v0.6.0 -m "First pre-built release"
git push origin v0.6.0
```

- [ ] **Step 2: Watch the workflow + verify the release**

https://github.com/JimZhang168872/vpnkit/actions — same flow as Task 7.

Open https://github.com/JimZhang168872/vpnkit/releases/tag/v0.6.0 and confirm:

- The release is **NOT marked as pre-release** (because the tag has no `-` suffix)
- It shows up as the new "Latest" release on the repo's main releases page
- 3 assets present

- [ ] **Step 3: Final install test (now using "latest" path)**

```bash
INSTALL_DIR=/tmp/vpnkit-install-final bash <(curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh)
/tmp/vpnkit-install-final/vpnkit --version
```

(Note: this time we don't set `VERSION=` — the script should resolve `v0.6.0` as latest because it's not a pre-release.)

Expected: prints `vpnkit v0.6.0 (commit ..., built ...)`.

- [ ] **Step 4: Cleanup test install**

```bash
rm -rf /tmp/vpnkit-install-final
```

---

## Self-Review

**1. Spec coverage:**

| Spec section | Task |
|---|---|
| §3 file map | T1 (main.go), T2 (.goreleaser.yaml), T3 (release.yml), T4 (install.sh), T5 (README), T6 (USAGE.md) |
| §4 .goreleaser.yaml | T2 |
| §5 release.yml | T3 |
| §6 main.go modifications | T1 |
| §7 install.sh | T4 |
| §8 README + USAGE.md | T5 + T6 |
| §9 testing — local snapshot | T2 step 2 (optional) |
| §9 testing — live rc + verify + install test | T7 |
| §9 testing — promote to v0.6.0 | T8 |

All spec sections have at least one task implementing them.

**2. Placeholder scan:** no TBDs, no "implement later", no "similar to Task N" — every step has explicit code or commands.

**3. Type / name consistency:**
- `version`, `commit`, `date` Go vars (T1) ↔ ldflags `-X main.version=` etc. (T2). ✓
- Tarball name `vpnkit_{Version}_linux_{Arch}.tar.gz` (T2) ↔ `install.sh`'s `TARBALL="vpnkit_${VER_NUM}_linux_${arch}.tar.gz"` where `VER_NUM="${VERSION#v}"` (T4). ✓ — GoReleaser v2's `{{.Version}}` excludes the `v` prefix; `${VERSION#v}` does the same in bash.
- `SHA256SUMS` filename matches between `.goreleaser.yaml` `checksum.name_template` (T2) and `install.sh`'s download (T4). ✓
- "Pre-built binaries" / "预编译二进制" headings consistent between README (T5) and USAGE.md (T6). ✓

End of plan.
