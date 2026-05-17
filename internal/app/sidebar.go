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

func renderSidebar(active Tab, height int, focused bool) string {
	var b strings.Builder
	// Focus dot ●/○ — at-a-glance signal of which panel ↑/↓ will affect.
	if focused {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("● "))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○ "))
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("vpnkit"))
	b.WriteString("\n\n")
	// Cursor color: bright 212 when sidebar has focus, dim 240 otherwise.
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
	hint := lipgloss.NewStyle().Faint(true).Render("\n[← →] focus\n[↑↓] navigate")
	b.WriteString(hint)
	return lipgloss.NewStyle().Width(sidebarWidth).Height(height).
		BorderRight(true).BorderStyle(lipgloss.NormalBorder()).Render(b.String())
}
