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

// cursorPos identifies a navigable row in the proxies view.
// nodeIdx == -1 means the cursor is on the group row itself; otherwise it
// points at a node row inside the expanded group.
type cursorPos struct {
	groupIdx int
	nodeIdx  int
}

// Model is the Proxies tab.
type Model struct {
	groups   map[string]msg.ProxyGroup
	delays   map[string]map[string]int
	order    []string
	cursor   cursorPos
	expanded map[string]bool
}

// New returns an empty Proxies tab model.
func New() Model {
	return Model{
		groups:   map[string]msg.ProxyGroup{},
		delays:   map[string]map[string]int{},
		expanded: map[string]bool{},
		cursor:   cursorPos{0, -1},
	}
}

func (Model) Init() tea.Cmd { return nil }

// Update absorbs ProxiesSnapshot + DelayResults.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	switch ev := message.(type) {
	case msg.ProxiesSnapshot:
		m.groups = ev.Groups
		m.order = sortedSelectableGroups(ev.Groups)
		if m.cursor.groupIdx >= len(m.order) {
			m.cursor = cursorPos{0, -1}
		}
	case msg.DelayResults:
		m.delays[ev.Group] = ev.Results
	}
	return m, nil
}

// MoveDown advances the cursor by one navigable row.
func (m *Model) MoveDown() {
	if len(m.order) == 0 {
		return
	}
	g := m.order[m.cursor.groupIdx]
	if m.cursor.nodeIdx == -1 {
		// On group row. If expanded and has nodes, descend into node 0.
		if m.expanded[g] && len(m.groups[g].All) > 0 {
			m.cursor.nodeIdx = 0
			return
		}
		// Else, advance to next group row.
		if m.cursor.groupIdx < len(m.order)-1 {
			m.cursor.groupIdx++
			m.cursor.nodeIdx = -1
		}
		return
	}
	// On a node row. Advance to next node, or fall through to next group row.
	nodes := m.groups[g].All
	if m.cursor.nodeIdx < len(nodes)-1 {
		m.cursor.nodeIdx++
		return
	}
	if m.cursor.groupIdx < len(m.order)-1 {
		m.cursor.groupIdx++
		m.cursor.nodeIdx = -1
	}
}

// MoveUp moves the cursor up one navigable row.
func (m *Model) MoveUp() {
	if len(m.order) == 0 {
		return
	}
	if m.cursor.nodeIdx > 0 {
		m.cursor.nodeIdx--
		return
	}
	if m.cursor.nodeIdx == 0 {
		// On first node of current group → back to group row.
		m.cursor.nodeIdx = -1
		return
	}
	// On group row. Move to previous group; if it's expanded, land on its
	// last node so navigation feels continuous.
	if m.cursor.groupIdx == 0 {
		return
	}
	m.cursor.groupIdx--
	prev := m.order[m.cursor.groupIdx]
	if m.expanded[prev] && len(m.groups[prev].All) > 0 {
		m.cursor.nodeIdx = len(m.groups[prev].All) - 1
	} else {
		m.cursor.nodeIdx = -1
	}
}

// SelectedGroup returns the group name at the cursor (whether on the group
// row or a node row of that group). Empty when there are no groups.
func (m Model) SelectedGroup() string {
	if len(m.order) == 0 || m.cursor.groupIdx >= len(m.order) {
		return ""
	}
	return m.order[m.cursor.groupIdx]
}

// SelectedNode returns (group, node, true) if the cursor is on a node row.
// Returns (_, _, false) when on a group row or when there are no groups.
func (m Model) SelectedNode() (string, string, bool) {
	if m.cursor.nodeIdx < 0 || len(m.order) == 0 || m.cursor.groupIdx >= len(m.order) {
		return "", "", false
	}
	g := m.order[m.cursor.groupIdx]
	nodes := m.groups[g].All
	if m.cursor.nodeIdx >= len(nodes) {
		return "", "", false
	}
	return g, nodes[m.cursor.nodeIdx], true
}

// ToggleExpand expands or collapses the currently-selected group.
// If the cursor is currently on a node row, it pulls back to the group row
// before collapsing so it doesn't become orphaned.
func (m *Model) ToggleExpand() {
	g := m.SelectedGroup()
	if g == "" {
		return
	}
	m.expanded[g] = !m.expanded[g]
	if !m.expanded[g] && m.cursor.nodeIdx >= 0 {
		m.cursor.nodeIdx = -1
	}
}

// View renders the tab.
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("🚀 Proxies")
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	var rows []string
	rows = append(rows, header, "")
	if len(m.order) == 0 {
		rows = append(rows, "  No proxy groups (mihomo not yet running or no subscription active)")
	}
	for i, g := range m.order {
		group := m.groups[g]
		expanded := m.expanded[g]
		prefix := "  "
		if i == m.cursor.groupIdx && m.cursor.nodeIdx == -1 {
			prefix = cur.Render("▶ ")
		}
		rows = append(rows, fmt.Sprintf("%s%-20s  %-12s  → %s", prefix, group.Name, group.Type, group.Now))
		if expanded {
			for j, node := range group.All {
				delayStr := ""
				if d, ok := m.delays[g][node]; ok {
					delayStr = fmt.Sprintf("%d ms", d)
				}
				marker := "   "
				if node == group.Now {
					marker = "  ✓"
				}
				nodePrefix := "    "
				if i == m.cursor.groupIdx && m.cursor.nodeIdx == j {
					nodePrefix = "  " + cur.Render("▶ ")
				}
				rows = append(rows, fmt.Sprintf("%s%s %-30s  %s", nodePrefix, marker, node, delayStr))
			}
		}
	}
	rows = append(rows, "", "[Enter] expand or switch  [t] delay test  [↑↓] navigate")
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
