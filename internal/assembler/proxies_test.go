package assembler

import (
	"strings"
	"testing"

	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

func TestAssembleNamespacesProxies(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{
			{"name": "HK-A", "type": "ss", "server": "1.2.3.4", "port": 443},
		},
	})
	local := localnodes.New()
	_ = local.Add(localnodes.Node{Name: "HK-Manual", Proto: "hysteria2", Server: "5.6.7.8", Port: 443, Fields: map[string]any{"password": "x"}})
	out, err := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{sub},
		LocalNodes:       groups.NewLocalNodesGroup("local", local),
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "name: doge:HK-A") {
		t.Errorf("subscription proxy not namespaced: %s", s)
	}
	if !strings.Contains(s, "name: local:HK-Manual") {
		t.Errorf("local node not namespaced: %s", s)
	}
}
