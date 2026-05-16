package extensions

import (
	"strings"
	"testing"
)

func TestValidateShapeAcceptsValidExt(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{{Node: "A", Via: "B"}},
		Groups: []Group{
			{Name: "G", Type: "select", Proxies: []string{"DIRECT"}},
		},
	}
	if err := Validate(ext); err != nil {
		t.Fatalf("Validate: unexpected error %v", err)
	}
}

func TestValidateRejectsEmptyChainNode(t *testing.T) {
	ext := Extensions{Chains: []Chain{{Node: "", Via: "B"}}}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "chain.node empty") {
		t.Fatalf("want 'chain.node empty' error, got %v", err)
	}
}

func TestValidateRejectsEmptyChainVia(t *testing.T) {
	ext := Extensions{Chains: []Chain{{Node: "A", Via: ""}}}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "chain.via empty") {
		t.Fatalf("want 'chain.via empty' error, got %v", err)
	}
}

func TestValidateRejectsSelfChain(t *testing.T) {
	ext := Extensions{Chains: []Chain{{Node: "A", Via: "A"}}}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestValidateRejectsTwoNodeCycle(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "B", Via: "A"},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestValidateRejectsThreeNodeCycle(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "B", Via: "C"},
			{Node: "C", Via: "A"},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestValidateAcceptsLinearChain(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "B", Via: "C"},
		},
	}
	if err := Validate(ext); err != nil {
		t.Fatalf("linear chain should be accepted: %v", err)
	}
}

func TestValidateRejectsDuplicateChainNode(t *testing.T) {
	ext := Extensions{
		Chains: []Chain{
			{Node: "A", Via: "B"},
			{Node: "A", Via: "C"},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate error, got %v", err)
	}
}

func TestValidateRejectsUnknownGroupType(t *testing.T) {
	ext := Extensions{
		Groups: []Group{{Name: "X", Type: "weird", Proxies: []string{"a"}}},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("want type error, got %v", err)
	}
}

func TestValidateAcceptsAllSupportedGroupTypes(t *testing.T) {
	for _, typ := range []string{"select", "url-test", "fallback", "load-balance", "relay"} {
		ext := Extensions{
			Groups: []Group{{Name: "X", Type: typ, Proxies: []string{"a"}}},
		}
		if err := Validate(ext); err != nil {
			t.Fatalf("type %q rejected: %v", typ, err)
		}
	}
}

func TestValidateRejectsEmptyGroupName(t *testing.T) {
	ext := Extensions{
		Groups: []Group{{Name: "", Type: "select", Proxies: []string{"a"}}},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("want name error, got %v", err)
	}
}

func TestValidateRejectsDuplicateGroupName(t *testing.T) {
	ext := Extensions{
		Groups: []Group{
			{Name: "G", Type: "select", Proxies: []string{"a"}},
			{Name: "G", Type: "select", Proxies: []string{"b"}},
		},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate error, got %v", err)
	}
}

func TestValidateRejectsEmptyGroupProxies(t *testing.T) {
	ext := Extensions{
		Groups: []Group{{Name: "G", Type: "select", Proxies: nil}},
	}
	err := Validate(ext)
	if err == nil || !strings.Contains(err.Error(), "proxies") {
		t.Fatalf("want proxies error, got %v", err)
	}
}
