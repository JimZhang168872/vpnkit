package app

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/tabs/dashboard"
	"vpnkit/internal/tabs/stub"
	tabconnections "vpnkit/internal/tabs/connections"
	tabprofiles    "vpnkit/internal/tabs/profiles"
	tabproxies     "vpnkit/internal/tabs/proxies"
	tabrules       "vpnkit/internal/tabs/rules"
	tabsettings    "vpnkit/internal/tabs/settings"
)

// Tab is the index of the currently-active tab.
type Tab int

const (
	TabDashboard Tab = iota
	TabProxies
	TabProfiles
	TabConnections
	TabRules
	TabSettings
	NumTabs
)

// AppFocus tracks whether the input focus is on the main sidebar (top tab
// list) or on the active tab's body. Default is FocusTabBody so existing
// muscle memory (↑/↓ scrolls Profiles/Rules cursor) keeps working — the
// user opts into top-tab navigation by pressing ← to "escape" to the
// sidebar.
type AppFocus int

const (
	FocusTabBody AppFocus = iota
	FocusMainSidebar
)

var TabNames = [NumTabs]string{
	"🏠 Dashboard", "🚀 Proxies", "📋 Profiles", "🔗 Connections", "📜 Rules", "⚙️  Settings",
}

// Model is the top-level bubbletea model.
type Model struct {
	keys      KeyMap
	activeTab Tab
	width     int
	height    int

	dashboard      dashboard.Model
	profilesTab    tabprofiles.Model
	proxiesTab     tabproxies.Model
	connectionsTab tabconnections.Model
	rulesTab       tabrules.Model
	settingsTab    tabsettings.Model
	stubs          [NumTabs]stub.Model // index 0 unused; entries for non-profiles tabs

	// profilesMgr removed — v1 uses Pipeline. TODO(v1-phase8): remove this tab entirely.
	showAddForm bool
	addForm     tabprofiles.Form

	apiClient *api.Client
	// applyCfg nudges mihomo to pick up config.yaml from disk. Falls back to a
	// full service restart on reload failure (e.g. secret drift after toml
	// regeneration). May be nil in tests.
	applyCfg func(context.Context) error
	flash    string // single-line transient
	// updateBadge is set when pollUpdate finds a new release. Format is the
	// short string we drop into the status bar; e.g. "⚡ v0.9.0".
	updateBadge string
	// appFocus is the global focus level (Bug N). MainSidebar → ↑/↓ cycles
	// top tabs; TabBody → ↑/↓ delegates to active tab's nav.
	appFocus AppFocus

	// proxyNames is the deduped union of mihomo proxy names + group names
	// from the latest /proxies snapshot. Used by Settings → Extensions for
	// autocomplete hints. Held by pointer because bubbletea copies Model by
	// value (a sync.Mutex embedded directly would fail go vet copylocks).
	proxyNames *proxyNamesState
}

type proxyNamesState struct {
	mu    sync.Mutex
	names []string
}

// AppFocus exposes the app-level focus state (for tests / rendering).
func (m *Model) AppFocus() AppFocus { return m.appFocus }

// inputOpen reports whether some textinput-style overlay is consuming
// keypresses (Profiles add-form, or a filter input). When true the global
// focus shifter must NOT eat keys.
func (m Model) inputOpen() bool {
	if m.showAddForm {
		return true
	}
	if m.connectionsTab.IsFiltering() {
		return true
	}
	if m.rulesTab.IsFiltering() {
		return true
	}
	// Extensions inline form (add/edit chain/group) absorbs every key
	// including ←/→ for cursor positioning inside the textinput; the
	// app-level focus shifter must NOT eat them.
	if m.activeTab == TabSettings && m.settingsTab.InputOpen() {
		return true
	}
	return false
}

// shiftFocusLeft moves focus one step toward the main sidebar.
//   Settings content → Settings sidebar
//   Settings sidebar (or any other TabBody) → MainSidebar
//   MainSidebar → no-op
func (m Model) shiftFocusLeft() Model {
	if m.activeTab == TabSettings && m.appFocus == FocusTabBody {
		if m.settingsTab.Focus() == tabsettings.FocusContent {
			m.settingsTab.SetFocus(tabsettings.FocusSidebar)
			return m
		}
	}
	m.appFocus = FocusMainSidebar
	return m
}

// shiftFocusRight moves focus one step away from the main sidebar.
//   MainSidebar → TabBody
//   Settings sidebar (on a sub-page that owns content) → Settings content
//   Otherwise → no-op
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
	return m
}

// CurrentProxyNames returns the latest known set of mihomo proxy + group
// names. Safe for concurrent reads; returns a defensive copy.
func (m *Model) CurrentProxyNames() []string {
	if m == nil || m.proxyNames == nil {
		return nil
	}
	m.proxyNames.mu.Lock()
	defer m.proxyNames.mu.Unlock()
	out := make([]string, len(m.proxyNames.names))
	copy(out, m.proxyNames.names)
	return out
}

// recordProxyNames captures the deduped union of group names and their
// member proxy names from the latest snapshot.
func (m *Model) recordProxyNames(snap ProxiesSnapshot) {
	if m.proxyNames == nil {
		return
	}
	m.proxyNames.mu.Lock()
	defer m.proxyNames.mu.Unlock()
	m.proxyNames.names = m.proxyNames.names[:0]
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		m.proxyNames.names = append(m.proxyNames.names, name)
	}
	for name, g := range snap.Groups {
		add(name)
		for _, n := range g.All {
			add(n)
		}
	}
}

// NewModel constructs the initial model. client may be nil during tests.
// TODO(v1-phase8): mgr param removed; second arg is now ignored (passes nil from run.go).
func NewModel(client *api.Client, _ any, settingsDeps tabsettings.Deps, applyCfg func(context.Context) error) Model {
	stubs := [NumTabs]stub.Model{}
	for i := TabProxies; i < NumTabs; i++ {
		if i == TabProfiles || i == TabProxies || i == TabConnections || i == TabRules || i == TabSettings {
			continue
		}
		stubs[i] = stub.New(TabNames[i])
	}
	pt := tabprofiles.New(nil)
	return Model{
		keys:           DefaultKeys(),
		activeTab:      TabDashboard,
		dashboard:      dashboard.New(),
		profilesTab:    pt,
		proxiesTab:     tabproxies.New(),
		connectionsTab: tabconnections.New(),
		rulesTab:       tabrules.New(),
		settingsTab:    tabsettings.New(settingsDeps),
		stubs:          stubs,
		apiClient:      client,
		applyCfg:       applyCfg,
		proxyNames:     &proxyNamesState{},
	}
}

// Init returns startup commands.
func (m Model) Init() tea.Cmd {
	return nil // bootstrap & subscriptions are wired in app.Run
}
