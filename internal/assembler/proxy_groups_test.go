package assembler

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
		LocalGroups:      []groups.Group{groups.NewLocalNodesGroup("local", local)},
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

// TestAssembleNoSelfReferenceWhenGlobalTargetIsTopGroup regresses the rc.6
// startup-crash bug: store.Cfg.GlobalTarget defaulted to "🚀 Proxy" (the
// name of the top-level Selector itself). The assembler then prepended
// "🚀 Proxy" to topProxies via withTargetFirst, so the emitted config
// had `🚀 Proxy` group referencing itself → mihomo refused to load with:
//   Parse config error: loop is detected in ProxyGroup, please check
//   following ProxyGroups: [🚀 Proxy]
//
// Fix is two-layered: store.Load migrates this away on disk, AND the
// assembler refuses to put "🚀 Proxy" inside its own member list.
func TestAssembleNoSelfReferenceWhenGlobalTargetIsTopGroup(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "HK-A", "type": "ss"}},
	})
	out, err := Assemble(Input{
		Subscriptions:    []groups.Group{sub},
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		GlobalTarget:     "🚀 Proxy", // the bug condition
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	// Parse the emitted yaml and inspect proxy-groups structurally — string
	// matching trips over the same name appearing in `rules:` (MATCH,🚀 Proxy)
	// which is legal.
	var parsed struct {
		ProxyGroups []map[string]any `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	for _, g := range parsed.ProxyGroups {
		if g["name"] != "\U0001F680 Proxy" {
			continue
		}
		members, _ := g["proxies"].([]any)
		for _, m := range members {
			if m == "\U0001F680 Proxy" {
				t.Errorf("'🚀 Proxy' lists itself as a member: %v", members)
			}
		}
		return
	}
	t.Fatalf("'🚀 Proxy' group not found in proxy-groups:\n%s", string(out))
}

// TestWithTargetFirstDropsSelfReference is the unit-level guarantee that
// the helper never emits the top-level group's own name into a member
// list. This is the choke point that protects every Assemble() output.
func TestWithTargetFirstDropsSelfReference(t *testing.T) {
	got := withTargetFirst([]string{"doge-auto", "doge", "DIRECT"}, "\U0001F680 Proxy")
	for _, name := range got {
		if name == "\U0001F680 Proxy" {
			t.Errorf("withTargetFirst leaked '🚀 Proxy' into list: %v", got)
		}
	}
}

func TestTopProxyMembersForFallbacksWhenActiveHasNoNodes(t *testing.T) {
	// Sub registered but never fetched (NodeCount = 0). Without this guard
	// 🚀 Proxy would reference "doge-auto" — a group emitPair skips for
	// the same 0-node reason — and mihomo would 400 on PUT /configs.
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{})
	got := topProxyMembersFor("doge", []groups.Group{sub}, nil)
	if len(got) != 1 || got[0] != "DIRECT" {
		t.Errorf("0-node active source should yield [DIRECT], got %v", got)
	}
}

func TestGlobalTargetExistsRecognizesValidTargets(t *testing.T) {
	sub := groups.NewSubscriptionGroup("doge", true, &subscription.Result{
		Proxies: []proto.Proxy{{"name": "HK-1", "type": "ss", "server": "1.1", "port": 1}},
	})
	cases := []struct {
		target string
		want   bool
	}{
		{"DIRECT", true},
		{"REJECT", true},
		{"🚀 Proxy", true},
		{"🎯 Direct", true},
		{"🛑 Reject", true},
		{"doge", true},
		{"doge-auto", true},
		{"doge:HK-1", true},
		{"sub-doggy-auto", false}, // not enabled / not present — the stale-target case
		{"completely-random", false},
	}
	for _, tc := range cases {
		if got := globalTargetExists(tc.target, []groups.Group{sub}, nil); got != tc.want {
			t.Errorf("globalTargetExists(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}
}

func TestEmitProxyGroupsDropsStaleGlobalTarget(t *testing.T) {
	// Active is "local", but GlobalTarget points at a deleted "sub-doggy-auto".
	// Previously this prepended "sub-doggy-auto" into 🚀 Proxy's members and
	// mihomo PUT /configs 400'd. Guard should now silently drop the stale
	// target so the assembled config is valid.
	localM := localnodes.New()
	_ = localM.Add(localnodes.Node{Name: "Hub", Proto: "vmess", Server: "a.b", Port: 443})
	lg := groups.NewLocalNodesGroupForGroup("local", localM)

	out := emitProxyGroups(nil, []groups.Group{lg}, "local", "sub-doggy-auto")

	var top map[string]any
	for _, g := range out {
		gm := g.(map[string]any)
		if gm["name"] == topLevelProxyGroup {
			top = gm
			break
		}
	}
	if top == nil {
		t.Fatal("🚀 Proxy group not emitted")
	}
	for _, m := range top["proxies"].([]string) {
		if m == "sub-doggy-auto" {
			t.Errorf("🚀 Proxy members still include stale 'sub-doggy-auto': %v", top["proxies"])
		}
	}
}
