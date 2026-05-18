package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/app"
	"vpnkit/internal/store"
)

// makeActiveTestStore builds a Store on a tempfile seeded with two
// enabled subs (`doge`, `boost`) and one enabled local group `Local`.
// Returns the store + the mihomo config.yaml path Pipeline writes to.
func makeActiveTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "config.toml")
	st, err := store.Load(storePath)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	st.Cfg.Subscriptions = []store.Subscription{
		{Name: "doge", URL: "https://x", Enabled: true},
		{Name: "boost", URL: "https://y", Enabled: true},
	}
	st.Cfg.LocalNodeGroups = []store.LocalNodeGroup{
		{Name: "Local", Enabled: true},
	}
	st.Cfg.ActiveSource = "doge"
	if err := st.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return st, filepath.Join(dir, "mihomo.yaml")
}

func TestRunActiveShowText(t *testing.T) {
	st, _ := makeActiveTestStore(t)
	var buf bytes.Buffer
	runActiveShow(&buf, app.NewPipeline(st, filepath.Join(t.TempDir(), "m.yaml")), false)
	got := strings.TrimSpace(buf.String())
	if got != "doge  (subscription)" {
		t.Errorf("text mode: got %q, want %q", got, "doge  (subscription)")
	}
}

func TestRunActiveShowJSON(t *testing.T) {
	st, _ := makeActiveTestStore(t)
	var buf bytes.Buffer
	runActiveShow(&buf, app.NewPipeline(st, filepath.Join(t.TempDir(), "m.yaml")), true)
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["active_source"] != "doge" {
		t.Errorf("active_source: %v", got["active_source"])
	}
	if got["kind"] != "subscription" {
		t.Errorf("kind: %v", got["kind"])
	}
}

func TestRunActiveShowEmpty(t *testing.T) {
	st, _ := makeActiveTestStore(t)
	st.Cfg.ActiveSource = ""
	var buf bytes.Buffer
	runActiveShow(&buf, app.NewPipeline(st, filepath.Join(t.TempDir(), "m.yaml")), false)
	if !strings.Contains(buf.String(), "none") {
		t.Errorf("empty active: %q", buf.String())
	}
}

// TestRunActiveSetSwitchesSub — happy path: switch from doge to boost
// and assert the store persists the change.
func TestRunActiveSetSwitchesSub(t *testing.T) {
	st, configPath := makeActiveTestStore(t)
	pl := app.NewPipeline(st, configPath)

	var buf bytes.Buffer
	if err := runActiveSet(&buf, pl, "boost", false); err != nil {
		t.Fatalf("runActiveSet: %v", err)
	}
	if !strings.Contains(buf.String(), "boost") {
		t.Errorf("output should confirm boost: %q", buf.String())
	}
	if st.Cfg.ActiveSource != "boost" {
		t.Errorf("store ActiveSource: got %q, want boost", st.Cfg.ActiveSource)
	}
}

// TestRunActiveSetLocalGroup — local-node group is a legal active source
// and the kind label reports "local" (not "subscription").
func TestRunActiveSetLocalGroup(t *testing.T) {
	st, configPath := makeActiveTestStore(t)
	pl := app.NewPipeline(st, configPath)

	var buf bytes.Buffer
	if err := runActiveSet(&buf, pl, "Local", false); err != nil {
		t.Fatalf("runActiveSet: %v", err)
	}
	if !strings.Contains(buf.String(), "(local)") {
		t.Errorf("kind label should be `local`: %q", buf.String())
	}
}

// TestRunActiveSetUnknown — setting an active source that isn't an
// enabled sub or local group must fail loud rather than silently writing
// a stale name. SetActiveSource enforces the validation.
func TestRunActiveSetUnknown(t *testing.T) {
	st, configPath := makeActiveTestStore(t)
	pl := app.NewPipeline(st, configPath)
	var buf bytes.Buffer
	err := runActiveSet(&buf, pl, "nonexistent", false)
	if err == nil {
		t.Fatal("expected error for unknown source, got nil")
	}
}
