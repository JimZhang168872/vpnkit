package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 16

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
