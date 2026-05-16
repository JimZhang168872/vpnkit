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

	type profileSummary struct {
		Name        string `json:"name"`
		NodeCount   int    `json:"node_count"`
		LastUpdated string `json:"last_updated,omitempty"`
	}
	var profile *profileSummary
	if st != nil && st.Cfg.ActiveProfile != "" {
		for _, p := range st.Cfg.Profiles {
			if p.Name == st.Cfg.ActiveProfile {
				profile = &profileSummary{
					Name:        p.Name,
					NodeCount:   0,
					LastUpdated: p.LastUpdated.Format(time.RFC3339),
				}
				break
			}
		}
	}

	if jsonOut {
		payload := map[string]any{
			"mihomo": map[string]any{"version": v.Version, "running": true},
			"mode":   cfg.Mode,
			"ports":  map[string]int{"mixed": cfg.MixedPort, "controller": controllerPortFromClient(c)},
			"groups": groups,
		}
		if profile != nil {
			payload["active_profile"] = profile
		}
		return writeJSON(out, payload)
	}

	fmt.Fprintf(out, "mihomo  %s   ● running\n", v.Version)
	fmt.Fprintf(out, "mode    %s\n", cfg.Mode)
	fmt.Fprintf(out, "ports   mixed=%d   controller=%d\n", cfg.MixedPort, controllerPortFromClient(c))

	if len(groups) == 0 {
		fmt.Fprintln(out, "groups  none")
	} else {
		summary := ""
		for i, g := range groups {
			if i > 0 {
				summary += ", "
			}
			summary += fmt.Sprintf("%s → %s", g.Name, g.Now)
		}
		fmt.Fprintf(out, "groups  %d selectable (%s)\n", len(groups), summary)
	}

	if profile != nil {
		fmt.Fprintf(out, "profile %s\n", profile.Name)
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
