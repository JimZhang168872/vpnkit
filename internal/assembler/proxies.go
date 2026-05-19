package assembler

import (
	"fmt"

	"vpnkit/internal/groups"
	"vpnkit/internal/subscription/proto"
)

// emitProxies returns a flat slice of mihomo proxy maps with every node's
// name rewritten to "<group>:<original-name>" so cross-group duplicates
// don't collide in mihomo's flat namespace. localGroups is one Group per
// enabled local-nodes-group.
//
// dialer-proxy field rewriting:
// Local nodes can chain through another node via `dialer-proxy`. The user
// supplies the chain target by its UNQUALIFIED name (e.g. "New York-phone"),
// but mihomo's flat namespace only knows the namespaced form
// ("local:New York-phone") after this pass. Without rewriting, mihomo
// rejects the config with "dialer-proxy [New York-phone] not found".
//
// We resolve `dialer-proxy` in two passes:
//  1. Build a map { original-name → "<group>:<original-name>" } across all
//     emitted nodes (subs + local groups).
//  2. For every emitted node carrying `dialer-proxy`, look up the value:
//     - matches a node's original name → rewrite to namespaced form
//     - matches a proxy-group name (sub/local group, "DIRECT", "REJECT", or
//       a "🚀 Proxy" / "🎯 Direct" / "🛑 Reject" helper) → leave as-is
//     - otherwise leave as-is so the validation error surfaces with the
//       user's literal string in the message (debuggable).
//
// Group names take priority on collision: if a user has a local-nodes-group
// named "JP-Hub" and a node also literally named "JP-Hub" inside another
// group, dialer-proxy: JP-Hub resolves to the group (proxy-group selection
// → url-test fastest member).
func emitProxies(subs []groups.Group, localGroups []groups.Group) []any {
	// Pass 1: collect raw → namespaced mapping. Same key from a later group
	// overwrites; that ambiguity is acceptable because user-side workflows
	// avoid duplicate node names within a sourced session.
	nameMap := map[string]string{}
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		for _, p := range g.Proxies() {
			if orig, ok := p["name"].(string); ok && orig != "" {
				nameMap[orig] = fmt.Sprintf("%s:%s", g.Name(), orig)
			}
		}
	}
	for _, g := range localGroups {
		if !g.Enabled() {
			continue
		}
		for _, p := range g.Proxies() {
			if orig, ok := p["name"].(string); ok && orig != "" {
				nameMap[orig] = fmt.Sprintf("%s:%s", g.Name(), orig)
			}
		}
	}
	groupNames := map[string]bool{
		"DIRECT": true, "REJECT": true,
		topLevelProxyGroup: true, "🎯 Direct": true, "🛑 Reject": true,
	}
	for _, g := range subs {
		if g.Enabled() {
			groupNames[g.Name()] = true
			groupNames[g.Name()+"-auto"] = true
		}
	}
	for _, g := range localGroups {
		if g.Enabled() {
			groupNames[g.Name()] = true
			groupNames[g.Name()+"-auto"] = true
		}
	}

	// Pass 2: emit with dialer-proxy resolution.
	out := []any{}
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		for _, p := range g.Proxies() {
			out = append(out, namespaced(g.Name(), p, nameMap, groupNames))
		}
	}
	for _, g := range localGroups {
		if !g.Enabled() {
			continue
		}
		for _, p := range g.Proxies() {
			out = append(out, namespaced(g.Name(), p, nameMap, groupNames))
		}
	}
	return out
}

func namespaced(groupName string, p proto.Proxy, nameMap map[string]string, groupNames map[string]bool) map[string]any {
	dup := make(map[string]any, len(p))
	for k, v := range p {
		dup[k] = v
	}
	origName, _ := dup["name"].(string)
	dup["name"] = fmt.Sprintf("%s:%s", groupName, origName)

	// Resolve dialer-proxy if present. Group names take priority over node
	// names (see emitProxies header for why) — if both happen to match, the
	// group wins.
	if via, ok := dup["dialer-proxy"].(string); ok && via != "" {
		if !groupNames[via] {
			if mapped, found := nameMap[via]; found {
				dup["dialer-proxy"] = mapped
			}
		}
	}
	return dup
}
