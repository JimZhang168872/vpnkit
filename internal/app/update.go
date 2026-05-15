package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(v, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(v, m.keys.Tab1):
			m.activeTab = TabDashboard
		case key.Matches(v, m.keys.Tab2):
			m.activeTab = TabProxies
		case key.Matches(v, m.keys.Tab3):
			m.activeTab = TabProfiles
		case key.Matches(v, m.keys.Tab4):
			m.activeTab = TabConnections
		case key.Matches(v, m.keys.Tab5):
			m.activeTab = TabRules
		case key.Matches(v, m.keys.Tab6):
			m.activeTab = TabSettings
		case key.Matches(v, m.keys.NextTab):
			m.activeTab = (m.activeTab + 1) % NumTabs
		case key.Matches(v, m.keys.PrevTab):
			m.activeTab = (m.activeTab + NumTabs - 1) % NumTabs
		}
	case TrafficMsg, VersionMsg, ServiceStatusMsg:
		m.dashboard, cmd = m.dashboard.Update(msg)
	case BootstrapProgressMsg:
		switch v.Phase {
		case "ready":
			m.flash = "mihomo ready"
		case "error":
			if v.Err != nil {
				m.flash = "bootstrap: " + v.Err.Error()
			}
		default:
			m.flash = "bootstrapping: " + v.Phase
		}
	}
	return m, cmd
}
