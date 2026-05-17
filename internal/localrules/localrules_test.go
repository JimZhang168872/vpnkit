package localrules

import (
	"testing"
)

func TestRender(t *testing.T) {
	cases := []struct {
		in   Rule
		want string
	}{
		{Rule{"DOMAIN-SUFFIX", "baidu.com", "🎯 Direct"}, "DOMAIN-SUFFIX,baidu.com,🎯 Direct"},
		{Rule{"MATCH", "", "🚀 Proxy"}, "MATCH,🚀 Proxy"}, // MATCH has empty payload
		{Rule{"GEOIP", "CN", "🎯 Direct"}, "GEOIP,CN,🎯 Direct"},
		{Rule{"RULE-SET", "gfw", "🚀 Proxy"}, "RULE-SET,gfw,🚀 Proxy"},
	}
	for _, tc := range cases {
		got := tc.in.Render()
		if got != tc.want {
			t.Errorf("Render(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestManagerCRUDAndReorder(t *testing.T) {
	m := New()
	_ = m.Add(Rule{"DOMAIN-SUFFIX", "a.com", "🎯 Direct"})
	_ = m.Add(Rule{"DOMAIN-SUFFIX", "b.com", "🚀 Proxy"})
	_ = m.Add(Rule{"DOMAIN-SUFFIX", "c.com", "🛑 Reject"})
	if len(m.All()) != 3 {
		t.Errorf("All len: %d", len(m.All()))
	}
	if err := m.Move(0, 2); err != nil {
		t.Fatalf("Move: %v", err)
	}
	all := m.All()
	if all[0].Payload != "b.com" || all[1].Payload != "c.com" || all[2].Payload != "a.com" {
		t.Errorf("after Move(0,2): %+v", all)
	}
	if err := m.Remove(1); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(m.All()) != 2 {
		t.Errorf("len after Remove: %d", len(m.All()))
	}
}

func TestValidateRejectsUnknownType(t *testing.T) {
	m := New()
	err := m.Add(Rule{"BOGUS-TYPE", "x", "y"})
	if err == nil {
		t.Error("expected validation error for unknown type")
	}
}

func TestValidateRejectsEmptyPayload(t *testing.T) {
	err := Validate(Rule{Type: "DOMAIN-SUFFIX", Payload: "", Target: "🎯 Direct"})
	if err == nil {
		t.Error("expected error for empty payload on non-MATCH type")
	}
}

func TestValidateRejectsEmptyTarget(t *testing.T) {
	err := Validate(Rule{Type: "MATCH", Payload: "", Target: ""})
	if err == nil {
		t.Error("expected error for empty target")
	}
}

func TestManagerLoad(t *testing.T) {
	m := New()
	initial := []Rule{
		{"DOMAIN", "example.com", "🎯 Direct"},
		{"MATCH", "", "🚀 Proxy"},
	}
	m.Load(initial)
	all := m.All()
	if len(all) != 2 {
		t.Fatalf("Load: expected 2 rules, got %d", len(all))
	}
	if all[0].Payload != "example.com" || all[1].Type != "MATCH" {
		t.Errorf("Load: unexpected rules: %+v", all)
	}
	// Verify Load gives a copy (mutating initial does not affect Manager).
	initial[0].Payload = "changed.com"
	if m.All()[0].Payload != "example.com" {
		t.Error("Load should copy the slice, not share it")
	}
}

func TestRemoveOutOfRange(t *testing.T) {
	m := New()
	_ = m.Add(Rule{"DOMAIN", "x.com", "🎯 Direct"})
	if err := m.Remove(5); err == nil {
		t.Error("expected out-of-range error")
	}
	if err := m.Remove(-1); err == nil {
		t.Error("expected negative-index error")
	}
}

func TestMoveOutOfRange(t *testing.T) {
	m := New()
	_ = m.Add(Rule{"DOMAIN", "x.com", "🎯 Direct"})
	_ = m.Add(Rule{"DOMAIN", "y.com", "🚀 Proxy"})
	if err := m.Move(0, 5); err == nil {
		t.Error("expected out-of-range error for to")
	}
	if err := m.Move(-1, 0); err == nil {
		t.Error("expected out-of-range error for from")
	}
}
