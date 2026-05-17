package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"vpnkit/internal/api"
	"vpnkit/internal/store"
)

// runStatus prints a snapshot of mihomo state. store may be nil (tests).
func runStatus(out io.Writer, c *api.Client, st *store.Store, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	v, err := c.Version(ctx)
	if err != nil {
		return fmt.Errorf("mihomo not reachable: %w", err)
	}
	cfg, err := c.GetConfigs(ctx)
	if err != nil {
		return fmt.Errorf("get configs: %w", err)
	}
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return fmt.Errorf("get proxies: %w", err)
	}

	type groupSummary struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Now     string `json:"now"`
		Members int    `json:"members"`
	}

	var groups []groupSummary
	for name, info := range proxies {
		if !isUserSelectableType(info.Type) {
			continue
		}
		if len(info.All) == 0 {
			continue
		}
		groups = append(groups, groupSummary{Name: name, Type: info.Type, Now: info.Now, Members: len(info.All)})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	var (
		subCount       int
		localNodeCount int
		storeMode      string
		globalTarget   string
	)
	if st != nil {
		subCount = len(st.Cfg.Subscriptions)
		localNodeCount = len(st.Cfg.LocalNodes)
		storeMode = st.Cfg.Mode
		globalTarget = st.Cfg.GlobalTarget
	}

	if jsonOut {
		payload := map[string]any{
			"mihomo":       map[string]any{"version": v.Version, "running": true},
			"mode":         cfg.Mode,
			"ports":        map[string]int{"mixed": cfg.MixedPort, "controller": controllerPortFromClient(c)},
			"groups":       groups,
			"subscriptions": subCount,
			"local_nodes":  localNodeCount,
			"store_mode":   storeMode,
			"global_target": globalTarget,
		}
		return writeJSON(out, payload)
	}

	fmt.Fprintf(out, "🟢 mihomo  %s   running\n", v.Version)
	fmt.Fprintf(out, "🔧 mode    %s\n", cfg.Mode)
	fmt.Fprintf(out, "🚪 ports   mixed=%d   controller=%d\n", cfg.MixedPort, controllerPortFromClient(c))

	if len(groups) == 0 {
		fmt.Fprintln(out, "🚀 groups  none")
	} else {
		summary := ""
		for i, g := range groups {
			if i > 0 {
				summary += ", "
			}
			summary += fmt.Sprintf("%s → %s", g.Name, g.Now)
		}
		fmt.Fprintf(out, "🚀 groups  %d selectable (%s)\n", len(groups), summary)
	}

	if st != nil {
		fmt.Fprintf(out, "📚 sources   %d subs + %d local nodes\n", subCount, localNodeCount)
		fmt.Fprintf(out, "🔀 routing   mode=%s  target=%s\n", storeMode, globalTarget)
	}
	return nil
}

// isUserSelectableType keeps only proxy-group types the user can switch.
func isUserSelectableType(t string) bool {
	switch t {
	case "Selector", "URLTest", "Fallback", "LoadBalance":
		return true
	}
	return false
}

// controllerPortFromClient parses the port out of the api.Client's BaseURL.
func controllerPortFromClient(c *api.Client) int {
	var port int
	_, _ = fmt.Sscanf(c.BaseURL, "http://127.0.0.1:%d", &port)
	return port
}
