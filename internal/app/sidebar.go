package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/tabs/viewport"
)

// sidebarWidth is wide enough to fit "▶ [N] EMOJI Settings" without wrapping.
// Each emoji takes 2 terminal columns, plus the bracket prefix, space, label,
// active-marker space, and the right-border character.
const sidebarWidth = 24

var (
	activeStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	inactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func renderSidebar(active Tab, height int, focused bool) string {
	var b strings.Builder
	// Focus dot at top-left — consistent with every other panel.
	b.WriteString(viewport.FocusDot(focused))
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("vpnkit"))
	b.WriteString("\n\n")
	activeColor := lipgloss.Color("240")
	if focused {
		activeColor = lipgloss.Color("212")
	}
	active212 := lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	for i := Tab(0); i < NumTabs; i++ {
		line := fmt.Sprintf("[%d] %s", int(i)+1, TabNames[i])
		if i == active {
			b.WriteString(active212.Render("▶ " + line))
		} else {
			b.WriteString(inactiveStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	// Hint shows only the keys that actually do something here. ← has no
	// leftward target on the MainSidebar so it's omitted; → enters the
	// tab body (only meaningful when this sidebar is focused).
	var hint string
	if focused {
		hint = lipgloss.NewStyle().Faint(true).Render("\n[↑↓] navigate\n[→] enter body")
	} else {
		hint = lipgloss.NewStyle().Faint(true).Render("\n[←] focus menu")
	}
	b.WriteString(hint)
	return lipgloss.NewStyle().Width(sidebarWidth).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).Render(b.String())
}
