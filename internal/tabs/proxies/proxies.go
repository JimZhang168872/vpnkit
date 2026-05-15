// Package proxies implements the Proxies tab (group/node selection + delay tests).
package proxies

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
)

// Model is the Proxies tab.
type Model struct {
	groups   map[string]msg.ProxyGroup
	delays   map[string]map[string]int
	order    []string
	cursor   int
	expanded map[string]bool
}

// New returns an empty Proxies tab model.
func New() Model {
	return Model{
		groups:   map[string]msg.ProxyGroup{},
		delays:   map[string]map[string]int{},
		expanded: map[string]bool{},
	}
}

func (Model) Init() tea.Cmd { return nil }

// Update absorbs ProxiesSnapshot + DelayResults.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	switch ev := message.(type) {
	case msg.ProxiesSnapshot:
		m.groups = ev.Groups
		m.order = sortedSelectableGroups(ev.Groups)
		if m.cursor >= len(m.order) {
			m.cursor = 0
		}
	case msg.DelayResults:
		m.delays[ev.Group] = ev.Results
	}
	return m, nil
}

func (m *Model) MoveDown() {
	if m.cursor < len(m.order)-1 {
		m.cursor++
	}
}

func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m Model) SelectedGroup() string {
	if m.cursor >= len(m.order) {
		return ""
	}
	return m.order[m.cursor]
}

func (m *Model) ToggleExpand() {
	g := m.SelectedGroup()
	if g != "" {
		m.expanded[g] = !m.expanded[g]
	}
}

// View renders the tab.
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Proxies")
	var rows []string
	rows = append(rows, header, "")
	if len(m.order) == 0 {
		rows = append(rows, "  No proxy groups (mihomo not yet running or no subscription active)")
	}
	for i, g := range m.order {
		group := m.groups[g]
		expanded := m.expanded[g]
		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ ")
		}
		rows = append(rows, fmt.Sprintf("%s%-20s  %-12s  → %s", prefix, group.Name, group.Type, group.Now))
		if expanded {
			for _, node := range group.All {
				delayStr := ""
				if d, ok := m.delays[g][node]; ok {
					delayStr = fmt.Sprintf("%d ms", d)
				}
				marker := "   "
				if node == group.Now {
					marker = "  ✓"
				}
				rows = append(rows, fmt.Sprintf("    %s %-30s  %s", marker, node, delayStr))
			}
		}
	}
	rows = append(rows, "", "[Enter] expand/switch  [t] delay test  [↑↓] navigate")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func sortedSelectableGroups(in map[string]msg.ProxyGroup) []string {
	var names []string
	for k, g := range in {
		if g.Type == "Direct" || g.Type == "Reject" || g.Type == "Pass" {
			continue
		}
		if len(g.All) <= 0 {
			continue
		}
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
