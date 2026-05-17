// Package groups implements the Groups tab: list all proxy groups
// (subscription groups + the local-nodes group) on the left, nodes of the
// selected group on the right. Two-pane focus model:
//   - SubFocusLeft  : ↑/↓ moves between groups
//   - SubFocusRight : ↑/↓ moves between nodes of the selected group
//   - Enter on right-focus calls back into the app to PutProxy(group, node).
package groups

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/viewport"
)

// SubFocus is which pane owns ↑/↓ inside the Groups tab.
type SubFocus int

const (
	SubFocusLeft SubFocus = iota
	SubFocusRight
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
	GetSubs        func() []store.Subscription
	GetSubNodes    func(name string) []SubNode
	GetLocalGroups func() []store.LocalNodeGroup
	GetLocalNodes  func(group string) []SubNode
}

// Model is the Groups tab.
type Model struct {
	deps        Deps
	groups      []groupEntry
	cursor      int
	rightCursor int
	subFocus    SubFocus
	// nowByGroup mirrors mihomo's /proxies → <group>.now. Updated on every
	// ProxiesSnapshot we receive via Update().
	nowByGroup map[string]string
}

type groupEntry struct {
	name  string
	kind  string // "subscription" | "local"
	nodes []SubNode
}

// New returns an empty Groups tab model.
func New(deps Deps) Model {
	return Model{deps: deps, nowByGroup: map[string]string{}}
}

// Refresh rebuilds the group list from deps. Preserves rightCursor when
// possible, clamps when the selected group's node list shrinks.
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
	if m.deps.GetLocalGroups != nil && m.deps.GetLocalNodes != nil {
		for _, lg := range m.deps.GetLocalGroups() {
			nodes := m.deps.GetLocalNodes(lg.Name)
			m.groups = append(m.groups, groupEntry{name: lg.Name, kind: "local", nodes: nodes})
		}
	}

	if m.cursor >= len(m.groups) && len(m.groups) > 0 {
		m.cursor = len(m.groups) - 1
	}
	m.clampRightCursor()
}

func (m *Model) clampRightCursor() {
	if m.cursor < 0 || m.cursor >= len(m.groups) {
		m.rightCursor = 0
		return
	}
	n := len(m.groups[m.cursor].nodes)
	if m.rightCursor >= n {
		if n > 0 {
			m.rightCursor = n - 1
		} else {
			m.rightCursor = 0
		}
	}
	if m.rightCursor < 0 {
		m.rightCursor = 0
	}
}

// MoveDown advances the cursor in the focused pane.
func (m *Model) MoveDown() {
	switch m.subFocus {
	case SubFocusLeft:
		if m.cursor < len(m.groups)-1 {
			m.cursor++
			m.rightCursor = 0
		}
	case SubFocusRight:
		if m.cursor >= 0 && m.cursor < len(m.groups) {
			if m.rightCursor < len(m.groups[m.cursor].nodes)-1 {
				m.rightCursor++
			}
		}
	}
}

// MoveUp moves the cursor up in the focused pane.
func (m *Model) MoveUp() {
	switch m.subFocus {
	case SubFocusLeft:
		if m.cursor > 0 {
			m.cursor--
			m.rightCursor = 0
		}
	case SubFocusRight:
		if m.rightCursor > 0 {
			m.rightCursor--
		}
	}
}

// SubFocus / SetSubFocus expose the inner focus state to the app.
func (m Model) SubFocus() SubFocus       { return m.subFocus }
func (m *Model) SetSubFocus(f SubFocus)  { m.subFocus = f; m.clampRightCursor() }

// SelectedGroup returns the group name currently under the left-pane cursor,
// or "" if no group is selected.
func (m Model) SelectedGroup() string {
	if m.cursor < 0 || m.cursor >= len(m.groups) {
		return ""
	}
	return m.groups[m.cursor].name
}

// SelectedNode returns the node name currently under the right-pane cursor
// (namespaced, e.g. "doge:JP-B"), or "" if the group has no nodes. The
// returned name matches what mihomo expects in PutProxy(group, member).
func (m Model) SelectedNode() string {
	if m.cursor < 0 || m.cursor >= len(m.groups) {
		return ""
	}
	g := m.groups[m.cursor]
	if m.rightCursor < 0 || m.rightCursor >= len(g.nodes) {
		return ""
	}
	// assembler.namespaced renames every proxy "<group>:<original-name>";
	// that's the name mihomo sees, so build the namespaced form here too.
	return fmt.Sprintf("%s:%s", g.name, g.nodes[m.rightCursor].Name)
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update reacts to ProxiesSnapshot messages by mirroring each group's `now`
// into nowByGroup so the View can highlight it. All other messages are
// passed through unchanged — keystrokes are handled at the app level so
// the focus model can route them correctly.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if snap, ok := message.(msg.ProxiesSnapshot); ok {
		for name, g := range snap.Groups {
			m.nowByGroup[name] = g.Now
		}
	}
	return m, nil
}

// View renders the tab; defaults to focused for direct callers (tests).
func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused = View + focus dot.
func (m Model) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("🌐 Groups")

	leftW := width / 3
	if leftW < 24 {
		leftW = 24
	}
	rightW := width - leftW - 1

	leftFocused := focused && m.subFocus == SubFocusLeft
	rightFocused := focused && m.subFocus == SubFocusRight

	// Left pane: group list with current "now" annotation.
	leftRows := []string{header, ""}
	curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	for i, g := range m.groups {
		count := fmt.Sprintf(" (%d)", len(g.nodes))
		nowLabel := ""
		if now := m.nowByGroup[g.name]; now != "" {
			// Strip "<group>:" prefix for compact display.
			short := strings.TrimPrefix(now, g.name+":")
			nowLabel = lipgloss.NewStyle().Faint(true).Render(" → " + short)
		}
		line := viewport.TruncateDisplay(g.name+count, leftW-6)
		if i == m.cursor {
			if leftFocused {
				leftRows = append(leftRows, curStyle.Render("▶ ")+line+nowLabel)
			} else {
				leftRows = append(leftRows, lipgloss.NewStyle().Faint(true).Render("▶ ")+line+nowLabel)
			}
		} else {
			leftRows = append(leftRows, "  "+line+nowLabel)
		}
	}
	if len(m.groups) == 0 {
		leftRows = append(leftRows, "  (none)")
	}
	leftRows = append(leftRows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] group  [→] nodes"))

	leftPane := lipgloss.NewStyle().
		Width(leftW).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).
		Padding(1, 1).
		Render(strings.Join(leftRows, "\n"))

	// Right pane: nodes for the selected group with `●` on current `now`.
	rightHeader := viewport.FocusDot(rightFocused) +
		lipgloss.NewStyle().Bold(true).Render("Nodes")
	rightRows := []string{rightHeader, ""}
	if m.cursor >= 0 && m.cursor < len(m.groups) {
		g := m.groups[m.cursor]
		now := m.nowByGroup[g.name]
		kind := "subscription"
		if g.kind == "local" {
			kind = "local"
		}
		subtitle := lipgloss.NewStyle().Faint(true).Render(g.name + "  " + kind)
		rightRows = append(rightRows, subtitle, "")
		if len(g.nodes) == 0 {
			helpMsg := "  (no nodes cached — run `vpnkit subs update " + g.name + "`)"
			if g.kind == "local" {
				helpMsg = "  (no local nodes — add via Sources → Local Nodes)"
			}
			rightRows = append(rightRows, helpMsg)
		} else {
			maxRows := height - 8
			if maxRows < 3 {
				maxRows = 3
			}
			for idx, n := range g.nodes {
				if idx >= maxRows {
					rightRows = append(rightRows, fmt.Sprintf("  … and %d more", len(g.nodes)-maxRows))
					break
				}
				namespaced := fmt.Sprintf("%s:%s", g.name, n.Name)
				marker := "  "
				if namespaced == now {
					marker = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("● ")
				}
				portStr := ""
				if n.Port > 0 {
					portStr = fmt.Sprintf(":%d", n.Port)
				}
				line := fmt.Sprintf("%-28s  %-8s  %s%s", n.Name, n.Proto, n.Server, portStr)
				lineRendered := viewport.TruncateDisplay(line, rightW-6)
				if idx == m.rightCursor && rightFocused {
					rightRows = append(rightRows, curStyle.Render("▶ ")+marker+lineRendered)
				} else if idx == m.rightCursor {
					rightRows = append(rightRows, lipgloss.NewStyle().Faint(true).Render("▶ ")+marker+lineRendered)
				} else {
					rightRows = append(rightRows, "  "+marker+lineRendered)
				}
			}
			rightRows = append(rightRows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] node  [Enter] use this node  [←] back"))
		}
	}
	rightPane := lipgloss.NewStyle().
		Width(rightW).Height(height).
		Padding(1, 2).
		Render(strings.Join(rightRows, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}
