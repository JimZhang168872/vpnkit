package app

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	apiPkg "vpnkit/internal/api"
	tabgroups "vpnkit/internal/tabs/groups"
	tabrules "vpnkit/internal/tabs/rules"
	tabsettings "vpnkit/internal/tabs/settings"
	tabsources "vpnkit/internal/tabs/sources"
)

// groupSwitchMsg is the result of PutProxy launched from Groups tab Enter.
type groupSwitchMsg struct {
	group string
	node  string
	err   error
}

// delayErrMsg surfaces failures from the Groups tab `t` (delay test) cmd.
// Success comes back as DelayResults — only errors take this path.
type delayErrMsg struct {
	group string
	err   error
}

// pipelineAppliedMsg reports the outcome of the async applyCfg fired after
// every Sources tab mutation. Drives the success/error flash.
type pipelineAppliedMsg struct {
	err error
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

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
		// is open (Sources form, Connections/Rules filter) so typing in
		// those fields isn't hijacked.
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
		// Logs-tab-specific keys.
		if m.activeTab == TabLogs {
			if v.String() == "p" {
				// `[p] pause/resume` — pre-rc.7 the hint was rendered
				// but the key was never wired. TogglePause has a
				// pointer receiver; take the address on this Model's
				// field so the toggle persists in the returned m.
				(&m.logsTab).TogglePause()
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
			// Tab is OWNED by the global tab cycler — do NOT intercept it
			// here. Previously `case "tab"` consumed Tab to toggle the
			// Rules sub-page (Live ↔ Local), which permanently trapped
			// the user on the Rules tab. The sub-page now uses `T`
			// (shift+t) instead.
			case "T":
				var c tea.Cmd
				m.rulesTab, c = m.rulesTab.Update(msg)
				return m, c
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
			case "d", "K", "J":
				// Local Rules CRUD — forward to rulesTab.
				var c tea.Cmd
				m.rulesTab, c = m.rulesTab.Update(msg)
				return m, c
			}
		}
		// Groups-tab-specific keys.
		if m.activeTab == TabGroups {
			switch v.String() {
			case "up", "k":
				m.groupsTab.MoveUp()
				m.groupsTab.Refresh()
				return m, nil
			case "down", "j":
				m.groupsTab.MoveDown()
				m.groupsTab.Refresh()
				return m, nil
			case "r":
				m.groupsTab.Refresh()
				return m, nil
			case "t":
				// Trigger a group-wide delay test against the highlighted
				// group. Uses MeasureGroup which handles the Selector vs
				// url-test split (vpnkit Selectors return 404 on direct
				// /group/<name>/delay; MeasureGroup retries with the
				// "<group>-auto" companion and finally falls back to
				// per-member /proxies/<member>/delay).
				//
				// mihomo already returns namespaced node names ("doge:HK-A"),
				// matching what groups.View() looks up in delayByNode — no
				// re-namespacing here.
				//
				// Self-heal: if the first probe returns a transport-layer
				// failure (mihomo not running / crashed), call applyCfg
				// which reassembles config + restarts the service via
				// service.Manager, wait briefly, retry once. This rescues
				// the common case where mihomo died between the user adding
				// a node and pressing `t` — they shouldn't have to know to
				// go to Settings → Service [r].
				group := m.groupsTab.SelectedGroup()
				if group == "" || m.apiClient == nil {
					return m, nil
				}
				m.flash = "⏱  testing " + group + "…"
				client := m.apiClient
				applyCfg := m.applyCfg
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
					defer cancel()
					results, err := client.MeasureGroup(ctx, group, "https://www.gstatic.com/generate_204", 5000)
					if err != nil && apiPkg.IsUnreachable(err) && applyCfg != nil {
						if applyErr := applyCfg(ctx); applyErr == nil {
							time.Sleep(2 * time.Second)
							results, err = client.MeasureGroup(ctx, group, "https://www.gstatic.com/generate_204", 5000)
						}
					}
					if err != nil {
						return delayErrMsg{group: group, err: err}
					}
					return DelayResults{Group: group, Results: results}
				}
			case "enter":
				// Switch the selected group's `now` to the highlighted node
				// (only meaningful with right-pane focus, but allow from left
				// too — it'll pick rightCursor=0). Calls mihomo controller
				// PutProxy; flash reports outcome.
				if m.groupsTab.SubFocus() != tabgroups.SubFocusRight {
					m.flash = "press → to focus nodes, then Enter to switch"
					return m, nil
				}
				group := m.groupsTab.SelectedGroup()
				node := m.groupsTab.SelectedNode()
				if group == "" || node == "" || m.apiClient == nil {
					return m, nil
				}
				client := m.apiClient
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := client.PutProxy(ctx, group, node); err != nil {
						return groupSwitchMsg{group: group, node: node, err: err}
					}
					return groupSwitchMsg{group: group, node: node}
				}
			}
		}
		// Sources-tab-specific keys: forward to sourcesTab unless global key.
		// When a text-input overlay is open, swallow EVERYTHING (including
		// digits and Tab) so URL/UA fields can be typed without the global
		// 1-7 / Tab tab-switcher hijacking the input.
		if m.activeTab == TabSources {
			if m.sourcesTab.InputOpen() {
				// Ctrl-C must always be a quit path — even inside a
				// textinput. Without this carve-out the form swallows it
				// and the user has no fast escape to abort the app.
				if v.String() == "ctrl+c" {
					return m, tea.Quit
				}
				var c tea.Cmd
				m.sourcesTab, c = m.sourcesTab.Update(msg)
				return m, c
			}
			if v.String() == "1" || v.String() == "2" || v.String() == "3" ||
				v.String() == "4" || v.String() == "5" || v.String() == "6" ||
				v.String() == "7" ||
				v.String() == "tab" || v.String() == "shift+tab" || v.String() == "q" || v.String() == "ctrl+c" {
				// Fall through to global cascade.
			} else {
				var c tea.Cmd
				m.sourcesTab, c = m.sourcesTab.Update(msg)
				return m, c
			}
		}
		// Settings-tab-specific keys: same rule — input overlay swallows all
		// EXCEPT ctrl+c (which must remain a quit escape hatch).
		if m.activeTab == TabSettings {
			if m.settingsTab.InputOpen() {
				if v.String() == "ctrl+c" {
					return m, tea.Quit
				}
				var c tea.Cmd
				m.settingsTab, c = m.settingsTab.Update(msg)
				return m, c
			}
			if v.String() == "1" || v.String() == "2" || v.String() == "3" ||
				v.String() == "4" || v.String() == "5" || v.String() == "6" ||
				v.String() == "7" ||
				v.String() == "tab" || v.String() == "shift+tab" || v.String() == "q" || v.String() == "ctrl+c" {
				// Fall through to global cascade.
			} else {
				// Up/Down within Settings = sub-page navigation. Clear
				// stale flash so help/mode hints set on one sub-page
				// don't haunt the next one. Keys that intentionally set
				// a new flash (handled inside sub-page Update) write to
				// m.flash AFTER this clear, so the new flash wins.
				if v.String() == "up" || v.String() == "down" || v.String() == "k" || v.String() == "j" {
					m.flash = ""
				}
				var c tea.Cmd
				m.settingsTab, c = m.settingsTab.Update(msg)
				return m, c
			}
		}
		// Global key cascade. Tab changes clear the statusbar flash —
		// pre-rc.7 a stale flash from one tab persisted forever across
		// navigation (e.g. press `?` on Dashboard, switch tabs, the
		// keymap text would haunt every other tab's statusbar).
		prevTab := m.activeTab
		switch {
		case key.Matches(v, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(v, m.keys.Tab1):
			m.activeTab = TabDashboard
		case key.Matches(v, m.keys.Tab2):
			m.activeTab = TabGroups
		case key.Matches(v, m.keys.Tab3):
			m.activeTab = TabSources
		case key.Matches(v, m.keys.Tab4):
			m.activeTab = TabRules
		case key.Matches(v, m.keys.Tab5):
			m.activeTab = TabConnections
		case key.Matches(v, m.keys.Tab6):
			m.activeTab = TabLogs
		case key.Matches(v, m.keys.Tab7):
			m.activeTab = TabSettings
		case key.Matches(v, m.keys.NextTab):
			m.activeTab = (m.activeTab + 1) % NumTabs
		case key.Matches(v, m.keys.PrevTab):
			m.activeTab = (m.activeTab + NumTabs - 1) % NumTabs
		case key.Matches(v, m.keys.Help):
			// Statusbar is single-line; the verbose hint was truncated
			// at 60 cols. Use a terse mnemonic instead; full help is
			// in docs/USAGE.md.
			m.flash = "Keys: 1-7 tab • /filter • a add • t test • q quit"
		case key.Matches(v, m.keys.Mode):
			// Direct users to the canonical UI for mode changes.
			m.flash = "Cycle mode in Settings → Routing or `vpnkit mode [rule|global|direct]`"
		case key.Matches(v, m.keys.Restart):
			// Same: direct to Settings → Service rather than firing a
			// blocking systemctl call from the global handler.
			m.flash = "Restart mihomo in Settings → Service (press r there) or `systemctl --user restart mihomo`"
		case key.Matches(v, m.keys.Palette):
			m.flash = "Command palette: not implemented yet"
		}
		// Clear the stale flash on any successful tab change. The
		// global keymap handlers above (?, m, r, :) DELIBERATELY set
		// m.flash AFTER this conditional fires (they don't change
		// activeTab), so this only clears on genuine navigation. Keeps
		// help / mode hints visible while preventing them from
		// haunting every other tab the user visits next.
		if m.activeTab != prevTab {
			m.flash = ""
		}
	case TrafficMsg, VersionMsg, ServiceStatusMsg:
		m.dashboard, cmd = m.dashboard.Update(msg)
		// Settings → Service sub-page renders live status from the poll
		// loop too, so forward the same snapshot. Without this, the
		// settings tab would have to call mgr.Status synchronously on
		// every View (blocking systemctl call on the render goroutine).
		if _, ok := msg.(ServiceStatusMsg); ok {
			m.settingsTab, _ = m.settingsTab.Update(msg)
		}
	case ProxiesSnapshot:
		// Forward to groupsTab so it can mirror each group's `now` for the
		// right-pane node highlight.
		m.groupsTab, _ = m.groupsTab.Update(v)
	case DelayResults:
		// Forward to groupsTab so per-node delay overlays render. Also
		// clear the "⏱  testing …" flash that the `t` handler set.
		m.groupsTab, _ = m.groupsTab.Update(v)
		m.flash = fmt.Sprintf("✓ %s: tested %d nodes", v.Group, len(v.Results))
		return m, nil
	case delayErrMsg:
		if apiPkg.IsUnreachable(v.err) {
			m.flash = "❌ mihomo unreachable — try Settings → Service [r] restart, or `journalctl --user -u mihomo` to inspect"
		} else {
			m.flash = "❌ delay " + v.group + ": " + v.err.Error()
		}
		return m, nil
	case groupSwitchMsg:
		if v.err != nil {
			m.flash = "❌ " + v.group + " → " + v.node + ": " + v.err.Error()
		} else {
			m.flash = "✓ " + v.group + " → " + v.node
		}
		return m, nil
	case ConnectionsSnapshot:
		m.connectionsTab, cmd = m.connectionsTab.Update(msg)
	case RulesSnapshot:
		m.rulesTab, cmd = m.rulesTab.Update(msg)
	case LogLine:
		m.logsTab, _ = m.logsTab.Update(msg)
	case tabsettings.RoutingApplyDoneMsg, tabsettings.ActiveApplyDoneMsg:
		// Async applyFunc completion messages from Settings → Routing /
		// Active Source. Must reach settingsTab.Update so the relevant
		// sub-page can clear its `busy` flag — without explicit routing
		// the message dies in the top-level switch and the user sees
		// "⏳ reloading mihomo…" forever. Same dispatch-hole class of
		// bug as tabsources.RefreshDoneMsg below.
		m.settingsTab, cmd = m.settingsTab.Update(msg)
		return m, cmd
	case tabsources.RefreshDoneMsg, tabsources.RefreshErrMsg:
		// Subscription-refresh tea.Cmds (fired by Sources tab's `u`) come
		// back as these messages. They need to land on sourcesTab so its
		// internal handler updates the flash, reloads the on-display list,
		// and — critically — emits PipelineMutatedMsg downstream. Without
		// this explicit forwarding the message dies in the top-level
		// switch's default fallthrough and config.yaml stays stale,
		// leading to "delay <name>: group not found in /proxies" when the
		// user follows up with a delay test. See model_test.go's
		// TestSubsRefreshDoneTriggersApplyCfg.
		m.sourcesTab, cmd = m.sourcesTab.Update(msg)
		return m, cmd
	case tabsources.PipelineMutatedMsg:
		// Sources mutations (subs / local-node / local-group CRUD) change
		// the assembled mihomo config — must rewrite config.yaml AND
		// reload (or restart) the running mihomo, else the controller has
		// no idea about the new group / node, and follow-up actions like
		// delay test or `Enter` to switch will fail with 404. The flash
		// reports the apply outcome so the user can tell things landed.
		m.groupsTab.Refresh()
		if m.applyCfg == nil {
			return m, nil
		}
		applyCfg := m.applyCfg
		m.flash = "⏳ reloading mihomo…"
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := applyCfg(ctx)
			return pipelineAppliedMsg{err: err}
		}
	case pipelineAppliedMsg:
		if v.err != nil {
			m.flash = "❌ apply: " + v.err.Error()
		} else {
			m.flash = "✓ mihomo reloaded"
		}
		return m, nil
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
