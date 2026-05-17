package assembler

import (
	"strings"
	"testing"

	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

func TestAssembleRejectsZeroPorts(t *testing.T) {
	cases := []Input{
		{ControllerPort: 32645, ControllerSecret: "s"},    // MixedPort=0
		{MixedPort: 50595, ControllerSecret: "s"},         // ControllerPort=0
	}
	for i, in := range cases {
		_, err := Assemble(in)
		if err == nil {
			t.Errorf("case %d: expected error for zero port, got nil", i)
		}
	}
}

func TestAssembleMultiLocalGroupsWithVia(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "JP-1", "type": "vmess", "server": "5.6.7.8", "port": 443}},
	})

	homeMgr := localnodes.New()
	_ = homeMgr.Add(localnodes.Node{
		Name: "HK-manual", Group: "home", Via: "doge:JP-1",
		Proto: "hysteria2", Server: "1.2.3.4", Port: 443,
		Fields: map[string]any{"password": "x", "up": "100 Mbps", "down": "200 Mbps"},
	})
	officeMgr := localnodes.New()
	_ = officeMgr.Add(localnodes.Node{
		Name: "WORK-1", Group: "office", Proto: "trojan",
		Server: "9.9.9.9", Port: 443,
		Fields: map[string]any{"password": "p"},
	})

	homeGroup := groups.NewLocalNodesGroupForGroup("home", homeMgr)
	officeGroup := groups.NewLocalNodesGroupForGroup("office", officeMgr)

	out, err := Assemble(Input{
		Mode:             ModeRule,
		Subscriptions:    []groups.Group{sub},
		LocalGroups:      []groups.Group{homeGroup, officeGroup},
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "s",
		GlobalTarget:     "🚀 Proxy",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)

	for _, want := range []string{
		"name: home:HK-manual",    // namespaced under home
		"name: office:WORK-1",     // namespaced under office
		"dialer-proxy: doge:JP-1", // Via flowed through
		"name: home",              // home select group
		"name: home-auto",         // home url-test group
		"name: office",
		"name: office-auto",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in assembled config:\n%s", want, s)
		}
	}
}

func TestAssembleEmitsBaseConfig(t *testing.T) {
	out, err := Assemble(Input{
		Mode:             ModeRule,
		GlobalTarget:     "🚀 Proxy",
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "secret",
		ProxyUser:        "vpnkit-user",
		ProxyPass:        "pass",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"mixed-port: 50595",
		"external-controller: 127.0.0.1:32645",
		"secret: secret",
		"bind-address: 127.0.0.1",
		"allow-lan: false",
		"mode: rule",
		"vpnkit-",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}
