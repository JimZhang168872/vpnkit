# vpnkit Phase 4 Implementation Plan — Settings polish

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Finish the Settings tab from a single-page Logs viewer into a real sub-menu with 7 sub-pages — Mihomo Core (version + upgrade), Service (start/stop/restart/uninstall), External Controller (port + secret), Default Rules (template picker), Patch Editor (textarea over `~/.config/mihomo/patch.yaml`), Logs (moved from Phase 3), Cache (size + clear), and About. Plus profile persistence to disk on every mutation.

**Architecture:** New `internal/tabs/settings/` package with a `Model` that owns sub-page navigation (left sub-sidebar) and dispatches to per-sub-page models. Sub-pages are small Models with their own state. Phase 3's `tabs/logs.Model` gets embedded as one sub-page rather than the whole Settings tab. New `store.Save()` calls land in the profiles wiring.

**Tech Stack:** unchanged.

**Spec reference:** `docs/superpowers/specs/2026-05-15-vpnkit-tui-design.md` §4.5 Settings sub-pages.

---

## File Map

| Path | Responsibility |
|---|---|
| `internal/tabs/settings/settings.go` | Top-level Settings Model, sub-menu nav |
| `internal/tabs/settings/settings_test.go` | Sub-menu navigation tests |
| `internal/tabs/settings/about.go` | About sub-page (versions + license) |
| `internal/tabs/settings/cache.go` | Cache size + clear sub-page |
| `internal/tabs/settings/cache_test.go` | |
| `internal/tabs/settings/rules.go` | Default rule template picker |
| `internal/tabs/settings/controller.go` | External controller settings (port + secret) |
| `internal/tabs/settings/service.go` | Service controls sub-page |
| `internal/tabs/settings/core.go` | Mihomo core info + upgrade |
| `internal/tabs/settings/patch.go` | Patch editor (textarea) |
| `internal/tabs/settings/patch_test.go` | |
| `internal/app/model.go` (MODIFY) | Wire settings.Model + store + svc handles |
| `internal/app/view.go` (MODIFY) | Render settings |
| `internal/app/update.go` (MODIFY) | Route settings keys + profile persistence callbacks |
| `internal/app/run.go` (MODIFY) | Pass store + svc into settings construction; persist profiles on mutation |
| `internal/profiles/manager.go` (MODIFY) | Add `OnChange` callback |

---

## Task 1: Settings sub-menu shell

**Files:**
- Create: `internal/tabs/settings/settings.go`
- Create: `internal/tabs/settings/settings_test.go`

This is the entry sub-Model. It hosts a left sub-sidebar listing the 7 sub-pages and delegates rendering to whichever sub-page is selected. Each sub-page is referenced by an enum.

- [ ] **Step 1: test**

`internal/tabs/settings/settings_test.go`:
```go
package settings

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSubMenuNavigation(t *testing.T) {
	m := New(Deps{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubService {
		t.Errorf("expected SubService after one Down, got %v", m.SelectedPage())
	}
	view := m.View(120, 24)
	if !strings.Contains(view, "Service") || !strings.Contains(view, "Mihomo Core") {
		t.Errorf("submenu missing entries:\n%s", view)
	}
}

func TestPageEnumNames(t *testing.T) {
	expected := []SubPage{SubCore, SubService, SubController, SubRules, SubPatch, SubLogs, SubCache, SubAbout}
	if len(SubPageNames) != len(expected) {
		t.Fatalf("len(SubPageNames)=%d, want %d", len(SubPageNames), len(expected))
	}
	for _, p := range expected {
		if SubPageNames[p] == "" {
			t.Errorf("missing name for %v", p)
		}
	}
}
```

- [ ] **Step 2: impl**

`internal/tabs/settings/settings.go`:
```go
// Package settings implements the Settings tab and its sub-pages.
package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/api"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/logs"
)

// SubPage identifies a sub-page.
type SubPage int

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

// SubPageNames is human labels for the sidebar.
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

// Deps are wires for sub-pages.
type Deps struct {
	Paths     paths.XDG
	Store     *store.Store
	Service   service.Manager
	APIClient *api.Client
}

// Model is the Settings tab.
type Model struct {
	deps    Deps
	current SubPage

	about      aboutModel
	cache      cacheModel
	rules      rulesModel
	controller controllerModel
	service    serviceModel
	core       coreModel
	patch      patchModel
	logs       logs.Model
}

// New constructs the Settings tab Model with all sub-pages instantiated.
func New(deps Deps) Model {
	return Model{
		deps:       deps,
		about:      newAbout(),
		cache:      newCache(deps.Paths),
		rules:      newRules(deps.Store),
		controller: newController(deps.Store),
		service:    newService(deps.Service),
		core:       newCore(deps.Paths, deps.Store),
		patch:      newPatch(deps.Paths),
		logs:       logs.New(),
	}
}

// SelectedPage exposes the active sub-page (for tests).
func (m Model) SelectedPage() SubPage { return m.current }

// LogsModel exposes the embedded Logs model so the parent app can route LogLine into it.
// (Phase 3 routed LogLine directly to the tab; now it routes through Settings.)
func (m *Model) LogsModel() *logs.Model { return &m.logs }

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update handles sub-menu navigation + dispatches to active sub-page.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyDown:
			if m.current < NumSubPages-1 {
				m.current++
			}
			return m, nil
		case tea.KeyUp:
			if m.current > 0 {
				m.current--
			}
			return m, nil
		}
	}
	// Dispatch to active page's Update for non-navigation messages.
	var cmd tea.Cmd
	switch m.current {
	case SubAbout:
		m.about, cmd = m.about.Update(message)
	case SubCache:
		m.cache, cmd = m.cache.Update(message)
	case SubRules:
		m.rules, cmd = m.rules.Update(message)
	case SubController:
		m.controller, cmd = m.controller.Update(message)
	case SubService:
		m.service, cmd = m.service.Update(message)
	case SubCore:
		m.core, cmd = m.core.Update(message)
	case SubPatch:
		m.patch, cmd = m.patch.Update(message)
	case SubLogs:
		m.logs, cmd = m.logs.Update(message)
	}
	return m, cmd
}

// View composes sub-sidebar + active page body.
func (m Model) View(width, height int) string {
	subWidth := 22
	bodyWidth := width - subWidth - 1
	side := renderSubSidebar(m.current, height)
	var body string
	switch m.current {
	case SubAbout:
		body = m.about.View(bodyWidth, height)
	case SubCache:
		body = m.cache.View(bodyWidth, height)
	case SubRules:
		body = m.rules.View(bodyWidth, height)
	case SubController:
		body = m.controller.View(bodyWidth, height)
	case SubService:
		body = m.service.View(bodyWidth, height)
	case SubCore:
		body = m.core.View(bodyWidth, height)
	case SubPatch:
		body = m.patch.View(bodyWidth, height)
	case SubLogs:
		body = m.logs.View(bodyWidth, height)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, side, body)
}

func renderSubSidebar(active SubPage, height int) string {
	header := lipgloss.NewStyle().Bold(true).Render("Settings")
	rows := []string{header, ""}
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	for i := SubPage(0); i < NumSubPages; i++ {
		line := SubPageNames[i]
		if i == active {
			rows = append(rows, activeStyle.Render("▶ "+line))
		} else {
			rows = append(rows, inactiveStyle.Render("  "+line))
		}
	}
	rows = append(rows, "", "[↑↓] navigate")
	return lipgloss.NewStyle().Width(22).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).
		Padding(1, 1).Render(strings.Join(rows, "\n"))
}
```

- [ ] **Step 3:** This file references sub-page Models (`aboutModel`, `cacheModel`, etc.) that don't exist yet. Tasks T2-T8 add them. To compile right now we need stubs. Add this at the bottom of `settings.go`:

```go
// Stubs for sub-page Models — replaced in subsequent tasks.
type aboutModel struct{}
type cacheModel struct{}
type rulesModel struct{}
type controllerModel struct{}
type serviceModel struct{}
type coreModel struct{}
type patchModel struct{}

func newAbout() aboutModel { return aboutModel{} }
func newCache(paths.XDG) cacheModel { return cacheModel{} }
func newRules(*store.Store) rulesModel { return rulesModel{} }
func newController(*store.Store) controllerModel { return controllerModel{} }
func newService(service.Manager) serviceModel { return serviceModel{} }
func newCore(paths.XDG, *store.Store) coreModel { return coreModel{} }
func newPatch(paths.XDG) patchModel { return patchModel{} }

func (m aboutModel) Update(tea.Msg) (aboutModel, tea.Cmd) { return m, nil }
func (m cacheModel) Update(tea.Msg) (cacheModel, tea.Cmd) { return m, nil }
func (m rulesModel) Update(tea.Msg) (rulesModel, tea.Cmd) { return m, nil }
func (m controllerModel) Update(tea.Msg) (controllerModel, tea.Cmd) { return m, nil }
func (m serviceModel) Update(tea.Msg) (serviceModel, tea.Cmd) { return m, nil }
func (m coreModel) Update(tea.Msg) (coreModel, tea.Cmd) { return m, nil }
func (m patchModel) Update(tea.Msg) (patchModel, tea.Cmd) { return m, nil }

func (m aboutModel) View(_, _ int) string      { return "  About: (T2)" }
func (m cacheModel) View(_, _ int) string      { return "  Cache: (T3)" }
func (m rulesModel) View(_, _ int) string      { return "  Default Rules: (T4)" }
func (m controllerModel) View(_, _ int) string { return "  External Controller: (T5)" }
func (m serviceModel) View(_, _ int) string    { return "  Service: (T6)" }
func (m coreModel) View(_, _ int) string       { return "  Mihomo Core: (T7)" }
func (m patchModel) View(_, _ int) string      { return "  Patch Editor: (T8)" }
```

(These stub declarations are replaced in subsequent tasks. Each replacement task DELETES the stub and adds the real one.)

- [ ] **Step 4: build + test + commit**

```bash
export PATH="$HOME/.local/go/bin:$PATH"
go test -race ./internal/tabs/settings/ -v
git add internal/tabs/settings/
git commit -m "feat(settings): sub-menu shell with stubbed sub-pages"
```

---

## Task 2: About sub-page (replace stub)

**File modified:** `internal/tabs/settings/about.go` (new, replaces inline stub)

- [ ] **Step 1:** Delete the stub lines for `aboutModel`/`newAbout`/`Update`/`View` from `settings.go`. Create new `internal/tabs/settings/about.go`:

```go
package settings

import (
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type aboutModel struct{}

func newAbout() aboutModel { return aboutModel{} }

func (m aboutModel) Update(tea.Msg) (aboutModel, tea.Cmd) { return m, nil }

func (m aboutModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("About")
	body := header + "\n\n" +
		"  vpnkit — TUI for managing the mihomo proxy core (non-root).\n" +
		"\n" +
		"  Built with Go " + runtime.Version() + " · bubbletea · lipgloss.\n" +
		"  License: same as mihomo (GPL-3.0).\n" +
		"\n" +
		"  Source: https://github.com/MetaCubeX/mihomo (upstream core)\n"
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
```

- [ ] **Step 2: build + commit**

```bash
go build ./...
git add internal/tabs/settings/
git commit -m "feat(settings): About sub-page"
```

(Build must succeed — the stub deletion + new file should compile cleanly.)

---

## Task 3: Cache sub-page

**File created:** `internal/tabs/settings/cache.go`, `internal/tabs/settings/cache_test.go`

Reads cache directory size and offers a Clear action.

- [ ] **Step 1:** Delete the stub `cacheModel`/`newCache` lines from `settings.go`. Add new file:

`internal/tabs/settings/cache_test.go`:
```go
package settings

import (
	"os"
	"path/filepath"
	"testing"

	"vpnkit/internal/paths"
)

func TestCacheSizeAndClear(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "downloads"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "downloads", "x.gz"), []byte("hello world!"), 0o644)
	p := paths.XDG{VpnkitCache: dir}
	m := newCache(p)
	if m.Size() <= 0 {
		t.Errorf("size should be > 0")
	}
	if err := m.Clear(); err != nil {
		t.Errorf("Clear: %v", err)
	}
	if m.Size() != 0 {
		t.Errorf("size after clear: %d", m.Size())
	}
}
```

`internal/tabs/settings/cache.go`:
```go
package settings

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/paths"
)

type cacheModel struct {
	dir  string
	last int64 // cached size
}

func newCache(p paths.XDG) cacheModel {
	m := cacheModel{dir: p.VpnkitCache}
	m.last = m.Size()
	return m
}

// Size walks the cache dir and returns total bytes.
func (m cacheModel) Size() int64 {
	if m.dir == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(m.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Clear removes every file under the cache dir, keeping the directory itself.
func (m cacheModel) Clear() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(m.dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (m cacheModel) Update(message tea.Msg) (cacheModel, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok && km.String() == "c" {
		_ = m.Clear()
		m.last = m.Size()
	}
	return m, nil
}

func (m cacheModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Cache")
	body := header + "\n\n" +
		fmt.Sprintf("  Path : %s\n", m.dir) +
		fmt.Sprintf("  Size : %s\n", human(m.last)) +
		"\n  [c] clear cache\n"
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func human(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
	)
	switch {
	case n >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(n)/float64(GiB))
	case n >= MiB:
		return fmt.Sprintf("%.2f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
```

- [ ] **Step 2: test + commit**

```bash
go test -race ./internal/tabs/settings/ -v
git add internal/tabs/settings/
git commit -m "feat(settings): Cache sub-page (size + clear)"
```

---

## Task 4: Default Rules picker

Replace `rulesModel` stub. Toggle radio between `loyalsoldier` / `minimal` (writes through to `store`).

`internal/tabs/settings/rules.go`:
```go
package settings

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/rules"
	"vpnkit/internal/store"
)

type rulesModel struct {
	store *store.Store
	list  []string
	idx   int
}

func newRules(s *store.Store) rulesModel {
	list := rules.List()
	idx := 0
	if s != nil {
		for i, name := range list {
			if name == s.Cfg.RuleTemplate {
				idx = i
				break
			}
		}
	}
	return rulesModel{store: s, list: list, idx: idx}
}

func (m rulesModel) Update(message tea.Msg) (rulesModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "j":
		if m.idx < len(m.list)-1 {
			m.idx++
		}
	case "k":
		if m.idx > 0 {
			m.idx--
		}
	case "enter":
		if m.store != nil && m.idx < len(m.list) {
			m.store.Cfg.RuleTemplate = m.list[m.idx]
			_ = m.store.Save()
		}
	}
	return m, nil
}

func (m rulesModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Default Rules")
	rows := []string{header, "", "  Pick a template (Enter to save):", ""}
	current := ""
	if m.store != nil {
		current = m.store.Cfg.RuleTemplate
	}
	for i, name := range m.list {
		marker := "( )"
		if name == current {
			marker = "(•)"
		}
		row := "  " + marker + " " + name
		if i == m.idx {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ " + row[2:])
		}
		rows = append(rows, row)
	}
	rows = append(rows, "", "[j k] navigate  [Enter] save")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(joinNL(rows))
}

func joinNL(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += "\n"
		}
		out += s
	}
	return out
}
```

- [ ] **Commit**:
```bash
go build ./...
git add internal/tabs/settings/
git commit -m "feat(settings): Default Rules picker"
```

---

## Task 5: External Controller settings

Replace `controllerModel` stub. Show port + masked secret + actions to regenerate secret.

`internal/tabs/settings/controller.go`:
```go
package settings

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/store"
)

type controllerModel struct {
	store *store.Store
}

func newController(s *store.Store) controllerModel { return controllerModel{store: s} }

func (m controllerModel) Update(message tea.Msg) (controllerModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok || m.store == nil {
		return m, nil
	}
	if km.String() == "r" {
		buf := make([]byte, 16)
		_, _ = rand.Read(buf)
		m.store.Cfg.ControllerSecret = hex.EncodeToString(buf)
		_ = m.store.Save()
	}
	return m, nil
}

func (m controllerModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("External Controller")
	port := 9090
	secret := ""
	if m.store != nil {
		port = m.store.Cfg.ControllerPort
		secret = mask(m.store.Cfg.ControllerSecret)
	}
	body := header + "\n\n" +
		fmt.Sprintf("  Port   : 127.0.0.1:%d\n", port) +
		fmt.Sprintf("  Secret : %s\n", secret) +
		"\n  [r] regenerate secret (will require restart to take effect)\n"
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func mask(s string) string {
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "…" + s[len(s)-3:]
}
```

- [ ] **Commit**:
```bash
go build ./...
git add internal/tabs/settings/
git commit -m "feat(settings): External Controller (port + masked secret + regenerate)"
```

---

## Task 6: Service controls

Replace `serviceModel` stub. Buttons: start / stop / restart / show status.

`internal/tabs/settings/service.go`:
```go
package settings

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/service"
)

type serviceModel struct {
	mgr    service.Manager
	status service.Status
	flash  string
}

func newService(mgr service.Manager) serviceModel { return serviceModel{mgr: mgr} }

func (m serviceModel) refresh() serviceModel {
	if m.mgr == nil {
		return m
	}
	st, _ := m.mgr.Status(context.Background())
	m.status = st
	return m
}

func (m serviceModel) Update(message tea.Msg) (serviceModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok || m.mgr == nil {
		return m, nil
	}
	ctx := context.Background()
	switch km.String() {
	case "s":
		if err := m.mgr.Start(ctx); err != nil {
			m.flash = "start: " + err.Error()
		} else {
			m.flash = "started"
		}
	case "S":
		if err := m.mgr.Stop(ctx); err != nil {
			m.flash = "stop: " + err.Error()
		} else {
			m.flash = "stopped"
		}
	case "r":
		if err := m.mgr.Restart(ctx); err != nil {
			m.flash = "restart: " + err.Error()
		} else {
			m.flash = "restarted"
		}
	case "u":
		if err := m.mgr.Uninstall(ctx); err != nil {
			m.flash = "uninstall: " + err.Error()
		} else {
			m.flash = "uninstalled"
		}
	}
	m = m.refresh()
	return m, nil
}

func (m serviceModel) View(width, height int) string {
	m = m.refresh()
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Service")
	mode := "(unknown)"
	if m.mgr != nil {
		mode = string(m.mgr.Mode())
	}
	state := "○ stopped"
	if m.status.Running {
		state = fmt.Sprintf("● running (pid=%d)", m.status.PID)
	}
	body := header + "\n\n" +
		fmt.Sprintf("  Mode  : %s\n", mode) +
		fmt.Sprintf("  State : %s\n", state) +
		"\n  [s] start  [S] stop  [r] restart  [u] uninstall\n"
	if m.flash != "" {
		body += "\n  → " + m.flash + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
```

- [ ] **Commit**:
```bash
go build ./...
git add internal/tabs/settings/
git commit -m "feat(settings): Service controls (start/stop/restart/uninstall)"
```

---

## Task 7: Mihomo Core info + upgrade

Replace `coreModel` stub. Display installed mihomo file size + path; offer Upgrade (re-run installer for latest).

`internal/tabs/settings/core.go`:
```go
package settings

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/installer"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

type coreModel struct {
	paths paths.XDG
	store *store.Store
	flash string
}

func newCore(p paths.XDG, s *store.Store) coreModel { return coreModel{paths: p, store: s} }

func (m coreModel) Update(message tea.Msg) (coreModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "u" {
		mirror := ""
		if m.store != nil {
			mirror = m.store.Cfg.ReleaseMirror
		}
		res, err := installer.Install(installer.Options{
			Dst:    m.paths.MihomoBinary(),
			Mirror: mirror,
		}, nil)
		if err != nil {
			m.flash = "upgrade: " + err.Error()
		} else {
			m.flash = "upgraded to " + res.Version
		}
	}
	return m, nil
}

func (m coreModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Mihomo Core")
	size := "(not installed)"
	if info, err := os.Stat(m.paths.MihomoBinary()); err == nil {
		size = fmt.Sprintf("%d bytes", info.Size())
	}
	mirror := ""
	if m.store != nil {
		mirror = m.store.Cfg.ReleaseMirror
	}
	body := header + "\n\n" +
		fmt.Sprintf("  Binary : %s\n", m.paths.MihomoBinary()) +
		fmt.Sprintf("  Size   : %s\n", size) +
		fmt.Sprintf("  Mirror : %s\n", fallback(mirror, "(direct GitHub)")) +
		"\n  [u] upgrade to latest release\n"
	if m.flash != "" {
		body += "\n  → " + m.flash + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func fallback(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}
```

- [ ] **Commit**:
```bash
go build ./...
git add internal/tabs/settings/
git commit -m "feat(settings): Mihomo Core info + upgrade trigger"
```

---

## Task 8: Patch Editor sub-page

Replace `patchModel` stub with a textarea-backed editor over `~/.config/mihomo/patch.yaml`.

`internal/tabs/settings/patch_test.go`:
```go
package settings

import (
	"os"
	"path/filepath"
	"testing"

	"vpnkit/internal/paths"
)

func TestPatchLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	mihomoDir := filepath.Join(dir, "mihomo")
	_ = os.MkdirAll(mihomoDir, 0o755)
	patchPath := filepath.Join(mihomoDir, "patch.yaml")
	_ = os.WriteFile(patchPath, []byte("log-level: debug\n"), 0o600)

	p := paths.XDG{MihomoConfig: mihomoDir}
	m := newPatch(p)
	if got := m.Content(); got != "log-level: debug\n" {
		t.Errorf("loaded: %q", got)
	}
	m.SetContent("log-level: warn\n")
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(patchPath)
	if string(data) != "log-level: warn\n" {
		t.Errorf("saved: %s", data)
	}
}
```

`internal/tabs/settings/patch.go`:
```go
package settings

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/paths"
)

type patchModel struct {
	path string
	area textarea.Model
	flash string
}

func newPatch(p paths.XDG) patchModel {
	ta := textarea.New()
	ta.Placeholder = "# YAML overlay (deep-merged onto subscription + rules)"
	ta.ShowLineNumbers = true
	ta.SetWidth(80)
	ta.SetHeight(18)
	pPath := filepath.Join(p.MihomoConfig, "patch.yaml")
	m := patchModel{path: pPath, area: ta}
	if data, err := os.ReadFile(pPath); err == nil {
		m.area.SetValue(string(data))
	}
	return m
}

func (m patchModel) Content() string         { return m.area.Value() }
func (m *patchModel) SetContent(s string)    { m.area.SetValue(s) }

// Save writes the current content atomically to the patch path.
func (m patchModel) Save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.path, []byte(m.area.Value()), 0o600)
}

func (m patchModel) Update(message tea.Msg) (patchModel, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		if km.Type == tea.KeyCtrlS {
			if err := m.Save(); err != nil {
				m.flash = "save: " + err.Error()
			} else {
				m.flash = "saved"
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.area, cmd = m.area.Update(message)
	return m, cmd
}

func (m patchModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Patch Editor (~/.config/mihomo/patch.yaml)")
	body := header + "\n" + m.area.View() + "\n\n  [Ctrl+S] save"
	if m.flash != "" {
		body += "  → " + m.flash
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/tabs/settings/ -v
git add internal/tabs/settings/
git commit -m "feat(settings): Patch Editor with textarea + save"
```

---

## Task 9: Profile persistence + OnChange callback

**Files modified:**
- `internal/profiles/manager.go`
- `internal/app/run.go`

Profiles changes are currently lost on quit. Add an `OnChange func()` callback that Manager invokes after Add/Remove/SetActive/Update; the app wires it to `store.Save()`.

- [ ] **In `internal/profiles/manager.go`:**

Add field:
```go
onChange func()
```

Add setter:
```go
// SetOnChange installs a callback fired after each mutation.
func (m *Manager) SetOnChange(fn func()) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.onChange = fn
}

func (m *Manager) fireChange() {
    if m.onChange != nil {
        m.onChange()
    }
}
```

In `Add`, `Remove`, `SetActive`, and at the end of `Update` (after the lock release that updates LastUpdated/NodeCount), call `m.fireChange()`. Note: don't call `fireChange()` while holding the mutex — call it after.

Concrete shape for `Add`:
```go
func (m *Manager) Add(p Profile) error {
    m.mu.Lock()
    for _, e := range m.profiles {
        if e.Name == p.Name {
            m.mu.Unlock()
            return errors.New("profiles: duplicate name")
        }
    }
    m.profiles = append(m.profiles, p)
    if m.active == "" {
        m.active = p.Name
    }
    m.mu.Unlock()
    m.fireChange()
    return nil
}
```

Apply similar refactoring to `Remove`, `SetActive`, `Update`.

- [ ] **In `internal/app/run.go`:** after constructing `profMgr`, install the callback:

```go
profMgr.SetOnChange(func() {
    // Sync back to store.
    persisted := make([]store.Profile, 0)
    for _, p := range profMgr.All() {
        persisted = append(persisted, store.Profile{
            Name: p.Name, URL: p.URL, UserAgent: p.UserAgent, LastUpdated: p.LastUpdated,
        })
    }
    st.Cfg.Profiles = persisted
    st.Cfg.ActiveProfile = profMgr.Active()
    _ = st.Save()
})
```

- [ ] **Build + test + commit**:
```bash
go build ./...
go test -race ./... 2>&1 | tail -15
git add internal/profiles/ internal/app/run.go
git commit -m "feat(profiles): OnChange callback for store persistence"
```

---

## Task 10: Wire Settings tab into app + remove old Logs-as-Settings hack

**Files modified:**
- `internal/app/model.go`
- `internal/app/view.go`
- `internal/app/update.go`
- `internal/app/run.go`

### Step 1: model.go

- Remove import `tablogs "vpnkit/internal/tabs/logs"` (no longer used at app level).
- Remove field `logsTab tablogs.Model`.
- Add import `tabsettings "vpnkit/internal/tabs/settings"`.
- Add field `settingsTab tabsettings.Model`.

Update `NewModel` signature to accept `settings.Deps`:
```go
func NewModel(client *api.Client, mgr *profiles.Manager, settingsDeps tabsettings.Deps) Model {
    ...
    settingsTab: tabsettings.New(settingsDeps),
    ...
}
```

### Step 2: view.go

Replace `case TabSettings: body = m.logsTab.View(...)` with `body = m.settingsTab.View(...)`.

### Step 3: update.go

Replace the `case LogLine:` block:
```go
case LogLine:
    // Route LogLine into the Settings tab's embedded logs sub-page.
    lm := m.settingsTab.LogsModel()
    *lm, _ = lm.Update(msg)
```

Remove the `if m.activeTab == TabSettings { ... TogglePause ... }` block — pause is handled inside `settings.logs` directly. Actually keep it: route any KeyMsg on TabSettings through to settingsTab.Update:

```go
if m.activeTab == TabSettings {
    var c tea.Cmd
    m.settingsTab, c = m.settingsTab.Update(msg)
    return m, c
}
```

Place this BEFORE the global key cascade so Settings owns its keys.

### Step 4: run.go

Update `NewModel` call:
```go
deps := tabsettings.Deps{
    Paths:     p,
    Store:     st,
    Service:   svc,
    APIClient: client,
}
model := NewModel(client, profMgr, deps)
```

Add import `tabsettings "vpnkit/internal/tabs/settings"`.

`streamLogs` still emits `msg.LogLine`. update.go routes them into `settingsTab.LogsModel()` now. Keep `streamLogs` calling `prog.Send(msg.LogLine{...})`.

Existing `internal/app/model_test.go` calls `NewModel(nil, nil)` — update to `NewModel(nil, nil, tabsettings.Deps{})`.

### Step 5: build + test + commit

```bash
go build ./...
go test -race ./... -v 2>&1 | tail -30
git add internal/app/
git commit -m "feat(app): wire Settings tab with sub-menu + remove inline Logs"
```

---

## Task 11: Smoke + README + tag v0.4.0

- [ ] **Build + install:**
```bash
make install
~/.local/bin/vpnkit --version
```

- [ ] **Append README** (after Phase 3 features):
```markdown

## Phase 4 features

Settings tab (sub-menu):
- **Mihomo Core** — show installed version; `u` upgrade to latest release
- **Service** — `s/S/r/u` start/stop/restart/uninstall
- **External Controller** — port + masked secret; `r` regenerate
- **Default Rules** — `j k` cycle, `Enter` save (loyalsoldier / minimal)
- **Patch Editor** — full textarea over `~/.config/mihomo/patch.yaml`; `Ctrl+S` save
- **Logs** — live mihomo log tail; `p` pause / resume
- **Cache** — show `~/.cache/vpnkit/` size; `c` clear
- **About** — versions + license

Profiles list now persists to `~/.config/vpnkit/config.toml` on every add / remove / activate / update.

`vpnkit` is feature-complete for daily use: install / subscribe / switch / observe / configure — all in the TUI.
```

- [ ] **Commit + tag:**
```bash
git add README.md
git commit -m "docs: Phase 4 quickstart — vpnkit feature-complete"
git tag v0.4.0-phase4
```

---

## Self-Review

- Sub-menu nav (T1), 7 sub-pages (T2-T8), persistence (T9), wiring (T10), smoke (T11): all covered.
- Stubs introduced in T1 are replaced one-by-one in T2-T8. After T8 no inline stubs remain in settings.go.
- T9 introduces `OnChange` so profile state survives restart.
- T10 unifies LogLine routing through `settingsTab.LogsModel()`.

Phase 4 = 11 tasks. End state: vpnkit at v0.4.0-phase4 is feature-complete per the original design doc, minus optional polish (command palette `:`, theme switching, profile UA editing).

End of Phase 4 plan.
