package extensions

import (
	"reflect"
	"testing"
)

func TestApplyChainInjectsDialerProxy(t *testing.T) {
	doc := map[string]any{
		"proxies": []any{
			map[string]any{"name": "🇺🇸 US-1", "type": "ss"},
			map[string]any{"name": "🇯🇵 JP-Relay", "type": "ss"},
		},
		"proxy-groups": []any{},
	}
	ext := Extensions{
		Chains: []Chain{{Node: "🇺🇸 US-1", Via: "🇯🇵 JP-Relay"}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	proxies := doc["proxies"].([]any)
	us1 := proxies[0].(map[string]any)
	if us1["dialer-proxy"] != "🇯🇵 JP-Relay" {
		t.Fatalf("dialer-proxy: want 🇯🇵 JP-Relay, got %v", us1["dialer-proxy"])
	}
	jp := proxies[1].(map[string]any)
	if _, has := jp["dialer-proxy"]; has {
		t.Fatalf("JP-Relay should not have dialer-proxy, got %v", jp["dialer-proxy"])
	}
}

func TestApplyChainMissingNodeIsSkippedNotError(t *testing.T) {
	doc := map[string]any{
		"proxies":      []any{map[string]any{"name": "X", "type": "ss"}},
		"proxy-groups": []any{},
	}
	ext := Extensions{
		Chains: []Chain{{Node: "Y", Via: "X"}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: unexpected error %v", err)
	}
	x := doc["proxies"].([]any)[0].(map[string]any)
	if _, has := x["dialer-proxy"]; has {
		t.Fatalf("X should not have been mutated, got %v", x)
	}
}

func TestApplyAppendsGroups(t *testing.T) {
	doc := map[string]any{
		"proxies": []any{},
		"proxy-groups": []any{
			map[string]any{"name": "🚀 Proxy", "type": "select"},
		},
	}
	ext := Extensions{
		Groups: []Group{
			{Name: "🎯 Stream", Type: "select", Proxies: []string{"DIRECT"}},
		},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	groups := doc["proxy-groups"].([]any)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d: %+v", len(groups), groups)
	}
	last := groups[1].(map[string]any)
	if last["name"] != "🎯 Stream" {
		t.Fatalf("last group name: want 🎯 Stream, got %v", last["name"])
	}
	if last["type"] != "select" {
		t.Fatalf("last group type: want select, got %v", last["type"])
	}
	proxies, _ := last["proxies"].([]any)
	if len(proxies) != 1 || proxies[0] != "DIRECT" {
		t.Fatalf("last group proxies: want [DIRECT], got %v", proxies)
	}
}

func TestApplyOptionalFieldsForUrlTestGroup(t *testing.T) {
	doc := map[string]any{
		"proxies":      []any{},
		"proxy-groups": []any{},
	}
	ext := Extensions{
		Groups: []Group{{
			Name:      "♻️ Auto-US",
			Type:      "url-test",
			Proxies:   []string{"a", "b"},
			URL:       "https://www.gstatic.com/generate_204",
			Interval:  300,
			Tolerance: 50,
		}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := doc["proxy-groups"].([]any)[0].(map[string]any)
	want := map[string]any{
		"name":      "♻️ Auto-US",
		"type":      "url-test",
		"proxies":   []any{"a", "b"},
		"url":       "https://www.gstatic.com/generate_204",
		"interval":  300,
		"tolerance": 50,
	}
	if !reflect.DeepEqual(g, want) {
		t.Fatalf("group:\nwant %+v\n got %+v", want, g)
	}
}

func TestApplyNoProxyGroupsKeyInitializes(t *testing.T) {
	doc := map[string]any{"proxies": []any{}}
	ext := Extensions{
		Groups: []Group{{Name: "X", Type: "select", Proxies: []string{"DIRECT"}}},
	}
	if err := Apply(doc, ext); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, has := doc["proxy-groups"]; !has {
		t.Fatalf("proxy-groups key not created")
	}
}
