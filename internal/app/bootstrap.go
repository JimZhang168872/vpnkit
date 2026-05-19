package app

import (
	"bytes"
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/installer"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
)

// BootstrapDeps are injectable to ease testing later.
type BootstrapDeps struct {
	Paths   paths.XDG
	Store   *store.Store
	Service service.Manager
	// InstallFunc lets tests stub installer.Install.
	InstallFunc func(opts installer.Options, prog installer.ProgressFunc) (installer.Result, error)
}

// MaybeBootstrap returns a tea.Cmd that performs first-run setup only if needed.
// It emits a single BootstrapProgressMsg with Phase=ready or error. Detailed
// per-step output is captured into the error's Note for surfacing in the
// Service tab.
func MaybeBootstrap(d BootstrapDeps) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		if err := RunBootstrap(context.Background(), d, io.MultiWriter(&buf)); err != nil {
			return BootstrapProgressMsg{
				Phase: "error",
				Note:  buf.String(),
				Err:   err,
			}
		}
		return BootstrapProgressMsg{Phase: "ready", Note: buf.String()}
	}
}
