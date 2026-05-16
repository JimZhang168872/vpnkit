# vpnkit GitHub Releases — pre-built binaries (Design)

- **Date:** 2026-05-16
- **Author:** Jim (with Claude)
- **Status:** Draft for review
- **Scope:** Add a tag-driven release pipeline that produces pre-built `vpnkit` binaries for `linux/amd64` and `linux/arm64`, packaged as `tar.gz` with bundled docs, and uploaded to GitHub Releases. Also add a one-line `curl | bash` install script.

---

## 1. Goals & Non-Goals

### Goals

- A user on Linux can install vpnkit without `git clone` + `go install` — `curl … | bash` or download a tarball and untar.
- Releases are reproducible (`-trimpath`, pinned Go version) and tamper-evident (SHA256SUMS file).
- The release pipeline is declarative (one config file) and triggered automatically on `v*` tag push.
- `vpnkit --version` shows the actual release version + git commit + build date instead of `dev`.

### Non-Goals

- Windows / macOS binaries.
- Docker image / `apt` package / `homebrew` formula.
- Code signing (Sigstore / GPG).
- Auto-update of an installed `vpnkit` (out of scope; the upgrade flow stays Settings → Mihomo Core for mihomo only).
- Pre-release prep beyond what GoReleaser already does (no manual changelog file).

---

## 2. Approach

**Use [GoReleaser](https://goreleaser.com/) v2** as the build/package/upload tool, invoked from a GitHub Actions workflow on `push: tags: ['v*']`. GoReleaser is the de-facto Go release tool — declarative YAML config, handles cross-compilation, archives, checksums, and the GitHub Releases API call in one shot.

GoReleaser does NOT enter the vpnkit project as a Go dependency; it runs only inside the CI runner via the official `goreleaser/goreleaser-action@v6`.

GitHub auto-generates release notes from commits between the previous and current tag (`generate_release_notes: true`).

---

## 3. File map

| Path | Status | Purpose |
|---|---|---|
| `.goreleaser.yaml` | NEW | declarative build / archive / release config |
| `.github/workflows/release.yml` | NEW | tag-trigger workflow that runs goreleaser |
| `install.sh` | NEW | `curl | bash` installer (repo root); arch-detects, SHA-verifies, extracts to `~/.local/bin` |
| `cmd/vpnkit/main.go` | MODIFY | extend `version` package var with `commit` + `date`; richer `--version` output |
| `README.md` | MODIFY | Install section — add "Pre-built binaries" subsection |
| `docs/USAGE.md` | MODIFY | §1.2 Build → add `install.sh` one-liner alternative |

---

## 4. `.goreleaser.yaml`

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
- `prerelease: auto` — tags containing `-rc`, `-beta`, `-pre`, etc. are marked as prereleases automatically. Plain `v0.6.0` (pure semver) is a full release. **Going forward, the project switches to pure-semver tags for finalised releases**; the existing prerelease-style tags from earlier phases (`v0.4.1-phase4-fixes`, `v0.5.0-ctl`, …) stay as-is for history.
- `mode: replace` — re-running goreleaser against the same tag (e.g. amending and re-pushing) overwrites prior assets.
- `version: 2` — required by GoReleaser v2+; the project doesn't carry GoReleaser v1 baggage.

---

## 5. `.github/workflows/release.yml`

```yaml
name: release

on:
  push:
    tags: ['v*']

permissions:
  contents: write   # GoReleaser needs this to create the release

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

GitHub auto-generated notes are enabled by default in `release.yml`'s GitHub release creation; if we want to be explicit we'd add a `release.use: github-native` block in `.goreleaser.yaml` — defer until the first run shows whether the auto notes are good enough.

---

## 6. `cmd/vpnkit/main.go` modifications

Replace:

```go
var version = "dev"
```

with:

```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

Update `runVersion()` to include commit (short SHA) and build date:

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

Sample output after release:
```
vpnkit v0.5.1  (commit a1b2c3d, built 2026-05-16T12:34:56Z)
mihomo binary: /home/u/.local/bin/mihomo (33886356 bytes)
```

Sample output for source builds (without ldflags):
```
vpnkit dev  (commit none, built unknown)
```

This is the only behavioural change in vpnkit itself; everything else is build/CI plumbing.

---

## 7. `install.sh` (repo root)

```bash
#!/usr/bin/env bash
set -euo pipefail
# vpnkit installer
# Usage:
#   curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
#   VERSION=v0.6.0 INSTALL_DIR=$HOME/bin bash <(curl -sSL .../install.sh)
#
# Env:
#   VERSION       pin a tag (default: latest release on GitHub)
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
  VERSION=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -oP '"tag_name":\s*"\K[^"]+' || true)
fi
[ -n "$VERSION" ] || { echo "vpnkit: cannot resolve latest version (set VERSION=v…)" >&2; exit 1; }

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

Behaviour:
- Auto-detects `amd64` / `arm64`; refuses anything else with a clear message.
- Resolves latest tag via the GitHub Releases API unless `VERSION=` is set.
- Downloads the tarball + `SHA256SUMS`, runs `sha256sum -c` (fails fast on tamper).
- Extracts to a temp dir, `install -m 0755`'s the binary into `$INSTALL_DIR` (default `~/.local/bin`).
- Prints version after install for sanity.

Failure modes:
- `curl -f` ensures non-2xx HTTP fails the script.
- SHA mismatch → `sha256sum -c` exits non-zero → script aborts before `install`.
- The tmp dir is cleaned up via `trap` regardless.

---

## 8. README + USAGE.md changes

`README.md` — Install section, add a "Pre-built binaries" sub-section ABOVE the existing "From source" block (in both EN and 中文 halves):

```markdown
#### Pre-built binaries (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

Or grab a tarball directly from the [releases page](https://github.com/JimZhang168872/vpnkit/releases) and extract `vpnkit` to a directory on your `PATH`.

#### From source

(existing block unchanged)
```

`docs/USAGE.md` — §1.2 "Build and install vpnkit", add an alternative one-liner before the `make install` block (both languages):

```markdown
The fastest way:

```bash
curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
```

Or build from source:
(existing instructions)
```

---

## 9. Testing & verification plan

**Local dry-run (no upload):**

```bash
# Install goreleaser locally, OR trust the CI (skip local test entirely)
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser release --snapshot --clean
ls dist/
# vpnkit_<snapshot>_linux_amd64.tar.gz
# vpnkit_<snapshot>_linux_arm64.tar.gz
# SHA256SUMS
```

**Live run — first release is a pre-release for validation:**

Going forward, finalised releases use pure-semver tags (`v0.6.0`, `v0.6.1`, `v1.0.0`) and pre-releases use a `-rc<N>` suffix (`v0.6.0-rc1`). `prerelease: auto` in `.goreleaser.yaml` flags the latter as GitHub pre-releases automatically.

For the first release we tag a release candidate so we can verify the pipeline before tagging the canonical `v0.6.0`:

```bash
git tag v0.6.0-rc1 -m "First pre-built release (pipeline verification)"
git push origin v0.6.0-rc1
# Watch https://github.com/JimZhang168872/vpnkit/actions
```

After a successful run, verify:

1. `https://github.com/JimZhang168872/vpnkit/releases/tag/v0.6.0-rc1` exists with 3 assets (2 tar.gz + 1 SHA256SUMS) and is **flagged as Pre-release**.
2. Auto-generated release notes show commits since the prior tag.
3. The install script works end-to-end (need `VERSION=` because the latest endpoint skips pre-releases):

```bash
VERSION=v0.6.0-rc1 bash <(curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh)
~/.local/bin/vpnkit --version
# vpnkit v0.6.0-rc1  (commit <sha>, built <date>)
```

If everything is green, retag as the canonical `v0.6.0` (still goes through the same workflow; that release is a non-prerelease):

```bash
git tag v0.6.0 -m "First pre-built release"
git push origin v0.6.0
```

If anything's off in the rc, fix and force-re-push the same rc tag (GoReleaser's `mode: replace` overwrites assets).

If anything's off, fix and force-re-push (GoReleaser's `mode: replace` overwrites assets).

---

## 10. Open items / decisions deferred

- **Custom changelog template** — defer until the first auto-generated notes look bad in practice.
- **Multi-OS support** — defer; current spec is Linux only.
- **Auto-update for vpnkit itself** — explicitly out of scope (upgrade is `curl install.sh | bash` again).
- **Apt repo / Homebrew tap** — defer; tarball + script is enough for v1.

End of design.
