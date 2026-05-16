package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"vpnkit/internal/api"
)

var allowedModes = map[string]bool{"rule": true, "global": true, "direct": true}

// runMode shows or sets mihomo's mode.
//   args == []           → show
//   args == ["rule"]     → set
func runMode(out io.Writer, c *api.Client, args []string, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if len(args) == 0 {
		cfg, err := c.GetConfigs(ctx)
		if err != nil {
			return fmt.Errorf("get configs: %w", err)
		}
		if jsonOut {
			return writeJSON(out, map[string]any{"mode": cfg.Mode})
		}
		fmt.Fprintln(out, cfg.Mode)
		return nil
	}

	target := args[0]
	if !allowedModes[target] {
		return fmt.Errorf("invalid mode %q (allowed: rule, global, direct)", target)
	}
	cfg, err := c.GetConfigs(ctx)
	if err != nil {
		return fmt.Errorf("get configs: %w", err)
	}
	if cfg.Mode == target {
		if jsonOut {
			return writeJSON(out, map[string]any{"from": target, "to": target})
		}
		fmt.Fprintf(out, "mode: %s (no change)\n", target)
		return nil
	}
	if err := c.SetMode(ctx, target); err != nil {
		return fmt.Errorf("set mode: %w", err)
	}
	if jsonOut {
		return writeJSON(out, map[string]any{"from": cfg.Mode, "to": target})
	}
	fmt.Fprintf(out, "mode: %s → %s\n", cfg.Mode, target)
	return nil
}
