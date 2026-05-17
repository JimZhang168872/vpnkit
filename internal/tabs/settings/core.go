package settings

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/installer"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

type coreModel struct {
	paths paths.XDG
	store *store.Store
	flash string
}

func newCore(p paths.XDG, s *store.Store) coreModel { return coreModel{paths: p, store: s} }

func (m coreModel) Update(message tea.Msg) (coreModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "u" {
		res, err := installer.Install(installer.Options{
			Dst: m.paths.MihomoBinary(),
		}, nil)
		if err != nil {
			m.flash = "upgrade: " + err.Error()
		} else {
			m.flash = "upgraded to " + res.Version
		}
	}
	return m, nil
}

func (m coreModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Mihomo Core")
	size := "(not installed)"
	if info, err := os.Stat(m.paths.MihomoBinary()); err == nil {
		size = fmt.Sprintf("%d bytes", info.Size())
	}
	body := header + "\n\n" +
		fmt.Sprintf("  Binary : %s\n", m.paths.MihomoBinary()) +
		fmt.Sprintf("  Size   : %s\n", size) +
		"\n  [u] upgrade to latest release\n"
	if m.flash != "" {
		body += "\n  → " + m.flash + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
