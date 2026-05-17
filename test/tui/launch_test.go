package tui

import "testing"

func TestTUILaunchesWith7Tabs(t *testing.T) {
	iso := newIsolatedHome(t)
	sess := newTUISession(t, iso)
	for _, want := range []string{
		"🏠 Dashboard",
		"🌐 Groups",
		"📚 Sources",
		"📜 Rules",
		"🔗 Connections",
		"📓 Logs",
		"⚙",
	} {
		sess.MustContain(want)
	}
}
