package assembler

import (
	"fmt"

	"vpnkit/internal/groups"
)

const (
	healthURL      = "http://www.gstatic.com/generate_204"
	healthInterval = 300
)

// emitProxyGroups builds the full proxy-groups slice:
//
//   - One <name> Selector + <name>-auto URLTest per enabled subscription
//     AND per enabled local-nodes group. Symmetrical; mihomo doesn't
//     distinguish source kind.
//   - Top-level 🚀 Proxy Selector whose members come ONLY from the active
//     source (+ DIRECT). MATCH,🚀 Proxy in the emitted rules then routes
//     unmatched traffic to the active source's url-test best pick.
//   - 🎯 Direct + 🛑 Reject helper selectors so template rules that
//     reference them resolve cleanly.
//
// Non-active sources are still emitted as their own proxy-groups so the
// user can switch active without re-assembling. They're just not part of
// 🚀 Proxy's membership.
func emitProxyGroups(subs []groups.Group, localGroups []groups.Group, activeSource, globalTarget string) []any {
	out := []any{}

	emitPair := func(name string, nodes []string) {
		autoName := name + "-auto"
		out = append(out, map[string]any{
			"name":    name,
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
	}

	// Subscription groups (each → <name> select + <name>-auto url-test).
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		nodes := nodeNames(g)
		if len(nodes) == 0 {
			continue
		}
		emitPair(g.Name(), nodes)
	}
	// Local-nodes groups (symmetric with subs).
	for _, lg := range localGroups {
		if !lg.Enabled() {
			continue
		}
		nodes := nodeNames(lg)
		if len(nodes) == 0 {
			continue
		}
		emitPair(lg.Name(), nodes)
	}

	// 🚀 Proxy's members come ONLY from the active source. Other sources
	// remain in the config but stay out of the top-level Selector to keep
	// MATCH routing deterministic per the user's "选谁用谁" intent.
	topProxies := topProxyMembersFor(activeSource, subs, localGroups)
	if globalTarget != "" {
		topProxies = withTargetFirst(topProxies, globalTarget)
	}

	out = append(out,
		map[string]any{"name": topLevelProxyGroup, "type": "select", "proxies": topProxies},
		map[string]any{"name": "🎯 Direct", "type": "select", "proxies": []string{"DIRECT"}},
		map[string]any{"name": "🛑 Reject", "type": "select", "proxies": []string{"REJECT", "DIRECT"}},
	)
	return out
}

// topProxyMembersFor returns the 🚀 Proxy Selector's member list for the
// given active source: [<active>-auto, <active>, DIRECT].
//
// If the active source name doesn't match any enabled source (e.g. user
// removed it without picking a replacement), or is empty, fall back to
// [DIRECT] — at least traffic doesn't break, and the user gets a visible
// "no proxy active" signal.
func topProxyMembersFor(activeSource string, subs, localGroups []groups.Group) []string {
	if activeSource == "" {
		return []string{"DIRECT"}
	}
	found := false
	for _, g := range subs {
		if g.Enabled() && g.Name() == activeSource {
			found = true
			break
		}
	}
	if !found {
		for _, lg := range localGroups {
			if lg.Enabled() && lg.Name() == activeSource {
				found = true
				break
			}
		}
	}
	if !found {
		return []string{"DIRECT"}
	}
	return []string{activeSource + "-auto", activeSource, "DIRECT"}
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

// withTargetFirst moves target to index 0 so mihomo treats it as the
// Selector's default member.
//
// Critical safety guard: refuses to emit the top-level group's own name
// ("🚀 Proxy") into its member list. Older vpnkit installs (rc.x) had
// store.Cfg.GlobalTarget = "🚀 Proxy" as the default, which without this
// guard produced a self-referential proxy-group:
//
//   - name: 🚀 Proxy
//     type: select
//     proxies: [🚀 Proxy, doge-auto, doge, ...]   ← cycle!
//
// mihomo refuses to load that config:
//   Parse config error: loop is detected in ProxyGroup, please check
//   following ProxyGroups: [🚀 Proxy]
//
// store.Load also migrates this away on disk, but the assembler-level
// guard is the choke point — it stays correct regardless of how
// GlobalTarget got into the input (TUI form, CLI `vpnkit target`, a
// future migration that misses an edge case, hand-edited store.toml).
func withTargetFirst(list []string, target string) []string {
	if target == "" || target == topLevelProxyGroup {
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

// topLevelProxyGroup names the synthetic Selector group that vpnkit
// always emits as the entry point. Centralized as a constant so the
// self-reference guard in withTargetFirst stays in sync with the literal
// emitted in emitProxyGroups below.
const topLevelProxyGroup = "🚀 Proxy"
