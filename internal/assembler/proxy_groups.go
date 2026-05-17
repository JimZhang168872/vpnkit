package assembler

import (
	"fmt"

	"vpnkit/internal/groups"
)

const (
	healthURL      = "http://www.gstatic.com/generate_204"
	healthInterval = 300
)

// emitProxyGroups builds the full proxy-groups slice: per-subscription
// select+url-test pairs, one select+url-test per local-nodes group, and the
// three top-level routing groups (🚀 Proxy / 🎯 Direct / 🛑 Reject).
func emitProxyGroups(subs []groups.Group, localGroups []groups.Group, globalTarget string) []any {
	out := []any{}
	topProxies := []string{}

	// Subscription groups (each → <name> select + <name>-auto url-test).
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		nodes := nodeNames(g)
		if len(nodes) == 0 {
			continue
		}
		autoName := g.Name() + "-auto"
		out = append(out, map[string]any{
			"name":    g.Name(),
			"type":    "select",
			"proxies": append([]string{autoName}, nodes...),
		})
		out = append(out, map[string]any{
			"name":     autoName,
			"type":     "url-test",
			"proxies":  nodes,
			"url":      healthURL,
			"interval": healthInterval,
		})
		topProxies = append(topProxies, autoName, g.Name())
	}

	// Local-nodes groups (symmetric with subs: <name> select + <name>-auto url-test).
	for _, lg := range localGroups {
		if !lg.Enabled() {
			continue
		}
		nodes := nodeNames(lg)
		if len(nodes) == 0 {
			continue
		}
		autoName := lg.Name() + "-auto"
		out = append(out, map[string]any{
			"name":    lg.Name(),
			"type":    "select",
			"proxies": append([]string{autoName}, nodes...),
		})
		out = append(out, map[string]any{
			"name":     autoName,
			"type":     "url-test",
			"proxies":  nodes,
			"url":      healthURL,
			"interval": healthInterval,
		})
		topProxies = append(topProxies, autoName, lg.Name())
	}

	topProxies = append(topProxies, "DIRECT")

	// GlobalTarget goes first so mihomo picks it as the select default.
	topProxies = withTargetFirst(topProxies, globalTarget)

	out = append(out,
		map[string]any{"name": "🚀 Proxy", "type": "select", "proxies": topProxies},
		map[string]any{"name": "🎯 Direct", "type": "select", "proxies": []string{"DIRECT"}},
		map[string]any{"name": "🛑 Reject", "type": "select", "proxies": []string{"REJECT", "DIRECT"}},
	)
	return out
}

func nodeNames(g groups.Group) []string {
	prox := g.Proxies()
	out := make([]string, 0, len(prox))
	for _, p := range prox {
		origName, _ := p["name"].(string)
		out = append(out, fmt.Sprintf("%s:%s", g.Name(), origName))
	}
	return out
}

// withTargetFirst moves target to index 0. Defensive: Assemble fills a
// default GlobalTarget before calling, so target=="" should be unreachable;
// the guard keeps the function safe for direct unit testing.
func withTargetFirst(list []string, target string) []string {
	if target == "" {
		return list
	}
	for i, x := range list {
		if x == target {
			return append([]string{target}, append(list[:i:i], list[i+1:]...)...)
		}
	}
	// Target not in list (e.g. user typed a specific node). Prepend it; mihomo
	// will accept the value even if it's a leaf proxy.
	return append([]string{target}, list...)
}
