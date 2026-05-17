package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"vpnkit/internal/api"
	"vpnkit/internal/app"
	"vpnkit/internal/paths"
)

// runMode shows or sets vpnkit's routing mode.
//
//	args == []         → show (reads from store)
//	args == ["rule"]   → set: writes store + reassembles + reloads mihomo
func runMode(out io.Writer, c *api.Client, args []string, jsonOut bool) error {
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		return err
	}

	if len(args) == 0 {
		if jsonOut {
			return writeJSON(out, map[string]any{"mode": st.Cfg.Mode})
		}
		fmt.Fprintln(out, st.Cfg.Mode)
		return nil
	}

	v := strings.ToLower(args[0])
	switch v {
	case "rule", "global", "direct":
	default:
		return fmt.Errorf("invalid mode %q (allowed: rule, global, direct)", v)
	}

	prev := st.Cfg.Mode
	st.Cfg.Mode = v
	if err := st.Save(); err != nil {
		return fmt.Errorf("save store: %w", err)
	}

	// Trigger a config rewrite + mihomo reload.
	pl := app.NewPipeline(st, p.MihomoConfigFile(), p.VpnkitConfig+"/extensions.toml")
	if err := pl.Assemble(); err != nil {
		return fmt.Errorf("assemble: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ReloadConfig(ctx, ""); err != nil {
		return fmt.Errorf("reload mihomo: %w", err)
	}

	if jsonOut {
		return writeJSON(out, map[string]any{"from": prev, "to": v})
	}
	fmt.Fprintf(out, "mode: %s → %s\n", prev, v)
	return nil
}

