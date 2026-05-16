# vpnkit CLI ctl subcommands — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 6 non-interactive top-level subcommands (`status`, `ip`, `mode`, `groups`, `nodes`, `use`) to vpnkit so common state queries and switching can be done from a shell prompt or script, without launching the TUI. Each accepts `--json` for structured output.

**Architecture:** Extend the existing `switch os.Args[1]` in `cmd/vpnkit/main.go` with the 6 new cases. Each subcommand lives in its own `cmd/vpnkit/cmd_*.go` file with a corresponding `_test.go`. Shared helpers (`cmd_common.go`) handle config loading, `--json` parsing, table rendering, and exit-code routing. One small new API method (`GetConfigs`) and one extension to `ProxyInfo` (per-node delay history). No new third-party deps.

**Tech Stack:** Same as vpnkit today (Go 1.22, stdlib `net/http` + `encoding/json`). All tests use `httptest.Server` to mock mihomo + ipinfo.io.

**Spec reference:** [`docs/superpowers/specs/2026-05-16-vpnkit-ctl-design.md`](../specs/2026-05-16-vpnkit-ctl-design.md).

---

## File Map

| Path | Status | Responsibility |
|---|---|---|
| `internal/api/configs.go` | NEW | `Configs` struct + `GetConfigs(ctx)` |
| `internal/api/configs_test.go` | NEW | unit test against `httptest.Server` |
| `internal/api/proxies.go` | MODIFY | extend `ProxyInfo` with `History []ProxyHistory`; add `ProxyHistory` struct |
| `internal/api/proxies_test.go` | MODIFY | extend `TestGetProxies` to assert `History` decoded |
| `cmd/vpnkit/cmd_common.go` | NEW | `loadClient`, `parseFlags`, `renderTable`, `writeJSON`, `dieUserErr`, `dieRuntime` |
| `cmd/vpnkit/cmd_common_test.go` | NEW | unit tests for `parseFlags`, `renderTable`, `writeJSON` |
| `cmd/vpnkit/cmd_status.go` | NEW | `runStatus` |
| `cmd/vpnkit/cmd_status_test.go` | NEW | mock mihomo, assert output |
| `cmd/vpnkit/cmd_ip.go` | NEW | `runIP` (HTTP fetch via mihomo proxy) |
| `cmd/vpnkit/cmd_ip_test.go` | NEW | mock mihomo `/configs` + mock ipinfo |
| `cmd/vpnkit/cmd_mode.go` | NEW | `runMode` (no-arg show, with-arg set) |
| `cmd/vpnkit/cmd_mode_test.go` | NEW | three branches: show / set / invalid |
| `cmd/vpnkit/cmd_groups.go` | NEW | `runGroups` (filter builtins, render table) |
| `cmd/vpnkit/cmd_groups_test.go` | NEW | mock `/proxies`, assert filtering |
| `cmd/vpnkit/cmd_nodes.go` | NEW | `runNodes` (group lookup + history rendering) |
| `cmd/vpnkit/cmd_nodes_test.go` | NEW | happy + group-not-found |
| `cmd/vpnkit/cmd_use.go` | NEW | `runUse` (validate locally, then PUT) |
| `cmd/vpnkit/cmd_use_test.go` | NEW | happy + node-not-in-group + 4xx |
| `cmd/vpnkit/main.go` | MODIFY | extend the dispatcher switch |
| `README.md` | MODIFY | add new subcommands to Usage section |
| `docs/USAGE.md` | MODIFY | add a §1.8 (CLI scripting) section after the per-tab walkthrough |

---

## Task 1: Extend `ProxyInfo` with history

**Files:**
- Modify: `internal/api/proxies.go`
- Modify: `internal/api/proxies_test.go`

- [ ] **Step 1: Extend the failing test first**

In `internal/api/proxies_test.go`, find `TestGetProxies`. Update the mock body to include a non-group entry with `history`:

```go
func TestGetProxies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"GLOBAL": map[string]any{"type": "Selector", "now": "DIRECT", "all": []string{"DIRECT", "REJECT"}},
				"DIRECT": map[string]any{"type": "Direct"},
				"HK-01": map[string]any{
					"type": "Shadowsocks",
					"history": []map[string]any{
						{"time": "2026-05-16T10:00:00Z", "delay": 45},
						{"time": "2026-05-16T10:01:00Z", "delay": 47},
					},
				},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	out, err := c.GetProxies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	g, ok := out["GLOBAL"]
	if !ok || g.Type != "Selector" || g.Now != "DIRECT" {
		t.Errorf("GLOBAL: %+v", g)
	}
	if len(g.All) != 2 {
		t.Errorf("All: %v", g.All)
	}
	hk, ok := out["HK-01"]
	if !ok {
		t.Fatal("HK-01 missing")
	}
	if len(hk.History) != 2 || hk.History[1].Delay != 47 {
		t.Errorf("HK-01 history: %+v", hk.History)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
export PATH="$HOME/.local/go/bin:$PATH"
go test -race ./internal/api/ -run TestGetProxies -v
```

Expected: FAIL — `ProxyInfo` has no `History` field.

- [ ] **Step 3: Add the type and field**

In `internal/api/proxies.go`, replace the `ProxyInfo` struct and add `ProxyHistory` above it:

```go
// ProxyHistory is one delay measurement entry from mihomo's per-node history.
type ProxyHistory struct {
	Time  string `json:"time"`
	Delay int    `json:"delay"`
}

// ProxyInfo mirrors one entry in /proxies' "proxies" map.
type ProxyInfo struct {
	Type    string         `json:"type"`
	Now     string         `json:"now"`
	All     []string       `json:"all"`
	History []ProxyHistory `json:"history"`
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -race ./internal/api/ -run TestGetProxies -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/proxies.go internal/api/proxies_test.go
git commit -m "feat(api): expose per-node delay History on ProxyInfo"
```

---

## Task 2: Add `GetConfigs` API method

**Files:**
- Create: `internal/api/configs.go`
- Create: `internal/api/configs_test.go`

- [ ] **Step 1: Write failing test**

`internal/api/configs_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetConfigs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs" || r.Method != http.MethodGet {
			t.Fatalf("got %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode":       "rule",
			"log-level":  "info",
			"mixed-port": 7890,
			"allow-lan":  false,
			"secret":     "abc",
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	cfg, err := c.GetConfigs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "rule" || cfg.MixedPort != 7890 || cfg.LogLevel != "info" {
		t.Errorf("got %+v", cfg)
	}
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test -race ./internal/api/ -run TestGetConfigs -v
```

Expected: compile error — `GetConfigs` undefined.

- [ ] **Step 3: Implement**

`internal/api/configs.go`:

```go
package api

import (
	"context"
	"net/http"
)

// Configs mirrors the subset of /configs we use.
type Configs struct {
	Mode      string `json:"mode"`
	LogLevel  string `json:"log-level"`
	MixedPort int    `json:"mixed-port"`
	AllowLAN  bool   `json:"allow-lan"`
	Secret    string `json:"secret"`
}

// GetConfigs fetches /configs.
func (c *Client) GetConfigs(ctx context.Context) (Configs, error) {
	var out Configs
	err := c.do(ctx, http.MethodGet, "/configs", nil, &out)
	return out, err
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./internal/api/ -v
```

Expected: all api tests pass (existing + new).

- [ ] **Step 5: Commit**

```bash
git add internal/api/configs.go internal/api/configs_test.go
git commit -m "feat(api): GetConfigs method for /configs snapshot"
```

---

## Task 3: Common helpers (`cmd_common.go`)

**Files:**
- Create: `cmd/vpnkit/cmd_common.go`
- Create: `cmd/vpnkit/cmd_common_test.go`

- [ ] **Step 1: Write failing tests**

`cmd/vpnkit/cmd_common_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFlagsExtractsJSON(t *testing.T) {
	json, rest := parseFlags([]string{"--json", "Proxy", "HK-01"})
	if !json {
		t.Error("expected json=true")
	}
	if len(rest) != 2 || rest[0] != "Proxy" || rest[1] != "HK-01" {
		t.Errorf("rest=%v", rest)
	}
}

func TestParseFlagsJSONLast(t *testing.T) {
	json, rest := parseFlags([]string{"Proxy", "HK-01", "--json"})
	if !json {
		t.Error("expected json=true")
	}
	if len(rest) != 2 {
		t.Errorf("rest=%v", rest)
	}
}

func TestParseFlagsNoJSON(t *testing.T) {
	json, rest := parseFlags([]string{"Proxy"})
	if json {
		t.Error("expected json=false")
	}
	if len(rest) != 1 {
		t.Errorf("rest=%v", rest)
	}
}

func TestRenderTableAlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	renderTable(&buf, []string{"GROUP", "TYPE"}, [][]string{
		{"🚀 Proxy", "Selector"},
		{"♻️ Auto", "URLTest"},
	})
	out := buf.String()
	if !strings.Contains(out, "GROUP") || !strings.Contains(out, "TYPE") {
		t.Errorf("missing headers: %s", out)
	}
	if !strings.Contains(out, "🚀 Proxy") || !strings.Contains(out, "Selector") {
		t.Errorf("missing rows: %s", out)
	}
}

func TestWriteJSONCompactWithNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, map[string]any{"a": 1, "b": "x"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected trailing newline: %q", out)
	}
	if !strings.Contains(out, `"a":1`) || !strings.Contains(out, `"b":"x"`) {
		t.Errorf("output: %s", out)
	}
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test -race ./cmd/vpnkit/ -v
```

Expected: compile error.

- [ ] **Step 3: Write implementation**

`cmd/vpnkit/cmd_common.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"vpnkit/internal/api"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// loadClient reads vpnkit's config.toml and returns an api.Client + Store.
// Exits with code 2 on failure (mihomo unreachable from the caller's perspective is
// surfaced later by the actual API calls; here we only fail on file IO).
func loadClient() (*api.Client, *store.Store, error) {
	p := paths.Resolve()
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		return nil, nil, fmt.Errorf("load store: %w", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", st.Cfg.ControllerPort)
	return api.New(url, st.Cfg.ControllerSecret), st, nil
}

// parseFlags extracts a leading or trailing `--json` flag from args.
// Only this one flag is supported across all subcommands.
func parseFlags(args []string) (jsonOut bool, rest []string) {
	rest = make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		rest = append(rest, a)
	}
	return
}

// renderTable writes a left-aligned ASCII table to out.
// Column widths are sized to the longest cell (counted in runes, not bytes,
// so emoji width is approximate; that's acceptable for a CLI table).
func renderTable(out io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = runeLen(h)
	}
	for _, row := range rows {
		for i, c := range row {
			if i >= len(widths) {
				continue
			}
			if l := runeLen(c); l > widths[i] {
				widths[i] = l
			}
		}
	}
	writeRow(out, headers, widths)
	for _, row := range rows {
		writeRow(out, row, widths)
	}
}

func writeRow(out io.Writer, cols []string, widths []int) {
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(out, "  ")
		}
		fmt.Fprint(out, c)
		pad := widths[i] - runeLen(c)
		for p := 0; p < pad; p++ {
			fmt.Fprint(out, " ")
		}
	}
	fmt.Fprintln(out)
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// writeJSON marshals v compactly and writes a trailing newline.
func writeJSON(out io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	_, err = out.Write([]byte("\n"))
	return err
}

// dieUserErr writes to stderr and exits 1.
func dieUserErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// dieRuntime writes to stderr and exits 2.
func dieRuntime(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./cmd/vpnkit/ -v
```

Expected: 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/cmd_common.go cmd/vpnkit/cmd_common_test.go
git commit -m "feat(cmd): shared helpers for ctl subcommands (parseFlags, renderTable, writeJSON, loadClient)"
```

---

## Task 4: `vpnkit status`

**Files:**
- Create: `cmd/vpnkit/cmd_status.go`
- Create: `cmd/vpnkit/cmd_status_test.go`

- [ ] **Step 1: Write failing test**

`cmd/vpnkit/cmd_status_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vpnkit/internal/api"
)

func mockMihomo(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	return httptest.NewServer(h)
}

func TestStatusHumanOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "v1.19.16", "meta": true})
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": 7890})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"DIRECT":   map[string]any{"type": "Direct"},
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01", "JP-02"}},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runStatus(&buf, c, nil, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"v1.19.16", "running", "rule", "mixed=7890", "🚀 Proxy", "HK-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStatusJSONOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "v1.0.0", "meta": true})
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "global", "mixed-port": 7891})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runStatus(&buf, c, nil, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["mode"] != "global" {
		t.Errorf("mode: %v", got["mode"])
	}
}

func TestStatusUnreachable(t *testing.T) {
	c := api.New("http://127.0.0.1:1", "")
	c.HTTP.Timeout = 200 * time.Millisecond // fail fast instead of waiting 5s
	var buf bytes.Buffer
	err := runStatus(&buf, c, nil, false)
	if err == nil {
		t.Error("expected error for unreachable mihomo")
	}
	_ = context.Canceled
}
```

- [ ] **Step 2: Verify fail**

```bash
go test -race ./cmd/vpnkit/ -run TestStatus -v
```

Expected: compile error — `runStatus` undefined.

- [ ] **Step 3: Implement**

`cmd/vpnkit/cmd_status.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"vpnkit/internal/api"
	"vpnkit/internal/store"
)

// runStatus prints a snapshot of mihomo state. store may be nil (tests).
func runStatus(out io.Writer, c *api.Client, st *store.Store, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	v, err := c.Version(ctx)
	if err != nil {
		return fmt.Errorf("mihomo not reachable: %w", err)
	}
	cfg, err := c.GetConfigs(ctx)
	if err != nil {
		return fmt.Errorf("get configs: %w", err)
	}
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return fmt.Errorf("get proxies: %w", err)
	}

	type groupSummary struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Now     string `json:"now"`
		Members int    `json:"members"`
	}

	var groups []groupSummary
	for name, info := range proxies {
		if !isUserSelectableType(info.Type) {
			continue
		}
		if len(info.All) == 0 {
			continue
		}
		groups = append(groups, groupSummary{Name: name, Type: info.Type, Now: info.Now, Members: len(info.All)})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	type profileSummary struct {
		Name        string `json:"name"`
		NodeCount   int    `json:"node_count"`
		LastUpdated string `json:"last_updated,omitempty"`
	}
	var profile *profileSummary
	if st != nil && st.Cfg.ActiveProfile != "" {
		for _, p := range st.Cfg.Profiles {
			if p.Name == st.Cfg.ActiveProfile {
				profile = &profileSummary{
					Name:        p.Name,
					NodeCount:   0,
					LastUpdated: p.LastUpdated.Format(time.RFC3339),
				}
				break
			}
		}
	}

	if jsonOut {
		payload := map[string]any{
			"mihomo": map[string]any{"version": v.Version, "running": true},
			"mode":   cfg.Mode,
			"ports":  map[string]int{"mixed": cfg.MixedPort, "controller": controllerPortFromClient(c)},
			"groups": groups,
		}
		if profile != nil {
			payload["active_profile"] = profile
		}
		return writeJSON(out, payload)
	}

	fmt.Fprintf(out, "mihomo  %s   ● running\n", v.Version)
	fmt.Fprintf(out, "mode    %s\n", cfg.Mode)
	fmt.Fprintf(out, "ports   mixed=%d   controller=%d\n", cfg.MixedPort, controllerPortFromClient(c))

	if len(groups) == 0 {
		fmt.Fprintln(out, "groups  none")
	} else {
		summary := ""
		for i, g := range groups {
			if i > 0 {
				summary += ", "
			}
			summary += fmt.Sprintf("%s → %s", g.Name, g.Now)
		}
		fmt.Fprintf(out, "groups  %d selectable (%s)\n", len(groups), summary)
	}

	if profile != nil {
		fmt.Fprintf(out, "profile %s\n", profile.Name)
	}
	return nil
}

// isUserSelectableType keeps only proxy-group types the user can switch.
func isUserSelectableType(t string) bool {
	switch t {
	case "Selector", "URLTest", "Fallback", "LoadBalance":
		return true
	}
	return false
}

// controllerPortFromClient parses the port out of the api.Client's BaseURL.
// Returns 0 if it can't be parsed (we don't fail the command for this).
func controllerPortFromClient(c *api.Client) int {
	var port int
	_, _ = fmt.Sscanf(c.BaseURL, "http://127.0.0.1:%d", &port)
	return port
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./cmd/vpnkit/ -run TestStatus -v
```

Expected: 3 tests pass (the unreachable test relies on the connection actually failing fast — with a real network test it should error within a couple seconds; that's acceptable for a one-off test).

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/cmd_status.go cmd/vpnkit/cmd_status_test.go
git commit -m "feat(cmd): vpnkit status — version, mode, ports, groups, active profile"
```

---

## Task 5: `vpnkit mode`

**Files:**
- Create: `cmd/vpnkit/cmd_mode.go`
- Create: `cmd/vpnkit/cmd_mode_test.go`

- [ ] **Step 1: Write failing test**

`cmd/vpnkit/cmd_mode_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestModeShow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": 7890})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runMode(&buf, c, nil, false); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "rule" {
		t.Errorf("got %q", buf.String())
	}
}

func TestModeSet(t *testing.T) {
	calls := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule"})
		case http.MethodPatch:
			// no body needed in response
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runMode(&buf, c, []string{"global"}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "rule → global") {
		t.Errorf("output: %s", buf.String())
	}
	if len(calls) != 2 || calls[0] != http.MethodGet || calls[1] != http.MethodPatch {
		t.Errorf("calls: %v", calls)
	}
}

func TestModeInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runMode(&buf, c, []string{"foobar"}, false)
	if err == nil || !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("expected invalid-mode error, got %v", err)
	}
}
```

- [ ] **Step 2: Verify fail**

```bash
go test -race ./cmd/vpnkit/ -run TestMode -v
```

Expected: compile error.

- [ ] **Step 3: Implement**

`cmd/vpnkit/cmd_mode.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"vpnkit/internal/api"
)

var allowedModes = map[string]bool{"rule": true, "global": true, "direct": true}

// runMode shows or sets mihomo's mode.
//   args == []           → show
//   args == ["rule"]     → set
func runMode(out io.Writer, c *api.Client, args []string, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if len(args) == 0 {
		cfg, err := c.GetConfigs(ctx)
		if err != nil {
			return fmt.Errorf("get configs: %w", err)
		}
		if jsonOut {
			return writeJSON(out, map[string]any{"mode": cfg.Mode})
		}
		fmt.Fprintln(out, cfg.Mode)
		return nil
	}

	target := args[0]
	if !allowedModes[target] {
		return fmt.Errorf("invalid mode %q (allowed: rule, global, direct)", target)
	}
	cfg, err := c.GetConfigs(ctx)
	if err != nil {
		return fmt.Errorf("get configs: %w", err)
	}
	if cfg.Mode == target {
		if jsonOut {
			return writeJSON(out, map[string]any{"from": target, "to": target})
		}
		fmt.Fprintf(out, "mode: %s (no change)\n", target)
		return nil
	}
	if err := c.SetMode(ctx, target); err != nil {
		return fmt.Errorf("set mode: %w", err)
	}
	if jsonOut {
		return writeJSON(out, map[string]any{"from": cfg.Mode, "to": target})
	}
	fmt.Fprintf(out, "mode: %s → %s\n", cfg.Mode, target)
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./cmd/vpnkit/ -run TestMode -v
```

Expected: 3 pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/cmd_mode.go cmd/vpnkit/cmd_mode_test.go
git commit -m "feat(cmd): vpnkit mode [rule|global|direct]"
```

---

## Task 6: `vpnkit groups`

**Files:**
- Create: `cmd/vpnkit/cmd_groups.go`
- Create: `cmd/vpnkit/cmd_groups_test.go`

- [ ] **Step 1: Write failing test**

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestGroupsFiltersBuiltinsAndRendersTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"DIRECT":   map[string]any{"type": "Direct"},
				"REJECT":   map[string]any{"type": "Reject"},
				"GLOBAL":   map[string]any{"type": "Selector", "now": "DIRECT", "all": []string{"DIRECT"}},
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01", "JP-02"}},
				"♻️ Auto":  map[string]any{"type": "URLTest", "now": "HK-01", "all": []string{"HK-01", "JP-02"}},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runGroups(&buf, c, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"🚀 Proxy", "♻️ Auto", "Selector", "URLTest", "HK-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "DIRECT") || strings.Contains(out, "REJECT") || strings.Contains(out, "GLOBAL") {
		t.Errorf("builtin not filtered:\n%s", out)
	}
}

func TestGroupsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"P": map[string]any{"type": "Selector", "now": "n1", "all": []string{"n1"}},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runGroups(&buf, c, true); err != nil {
		t.Fatal(err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &arr); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if len(arr) != 1 || arr[0]["name"] != "P" {
		t.Errorf("got %v", arr)
	}
}
```

- [ ] **Step 2: Verify fail**

```bash
go test -race ./cmd/vpnkit/ -run TestGroups -v
```

Expected: compile error.

- [ ] **Step 3: Implement**

`cmd/vpnkit/cmd_groups.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"vpnkit/internal/api"
)

func runGroups(out io.Writer, c *api.Client, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return fmt.Errorf("get proxies: %w", err)
	}

	type entry struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Now     string `json:"now"`
		Members int    `json:"members"`
	}
	var rows []entry
	for name, info := range proxies {
		if !isUserSelectableType(info.Type) {
			continue
		}
		if len(info.All) == 0 {
			continue
		}
		rows = append(rows, entry{Name: name, Type: info.Type, Now: info.Now, Members: len(info.All)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	if jsonOut {
		return writeJSON(out, rows)
	}
	tbl := make([][]string, 0, len(rows))
	for _, e := range rows {
		tbl = append(tbl, []string{e.Name, e.Type, e.Now, fmt.Sprintf("%d", e.Members)})
	}
	renderTable(out, []string{"GROUP", "TYPE", "CURRENT", "MEMBERS"}, tbl)
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./cmd/vpnkit/ -run TestGroups -v
```

Expected: 2 pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/cmd_groups.go cmd/vpnkit/cmd_groups_test.go
git commit -m "feat(cmd): vpnkit groups — list user-selectable proxy groups"
```

---

## Task 7: `vpnkit nodes <group>`

**Files:**
- Create: `cmd/vpnkit/cmd_nodes.go`
- Create: `cmd/vpnkit/cmd_nodes_test.go`

- [ ] **Step 1: Write failing test**

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestNodesHumanShowsCurrentAndDelays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01", "JP-02", "KR-04"}},
				"HK-01": map[string]any{
					"type":    "Shadowsocks",
					"history": []map[string]any{{"time": "t1", "delay": 45}},
				},
				"JP-02": map[string]any{
					"type":    "Shadowsocks",
					"history": []map[string]any{{"time": "t1", "delay": 87}},
				},
				"KR-04": map[string]any{"type": "Shadowsocks"},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runNodes(&buf, c, "🚀 Proxy", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"✓ HK-01", "JP-02", "45", "87", "(no test)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestNodesGroupNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{"P": map[string]any{"type": "Selector", "all": []string{}}},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runNodes(&buf, c, "NotExist", false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestNodesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"P":  map[string]any{"type": "Selector", "now": "n1", "all": []string{"n1"}},
				"n1": map[string]any{"type": "Shadowsocks", "history": []map[string]any{{"time": "t", "delay": 12}}},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runNodes(&buf, c, "P", true); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if got["group"] != "P" || got["current"] != "n1" {
		t.Errorf("got %v", got)
	}
}
```

- [ ] **Step 2: Verify fail**

```bash
go test -race ./cmd/vpnkit/ -run TestNodes -v
```

Expected: compile error.

- [ ] **Step 3: Implement**

`cmd/vpnkit/cmd_nodes.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"vpnkit/internal/api"
)

func runNodes(out io.Writer, c *api.Client, group string, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return fmt.Errorf("get proxies: %w", err)
	}
	g, ok := proxies[group]
	if !ok || !isUserSelectableType(g.Type) || len(g.All) == 0 {
		return fmt.Errorf("group %q not found (try 'vpnkit groups')", group)
	}

	type node struct {
		Name  string `json:"name"`
		Delay *int   `json:"delay"`
	}
	out2 := make([]node, 0, len(g.All))
	for _, name := range g.All {
		n := node{Name: name}
		info, ok := proxies[name]
		if ok && len(info.History) > 0 {
			d := info.History[len(info.History)-1].Delay
			n.Delay = &d
		}
		out2 = append(out2, n)
	}

	if jsonOut {
		return writeJSON(out, map[string]any{
			"group":   group,
			"current": g.Now,
			"nodes":   out2,
		})
	}

	for _, n := range out2 {
		marker := "  "
		if n.Name == g.Now {
			marker = "✓ "
		}
		delayStr := "(no test)"
		if n.Delay != nil {
			delayStr = fmt.Sprintf("%d ms", *n.Delay)
		}
		fmt.Fprintf(out, "%s%-20s  %s\n", marker, n.Name, delayStr)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./cmd/vpnkit/ -run TestNodes -v
```

Expected: 3 pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/cmd_nodes.go cmd/vpnkit/cmd_nodes_test.go
git commit -m "feat(cmd): vpnkit nodes <group> — list members + cached delay"
```

---

## Task 8: `vpnkit use <group> <node>`

**Files:**
- Create: `cmd/vpnkit/cmd_use.go`
- Create: `cmd/vpnkit/cmd_use_test.go`

- [ ] **Step 1: Write failing test**

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestUseHappy(t *testing.T) {
	put := false
	mux := http.NewServeMux()
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "JP-02", "all": []string{"HK-01", "JP-02"}},
			},
		})
	})
	mux.HandleFunc("/proxies/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			put = true
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runUse(&buf, c, "🚀 Proxy", "HK-01", false); err != nil {
		t.Fatal(err)
	}
	if !put {
		t.Error("expected PUT to /proxies/{group}")
	}
	if !strings.Contains(buf.String(), "Proxy → HK-01") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestUseNodeNotInGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"P": map[string]any{"type": "Selector", "now": "n1", "all": []string{"n1"}},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runUse(&buf, c, "P", "n2", false)
	if err == nil || !strings.Contains(err.Error(), "not in group") {
		t.Errorf("got %v", err)
	}
}

func TestUseGroupNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{}})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runUse(&buf, c, "NoSuch", "n", false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("got %v", err)
	}
}
```

- [ ] **Step 2: Verify fail**

```bash
go test -race ./cmd/vpnkit/ -run TestUse -v
```

Expected: compile error.

- [ ] **Step 3: Implement**

`cmd/vpnkit/cmd_use.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"vpnkit/internal/api"
)

func runUse(out io.Writer, c *api.Client, group, node string, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return fmt.Errorf("get proxies: %w", err)
	}
	g, ok := proxies[group]
	if !ok || !isUserSelectableType(g.Type) || len(g.All) == 0 {
		return fmt.Errorf("group %q not found (try 'vpnkit groups')", group)
	}
	found := false
	for _, m := range g.All {
		if m == node {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("node %q not in group %q", node, group)
	}
	if err := c.PutProxy(ctx, group, node); err != nil {
		return fmt.Errorf("set proxy: %w", err)
	}

	if jsonOut {
		return writeJSON(out, map[string]any{"group": group, "now": node})
	}
	fmt.Fprintf(out, "✓ %s → %s\n", group, node)
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./cmd/vpnkit/ -run TestUse -v
```

Expected: 3 pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/cmd_use.go cmd/vpnkit/cmd_use_test.go
git commit -m "feat(cmd): vpnkit use <group> <node> — switch with local validation"
```

---

## Task 9: `vpnkit ip`

**Files:**
- Create: `cmd/vpnkit/cmd_ip.go`
- Create: `cmd/vpnkit/cmd_ip_test.go`

- [ ] **Step 1: Write failing test**

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestIPHuman(t *testing.T) {
	// Mock ipinfo.io
	ipsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ip": "203.0.113.42", "country": "HK", "region": "Central and Western",
			"city": "Hong Kong", "org": "AS12345 Example Hosting Ltd.",
		})
	}))
	defer ipsrv.Close()
	// Mock mihomo /configs (returns mixed-port that points BACK to ipsrv so the
	// "proxy fetch" actually succeeds without a real proxy in the loop)
	ipURL, _ := url.Parse(ipsrv.URL)
	port, _ := strconv.Atoi(ipURL.Port())

	mux := http.NewServeMux()
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": port})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01"}},
			},
		})
	})
	mihomoSrv := httptest.NewServer(mux)
	defer mihomoSrv.Close()

	c := api.New(mihomoSrv.URL, "")
	var buf bytes.Buffer
	// Override the ipinfo URL the runIP function would use, by passing it explicitly.
	if err := runIP(&buf, c, ipsrv.URL+"/json", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"203.0.113.42", "HK", "Hong Kong", "AS12345", "🚀 Proxy"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPMihomoUnreachable(t *testing.T) {
	c := api.New("http://127.0.0.1:1", "")
	var buf bytes.Buffer
	err := runIP(&buf, c, "https://ipinfo.io/json", false)
	if err == nil {
		t.Error("expected error")
	}
}
```

NOTE: the test uses ipsrv as BOTH the proxy and the destination. When `runIP` configures `http.Transport.Proxy = http.ProxyURL("http://127.0.0.1:<mixed-port>")`, the request goes to ipsrv (acting as the "proxy"), which simply ignores the proxy semantics and serves its own response. That's enough to exercise our code path without needing a real HTTP-CONNECT tunnel.

- [ ] **Step 2: Verify fail**

```bash
go test -race ./cmd/vpnkit/ -run TestIP -v
```

Expected: compile error.

- [ ] **Step 3: Implement**

`cmd/vpnkit/cmd_ip.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"

	"vpnkit/internal/api"
)

const defaultIPInfoURL = "https://ipinfo.io/json"

// runIP fetches ipinfoURL through mihomo's mixed-port proxy.
// If ipinfoURL is empty, defaultIPInfoURL is used.
func runIP(out io.Writer, c *api.Client, ipinfoURL string, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if ipinfoURL == "" {
		ipinfoURL = defaultIPInfoURL
	}

	cfg, err := c.GetConfigs(ctx)
	if err != nil {
		return fmt.Errorf("mihomo not reachable: %w", err)
	}
	if cfg.MixedPort == 0 {
		return fmt.Errorf("mihomo mixed-port not configured")
	}
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", cfg.MixedPort))
	client := &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ipinfoURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ipinfo unreachable through proxy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("ipinfo status %d", resp.StatusCode)
	}
	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("decode ipinfo: %w", err)
	}

	via := pickVia(c, ctx)
	info["via"] = via

	if jsonOut {
		return writeJSON(out, info)
	}
	rows := []struct {
		k, v string
	}{
		{"ip", asString(info["ip"])},
		{"country", asString(info["country"])},
		{"region", asString(info["region"])},
		{"city", asString(info["city"])},
		{"org", asString(info["org"])},
		{"via", via},
	}
	for _, r := range rows {
		fmt.Fprintf(out, "%-8s %s\n", r.k, r.v)
	}
	return nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// pickVia returns "<group> → <now>" for the first user-selectable group.
// Returns "" if nothing matches or proxies fetch fails (best-effort).
func pickVia(c *api.Client, ctx context.Context) string {
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return ""
	}
	var names []string
	for n, info := range proxies {
		if isUserSelectableType(info.Type) && len(info.All) > 0 {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	g := proxies[names[0]]
	return fmt.Sprintf("%s → %s", names[0], g.Now)
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./cmd/vpnkit/ -run TestIP -v
```

Expected: 2 pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/cmd_ip.go cmd/vpnkit/cmd_ip_test.go
git commit -m "feat(cmd): vpnkit ip — fetch ipinfo through mihomo proxy"
```

---

## Task 10: Wire dispatcher in `main.go`

**Files:**
- Modify: `cmd/vpnkit/main.go`

- [ ] **Step 1: Read current main.go**

```bash
cat cmd/vpnkit/main.go
```

You'll see a `switch os.Args[1]` with cases for `--version` and `env`. The default falls through to the TUI.

- [ ] **Step 2: Replace the switch with the extended version**

The new `main` function:

```go
func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			runVersion()
			return
		case "env":
			runEnv(os.Args[2:])
			return
		case "status":
			dispatchStatus(os.Args[2:])
			return
		case "ip":
			dispatchIP(os.Args[2:])
			return
		case "mode":
			dispatchMode(os.Args[2:])
			return
		case "groups":
			dispatchGroups(os.Args[2:])
			return
		case "nodes":
			dispatchNodes(os.Args[2:])
			return
		case "use":
			dispatchUse(os.Args[2:])
			return
		}
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "vpnkit:", err)
		os.Exit(1)
	}
}
```

Append the dispatchers below the existing `runEnv`. Each dispatcher loads the client, parses `--json`, calls the `runX` function, and routes errors via `dieRuntime` / `dieUserErr`:

```go
func dispatchStatus(args []string) {
	jsonOut, _ := parseFlags(args)
	c, st, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit status: %v", err)
	}
	if err := runStatus(os.Stdout, c, st, jsonOut); err != nil {
		dieRuntime("vpnkit status: %v", err)
	}
}

func dispatchIP(args []string) {
	jsonOut, _ := parseFlags(args)
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit ip: %v", err)
	}
	if err := runIP(os.Stdout, c, "", jsonOut); err != nil {
		dieRuntime("vpnkit ip: %v", err)
	}
}

func dispatchMode(args []string) {
	jsonOut, rest := parseFlags(args)
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit mode: %v", err)
	}
	if err := runMode(os.Stdout, c, rest, jsonOut); err != nil {
		dieUserErr("vpnkit mode: %v", err)
	}
}

func dispatchGroups(args []string) {
	jsonOut, _ := parseFlags(args)
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit groups: %v", err)
	}
	if err := runGroups(os.Stdout, c, jsonOut); err != nil {
		dieRuntime("vpnkit groups: %v", err)
	}
}

func dispatchNodes(args []string) {
	jsonOut, rest := parseFlags(args)
	if len(rest) < 1 {
		dieUserErr("vpnkit nodes: usage: vpnkit nodes <group> [--json]")
	}
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit nodes: %v", err)
	}
	if err := runNodes(os.Stdout, c, rest[0], jsonOut); err != nil {
		dieUserErr("vpnkit nodes: %v", err)
	}
}

func dispatchUse(args []string) {
	jsonOut, rest := parseFlags(args)
	if len(rest) < 2 {
		dieUserErr("vpnkit use: usage: vpnkit use <group> <node> [--json]")
	}
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit use: %v", err)
	}
	if err := runUse(os.Stdout, c, rest[0], rest[1], jsonOut); err != nil {
		dieUserErr("vpnkit use: %v", err)
	}
}
```

- [ ] **Step 3: Build and smoke-test**

```bash
go build ./...
./bin/vpnkit             # not run here — would launch TUI
./bin/vpnkit --version   # should still work
```

(Smoke-testing the new commands requires mihomo running. Skip in this step; Task 11 covers manual smoke.)

- [ ] **Step 4: Run all tests**

```bash
go test -race ./...
```

Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add cmd/vpnkit/main.go
git commit -m "feat(cmd): wire status/ip/mode/groups/nodes/use into main dispatcher"
```

---

## Task 11: Manual smoke + docs + tag

**Files:**
- Modify: `README.md`
- Modify: `docs/USAGE.md`

- [ ] **Step 1: Reinstall + smoke**

```bash
make install
~/.local/bin/vpnkit status
~/.local/bin/vpnkit groups
~/.local/bin/vpnkit groups --json | head -3
~/.local/bin/vpnkit mode
~/.local/bin/vpnkit nodes 'Proxy' 2>&1 | head -5    # may need quoting if group has emoji
~/.local/bin/vpnkit ip                              # uses real mihomo + real ipinfo.io
```

If `vpnkit ip` times out, that's OK as long as the error is the documented `ipinfo unreachable through proxy` shape — record the actual outputs in the commit message of step 4.

- [ ] **Step 2: Update README**

In `README.md`, find the "Usage" section that lists `vpnkit env` etc. Add the new commands under both English and 简体中文 halves.

In English under `## Usage` or wherever the current command list lives, add:

```markdown
### CLI commands

```bash
vpnkit status              # mihomo state, mode, ports, groups, active profile
vpnkit ip                  # exit IP via mihomo proxy
vpnkit mode [rule|global|direct]   # show or set mode
vpnkit groups              # list user-selectable proxy groups
vpnkit nodes <group>       # list members of a group
vpnkit use <group> <node>  # switch a Selector group to a specific node
```

All of the above accept `--json` for scripting.
```

In 简体中文 add:

```markdown
### 命令行子命令

```bash
vpnkit status              # mihomo 状态、模式、端口、代理组、当前订阅
vpnkit ip                  # 经 mihomo 代理查出口 IP
vpnkit mode [rule|global|direct]   # 显示或设置模式
vpnkit groups              # 列出用户可选 proxy 组
vpnkit nodes <group>       # 列出某组成员
vpnkit use <group> <node>  # 切 Selector 组到指定节点
```

每条都接受 `--json`，方便脚本化。
```

- [ ] **Step 3: Update USAGE.md**

In `docs/USAGE.md`, after §1.7 "Use the proxy from your terminal", insert new §1.8:

```markdown
#### 1.8 CLI scripting

If you'd rather not stay in the TUI, six top-level subcommands cover everyday
operations:

```bash
vpnkit status                            # mihomo state, mode, ports, groups, profile
vpnkit ip                                # exit IP via mihomo proxy
vpnkit mode                              # show current mode
vpnkit mode global                       # switch mode
vpnkit groups                            # list user-selectable groups
vpnkit nodes 'Proxy'                     # list members + cached delays
vpnkit use 'Proxy' 'HK-01'               # switch node
```

Append `--json` to anything for `jq`-able output. Exit codes: `0` success, `1`
user error (bad arg, group missing), `2` runtime error (mihomo unreachable).
```

And the equivalent in the 中文 §1.8 below the EN one.

- [ ] **Step 4: Commit + tag**

```bash
git add README.md docs/USAGE.md
git commit -m "docs: document new vpnkit ctl subcommands"
git tag v0.5.0-ctl -m "CLI ctl subcommands: status / ip / mode / groups / nodes / use"
```

- [ ] **Step 5: Push**

```bash
git push origin main
git push origin v0.5.0-ctl
```

---

## Self-Review

**Spec coverage:**
- §1 Goals — covered by tasks 4–9.
- §2 Entry shape — task 10.
- §3.1 status — task 4.
- §3.2 ip — task 9.
- §3.3 mode — task 5.
- §3.4 groups — task 6.
- §3.5 nodes — task 7 (note: `--test` deferred per spec, not added here).
- §3.6 use — task 8 (forwards mihomo's 4xx unchanged via the `set proxy: %w` wrapper, hits exit 2 via dispatcher's `dieRuntime`; matches spec language about URLTest/Fallback behavior).
- §4 helpers — task 3.
- §5 API additions — tasks 1 + 2.
- §6 file map — implemented across tasks; matches.
- §7 testing strategy — every runX has unit tests (tasks 4–9); ipinfo test mocks both servers (task 9).
- §8 exit codes — wired in task 10's dispatchers.
- §9 out-of-scope — nothing added beyond the 6 commands.

**Placeholder scan:** no TBDs, every step has runnable commands or complete code.

**Type consistency:** `runStatus(out, c, st, jsonOut)` signature consistent across status; `runMode(out, c, args, jsonOut)`; `runIP(out, c, ipinfoURL, jsonOut)` injects URL for testability. `isUserSelectableType` defined once in cmd_status.go, reused by cmd_groups.go, cmd_nodes.go, cmd_ip.go (same package). `controllerPortFromClient` likewise package-local.

End of plan.
