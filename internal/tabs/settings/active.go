// Settings → Active Source sub-page (rc.7+).
//
// Renders a single Selector-style list of every enabled source
// (subscriptions first, then local-node groups). User picks one with
// Enter; the underlying Pipeline.SetActiveSource validates + persists,
// then ApplyFunc triggers a config.yaml reassemble + mihomo reload so
// the routing flip is immediate.
package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/viewport"
)

type activeModel struct {
	st        *store.Store
	pl        PipelineFace
	applyFunc func() error

	cursor int
	flash  string // last error / success message for visual feedback
}

func newActive(st *store.Store, pl PipelineFace, applyFunc func() error) activeModel {
	return activeModel{st: st, pl: pl, applyFunc: applyFunc}
}

func (activeModel) Init() tea.Cmd { return nil }

func (m activeModel) Update(message tea.Msg) (activeModel, tea.Cmd) {
	if m.st == nil {
		return m, nil
	}
	km, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	items := m.sourceList()
	switch km.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(items)-1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor >= 0 && m.cursor < len(items) && m.pl != nil {
			pick := items[m.cursor].name
			if err := m.pl.SetActiveSource(pick); err != nil {
				m.flash = "❌ " + err.Error()
				return m, nil
			}
			m.flash = "✅ active → " + pick
			if m.applyFunc != nil {
				_ = m.applyFunc()
			}
		}
	}
	return m, nil
}

type sourceItem struct {
	name string
	kind string // "subscription" / "local"
}

// sourceList builds the picker rows in display order: enabled
// subscriptions first, then enabled local-node groups. Disabled sources
// are intentionally hidden — making them active would do nothing useful
// (their nodes aren't in mihomo's config), so showing them only invites
// mistakes.
func (m activeModel) sourceList() []sourceItem {
	out := []sourceItem{}
	if m.st == nil {
		return out
	}
	for _, s := range m.st.Cfg.Subscriptions {
		if s.Enabled {
			out = append(out, sourceItem{name: s.Name, kind: "subscription"})
		}
	}
	for _, g := range m.st.Cfg.LocalNodeGroups {
		if g.Enabled {
			out = append(out, sourceItem{name: g.Name, kind: "local"})
		}
	}
	return out
}

func (m activeModel) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

func (m activeModel) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Active Source")

	rows := []string{header, ""}
	if m.st == nil {
		rows = append(rows, "  (store not available)")
		return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).
			Render(strings.Join(rows, "\n"))
	}

	rows = append(rows,
		lipgloss.NewStyle().Faint(true).Render(
			"Pick the source whose nodes back 🚀 Proxy and whose rules drive routing."),
		lipgloss.NewStyle().Faint(true).Render(
			"  • subscription with rules → use its rules"),
		lipgloss.NewStyle().Faint(true).Render(
			"  • subscription without rules / local group → loyalsoldier template"),
		"",
	)

	items := m.sourceList()
	active := m.st.Cfg.ActiveSource
	if len(items) == 0 {
		rows = append(rows, "  (no enabled sources — add one in Sources tab)")
	} else {
		curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
		selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
		for i, it := range items {
			marker := "[ ]"
			if it.name == active {
				marker = selectedStyle.Render("[x]")
			}
			kindLabel := lipgloss.NewStyle().Faint(true).Render("  (" + it.kind + ")")
			line := marker + " " + it.name + kindLabel
			if i == m.cursor {
				rows = append(rows, curStyle.Render("▶ ")+line)
			} else {
				rows = append(rows, "  "+line)
			}
		}
	}

	if m.flash != "" {
		rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(m.flash))
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(
		"[↑↓] navigate  [Enter/Space] select"))
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}
