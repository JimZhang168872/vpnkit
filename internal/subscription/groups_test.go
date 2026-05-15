package subscription

import (
	"testing"

	"vpnkit/internal/subscription/proto"
)

func TestSynthesizeGroups(t *testing.T) {
	proxies := []proto.Proxy{
		{"name": "HK-01", "type": "ss"},
		{"name": "JP-02", "type": "vmess"},
	}
	g := SynthesizeGroups(proxies)
	if len(g) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(g))
	}
	names := map[string]bool{}
	for _, grp := range g {
		names[grp["name"].(string)] = true
	}
	for _, want := range []string{"🚀 Proxy", "♻️ Auto", "🎯 Direct", "🛑 Reject"} {
		if !names[want] {
			t.Errorf("missing group %s", want)
		}
	}
	for _, grp := range g {
		if grp["name"] == "🚀 Proxy" {
			members := grp["proxies"].([]string)
			if !contains(members, "HK-01") || !contains(members, "JP-02") {
				t.Errorf("Proxy group members: %v", members)
			}
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
