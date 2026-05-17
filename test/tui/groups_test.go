package tui

import "testing"

func TestTUIGroupsFocusAndEnter(t *testing.T) {
	iso := newIsolatedHome(t)
	sess := newTUISession(t, iso)
	sess.SendKeys("Left", "Down")
	sess.SendKeys("Right")
	sess.SendKeys("Right")
	sess.MustContain("Groups")
}
