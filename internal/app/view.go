package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View composes sidebar + tab body + statusbar.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	// Minimum usable terminal: sidebar (24 cols) + at least 36 cols of
	// content. Below this, About-page URLs wrap into ~10-col residuals
	// and sub-sidebar labels mangle ("External Controller" splits to
	// two lines with missing border). Show a clear gate message instead
	// of letting layout silently corrupt.
	if m.width < 60 {
		return fmt.Sprintf("vpnkit: terminal too narrow (need ≥60 cols, got %d)\nresize and press any key.", m.width)
	}
	if m.height < 16 {
		return fmt.Sprintf("vpnkit: terminal too short (need ≥16 rows, got %d)\nresize and press any key.", m.height)
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
	case TabGroups:
		body = m.groupsTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabSources:
		body = m.sourcesTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabRules:
		body = m.rulesTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabConnections:
		body = m.connectionsTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabLogs:
		body = m.logsTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	case TabSettings:
		body = m.settingsTab.ViewFocused(bodyWidth, bodyHeight, bodyFocused)
	default:
		body = m.stubs[m.activeTab].View(bodyWidth, bodyHeight)
	}

	rows := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
	return rows + "\n" + m.renderStatusBar(m.width)
}
