package app

import "github.com/charmbracelet/lipgloss"

// View composes sidebar + tab body + statusbar.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	bodyHeight := m.height - 1 // reserve a line for status bar
	sidebar := renderSidebar(m.activeTab, bodyHeight, m.appFocus == FocusMainSidebar)
	bodyWidth := m.width - sidebarWidth

	var body string
	switch m.activeTab {
	case TabDashboard:
		body = m.dashboard.View(bodyWidth, bodyHeight)
	case TabProxies:
		body = m.proxiesTab.View(bodyWidth, bodyHeight)
	case TabProfiles:
		body = m.profilesTab.View(bodyWidth, bodyHeight)
	case TabConnections:
		body = m.connectionsTab.View(bodyWidth, bodyHeight)
	case TabRules:
		body = m.rulesTab.View(bodyWidth, bodyHeight)
	case TabSettings:
		body = m.settingsTab.View(bodyWidth, bodyHeight)
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
