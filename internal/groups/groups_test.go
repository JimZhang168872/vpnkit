package groups

import (
	"testing"

	"vpnkit/internal/localrules"
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
