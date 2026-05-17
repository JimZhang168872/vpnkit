package settings

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/extensions"
)

// ProxyNamesFunc returns the current set of mihomo proxy names + group names
// (used for autocomplete hints in the Extensions sub-page). Caller supplies
// a closure so we don't depend directly on the api.Client.
type ProxyNamesFunc func() []string

type extPane int

const (
	paneChains extPane = iota
	paneGroups
)

type extForm struct {
	pane      extPane
	editIndex int      // -1 = add; >= 0 = edit existing
	labels    []string // visible labels for each field
	fields    []string // current values
	focus     int
}

func newChainForm(editIndex int, pref extensions.Chain) *extForm {
	return &extForm{
		pane:      paneChains,
		editIndex: editIndex,
		labels:    []string{"Node", "Via"},
		fields:    []string{pref.Node, pref.Via},
	}
}

func newGroupForm(editIndex int, pref extensions.Group) *extForm {
	return &extForm{
		pane:      paneGroups,
		editIndex: editIndex,
		labels: []string{
			"Name", "Type (select|url-test|fallback|load-balance|relay)",
			"Proxies (comma-separated)", "URL (optional)",
			"Interval (optional, int)", "Tolerance (optional, int)",
		},
		fields: []string{
			pref.Name, pref.Type,
			strings.Join(pref.Proxies, ","),
			pref.URL, intStr(pref.Interval), intStr(pref.Tolerance),
		},
	}
}

func intStr(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

type extensionsModel struct {
	path      string
	ext       extensions.Extensions
	pane      extPane
	row       int
	flash     string
	names     ProxyNamesFunc
	form      *extForm
	applyFunc func() error
}

func newExtensions(path string, names ProxyNamesFunc) extensionsModel {
	ext, _ := extensions.Load(path)
	return extensionsModel{path: path, ext: ext, pane: paneChains, names: names}
}

func (m extensionsModel) formOpen() bool { return m.form != nil }

func (m extensionsModel) Update(msg tea.Msg) (extensionsModel, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.form != nil {
		return m.updateForm(km)
	}
	switch km.String() {
	case "c":
		m.pane = paneChains
		m.row = 0
	case "g":
		m.pane = paneGroups
		m.row = 0
	case "up", "k":
		if m.row > 0 {
			m.row--
		}
	case "down", "j":
		size := m.activeLen()
		if m.row < size-1 {
			m.row++
		}
	case "a":
		if m.pane == paneChains {
			m.form = newChainForm(-1, extensions.Chain{})
		} else {
			m.form = newGroupForm(-1, extensions.Group{Type: "select"})
		}
		return m, nil
	case "e":
		if m.pane == paneChains && m.row < len(m.ext.Chains) {
			m.form = newChainForm(m.row, m.ext.Chains[m.row])
		}
		if m.pane == paneGroups && m.row < len(m.ext.Groups) {
			m.form = newGroupForm(m.row, m.ext.Groups[m.row])
		}
		return m, nil
	case "d":
		m.deleteCurrent()
	case "r":
		if m.applyFunc != nil {
			if err := m.applyFunc(); err != nil {
				m.flash = "apply: " + err.Error()
			} else {
				m.flash = "applied + reloaded"
			}
		} else {
			m.flash = "apply unwired (run from full app)"
		}
	}
	return m, nil
}

func (m extensionsModel) updateForm(km tea.KeyMsg) (extensionsModel, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		m.form = nil
		m.flash = "cancelled"
		return m, nil
	case tea.KeyEnter:
		return m.commitForm(), nil
	case tea.KeyTab:
		m.form.focus = (m.form.focus + 1) % len(m.form.fields)
		return m, nil
	case tea.KeyShiftTab:
		m.form.focus = (m.form.focus + len(m.form.fields) - 1) % len(m.form.fields)
		return m, nil
	case tea.KeyBackspace:
		if len(m.form.fields[m.form.focus]) > 0 {
			s := m.form.fields[m.form.focus]
			m.form.fields[m.form.focus] = s[:len(s)-1]
		}
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		m.form.fields[m.form.focus] += string(km.Runes)
		return m, nil
	}
	return m, nil
}

func (m extensionsModel) commitForm() extensionsModel {
	switch m.form.pane {
	case paneChains:
		c := extensions.Chain{Node: m.form.fields[0], Via: m.form.fields[1]}
		newChains := append([]extensions.Chain{}, m.ext.Chains...)
		if m.form.editIndex >= 0 && m.form.editIndex < len(newChains) {
			newChains[m.form.editIndex] = c
		} else {
			newChains = append(newChains, c)
		}
		candidate := m.ext
		candidate.Chains = newChains
		if err := extensions.Validate(candidate); err != nil {
			m.flash = "validate: " + err.Error()
			return m
		}
		m.ext = candidate
	case paneGroups:
		interval, _ := strconv.Atoi(m.form.fields[4])
		tolerance, _ := strconv.Atoi(m.form.fields[5])
		g := extensions.Group{
			Name:      m.form.fields[0],
			Type:      m.form.fields[1],
			Proxies:   splitCSV(m.form.fields[2]),
			URL:       m.form.fields[3],
			Interval:  interval,
			Tolerance: tolerance,
		}
		newGroups := append([]extensions.Group{}, m.ext.Groups...)
		if m.form.editIndex >= 0 && m.form.editIndex < len(newGroups) {
			newGroups[m.form.editIndex] = g
		} else {
			newGroups = append(newGroups, g)
		}
		candidate := m.ext
		candidate.Groups = newGroups
		if err := extensions.Validate(candidate); err != nil {
			m.flash = "validate: " + err.Error()
			return m
		}
		m.ext = candidate
	}
	if err := extensions.Save(m.path, m.ext); err != nil {
		m.flash = "save: " + err.Error()
		return m
	}
	m.flash = "saved"
	m.form = nil
	return m
}

func (m extensionsModel) View(width, height int) string {
	if m.form != nil {
		return m.renderForm(width, height)
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Extensions")
	tabs := m.renderTabs()
	body := m.renderList()
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
		"[c]hains [g]roups   [↑↓] navigate  [a]dd  [e]dit  [d]el  [r] apply",
	)
	out := header + "\n\n" + tabs + "\n\n" + body + "\n\n" + footer
	if m.flash != "" {
		out += "\n  → " + m.flash
	}
	out += fmt.Sprintf("\n\nfile: %s", m.path)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(out)
}

func (m extensionsModel) renderTabs() string {
	style := func(active bool) lipgloss.Style {
		if active {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	}
	return style(m.pane == paneChains).Render(fmt.Sprintf("[c] Chains (%d)", len(m.ext.Chains))) +
		"    " +
		style(m.pane == paneGroups).Render(fmt.Sprintf("[g] Groups (%d)", len(m.ext.Groups)))
}

func (m extensionsModel) renderList() string {
	lines := []string{}
	cursor := func(i int) string {
		if i == m.row {
			return "▶ "
		}
		return "  "
	}
	switch m.pane {
	case paneChains:
		for i, c := range m.ext.Chains {
			lines = append(lines, cursor(i)+fmt.Sprintf("%-30s → %s", c.Node, c.Via))
		}
		if len(lines) == 0 {
			lines = append(lines, "  (no chains)")
		}
	case paneGroups:
		for i, g := range m.ext.Groups {
			lines = append(lines, cursor(i)+fmt.Sprintf("%-20s [%s] %s", g.Name, g.Type, strings.Join(g.Proxies, ",")))
		}
		if len(lines) == 0 {
			lines = append(lines, "  (no groups)")
		}
	}
	return strings.Join(lines, "\n")
}

func (m extensionsModel) renderForm(width, height int) string {
	rows := []string{lipgloss.NewStyle().Bold(true).Render("Add/Edit:") + "  [Enter] save  [Esc] cancel  [Tab] next field"}
	for i, lbl := range m.form.labels {
		marker := "  "
		if i == m.form.focus {
			marker = "▶ "
		}
		rows = append(rows, fmt.Sprintf("%s%-46s %s", marker, lbl, m.form.fields[i]))
	}
	if m.names != nil && len(m.names()) > 0 {
		preview := m.names()
		if len(preview) > 8 {
			preview = preview[:8]
		}
		rows = append(rows, "", "known names: "+strings.Join(preview, ", "))
	}
	if m.flash != "" {
		rows = append(rows, "", "→ "+m.flash)
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func (m extensionsModel) activeLen() int {
	if m.pane == paneChains {
		return len(m.ext.Chains)
	}
	return len(m.ext.Groups)
}

func (m *extensionsModel) deleteCurrent() {
	if m.pane == paneChains && m.row < len(m.ext.Chains) {
		m.ext.Chains = append(m.ext.Chains[:m.row], m.ext.Chains[m.row+1:]...)
	}
	if m.pane == paneGroups && m.row < len(m.ext.Groups) {
		m.ext.Groups = append(m.ext.Groups[:m.row], m.ext.Groups[m.row+1:]...)
	}
	if m.row > 0 && m.row >= m.activeLen() {
		m.row--
	}
	if err := extensions.Save(m.path, m.ext); err != nil {
		m.flash = "save: " + err.Error()
		return
	}
	m.flash = "deleted"
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
