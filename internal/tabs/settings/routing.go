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
	applyFunc func() error

	// cursor: 0 = Mode row, 1+ = GlobalTarget rows.
	cursor   int
	modeOpts []string
	numModes int
}

func newRouting(st *store.Store, applyFunc func() error) routingModel {
	modes := []string{"rule", "global", "direct"}
	return routingModel{
		st:        st,
		applyFunc: applyFunc,
		modeOpts:  modes,
		numModes:  len(modes),
	}
}

func (routingModel) Init() tea.Cmd { return nil }

func (m routingModel) Update(message tea.Msg) (routingModel, tea.Cmd) {
	if m.st == nil {
		return m, nil
	}
	if km, ok := message.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			maxRow := m.numModes - 1 // only Mode rows for now
			if m.cursor < maxRow {
				m.cursor++
			}
		case "enter", " ":
			// Selecting a mode row sets Mode.
			if m.cursor < m.numModes {
				m.st.Cfg.Mode = m.modeOpts[m.cursor]
				_ = m.st.Save()
				if m.applyFunc != nil {
					_ = m.applyFunc()
				}
			}
		}
	}
	return m, nil
}

func (m routingModel) View(width, height int) string {
	header := viewport.FocusDot(false) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Routing")

	rows := []string{header, ""}

	if m.st == nil {
		rows = append(rows, "  (store not available)")
		return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).
			Render(strings.Join(rows, "\n"))
	}

	// Mode selection.
	rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Mode"), "")
	curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	activeMode := strings.ToLower(m.st.Cfg.Mode)
	modeLabels := map[string]string{
		"rule":   "Rule    — traffic is matched against rule set",
		"global": "Global  — all traffic routes to Global Target",
		"direct": "Direct  — all traffic bypasses the proxy",
	}
	for i, opt := range m.modeOpts {
		label := modeLabels[opt]
		if label == "" {
			label = opt
		}
		selected := "○"
		if opt == activeMode {
			selected = "●"
		}
		line := selected + " " + label
		if i == m.cursor {
			rows = append(rows, curStyle.Render("▶ "+line))
		} else {
			rows = append(rows, "  "+line)
		}
	}

	// Global Target (display only for now).
	rows = append(rows, "",
		lipgloss.NewStyle().Bold(true).Render("Global Target"),
		"",
		"  Current: "+m.st.Cfg.GlobalTarget,
		lipgloss.NewStyle().Faint(true).Render("  (edit via `vpnkit target set <name>` CLI)"),
	)

	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] navigate  [Enter/Space] select"))
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}
