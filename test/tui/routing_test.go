package tui

import "testing"

func TestTUIRoutingModeRadioPersists(t *testing.T) {
	iso := newIsolatedHome(t)
	sess := newTUISession(t, iso)
	sess.SendKeys("Left")
	for i := 0; i < 6; i++ {
		sess.SendKeys("Down")
	}
	sess.SendKeys("Right")
	sess.SendKeys("Down", "Down", "Down")
	sess.SendKeys("Right")
	sess.SendKeys("Down")
	sess.SendKeys("Enter")
	st := loadStoreTOML(t, iso)
	if st["mode"] != "global" {
		t.Errorf("mode not persisted: %v", st["mode"])
	}
}
