package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTabSwitching(t *testing.T) {
	m := NewModel(nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if m.activeTab != TabProfiles {
		t.Errorf("expected Profiles, got %v", m.activeTab)
	}
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.activeTab != TabConnections {
		t.Errorf("Tab cycle failed: %v", m.activeTab)
	}

	view := m.View()
	if !strings.Contains(view, "Profiles") || !strings.Contains(view, "vpnkit") {
		t.Errorf("view missing chrome:\n%s", view)
	}
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	mm, cmd := m.Update(msg)
	return mm.(Model), cmd
}
