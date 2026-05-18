// Package settings implements the Settings tab and its sub-pages.
package settings

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/api"
	"vpnkit/internal/msg"
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
	SubActive
	SubRules
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
	// meaningful on sub-pages whose subPageOwnsArrows returns true).
	FocusContent
)

// SubPageNames is human labels for the sidebar.
var SubPageNames = [NumSubPages]string{
	"Mihomo Core",
	"Service",
	"External Controller",
	"Routing",
	"Active Source",
	"Rule Template",
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
	// rc.7 active-source picker (Settings → Active Source sub-page).
	ActiveSource() string
	SetActiveSource(name string) error
	// Routing knobs — routed through Pipeline so mutation is serialized
	// under p.mu. Direct store.Cfg mutation from the TUI goroutine races
	// with concurrent Assemble().
	SetMode(mode string) error
	Mode() string
	RegenerateControllerSecret() error
	// Snapshots for read paths — return safe copies under p.mu so the
	// TUI render/Update goroutine never races with concurrent Pipeline
	// mutations on store.Cfg.
	SubscriptionNames() []store.Subscription
	LocalNodeGroups() []store.LocalNodeGroup
}

// Deps are wires for sub-pages.
type Deps struct {
	Paths     paths.XDG
	Store     *store.Store
	Service   service.Manager
	APIClient *api.Client
	Pipeline  PipelineFace // v1 multi-source pipeline; nil until wired in run.go
	ApplyFunc func() error // reassemble + reload mihomo; nil in tests
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
	routing    routingModel
	active     activeModel
}

// Focus exposes the active focus state (for tests / rendering).
func (m Model) Focus() SettingsFocus { return m.focus }

// SetFocus updates the inner focus state. Called by the app-level ←/→
// handler when it wants to shift focus between this tab's sub-sidebar and
// the active sub-page's content panel.
func (m *Model) SetFocus(f SettingsFocus) { m.focus = f }

// SubPageOwnsContent reports whether the active sub-page has a navigable
// content panel that the user can shift focus into. App-level →/← uses this
// to decide whether to advance inner focus or to bounce focus all the way to
// MainSidebar.
func (m Model) SubPageOwnsContent() bool { return subPageOwnsArrows(m.current) }

// InputOpen reports whether a sub-page is currently in a state where every
// key should be delivered to it. Currently no Settings sub-page opens a
// textinput overlay, so this is always false — kept on the interface so the
// app-level focus shifter has a consistent escape hatch.
func (m Model) InputOpen() bool { return false }

// New constructs the Settings tab Model with all sub-pages instantiated.
func New(deps Deps) Model {
	return Model{
		deps:       deps,
		about:      newAbout(),
		cache:      newCache(deps.Paths),
		rules:      newRules(deps.Store),
		controller: newController(deps.Store, deps.Pipeline),
		service:    newService(deps.Service),
		core:       newCore(deps.Paths, deps.Store),
		routing:    newRouting(deps.Store, deps.Pipeline, deps.ApplyFunc),
		active:     newActive(deps.Store, deps.Pipeline, deps.ApplyFunc),
	}
}

// SelectedPage exposes the active sub-page (for tests).
func (m Model) SelectedPage() SubPage { return m.current }

// subPageOwnsArrows reports whether ↑/↓ should be delegated to the sub-page
// (because it has its own list navigation) rather than used at the parent
// level to switch between sub-pages. Add new sub-pages with internal nav
// here.
func subPageOwnsArrows(p SubPage) bool {
	return p == SubRouting || p == SubRules || p == SubActive
}

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		// Focus-based navigation model:
		//   ←/→ : owned by app-level handler — shifts focus
		//         (MainSidebar ↔ Settings sidebar ↔ Settings content) in one
		//         consistent pattern across the whole app.
		//   ↑↓  : on FocusContent + sub-page-owns-arrows → delegate to sub-page;
		//         else switch sub-page (and reset focus to sidebar).
		// Any sub-page change resets focus to sidebar so the user doesn't
		// land on a non-arrow-owning page with stale FocusContent.
		switch km.Type {
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
	// Service status is pushed by an app-level poller and must reach the
	// service sub-page regardless of which page is currently focused.
	// Skip when SubService is the active page — the page-dispatch switch
	// below would otherwise call serviceModel.Update twice with the same
	// message (currently idempotent, but fragile if the handler ever
	// gains side effects like appending to a log).
	if _, ok := message.(msg.ServiceStatus); ok && m.current != SubService {
		m.service, _ = m.service.Update(message)
	}
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
	case SubRouting:
		m.routing, cmd = m.routing.Update(message)
	case SubActive:
		m.active, cmd = m.active.Update(message)
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
		body = m.rules.ViewFocused(bodyWidth, height, contentFocused)
	case SubController:
		body = m.controller.View(bodyWidth, height)
	case SubService:
		body = m.service.View(bodyWidth, height)
	case SubCore:
		body = m.core.View(bodyWidth, height)
	case SubRouting:
		body = m.routing.ViewFocused(bodyWidth, height, contentFocused)
	case SubActive:
		body = m.active.ViewFocused(bodyWidth, height, contentFocused)
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
