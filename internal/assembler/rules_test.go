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
