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
// Mode=rule   → local rules (user override) → per-subscription rules
//               (with target rewriting) → templateRules (baseline like
//               loyalsoldier's CN/GFW split) → MATCH,🚀 Proxy fallback.
//
// templateRules comes pre-trimmed from Assemble (trailing MATCH stripped
// so emitRules' own MATCH stays the sole catch-all).
func emitRules(mode Mode, locals []localrules.Rule, subs []groups.Group, templateRules []any) []any {
	if mode == ModeGlobal {
		return []any{"MATCH,🚀 Proxy"}
	}
	if mode == ModeDirect {
		return []any{"MATCH,🎯 Direct"}
	}

	// Build the set of all enabled subscription group names so rewriteTarget
	// can recognize a target that names a *sibling* group and pass it through
	// rather than blindly rewriting to the current group.
	siblingGroups := make(map[string]bool, len(subs))
	for _, g := range subs {
		if g.Enabled() {
			siblingGroups[g.Name()] = true
		}
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
			rewritten := rewriteTarget(r, g.Name(), nodeMap, siblingGroups)
			if rewritten.Target == "" {
				continue // dropped
			}
			out = append(out, rewritten.Render())
		}
	}
	// 3. template baseline (RULE-SET → reject / direct / proxy; GEOIP,CN
	// etc.). Trailing MATCH was already stripped in Assemble so it can't
	// short-circuit subscription rules above OR the fallback below.
	out = append(out, templateRules...)
	// 4. MATCH fallback — unmatched traffic flows through 🚀 Proxy. Whether
	// that resolves to a proxy or DIRECT depends on which member 🚀 Proxy
	// is set to (GlobalTarget controls the default).
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
