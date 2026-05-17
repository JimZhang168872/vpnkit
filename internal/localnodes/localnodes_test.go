package localnodes

import (
	"testing"
)

func TestManagerCRUD(t *testing.T) {
	m := New()
	n := Node{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 443, Fields: map[string]any{"password": "x"}}
	if err := m.Add(n); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := m.Add(n); err == nil {
		t.Error("expected duplicate-name error")
	}
	got, ok := m.Get("HK-A")
	if !ok {
		t.Fatal("Get HK-A: not found")
	}
	if got.Server != "1.2.3.4" {
		t.Errorf("server: got %q", got.Server)
	}
	all := m.All()
	if len(all) != 1 {
		t.Errorf("All len: %d", len(all))
	}
	if err := m.Remove("HK-A"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := m.Get("HK-A"); ok {
		t.Error("Get after Remove: still present")
	}
}

func TestManagerAddEmptyName(t *testing.T) {
	m := New()
	if err := m.Add(Node{}); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestManagerRemoveNotFound(t *testing.T) {
	m := New()
	if err := m.Remove("nonexistent"); err == nil {
		t.Error("expected error removing nonexistent node")
	}
}

func TestManagerLoad(t *testing.T) {
	m := New()
	initial := []Node{
		{Name: "A", Proto: "ss", Server: "1.1.1.1", Port: 80},
		{Name: "B", Proto: "vmess", Server: "2.2.2.2", Port: 443},
	}
	m.Load(initial)
	all := m.All()
	if len(all) != 2 {
		t.Fatalf("after Load: got %d nodes, want 2", len(all))
	}
	if all[0].Name != "A" || all[1].Name != "B" {
		t.Errorf("order after Load: %v", all)
	}
}

func TestManagerUpdate(t *testing.T) {
	m := New()
	_ = m.Add(Node{Name: "X", Proto: "trojan", Server: "3.3.3.3", Port: 443})
	if err := m.Update("X", func(n *Node) error {
		n.Server = "4.4.4.4"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := m.Get("X")
	if got.Server != "4.4.4.4" {
		t.Errorf("after Update: server = %q", got.Server)
	}
}

func TestManagerUpdateNotFound(t *testing.T) {
	m := New()
	if err := m.Update("missing", func(n *Node) error { return nil }); err == nil {
		t.Error("expected error updating nonexistent node")
	}
}
