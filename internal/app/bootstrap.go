package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/config"
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
// It emits BootstrapProgressMsg at each phase; the top-level Model can render them.
func MaybeBootstrap(d BootstrapDeps) tea.Cmd {
	return func() tea.Msg {
		// 1. Ensure XDG dirs exist.
		if err := d.Paths.Ensure(); err != nil {
			return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("paths: %w", err)}
		}
		// 2. Install mihomo if missing.
		if _, err := os.Stat(d.Paths.MihomoBinary()); errors.Is(err, fs.ErrNotExist) {
			if d.InstallFunc == nil {
				d.InstallFunc = installer.Install
			}
			_, err := d.InstallFunc(installer.Options{
				Dst:     d.Paths.MihomoBinary(),
				Mirror:  d.Store.Cfg.ReleaseMirror,
				APIBase: "",
			}, nil)
			if err != nil {
				return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("install: %w", err)}
			}
		}
		// 3. Generate config.yaml if missing.
		if _, err := os.Stat(d.Paths.MihomoConfigFile()); errors.Is(err, fs.ErrNotExist) {
			data, err := config.BuildSkeleton(config.SkeletonInput{
				MixedPort:        7890,
				ControllerPort:   d.Store.Cfg.ControllerPort,
				ControllerSecret: d.Store.Cfg.ControllerSecret,
				RuleTemplate:     d.Store.Cfg.RuleTemplate,
			})
			if err != nil {
				return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("config: %w", err)}
			}
			if err := config.AtomicWrite(d.Paths.MihomoConfigFile(), data, 0o600); err != nil {
				return BootstrapProgressMsg{Phase: "error", Err: err}
			}
		}
		// 4. Install + start the service.
		ctx := context.Background()
		if err := d.Service.Install(ctx); err != nil {
			return BootstrapProgressMsg{Phase: "error", Err: fmt.Errorf("service install: %w", err)}
		}
		// systemd Install already does enable --now; PID Install is a no-op so we Start.
		_ = d.Service.Start(ctx)
		return BootstrapProgressMsg{Phase: "ready"}
	}
}
