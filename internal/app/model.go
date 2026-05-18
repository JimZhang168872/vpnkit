package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/store"
	"vpnkit/internal/tabs/dashboard"
	"vpnkit/internal/tabs/stub"
	tabconnections "vpnkit/internal/tabs/connections"
	tabgroups      "vpnkit/internal/tabs/groups"
	tablogs        "vpnkit/internal/tabs/logs"
	tabrules       "vpnkit/internal/tabs/rules"
	tabsettings    "vpnkit/internal/tabs/settings"
	tabsources     "vpnkit/internal/tabs/sources"
)

// Tab is the index of the currently-active tab.
type Tab int

const (
	TabDashboard Tab = iota
	TabGroups
	TabSources
	TabRules
	TabConnections
	TabLogs
	TabSettings
	NumTabs
)

// AppFocus tracks whether the input focus is on the main sidebar (top tab
// list) or on the active tab's body. Default is FocusTabBody so existing
// muscle memory (↑/↓ scrolls rules cursor) keeps working — the user opts
// into top-tab navigation by pressing ← to "escape" to the sidebar.
type AppFocus int

const (
	FocusTabBody AppFocus = iota
	FocusMainSidebar
)

var TabNames = [NumTabs]string{
	"🏠 Dashboard", "🌐 Groups", "📚 Sources", "📜 Rules", "🔗 Connections", "📓 Logs", "⚙️  Settings",
}

// Model is the top-level bubbletea model.
type Model struct {
	keys      KeyMap
	activeTab Tab
	width     int
	height    int

	dashboard      dashboard.Model
	groupsTab      tabgroups.Model
	sourcesTab     tabsources.Model
	connectionsTab tabconnections.Model
	rulesTab       tabrules.Model
	logsTab        tablogs.Model
	settingsTab    tabsettings.Model
	stubs          [NumTabs]stub.Model

	apiClient *api.Client
	// applyCfg nudges mihomo to pick up config.yaml from disk. Falls back to a
	// full service restart on reload failure (e.g. secret drift after toml
	// regeneration). May be nil in tests.
	applyCfg func(context.Context) error
	flash    string // single-line transient
	// updateBadge is set when pollUpdate finds a new release.
	updateBadge string
	// appFocus is the global focus level (Bug N). MainSidebar → ↑/↓ cycles
	// top tabs; TabBody → ↑/↓ delegates to active tab's nav.
	appFocus AppFocus
}

// AppFocus exposes the app-level focus state (for tests / rendering).
func (m *Model) AppFocus() AppFocus { return m.appFocus }

// inputOpen reports whether some textinput-style overlay is consuming
// keypresses. When true the global focus shifter must NOT eat keys.
func (m Model) inputOpen() bool {
	if m.connectionsTab.IsFiltering() {
		return true
	}
	if m.rulesTab.IsFiltering() {
		return true
	}
	if m.activeTab == TabSettings && m.settingsTab.InputOpen() {
		return true
	}
	if m.activeTab == TabSources && m.sourcesTab.InputOpen() {
		return true
	}
	return false
}

// shiftFocusLeft moves focus one step toward the main sidebar.
func (m Model) shiftFocusLeft() Model {
	if m.activeTab == TabSettings && m.appFocus == FocusTabBody {
		if m.settingsTab.Focus() == tabsettings.FocusContent {
			m.settingsTab.SetFocus(tabsettings.FocusSidebar)
			return m
		}
	}
	if m.activeTab == TabSources && m.appFocus == FocusTabBody {
		if m.sourcesTab.Focus() == tabsources.FocusContent {
			m.sourcesTab.SetFocus(tabsources.FocusSidebar)
			return m
		}
	}
	if m.activeTab == TabGroups && m.appFocus == FocusTabBody {
		if m.groupsTab.SubFocus() == tabgroups.SubFocusRight {
			m.groupsTab.SetSubFocus(tabgroups.SubFocusLeft)
			return m
		}
	}
	m.appFocus = FocusMainSidebar
	return m
}

// shiftFocusRight moves focus one step away from the main sidebar.
func (m Model) shiftFocusRight() Model {
	if m.appFocus == FocusMainSidebar {
		m.appFocus = FocusTabBody
		return m
	}
	if m.activeTab == TabSettings &&
		m.settingsTab.Focus() == tabsettings.FocusSidebar &&
		m.settingsTab.SubPageOwnsContent() {
		m.settingsTab.SetFocus(tabsettings.FocusContent)
	}
	if m.activeTab == TabSources && m.sourcesTab.Focus() == tabsources.FocusSidebar {
		// Sources sub-pages always have a navigable content panel (list).
		m.sourcesTab.SetFocus(tabsources.FocusContent)
	}
	if m.activeTab == TabGroups && m.groupsTab.SubFocus() == tabgroups.SubFocusLeft {
		// Drill into the selected group's node list.
		m.groupsTab.SetSubFocus(tabgroups.SubFocusRight)
	}
	return m
}

// NewModel constructs the initial model. client may be nil during tests.
func NewModel(client *api.Client, settingsDeps tabsettings.Deps, applyCfg func(context.Context) error) Model {
	stubs := [NumTabs]stub.Model{}
	// Only tabs without a real implementation get a stub — all current tabs
	// have implementations so this loop is effectively a no-op.
	for i := Tab(0); i < NumTabs; i++ {
		stubs[i] = stub.New(TabNames[i])
	}
	return Model{
		keys:           DefaultKeys(),
		activeTab:      TabDashboard,
		dashboard:      dashboard.New(),
		groupsTab:      tabgroups.New(tabgroups.Deps{}), // deps wired in run.go via SetDeps
		sourcesTab:     tabsources.New(tabsources.Deps{}),
		connectionsTab: tabconnections.New(),
		rulesTab:       tabrules.New(),
		logsTab:        tablogs.New(),
		settingsTab:    tabsettings.New(settingsDeps),
		stubs:          stubs,
		apiClient:      client,
		applyCfg:       applyCfg,
	}
}

// WirePipeline injects the Pipeline into the groups and sources tabs.
// Must be called in run.go after NewModel and before prog.Run.
func (m *Model) WirePipeline(pl *Pipeline) {
	// Groups tab deps — convert app.SubNode to groups.SubNode in closures.
	m.groupsTab = tabgroups.New(tabgroups.Deps{
		GetSubs: func() []store.Subscription {
			return pl.SubscriptionNames()
		},
		GetSubNodes: func(name string) []tabgroups.SubNode {
			raw := pl.SubscriptionNodes(name)
			if raw == nil {
				return nil
			}
			out := make([]tabgroups.SubNode, len(raw))
			for i, n := range raw {
				out[i] = tabgroups.SubNode{Name: n.Name, Proto: n.Proto, Server: n.Server, Port: n.Port}
			}
			return out
		},
		GetLocalGroups: func() []store.LocalNodeGroup {
			return pl.LocalNodeGroups()
		},
		GetLocalNodes: func(group string) []tabgroups.SubNode {
			all := pl.LocalNodes().All()
			out := []tabgroups.SubNode{}
			for _, n := range all {
				if n.Group != group {
					continue
				}
				out = append(out, tabgroups.SubNode{
					Name: n.Name, Proto: n.Proto, Server: n.Server, Port: n.Port,
				})
			}
			return out
		},
		GetActiveSource: pl.ActiveSource,
	})
	// Sources tab — reuse the PipelineFace interface directly; Pipeline satisfies it.
	m.sourcesTab = tabsources.New(tabsources.Deps{Pipeline: pl})
	// Rules tab — wire pipeline for Local Rules sub-page.
	m.rulesTab.SetPipeline(pl)
	// Pull initial data from store/pipeline into the just-constructed tab models.
	// Without these, Groups shows "(none)" and Sources shows "(no subscriptions)"
	// even when the store already has entries (e.g. from CLI subs add before launch).
	m.groupsTab.Refresh()
	m.sourcesTab.Refresh()
}

// Init returns startup commands.
func (m Model) Init() tea.Cmd {
	return nil // bootstrap & subscriptions are wired in app.Run
}
