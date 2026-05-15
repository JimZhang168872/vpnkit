package settings

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/store"
)

type controllerModel struct {
	store *store.Store
}

func newController(s *store.Store) controllerModel { return controllerModel{store: s} }

func (m controllerModel) Update(message tea.Msg) (controllerModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok || m.store == nil {
		return m, nil
	}
	if km.String() == "r" {
		buf := make([]byte, 16)
		_, _ = rand.Read(buf)
		m.store.Cfg.ControllerSecret = hex.EncodeToString(buf)
		_ = m.store.Save()
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
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func mask(s string) string {
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "…" + s[len(s)-3:]
}
