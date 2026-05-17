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
	expected := []SubPage{SubCore, SubService, SubController, SubRules, SubExtensions, SubLogs, SubCache, SubAbout}
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
// explicitly focused the Extensions content panel via → and then uses ↑/↓
// for list navigation. The focus model means ↑/↓ only reaches the
// sub-page's list when FocusContent is active; pressing ↑/↓ in
// FocusSidebar still switches sub-page.
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
	// Switch to Extensions via PgDown × 4 (SubCore=0 → SubExtensions=4).
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	// Switch focus into content via →.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Focus() != FocusContent {
		t.Fatalf("setup: focus should be Content")
	}
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
	// PgUp on Extensions is the "force exit" — switches sub-page even when
	// content is focused.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.SelectedPage() != SubRules {
		t.Errorf("PgUp on Extensions should switch to SubRules, got %v", m.SelectedPage())
	}
}

// TestLeftRightArrowsSwitchSubPage covers the user-reported Settings ←/→
// "卡住" bug: with no handler for tea.KeyLeft/KeyRight the keys silently
// no-op, which feels frozen. On sub-pages without internal panels, ←/→
// still switch sub-page (mirrors ↑/↓).
func TestLeftRightArrowsSwitchSubPage(t *testing.T) {
	m := New(Deps{})
	// On SubCore (no internal nav), → should switch to SubService.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.SelectedPage() != SubService {
		t.Errorf("expected SubService after →, got %v", m.SelectedPage())
	}
	// ← should go back to SubCore.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.SelectedPage() != SubCore {
		t.Errorf("expected SubCore after ←, got %v", m.SelectedPage())
	}
}

// TestFocusToggleInExtensions: on the Extensions sub-page, →/← shift focus
// between the sub-sidebar and the content panel (so the user knows which
// panel ↑/↓ will affect). PgUp/PgDn always switches sub-page even when
// content is focused (force exit).
func TestFocusToggleInExtensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}, {Node: "C", Via: "D"}},
	})
	m := New(Deps{ExtensionsPath: path})
	// Switch to Extensions.
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if m.SelectedPage() != SubExtensions {
		t.Fatalf("setup: expected SubExtensions, got %v", m.SelectedPage())
	}
	if m.Focus() != FocusSidebar {
		t.Errorf("default focus on entering Extensions should be Sidebar, got %v", m.Focus())
	}
	// → moves focus to content (does NOT switch sub-page).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Focus() != FocusContent {
		t.Errorf("after → on Extensions, focus should be Content, got %v", m.Focus())
	}
	if m.SelectedPage() != SubExtensions {
		t.Errorf("→ on Extensions should NOT switch sub-page, got %v", m.SelectedPage())
	}
	// ↓ now navigates extensions list (not sub-page).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.extensions.row != 1 {
		t.Errorf("↓ on Extensions+FocusContent should move list cursor to 1, got %d", m.extensions.row)
	}
	// ← returns focus to sidebar (does NOT switch sub-page).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Focus() != FocusSidebar {
		t.Errorf("after ← on Extensions+FocusContent, focus should be Sidebar, got %v", m.Focus())
	}
	if m.SelectedPage() != SubExtensions {
		t.Errorf("← on Extensions+FocusContent should NOT switch sub-page, got %v", m.SelectedPage())
	}
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
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // focus content
	if m.Focus() != FocusContent {
		t.Fatalf("setup: focus should be Content")
	}
	// PgDown forces sub-page change → focus should reset to Sidebar.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.SelectedPage() != SubLogs {
		t.Errorf("expected SubLogs, got %v", m.SelectedPage())
	}
	if m.Focus() != FocusSidebar {
		t.Errorf("focus should reset to Sidebar on sub-page change, got %v", m.Focus())
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
