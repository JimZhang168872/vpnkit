package main

import (
	"bytes"
	"strings"
	"testing"

	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// The SS URI used in Phase 2 parse tests: ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-A
const testSSURI = "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-A"

func TestLocalNodesAddSSURI(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	if err := runLocalNodesAdd(st, testSSURI); err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(st.Cfg.LocalNodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(st.Cfg.LocalNodes))
	}
	n := st.Cfg.LocalNodes[0]
	if n.Name != "HK-A" {
		t.Errorf("name: %q", n.Name)
	}
	if n.Proto != "ss" {
		t.Errorf("proto: %q", n.Proto)
	}
	if n.Server != "1.2.3.4" || n.Port != 8388 {
		t.Errorf("server/port: %q/%d", n.Server, n.Port)
	}
	if n.Fields["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher: %v", n.Fields["cipher"])
	}
}

func TestLocalNodesAddDuplicate(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	if err := runLocalNodesAdd(st, testSSURI); err != nil {
		t.Fatalf("first add: %v", err)
	}
	err := runLocalNodesAdd(st, testSSURI)
	if err == nil {
		t.Error("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists', got: %v", err)
	}
}

func TestLocalNodesAddBadURI(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runLocalNodesAdd(st, "notauri://something")
	if err == nil {
		t.Error("expected error for unsupported scheme")
	}
}

func TestLocalNodesRm(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalNodes: []store.LocalNode{
			{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388},
		},
	}}
	if err := runLocalNodesRm(st, "HK-A"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if len(st.Cfg.LocalNodes) != 0 {
		t.Errorf("not removed: %+v", st.Cfg.LocalNodes)
	}
}

func TestLocalNodesRmNotFound(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runLocalNodesRm(st, "nonexistent")
	if err == nil {
		t.Error("expected error for missing node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found', got: %v", err)
	}
}

func TestLocalNodesEditFields(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalNodes: []store.LocalNode{
			{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388,
				Fields: map[string]any{"password": "old"}},
		},
	}}
	if err := runLocalNodesEdit(st, "HK-A", []string{"password=newpw", "server=5.6.7.8", "port=443"}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	n := st.Cfg.LocalNodes[0]
	if n.Fields["password"] != "newpw" {
		t.Errorf("password not updated: %v", n.Fields["password"])
	}
	if n.Server != "5.6.7.8" {
		t.Errorf("server not updated: %q", n.Server)
	}
	if n.Port != 443 {
		t.Errorf("port not updated: %d", n.Port)
	}
}

func TestLocalNodesEditProto(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalNodes: []store.LocalNode{
			{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388},
		},
	}}
	if err := runLocalNodesEdit(st, "HK-A", []string{"proto=vmess"}); err != nil {
		t.Fatalf("edit proto: %v", err)
	}
	if st.Cfg.LocalNodes[0].Proto != "vmess" {
		t.Errorf("proto not updated: %q", st.Cfg.LocalNodes[0].Proto)
	}
}

func TestLocalNodesEditNotFound(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runLocalNodesEdit(st, "nonexistent", []string{"password=x"})
	if err == nil {
		t.Error("expected error for missing node")
	}
}

func TestLocalNodesEditBadPort(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalNodes: []store.LocalNode{
			{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388},
		},
	}}
	err := runLocalNodesEdit(st, "HK-A", []string{"port=notanumber"})
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestLocalNodesEditInvalidKV(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalNodes: []store.LocalNode{
			{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388},
		},
	}}
	err := runLocalNodesEdit(st, "HK-A", []string{"nokeyequals"})
	if err == nil {
		t.Error("expected error for missing '='")
	}
}

func TestLocalNodesList(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalNodes: []store.LocalNode{
			{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388},
			{Name: "JP-B", Proto: "vmess", Server: "5.6.7.8", Port: 443},
		},
	}}
	var out bytes.Buffer
	if err := runLocalNodesList(&out, st, false); err != nil {
		t.Fatalf("list: %v", err)
	}
	s := out.String()
	for _, want := range []string{"HK-A", "ss", "1.2.3.4", "JP-B", "vmess"} {
		if !strings.Contains(s, want) {
			t.Errorf("list missing %q: %s", want, s)
		}
	}
}

func TestLocalNodesListJSON(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		LocalNodes: []store.LocalNode{
			{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388},
		},
	}}
	var out bytes.Buffer
	if err := runLocalNodesList(&out, st, true); err != nil {
		t.Fatalf("list json: %v", err)
	}
	if !strings.Contains(out.String(), `"HK-A"`) {
		t.Errorf("json missing HK-A: %s", out.String())
	}
}

func TestLocalNodesAddWithGroupAndVia(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	dispatchLocalNodes([]string{
		"add",
		"ss://YWVzLTI1Ni1nY206TXlQYXNzMTIz@1.2.3.4:8388#HK-A",
		"--group=home",
		"--via=doge-auto",
	})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if len(st.Cfg.LocalNodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(st.Cfg.LocalNodes))
	}
	if st.Cfg.LocalNodes[0].Group != "home" {
		t.Errorf("Group: got %q", st.Cfg.LocalNodes[0].Group)
	}
	if st.Cfg.LocalNodes[0].Via != "doge-auto" {
		t.Errorf("Via: got %q", st.Cfg.LocalNodes[0].Via)
	}
}

func TestLocalNodesMv(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	dispatchLocalGroups([]string{"add", "office"})
	dispatchLocalNodes([]string{"add", "ss://YWVzLTI1Ni1nY206TXlQYXNzMTIz@1.2.3.4:8388#HK-A", "--group=home"})
	dispatchLocalNodes([]string{"mv", "HK-A", "office"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if st.Cfg.LocalNodes[0].Group != "office" {
		t.Errorf("mv: Group not updated: %q", st.Cfg.LocalNodes[0].Group)
	}
}

func TestLocalNodesAddFlagFirstOrder(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	// Flag BEFORE URI (the case the original implementation broke).
	dispatchLocalNodes([]string{
		"add",
		"--group=home",
		"--via=doge-auto",
		"ss://YWVzLTI1Ni1nY206TXlQYXNzMTIz@1.2.3.4:8388#HK-A",
	})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if len(st.Cfg.LocalNodes) != 1 || st.Cfg.LocalNodes[0].Group != "home" || st.Cfg.LocalNodes[0].Via != "doge-auto" {
		t.Errorf("flag-first add: %+v", st.Cfg.LocalNodes)
	}
}

// TestLocalNodesMvRejectsUnknownGroup — round-2 QA found that mv to a
// nonexistent group used to silently auto-create it, which let a typo
// birth a phantom group that escaped every reserved-name / cross-
// namespace guard (`mv n1 DIRECT` would create a `DIRECT` local-group).
// Now it must reject explicitly so users `local-groups add` first.
func TestLocalNodesMvRejectsUnknownGroup(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	dispatchLocalNodes([]string{"add", "ss://YWVzLTI1Ni1nY206TXlQYXNzMTIz@1.2.3.4:8388#HK-A", "--group=home"})
	mustPanicWith(t, "does not exist", func() {
		dispatchLocalNodes([]string{"mv", "HK-A", "freshgroup"})
	})
	// Verify node was NOT moved and group was NOT created.
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if st.Cfg.LocalNodes[0].Group != "home" {
		t.Errorf("Group should remain home after failed mv: %+v", st.Cfg.LocalNodes[0])
	}
	for _, g := range st.Cfg.LocalNodeGroups {
		if g.Name == "freshgroup" {
			t.Errorf("freshgroup must NOT be auto-created: %+v", st.Cfg.LocalNodeGroups)
		}
	}
}

func TestLocalNodesRmNamespacedNonExistentFails(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	mustPanicWith(t, "not found", func() {
		dispatchLocalNodes([]string{"rm", "home:NoSuchNode"})
	})
}

func TestLocalNodesNamespacedRefRm(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "home"})
	dispatchLocalGroups([]string{"add", "office"})
	dispatchLocalNodes([]string{"add", "ss://YWVzLTI1Ni1nY206TXlQYXNzMTIz@1.2.3.4:8388#X", "--group=home"})
	dispatchLocalNodes([]string{"add", "ss://YWVzLTI1Ni1nY206TXlQYXNzMTIz@9.9.9.9:8388#X", "--group=office"})
	restoreDie := panicOnDie(t)
	mustPanicWith(t, "ambiguous", func() { dispatchLocalNodes([]string{"rm", "X"}) })
	restoreDie()
	dispatchLocalNodes([]string{"rm", "office:X"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if len(st.Cfg.LocalNodes) != 1 || st.Cfg.LocalNodes[0].Group != "home" {
		t.Errorf("expected only home:X to remain, got %+v", st.Cfg.LocalNodes)
	}
}
