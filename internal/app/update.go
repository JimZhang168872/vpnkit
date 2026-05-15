package app

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/profiles"
	tabprofiles "vpnkit/internal/tabs/profiles"
	tabproxies "vpnkit/internal/tabs/proxies"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// When the add-form overlay is open, route all key input to it (except Enter/Esc).
	if m.showAddForm {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyEnter:
				name := m.addForm.Name()
				url := m.addForm.URL()
				m.showAddForm = false
				if name != "" && url != "" && m.profilesMgr != nil {
					if err := m.profilesMgr.Add(profiles.Profile{Name: name, URL: url}); err != nil {
						m.flash = "add: " + err.Error()
					} else {
						m.profilesTab.SetProfiles(m.profilesMgr.All(), m.profilesMgr.Active())
						m.flash = "added " + name
					}
				}
				return m, nil
			case tea.KeyEsc:
				m.showAddForm = false
				return m, nil
			}
			var c tea.Cmd
			m.addForm, c = m.addForm.Update(msg)
			return m, c
		}
	}

	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		return m, nil
	case tea.KeyMsg:
		// Proxies-tab-specific keys.
		if m.activeTab == TabProxies {
			switch v.String() {
			case "up", "k":
				m.proxiesTab.MoveUp()
				return m, nil
			case "down", "j":
				m.proxiesTab.MoveDown()
				return m, nil
			case "enter":
				m.proxiesTab.ToggleExpand()
				return m, nil
			case "t":
				grp := m.proxiesTab.SelectedGroup()
				if grp != "" && m.apiClient != nil {
					return m, tabproxies.DelayCmd(m.apiClient, grp)
				}
				return m, nil
			}
		}
		// Profiles-tab-specific keys (only when not showing form).
		if m.activeTab == TabProfiles && !m.showAddForm {
			switch v.String() {
			case "a":
				m.addForm = tabprofiles.NewForm()
				m.showAddForm = true
				return m, nil
			case "u":
				sel := m.profilesTab.Selected()
				if sel.Name != "" && m.profilesMgr != nil {
					mgr := m.profilesMgr
					pt := &m.profilesTab
					client := m.apiClient
					cmd = func() tea.Msg {
						n, err := mgr.Update(context.Background(), sel.Name)
						if err != nil {
							return ProfileError{Name: sel.Name, Err: err}
						}
						pt.SetProfiles(mgr.All(), mgr.Active())
						if client != nil {
							_ = client.ReloadConfig(context.Background(), "")
						}
						return ProfileUpdated{Name: sel.Name, NodeCount: n}
					}
					return m, cmd
				}
				return m, nil
			case "d":
				sel := m.profilesTab.Selected()
				if sel.Name != "" && m.profilesMgr != nil {
					m.profilesMgr.Remove(sel.Name)
					m.profilesTab.SetProfiles(m.profilesMgr.All(), m.profilesMgr.Active())
					m.flash = "removed " + sel.Name
				}
				return m, nil
			case "enter":
				sel := m.profilesTab.Selected()
				if sel.Name != "" && m.profilesMgr != nil {
					m.profilesMgr.SetActive(sel.Name)
					m.profilesTab.SetProfiles(m.profilesMgr.All(), m.profilesMgr.Active())
					m.flash = "active = " + sel.Name
				}
				return m, nil
			case "up", "k":
				m.profilesTab.MoveUp()
				return m, nil
			case "down", "j":
				m.profilesTab.MoveDown()
				return m, nil
			}
		}
		// Global key cascade.
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
	case ProxiesSnapshot, DelayResults:
		m.proxiesTab, cmd = m.proxiesTab.Update(msg)
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
	case ProfileUpdated:
		m.flash = fmt.Sprintf("%s: %d nodes", v.Name, v.NodeCount)
	case ProfileError:
		if v.Err != nil {
			m.flash = v.Name + ": " + v.Err.Error()
		}
	}
	return m, cmd
}
