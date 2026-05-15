package rules

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestRulesRenders(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{
		Rules: []msg.RuleEntry{
			{Type: "RULE-SET", Payload: "reject", Proxy: "🛑 Reject"},
			{Type: "MATCH", Payload: "", Proxy: "🚀 Proxy"},
		},
		Providers: []msg.RuleProviderEntry{
			{Name: "reject", Behavior: "Domain", RuleCount: 1234, UpdatedAt: "2026-05-15T20:00:00Z"},
		},
	})
	view := m.View(120, 24)
	for _, want := range []string{"RULE-SET", "reject", "🚀 Proxy", "1234"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in:\n%s", want, view)
		}
	}
}

func TestRulesFilter(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{
		Rules: []msg.RuleEntry{
			{Type: "RULE-SET", Payload: "reject", Proxy: "R"},
			{Type: "RULE-SET", Payload: "google", Proxy: "P"},
		},
	})
	m.SetFilter("google")
	view := m.View(120, 24)
	if !strings.Contains(view, "google") || strings.Contains(view, "reject") {
		t.Errorf("filter broken:\n%s", view)
	}
}
