package settings

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/service"
)

type serviceModel struct {
	mgr    service.Manager
	status service.Status
	flash  string
	busy   string // "starting" / "restarting" / "" — disables key dispatch
}

func newService(mgr service.Manager) serviceModel { return serviceModel{mgr: mgr} }

// serviceOpDoneMsg is the result of an async service operation
// (start/stop/restart/uninstall). The poll loop in app/run.go pushes
// status snapshots independently via msg.ServiceStatus; this message
// just clears the busy banner and surfaces the operation outcome.
type serviceOpDoneMsg struct {
	op  string
	err error
}

func (m serviceModel) Update(message tea.Msg) (serviceModel, tea.Cmd) {
	// Bubble live status snapshots from app/run.go's pollServiceStatus
	// poller so the view can render without ever calling mgr.Status from
	// the render path (which used to block on systemctl).
	if s, ok := message.(msg.ServiceStatus); ok {
		m.status = service.Status{
			Running: s.Running,
			PID:     s.PID,
			Mode:    service.Mode(s.Mode),
		}
		return m, nil
	}
	if done, ok := message.(serviceOpDoneMsg); ok {
		m.busy = ""
		if done.err != nil {
			m.flash = done.op + ": " + done.err.Error()
		} else {
			m.flash = done.op
		}
		return m, nil
	}
	km, ok := message.(tea.KeyMsg)
	if !ok || m.mgr == nil || m.busy != "" {
		return m, nil
	}
	mgr := m.mgr
	switch km.String() {
	case "s":
		m.busy = "starting"
		return m, runServiceOp("started", mgr.Start)
	case "S":
		m.busy = "stopping"
		return m, runServiceOp("stopped", mgr.Stop)
	case "r":
		m.busy = "restarting"
		return m, runServiceOp("restarted", mgr.Restart)
	case "u":
		m.busy = "uninstalling"
		return m, runServiceOp("uninstalled", mgr.Uninstall)
	}
	return m, nil
}

// runServiceOp wraps a blocking service-manager call in a tea.Cmd so
// systemctl invocations run on their own goroutine instead of freezing
// the bubbletea event loop. 30s timeout covers worst-case "daemon-reload
// + restart with hold-off delay" without blocking forever if dbus is
// stuck.
func runServiceOp(label string, op func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return serviceOpDoneMsg{op: label, err: op(ctx)}
	}
}

func (m serviceModel) View(width, height int) string {
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
	if m.busy != "" {
		body += "\n  ⏳ " + m.busy + "…\n"
	}
	if m.flash != "" {
		body += "\n  → " + m.flash + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
