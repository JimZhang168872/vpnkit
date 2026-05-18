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
	return NewPipeline(st, cfgPath)
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
	storePath := filepath.Join(dir, "config.toml")

	st, _ := store.Load(storePath)
	p := NewPipeline(st, cfgPath)
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
	p2 := NewPipeline(st2, cfgPath)
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

// TestAddSubscriptionNudgesGlobalTargetWhenDirect regresses the bug where
// fresh installs left GlobalTarget=DIRECT after the first sub was added,
// so MATCH,🚀 Proxy resolved to direct connections. After this nudge, the
// first sub auto-becomes the default proxy choice.
func TestAddSubscriptionNudgesGlobalTargetWhenDirect(t *testing.T) {
	p := newTestPipeline(t)
	// initial: post-migration DIRECT default
	if p.store.Cfg.GlobalTarget != "DIRECT" {
		t.Fatalf("setup: expected GlobalTarget=DIRECT, got %q", p.store.Cfg.GlobalTarget)
	}
	if err := p.AddSubscription(store.Subscription{Name: "doge", URL: "https://x", Enabled: true}); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if got := p.store.Cfg.GlobalTarget; got != "doge-auto" {
		t.Errorf("first sub should nudge GlobalTarget → doge-auto, got %q", got)
	}
	// Second sub must NOT overwrite — user already has a default choice.
	if err := p.AddSubscription(store.Subscription{Name: "boost", URL: "https://y", Enabled: true}); err != nil {
		t.Fatalf("AddSubscription #2: %v", err)
	}
	if got := p.store.Cfg.GlobalTarget; got != "doge-auto" {
		t.Errorf("second sub must not overwrite GlobalTarget, got %q", got)
	}
}

// TestAddSubscriptionPreservesExplicitTarget — if the user has already
// set GlobalTarget to a specific proxy (via `vpnkit target` or TUI), the
// nudge must NOT clobber it on the next AddSubscription.
func TestAddSubscriptionPreservesExplicitTarget(t *testing.T) {
	p := newTestPipeline(t)
	p.store.Cfg.GlobalTarget = "doge-auto" // user already picked
	if err := p.AddSubscription(store.Subscription{Name: "boost", URL: "https://y", Enabled: true}); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if got := p.store.Cfg.GlobalTarget; got != "doge-auto" {
		t.Errorf("user's target must be preserved, got %q", got)
	}
}

// TestAddLocalGroupNudgesGlobalTargetWhenDirect — same first-source nudge
// for users who only use hand-entered local nodes (no subscriptions).
func TestAddLocalGroupNudgesGlobalTargetWhenDirect(t *testing.T) {
	p := newTestPipeline(t)
	if err := p.AddLocalGroup("home"); err != nil {
		t.Fatalf("AddLocalGroup: %v", err)
	}
	if got := p.store.Cfg.GlobalTarget; got != "home-auto" {
		t.Errorf("first local group should nudge GlobalTarget → home-auto, got %q", got)
	}
}

// TestAddDisabledSubscriptionDoesNotSetActiveSource — round-3 regression:
// AddSubscription used to unconditionally claim ActiveSource if the slot
// was empty. If the caller passed Enabled:false (e.g. `vpnkit subs add
// --disabled`), the active source would point at a disabled sub →
// topProxyMembersFor sees no enabled match → 🚀 Proxy degrades to
// [DIRECT] and traffic silently routes direct.
func TestAddDisabledSubscriptionDoesNotSetActiveSource(t *testing.T) {
	p := newTestPipeline(t)
	if err := p.AddSubscription(store.Subscription{Name: "x", URL: "https://x", Enabled: false}); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if p.store.Cfg.ActiveSource != "" {
		t.Errorf("disabled-first-add must NOT set ActiveSource, got %q", p.store.Cfg.ActiveSource)
	}
}

// TestAddSubscriptionSetsActiveSourceWhenEmpty — rc.7 first-source nudge
// for ActiveSource. Mirror of the GlobalTarget nudge: the very first sub
// added (when ActiveSource is empty) should claim the slot.
func TestAddSubscriptionSetsActiveSourceWhenEmpty(t *testing.T) {
	p := newTestPipeline(t)
	if p.store.Cfg.ActiveSource != "" {
		t.Fatalf("setup: ActiveSource should be empty, got %q", p.store.Cfg.ActiveSource)
	}
	if err := p.AddSubscription(store.Subscription{Name: "doge", URL: "https://x", Enabled: true}); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if p.store.Cfg.ActiveSource != "doge" {
		t.Errorf("first sub should set ActiveSource=doge, got %q", p.store.Cfg.ActiveSource)
	}
	// Second sub must not displace.
	if err := p.AddSubscription(store.Subscription{Name: "boost", URL: "https://y", Enabled: true}); err != nil {
		t.Fatalf("AddSubscription #2: %v", err)
	}
	if p.store.Cfg.ActiveSource != "doge" {
		t.Errorf("second sub must not overwrite ActiveSource, got %q", p.store.Cfg.ActiveSource)
	}
}

// TestDeleteActiveSubscriptionClearsActiveSource — when the active sub is
// deleted, ActiveSource must be cleared so the next Assemble's
// firstEnabledSourceName fallback picks a still-valid source instead of
// emitting a 🚀 Proxy with `[<deleted>-auto, <deleted>, DIRECT]` that
// mihomo can't satisfy.
func TestDeleteActiveSubscriptionClearsActiveSource(t *testing.T) {
	p := newTestPipeline(t)
	_ = p.AddSubscription(store.Subscription{Name: "doge", URL: "https://x", Enabled: true})
	_ = p.AddSubscription(store.Subscription{Name: "boost", URL: "https://y", Enabled: true})
	// At this point doge is active (first-source nudge).
	if p.store.Cfg.ActiveSource != "doge" {
		t.Fatalf("setup: ActiveSource should be doge, got %q", p.store.Cfg.ActiveSource)
	}
	if err := p.DeleteSubscription("doge"); err != nil {
		t.Fatalf("DeleteSubscription: %v", err)
	}
	if p.store.Cfg.ActiveSource != "" {
		t.Errorf("deleting active sub should clear ActiveSource, got %q", p.store.Cfg.ActiveSource)
	}
}

// TestToggleDisableActiveSubscriptionClearsActiveSource — same as above
// but via disable instead of delete. A disabled sub isn't in mihomo's
// proxy-groups, so 🚀 Proxy pointing at it would degrade to DIRECT-only.
func TestToggleDisableActiveSubscriptionClearsActiveSource(t *testing.T) {
	p := newTestPipeline(t)
	_ = p.AddSubscription(store.Subscription{Name: "doge", URL: "https://x", Enabled: true})
	if err := p.ToggleSubscriptionEnabled("doge"); err != nil {
		t.Fatalf("ToggleSubscriptionEnabled: %v", err)
	}
	if p.store.Cfg.ActiveSource != "" {
		t.Errorf("disabling active sub should clear ActiveSource, got %q", p.store.Cfg.ActiveSource)
	}
	// Re-enable should NOT auto-restore — user must explicitly re-select.
	if err := p.ToggleSubscriptionEnabled("doge"); err != nil {
		t.Fatalf("ToggleSubscriptionEnabled re-enable: %v", err)
	}
	if p.store.Cfg.ActiveSource != "" {
		t.Errorf("re-enable should not silently restore ActiveSource, got %q", p.store.Cfg.ActiveSource)
	}
}

// TestSetModeValidatesMode — SetMode must reject anything that isn't
// one of the three canonical modes. emitRules quietly falls through to
// ModeRule semantics on garbage input, so without validation a typo in
// a future CLI / config-import path could silently route differently
// than the user intended.
func TestSetModeValidatesMode(t *testing.T) {
	p := newTestPipeline(t)
	for _, ok := range []string{"rule", "global", "direct"} {
		if err := p.SetMode(ok); err != nil {
			t.Errorf("SetMode(%q) should succeed, got %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Rule", "globl", "rules", "RULE"} {
		if err := p.SetMode(bad); err == nil {
			t.Errorf("SetMode(%q) must error, got nil", bad)
		}
	}
}

// TestDeleteActiveLocalGroupClearsActiveSource — local-group equivalent.
func TestDeleteActiveLocalGroupClearsActiveSource(t *testing.T) {
	p := newTestPipeline(t)
	_ = p.AddLocalGroup("home")
	if p.store.Cfg.ActiveSource != "home" {
		t.Fatalf("setup: ActiveSource should be home, got %q", p.store.Cfg.ActiveSource)
	}
	if err := p.DeleteLocalGroup("home", false); err != nil {
		t.Fatalf("DeleteLocalGroup: %v", err)
	}
	if p.store.Cfg.ActiveSource != "" {
		t.Errorf("deleting active local group should clear ActiveSource, got %q", p.store.Cfg.ActiveSource)
	}
}
