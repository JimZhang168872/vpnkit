package settings

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/store"
)

type controllerModel struct {
	store *store.Store
	pl    PipelineFace
	flash string
}

func newController(s *store.Store, pl PipelineFace) controllerModel {
	return controllerModel{store: s, pl: pl}
}

func (m controllerModel) Update(message tea.Msg) (controllerModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok || m.store == nil {
		return m, nil
	}
	if km.String() == "r" && m.pl != nil {
		// Route through Pipeline so the rand-read + secret mutation +
		// save is serialized under p.mu. Direct store.Cfg writes from
		// here race with any concurrent Pipeline read (Assemble, etc.).
		if err := m.pl.RegenerateControllerSecret(); err != nil {
			m.flash = "❌ regenerate: " + err.Error()
			return m, nil
		}
		m.flash = "✅ secret rotated (mihomo restart required)"
	}
	return m, nil
}

func (m controllerModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("External Controller")
	port := 9090
	secret := ""
	if m.store != nil {
		port = m.store.Cfg.ControllerPort
		secret = mask(m.store.Cfg.ControllerSecret)
	}
	body := header + "\n\n" +
		fmt.Sprintf("  Port   : 127.0.0.1:%d\n", port) +
		fmt.Sprintf("  Secret : %s\n", secret) +
		"\n  [r] regenerate secret (will require restart to take effect)\n"
	if m.flash != "" {
		body += "\n  " + lipgloss.NewStyle().Faint(true).Render(m.flash) + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func mask(s string) string {
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "…" + s[len(s)-3:]
}
