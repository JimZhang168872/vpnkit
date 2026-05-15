package dashboard

import (
	"strings"
	"testing"

	"vpnkit/internal/app"
)

func TestDashboardRendersTrafficText(t *testing.T) {
	m := New()
	m, _ = m.Update(app.TrafficMsg{Up: 1024, Down: 2048})
	view := m.View(80, 24)
	for _, want := range []string{"↑", "↓", "Mihomo"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in view:\n%s", want, view)
		}
	}
}

func TestDashboardKeepsSparklineHistory(t *testing.T) {
	m := New()
	for i := int64(0); i < 100; i++ {
		m, _ = m.Update(app.TrafficMsg{Up: i, Down: i})
	}
	if got := len(m.UpHistory()); got != 60 {
		t.Errorf("expected ring of 60, got %d", got)
	}
}
