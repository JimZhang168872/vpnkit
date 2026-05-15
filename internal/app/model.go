package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/profiles"
	"vpnkit/internal/tabs/dashboard"
	"vpnkit/internal/tabs/stub"
	tabprofiles "vpnkit/internal/tabs/profiles"
	tabproxies "vpnkit/internal/tabs/proxies"
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
	"Dashboard", "Proxies", "Profiles", "Connections", "Rules", "Settings",
}

// Model is the top-level bubbletea model.
type Model struct {
	keys      KeyMap
	activeTab Tab
	width     int
	height    int

	dashboard   dashboard.Model
	profilesTab tabprofiles.Model
	proxiesTab  tabproxies.Model
	stubs       [NumTabs]stub.Model // index 0 unused; entries for non-profiles tabs

	profilesMgr *profiles.Manager
	showAddForm bool
	addForm     tabprofiles.Form

	apiClient *api.Client
	flash     string // single-line transient
}

// NewModel constructs the initial model. client and mgr may be nil during tests.
func NewModel(client *api.Client, mgr *profiles.Manager) Model {
	stubs := [NumTabs]stub.Model{}
	for i := TabProxies; i < NumTabs; i++ {
		if i == TabProfiles {
			continue
		}
		if i == TabProxies {
			continue
		}
		stubs[i] = stub.New(TabNames[i])
	}
	pt := tabprofiles.New(mgr)
	if mgr != nil {
		pt.SetProfiles(mgr.All(), mgr.Active())
	}
	return Model{
		keys:        DefaultKeys(),
		activeTab:   TabDashboard,
		dashboard:   dashboard.New(),
		profilesTab: pt,
		profilesMgr: mgr,
		proxiesTab:  tabproxies.New(),
		stubs:       stubs,
		apiClient:   client,
	}
}

// Init returns startup commands.
func (m Model) Init() tea.Cmd {
	return nil // bootstrap & subscriptions are wired in app.Run
}
