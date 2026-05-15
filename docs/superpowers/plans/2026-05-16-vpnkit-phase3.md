# vpnkit Phase 3 Implementation Plan — Connections / Rules / Logs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Wire up the remaining real-time/observation surfaces — connections WebSocket stream, rules listing + rule-provider refresh, and a Logs sub-page inside Settings that tails `journalctl --user -u mihomo` (or the PID-mode log file).

**Architecture:** Three new packages — `internal/tabs/connections`, `internal/tabs/rules`, `internal/tabs/logs`. Two new API client methods — `Connections` (WebSocket) and `Rules` (REST). Logs reuses `service.Manager.Logs(ctx, follow)` from Phase 1. Wired into the existing app shell.

**Tech Stack:** Same as before. `github.com/coder/websocket` is already in `go.mod`. No new third-party deps.

**Spec reference:** `docs/superpowers/specs/2026-05-15-vpnkit-tui-design.md` §4.5 (tabs), §8 (REST API). The Logs viewer is described as a Settings sub-page; for Phase 3 we make it accessible from a "Logs" 7th sub-context via the existing Settings tab.

---

## File Map

| Path | Responsibility |
|---|---|
| `internal/api/connections.go` | WebSocket consumer of `/connections` streaming snapshots |
| `internal/api/connections_test.go` | Unit test against `httptest` WS server |
| `internal/api/rules.go` | REST: `GET /rules`, `GET /providers/rules`, `PUT /providers/rules/{name}` |
| `internal/api/rules_test.go` | |
| `internal/msg/msg.go` (MODIFY) | Add `ConnectionsSnapshot`, `RulesSnapshot`, `LogLine` |
| `internal/tabs/connections/connections.go` | Connections tab Model (real-time table) |
| `internal/tabs/connections/connections_test.go` | |
| `internal/tabs/rules/rules.go` | Rules tab Model (list + provider status) |
| `internal/tabs/rules/rules_test.go` | |
| `internal/tabs/logs/logs.go` | Logs viewer Model (scrollback + tail) |
| `internal/tabs/logs/logs_test.go` | |
| `internal/app/model.go` (MODIFY) | Add tab models + connections cmd flag |
| `internal/app/view.go` (MODIFY) | Render new tabs |
| `internal/app/update.go` (MODIFY) | Route messages + tab-specific keys |
| `internal/app/run.go` (MODIFY) | Start `streamConnections` + `pollRules` goroutines |
| `internal/app/messages.go` (MODIFY) | Add tab-message aliases |
| `internal/app/keys.go` (MODIFY) | (No new bindings — reuse existing) |

---

## Task 1: API WebSocket connections client

**Files:**
- Create: `internal/api/connections.go`
- Create: `internal/api/connections_test.go`

- [ ] **Step 1: Write failing test**

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestConnectionsStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "bye")
		payload, _ := json.Marshal(map[string]any{
			"downloadTotal": 1024,
			"uploadTotal":   2048,
			"connections": []map[string]any{
				{"id": "abc", "metadata": map[string]any{"host": "example.com", "destinationPort": "443"}, "rule": "Match", "chains": []string{"🚀 Proxy"}, "upload": 100, "download": 200},
			},
		})
		_ = c.Write(r.Context(), websocket.MessageText, payload)
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(strings.Replace(srv.URL, "http://", "http://", 1), "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, errCh := c.Connections(ctx)
	select {
	case snap := <-ch:
		if snap.DownloadTotal != 1024 || len(snap.Connections) != 1 {
			t.Errorf("snap: %+v", snap)
		}
		if snap.Connections[0].Host != "example.com" {
			t.Errorf("host: %s", snap.Connections[0].Host)
		}
	case err := <-errCh:
		t.Fatalf("err: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}
```

- [ ] **Step 2: Implementation**

```go
package api

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/coder/websocket"
)

// ConnectionsSnapshot is one /connections tick.
type ConnectionsSnapshot struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
}

// Connection is one active connection entry.
type Connection struct {
	ID       string
	Host     string
	Port     string
	Network  string
	Type     string
	Rule     string
	Chains   []string
	Upload   int64
	Download int64
}

type rawConnection struct {
	ID       string          `json:"id"`
	Metadata json.RawMessage `json:"metadata"`
	Rule     string          `json:"rule"`
	Chains   []string        `json:"chains"`
	Upload   int64           `json:"upload"`
	Download int64           `json:"download"`
}

type rawConnsResp struct {
	DownloadTotal int64           `json:"downloadTotal"`
	UploadTotal   int64           `json:"uploadTotal"`
	Connections   []rawConnection `json:"connections"`
}

type rawMetadata struct {
	Host    string `json:"host"`
	DstIP   string `json:"destinationIP"`
	DstPort string `json:"destinationPort"`
	Network string `json:"network"`
	Type    string `json:"type"`
}

// Connections opens a WebSocket to /connections and emits snapshots until ctx cancels.
func (c *Client) Connections(ctx context.Context) (<-chan ConnectionsSnapshot, <-chan error) {
	out := make(chan ConnectionsSnapshot, 8)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		wsURL := strings.Replace(c.BaseURL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		header := map[string][]string{}
		if c.Secret != "" {
			header["Authorization"] = []string{"Bearer " + c.Secret}
		}
		conn, _, err := websocket.Dial(ctx, wsURL+"/connections", &websocket.DialOptions{
			HTTPHeader: header,
		})
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				if ctx.Err() == nil {
					errCh <- err
				}
				return
			}
			var raw rawConnsResp
			if err := json.Unmarshal(data, &raw); err != nil {
				continue
			}
			snap := ConnectionsSnapshot{
				DownloadTotal: raw.DownloadTotal,
				UploadTotal:   raw.UploadTotal,
			}
			for _, rc := range raw.Connections {
				var meta rawMetadata
				_ = json.Unmarshal(rc.Metadata, &meta)
				host := meta.Host
				if host == "" {
					host = meta.DstIP
				}
				snap.Connections = append(snap.Connections, Connection{
					ID:       rc.ID,
					Host:     host,
					Port:     meta.DstPort,
					Network:  meta.Network,
					Type:     meta.Type,
					Rule:     rc.Rule,
					Chains:   rc.Chains,
					Upload:   rc.Upload,
					Download: rc.Download,
				})
			}
			select {
			case out <- snap:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errCh
}

// CloseConnection sends DELETE /connections/{id} to mihomo.
func (c *Client) CloseConnection(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/connections/"+id, nil, nil)
}
```

- [ ] **Step 3: Test + commit**

```bash
export PATH="$HOME/.local/go/bin:$PATH"
go test -race ./internal/api/ -v
git add internal/api/connections.go internal/api/connections_test.go
git commit -m "feat(api): /connections WebSocket consumer + close"
```

---

## Task 2: API Rules + rule-providers

- [ ] **Test** `internal/api/rules_test.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRules(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rules": []map[string]any{
				{"type": "RULE-SET", "payload": "reject", "proxy": "🛑 Reject"},
				{"type": "MATCH", "payload": "", "proxy": "🚀 Proxy"},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	rs, err := c.GetRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 || rs[0].Type != "RULE-SET" {
		t.Errorf("got %+v", rs)
	}
}

func TestGetRuleProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"providers": map[string]any{
				"reject": map[string]any{"name": "reject", "behavior": "Domain", "ruleCount": 1234, "updatedAt": "2026-05-15T20:00:00Z"},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	ps, err := c.GetRuleProviders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	p, ok := ps["reject"]
	if !ok || p.RuleCount != 1234 {
		t.Errorf("provider: %+v", p)
	}
}

func TestRefreshRuleProvider(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/providers/rules/reject" {
			called = true
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.RefreshRuleProvider(context.Background(), "reject"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("PUT not seen")
	}
}
```

- [ ] **Impl** `internal/api/rules.go`:
```go
package api

import (
	"context"
	"net/http"
	"net/url"
)

// Rule is one mihomo rule entry.
type Rule struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

type rulesResp struct {
	Rules []Rule `json:"rules"`
}

// GetRules fetches /rules.
func (c *Client) GetRules(ctx context.Context) ([]Rule, error) {
	var r rulesResp
	if err := c.do(ctx, http.MethodGet, "/rules", nil, &r); err != nil {
		return nil, err
	}
	return r.Rules, nil
}

// RuleProvider mirrors one entry in /providers/rules.
type RuleProvider struct {
	Name      string `json:"name"`
	Behavior  string `json:"behavior"`
	RuleCount int    `json:"ruleCount"`
	UpdatedAt string `json:"updatedAt"`
	Type      string `json:"type"`
	VehicleType string `json:"vehicleType"`
}

type ruleProvidersResp struct {
	Providers map[string]RuleProvider `json:"providers"`
}

// GetRuleProviders fetches /providers/rules.
func (c *Client) GetRuleProviders(ctx context.Context) (map[string]RuleProvider, error) {
	var r ruleProvidersResp
	if err := c.do(ctx, http.MethodGet, "/providers/rules", nil, &r); err != nil {
		return nil, err
	}
	return r.Providers, nil
}

// RefreshRuleProvider triggers a re-download for a single provider.
func (c *Client) RefreshRuleProvider(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodPut, "/providers/rules/"+url.PathEscape(name), nil, nil)
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/api/ -v
git add internal/api/rules.go internal/api/rules_test.go
git commit -m "feat(api): /rules + /providers/rules"
```

---

## Task 3: New msg types

Append to `internal/msg/msg.go`:

```go
// ConnectionsSnapshot wraps api's snapshot for tea routing.
type ConnectionsSnapshot struct {
	DownloadTotal int64
	UploadTotal   int64
	Items         []ConnectionItem
}

// ConnectionItem is a UI-friendly connection entry.
type ConnectionItem struct {
	ID       string
	Host     string
	Port     string
	Network  string
	Rule     string
	Chains   []string
	Upload   int64
	Download int64
}

// RulesSnapshot wraps the /rules + /providers state.
type RulesSnapshot struct {
	Rules     []RuleEntry
	Providers []RuleProviderEntry
}

// RuleEntry is a single rule for UI.
type RuleEntry struct {
	Type    string
	Payload string
	Proxy   string
}

// RuleProviderEntry is one rule-provider's state.
type RuleProviderEntry struct {
	Name      string
	Behavior  string
	RuleCount int
	UpdatedAt string
}

// LogLine is one log line streamed from the service manager.
type LogLine struct {
	Text string
}
```

- [ ] **Commit**:
```bash
go build ./...
git add internal/msg/msg.go
git commit -m "feat(msg): connections / rules / log line messages"
```

---

## Task 4: Connections tab Model

**Files:**
- Create: `internal/tabs/connections/connections.go`
- Create: `internal/tabs/connections/connections_test.go`

- [ ] **Test:**
```go
package connections

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestRendersConnections(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ConnectionsSnapshot{
		DownloadTotal: 1024,
		Items: []msg.ConnectionItem{
			{ID: "1", Host: "example.com", Port: "443", Rule: "Match", Upload: 100, Download: 200, Chains: []string{"🚀 Proxy"}},
			{ID: "2", Host: "google.com", Port: "443", Rule: "DOMAIN-SUFFIX", Upload: 50, Download: 80, Chains: []string{"DIRECT"}},
		},
	})
	view := m.View(120, 24)
	if !strings.Contains(view, "example.com") || !strings.Contains(view, "google.com") {
		t.Errorf("missing hosts:\n%s", view)
	}
}

func TestFilterHidesNonMatching(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ConnectionsSnapshot{Items: []msg.ConnectionItem{
		{ID: "1", Host: "alpha.example.com"},
		{ID: "2", Host: "beta.example.com"},
	}})
	m.SetFilter("alpha")
	view := m.View(120, 24)
	if !strings.Contains(view, "alpha") {
		t.Errorf("alpha missing")
	}
	if strings.Contains(view, "beta") {
		t.Errorf("beta should be filtered out:\n%s", view)
	}
}
```

- [ ] **Implementation:**
```go
// Package connections implements the Connections tab (real-time connection table).
package connections

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
)

// Model is the Connections tab.
type Model struct {
	items   []msg.ConnectionItem
	totalUp int64
	totalDn int64
	filter  string
	cursor  int
}

// New returns an empty tab model.
func New() Model { return Model{} }

func (Model) Init() tea.Cmd { return nil }

// Update absorbs ConnectionsSnapshot.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if ev, ok := message.(msg.ConnectionsSnapshot); ok {
		m.items = ev.Items
		m.totalUp = ev.UploadTotal
		m.totalDn = ev.DownloadTotal
		if m.cursor >= len(m.items) {
			m.cursor = 0
		}
	}
	return m, nil
}

// SetFilter changes the substring filter.
func (m *Model) SetFilter(s string) { m.filter = s }

// MoveUp / MoveDown navigate filtered rows.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}
func (m *Model) MoveDown() {
	visible := m.visible()
	if m.cursor < len(visible)-1 {
		m.cursor++
	}
}

// SelectedID returns the highlighted connection ID (empty if none).
func (m Model) SelectedID() string {
	visible := m.visible()
	if m.cursor >= len(visible) {
		return ""
	}
	return visible[m.cursor].ID
}

func (m Model) visible() []msg.ConnectionItem {
	if m.filter == "" {
		return m.items
	}
	var out []msg.ConnectionItem
	for _, it := range m.items {
		if strings.Contains(it.Host, m.filter) || strings.Contains(it.Rule, m.filter) {
			out = append(out, it)
		}
	}
	return out
}

// View renders the table.
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Connections")
	stats := fmt.Sprintf("  ↑ %s    ↓ %s    %d active", human(m.totalUp), human(m.totalDn), len(m.items))
	rows := []string{header, stats, ""}
	rows = append(rows, fmt.Sprintf("  %-30s  %-6s  %-12s  %-12s  %s", "HOST", "PORT", "UP", "DOWN", "RULE"))
	visible := m.visible()
	for i, it := range visible {
		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ ")
		}
		rows = append(rows, fmt.Sprintf("%s%-30s  %-6s  %-12s  %-12s  %s",
			prefix, truncate(it.Host, 30), it.Port, human(it.Upload), human(it.Download), it.Rule))
	}
	rows = append(rows, "", "[/] filter  [k] close selected  [↑↓] navigate")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func human(n int64) string {
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
```

- [ ] **Commit:**
```bash
go test -race ./internal/tabs/connections/ -v
git add internal/tabs/connections/
git commit -m "feat(tabs): connections tab with filter + cursor"
```

---

## Task 5: Rules tab Model

**Files:**
- Create: `internal/tabs/rules/rules.go`
- Create: `internal/tabs/rules/rules_test.go`

- [ ] **Test:**
```go
package rules

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestRulesRenders(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{
		Rules: []msg.RuleEntry{
			{Type: "RULE-SET", Payload: "reject", Proxy: "🛑 Reject"},
			{Type: "MATCH", Payload: "", Proxy: "🚀 Proxy"},
		},
		Providers: []msg.RuleProviderEntry{
			{Name: "reject", Behavior: "Domain", RuleCount: 1234, UpdatedAt: "2026-05-15T20:00:00Z"},
		},
	})
	view := m.View(120, 24)
	for _, want := range []string{"RULE-SET", "reject", "🚀 Proxy", "1234"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in:\n%s", want, view)
		}
	}
}

func TestRulesFilter(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{
		Rules: []msg.RuleEntry{
			{Type: "RULE-SET", Payload: "reject", Proxy: "R"},
			{Type: "RULE-SET", Payload: "google", Proxy: "P"},
		},
	})
	m.SetFilter("google")
	view := m.View(120, 24)
	if !strings.Contains(view, "google") || strings.Contains(view, "reject") {
		t.Errorf("filter broken:\n%s", view)
	}
}
```

- [ ] **Implementation:**
```go
// Package rules implements the Rules tab (rule list + providers status).
package rules

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
)

// Model is the Rules tab.
type Model struct {
	rules     []msg.RuleEntry
	providers []msg.RuleProviderEntry
	filter    string
}

func New() Model { return Model{} }

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if ev, ok := message.(msg.RulesSnapshot); ok {
		m.rules = ev.Rules
		m.providers = ev.Providers
	}
	return m, nil
}

func (m *Model) SetFilter(s string) { m.filter = s }

func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Rules")
	rows := []string{header, ""}

	if len(m.providers) > 0 {
		rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Rule Providers"))
		for _, p := range m.providers {
			rows = append(rows, fmt.Sprintf("  %-20s  %-8s  count=%d  updated=%s",
				p.Name, p.Behavior, p.RuleCount, p.UpdatedAt))
		}
		rows = append(rows, "")
	}

	rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Rules"))
	for _, r := range m.rules {
		if m.filter != "" && !strings.Contains(r.Payload, m.filter) && !strings.Contains(r.Type, m.filter) && !strings.Contains(r.Proxy, m.filter) {
			continue
		}
		rows = append(rows, fmt.Sprintf("  %-14s  %-30s  → %s", r.Type, truncate(r.Payload, 30), r.Proxy))
	}
	rows = append(rows, "", "[/] filter  [u] refresh providers")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
```

- [ ] **Commit:**
```bash
go test -race ./internal/tabs/rules/ -v
git add internal/tabs/rules/
git commit -m "feat(tabs): rules tab with provider status + filter"
```

---

## Task 6: Logs tab Model

This is technically a Settings sub-page per the spec, but for Phase 3 we replace the Settings stub with a Logs view directly (Settings sub-page menu lands in Phase 4).

**Files:**
- Create: `internal/tabs/logs/logs.go`
- Create: `internal/tabs/logs/logs_test.go`

- [ ] **Test:**
```go
package logs

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestLogsAppend(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.LogLine{Text: "starting mihomo"})
	m, _ = m.Update(msg.LogLine{Text: "listening 7890"})
	view := m.View(120, 24)
	if !strings.Contains(view, "starting") || !strings.Contains(view, "7890") {
		t.Errorf("view:\n%s", view)
	}
}

func TestLogsRingBound(t *testing.T) {
	m := New()
	for i := 0; i < 1500; i++ {
		m, _ = m.Update(msg.LogLine{Text: "x"})
	}
	if got := len(m.Lines()); got != 1000 {
		t.Errorf("expected ring of 1000, got %d", got)
	}
}
```

- [ ] **Implementation:**
```go
// Package logs implements the Logs viewer (tail of mihomo log).
package logs

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
)

const ringSize = 1000

// Model is the Logs tab.
type Model struct {
	lines  []string
	paused bool
}

func New() Model { return Model{} }

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if ev, ok := message.(msg.LogLine); ok && !m.paused {
		if len(m.lines) >= ringSize {
			m.lines = m.lines[1:]
		}
		m.lines = append(m.lines, ev.Text)
	}
	return m, nil
}

// Lines exposes the buffered lines for tests.
func (m Model) Lines() []string { return m.lines }

// TogglePause flips the pause flag.
func (m *Model) TogglePause() { m.paused = !m.paused }

// View renders the tail (most recent height-4 lines).
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Logs")
	pauseMark := ""
	if m.paused {
		pauseMark = " [PAUSED]"
	}
	rows := []string{header + pauseMark, ""}
	tailSize := height - 4
	if tailSize < 1 {
		tailSize = 1
	}
	start := 0
	if len(m.lines) > tailSize {
		start = len(m.lines) - tailSize
	}
	for _, l := range m.lines[start:] {
		rows = append(rows, "  "+l)
	}
	rows = append(rows, "", "[p] pause/resume")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}
```

- [ ] **Commit:**
```bash
go test -race ./internal/tabs/logs/ -v
git add internal/tabs/logs/
git commit -m "feat(tabs): logs viewer with 1000-line ring buffer"
```

---

## Task 7: Wire Connections / Rules / Logs into app

**Files modified:**
- `internal/app/model.go`
- `internal/app/view.go`
- `internal/app/update.go`
- `internal/app/run.go`
- `internal/app/messages.go`

### Step 1: `internal/app/messages.go`

Append to the `type (...)` block:
```go
ConnectionsSnapshot = msg.ConnectionsSnapshot
ConnectionItem      = msg.ConnectionItem
RulesSnapshot       = msg.RulesSnapshot
RuleEntry           = msg.RuleEntry
RuleProviderEntry   = msg.RuleProviderEntry
LogLine             = msg.LogLine
```

### Step 2: `internal/app/model.go`

Add imports:
```go
tabconnections "vpnkit/internal/tabs/connections"
tabrules       "vpnkit/internal/tabs/rules"
tablogs        "vpnkit/internal/tabs/logs"
```

Add fields:
```go
connectionsTab tabconnections.Model
rulesTab       tabrules.Model
logsTab        tablogs.Model
```

In `NewModel`, initialize:
```go
connectionsTab: tabconnections.New(),
rulesTab:       tabrules.New(),
logsTab:        tablogs.New(),
```

Add skip clauses in the stub initialization for `TabConnections`, `TabRules`, `TabSettings` (Settings now hosts logs as default sub-view).

### Step 3: `internal/app/view.go`

Add to the body switch:
```go
case TabConnections:
    body = m.connectionsTab.View(bodyWidth, bodyHeight)
case TabRules:
    body = m.rulesTab.View(bodyWidth, bodyHeight)
case TabSettings:
    body = m.logsTab.View(bodyWidth, bodyHeight)
```

### Step 4: `internal/app/update.go`

Route messages:
```go
case ConnectionsSnapshot:
    m.connectionsTab, cmd = m.connectionsTab.Update(msg)
case RulesSnapshot:
    m.rulesTab, cmd = m.rulesTab.Update(msg)
case LogLine:
    m.logsTab, cmd = m.logsTab.Update(msg)
```

Tab-specific keys (add before global cascade):
```go
if m.activeTab == TabConnections {
    switch v.String() {
    case "up", "k": m.connectionsTab.MoveUp(); return m, nil
    case "down", "j": m.connectionsTab.MoveDown(); return m, nil
    case "k":
        if id := m.connectionsTab.SelectedID(); id != "" && m.apiClient != nil {
            return m, func() tea.Msg { _ = m.apiClient.CloseConnection(context.Background(), id); return nil }
        }
        return m, nil
    }
}
if m.activeTab == TabSettings {
    switch v.String() {
    case "p":
        m.logsTab.TogglePause()
        return m, nil
    }
}
```

Note: the `k` key is overloaded (move up + close). Drop the `k` close binding and use `x` instead to avoid conflict:
```go
case "x":
    if id := m.connectionsTab.SelectedID(); id != "" && m.apiClient != nil {
        return m, func() tea.Msg { _ = m.apiClient.CloseConnection(context.Background(), id); return nil }
    }
    return m, nil
```

### Step 5: `internal/app/run.go`

Add three new goroutines:

```go
func streamConnections(prog *tea.Program, client *api.Client) {
    for {
        ctx, cancel := context.WithCancel(context.Background())
        ch, errCh := client.Connections(ctx)
    loop:
        for {
            select {
            case snap, ok := <-ch:
                if !ok {
                    break loop
                }
                items := make([]msg.ConnectionItem, 0, len(snap.Connections))
                for _, c := range snap.Connections {
                    items = append(items, msg.ConnectionItem{
                        ID: c.ID, Host: c.Host, Port: c.Port, Network: c.Network,
                        Rule: c.Rule, Chains: c.Chains, Upload: c.Upload, Download: c.Download,
                    })
                }
                prog.Send(msg.ConnectionsSnapshot{
                    DownloadTotal: snap.DownloadTotal, UploadTotal: snap.UploadTotal, Items: items,
                })
            case <-errCh:
                break loop
            }
        }
        cancel()
        time.Sleep(2 * time.Second)
    }
}

func pollRules(prog *tea.Program, client *api.Client) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        rules, errR := client.GetRules(ctx)
        providers, errP := client.GetRuleProviders(ctx)
        cancel()
        if errR == nil && errP == nil {
            snap := msg.RulesSnapshot{}
            for _, r := range rules {
                snap.Rules = append(snap.Rules, msg.RuleEntry{Type: r.Type, Payload: r.Payload, Proxy: r.Proxy})
            }
            for _, p := range providers {
                snap.Providers = append(snap.Providers, msg.RuleProviderEntry{
                    Name: p.Name, Behavior: p.Behavior, RuleCount: p.RuleCount, UpdatedAt: p.UpdatedAt,
                })
            }
            prog.Send(snap)
        }
        <-ticker.C
    }
}

func streamLogs(prog *tea.Program, svc service.Manager) {
    for {
        ctx, cancel := context.WithCancel(context.Background())
        reader, err := svc.Logs(ctx, true)
        if err != nil {
            cancel()
            time.Sleep(5 * time.Second)
            continue
        }
        scanner := bufio.NewScanner(reader)
        scanner.Buffer(make([]byte, 0, 4096), 1<<20)
        for scanner.Scan() {
            prog.Send(msg.LogLine{Text: scanner.Text()})
        }
        reader.Close()
        cancel()
        time.Sleep(2 * time.Second)
    }
}
```

Add imports: `"bufio"`, `"vpnkit/internal/service"`.

In `Run()`:
```go
go streamConnections(prog, client)
go pollRules(prog, client)
go streamLogs(prog, svc)
```

### Build + test + commit

```bash
go build ./...
go test -race ./... -v 2>&1 | tail -30
git add internal/app/
git commit -m "feat(app): wire connections / rules / logs tabs + live streams"
```

---

## Task 8: Phase 3 smoke + README + tag

- [ ] **Build + install:**
```bash
make install
```

- [ ] **Append README** (after Phase 2 features):

```markdown

## Phase 3 features

Connections tab:
- Real-time table of active connections (WebSocket `/connections`)
- `↑↓` / `j k` navigate; `x` close selected connection; `/` filter

Rules tab:
- Active rule list + rule-providers status (polled every 30 s)
- `/` filter rules by type / payload / target proxy

Settings tab (Phase 3 = Logs viewer):
- Live tail of mihomo logs (journalctl or PID-mode log file)
- `p` pause / resume

Full Settings polish (core version switch, patch editor, theme, command palette)
lands in Phase 4.
```

- [ ] **Commit + tag:**
```bash
git add README.md
git commit -m "docs: Phase 3 quickstart"
git tag v0.3.0-phase3
```

---

## Self-Review

- Spec §4.5: Connections (T4), Rules (T5), Logs sub-page in Settings (T6) — all covered.
- Spec §8: `/connections` WS (T1), `/connections/{id}` DELETE (T1), `/rules` (T2), `/providers/rules` (T2), `PUT /providers/rules/{name}` (T2) — all covered.
- Spec §10 error handling: streams reconnect with backoff in T7's goroutines.
- No placeholders; types defined before use.

8 tasks total. End state: vpnkit has full observation surfaces. Phase 4 finishes Settings (mihomo version mgmt, patch editor, theme, cache mgmt, command palette).
