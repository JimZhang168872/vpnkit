package groups

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
	"vpnkit/internal/store"
)

// fakeDeps returns deps with one subscription "doge" carrying two named
// nodes. Suitable for exercising display + delay overlay logic.
func fakeDeps() Deps {
	return Deps{
		GetSubs: func() []store.Subscription {
			return []store.Subscription{{Name: "doge", Enabled: true, NodeCount: 2}}
		},
		GetSubNodes: func(name string) []SubNode {
			if name != "doge" {
				return nil
			}
			return []SubNode{
				{Name: "HK-01", Proto: "vmess", Server: "hk.example.com", Port: 443},
				{Name: "JP-02", Proto: "trojan", Server: "jp.example.com", Port: 443},
			}
		},
		GetLocalGroups: func() []store.LocalNodeGroup { return nil },
		GetLocalNodes:  func(group string) []SubNode { return nil },
	}
}

// TestDelayResultsPopulatesPerNodeMap verifies the Update handler stores
// incoming delay measurements keyed by namespaced node name.
func TestDelayResultsPopulatesPerNodeMap(t *testing.T) {
	m := New(fakeDeps())
	m.Refresh()
	m.SetSubFocus(SubFocusRight) // cursor lands on first node of "doge"

	m, _ = m.Update(msg.DelayResults{
		Group:   "doge",
		Results: map[string]int{"doge:HK-01": 234, "doge:JP-02": 0},
	})

	got := m.DelayByNode()
	if got["doge:HK-01"] != 234 {
		t.Errorf("HK-01 delay = %d, want 234", got["doge:HK-01"])
	}
	if _, ok := got["doge:JP-02"]; !ok {
		t.Errorf("JP-02 missing from delayByNode: %+v", got)
	}
	if got["doge:JP-02"] != 0 {
		t.Errorf("JP-02 should keep 0 (timeout signal), got %d", got["doge:JP-02"])
	}
}

// TestViewRendersDelayBesideNode checks the right pane shows the measured
// delay next to each node row after a DelayResults message arrives.
func TestViewRendersDelayBesideNode(t *testing.T) {
	m := New(fakeDeps())
	m.Refresh()
	m.SetSubFocus(SubFocusRight)
	m, _ = m.Update(msg.DelayResults{
		Group:   "doge",
		Results: map[string]int{"doge:HK-01": 234, "doge:JP-02": 0},
	})

	view := m.View(120, 24)
	if !strings.Contains(view, "234 ms") {
		t.Errorf("view missing '234 ms':\n%s", view)
	}
	if !strings.Contains(view, "timeout") {
		t.Errorf("view missing 'timeout' for zero-delay node:\n%s", view)
	}
}

// TestSelectedGroupExposesCurrent gives the app-level test helper access to
// the highlighted group so it can fire the delay test against the right one.
func TestSelectedGroupExposesCurrent(t *testing.T) {
	m := New(fakeDeps())
	m.Refresh()
	if g := m.SelectedGroup(); g != "doge" {
		t.Errorf("SelectedGroup = %q, want doge", g)
	}
}
