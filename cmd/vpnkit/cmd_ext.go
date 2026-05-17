package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"vpnkit/internal/app"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

type runExtApplyDeps struct {
	ExtensionsPath string
	// TODO(v1-phase7): ActiveProfile removed; Reassemble now calls Pipeline.Assemble directly.
	Reassemble func() error // typically pl.Assemble()
	Reload     func() error // typically client.ReloadConfig(ctx, "")
}

func runExtApply(out io.Writer, d runExtApplyDeps) error {
	// TODO(v1-phase7): ActiveProfile check removed — extensions apply to the full assembled config.
	if err := d.Reassemble(); err != nil {
		return fmt.Errorf("reassemble: %w", err)
	}
	if err := d.Reload(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}
	fmt.Fprintln(out, "applied: config reassembled with extensions and mihomo reloaded")
	return nil
}

func dispatchExt(args []string) {
	if len(args) < 1 || args[0] != "apply" {
		dieUserErr("vpnkit ext: usage: vpnkit ext apply")
	}
	p := paths.Resolve()
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit ext apply: %v", err)
	}
	pl := app.NewPipeline(st, p.MihomoConfigFile(), extensionsPath())

	client, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit ext apply: mihomo not reachable: %v", err)
	}

	deps := runExtApplyDeps{
		ExtensionsPath: extensionsPath(),
		Reassemble: func() error {
			return pl.Assemble()
		},
		Reload: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return client.ReloadConfig(ctx, "")
		},
	}
	if err := runExtApply(os.Stdout, deps); err != nil {
		dieRuntime("vpnkit ext apply: %v", err)
	}
}
