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
		LocalGroups:      []groups.Group{groups.NewLocalNodesGroup("local", local)},
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

// localGroupFromProxies builds a Group exposing arbitrary node maps under
// groupName. Helper for tests that need to inject specific fields (e.g.
// dialer-proxy) without going through the full localnodes Add validation.
func localGroupFromProxies(groupName string, proxies []map[string]any) groups.Group {
	local := localnodes.New()
	for _, p := range proxies {
		name, _ := p["name"].(string)
		typ, _ := p["type"].(string)
		server, _ := p["server"].(string)
		port, _ := p["port"].(int)
		via, _ := p["dialer-proxy"].(string)
		fields := map[string]any{}
		for k, v := range p {
			switch k {
			case "name", "type", "server", "port", "dialer-proxy":
			default:
				fields[k] = v
			}
		}
		_ = local.Add(localnodes.Node{
			Name: name, Proto: typ, Server: server, Port: port,
			Via: via, Fields: fields, Group: groupName,
		})
	}
	return groups.NewLocalNodesGroupForGroup(groupName, local)
}

func TestEmitProxiesRewritesDialerProxyToNamespacedName(t *testing.T) {
	g := localGroupFromProxies("local", []map[string]any{
		{"name": "Hub", "type": "vmess", "server": "h.b", "port": 443},
		{"name": "Hop", "type": "socks5", "server": "p.q", "port": 1080, "dialer-proxy": "Hub"},
	})
	out := emitProxies(nil, []groups.Group{g})
	if len(out) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(out))
	}
	hop := out[1].(map[string]any)
	if hop["name"] != "local:Hop" {
		t.Errorf("hop name = %v, want local:Hop", hop["name"])
	}
	if got := hop["dialer-proxy"]; got != "local:Hub" {
		t.Errorf("dialer-proxy = %v, want local:Hub (auto-namespace)", got)
	}
}

func TestEmitProxiesPreservesDialerProxyWhenItPointsAtGroup(t *testing.T) {
	g := localGroupFromProxies("local", []map[string]any{
		{"name": "Solo", "type": "ss", "server": "s.s", "port": 9, "dialer-proxy": "DIRECT"},
	})
	out := emitProxies(nil, []groups.Group{g})
	if len(out) != 1 {
		t.Fatalf("len %d", len(out))
	}
	if got := out[0].(map[string]any)["dialer-proxy"]; got != "DIRECT" {
		t.Errorf("dialer-proxy = %v, want DIRECT (preserved)", got)
	}
}

func TestEmitProxiesLeavesUnknownDialerProxyUntouched(t *testing.T) {
	g := localGroupFromProxies("local", []map[string]any{
		{"name": "Only", "type": "ss", "server": "s.s", "port": 9, "dialer-proxy": "Ghost"},
	})
	out := emitProxies(nil, []groups.Group{g})
	if got := out[0].(map[string]any)["dialer-proxy"]; got != "Ghost" {
		t.Errorf("dialer-proxy = %v, want Ghost (untouched)", got)
	}
}
