// Package settings implements the Settings tab and its sub-pages.
package settings

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/api"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/viewport"
)

// SubPage identifies a sub-page.
type SubPage int

const (
	SubCore SubPage = iota
	SubService
	SubController
	SubRouting
	SubRules
	SubExtensions
	SubCache
	SubAbout
	NumSubPages
)

// SettingsFocus is the two-state focus the user toggles with ←/→ inside
// Settings to indicate which panel ↑/↓ should affect. Without this, ↑/↓
// behavior depended on which sub-page was active and the user couldn't
// tell what they were navigating.
type SettingsFocus int

const (
	// FocusSidebar = ↑/↓ moves between sub-pages.
	FocusSidebar SettingsFocus = iota
	// FocusContent = ↑/↓ goes to the sub-page's internal list (only
	// meaningful on sub-pages that own arrows, currently SubExtensions).
	FocusContent
)

// SubPageNames is human labels for the sidebar.
var SubPageNames = [NumSubPages]string{
	"Mihomo Core",
	"Service",
	"External Controller",
	"Routing",
	"Default Rules",
	"Extensions",
	"Cache",
	"About",
}

// PipelineFace is the subset of *app.Pipeline that the settings tab needs.
// Declared here (not in app/) to break the package import cycle: settings
// cannot import app because app imports settings.
type PipelineFace interface {
	RefreshSubscription(ctx context.Context, name string) (int, error)
	Assemble() error
	SaveLocal() error
}

// Deps are wires for sub-pages.
type Deps struct {
	Paths          paths.XDG
	Store          *store.Store
	Service        service.Manager
	APIClient      *api.Client
	ExtensionsPath string         // ~/.config/vpnkit/extensions.toml (empty in tests = uses Paths)
	Pipeline       PipelineFace   // v1 multi-source pipeline; nil until wired in run.go
	ProxyNames     ProxyNamesFunc // returns proxy+group names from latest snapshot
	ApplyFunc      func() error   // reassemble + reload mihomo; nil in tests
}

// Model is the Settings tab.
type Model struct {
	deps    Deps
	current SubPage
	focus   SettingsFocus

	about      aboutModel
	cache      cacheModel
	rules      rulesModel
	controller controllerModel
	service    serviceModel
	core       coreModel
	extensions extensionsModel
	routing    routingModel
}

// Focus exposes the active focus state (for tests / rendering).
func (m Model) Focus() SettingsFocus { return m.focus }

// SetFocus updates the inner focus state. Called by the app-level ←/→
// handler when it wants to shift focus between this tab's sub-sidebar and
// the active sub-page's content panel.
func (m *Model) SetFocus(f SettingsFocus) { m.focus = f }

// SubPageOwnsContent reports whether the active sub-page has a navigable
// content panel that the user can shift focus into (currently Extensions
// is the only one). App-level →/← uses this to decide whether to advance
// inner focus or to bounce focus all the way to MainSidebar.
func (m Model) SubPageOwnsContent() bool { return subPageOwnsArrows(m.current) }

// InputOpen reports whether a sub-page is currently in a state where every
// key should be delivered to it (e.g. Extensions add/edit form). Used by
// the app's global focus shifter to step aside.
func (m Model) InputOpen() bool {
	return m.current == SubExtensions && m.extensions.formOpen()
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
		routing:    newRouting(deps.Store, deps.ApplyFunc),
	}
}

// SelectedPage exposes the active sub-page (for tests).
func (m Model) SelectedPage() SubPage { return m.current }

// subPageOwnsArrows reports whether ↑/↓ should be delegated to the sub-page
// (because it has its own list navigation) rather than used at the parent
// level to switch between sub-pages. Add new sub-pages with internal nav
// here.
func subPageOwnsArrows(p SubPage) bool {
	return p == SubExtensions || p == SubRouting
}

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		// Focus-based navigation model (Bug M):
		//   ←  : if Extensions+FocusContent → focus sidebar; else prev sub-page
		//   →  : if Extensions+FocusSidebar → focus content; else next sub-page
		//   ↑↓ : on FocusContent → delegate to sub-page; else switch sub-page
		//   PgUp/PgDn: ALWAYS switch sub-page (force exit from content)
		// Any sub-page change resets focus to sidebar so the user doesn't
		// land on a non-Extensions page with stale FocusContent.
		switch km.Type {
		// ←/→ are owned by the app-level handler now (Bug N): they shift
		// focus between MainSidebar / Settings sidebar / Settings content
		// in one consistent model across the whole app. Settings.Update
		// no longer consumes them — the app intercepts before delegating.
		case tea.KeyPgDown:
			if m.current < NumSubPages-1 {
				m.current++
			}
			m.focus = FocusSidebar
			return m, nil
		case tea.KeyPgUp:
			if m.current > 0 {
				m.current--
			}
			m.focus = FocusSidebar
			return m, nil
		case tea.KeyDown:
			if subPageOwnsArrows(m.current) && m.focus == FocusContent {
				// fall through to sub-page delegation below
				break
			}
			if m.current < NumSubPages-1 {
				m.current++
			}
			m.focus = FocusSidebar
			return m, nil
		case tea.KeyUp:
			if subPageOwnsArrows(m.current) && m.focus == FocusContent {
				break
			}
			if m.current > 0 {
				m.current--
			}
			m.focus = FocusSidebar
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
	case SubExtensions:
		m.extensions, cmd = m.extensions.Update(message)
	case SubRouting:
		m.routing, cmd = m.routing.Update(message)
	}
	return m, cmd
}

// View defaults to TabBody-focused for direct callers (tests). app/view.go
// passes the app-level focus via ViewFocused.
func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused renders Settings with the given app-level "is this tab body
// focused?" flag. The inner Settings focus state (Sidebar / Content) is
// combined with it so an unfocused tab never shows a bright cursor —
// neither sub-sidebar nor content can "own" input when the user has
// shifted focus to the MainSidebar.
func (m Model) ViewFocused(width, height int, tabBodyFocused bool) string {
	subWidth := 22
	bodyWidth := width - subWidth - 1
	sidebarFocused := tabBodyFocused && m.focus == FocusSidebar
	contentFocused := tabBodyFocused && m.focus == FocusContent
	side := renderSubSidebar(m.current, height, sidebarFocused)
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
		body = m.extensions.ViewFocused(bodyWidth, height, contentFocused)
	case SubRouting:
		body = m.routing.View(bodyWidth, height)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, side, body)
}

func renderSubSidebar(active SubPage, height int, focused bool) string {
	// Focus dot lives at top-left of EVERY panel — consistent UX across
	// MainSidebar / Settings sub-sidebar / Settings content / each tab body.
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Render("Settings")
	rows := []string{header, ""}
	activeColor := lipgloss.Color("240") // dim when not focused
	if focused {
		activeColor = lipgloss.Color("212")
	}
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	for i := SubPage(0); i < NumSubPages; i++ {
		line := SubPageNames[i]
		if i == active {
			rows = append(rows, activeStyle.Render("▶ "+line))
		} else {
			rows = append(rows, inactiveStyle.Render("  "+line))
		}
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render("[↑↓] page"))
	return lipgloss.NewStyle().Width(22).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).
		Padding(1, 1).Render(strings.Join(rows, "\n"))
}
