package assembler

import (
	"strings"
	"testing"

	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/localrules"
	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

func TestAssembleRulesOrdering(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "X", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{
				"DOMAIN-SUFFIX,youtube.com,🚀 Proxy",
			},
		},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{sub},
		LocalRules:       []localrules.Rule{{Type: "DOMAIN-SUFFIX", Payload: "baidu.com", Target: "🎯 Direct"}},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	s := string(out)
	idxBaidu := strings.Index(s, "DOMAIN-SUFFIX,baidu.com")
	idxYoutube := strings.Index(s, "DOMAIN-SUFFIX,youtube.com")
	idxMatch := strings.Index(s, "MATCH,")
	if idxBaidu < 0 || idxYoutube < 0 || idxMatch < 0 {
		t.Fatalf("missing rules:\n%s", s)
	}
	if !(idxBaidu < idxYoutube && idxYoutube < idxMatch) {
		t.Errorf("expected order baidu < youtube < MATCH but got %d %d %d", idxBaidu, idxYoutube, idxMatch)
	}
}

func TestAssembleSubscriptionRuleRewritesTarget(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "HK-A", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{
				"DOMAIN-SUFFIX,a.com,Hong-Kong", // internal group name → rewrite to doge
				"DOMAIN-SUFFIX,b.com,HK-A",     // internal node name → doge:HK-A
				"DOMAIN-SUFFIX,c.com,🚀 Proxy",  // reserved → keep
			},
		},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{sub},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	s := string(out)
	if !strings.Contains(s, "DOMAIN-SUFFIX,a.com,doge") {
		t.Errorf("group-name rewrite missing: %s", s)
	}
	if !strings.Contains(s, "DOMAIN-SUFFIX,b.com,doge:HK-A") {
		t.Errorf("node-name rewrite missing: %s", s)
	}
	if !strings.Contains(s, "DOMAIN-SUFFIX,c.com,\U0001F680 Proxy") {
		t.Errorf("reserved target should be preserved: %s", s)
	}
}

func TestAssembleSubscriptionRuleSiblingGroupPassthrough(t *testing.T) {
	subDoge := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "HK-A", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{
				"DOMAIN-SUFFIX,example.com,boost", // sibling group name → pass through
			},
		},
	})
	subBoost := groups.NewSubscriptionGroup("boost", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "SG-1", "type": "ss"}},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{subDoge, subBoost},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	s := string(out)
	if !strings.Contains(s, "DOMAIN-SUFFIX,example.com,boost") {
		t.Errorf("sibling-group target should pass through, got:\n%s", s)
	}
	if strings.Contains(s, "DOMAIN-SUFFIX,example.com,doge") {
		t.Errorf("sibling-group target should NOT be rewritten to current group, got:\n%s", s)
	}
}

func TestAssembleModeGlobal(t *testing.T) {
	out, _ := Assemble(Input{
		Mode: ModeGlobal, MixedPort: 50595, ControllerPort: 32645, ControllerSecret: "s",
	})
	s := string(out)
	if !strings.Contains(s, "MATCH,\U0001F680 Proxy") {
		t.Errorf("mode=global rules should be MATCH,🚀 Proxy only:\n%s", s)
	}
	// Must NOT contain anything else in rules section.
	if strings.Count(s, "DOMAIN-SUFFIX") > 0 {
		t.Errorf("mode=global must skip user rules: %s", s)
	}
}

func TestAssembleModeDirect(t *testing.T) {
	out, _ := Assemble(Input{
		Mode: ModeDirect, MixedPort: 50595, ControllerPort: 32645, ControllerSecret: "s",
	})
	if !strings.Contains(string(out), "MATCH,\U0001F3AF Direct") {
		t.Errorf("mode=direct must end with MATCH,🎯 Direct: %s", out)
	}
}

// TestAssembleBakesRuleTemplateProviders regresses a bug where the rule
// template (bootstrap-time only via config.BuildSkeleton) got stripped
// from config.yaml on the very next assemble. After a single TUI Sources
// mutation, mihomo lost every RULE-SET reference and matched only the
// final MATCH fallback → user perception "rules disappear / never come
// back after first launch."
//
// Fix: assembler.Input carries RuleTemplate, Assemble loads the named
// embedded template, merges its rule-providers into the doc, and emits
// its rules as the baseline tier (after local + subscription rules,
// before the catch-all MATCH).
func TestAssembleBakesRuleTemplateProviders(t *testing.T) {
	out, err := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    nil,
		LocalGroups:      nil,
		LocalRules:       nil,
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		GlobalTarget:     "DIRECT",
		RuleTemplate:     "loyalsoldier",
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"rule-providers:",          // section present
		"reject:",                  // a provider key from the template
		"cncidr:",                  // another
		"RULE-SET,reject",          // a rule from the template
		"GEOIP,CN",                 // baseline china-direct
		"MATCH,\U0001F680 Proxy",   // final fallback
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in output:\n%s", want, s)
		}
	}
}

// TestAssembleMinimalTemplate covers a happy-path tighter template, and
// asserts user's local rules still win over template rules (priority
// order: local → subscription → template → MATCH).
func TestAssembleMinimalTemplate(t *testing.T) {
	out, err := Assemble(Input{
		Mode:             ModeRule,
		LocalRules:       []localrules.Rule{{Type: "DOMAIN", Payload: "private.example.com", Target: "DIRECT"}},
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		GlobalTarget:     "DIRECT",
		RuleTemplate:     "minimal",
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	s := string(out)
	idxLocal := strings.Index(s, "DOMAIN,private.example.com,DIRECT")
	idxTemplate := strings.Index(s, "GEOIP,CN")
	if idxLocal < 0 || idxTemplate < 0 {
		t.Fatalf("missing rules:\n%s", s)
	}
	if idxLocal > idxTemplate {
		t.Errorf("local rule must come before template rule:\nlocal at %d, template at %d", idxLocal, idxTemplate)
	}
}

// ─── active-source model tests (rc.7+) ───────────────────────────────────
//
// vpnkit's rules model: one "active source" (subscription OR local-node
// group) drives routing. The active source's own rules are emitted; if it
// has none (or it's a local group, which never carries rules), the rule
// template (loyalsoldier by default) fills in. User local-rules are
// always prepended so they apply regardless of active source.

// TestActiveSourceEmitsOnlyActiveSubscriptionRules — with two subscriptions
// `doge` and `boost`, and ActiveSource="boost", only boost's rules
// must appear in the output. doge's rules are completely ignored even
// though doge is otherwise enabled and present in proxy-groups.
func TestActiveSourceEmitsOnlyActiveSubscriptionRules(t *testing.T) {
	doge := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "HK-A", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{"DOMAIN-SUFFIX,doge-only.example,🚀 Proxy"},
		},
	})
	boost := groups.NewSubscriptionGroup("boost", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "SG-1", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{"DOMAIN-SUFFIX,boost-only.example,🚀 Proxy"},
		},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		ActiveSource:     "boost",
		Subscriptions:    []groups.Group{doge, boost},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	s := string(out)
	if !strings.Contains(s, "boost-only.example") {
		t.Errorf("active sub's rules must be present:\n%s", s)
	}
	if strings.Contains(s, "doge-only.example") {
		t.Errorf("inactive sub's rules must NOT leak into output:\n%s", s)
	}
}

// TestActiveSourceFallsBackToTemplateWhenSubHasNoRules — a subscription
// that returned only proxies (no rules section) must trigger the
// loyalsoldier template fallback. Critical for "subs only carrying nodes
// but no routing" providers and for hand-entered single-URI subs.
func TestActiveSourceFallsBackToTemplateWhenSubHasNoRules(t *testing.T) {
	bare := groups.NewSubscriptionGroup("bare", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "X", "type": "ss"}},
		Raw:     map[string]any{}, // no "rules" key
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		ActiveSource:     "bare",
		Subscriptions:    []groups.Group{bare},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
		RuleTemplate:     "loyalsoldier",
	})
	s := string(out)
	if !strings.Contains(s, "GEOIP,CN") {
		t.Errorf("template fallback (loyalsoldier) must kick in when active sub has no rules:\n%s", s)
	}
}

// TestActiveSourceLocalGroupAlwaysUsesTemplate — local-node groups never
// carry their own rules (they're collections of user-entered URIs), so
// when active=local-group the template fallback is the only routing
// source besides user local-rules.
func TestActiveSourceLocalGroupAlwaysUsesTemplate(t *testing.T) {
	mgr := localnodes.New()
	_ = mgr.Add(localnodes.Node{
		Name: "jim-hy2", Group: "Local",
		Proto: "hysteria2", Server: "1.2.3.4", Port: 443,
		Fields: map[string]any{"password": "x"},
	})
	local := groups.NewLocalNodesGroupForGroup("Local", mgr)
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		ActiveSource:     "Local",
		LocalGroups:      []groups.Group{local},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
		RuleTemplate:     "loyalsoldier",
	})
	s := string(out)
	if !strings.Contains(s, "GEOIP,CN") {
		t.Errorf("local-group active source must emit template rules:\n%s", s)
	}
}

// TestActiveSourceLocalRulesAlwaysPrepended — the user's CLI/TUI
// local-rules win over the active sub's rules regardless. Per user's
// rc.7 design: "用户加的规则在选中节点上永远生效". When active sub has its
// own rules, the template is intentionally NOT emitted (active sub's
// rules are the routing source of truth).
func TestActiveSourceLocalRulesAlwaysPrepended(t *testing.T) {
	boost := groups.NewSubscriptionGroup("boost", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "SG-1", "type": "ss"}},
		Raw: map[string]any{
			"rules": []any{"DOMAIN-SUFFIX,boost-rule.example,🚀 Proxy"},
		},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		ActiveSource:     "boost",
		Subscriptions:    []groups.Group{boost},
		LocalRules:       []localrules.Rule{{Type: "DOMAIN-SUFFIX", Payload: "user-rule.example", Target: "DIRECT"}},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
		RuleTemplate:     "loyalsoldier",
	})
	s := string(out)
	idxLocal := strings.Index(s, "DOMAIN-SUFFIX,user-rule.example,DIRECT")
	idxSub := strings.Index(s, "DOMAIN-SUFFIX,boost-rule.example")
	idxMatch := strings.Index(s, "MATCH,\U0001F680 Proxy")
	if idxLocal < 0 || idxSub < 0 || idxMatch < 0 {
		t.Fatalf("missing rules:\n%s", s)
	}
	if !(idxLocal < idxSub && idxSub < idxMatch) {
		t.Errorf("expected order local < sub < MATCH; got %d %d %d", idxLocal, idxSub, idxMatch)
	}
	// Template MUST NOT emit when active sub provides its own rules —
	// "选谁用谁": the active sub's intent overrides the template.
	if strings.Contains(s, "GEOIP,CN") {
		t.Errorf("template should NOT emit when active sub has its own rules:\n%s", s)
	}
}

// TestActiveSourceTopProxyMembersAreOnlyActive — 🚀 Proxy Selector
// members come from the active source only (+ DIRECT), so MATCH traffic
// routes to active. Other groups (doge, doge-auto, ...) are still emitted
// as separate proxy-groups so the user can switch active without
// re-assembling, but they're not part of 🚀 Proxy's membership.
func TestActiveSourceTopProxyMembersAreOnlyActive(t *testing.T) {
	doge := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "HK-A", "type": "ss"}},
	})
	boost := groups.NewSubscriptionGroup("boost", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "SG-1", "type": "ss"}},
	})
	out, _ := Assemble(Input{
		Mode:             ModeRule,
		ActiveSource:     "boost",
		Subscriptions:    []groups.Group{doge, boost},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	s := string(out)
	// Find the 🚀 Proxy group definition and look at its `proxies:` line.
	// Crude but adequate — full YAML round-trip would over-couple to layout.
	// yaml.v3 quotes the emoji-bearing name; match either quoted or bare.
	topIdx := strings.Index(s, "\"\U0001F680 Proxy\"")
	if topIdx < 0 {
		topIdx = strings.Index(s, "\U0001F680 Proxy")
	}
	if topIdx < 0 {
		t.Fatalf("no 🚀 Proxy group in output:\n%s", s)
	}
	rest := s[topIdx:]
	endIdx := strings.Index(rest[1:], "- name:") // next proxy-group entry
	if endIdx < 0 {
		endIdx = len(rest)
	}
	topBlock := rest[:endIdx+1]
	if !strings.Contains(topBlock, "boost-auto") {
		t.Errorf("🚀 Proxy should contain active source `boost-auto`:\n%s", topBlock)
	}
	if strings.Contains(topBlock, "doge-auto") || strings.Contains(topBlock, "- doge\n") {
		t.Errorf("🚀 Proxy should NOT contain inactive source `doge`:\n%s", topBlock)
	}
}

// TestAssembleEmptyRuleTemplate stays backwards-compatible: callers that
// don't specify a template (or pass "") get the previous behavior — no
// rule-providers section, only their own + subscription rules + MATCH.
func TestAssembleEmptyRuleTemplate(t *testing.T) {
	out, err := Assemble(Input{
		Mode:             ModeRule,
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if strings.Contains(string(out), "rule-providers:") {
		t.Errorf("empty RuleTemplate should not emit rule-providers section:\n%s", out)
	}
}
