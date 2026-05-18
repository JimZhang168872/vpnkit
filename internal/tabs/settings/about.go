package settings

import (
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type aboutModel struct {
	version string // vpnkit ldflags-baked version, or "dev" / "" in source builds
}

func newAbout(version string) aboutModel {
	if version == "" {
		version = "dev"
	}
	return aboutModel{version: version}
}

func (m aboutModel) Update(tea.Msg) (aboutModel, tea.Cmd) { return m, nil }

func (m aboutModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("About")
	body := header + "\n\n" +
		"  vpnkit " + m.version + " — TUI for managing the mihomo proxy core (non-root).\n" +
		"\n" +
		"  Built with Go " + runtime.Version() + " · bubbletea · lipgloss.\n" +
		"  License: MIT (vpnkit) · GPL-3.0 (mihomo upstream).\n" +
		"\n" +
		"  Source : https://github.com/JimZhang168872/vpnkit\n" +
		"  Upstream: https://github.com/MetaCubeX/mihomo\n"
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
