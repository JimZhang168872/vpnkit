// Package settings implements the Settings tab and its sub-pages.
package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/api"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/logs"
)

// SubPage identifies a sub-page.
type SubPage int

const (
	SubCore SubPage = iota
	SubService
	SubController
	SubRules
	SubPatch
	SubLogs
	SubCache
	SubAbout
	NumSubPages
)

// SubPageNames is human labels for the sidebar.
var SubPageNames = [NumSubPages]string{
	"Mihomo Core",
	"Service",
	"External Controller",
	"Default Rules",
	"Patch Editor",
	"Logs",
	"Cache",
	"About",
}

// Deps are wires for sub-pages.
type Deps struct {
	Paths     paths.XDG
	Store     *store.Store
	Service   service.Manager
	APIClient *api.Client
}

// Model is the Settings tab.
type Model struct {
	deps    Deps
	current SubPage

	about      aboutModel
	cache      cacheModel
	rules      rulesModel
	controller controllerModel
	service    serviceModel
	core       coreModel
	patch      patchModel
	logs       logs.Model
}

// New constructs the Settings tab Model with all sub-pages instantiated.
func New(deps Deps) Model {
	return Model{
		deps:       deps,
		about:      newAbout(),
		cache:      newCache(deps.Paths),
		rules:      newRules(deps.Store),
		controller: newController(deps.Store),
		service:    newService(deps.Service),
		core:       newCore(deps.Paths, deps.Store),
		patch:      newPatch(deps.Paths),
		logs:       logs.New(),
	}
}

// SelectedPage exposes the active sub-page (for tests).
func (m Model) SelectedPage() SubPage { return m.current }

// LogsModel exposes the embedded Logs model so the parent app can route LogLine into it.
func (m *Model) LogsModel() *logs.Model { return &m.logs }

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyDown:
			if m.current < NumSubPages-1 {
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
	var cmd tea.Cmd
	switch m.current {
	case SubAbout:
		m.about, cmd = m.about.Update(message)
	case SubCache:
		m.cache, cmd = m.cache.Update(message)
	case SubRules:
		m.rules, cmd = m.rules.Update(message)
	case SubController:
		m.controller, cmd = m.controller.Update(message)
	case SubService:
		m.service, cmd = m.service.Update(message)
	case SubCore:
		m.core, cmd = m.core.Update(message)
	case SubPatch:
		m.patch, cmd = m.patch.Update(message)
	case SubLogs:
		m.logs, cmd = m.logs.Update(message)
	}
	return m, cmd
}

func (m Model) View(width, height int) string {
	subWidth := 22
	bodyWidth := width - subWidth - 1
	side := renderSubSidebar(m.current, height)
	var body string
	switch m.current {
	case SubAbout:
		body = m.about.View(bodyWidth, height)
	case SubCache:
		body = m.cache.View(bodyWidth, height)
	case SubRules:
		body = m.rules.View(bodyWidth, height)
	case SubController:
		body = m.controller.View(bodyWidth, height)
	case SubService:
		body = m.service.View(bodyWidth, height)
	case SubCore:
		body = m.core.View(bodyWidth, height)
	case SubPatch:
		body = m.patch.View(bodyWidth, height)
	case SubLogs:
		body = m.logs.View(bodyWidth, height)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, side, body)
}

func renderSubSidebar(active SubPage, height int) string {
	header := lipgloss.NewStyle().Bold(true).Render("Settings")
	rows := []string{header, ""}
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	for i := SubPage(0); i < NumSubPages; i++ {
		line := SubPageNames[i]
		if i == active {
			rows = append(rows, activeStyle.Render("▶ "+line))
		} else {
			rows = append(rows, inactiveStyle.Render("  "+line))
		}
	}
	rows = append(rows, "", "[↑↓] navigate")
	return lipgloss.NewStyle().Width(22).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).
		Padding(1, 1).Render(strings.Join(rows, "\n"))
}

// Stubs for sub-page Models — replaced in subsequent tasks.
type serviceModel struct{}
type coreModel struct{}
type patchModel struct{}

func newService(service.Manager) serviceModel         { return serviceModel{} }
func newCore(paths.XDG, *store.Store) coreModel       { return coreModel{} }
func newPatch(paths.XDG) patchModel                   { return patchModel{} }

func (m serviceModel) Update(tea.Msg) (serviceModel, tea.Cmd)       { return m, nil }
func (m coreModel) Update(tea.Msg) (coreModel, tea.Cmd)             { return m, nil }
func (m patchModel) Update(tea.Msg) (patchModel, tea.Cmd)           { return m, nil }

func (m serviceModel) View(_, _ int) string    { return "  Service: (T6)" }
func (m coreModel) View(_, _ int) string       { return "  Mihomo Core: (T7)" }
func (m patchModel) View(_, _ int) string      { return "  Patch Editor: (T8)" }
