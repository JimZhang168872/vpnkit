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

// TestArrowKeysDelegateToActiveSubPage is the regression for Bug A:
// Settings.Update used to intercept tea.KeyUp/Down at the parent level
// and short-circuit, so any sub-page that wanted ↑/↓ for its own list
// navigation (Extensions chains/groups) never received the key. After
// the fix, ↑/↓ must pass through to sub-pages that own list navigation
// (currently only SubExtensions).
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
	if m.SelectedPage() != SubExtensions {
		t.Fatalf("expected SubExtensions, got %v", m.SelectedPage())
	}
	// Initial row = 0. ↓ should move chains-list cursor, NOT switch sub-page.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubExtensions {
		t.Errorf("↓ on Extensions should NOT switch sub-page, got %v", m.SelectedPage())
	}
	if m.extensions.row != 1 {
		t.Errorf("Extensions row after ↓: want 1, got %d", m.extensions.row)
	}
	// ↑ should bring it back to 0 without switching sub-page.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.extensions.row != 0 {
		t.Errorf("Extensions row after ↑: want 0, got %d", m.extensions.row)
	}
	if m.SelectedPage() != SubExtensions {
		t.Errorf("↑ on Extensions should NOT switch sub-page, got %v", m.SelectedPage())
	}
	// PgUp on Extensions is the "force exit" — switches sub-page even though
	// the sub-page owns ↑/↓.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.SelectedPage() != SubRules {
		t.Errorf("PgUp on Extensions should switch to SubRules, got %v", m.SelectedPage())
	}
}

// TestLeftRightArrowsSwitchSubPage covers the user-reported Settings ←/→
// "卡住" bug: with no handler for tea.KeyLeft/KeyRight the keys silently
// no-op, which feels frozen. We mirror ↑/↓ semantics on ←/→ so the keys
// always do something visible.
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
