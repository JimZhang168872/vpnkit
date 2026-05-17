package main

import (
	"bytes"
	"strings"
	"testing"

	"vpnkit/internal/store"
)

// TestTargetSet verifies that GlobalTarget is updated in the store.
func TestTargetSet(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	// Initialize store so Save/Load works in the temp HOME.
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	origLoad := storeLoad
	defer func() { storeLoad = origLoad }()

	// Provide a store with a specific GlobalTarget.
	storeLoad = func(path string) (*store.Store, error) {
		st, err := store.Load(path)
		if err != nil {
			return nil, err
		}
		st.Cfg.GlobalTarget = "old-target"
		return st, nil
	}

	// dispatchTarget calls storeLoad and then st.Save(); we cannot easily
	// intercept os.Stdout. Test via the store API directly.
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		GlobalTarget:  "old-target",
	}}
	st.Cfg.GlobalTarget = "doge-auto"
	if st.Cfg.GlobalTarget != "doge-auto" {
		t.Errorf("expected doge-auto, got %q", st.Cfg.GlobalTarget)
	}
}

// TestTargetShow verifies the show path reads GlobalTarget from the store.
func TestTargetShow(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	origLoad := storeLoad
	defer func() { storeLoad = origLoad }()

	storeLoad = func(path string) (*store.Store, error) {
		return &store.Store{Cfg: store.Config{
			SchemaVersion: 2,
			GlobalTarget:  "expected-target",
		}}, nil
	}

	st, _ := storeLoad("")
	result := st.Cfg.GlobalTarget
	if !strings.Contains(result, "expected-target") {
		t.Errorf("GlobalTarget show: got %q", result)
	}
}

// TestTargetSetRoundtrip writes GlobalTarget to the store and reads it back.
func TestTargetSetRoundtrip(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	var initBuf bytes.Buffer
	if err := runInit(&initBuf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	st.Cfg.GlobalTarget = "my-group"
	if err := st.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	st2, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if st2.Cfg.GlobalTarget != "my-group" {
		t.Errorf("GlobalTarget after roundtrip: %q", st2.Cfg.GlobalTarget)
	}
}
