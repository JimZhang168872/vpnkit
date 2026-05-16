package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sidebarWidth is wide enough to fit "▶ [N] EMOJI Settings" without wrapping.
// Each emoji takes 2 terminal columns, plus the bracket prefix, space, label,
// active-marker space, and the right-border character.
const sidebarWidth = 24

var (
	activeStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	inactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func renderSidebar(active Tab, height int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("vpnkit"))
	b.WriteString("\n\n")
	for i := Tab(0); i < NumTabs; i++ {
		line := fmt.Sprintf("[%d] %s", int(i)+1, TabNames[i])
		if i == active {
			b.WriteString(activeStyle.Render("▶ " + line))
		} else {
			b.WriteString(inactiveStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Width(sidebarWidth).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).Render(b.String())
}
