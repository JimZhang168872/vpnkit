package settings

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/service"
)

type serviceModel struct {
	mgr    service.Manager
	status service.Status
	flash  string
}

func newService(mgr service.Manager) serviceModel { return serviceModel{mgr: mgr} }

func (m serviceModel) refresh() serviceModel {
	if m.mgr == nil {
		return m
	}
	st, _ := m.mgr.Status(context.Background())
	m.status = st
	return m
}

func (m serviceModel) Update(message tea.Msg) (serviceModel, tea.Cmd) {
	km, ok := message.(tea.KeyMsg)
	if !ok || m.mgr == nil {
		return m, nil
	}
	ctx := context.Background()
	switch km.String() {
	case "s":
		if err := m.mgr.Start(ctx); err != nil {
			m.flash = "start: " + err.Error()
		} else {
			m.flash = "started"
		}
	case "S":
		if err := m.mgr.Stop(ctx); err != nil {
			m.flash = "stop: " + err.Error()
		} else {
			m.flash = "stopped"
		}
	case "r":
		if err := m.mgr.Restart(ctx); err != nil {
			m.flash = "restart: " + err.Error()
		} else {
			m.flash = "restarted"
		}
	case "u":
		if err := m.mgr.Uninstall(ctx); err != nil {
			m.flash = "uninstall: " + err.Error()
		} else {
			m.flash = "uninstalled"
		}
	}
	m = m.refresh()
	return m, nil
}

func (m serviceModel) View(width, height int) string {
	m = m.refresh()
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Service")
	mode := "(unknown)"
	if m.mgr != nil {
		mode = string(m.mgr.Mode())
	}
	state := "○ stopped"
	if m.status.Running {
		state = fmt.Sprintf("● running (pid=%d)", m.status.PID)
	}
	body := header + "\n\n" +
		fmt.Sprintf("  Mode  : %s\n", mode) +
		fmt.Sprintf("  State : %s\n", state) +
		"\n  [s] start  [S] stop  [r] restart  [u] uninstall\n"
	if m.flash != "" {
		body += "\n  → " + m.flash + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
