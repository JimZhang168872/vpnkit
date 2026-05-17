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
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Rules")
	rows := []string{header, ""}

	if len(m.providers) > 0 {
		rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Rule Providers"))
		for _, p := range m.providers {
			rows = append(rows, fmt.Sprintf("  %-20s  %-8s  count=%d  updated=%s",
				p.Name, p.Behavior, p.RuleCount, p.UpdatedAt))
		}
		rows = append(rows, "")
	}

	// Build filtered list first so we can window it.
	filtered := make([]msg.RuleEntry, 0, len(m.rules))
	for _, r := range m.rules {
		if m.filter != "" && !strings.Contains(r.Payload, m.filter) && !strings.Contains(r.Type, m.filter) && !strings.Contains(r.Proxy, m.filter) {
			continue
		}
		filtered = append(filtered, r)
	}

	// Reserve rows: header(1) + blank + "Rules"(1) + blank + footer(1-2) + padding(2)
	// + Rule Providers section (variable). Estimate provider rows used:
	providerRows := 0
	if len(m.providers) > 0 {
		providerRows = 1 + len(m.providers) + 1 // header + N rows + blank
	}
	maxList := height - providerRows - 8
	if maxList < 3 {
		maxList = 3
	}
	start, end := viewport.Window(len(filtered), 0, maxList)
	indicator := viewport.Indicator(start, len(filtered), maxList, 0)

	rulesHeader := lipgloss.NewStyle().Bold(true).Render("Rules")
	if indicator != "" {
		rulesHeader += "   " + lipgloss.NewStyle().Faint(true).Render(indicator)
	}
	rows = append(rows, rulesHeader)
	for i := start; i < end; i++ {
		r := filtered[i]
		rows = append(rows, fmt.Sprintf("  %-14s  %-30s  → %s", r.Type, truncate(r.Payload, 30), r.Proxy))
	}
	if m.filtering {
		rows = append(rows, "", m.filterInput.View(), "[Enter] apply  [Esc] clear")
	} else {
		rows = append(rows, "", "[/] filter  [u] refresh providers")
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
