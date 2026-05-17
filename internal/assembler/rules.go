package assembler

import (
	"vpnkit/internal/groups"
	"vpnkit/internal/localrules"
)

// reservedTargets are mihomo/vpnkit builtin names that pass through without
// rewriting when they appear as a rule target inside a subscription's rules.
var reservedTargets = map[string]bool{
	"🚀 Proxy":  true,
	"🎯 Direct": true,
	"🛑 Reject": true,
	"DIRECT":    true,
	"REJECT":    true,
}

// emitRules builds the final rules slice.
//
// Mode=global → single MATCH,🚀 Proxy (user rules not emitted).
// Mode=direct → single MATCH,🎯 Direct.
// Mode=rule   → local rules, then per-subscription rules (with target
// rewriting), then MATCH,🚀 Proxy fallback.
func emitRules(mode Mode, locals []localrules.Rule, subs []groups.Group) []any {
	if mode == ModeGlobal {
		return []any{"MATCH,🚀 Proxy"}
	}
	if mode == ModeDirect {
		return []any{"MATCH,🎯 Direct"}
	}

	out := make([]any, 0, len(locals)+8)
	// 1. local rules (highest priority)
	for _, r := range locals {
		out = append(out, r.Render())
	}
	// 2. each subscription's own rules, with target rewriting
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		nodeMap := nodeNameSet(g) // original name → namespaced name
		for _, r := range g.Rules() {
			rewritten := rewriteTarget(r, g.Name(), nodeMap)
			if rewritten.Target == "" {
				continue // dropped
			}
			out = append(out, rewritten.Render())
		}
	}
	// 3. MATCH fallback
	out = append(out, "MATCH,🚀 Proxy")
	return out
}

// nodeNameSet builds a map from the subscription node's original name to its
// namespaced form "<group>:<original>".
func nodeNameSet(g groups.Group) map[string]string {
	m := make(map[string]string)
	for _, p := range g.Proxies() {
		orig, _ := p["name"].(string)
		m[orig] = g.Name() + ":" + orig
	}
	return m
}

// rewriteTarget adjusts a subscription rule's target:
//   - reserved target (🚀 Proxy / 🎯 Direct / 🛑 Reject / DIRECT / REJECT) → unchanged
//   - original node name present in nodeMap → "<group>:<node>"
//   - anything else (likely an internal proxy-group name) → group name
func rewriteTarget(r localrules.Rule, groupName string, nodeMap map[string]string) localrules.Rule {
	if reservedTargets[r.Target] {
		return r
	}
	if ns, ok := nodeMap[r.Target]; ok {
		r.Target = ns
		return r
	}
	// Heuristic: any other unrecognized target (often an internal proxy-group
	// name from the subscription) is mapped to the subscription's group name
	// so user routing intent is preserved at the group level.
	r.Target = groupName
	return r
}
