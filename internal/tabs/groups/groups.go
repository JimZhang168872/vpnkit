// Package groups implements the Groups tab: a read-only list of all proxy groups
// (subscription groups + the local-nodes group) with node detail on the right.
package groups

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/viewport"
)

// SubNode is one proxy node's display info.
type SubNode struct {
	Name   string
	Proto  string
	Server string
	Port   int
}

// Deps holds the data providers for the Groups tab.
type Deps struct {
	// GetSubs returns the current subscription list (names, enabled flag, node count).
	GetSubs func() []store.Subscription
	// GetSubNodes returns the cached node list for one subscription by name.
	// Returns nil if not yet fetched.
	GetSubNodes func(name string) []SubNode
	// GetLocalNodes returns the current local nodes.
	GetLocalNodes func() []SubNode
}

// Model is the Groups tab.
type Model struct {
	deps   Deps
	groups []groupEntry
	cursor int
}

type groupEntry struct {
	name  string
	kind  string // "subscription" | "local"
	nodes []SubNode
}

// New returns an empty Groups tab model.
func New(deps Deps) Model { return Model{deps: deps} }

// Refresh rebuilds the group list from current deps data.
func (m *Model) Refresh() {
	m.groups = nil
	if m.deps.GetSubs != nil {
		for _, s := range m.deps.GetSubs() {
			var nodes []SubNode
			if m.deps.GetSubNodes != nil {
				nodes = m.deps.GetSubNodes(s.Name)
			}
			m.groups = append(m.groups, groupEntry{name: s.Name, kind: "subscription", nodes: nodes})
		}
	}
	// Local nodes group (always present).
	var localNodes []SubNode
	if m.deps.GetLocalNodes != nil {
		localNodes = m.deps.GetLocalNodes()
	}
	m.groups = append(m.groups, groupEntry{name: "local", kind: "local", nodes: localNodes})

	if m.cursor >= len(m.groups) && len(m.groups) > 0 {
		m.cursor = len(m.groups) - 1
	}
}

// MoveDown advances the cursor one row.
func (m *Model) MoveDown() {
	if m.cursor < len(m.groups)-1 {
		m.cursor++
	}
}

// MoveUp moves the cursor up one row.
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update absorbs tea.Msg (currently stateless; data flows in via Refresh).
func (m Model) Update(_ tea.Msg) (Model, tea.Cmd) { return m, nil }

// View renders the tab; defaults to focused for direct callers (tests).
func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused = View + focus dot.
func (m Model) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("🌐 Groups")

	// Split width: left pane ~30%, right pane ~70%.
	leftW := width / 3
	if leftW < 20 {
		leftW = 20
	}
	rightW := width - leftW - 1

	// Left pane: group list.
	leftRows := []string{header, ""}
	curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	for i, g := range m.groups {
		count := fmt.Sprintf(" (%d)", len(g.nodes))
		line := viewport.TruncateDisplay(g.name+count, leftW-4)
		if i == m.cursor {
			leftRows = append(leftRows, curStyle.Render("▶ ")+line)
		} else {
			leftRows = append(leftRows, "  "+line)
		}
	}
	if len(m.groups) == 0 {
		leftRows = append(leftRows, "  (none)")
	}
	leftRows = append(leftRows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] group"))

	leftPane := lipgloss.NewStyle().
		Width(leftW).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).
		Padding(1, 1).
		Render(strings.Join(leftRows, "\n"))

	// Right pane: nodes for selected group.
	rightRows := []string{"", ""}
	if m.cursor >= 0 && m.cursor < len(m.groups) {
		g := m.groups[m.cursor]
		kind := "subscription"
		if g.kind == "local" {
			kind = "local"
		}
		title := lipgloss.NewStyle().Bold(true).Render(g.name) +
			lipgloss.NewStyle().Faint(true).Render("  " + kind)
		rightRows[0] = title
		if len(g.nodes) == 0 {
			helpMsg := "  (no nodes cached — run `vpnkit subs update " + g.name + "`)"
			if g.kind == "local" {
				helpMsg = "  (no local nodes — add via Sources → Local Nodes)"
			}
			rightRows = append(rightRows, helpMsg)
		} else {
			maxRows := height - 6
			if maxRows < 3 {
				maxRows = 3
			}
			for idx, n := range g.nodes {
				if idx >= maxRows {
					rightRows = append(rightRows, fmt.Sprintf("  … and %d more", len(g.nodes)-maxRows))
					break
				}
				portStr := ""
				if n.Port > 0 {
					portStr = fmt.Sprintf(":%d", n.Port)
				}
				line := fmt.Sprintf("%-28s  %-8s  %s%s", n.Name, n.Proto, n.Server, portStr)
				rightRows = append(rightRows, "  "+viewport.TruncateDisplay(line, rightW-4))
			}
		}
	}
	rightPane := lipgloss.NewStyle().
		Width(rightW).Height(height).
		Padding(1, 2).
		Render(strings.Join(rightRows, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}
