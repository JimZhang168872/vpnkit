package settings

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/rules"
	"vpnkit/internal/store"
)

type rulesModel struct {
	store *store.Store
	list  []string
	idx   int
}

func newRules(s *store.Store) rulesModel {
	list := rules.List()
	idx := 0
	if s != nil {
		for i, name := range list {
			if name == s.Cfg.RuleTemplate {
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
	case "j":
		if m.idx < len(m.list)-1 {
			m.idx++
		}
	case "k":
		if m.idx > 0 {
			m.idx--
		}
	case "enter":
		if m.store != nil && m.idx < len(m.list) {
			m.store.Cfg.RuleTemplate = m.list[m.idx]
			_ = m.store.Save()
		}
	}
	return m, nil
}

func (m rulesModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Default Rules")
	rows := []string{header, "", "  Pick a template (Enter to save):", ""}
	current := ""
	if m.store != nil {
		current = m.store.Cfg.RuleTemplate
	}
	for i, name := range m.list {
		marker := "( )"
		if name == current {
			marker = "(•)"
		}
		row := "  " + marker + " " + name
		if i == m.idx {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ " + marker + " " + name)
		}
		rows = append(rows, row)
	}
	rows = append(rows, "", "[j k] navigate  [Enter] save")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(joinNL(rows))
}

func joinNL(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += "\n"
		}
		out += s
	}
	return out
}
