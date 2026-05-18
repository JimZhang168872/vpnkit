package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/rules"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/viewport"
)

// rulesModel is the Settings → Rule Template sub-page. Despite the file
// name (historical), this picks the built-in rules template that fills in
// when a subscription does not provide its own `rules:` block. For
// per-entry routing (DOMAIN-SUFFIX,…) see the Rules tab → Local Rules.
type rulesModel struct {
	store *store.Store
	list  []string
	idx   int
	flash string
}

func newRules(s *store.Store) rulesModel {
	list := rules.List()
	idx := 0
	if s != nil {
		for i, name := range list {
			if name == s.Cfg.LegacyRuleTemplate {
				idx = i
				break
			}
		}
	}
	return rulesModel{store: s, list: list, idx: idx}
}

func (m rulesModel) Update(message tea.Msg) (rulesModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "down", "j":
		if m.idx < len(m.list)-1 {
			m.idx++
		}
	case "up", "k":
		if m.idx > 0 {
			m.idx--
		}
	case "enter", " ":
		if m.store != nil && m.idx < len(m.list) {
			m.store.Cfg.LegacyRuleTemplate = m.list[m.idx]
			if err := m.store.Save(); err != nil {
				m.flash = "❌ save template: " + err.Error()
			} else {
				m.flash = "✅ template → " + m.list[m.idx]
			}
		}
	}
	return m, nil
}

func (m rulesModel) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused renders with parent FocusContent state so the top ●/○ dot
// reflects whether content owns input.
func (m rulesModel) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Rule Template")

	desc := lipgloss.NewStyle().Faint(true).Render(
		"Fallback rules used only when a subscription does not ship its own.\n" +
			"For per-entry routing (DOMAIN-SUFFIX,…) see Rules tab → Local Rules.")

	rows := []string{header, "", desc, ""}
	current := ""
	if m.store != nil {
		current = m.store.Cfg.LegacyRuleTemplate
	}
	curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	for i, name := range m.list {
		marker := "[ ]"
		if name == current {
			marker = selStyle.Render("[x]")
		}
		line := marker + " " + name
		if i == m.idx {
			rows = append(rows, curStyle.Render("▶ ")+line)
		} else {
			rows = append(rows, "  "+line)
		}
	}
	if m.flash != "" {
		rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(m.flash))
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] navigate  [Enter/Space] select"))
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}
