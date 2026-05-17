package groups

import (
	"testing"

	"vpnkit/internal/localrules"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

func TestSubscriptionGroupContract(t *testing.T) {
	res := &subscription.Result{
		Source: "clash",
		Proxies: []proto.Proxy{
			{"name": "HK-A", "type": "ss", "server": "1.2.3.4", "port": 443, "cipher": "aes-256-gcm", "password": "x"},
			{"name": "JP-B", "type": "vmess", "server": "5.6.7.8", "port": 8443, "uuid": "u"},
		},
		Raw: map[string]any{
			"rules": []any{
				"DOMAIN-SUFFIX,youtube.com,🚀 Proxy",
				"DOMAIN-SUFFIX,netflix.com,🚀 Proxy",
			},
		},
	}
	g := NewSubscriptionGroup("doge", true, res)
	if g.Name() != "doge" {
		t.Errorf("Name: %q", g.Name())
	}
	if g.Kind() != KindSubscription {
		t.Errorf("Kind: %v", g.Kind())
	}
	if !g.Enabled() {
		t.Error("Enabled should be true")
	}
	if len(g.Proxies()) != 2 {
		t.Errorf("Proxies len: %d", len(g.Proxies()))
	}
	rules := g.Rules()
	if len(rules) != 2 {
		t.Fatalf("Rules len: %d", len(rules))
	}
	if rules[0] != (localrules.Rule{Type: "DOMAIN-SUFFIX", Payload: "youtube.com", Target: "🚀 Proxy"}) {
		t.Errorf("Rules[0]: %+v", rules[0])
	}
}

func TestLocalNodesGroupContract(t *testing.T) {
	m := localnodes.New()
	_ = m.Add(localnodes.Node{Name: "HK-Manual", Proto: "hysteria2", Server: "1.2.3.4", Port: 443, Fields: map[string]any{"password": "x"}})
	g := NewLocalNodesGroup("local", m)
	if g.Kind() != KindLocalNodes {
		t.Errorf("Kind: %v", g.Kind())
	}
	if !g.Enabled() {
		t.Error("Enabled should be true (always for local)")
	}
	prox := g.Proxies()
	if len(prox) != 1 || prox[0]["name"] != "HK-Manual" {
		t.Errorf("Proxies: %v", prox)
	}
	if g.Rules() != nil {
		t.Errorf("LocalNodesGroup should expose nil Rules: %v", g.Rules())
	}
}

func TestSubscriptionGroupNilResultSafe(t *testing.T) {
	g := NewSubscriptionGroup("orphan", true, nil)
	if g.Name() != "orphan" {
		t.Errorf("Name: %q", g.Name())
	}
	if g.Kind() != KindSubscription {
		t.Errorf("Kind: %v", g.Kind())
	}
	if !g.Enabled() {
		t.Error("Enabled: should be true")
	}
	if g.Proxies() != nil {
		t.Errorf("Proxies should be nil for nil result, got %v", g.Proxies())
	}
	if g.Rules() != nil {
		t.Errorf("Rules should be nil for nil result, got %v", g.Rules())
	}
}

func TestSubscriptionGroupMalformedRuleLineSkipped(t *testing.T) {
	res := &subscription.Result{
		Raw: map[string]any{
			"rules": []any{
				"JUST-ONE-FIELD",                     // 1-part → skipped
				42,                                   // non-string → skipped
				"DOMAIN-SUFFIX,kept.com,🚀 Proxy",   // 3-part → kept
				"MATCH,🚀 Proxy",                     // 2-part → kept (no payload)
			},
		},
	}
	g := NewSubscriptionGroup("x", true, res)
	rs := g.Rules()
	if len(rs) != 2 {
		t.Fatalf("expected 2 rules after skipping malformed, got %d (%v)", len(rs), rs)
	}
	if rs[0].Payload != "kept.com" {
		t.Errorf("rs[0]: %+v", rs[0])
	}
	if rs[1].Type != "MATCH" || rs[1].Payload != "" {
		t.Errorf("rs[1] MATCH should have empty payload: %+v", rs[1])
	}
}

func TestSubscriptionGroupDisabled(t *testing.T) {
	g := NewSubscriptionGroup("off", false, &subscription.Result{})
	if g.Enabled() {
		t.Error("Enabled should be false")
	}
}
