package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"vpnkit/internal/api"
)

func runUse(out io.Writer, c *api.Client, group, node string, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return fmt.Errorf("get proxies: %w", err)
	}
	g, ok := proxies[group]
	if !ok || !isUserSelectableType(g.Type) || len(g.All) == 0 {
		return fmt.Errorf("group %q not found (try 'vpnkit groups')", group)
	}
	found := false
	for _, m := range g.All {
		if m == node {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("node %q not in group %q", node, group)
	}
	if err := c.PutProxy(ctx, group, node); err != nil {
		return fmt.Errorf("set proxy: %w", err)
	}

	if jsonOut {
		return writeJSON(out, map[string]any{"group": group, "now": node})
	}
	fmt.Fprintf(out, "✓ %s → %s\n", group, node)
	return nil
}
