package proxies

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func mkSnap() msg.ProxiesSnapshot {
	return msg.ProxiesSnapshot{
		Groups: map[string]msg.ProxyGroup{
			"A": {Name: "A", Type: "Selector", Now: "a1", All: []string{"a1", "a2", "a3"}},
			"B": {Name: "B", Type: "Selector", Now: "b1", All: []string{"b1", "b2"}},
		},
	}
}

// loadSnap returns a model with snapshot applied, cursor at (0, -1).
func loadSnap(t *testing.T) Model {
	t.Helper()
	m := New()
	m, _ = m.Update(mkSnap())
	if len(m.order) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(m.order))
	}
	return m
}

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

func TestMoveDownCollapsed(t *testing.T) {
	m := loadSnap(t)
	// (0,-1) → (1,-1) since group 0 collapsed
	m.MoveDown()
	if m.cursor.groupIdx != 1 || m.cursor.nodeIdx != -1 {
		t.Errorf("got %+v, want {1,-1}", m.cursor)
	}
	// already on last → stays
	m.MoveDown()
	if m.cursor.groupIdx != 1 || m.cursor.nodeIdx != -1 {
		t.Errorf("clamp failed: %+v", m.cursor)
	}
}

func TestMoveDownIntoExpandedGroup(t *testing.T) {
	m := loadSnap(t)
	m.ToggleExpand() // expand A
	m.MoveDown()
	// (0,-1) → (0,0) — first node of A
	if m.cursor.groupIdx != 0 || m.cursor.nodeIdx != 0 {
		t.Errorf("did not descend into expanded group: %+v", m.cursor)
	}
	m.MoveDown()
	if m.cursor.nodeIdx != 1 {
		t.Errorf("next node: %+v", m.cursor)
	}
	m.MoveDown()
	if m.cursor.nodeIdx != 2 {
		t.Errorf("last node: %+v", m.cursor)
	}
	m.MoveDown()
	// past last node of A → next group B
	if m.cursor.groupIdx != 1 || m.cursor.nodeIdx != -1 {
		t.Errorf("exit to next group: %+v", m.cursor)
	}
}

func TestMoveUpFromNode(t *testing.T) {
	m := loadSnap(t)
	m.ToggleExpand()
	m.MoveDown() // (0,0)
	m.MoveDown() // (0,1)
	m.MoveUp()
	if m.cursor.groupIdx != 0 || m.cursor.nodeIdx != 0 {
		t.Errorf("got %+v want {0,0}", m.cursor)
	}
	m.MoveUp()
	if m.cursor.groupIdx != 0 || m.cursor.nodeIdx != -1 {
		t.Errorf("back to group row: %+v", m.cursor)
	}
	m.MoveUp() // clamp at top
	if m.cursor.groupIdx != 0 || m.cursor.nodeIdx != -1 {
		t.Errorf("top clamp: %+v", m.cursor)
	}
}

func TestMoveUpFromCollapsedNextGroupGoesToPrevGroupLastNodeIfExpanded(t *testing.T) {
	m := loadSnap(t)
	m.ToggleExpand()       // expand A
	m.cursor = cursorPos{1, -1} // simulate cursor on group B
	m.MoveUp()
	// should land on A's last node (index 2)
	if m.cursor.groupIdx != 0 || m.cursor.nodeIdx != 2 {
		t.Errorf("got %+v want {0,2}", m.cursor)
	}
}

func TestSelectedNode(t *testing.T) {
	m := loadSnap(t)
	m.ToggleExpand()
	m.MoveDown()
	m.MoveDown() // (0,1) = a2
	grp, node, ok := m.SelectedNode()
	if !ok || grp != "A" || node != "a2" {
		t.Errorf("got (%s,%s,%v) want (A,a2,true)", grp, node, ok)
	}
	// on group row → not ok
	m.cursor = cursorPos{0, -1}
	if _, _, ok := m.SelectedNode(); ok {
		t.Error("expected ok=false on group row")
	}
}

func TestEmptyModelHelpersDoNotPanic(t *testing.T) {
	m := New()
	if m.SelectedGroup() != "" {
		t.Errorf("SelectedGroup on empty model should be empty")
	}
	if _, _, ok := m.SelectedNode(); ok {
		t.Errorf("SelectedNode on empty model should not be ok")
	}
	m.MoveDown() // must not panic
	m.MoveUp()
	m.ToggleExpand()
}

func TestViewHighlightsNodeRow(t *testing.T) {
	m := loadSnap(t)
	m.ToggleExpand()
	m.MoveDown() // cursor on a1
	view := m.View(120, 24)
	// the node row should carry the cursor marker (▶ or 👉) before "a1"
	// we check that the line containing "a1" has a non-space leading marker
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "a1") {
			trimmed := strings.TrimLeft(line, " ")
			if !strings.HasPrefix(trimmed, "▶") && !strings.HasPrefix(trimmed, "👉") {
				continue // try next match
			}
			return // found
		}
	}
	t.Errorf("no highlighted node row found:\n%s", view)
}
