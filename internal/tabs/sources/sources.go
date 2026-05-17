// Package sources implements the Sources tab with two sub-pages:
// Subscriptions (remote feed CRUD) and Local Nodes (hand-entered proxies).
package sources

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/store"
	"vpnkit/internal/subscription/proto"
	"vpnkit/internal/tabs/viewport"
)

// SubPage identifies a sub-page within Sources.
type SubPage int

const (
	SubSubscriptions SubPage = iota
	SubLocalNodes
	numSubPages
)

var subPageNames = [numSubPages]string{"Subscriptions", "Local Nodes"}

// PipelineMutatedMsg is emitted after any Add/Delete/Toggle/Refresh mutation
// inside Sources so other tabs (Groups in particular) can refresh their
// derived views. App-level Update handles this by calling
// m.groupsTab.Refresh() — Sources cannot reach across because it doesn't
// know about other tabs.
type PipelineMutatedMsg struct{}

func emitPipelineMutated() tea.Cmd {
	return func() tea.Msg { return PipelineMutatedMsg{} }
}

// PipelineFace is the minimal interface needed for mutations.
type PipelineFace interface {
	SubscriptionNames() []store.Subscription
	AddSubscription(sub store.Subscription) error
	DeleteSubscription(name string) error
	ToggleSubscriptionEnabled(name string) error
	RefreshSubscription(ctx context.Context, name string) (int, error)
	LocalNodes() *localnodes.Manager
	SaveLocal() error
	// Local groups (new in rc.3).
	LocalNodeGroups() []store.LocalNodeGroup
	AddLocalGroup(name string) error
	DeleteLocalGroup(name string, force bool) error
	ToggleLocalGroupEnabled(name string) error
	RenameLocalGroup(oldName, newName string) error
}

// Deps wires external dependencies.
type Deps struct {
	Pipeline  PipelineFace
	ApplyFunc func() error // reassemble + reload mihomo
}

// SubFocus tracks which panel inside Sources has focus.
type SubFocus int

const (
	FocusSidebar SubFocus = iota
	FocusContent
)

// Model is the Sources tab model.
type Model struct {
	deps    Deps
	current SubPage
	focus   SubFocus

	subs   subsModel
	locals localNodesModel
}

// InputOpen reports whether a text input overlay is open.
func (m Model) InputOpen() bool {
	switch m.current {
	case SubSubscriptions:
		return m.subs.formOpen()
	case SubLocalNodes:
		return m.locals.formOpen()
	}
	return false
}

// Focus returns the current sub-focus.
func (m Model) Focus() SubFocus { return m.focus }

// SetFocus updates the sub-focus.
func (m *Model) SetFocus(f SubFocus) { m.focus = f }

// New returns an initialized Sources tab model with data pulled from Pipeline.
// Pipeline may be nil (tests) — Refresh handles that.
func New(deps Deps) Model {
	m := Model{
		deps:   deps,
		subs:   newSubsModel(deps),
		locals: newLocalNodesModel(deps),
	}
	m.Refresh()
	return m
}

// Refresh reloads data from pipeline.
func (m *Model) Refresh() {
	if m.deps.Pipeline != nil {
		m.subs.setData(m.deps.Pipeline.SubscriptionNames())
		m.locals.setData(m.deps.Pipeline.LocalNodes().All())
		m.locals.setGroups(m.deps.Pipeline.LocalNodeGroups())
	}
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update handles keypresses.
//
// Focus model mirrors Settings:
//   - FocusSidebar: ↑/↓ switches between sub-pages (Subscriptions / Local Nodes)
//   - FocusContent: ↑/↓ are delegated to the active sub-page for list navigation
//
// ← / → for focus shifts (Sidebar ↔ Content) are intercepted by the app-level
// handler before this Update fires.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		if !m.InputOpen() && m.focus == FocusSidebar {
			switch km.Type {
			case tea.KeyDown:
				if m.current < numSubPages-1 {
					m.current++
				}
				return m, nil
			case tea.KeyUp:
				if m.current > 0 {
					m.current--
				}
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	switch m.current {
	case SubSubscriptions:
		m.subs, cmd = m.subs.Update(message)
	case SubLocalNodes:
		m.locals, cmd = m.locals.Update(message)
	}
	return m, cmd
}

// View renders the Sources tab.
func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused = View + focus state.
func (m Model) ViewFocused(width, height int, tabBodyFocused bool) string {
	subW := 20
	bodyW := width - subW - 1
	sidebarFocused := tabBodyFocused && m.focus == FocusSidebar
	contentFocused := tabBodyFocused && m.focus == FocusContent

	side := renderSubSidebar(m.current, height, sidebarFocused)
	var body string
	switch m.current {
	case SubSubscriptions:
		body = m.subs.View(bodyW, height, contentFocused)
	case SubLocalNodes:
		body = m.locals.View(bodyW, height, contentFocused)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, side, body)
}

func renderSubSidebar(active SubPage, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Render("Sources")
	rows := []string{header, ""}
	activeColor := lipgloss.Color("240")
	if focused {
		activeColor = lipgloss.Color("212")
	}
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	for i := SubPage(0); i < numSubPages; i++ {
		line := subPageNames[i]
		if i == active {
			rows = append(rows, activeStyle.Render("▶ "+line))
		} else {
			rows = append(rows, inactiveStyle.Render("  "+line))
		}
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] page"))
	return lipgloss.NewStyle().Width(20).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).
		Padding(1, 1).Render(strings.Join(rows, "\n"))
}

// ─── Subscriptions sub-page ───────────────────────────────────────────────

type subsModel struct {
	deps   Deps
	list   []store.Subscription
	cursor int
	form   *subsForm
	flash  string
}

func newSubsModel(deps Deps) subsModel {
	return subsModel{deps: deps}
}

func (m *subsModel) setData(subs []store.Subscription) {
	m.list = subs
	if m.cursor >= len(m.list) && len(m.list) > 0 {
		m.cursor = len(m.list) - 1
	}
}

func (m subsModel) formOpen() bool { return m.form != nil }

func (m subsModel) Update(message tea.Msg) (subsModel, tea.Cmd) {
	if m.form != nil {
		if km, ok := message.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyEsc:
				m.form = nil
				return m, nil
			case tea.KeyEnter:
				if m.form.focused < len(m.form.inputs)-1 {
					m.form.inputs[m.form.focused].Blur()
					m.form.focused++
					m.form.inputs[m.form.focused].Focus()
					return m, nil
				}
				// Submit on last field.
				name := strings.TrimSpace(m.form.inputs[0].Value())
				url := strings.TrimSpace(m.form.inputs[1].Value())
				ua := strings.TrimSpace(m.form.inputs[2].Value())
				if name == "" || url == "" {
					m.flash = "name and URL are required"
					m.form = nil
					return m, nil
				}
				if m.deps.Pipeline != nil {
					if err := m.deps.Pipeline.AddSubscription(store.Subscription{
						Name:      name,
						URL:       url,
						UserAgent: ua,
						Enabled:   true,
					}); err != nil {
						m.flash = "add: " + err.Error()
						m.form = nil
						return m, nil
					}
					m.flash = "added " + name
					m.list = m.deps.Pipeline.SubscriptionNames()
					m.form = nil
					return m, emitPipelineMutated()
				}
				m.form = nil
				return m, nil
			case tea.KeyTab, tea.KeyDown:
				if m.form.focused < len(m.form.inputs)-1 {
					m.form.inputs[m.form.focused].Blur()
					m.form.focused++
					m.form.inputs[m.form.focused].Focus()
				}
				return m, nil
			case tea.KeyShiftTab, tea.KeyUp:
				if m.form.focused > 0 {
					m.form.inputs[m.form.focused].Blur()
					m.form.focused--
					m.form.inputs[m.form.focused].Focus()
				}
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.form.inputs[m.form.focused], cmd = m.form.inputs[m.form.focused].Update(message)
		return m, cmd
	}

	if km, ok := message.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.list)-1 {
				m.cursor++
			}
		case "a":
			m.form = newSubsForm()
		case "d":
			if m.cursor < len(m.list) && m.deps.Pipeline != nil {
				name := m.list[m.cursor].Name
				if err := m.deps.Pipeline.DeleteSubscription(name); err != nil {
					m.flash = "delete: " + err.Error()
				} else {
					m.flash = "deleted " + name
					m.list = m.deps.Pipeline.SubscriptionNames()
					if m.cursor > 0 && m.cursor >= len(m.list) {
						m.cursor = len(m.list) - 1
					}
					return m, emitPipelineMutated()
				}
			}
		case "u":
			if m.cursor < len(m.list) && m.deps.Pipeline != nil {
				name := m.list[m.cursor].Name
				pl := m.deps.Pipeline
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()
					n, err := pl.RefreshSubscription(ctx, name)
					if err != nil {
						return refreshErrMsg{name: name, err: err}
					}
					return refreshDoneMsg{name: name, count: n}
				}
			}
		case "e":
			if m.cursor < len(m.list) && m.deps.Pipeline != nil {
				name := m.list[m.cursor].Name
				if err := m.deps.Pipeline.ToggleSubscriptionEnabled(name); err != nil {
					m.flash = "toggle: " + err.Error()
				} else {
					m.list = m.deps.Pipeline.SubscriptionNames()
					return m, emitPipelineMutated()
				}
			}
		}
	}
	switch ev := message.(type) {
	case refreshDoneMsg:
		m.flash = fmt.Sprintf("✅ %s: %d nodes", ev.name, ev.count)
		if m.deps.Pipeline != nil {
			m.list = m.deps.Pipeline.SubscriptionNames()
		}
		return m, emitPipelineMutated()
	case refreshErrMsg:
		if ev.err != nil {
			m.flash = "❌ " + ev.name + ": " + ev.err.Error()
		}
	}
	return m, nil
}

type refreshDoneMsg struct {
	name  string
	count int
}

type refreshErrMsg struct {
	name string
	err  error
}

func (m subsModel) View(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Subscriptions")
	rows := []string{header, ""}

	if m.flash != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(m.flash), "")
	}

	if m.form != nil {
		rows = append(rows, renderSubsForm(m.form))
		return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).
			Render(strings.Join(rows, "\n"))
	}

	if len(m.list) == 0 {
		rows = append(rows, "  (no subscriptions — press [a] to add)")
	} else {
		innerW := width - 6
		if innerW < 20 {
			innerW = 20
		}
		curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
		for i, s := range m.list {
			enabled := "✓"
			if !s.Enabled {
				enabled = "✗"
			}
			line := fmt.Sprintf("[%s] %-20s  nodes=%-4d  %s", enabled, s.Name, s.NodeCount,
				viewport.TruncateDisplay(s.URL, innerW-40))
			line = viewport.TruncateDisplay(line, innerW)
			if i == m.cursor {
				rows = append(rows, curStyle.Render("▶ ")+line)
			} else {
				rows = append(rows, "  "+line)
			}
		}
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[a] add  [d] delete  [u] update now  [e] toggle enabled"))
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

type subsForm struct {
	inputs  []textinput.Model
	focused int
}

func newSubsForm() *subsForm {
	f := &subsForm{
		inputs: []textinput.Model{
			newTextInput("name (required)", ""),
			newTextInput("URL (required)", ""),
			newTextInput("User-Agent (optional)", ""),
		},
	}
	f.inputs[0].Focus()
	return f
}

func renderSubsForm(f *subsForm) string {
	labels := []string{"Name:", "URL:", "User-Agent:"}
	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Render("Add Subscription"), "")
	for i, label := range labels {
		rows = append(rows, "  "+label)
		rows = append(rows, "  "+f.inputs[i].View())
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[Tab/↑↓] navigate  [Enter] confirm  [Esc] cancel"))
	return strings.Join(rows, "\n")
}

// ─── Local Nodes sub-page ─────────────────────────────────────────────────

type localNodesModel struct {
	deps         Deps
	nodes        []localnodes.Node
	groups       []store.LocalNodeGroup
	currentGroup string
	cursor       int
	form         *localNodeForm
	flash        string
}

func newLocalNodesModel(deps Deps) localNodesModel {
	return localNodesModel{deps: deps}
}

func (m *localNodesModel) setData(nodes []localnodes.Node) {
	m.nodes = nodes
	if m.cursor >= len(m.filteredNodes()) && len(m.filteredNodes()) > 0 {
		m.cursor = len(m.filteredNodes()) - 1
	}
}

func (m *localNodesModel) setGroups(groups []store.LocalNodeGroup) {
	m.groups = groups
	if m.currentGroup == "" && len(m.groups) > 0 {
		m.currentGroup = m.groups[0].Name
	}
}

func (m *localNodesModel) filteredNodes() []localnodes.Node {
	if m.currentGroup == "" {
		return m.nodes
	}
	out := make([]localnodes.Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		if n.Group == m.currentGroup {
			out = append(out, n)
		}
	}
	return out
}

func (m localNodesModel) formOpen() bool { return m.form != nil }

func (m localNodesModel) Update(message tea.Msg) (localNodesModel, tea.Cmd) {
	if m.form != nil {
		if km, ok := message.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyEsc:
				m.form = nil
				return m, nil
			case tea.KeyEnter:
				uri := strings.TrimSpace(m.form.input.Value())
				if uri == "" {
					m.flash = "URI required"
					m.form = nil
					return m, nil
				}
				if m.deps.Pipeline != nil {
					pl := m.deps.Pipeline
					if err := addNodeFromURI(pl, uri); err != nil {
						m.flash = "add: " + err.Error()
						m.form = nil
						return m, nil
					}
					m.flash = "added node"
					m.nodes = pl.LocalNodes().All()
					_ = pl.SaveLocal()
					m.form = nil
					return m, emitPipelineMutated()
				}
				m.form = nil
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.form.input, cmd = m.form.input.Update(message)
		return m, cmd
	}

	if km, ok := message.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
			}
		case "a":
			m.form = newLocalNodeForm()
		case "d":
			if m.cursor < len(m.nodes) && m.deps.Pipeline != nil {
				name := m.nodes[m.cursor].Name
				pl := m.deps.Pipeline
				if err := pl.LocalNodes().Remove(name); err != nil {
					m.flash = "delete: " + err.Error()
				} else if err := pl.SaveLocal(); err != nil {
					m.flash = "save: " + err.Error()
				} else {
					m.flash = "deleted " + name
					m.nodes = pl.LocalNodes().All()
					if m.cursor > 0 && m.cursor >= len(m.nodes) {
						m.cursor = len(m.nodes) - 1
					}
					return m, emitPipelineMutated()
				}
			}
		}
	}
	return m, nil
}

// addNodeFromURI parses a proxy URI and adds it to the local nodes manager.
func addNodeFromURI(pl PipelineFace, uri string) error {
	p, err := proto.Parse(uri)
	if err != nil {
		return err
	}
	node := localnodes.Node{}
	if v, ok := p["name"].(string); ok {
		node.Name = v
	}
	if v, ok := p["type"].(string); ok {
		node.Proto = v
	}
	if v, ok := p["server"].(string); ok {
		node.Server = v
	}
	switch pt := p["port"].(type) {
	case int:
		node.Port = pt
	case float64:
		node.Port = int(pt)
	}
	if node.Name == "" {
		return fmt.Errorf("URI missing name field")
	}
	return pl.LocalNodes().Add(node)
}

func (m localNodesModel) View(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Local Nodes")
	rows := []string{header, ""}

	if m.flash != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(m.flash), "")
	}

	if m.form != nil {
		rows = append(rows, renderLocalNodeForm(m.form))
		return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).
			Render(strings.Join(rows, "\n"))
	}

	if len(m.nodes) == 0 {
		rows = append(rows, "  (no local nodes — press [a] to add via URI)")
	} else {
		innerW := width - 6
		if innerW < 20 {
			innerW = 20
		}
		curStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
		for i, n := range m.nodes {
			portStr := ""
			if n.Port > 0 {
				portStr = fmt.Sprintf(":%d", n.Port)
			}
			line := fmt.Sprintf("%-24s  %-8s  %s%s", n.Name, n.Proto, n.Server, portStr)
			line = viewport.TruncateDisplay(line, innerW)
			if i == m.cursor {
				rows = append(rows, curStyle.Render("▶ ")+line)
			} else {
				rows = append(rows, "  "+line)
			}
		}
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[a] add URI  [d] delete  [↑↓] navigate"))
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

type localNodeForm struct {
	input textinput.Model
}

func newLocalNodeForm() *localNodeForm {
	ti := newTextInput("proxy URI (e.g. vmess://...)", "")
	ti.Focus()
	return &localNodeForm{input: ti}
}

func renderLocalNodeForm(f *localNodeForm) string {
	return strings.Join([]string{
		lipgloss.NewStyle().Bold(true).Render("Add Local Node"),
		"",
		"  Enter proxy URI:",
		"  " + f.input.View(),
		"",
		lipgloss.NewStyle().Faint(true).Render("[Enter] add  [Esc] cancel"),
	}, "\n")
}

func newTextInput(placeholder, value string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 512
	ti.Width = 60
	if value != "" {
		ti.SetValue(value)
	}
	return ti
}
