// Package rules implements the Rules tab (rule list + providers status).
package rules

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
)

// Model is the Rules tab.
type Model struct {
	rules     []msg.RuleEntry
	providers []msg.RuleProviderEntry
	filter    string
}

func New() Model { return Model{} }

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if ev, ok := message.(msg.RulesSnapshot); ok {
		m.rules = ev.Rules
		m.providers = ev.Providers
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

	rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Rules"))
	for _, r := range m.rules {
		if m.filter != "" && !strings.Contains(r.Payload, m.filter) && !strings.Contains(r.Type, m.filter) && !strings.Contains(r.Proxy, m.filter) {
			continue
		}
		rows = append(rows, fmt.Sprintf("  %-14s  %-30s  → %s", r.Type, truncate(r.Payload, 30), r.Proxy))
	}
	rows = append(rows, "", "[/] filter  [u] refresh providers")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
