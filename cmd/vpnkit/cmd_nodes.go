package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"vpnkit/internal/api"
)

func runNodes(out io.Writer, c *api.Client, group string, jsonOut bool) error {
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

	type node struct {
		Name  string `json:"name"`
		Delay *int   `json:"delay"`
	}
	out2 := make([]node, 0, len(g.All))
	for _, name := range g.All {
		n := node{Name: name}
		info, ok := proxies[name]
		if ok && len(info.History) > 0 {
			d := info.History[len(info.History)-1].Delay
			n.Delay = &d
		}
		out2 = append(out2, n)
	}

	if jsonOut {
		return writeJSON(out, map[string]any{
			"group":   group,
			"current": g.Now,
			"nodes":   out2,
		})
	}

	for _, n := range out2 {
		marker := "  "
		if n.Name == g.Now {
			marker = "✓ "
		}
		delayStr := "(no test)"
		if n.Delay != nil {
			delayStr = fmt.Sprintf("%d ms", *n.Delay)
		}
		fmt.Fprintf(out, "%s%-20s  %s\n", marker, n.Name, delayStr)
	}
	return nil
}
