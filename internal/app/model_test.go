package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	tabsettings "vpnkit/internal/tabs/settings"
)

func TestTabSwitching(t *testing.T) {
	m := NewModel(nil, tabsettings.Deps{}, nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	// Key "3" → TabSources (index 2).
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if m.activeTab != TabSources {
		t.Errorf("expected TabSources after pressing 3, got %v", m.activeTab)
	}
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.activeTab != TabRules {
		t.Errorf("Tab cycle failed: expected TabRules, got %v", m.activeTab)
	}

	view := m.View()
	if !strings.Contains(view, "Rules") || !strings.Contains(view, "vpnkit") {
		t.Errorf("view missing chrome:\n%s", view)
	}
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	mm, cmd := m.Update(msg)
	return mm.(Model), cmd
}

// TestAppFocusDefaultIsTabBody — existing keyboards keep working without
// users having to learn the new ← / → focus flow.
func TestAppFocusDefaultIsTabBody(t *testing.T) {
	m := NewModel(nil, tabsettings.Deps{}, nil)
	if m.AppFocus() != FocusTabBody {
		t.Errorf("default appFocus should be TabBody, got %v", m.AppFocus())
	}
}

// TestLeftArrowEntersMainSidebar — pressing ← anywhere with appFocus=TabBody
// shifts focus to the main sidebar (where ↑/↓ then cycles top tabs).
func TestLeftArrowEntersMainSidebar(t *testing.T) {
	m := NewModel(nil, tabsettings.Deps{}, nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.AppFocus() != FocusMainSidebar {
		t.Errorf("← should shift to MainSidebar, got %v", m.AppFocus())
	}
}

// TestRightArrowReturnsToTabBody — symmetric: → from MainSidebar goes back
// to TabBody.
func TestRightArrowReturnsToTabBody(t *testing.T) {
	m := NewModel(nil, tabsettings.Deps{}, nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyLeft})  // go to sidebar
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRight}) // back to body
	if m.AppFocus() != FocusTabBody {
		t.Errorf("→ on MainSidebar should return to TabBody, got %v", m.AppFocus())
	}
}

// TestUpDownOnMainSidebarCyclesTabs — the core "user wants ↑/↓ to navigate
// top tabs" feature.
func TestUpDownOnMainSidebarCyclesTabs(t *testing.T) {
	m := NewModel(nil, tabsettings.Deps{}, nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyLeft}) // sidebar focus
	if m.activeTab != TabDashboard {
		t.Fatalf("setup: expected TabDashboard, got %v", m.activeTab)
	}
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.activeTab != TabGroups {
		t.Errorf("↓ on MainSidebar should advance to TabGroups, got %v", m.activeTab)
	}
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.activeTab != TabDashboard {
		t.Errorf("↑ on MainSidebar should return to TabDashboard, got %v", m.activeTab)
	}
	// Spam clamp.
	for i := 0; i < int(NumTabs)+5; i++ {
		m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.activeTab != NumTabs-1 {
		t.Errorf("↓ spam should clamp at last tab, got %v", m.activeTab)
	}
}

// TestUpDownOnTabBodyDelegates — when focus is on TabBody, ↑/↓ keep doing
// what they used to (move the active tab's cursor), not cycle top tabs.
func TestUpDownOnTabBodyDelegates(t *testing.T) {
	m := NewModel(nil, tabsettings.Deps{}, nil)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	// default focus = TabBody; activeTab = Dashboard.
	// Switch to Rules tab (has a cursor) and test that ↓ doesn't cycle tabs.
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	if m.activeTab != TabRules {
		t.Fatalf("setup: expected TabRules (key 4), got %v", m.activeTab)
	}
	// ↓ should NOT change activeTab (since focus = TabBody).
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.activeTab != TabRules {
		t.Errorf("↓ on TabBody should NOT cycle tabs, got activeTab=%v", m.activeTab)
	}
}
