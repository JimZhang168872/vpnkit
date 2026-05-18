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
	busy  bool // true while async installer.Install is in flight
}

func newCore(p paths.XDG, s *store.Store) coreModel { return coreModel{paths: p, store: s} }

// coreUpgradeDoneMsg signals the end of an async installer.Install call.
type coreUpgradeDoneMsg struct {
	version string
	err     error
}

func (m coreModel) Update(message tea.Msg) (coreModel, tea.Cmd) {
	if done, ok := message.(coreUpgradeDoneMsg); ok {
		m.busy = false
		if done.err != nil {
			m.flash = "upgrade: " + done.err.Error()
		} else {
			m.flash = "upgraded to " + done.version
		}
		return m, nil
	}
	km, ok := message.(tea.KeyMsg)
	if !ok || m.busy {
		return m, nil
	}
	if km.String() == "u" {
		m.busy = true
		dst := m.paths.MihomoBinary()
		// installer.Install downloads from GitHub (~5min worst case).
		// Run it on a goroutine via tea.Cmd so the bubbletea event loop
		// stays responsive — pressing this key used to freeze the entire
		// TUI for the duration of the download.
		return m, func() tea.Msg {
			res, err := installer.Install(installer.Options{Dst: dst}, nil)
			if err != nil {
				return coreUpgradeDoneMsg{err: err}
			}
			return coreUpgradeDoneMsg{version: res.Version}
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
	if m.busy {
		body += "\n  ⏳ downloading…\n"
	}
	if m.flash != "" {
		body += "\n  → " + m.flash + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
