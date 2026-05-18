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
