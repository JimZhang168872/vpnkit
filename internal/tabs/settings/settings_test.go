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
