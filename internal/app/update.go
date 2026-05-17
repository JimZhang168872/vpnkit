package app

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	tabprofiles "vpnkit/internal/tabs/profiles"
	tabproxies "vpnkit/internal/tabs/proxies"
	tabrules "vpnkit/internal/tabs/rules"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// When the add-form overlay is open, route all key input to it (except Enter/Esc).
	// TODO(v1-phase8): add-form will be replaced by Sources sub-page in TUI restructure.
	if m.showAddForm {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyEnter:
				m.showAddForm = false
				m.flash = "⚠️  Profiles add not available in v1 — use `vpnkit subs add` CLI"
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
		// Bug N — global focus handler. ←/→ shift focus across the levels
		// (MainSidebar ↔ Settings sub-sidebar ↔ Settings content), and ↑/↓
		// on MainSidebar cycles top tabs. These intercept BEFORE tab-
		// specific handlers so list cursors no longer eat the key when
		// the user is operating the sidebar.
		//
		// Skip the global handler entirely when a textinput-style overlay
		// is open (Profiles add-form, Connections/Rules filter) so typing
		// in those fields isn't hijacked.
		if !m.inputOpen() {
			switch v.String() {
			case "left", "h":
				m = m.shiftFocusLeft()
				return m, nil
			case "right", "l":
				m = m.shiftFocusRight()
				return m, nil
			case "up", "k":
				if m.appFocus == FocusMainSidebar {
					if m.activeTab > 0 {
						m.activeTab--
					}
					return m, nil
				}
				// else: fall through to tab-specific handlers below
			case "down", "j":
				if m.appFocus == FocusMainSidebar {
					if m.activeTab < NumTabs-1 {
						m.activeTab++
					}
					return m, nil
				}
				// else: fall through
			}
		}
		// Connections-tab-specific keys.
		if m.activeTab == TabConnections {
			if m.connectionsTab.IsFiltering() {
				var c tea.Cmd
				m.connectionsTab, c = m.connectionsTab.Update(msg)
				return m, c
			}
			switch v.String() {
			case "/":
				cmd := m.connectionsTab.StartFilter()
				return m, cmd
			case "up", "k":
				m.connectionsTab.MoveUp()
				return m, nil
			case "down", "j":
				m.connectionsTab.MoveDown()
				return m, nil
			case "pgup":
				m.connectionsTab.MovePageUp()
				return m, nil
			case "pgdown":
				m.connectionsTab.MovePageDown()
				return m, nil
			case "x":
				if id := m.connectionsTab.SelectedID(); id != "" && m.apiClient != nil {
					client := m.apiClient
					return m, func() tea.Msg {
						_ = client.CloseConnection(context.Background(), id)
						return nil
					}
				}
				return m, nil
			}
		}
		// Rules-tab-specific keys.
		if m.activeTab == TabRules {
			if m.rulesTab.IsFiltering() {
				var c tea.Cmd
				m.rulesTab, c = m.rulesTab.Update(msg)
				return m, c
			}
			switch v.String() {
			case "/":
				cmd := m.rulesTab.StartFilter()
				return m, cmd
			case "u":
				if m.apiClient != nil {
					return m, tabrules.RefreshAllProvidersCmd(m.apiClient, m.rulesTab.ProviderNames())
				}
				return m, nil
			case "up", "k":
				m.rulesTab.MoveUp()
				return m, nil
			case "down", "j":
				m.rulesTab.MoveDown()
				return m, nil
			case "pgup":
				m.rulesTab.MovePageUp()
				return m, nil
			case "pgdown":
				m.rulesTab.MovePageDown()
				return m, nil
			}
		}
		// Settings-tab-specific keys: forward to settingsTab unless it's a global tab/quit key.
		if m.activeTab == TabSettings {
			if v.String() == "1" || v.String() == "2" || v.String() == "3" ||
				v.String() == "4" || v.String() == "5" || v.String() == "6" ||
				v.String() == "tab" || v.String() == "shift+tab" || v.String() == "q" || v.String() == "ctrl+c" {
				// Fall through to global cascade.
			} else {
				var c tea.Cmd
				m.settingsTab, c = m.settingsTab.Update(msg)
				return m, c
			}
		}
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
				if grp, node, ok := m.proxiesTab.SelectedNode(); ok {
					if m.apiClient != nil {
						client := m.apiClient
						return m, func() tea.Msg {
							ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
							defer cancel()
							if err := client.PutProxy(ctx, grp, node); err != nil {
								return ProfileError{Name: grp + "/" + node, Err: err}
							}
							return nil
						}
					}
					return m, nil
				}
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
		// TODO(v1-phase8): this tab will be replaced by Groups/Sources in Phase 8.
		// Until then, navigation works but add/update/delete/activate are no-ops.
		if m.activeTab == TabProfiles && !m.showAddForm {
			switch v.String() {
			case "a":
				m.addForm = tabprofiles.NewForm()
				m.showAddForm = true
				return m, nil
			case "u":
				m.flash = "⚠️  Use `vpnkit subs update <name>` CLI — TODO(v1-phase8)"
				return m, nil
			case "d":
				m.flash = "⚠️  Use `vpnkit subs rm <name>` CLI — TODO(v1-phase8)"
				return m, nil
			case "enter":
				m.flash = "⚠️  Profile activation not available in v1 — TODO(v1-phase8)"
				return m, nil
			case "up", "k":
				m.profilesTab.MoveUp()
				return m, nil
			case "down", "j":
				m.profilesTab.MoveDown()
				return m, nil
			case "pgup":
				m.profilesTab.MovePageUp()
				return m, nil
			case "pgdown":
				m.profilesTab.MovePageDown()
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
		if snap, ok := msg.(ProxiesSnapshot); ok {
			m.recordProxyNames(snap)
		}
	case ConnectionsSnapshot:
		m.connectionsTab, cmd = m.connectionsTab.Update(msg)
	case RulesSnapshot:
		m.rulesTab, cmd = m.rulesTab.Update(msg)
	case LogLine:
		lm := m.settingsTab.LogsModel()
		*lm, _ = lm.Update(msg)
	case BootstrapProgressMsg:
		switch v.Phase {
		case "ready":
			m.flash = "🟢 mihomo ready"
		case "error":
			if v.Err != nil {
				m.flash = "❌ bootstrap: " + v.Err.Error()
			}
		default:
			m.flash = "🟡 bootstrapping: " + v.Phase
		}
	case ProfileUpdated:
		m.flash = fmt.Sprintf("✅ %s: %d nodes", v.Name, v.NodeCount)
	case ProfileError:
		if v.Err != nil {
			m.flash = "❌ " + v.Name + ": " + v.Err.Error()
		}
	case UpdateAvailableMsg:
		// Build the badge string from whatever piece is upgradable. Keep it
		// short — the status bar is tight.
		switch {
		case v.Info.VpnkitNeedsUpdate && v.Info.MihomoNeedsUpdate:
			m.updateBadge = "⚡ vpnkit " + v.Info.VpnkitLatest + " + mihomo " + v.Info.MihomoLatest
		case v.Info.VpnkitNeedsUpdate:
			m.updateBadge = "⚡ vpnkit " + v.Info.VpnkitLatest + " (run `vpnkit update`)"
		case v.Info.MihomoNeedsUpdate:
			m.updateBadge = "⚡ mihomo " + v.Info.MihomoLatest + " (run `vpnkit update`)"
		}
	}
	return m, cmd
}
