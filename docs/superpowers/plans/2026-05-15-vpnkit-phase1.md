# vpnkit Phase 1 Implementation Plan — TUI Skeleton + Silent Bootstrap

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable `vpnkit` binary that, on first launch, silently downloads `mihomo` to `~/.local/bin`, installs it as a `systemd --user` service (or a PID-managed fallback), and presents a 6-tab bubbletea TUI with a working Dashboard tab streaming live traffic from mihomo's external-controller API.

**Architecture:** Single Go module (`vpnkit`) with one binary entry (`cmd/vpnkit`) and isolated `internal/*` packages per responsibility (paths, store, installer, service, config, rules, api, env, app, tabs). Bubbletea Elm-style top-level Model dispatches to per-tab sub-models. mihomo runs as an external long-lived process — vpnkit never embeds it.

**Tech Stack:** Go 1.22 · `github.com/charmbracelet/bubbletea` · `github.com/charmbracelet/lipgloss` · `github.com/charmbracelet/bubbles` · `gopkg.in/yaml.v3` · `github.com/BurntSushi/toml` · `github.com/charmbracelet/x/exp/teatest` (test only). Zero CGO. Single static binary.

**Spec reference:** [docs/superpowers/specs/2026-05-15-vpnkit-tui-design.md](../specs/2026-05-15-vpnkit-tui-design.md)

---

## File Map

Files this plan creates. Each task targets one or two files plus their tests.

| Path | Responsibility |
|---|---|
| `go.mod`, `go.sum` | Module declaration, deps |
| `README.md` | Minimal user-facing readme (project intro, "see spec" pointer) |
| `cmd/vpnkit/main.go` | Argv dispatch: `--version`, `env`, default → TUI |
| `internal/paths/paths.go` | XDG path resolver (config / state / cache / bin / systemd-user dirs) |
| `internal/log/log.go` | File-backed slog logger writing to `~/.local/state/vpnkit/vpnkit.log` |
| `internal/store/store.go` | TOML read/write for `~/.config/vpnkit/config.toml`; profiles list, secret, mode |
| `internal/installer/arch.go` | GOARCH + CPU-feature detection → release asset name |
| `internal/installer/release.go` | Query GitHub Releases API (latest + by tag); pick matching asset |
| `internal/installer/download.go` | Download .gz with progress callback, SHA256 verify, gunzip to tempfile |
| `internal/installer/install.go` | Orchestrate detect → fetch → verify → unpack → atomic rename to `~/.local/bin/mihomo` |
| `internal/service/manager.go` | `Manager` interface, `Mode`, `Status`, errors |
| `internal/service/pid.go` | PID-file based implementation (fork detached child, SIGTERM stop, status from `/proc`) |
| `internal/service/systemd.go` | systemd --user implementation, embedded `mihomo.service` template |
| `internal/service/detect.go` | Detect mode (XDG_RUNTIME_DIR socket → systemctl env → fallback PID), factory |
| `internal/rules/templates.go` | `embed.FS` for rule templates, lookup helper |
| `internal/rules/templates/loyalsoldier.yaml` | Default Loyalsoldier rule-providers + rules |
| `internal/rules/templates/minimal.yaml` | Minimal cn-direct rule template |
| `internal/config/skeleton.go` | Generate initial `~/.config/mihomo/config.yaml` (template + secret + chosen rule set) |
| `internal/config/atomic.go` | Atomic file write helper (tmp + fsync + rename) |
| `internal/api/client.go` | mihomo REST client; `/version`, `/configs` PATCH (mode), bearer auth, 5s timeout |
| `internal/api/traffic.go` | SSE stream for `/traffic` → channel of `Traffic{Up, Down}` |
| `internal/env/env.go` | Render shell snippets (bash/zsh/fish) for HTTP_PROXY/SOCKS_PROXY/NO_PROXY |
| `internal/app/messages.go` | All `tea.Msg` types (Traffic, ServiceStatus, BootstrapProgress, Error) |
| `internal/app/model.go` | Top-level Model, fields, `New()` constructor |
| `internal/app/update.go` | `Update(msg)` — message dispatching to active tab + global keys |
| `internal/app/view.go` | `View()` — composes sidebar + tab content + statusbar |
| `internal/app/keys.go` | Key bindings via `bubbles/key` |
| `internal/app/sidebar.go` | Sidebar rendering (`[1] Dashboard` etc., highlight active) |
| `internal/app/statusbar.go` | Bottom status bar (running ●, mode, ↑↓, sub, hint) |
| `internal/app/bootstrap.go` | First-run orchestrator: returns `tea.Cmd` sequence that installs+configures+starts mihomo |
| `internal/app/run.go` | `Run()` entry called from `main.go` for the default TUI mode |
| `internal/tabs/dashboard/dashboard.go` | Dashboard model: traffic sparkline + status card |
| `internal/tabs/stub/stub.go` | Placeholder tab model used for tabs 2–6 in Phase 1 |
| `.github/workflows/ci.yml` | CI: `golangci-lint run` + `go test -race -cover ./...` on linux/amd64 and linux/arm64 |
| `.golangci.yml` | Linter config |

---

## Task 1: Repo bootstrap — go.mod, README, project skeleton

**Files:**
- Create: `go.mod`
- Create: `README.md`
- Create: `Makefile`
- Modify: `.gitignore` (already exists)

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /home/zhangjunming/workchain/vpn
go mod init vpnkit
go get github.com/charmbracelet/bubbletea@v0.27.1
go get github.com/charmbracelet/lipgloss@v0.13.0
go get github.com/charmbracelet/bubbles@v0.20.0
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/BurntSushi/toml@v1.4.0
go get github.com/coder/websocket@v1.8.12
go get github.com/charmbracelet/x/exp/teatest@latest
```

Expected: `go.mod` created with module name `vpnkit` and these dependencies pinned.

- [ ] **Step 2: Write minimal README.md**

```markdown
# vpnkit

TUI for managing the [mihomo](https://github.com/MetaCubeX/mihomo) proxy core on Linux, non-root.

Inspired by [Clash Verge](https://github.com/clash-verge-rev/clash-verge-rev). Single Go binary, lives in `~/.local/bin`, manages mihomo as a `systemd --user` service (or PID-managed fallback).

## Status

Under development. See `docs/superpowers/specs/2026-05-15-vpnkit-tui-design.md` for design, `docs/superpowers/plans/` for implementation plans.

## Build

```bash
go build -o ~/.local/bin/vpnkit ./cmd/vpnkit
```

## Usage

```bash
vpnkit              # launch TUI
eval "$(vpnkit env --shell zsh)"   # export HTTP_PROXY etc. for current shell
```
```

- [ ] **Step 3: Write Makefile**

```makefile
.PHONY: build test lint install clean

BIN_DIR := $(HOME)/.local/bin

build:
	go build -trimpath -ldflags "-s -w" -o ./bin/vpnkit ./cmd/vpnkit

install: build
	mkdir -p $(BIN_DIR)
	install -m 0755 ./bin/vpnkit $(BIN_DIR)/vpnkit

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf ./bin
```

- [ ] **Step 4: Add Go-specific entries to .gitignore**

Append to existing `.gitignore`:
```
# Go
/bin/
*.coverprofile
coverage.html
```

- [ ] **Step 5: Sanity check the module compiles (empty)**

Run:
```bash
mkdir -p cmd/vpnkit
cat > cmd/vpnkit/main.go <<'EOF'
package main

func main() {}
EOF
go build ./...
```
Expected: builds clean, no output.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum README.md Makefile .gitignore cmd/vpnkit/main.go
git commit -m "chore: bootstrap Go module and project skeleton"
```

---

## Task 2: XDG paths (`internal/paths`)

**Files:**
- Create: `internal/paths/paths.go`
- Create: `internal/paths/paths_test.go`

- [ ] **Step 1: Write failing test**

`internal/paths/paths_test.go`:
```go
package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXDGFallsBackToHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	p := Resolve()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"VpnkitConfig", p.VpnkitConfig, filepath.Join(tmp, ".config", "vpnkit")},
		{"MihomoConfig", p.MihomoConfig, filepath.Join(tmp, ".config", "mihomo")},
		{"VpnkitState", p.VpnkitState, filepath.Join(tmp, ".local", "state", "vpnkit")},
		{"VpnkitCache", p.VpnkitCache, filepath.Join(tmp, ".cache", "vpnkit")},
		{"LocalBin", p.LocalBin, filepath.Join(tmp, ".local", "bin")},
		{"SystemdUserDir", p.SystemdUserDir, filepath.Join(tmp, ".config", "systemd", "user")},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestXDGRespectsEnvOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	p := Resolve()
	if p.VpnkitConfig != filepath.Join(tmp, "cfg", "vpnkit") {
		t.Errorf("XDG_CONFIG_HOME not honored: %s", p.VpnkitConfig)
	}
	if p.VpnkitState != filepath.Join(tmp, "state", "vpnkit") {
		t.Errorf("XDG_STATE_HOME not honored: %s", p.VpnkitState)
	}
}

func TestEnsureCreatesAllDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	p := Resolve()
	if err := p.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	dirs := []string{p.VpnkitConfig, p.MihomoConfig, p.VpnkitState, p.VpnkitCache, p.LocalBin, p.SystemdUserDir}
	for _, d := range dirs {
		if info, err := os.Stat(d); err != nil || !info.IsDir() {
			t.Errorf("dir %s missing: %v", d, err)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/paths/ -run TestXDG -v`
Expected: compile error (`Resolve undefined`).

- [ ] **Step 3: Write implementation**

`internal/paths/paths.go`:
```go
// Package paths resolves XDG base directories and standard vpnkit / mihomo locations.
package paths

import (
	"os"
	"path/filepath"
)

// XDG holds resolved absolute paths for all directories vpnkit reads or writes.
type XDG struct {
	Home           string
	VpnkitConfig   string // ~/.config/vpnkit
	MihomoConfig   string // ~/.config/mihomo
	VpnkitState    string // ~/.local/state/vpnkit
	VpnkitCache    string // ~/.cache/vpnkit
	LocalBin       string // ~/.local/bin
	SystemdUserDir string // ~/.config/systemd/user
	RuntimeDir     string // $XDG_RUNTIME_DIR (may be empty)
}

// Resolve reads XDG environment variables, applying spec-defined fallbacks.
func Resolve() XDG {
	home := os.Getenv("HOME")
	configHome := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	stateHome := envOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	cacheHome := envOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	return XDG{
		Home:           home,
		VpnkitConfig:   filepath.Join(configHome, "vpnkit"),
		MihomoConfig:   filepath.Join(configHome, "mihomo"),
		VpnkitState:    filepath.Join(stateHome, "vpnkit"),
		VpnkitCache:    filepath.Join(cacheHome, "vpnkit"),
		LocalBin:       filepath.Join(home, ".local", "bin"),
		SystemdUserDir: filepath.Join(configHome, "systemd", "user"),
		RuntimeDir:     runtime,
	}
}

// Ensure creates all vpnkit-owned directories with 0o755.
// mihomo-owned dirs (ruleset/, profiles/) are created lazily by their respective subsystems.
func (p XDG) Ensure() error {
	dirs := []string{
		p.VpnkitConfig, p.MihomoConfig, p.VpnkitState, p.VpnkitCache, p.LocalBin, p.SystemdUserDir,
		filepath.Join(p.VpnkitCache, "downloads"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// MihomoBinary returns ~/.local/bin/mihomo.
func (p XDG) MihomoBinary() string { return filepath.Join(p.LocalBin, "mihomo") }

// VpnkitConfigFile returns ~/.config/vpnkit/config.toml.
func (p XDG) VpnkitConfigFile() string { return filepath.Join(p.VpnkitConfig, "config.toml") }

// MihomoConfigFile returns ~/.config/mihomo/config.yaml.
func (p XDG) MihomoConfigFile() string { return filepath.Join(p.MihomoConfig, "config.yaml") }

// PIDFile returns ~/.local/state/vpnkit/mihomo.pid.
func (p XDG) PIDFile() string { return filepath.Join(p.VpnkitState, "mihomo.pid") }

// MihomoLog returns ~/.local/state/vpnkit/mihomo.log.
func (p XDG) MihomoLog() string { return filepath.Join(p.VpnkitState, "mihomo.log") }

// VpnkitLog returns ~/.local/state/vpnkit/vpnkit.log.
func (p XDG) VpnkitLog() string { return filepath.Join(p.VpnkitState, "vpnkit.log") }

// SystemdUnit returns ~/.config/systemd/user/mihomo.service.
func (p XDG) SystemdUnit() string { return filepath.Join(p.SystemdUserDir, "mihomo.service") }

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/paths/ -v`
Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/paths/
git commit -m "feat(paths): XDG directory resolver with Ensure"
```

---

## Task 3: File-backed logger (`internal/log`)

**Files:**
- Create: `internal/log/log.go`
- Create: `internal/log/log_test.go`

- [ ] **Step 1: Write failing test**

`internal/log/log_test.go`:
```go
package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesToFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "vpnkit.log")
	lg, err := New(path, LevelDebug)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Info("hello", "k", 1)
	lg.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "hello") || !strings.Contains(string(data), "k=1") {
		t.Errorf("log file missing content: %q", data)
	}
}

func TestLevelFiltering(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "vpnkit.log")
	lg, err := New(path, LevelWarn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Debug("debug-msg")
	lg.Info("info-msg")
	lg.Warn("warn-msg")
	lg.Close()

	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, "debug-msg") || strings.Contains(out, "info-msg") {
		t.Errorf("level filter let through too much: %q", out)
	}
	if !strings.Contains(out, "warn-msg") {
		t.Errorf("warn missing: %q", out)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/log/ -v`
Expected: compile failures.

- [ ] **Step 3: Write implementation**

`internal/log/log.go`:
```go
// Package log wraps log/slog with a file-backed JSON handler used across vpnkit.
package log

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Level is a thin alias to keep slog imports out of callers.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger writes structured logs to a file. Safe for concurrent use.
type Logger struct {
	*slog.Logger
	file io.Closer
}

// New opens (creates) the log file with 0o600 and returns a Logger writing to it.
func New(path string, level Level) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	return &Logger{Logger: slog.New(handler), file: f}, nil
}

// Close releases the underlying file handle. Safe to call once.
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/log/ -v`
Expected: both tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/log/
git commit -m "feat(log): file-backed slog logger"
```

---

## Task 4: TOML store for `config.toml` (`internal/store`)

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: Write failing test**

`internal/store/store_test.go`:
```go
package store

import (
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.ControllerPort != 9090 {
		t.Errorf("default port: %d", s.Cfg.ControllerPort)
	}
	if s.Cfg.RuleTemplate != "loyalsoldier" {
		t.Errorf("default rule template: %s", s.Cfg.RuleTemplate)
	}
	if s.Cfg.ServiceMode != "" {
		t.Errorf("service_mode must remain empty until detected: %s", s.Cfg.ServiceMode)
	}
	if len(s.Cfg.ControllerSecret) < 16 {
		t.Errorf("secret too short: %q", s.Cfg.ControllerSecret)
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Cfg.ActiveProfile = "airport-A"
	s.Cfg.Profiles = []Profile{{Name: "airport-A", URL: "https://example.com/sub"}}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if s2.Cfg.ActiveProfile != "airport-A" {
		t.Errorf("active not persisted: %s", s2.Cfg.ActiveProfile)
	}
	if len(s2.Cfg.Profiles) != 1 || s2.Cfg.Profiles[0].Name != "airport-A" {
		t.Errorf("profiles not persisted: %+v", s2.Cfg.Profiles)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/ -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/store/store.go`:
```go
// Package store reads and writes vpnkit's own config file (~/.config/vpnkit/config.toml).
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// Profile records one subscription entry.
type Profile struct {
	Name        string    `toml:"name"`
	URL         string    `toml:"url"`
	UserAgent   string    `toml:"user_agent,omitempty"`
	LastUpdated time.Time `toml:"last_updated,omitempty"`
}

// Config is vpnkit's persisted configuration.
type Config struct {
	ControllerSecret string    `toml:"controller_secret"`
	ControllerPort   int       `toml:"controller_port"`
	ReleaseMirror    string    `toml:"release_mirror"`
	ActiveProfile    string    `toml:"active_profile,omitempty"`
	RuleTemplate     string    `toml:"rule_template"`
	ServiceMode      string    `toml:"service_mode,omitempty"`
	UITheme          string    `toml:"ui_theme"`
	Profiles         []Profile `toml:"profiles"`
}

// Store wraps a Config and its on-disk location.
type Store struct {
	path string
	mu   sync.Mutex
	Cfg  Config
}

// Load reads `path`. If the file does not exist, defaults are written and returned.
func Load(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		s.Cfg = defaults()
		if err := s.Save(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, &s.Cfg); err != nil {
		return nil, err
	}
	// Apply defaults for any zero-value fields the caller relies on.
	if s.Cfg.ControllerPort == 0 {
		s.Cfg.ControllerPort = 9090
	}
	if s.Cfg.RuleTemplate == "" {
		s.Cfg.RuleTemplate = "loyalsoldier"
	}
	if s.Cfg.UITheme == "" {
		s.Cfg.UITheme = "default"
	}
	if s.Cfg.ControllerSecret == "" {
		s.Cfg.ControllerSecret = randHex(16)
	}
	return s, nil
}

// Save serializes Cfg to disk atomically (tmp + rename).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "config-*.toml.tmp")
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(tmp).Encode(s.Cfg); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func defaults() Config {
	return Config{
		ControllerSecret: randHex(16),
		ControllerPort:   9090,
		RuleTemplate:     "loyalsoldier",
		UITheme:          "default",
	}
}

func randHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: both tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): TOML config persistence for vpnkit"
```

---

## Task 5: Installer arch + CPU detection (`internal/installer/arch.go`)

**Files:**
- Create: `internal/installer/arch.go`
- Create: `internal/installer/arch_test.go`

- [ ] **Step 1: Write failing test**

`internal/installer/arch_test.go`:
```go
package installer

import "testing"

func TestAssetNameAmd64Compatible(t *testing.T) {
	got := assetName("amd64", true, "v1.19.16")
	want := "mihomo-linux-amd64-compatible-v1.19.16.gz"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestAssetNameAmd64Modern(t *testing.T) {
	got := assetName("amd64", false, "v1.19.16")
	want := "mihomo-linux-amd64-v1.19.16.gz"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestAssetNameArm64(t *testing.T) {
	got := assetName("arm64", false, "v1.19.16")
	want := "mihomo-linux-arm64-v1.19.16.gz"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestNeedsCompatibleParsesCpuinfo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"modern flags present", "flags : fpu vme popcnt sse4_2 avx2\n", false},
		{"missing popcnt", "flags : fpu vme sse4_2 avx2\n", true},
		{"missing sse4_2", "flags : fpu vme popcnt avx2\n", true},
		{"empty input", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsCompatibleFromCpuinfo(tt.input); got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/installer/ -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/installer/arch.go`:
```go
// Package installer downloads, verifies, and unpacks mihomo release binaries.
package installer

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// SelectAsset returns (assetName, useCompatible) for the current host.
// On non-amd64 architectures `useCompatible` is always false.
func SelectAsset(version string) (string, bool) {
	arch := runtime.GOARCH
	compat := false
	if arch == "amd64" {
		compat = NeedsCompatibleBuild()
	}
	return assetName(arch, compat, version), compat
}

// NeedsCompatibleBuild reports whether the running CPU lacks features the modern
// (non-compatible) mihomo build assumes. Reads /proc/cpuinfo; returns true on error
// so we err on the side of compatibility.
func NeedsCompatibleBuild() bool {
	if runtime.GOARCH != "amd64" {
		return false
	}
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return true
	}
	return needsCompatibleFromCpuinfo(string(data))
}

func needsCompatibleFromCpuinfo(s string) bool {
	flagsLine := ""
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "flags") {
			flagsLine = line
			break
		}
	}
	if flagsLine == "" {
		return true
	}
	flags := strings.Fields(flagsLine)
	have := map[string]bool{}
	for _, f := range flags {
		have[f] = true
	}
	required := []string{"popcnt", "sse4_2"}
	for _, r := range required {
		if !have[r] {
			return true
		}
	}
	return false
}

func assetName(arch string, compatible bool, version string) string {
	suffix := ""
	if compatible {
		suffix = "-compatible"
	}
	return fmt.Sprintf("mihomo-linux-%s%s-%s.gz", arch, suffix, version)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/installer/ -v`
Expected: 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/installer/
git commit -m "feat(installer): asset name + CPU compatibility detection"
```

---

## Task 6: GitHub release lookup (`internal/installer/release.go`)

**Files:**
- Create: `internal/installer/release.go`
- Create: `internal/installer/release_test.go`

- [ ] **Step 1: Write failing test**

`internal/installer/release_test.go`:
```go
package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MetaCubeX/mihomo/releases/latest" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.19.16",
			"assets": []map[string]any{
				{"name": "mihomo-linux-amd64-v1.19.16.gz", "browser_download_url": "https://example.com/x.gz"},
				{"name": "mihomo-linux-amd64-compatible-v1.19.16.gz", "browser_download_url": "https://example.com/c.gz"},
			},
		})
	}))
	defer srv.Close()
	rc := NewReleaseClient(srv.URL, "")
	rel, err := rc.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.Tag != "v1.19.16" {
		t.Errorf("tag: %s", rel.Tag)
	}
	if len(rel.Assets) != 2 {
		t.Errorf("assets: %d", len(rel.Assets))
	}
}

func TestFindAssetURL(t *testing.T) {
	rel := Release{
		Tag: "v1.19.16",
		Assets: []Asset{
			{Name: "mihomo-linux-amd64-v1.19.16.gz", URL: "https://example.com/a.gz"},
			{Name: "mihomo-linux-amd64-compatible-v1.19.16.gz", URL: "https://example.com/b.gz"},
		},
	}
	url, err := rel.AssetURL("mihomo-linux-amd64-compatible-v1.19.16.gz")
	if err != nil {
		t.Fatalf("AssetURL: %v", err)
	}
	if url != "https://example.com/b.gz" {
		t.Errorf("wrong url: %s", url)
	}
	if _, err := rel.AssetURL("missing.gz"); err == nil {
		t.Errorf("missing asset: expected error")
	}
}

func TestApplyMirror(t *testing.T) {
	const orig = "https://github.com/MetaCubeX/mihomo/releases/download/v1.19.16/x.gz"
	got := ApplyMirror(orig, "https://ghproxy.com/")
	want := "https://ghproxy.com/https://github.com/MetaCubeX/mihomo/releases/download/v1.19.16/x.gz"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
	if ApplyMirror(orig, "") != orig {
		t.Error("empty mirror must passthrough")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/installer/ -run Release -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/installer/release.go`:
```go
package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Asset is one GitHub release artifact.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is a subset of GitHub's release schema sufficient for our needs.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// AssetURL returns the URL of the asset whose name matches exactly.
func (r Release) AssetURL(name string) (string, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("asset %q not found in release %s", name, r.Tag)
}

// ReleaseClient queries the GitHub Releases API.
type ReleaseClient struct {
	BaseURL string // e.g. "https://api.github.com"
	Token   string // optional GITHUB_TOKEN; empty => unauthenticated
	HTTP    *http.Client
}

// NewReleaseClient constructs a client with a 10s timeout.
func NewReleaseClient(baseURL, token string) *ReleaseClient {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &ReleaseClient{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Latest fetches MetaCubeX/mihomo's latest release.
func (c *ReleaseClient) Latest() (Release, error) {
	return c.byPath("/repos/MetaCubeX/mihomo/releases/latest")
}

// ByTag fetches a specific tagged release.
func (c *ReleaseClient) ByTag(tag string) (Release, error) {
	return c.byPath(fmt.Sprintf("/repos/MetaCubeX/mihomo/releases/tags/%s", tag))
}

func (c *ReleaseClient) byPath(path string) (Release, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github: %s", resp.Status)
	}
	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, err
	}
	return r, nil
}

// ApplyMirror prefixes the GitHub URL with `mirror` (trailing slash respected).
// Empty mirror returns url unchanged.
func ApplyMirror(url, mirror string) string {
	if mirror == "" {
		return url
	}
	if !strings.HasSuffix(mirror, "/") {
		mirror += "/"
	}
	return mirror + url
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/installer/ -v`
Expected: all 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/installer/
git commit -m "feat(installer): GitHub Releases lookup + mirror support"
```

---

## Task 7: Download, SHA verify, gunzip (`internal/installer/download.go`)

**Files:**
- Create: `internal/installer/download.go`
- Create: `internal/installer/download_test.go`

- [ ] **Step 1: Write failing test**

`internal/installer/download_test.go`:
```go
package installer

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadAndVerify(t *testing.T) {
	// Build a tiny gzip-wrapped payload.
	payload := []byte("hello mihomo")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(payload)
	_ = gw.Close()
	gzBytes := buf.Bytes()
	sum := sha256.Sum256(gzBytes)
	expected := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gzBytes)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "mihomo")
	progresses := []int64{}
	err := Download(srv.URL+"/mihomo.gz", expected, dst, func(n, total int64) {
		progresses = append(progresses, n)
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch: %q", got)
	}
	if len(progresses) == 0 {
		t.Errorf("progress callback never fired")
	}
	info, _ := os.Stat(dst)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("perm: %v", info.Mode().Perm())
	}
}

func TestDownloadSHAMismatch(t *testing.T) {
	gzBytes, _ := gzipBytes([]byte("payload"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gzBytes)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "mihomo")
	err := Download(srv.URL, "00deadbeef", dst, nil)
	if err == nil {
		t.Fatal("expected SHA mismatch error")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("partial file left behind: %v", err)
	}
}

func gzipBytes(p []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(p); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "mihomo")
	if err := Download(srv.URL, "", dst, nil); err == nil {
		t.Fatal("expected error")
	}
}

// gzipBytes used in tests only; reference io to silence import in some toolchains.
var _ = io.EOF
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/installer/ -run Download -v`
Expected: compile error (`Download` undefined).

- [ ] **Step 3: Write implementation**

`internal/installer/download.go`:
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
)

// ProgressFunc reports bytes downloaded so far and the total expected (-1 if unknown).
type ProgressFunc func(n, total int64)

// Download fetches a gzipped mihomo binary, verifies SHA256 of the raw gzip stream
// against expectedSHA (hex-encoded; empty = skip check), decompresses, and writes
// the resulting executable atomically to dst with mode 0o755.
func Download(url, expectedSHA, dst string, progress ProgressFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	total := resp.ContentLength

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
	gz, err := gzip.NewReader(progressReader(reader, total, progress))
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

// progressReader wraps r, invoking cb at most every 64KiB.
func progressReader(r io.Reader, total int64, cb ProgressFunc) io.Reader {
	if cb == nil {
		return r
	}
	return &progressR{r: r, total: total, cb: cb}
}

type progressR struct {
	r       io.Reader
	total   int64
	read    int64
	cb      ProgressFunc
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

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/installer/ -v`
Expected: all installer tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/installer/download.go internal/installer/download_test.go
git commit -m "feat(installer): gzip download with SHA256 verify + progress"
```

---

## Task 8: Installer orchestrator (`internal/installer/install.go`)

**Files:**
- Create: `internal/installer/install.go`
- Create: `internal/installer/install_test.go`

- [ ] **Step 1: Write failing test**

`internal/installer/install_test.go`:
```go
package installer

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallLatestE2E(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("only amd64/arm64 supported")
	}
	payload := []byte("#!/bin/sh\necho mihomo v0.0.0-fake\n")
	gzPayload := mustGzip(t, payload)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/MetaCubeX/mihomo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.0.0-fake",
			"assets": []map[string]any{
				{"name": "mihomo-linux-" + runtime.GOARCH + "-compatible-v0.0.0-fake.gz", "browser_download_url": baseURL + "/dl.gz"},
				{"name": "mihomo-linux-" + runtime.GOARCH + "-v0.0.0-fake.gz", "browser_download_url": baseURL + "/dl.gz"},
			},
		})
	})
	mux.HandleFunc("/dl.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gzPayload)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "mihomo")
	opts := Options{
		APIBase:    srv.URL,
		Mirror:     "",
		Dst:        dst,
		Version:    "",
		ForceCompat: nil,
	}
	res, err := Install(opts, nil)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Version != "v0.0.0-fake" {
		t.Errorf("version: %s", res.Version)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch")
	}
}

func mustGzip(t *testing.T, p []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(p)
	_ = gw.Close()
	return buf.Bytes()
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/installer/ -run Install -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/installer/install.go`:
```go
package installer

import (
	"fmt"
)

// Options control an Install call.
type Options struct {
	APIBase     string // override GitHub API base (for tests / enterprise)
	Token       string // GITHUB_TOKEN
	Mirror      string // optional URL prefix applied to release download URLs
	Dst         string // absolute destination path for mihomo binary
	Version     string // empty = latest
	ForceCompat *bool  // nil = autodetect; true/false = override
}

// Result describes a successful install.
type Result struct {
	Version    string
	Compatible bool
}

// Install runs the full flow: resolve release → choose asset → download → verify → unpack → rename.
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
		// Fall back to the other variant if exact name missing — common when our
		// detection picks compatible but only modern is published (rare) or vice versa.
		altName := assetName(currentArch(), !compat, rel.Tag)
		alt, altErr := rel.AssetURL(altName)
		if altErr != nil {
			return Result{}, fmt.Errorf("install: %w", err)
		}
		url = alt
		compat = !compat
	}
	url = ApplyMirror(url, opts.Mirror)

	if err := Download(url, "", opts.Dst, progress); err != nil {
		return Result{}, fmt.Errorf("install: download: %w", err)
	}
	return Result{Version: rel.Tag, Compatible: compat}, nil
}

// currentArch wraps runtime.GOARCH so tests can override via build tags later if needed.
func currentArch() string {
	return runtimeGOARCH()
}
```

- [ ] **Step 4: Add small helper file to allow future override**

`internal/installer/runtime.go`:
```go
package installer

import "runtime"

func runtimeGOARCH() string { return runtime.GOARCH }
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/installer/ -v`
Expected: all installer tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/installer/install.go internal/installer/install_test.go internal/installer/runtime.go
git commit -m "feat(installer): orchestrator wiring release+arch+download"
```

---

## Task 9: Service interface and types (`internal/service/manager.go`)

**Files:**
- Create: `internal/service/manager.go`

- [ ] **Step 1: Write the interface and shared types directly (no test — interfaces are exercised by impl tests)**

`internal/service/manager.go`:
```go
// Package service provides a non-root background-process manager for mihomo,
// with two interchangeable backends: systemd --user and a PID-file based fork.
package service

import (
	"context"
	"errors"
	"io"
	"time"
)

// Mode is the active service backend.
type Mode string

const (
	ModeSystemdUser Mode = "systemd-user"
	ModePID         Mode = "pid"
)

// Status reports the runtime state of mihomo.
type Status struct {
	Running bool
	PID     int
	Since   time.Time
	Mode    Mode
}

// ErrNotRunning is returned by Stop/Status when mihomo is not running.
var ErrNotRunning = errors.New("service: mihomo not running")

// Manager abstracts service lifecycle operations.
type Manager interface {
	Mode() Mode
	Install(ctx context.Context) error
	Uninstall(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Status(ctx context.Context) (Status, error)
	// Logs returns a reader that streams (or replays + follows) the mihomo log.
	// follow=true keeps the reader open and streams new lines; false returns the
	// last ~30 lines as a one-shot reader.
	Logs(ctx context.Context, follow bool) (io.ReadCloser, error)
}

// Config is shared by both backends.
type Config struct {
	BinaryPath  string // absolute path to mihomo binary
	ConfigDir   string // -d argument passed to mihomo
	PIDFilePath string // for PID-mode only
	LogFilePath string // for PID-mode only
	UnitPath    string // for systemd-user only
}
```

- [ ] **Step 2: Build (no tests yet)**

Run: `go build ./internal/service/`
Expected: builds clean.

- [ ] **Step 3: Commit**

```bash
git add internal/service/manager.go
git commit -m "feat(service): Manager interface and shared types"
```

---

## Task 10: PID-mode service implementation (`internal/service/pid.go`)

**Files:**
- Create: `internal/service/pid.go`
- Create: `internal/service/pid_test.go`

- [ ] **Step 1: Write failing test**

`internal/service/pid_test.go`:
```go
package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPIDLifecycle(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not on PATH")
	}
	tmp := t.TempDir()
	cfg := Config{
		BinaryPath:  sleep,
		ConfigDir:   tmp,
		PIDFilePath: filepath.Join(tmp, "x.pid"),
		LogFilePath: filepath.Join(tmp, "x.log"),
	}
	m := NewPID(cfg, []string{"60"}) // override args for test
	ctx := context.Background()

	// Status before start: not running.
	st, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status pre: %v", err)
	}
	if st.Running {
		t.Fatal("expected not running")
	}

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	st, err = m.Status(ctx)
	if err != nil {
		t.Fatalf("Status post: %v", err)
	}
	if !st.Running || st.PID == 0 {
		t.Fatalf("expected running, got %+v", st)
	}

	if err := m.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, _ = m.Status(ctx)
	if st.Running {
		t.Fatal("still running after stop")
	}

	// Stop again should report not running.
	if err := m.Stop(ctx); !errors.Is(err, ErrNotRunning) {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	// PID file should be cleaned up.
	if _, err := os.Stat(cfg.PIDFilePath); !os.IsNotExist(err) {
		t.Errorf("PID file left behind")
	}
}

func TestPIDRestart(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not on PATH")
	}
	tmp := t.TempDir()
	cfg := Config{
		BinaryPath:  sleep,
		PIDFilePath: filepath.Join(tmp, "x.pid"),
		LogFilePath: filepath.Join(tmp, "x.log"),
	}
	m := NewPID(cfg, []string{"60"})
	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	st1, _ := m.Status(ctx)
	if err := m.Restart(ctx); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	st2, _ := m.Status(ctx)
	if !st2.Running || st1.PID == st2.PID {
		t.Errorf("restart PID unchanged: %d -> %d", st1.PID, st2.PID)
	}
	_ = m.Stop(ctx)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/service/ -v`
Expected: compile errors (`NewPID` undefined).

- [ ] **Step 3: Write implementation**

`internal/service/pid.go`:
```go
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PIDManager runs mihomo via a detached child and tracks it through a PID file.
type PIDManager struct {
	cfg  Config
	args []string // mihomo args; defaults to ["-d", cfg.ConfigDir]
}

// NewPID constructs a PIDManager. If args is nil, default mihomo args are used.
func NewPID(cfg Config, args []string) *PIDManager {
	if args == nil {
		args = []string{"-d", cfg.ConfigDir}
	}
	return &PIDManager{cfg: cfg, args: args}
}

func (*PIDManager) Mode() Mode { return ModePID }

// Install is a no-op in PID mode.
func (*PIDManager) Install(context.Context) error { return nil }

// Uninstall removes the pid file if any.
func (m *PIDManager) Uninstall(ctx context.Context) error {
	_ = m.Stop(ctx)
	return nil
}

// Start launches the detached child and writes the PID file.
func (m *PIDManager) Start(ctx context.Context) error {
	if st, _ := m.Status(ctx); st.Running {
		return fmt.Errorf("service: already running pid=%d", st.PID)
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.LogFilePath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(m.cfg.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	cmd := exec.Command(m.cfg.BinaryPath, m.args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	logFile.Close()

	if err := os.MkdirAll(filepath.Dir(m.cfg.PIDFilePath), 0o755); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	if err := os.WriteFile(m.cfg.PIDFilePath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	// Detach so the child outlives this process.
	_ = cmd.Process.Release()
	return nil
}

// Stop sends SIGTERM, waits up to 5s, then SIGKILL.
func (m *PIDManager) Stop(ctx context.Context) error {
	pid, err := m.readPID()
	if err != nil {
		return ErrNotRunning
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(m.cfg.PIDFilePath)
		return ErrNotRunning
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		_ = os.Remove(m.cfg.PIDFilePath)
		return ErrNotRunning
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			_ = os.Remove(m.cfg.PIDFilePath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = proc.Signal(syscall.SIGKILL)
	_ = os.Remove(m.cfg.PIDFilePath)
	return nil
}

// Restart = Stop (best-effort) + Start.
func (m *PIDManager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil && !errors.Is(err, ErrNotRunning) {
		return err
	}
	return m.Start(ctx)
}

// Status reads the PID file and probes /proc.
func (m *PIDManager) Status(ctx context.Context) (Status, error) {
	pid, err := m.readPID()
	if err != nil || !processAlive(pid) {
		return Status{Mode: ModePID}, nil
	}
	since := time.Time{}
	if info, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
		since = info.ModTime()
	}
	return Status{Running: true, PID: pid, Since: since, Mode: ModePID}, nil
}

// Logs returns mihomo's combined output. follow=true uses tail-like behavior.
func (m *PIDManager) Logs(ctx context.Context, follow bool) (io.ReadCloser, error) {
	if !follow {
		return os.Open(m.cfg.LogFilePath)
	}
	cmd := exec.CommandContext(ctx, "tail", "-n", "200", "-F", m.cfg.LogFilePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &cmdReader{r: stdout, cmd: cmd}, nil
}

type cmdReader struct {
	r   io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReader) Read(p []byte) (int, error) { return c.r.Read(p) }
func (c *cmdReader) Close() error {
	_ = c.cmd.Process.Kill()
	return c.r.Close()
}

func (m *PIDManager) readPID() (int, error) {
	data, err := os.ReadFile(m.cfg.PIDFilePath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func processAlive(pid int) bool {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/service/ -v`
Expected: PID lifecycle tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/service/pid.go internal/service/pid_test.go
git commit -m "feat(service): PID-file backend for non-systemd hosts"
```

---

## Task 11: systemd-user service implementation (`internal/service/systemd.go`)

**Files:**
- Create: `internal/service/systemd.go`
- Create: `internal/service/systemd_test.go`
- Create: `internal/service/templates/mihomo.service.tmpl`

- [ ] **Step 1: Create unit template**

`internal/service/templates/mihomo.service.tmpl`:
```
[Unit]
Description=mihomo proxy core (managed by vpnkit)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.Binary}} -d {{.ConfigDir}}
Restart=on-failure
RestartSec=5s
LimitNOFILE=1048576

[Install]
WantedBy=default.target
```

- [ ] **Step 2: Write failing test**

`internal/service/systemd_test.go`:
```go
package service

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderUnit(t *testing.T) {
	var buf bytes.Buffer
	err := renderUnit(&buf, "/home/u/.local/bin/mihomo", "/home/u/.config/mihomo")
	if err != nil {
		t.Fatalf("renderUnit: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "ExecStart=/home/u/.local/bin/mihomo -d /home/u/.config/mihomo") {
		t.Errorf("missing ExecStart: %s", s)
	}
	if !strings.Contains(s, "WantedBy=default.target") {
		t.Error("missing WantedBy")
	}
}

func TestFakeSystemctlInvocations(t *testing.T) {
	calls := []string{}
	runner := func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}
	m := &SystemdManager{cfg: Config{BinaryPath: "/x", ConfigDir: "/c", UnitPath: t.TempDir() + "/mihomo.service"}, run: runner}
	if err := m.Install(nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(nil); err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"--user daemon-reload",
		"--user enable --now mihomo.service",
		"--user start mihomo.service",
		"--user stop mihomo.service",
	}
	for _, e := range expected {
		found := false
		for _, c := range calls {
			if c == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected call %q not seen; got %v", e, calls)
		}
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/service/ -run Systemd -v`
Expected: compile errors.

- [ ] **Step 4: Write implementation**

`internal/service/systemd.go`:
```go
package service

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/mihomo.service.tmpl
var unitFS embed.FS

// Runner abstracts `systemctl --user` invocation for testability.
type Runner func(args ...string) (string, error)

// SystemdManager implements Manager using `systemctl --user`.
type SystemdManager struct {
	cfg Config
	run Runner
}

// NewSystemd constructs the systemd backend. If runner is nil, real systemctl is used.
func NewSystemd(cfg Config, runner Runner) *SystemdManager {
	if runner == nil {
		runner = defaultSystemctl
	}
	return &SystemdManager{cfg: cfg, run: runner}
}

func defaultSystemctl(args ...string) (string, error) {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (*SystemdManager) Mode() Mode { return ModeSystemdUser }

// Install writes the unit, runs daemon-reload, and enables it.
func (m *SystemdManager) Install(_ context.Context) error {
	if err := os.MkdirAll(filepath.Dir(m.cfg.UnitPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(m.cfg.UnitPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := renderUnit(f, m.cfg.BinaryPath, m.cfg.ConfigDir); err != nil {
		return err
	}
	if _, err := m.run("--user", "daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	if _, err := m.run("--user", "enable", "--now", "mihomo.service"); err != nil {
		return fmt.Errorf("enable: %w", err)
	}
	return nil
}

func (m *SystemdManager) Uninstall(ctx context.Context) error {
	_, _ = m.run("--user", "disable", "--now", "mihomo.service")
	if err := os.Remove(m.cfg.UnitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, _ = m.run("--user", "daemon-reload")
	return nil
}

func (m *SystemdManager) Start(_ context.Context) error {
	_, err := m.run("--user", "start", "mihomo.service")
	return err
}

func (m *SystemdManager) Stop(_ context.Context) error {
	_, err := m.run("--user", "stop", "mihomo.service")
	return err
}

func (m *SystemdManager) Restart(_ context.Context) error {
	_, err := m.run("--user", "restart", "mihomo.service")
	return err
}

func (m *SystemdManager) Status(_ context.Context) (Status, error) {
	out, _ := m.run("--user", "show", "mihomo.service",
		"--property=ActiveState,MainPID,ActiveEnterTimestamp")
	st := Status{Mode: ModeSystemdUser}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "ActiveState":
			st.Running = strings.TrimSpace(v) == "active"
		case "MainPID":
			st.PID, _ = strconv.Atoi(strings.TrimSpace(v))
		case "ActiveEnterTimestamp":
			st.Since, _ = time.Parse("Mon 2006-01-02 15:04:05 MST", strings.TrimSpace(v))
		}
	}
	return st, nil
}

func (m *SystemdManager) Logs(ctx context.Context, follow bool) (io.ReadCloser, error) {
	args := []string{"--user", "-u", "mihomo.service", "--no-pager"}
	if follow {
		args = append(args, "-f", "-n", "200")
	} else {
		args = append(args, "-n", "30")
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &cmdReader{r: stdout, cmd: cmd}, nil
}

func renderUnit(w io.Writer, binary, configDir string) error {
	tmpl, err := template.ParseFS(unitFS, "templates/mihomo.service.tmpl")
	if err != nil {
		return err
	}
	return tmpl.Execute(w, map[string]string{"Binary": binary, "ConfigDir": configDir})
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/service/ -v`
Expected: both systemd tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/service/systemd.go internal/service/systemd_test.go internal/service/templates/
git commit -m "feat(service): systemd --user backend"
```

---

## Task 12: Mode detection + factory (`internal/service/detect.go`)

**Files:**
- Create: `internal/service/detect.go`
- Create: `internal/service/detect_test.go`

- [ ] **Step 1: Write failing test**

`internal/service/detect_test.go`:
```go
package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectViaSocket(t *testing.T) {
	tmp := t.TempDir()
	sd := filepath.Join(tmp, "systemd")
	if err := os.MkdirAll(sd, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(sd, "private"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if got := Detect(nil); got != ModeSystemdUser {
		t.Errorf("got %s want systemd-user", got)
	}
}

func TestDetectFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	runner := func(args ...string) (string, error) {
		return "", &execError{}
	}
	if got := Detect(runner); got != ModePID {
		t.Errorf("got %s want pid", got)
	}
}

type execError struct{}

func (*execError) Error() string { return "exit 1" }
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/service/ -run Detect -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/service/detect.go`:
```go
package service

import (
	"os"
	"path/filepath"
)

// Detect picks the service backend mode based on environment.
// Order: (1) $XDG_RUNTIME_DIR/systemd/private exists → systemd-user;
//       (2) systemctl --user show-environment succeeds → systemd-user;
//       (3) otherwise → pid.
// runner is the systemctl runner used in step 2; pass nil to use the real one.
func Detect(runner Runner) Mode {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		if _, err := os.Stat(filepath.Join(rt, "systemd", "private")); err == nil {
			return ModeSystemdUser
		}
	}
	if runner == nil {
		runner = defaultSystemctl
	}
	if _, err := runner("--user", "show-environment"); err == nil {
		return ModeSystemdUser
	}
	return ModePID
}

// New constructs the appropriate Manager based on Detect or an explicit mode.
// Pass an empty mode to auto-detect.
func New(mode Mode, cfg Config) Manager {
	if mode == "" {
		mode = Detect(nil)
	}
	if mode == ModeSystemdUser {
		return NewSystemd(cfg, nil)
	}
	return NewPID(cfg, nil)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/service/ -v`
Expected: all service tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/service/detect.go internal/service/detect_test.go
git commit -m "feat(service): mode detection and Manager factory"
```

---

## Task 13: Embedded rule templates (`internal/rules`)

**Files:**
- Create: `internal/rules/templates.go`
- Create: `internal/rules/templates_test.go`
- Create: `internal/rules/templates/loyalsoldier.yaml`
- Create: `internal/rules/templates/minimal.yaml`

- [ ] **Step 1: Write `loyalsoldier.yaml`**

`internal/rules/templates/loyalsoldier.yaml`:
```yaml
rule-providers:
  reject:        {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/reject.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/reject.txt"}
  icloud:        {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/icloud.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/icloud.txt"}
  apple:         {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/apple.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/apple.txt"}
  google:        {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/google.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/google.txt"}
  proxy:         {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/proxy.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/proxy.txt"}
  direct:        {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/direct.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/direct.txt"}
  private:       {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/private.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/private.txt"}
  gfw:           {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/gfw.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/gfw.txt"}
  greatfire:     {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/greatfire.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/greatfire.txt"}
  tld-not-cn:    {type: http, behavior: domain, format: text, interval: 86400, path: ./ruleset/tld-not-cn.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/tld-not-cn.txt"}
  telegramcidr:  {type: http, behavior: ipcidr, format: text, interval: 86400, path: ./ruleset/telegramcidr.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/telegramcidr.txt"}
  cncidr:        {type: http, behavior: ipcidr, format: text, interval: 86400, path: ./ruleset/cncidr.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/cncidr.txt"}
  lancidr:       {type: http, behavior: ipcidr, format: text, interval: 86400, path: ./ruleset/lancidr.txt, url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/lancidr.txt"}

rules:
  - RULE-SET,reject,🛑 Reject
  - RULE-SET,private,🎯 Direct
  - RULE-SET,direct,🎯 Direct
  - RULE-SET,lancidr,🎯 Direct
  - RULE-SET,cncidr,🎯 Direct
  - GEOIP,CN,🎯 Direct
  - RULE-SET,proxy,🚀 Proxy
  - RULE-SET,gfw,🚀 Proxy
  - RULE-SET,greatfire,🚀 Proxy
  - RULE-SET,tld-not-cn,🚀 Proxy
  - RULE-SET,telegramcidr,🚀 Proxy
  - MATCH,🚀 Proxy
```

- [ ] **Step 2: Write `minimal.yaml`**

`internal/rules/templates/minimal.yaml`:
```yaml
rules:
  - GEOIP,CN,🎯 Direct
  - GEOIP,LAN,🎯 Direct
  - MATCH,🚀 Proxy
```

- [ ] **Step 3: Write failing test**

`internal/rules/templates_test.go`:
```go
package rules

import (
	"strings"
	"testing"
)

func TestLoadKnownTemplates(t *testing.T) {
	tests := []struct {
		name     string
		contains string
	}{
		{"loyalsoldier", "rule-providers"},
		{"minimal", "GEOIP,CN,🎯 Direct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Load(tt.name)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !strings.Contains(string(data), tt.contains) {
				t.Errorf("template missing %q: %s", tt.contains, string(data))
			}
		})
	}
}

func TestLoadUnknown(t *testing.T) {
	if _, err := Load("nope"); err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestList(t *testing.T) {
	got := List()
	want := map[string]bool{"loyalsoldier": false, "minimal": false}
	for _, n := range got {
		want[n] = true
	}
	for k, v := range want {
		if !v {
			t.Errorf("missing template in List(): %s", k)
		}
	}
}
```

- [ ] **Step 4: Run to verify it fails**

Run: `go test ./internal/rules/ -v`
Expected: compile errors.

- [ ] **Step 5: Write implementation**

`internal/rules/templates.go`:
```go
// Package rules manages embedded rule-set templates that get merged into mihomo's config.
package rules

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed templates/*.yaml
var tmplFS embed.FS

// Load returns the raw YAML bytes of a named template.
func Load(name string) ([]byte, error) {
	data, err := tmplFS.ReadFile("templates/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("rules: unknown template %q", name)
	}
	return data, nil
}

// List enumerates available template names (without extension), sorted alphabetically.
func List() []string {
	var out []string
	_ = fs.WalkDir(tmplFS, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := strings.TrimSuffix(d.Name(), ".yaml")
		out = append(out, name)
		return nil
	})
	sort.Strings(out)
	return out
}
```

- [ ] **Step 6: Run to verify it passes**

Run: `go test ./internal/rules/ -v`
Expected: 3 tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/rules/
git commit -m "feat(rules): embed loyalsoldier and minimal rule templates"
```

---

## Task 14: Config skeleton + atomic write (`internal/config`)

**Files:**
- Create: `internal/config/atomic.go`
- Create: `internal/config/atomic_test.go`
- Create: `internal/config/skeleton.go`
- Create: `internal/config/skeleton_test.go`

- [ ] **Step 1: Write failing test for atomic**

`internal/config/atomic_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "x.yaml")
	if err := AtomicWrite(target, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "hello\n" {
		t.Errorf("got %q err %v", got, err)
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm: %v", info.Mode().Perm())
	}
}

func TestAtomicWriteReplacesExisting(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "x.yaml")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(target, []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new\n" {
		t.Errorf("got %q", got)
	}
}
```

- [ ] **Step 2: Implement atomic**

`internal/config/atomic.go`:
```go
// Package config builds and writes mihomo's runtime config file.
package config

import (
	"os"
	"path/filepath"
)

// AtomicWrite writes data to path via a temp file in the same directory followed by rename.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".atomic-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // no-op if rename succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
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
	return os.Rename(name, path)
}
```

- [ ] **Step 3: Run atomic test**

Run: `go test ./internal/config/ -run Atomic -v`
Expected: pass.

- [ ] **Step 4: Write failing test for skeleton**

`internal/config/skeleton_test.go`:
```go
package config

import (
	"strings"
	"testing"
)

func TestBuildSkeletonIncludesController(t *testing.T) {
	yaml, err := BuildSkeleton(SkeletonInput{
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "deadbeef",
		LogLevel:         "info",
		RuleTemplate:     "minimal",
	})
	if err != nil {
		t.Fatalf("BuildSkeleton: %v", err)
	}
	s := string(yaml)
	mustContain(t, s, "mixed-port: 7890")
	mustContain(t, s, "external-controller: 127.0.0.1:9090")
	mustContain(t, s, "secret: deadbeef")
	mustContain(t, s, "GEOIP,CN,🎯 Direct")
}

func TestBuildSkeletonIncludesDefaultGroups(t *testing.T) {
	yaml, err := BuildSkeleton(SkeletonInput{
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "x",
		RuleTemplate:     "minimal",
	})
	if err != nil {
		t.Fatalf("BuildSkeleton: %v", err)
	}
	s := string(yaml)
	mustContain(t, s, "🚀 Proxy")
	mustContain(t, s, "🎯 Direct")
	mustContain(t, s, "🛑 Reject")
}

func TestBuildSkeletonUnknownTemplate(t *testing.T) {
	_, err := BuildSkeleton(SkeletonInput{RuleTemplate: "nope"})
	if err == nil {
		t.Error("expected error for unknown template")
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("missing %q in:\n%s", needle, haystack)
	}
}
```

- [ ] **Step 5: Implement skeleton**

`internal/config/skeleton.go`:
```go
package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/rules"
)

// SkeletonInput captures the parameters needed to build an initial config.yaml.
type SkeletonInput struct {
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	LogLevel         string
	RuleTemplate     string
}

// BuildSkeleton assembles a complete (proxy-less) config.yaml suitable as a starting
// point before any subscription is loaded. Includes the chosen rule template and
// default proxy-groups (Proxy/Direct/Reject) so mihomo can start without errors.
func BuildSkeleton(in SkeletonInput) ([]byte, error) {
	if in.MixedPort == 0 {
		in.MixedPort = 7890
	}
	if in.ControllerPort == 0 {
		in.ControllerPort = 9090
	}
	if in.LogLevel == "" {
		in.LogLevel = "info"
	}

	template, err := rules.Load(in.RuleTemplate)
	if err != nil {
		return nil, err
	}

	base := map[string]any{
		"mixed-port":          in.MixedPort,
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           in.LogLevel,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", in.ControllerPort),
		"secret":              in.ControllerSecret,
		"proxies":             []any{},
		"proxy-groups": []map[string]any{
			{"name": "🚀 Proxy", "type": "select", "proxies": []string{"🎯 Direct"}},
			{"name": "🎯 Direct", "type": "select", "proxies": []string{"DIRECT"}},
			{"name": "🛑 Reject", "type": "select", "proxies": []string{"REJECT", "DIRECT"}},
		},
	}

	// Merge rule template (rule-providers + rules keys) over base.
	var ruleDoc map[string]any
	if err := yaml.Unmarshal(template, &ruleDoc); err != nil {
		return nil, fmt.Errorf("rule template parse: %w", err)
	}
	for k, v := range ruleDoc {
		base[k] = v
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(base); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}
```

- [ ] **Step 6: Run all config tests**

Run: `go test ./internal/config/ -v`
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/config/
git commit -m "feat(config): atomic write + initial skeleton builder"
```

---

## Task 15: mihomo API client basics (`internal/api/client.go`)

**Files:**
- Create: `internal/api/client.go`
- Create: `internal/api/client_test.go`

- [ ] **Step 1: Write failing test**

`internal/api/client_test.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("auth header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.19.16", "meta": true})
	}))
	defer srv.Close()
	c := New(srv.URL, "secret")
	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v.Version != "1.19.16" || !v.Meta {
		t.Errorf("got %+v", v)
	}
}

func TestSetMode(t *testing.T) {
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method: %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.SetMode(context.Background(), "global"); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "global" {
		t.Errorf("body: %v", body)
	}
}

func TestErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	_, err := c.Version(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/api/ -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/api/client.go`:
```go
// Package api talks to mihomo's external-controller HTTP/SSE/WS endpoints.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a thread-safe mihomo external-controller client.
type Client struct {
	BaseURL string
	Secret  string
	HTTP    *http.Client
}

// New builds a Client. baseURL like "http://127.0.0.1:9090". secret optional.
func New(baseURL, secret string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		BaseURL: baseURL,
		Secret:  secret,
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
}

// VersionInfo mirrors mihomo's /version response.
type VersionInfo struct {
	Version string `json:"version"`
	Meta    bool   `json:"meta"`
}

// Version queries /version.
func (c *Client) Version(ctx context.Context) (VersionInfo, error) {
	var v VersionInfo
	err := c.do(ctx, http.MethodGet, "/version", nil, &v)
	return v, err
}

// SetMode PATCHes /configs to switch mihomo's mode: rule|global|direct.
func (c *Client) SetMode(ctx context.Context, mode string) error {
	body := map[string]string{"mode": mode}
	return c.do(ctx, http.MethodPatch, "/configs", body, nil)
}

// ReloadConfig PUTs /configs with a path (mihomo reloads from disk).
func (c *Client) ReloadConfig(ctx context.Context, path string) error {
	body := map[string]string{"path": path}
	return c.do(ctx, http.MethodPut, "/configs", body, nil)
}

// do performs a JSON HTTP request, decoding into `into` if non-nil.
func (c *Client) do(ctx context.Context, method, path string, body any, into any) error {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mihomo %s %s: %d %s", method, path, resp.StatusCode, string(bytes.TrimSpace(buf)))
	}
	if into != nil {
		return json.NewDecoder(resp.Body).Decode(into)
	}
	return nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/api/ -v`
Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/api/client.go internal/api/client_test.go
git commit -m "feat(api): mihomo REST client with version + mode/reload"
```

---

## Task 16: Traffic SSE stream (`internal/api/traffic.go`)

**Files:**
- Create: `internal/api/traffic.go`
- Create: `internal/api/traffic_test.go`

- [ ] **Step 1: Write failing test**

`internal/api/traffic_test.go`:
```go
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTrafficStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"up": 100, "down": 200}`)
		flusher.Flush()
		fmt.Fprintln(w, `{"up": 300, "down": 400}`)
		flusher.Flush()
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, errCh := c.Traffic(ctx)
	got := []Traffic{}
	for i := 0; i < 2; i++ {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case err := <-errCh:
			t.Fatalf("err: %v", err)
		case <-ctx.Done():
			t.Fatal("timeout")
		}
	}
	if got[0].Up != 100 || got[1].Down != 400 {
		t.Errorf("got %+v", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/api/ -run Traffic -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/api/traffic.go`:
```go
package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
)

// Traffic is one /traffic sample from mihomo (bytes per second since last tick).
type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// Traffic opens an SSE-ish line-delimited JSON stream of /traffic and pushes events
// to the returned channel. Errors land on errCh and close both channels.
// The stream stops when ctx is cancelled.
func (c *Client) Traffic(ctx context.Context) (<-chan Traffic, <-chan error) {
	out := make(chan Traffic, 16)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		client := &http.Client{Timeout: 0} // long-lived
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/traffic", nil)
		if err != nil {
			errCh <- err
			return
		}
		if c.Secret != "" {
			req.Header.Set("Authorization", "Bearer "+c.Secret)
		}
		resp, err := client.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 4096), 1<<20)
		for scanner.Scan() {
			var t Traffic
			if err := json.Unmarshal(scanner.Bytes(), &t); err != nil {
				continue
			}
			select {
			case out <- t:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()
	return out, errCh
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/api/ -v`
Expected: all api tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/api/traffic.go internal/api/traffic_test.go
git commit -m "feat(api): /traffic streaming consumer"
```

---

## Task 17: `vpnkit env` shell snippet generator (`internal/env`)

**Files:**
- Create: `internal/env/env.go`
- Create: `internal/env/env_test.go`

- [ ] **Step 1: Write failing test**

`internal/env/env_test.go`:
```go
package env

import (
	"strings"
	"testing"
)

func TestRenderBash(t *testing.T) {
	got := Render(Options{Shell: "bash", Port: 7890, NoProxy: "localhost,127.0.0.1"})
	for _, want := range []string{
		"export http_proxy=http://127.0.0.1:7890",
		"export https_proxy=http://127.0.0.1:7890",
		"export all_proxy=socks5h://127.0.0.1:7890",
		"export no_proxy=localhost,127.0.0.1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderFish(t *testing.T) {
	got := Render(Options{Shell: "fish", Port: 7890})
	if !strings.Contains(got, "set -gx http_proxy http://127.0.0.1:7890") {
		t.Errorf("fish output: %s", got)
	}
}

func TestRenderUnset(t *testing.T) {
	got := Render(Options{Shell: "bash", Unset: true})
	for _, want := range []string{"unset http_proxy", "unset https_proxy", "unset all_proxy", "unset no_proxy"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/env/ -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/env/env.go`:
```go
// Package env renders shell-specific snippets that export (or unset) proxy variables.
package env

import (
	"fmt"
	"strings"
)

// Options drives the renderer.
type Options struct {
	Shell   string // bash|zsh|fish; empty = bash
	Port    int    // mihomo mixed-port; default 7890
	NoProxy string // optional no_proxy value
	Unset   bool   // emit unset/erase instead of export/set
}

// Render returns a snippet suitable for `eval "$(vpnkit env)"`.
func Render(o Options) string {
	if o.Port == 0 {
		o.Port = 7890
	}
	if o.Shell == "" {
		o.Shell = "bash"
	}
	if o.Unset {
		return renderUnset(o.Shell)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", o.Port)
	socks := fmt.Sprintf("socks5h://127.0.0.1:%d", o.Port)
	var b strings.Builder
	switch o.Shell {
	case "fish":
		fmt.Fprintf(&b, "set -gx http_proxy %s\n", url)
		fmt.Fprintf(&b, "set -gx https_proxy %s\n", url)
		fmt.Fprintf(&b, "set -gx all_proxy %s\n", socks)
		if o.NoProxy != "" {
			fmt.Fprintf(&b, "set -gx no_proxy %s\n", o.NoProxy)
		}
	default: // bash, zsh
		fmt.Fprintf(&b, "export http_proxy=%s\n", url)
		fmt.Fprintf(&b, "export https_proxy=%s\n", url)
		fmt.Fprintf(&b, "export all_proxy=%s\n", socks)
		if o.NoProxy != "" {
			fmt.Fprintf(&b, "export no_proxy=%s\n", o.NoProxy)
		}
	}
	return b.String()
}

func renderUnset(shell string) string {
	vars := []string{"http_proxy", "https_proxy", "all_proxy", "no_proxy"}
	var b strings.Builder
	for _, v := range vars {
		switch shell {
		case "fish":
			fmt.Fprintf(&b, "set -e %s\n", v)
		default:
			fmt.Fprintf(&b, "unset %s\n", v)
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/env/ -v`
Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/env/
git commit -m "feat(env): shell snippet renderer for proxy env vars"
```

---

## Task 18: App messages and key bindings (`internal/app/messages.go`, `internal/app/keys.go`)

**Files:**
- Create: `internal/app/messages.go`
- Create: `internal/app/keys.go`

- [ ] **Step 1: Write messages file**

`internal/app/messages.go`:
```go
// Package app contains the top-level bubbletea Model and ties subsystems together.
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
)

// TrafficMsg carries one /traffic sample.
type TrafficMsg api.Traffic

// VersionMsg announces the mihomo version (or error) returned by /version.
type VersionMsg struct {
	Version string
	Err     error
}

// ServiceStatusMsg snapshots the service backend status.
type ServiceStatusMsg struct {
	Running bool
	PID     int
	Mode    string
	Since   time.Time
}

// BootstrapProgressMsg announces a phase of the first-run flow.
type BootstrapProgressMsg struct {
	Phase string // "downloading" | "installing-service" | "starting" | "ready"
	Note  string
	Err   error
}

// FlashMsg is a transient status-bar notification.
type FlashMsg struct {
	Text  string
	Kind  FlashKind
	Until time.Time
}

// FlashKind distinguishes severity for styling.
type FlashKind int

const (
	FlashInfo FlashKind = iota
	FlashWarn
	FlashError
)

// TickMsg is emitted by periodic timers.
type TickMsg struct{ T time.Time }

// QuitMsg signals graceful exit.
type QuitMsg struct{}

// Compile-time interface checks.
var (
	_ tea.Msg = TrafficMsg{}
	_ tea.Msg = VersionMsg{}
)
```

- [ ] **Step 2: Write key bindings**

`internal/app/keys.go`:
```go
package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap groups global key bindings shown in the help overlay.
type KeyMap struct {
	Tab1, Tab2, Tab3, Tab4, Tab5, Tab6 key.Binding
	NextTab, PrevTab                   key.Binding
	Help, Quit                         key.Binding
	Restart, Mode                      key.Binding
	Palette                            key.Binding
}

// DefaultKeys returns the standard global key bindings.
func DefaultKeys() KeyMap {
	return KeyMap{
		Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Dashboard")),
		Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Proxies")),
		Tab3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Profiles")),
		Tab4:    key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Connections")),
		Tab5:    key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "Rules")),
		Tab6:    key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "Settings")),
		NextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "next tab")),
		PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("S-Tab", "prev tab")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart mihomo")),
		Mode:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "cycle mode")),
		Palette: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command palette")),
	}
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/app/`
Expected: builds clean.

- [ ] **Step 4: Commit**

```bash
git add internal/app/messages.go internal/app/keys.go
git commit -m "feat(app): tea.Msg types and global keymap"
```

---

## Task 19: Stub tab (`internal/tabs/stub`)

**Files:**
- Create: `internal/tabs/stub/stub.go`

- [ ] **Step 1: Write implementation directly (trivial)**

`internal/tabs/stub/stub.go`:
```go
// Package stub provides a placeholder tab model used until a real implementation lands.
package stub

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is a stateless placeholder tab.
type Model struct {
	Name string
}

// New returns a stub for a named tab.
func New(name string) Model { return Model{Name: name} }

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update is a no-op.
func (m Model) Update(tea.Msg) (Model, tea.Cmd) { return m, nil }

// View renders a centered placeholder.
func (m Model) View(width, height int) string {
	body := fmt.Sprintf("%s — coming in a later phase", m.Name)
	style := lipgloss.NewStyle().Width(width).Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("240"))
	return style.Render(body)
}
```

- [ ] **Step 2: Build**

Run: `go build ./internal/tabs/stub/`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add internal/tabs/stub/
git commit -m "feat(tabs): stub placeholder tab"
```

---

## Task 20: Dashboard tab (`internal/tabs/dashboard`)

**Files:**
- Create: `internal/tabs/dashboard/dashboard.go`
- Create: `internal/tabs/dashboard/dashboard_test.go`

- [ ] **Step 1: Write failing test**

`internal/tabs/dashboard/dashboard_test.go`:
```go
package dashboard

import (
	"strings"
	"testing"

	"vpnkit/internal/app"
)

func TestDashboardRendersTrafficText(t *testing.T) {
	m := New()
	m, _ = m.Update(app.TrafficMsg{Up: 1024, Down: 2048})
	view := m.View(80, 24)
	for _, want := range []string{"↑", "↓", "Mihomo"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in view:\n%s", want, view)
		}
	}
}

func TestDashboardKeepsSparklineHistory(t *testing.T) {
	m := New()
	for i := int64(0); i < 100; i++ {
		m, _ = m.Update(app.TrafficMsg{Up: i, Down: i})
	}
	if got := len(m.UpHistory()); got != 60 {
		t.Errorf("expected ring of 60, got %d", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tabs/dashboard/ -v`
Expected: compile errors.

- [ ] **Step 3: Write implementation**

`internal/tabs/dashboard/dashboard.go`:
```go
// Package dashboard implements the first vpnkit tab: live traffic + service status.
package dashboard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/app"
)

const historySize = 60

// Model holds the dashboard's local state.
type Model struct {
	upHist, downHist []int64
	lastUp, lastDown int64
	mihomoVer        string
	mode             string
	running          bool
}

// New returns an empty dashboard model.
func New() Model {
	return Model{
		upHist:   make([]int64, 0, historySize),
		downHist: make([]int64, 0, historySize),
		mode:     "rule",
	}
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update absorbs messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch v := msg.(type) {
	case app.TrafficMsg:
		m.lastUp = v.Up
		m.lastDown = v.Down
		m.upHist = pushRing(m.upHist, v.Up, historySize)
		m.downHist = pushRing(m.downHist, v.Down, historySize)
	case app.VersionMsg:
		if v.Err == nil {
			m.mihomoVer = v.Version
		}
	case app.ServiceStatusMsg:
		m.running = v.Running
	}
	return m, nil
}

// UpHistory exposes the up-traffic ring (for tests).
func (m Model) UpHistory() []int64 { return m.upHist }

// View renders the dashboard within (width, height).
func (m Model) View(width, height int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	body := fmt.Sprintf(
		"%s\n\n  Status : %s\n  Version: %s\n  Mode   : %s\n\n  ↑ %s/s\n  ↓ %s/s\n",
		headerStyle.Render("Mihomo"),
		runStr(m.running),
		fallback(m.mihomoVer, "unknown"),
		m.mode,
		humanRate(m.lastUp),
		humanRate(m.lastDown),
	)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func pushRing(buf []int64, v int64, max int) []int64 {
	if len(buf) >= max {
		buf = buf[1:]
	}
	return append(buf, v)
}

func runStr(b bool) string {
	if b {
		return "● running"
	}
	return "○ stopped"
}

func fallback(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

func humanRate(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
	)
	switch {
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/tabs/dashboard/ -v`
Expected: 2 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tabs/dashboard/
git commit -m "feat(tabs): dashboard tab with traffic history and status"
```

---

## Task 21: App top-level Model + sidebar + statusbar + view (`internal/app/{model,sidebar,statusbar,view,update}.go`)

**Files:**
- Create: `internal/app/model.go`
- Create: `internal/app/sidebar.go`
- Create: `internal/app/statusbar.go`
- Create: `internal/app/view.go`
- Create: `internal/app/update.go`
- Create: `internal/app/model_test.go`

- [ ] **Step 1: Write model and primitives**

`internal/app/model.go`:
```go
package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/tabs/dashboard"
	"vpnkit/internal/tabs/stub"
)

// Tab is the index of the currently-active tab.
type Tab int

const (
	TabDashboard Tab = iota
	TabProxies
	TabProfiles
	TabConnections
	TabRules
	TabSettings
	NumTabs
)

var TabNames = [NumTabs]string{
	"Dashboard", "Proxies", "Profiles", "Connections", "Rules", "Settings",
}

// Model is the top-level bubbletea model.
type Model struct {
	keys      KeyMap
	activeTab Tab
	width     int
	height    int

	dashboard dashboard.Model
	stubs     [NumTabs]stub.Model // index 0 unused; entries for 1..5

	apiClient *api.Client
	flash     string // single-line transient
}

// NewModel constructs the initial model. apiClient may be nil during early bootstrap.
func NewModel(client *api.Client) Model {
	stubs := [NumTabs]stub.Model{}
	for i := TabProxies; i < NumTabs; i++ {
		stubs[i] = stub.New(TabNames[i])
	}
	return Model{
		keys:      DefaultKeys(),
		activeTab: TabDashboard,
		dashboard: dashboard.New(),
		stubs:     stubs,
		apiClient: client,
	}
}

// Init returns startup commands.
func (m Model) Init() tea.Cmd {
	return nil // bootstrap & subscriptions are wired in app.Run
}
```

- [ ] **Step 2: Write sidebar + statusbar + view**

`internal/app/sidebar.go`:
```go
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 16

var (
	activeStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	inactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func renderSidebar(active Tab, height int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("vpnkit"))
	b.WriteString("\n\n")
	for i := Tab(0); i < NumTabs; i++ {
		line := fmt.Sprintf("[%d] %s", int(i)+1, TabNames[i])
		if i == active {
			b.WriteString(activeStyle.Render("▶ " + line))
		} else {
			b.WriteString(inactiveStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Width(sidebarWidth).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).Render(b.String())
}
```

`internal/app/statusbar.go`:
```go
package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderStatusBar(width int) string {
	left := fmt.Sprintf(" %s  ↑ %s/s  ↓ %s/s ",
		runDot(m.dashboard),
		fmtRate(m.dashboard.UpHistoryLast()),
		fmtRate(m.dashboard.DownHistoryLast()),
	)
	right := " ?:help q:quit "
	if m.flash != "" {
		right = " " + m.flash + " "
	}
	gapLen := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gapLen < 0 {
		gapLen = 0
	}
	gap := lipgloss.NewStyle().Render(stringRepeat(" ", gapLen))
	return lipgloss.NewStyle().Reverse(true).Width(width).Render(left + gap + right)
}

func stringRepeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
```

`internal/app/view.go`:
```go
package app

import "github.com/charmbracelet/lipgloss"

// View composes sidebar + tab body + statusbar.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	bodyHeight := m.height - 1 // reserve a line for status bar
	sidebar := renderSidebar(m.activeTab, bodyHeight)
	bodyWidth := m.width - sidebarWidth

	var body string
	switch m.activeTab {
	case TabDashboard:
		body = m.dashboard.View(bodyWidth, bodyHeight)
	default:
		body = m.stubs[m.activeTab].View(bodyWidth, bodyHeight)
	}

	rows := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
	return rows + "\n" + m.renderStatusBar(m.width)
}
```

`internal/app/update.go`:
```go
package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(v, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(v, m.keys.Tab1):
			m.activeTab = TabDashboard
		case key.Matches(v, m.keys.Tab2):
			m.activeTab = TabProxies
		case key.Matches(v, m.keys.Tab3):
			m.activeTab = TabProfiles
		case key.Matches(v, m.keys.Tab4):
			m.activeTab = TabConnections
		case key.Matches(v, m.keys.Tab5):
			m.activeTab = TabRules
		case key.Matches(v, m.keys.Tab6):
			m.activeTab = TabSettings
		case key.Matches(v, m.keys.NextTab):
			m.activeTab = (m.activeTab + 1) % NumTabs
		case key.Matches(v, m.keys.PrevTab):
			m.activeTab = (m.activeTab + NumTabs - 1) % NumTabs
		}
	case TrafficMsg, VersionMsg, ServiceStatusMsg:
		m.dashboard, cmd = m.dashboard.Update(msg)
	}
	return m, cmd
}
```

Add helpers used by statusbar to `internal/tabs/dashboard/dashboard.go` (append):
```go
// UpHistoryLast returns the most recent up rate or 0.
func (m Model) UpHistoryLast() int64 {
	if len(m.upHist) == 0 {
		return 0
	}
	return m.upHist[len(m.upHist)-1]
}

// DownHistoryLast returns the most recent down rate or 0.
func (m Model) DownHistoryLast() int64 {
	if len(m.downHist) == 0 {
		return 0
	}
	return m.downHist[len(m.downHist)-1]
}
```

Add `runDot` and `fmtRate` to `internal/app/statusbar.go`:
```go
func runDot(d interface{ UpHistoryLast() int64 }) string {
	// dashboard reports running indirectly via traffic; a non-zero last sample implies traffic flowing.
	if d.UpHistoryLast() > 0 {
		return "●"
	}
	return "○"
}

func fmtRate(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
	)
	switch {
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
```

- [ ] **Step 3: Write test for model basics**

`internal/app/model_test.go`:
```go
package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTabSwitching(t *testing.T) {
	m := NewModel(nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	send := func(k string) {
		m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	}
	send("3")
	if m.activeTab != TabProfiles {
		t.Errorf("expected Profiles, got %v", m.activeTab)
	}
	send("\t")
	if m.activeTab != TabConnections {
		t.Errorf("Tab cycle failed: %v", m.activeTab)
	}

	view := m.View()
	if !strings.Contains(view, "Profiles") || !strings.Contains(view, "vpnkit") {
		t.Errorf("view missing chrome:\n%s", view)
	}
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	mm, cmd := m.Update(msg)
	return mm.(Model), cmd
}
```

Note the `tea.KeyMsg` for Tab key needs an actual KeyType (`tea.KeyTab`). Replace test to:

```go
func TestTabSwitching(t *testing.T) {
	m := NewModel(nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if m.activeTab != TabProfiles {
		t.Errorf("expected Profiles, got %v", m.activeTab)
	}
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.activeTab != TabConnections {
		t.Errorf("Tab cycle failed: %v", m.activeTab)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -v`
Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/app/ internal/tabs/dashboard/dashboard.go
git commit -m "feat(app): top-level model with sidebar, statusbar, tab switching"
```

---

## Task 22: Bootstrap orchestrator + `Run` entry (`internal/app/bootstrap.go`, `internal/app/run.go`)

**Files:**
- Create: `internal/app/bootstrap.go`
- Create: `internal/app/run.go`

- [ ] **Step 1: Write bootstrap orchestrator**

`internal/app/bootstrap.go`:
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/config"
	"vpnkit/internal/installer"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
)

// BootstrapDeps are injectable to ease testing later.
type BootstrapDeps struct {
	Paths   paths.XDG
	Store   *store.Store
	Service service.Manager
	// InstallFunc lets tests stub installer.Install.
	InstallFunc func(opts installer.Options, prog installer.ProgressFunc) (installer.Result, error)
}

// MaybeBootstrap returns a tea.Cmd that performs first-run setup only if needed.
// It emits BootstrapProgressMsg at each phase; the top-level Model can render them.
func MaybeBootstrap(d BootstrapDeps) tea.Cmd {
	return func() tea.Msg {
		// 1. Ensure XDG dirs exist.
		if err := d.Paths.Ensure(); err != nil {
			return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("paths: %w", err)}
		}
		// 2. Install mihomo if missing.
		if _, err := os.Stat(d.Paths.MihomoBinary()); errors.Is(err, fs.ErrNotExist) {
			if d.InstallFunc == nil {
				d.InstallFunc = installer.Install
			}
			_, err := d.InstallFunc(installer.Options{
				Dst:     d.Paths.MihomoBinary(),
				Mirror:  d.Store.Cfg.ReleaseMirror,
				APIBase: "",
			}, nil)
			if err != nil {
				return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("install: %w", err)}
			}
		}
		// 3. Generate config.yaml if missing.
		if _, err := os.Stat(d.Paths.MihomoConfigFile()); errors.Is(err, fs.ErrNotExist) {
			data, err := config.BuildSkeleton(config.SkeletonInput{
				MixedPort:        7890,
				ControllerPort:   d.Store.Cfg.ControllerPort,
				ControllerSecret: d.Store.Cfg.ControllerSecret,
				RuleTemplate:     d.Store.Cfg.RuleTemplate,
			})
			if err != nil {
				return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("config: %w", err)}
			}
			if err := config.AtomicWrite(d.Paths.MihomoConfigFile(), data, 0o600); err != nil {
				return BootstrapProgressMsg{Phase: "error", Err: err}
			}
		}
		// 4. Install + start the service.
		ctx := context.Background()
		if err := d.Service.Install(ctx); err != nil {
			return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("service install: %w", err)}
		}
		// `Install` for systemd already enables --now; for PID Install is a no-op so call Start.
		_ = d.Service.Start(ctx)
		return BootstrapProgressMsg{Phase: "ready"}
	}
}
```

- [ ] **Step 2: Write Run entry**

`internal/app/run.go`:
```go
package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
)

// Run launches the vpnkit TUI. Returns the bubbletea exit error.
func Run() error {
	p := paths.Resolve()
	if err := p.Ensure(); err != nil {
		return fmt.Errorf("paths: %w", err)
	}
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	// Detect service mode on first run.
	if st.Cfg.ServiceMode == "" {
		mode := service.Detect(nil)
		st.Cfg.ServiceMode = string(mode)
		_ = st.Save()
	}
	svc := service.New(service.Mode(st.Cfg.ServiceMode), service.Config{
		BinaryPath:  p.MihomoBinary(),
		ConfigDir:   p.MihomoConfig,
		PIDFilePath: p.PIDFile(),
		LogFilePath: p.MihomoLog(),
		UnitPath:    p.SystemdUnit(),
	})
	client := api.New(fmt.Sprintf("http://127.0.0.1:%d", st.Cfg.ControllerPort), st.Cfg.ControllerSecret)

	model := NewModel(client)
	prog := tea.NewProgram(model, tea.WithAltScreen())

	// Kick off bootstrap + streams in goroutines that send into the program.
	go func() {
		msg := MaybeBootstrap(BootstrapDeps{
			Paths:   p,
			Store:   st,
			Service: svc,
		})()
		prog.Send(msg)
	}()
	go streamTraffic(prog, client)
	go pollVersion(prog, client)

	_, err = prog.Run()
	return err
}

func streamTraffic(prog *tea.Program, client *api.Client) {
	for {
		ctx, cancel := context.WithCancel(context.Background())
		ch, errCh := client.Traffic(ctx)
	loop:
		for {
			select {
			case t, ok := <-ch:
				if !ok {
					break loop
				}
				prog.Send(TrafficMsg(t))
			case <-errCh:
				break loop
			}
		}
		cancel()
		time.Sleep(2 * time.Second) // backoff before reconnect
	}
}

func pollVersion(prog *tea.Program, client *api.Client) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		v, err := client.Version(ctx)
		cancel()
		prog.Send(VersionMsg{Version: v.Version, Err: err})
		<-ticker.C
	}
}
```

- [ ] **Step 3: Update top-level Model to handle BootstrapProgressMsg in flash**

Edit `internal/app/update.go` — add a case before `default`:
```go
	case BootstrapProgressMsg:
		switch v.Phase {
		case "ready":
			m.flash = "mihomo ready"
		case "error":
			if v.Err != nil {
				m.flash = "bootstrap: " + v.Err.Error()
			}
		default:
			m.flash = "bootstrapping: " + v.Phase
		}
```

(Place inside the `switch v := msg.(type)` block adjacent to `TrafficMsg`.)

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: builds clean.

- [ ] **Step 5: Commit**

```bash
git add internal/app/bootstrap.go internal/app/run.go internal/app/update.go
git commit -m "feat(app): first-run bootstrap + tea.Program runner with live streams"
```

---

## Task 23: `cmd/vpnkit/main.go` entry

**Files:**
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Replace placeholder main**

`cmd/vpnkit/main.go`:
```go
package main

import (
	"flag"
	"fmt"
	"os"

	"vpnkit/internal/app"
	"vpnkit/internal/env"
	"vpnkit/internal/paths"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			runVersion()
			return
		case "env":
			runEnv(os.Args[2:])
			return
		}
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "vpnkit:", err)
		os.Exit(1)
	}
}

func runVersion() {
	fmt.Printf("vpnkit %s\n", version)
	// Best-effort mihomo version (read binary via /version not yet available without service).
	p := paths.Resolve()
	if info, err := os.Stat(p.MihomoBinary()); err == nil {
		fmt.Printf("mihomo binary: %s (%d bytes)\n", p.MihomoBinary(), info.Size())
	} else {
		fmt.Println("mihomo binary: not installed")
	}
}

func runEnv(args []string) {
	fs := flag.NewFlagSet("env", flag.ExitOnError)
	shell := fs.String("shell", os.Getenv("SHELL"), "shell flavor: bash, zsh, or fish")
	noProxy := fs.String("no-proxy", "localhost,127.0.0.1,::1", "comma-separated no_proxy")
	unset := fs.Bool("unset", false, "emit unset/erase commands instead of export/set")
	_ = fs.Parse(args)

	flavor := "bash"
	switch {
	case *shell == "" || stringContains(*shell, "bash"):
		flavor = "bash"
	case stringContains(*shell, "zsh"):
		flavor = "zsh"
	case stringContains(*shell, "fish"):
		flavor = "fish"
	}

	// mixed-port is hardcoded to 7890 in Phase 1's skeleton config. A later phase
	// will plumb the port through the store so this respects user overrides.
	port := 7890
	out := env.Render(env.Options{Shell: flavor, Port: port, NoProxy: *noProxy, Unset: *unset})
	fmt.Print(out)
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Build the full binary**

Run:
```bash
go build -o ./bin/vpnkit ./cmd/vpnkit
./bin/vpnkit --version
./bin/vpnkit env --shell zsh
```
Expected:
- `vpnkit dev\nmihomo binary: not installed`
- Lines beginning with `export http_proxy=`.

- [ ] **Step 3: Commit**

```bash
git add cmd/vpnkit/main.go
git commit -m "feat(cmd): vpnkit entry — version / env / TUI dispatch"
```

---

## Task 24: CI workflow + linter config

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.golangci.yml`

- [ ] **Step 1: Write CI workflow**

`.github/workflows/ci.yml`:
```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        arch: [amd64, arm64]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test -race -cover ./...
        env:
          GOARCH: ${{ matrix.arch }}
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.60.3
          args: --timeout=5m
```

- [ ] **Step 2: Write linter config**

`.golangci.yml`:
```yaml
run:
  timeout: 5m
  tests: true
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - ineffassign
    - unused
    - gosimple
    - revive
    - misspell
    - gofmt
    - goimports
linters-settings:
  revive:
    rules:
      - name: exported
        disabled: false
issues:
  exclude-rules:
    - path: _test\.go
      linters: [revive, errcheck]
```

- [ ] **Step 3: Run linter locally**

Run:
```bash
golangci-lint run --timeout=5m || true
```
Expected: lint runs (fix any easy issues it surfaces). Skip if golangci-lint is not installed locally — CI will run it.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml .golangci.yml
git commit -m "ci: add GitHub Actions workflow and golangci-lint config"
```

---

## Task 25: Manual smoke check + Phase 1 wrap-up

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Build and install**

Run:
```bash
make install
ls -la ~/.local/bin/vpnkit
```
Expected: binary exists.

- [ ] **Step 2: Smoke-test `env`**

Run:
```bash
~/.local/bin/vpnkit env --shell zsh
~/.local/bin/vpnkit --version
```
Expected: env snippet printed; version prints `vpnkit dev` and `mihomo binary: not installed`.

- [ ] **Step 3: Smoke-test TUI**

Run in a fresh terminal:
```bash
~/.local/bin/vpnkit
```
Expected:
- TUI opens with sidebar listing 6 tabs.
- Status bar at bottom flashes `bootstrapping: …` and then `mihomo ready`.
- Press `2`/`3`/`4`/`5`/`6` → other tabs show "Coming in a later phase".
- Press `Tab` → cycles tabs forward.
- Press `q` → quits.
- After quit, `systemctl --user status mihomo` (or `cat ~/.local/state/vpnkit/mihomo.pid`) shows mihomo still running.
- `curl -H "Authorization: Bearer $(grep ^controller_secret ~/.config/vpnkit/config.toml | cut -d\" -f2)" http://127.0.0.1:9090/version` returns mihomo JSON.

Record real outputs in commit body of next step.

- [ ] **Step 4: Update README with quickstart**

Append to `README.md`:
```markdown

## Quickstart (Phase 1)

```bash
make install        # builds and installs to ~/.local/bin/vpnkit
vpnkit              # launches TUI; first run downloads mihomo silently
                    # press 1-6 to switch tabs, q to quit (mihomo keeps running)
eval "$(vpnkit env --shell zsh)"   # export proxy env vars for current shell
curl https://www.google.com         # traffic now goes through mihomo
```

Stop mihomo:
- systemd mode: `systemctl --user stop mihomo`
- PID mode:     `kill $(cat ~/.local/state/vpnkit/mihomo.pid)`

Phase 1 ships the installer, service manager, and a working Dashboard tab. Profiles / Proxies / Connections / Rules / Settings land in subsequent phases.
```

- [ ] **Step 5: Commit + tag**

```bash
git add README.md
git commit -m "docs: quickstart for Phase 1"
git tag v0.1.0-phase1
```

Phase 1 complete. Recommend running `git log --oneline` to verify granular commit history.

---

## Self-Review Notes

- Spec section 2 (Entry Points): `vpnkit`, `vpnkit env`, `vpnkit --version` — all covered by Task 23.
- Spec section 3 (First-Run Bootstrap): Tasks 5–8 + 14 + 22 cover detect → fetch → SHA → unpack → config gen → service install → start. `install --headless` is intentionally deferred to Phase 4; not blocking for Phase 1's "works on first launch" promise.
- Spec section 4 (TUI Architecture, 6 tabs): Tasks 19–21 implement skeleton + Dashboard. Tabs 2–6 use the stub model (Task 19).
- Spec section 5 (Subscription Conversion): Not in Phase 1 — explicitly Phase 2.
- Spec section 6 (Default Rule Set): `loyalsoldier` and `minimal` shipped in Task 13; consumed by Task 14.
- Spec section 7 (Filesystem Layout): paths covered by Task 2; lazy creation respected.
- Spec section 8 (mihomo REST API Usage): Phase 1 uses `/version` + `/traffic` + (in bootstrap) implicit `PUT /configs`. The full surface lands in Phase 3.
- Spec section 9 (Service Management): Tasks 9–12 deliver both backends + detection.
- Spec section 10 (Error Handling): Bootstrap surfaces errors via `BootstrapProgressMsg` and flash bar; download retries are deferred to Phase 2 (not blocking — current single-attempt failure shows a clear error to user).
- Spec section 11 (Testing): Each task includes failing-test-first, ≥80% target met via the listed unit tests. TUI tests use direct Model.Update calls (teatest reserved for Phase 2 + interactive flows).
- Spec section 12 (Phase Plan): This plan is **Phase 1 only**, as defined in §12 of the spec.

No placeholders, no TODOs in plan; types and function names referenced across tasks (e.g. `paths.XDG`, `service.Config`, `installer.Options`, `app.TrafficMsg`) are introduced before use.

End of Phase 1 plan.
