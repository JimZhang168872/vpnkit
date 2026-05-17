// Package profiles implements the Profiles tab (subscription CRUD).
// TODO(v1-phase8): This tab will be replaced by the Groups/Sources tab in Phase 8 TUI restructure.
package profiles

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/tabs/viewport"
)

// Profile is a stub type used until Phase 8 replaces this tab.
// TODO(v1-phase8): wire to store.Subscription after TUI restructure.
type Profile struct {
	Name        string
	URL         string
	NodeCount   int
}

// Model is the Profiles tab.
type Model struct {
	list   []Profile
	active string
	cursor int
}

// New builds an empty Profiles tab.
// TODO(v1-phase8): mgr param removed; wire Pipeline.LocalNodes/Subscriptions after TUI restructure.
func New(_ any) Model {
	return Model{}
}

// SetProfiles refreshes the rendered list (called when manager state changes).
func (m *Model) SetProfiles(list []Profile, active string) {
	m.list = list
	m.active = active
	if m.cursor >= len(m.list) {
		m.cursor = 0
	}
}

// Selected returns the currently-highlighted profile.
func (m Model) Selected() Profile {
	if m.cursor >= len(m.list) {
		return Profile{}
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

// PageSize controls how far MovePageUp/Down jump.
const PageSize = 10

// MovePageDown jumps the cursor PageSize rows downward, clamped to the last row.
func (m *Model) MovePageDown() {
	max := len(m.list) - 1
	if max < 0 {
		return
	}
	m.cursor += PageSize
	if m.cursor > max {
		m.cursor = max
	}
}

// MovePageUp jumps the cursor PageSize rows upward, clamped at 0.
func (m *Model) MovePageUp() {
	m.cursor -= PageSize
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update absorbs tea.Msg.
func (m Model) Update(_ tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// View renders the tab; defaults to focused for direct callers (tests).
func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused = View + focus dot.
func (m Model) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("📋 Profiles")
	var rows []string
	if len(m.list) == 0 {
		rows = append(rows, header, "",
			"  Subscriptions now managed via Settings → Sources",
			"  TODO(v1-phase8): Groups/Sources tab coming in Phase 8",
			"", "[↑↓] navigate")
		return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
	}
	// Reserve: header(1) + blank + footer(1) + padding(2) ≈ 5.
	maxRows := height - 5
	if maxRows < 3 {
		maxRows = 3
	}
	innerWidth := width - 6
	if innerWidth < 20 {
		innerWidth = 20
	}
	start, end := viewport.Window(len(m.list), m.cursor, maxRows)
	indicator := viewport.Indicator(start, len(m.list), maxRows, m.cursor)
	titleLine := header
	if indicator != "" {
		titleLine += "   " + lipgloss.NewStyle().Faint(true).Render(indicator)
	}
	rows = append(rows, titleLine, "")
	for i := start; i < end; i++ {
		p := m.list[i]
		marker := "  "
		if p.Name == m.active {
			marker = "⭐ "
		}
		row := fmt.Sprintf("%s%-12s  %-40s  nodes=%d", marker, p.Name, truncate(p.URL, 40), p.NodeCount)
		row = viewport.TruncateDisplay(row, innerWidth)
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ " + row)
		} else {
			row = "  " + row
		}
		rows = append(rows, row)
	}
	rows = append(rows, "", "[↑↓] navigate")
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).
		Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
