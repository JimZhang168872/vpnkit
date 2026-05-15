package proxies

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestRendersGroups(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ProxiesSnapshot{
		Groups: map[string]msg.ProxyGroup{
			"GLOBAL": {Name: "GLOBAL", Type: "Selector", Now: "DIRECT", All: []string{"DIRECT", "REJECT"}},
		},
	})
	view := m.View(80, 24)
	if !strings.Contains(view, "GLOBAL") || !strings.Contains(view, "DIRECT") {
		t.Errorf("view:\n%s", view)
	}
}

func TestRendersDelayResults(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ProxiesSnapshot{
		Groups: map[string]msg.ProxyGroup{
			"G": {Name: "G", Type: "Selector", Now: "n1", All: []string{"n1", "n2"}},
		},
	})
	m.ToggleExpand()
	m, _ = m.Update(msg.DelayResults{Group: "G", Results: map[string]int{"n1": 42, "n2": 99}})
	view := m.View(80, 24)
	if !strings.Contains(view, "42") || !strings.Contains(view, "99") {
		t.Errorf("delays not rendered:\n%s", view)
	}
}
