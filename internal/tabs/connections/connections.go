// Package connections implements the Connections tab (real-time connection table).
package connections

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/tabs/viewport"
)

// Model is the Connections tab.
type Model struct {
	items       []msg.ConnectionItem
	totalUp     int64
	totalDn     int64
	filter      string
	cursor      int
	filterInput textinput.Model
	filtering   bool
}

// New returns an empty tab model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "filter (host or rule)…"
	ti.Prompt = "/ "
	ti.CharLimit = 64
	return Model{filterInput: ti}
}

func (Model) Init() tea.Cmd { return nil }

// IsFiltering reports whether the filter input is currently focused.
func (m Model) IsFiltering() bool { return m.filtering }

// StartFilter focuses the input and switches the tab into filter mode.
func (m *Model) StartFilter() tea.Cmd {
	m.filtering = true
	m.filterInput.SetValue(m.filter)
	return m.filterInput.Focus()
}

// Update absorbs ConnectionsSnapshot and filter-input key events.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if ev, ok := message.(msg.ConnectionsSnapshot); ok {
		m.items = ev.Items
		m.totalUp = ev.UploadTotal
		m.totalDn = ev.DownloadTotal
		if m.cursor >= len(m.items) {
			m.cursor = 0
		}
		return m, nil
	}
	if m.filtering {
		if km, ok := message.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyEsc:
				m.filterInput.Blur()
				m.filterInput.SetValue("")
				m.filter = ""
				m.filtering = false
				return m, nil
			case tea.KeyEnter:
				m.filterInput.Blur()
				m.filtering = false
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(message)
		m.filter = m.filterInput.Value()
		return m, cmd
	}
	return m, nil
}

// SetFilter changes the substring filter.
func (m *Model) SetFilter(s string) { m.filter = s }

// MoveUp / MoveDown navigate filtered rows.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}
func (m *Model) MoveDown() {
	visible := m.visible()
	if m.cursor < len(visible)-1 {
		m.cursor++
	}
}

// SelectedID returns the highlighted connection ID (empty if none).
func (m Model) SelectedID() string {
	visible := m.visible()
	if m.cursor >= len(visible) {
		return ""
	}
	return visible[m.cursor].ID
}

func (m Model) visible() []msg.ConnectionItem {
	if m.filter == "" {
		return m.items
	}
	var out []msg.ConnectionItem
	for _, it := range m.items {
		if strings.Contains(it.Host, m.filter) || strings.Contains(it.Rule, m.filter) {
			out = append(out, it)
		}
	}
	return out
}

// View renders the table.
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Connections")
	stats := fmt.Sprintf("  ↑ %s    ↓ %s    %d active", human(m.totalUp), human(m.totalDn), len(m.items))
	visible := m.visible()
	// Reserve: header(1) + stats(1) + blank + colhead(1) + blank + footer(1) + padding(2) ≈ 7.
	maxRows := height - 7
	if maxRows < 3 {
		maxRows = 3
	}
	start, end := viewport.Window(len(visible), m.cursor, maxRows)
	indicator := viewport.Indicator(start, len(visible), maxRows, m.cursor)

	colHead := fmt.Sprintf("  %-30s  %-6s  %-12s  %-12s  %s", "HOST", "PORT", "UP", "DOWN", "RULE")
	if indicator != "" {
		colHead += "   " + lipgloss.NewStyle().Faint(true).Render(indicator)
	}
	rows := []string{header, stats, "", colHead}
	for i := start; i < end; i++ {
		it := visible[i]
		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ ")
		}
		rows = append(rows, fmt.Sprintf("%s%-30s  %-6s  %-12s  %-12s  %s",
			prefix, truncate(it.Host, 30), it.Port, human(it.Upload), human(it.Download), it.Rule))
	}
	if m.filtering {
		rows = append(rows, "", m.filterInput.View(), "[Enter] apply  [Esc] clear")
	} else {
		rows = append(rows, "", "[/] filter  [x] close selected  [↑↓] navigate")
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func human(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
	)
	switch {
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
