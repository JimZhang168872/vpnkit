package assembler

import (
	"strings"
	"testing"

	"vpnkit/internal/groups"
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
