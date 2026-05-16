package app

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/profiles"
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

	profilesMgr *profiles.Manager
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

// NewModel constructs the initial model. client and mgr may be nil during tests.
func NewModel(client *api.Client, mgr *profiles.Manager, settingsDeps tabsettings.Deps, applyCfg func(context.Context) error) Model {
	stubs := [NumTabs]stub.Model{}
	for i := TabProxies; i < NumTabs; i++ {
		if i == TabProfiles || i == TabProxies || i == TabConnections || i == TabRules || i == TabSettings {
			continue
		}
		stubs[i] = stub.New(TabNames[i])
	}
	pt := tabprofiles.New(mgr)
	if mgr != nil {
		pt.SetProfiles(mgr.All(), mgr.Active())
	}
	return Model{
		keys:           DefaultKeys(),
		activeTab:      TabDashboard,
		dashboard:      dashboard.New(),
		profilesTab:    pt,
		profilesMgr:    mgr,
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
