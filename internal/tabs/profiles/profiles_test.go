package profiles

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
	"vpnkit/internal/profiles"
)

func TestRendersProfiles(t *testing.T) {
	m := New(profiles.New(profiles.Config{}))
	m.SetProfiles([]profiles.Profile{
		{Name: "main", URL: "https://example.com/sub", NodeCount: 3},
	}, "main")
	view := m.View(80, 24)
	if !strings.Contains(view, "main") || !strings.Contains(view, "⭐") {
		t.Errorf("missing label or active marker:\n%s", view)
	}
}

func TestSelectionAdvances(t *testing.T) {
	mgr := profiles.New(profiles.Config{})
	_ = mgr.Add(profiles.Profile{Name: "a"})
	_ = mgr.Add(profiles.Profile{Name: "b"})
	m := New(mgr)
	m.SetProfiles(mgr.All(), "")
	if m.Selected().Name != "a" {
		t.Errorf("first: %v", m.Selected())
	}
	m, _ = m.Update(msg.ProfileUpdated{Name: "ignored"})
	m.MoveDown()
	if m.Selected().Name != "b" {
		t.Errorf("after down: %v", m.Selected())
	}
}
