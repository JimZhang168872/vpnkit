package groups

import (
	"fmt"
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

// manyNodesDeps returns a single subscription with `count` named nodes
// (n00, n01, ...). Used by the scroll-window tests below.
func manyNodesDeps(count int) Deps {
	nodes := make([]SubNode, count)
	for i := range nodes {
		nodes[i] = SubNode{
			Name:   fmt.Sprintf("n%02d", i),
			Proto:  "anytls",
			Server: "host.example.com",
			Port:   1000 + i,
		}
	}
	return Deps{
		GetSubs: func() []store.Subscription {
			return []store.Subscription{{Name: "boost", Enabled: true, NodeCount: count}}
		},
		GetSubNodes: func(name string) []SubNode {
			if name != "boost" {
				return nil
			}
			return nodes
		},
		GetLocalGroups: func() []store.LocalNodeGroup { return nil },
		GetLocalNodes:  func(group string) []SubNode { return nil },
	}
}

// TestViewScrollsToCursorOnLongList — regression for the
// "<provider> imported 50 nodes but only 22 visible" complaint. With
// height too small to fit every node, the visible window must follow the
// right-pane cursor as it moves down, not hard-truncate at maxRows. We
// assert node 49 (the last one) becomes visible when the cursor lands
// there. As a sanity check, also assert node 0 disappears (otherwise the
// "fix" would be just "render everything" which would clip past the pane
// border).
func TestViewScrollsToCursorOnLongList(t *testing.T) {
	m := New(manyNodesDeps(50))
	m.Refresh()
	m.SetSubFocus(SubFocusRight)
	// Park the right-pane cursor on the last node. Each MoveDown advances
	// the cursor by 1; the model clamps to len-1, so 60 calls is plenty.
	for i := 0; i < 60; i++ {
		m.MoveDown()
	}
	out := m.View(120, 30) // height=30 → roughly 22 visible rows
	if !strings.Contains(out, "n49") {
		t.Errorf("last node n49 should be visible after cursor moves to bottom; output:\n%s", out)
	}
	if strings.Contains(out, "n00") {
		t.Errorf("first node n00 should have scrolled off when cursor is at end; output:\n%s", out)
	}
}

// TestViewShowsScrollIndicatorOnOverflow asserts the user gets a visual
// hint that there's more content past the visible window. Without this,
// people don't realize they can scroll and assume nodes are missing.
func TestViewShowsScrollIndicatorOnOverflow(t *testing.T) {
	m := New(manyNodesDeps(50))
	m.Refresh()
	out := m.View(120, 30)
	// Expect a `1-N/50` viewport.Indicator() suffix on the right-pane
	// header. The bare `50` in "boost (50)" on the left pane is the
	// group node count, not a scroll cue, so require the slash form.
	if !strings.Contains(out, "/50") {
		t.Errorf("scroll indicator should include `/50` total; output:\n%s", out)
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
