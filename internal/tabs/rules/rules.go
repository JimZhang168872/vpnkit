// Package rules implements the Rules tab (rule list + providers status).
package rules

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/tabs/viewport"
)

// Model is the Rules tab.
type Model struct {
	rules       []msg.RuleEntry
	providers   []msg.RuleProviderEntry
	filter      string
	filterInput textinput.Model
	filtering   bool
	cursor      int // index into the filtered rules slice
}

// MoveDown advances the cursor by one filtered row, clamped to the last row.
func (m *Model) MoveDown() {
	max := len(m.filtered()) - 1
	if m.cursor < max {
		m.cursor++
	}
}

// MoveUp moves the cursor up one filtered row, clamped at 0.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// PageSize is the cursor jump for MovePageUp/MovePageDown. Chosen as a
// constant rather than view-height-aware so the model stays decoupled
// from rendering; 10 is roughly a "screen" on a typical 24-row terminal
// once header/footer/providers are accounted for.
const PageSize = 10

// MovePageDown jumps the cursor PageSize rows downward, clamped.
func (m *Model) MovePageDown() {
	max := len(m.filtered()) - 1
	if max < 0 {
		return
	}
	m.cursor += PageSize
	if m.cursor > max {
		m.cursor = max
	}
}

// MovePageUp jumps the cursor PageSize rows upward, clamped.
func (m *Model) MovePageUp() {
	m.cursor -= PageSize
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// filtered returns the post-filter slice of rules; computed lazily on each
// call so cursor + view always see the same data.
func (m Model) filtered() []msg.RuleEntry {
	if m.filter == "" {
		return m.rules
	}
	out := make([]msg.RuleEntry, 0, len(m.rules))
	for _, r := range m.rules {
		if strings.Contains(r.Payload, m.filter) ||
			strings.Contains(r.Type, m.filter) ||
			strings.Contains(r.Proxy, m.filter) {
			out = append(out, r)
		}
	}
	return out
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "filter (type, payload or proxy)…"
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

// ProviderNames returns the names of all currently-known rule providers.
func (m Model) ProviderNames() []string {
	out := make([]string, 0, len(m.providers))
	for _, p := range m.providers {
		out = append(out, p.Name)
	}
	return out
}

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if ev, ok := message.(msg.RulesSnapshot); ok {
		m.rules = ev.Rules
		m.providers = ev.Providers
		// Clamp cursor in case the rule count shrank.
		max := len(m.filtered()) - 1
		if m.cursor > max {
			if max < 0 {
				m.cursor = 0
			} else {
				m.cursor = max
			}
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

func (m *Model) SetFilter(s string) { m.filter = s }

func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused = View + focus dot.
func (m Model) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Rules")
	rows := []string{header, ""}

	innerWidthProv := width - 6
	if innerWidthProv < 10 {
		innerWidthProv = 10
	}
	if len(m.providers) > 0 {
		rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Rule Providers"))
		for _, p := range m.providers {
			line := fmt.Sprintf("%-20s  %-8s  count=%d  updated=%s",
				p.Name, p.Behavior, p.RuleCount, p.UpdatedAt)
			rows = append(rows, "  "+viewport.TruncateDisplay(line, innerWidthProv))
		}
		rows = append(rows, "")
	}

	filtered := m.filtered()

	// Reserve rows: header(1) + blank + "Rules"(1) + filter-line(2) + footer(1)
	// + padding(2) + provider block (variable).
	providerRows := 0
	if len(m.providers) > 0 {
		providerRows = 1 + len(m.providers) + 1 // header + N rows + blank
	}
	maxList := height - providerRows - 8
	if maxList < 3 {
		maxList = 3
	}
	start, end := viewport.Window(len(filtered), m.cursor, maxList)
	indicator := viewport.Indicator(start, len(filtered), maxList, m.cursor)

	rulesHeader := lipgloss.NewStyle().Bold(true).Render("Rules")
	if indicator != "" {
		rulesHeader += "   " + lipgloss.NewStyle().Faint(true).Render(indicator)
	}
	rows = append(rows, rulesHeader)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	// Hard-truncate each row to the body's inner width so lipgloss never
	// soft-wraps a long rule onto a second line — wrapping doubles the row
	// count and overflows Height, which is what was pushing the sidebar
	// off-screen on the Rules tab.
	innerWidth := width - 6 // -4 padding (2*2) - 2 cursor/marker prefix slack
	if innerWidth < 10 {
		innerWidth = 10
	}
	for i := start; i < end; i++ {
		r := filtered[i]
		line := fmt.Sprintf("%-14s  %-30s  → %s", r.Type, truncate(r.Payload, 30), r.Proxy)
		line = viewport.TruncateDisplay(line, innerWidth)
		if i == m.cursor {
			rows = append(rows, cursorStyle.Render("▶ "+line))
		} else {
			rows = append(rows, "  "+line)
		}
	}
	if m.filtering {
		rows = append(rows, "", m.filterInput.View(), "[Enter] apply  [Esc] clear")
	} else {
		rows = append(rows, "", "[/] filter  [u] refresh providers  [↑↓] navigate")
	}
	// MaxHeight enforces clip at body height — without it, lipgloss lets
	// content extend below the box, and JoinHorizontal then misaligns the
	// sidebar.
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).
		Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
