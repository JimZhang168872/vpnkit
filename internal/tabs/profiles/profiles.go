// Package profiles implements the Profiles tab (subscription CRUD).
package profiles

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/profiles"
)

// Model is the Profiles tab.
type Model struct {
	mgr    *profiles.Manager
	list   []profiles.Profile
	active string
	cursor int
}

// New builds an empty Profiles tab. The owner injects a *profiles.Manager.
func New(mgr *profiles.Manager) Model {
	return Model{mgr: mgr}
}

// SetProfiles refreshes the rendered list (called when manager state changes).
func (m *Model) SetProfiles(list []profiles.Profile, active string) {
	m.list = list
	m.active = active
	if m.cursor >= len(m.list) {
		m.cursor = 0
	}
}

// Selected returns the currently-highlighted profile.
func (m Model) Selected() profiles.Profile {
	if m.cursor >= len(m.list) {
		return profiles.Profile{}
	}
	return m.list[m.cursor]
}

// MoveDown / MoveUp control the cursor.
func (m *Model) MoveDown() {
	if m.cursor < len(m.list)-1 {
		m.cursor++
	}
}
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update absorbs tea.Msg.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	switch ev := message.(type) {
	case msg.ProfileUpdated:
		if m.mgr != nil {
			m.list = m.mgr.All()
			m.active = m.mgr.Active()
		}
	case msg.ProfileError:
		_ = ev
	}
	return m, nil
}

// View renders the tab.
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("📋 Profiles")
	var rows []string
	rows = append(rows, header, "")
	if len(m.list) == 0 {
		rows = append(rows, "  No subscriptions yet — press 'a' to add")
	}
	for i, p := range m.list {
		marker := "  "
		if p.Name == m.active {
			marker = "⭐ "
		}
		row := fmt.Sprintf("%s%-12s  %-40s  nodes=%d", marker, p.Name, truncate(p.URL, 40), p.NodeCount)
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ " + row)
		} else {
			row = "  " + row
		}
		rows = append(rows, row)
	}
	rows = append(rows, "", "[a] add  [u] update  [Enter] activate  [d] delete  [↑↓] navigate")
	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
