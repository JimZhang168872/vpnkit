package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/tabs/dashboard"
	"vpnkit/internal/tabs/stub"
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

	dashboard dashboard.Model
	stubs     [NumTabs]stub.Model // index 0 unused; entries for 1..5

	apiClient *api.Client
	flash     string // single-line transient
}

// NewModel constructs the initial model. apiClient may be nil during early bootstrap.
func NewModel(client *api.Client) Model {
	stubs := [NumTabs]stub.Model{}
	for i := TabProxies; i < NumTabs; i++ {
		stubs[i] = stub.New(TabNames[i])
	}
	return Model{
		keys:      DefaultKeys(),
		activeTab: TabDashboard,
		dashboard: dashboard.New(),
		stubs:     stubs,
		apiClient: client,
	}
}

// Init returns startup commands.
func (m Model) Init() tea.Cmd {
	return nil // bootstrap & subscriptions are wired in app.Run
}
