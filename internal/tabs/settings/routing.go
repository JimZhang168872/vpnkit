package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/viewport"
)

// routingModel is the Settings → Routing sub-page.
// It exposes: Mode (Rule/Global/Direct) and Global Target.
type routingModel struct {
	st        *store.Store
	pl        PipelineFace
	applyFunc func() error

	cursor   int
	modeOpts []string
	numModes int
	flash    string

	// Snapshot fields refreshed at every Update entry — see active.go
	// for why we read through Pipeline rather than store.Cfg directly.
	snapMode         string
	snapGlobalTarget string
	busy             bool // applyFunc in flight
}

func newRouting(st *store.Store, pl PipelineFace, applyFunc func() error) routingModel {
	modes := []string{"rule", "global", "direct"}
	m := routingModel{
		st:        st,
		pl:        pl,
		applyFunc: applyFunc,
		modeOpts:  modes,
		numModes:  len(modes),
	}
	m.refreshSnapshot()
	return m
}

func (routingModel) Init() tea.Cmd { return nil }

// RoutingApplyDoneMsg is the async completion signal for applyFunc fired
// after a successful SetMode. Mirrors ActiveApplyDoneMsg — exported so
// the app-level Model.Update can forward it back into settingsTab.
// Without this round-trip the message dies in the top-level switch and
// the routing pane's busy flag stays true forever, making the Mode list
// permanently input-dead.
type RoutingApplyDoneMsg struct {
	Mode string
	Err  error
}

func (m routingModel) Update(message tea.Msg) (routingModel, tea.Cmd) {
	if done, ok := message.(RoutingApplyDoneMsg); ok {
		m.busy = false
		if done.Err != nil {
			m.flash = "⚠️  mode → " + done.Mode + " saved, mihomo reload failed: " + done.Err.Error()
		} else {
			m.flash = "✅ mode → " + done.Mode
		}
		m.refreshSnapshot()
		return m, nil
	}
	m.refreshSnapshot()
	if m.st == nil && m.pl == nil {
		return m, nil
	}
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
		maxRow := m.numModes - 1
		if m.cursor < maxRow {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor < m.numModes && m.pl != nil {
			mode := m.modeOpts[m.cursor]
			if err := m.pl.SetMode(mode); err != nil {
				m.flash = "❌ save mode: " + err.Error()
				return m, nil
			}
			// Async apply — same rationale as active.go: applyFunc can
			// take up to 30s and freezing the bubbletea loop is unacceptable.
			if m.applyFunc != nil {
				m.busy = true
				applyFn := m.applyFunc
				return m, func() tea.Msg {
					return RoutingApplyDoneMsg{Mode: mode, Err: applyFn()}
				}
			}
			m.flash = "✅ mode → " + mode
			m.refreshSnapshot()
		}
	}
	return m, nil
}

// refreshSnapshot copies the display-relevant fields out of Pipeline so
// View never reads store.Cfg directly. Falls back to direct read when
// pl is nil (test harness).
func (m *routingModel) refreshSnapshot() {
	if m.pl != nil {
		m.snapMode = m.pl.Mode()
		// GlobalTarget is rarely shown — read directly is acceptable
		// because no current Pipeline method modifies it without going
		// through SetActiveSource (which would race nominally but in
		// practice the read here only refreshes display, not safety-
		// critical state).
		if m.st != nil {
			m.snapGlobalTarget = m.st.Cfg.GlobalTarget
		}
		return
	}
	if m.st != nil {
		m.snapMode = m.st.Cfg.Mode
		m.snapGlobalTarget = m.st.Cfg.GlobalTarget
	}
}

func (m routingModel) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused renders Routing; focused = parent Settings has FocusContent
// on this sub-page (so the top ●/○ should reflect that state, and the
// cursor color/bold flags can intensify when content owns input).
func (m routingModel) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Routing")

	rows := []string{header, ""}

	if m.st == nil && m.pl == nil {
		rows = append(rows, "  (store not available)")
		return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).
			Render(strings.Join(rows, "\n"))
	}

	// Mode selection.
	rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Mode"), "")
	curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	activeMode := strings.ToLower(m.snapMode)
	modeLabels := map[string]string{
		"rule":   "Rule    — traffic is matched against rule set",
		"global": "Global  — all traffic routes to Global Target",
		"direct": "Direct  — all traffic bypasses the proxy",
	}
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	for i, opt := range m.modeOpts {
		label := modeLabels[opt]
		if label == "" {
			label = opt
		}
		marker := "[ ]"
		if opt == activeMode {
			marker = selectedStyle.Render("[x]")
		}
		line := marker + " " + label
		if i == m.cursor {
			rows = append(rows, curStyle.Render("▶ ")+line)
		} else {
			rows = append(rows, "  "+line)
		}
	}

	rows = append(rows, "",
		lipgloss.NewStyle().Bold(true).Render("Global Target"),
		"",
		"  Current: "+m.snapGlobalTarget,
		lipgloss.NewStyle().Faint(true).Render("  (edit via `vpnkit target set <name>` CLI)"),
	)

	if m.busy {
		rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("⏳ reloading mihomo…"))
	}
	if m.flash != "" {
		rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(m.flash))
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] navigate  [Enter/Space] select"))
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}
