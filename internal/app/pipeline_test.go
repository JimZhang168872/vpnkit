package app

import (
	"path/filepath"
	"testing"

	"vpnkit/internal/localnodes"
	"vpnkit/internal/store"
)

// newTestPipeline creates a Pipeline backed by a temp store for unit testing.
func newTestPipeline(t *testing.T) *Pipeline {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Load(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	extPath := filepath.Join(dir, "extensions.toml")
	return NewPipeline(st, cfgPath, extPath)
}

func TestPipelineLocalNodeGroupsCRUD(t *testing.T) {
	p := newTestPipeline(t)

	// Initially empty.
	if gs := p.LocalNodeGroups(); len(gs) != 0 {
		t.Fatalf("expected 0 groups initially, got %d", len(gs))
	}

	// AddLocalGroup: create two groups.
	if err := p.AddLocalGroup("home"); err != nil {
		t.Fatalf("AddLocalGroup home: %v", err)
	}
	if err := p.AddLocalGroup("office"); err != nil {
		t.Fatalf("AddLocalGroup office: %v", err)
	}
	gs := p.LocalNodeGroups()
	if len(gs) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(gs))
	}

	// Duplicate name must fail.
	if err := p.AddLocalGroup("home"); err == nil {
		t.Error("expected error on duplicate AddLocalGroup")
	}

	// Empty name must fail.
	if err := p.AddLocalGroup(""); err == nil {
		t.Error("expected error on empty AddLocalGroup name")
	}

	// ToggleLocalGroupEnabled.
	gs = p.LocalNodeGroups()
	initialEnabled := gs[0].Enabled
	if err := p.ToggleLocalGroupEnabled("home"); err != nil {
		t.Fatalf("ToggleLocalGroupEnabled: %v", err)
	}
	gs = p.LocalNodeGroups()
	if gs[0].Enabled == initialEnabled {
		t.Error("ToggleLocalGroupEnabled did not flip the flag")
	}

	// Toggle nonexistent group must fail.
	if err := p.ToggleLocalGroupEnabled("nonexistent"); err == nil {
		t.Error("expected error toggling nonexistent group")
	}

	// RenameLocalGroup.
	if err := p.RenameLocalGroup("home", "lan"); err != nil {
		t.Fatalf("RenameLocalGroup: %v", err)
	}
	gs = p.LocalNodeGroups()
	found := false
	for _, g := range gs {
		if g.Name == "lan" {
			found = true
		}
	}
	if !found {
		t.Errorf("after rename 'home' → 'lan', 'lan' not found in groups: %v", gs)
	}

	// Rename to empty must fail.
	if err := p.RenameLocalGroup("lan", ""); err == nil {
		t.Error("expected error renaming to empty name")
	}

	// Rename nonexistent must fail.
	if err := p.RenameLocalGroup("nonexistent", "x"); err == nil {
		t.Error("expected error renaming nonexistent group")
	}

	// Rename to existing name must fail.
	if err := p.RenameLocalGroup("lan", "office"); err == nil {
		t.Error("expected error renaming to existing name")
	}

	// Rename to same name is a no-op (no error).
	if err := p.RenameLocalGroup("lan", "lan"); err != nil {
		t.Errorf("rename to same name should be a no-op: %v", err)
	}
}

func TestPipelineDeleteLocalGroup(t *testing.T) {
	p := newTestPipeline(t)
	_ = p.AddLocalGroup("home")

	// Delete nonexistent must fail.
	if err := p.DeleteLocalGroup("missing", false); err == nil {
		t.Error("expected error deleting nonexistent group")
	}

	// Delete empty group (force=false) should succeed.
	if err := p.DeleteLocalGroup("home", false); err != nil {
		t.Fatalf("DeleteLocalGroup empty: %v", err)
	}
	if gs := p.LocalNodeGroups(); len(gs) != 0 {
		t.Fatalf("expected 0 groups after delete, got %d", len(gs))
	}
}

func TestPipelineDeleteLocalGroupForce(t *testing.T) {
	p := newTestPipeline(t)
	_ = p.AddLocalGroup("home")

	// Add a node belonging to "home" via store directly + reload.
	p.store.Cfg.LocalNodes = append(p.store.Cfg.LocalNodes, store.LocalNode{
		Name: "HK-1", Group: "home", Proto: "ss", Server: "1.2.3.4", Port: 8388,
	})
	p.localNodes.Load(toLocalNodes(p.store.Cfg.LocalNodes))

	// Delete without force must fail (group not empty).
	if err := p.DeleteLocalGroup("home", false); err == nil {
		t.Error("expected error deleting non-empty group without force")
	}

	// Delete with force=true cascades nodes.
	if err := p.DeleteLocalGroup("home", true); err != nil {
		t.Fatalf("DeleteLocalGroup force: %v", err)
	}
	if gs := p.LocalNodeGroups(); len(gs) != 0 {
		t.Fatalf("expected 0 groups after force-delete, got %d", len(gs))
	}
	if nodes := p.localNodes.All(); len(nodes) != 0 {
		t.Errorf("expected 0 nodes after force-delete, got %d", len(nodes))
	}
}

func TestPipelineSaveLocalGroupViaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	extPath := filepath.Join(dir, "extensions.toml")
	storePath := filepath.Join(dir, "config.toml")

	st, _ := store.Load(storePath)
	p := NewPipeline(st, cfgPath, extPath)
	_ = p.AddLocalGroup("home")

	// Add node with Group + Via directly via localnodes.Manager then SaveLocal.
	mgr := p.LocalNodes()
	if err := mgr.Add(localnodes.Node{
		Name: "HK-A", Group: "home", Via: "doge:JP-1",
		Proto: "hysteria2", Server: "1.2.3.4", Port: 443,
		Fields: map[string]any{"password": "x"},
	}); err != nil {
		t.Fatalf("Add node: %v", err)
	}
	if err := p.SaveLocal(); err != nil {
		t.Fatalf("SaveLocal: %v", err)
	}

	// Reload from disk and verify Group + Via are persisted.
	st2, _ := store.Load(storePath)
	p2 := NewPipeline(st2, cfgPath, extPath)
	all := p2.LocalNodes().All()
	if len(all) != 1 {
		t.Fatalf("expected 1 node after reload, got %d", len(all))
	}
	if all[0].Group != "home" {
		t.Errorf("Group round-trip: got %q want \"home\"", all[0].Group)
	}
	if all[0].Via != "doge:JP-1" {
		t.Errorf("Via round-trip: got %q want \"doge:JP-1\"", all[0].Via)
	}
}
