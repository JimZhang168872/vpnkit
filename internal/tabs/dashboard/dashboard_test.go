package dashboard

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestDashboardRendersTrafficText(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.Traffic{Up: 1024, Down: 2048})
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
		m, _ = m.Update(msg.Traffic{Up: i, Down: i})
	}
	if got := len(m.UpHistory()); got != 60 {
		t.Errorf("expected ring of 60, got %d", got)
	}
}

// TestDashboardServiceStatusRunning is the regression for Bug G:
// dashboard rendered "○ stopped" forever because no goroutine ever sent
// msg.ServiceStatus. The fix wires a pollServiceStatus loop in app/run.go;
// here we cover the model side — given the message, the view changes.
func TestDashboardServiceStatusRunning(t *testing.T) {
	m := New()
	view := m.View(80, 24)
	if !strings.Contains(view, "○ stopped") {
		t.Fatalf("default state should be stopped, got:\n%s", view)
	}
	m, _ = m.Update(msg.ServiceStatus{Running: true, PID: 12345})
	view = m.View(80, 24)
	if !strings.Contains(view, "● running") {
		t.Fatalf("after ServiceStatus{Running:true}, view should show running:\n%s", view)
	}
	if strings.Contains(view, "○ stopped") {
		t.Fatalf("after ServiceStatus{Running:true}, view should no longer show stopped:\n%s", view)
	}
}
