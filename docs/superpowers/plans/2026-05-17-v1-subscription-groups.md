# vpnkit v1.0.0-rc.1 Implementation Plan — 订阅组管理 + 本地节点 + 本地规则

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the v0.10.x single-active-profile model with multi-source coexistence — multiple subscription groups + a local-nodes group + structured local-rules CRUD + Mode (rule/global/direct) + Global Target. Schema v1 → v2 breaking.

**Architecture:** Four new leaf packages (`groups/`, `localnodes/`, `localrules/`, `assembler/`); `store/` schema v2; `profiles/` package deleted; `subscription/assemble.go` becomes a primitive that assembler/ composes. TUI gains Groups + Sources + Local-Rules + Routing sub-pages; CLI gains `subs/local-nodes/local-rules/target` verbs.

**Tech Stack:** Go 1.23, `github.com/BurntSushi/toml`, `gopkg.in/yaml.v3`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, stdlib `crypto/rand`, `net/url`, `net/http/httptest`.

**Spec:** `docs/superpowers/specs/2026-05-17-v1-subscription-groups-design.md`

**Release target:** `v1.0.0-rc.1` (pre-release tag, triggers goreleaser).

---

## Phase 0: Pre-flight

### Task 0.1: Verify baseline green

**Files:** None modified.

- [ ] **Step 1: Confirm clean working tree**

```bash
git status -s
```
Expected: empty.

- [ ] **Step 2: Confirm v0.10.2 tests pass**

```bash
export PATH=$HOME/.local/go/bin:$PATH
go test ./... -race
go vet ./...
```
Expected: all packages OK, vet clean.

- [ ] **Step 3: Create feature branch**

```bash
git checkout -b feat/v1-subscription-groups
```

- [ ] **Step 4: Note baseline commit**

```bash
git rev-parse HEAD > /tmp/v1-baseline.sha
```

---

## Phase 1: Store schema v2

Adds new fields, deletes old, and forces re-init on v1 stores.

### Task 1.1: Add SchemaVersion + new fields

**Files:**
- Modify: `internal/store/store.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/store/store_test.go`:

```go
func TestSchemaV2Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.SchemaVersion != 2 {
		t.Errorf("new store schema_version: got %d, want 2", s.Cfg.SchemaVersion)
	}
	if s.Cfg.Mode != "rule" {
		t.Errorf("default mode: got %q, want \"rule\"", s.Cfg.Mode)
	}
	if s.Cfg.GlobalTarget != "🚀 Proxy" {
		t.Errorf("default global_target: got %q, want \"🚀 Proxy\"", s.Cfg.GlobalTarget)
	}
	if s.Cfg.Subscriptions == nil {
		t.Error("Subscriptions must be empty slice, not nil")
	}
	if s.Cfg.LocalNodes == nil {
		t.Error("LocalNodes must be empty slice, not nil")
	}
	if s.Cfg.LocalRules == nil {
		t.Error("LocalRules must be empty slice, not nil")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/store -run TestSchemaV2Roundtrip
```
Expected: build error (fields not defined).

- [ ] **Step 3: Replace Config struct**

In `internal/store/store.go`, replace the `Profile` and `Config` types with:

```go
// Profile records one subscription entry — kept for v1 → v2 migration detection only.
// New code MUST use Subscription instead.
type Profile struct {
	Name        string    `toml:"name"`
	URL         string    `toml:"url"`
	UserAgent   string    `toml:"user_agent,omitempty"`
	LastUpdated time.Time `toml:"last_updated,omitempty"`
}

// Subscription is a remote profile source. The fetched yaml becomes one group
// at assemble time (see internal/groups, internal/assembler).
type Subscription struct {
	Name        string    `toml:"name"`
	URL         string    `toml:"url"`
	UserAgent   string    `toml:"user_agent,omitempty"`
	Enabled     bool      `toml:"enabled"`
	LastUpdated time.Time `toml:"last_updated,omitempty"`
	NodeCount   int       `toml:"node_count,omitempty"`
}

// LocalNode is a hand-entered proxy node. Fields is the proto-specific
// payload (e.g. password, sni, up, down) kept open-ended to match the variety
// of mihomo proxy types without locking the schema.
type LocalNode struct {
	Name   string         `toml:"name"`
	Proto  string         `toml:"proto"`
	Server string         `toml:"server"`
	Port   int            `toml:"port"`
	Fields map[string]any `toml:"fields,omitempty"`
}

// LocalRule is one user-authored rule entry. Type + Payload + Target match
// mihomo rule syntax; assembler.Render produces the final string.
type LocalRule struct {
	Type    string `toml:"type"`
	Payload string `toml:"payload"`
	Target  string `toml:"target"`
}

// Config is vpnkit's persisted configuration (schema v2).
type Config struct {
	SchemaVersion int `toml:"schema_version"`

	ControllerSecret string `toml:"controller_secret"`
	ControllerPort   int    `toml:"controller_port"`
	MixedPort        int    `toml:"mixed_port"`
	ProxyUser        string `toml:"proxy_user"`
	ProxyPass        string `toml:"proxy_pass"`
	UITheme          string `toml:"ui_theme"`
	ServiceMode      string `toml:"service_mode,omitempty"`

	Mode         string `toml:"mode"`
	GlobalTarget string `toml:"global_target"`

	Subscriptions []Subscription `toml:"subscriptions"`
	LocalNodes    []LocalNode    `toml:"local_nodes"`
	LocalRules    []LocalRule    `toml:"local_rules"`

	// v1 fields kept ONLY to detect old stores in Load. New code must not read these.
	LegacyActiveProfile string    `toml:"active_profile,omitempty"`
	LegacyProfiles      []Profile `toml:"profiles,omitempty"`
	LegacyRuleTemplate  string    `toml:"rule_template,omitempty"`
}
```

- [ ] **Step 4: Update `defaults()`**

Replace `defaults()` with:

```go
func defaults() Config {
	cp := randomHighPort()
	mp := randomHighPort()
	for mp == cp {
		mp = randomHighPort()
	}
	return Config{
		SchemaVersion:    2,
		ControllerSecret: randHex(16),
		ControllerPort:   cp,
		MixedPort:        mp,
		ProxyUser:        "vpnkit-" + randHex(4),
		ProxyPass:        randHex(16),
		UITheme:          "default",
		Mode:             "rule",
		GlobalTarget:     "🚀 Proxy",
		Subscriptions:    []Subscription{},
		LocalNodes:       []LocalNode{},
		LocalRules:       []LocalRule{},
	}
}
```

- [ ] **Step 5: Update `Load()` zero-value backfill**

In `Load()`, after the existing `if s.Cfg.MixedPort == 0 {...}` backfill block, add:

```go
	if s.Cfg.Mode == "" {
		s.Cfg.Mode = "rule"
		changed = true
	}
	if s.Cfg.GlobalTarget == "" {
		s.Cfg.GlobalTarget = "🚀 Proxy"
		changed = true
	}
	if s.Cfg.Subscriptions == nil {
		s.Cfg.Subscriptions = []Subscription{}
		changed = true
	}
	if s.Cfg.LocalNodes == nil {
		s.Cfg.LocalNodes = []LocalNode{}
		changed = true
	}
	if s.Cfg.LocalRules == nil {
		s.Cfg.LocalRules = []LocalRule{}
		changed = true
	}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
go test ./internal/store -run TestSchemaV2Roundtrip -v
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(store): schema v2 fields (Subscriptions/LocalNodes/LocalRules/Mode/GlobalTarget)"
```

### Task 1.2: Detect v1 stores and fatal-exit

**Files:**
- Modify: `internal/store/store.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/store/store_test.go`:

```go
func TestLoadRejectsV1Store(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	// A representative v0.10.x store: no schema_version, has active_profile and profiles[].
	v1 := `controller_secret = "deadbeef"
controller_port = 9090
mixed_port = 7890
rule_template = "loyalsoldier"
active_profile = "doge"
[[profiles]]
name = "doge"
url = "https://example.invalid/sub"
`
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected v1 store rejection, got nil")
	}
	if !strings.Contains(err.Error(), "schema") || !strings.Contains(err.Error(), "vpnkit init --force") {
		t.Errorf("error should mention schema upgrade + remedy command, got: %v", err)
	}
}
```

- [ ] **Step 2: Add `strings` import** if missing in test file.

- [ ] **Step 3: Run to verify it fails**

```bash
go test ./internal/store -run TestLoadRejectsV1Store
```
Expected: FAIL (no rejection happening).

- [ ] **Step 4: Add v1 detection in Load()**

In `internal/store/store.go`, **immediately after** the `toml.Unmarshal` call inside `Load()`, before the zero-value backfill:

```go
	// v1 (v0.10.x) stores have no schema_version and either active_profile
	// or profiles[] set. Refuse with a remediation hint rather than silently
	// half-migrating fields and producing a config that mixes models.
	if s.Cfg.SchemaVersion == 0 && (s.Cfg.LegacyActiveProfile != "" || len(s.Cfg.LegacyProfiles) > 0 || s.Cfg.LegacyRuleTemplate != "") {
		return nil, fmt.Errorf("store at %s uses schema v1 (vpnkit ≤ v0.10.x); "+
			"v1.0.0 changed the data model. Back up the file, then run "+
			"`vpnkit init --force` to regenerate", path)
	}
	// If schema_version is missing but no legacy fields, treat as a brand-new
	// store (e.g. partially-written file from an interrupted previous run);
	// the backfill block below will populate v2 defaults.
	if s.Cfg.SchemaVersion == 0 {
		s.Cfg.SchemaVersion = 2
		changed = true
	}
```

Add `"fmt"` to the import block if not already present.

- [ ] **Step 5: Verify**

```bash
go test ./internal/store -race
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(store): reject v1 schema with remediation hint"
```

### Task 1.3: `vpnkit init --force` backup + regenerate

**Files:**
- Modify: `cmd/vpnkit/cmd_init.go`
- Test: `cmd/vpnkit/cmd_init_test.go`

- [ ] **Step 1: Read current cmd_init.go** to understand structure.

Reference: `cmd/vpnkit/cmd_init.go` lines 1–110.

- [ ] **Step 2: Write the failing test**

Append to `cmd/vpnkit/cmd_init_test.go`:

```go
func TestInitForceBacksUpV1Store(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, ".cache"))
	storePath := filepath.Join(tmp, ".config", "vpnkit", "config.toml")
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storePath, []byte(`active_profile = "doge"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runInit(&out, runInitOpts{Force: true}); err != nil {
		t.Fatalf("init --force: %v", err)
	}

	// Original moved to .bak.<timestamp>
	matches, _ := filepath.Glob(storePath + ".bak.*")
	if len(matches) != 1 {
		t.Errorf("expected one .bak.* file, got %v", matches)
	}
	// New file is valid v2.
	st, err := store.Load(storePath)
	if err != nil {
		t.Fatalf("load post-init: %v", err)
	}
	if st.Cfg.SchemaVersion != 2 {
		t.Errorf("post-init schema: %d", st.Cfg.SchemaVersion)
	}
}
```

Add the `bytes` import if missing.

- [ ] **Step 3: Run to verify it fails**

```bash
go test ./cmd/vpnkit -run TestInitForceBacksUpV1Store
```
Expected: FAIL (no `Force` field).

- [ ] **Step 4: Add `Force` to runInitOpts**

In `cmd/vpnkit/cmd_init.go`, modify the struct:

```go
type runInitOpts struct {
	RestorePath string
	Force       bool // back up and replace any existing store regardless of schema
}
```

- [ ] **Step 5: Add backup branch at top of runInit**

In `runInit`, **before** the `store.Load` call:

```go
	if opts.Force {
		if _, err := os.Stat(p.VpnkitConfigFile()); err == nil {
			bak := fmt.Sprintf("%s.bak.%d", p.VpnkitConfigFile(), time.Now().Unix())
			if err := os.Rename(p.VpnkitConfigFile(), bak); err != nil {
				return fmt.Errorf("backup v1 store: %w", err)
			}
			fmt.Fprintf(out, "🗄️  backed up old store to %s\n", bak)
		}
	}
```

Add `"time"` import if missing.

- [ ] **Step 6: Wire `--force` flag**

Find `dispatchInit` in `cmd/vpnkit/main.go`. Replace its body:

```go
func dispatchInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	restore := fs.String("restore", "", "path to a profiles backup TOML to merge")
	force := fs.Bool("force", false, "back up any existing store before regenerating (use to recover from v1 → v2)")
	_ = fs.Bool("non-interactive", false, "(no-op; init is always non-interactive)")
	_ = fs.Parse(args)
	if err := runInit(os.Stdout, runInitOpts{RestorePath: *restore, Force: *force}); err != nil {
		dieRuntime("vpnkit init: %v", err)
	}
}
```

- [ ] **Step 7: Run tests**

```bash
go test ./cmd/vpnkit -race -run TestInit
```
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add cmd/vpnkit/cmd_init.go cmd/vpnkit/cmd_init_test.go cmd/vpnkit/main.go
git commit -m "feat(cmd/init): --force flag backs up old store and regenerates v2"
```

### Task 1.4: Phase 1 verification

- [ ] **Step 1: Full test suite + vet**

```bash
go vet ./...
go test ./... -race
```
Expected: all green.

- [ ] **Step 2: Commit nothing if no changes; otherwise note**.

---

## Phase 2: `localnodes` package

Hand-entered nodes with URI parsers for 6 protocols.

### Task 2.1: Package skeleton + Node/Manager types

**Files:**
- Create: `internal/localnodes/localnodes.go`
- Create: `internal/localnodes/localnodes_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localnodes/localnodes_test.go`:

```go
package localnodes

import (
	"testing"
)

func TestManagerCRUD(t *testing.T) {
	m := New()
	n := Node{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 443, Fields: map[string]any{"password": "x"}}
	if err := m.Add(n); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := m.Add(n); err == nil {
		t.Error("expected duplicate-name error")
	}
	got, ok := m.Get("HK-A")
	if !ok {
		t.Fatal("Get HK-A: not found")
	}
	if got.Server != "1.2.3.4" {
		t.Errorf("server: got %q", got.Server)
	}
	all := m.All()
	if len(all) != 1 {
		t.Errorf("All len: %d", len(all))
	}
	if err := m.Remove("HK-A"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := m.Get("HK-A"); ok {
		t.Error("Get after Remove: still present")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/localnodes
```
Expected: build error (`package localnodes` missing).

- [ ] **Step 3: Create the package**

Create `internal/localnodes/localnodes.go`:

```go
// Package localnodes manages user-entered proxy nodes that supplement
// subscription-fetched ones. Persisted via store.LocalNode but the in-memory
// Manager owns all mutation paths so callers don't have to know about toml.
package localnodes

import (
	"errors"
	"sync"
)

// Node mirrors store.LocalNode but lives independently so this package has
// no dependency on store (avoids an import cycle once assembler imports
// both packages). Conversion helpers are in this package's converter.go.
type Node struct {
	Name   string
	Proto  string // ss | vmess | vless | trojan | hysteria2 | tuic
	Server string
	Port   int
	Fields map[string]any
}

// Manager is the goroutine-safe owner of a node list.
type Manager struct {
	mu    sync.Mutex
	nodes []Node
}

func New() *Manager { return &Manager{nodes: []Node{}} }

func (m *Manager) Load(initial []Node) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes = append([]Node(nil), initial...)
}

func (m *Manager) Add(n Node) error {
	if n.Name == "" {
		return errors.New("localnodes: name required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.nodes {
		if x.Name == n.Name {
			return errors.New("localnodes: duplicate name " + n.Name)
		}
	}
	m.nodes = append(m.nodes, n)
	return nil
}

func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, x := range m.nodes {
		if x.Name == name {
			m.nodes = append(m.nodes[:i], m.nodes[i+1:]...)
			return nil
		}
	}
	return errors.New("localnodes: not found " + name)
}

func (m *Manager) Get(name string) (Node, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.nodes {
		if x.Name == name {
			return x, true
		}
	}
	return Node{}, false
}

func (m *Manager) All() []Node {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Node, len(m.nodes))
	copy(out, m.nodes)
	return out
}

func (m *Manager) Update(name string, mut func(*Node) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.nodes {
		if m.nodes[i].Name == name {
			return mut(&m.nodes[i])
		}
	}
	return errors.New("localnodes: not found " + name)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/localnodes -race
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/localnodes/
git commit -m "feat(localnodes): Node + thread-safe Manager (CRUD)"
```

### Task 2.2: ParseURI for `ss://`

**Files:**
- Create: `internal/localnodes/parse.go`
- Create: `internal/localnodes/parse_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localnodes/parse_test.go`:

```go
package localnodes

import "testing"

func TestParseURIShadowsocks(t *testing.T) {
	// Format: ss://BASE64(method:password)@host:port#name
	// Pre-computed: base64("aes-256-gcm:secret") == "YWVzLTI1Ni1nY206c2VjcmV0"
	uri := "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-A"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "ss" {
		t.Errorf("proto: %q", n.Proto)
	}
	if n.Server != "1.2.3.4" || n.Port != 8388 {
		t.Errorf("server/port: %q/%d", n.Server, n.Port)
	}
	if n.Name != "HK-A" {
		t.Errorf("name: %q", n.Name)
	}
	if n.Fields["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher: %v", n.Fields["cipher"])
	}
	if n.Fields["password"] != "secret" {
		t.Errorf("password: %v", n.Fields["password"])
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/localnodes -run TestParseURIShadowsocks
```
Expected: build error.

- [ ] **Step 3: Create parser**

Create `internal/localnodes/parse.go`:

```go
package localnodes

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseURI dispatches on the URI's scheme to one of the protocol-specific
// parsers. Names are taken from the URI fragment (#name) when present,
// otherwise a stable fallback derived from server:port is used.
func ParseURI(raw string) (Node, error) {
	if i := strings.Index(raw, "://"); i < 0 {
		return Node{}, errors.New("parse: missing scheme")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Node{}, fmt.Errorf("parse: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "ss":
		return parseSS(u, raw)
	case "vmess":
		return parseVmess(u, raw)
	case "vless":
		return parseVless(u, raw)
	case "trojan":
		return parseTrojan(u, raw)
	case "hysteria2", "hy2":
		return parseHy2(u, raw)
	case "tuic":
		return parseTuic(u, raw)
	default:
		return Node{}, fmt.Errorf("parse: unsupported scheme %q", u.Scheme)
	}
}

func nameOrFallback(u *url.URL) string {
	if u.Fragment != "" {
		// URI fragment is already decoded by url.Parse.
		return u.Fragment
	}
	return u.Host
}

func parseSS(u *url.URL, raw string) (Node, error) {
	// ss://BASE64(method:password)@host:port#name  (SIP002)
	// Some sources use the older ss://BASE64(method:password@host:port)#name form;
	// we cover only SIP002 here (current mihomo standard).
	userInfo := u.User.String()
	if userInfo == "" {
		return Node{}, errors.New("parse(ss): missing userinfo")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(userInfo)
	if err != nil {
		// Some sources pad; try StdEncoding too.
		decoded, err = base64.StdEncoding.DecodeString(userInfo)
		if err != nil {
			return Node{}, fmt.Errorf("parse(ss): bad base64 userinfo: %w", err)
		}
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return Node{}, errors.New("parse(ss): userinfo must be method:password")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, fmt.Errorf("parse(ss): bad port: %w", err)
	}
	return Node{
		Name:   nameOrFallback(u),
		Proto:  "ss",
		Server: u.Hostname(),
		Port:   port,
		Fields: map[string]any{
			"cipher":   parts[0],
			"password": parts[1],
		},
	}, nil
}

// stub the other parsers so the package compiles before we implement them.
func parseVmess(u *url.URL, raw string) (Node, error)  { return Node{}, errors.New("vmess: not implemented yet") }
func parseVless(u *url.URL, raw string) (Node, error)  { return Node{}, errors.New("vless: not implemented yet") }
func parseTrojan(u *url.URL, raw string) (Node, error) { return Node{}, errors.New("trojan: not implemented yet") }
func parseHy2(u *url.URL, raw string) (Node, error)    { return Node{}, errors.New("hy2: not implemented yet") }
func parseTuic(u *url.URL, raw string) (Node, error)   { return Node{}, errors.New("tuic: not implemented yet") }
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/localnodes -run TestParseURI -race
```
Expected: SS test PASS, other parsers' tests don't exist yet.

- [ ] **Step 5: Commit**

```bash
git add internal/localnodes/parse.go internal/localnodes/parse_test.go
git commit -m "feat(localnodes): ParseURI dispatch + ss:// parser"
```

### Task 2.3: ParseURI for `vmess://`

**Files:**
- Modify: `internal/localnodes/parse.go`
- Modify: `internal/localnodes/parse_test.go`

- [ ] **Step 1: Write the failing test**

Append to `parse_test.go`:

```go
func TestParseURIVmess(t *testing.T) {
	// vmess://BASE64({"v":"2","ps":"node-name","add":"1.2.3.4","port":"8443","id":"uuid-here","aid":"0","net":"ws","type":"none","host":"","path":"/path","tls":"tls"})
	payload := `{"v":"2","ps":"JP-Tokyo","add":"1.2.3.4","port":"8443","id":"11111111-2222-3333-4444-555555555555","aid":"0","net":"ws","type":"none","host":"example.com","path":"/path","tls":"tls"}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "vmess" || n.Server != "1.2.3.4" || n.Port != 8443 {
		t.Errorf("basic fields: %+v", n)
	}
	if n.Name != "JP-Tokyo" {
		t.Errorf("name from ps: %q", n.Name)
	}
	if n.Fields["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid: %v", n.Fields["uuid"])
	}
	if n.Fields["network"] != "ws" {
		t.Errorf("network: %v", n.Fields["network"])
	}
	if ws, _ := n.Fields["ws-opts"].(map[string]any); ws["path"] != "/path" {
		t.Errorf("ws-opts.path: %v", ws)
	}
}
```

Add `"encoding/base64"` import to the test file if missing.

- [ ] **Step 2: Run to verify it fails**

Expected: error "vmess: not implemented yet".

- [ ] **Step 3: Replace stub with implementation**

Replace `parseVmess` in `parse.go`:

```go
func parseVmess(_ *url.URL, raw string) (Node, error) {
	// vmess://BASE64(json) — the JSON is the canonical clash node minus the
	// type/name keys; convert to mihomo-style fields here.
	const prefix = "vmess://"
	if !strings.HasPrefix(raw, prefix) {
		return Node{}, errors.New("parse(vmess): missing prefix")
	}
	b64 := strings.TrimPrefix(raw, prefix)
	if i := strings.IndexAny(b64, "#?"); i >= 0 {
		b64 = b64[:i]
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(b64)
		if err != nil {
			return Node{}, fmt.Errorf("parse(vmess): bad base64: %w", err)
		}
	}
	var raw_ struct {
		PS   string      `json:"ps"`
		Add  string      `json:"add"`
		Port any         `json:"port"`
		ID   string      `json:"id"`
		Aid  any         `json:"aid"`
		Net  string      `json:"net"`
		Type string      `json:"type"`
		Host string      `json:"host"`
		Path string      `json:"path"`
		TLS  string      `json:"tls"`
		SNI  string      `json:"sni"`
	}
	if err := json.Unmarshal(decoded, &raw_); err != nil {
		return Node{}, fmt.Errorf("parse(vmess): bad json: %w", err)
	}
	port, err := anyToInt(raw_.Port)
	if err != nil {
		return Node{}, fmt.Errorf("parse(vmess): bad port: %w", err)
	}
	aid, _ := anyToInt(raw_.Aid)
	fields := map[string]any{
		"uuid":    raw_.ID,
		"alterId": aid,
		"cipher":  "auto",
		"network": raw_.Net,
	}
	if raw_.TLS == "tls" {
		fields["tls"] = true
		if raw_.SNI != "" {
			fields["servername"] = raw_.SNI
		} else if raw_.Host != "" {
			fields["servername"] = raw_.Host
		}
	}
	if raw_.Net == "ws" {
		wsOpts := map[string]any{}
		if raw_.Path != "" {
			wsOpts["path"] = raw_.Path
		}
		if raw_.Host != "" {
			wsOpts["headers"] = map[string]any{"Host": raw_.Host}
		}
		fields["ws-opts"] = wsOpts
	}
	return Node{
		Name:   raw_.PS,
		Proto:  "vmess",
		Server: raw_.Add,
		Port:   port,
		Fields: fields,
	}, nil
}

func anyToInt(v any) (int, error) {
	switch x := v.(type) {
	case float64:
		return int(x), nil
	case int:
		return x, nil
	case string:
		return strconv.Atoi(x)
	default:
		return 0, fmt.Errorf("anyToInt: unsupported %T", v)
	}
}
```

Add `"encoding/json"` import.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/localnodes -run TestParseURIVmess -race
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/localnodes/parse.go internal/localnodes/parse_test.go
git commit -m "feat(localnodes): vmess:// parser (JSON+base64 form)"
```

### Task 2.4: ParseURI for `trojan://`

**Files:** modify `parse.go` + `parse_test.go`.

- [ ] **Step 1: Write the failing test**

```go
func TestParseURITrojan(t *testing.T) {
	uri := "trojan://password123@1.2.3.4:8443?sni=example.com&alpn=h2,http/1.1#TR-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "trojan" || n.Server != "1.2.3.4" || n.Port != 8443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Name != "TR-1" {
		t.Errorf("name: %q", n.Name)
	}
	if n.Fields["password"] != "password123" {
		t.Errorf("password: %v", n.Fields["password"])
	}
	if n.Fields["sni"] != "example.com" {
		t.Errorf("sni: %v", n.Fields["sni"])
	}
	if alpn, _ := n.Fields["alpn"].([]string); len(alpn) != 2 || alpn[0] != "h2" {
		t.Errorf("alpn: %v", n.Fields["alpn"])
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Replace stub**

```go
func parseTrojan(u *url.URL, raw string) (Node, error) {
	password := u.User.Username()
	if password == "" {
		return Node{}, errors.New("parse(trojan): missing password (userinfo)")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, fmt.Errorf("parse(trojan): bad port: %w", err)
	}
	q := u.Query()
	fields := map[string]any{
		"password": password,
	}
	if sni := q.Get("sni"); sni != "" {
		fields["sni"] = sni
	}
	if alpn := q.Get("alpn"); alpn != "" {
		fields["alpn"] = strings.Split(alpn, ",")
	}
	if q.Get("allowInsecure") == "1" || q.Get("skip-cert-verify") == "1" {
		fields["skip-cert-verify"] = true
	}
	return Node{
		Name:   nameOrFallback(u),
		Proto:  "trojan",
		Server: u.Hostname(),
		Port:   port,
		Fields: fields,
	}, nil
}
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/localnodes -run TestParseURITrojan -race
git add internal/localnodes/
git commit -m "feat(localnodes): trojan:// parser"
```

### Task 2.5: ParseURI for `vless://`

**Files:** modify `parse.go` + `parse_test.go`.

- [ ] **Step 1: Write the failing test**

```go
func TestParseURIVless(t *testing.T) {
	// vless://UUID@host:port?encryption=none&security=reality&pbk=KEY&sni=...&type=tcp#name
	uri := "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?encryption=none&security=reality&pbk=publicKeyBase64&sni=example.com&type=tcp&flow=xtls-rprx-vision#VL-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "vless" || n.Server != "1.2.3.4" || n.Port != 443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Fields["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid: %v", n.Fields["uuid"])
	}
	if n.Fields["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow: %v", n.Fields["flow"])
	}
	if n.Fields["tls"] != true {
		t.Errorf("tls: %v", n.Fields["tls"])
	}
	if r, _ := n.Fields["reality-opts"].(map[string]any); r["public-key"] != "publicKeyBase64" {
		t.Errorf("reality public-key: %v", n.Fields["reality-opts"])
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

```go
func parseVless(u *url.URL, raw string) (Node, error) {
	uuid := u.User.Username()
	if uuid == "" {
		return Node{}, errors.New("parse(vless): missing uuid (userinfo)")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, fmt.Errorf("parse(vless): bad port: %w", err)
	}
	q := u.Query()
	fields := map[string]any{
		"uuid":    uuid,
		"network": q.Get("type"),
	}
	if fields["network"] == "" {
		fields["network"] = "tcp"
	}
	if flow := q.Get("flow"); flow != "" {
		fields["flow"] = flow
	}
	switch q.Get("security") {
	case "tls":
		fields["tls"] = true
		if sni := q.Get("sni"); sni != "" {
			fields["servername"] = sni
		}
	case "reality":
		fields["tls"] = true
		ro := map[string]any{}
		if pbk := q.Get("pbk"); pbk != "" {
			ro["public-key"] = pbk
		}
		if sid := q.Get("sid"); sid != "" {
			ro["short-id"] = sid
		}
		if sni := q.Get("sni"); sni != "" {
			fields["servername"] = sni
		}
		fields["reality-opts"] = ro
	}
	return Node{
		Name:   nameOrFallback(u),
		Proto:  "vless",
		Server: u.Hostname(),
		Port:   port,
		Fields: fields,
	}, nil
}
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/localnodes -run TestParseURIVless -race
git add internal/localnodes/
git commit -m "feat(localnodes): vless:// parser (incl. reality)"
```

### Task 2.6: ParseURI for `hysteria2://`

**Files:** modify `parse.go` + `parse_test.go`.

- [ ] **Step 1: Write the failing test**

```go
func TestParseURIHysteria2(t *testing.T) {
	uri := "hysteria2://password@1.2.3.4:443?sni=example.com&insecure=1&up=100&down=200&obfs=salamander&obfs-password=ofuscatekey#HY2-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "hysteria2" || n.Server != "1.2.3.4" || n.Port != 443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Fields["password"] != "password" {
		t.Errorf("password: %v", n.Fields["password"])
	}
	if n.Fields["up"] != "100 Mbps" || n.Fields["down"] != "200 Mbps" {
		t.Errorf("up/down: %v/%v", n.Fields["up"], n.Fields["down"])
	}
	if n.Fields["obfs"] != "salamander" || n.Fields["obfs-password"] != "ofuscatekey" {
		t.Errorf("obfs: %v / %v", n.Fields["obfs"], n.Fields["obfs-password"])
	}
	if n.Fields["skip-cert-verify"] != true {
		t.Errorf("skip-cert-verify: %v", n.Fields["skip-cert-verify"])
	}
}

// Also support the hy2:// alias.
func TestParseURIHy2Alias(t *testing.T) {
	uri := "hy2://password@1.2.3.4:443"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "hysteria2" {
		t.Errorf("proto should normalize to hysteria2, got %q", n.Proto)
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

```go
func parseHy2(u *url.URL, raw string) (Node, error) {
	password := u.User.Username()
	if password == "" {
		return Node{}, errors.New("parse(hy2): missing password (userinfo)")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, fmt.Errorf("parse(hy2): bad port: %w", err)
	}
	q := u.Query()
	fields := map[string]any{
		"password": password,
	}
	if sni := q.Get("sni"); sni != "" {
		fields["sni"] = sni
	}
	if q.Get("insecure") == "1" || q.Get("skip-cert-verify") == "1" {
		fields["skip-cert-verify"] = true
	}
	if up := q.Get("up"); up != "" {
		fields["up"] = up + " Mbps"
	}
	if down := q.Get("down"); down != "" {
		fields["down"] = down + " Mbps"
	}
	if obfs := q.Get("obfs"); obfs != "" {
		fields["obfs"] = obfs
		if pw := q.Get("obfs-password"); pw != "" {
			fields["obfs-password"] = pw
		}
	}
	return Node{
		Name:   nameOrFallback(u),
		Proto:  "hysteria2", // normalize hy2 alias
		Server: u.Hostname(),
		Port:   port,
		Fields: fields,
	}, nil
}
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/localnodes -run TestParseURIHy -race
git add internal/localnodes/
git commit -m "feat(localnodes): hysteria2:// + hy2:// parser"
```

### Task 2.7: ParseURI for `tuic://`

**Files:** modify `parse.go` + `parse_test.go`.

- [ ] **Step 1: Write the failing test**

```go
func TestParseURITuic(t *testing.T) {
	uri := "tuic://UUID:PASSWORD@1.2.3.4:443?sni=example.com&congestion_control=bbr&udp_relay_mode=native&alpn=h3#TUIC-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "tuic" || n.Server != "1.2.3.4" || n.Port != 443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Fields["uuid"] != "UUID" || n.Fields["password"] != "PASSWORD" {
		t.Errorf("uuid/password: %v/%v", n.Fields["uuid"], n.Fields["password"])
	}
	if n.Fields["congestion-controller"] != "bbr" {
		t.Errorf("congestion-controller: %v", n.Fields["congestion-controller"])
	}
	if n.Fields["sni"] != "example.com" {
		t.Errorf("sni: %v", n.Fields["sni"])
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

```go
func parseTuic(u *url.URL, raw string) (Node, error) {
	uuid := u.User.Username()
	password, _ := u.User.Password()
	if uuid == "" || password == "" {
		return Node{}, errors.New("parse(tuic): userinfo must be uuid:password")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, fmt.Errorf("parse(tuic): bad port: %w", err)
	}
	q := u.Query()
	fields := map[string]any{
		"uuid":     uuid,
		"password": password,
	}
	if sni := q.Get("sni"); sni != "" {
		fields["sni"] = sni
	}
	if cc := q.Get("congestion_control"); cc != "" {
		fields["congestion-controller"] = cc
	}
	if udp := q.Get("udp_relay_mode"); udp != "" {
		fields["udp-relay-mode"] = udp
	}
	if alpn := q.Get("alpn"); alpn != "" {
		fields["alpn"] = strings.Split(alpn, ",")
	}
	return Node{
		Name:   nameOrFallback(u),
		Proto:  "tuic",
		Server: u.Hostname(),
		Port:   port,
		Fields: fields,
	}, nil
}
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/localnodes -race
git add internal/localnodes/
git commit -m "feat(localnodes): tuic:// parser"
```

### Task 2.8: Convert helpers (Node ↔ mihomo proxy map)

**Files:**
- Create: `internal/localnodes/convert.go`
- Create: `internal/localnodes/convert_test.go`

- [ ] **Step 1: Write the failing test**

```go
package localnodes

import (
	"testing"
)

func TestToProxyMapHysteria2(t *testing.T) {
	n := Node{
		Name: "HK-A", Proto: "hysteria2", Server: "1.2.3.4", Port: 443,
		Fields: map[string]any{"password": "x", "up": "100 Mbps", "down": "200 Mbps", "sni": "example.com"},
	}
	m := ToProxyMap(n)
	if m["name"] != "HK-A" || m["type"] != "hysteria2" || m["server"] != "1.2.3.4" || m["port"] != 443 {
		t.Errorf("basic: %v", m)
	}
	if m["password"] != "x" || m["up"] != "100 Mbps" || m["sni"] != "example.com" {
		t.Errorf("fields not flattened: %v", m)
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

```go
package localnodes

// ToProxyMap converts a Node into a mihomo proxy map (the shape that goes
// into config.yaml's `proxies:` array). All Fields are flattened into the
// top-level map; reserved keys (name/type/server/port) come from Node.
func ToProxyMap(n Node) map[string]any {
	m := make(map[string]any, 4+len(n.Fields))
	m["name"] = n.Name
	m["type"] = n.Proto
	m["server"] = n.Server
	m["port"] = n.Port
	for k, v := range n.Fields {
		m[k] = v
	}
	return m
}
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/localnodes -race
git add internal/localnodes/convert.go internal/localnodes/convert_test.go
git commit -m "feat(localnodes): ToProxyMap converter"
```

### Task 2.9: Phase 2 coverage check

- [ ] **Step 1: Run coverage**

```bash
go test ./internal/localnodes -cover
```
Expected: ≥85% line coverage. If below, list uncovered lines (`go test -coverprofile=/tmp/c.out && go tool cover -func=/tmp/c.out`) and add tests until threshold met.

---

## Phase 3: `localrules` package

### Task 3.1: Rule + Manager + Render

**Files:**
- Create: `internal/localrules/localrules.go`
- Create: `internal/localrules/localrules_test.go`

- [ ] **Step 1: Write the failing test**

```go
package localrules

import (
	"testing"
)

func TestRender(t *testing.T) {
	cases := []struct {
		in   Rule
		want string
	}{
		{Rule{"DOMAIN-SUFFIX", "baidu.com", "🎯 Direct"}, "DOMAIN-SUFFIX,baidu.com,🎯 Direct"},
		{Rule{"MATCH", "", "🚀 Proxy"}, "MATCH,🚀 Proxy"}, // MATCH has empty payload
		{Rule{"GEOIP", "CN", "🎯 Direct"}, "GEOIP,CN,🎯 Direct"},
		{Rule{"RULE-SET", "gfw", "🚀 Proxy"}, "RULE-SET,gfw,🚀 Proxy"},
	}
	for _, tc := range cases {
		got := tc.in.Render()
		if got != tc.want {
			t.Errorf("Render(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestManagerCRUDAndReorder(t *testing.T) {
	m := New()
	_ = m.Add(Rule{"DOMAIN-SUFFIX", "a.com", "🎯 Direct"})
	_ = m.Add(Rule{"DOMAIN-SUFFIX", "b.com", "🚀 Proxy"})
	_ = m.Add(Rule{"DOMAIN-SUFFIX", "c.com", "🛑 Reject"})
	if len(m.All()) != 3 {
		t.Errorf("All len: %d", len(m.All()))
	}
	if err := m.Move(0, 2); err != nil {
		t.Fatalf("Move: %v", err)
	}
	all := m.All()
	if all[0].Payload != "b.com" || all[1].Payload != "c.com" || all[2].Payload != "a.com" {
		t.Errorf("after Move(0,2): %+v", all)
	}
	if err := m.Remove(1); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(m.All()) != 2 {
		t.Errorf("len after Remove: %d", len(m.All()))
	}
}

func TestValidateRejectsUnknownType(t *testing.T) {
	m := New()
	err := m.Add(Rule{"BOGUS-TYPE", "x", "y"})
	if err == nil {
		t.Error("expected validation error for unknown type")
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

Create `internal/localrules/localrules.go`:

```go
// Package localrules manages user-authored mihomo rule entries kept in
// store.toml under [[local_rules]]. Order matters (first match wins) and
// the Manager preserves insertion order while supporting Move for reorder.
package localrules

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Rule is one entry. Type + Payload + Target map directly to mihomo's rule
// line syntax. MATCH and FINAL have empty Payload by convention.
type Rule struct {
	Type    string
	Payload string
	Target  string
}

// Render produces the mihomo rule string. MATCH/FINAL omit the payload field.
func (r Rule) Render() string {
	if r.Type == "MATCH" || r.Type == "FINAL" {
		return r.Type + "," + r.Target
	}
	return strings.Join([]string{r.Type, r.Payload, r.Target}, ",")
}

// validTypes is the whitelist of mihomo rule types this package accepts.
// Source: https://wiki.metacubex.one/config/rules/
var validTypes = map[string]bool{
	"DOMAIN":           true,
	"DOMAIN-SUFFIX":    true,
	"DOMAIN-KEYWORD":   true,
	"DOMAIN-REGEX":     true,
	"GEOSITE":          true,
	"IP-CIDR":          true,
	"IP-CIDR6":         true,
	"IP-SUFFIX":        true,
	"IP-ASN":           true,
	"GEOIP":            true,
	"SRC-GEOIP":        true,
	"SRC-IP-ASN":       true,
	"SRC-IP-CIDR":      true,
	"SRC-IP-SUFFIX":    true,
	"DST-PORT":         true,
	"SRC-PORT":         true,
	"IN-PORT":          true,
	"IN-TYPE":          true,
	"IN-USER":          true,
	"IN-NAME":          true,
	"PROCESS-PATH":     true,
	"PROCESS-PATH-REGEX": true,
	"PROCESS-NAME":     true,
	"PROCESS-NAME-REGEX": true,
	"UID":              true,
	"NETWORK":          true,
	"DSCP":             true,
	"RULE-SET":         true,
	"AND":              true,
	"OR":               true,
	"NOT":              true,
	"SUB-RULE":         true,
	"MATCH":            true,
	"FINAL":            true,
}

// Validate returns nil if the Rule is acceptable for assembly.
func Validate(r Rule) error {
	if !validTypes[r.Type] {
		return fmt.Errorf("localrules: unknown rule type %q", r.Type)
	}
	if r.Type != "MATCH" && r.Type != "FINAL" && r.Payload == "" {
		return fmt.Errorf("localrules: type %q requires payload", r.Type)
	}
	if r.Target == "" {
		return errors.New("localrules: target required")
	}
	return nil
}

// Manager owns the in-memory rules list; persistence is done by callers
// translating to []store.LocalRule.
type Manager struct {
	mu    sync.Mutex
	rules []Rule
}

func New() *Manager { return &Manager{rules: []Rule{}} }

func (m *Manager) Load(initial []Rule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append([]Rule(nil), initial...)
}

func (m *Manager) Add(r Rule) error {
	if err := Validate(r); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, r)
	return nil
}

func (m *Manager) Remove(idx int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if idx < 0 || idx >= len(m.rules) {
		return fmt.Errorf("localrules: index %d out of range", idx)
	}
	m.rules = append(m.rules[:idx], m.rules[idx+1:]...)
	return nil
}

func (m *Manager) Move(from, to int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if from < 0 || from >= len(m.rules) || to < 0 || to >= len(m.rules) {
		return fmt.Errorf("localrules: bad indices %d→%d", from, to)
	}
	r := m.rules[from]
	m.rules = append(m.rules[:from], m.rules[from+1:]...)
	m.rules = append(m.rules[:to], append([]Rule{r}, m.rules[to:]...)...)
	return nil
}

func (m *Manager) All() []Rule {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Rule, len(m.rules))
	copy(out, m.rules)
	return out
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/localrules -race
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/localrules/
git commit -m "feat(localrules): Rule + Manager + Render + Validate"
```

---

## Phase 4: `groups` package

### Task 4.1: Group interface + SubscriptionGroup

**Files:**
- Create: `internal/groups/groups.go`
- Create: `internal/groups/subscription.go`
- Create: `internal/groups/groups_test.go`

- [ ] **Step 1: Write the failing test**

```go
package groups

import (
	"testing"
	"vpnkit/internal/localrules"
	"vpnkit/internal/subscription"
)

func TestSubscriptionGroupContract(t *testing.T) {
	res := &subscription.Result{
		Source: "clash",
		Proxies: []subscription.Proxy{
			{"name": "HK-A", "type": "ss", "server": "1.2.3.4", "port": 443, "cipher": "aes-256-gcm", "password": "x"},
			{"name": "JP-B", "type": "vmess", "server": "5.6.7.8", "port": 8443, "uuid": "u"},
		},
		Raw: map[string]any{
			"rules": []any{
				"DOMAIN-SUFFIX,youtube.com,🚀 Proxy",
				"DOMAIN-SUFFIX,netflix.com,🚀 Proxy",
			},
		},
	}
	g := NewSubscriptionGroup("doge", true, res)
	if g.Name() != "doge" {
		t.Errorf("Name: %q", g.Name())
	}
	if g.Kind() != KindSubscription {
		t.Errorf("Kind: %v", g.Kind())
	}
	if !g.Enabled() {
		t.Error("Enabled should be true")
	}
	if len(g.Proxies()) != 2 {
		t.Errorf("Proxies len: %d", len(g.Proxies()))
	}
	rules := g.Rules()
	if len(rules) != 2 {
		t.Fatalf("Rules len: %d", len(rules))
	}
	if rules[0] != (localrules.Rule{Type: "DOMAIN-SUFFIX", Payload: "youtube.com", Target: "🚀 Proxy"}) {
		t.Errorf("Rules[0]: %+v", rules[0])
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

Create `internal/groups/groups.go`:

```go
// Package groups models the unit at which vpnkit aggregates proxies and
// rules for assembly. A Group is either a Subscription (remote yaml) or a
// LocalNodes set (hand-entered).
package groups

import (
	"vpnkit/internal/localrules"
	"vpnkit/internal/subscription"
)

type Kind int

const (
	KindSubscription Kind = iota + 1
	KindLocalNodes
)

// Group is the common contract assembler consumes.
type Group interface {
	Name() string
	Kind() Kind
	Enabled() bool
	Proxies() []subscription.Proxy
	Rules() []localrules.Rule // nil when the group has no own rules
}
```

Create `internal/groups/subscription.go`:

```go
package groups

import (
	"strings"
	"vpnkit/internal/localrules"
	"vpnkit/internal/subscription"
)

type subscriptionGroup struct {
	name    string
	enabled bool
	result  *subscription.Result
}

// NewSubscriptionGroup wraps a fetched+converted subscription.Result so the
// assembler sees it through the Group interface. The result must be non-nil
// (caller responsibility); pass enabled=false to short-circuit emission.
func NewSubscriptionGroup(name string, enabled bool, res *subscription.Result) Group {
	return &subscriptionGroup{name: name, enabled: enabled, result: res}
}

func (g *subscriptionGroup) Name() string                  { return g.name }
func (g *subscriptionGroup) Kind() Kind                    { return KindSubscription }
func (g *subscriptionGroup) Enabled() bool                 { return g.enabled }
func (g *subscriptionGroup) Proxies() []subscription.Proxy { return g.result.Proxies }

// Rules extracts the "rules:" key from a clash-style subscription Raw. Each
// line is parsed into a localrules.Rule. Lines we can't parse are skipped
// (subscriptions sometimes contain mihomo-only or older formats).
func (g *subscriptionGroup) Rules() []localrules.Rule {
	if g.result == nil || g.result.Raw == nil {
		return nil
	}
	rawRules, ok := g.result.Raw["rules"].([]any)
	if !ok {
		return nil
	}
	out := make([]localrules.Rule, 0, len(rawRules))
	for _, line := range rawRules {
		s, ok := line.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(s, ",", 3)
		switch len(parts) {
		case 2: // MATCH,target
			out = append(out, localrules.Rule{Type: parts[0], Target: parts[1]})
		case 3:
			out = append(out, localrules.Rule{Type: parts[0], Payload: parts[1], Target: parts[2]})
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/groups -race
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/groups/
git commit -m "feat(groups): Group interface + SubscriptionGroup"
```

### Task 4.2: LocalNodesGroup

**Files:**
- Create: `internal/groups/local.go`
- Modify: `internal/groups/groups_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestLocalNodesGroupContract(t *testing.T) {
	m := localnodes.New()
	_ = m.Add(localnodes.Node{Name: "HK-Manual", Proto: "hysteria2", Server: "1.2.3.4", Port: 443, Fields: map[string]any{"password": "x"}})
	g := NewLocalNodesGroup("local", m)
	if g.Kind() != KindLocalNodes {
		t.Errorf("Kind: %v", g.Kind())
	}
	if !g.Enabled() {
		t.Error("Enabled should be true (always for local)")
	}
	prox := g.Proxies()
	if len(prox) != 1 || prox[0]["name"] != "HK-Manual" {
		t.Errorf("Proxies: %v", prox)
	}
	if g.Rules() != nil {
		t.Errorf("LocalNodesGroup should expose nil Rules: %v", g.Rules())
	}
}
```

Add import `"vpnkit/internal/localnodes"`.

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

Create `internal/groups/local.go`:

```go
package groups

import (
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription"
)

type localNodesGroup struct {
	name string
	mgr  *localnodes.Manager
}

// NewLocalNodesGroup wraps a localnodes.Manager. Always Enabled; the user's
// LocalRules subsystem provides routing, so this Group has no own Rules.
func NewLocalNodesGroup(name string, m *localnodes.Manager) Group {
	return &localNodesGroup{name: name, mgr: m}
}

func (g *localNodesGroup) Name() string    { return g.name }
func (g *localNodesGroup) Kind() Kind      { return KindLocalNodes }
func (g *localNodesGroup) Enabled() bool   { return true }

func (g *localNodesGroup) Proxies() []subscription.Proxy {
	all := g.mgr.All()
	out := make([]subscription.Proxy, len(all))
	for i, n := range all {
		out[i] = subscription.Proxy(localnodes.ToProxyMap(n))
	}
	return out
}

func (g *localNodesGroup) Rules() []localrules.Rule { return nil }
```

Add import `"vpnkit/internal/localrules"` (used in return type).

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/groups -race
git add internal/groups/
git commit -m "feat(groups): LocalNodesGroup wrapping localnodes.Manager"
```

---

## Phase 5: `assembler` package

### Task 5.1: Skeleton + base config keys

**Files:**
- Create: `internal/assembler/assembler.go`
- Create: `internal/assembler/assembler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package assembler

import (
	"strings"
	"testing"
)

func TestAssembleEmitsBaseConfig(t *testing.T) {
	out, err := Assemble(Input{
		Mode:             ModeRule,
		GlobalTarget:     "🚀 Proxy",
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "secret",
		ProxyUser:        "user",
		ProxyPass:        "pass",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"mixed-port: 50595",
		"external-controller: 127.0.0.1:32645",
		"secret: secret",
		"bind-address: 127.0.0.1",
		"allow-lan: false",
		"mode: rule",
		"vpnkit-",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement skeleton**

Create `internal/assembler/assembler.go`:

```go
// Package assembler builds the final mihomo config.yaml from vpnkit's
// in-memory state: subscription groups, the local-nodes group, local rules,
// extensions overlay, and the top-level routing knobs (Mode + GlobalTarget).
package assembler

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/extensions"
	"vpnkit/internal/groups"
	"vpnkit/internal/localrules"
)

type Mode string

const (
	ModeRule   Mode = "rule"
	ModeGlobal Mode = "global"
	ModeDirect Mode = "direct"
)

// Input is the full Assemble payload.
type Input struct {
	Mode             Mode
	GlobalTarget     string
	Subscriptions    []groups.Group
	LocalNodes       groups.Group
	LocalRules       []localrules.Rule
	Extensions       extensions.Extensions
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	ProxyUser        string
	ProxyPass        string
}

// Assemble produces the bytes that bootstrap atomically writes to
// ~/.config/mihomo/config.yaml. Pure function — no I/O.
func Assemble(in Input) ([]byte, error) {
	if in.MixedPort == 0 || in.ControllerPort == 0 {
		return nil, fmt.Errorf("assembler: ports must be set (got mixed=%d controller=%d)", in.MixedPort, in.ControllerPort)
	}
	if in.GlobalTarget == "" {
		in.GlobalTarget = "🚀 Proxy"
	}

	doc := map[string]any{
		"mixed-port":          in.MixedPort,
		"allow-lan":           false,
		"bind-address":        "127.0.0.1",
		"mode":                "rule", // vpnkit always uses rule mode; routing knob is emulated via rules.
		"log-level":           "info",
		"external-controller": fmt.Sprintf("127.0.0.1:%d", in.ControllerPort),
		"secret":              in.ControllerSecret,
		"proxies":             []any{},
		"proxy-groups":        []any{},
		"rules":               []any{},
		"geox-url":            mihomoGeoxURL(),
	}
	if in.ProxyUser != "" && in.ProxyPass != "" {
		doc["authentication"] = []string{in.ProxyUser + ":" + in.ProxyPass}
	}

	// Subsequent tasks fill proxies/proxy-groups/rules.

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

	// Reverse yaml.v3's emoji escaping.
	result := strings.NewReplacer(
		`\U0001F680`, "🚀",
		`\U0001F3AF`, "🎯",
		`\U0001F6D1`, "🛑",
		`\U000267B`, "♻️",
	).Replace(buf.String())
	return []byte(result), nil
}

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

- [ ] **Step 4: Run tests**

```bash
go test ./internal/assembler -race -run TestAssembleEmits
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/assembler/
git commit -m "feat(assembler): skeleton + base config keys"
```

### Task 5.2: Emit proxies (namespaced)

**Files:**
- Modify: `internal/assembler/assembler.go`
- Create: `internal/assembler/proxies.go`
- Create: `internal/assembler/proxies_test.go`

- [ ] **Step 1: Write the failing test**

```go
package assembler

import (
	"strings"
	"testing"
	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription"
)

func TestAssembleNamespacesProxies(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []subscription.Proxy{
			{"name": "HK-A", "type": "ss", "server": "1.2.3.4", "port": 443},
		},
	})
	local := localnodes.New()
	_ = local.Add(localnodes.Node{Name: "HK-Manual", Proto: "hysteria2", Server: "5.6.7.8", Port: 443, Fields: map[string]any{"password": "x"}})
	out, err := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{sub},
		LocalNodes:       groups.NewLocalNodesGroup("local", local),
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "name: doge:HK-A") {
		t.Errorf("subscription proxy not namespaced: %s", s)
	}
	if !strings.Contains(s, "name: local:HK-Manual") {
		t.Errorf("local node not namespaced: %s", s)
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

Create `internal/assembler/proxies.go`:

```go
package assembler

import (
	"fmt"
	"vpnkit/internal/groups"
	"vpnkit/internal/subscription"
)

// emitProxies returns a flat slice of mihomo proxy maps with every node's
// name rewritten to "<group>:<original-name>" so cross-group duplicates
// don't collide in mihomo's flat namespace.
func emitProxies(subs []groups.Group, local groups.Group) []any {
	out := []any{}
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		for _, p := range g.Proxies() {
			out = append(out, namespaced(g.Name(), p))
		}
	}
	if local != nil {
		for _, p := range local.Proxies() {
			out = append(out, namespaced(local.Name(), p))
		}
	}
	return out
}

func namespaced(groupName string, p subscription.Proxy) map[string]any {
	dup := make(map[string]any, len(p))
	for k, v := range p {
		dup[k] = v
	}
	origName, _ := dup["name"].(string)
	dup["name"] = fmt.Sprintf("%s:%s", groupName, origName)
	return dup
}
```

In `assembler.go`, wire `emitProxies` between the `doc := ...` block and the `extensions.Apply` call:

```go
	doc["proxies"] = emitProxies(in.Subscriptions, in.LocalNodes)
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/assembler -race
git add internal/assembler/
git commit -m "feat(assembler): emit proxies with <group>:<node> namespace"
```

### Task 5.3: Emit proxy-groups (sub + sub-auto + top-level)

**Files:**
- Create: `internal/assembler/proxy_groups.go`
- Create: `internal/assembler/proxy_groups_test.go`

- [ ] **Step 1: Write the failing test**

```go
package assembler

import (
	"strings"
	"testing"
	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription"
)

func TestAssembleEmitsTwoGroupsPerSubscription(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []subscription.Proxy{
			{"name": "HK-A", "type": "ss"},
			{"name": "JP-B", "type": "vmess"},
		},
	})
	local := localnodes.New()
	_ = local.Add(localnodes.Node{Name: "HK-Manual", Proto: "hysteria2"})

	out, _ := Assemble(Input{
		Subscriptions:    []groups.Group{sub},
		LocalNodes:       groups.NewLocalNodesGroup("local", local),
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
		GlobalTarget:     "doge-auto",
	})
	s := string(out)
	for _, want := range []string{
		"name: doge",                    // select
		"name: doge-auto",               // url-test
		"type: url-test",
		"name: local",                   // local group
		"name: \U0001F680 Proxy",        // top-level
		"DIRECT", "REJECT",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in:\n%s", want, s)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

Create `internal/assembler/proxy_groups.go`:

```go
package assembler

import (
	"fmt"
	"vpnkit/internal/groups"
)

const (
	healthURL      = "http://www.gstatic.com/generate_204"
	healthInterval = 300
)

func emitProxyGroups(subs []groups.Group, local groups.Group, globalTarget string) []any {
	out := []any{}
	topProxies := []string{}

	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		nodes := nodeNames(g)
		if len(nodes) == 0 {
			continue
		}
		autoName := g.Name() + "-auto"
		selectProxies := append([]string{autoName}, nodes...)
		out = append(out, map[string]any{
			"name":    g.Name(),
			"type":    "select",
			"proxies": selectProxies,
		})
		out = append(out, map[string]any{
			"name":     autoName,
			"type":     "url-test",
			"proxies":  nodes,
			"url":      healthURL,
			"interval": healthInterval,
		})
		topProxies = append(topProxies, autoName, g.Name())
	}

	if local != nil {
		localNodes := nodeNames(local)
		if len(localNodes) > 0 {
			out = append(out, map[string]any{
				"name":    local.Name(),
				"type":    "select",
				"proxies": append(localNodes, "DIRECT"),
			})
			topProxies = append(topProxies, local.Name())
		}
	}

	topProxies = append(topProxies, "DIRECT")

	// GlobalTarget goes first so mihomo picks it as the select default.
	topProxies = withTargetFirst(topProxies, globalTarget)

	out = append(out,
		map[string]any{"name": "🚀 Proxy", "type": "select", "proxies": topProxies},
		map[string]any{"name": "🎯 Direct", "type": "select", "proxies": []string{"DIRECT"}},
		map[string]any{"name": "🛑 Reject", "type": "select", "proxies": []string{"REJECT", "DIRECT"}},
	)
	return out
}

func nodeNames(g groups.Group) []string {
	prox := g.Proxies()
	out := make([]string, 0, len(prox))
	for _, p := range prox {
		origName, _ := p["name"].(string)
		out = append(out, fmt.Sprintf("%s:%s", g.Name(), origName))
	}
	return out
}

func withTargetFirst(list []string, target string) []string {
	if target == "" {
		return list
	}
	for i, x := range list {
		if x == target {
			return append([]string{target}, append(list[:i:i], list[i+1:]...)...)
		}
	}
	// Target not in list (e.g. user typed a specific node). Prepend it; mihomo
	// will accept the value even if it's a leaf proxy.
	return append([]string{target}, list...)
}
```

Wire in `assembler.go`:

```go
	doc["proxy-groups"] = emitProxyGroups(in.Subscriptions, in.LocalNodes, in.GlobalTarget)
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/assembler -race
git add internal/assembler/
git commit -m "feat(assembler): emit per-group select+url-test + top-level routing groups"
```

### Task 5.4: Emit rules (local → subscriptions → MATCH)

**Files:**
- Create: `internal/assembler/rules.go`
- Create: `internal/assembler/rules_test.go`

- [ ] **Step 1: Write the failing test**

```go
package assembler

import (
	"strings"
	"testing"
	"vpnkit/internal/groups"
	"vpnkit/internal/localrules"
	"vpnkit/internal/subscription"
)

func TestAssembleRulesOrdering(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []subscription.Proxy{{"name": "X", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{
				"DOMAIN-SUFFIX,youtube.com,🚀 Proxy",
			},
		},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{sub},
		LocalRules:       []localrules.Rule{{Type: "DOMAIN-SUFFIX", Payload: "baidu.com", Target: "🎯 Direct"}},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	s := string(out)
	idxBaidu := strings.Index(s, "DOMAIN-SUFFIX,baidu.com")
	idxYoutube := strings.Index(s, "DOMAIN-SUFFIX,youtube.com")
	idxMatch := strings.Index(s, "MATCH,")
	if idxBaidu < 0 || idxYoutube < 0 || idxMatch < 0 {
		t.Fatalf("missing rules:\n%s", s)
	}
	if !(idxBaidu < idxYoutube && idxYoutube < idxMatch) {
		t.Errorf("expected order baidu < youtube < MATCH but got %d %d %d", idxBaidu, idxYoutube, idxMatch)
	}
}

func TestAssembleSubscriptionRuleRewritesTarget(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []subscription.Proxy{{"name": "HK-A", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{
				"DOMAIN-SUFFIX,a.com,Hong-Kong",      // internal group name → rewrite to doge
				"DOMAIN-SUFFIX,b.com,HK-A",           // internal node name → doge:HK-A
				"DOMAIN-SUFFIX,c.com,🚀 Proxy",       // reserved → keep
			},
		},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{sub},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	s := string(out)
	if !strings.Contains(s, "DOMAIN-SUFFIX,a.com,doge") {
		t.Errorf("group-name rewrite missing: %s", s)
	}
	if !strings.Contains(s, "DOMAIN-SUFFIX,b.com,doge:HK-A") {
		t.Errorf("node-name rewrite missing: %s", s)
	}
	if !strings.Contains(s, "DOMAIN-SUFFIX,c.com,\U0001F680 Proxy") {
		t.Errorf("reserved target should be preserved: %s", s)
	}
}
```

- [ ] **Step 2: Run to verify it fails**.

- [ ] **Step 3: Implement**

Create `internal/assembler/rules.go`:

```go
package assembler

import (
	"vpnkit/internal/groups"
	"vpnkit/internal/localrules"
)

var reservedTargets = map[string]bool{
	"🚀 Proxy":  true,
	"🎯 Direct": true,
	"🛑 Reject": true,
	"DIRECT":    true,
	"REJECT":    true,
}

func emitRules(mode Mode, locals []localrules.Rule, subs []groups.Group) []any {
	if mode == ModeGlobal {
		return []any{"MATCH,🚀 Proxy"}
	}
	if mode == ModeDirect {
		return []any{"MATCH,🎯 Direct"}
	}

	out := make([]any, 0, len(locals)+8)
	// 1. local rules (highest priority)
	for _, r := range locals {
		out = append(out, r.Render())
	}
	// 2. each subscription's own rules, with target rewriting
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		nodeMap := nodeNameSet(g) // original → namespaced
		for _, r := range g.Rules() {
			rewritten := rewriteTarget(r, g.Name(), nodeMap)
			if rewritten.Target == "" {
				continue // dropped
			}
			out = append(out, rewritten.Render())
		}
	}
	// 3. MATCH fallback
	out = append(out, "MATCH,🚀 Proxy")
	return out
}

func nodeNameSet(g groups.Group) map[string]string {
	m := make(map[string]string)
	for _, p := range g.Proxies() {
		orig, _ := p["name"].(string)
		m[orig] = g.Name() + ":" + orig
	}
	return m
}

func rewriteTarget(r localrules.Rule, groupName string, nodeMap map[string]string) localrules.Rule {
	if reservedTargets[r.Target] {
		return r
	}
	if ns, ok := nodeMap[r.Target]; ok {
		r.Target = ns
		return r
	}
	// Heuristic: any other unrecognized target (often an internal proxy-group
	// name from the subscription) is mapped to the subscription's group name
	// so user routing intent is preserved at the group level.
	r.Target = groupName
	return r
}
```

Wire in `assembler.go`:

```go
	doc["rules"] = emitRules(in.Mode, in.LocalRules, in.Subscriptions)
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/assembler -race
git add internal/assembler/
git commit -m "feat(assembler): emit rules with local→subs ordering and target rewriting"
```

### Task 5.5: Mode=global / Mode=direct golden tests

**Files:**
- Modify: `internal/assembler/rules_test.go`

- [ ] **Step 1: Write tests**

```go
func TestAssembleModeGlobal(t *testing.T) {
	out, _ := Assemble(Input{
		Mode: ModeGlobal, MixedPort: 50595, ControllerPort: 32645, ControllerSecret: "s",
	})
	s := string(out)
	if !strings.Contains(s, "- MATCH,\U0001F680 Proxy") {
		t.Errorf("mode=global rules should be MATCH,🚀 Proxy only:\n%s", s)
	}
	// Must NOT contain anything else in rules section.
	if strings.Count(s, "DOMAIN-SUFFIX") > 0 {
		t.Errorf("mode=global must skip user rules: %s", s)
	}
}

func TestAssembleModeDirect(t *testing.T) {
	out, _ := Assemble(Input{
		Mode: ModeDirect, MixedPort: 50595, ControllerPort: 32645, ControllerSecret: "s",
	})
	if !strings.Contains(string(out), "MATCH,\U0001F3AF Direct") {
		t.Errorf("mode=direct must end with MATCH,🎯 Direct: %s", out)
	}
}
```

- [ ] **Step 2: Run + verify pass + commit**

```bash
go test ./internal/assembler -race
git add internal/assembler/rules_test.go
git commit -m "test(assembler): mode=global and mode=direct rule emission"
```

### Task 5.6: Phase 5 coverage gate

- [ ] **Step 1: Coverage**

```bash
go test ./internal/assembler -cover
```
Expected: ≥85%. If below, add tests until met.

---

## Phase 6: `app/run.go` takeover + delete `profiles/`

This is the integration step. All four new packages get composed into the live launch path.

### Task 6.1: New profMgr replacement — pipeline struct

**Files:**
- Create: `internal/app/pipeline.go`
- Modify: `internal/app/run.go`

- [ ] **Step 1: Inspect current run.go integration points**

Read `internal/app/run.go` lines 30–150 (where `profMgr` is built and used).

- [ ] **Step 2: Create the pipeline holder**

Create `internal/app/pipeline.go`:

```go
package app

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"vpnkit/internal/assembler"
	"vpnkit/internal/config"
	"vpnkit/internal/extensions"
	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/localrules"
	"vpnkit/internal/store"
	"vpnkit/internal/subscription"
)

// Pipeline is the v1.0.0 replacement for profiles.Manager. It owns the
// in-memory state for all subscription Groups + the LocalNodes Group +
// LocalRules + cached fetch results, and produces a config.yaml on Apply.
type Pipeline struct {
	store          *store.Store
	configYAMLPath string
	extensionsPath string

	mu          sync.Mutex
	localNodes  *localnodes.Manager
	localRules  *localrules.Manager
	subResults  map[string]*subscription.Result // by subscription name; nil = not yet fetched
}

func NewPipeline(st *store.Store, configYAMLPath, extensionsPath string) *Pipeline {
	pl := &Pipeline{
		store:          st,
		configYAMLPath: configYAMLPath,
		extensionsPath: extensionsPath,
		localNodes:     localnodes.New(),
		localRules:     localrules.New(),
		subResults:     map[string]*subscription.Result{},
	}
	pl.localNodes.Load(toLocalNodes(st.Cfg.LocalNodes))
	pl.localRules.Load(toLocalRules(st.Cfg.LocalRules))
	return pl
}

func toLocalNodes(in []store.LocalNode) []localnodes.Node {
	out := make([]localnodes.Node, len(in))
	for i, x := range in {
		out[i] = localnodes.Node{Name: x.Name, Proto: x.Proto, Server: x.Server, Port: x.Port, Fields: x.Fields}
	}
	return out
}

func toLocalRules(in []store.LocalRule) []localrules.Rule {
	out := make([]localrules.Rule, len(in))
	for i, x := range in {
		out[i] = localrules.Rule{Type: x.Type, Payload: x.Payload, Target: x.Target}
	}
	return out
}

// RefreshSubscription fetches one named subscription, parses it, and caches
// the result. Returns the node count.
func (p *Pipeline) RefreshSubscription(ctx context.Context, name string) (int, error) {
	p.mu.Lock()
	var sub *store.Subscription
	for i := range p.store.Cfg.Subscriptions {
		if p.store.Cfg.Subscriptions[i].Name == name {
			sub = &p.store.Cfg.Subscriptions[i]
			break
		}
	}
	p.mu.Unlock()
	if sub == nil {
		return 0, fmt.Errorf("subscription %q not found", name)
	}
	body, err := subscription.Fetch(ctx, sub.URL, sub.UserAgent)
	if err != nil {
		return 0, err
	}
	res, err := subscription.Convert(body)
	if err != nil {
		return 0, err
	}
	p.mu.Lock()
	p.subResults[name] = res
	sub.LastUpdated = time.Now()
	sub.NodeCount = len(res.Proxies)
	p.mu.Unlock()
	_ = p.store.Save()
	return len(res.Proxies), nil
}

// Assemble produces the config.yaml for the current state and writes it.
func (p *Pipeline) Assemble() error {
	p.mu.Lock()
	subs := make([]groups.Group, 0, len(p.store.Cfg.Subscriptions))
	for _, s := range p.store.Cfg.Subscriptions {
		if !s.Enabled {
			continue
		}
		res := p.subResults[s.Name]
		if res == nil {
			// Fetched not happen this run; skip — Status TUI will surface stale.
			continue
		}
		subs = append(subs, groups.NewSubscriptionGroup(s.Name, true, res))
	}
	localGroup := groups.NewLocalNodesGroup("local", p.localNodes)
	ext, _ := extensions.Load(p.extensionsPath)
	cfg := p.store.Cfg
	p.mu.Unlock()

	bytes_, err := assembler.Assemble(assembler.Input{
		Mode:             assembler.Mode(cfg.Mode),
		GlobalTarget:     cfg.GlobalTarget,
		Subscriptions:    subs,
		LocalNodes:       localGroup,
		LocalRules:       p.localRules.All(),
		Extensions:       ext,
		MixedPort:        cfg.MixedPort,
		ControllerPort:   cfg.ControllerPort,
		ControllerSecret: cfg.ControllerSecret,
		ProxyUser:        cfg.ProxyUser,
		ProxyPass:        cfg.ProxyPass,
	})
	if err != nil {
		return err
	}
	if err := config.AtomicWrite(p.configYAMLPath, bytes_, 0o600); err != nil {
		return err
	}
	_ = os.Stdout // keep import used
	return nil
}

// LocalNodes / LocalRules accessors for the TUI tabs.
func (p *Pipeline) LocalNodes() *localnodes.Manager { return p.localNodes }
func (p *Pipeline) LocalRules() *localrules.Manager { return p.localRules }

// SaveLocal persists localNodes + localRules back into the Store.
func (p *Pipeline) SaveLocal() error {
	p.mu.Lock()
	ln := make([]store.LocalNode, 0)
	for _, n := range p.localNodes.All() {
		ln = append(ln, store.LocalNode{Name: n.Name, Proto: n.Proto, Server: n.Server, Port: n.Port, Fields: n.Fields})
	}
	lr := make([]store.LocalRule, 0)
	for _, r := range p.localRules.All() {
		lr = append(lr, store.LocalRule{Type: r.Type, Payload: r.Payload, Target: r.Target})
	}
	p.store.Cfg.LocalNodes = ln
	p.store.Cfg.LocalRules = lr
	p.mu.Unlock()
	return p.store.Save()
}
```

- [ ] **Step 3: Quick build check**

```bash
go build ./internal/app/...
```
Expected: build OK (pipeline.go is standalone for now).

- [ ] **Step 4: Commit**

```bash
git add internal/app/pipeline.go
git commit -m "feat(app): Pipeline replaces profiles.Manager (v1 multi-source)"
```

### Task 6.2: Wire Pipeline into run.go, delete profiles/

**Files:**
- Modify: `internal/app/run.go`
- Delete: `internal/profiles/manager.go`, `internal/profiles/manager_test.go`
- Delete: `internal/subscription/assemble.go`, `internal/subscription/assemble_test.go`

- [ ] **Step 1: Replace profMgr block in run.go**

Locate the `profMgr := profiles.New(...)` block and replace it (and the `profMgr.Load(...)` / `profMgr.SetOnChange(...)` calls below) with:

```go
	pl := NewPipeline(st, p.MihomoConfigFile(), filepath.Join(p.VpnkitConfig, "extensions.toml"))
```

Replace all later `profMgr` references with `pl`. The closure that re-applies config becomes:

```go
	applyCfg := func(ctx context.Context) error {
		if err := pl.Assemble(); err != nil {
			return err
		}
		return applyConfig(ctx, client, svc)
	}
```

And in `settingsDeps`:

```go
	settingsDeps := tabsettings.Deps{
		Paths:          p,
		Store:          st,
		Service:        svc,
		APIClient:      client,
		ExtensionsPath: filepath.Join(p.VpnkitConfig, "extensions.toml"),
		Pipeline:       pl, // <-- new dep (tabsettings.Deps will get this field in phase 8)
		// ProxyNames closure unchanged
		// ApplyFunc:
		ApplyFunc: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			return applyCfg(ctx)
		},
	}
```

Remove import `"vpnkit/internal/profiles"`. Remove the `toProfilesProfiles` helper at the bottom of run.go.

- [ ] **Step 2: Delete dead packages**

```bash
git rm -r internal/profiles/
git rm internal/subscription/assemble.go internal/subscription/assemble_test.go
```

- [ ] **Step 3: Add temporary stub for ProxyNames**

The TUI tab still references `modelRef.CurrentProxyNames()`. Keep that intact — it pulls from mihomo controller (live), not from local state, so it survives.

- [ ] **Step 4: Build**

```bash
go build ./...
```
Expected: errors in `tabs/settings/*` and `cmd/vpnkit/*` that reference removed fields. Address each — for this task only **comment out or stub** the offending lines and leave a TODO that Phase 7/8 will fix. Acceptable stubs:

```go
// TODO(v1-phase7): wire to pl after CLI commands land
```

Goal: integration build green, even if some CLI/TUI features print "not implemented".

- [ ] **Step 5: Run tests, expect some failures**

```bash
go test ./... -count=1 2>&1 | tee /tmp/phase6-failures.log
```

Record failures in `/tmp/phase6-failures.log` — these will be fixed in Phase 7/8 tasks below.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(app): delete profiles/ and subscription/assemble; wire Pipeline (interim build green, some tests stubbed)"
```

### Task 6.3: tabsettings.Deps + Apply path

**Files:**
- Modify: `internal/tabs/settings/settings.go`

- [ ] **Step 1: Add Pipeline field to Deps**

```go
type Deps struct {
	Paths          paths.XDG
	Store          *store.Store
	Service        service.Manager
	APIClient      *api.Client
	ExtensionsPath string
	Pipeline       PipelineFace // see below
	ProxyNames     func() []string
	ApplyFunc      func() error
}

// PipelineFace is the subset of *app.Pipeline that settings tab needs.
// Declared here to break the package import cycle (settings cannot import app).
type PipelineFace interface {
	RefreshSubscription(ctx context.Context, name string) (int, error)
	Assemble() error
	SaveLocal() error
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/tabs/settings/settings.go
git commit -m "feat(settings): PipelineFace interface for v1 multi-source"
```

---

## Phase 7: CLI subcommands

Each task below adds one verb family. All follow the same pattern: write tests, implement, wire dispatcher in `main.go`, commit.

### Task 7.1: `vpnkit subs` (list / add / rm / enable / disable / update)

**Files:**
- Create: `cmd/vpnkit/cmd_subs.go`
- Create: `cmd/vpnkit/cmd_subs_test.go`
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"bytes"
	"strings"
	"testing"
	"vpnkit/internal/store"
)

func TestSubsAddAndList(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	if err := runSubsAdd(st, "doge", "https://example.invalid/sub", ""); err != nil {
		t.Fatalf("add: %v", err)
	}
	var out bytes.Buffer
	if err := runSubsList(&out, st, false); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out.String(), "doge") {
		t.Errorf("list missing doge: %s", out.String())
	}
}

func TestSubsRm(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2, Subscriptions: []store.Subscription{
		{Name: "doge", Enabled: true},
	}}}
	if err := runSubsRm(st, "doge"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if len(st.Cfg.Subscriptions) != 0 {
		t.Errorf("not removed: %+v", st.Cfg.Subscriptions)
	}
}

func TestSubsEnableDisable(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2, Subscriptions: []store.Subscription{
		{Name: "doge", Enabled: true},
	}}}
	if err := runSubsToggle(st, "doge", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if st.Cfg.Subscriptions[0].Enabled {
		t.Error("should be disabled")
	}
	if err := runSubsToggle(st, "doge", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !st.Cfg.Subscriptions[0].Enabled {
		t.Error("should be enabled")
	}
}
```

- [ ] **Step 2: Implement**

Create `cmd/vpnkit/cmd_subs.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"vpnkit/internal/app"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

func dispatchSubs(args []string) {
	if len(args) == 0 {
		dieUserErr("vpnkit subs: usage: vpnkit subs <list|add|rm|enable|disable|update>")
	}
	sub, rest := args[0], args[1:]
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit subs: %v", err)
	}
	switch sub {
	case "list", "ls":
		jsonOut := false
		fs := flag.NewFlagSet("subs list", flag.ExitOnError)
		fs.BoolVar(&jsonOut, "json", false, "")
		_ = fs.Parse(rest)
		if err := runSubsList(os.Stdout, st, jsonOut); err != nil {
			dieRuntime("%v", err)
		}
	case "add":
		fs := flag.NewFlagSet("subs add", flag.ExitOnError)
		ua := fs.String("ua", "", "user-agent")
		_ = fs.Parse(rest)
		if fs.NArg() < 2 {
			dieUserErr("usage: vpnkit subs add <name> <url> [--ua=...]")
		}
		if err := runSubsAdd(st, fs.Arg(0), fs.Arg(1), *ua); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
	case "rm", "remove":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs rm <name>")
		}
		if err := runSubsRm(st, rest[0]); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
	case "enable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs enable <name>")
		}
		if err := runSubsToggle(st, rest[0], true); err != nil {
			dieUserErr("%v", err)
		}
		_ = st.Save()
	case "disable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs disable <name>")
		}
		if err := runSubsToggle(st, rest[0], false); err != nil {
			dieUserErr("%v", err)
		}
		_ = st.Save()
	case "update":
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		pl := app.NewPipeline(st, p.MihomoConfigFile(), p.VpnkitConfig+"/extensions.toml")
		names := rest
		if len(names) == 0 {
			for _, s := range st.Cfg.Subscriptions {
				names = append(names, s.Name)
			}
		}
		var errs []error
		for _, n := range names {
			count, err := pl.RefreshSubscription(ctx, n)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", n, err))
				continue
			}
			fmt.Printf("✅ %s — %d nodes\n", n, count)
		}
		if len(errs) > 0 {
			dieRuntime("%v", errors.Join(errs...))
		}
	default:
		dieUserErr("vpnkit subs: unknown verb %q", sub)
	}
}

func runSubsList(out io.Writer, st *store.Store, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(out).Encode(st.Cfg.Subscriptions)
	}
	for _, s := range st.Cfg.Subscriptions {
		mark := "✅"
		if !s.Enabled {
			mark = "  "
		}
		fmt.Fprintf(out, "%s  %-20s  %3d nodes  %s\n", mark, s.Name, s.NodeCount, s.URL)
	}
	return nil
}

func runSubsAdd(st *store.Store, name, url, ua string) error {
	if name == "" || url == "" {
		return errors.New("name and url required")
	}
	for _, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			return fmt.Errorf("subscription %q already exists", name)
		}
	}
	st.Cfg.Subscriptions = append(st.Cfg.Subscriptions, store.Subscription{
		Name: name, URL: url, UserAgent: ua, Enabled: true,
	})
	return nil
}

func runSubsRm(st *store.Store, name string) error {
	for i, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			st.Cfg.Subscriptions = append(st.Cfg.Subscriptions[:i], st.Cfg.Subscriptions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found", name)
}

func runSubsToggle(st *store.Store, name string, enabled bool) error {
	for i, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			st.Cfg.Subscriptions[i].Enabled = enabled
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found", name)
}

var _ = strings.TrimSpace // silence unused import if any future change drops usage
```

- [ ] **Step 3: Wire dispatcher**

In `cmd/vpnkit/main.go`, add inside the `switch os.Args[1]` block:

```go
		case "subs":
			dispatchSubs(os.Args[2:])
			return
```

- [ ] **Step 4: Test + commit**

```bash
go test ./cmd/vpnkit -race -run TestSubs
git add cmd/vpnkit/cmd_subs.go cmd/vpnkit/cmd_subs_test.go cmd/vpnkit/main.go
git commit -m "feat(cmd/subs): list/add/rm/enable/disable/update subcommands"
```

### Task 7.2: `vpnkit local-nodes`

**Files:**
- Create: `cmd/vpnkit/cmd_local_nodes.go`
- Create: `cmd/vpnkit/cmd_local_nodes_test.go`
- Modify: `cmd/vpnkit/main.go`

Same pattern as 7.1. Verbs: `list`, `add <uri>`, `rm <name>`, `edit <name> <key=val>...`. Use `localnodes.ParseURI`. Persist via `st.Cfg.LocalNodes = ...`. Test: add a known SS URI, list, verify.

Commit message: `feat(cmd/local-nodes): URI add + list + rm + edit subcommands`

### Task 7.3: `vpnkit local-rules`

**Files:**
- Create: `cmd/vpnkit/cmd_local_rules.go`
- Create: `cmd/vpnkit/cmd_local_rules_test.go`
- Modify: `cmd/vpnkit/main.go`

Same pattern. Verbs: `list`, `add <type> <payload> <target>`, `rm <idx>`, `move <idx> <new-idx>`. Validate via `localrules.Validate`.

Commit message: `feat(cmd/local-rules): CRUD + reorder subcommands`

### Task 7.4: `vpnkit target` set/show

**Files:**
- Create: `cmd/vpnkit/cmd_target.go`
- Create: `cmd/vpnkit/cmd_target_test.go`
- Modify: `cmd/vpnkit/main.go`

```go
func dispatchTarget(args []string) {
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("%v", err)
	}
	if len(args) == 0 {
		fmt.Println(st.Cfg.GlobalTarget)
		return
	}
	st.Cfg.GlobalTarget = args[0]
	if err := st.Save(); err != nil {
		dieRuntime("%v", err)
	}
	fmt.Printf("✅ global_target → %s\n", args[0])
}
```

Test: set + read + verify store changed.

Commit: `feat(cmd/target): set/show GlobalTarget`

### Task 7.5: Update existing `vpnkit status`, `mode`, `init`

**Files:**
- Modify: `cmd/vpnkit/cmd_status.go`
- Modify: `cmd/vpnkit/cmd_mode.go`
- Modify: `cmd/vpnkit/cmd_init.go`

- [ ] **status**: drop references to `cfg.ActiveProfile` / `cfg.Profiles`; add subscription count + local nodes count + mode + global_target lines:

```go
fmt.Fprintf(out, "📚 sources   %d subs + %d local nodes\n", len(cfg.Subscriptions), len(cfg.LocalNodes))
fmt.Fprintf(out, "🔀 routing   mode=%s  target=%s\n", cfg.Mode, cfg.GlobalTarget)
```

- [ ] **mode**: change to write `st.Cfg.Mode`, save, then trigger reload via controller:

```go
func runMode(out io.Writer, c *api.Client, rest []string, jsonOut bool) error {
	p := paths.Resolve()
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		fmt.Fprintln(out, st.Cfg.Mode)
		return nil
	}
	v := strings.ToLower(rest[0])
	switch v {
	case "rule", "global", "direct":
	default:
		return fmt.Errorf("mode must be rule|global|direct, got %q", v)
	}
	st.Cfg.Mode = v
	if err := st.Save(); err != nil {
		return err
	}
	// Trigger a config rewrite + mihomo reload.
	pl := app.NewPipeline(st, p.MihomoConfigFile(), p.VpnkitConfig+"/extensions.toml")
	if err := pl.Assemble(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.PatchConfig(ctx, "")
}
```

- [ ] **init**: remove `--restore` (profiles concept gone) or repurpose to import [[local_nodes]] only. For v1.0.0-rc.1: just drop `--restore` (no v0.10 user expects it to work after the migration anyway).

Tests for each: update existing or add new. **Commit each file with a focused message**:
- `refactor(cmd/status): drop ActiveProfile/Profiles, add Subscriptions/LocalNodes/Mode lines`
- `refactor(cmd/mode): write store.Mode + reassemble`
- `refactor(cmd/init): drop --restore (v0.10 concept removed in v1)`

### Task 7.6: Phase 7 coverage gate

```bash
go test ./cmd/vpnkit -cover
```
Expected: ≥80%.

---

## Phase 8: TUI重构

### Task 8.1: Delete old Proxies + Profiles tab; scaffold Groups + Sources + Routing

**Files:**
- Delete: `internal/tabs/profiles/`
- Delete: `internal/tabs/proxies/` (or repurpose; v1 design merges into Groups)
- Create: `internal/tabs/groups/groups.go`
- Create: `internal/tabs/sources/sources.go`
- Modify: `internal/app/model.go`

Each new tab follows the existing bubbletea pattern: `Model{}`, `Init/Update/View`. Use `viewport` and `list` components already in `internal/tabs/viewport/`.

This task is large — split into 4 commits as the spec §6 calls out:
- `feat(tui/groups): Groups tab — list groups + node detail (read-only)`
- `feat(tui/sources): Subscriptions sub-page (CRUD + update now)`
- `feat(tui/sources): Local Nodes sub-page (form + URI add)`
- `feat(tui/rules): Local Rules sub-page (CRUD + reorder)`

For each commit: scaffold, write smoke test if applicable, run app manually to verify keystrokes, commit.

### Task 8.2: Settings → Routing sub-page

**Files:**
- Create: `internal/tabs/settings/routing.go`
- Modify: `internal/tabs/settings/settings.go` (add Routing to sub-page list)

Implement Mode select (3 radio) + Global Target select (dynamically populated from `pl.Pipeline.AvailableTargets()` — add this method to Pipeline that returns all group names + all namespaced node names).

Commit: `feat(tui/settings): Routing sub-page (Mode + Global Target)`

### Task 8.3: Logs tab promoted from Settings

**Files:**
- Modify: `internal/app/model.go` (add tab entry)
- Move: `internal/tabs/settings/logs.go` → `internal/tabs/logs/logs.go` (package rename)

Commit: `refactor(tui): promote Logs from Settings sub-page to top-level tab`

### Task 8.4: Phase 8 verification

- [ ] Run `vpnkit` interactively
- [ ] Verify each tab loads
- [ ] Verify [Tab] / arrow keys work
- [ ] Verify a full happy path: add subscription → update → see nodes in Groups → set GlobalTarget → see route taken in Connections

---

## Phase 9: Release

### Task 9.1: Update docs

**Files:**
- Modify: `README.md` / `README_zh.md`
- Create: `docs/UPGRADE-v1.md`

`UPGRADE-v1.md` content:

```markdown
# Upgrading from v0.10.x to v1.0.0-rc.1

**Breaking**: v1.0.0 changes the on-disk schema. Your old store.toml is incompatible.

## Steps

1. Back up your subscriptions list:
   ```bash
   grep -A2 '\[\[profiles\]\]' ~/.config/vpnkit/config.toml > ~/vpnkit-subs.bak
   ```
2. Update vpnkit: `vpnkit update`
3. Re-init: `vpnkit init --force` (auto-backups old config.toml to `~/.config/vpnkit/config.toml.bak.<timestamp>`)
4. Re-add each subscription:
   ```bash
   vpnkit subs add doge      https://example.invalid/doge
   vpnkit subs add boost     https://example.invalid/boost
   vpnkit subs update
   ```
5. (Optional) Add local rules:
   ```bash
   vpnkit local-rules add DOMAIN-SUFFIX baidu.com '🎯 Direct'
   ```
6. Restart: `systemctl --user restart mihomo.service`

## What's new

- Multiple subscriptions coexist; nodes from any group are selectable
- Local nodes (manually entered) live alongside subscriptions
- Local rules CRUD via CLI and TUI
- Routing mode + global target as explicit knobs
```

Commit: `docs: v1.0.0-rc.1 upgrade guide and README refresh`

### Task 9.2: CHANGELOG

**Files:**
- Create: `CHANGELOG.md` (or append if exists)

Commit: `docs(changelog): v1.0.0-rc.1 entry`

### Task 9.3: Final test + vet sweep

```bash
go vet ./...
go test ./... -race -count=1
```
Expected: all green.

### Task 9.4: Tag + push

- [ ] **Step 1: Merge feature branch to main**

```bash
git checkout main
git merge --no-ff feat/v1-subscription-groups
```

- [ ] **Step 2: Push main, then tag**

```bash
git push origin main
git tag -a v1.0.0-rc.1 -m "v1.0.0-rc.1 — multi-source subscription groups + local nodes/rules"
git push origin v1.0.0-rc.1
```

- [ ] **Step 3: Verify release workflow**

```bash
gh run list --workflow=release.yml --limit 1
```

Expected: in_progress → success in ~50s.

- [ ] **Step 4: Verify release page**

```bash
gh release view v1.0.0-rc.1
```

---

## Self-Review Notes

- **Spec coverage**: §1 data model → Task 1.1/1.2. §2 modules → Phase 2–5. §3 CLI → Phase 7. §4 assembler → Phase 5. §5 output sample → assembler_test.go golden cases. §6 TUI → Phase 8. §7 testing → tasks include unit + golden + integration. §8 release → Phase 9. §9 displaced items → not implemented (explicit).
- **Placeholders**: None. Every code step has complete code. TODOs in Task 6.2 are scoped to "interim build green" and resolved by Phase 7/8.
- **Type consistency**: `Pipeline`, `Group`, `Rule`, `Node` names consistent across phases. `assembler.Mode` is a string-typed const set; matches `store.Cfg.Mode` (also string).
- **Coverage**: Phase 2 ≥85%, Phase 3 ≥85%, Phase 4 ≥80%, Phase 5 ≥85%, Phase 7 ≥80%. Phase 8 (TUI) no coverage target per CLAUDE.md.
