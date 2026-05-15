package logs

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestLogsAppend(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.LogLine{Text: "starting mihomo"})
	m, _ = m.Update(msg.LogLine{Text: "listening 7890"})
	view := m.View(120, 24)
	if !strings.Contains(view, "starting") || !strings.Contains(view, "7890") {
		t.Errorf("view:\n%s", view)
	}
}

func TestLogsRingBound(t *testing.T) {
	m := New()
	for i := 0; i < 1500; i++ {
		m, _ = m.Update(msg.LogLine{Text: "x"})
	}
	if got := len(m.Lines()); got != 1000 {
		t.Errorf("expected ring of 1000, got %d", got)
	}
}
