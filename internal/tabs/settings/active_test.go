package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/store"
)

// stubPipeline implements PipelineFace for active-source tests without
// pulling in the heavy app.Pipeline. SetActiveSource records the last
// call so tests can assert "Enter actually fired the swap".
type stubPipeline struct {
	current    string
	setCalls   []string
	setReturns error
}

func (s *stubPipeline) RefreshSubscription(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (s *stubPipeline) Assemble() error  { return nil }
func (s *stubPipeline) SaveLocal() error { return nil }
func (s *stubPipeline) ActiveSource() string {
	return s.current
}
func (s *stubPipeline) SetActiveSource(name string) error {
	s.setCalls = append(s.setCalls, name)
	if s.setReturns != nil {
		return s.setReturns
	}
	s.current = name
	return nil
}

// activeTestStore returns a store seeded with two enabled subs + one
// enabled local group + one DISABLED sub. The disabled one must be
// hidden from the picker so the user can't accidentally activate a
// source whose nodes aren't even in mihomo.
func activeTestStore(active string) *store.Store {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		ActiveSource:  active,
		Subscriptions: []store.Subscription{
			{Name: "doge", Enabled: true},
			{Name: "boost", Enabled: true},
			{Name: "old-sub", Enabled: false},
		},
		LocalNodeGroups: []store.LocalNodeGroup{
			{Name: "Local", Enabled: true},
		},
	}}
	return st
}

func TestActiveViewShowsEnabledSources(t *testing.T) {
	st := activeTestStore("doge")
	pl := &stubPipeline{current: "doge"}
	m := newActive(st, pl, nil)
	out := m.View(80, 24)
	for _, want := range []string{"doge", "boost", "Local"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in view:\n%s", want, out)
		}
	}
	if strings.Contains(out, "old-sub") {
		t.Errorf("disabled source must NOT appear:\n%s", out)
	}
	if !strings.Contains(out, "(subscription)") || !strings.Contains(out, "(local)") {
		t.Errorf("kind labels missing:\n%s", out)
	}
}

func TestActiveViewMarksCurrentSelection(t *testing.T) {
	st := activeTestStore("boost")
	pl := &stubPipeline{current: "boost"}
	m := newActive(st, pl, nil)
	out := m.View(80, 24)
	// "[x] boost" must appear; "[x] doge" must not.
	dogeLine := findFirstLine(out, "doge")
	boostLine := findFirstLine(out, "boost")
	if strings.Contains(dogeLine, "[x]") {
		t.Errorf("doge should NOT be marked [x]:\n%s", dogeLine)
	}
	if !strings.Contains(boostLine, "[x]") {
		t.Errorf("boost should be marked [x]:\n%s", boostLine)
	}
}

func TestActiveEnterCallsSetActiveSource(t *testing.T) {
	st := activeTestStore("doge")
	pl := &stubPipeline{current: "doge"}
	called := false
	m := newActive(st, pl, func() error { called = true; return nil })
	// Down once → cursor on boost (doge=0, boost=1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(pl.setCalls) != 1 || pl.setCalls[0] != "boost" {
		t.Errorf("Enter should call SetActiveSource(\"boost\"); got %v", pl.setCalls)
	}
	if !called {
		t.Error("Enter should invoke applyFunc to push the new config to mihomo")
	}
}

func TestActiveSetErrorIsSurfacedInFlash(t *testing.T) {
	st := activeTestStore("doge")
	pl := &stubPipeline{current: "doge", setReturns: errors.New("nope")}
	m := newActive(st, pl, nil)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	out := m.View(80, 24)
	if !strings.Contains(out, "nope") {
		t.Errorf("SetActiveSource error should appear in flash:\n%s", out)
	}
}

// TestActiveSetSurfacesApplyError — store update can succeed while
// mihomo reload fails (mihomo down, file write race, etc.). The flash
// must reflect that mismatch so the user knows their view of "active is
// boost now" doesn't match what mihomo is actually routing through.
// Otherwise it's silent split-brain: store says boost, mihomo still
// routes via the previous active source.
func TestActiveSetSurfacesApplyError(t *testing.T) {
	st := activeTestStore("doge")
	pl := &stubPipeline{current: "doge"}
	apply := func() error { return errors.New("mihomo down") }
	m := newActive(st, pl, apply)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // cursor → boost
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	out := m.View(80, 24)
	if !strings.Contains(out, "mihomo down") {
		t.Errorf("applyFunc error should appear in flash:\n%s", out)
	}
	if strings.Contains(out, "✅ active → boost") {
		t.Errorf("flash must NOT report success when apply failed:\n%s", out)
	}
}

// TestActiveDoesNotPanicWithoutStore — defensive: tests / harnesses
// construct Settings with Deps{} which leaves Store nil. The sub-page
// must show a placeholder rather than crash.
func TestActiveDoesNotPanicWithoutStore(t *testing.T) {
	m := newActive(nil, nil, nil)
	out := m.View(80, 24)
	if !strings.Contains(out, "not available") {
		t.Errorf("nil-store fallback message missing:\n%s", out)
	}
}

func findFirstLine(out, needle string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
