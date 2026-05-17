package main

import (
	"bytes"
	"strings"
	"testing"

	"vpnkit/internal/store"
)

func TestLocalRulesAddValid(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	if err := runLocalRulesAdd(st, "DOMAIN-SUFFIX", "baidu.com", "🎯 Direct"); err != nil {
		t.Fatalf("add valid: %v", err)
	}
	if len(st.Cfg.LocalRules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(st.Cfg.LocalRules))
	}
	r := st.Cfg.LocalRules[0]
	if r.Type != "DOMAIN-SUFFIX" || r.Payload != "baidu.com" || r.Target != "🎯 Direct" {
		t.Errorf("unexpected rule: %+v", r)
	}
}

func TestLocalRulesAddInvalidType(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runLocalRulesAdd(st, "BOGUS-TYPE", "baidu.com", "🎯 Direct")
	if err == nil {
		t.Error("expected error for unknown rule type")
	}
	if !strings.Contains(err.Error(), "unknown rule type") {
		t.Errorf("expected 'unknown rule type' error, got: %v", err)
	}
}

func TestLocalRulesAddEmptyTarget(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runLocalRulesAdd(st, "DOMAIN-SUFFIX", "baidu.com", "")
	if err == nil {
		t.Error("expected error for empty target")
	}
}

func TestLocalRulesRm(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalRules: []store.LocalRule{
			{Type: "DOMAIN-SUFFIX", Payload: "a.com", Target: "🎯 Direct"},
			{Type: "DOMAIN-SUFFIX", Payload: "b.com", Target: "🚀 Proxy"},
		},
	}}
	if err := runLocalRulesRm(st, 0); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if len(st.Cfg.LocalRules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(st.Cfg.LocalRules))
	}
	if st.Cfg.LocalRules[0].Payload != "b.com" {
		t.Errorf("wrong rule remaining: %+v", st.Cfg.LocalRules[0])
	}
}

func TestLocalRulesRmOutOfBounds(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalRules: []store.LocalRule{
			{Type: "DOMAIN-SUFFIX", Payload: "a.com", Target: "🎯 Direct"},
		},
	}}
	if err := runLocalRulesRm(st, 5); err == nil {
		t.Error("expected out-of-bounds error")
	}
	if err := runLocalRulesRm(st, -1); err == nil {
		t.Error("expected negative index error")
	}
}

func TestLocalRulesMove(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalRules: []store.LocalRule{
			{Type: "DOMAIN-SUFFIX", Payload: "a.com", Target: "🎯 Direct"},
			{Type: "DOMAIN-SUFFIX", Payload: "b.com", Target: "🚀 Proxy"},
			{Type: "DOMAIN-SUFFIX", Payload: "c.com", Target: "🛑 Reject"},
		},
	}}
	// Move a.com (0) to position 2.
	if err := runLocalRulesMove(st, 0, 2); err != nil {
		t.Fatalf("move: %v", err)
	}
	rules := st.Cfg.LocalRules
	if rules[0].Payload != "b.com" || rules[1].Payload != "c.com" || rules[2].Payload != "a.com" {
		t.Errorf("after Move(0,2): %+v", rules)
	}
}

func TestLocalRulesMoveNoop(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalRules: []store.LocalRule{
			{Type: "DOMAIN-SUFFIX", Payload: "a.com", Target: "🎯 Direct"},
		},
	}}
	// move to same index is a no-op, not an error.
	if err := runLocalRulesMove(st, 0, 0); err != nil {
		t.Fatalf("move same index: %v", err)
	}
}

func TestLocalRulesMoveOutOfBounds(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalRules: []store.LocalRule{
			{Type: "DOMAIN-SUFFIX", Payload: "a.com", Target: "🎯 Direct"},
		},
	}}
	if err := runLocalRulesMove(st, 0, 5); err == nil {
		t.Error("expected out-of-bounds error")
	}
	if err := runLocalRulesMove(st, 5, 0); err == nil {
		t.Error("expected out-of-bounds error")
	}
}

func TestLocalRulesList(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalRules: []store.LocalRule{
			{Type: "DOMAIN-SUFFIX", Payload: "baidu.com", Target: "🎯 Direct"},
			{Type: "DOMAIN-KEYWORD", Payload: "internal", Target: "🎯 Direct"},
		},
	}}
	var out bytes.Buffer
	if err := runLocalRulesList(&out, st, false); err != nil {
		t.Fatalf("list: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "[0] DOMAIN-SUFFIX,baidu.com,🎯 Direct") {
		t.Errorf("missing rule 0: %s", s)
	}
	if !strings.Contains(s, "[1] DOMAIN-KEYWORD,internal,🎯 Direct") {
		t.Errorf("missing rule 1: %s", s)
	}
}

func TestLocalRulesListJSON(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalRules: []store.LocalRule{
			{Type: "DOMAIN-SUFFIX", Payload: "baidu.com", Target: "🎯 Direct"},
		},
	}}
	var out bytes.Buffer
	if err := runLocalRulesList(&out, st, true); err != nil {
		t.Fatalf("list json: %v", err)
	}
	if !strings.Contains(out.String(), `"baidu.com"`) {
		t.Errorf("json missing baidu.com: %s", out.String())
	}
}
