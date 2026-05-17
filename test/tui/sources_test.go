package tui

import (
	"testing"
	"time"
)

func TestTUISourcesSubFormDigitsNotHijacked(t *testing.T) {
	iso := newIsolatedHome(t)
	sess := newTUISession(t, iso)
	sess.SendKeys("Left", "Down", "Down", "Right")
	sess.SendKeys("a")
	sess.WaitFor("Add Subscription", 3*time.Second)
	sess.SendLiteral("test-airport")
	sess.SendKeys("Tab")
	sess.SendLiteral("https://example.com:8443/sub?token=12345&user=2")
	sess.MustContain("Add Subscription")
}
