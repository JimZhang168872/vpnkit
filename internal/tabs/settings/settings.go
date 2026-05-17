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
	SubExtensions
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
	"Extensions",
	"Logs",
	"Cache",
	"About",
}

// Deps are wires for sub-pages.
type Deps struct {
	Paths          paths.XDG
	Store          *store.Store
	Service        service.Manager
	APIClient      *api.Client
	ExtensionsPath string         // ~/.config/vpnkit/extensions.toml (empty in tests = uses Paths)
	ProxyNames     ProxyNamesFunc // returns proxy+group names from latest snapshot
	ApplyFunc      func() error   // reassemble + reload mihomo; nil in tests
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
	extensions extensionsModel
	logs       logs.Model
}

// New constructs the Settings tab Model with all sub-pages instantiated.
func New(deps Deps) Model {
	extPath := deps.ExtensionsPath
	if extPath == "" {
		// Fallback for tests / unwired callers — keeps the sub-page usable.
		extPath = "/tmp/extensions.toml"
	}
	ex := newExtensions(extPath, deps.ProxyNames)
	ex.applyFunc = deps.ApplyFunc
	return Model{
		deps:       deps,
		about:      newAbout(),
		cache:      newCache(deps.Paths),
		rules:      newRules(deps.Store),
		controller: newController(deps.Store),
		service:    newService(deps.Service),
		core:       newCore(deps.Paths, deps.Store),
		extensions: ex,
		logs:       logs.New(),
	}
}

// SelectedPage exposes the active sub-page (for tests).
func (m Model) SelectedPage() SubPage { return m.current }

// subPageOwnsArrows reports whether ↑/↓ should be delegated to the sub-page
// (because it has its own list navigation) rather than used at the parent
// level to switch between sub-pages. Add new sub-pages with internal nav
// here.
func subPageOwnsArrows(p SubPage) bool {
	return p == SubExtensions
}

// LogsModel exposes the embedded Logs model so the parent app can route LogLine into it.
func (m *Model) LogsModel() *logs.Model { return &m.logs }

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		// Smart ↑/↓ dispatch:
		//   - PgUp / PgDown always switch sub-page (force-exit a sub-page
		//     that owns its arrow keys).
		//   - ↑ / ↓:
		//       * On a sub-page that owns its own list navigation
		//         (currently only SubExtensions) → delegate to the sub-page.
		//       * On a sub-page with no internal list → switch sub-page
		//         (the user's intuition).
		// Previous attempts at "always intercept ↑/↓" broke Extensions; at
		// "never intercept ↑/↓" broke every other sub-page (they didn't
		// react at all because they don't consume the key).
		switch km.Type {
		case tea.KeyPgDown:
			if m.current < NumSubPages-1 {
				m.current++
			}
			return m, nil
		case tea.KeyPgUp:
			if m.current > 0 {
				m.current--
			}
			return m, nil
		case tea.KeyDown:
			if !subPageOwnsArrows(m.current) {
				if m.current < NumSubPages-1 {
					m.current++
				}
				return m, nil
			}
		case tea.KeyUp:
			if !subPageOwnsArrows(m.current) {
				if m.current > 0 {
					m.current--
				}
				return m, nil
			}
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
	case SubExtensions:
		m.extensions, cmd = m.extensions.Update(message)
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
	case SubExtensions:
		body = m.extensions.View(bodyWidth, height)
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
	rows = append(rows, "", "[↑↓ / PgUp/PgDn]")
	return lipgloss.NewStyle().Width(22).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).
		Padding(1, 1).Render(strings.Join(rows, "\n"))
}
