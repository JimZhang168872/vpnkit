package app

import "github.com/charmbracelet/lipgloss"

// View composes sidebar + tab body + statusbar.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	bodyHeight := m.height - 1 // reserve a line for status bar
	mainFocused := m.appFocus == FocusMainSidebar
	sidebar := renderSidebar(m.activeTab, bodyHeight, mainFocused)
	bodyWidth := m.width - sidebarWidth
	bodyFocused := !mainFocused // i.e., appFocus == TabBody

	var body string
	switch m.activeTab {
	case TabDashboard:
		body = m.dashboard.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabProxies:
		body = m.proxiesTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabProfiles:
		body = m.profilesTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabConnections:
		body = m.connectionsTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabRules:
		body = m.rulesTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabSettings:
		body = m.settingsTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	default:
		body = m.stubs[m.activeTab].View(bodyWidth, bodyHeight)
	}

	rows := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)

	if m.showAddForm {
		formView := m.addForm.View()
		body = lipgloss.Place(bodyWidth, bodyHeight, lipgloss.Center, lipgloss.Center, formView)
		rows = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
	}

	return rows + "\n" + m.renderStatusBar(m.width)
}
