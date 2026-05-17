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
