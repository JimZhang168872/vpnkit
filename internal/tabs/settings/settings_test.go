package settings

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/extensions"
)

func TestSubMenuNavigation(t *testing.T) {
	m := New(Deps{})
	// On Mihomo Core (no internal nav), ↓ should switch sub-page.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubService {
		t.Errorf("expected SubService after one ↓ on SubCore, got %v", m.SelectedPage())
	}
	// PgDown also switches.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.SelectedPage() != SubController {
		t.Errorf("expected SubController after one PgDown, got %v", m.SelectedPage())
	}
	view := m.View(120, 24)
	if !strings.Contains(view, "Service") || !strings.Contains(view, "Mihomo Core") {
		t.Errorf("submenu missing entries:\n%s", view)
	}
}

func TestPageEnumNames(t *testing.T) {
	expected := []SubPage{SubCore, SubService, SubController, SubRouting, SubRules, SubExtensions, SubLogs, SubCache, SubAbout}
	if len(SubPageNames) != len(expected) {
		t.Fatalf("len(SubPageNames)=%d, want %d", len(SubPageNames), len(expected))
	}
	for _, p := range expected {
		if SubPageNames[p] == "" {
			t.Errorf("missing name for %v", p)
		}
	}
}

// TestArrowKeysDelegateToActiveSubPage covers the case where the user has
// focused the Extensions content panel (via SetFocus from app-level →) and
// then uses ↑/↓ for list navigation.
func TestArrowKeysDelegateToActiveSubPage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{
			{Node: "A", Via: "B"},
			{Node: "C", Via: "D"},
			{Node: "E", Via: "F"},
		},
	})
	m := New(Deps{ExtensionsPath: path})
	// SubExtensions is now at index 5 (Core=0,Service=1,Controller=2,Routing=3,Rules=4,Extensions=5).
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	m.SetFocus(FocusContent)
	// ↓ should now move chains-list cursor (not switch sub-page).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubExtensions {
		t.Errorf("↓ on Extensions+FocusContent should NOT switch sub-page, got %v", m.SelectedPage())
	}
	if m.extensions.row != 1 {
		t.Errorf("Extensions row after ↓: want 1, got %d", m.extensions.row)
	}
	// ↑ back to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.extensions.row != 0 {
		t.Errorf("Extensions row after ↑: want 0, got %d", m.extensions.row)
	}
	// PgUp force-switches sub-page even when content is focused.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.SelectedPage() != SubRules {
		t.Errorf("PgUp on Extensions should switch to SubRules, got %v", m.SelectedPage())
	}
}

// TestFocusToggleInExtensions covers the focus-state model. ←/→ at app
// level translate to SetFocus calls; here we test the model contract: when
// FocusContent is set and the user presses ↓, the extensions list cursor
// moves; when FocusSidebar is set, ↓ switches sub-page.
func TestFocusToggleInExtensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}, {Node: "C", Via: "D"}},
	})
	m := New(Deps{ExtensionsPath: path})
	// Switch to Extensions (now at index 5).
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if m.SelectedPage() != SubExtensions {
		t.Fatalf("setup: expected SubExtensions, got %v", m.SelectedPage())
	}
	if m.Focus() != FocusSidebar {
		t.Errorf("default focus on entering Extensions should be Sidebar, got %v", m.Focus())
	}
	// App-level handler shifts focus → content.
	m.SetFocus(FocusContent)
	if m.Focus() != FocusContent {
		t.Fatalf("SetFocus(Content) failed, got %v", m.Focus())
	}
	// ↓ now navigates extensions list (because focus = content).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.extensions.row != 1 {
		t.Errorf("↓ on Extensions+FocusContent should move list cursor to 1, got %d", m.extensions.row)
	}
	if m.SelectedPage() != SubExtensions {
		t.Errorf("↓ on Extensions+FocusContent should NOT switch sub-page, got %v", m.SelectedPage())
	}
	// Shift back to sidebar.
	m.SetFocus(FocusSidebar)
	// ↓ on Extensions+FocusSidebar switches sub-page (sidebar nav).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubLogs {
		t.Errorf("↓ on Extensions+FocusSidebar should switch sub-page to SubLogs, got %v", m.SelectedPage())
	}
}

// TestFocusResetsOnSubPageChange ensures focus snaps back to sidebar whenever
// the user navigates to a different sub-page (so they don't end up with
// FocusContent on a sub-page that has no content panel).
func TestFocusResetsOnSubPageChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	m := New(Deps{ExtensionsPath: path})
	// Navigate to Extensions (now at index 5).
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	m.SetFocus(FocusContent)
	// PgDown forces sub-page change → focus should reset to Sidebar.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.SelectedPage() != SubLogs {
		t.Errorf("expected SubLogs, got %v", m.SelectedPage())
	}
	if m.Focus() != FocusSidebar {
		t.Errorf("focus should reset to Sidebar on sub-page change, got %v", m.Focus())
	}
}

// TestSubPageOwnsContent reports whether the active sub-page can accept
// FocusContent (used by app-level → handler).
func TestSubPageOwnsContent(t *testing.T) {
	m := New(Deps{})
	if m.SubPageOwnsContent() {
		t.Errorf("default sub-page (Core) should NOT own content")
	}
	// Extensions is now at index 5.
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if !m.SubPageOwnsContent() {
		t.Errorf("Extensions sub-page should own content")
	}
}

// TestPgUpDownInWrapAroundEnd asserts PgDown stops at last page and PgUp at first.
func TestPgUpDownInWrapAroundEnd(t *testing.T) {
	m := New(Deps{})
	// PgDown until end.
	for i := 0; i < int(NumSubPages)+5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if m.SelectedPage() != SubAbout {
		t.Errorf("PgDown spam should clamp to SubAbout, got %v", m.SelectedPage())
	}
	for i := 0; i < int(NumSubPages)+5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	}
	if m.SelectedPage() != SubCore {
		t.Errorf("PgUp spam should clamp to SubCore, got %v", m.SelectedPage())
	}
}
