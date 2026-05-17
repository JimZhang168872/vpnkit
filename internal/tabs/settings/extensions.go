package settings

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/extensions"
	"vpnkit/internal/tabs/viewport"
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

// extForm holds an in-progress add/edit. Inputs use bubbles/textinput so
// cursor positioning, Home/End, paste, and Delete behave correctly — the
// hand-rolled []string + focus that lived here before couldn't.
type extForm struct {
	pane      extPane
	editIndex int                 // -1 = add; >= 0 = edit existing
	labels    []string            // visible labels for each field
	inputs    []textinput.Model   // one textinput per labeled field
	focus     int
}

func newChainForm(editIndex int, pref extensions.Chain) *extForm {
	f := &extForm{
		pane:      paneChains,
		editIndex: editIndex,
		labels:    []string{"Node", "Via"},
		inputs:    []textinput.Model{newInput("node", pref.Node), newInput("upstream", pref.Via)},
	}
	f.inputs[0].Focus()
	return f
}

func newGroupForm(editIndex int, pref extensions.Group) *extForm {
	f := &extForm{
		pane:      paneGroups,
		editIndex: editIndex,
		labels: []string{
			"Name", "Type (select|url-test|fallback|load-balance|relay)",
			"Proxies (comma-separated)", "URL (optional)",
			"Interval (optional, int)", "Tolerance (optional, int)",
		},
		inputs: []textinput.Model{
			newInput("group name", pref.Name),
			newInput("select", pref.Type),
			newInput("DIRECT,A,B,...", strings.Join(pref.Proxies, ",")),
			newInput("https://www.gstatic.com/generate_204", pref.URL),
			newInput("300", intStr(pref.Interval)),
			newInput("50", intStr(pref.Tolerance)),
		},
	}
	f.inputs[0].Focus()
	return f
}

func newInput(placeholder, value string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 40
	if value != "" {
		ti.SetValue(value)
	}
	return ti
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
	height    int // last known body height, set on each View call
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
		m.form.advanceFocus(+1)
		return m, nil
	case tea.KeyShiftTab:
		m.form.advanceFocus(-1)
		return m, nil
	}
	// Delegate everything else (printable runes, backspace, delete, arrow keys,
	// home/end, ctrl-a/e, paste) to the active textinput.Model.
	var cmd tea.Cmd
	m.form.inputs[m.form.focus], cmd = m.form.inputs[m.form.focus].Update(km)
	return m, cmd
}

// advanceFocus moves focus by step (+1 or -1) and blurs/focuses inputs.
func (f *extForm) advanceFocus(step int) {
	n := len(f.inputs)
	if n == 0 {
		return
	}
	f.inputs[f.focus].Blur()
	f.focus = ((f.focus + step) % n + n) % n
	f.inputs[f.focus].Focus()
}

func (m extensionsModel) commitForm() extensionsModel {
	values := make([]string, len(m.form.inputs))
	for i, in := range m.form.inputs {
		values[i] = strings.TrimSpace(in.Value())
	}
	switch m.form.pane {
	case paneChains:
		c := extensions.Chain{Node: values[0], Via: values[1]}
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
		interval, _ := strconv.Atoi(values[4])
		tolerance, _ := strconv.Atoi(values[5])
		g := extensions.Group{
			Name:      values[0],
			Type:      values[1],
			Proxies:   splitCSV(values[2]),
			URL:       values[3],
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

// View renders the Extensions sub-page assuming this panel has the input
// focus (tests + standalone callers go here). Settings.View prefers
// ViewFocused so it can dim the cursor when the sub-sidebar is focused.
func (m extensionsModel) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused is View + an explicit focused-state flag controlling the
// cursor color (212 bright when focused, 240 dim when sidebar is focused).
func (m extensionsModel) ViewFocused(width, height int, focused bool) string {
	m.height = height
	if m.form != nil {
		return m.renderForm(width, height)
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Extensions")
	tabs := m.renderTabs()
	// Reserve rows: header(1) + blank + tabs(1) + blank + blank + footer(1) +
	// flash(1 if present) + blank + file:(1) + padding(2) ≈ 10. The
	// viewport gets whatever's left.
	maxList := height - 11
	if maxList < 3 {
		maxList = 3
	}
	innerWidth := width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}
	body, indicator := m.renderList(maxList, innerWidth, focused)
	hint := "[← →] panels  [↑↓] navigate  [a]dd  [e]dit  [d]el  [r] apply  [c]hains [g]roups"
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
		viewport.TruncateDisplay(hint, innerWidth),
	)
	// Header gets a focus dot (● bright when this panel owns input, ○ dim
	// otherwise) so the user can see at a glance which panel ↑/↓ will hit.
	dot := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○ ")
	if focused {
		dot = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("● ")
	}
	out := dot + header + "\n\n" + tabs
	if indicator != "" {
		out += "   " + lipgloss.NewStyle().Faint(true).Render(indicator)
	}
	out += "\n\n" + body + "\n\n" + footer
	if m.flash != "" {
		out += "\n  → " + viewport.TruncateDisplay(m.flash, innerWidth-4)
	}
	out += fmt.Sprintf("\n\nfile: %s", viewport.TruncateDisplay(m.path, innerWidth-6))
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).
		Padding(1, 2).Render(out)
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

func (m extensionsModel) renderList(maxRows, innerWidth int, focused bool) (body, indicator string) {
	total := m.activeLen()
	if total == 0 {
		switch m.pane {
		case paneChains:
			return "  (no chains)", ""
		case paneGroups:
			return "  (no groups)", ""
		}
	}
	start, end := viewport.Window(total, m.row, maxRows)
	// Cursor color: bright pink when this panel is focused, gray when not —
	// the user can see at a glance whether ↑/↓ will move this cursor.
	cursorColor := lipgloss.Color("240")
	if focused {
		cursorColor = lipgloss.Color("212")
	}
	cursorStyle := lipgloss.NewStyle().Foreground(cursorColor)
	mark := func(i int) string {
		if i == m.row {
			return cursorStyle.Render("▶ ")
		}
		return "  "
	}
	lines := []string{}
	switch m.pane {
	case paneChains:
		for i := start; i < end; i++ {
			c := m.ext.Chains[i]
			line := fmt.Sprintf("%-30s → %s", c.Node, c.Via)
			lines = append(lines, mark(i)+viewport.TruncateDisplay(line, innerWidth-2))
		}
	case paneGroups:
		for i := start; i < end; i++ {
			g := m.ext.Groups[i]
			line := fmt.Sprintf("%-20s [%s] %s", g.Name, g.Type, strings.Join(g.Proxies, ","))
			lines = append(lines, mark(i)+viewport.TruncateDisplay(line, innerWidth-2))
		}
	}
	return strings.Join(lines, "\n"), viewport.Indicator(start, total, maxRows, m.row)
}

func (m extensionsModel) renderForm(width, height int) string {
	rows := []string{lipgloss.NewStyle().Bold(true).Render("Add/Edit:") + "  [Enter] save  [Esc] cancel  [Tab] next field"}
	for i, lbl := range m.form.labels {
		marker := "  "
		if i == m.form.focus {
			marker = "▶ "
		}
		rows = append(rows, fmt.Sprintf("%s%-46s", marker, lbl))
		rows = append(rows, "    "+m.form.inputs[i].View())
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
