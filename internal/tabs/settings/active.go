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

	// Snapshot fields — refreshed at every Update entry so View never
	// reads store.Cfg directly (which races with Pipeline mutations
	// happening on tea.Cmd goroutines). All View renders read from
	// these fields only.
	snapItems  []sourceItem
	snapActive string
	busy       bool // applyFunc in flight; key handler ignored
}

func newActive(st *store.Store, pl PipelineFace, applyFunc func() error) activeModel {
	m := activeModel{st: st, pl: pl, applyFunc: applyFunc}
	// Initial snapshot so View renders correctly before the first Update.
	m.refreshSnapshot()
	return m
}

func (activeModel) Init() tea.Cmd { return nil }

// activeApplyDoneMsg signals the end of an async applyFunc fired after a
// successful SetActiveSource. The Update handler clears busy and turns
// the result into a flash. Without this round-trip the applyFunc (which
// can take up to 30s due to mihomo reload + svc.Restart fallback) would
// run synchronously on the bubbletea event loop and freeze the TUI.
type activeApplyDoneMsg struct {
	pick string
	err  error
}

func (m activeModel) Update(message tea.Msg) (activeModel, tea.Cmd) {
	if done, ok := message.(activeApplyDoneMsg); ok {
		m.busy = false
		if done.err != nil {
			m.flash = "⚠️  saved → " + done.pick + ", but mihomo reload failed: " + done.err.Error()
		} else {
			m.flash = "✅ active → " + done.pick
		}
		m.refreshSnapshot()
		return m, nil
	}
	// Pull a fresh snapshot at the top of every Update so cursor math +
	// View rendering see consistent data.
	m.refreshSnapshot()
	km, ok := message.(tea.KeyMsg)
	if !ok || m.busy {
		return m, nil
	}
	switch km.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.snapItems)-1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor >= 0 && m.cursor < len(m.snapItems) && m.pl != nil {
			pick := m.snapItems[m.cursor].name
			if err := m.pl.SetActiveSource(pick); err != nil {
				m.flash = "❌ " + err.Error()
				return m, nil
			}
			// Save succeeded. Fire applyFunc on a goroutine so the
			// bubbletea event loop stays responsive — mihomo reload can
			// take up to 30s and freezes everything if we await it here.
			if m.applyFunc != nil {
				m.busy = true
				applyFn := m.applyFunc
				return m, func() tea.Msg {
					return activeApplyDoneMsg{pick: pick, err: applyFn()}
				}
			}
			m.flash = "✅ active → " + pick
			m.refreshSnapshot()
		}
	}
	return m, nil
}

type sourceItem struct {
	name string
	kind string // "subscription" / "local"
}

// refreshSnapshot pulls the current source list + active marker from
// Pipeline (which holds p.mu while copying). Stores in model fields so
// View can read without locking. Called at every Update entry.
//
// If pl is nil (test harness mode), we fall back to a direct store.Cfg
// read — those tests don't run Pipeline-driven mutations concurrently
// so the read is safe.
func (m *activeModel) refreshSnapshot() {
	m.snapItems = m.snapItems[:0]
	if m.pl != nil {
		for _, s := range m.pl.SubscriptionNames() {
			if s.Enabled {
				m.snapItems = append(m.snapItems, sourceItem{name: s.Name, kind: "subscription"})
			}
		}
		for _, g := range m.pl.LocalNodeGroups() {
			if g.Enabled {
				m.snapItems = append(m.snapItems, sourceItem{name: g.Name, kind: "local"})
			}
		}
		m.snapActive = m.pl.ActiveSource()
	} else if m.st != nil {
		for _, s := range m.st.Cfg.Subscriptions {
			if s.Enabled {
				m.snapItems = append(m.snapItems, sourceItem{name: s.Name, kind: "subscription"})
			}
		}
		for _, g := range m.st.Cfg.LocalNodeGroups {
			if g.Enabled {
				m.snapItems = append(m.snapItems, sourceItem{name: g.Name, kind: "local"})
			}
		}
		m.snapActive = m.st.Cfg.ActiveSource
	}
	if m.cursor >= len(m.snapItems) && len(m.snapItems) > 0 {
		m.cursor = len(m.snapItems) - 1
	}
}

func (m activeModel) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

func (m activeModel) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Active Source")

	rows := []string{header, ""}
	if m.st == nil && m.pl == nil {
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

	if len(m.snapItems) == 0 {
		rows = append(rows, "  (no enabled sources — add one in Sources tab)")
	} else {
		curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
		selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
		for i, it := range m.snapItems {
			marker := "[ ]"
			if it.name == m.snapActive {
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

	if m.busy {
		rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("⏳ reloading mihomo…"))
	}
	if m.flash != "" {
		rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(m.flash))
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(
		"[↑↓] navigate  [Enter/Space] select"))
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}
