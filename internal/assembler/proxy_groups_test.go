package assembler

import (
	"strings"
	"testing"

	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

func TestAssembleEmitsTwoGroupsPerSubscription(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{
			{"name": "HK-A", "type": "ss"},
			{"name": "JP-B", "type": "vmess"},
		},
	})
	local := localnodes.New()
	_ = local.Add(localnodes.Node{Name: "HK-Manual", Proto: "hysteria2"})

	out, _ := Assemble(Input{
		Subscriptions:    []groups.Group{sub},
		LocalNodes:       groups.NewLocalNodesGroup("local", local),
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
		GlobalTarget:     "doge-auto",
	})
	s := string(out)
	for _, want := range []string{
		"name: doge",        // select
		"name: doge-auto",  // url-test
		"type: url-test",
		"name: local",      // local group
		"\U0001F680 Proxy",       // top-level (yaml may quote the name)
		"DIRECT", "REJECT",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in:\n%s", want, s)
		}
	}
}
