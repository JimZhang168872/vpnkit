# vpnkit: Direct-Only Network + Extensions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip the entire mirror/fallback HTTP layer from vpnkit, replace `patch.yaml` with a structured `Extensions` (chains + custom groups) subsystem, and surface it through both TUI and CLI — every existing CLI command and TUI operation must keep working.

**Architecture:**
- Mirror cleanup is purely subtractive: delete `internal/netx/fallback.go`, the `BuiltinGitHubMirrors` list, `Options.Mirror`, `store.ReleaseMirror`, the `--release-mirror` flag, the `INSTALL_MIRROR` env in `install.sh`, the `Mirror` TUI row, and the `mihomoGeoxURL(mirror)` parameter. All control-plane HTTP now uses the existing `netx.SmartClient(timeout)` directly (probes env proxy, falls through to direct).
- Extensions is a new leaf package `internal/extensions` with three small files: types+IO (`extensions.go`), apply (`apply.go`), validate (`validate.go`). Data lives in `~/.config/vpnkit/extensions.toml`.
- Subscription assemble pipeline calls `extensions.Apply(doc, ext)` right before write. `internal/patch` package is deleted in the same step that introduces this hook so the build never breaks.
- TUI: `Settings → Patch Editor` is replaced by `Settings → Extensions` (two-column: Chains / Groups, with add/edit/del/apply keys).
- CLI: new `chain`, `group`, `ext` verbs.

**Tech Stack:** Go 1.22+, `github.com/BurntSushi/toml`, `gopkg.in/yaml.v3`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `httptest` for HTTP tests.

**Spec:** `docs/superpowers/specs/2026-05-17-vpnkit-direct-network-and-extensions-design.md`

---

## Phase 0: Pre-flight

### Task 0.1: Verify baseline green

**Files:** none

- [ ] **Step 1: Run the full test suite to establish a green baseline**

Run:
```bash
go test -race ./...
```
Expected: PASS. If anything fails on `main`, surface to user immediately — don't start the plan on a red branch.

- [ ] **Step 2: Verify build**

Run:
```bash
go vet ./... && go build ./...
```
Expected: no output, exit 0.

---

## Phase 1: Extensions package (leaf, no project deps)

### Task 1.1: Types + Load/Save roundtrip

**Files:**
- Create: `internal/extensions/extensions.go`
- Create: `internal/extensions/extensions_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/extensions/extensions_test.go`:
```go
package extensions

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(filepath.Join(dir, "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load missing: unexpected error %v", err)
	}
	if len(got.Chains) != 0 || len(got.Groups) != 0 {
		t.Fatalf("Load missing: want empty, got %+v", got)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	want := Extensions{
		SchemaVersion: 1,
		Chains: []Chain{
			{Node: "🇺🇸 US-1", Via: "🇯🇵 JP-Relay"},
			{Node: "🇰🇷 KR-Edge", Via: "🇯🇵 JP-Relay"},
		},
		Groups: []Group{
			{
				Name: "🎯 Stream", Type: "select",
				Proxies: []string{"🇺🇸 US-1", "🇯🇵 JP-1", "DIRECT"},
			},
			{
				Name: "♻️ Auto-US", Type: "url-test",
				Proxies:   []string{"🇺🇸 US-1", "🇺🇸 US-2"},
				URL:       "https://www.gstatic.com/generate_204",
				Interval:  300,
				Tolerance: 50,
			},
		},
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm: want 0600, got %o", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roundtrip mismatch:\nwant %+v\n got %+v", want, got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
go test ./internal/extensions/ -run TestLoad -v
```
Expected: FAIL with "no Go files in internal/extensions" or "undefined: Load / Save / Extensions".

- [ ] **Step 3: Write the minimal implementation**

Create `internal/extensions/extensions.go`:
```go
// Package extensions models vpnkit's user-controlled overlay on top of the
// subscription-generated mihomo config: per-node dialer-proxy chains and
// custom proxy-groups. Stored in ~/.config/vpnkit/extensions.toml.
package extensions

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Chain pins one subscription node to dial through an upstream node/group
// (mihomo `dialer-proxy` field). Both Node and Via are mihomo proxy names.
type Chain struct {
	Node string `toml:"node"`
	Via  string `toml:"via"`
}

// Group is a user-defined proxy-group appended to the assembled config after
// any subscription-supplied or synthesized groups.
type Group struct {
	Name      string   `toml:"name"`
	Type      string   `toml:"type"` // select | url-test | fallback | load-balance | relay
	Proxies   []string `toml:"proxies"`
	URL       string   `toml:"url,omitempty"`
	Interval  int      `toml:"interval,omitempty"`
	Tolerance int      `toml:"tolerance,omitempty"`
}

// Extensions is the full content of extensions.toml.
type Extensions struct {
	SchemaVersion int     `toml:"schema_version"`
	Chains        []Chain `toml:"chains"`
	Groups        []Group `toml:"groups"`
}

// Load reads `path`. A missing file is treated as an empty Extensions value
// (not an error) so callers can run before the user has created one.
func Load(path string) (Extensions, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Extensions{}, nil
	}
	if err != nil {
		return Extensions{}, err
	}
	var ext Extensions
	if err := toml.Unmarshal(data, &ext); err != nil {
		return Extensions{}, err
	}
	return ext, nil
}

// Save writes `ext` to `path` atomically (tmp + rename) with mode 0600.
func Save(path string, ext Extensions) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if ext.SchemaVersion == 0 {
		ext.SchemaVersion = 1
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "extensions-*.toml.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Chmod(0o600)
	defer os.Remove(tmpName)
	if err := toml.NewEncoder(tmp).Encode(ext); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
```

- [ ] **Step 4: Run to verify it passes**

Run:
```bash
go test ./internal/extensions/ -run TestLoad -v
go test ./internal/extensions/ -run TestSave -v
go test ./internal/extensions/ -race -cover
```
Expected: PASS, coverage ≥ 80% (only 2 tests so far, more in later tasks).

- [ ] **Step 5: Commit**

```bash
git add internal/extensions/extensions.go internal/extensions/extensions_test.go
git commit -m "feat(extensions): types + Load/Save roundtrip for extensions.toml

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 1.2: Apply — chain injection

**Files:**
- Create: `internal/extensions/apply.go`
- Create: `internal/extensions/apply_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/extensions/apply_test.go`:
```go
package extensions

import (
	"reflect"
	"testing"
)

func TestApplyChainInjectsDialerProxy(t *testing.T) {
	doc := map[string]any{
		"proxies": []any{
			map[string]any{"name": "🇺🇸 US-1", "type": "ss"},
			map[string]any{"name": "🇯🇵 JP-Relay", "type": "ss"},
		},
		"proxy-groups": []any{},
	}
	ext := Extensions{
		Chains: []Chain{{Node: "🇺🇸 US-1", Via: "🇯🇵 JP-Relay"}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	proxies := doc["proxies"].([]any)
	us1 := proxies[0].(map[string]any)
	if us1["dialer-proxy"] != "🇯🇵 JP-Relay" {
		t.Fatalf("dialer-proxy: want 🇯🇵 JP-Relay, got %v", us1["dialer-proxy"])
	}
	jp := proxies[1].(map[string]any)
	if _, has := jp["dialer-proxy"]; has {
		t.Fatalf("JP-Relay should not have dialer-proxy, got %v", jp["dialer-proxy"])
	}
}

func TestApplyChainMissingNodeIsSkippedNotError(t *testing.T) {
	doc := map[string]any{
		"proxies":      []any{map[string]any{"name": "X", "type": "ss"}},
		"proxy-groups": []any{},
	}
	ext := Extensions{
		Chains: []Chain{{Node: "Y", Via: "X"}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: unexpected error %v", err)
	}
	x := doc["proxies"].([]any)[0].(map[string]any)
	if _, has := x["dialer-proxy"]; has {
		t.Fatalf("X should not have been mutated, got %v", x)
	}
}

func TestApplyAppendsGroups(t *testing.T) {
	doc := map[string]any{
		"proxies": []any{},
		"proxy-groups": []any{
			map[string]any{"name": "🚀 Proxy", "type": "select"},
		},
	}
	ext := Extensions{
		Groups: []Group{
			{Name: "🎯 Stream", Type: "select", Proxies: []string{"DIRECT"}},
		},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	groups := doc["proxy-groups"].([]any)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d: %+v", len(groups), groups)
	}
	last := groups[1].(map[string]any)
	if last["name"] != "🎯 Stream" {
		t.Fatalf("last group name: want 🎯 Stream, got %v", last["name"])
	}
	if last["type"] != "select" {
		t.Fatalf("last group type: want select, got %v", last["type"])
	}
	proxies, _ := last["proxies"].([]any)
	if len(proxies) != 1 || proxies[0] != "DIRECT" {
		t.Fatalf("last group proxies: want [DIRECT], got %v", proxies)
	}
}

func TestApplyOptionalFieldsForUrlTestGroup(t *testing.T) {
	doc := map[string]any{
		"proxies":      []any{},
		"proxy-groups": []any{},
	}
	ext := Extensions{
		Groups: []Group{{
			Name:      "♻️ Auto-US",
			Type:      "url-test",
			Proxies:   []string{"a", "b"},
			URL:       "https://www.gstatic.com/generate_204",
			Interval:  300,
			Tolerance: 50,
		}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := doc["proxy-groups"].([]any)[0].(map[string]any)
	want := map[string]any{
		"name":      "♻️ Auto-US",
		"type":      "url-test",
		"proxies":   []any{"a", "b"},
		"url":       "https://www.gstatic.com/generate_204",
		"interval":  300,
		"tolerance": 50,
	}
	if !reflect.DeepEqual(g, want) {
		t.Fatalf("group:\nwant %+v\n got %+v", want, g)
	}
}

func TestApplyNoProxyGroupsKeyInitializes(t *testing.T) {
	doc := map[string]any{"proxies": []any{}}
	ext := Extensions{
		Groups: []Group{{Name: "X", Type: "select", Proxies: []string{"DIRECT"}}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, has := doc["proxy-groups"]; !has {
		t.Fatalf("proxy-groups key not created")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
go test ./internal/extensions/ -run TestApply -v
```
Expected: FAIL with "undefined: Apply".

- [ ] **Step 3: Write the minimal implementation**

Create `internal/extensions/apply.go`:
```go
package extensions

// Apply mutates `doc` (an unmarshalled mihomo config) in place to reflect
// `ext`. Specifically:
//   - For each Chain, find the proxy whose name == chain.Node in doc["proxies"]
//     and set its "dialer-proxy" key to chain.Via. Missing nodes are skipped
//     silently (subscription may not include the node anymore).
//   - Each Group is appended to doc["proxy-groups"], preserving order.
//     If doc has no "proxy-groups" key it is initialized to an empty slice
//     before appending.
//
// Returns nil unless `doc` is structurally malformed (e.g. proxies is not a
// slice of maps). Live cross-checking against known proxies happens earlier
// in Validate.
func Apply(doc map[string]any, ext Extensions) error {
	if len(ext.Chains) > 0 {
		proxies, _ := doc["proxies"].([]any)
		index := map[string]map[string]any{}
		for _, p := range proxies {
			m, ok := p.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			if name != "" {
				index[name] = m
			}
		}
		for _, c := range ext.Chains {
			if m, ok := index[c.Node]; ok {
				m["dialer-proxy"] = c.Via
			}
		}
	}
	if len(ext.Groups) > 0 {
		groups, _ := doc["proxy-groups"].([]any)
		for _, g := range ext.Groups {
			groups = append(groups, groupToMap(g))
		}
		doc["proxy-groups"] = groups
	}
	return nil
}

// groupToMap converts a Group to mihomo's untyped map shape so it round-trips
// through yaml.v3 without surprises.
func groupToMap(g Group) map[string]any {
	out := map[string]any{
		"name":    g.Name,
		"type":    g.Type,
		"proxies": stringsToAny(g.Proxies),
	}
	if g.URL != "" {
		out["url"] = g.URL
	}
	if g.Interval != 0 {
		out["interval"] = g.Interval
	}
	if g.Tolerance != 0 {
		out["tolerance"] = g.Tolerance
	}
	return out
}

func stringsToAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
```

- [ ] **Step 4: Run to verify it passes**

Run:
```bash
go test ./internal/extensions/ -race -cover -v
```
Expected: ALL tests pass; coverage ≥ 80% on the package.

- [ ] **Step 5: Commit**

```bash
git add internal/extensions/apply.go internal/extensions/apply_test.go
git commit -m "feat(extensions): Apply injects dialer-proxy chains and appends groups

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 1.3: Validate — cycles, type whitelist, collisions

**Files:**
- Create: `internal/extensions/validate.go`
- Create: `internal/extensions/validate_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/extensions/validate_test.go`:
```go
package extensions

import (
	"strings"
	"testing"
)

func TestValidateShapeAcceptsValidExt(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{{Node: "A", Via: "B"}},
		Groups: []Group{
			{Name: "G", Type: "select", Proxies: []string{"DIRECT"}},
		},
	}
	if err := Validate(ext); err != nil {
		t.Fatalf("Validate: unexpected error %v", err)
	}
}

func TestValidateRejectsEmptyChainNode(t *testing.T) {
	ext := Extensions{Chains: []Chain{{Node: "", Via: "B"}}}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "chain.node empty") {
		t.Fatalf("want 'chain.node empty' error, got %v", err)
	}
}

func TestValidateRejectsEmptyChainVia(t *testing.T) {
	ext := Extensions{Chains: []Chain{{Node: "A", Via: ""}}}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "chain.via empty") {
		t.Fatalf("want 'chain.via empty' error, got %v", err)
	}
}

func TestValidateRejectsSelfChain(t *testing.T) {
	ext := Extensions{Chains: []Chain{{Node: "A", Via: "A"}}}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestValidateRejectsTwoNodeCycle(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "B", Via: "A"},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestValidateRejectsThreeNodeCycle(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "B", Via: "C"},
			{Node: "C", Via: "A"},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestValidateAcceptsLinearChain(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "B", Via: "C"},
		},
	}
	if err := Validate(ext); err != nil {
		t.Fatalf("linear chain should be accepted: %v", err)
	}
}

func TestValidateRejectsDuplicateChainNode(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "A", Via: "C"},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate error, got %v", err)
	}
}

func TestValidateRejectsUnknownGroupType(t *testing.T) {
	ext := Extensions{
		Groups: []Group{{Name: "X", Type: "weird", Proxies: []string{"a"}}},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("want type error, got %v", err)
	}
}

func TestValidateAcceptsAllSupportedGroupTypes(t *testing.T) {
	for _, typ := range []string{"select", "url-test", "fallback", "load-balance", "relay"} {
		ext := Extensions{
			Groups: []Group{{Name: "X", Type: typ, Proxies: []string{"a"}}},
		}
		if err := Validate(ext); err != nil {
			t.Fatalf("type %q rejected: %v", typ, err)
		}
	}
}

func TestValidateRejectsEmptyGroupName(t *testing.T) {
	ext := Extensions{
		Groups: []Group{{Name: "", Type: "select", Proxies: []string{"a"}}},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("want name error, got %v", err)
	}
}

func TestValidateRejectsDuplicateGroupName(t *testing.T) {
	ext := Extensions{
		Groups: []Group{
			{Name: "G", Type: "select", Proxies: []string{"a"}},
			{Name: "G", Type: "select", Proxies: []string{"b"}},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate error, got %v", err)
	}
}

func TestValidateRejectsEmptyGroupProxies(t *testing.T) {
	ext := Extensions{
		Groups: []Group{{Name: "G", Type: "select", Proxies: nil}},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "proxies") {
		t.Fatalf("want proxies error, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
go test ./internal/extensions/ -run TestValidate -v
```
Expected: FAIL with "undefined: Validate".

- [ ] **Step 3: Write the minimal implementation**

Create `internal/extensions/validate.go`:
```go
package extensions

import (
	"errors"
	"fmt"
)

// supportedGroupTypes lists the mihomo proxy-group types vpnkit's CLI/TUI
// can safely round-trip. mihomo supports more (e.g. select with `lazy`) but
// the Group struct is intentionally a small subset.
var supportedGroupTypes = map[string]bool{
	"select":       true,
	"url-test":     true,
	"fallback":     true,
	"load-balance": true,
	"relay":        true,
}

// Validate performs shape-only checks: no missing required fields, no
// duplicate chain origins or group names, no chain cycles, group type
// within the supported whitelist. It does NOT cross-check Chain.Node /
// Chain.Via / Group.Proxies against the running mihomo's proxy list —
// that happens at Apply time as a warn-only path.
func Validate(ext Extensions) error {
	if err := validateChains(ext.Chains); err != nil {
		return err
	}
	if err := validateGroups(ext.Groups); err != nil {
		return err
	}
	return nil
}

func validateChains(chains []Chain) error {
	seen := map[string]bool{}
	graph := map[string]string{}
	for i, c := range chains {
		if c.Node == "" {
			return fmt.Errorf("chains[%d]: chain.node empty", i)
		}
		if c.Via == "" {
			return fmt.Errorf("chains[%d]: chain.via empty", i)
		}
		if seen[c.Node] {
			return fmt.Errorf("chains[%d]: duplicate node %q", i, c.Node)
		}
		seen[c.Node] = true
		graph[c.Node] = c.Via
	}
	// DFS each node; if we revisit a node already on the current stack, cycle.
	for start := range graph {
		stack := map[string]bool{}
		cur := start
		for {
			if stack[cur] {
				return fmt.Errorf("chain cycle detected at node %q", cur)
			}
			stack[cur] = true
			next, ok := graph[cur]
			if !ok {
				break
			}
			cur = next
		}
	}
	return nil
}

func validateGroups(groups []Group) error {
	seen := map[string]bool{}
	for i, g := range groups {
		if g.Name == "" {
			return fmt.Errorf("groups[%d]: name empty", i)
		}
		if seen[g.Name] {
			return fmt.Errorf("groups[%d]: duplicate name %q", i, g.Name)
		}
		seen[g.Name] = true
		if !supportedGroupTypes[g.Type] {
			return fmt.Errorf("groups[%d]: unsupported type %q (want one of select|url-test|fallback|load-balance|relay)", i, g.Type)
		}
		if len(g.Proxies) == 0 {
			return fmt.Errorf("groups[%d] %q: proxies must be non-empty", i, g.Name)
		}
	}
	return nil
}

// ErrCycle is returned when chain validation finds a cycle. Exposed so
// CLI/TUI callers can match without parsing the message.
var ErrCycle = errors.New("chain cycle")
```

- [ ] **Step 4: Run to verify it passes**

Run:
```bash
go test ./internal/extensions/ -race -cover -v
```
Expected: ALL pass; coverage ≥ 80%.

- [ ] **Step 5: Commit**

```bash
git add internal/extensions/validate.go internal/extensions/validate_test.go
git commit -m "feat(extensions): Validate shape, cycles, duplicates, type whitelist

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 2: Assemble integration + patch deletion

Done in one phase so build never breaks: every step removes/adds related pieces together.

### Task 2.1: Subscription assemble integrates Extensions

**Files:**
- Modify: `internal/subscription/assemble.go`
- Modify: `internal/subscription/assemble_test.go`

- [ ] **Step 1: Write the failing test (extends existing test file)**

Append to `internal/subscription/assemble_test.go`:
```go
func TestAssembleAppliesExtensions(t *testing.T) {
	in := AssembleInput{
		Result: Result{
			Source: "uri",
			Proxies: []proto.Proxy{
				{"name": "🇺🇸 US-1", "type": "ss", "server": "x", "port": 1},
				{"name": "🇯🇵 JP-Relay", "type": "ss", "server": "y", "port": 2},
			},
		},
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		RuleTemplate:     "loyalsoldier",
		Extensions: extensions.Extensions{
			Chains: []extensions.Chain{
				{Node: "🇺🇸 US-1", Via: "🇯🇵 JP-Relay"},
			},
			Groups: []extensions.Group{
				{Name: "🎯 Stream", Type: "select", Proxies: []string{"🇺🇸 US-1", "DIRECT"}},
			},
		},
	}
	out, err := Assemble(in)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "dialer-proxy: 🇯🇵 JP-Relay") {
		t.Fatalf("expected dialer-proxy line, got:\n%s", s)
	}
	if !strings.Contains(s, "name: 🎯 Stream") {
		t.Fatalf("expected custom group 🎯 Stream, got:\n%s", s)
	}
}
```

If the existing test file doesn't import `strings` or the `extensions` package, add to the imports:
```go
import (
	"strings"
	"testing"

	"vpnkit/internal/extensions"
	"vpnkit/internal/subscription/proto"
)
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
go test ./internal/subscription/ -run TestAssembleApplies -v
```
Expected: FAIL — `AssembleInput` has no field `Extensions`.

- [ ] **Step 3: Modify implementation**

Modify `internal/subscription/assemble.go`:

(a) Add to imports:
```go
"vpnkit/internal/extensions"
```

(b) Modify `AssembleInput`:
- Remove field `PatchPath string`
- Remove field `ReleaseMirror string`
- Add field `Extensions extensions.Extensions`

(c) In `Assemble`, replace the `patch.Apply(...)` block with `extensions.Apply(doc, in.Extensions)`, and replace `mihomoGeoxURL(in.ReleaseMirror)` with `mihomoGeoxURL()`. Final shape:

```go
// Assemble produces the final config.yaml bytes by combining:
// base skeleton + subscription proxies + groups (synthesized or from clash) +
// rules + extensions overlay (chains + custom groups).
func Assemble(in AssembleInput) ([]byte, error) {
	if in.MixedPort == 0 {
		in.MixedPort = 7890
	}
	if in.ControllerPort == 0 {
		in.ControllerPort = 9090
	}
	if in.LogLevel == "" {
		in.LogLevel = "info"
	}
	ruleYAML, err := rules.Load(in.RuleTemplate)
	if err != nil {
		return nil, err
	}
	var ruleDoc map[string]any
	if err := yaml.Unmarshal(ruleYAML, &ruleDoc); err != nil {
		return nil, fmt.Errorf("rule template parse: %w", err)
	}

	doc := map[string]any{
		"mixed-port":          in.MixedPort,
		"allow-lan":           false,
		"bind-address":        "127.0.0.1",
		"mode":                "rule",
		"log-level":           in.LogLevel,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", in.ControllerPort),
		"secret":              in.ControllerSecret,
	}
	if in.ProxyUser != "" && in.ProxyPass != "" {
		doc["authentication"] = []string{in.ProxyUser + ":" + in.ProxyPass}
	}

	rawProxies := make([]any, 0, len(in.Result.Proxies))
	for _, p := range in.Result.Proxies {
		rawProxies = append(rawProxies, map[string]any(p))
	}
	doc["proxies"] = rawProxies

	if in.Result.Source == "clash" && in.Result.Raw != nil {
		if g, ok := in.Result.Raw["proxy-groups"]; ok {
			doc["proxy-groups"] = g
		}
	}
	if _, has := doc["proxy-groups"]; !has {
		doc["proxy-groups"] = groupsToAny(SynthesizeGroups(in.Result.Proxies))
	}

	for k, v := range ruleDoc {
		doc[k] = v
	}

	doc["geox-url"] = mihomoGeoxURL()

	if err := extensions.Apply(doc, in.Extensions); err != nil {
		return nil, fmt.Errorf("extensions: %w", err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	_ = enc.Close()

	result := strings.NewReplacer(
		`\U0001F680`, "🚀",
		`\U0001F3AF`, "🎯",
		`\U0001F6D1`, "🛑",
		`\U000267B`, "♻️",
	).Replace(string(buf.Bytes()))
	return []byte(result), nil
}
```

(d) Also remove the import line `"vpnkit/internal/patch"` and the `mihomoGeoxURL(mirror string)` body. Replace `mihomoGeoxURL` with:
```go
// mihomoGeoxURL returns the geox-url map for mihomo, pointing at
// MetaCubeX/meta-rules-dat GitHub Releases directly. No mirror layer.
func mihomoGeoxURL() map[string]string {
	const base = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	return map[string]string{
		"geoip":   base + "/geoip.metadb",
		"mmdb":    base + "/country.mmdb",
		"geosite": base + "/geosite.dat",
		"asn":     base + "/GeoLite2-ASN.mmdb",
	}
}
```

- [ ] **Step 4: Run to verify it passes (and old tests still pass)**

Run:
```bash
go test ./internal/subscription/ -race -v
```
Expected: ALL pass.

- [ ] **Step 5: Commit**

```bash
git add internal/subscription/assemble.go internal/subscription/assemble_test.go
git commit -m "feat(subscription): Assemble applies extensions, direct GitHub geox-url

- AssembleInput.PatchPath / ReleaseMirror removed
- AssembleInput.Extensions added (chains + custom groups)
- mihomoGeoxURL() parameter-less, points at GitHub Releases directly

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 2.2: profiles.Manager loads extensions and threads through Assemble

**Files:**
- Modify: `internal/profiles/manager.go`
- Modify: `internal/profiles/manager_test.go`

- [ ] **Step 1: Write the failing test (extends existing test file)**

Add to `internal/profiles/manager_test.go`:
```go
func TestManagerUpdateWiresExtensions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	extPath := filepath.Join(dir, "extensions.toml")

	// Stand up a fake subscription returning one node.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ss://YWVzLTI1Ni1nY206cGFzcw==@1.2.3.4:8388#NodeA")
	}))
	defer srv.Close()

	// Pre-write an extensions.toml that chains NodeA → DIRECT (no-op but
	// confirms the field reaches Assemble).
	if err := extensions.Save(extPath, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "NodeA", Via: "DIRECT"}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m := New(Config{
		ConfigYAMLPath:   cfgPath,
		ExtensionsPath:   extPath,
		ControllerPort:   9090,
		ControllerSecret: "s",
		MixedPort:        7890,
		RuleTemplate:     "loyalsoldier",
	})
	if err := m.Add(Profile{Name: "p", URL: srv.URL}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := m.Update(context.Background(), "p"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "dialer-proxy: DIRECT") {
		t.Fatalf("expected dialer-proxy line in config.yaml, got:\n%s", data)
	}
}
```

Add the new imports if missing: `"net/http"`, `"net/http/httptest"`, `"path/filepath"`, `"os"`, `"strings"`, `"vpnkit/internal/extensions"`.

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
go test ./internal/profiles/ -run TestManagerUpdateWires -v
```
Expected: FAIL — `Config.ExtensionsPath` does not exist; `Config.PatchPath` may also still exist as old code path.

- [ ] **Step 3: Modify implementation**

Modify `internal/profiles/manager.go`:

(a) Add to imports: `"vpnkit/internal/extensions"`

(b) In the `Config` struct, **remove**:
- `PatchPath string`
- `ReleaseMirror string`

and **add**:
- `ExtensionsPath string`

(c) In `Update`, replace the AssembleInput construction:
```go
ext, _ := extensions.Load(cfg.ExtensionsPath)
yamlBytes, err := subscription.Assemble(subscription.AssembleInput{
	Result:           res,
	MixedPort:        cfg.MixedPort,
	ControllerPort:   cfg.ControllerPort,
	ControllerSecret: cfg.ControllerSecret,
	RuleTemplate:     cfg.RuleTemplate,
	Extensions:       ext,
	ProxyUser:        cfg.ProxyUser,
	ProxyPass:        cfg.ProxyPass,
})
```

Note `PatchPath` and `ReleaseMirror` are dropped.

- [ ] **Step 4: Run to verify it passes**

Run:
```bash
go test ./internal/profiles/ -race -v
```
Expected: ALL pass.

- [ ] **Step 5: Commit**

```bash
git add internal/profiles/manager.go internal/profiles/manager_test.go
git commit -m "feat(profiles): Manager threads ExtensionsPath into Assemble

PatchPath + ReleaseMirror config fields removed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 2.3: app.run.go wires ExtensionsPath, drops Patch wiring

**Files:**
- Modify: `internal/app/run.go`

- [ ] **Step 1: Read current `profMgr` construction**

Read `internal/app/run.go:60-71` — the `profiles.New(profiles.Config{...})` call.

- [ ] **Step 2: Modify the call**

Replace:
```go
profMgr := profiles.New(profiles.Config{
	ConfigYAMLPath:   p.MihomoConfigFile(),
	PatchPath:        filepath.Join(p.MihomoConfig, "patch.yaml"),
	ControllerPort:   st.Cfg.ControllerPort,
	ControllerSecret: st.Cfg.ControllerSecret,
	MixedPort:        st.Cfg.MixedPort,
	RuleTemplate:     st.Cfg.RuleTemplate,
	ReleaseMirror:    st.Cfg.ReleaseMirror,
	ProxyUser:        st.Cfg.ProxyUser,
	ProxyPass:        st.Cfg.ProxyPass,
})
```

With:
```go
profMgr := profiles.New(profiles.Config{
	ConfigYAMLPath:   p.MihomoConfigFile(),
	ExtensionsPath:   filepath.Join(p.VpnkitConfig, "extensions.toml"),
	ControllerPort:   st.Cfg.ControllerPort,
	ControllerSecret: st.Cfg.ControllerSecret,
	MixedPort:        st.Cfg.MixedPort,
	RuleTemplate:     st.Cfg.RuleTemplate,
	ProxyUser:        st.Cfg.ProxyUser,
	ProxyPass:        st.Cfg.ProxyPass,
})
```

Note: If `p.VpnkitConfig` doesn't exist as a field on `paths.XDG`, derive the path another way: use `filepath.Dir(p.VpnkitConfigFile())`. Read `internal/paths/paths.go` to confirm the correct accessor. If only `VpnkitConfigFile()` exists, use `filepath.Join(filepath.Dir(p.VpnkitConfigFile()), "extensions.toml")`.

- [ ] **Step 3: Verify build (ReleaseMirror in store.Cfg still exists at this point — that's fine; it'll be deleted in Phase 3)**

Run:
```bash
go build ./...
go vet ./...
```
Expected: clean.

- [ ] **Step 4: Run the tests**

Run:
```bash
go test ./... -race
```
Expected: ALL pass (test files compiled may still reference `PatchPath` in other places — fix any that show up; specifically `cmd/vpnkit/cmd_init_test.go` does not touch profiles.Config so should be fine).

- [ ] **Step 5: Commit**

```bash
git add internal/app/run.go
git commit -m "feat(app): profMgr uses ExtensionsPath instead of PatchPath

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 2.4: Delete internal/patch package and Settings → Patch sub-page

**Files:**
- Delete: `internal/patch/patch.go`
- Delete: `internal/patch/patch_test.go`
- Delete: `internal/tabs/settings/patch.go`
- Delete: `internal/tabs/settings/patch_test.go`
- Modify: `internal/tabs/settings/settings.go`
- Modify: `internal/tabs/settings/settings_test.go`

- [ ] **Step 1: Verify the patch package is no longer imported**

Run:
```bash
grep -rn "vpnkit/internal/patch" --include="*.go"
```
Expected: returns only the file `internal/tabs/settings/patch.go` (and possibly that file's test). If any other file is found, fix it FIRST before deletion.

- [ ] **Step 2: Delete the four files**

```bash
git rm internal/patch/patch.go internal/patch/patch_test.go \
       internal/tabs/settings/patch.go internal/tabs/settings/patch_test.go
rmdir internal/patch  # may fail if directory not empty — that's OK
```

- [ ] **Step 3: Modify `internal/tabs/settings/settings.go`**

Replace:
```go
const (
	SubCore SubPage = iota
	SubService
	SubController
	SubRules
	SubPatch
	SubLogs
	SubCache
	SubAbout
	NumSubPages
)

var SubPageNames = [NumSubPages]string{
	"Mihomo Core",
	"Service",
	"External Controller",
	"Default Rules",
	"Patch Editor",
	"Logs",
	"Cache",
	"About",
}
```

With (temporarily — full Extensions wiring lands in Phase 4):
```go
const (
	SubCore SubPage = iota
	SubService
	SubController
	SubRules
	SubExtensions
	SubLogs
	SubCache
	SubAbout
	NumSubPages
)

var SubPageNames = [NumSubPages]string{
	"Mihomo Core",
	"Service",
	"External Controller",
	"Default Rules",
	"Extensions",
	"Logs",
	"Cache",
	"About",
}
```

Then remove the `patch patchModel` field from `Model`, remove `patch: newPatch(...)` from `New`, remove the `case SubPatch:` branches from `Update` and `View`. Leave the SubExtensions case empty for now (just render placeholder text). Add a temporary placeholder:
```go
case SubExtensions:
	// Real impl in Phase 4 — render placeholder.
	body = lipgloss.NewStyle().Width(bodyWidth).Height(height).Padding(1, 2).
		Render("Extensions: see Phase 4")
```

And in `Update`, leave SubExtensions out of the switch (no-op).

- [ ] **Step 4: Update settings_test.go**

Adjust any references to `SubPatch` → `SubExtensions`. If `settings_test.go` asserted page count or names, update them.

Run:
```bash
go test ./internal/tabs/settings/ -race -v
```
Expected: PASS.

- [ ] **Step 5: Verify build green**

Run:
```bash
go build ./...
go test -race ./...
```
Expected: PASS (any cross-package references to `patch` or `SubPatch` surface now; fix in this commit).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(patch): delete internal/patch + Settings Patch sub-page

Replaced by SubExtensions placeholder; full Extensions sub-page in Phase 4.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 3: Mirror cleanup (surgical, commit-by-commit)

Each task in this phase is one logical commit. Order chosen so the build stays green after every commit.

### Task 3.1: Rewrite installer/download.go to SmartClient + direct GET

**Files:**
- Modify: `internal/installer/download.go`
- Modify: `internal/installer/download_test.go`

- [ ] **Step 1: Update the test first**

Read `internal/installer/download_test.go`. Identify tests that pass `preferredMirror` or `onAttempt`. Rewrite each to use the new signature (no mirror, no onAttempt, returns only error).

Example new shape of one test:
```go
func TestDownloadDirectGitHub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve a tiny gzip-of-"hello"
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte("hello"))
		_ = gz.Close()
	}))
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "out")
	if err := Download(srv.URL, "", dst, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "hello" {
		t.Fatalf("want hello, got %q", data)
	}
}
```

Delete every test case that exists *only* to verify mirror behavior.

- [ ] **Step 2: Run to verify failure**

Run:
```bash
go test ./internal/installer/ -run TestDownload -v
```
Expected: FAIL (signature mismatch).

- [ ] **Step 3: Rewrite `internal/installer/download.go`**

```go
package installer

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"vpnkit/internal/netx"
)

// ProgressFunc reports bytes downloaded so far and the total expected (-1 if unknown).
type ProgressFunc func(n, total int64)

// Download fetches a gzipped mihomo binary directly from githubURL using
// netx.SmartClient (which honors a live env proxy if one is reachable, else
// goes direct). Verifies SHA256 of the raw gzip stream against expectedSHA
// (hex; empty = skip check), decompresses, and writes the resulting
// executable atomically to dst with mode 0o755.
//
// There is no mirror fallback chain. If the GET fails, the error is returned
// as-is; callers should surface it to the user with a hint to configure a
// proxy if they're inside a restricted network.
func Download(githubURL, expectedSHA, dst string, progress ProgressFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubURL, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", githubURL, err)
	}
	client := netx.SmartClient(0)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", githubURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: HTTP %s", githubURL, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "mihomo-*.dl")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { tmp.Close(); os.Remove(tmpName) }

	hasher := sha256.New()
	reader := io.TeeReader(resp.Body, hasher)
	gz, err := gzip.NewReader(progressReader(reader, -1, progress))
	if err != nil {
		cleanup()
		return err
	}
	if _, err := io.Copy(tmp, gz); err != nil {
		cleanup()
		return err
	}
	if err := gz.Close(); err != nil {
		cleanup()
		return err
	}
	if expectedSHA != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedSHA {
			cleanup()
			return fmt.Errorf("sha256 mismatch: got %s expected %s", got, expectedSHA)
		}
	}
	if err := tmp.Chmod(0o755); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func progressReader(r io.Reader, total int64, cb ProgressFunc) io.Reader {
	if cb == nil {
		return r
	}
	return &progressR{r: r, total: total, cb: cb}
}

type progressR struct {
	r        io.Reader
	total    int64
	read     int64
	cb       ProgressFunc
	lastEmit int64
}

func (p *progressR) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.read-p.lastEmit > 64*1024 || err == io.EOF {
		p.cb(p.read, p.total)
		p.lastEmit = p.read
	}
	if errors.Is(err, io.EOF) {
		return n, err
	}
	return n, err
}
```

- [ ] **Step 4: Update callers**

Search:
```bash
grep -rn "installer.Download\|installer\\.Download" --include="*.go"
```

Update `internal/installer/install.go`'s call site to the new signature: `if err := Download(url, "", opts.Dst, progress); err != nil { ... }` — drop `opts.Mirror`, drop `opts.OnAttempt`, drop the `winningMirror` return capture.

If `install.go` references `Result.Mirror`, that's removed in the next task — leave the field for now but set it to "".

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/installer/ -race -v
```
Expected: PASS (any remaining mirror-coupled tests will fail; remove or update them now).

- [ ] **Step 6: Commit**

```bash
git add internal/installer/download.go internal/installer/download_test.go internal/installer/install.go
git commit -m "refactor(installer): Download drops mirror fallback, uses SmartClient

Direct GET via netx.SmartClient (probes env proxy / direct).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.2: Trim installer.Install and ApplyMirror

**Files:**
- Modify: `internal/installer/install.go`
- Modify: `internal/installer/install_test.go`
- Modify: `internal/installer/release.go`
- Modify: `internal/installer/release_test.go`
- Delete: `internal/installer/proxy_regression_test.go`

- [ ] **Step 1: Update install_test.go**

Remove any test cases that pass `Mirror:` in `installer.Options`. Update remaining tests so their assertions don't reference `Result.Mirror`.

- [ ] **Step 2: Run to verify failures (or success — depends on existing tests)**

Run:
```bash
go test ./internal/installer/ -v
```

- [ ] **Step 3: Modify `install.go`**

Remove `Options.Mirror`, `Options.OnAttempt`, `Result.Mirror` fields.

Final `Install`:
```go
// Install runs the full flow: resolve release → choose asset → download → unpack → rename.
// No mirror layer. Errors from the network call propagate unchanged.
func Install(opts Options, progress ProgressFunc) (Result, error) {
	if opts.Dst == "" {
		return Result{}, fmt.Errorf("install: Dst is required")
	}
	rc := NewReleaseClient(opts.APIBase, opts.Token)
	var rel Release
	var err error
	if opts.Version == "" {
		rel, err = rc.Latest()
	} else {
		rel, err = rc.ByTag(opts.Version)
	}
	if err != nil {
		return Result{}, fmt.Errorf("install: fetch release: %w", err)
	}

	compat := false
	if opts.ForceCompat != nil {
		compat = *opts.ForceCompat
	} else {
		compat = NeedsCompatibleBuild()
	}
	name := assetName(currentArch(), compat, rel.Tag)
	url, err := rel.AssetURL(name)
	if err != nil {
		altName := assetName(currentArch(), !compat, rel.Tag)
		alt, altErr := rel.AssetURL(altName)
		if altErr != nil {
			return Result{}, fmt.Errorf("install: %w", err)
		}
		url = alt
		compat = !compat
	}

	if err := Download(url, "", opts.Dst, progress); err != nil {
		return Result{}, fmt.Errorf("install: download: %w", err)
	}
	return Result{Version: rel.Tag, Compatible: compat}, nil
}

type Options struct {
	APIBase     string
	Token       string
	Dst         string
	Version     string
	ForceCompat *bool
}

type Result struct {
	Version    string
	Compatible bool
}
```

- [ ] **Step 4: Modify `release.go`**

Delete the `ApplyMirror` function. Keep the rest.

- [ ] **Step 5: Update `release_test.go`**

Delete any tests of `ApplyMirror`.

- [ ] **Step 6: Delete `proxy_regression_test.go`**

```bash
git rm internal/installer/proxy_regression_test.go
```

- [ ] **Step 7: Find and fix all call sites that pass `Mirror` or read `Result.Mirror`**

Run:
```bash
grep -rn "installer.Options{" --include="*.go"
grep -rn "Result.Mirror\|res.Mirror" --include="*.go"
```

For each hit (likely `internal/tabs/settings/core.go`, `internal/app/bootstrap.go`, `internal/app/update_check.go`, `cmd/vpnkit/cmd_update.go`):
- Drop the `Mirror:` field from any `installer.Options{...}` literal.
- Drop the `OnAttempt:` field.
- Drop the `cacheWinningMirror(out, st, res.Mirror)` line.

These edits should be tiny — surgical 1-2 line removals.

- [ ] **Step 8: Build & test**

Run:
```bash
go build ./...
go test ./... -race
```
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "refactor(installer): drop Options.Mirror, Result.Mirror, ApplyMirror, OnAttempt

All call sites updated. proxy_regression_test deleted (mirror-only fixture).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.3: Delete internal/netx/fallback.go + test

**Files:**
- Delete: `internal/netx/fallback.go`
- Delete: `internal/netx/fallback_test.go`

- [ ] **Step 1: Verify no callers remain**

Run:
```bash
grep -rn "netx.OpenWithFallback\|BuiltinGitHubMirrors\|netx.OnAttempt" --include="*.go"
```
Expected: zero hits in non-test files (deletions in Task 3.2 should have removed them). If any remain, fix them first.

- [ ] **Step 2: Delete**

```bash
git rm internal/netx/fallback.go internal/netx/fallback_test.go
```

- [ ] **Step 3: Verify build & tests**

Run:
```bash
go build ./...
go test ./... -race
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(netx): delete fallback.go + BuiltinGitHubMirrors

No callers remain after installer/updater cleanup.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.4: Trim updater/apply.go (drop OpenWithFallback)

**Files:**
- Modify: `internal/updater/apply.go`
- Modify: `internal/updater/apply_test.go`

- [ ] **Step 1: Update apply_test.go**

Remove `preferredMirror` and `onAttempt` args from all calls; remove `winningMirror` return capture. Drop mirror-only test cases.

- [ ] **Step 2: Rewrite the function**

Replace `DownloadAndApplyVpnkit` body in `internal/updater/apply.go`:

```go
// DownloadAndApplyVpnkit fetches the .tar.gz at `githubURL` directly using
// netx.SmartClient (probes env proxy / direct), optionally verifies SHA256 of
// the raw tarball stream against `expectedSHA` (hex; empty = skip), extracts
// the inner `vpnkit` file, and atomically replaces `dstPath`.
//
// On SHA mismatch the existing binary at dstPath is left untouched.
func DownloadAndApplyVpnkit(githubURL, expectedSHA, dstPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubURL, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", githubURL, err)
	}
	client := netx.SmartClient(0)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", githubURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: HTTP %s", githubURL, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	tmpTarball, err := os.CreateTemp(filepath.Dir(dstPath), "vpnkit-up-*.tar.gz")
	if err != nil {
		return err
	}
	tmpTarballName := tmpTarball.Name()
	defer os.Remove(tmpTarballName)

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmpTarball, hasher), resp.Body); err != nil {
		tmpTarball.Close()
		return err
	}
	if err := tmpTarball.Close(); err != nil {
		return err
	}
	if expectedSHA != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedSHA {
			return fmt.Errorf("sha256 mismatch: got %s, want %s", got, expectedSHA)
		}
	}

	tmpBinary, err := os.CreateTemp(filepath.Dir(dstPath), "vpnkit-bin-*.tmp")
	if err != nil {
		return err
	}
	tmpBinaryName := tmpBinary.Name()
	defer os.Remove(tmpBinaryName)

	if err := extractVpnkit(tmpTarballName, tmpBinary); err != nil {
		tmpBinary.Close()
		return err
	}
	if err := tmpBinary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpBinaryName, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpBinaryName, dstPath)
}
```

Add `"net/http"` to imports if not present.

- [ ] **Step 3: Fix the caller in cmd/vpnkit/cmd_update.go**

Find `updater.DownloadAndApplyVpnkit(...)` call and update to new signature:
```go
if err := updater.DownloadAndApplyVpnkit(githubURL, "", dst); err != nil {
	return err
}
```
Drop `mirrorAttemptPrinter(out)` and `st.Cfg.ReleaseMirror` args. Drop the `winningMirror` return capture line.

- [ ] **Step 4: Build + test**

```bash
go build ./...
go test ./... -race
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(updater): DownloadAndApplyVpnkit drops mirror fallback

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.5: Trim cmd/vpnkit/cmd_update.go (drop mirror helpers)

**Files:**
- Modify: `cmd/vpnkit/cmd_update.go`

- [ ] **Step 1: Delete helper functions**

In `cmd/vpnkit/cmd_update.go`, delete:
- `mirrorAttemptPrinter`
- `cacheWinningMirror`
- `prefixedAPIBase`

In `upgradeMihomo`:
- Drop `Mirror: st.Cfg.ReleaseMirror` from `installer.Options`
- Drop `OnAttempt: mirrorAttemptPrinter(out)` from `installer.Options`
- Drop `cacheWinningMirror(out, st, res.Mirror)` call

In `upgradeVpnkit`:
- Drop `st.Cfg.ReleaseMirror` and `mirrorAttemptPrinter(out)` args from `updater.DownloadAndApplyVpnkit`
- Drop `winningMirror :=` capture; drop `cacheWinningMirror` call

In `runUpdate`:
- Replace `APIBase: prefixedAPIBase(st.Cfg.ReleaseMirror)` with no APIBase (default `""` → defaults to github.com).

Also drop unused imports: `"vpnkit/internal/netx"` if no longer needed.

- [ ] **Step 2: Build + test**

```bash
go build ./...
go test ./cmd/vpnkit/ -race -v
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/vpnkit/cmd_update.go
git commit -m "refactor(cmd/update): drop mirror helpers, prefixedAPIBase, cacheWinningMirror

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.6: Drop --release-mirror in cmd_init.go

**Files:**
- Modify: `cmd/vpnkit/cmd_init.go`
- Modify: `cmd/vpnkit/cmd_init_test.go`
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Update cmd_init_test.go**

Remove any test cases that pass `--release-mirror` or assert `runInitOpts.ReleaseMirror`. Update test fixtures so they don't expect `release_mirror` in the resulting config.toml.

- [ ] **Step 2: Modify cmd_init.go**

In `runInitOpts`, remove `ReleaseMirror string`.

In `runInit`:
- Remove the `if opts.ReleaseMirror != "" && st.Cfg.ReleaseMirror != opts.ReleaseMirror { ... }` block.
- Remove `ReleaseMirror: st.Cfg.ReleaseMirror` from the `config.SkeletonInput{}` literal (the SkeletonInput field will be deleted in a later task; for now passing zero is harmless if it still exists, or remove now if it's already gone).

- [ ] **Step 3: Modify main.go dispatchInit**

Remove the line:
```go
mirror := fs.String("release-mirror", "", "URL prefix for mihomo + geox downloads (GFW workaround)")
```

And the `*mirror` passthrough. Final shape:
```go
func dispatchInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	restore := fs.String("restore", "", "path to a profiles backup TOML to merge")
	_ = fs.Bool("non-interactive", false, "(no-op; init is always non-interactive)")
	_ = fs.Parse(args)
	if err := runInit(os.Stdout, runInitOpts{RestorePath: *restore}); err != nil {
		dieRuntime("vpnkit init: %v", err)
	}
}
```

- [ ] **Step 4: Build + test**

```bash
go build ./...
go test ./cmd/vpnkit/ -race -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/main.go cmd/vpnkit/cmd_init.go cmd/vpnkit/cmd_init_test.go
git commit -m "refactor(cmd/init): drop --release-mirror flag

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.7: Drop ReleaseMirror from store, config skeleton, reconcile

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/config/skeleton.go`
- Modify: `internal/config/skeleton_test.go`
- Modify: `internal/config/reconcile.go`
- Modify: `internal/config/reconcile_test.go`

- [ ] **Step 1: Update store_test.go**

Remove any assertions on `Cfg.ReleaseMirror`. Update default-construction tests.

- [ ] **Step 2: Modify store.go**

In `Config` struct, **delete** `ReleaseMirror string` field. The `defaults()` constructor already does not set it, so no change there.

The BurntSushi/toml decoder silently ignores unknown keys, so any user with `release_mirror = "..."` in their config.toml will see it dropped on next Save — that's the agreed behavior (Non-goal 2).

- [ ] **Step 3: Modify skeleton.go and skeleton_test.go**

In `SkeletonInput`, **delete** `ReleaseMirror string`.

Replace the `mihomoGeoxURL(mirror)` helper with the parameter-less version:
```go
func mihomoGeoxURL() map[string]string {
	const base = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	return map[string]string{
		"geoip":   base + "/geoip.metadb",
		"mmdb":    base + "/country.mmdb",
		"geosite": base + "/geosite.dat",
		"asn":     base + "/GeoLite2-ASN.mmdb",
	}
}
```

In `BuildSkeleton`, change `base["geox-url"] = mihomoGeoxURL(in.ReleaseMirror)` to `base["geox-url"] = mihomoGeoxURL()`.

Update `skeleton_test.go` to expect direct GitHub URLs.

- [ ] **Step 4: Modify reconcile.go and reconcile_test.go**

In `SecurityFields`, **delete** `ReleaseMirror string`.

In `EnsureSecurityFields`, change `mihomoGeoxURL(sf.ReleaseMirror)` → `mihomoGeoxURL()`. (This `mihomoGeoxURL` is the same helper in the `config` package.)

Update `reconcile_test.go` to not pass ReleaseMirror and to expect direct GitHub URLs.

- [ ] **Step 5: Find every remaining caller and fix**

Run:
```bash
grep -rn "ReleaseMirror" --include="*.go"
```

For each hit:
- `internal/app/run.go`: ensure `SkeletonInput{}` / `SecurityFields{}` literal doesn't set `ReleaseMirror`.
- `internal/app/bootstrap.go`: same.
- `internal/app/update_check.go`: ensure `updater.Opts{}` doesn't pass `prefixedAPIBase(...)` mirror.
- `cmd/vpnkit/cmd_init.go`: ensure no `ReleaseMirror:` literals remain.

Fix each. They should be tiny edits.

- [ ] **Step 6: Build + test**

```bash
go build ./...
go test ./... -race
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(config,store): drop ReleaseMirror from store.Config + SkeletonInput + SecurityFields

geox-url defaults to direct GitHub Releases (no jsdelivr).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.8: Drop Mirror row from Settings → Mihomo Core TUI

**Files:**
- Modify: `internal/tabs/settings/core.go`

- [ ] **Step 1: Modify core.go**

In `coreModel.Update`'s `u` case, drop `Mirror: mirror` from `installer.Options{}`:
```go
res, err := installer.Install(installer.Options{
	Dst: m.paths.MihomoBinary(),
}, nil)
```

In `coreModel.View`, delete:
```go
mirror := ""
if m.store != nil {
	mirror = m.store.Cfg.ReleaseMirror
}
```
and delete the `fmt.Sprintf("  Mirror : %s\n", coreFallback(mirror, "(direct GitHub)"))` line.

Delete the `coreFallback` helper.

The `store` field on `coreModel` is no longer used for anything; either drop it from the struct + `newCore` signature, or leave it for future use (the upgrade flow's `Install` will still pass it through env). Decision: **keep `store` field** (zero cost, supports future flags like a `Token` config field).

- [ ] **Step 2: Build + test**

```bash
go build ./...
go test ./internal/tabs/settings/ -race -v
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/tabs/settings/core.go
git commit -m "refactor(tui/settings/core): drop Mirror row from Mihomo Core view

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.9: Clean install.sh

**Files:**
- Modify: `install.sh`

- [ ] **Step 1: Rewrite install.sh**

Replace with:
```bash
#!/usr/bin/env bash
set -euo pipefail

# vpnkit installer
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/JimZhang168872/vpnkit/main/install.sh | bash
#   VERSION=v0.8.0 INSTALL_DIR=$HOME/bin bash <(curl -sSL .../install.sh)
#
# Env:
#   VERSION             pin a tag (default: latest non-prerelease release)
#   INSTALL_DIR         binary target (default: $HOME/.local/bin)
#   INSTALL_FORCE       1 = reinstall even when same version is already present
#   INSTALL_TAKEOVER    1 = overwrite ~/.config/mihomo/ if it was made by another clash tool
#
# Network: this installer reaches github.com directly. If you're behind a
# restrictive network, configure HTTPS_PROXY in your shell before running
# (or use another box to download the tarball and run the binary's
# `vpnkit init` locally).

log()  { printf '%s\n' "$*"; }
warn() { printf '⚠️  %s\n' "$*" >&2; }
fail() { printf '❌ %s\n' "$*" >&2; exit 1; }

command -v curl       >/dev/null || fail "curl is required"
command -v sha256sum  >/dev/null || fail "sha256sum is required (coreutils)"
command -v tar        >/dev/null || fail "tar is required"

REPO="JimZhang168872/vpnkit"
DEST="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
VPNKIT_CFG="$CONFIG_HOME/vpnkit/config.toml"
MIHOMO_CFG="$CONFIG_HOME/mihomo/config.yaml"

# ───────── arch detect ─────────
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail "unsupported arch $arch (only amd64 / arm64 are released)" ;;
esac

# ───────── version resolve ─────────
if [ -z "${VERSION:-}" ]; then
  log "🔎 resolving latest release …"
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -oP '"tag_name":\s*"\K[^"]+' || true)
fi
if [ -z "${VERSION:-}" ]; then
  warn "could not resolve latest version automatically"
  fail "set VERSION=v… and re-run"
fi

# ───────── pre-flight: existing install detection ─────────
backup_file=""
if [ -x "$DEST/vpnkit" ]; then
  current="$("$DEST/vpnkit" --version 2>/dev/null | head -1 | awk '{print $2}' || true)"
  if [ "v${current}" = "$VERSION" ] && [ -z "${INSTALL_FORCE:-}" ]; then
    log "✅ vpnkit $VERSION already installed at $DEST/vpnkit"
    log "   set INSTALL_FORCE=1 to reinstall anyway"
    exit 0
  fi
  log "🧹 found existing vpnkit ${current:-?} — cleaning up before reinstall"

  if [ -f "$VPNKIT_CFG" ] && grep -q '^\[\[profiles\]\]' "$VPNKIT_CFG"; then
    backup_file="/tmp/vpnkit-profiles-$(date +%Y%m%d-%H%M%S).toml"
    awk '/^\[\[profiles\]\]/{p=1} p' "$VPNKIT_CFG" > "$backup_file"
    chmod 600 "$backup_file"
    log "📦 backed up profiles → $backup_file"
  fi

  if [ -f "$CONFIG_HOME/systemd/user/mihomo.service" ]; then
    systemctl --user stop mihomo 2>/dev/null || true
    systemctl --user disable mihomo 2>/dev/null || true
    rm -f "$CONFIG_HOME/systemd/user/mihomo.service"
    systemctl --user daemon-reload 2>/dev/null || true
    log "🧹 removed systemd unit"
  fi

  STATE_HOME="${XDG_STATE_HOME:-$HOME/.local/state}"
  CACHE_HOME="${XDG_CACHE_HOME:-$HOME/.cache}"
  rm -rf \
    "$CONFIG_HOME/mihomo" \
    "$CONFIG_HOME/vpnkit" \
    "$STATE_HOME/vpnkit" \
    "$CACHE_HOME/vpnkit"

  rm -f "$DEST/vpnkit" "$DEST/mihomo"
  log "🧹 removed old binaries + config dirs"
fi

if [ -e "$MIHOMO_CFG" ] && [ ! -e "$VPNKIT_CFG" ]; then
  if [ -z "${INSTALL_TAKEOVER:-}" ]; then
    warn "$MIHOMO_CFG exists but no vpnkit config — likely from another clash tool"
    fail "set INSTALL_TAKEOVER=1 to overwrite, or move/remove it first"
  fi
  log "⚠️  taking over existing $MIHOMO_CFG (INSTALL_TAKEOVER=1)"
fi

# ───────── download ─────────
VER_NUM="${VERSION#v}"
TARBALL="vpnkit_${VER_NUM}_linux_${arch}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

log "⬇️  downloading $TARBALL …"
curl -fsSL -o "$tmp/$TARBALL" "$BASE/$TARBALL" || fail "download failed (configure HTTPS_PROXY if you're behind a restricted network)"
curl -fsSL -o "$tmp/SHA256SUMS" "$BASE/SHA256SUMS" || fail "checksum download failed"

if ( cd "$tmp" && grep " $TARBALL\$" SHA256SUMS | sha256sum -c - >/dev/null ); then
  log "✅ checksum verified"
else
  fail "checksum mismatch"
fi

tar -xzf "$tmp/$TARBALL" -C "$tmp"
mkdir -p "$DEST"
install -m 0755 "$tmp/vpnkit" "$DEST/vpnkit"
log "📦 installed $VERSION → $DEST/vpnkit"

# ───────── init config ─────────
log "🛠️  initializing config …"
init_args=()
[ -n "$backup_file" ] && [ -f "$backup_file" ] && init_args+=(--restore "$backup_file")
"$DEST/vpnkit" init "${init_args[@]}" || warn "init returned non-zero"

# ───────── done ─────────
log ""
log "🎉 vpnkit $VERSION ready"
log "   • $VPNKIT_CFG"
log "   • $MIHOMO_CFG"
log ""
log "next:"
log "  \$ vpnkit              # open TUI, add a subscription"
log "  \$ vpnkit status       # quick state check"
log "  \$ eval \"\$(vpnkit env --shell zsh)\"   # wire shell proxy env"
```

- [ ] **Step 2: Verify shell syntax**

Run:
```bash
bash -n install.sh
```
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add install.sh
git commit -m "refactor(install.sh): drop mirror_wrap and INSTALL_MIRROR

Pure direct downloads. README updated separately.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3.10: Verify Phase 3 acceptance — no mirror trace

**Files:** none (verification only)

- [ ] **Step 1: Grep for mirror residue**

Run:
```bash
git grep -i 'mirror'  -- '*.go' 'install.sh'
git grep -i 'jsdelivr' -- '*.go' 'install.sh' README.md README_zh.md
git grep 'release_mirror' -- '*.go' 'install.sh'
git grep -i 'INSTALL_MIRROR' -- '*.go' 'install.sh'
```
Expected: zero hits across all four greps. If any, fix in a follow-up commit before moving to Phase 4.

- [ ] **Step 2: Full test sweep**

Run:
```bash
go vet ./...
go test -race -cover ./...
```
Expected: PASS.

- [ ] **Step 3: No commit** — this is a gate.

---

## Phase 4: TUI Extensions sub-page

### Task 4.1: Scaffold internal/tabs/settings/extensions.go

**Files:**
- Create: `internal/tabs/settings/extensions.go`
- Create: `internal/tabs/settings/extensions_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tabs/settings/extensions_test.go`:
```go
package settings

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/extensions"
)

func TestExtensionsViewListsChainsAndGroups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
		Groups: []extensions.Group{
			{Name: "G1", Type: "select", Proxies: []string{"DIRECT"}},
		},
	})
	m := newExtensions(path, func() []string { return []string{"A", "B"} })
	out := m.View(80, 24)
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("chain entries missing: %s", out)
	}
	if !strings.Contains(out, "G1") {
		t.Fatalf("group entry missing: %s", out)
	}
}

func TestExtensionsToggleListsWithCAndG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "C-only-chain", Via: "Z"}},
		Groups: []extensions.Group{
			{Name: "G-only-group", Type: "select", Proxies: []string{"DIRECT"}},
		},
	})
	m := newExtensions(path, func() []string { return nil })
	// Default view = chains.
	if !strings.Contains(m.View(80, 24), "C-only-chain") {
		t.Fatalf("expected chains by default")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if !strings.Contains(m.View(80, 24), "G-only-group") {
		t.Fatalf("expected groups after g")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if !strings.Contains(m.View(80, 24), "C-only-chain") {
		t.Fatalf("expected chains after c")
	}
}

func TestExtensionsDeleteRemovesEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	m := newExtensions(path, func() []string { return nil })
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 0 {
		t.Fatalf("delete didn't persist: %+v", ext)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run:
```bash
go test ./internal/tabs/settings/ -run TestExtensions -v
```
Expected: FAIL (undefined `newExtensions`).

- [ ] **Step 3: Implement extensions.go**

Create `internal/tabs/settings/extensions.go`:
```go
package settings

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/extensions"
)

// ProxyNamesFunc returns the current set of mihomo proxy names + group names
// (used for autocomplete in add/edit forms). Caller supplies a closure so we
// don't depend directly on the api.Client.
type ProxyNamesFunc func() []string

type extPane int

const (
	paneChains extPane = iota
	paneGroups
)

type extensionsModel struct {
	path  string
	ext   extensions.Extensions
	pane  extPane
	row   int
	flash string
	names ProxyNamesFunc
}

func newExtensions(path string, names ProxyNamesFunc) extensionsModel {
	ext, _ := extensions.Load(path)
	return extensionsModel{path: path, ext: ext, pane: paneChains, names: names}
}

func (m extensionsModel) Update(msg tea.Msg) (extensionsModel, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "c":
		m.pane = paneChains
		m.row = 0
	case "g":
		m.pane = paneGroups
		m.row = 0
	case "up", "k":
		if m.row > 0 {
			m.row--
		}
	case "down", "j":
		size := m.activeLen()
		if m.row < size-1 {
			m.row++
		}
	case "d":
		m.deleteCurrent()
	}
	return m, nil
}

func (m extensionsModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Extensions")
	tabs := m.renderTabs()
	body := m.renderList(width - 2)
	footer := m.renderFooter()
	out := header + "\n\n" + tabs + "\n\n" + body + "\n\n" + footer
	if m.flash != "" {
		out += "\n  → " + m.flash + "\n"
	}
	out += fmt.Sprintf("\nfile: %s\n", m.path)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(out)
}

func (m extensionsModel) renderTabs() string {
	style := func(active bool) lipgloss.Style {
		if active {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	}
	return style(m.pane == paneChains).Render(fmt.Sprintf("[c] Chains (%d)", len(m.ext.Chains))) +
		"    " +
		style(m.pane == paneGroups).Render(fmt.Sprintf("[g] Groups (%d)", len(m.ext.Groups)))
}

func (m extensionsModel) renderList(width int) string {
	lines := []string{}
	cursor := func(i int) string {
		if i == m.row {
			return "▶ "
		}
		return "  "
	}
	switch m.pane {
	case paneChains:
		for i, c := range m.ext.Chains {
			lines = append(lines, cursor(i)+fmt.Sprintf("%-30s → %s", c.Node, c.Via))
		}
		if len(lines) == 0 {
			lines = append(lines, "  (no chains)")
		}
	case paneGroups:
		for i, g := range m.ext.Groups {
			lines = append(lines, cursor(i)+fmt.Sprintf("%-20s [%s] %s", g.Name, g.Type, strings.Join(g.Proxies, ",")))
		}
		if len(lines) == 0 {
			lines = append(lines, "  (no groups)")
		}
	}
	return strings.Join(lines, "\n")
}

func (m extensionsModel) renderFooter() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
		"[↑↓] navigate  [a] add  [e] edit  [d] del  [r] apply (reassemble + reload)",
	)
}

func (m extensionsModel) activeLen() int {
	if m.pane == paneChains {
		return len(m.ext.Chains)
	}
	return len(m.ext.Groups)
}

func (m *extensionsModel) deleteCurrent() {
	if m.pane == paneChains && m.row < len(m.ext.Chains) {
		m.ext.Chains = append(m.ext.Chains[:m.row], m.ext.Chains[m.row+1:]...)
	}
	if m.pane == paneGroups && m.row < len(m.ext.Groups) {
		m.ext.Groups = append(m.ext.Groups[:m.row], m.ext.Groups[m.row+1:]...)
	}
	if m.row > 0 && m.row >= m.activeLen() {
		m.row--
	}
	if err := extensions.Save(m.path, m.ext); err != nil {
		m.flash = "save: " + err.Error()
		return
	}
	m.flash = "deleted"
}
```

NB: `a` (add) / `e` (edit) / `r` (apply) keys are accepted (no panic) but no-op in this task — wired up in 4.2/4.3.

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/tabs/settings/ -race -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tabs/settings/extensions.go internal/tabs/settings/extensions_test.go
git commit -m "feat(tui/settings): Extensions sub-page skeleton (list + delete)

Add/edit/apply are stubbed; landed in subsequent tasks.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 4.2: Wire Extensions into Settings model

**Files:**
- Modify: `internal/tabs/settings/settings.go`
- Modify: `internal/app/run.go`
- Modify: `internal/app/model.go` (or wherever Settings is instantiated)

- [ ] **Step 1: Modify settings.go**

In `Deps`, add a field for proxy names + extensions path:
```go
type Deps struct {
	Paths          paths.XDG
	Store          *store.Store
	Service        service.Manager
	APIClient      *api.Client
	ExtensionsPath string
	ProxyNames     ProxyNamesFunc // returns proxy+group names from latest snapshot
}
```

In `Model`, add: `extensions extensionsModel`.

In `New(deps Deps)`, instantiate: `extensions: newExtensions(deps.ExtensionsPath, deps.ProxyNames)`.

Replace the placeholder `case SubExtensions:` branches in `Update` and `View`:
```go
// Update
case SubExtensions:
	m.extensions, cmd = m.extensions.Update(message)

// View
case SubExtensions:
	body = m.extensions.View(bodyWidth, height)
```

- [ ] **Step 2: Modify internal/app/run.go**

Where `settingsDeps` is built (around line 87), add:
```go
settingsDeps := tabsettings.Deps{
	Paths:          p,
	Store:          st,
	Service:        svc,
	APIClient:      client,
	ExtensionsPath: filepath.Join(filepath.Dir(p.VpnkitConfigFile()), "extensions.toml"),
	ProxyNames:     proxiesSnapshotNames(model), // see step 3
}
```

- [ ] **Step 3: Add a proxiesSnapshotNames helper**

The cleanest path is to expose the last polled snapshot via the Model. Add a method on the app's Model:
```go
// CurrentProxyNames returns the set of names (proxies + groups) from the
// last received ProxiesSnapshot. Safe to call from any goroutine that holds
// the Model reference — used by the Extensions sub-page for autocomplete.
func (m *Model) CurrentProxyNames() []string {
	if m == nil {
		return nil
	}
	m.proxyNamesMu.Lock()
	defer m.proxyNamesMu.Unlock()
	out := make([]string, len(m.proxyNames))
	copy(out, m.proxyNames)
	return out
}
```

In `internal/app/model.go`, add fields:
```go
proxyNamesMu sync.Mutex
proxyNames   []string
```

In `internal/app/update.go`, in the `ProxiesSnapshot` branch (or near `pollProxies`), after constructing the new snapshot, update `m.proxyNames` to `unique(snap.Groups.AllNames() ∪ ...)`. Simplest: in the `ProxiesSnapshot` case under `Update`, set:
```go
case ProxiesSnapshot:
	m.proxiesTab, cmd = m.proxiesTab.Update(msg)
	m.proxyNamesMu.Lock()
	m.proxyNames = m.proxyNames[:0]
	for name := range v.Groups {
		m.proxyNames = append(m.proxyNames, name)
	}
	// Also include `All` member names from each group.
	seen := map[string]bool{}
	for _, n := range m.proxyNames {
		seen[n] = true
	}
	for _, g := range v.Groups {
		for _, n := range g.All {
			if !seen[n] {
				seen[n] = true
				m.proxyNames = append(m.proxyNames, n)
			}
		}
	}
	m.proxyNamesMu.Unlock()
```

Use `v` from the case binding (`case v := <-msg`).

Then in `run.go`, instantiate before `NewModel`:
```go
mPlaceholder := &app.Model{} // tmp shim if needed
proxyNamesFunc := func() []string { return modelPtr.CurrentProxyNames() }
```

Simpler approach: pass the names closure as a thunk evaluated lazily:
```go
var mref *Model
settingsDeps := tabsettings.Deps{
	...
	ProxyNames: func() []string {
		if mref == nil {
			return nil
		}
		return mref.CurrentProxyNames()
	},
}
mref = NewModel(client, profMgr, settingsDeps, applyCfg)
```

Adjust to your codebase's actual `NewModel` signature.

- [ ] **Step 4: Build + test**

```bash
go build ./...
go test ./... -race
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(app,settings): wire Extensions sub-page into Settings model

ProxyNames closure feeds live snapshot into Extensions for autocomplete.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 4.3: Add Extensions form (a/e) + apply (r)

**Files:**
- Modify: `internal/tabs/settings/extensions.go`
- Modify: `internal/tabs/settings/extensions_test.go`
- Modify: `internal/app/model.go` and/or `internal/app/update.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/tabs/settings/extensions_test.go`:
```go
func TestExtensionsAddChainPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	m := newExtensions(path, func() []string { return []string{"NodeA", "NodeB"} })
	// Press a to open form
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !m.formOpen() {
		t.Fatalf("expected form open after a")
	}
	// Type "NodeA" then tab then "NodeB" then Enter
	for _, r := range "NodeA" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "NodeB" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Node != "NodeA" || ext.Chains[0].Via != "NodeB" {
		t.Fatalf("chain not persisted: %+v", ext)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run:
```bash
go test ./internal/tabs/settings/ -run TestExtensionsAdd -v
```
Expected: FAIL.

- [ ] **Step 3: Implement the form**

Extend `internal/tabs/settings/extensions.go`:

Add fields to `extensionsModel`:
```go
form      *extForm
applyFunc func() error // optional; nil = no-op (`r` is no-op until wired up)
```

Add the form type:
```go
type extForm struct {
	pane      extPane     // chain or group
	editIndex int         // -1 = add, >=0 = edit existing
	fields    []string    // chain: [node, via]; group: [name, type, proxies, url, interval, tolerance]
	focus     int
	chainOrigExt extensions.Extensions
}

func newChainForm(editIndex int, pref extensions.Chain) *extForm {
	return &extForm{
		pane:      paneChains,
		editIndex: editIndex,
		fields:    []string{pref.Node, pref.Via},
	}
}

func newGroupForm(editIndex int, pref extensions.Group) *extForm {
	return &extForm{
		pane:      paneGroups,
		editIndex: editIndex,
		fields: []string{
			pref.Name, pref.Type,
			strings.Join(pref.Proxies, ","),
			pref.URL, fmt.Sprint(pref.Interval), fmt.Sprint(pref.Tolerance),
		},
	}
}

func (m extensionsModel) formOpen() bool { return m.form != nil }
```

In `Update`, route key input when form is open:
```go
if m.form != nil {
	return m.updateForm(km)
}
```

And add the `a` / `e` / `r` branches above the existing switch:
```go
case "a":
	if m.pane == paneChains {
		m.form = newChainForm(-1, extensions.Chain{})
	} else {
		m.form = newGroupForm(-1, extensions.Group{Type: "select"})
	}
	return m, nil
case "e":
	if m.pane == paneChains && m.row < len(m.ext.Chains) {
		m.form = newChainForm(m.row, m.ext.Chains[m.row])
	}
	if m.pane == paneGroups && m.row < len(m.ext.Groups) {
		m.form = newGroupForm(m.row, m.ext.Groups[m.row])
	}
	return m, nil
case "r":
	if m.applyFunc != nil {
		if err := m.applyFunc(); err != nil {
			m.flash = "apply: " + err.Error()
		} else {
			m.flash = "applied + reloaded"
		}
	} else {
		m.flash = "apply unwired (run from full app)"
	}
	return m, nil
```

Add `updateForm`:
```go
func (m extensionsModel) updateForm(km tea.KeyMsg) (extensionsModel, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		m.form = nil
		m.flash = "cancelled"
		return m, nil
	case tea.KeyEnter:
		return m.commitForm(), nil
	case tea.KeyTab:
		m.form.focus = (m.form.focus + 1) % len(m.form.fields)
		return m, nil
	case tea.KeyShiftTab:
		m.form.focus = (m.form.focus + len(m.form.fields) - 1) % len(m.form.fields)
		return m, nil
	case tea.KeyBackspace:
		if len(m.form.fields[m.form.focus]) > 0 {
			s := m.form.fields[m.form.focus]
			m.form.fields[m.form.focus] = s[:len(s)-1]
		}
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		m.form.fields[m.form.focus] += string(km.Runes)
		return m, nil
	}
	return m, nil
}

func (m extensionsModel) commitForm() extensionsModel {
	switch m.form.pane {
	case paneChains:
		c := extensions.Chain{Node: m.form.fields[0], Via: m.form.fields[1]}
		newChains := append([]extensions.Chain{}, m.ext.Chains...)
		if m.form.editIndex >= 0 && m.form.editIndex < len(newChains) {
			newChains[m.form.editIndex] = c
		} else {
			newChains = append(newChains, c)
		}
		candidate := m.ext
		candidate.Chains = newChains
		if err := extensions.Validate(candidate); err != nil {
			m.flash = "validate: " + err.Error()
			return m
		}
		m.ext = candidate
	case paneGroups:
		interval, _ := strconv.Atoi(m.form.fields[4])
		tolerance, _ := strconv.Atoi(m.form.fields[5])
		g := extensions.Group{
			Name:      m.form.fields[0],
			Type:      m.form.fields[1],
			Proxies:   splitCSV(m.form.fields[2]),
			URL:       m.form.fields[3],
			Interval:  interval,
			Tolerance: tolerance,
		}
		newGroups := append([]extensions.Group{}, m.ext.Groups...)
		if m.form.editIndex >= 0 && m.form.editIndex < len(newGroups) {
			newGroups[m.form.editIndex] = g
		} else {
			newGroups = append(newGroups, g)
		}
		candidate := m.ext
		candidate.Groups = newGroups
		if err := extensions.Validate(candidate); err != nil {
			m.flash = "validate: " + err.Error()
			return m
		}
		m.ext = candidate
	}
	if err := extensions.Save(m.path, m.ext); err != nil {
		m.flash = "save: " + err.Error()
		return m
	}
	m.flash = "saved"
	m.form = nil
	return m
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
```

Add to imports: `"strconv"`.

Modify `View` to render the form when open:
```go
if m.form != nil {
	return m.renderForm(width, height)
}
```

Add:
```go
func (m extensionsModel) renderForm(width, height int) string {
	labels := []string{"Node", "Via"}
	if m.form.pane == paneGroups {
		labels = []string{"Name", "Type (select|url-test|fallback|load-balance|relay)", "Proxies (comma-separated)", "URL (optional)", "Interval (optional, int)", "Tolerance (optional, int)"}
	}
	rows := []string{lipgloss.NewStyle().Bold(true).Render("Add/Edit:") + "  [Enter] save  [Esc] cancel  [Tab] next field"}
	for i, lbl := range labels {
		marker := "  "
		if i == m.form.focus {
			marker = "▶ "
		}
		rows = append(rows, fmt.Sprintf("%s%-46s %s", marker, lbl, m.form.fields[i]))
	}
	if m.flash != "" {
		rows = append(rows, "", "→ "+m.flash)
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}
```

- [ ] **Step 4: Run test to verify pass**

```bash
go test ./internal/tabs/settings/ -race -v
```
Expected: PASS.

- [ ] **Step 5: Wire applyFunc from the app layer**

In `internal/app/run.go` after `model := NewModel(...)`, expose an `ApplyExtensions` method on the model that:
1. Calls `profMgr.Update(ctx, profMgr.Active())` to re-fetch the active subscription and re-assemble with current extensions
2. Calls `applyCfg(ctx)` to reload mihomo

Then pass it as the closure to `extensionsModel.applyFunc`. Simplest path: pass an `ApplyFunc` field through `Deps`.

Modify `Deps`:
```go
type Deps struct {
	Paths          paths.XDG
	Store          *store.Store
	Service        service.Manager
	APIClient      *api.Client
	ExtensionsPath string
	ProxyNames     ProxyNamesFunc
	ApplyFunc      func() error
}
```

In `New`:
```go
ex := newExtensions(deps.ExtensionsPath, deps.ProxyNames)
ex.applyFunc = deps.ApplyFunc
return Model{
	...
	extensions: ex,
}
```

In `run.go`, supply `ApplyFunc`:
```go
deps.ApplyFunc = func() error {
	if profMgr.Active() == "" {
		return errors.New("no active profile")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := profMgr.Update(ctx, profMgr.Active()); err != nil {
		return err
	}
	return applyCfg(ctx)
}
```

Need `"errors"` import.

- [ ] **Step 6: Build + test**

```bash
go build ./...
go test ./... -race
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(tui/settings/extensions): add/edit form + r apply

Form covers chain (node + via) and group (6 fields). Validation runs
before save. r triggers profMgr.Update + applyCfg.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 5: CLI commands (chain, group, ext)

### Task 5.1: cmd_chain.go

**Files:**
- Create: `cmd/vpnkit/cmd_chain.go`
- Create: `cmd/vpnkit/cmd_chain_test.go`
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/vpnkit/cmd_chain_test.go`:
```go
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/extensions"
)

func TestRunChainLsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	if err := runChainLs(&buf, path, false); err != nil {
		t.Fatalf("runChainLs: %v", err)
	}
	if !strings.Contains(buf.String(), "no chains") {
		t.Fatalf("expected 'no chains' line, got %q", buf.String())
	}
}

func TestRunChainSetAdds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	if err := runChainSet(&buf, path, "NodeA", "NodeB"); err != nil {
		t.Fatalf("runChainSet: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Node != "NodeA" || ext.Chains[0].Via != "NodeB" {
		t.Fatalf("not persisted: %+v", ext)
	}
}

func TestRunChainSetUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	var buf bytes.Buffer
	if err := runChainSet(&buf, path, "A", "C"); err != nil {
		t.Fatalf("runChainSet: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Via != "C" {
		t.Fatalf("not updated: %+v", ext)
	}
}

func TestRunChainUnsetRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}, {Node: "C", Via: "D"}},
	})
	var buf bytes.Buffer
	if err := runChainUnset(&buf, path, "A"); err != nil {
		t.Fatalf("runChainUnset: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Node != "C" {
		t.Fatalf("wrong chain remains: %+v", ext)
	}
}

func TestRunChainSetRejectsSelfChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	err := runChainSet(&buf, path, "A", "A")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle rejection, got %v", err)
	}
}

func TestRunChainLsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	var buf bytes.Buffer
	if err := runChainLs(&buf, path, true); err != nil {
		t.Fatalf("runChainLs json: %v", err)
	}
	if !strings.Contains(buf.String(), `"node":"A"`) || !strings.Contains(buf.String(), `"via":"B"`) {
		t.Fatalf("json missing fields: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run:
```bash
go test ./cmd/vpnkit/ -run TestRunChain -v
```
Expected: FAIL.

- [ ] **Step 3: Implement cmd_chain.go**

Create `cmd/vpnkit/cmd_chain.go`:
```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vpnkit/internal/extensions"
	"vpnkit/internal/paths"
)

// dispatchChain routes `vpnkit chain <subcommand>`.
func dispatchChain(args []string) {
	if len(args) < 1 {
		dieUserErr("vpnkit chain: usage: vpnkit chain <ls|set|unset> ...")
	}
	path := extensionsPath()
	switch args[0] {
	case "ls":
		fs := flag.NewFlagSet("chain ls", flag.ExitOnError)
		jsonOut := fs.Bool("json", false, "JSON output")
		_ = fs.Parse(args[1:])
		if err := runChainLs(os.Stdout, path, *jsonOut); err != nil {
			dieRuntime("vpnkit chain ls: %v", err)
		}
	case "set":
		if len(args) != 3 {
			dieUserErr("vpnkit chain set: usage: vpnkit chain set <node> <via>")
		}
		if err := runChainSet(os.Stdout, path, args[1], args[2]); err != nil {
			dieUserErr("vpnkit chain set: %v", err)
		}
	case "unset":
		if len(args) != 2 {
			dieUserErr("vpnkit chain unset: usage: vpnkit chain unset <node>")
		}
		if err := runChainUnset(os.Stdout, path, args[1]); err != nil {
			dieUserErr("vpnkit chain unset: %v", err)
		}
	default:
		dieUserErr("vpnkit chain: unknown subcommand %q", args[0])
	}
}

// extensionsPath returns the canonical path: ~/.config/vpnkit/extensions.toml
func extensionsPath() string {
	p := paths.Resolve()
	return filepath.Join(filepath.Dir(p.VpnkitConfigFile()), "extensions.toml")
}

func runChainLs(out io.Writer, path string, jsonOut bool) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(out)
		return enc.Encode(ext.Chains)
	}
	if len(ext.Chains) == 0 {
		fmt.Fprintln(out, "no chains configured")
		return nil
	}
	for _, c := range ext.Chains {
		fmt.Fprintf(out, "%s → %s\n", c.Node, c.Via)
	}
	return nil
}

func runChainSet(out io.Writer, path, node, via string) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	replaced := false
	for i, c := range ext.Chains {
		if c.Node == node {
			ext.Chains[i].Via = via
			replaced = true
			break
		}
	}
	if !replaced {
		ext.Chains = append(ext.Chains, extensions.Chain{Node: node, Via: via})
	}
	if err := extensions.Validate(ext); err != nil {
		return err
	}
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	verb := "added"
	if replaced {
		verb = "updated"
	}
	fmt.Fprintf(out, "%s: %s → %s\n", verb, node, via)
	return nil
}

func runChainUnset(out io.Writer, path, node string) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	next := ext.Chains[:0]
	removed := false
	for _, c := range ext.Chains {
		if c.Node == node {
			removed = true
			continue
		}
		next = append(next, c)
	}
	if !removed {
		fmt.Fprintf(out, "no chain for %s\n", node)
		return nil
	}
	ext.Chains = next
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	fmt.Fprintf(out, "removed chain for %s\n", node)
	return nil
}
```

- [ ] **Step 4: Wire dispatchChain into main.go**

Add to the `switch os.Args[1]` block in `main()`:
```go
case "chain":
	dispatchChain(os.Args[2:])
	return
```

- [ ] **Step 5: Build + test**

```bash
go build ./...
go test ./cmd/vpnkit/ -race -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/vpnkit/cmd_chain.go cmd/vpnkit/cmd_chain_test.go cmd/vpnkit/main.go
git commit -m "feat(cmd/chain): ls/set/unset chain entries (with --json)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 5.2: cmd_group.go

**Files:**
- Create: `cmd/vpnkit/cmd_group.go`
- Create: `cmd/vpnkit/cmd_group_test.go`
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/vpnkit/cmd_group_test.go`:
```go
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/extensions"
)

func TestRunGroupAddSelect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	opts := groupAddOpts{
		Name: "G1", Type: "select", Proxies: []string{"A", "B"},
	}
	if err := runGroupAdd(&buf, path, opts); err != nil {
		t.Fatalf("runGroupAdd: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Groups) != 1 || ext.Groups[0].Name != "G1" {
		t.Fatalf("not persisted: %+v", ext)
	}
}

func TestRunGroupAddUrlTest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	opts := groupAddOpts{
		Name: "♻️ Auto", Type: "url-test", Proxies: []string{"a", "b"},
		URL: "https://www.gstatic.com/generate_204", Interval: 300, Tolerance: 50,
	}
	if err := runGroupAdd(&buf, path, opts); err != nil {
		t.Fatalf("runGroupAdd: %v", err)
	}
	ext, _ := extensions.Load(path)
	if ext.Groups[0].URL == "" || ext.Groups[0].Interval != 300 {
		t.Fatalf("url-test fields not persisted: %+v", ext.Groups[0])
	}
}

func TestRunGroupAddRejectsBadType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	opts := groupAddOpts{Name: "X", Type: "weird", Proxies: []string{"a"}}
	err := runGroupAdd(&buf, path, opts)
	if err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("want type error, got %v", err)
	}
}

func TestRunGroupRmRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Groups: []extensions.Group{
			{Name: "G1", Type: "select", Proxies: []string{"a"}},
			{Name: "G2", Type: "select", Proxies: []string{"b"}},
		},
	})
	var buf bytes.Buffer
	if err := runGroupRm(&buf, path, "G1"); err != nil {
		t.Fatalf("runGroupRm: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Groups) != 1 || ext.Groups[0].Name != "G2" {
		t.Fatalf("wrong remaining: %+v", ext)
	}
}

func TestRunGroupLsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Groups: []extensions.Group{{Name: "G", Type: "select", Proxies: []string{"a"}}},
	})
	var buf bytes.Buffer
	if err := runGroupLs(&buf, path, true); err != nil {
		t.Fatalf("runGroupLs json: %v", err)
	}
	if !strings.Contains(buf.String(), `"name":"G"`) {
		t.Fatalf("json missing name: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./cmd/vpnkit/ -run TestRunGroup -v
```
Expected: FAIL.

- [ ] **Step 3: Implement cmd_group.go**

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"vpnkit/internal/extensions"
)

type groupAddOpts struct {
	Name      string
	Type      string
	Proxies   []string
	URL       string
	Interval  int
	Tolerance int
}

func dispatchGroup(args []string) {
	if len(args) < 1 {
		dieUserErr("vpnkit group: usage: vpnkit group <ls|add|rm> ...")
	}
	path := extensionsPath()
	switch args[0] {
	case "ls":
		fs := flag.NewFlagSet("group ls", flag.ExitOnError)
		jsonOut := fs.Bool("json", false, "JSON output")
		_ = fs.Parse(args[1:])
		if err := runGroupLs(os.Stdout, path, *jsonOut); err != nil {
			dieRuntime("vpnkit group ls: %v", err)
		}
	case "add":
		fs := flag.NewFlagSet("group add", flag.ExitOnError)
		typ := fs.String("type", "select", "group type")
		proxies := fs.String("proxies", "", "comma-separated proxy names")
		url := fs.String("url", "", "(optional) test URL")
		interval := fs.Int("interval", 0, "(optional) test interval seconds")
		tolerance := fs.Int("tolerance", 0, "(optional) tolerance ms")
		_ = fs.Parse(args[1:])
		if fs.NArg() < 1 {
			dieUserErr("vpnkit group add: usage: vpnkit group add <name> --type <t> --proxies a,b,c [...]")
		}
		opts := groupAddOpts{
			Name: fs.Arg(0), Type: *typ,
			Proxies:   splitCSVCmd(*proxies),
			URL:       *url,
			Interval:  *interval,
			Tolerance: *tolerance,
		}
		if err := runGroupAdd(os.Stdout, path, opts); err != nil {
			dieUserErr("vpnkit group add: %v", err)
		}
	case "rm":
		if len(args) != 2 {
			dieUserErr("vpnkit group rm: usage: vpnkit group rm <name>")
		}
		if err := runGroupRm(os.Stdout, path, args[1]); err != nil {
			dieUserErr("vpnkit group rm: %v", err)
		}
	default:
		dieUserErr("vpnkit group: unknown subcommand %q", args[0])
	}
}

func splitCSVCmd(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func runGroupLs(out io.Writer, path string, jsonOut bool) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	if jsonOut {
		return json.NewEncoder(out).Encode(ext.Groups)
	}
	if len(ext.Groups) == 0 {
		fmt.Fprintln(out, "no groups configured")
		return nil
	}
	for _, g := range ext.Groups {
		fmt.Fprintf(out, "%s [%s] %s\n", g.Name, g.Type, strings.Join(g.Proxies, ","))
	}
	return nil
}

func runGroupAdd(out io.Writer, path string, opts groupAddOpts) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	for i, g := range ext.Groups {
		if g.Name == opts.Name {
			ext.Groups[i] = extensions.Group{
				Name: opts.Name, Type: opts.Type, Proxies: opts.Proxies,
				URL: opts.URL, Interval: opts.Interval, Tolerance: opts.Tolerance,
			}
			if err := extensions.Validate(ext); err != nil {
				return err
			}
			if err := extensions.Save(path, ext); err != nil {
				return err
			}
			fmt.Fprintf(out, "updated: %s\n", opts.Name)
			return nil
		}
	}
	ext.Groups = append(ext.Groups, extensions.Group{
		Name: opts.Name, Type: opts.Type, Proxies: opts.Proxies,
		URL: opts.URL, Interval: opts.Interval, Tolerance: opts.Tolerance,
	})
	if err := extensions.Validate(ext); err != nil {
		return err
	}
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	fmt.Fprintf(out, "added: %s\n", opts.Name)
	return nil
}

func runGroupRm(out io.Writer, path, name string) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	next := ext.Groups[:0]
	removed := false
	for _, g := range ext.Groups {
		if g.Name == name {
			removed = true
			continue
		}
		next = append(next, g)
	}
	if !removed {
		fmt.Fprintf(out, "no group %s\n", name)
		return nil
	}
	ext.Groups = next
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	fmt.Fprintf(out, "removed group: %s\n", name)
	return nil
}
```

- [ ] **Step 4: Add `group` dispatcher to main.go**

```go
case "group":
	dispatchGroup(os.Args[2:])
	return
```

- [ ] **Step 5: Build + test**

```bash
go build ./...
go test ./cmd/vpnkit/ -race -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/vpnkit/cmd_group.go cmd/vpnkit/cmd_group_test.go cmd/vpnkit/main.go
git commit -m "feat(cmd/group): ls/add/rm custom proxy-groups

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 5.3: cmd_ext.go (apply)

**Files:**
- Create: `cmd/vpnkit/cmd_ext.go`
- Create: `cmd/vpnkit/cmd_ext_test.go`
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/vpnkit/cmd_ext_test.go`:
```go
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/api"
	"vpnkit/internal/extensions"
)

// TestRunExtApplyErrorsWhenNoActiveProfile asserts a clean error when there is
// no active profile to re-assemble against — apply is not meaningful in that case.
func TestRunExtApplyErrorsWhenNoActiveProfile(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	err := runExtApply(&buf, runExtApplyDeps{
		ExtensionsPath: filepath.Join(dir, "extensions.toml"),
		ActiveProfile:  "",
		Reassemble:     func() error { return nil },
		Reload:         func() error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("want 'no active profile' error, got %v", err)
	}
}

func TestRunExtApplyHappy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	var buf bytes.Buffer
	reassembled := false
	reloaded := false
	err := runExtApply(&buf, runExtApplyDeps{
		ExtensionsPath: path,
		ActiveProfile:  "p",
		Reassemble:     func() error { reassembled = true; return nil },
		Reload:         func() error { reloaded = true; return nil },
	})
	if err != nil {
		t.Fatalf("runExtApply: %v", err)
	}
	if !reassembled || !reloaded {
		t.Fatalf("expected reassemble+reload, got reassemble=%v reload=%v", reassembled, reloaded)
	}
	if !strings.Contains(buf.String(), "applied") {
		t.Fatalf("expected 'applied' line, got %q", buf.String())
	}
}

// Ensure imported api package compiles even if unused in tests.
var _ = api.Client{}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./cmd/vpnkit/ -run TestRunExtApply -v
```
Expected: FAIL.

- [ ] **Step 3: Implement cmd_ext.go**

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"vpnkit/internal/paths"
	"vpnkit/internal/profiles"
	"vpnkit/internal/store"
)

type runExtApplyDeps struct {
	ExtensionsPath string
	ActiveProfile  string
	Reassemble     func() error // typically profMgr.Update(ctx, active)
	Reload         func() error // typically applyCfg(ctx)
}

func runExtApply(out io.Writer, d runExtApplyDeps) error {
	if d.ActiveProfile == "" {
		return fmt.Errorf("no active profile — set one with `vpnkit use <group> <node>` (or the TUI) and try again")
	}
	if err := d.Reassemble(); err != nil {
		return fmt.Errorf("reassemble: %w", err)
	}
	if err := d.Reload(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}
	fmt.Fprintln(out, "applied: subscription reassembled with extensions and mihomo reloaded")
	return nil
}

func dispatchExt(args []string) {
	if len(args) < 1 || args[0] != "apply" {
		dieUserErr("vpnkit ext: usage: vpnkit ext apply")
	}
	p := paths.Resolve()
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit ext apply: %v", err)
	}
	if st.Cfg.ActiveProfile == "" {
		dieUserErr("vpnkit ext apply: no active profile — set one first")
	}
	mgr := profiles.New(profiles.Config{
		ConfigYAMLPath:   p.MihomoConfigFile(),
		ExtensionsPath:   extensionsPath(),
		ControllerPort:   st.Cfg.ControllerPort,
		ControllerSecret: st.Cfg.ControllerSecret,
		MixedPort:        st.Cfg.MixedPort,
		RuleTemplate:     st.Cfg.RuleTemplate,
		ProxyUser:        st.Cfg.ProxyUser,
		ProxyPass:        st.Cfg.ProxyPass,
	})
	mgr.Load(toProfilesProfilesCLI(st.Cfg.Profiles), st.Cfg.ActiveProfile)

	client, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit ext apply: mihomo not reachable: %v", err)
	}

	deps := runExtApplyDeps{
		ExtensionsPath: extensionsPath(),
		ActiveProfile:  st.Cfg.ActiveProfile,
		Reassemble: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := mgr.Update(ctx, st.Cfg.ActiveProfile)
			return err
		},
		Reload: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return client.PutConfigs(ctx, p.MihomoConfigFile())
		},
	}
	if err := runExtApply(os.Stdout, deps); err != nil {
		dieRuntime("vpnkit ext apply: %v", err)
	}
}

// toProfilesProfilesCLI mirrors the helper in internal/app/run.go because
// CLI is a separate binary boundary; intentionally not deduped to avoid
// adding a public conversion in internal/profiles.
func toProfilesProfilesCLI(in []store.Profile) []profiles.Profile {
	out := make([]profiles.Profile, len(in))
	for i, x := range in {
		out[i] = profiles.Profile{
			Name: x.Name, URL: x.URL, UserAgent: x.UserAgent, LastUpdated: x.LastUpdated,
		}
	}
	return out
}
```

If `api.Client.PutConfigs(ctx, path)` doesn't exist, replace with whatever the project's "hot reload" call is; check `internal/api/configs.go` (likely `PutConfig(ctx, path string)` or similar).

- [ ] **Step 4: Add `ext` to dispatcher in main.go**

```go
case "ext":
	dispatchExt(os.Args[2:])
	return
```

- [ ] **Step 5: Build + test**

```bash
go build ./...
go test ./cmd/vpnkit/ -race -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/vpnkit/cmd_ext.go cmd/vpnkit/cmd_ext_test.go cmd/vpnkit/main.go
git commit -m "feat(cmd/ext): ext apply re-assembles + reloads mihomo

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 6: Documentation + acceptance matrix

### Task 6.1: README updates

**Files:**
- Modify: `README.md`
- Modify: `README_zh.md`

- [ ] **Step 1: Edit README.md**

(a) Delete the entire "Behind the GFW" section.
(b) In the Install section, replace any "INSTALL_MIRROR" guidance with:
```
> vpnkit reaches github.com directly. If your network is restricted,
> configure `HTTPS_PROXY` in your shell before installing.
```
(c) Add a new section after "First run", or in a "Extensions" subsection of "Features":
```markdown
## Extensions: chains & custom groups

Chain one subscription node through another (multi-hop egress) and add
your own proxy-groups. Edits persist in `~/.config/vpnkit/extensions.toml`
and survive subscription updates.

### CLI

    vpnkit chain ls
    vpnkit chain set "🇺🇸 US-1" "🇯🇵 JP-Relay"
    vpnkit chain unset "🇺🇸 US-1"

    vpnkit group ls
    vpnkit group add "🎯 Stream" --type select --proxies "🇺🇸 US-1,🇯🇵 JP-1,DIRECT"
    vpnkit group add "♻️ Auto-US" --type url-test \
        --proxies "🇺🇸 US-1,🇺🇸 US-2" \
        --url https://www.gstatic.com/generate_204 \
        --interval 300 --tolerance 50
    vpnkit group rm "🎯 Stream"

    vpnkit ext apply   # reassemble active subscription + reload mihomo

### TUI

Settings → Extensions. `c` toggles to Chains, `g` to Groups, `a/e/d`
add/edit/delete the highlighted row, `r` reassembles + reloads.

### Migration from patch.yaml

vpnkit no longer reads `~/.config/mihomo/patch.yaml`. Move any
chain / proxy-group tweaks to `~/.config/vpnkit/extensions.toml` (the
TUI's "Extensions" sub-page generates a valid file for you).
```

- [ ] **Step 2: Edit README_zh.md**

Apply the same changes, in Chinese. Delete the 「墙内」 section; add the equivalent 扩展（链 + 自定义代理组）section and update install instructions.

- [ ] **Step 3: Commit**

```bash
git add README.md README_zh.md
git commit -m "docs(readme): drop GFW mirror section, add Extensions guide

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 6.2: Run full acceptance matrix

**Files:** none (verification only)

- [ ] **Step 1: Final mirror grep**

Run:
```bash
git grep -i 'mirror'      -- '*.go' install.sh
git grep -i 'jsdelivr'    -- '*.go' install.sh README.md README_zh.md
git grep    'release_mirror' -- '*.go' install.sh
git grep -i 'INSTALL_MIRROR' -- '*.go' install.sh
git grep    'patch.yaml'  -- '*.go'
```
Expected: zero hits in all five (README may mention "patch.yaml" exactly once in the migration note — that's allowed).

- [ ] **Step 2: Full build + race + cover**

```bash
go vet ./...
go test -race -cover ./...
```
Expected: PASS, every new package ≥ 80% coverage. Print the per-package coverage to verify:
```bash
go test -cover ./internal/extensions/ ./cmd/vpnkit/ ./internal/tabs/settings/
```

- [ ] **Step 3: CLI smoke (against a fake controller)**

Run the binary in a scratch dir with `XDG_CONFIG_HOME` set:
```bash
TMPDIR=$(mktemp -d) && export XDG_CONFIG_HOME="$TMPDIR/config"
go build -o /tmp/vpnkit-smoke ./cmd/vpnkit

/tmp/vpnkit-smoke --version
/tmp/vpnkit-smoke init                              # creates config skeleton
/tmp/vpnkit-smoke chain ls                          # → "no chains"
/tmp/vpnkit-smoke chain set "X" "Y"
/tmp/vpnkit-smoke chain ls                          # → X → Y
/tmp/vpnkit-smoke chain ls --json
/tmp/vpnkit-smoke chain unset "X"
/tmp/vpnkit-smoke group add "G" --type select --proxies "DIRECT"
/tmp/vpnkit-smoke group ls
/tmp/vpnkit-smoke group rm "G"

# `status` / `ip` / `mode` / `groups` / `nodes` / `use` require running mihomo
# — skip in scratch unless mihomo is up. If the user has a real install,
# verify against that. Otherwise document the limitation in the PR description.
```

Expected: each command exits 0 with the expected output. Record any failures.

- [ ] **Step 4: TUI smoke (manual)**

Start the TUI against a real install. Verify the following matrix; mark each as ✓ or ✗ with one-line notes:

```
[ ] Dashboard tab renders, traffic + version visible
[ ] Proxies tab: ↑↓ navigates, Enter switches node, t triggers delay
[ ] Profiles tab: a opens form, Enter submits, u updates subscription
[ ] Connections tab: ↑↓, / filter, x close
[ ] Rules tab: ↑↓, / filter, u refresh providers
[ ] Settings → Mihomo Core (NO Mirror row), u upgrade
[ ] Settings → Service: visible
[ ] Settings → External Controller: visible
[ ] Settings → Default Rules: visible
[ ] Settings → Extensions: c/g toggle, a/e/d works, r reloads
[ ] Settings → Logs: tail visible
[ ] Settings → Cache: visible
[ ] Settings → About: visible
[ ] Tab cycling 1/2/3/4/5/6 and Tab/Shift-Tab
[ ] q / Ctrl-C quits cleanly
```

- [ ] **Step 5: Document results**

Append the matrix outcome (with any failure notes) to the PR description (when the user opens a PR). If everything passed, you're done.

- [ ] **Step 6: No commit** — verification gate.

---

## Done

All 28 tasks complete. The acceptance criteria from the spec (§6) are mechanically enforced by the greps + tests + matrix in Task 6.2.

---

## Self-Review (filled in by plan author)

**1. Spec coverage:**
- § 3.1 (mirror strip): Tasks 3.1-3.10 ✓
- § 3.2 (extensions package): Tasks 1.1-1.3 ✓
- § 3.3 (assemble integration): Task 2.1 ✓
- § 3.4 (TUI sub-page): Tasks 4.1-4.3 ✓
- § 3.5 (CLI new commands): Tasks 5.1-5.3 ✓
- § 3.6 (error handling): woven into Validate (1.3) + Apply (1.2) + cmd error paths (5.1-5.3) ✓
- § 3.7 (testing): per-task tests; matrix in 6.2 ✓
- § 3.8 (patch.yaml deletion): Task 2.4 ✓

**2. Placeholder scan:** no "TBD", "TODO", "add appropriate handling". Every step has either code or an exact command.

**3. Type consistency:**
- `extensions.Extensions{SchemaVersion, Chains, Groups}` used identically across tasks
- `extensions.Chain{Node, Via}` consistent
- `extensions.Group{Name, Type, Proxies, URL, Interval, Tolerance}` consistent
- `extensions.Apply(doc, ext)`, `extensions.Validate(ext)`, `extensions.Load(path)`, `extensions.Save(path, ext)` consistent
- `extensionsPath()` helper used by CLI consistent
- `Deps.ExtensionsPath` / `Deps.ProxyNames` / `Deps.ApplyFunc` consistent across settings/app

No drift detected.
