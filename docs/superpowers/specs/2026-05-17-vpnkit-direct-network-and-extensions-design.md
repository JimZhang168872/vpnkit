# vpnkit: Direct-Only Network + Extensions (Chains & Custom Groups)

**Date:** 2026-05-17
**Status:** Draft (pending user review)
**Owner:** Jim
**Predecessors:**
- `2026-05-15-vpnkit-tui-design.md` (TUI shape)
- `2026-05-16-vpnkit-ctl-design.md` (CLI shape)
- `2026-05-16-vpnkit-auto-update.md` (update flow that touches the mirror layer)

---

## 1. Background & Motivation

### 1.1 Pain points today
1. **Network code is over-engineered for the "user already has a working proxy" case.** The current install/update path threads a mirror fallback through six different layers (`install.sh` `mirror_wrap`, `store.ReleaseMirror`, `netx.BuiltinGitHubMirrors`, `netx.OpenWithFallback`, `installer.ApplyMirror`, `mihomoGeoxURL` with jsdelivr default). For a user whose machine already routes traffic through mihomo (or another VPN), every one of those branches is dead code — but each adds files, tests, CLI flags, TUI rows, and README sections.
2. **`patch.yaml` is the only way to customize mihomo config beyond what `vpnkit` generates from a subscription.** Users have to hand-edit raw YAML and remember mihomo's schema. There's no first-class concept of "I want node A to dial through node B" or "I want an extra proxy-group spanning a subset of subscription nodes."

### 1.2 Goals
1. Strip the entire mirror/fallback apparatus. After this change there is exactly one HTTP path for control-plane traffic: `netx.SmartClient(timeout)` — use mihomo if it's alive, otherwise direct. No public-mirror chain. No `release_mirror` config. No jsdelivr default for geox-url.
2. Add a first-class **Extensions** subsystem with two concepts:
   - **Chains:** `node → upstream-node` mappings that inject `dialer-proxy` into the assembled mihomo config (multi-hop egress).
   - **Custom Groups:** user-defined `proxy-groups` appended after the subscription/synthesized ones.
3. The Extensions subsystem completely replaces `patch.yaml`. The `internal/patch` package and `Settings → Patch Editor` sub-page are deleted.
4. Both Extensions and the surviving control-plane operations (install/update/init/uninstall/status/use/etc.) ship verified-working through every existing CLI command and every TUI tab.

### 1.3 Non-goals
- **Not** building a script engine (no goja/Lua/etc.). Earlier brainstorming considered this; the agreed direction is structured data + UI, not embedded code.
- **Not** preserving backward compatibility with `release_mirror` in `~/.config/vpnkit/config.toml`. On first launch after upgrade the field is silently ignored (BurntSushi/toml decoder ignores unknown keys); next save drops it. No migration shim.
- **Not** supporting first-launch from inside the GFW without a pre-existing working proxy. README will state this explicitly.

---

## 2. Current state (recap)

Mirror logic touches:

| File | What it does today |
|---|---|
| `install.sh` | `mirror_wrap()` prepends `$INSTALL_MIRROR` to GitHub URLs; persists `INSTALL_MIRROR` into config.toml via `--release-mirror` |
| `internal/netx/fallback.go` | `BuiltinGitHubMirrors` (5 hardcoded sites) + `OpenWithFallback` (preferred → direct → builtins, errors.Join all attempts) |
| `internal/netx/fallback_test.go` | Tests for above |
| `internal/installer/release.go` | `ApplyMirror(url, mirror)` + `ReleaseClient` |
| `internal/installer/install.go` | `Options.Mirror`, `Result.Mirror`, passes mirror to Download |
| `internal/installer/download.go` | `Download(...preferredMirror...)` calls `OpenWithFallback` |
| `internal/installer/proxy_regression_test.go` | Asserts SmartClient still respects mirror chain |
| `internal/updater/apply.go` | `DownloadAndApplyVpnkit(...preferredMirror...)` calls `OpenWithFallback` |
| `internal/updater/check.go` | `Check(opts)` accepts `APIBase` (mirror-prefixed by caller) |
| `internal/config/skeleton.go` | `mihomoGeoxURL(mirror)` — jsdelivr default or `mirror+github` |
| `internal/config/reconcile.go` | `SecurityFields.ReleaseMirror` is preserved through ensures |
| `internal/subscription/assemble.go` | Same `mihomoGeoxURL` (duplicate of skeleton's helper, kept in sync by convention) |
| `internal/store/store.go` | `Config.ReleaseMirror` field, written by init and update |
| `internal/tabs/settings/core.go` | TUI row `Mirror : <value or "(direct GitHub)">` |
| `cmd/vpnkit/cmd_init.go` | `--release-mirror` flag → stores into config.toml |
| `cmd/vpnkit/cmd_update.go` | `prefixedAPIBase`, `cacheWinningMirror`, `mirrorAttemptPrinter` |
| `internal/app/run.go` | Wires `st.Cfg.ReleaseMirror` into `profMgr` |
| `internal/app/bootstrap.go` | Passes `d.Store.Cfg.ReleaseMirror` to first-launch installer |
| `internal/app/update_check.go` | Passes mirror to background update poll |
| `README.md`, `README_zh.md` | "Behind the GFW" section with mirror examples |

`patch.yaml` logic touches:

| File | What it does today |
|---|---|
| `internal/patch/patch.go` + `patch_test.go` | Load patch.yaml, apply on top of assembled config |
| `internal/tabs/settings/patch.go` + `patch_test.go` | Settings → Patch Editor sub-page (opens `$EDITOR` on the file) |
| `internal/subscription/assemble.go` | `AssembleInput.PatchPath`; calls `patch.Apply` after merge |
| `internal/profiles/manager.go` | `Config.PatchPath`; passes to Assemble |
| `internal/app/run.go` | Wires `filepath.Join(p.MihomoConfig, "patch.yaml")` |

---

## 3. Design

### 3.1 § 1 — Strip mirror layer

**Code deletions (entire files/packages):**
- `internal/netx/fallback.go`
- `internal/netx/fallback_test.go`

**Code modifications:**

| File | Change |
|---|---|
| `internal/installer/download.go` | Replace `OpenWithFallback` call with `netx.SmartClient(0).Do(http.NewRequestWithContext(ctx, GET, githubURL, nil))`. Drop `preferredMirror` and `onAttempt` parameters from `Download`'s signature. Drop the `winningMirror string` return value. |
| `internal/installer/install.go` | Drop `Options.Mirror`, `Result.Mirror`, `Options.OnAttempt`. Caller no longer passes mirror. |
| `internal/installer/release.go` | Delete `ApplyMirror`. `ReleaseClient` keeps SmartClient (already does the right thing). |
| `internal/installer/proxy_regression_test.go` | Delete (whole file — its purpose was asserting mirror chain behavior). |
| `internal/installer/download_test.go` / `install_test.go` / `release_test.go` | Drop any test cases that pass `Mirror`. Keep cases that exercise SmartClient through a fake httptest server. |
| `internal/updater/apply.go` | Same pattern as download.go: SmartClient + plain GET. Drop `preferredMirror`/`onAttempt` parameters. Drop `winningMirror` return. |
| `internal/updater/apply_test.go` | Drop mirror cases. |
| `internal/updater/check.go` | Keep `Opts.APIBase` (still useful for tests). Drop any logic that builds `mirror+api.github.com`. |
| `internal/updater/check_test.go` | Drop mirror cases. |
| `internal/store/store.go` | Delete `Config.ReleaseMirror` field. Update `defaults()` accordingly. |
| `internal/store/store_test.go` | Drop assertions on the field. |
| `internal/config/skeleton.go` | `mihomoGeoxURL` becomes parameter-less. Returns the direct-GitHub URL map (`https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/...`). Delete jsdelivr default. Update `SkeletonInput` to drop `ReleaseMirror`. |
| `internal/config/skeleton_test.go` | Update expectations to direct GitHub URLs. |
| `internal/config/reconcile.go` | Delete `SecurityFields.ReleaseMirror`. `EnsureSecurityFields` backfill of geox-url uses the new no-arg helper. |
| `internal/config/reconcile_test.go` | Drop mirror cases. |
| `internal/subscription/assemble.go` | Same: parameter-less `mihomoGeoxURL`; drop `AssembleInput.ReleaseMirror`. |
| `internal/subscription/assemble_test.go` | Update. |
| `internal/profiles/manager.go` | Drop `Config.ReleaseMirror`; drop from Assemble call. |
| `internal/profiles/manager_test.go` | Update. |
| `internal/tabs/settings/core.go` | Delete the `Mirror :` row from the View. `coreModel` no longer needs `store` for that purpose (still kept for upgrade button context). `installer.Install` call drops `Mirror:` arg. |
| `internal/tabs/settings/core_test.go` if present | Update / remove mirror assertion. |
| `cmd/vpnkit/main.go` | `dispatchInit`: drop `mirror` flag plumbing. |
| `cmd/vpnkit/cmd_init.go` | Drop `--release-mirror` flag; drop `runInitOpts.ReleaseMirror`; drop `st.Cfg.ReleaseMirror` assignment. |
| `cmd/vpnkit/cmd_init_test.go` | Drop mirror-related cases. |
| `cmd/vpnkit/cmd_update.go` | Delete `prefixedAPIBase`, `cacheWinningMirror`, `mirrorAttemptPrinter`. `upgradeMihomo` / `upgradeVpnkit` no longer pass `Mirror:` or `OnAttempt:`. |
| `internal/app/run.go` | Drop `ReleaseMirror` from `profiles.Config{}`. |
| `internal/app/bootstrap.go` | Drop `Mirror:` from `installer.Options{}`. |
| `internal/app/update_check.go` | Drop mirror arg from background poll. |
| `install.sh` | Delete `mirror_wrap()`. Delete every `$(mirror_wrap "$url")` call (revert to raw URL). Delete `INSTALL_MIRROR` env doc + `--release-mirror` arg to init. |
| `README.md`, `README_zh.md` | Delete the entire "Behind the GFW" / "墙内" section. Add a single line near the install instructions: "Requires a working network path to github.com — if you're inside the GFW, configure a working proxy before installing." |

**Behavioral effect after deletion:**
- All control-plane HTTP that hits GitHub uses `netx.SmartClient(timeout)`. SmartClient already does the right thing: if `$HTTP_PROXY/$HTTPS_PROXY` is set AND the proxy host answers a 500 ms TCP probe, it routes through it; otherwise it goes direct with no proxy. This preserves the "vpnkit is the proxy" use case (mihomo is the proxy reachable on 127.0.0.1) without needing fallback chains.
- `vpnkit init` no longer accepts `--release-mirror`. Old `release_mirror` keys in `~/.config/vpnkit/config.toml` are silently ignored by BurntSushi/toml (unknown field).
- `mihomo` geox-url points directly at GitHub Releases. Inside the GFW on a naked install this will fail on first launch — the README must be updated to say so.

### 3.2 § 2 — `internal/extensions` package

**New package layout:**
```
internal/extensions/
  extensions.go        # Type defs (Chain, Group, Extensions), Load(path), Save(path)
  extensions_test.go
  apply.go             # Apply(doc map[string]any, ext Extensions) error
  apply_test.go
  validate.go          # Validate(ext Extensions, knownNodes []string, knownGroups []string) error
  validate_test.go
```

**On-disk format** (`~/.config/vpnkit/extensions.toml`):
```toml
schema_version = 1

[[chains]]
node = "🇺🇸 US-1"
via  = "🇯🇵 JP-Relay"

[[groups]]
name      = "🎯 Stream"
type      = "select"
proxies   = ["🇺🇸 US-1", "🇯🇵 JP-1", "DIRECT"]
# Optional fields for url-test / fallback / load-balance:
url       = "https://www.gstatic.com/generate_204"
interval  = 300
tolerance = 50
```

**Go types:**
```go
package extensions

type Chain struct {
    Node string `toml:"node"`
    Via  string `toml:"via"`
}

type Group struct {
    Name      string   `toml:"name"`
    Type      string   `toml:"type"`              // select | url-test | fallback | load-balance | relay
    Proxies   []string `toml:"proxies"`
    URL       string   `toml:"url,omitempty"`
    Interval  int      `toml:"interval,omitempty"`
    Tolerance int      `toml:"tolerance,omitempty"`
}

type Extensions struct {
    SchemaVersion int     `toml:"schema_version"`
    Chains        []Chain `toml:"chains"`
    Groups        []Group `toml:"groups"`
}
```

**API surface:**
- `Load(path string) (Extensions, error)` — returns empty `Extensions{}` (no error) if file does not exist.
- `Save(path string, ext Extensions) error` — atomic write (tmp + rename), 0600.
- `Validate(ext Extensions, knownProxyNames []string, knownGroupNames []string) error`
  - rejects `chain.node` or `chain.via` not in `knownProxyNames ∪ knownGroupNames`
  - rejects chain cycles (DFS over the chain DAG)
  - rejects `group.name` colliding with subscription-supplied group names
  - rejects `group.type` outside the supported set
- `Apply(doc map[string]any, ext Extensions) error`
  - mutates `doc["proxies"]` in place: for each chain, find `proxies[i].name == chain.node` and set `proxies[i]["dialer-proxy"] = chain.via`. If the node is not present in doc, log warn (caller decides to surface) and continue.
  - appends `ext.Groups` to `doc["proxy-groups"]`.

### 3.3 § 3 — Assemble flow integration

`internal/subscription/assemble.go`:
- `AssembleInput` drops `PatchPath`, drops `ReleaseMirror`, adds `Extensions extensions.Extensions`.
- After the existing base/proxies/groups/rules merge, call `extensions.Apply(doc, in.Extensions)`. Return error from Apply.

`internal/profiles/manager.go`:
- `Config` drops `PatchPath`, `ReleaseMirror`; adds `ExtensionsPath string`.
- `Update(ctx, name)`:
  1. `extensions.Load(cfg.ExtensionsPath)` — empty struct if file missing.
  2. Pass into `AssembleInput.Extensions`.
- Validation against current `proxies`/`groups` is deferred to `Apply` (only logs warnings on missing refs) — strict validation is the TUI/CLI editor's responsibility before saving.

`internal/app/run.go`:
- Constructs `profMgr` with `ExtensionsPath: filepath.Join(p.VpnkitConfig, "extensions.toml")`.

### 3.4 § 4 — TUI: Settings → Extensions sub-page

**Replaces** Patch Editor in the Settings sub-sidebar:
```go
const (
    SubCore SubPage = iota
    SubService
    SubController
    SubRules
    SubExtensions  // ← was SubPatch
    SubLogs
    SubCache
    SubAbout
    NumSubPages
)
```

**File:** `internal/tabs/settings/extensions.go` (new) + `extensions_test.go`.

**Interaction model:**
```
Extensions
─────────────────────────────────────────────
▶ [c] Chains (2)        │  🇺🇸 US-1   → 🇯🇵 JP-Relay
  [g] Groups (2)        │  🇰🇷 KR-Edge → 🇯🇵 JP-Relay
                        │
                        │  [↑↓] select  [a] add  [e] edit
                        │  [d] delete   [r] apply (reassemble + reload)
                        │  flash: <last action result>
```

- `c`/`g` toggle between Chains list and Groups list.
- `↑↓` scroll within active list.
- `a` opens an inline form:
  - chain form: node (text input with autocomplete from `api.GetProxies()` snapshot) + via (same).
  - group form: name, type (select cycle), proxies (multi-select, autocomplete), url/interval/tolerance (visible only when type is url-test / load-balance / fallback).
- `e` reuses the same form pre-populated.
- `d` removes the highlighted row, persists immediately.
- `r` invokes `extensions.Apply` via a fresh subscription assemble + `applyConfig`; flashes success/fail.

**Data sources for autocomplete:**
- Live `api.GetProxies()` snapshot (already polled by `pollProxies`); fed via existing `msg.ProxiesSnapshot`.

### 3.5 § 5 — CLI commands

**New `cmd/vpnkit/cmd_chain.go`:**
```
vpnkit chain ls [--json]
vpnkit chain set <node> <via>
vpnkit chain unset <node>
```

**New `cmd/vpnkit/cmd_group.go`:**
```
vpnkit group ls [--json]
vpnkit group add <name> --type <select|url-test|fallback|load-balance|relay>
                        --proxies <a,b,c>
                        [--url <u>] [--interval <s>] [--tolerance <ms>]
vpnkit group rm <name>
```

**New `cmd/vpnkit/cmd_ext.go`:**
```
vpnkit ext apply           # reassemble active profile + hot-reload mihomo
```

All read `~/.config/vpnkit/extensions.toml` via the new package; all write through `extensions.Save` (atomic).

`vpnkit chain set` / `group add` validate shape + cycle + type-whitelist only (no dependency on running mihomo or current subscription). Live cross-checking against the running proxy list happens at `vpnkit ext apply` / TUI `r` time, where missing-ref warnings are logged but do not block. `vpnkit ext apply` will fail with a clear error if mihomo is not running.

**Dispatcher in `cmd/vpnkit/main.go`:**
Adds the new top-level verbs: `chain`, `group`, `ext`.

### 3.6 § 6 — Error handling & boundaries

| Failure mode | Behavior |
|---|---|
| extensions.toml corrupt | `Load` returns error; `cmd chain/group ls` exits 2 with the parse error; TUI Extensions sub-page shows the error in place of the list; mihomo run is unaffected (Apply falls through with empty Extensions). |
| chain cycle (A→B→A) | `Validate` rejects on `set` / `add`; existing on-disk cycle survives but `Apply` breaks the cycle by skipping the duplicate and logs a warn. |
| chain references unknown node | `Validate` (shape-only) accepts — there is no live cross-check at CLI time. `Apply` (at `ext apply` / TUI `r` / assemble) silently skips missing refs + log warn (we never want assemble to fail just because a referenced node was dropped from the latest subscription). |
| custom group name collides with subscription group | `Validate` rejects on `add`. |
| custom group references unknown proxy/group | warn only (subscription may change). |
| mihomo not running on `ext apply` | exit 2 with "mihomo is not running; start it first or run from the TUI Service tab". |
| download (install/update) hits a network with no proxy in GFW | propagates the http error; CLI prints "download failed: ..." — README points users to configure a proxy. No fallback chain. |

### 3.7 § 7 — Testing strategy

Coverage targets per `~/.claude/rules/common/workflow.md`: ≥ 80% on every new file.

| Test file | Coverage focus |
|---|---|
| `internal/extensions/extensions_test.go` | Load/Save roundtrip; missing-file → empty; permission 0600 |
| `internal/extensions/apply_test.go` | Chain injects `dialer-proxy`; group appended; missing-node logs warn but doesn't error; group order preserved |
| `internal/extensions/validate_test.go` | Cycle detection; unknown-ref rejection; type whitelist; collision detection |
| `internal/subscription/assemble_test.go` | Adds case "subscription + 2 chains + 1 group" — verifies end-to-end YAML output |
| `internal/profiles/manager_test.go` | `Update` reads extensions, passes through to Assemble |
| `cmd/vpnkit/cmd_chain_test.go` | ls/set/unset; --json output; invalid-input exit codes |
| `cmd/vpnkit/cmd_group_test.go` | ls/add/rm; flag parsing; type whitelist |
| `cmd/vpnkit/cmd_ext_test.go` | apply happy path (with fake api.Client); mihomo-not-running error path |
| `internal/tabs/settings/extensions_test.go` | List rendering; form open/save; autocomplete data flow |
| `internal/installer/download_test.go` (revised) | SmartClient round-trip against `httptest.Server`; no mirror logic |
| `internal/installer/install_test.go` (revised) | Same |
| `internal/updater/apply_test.go` (revised) | Same |
| `internal/updater/check_test.go` (revised) | Same |
| `internal/store/store_test.go` (revised) | No `ReleaseMirror` |
| `internal/config/skeleton_test.go` (revised) | geox-url points at github directly |

**Acceptance gate** (matches user's stated target — "system existing + target features all behave normally"):
- `go test -race -cover ./...` exits 0.
- `go vet ./...` exits 0.
- All CLI commands invoked against a fake controller produce expected output:
  - `vpnkit --version` / `status` / `ip` / `mode <get|set>` / `groups` / `nodes <g>` / `use <g> <n>` / `init` / `update --check` / `uninstall --yes --keep-mihomo` / `env --shell zsh`
  - **new:** `chain ls/set/unset` / `group ls/add/rm` / `ext apply`
- TUI smoke matrix (each verified in a real terminal session):
  - Dashboard renders (traffic, version)
  - Proxies tab: ↑↓ navigate, Enter to switch node, t to delay-test
  - Profiles tab: a to add (form open + close), u to update (subscription fetch + reload), d to delete, Enter to set active
  - Connections tab: ↑↓, / filter, x close
  - Rules tab: ↑↓, /, u refresh providers
  - Settings tab: ↑↓ between all sub-pages; each renders; Core has no Mirror row; Extensions has the new flow
  - Tab cycling: 1/2/3/4/5/6 + Tab/Shift-Tab
  - Quit: q / Ctrl-C

### 3.8 § 8 — `patch.yaml` deletion

| Path | Action |
|---|---|
| `internal/patch/patch.go` | delete |
| `internal/patch/patch_test.go` | delete |
| `internal/tabs/settings/patch.go` | delete |
| `internal/tabs/settings/patch_test.go` | delete |
| `internal/subscription/assemble.go` | delete `PatchPath`, delete `patch.Apply` call |
| `internal/profiles/manager.go` | delete `PatchPath` |
| `internal/app/run.go` | delete `filepath.Join(p.MihomoConfig, "patch.yaml")` and the field that consumed it |
| `internal/tabs/settings/settings.go` | delete `SubPatch` / `newPatch` / wiring (replaced by `SubExtensions` / `newExtensions`) |
| `README*.md` | delete any patch-editor references |

Disk-level migration: existing `~/.config/mihomo/patch.yaml` files are left in place (not deleted); they simply stop being read. Users who want to migrate can copy their tweaks into `extensions.toml`. README notes this.

---

## 4. Implementation order (informs writing-plans next step)

This is design-level; the implementation plan will turn this into TDD-shaped steps. High-level dependency order:

1. **Extensions package** (`internal/extensions/*`) — leaf, no project dependencies.
2. **Assemble integration** — `subscription` + `profiles` + `app/run.go`.
3. **Patch deletion** — done together with §2 so build stays green.
4. **Mirror deletion** — independent of §1-§3 but easier to do after the new path is in place so test churn is bounded. Order: `netx/fallback*` delete → `installer/*` → `updater/*` → `store/config/subscription` field deletions → `tabs/settings/core` → CLI commands → `install.sh` → README.
5. **TUI Extensions sub-page** + **CLI chain/group/ext commands** — last layer, depends on §1.
6. **Documentation + acceptance matrix** — last; updates README and runs the full smoke matrix.

---

## 5. Risks & open questions

| Risk | Mitigation |
|---|---|
| Mirror deletion breaks users who silently relied on `release_mirror` in their config.toml | README ships a 1-line note + example of using `HTTPS_PROXY` env var instead. We chose this over a migration shim per Non-goal 3. |
| jsdelivr default removal breaks first-launch in GFW without a proxy | README explicitly states this requirement. Acceptable per user direction "全部删". |
| `vpnkit chain set` autocomplete data depends on mihomo being up | CLI does not autocomplete; only TUI does. CLI validates against the on-disk extensions.toml plus an optional `--force` flag for cases where the target isn't in current subscription. |
| `dialer-proxy` chain creates a cycle that mihomo can't catch | We validate at `set` time + at `Apply` time (DFS). |
| Schema evolution of `extensions.toml` | `schema_version = 1` baked in; future bumps can migrate. |

**Open questions** (none blocking; flagged so user can override during implementation if they want):
- Should `vpnkit ext apply` be implicit on every `chain set` / `group add` (auto-apply) or explicit? **Decision in spec: explicit** (matches the explicit-action style of `vpnkit init`, `vpnkit update`). User can always run `vpnkit ext apply` as a one-liner after editing.
- Should the TUI Extensions sub-page show the underlying TOML file path? **Decision: yes, in a small footer line** ("file: ~/.config/vpnkit/extensions.toml") so power users can edit it directly.

---

## 6. Acceptance criteria (concrete)

A run of the implementation is "done" when **all** of the following hold:

1. `git grep -i "mirror"` in Go sources returns zero hits in `internal/`, `cmd/`, and `install.sh` (only README is allowed to mention the word in the negative — e.g. "no mirror fallback"). Test files may reference `httptest`/`httpmock` patterns but not GFW mirror sites.
2. `git grep -i "jsdelivr"` returns zero hits anywhere.
3. `git grep "release_mirror"` returns zero hits.
4. `git grep -i "INSTALL_MIRROR"` returns zero hits (including `install.sh`).
5. `git grep -i "patch.yaml"` returns zero hits except in a README migration note.
6. `go test -race -cover ./...` exits 0; every new package ≥ 80% coverage.
7. `go vet ./...` exits 0.
8. `go build ./...` exits 0.
9. Every CLI command in §3.7 acceptance gate runs without error against a fake controller.
10. The TUI smoke matrix in §3.7 is manually verified once on the dev machine; a checklist file is appended to the PR description.

---

*End of spec.*
