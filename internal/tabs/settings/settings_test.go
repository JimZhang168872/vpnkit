package settings

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSubMenuNavigation(t *testing.T) {
	m := New(Deps{})
	// On Mihomo Core (no internal nav), ↓ should switch sub-page.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubService {
		t.Errorf("expected SubService after one ↓ on SubCore, got %v", m.SelectedPage())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubController {
		t.Errorf("expected SubController after one ↓, got %v", m.SelectedPage())
	}
	view := m.View(120, 24)
	if !strings.Contains(view, "Service") || !strings.Contains(view, "Mihomo Core") {
		t.Errorf("submenu missing entries:\n%s", view)
	}
}

func TestPageEnumNames(t *testing.T) {
	expected := []SubPage{SubCore, SubService, SubController, SubRouting, SubRules, SubCache, SubAbout}
	if len(SubPageNames) != len(expected) {
		t.Fatalf("len(SubPageNames)=%d, want %d", len(SubPageNames), len(expected))
	}
	for _, p := range expected {
		if SubPageNames[p] == "" {
			t.Errorf("missing name for %v", p)
		}
	}
}

// TestFocusResetsOnSubPageChange ensures focus snaps back to sidebar whenever
// the user navigates to a different sub-page (so they don't end up with
// FocusContent on a sub-page that has no content panel).
func TestFocusResetsOnSubPageChange(t *testing.T) {
	m := New(Deps{})
	// Navigate to Routing (own-arrows page).
	for i := 0; i < int(SubRouting); i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.SelectedPage() != SubRouting {
		t.Fatalf("setup: expected SubRouting, got %v", m.SelectedPage())
	}
	m.SetFocus(FocusSidebar)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SelectedPage() != SubRules {
		t.Errorf("expected SubRules, got %v", m.SelectedPage())
	}
	if m.Focus() != FocusSidebar {
		t.Errorf("focus should remain Sidebar on sub-page change, got %v", m.Focus())
	}
}

// TestSubPageOwnsContent reports whether the active sub-page can accept
// FocusContent (used by app-level → handler).
func TestSubPageOwnsContent(t *testing.T) {
	m := New(Deps{})
	if m.SubPageOwnsContent() {
		t.Errorf("default sub-page (Core) should NOT own content")
	}
	// Routing owns arrows.
	for i := 0; i < int(SubRouting); i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !m.SubPageOwnsContent() {
		t.Errorf("Routing sub-page should own content")
	}
}

// TestArrowsClampAtEnds asserts ↓ stops at last page and ↑ at first.
func TestArrowsClampAtEnds(t *testing.T) {
	m := New(Deps{})
	for i := 0; i < int(NumSubPages)+5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.SelectedPage() != SubAbout {
		t.Errorf("↓ spam should clamp to SubAbout, got %v", m.SelectedPage())
	}
	for i := 0; i < int(NumSubPages)+5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.SelectedPage() != SubCore {
		t.Errorf("↑ spam should clamp to SubCore, got %v", m.SelectedPage())
	}
}
