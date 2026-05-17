package main

import (
	"bytes"
	"strings"
	"testing"

	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

func TestLocalGroupsAddList(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	dispatchLocalGroups([]string{"add", "office"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if len(st.Cfg.LocalNodeGroups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(st.Cfg.LocalNodeGroups))
	}
	if st.Cfg.LocalNodeGroups[0].Name != "home" || st.Cfg.LocalNodeGroups[1].Name != "office" {
		t.Errorf("group names: %+v", st.Cfg.LocalNodeGroups)
	}
}

func TestLocalGroupsAddDuplicateRejected(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	mustPanicWith(t, "die", func() { dispatchLocalGroups([]string{"add", "home"}) })
}

func TestLocalGroupsDisableEnable(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	dispatchLocalGroups([]string{"disable", "home"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if st.Cfg.LocalNodeGroups[0].Enabled {
		t.Error("disable did not flip flag")
	}
	dispatchLocalGroups([]string{"enable", "home"})
	st, _ = store.Load(paths.Resolve().VpnkitConfigFile())
	if !st.Cfg.LocalNodeGroups[0].Enabled {
		t.Error("enable did not flip flag")
	}
}

func TestLocalGroupsRename(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "old"})
	dispatchLocalGroups([]string{"rename", "old", "new"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if st.Cfg.LocalNodeGroups[0].Name != "new" {
		t.Errorf("rename not applied: %+v", st.Cfg.LocalNodeGroups)
	}
}

func TestLocalGroupsListContainsName(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	var out bytes.Buffer
	runLocalGroupsList(&out, st, false)
	if !strings.Contains(out.String(), "home") {
		t.Errorf("list output missing 'home': %q", out.String())
	}
}
