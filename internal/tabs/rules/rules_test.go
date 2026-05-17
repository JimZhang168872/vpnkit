package rules

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// TestRulesScrollViewport asserts a long rule list scrolls so the cursor
// stays visible — without scrolling, viewport.Window(_, 0, _) only ever
// shows the first page and rows past it are invisible.
func TestRulesScrollViewport(t *testing.T) {
	rules := make([]msg.RuleEntry, 0, 100)
	for i := 0; i < 100; i++ {
		rules = append(rules, msg.RuleEntry{Type: "DOMAIN", Payload: "rule" + itoa(i) + ".test", Proxy: "P"})
	}
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{Rules: rules})
	// Move cursor down 50 times.
	for i := 0; i < 50; i++ {
		m.MoveDown()
	}
	view := m.View(120, 30)
	// rule50 (the new cursor position) should be visible; rule0 should NOT.
	if !strings.Contains(view, "rule50.test") {
		t.Errorf("cursor row rule50 not visible:\n%s", view)
	}
	if strings.Contains(view, "rule0.test") {
		t.Errorf("rule0 should have scrolled off-screen, view:\n%s", view)
	}
}

// TestRulesPageNavigationJumpsBy10 covers the PgUp/PgDn paging feature.
func TestRulesPageNavigationJumpsBy10(t *testing.T) {
	rules := make([]msg.RuleEntry, 0, 50)
	for i := 0; i < 50; i++ {
		rules = append(rules, msg.RuleEntry{Type: "DOMAIN", Payload: "x", Proxy: "P"})
	}
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{Rules: rules})
	m.MovePageDown()
	if m.cursor != PageSize {
		t.Errorf("after MovePageDown: cursor = %d, want %d", m.cursor, PageSize)
	}
	m.MovePageDown()
	if m.cursor != 2*PageSize {
		t.Errorf("after 2nd MovePageDown: cursor = %d, want %d", m.cursor, 2*PageSize)
	}
	// Spam PageDown — clamps at last (49).
	for i := 0; i < 10; i++ {
		m.MovePageDown()
	}
	if m.cursor != 49 {
		t.Errorf("PageDown spam clamp: cursor = %d, want 49", m.cursor)
	}
	m.MovePageUp()
	if m.cursor != 49-PageSize {
		t.Errorf("PageUp: cursor = %d, want %d", m.cursor, 49-PageSize)
	}
	for i := 0; i < 20; i++ {
		m.MovePageUp()
	}
	if m.cursor != 0 {
		t.Errorf("PageUp clamp: cursor = %d, want 0", m.cursor)
	}
}

// TestRulesMoveCursorBounds ensures MoveUp/Down don't run past list edges.
func TestRulesMoveCursorBounds(t *testing.T) {
	rules := []msg.RuleEntry{
		{Type: "DOMAIN", Payload: "a", Proxy: "P"},
		{Type: "DOMAIN", Payload: "b", Proxy: "P"},
	}
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{Rules: rules})
	m.MoveUp() // already at 0
	if m.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.cursor)
	}
	for i := 0; i < 10; i++ {
		m.MoveDown()
	}
	if m.cursor != 1 {
		t.Errorf("cursor should clamp at last (1), got %d", m.cursor)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func TestRulesFilterInput(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.RulesSnapshot{Rules: []msg.RuleEntry{
		{Type: "RULE-SET", Payload: "google", Proxy: "P"},
		{Type: "RULE-SET", Payload: "reject", Proxy: "R"},
	}})
	_ = m.StartFilter()
	for _, r := range "goog" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	view := m.View(120, 24)
	if !strings.Contains(view, "google") || strings.Contains(view, "reject") {
		t.Errorf("filter not applied:\n%s", view)
	}
}
