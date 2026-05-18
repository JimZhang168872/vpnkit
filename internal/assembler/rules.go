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

// emitRules builds the final rules slice using the rc.7+ active-source
// model.
//
// Mode=global → single MATCH,🚀 Proxy (user rules not emitted).
// Mode=direct → single MATCH,🎯 Direct.
// Mode=rule:
//   1. local rules (user override, always prepended — they apply
//      regardless of which source is active)
//   2. EITHER the active source's own rules (subscription only — local
//      groups never carry rules) OR the template rules as fallback. Not
//      both: the active source's intent is "use my routing"; the template
//      is "the active source didn't ship routing, give me a sensible
//      default."
//   3. MATCH,🚀 Proxy — final catch-all. 🚀 Proxy's members are the
//      active source's nodes (+ DIRECT), so MATCH effectively routes
//      unmatched traffic to the active source.
//
// templateRules comes pre-trimmed from Assemble (trailing MATCH stripped).
func emitRules(mode Mode, locals []localrules.Rule, subs []groups.Group, activeSource string, templateRules []any) []any {
	if mode == ModeGlobal {
		return []any{"MATCH,🚀 Proxy"}
	}
	if mode == ModeDirect {
		return []any{"MATCH,🎯 Direct"}
	}

	out := make([]any, 0, len(locals)+8)
	// 1. local rules (highest priority, source-agnostic)
	for _, r := range locals {
		out = append(out, r.Render())
	}

	// 2. active source's own rules — only when active matches an enabled
	// subscription AND that sub returned a non-empty rules section.
	activeRules := pickActiveSubRules(subs, activeSource)
	if len(activeRules) > 0 {
		out = append(out, activeRules...)
	} else {
		// Fallback: template baseline (loyalsoldier's CN/GFW split, etc.)
		// Used when active=local-group OR active is a sub that shipped no
		// rules OR active is empty.
		out = append(out, templateRules...)
	}

	// 3. MATCH fallback — unmatched traffic flows through 🚀 Proxy, whose
	// member list (built in emitProxyGroups) is the active source's nodes
	// + DIRECT. So MATCH effectively routes through active.
	out = append(out, "MATCH,🚀 Proxy")
	return out
}

// pickActiveSubRules returns the named subscription's rendered rules
// (with target rewriting). Returns nil if:
//   - activeSource is empty,
//   - no enabled subscription has that name (e.g. it's a local group),
//   - the matched subscription has no rules.
//
// The empty return triggers the template fallback in emitRules.
func pickActiveSubRules(subs []groups.Group, activeSource string) []any {
	if activeSource == "" {
		return nil
	}
	siblingGroups := make(map[string]bool, len(subs))
	for _, g := range subs {
		if g.Enabled() {
			siblingGroups[g.Name()] = true
		}
	}
	for _, g := range subs {
		if !g.Enabled() || g.Name() != activeSource {
			continue
		}
		raw := g.Rules()
		if len(raw) == 0 {
			return nil
		}
		nodeMap := nodeNameSet(g)
		out := make([]any, 0, len(raw))
		for _, r := range raw {
			rewritten := rewriteTarget(r, g.Name(), nodeMap, siblingGroups)
			if rewritten.Target == "" {
				continue
			}
			out = append(out, rewritten.Render())
		}
		return out
	}
	return nil
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
//   - target names a sibling subscription group → pass through (cross-group routing)
//   - anything else (likely an internal proxy-group name) → group name
func rewriteTarget(r localrules.Rule, groupName string, nodeMap map[string]string, siblingGroups map[string]bool) localrules.Rule {
	if reservedTargets[r.Target] {
		return r
	}
	if ns, ok := nodeMap[r.Target]; ok {
		r.Target = ns
		return r
	}
	// Target names a sibling group — keep as-is so the user's cross-group
	// routing intent is preserved.
	if siblingGroups[r.Target] {
		return r
	}
	// Unknown target: heuristic — map to the current subscription's group,
	// preserving the user's high-level "this rule belongs to my group" intent.
	r.Target = groupName
	return r
}
