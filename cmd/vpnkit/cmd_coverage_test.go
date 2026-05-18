// cmd_coverage_test.go adds targeted tests for functions that have low or
// zero coverage, to help the package meet the ≥80% threshold.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/paths"
	"vpnkit/internal/store"
	"vpnkit/internal/updater"
)

// panicOnDie overrides dieUserErr and dieRuntime to panic with a sentinel so
// dispatch-level tests can catch exit paths without actually exiting.
// Returns a restore function that must be deferred.
func panicOnDie(t *testing.T) (restore func()) {
	t.Helper()
	origUser := dieUserErr
	origRuntime := dieRuntime
	dieUserErr = func(format string, args ...any) {
		panic(fmt.Sprintf("dieUserErr: "+format, args...))
	}
	dieRuntime = func(format string, args ...any) {
		panic(fmt.Sprintf("dieRuntime: "+format, args...))
	}
	return func() {
		dieUserErr = origUser
		dieRuntime = origRuntime
	}
}

// --- printPlan tests ---

func TestPrintPlanBothNeedUpdate(t *testing.T) {
	var buf bytes.Buffer
	printPlan(&buf, updater.Info{
		VpnkitNeedsUpdate: true,
		VpnkitCurrent:     "v0.9.0",
		VpnkitLatest:      "v1.0.0",
		MihomoNeedsUpdate: true,
		MihomoCurrent:     "v1.18.0",
		MihomoLatest:      "v1.19.0",
	})
	s := buf.String()
	if !strings.Contains(s, "vpnkit") || !strings.Contains(s, "mihomo") {
		t.Errorf("both update lines missing: %s", s)
	}
}

func TestPrintPlanVpnkitOnly(t *testing.T) {
	var buf bytes.Buffer
	printPlan(&buf, updater.Info{
		VpnkitNeedsUpdate: true,
		VpnkitCurrent:     "v0.9.0",
		VpnkitLatest:      "v1.0.0",
		MihomoNeedsUpdate: false,
		MihomoCurrent:     "v1.18.0",
	})
	s := buf.String()
	if !strings.Contains(s, "vpnkit") {
		t.Errorf("vpnkit update line missing: %s", s)
	}
	if !strings.Contains(s, "already at") {
		t.Errorf("mihomo already-at line missing: %s", s)
	}
}

func TestPrintPlanMihomoOnly(t *testing.T) {
	var buf bytes.Buffer
	printPlan(&buf, updater.Info{
		VpnkitNeedsUpdate: false,
		VpnkitCurrent:     "v1.0.0",
		MihomoNeedsUpdate: true,
		MihomoCurrent:     "v1.18.0",
		MihomoLatest:      "v1.19.0",
	})
	s := buf.String()
	if !strings.Contains(s, "mihomo") {
		t.Errorf("mihomo update line missing: %s", s)
	}
	if !strings.Contains(s, "already at") {
		t.Errorf("vpnkit already-at line missing: %s", s)
	}
}

func TestPrintPlanAlreadyUpToDate(t *testing.T) {
	var buf bytes.Buffer
	printPlan(&buf, updater.Info{
		VpnkitNeedsUpdate: false,
		MihomoNeedsUpdate: false,
	})
	s := buf.String()
	if !strings.Contains(s, "already up to date") {
		t.Errorf("expected up-to-date message: %s", s)
	}
}

func TestPrintPlanVpnkitOnlyNoMihomoVersion(t *testing.T) {
	var buf bytes.Buffer
	printPlan(&buf, updater.Info{
		VpnkitNeedsUpdate: true,
		VpnkitCurrent:     "v0.9.0",
		VpnkitLatest:      "v1.0.0",
		MihomoNeedsUpdate: false,
		MihomoCurrent:     "", // no mihomo installed
	})
	// should not crash and should show vpnkit update
	s := buf.String()
	if !strings.Contains(s, "vpnkit") {
		t.Errorf("vpnkit line missing: %s", s)
	}
}

func TestPrintPlanMihomoOnlyNoVpnkitVersion(t *testing.T) {
	var buf bytes.Buffer
	printPlan(&buf, updater.Info{
		VpnkitNeedsUpdate: false,
		VpnkitCurrent:     "", // no version
		MihomoNeedsUpdate: true,
		MihomoCurrent:     "v1.18.0",
		MihomoLatest:      "v1.19.0",
	})
	s := buf.String()
	if !strings.Contains(s, "mihomo") {
		t.Errorf("mihomo line missing: %s", s)
	}
}

// --- parseMihomoLine tests ---

func TestParseMihomoLine(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Mihomo Meta v1.18.7 linux amd64\n...", "v1.18.7"},
		{"Mihomo v1.0.0\n", "v1.0.0"},
		{"no version here\n", ""},
		{"", ""},
		{"v\n", ""},        // 'v' alone is not a version
		{"abc v2.0.0\n", "v2.0.0"},
	}
	for _, tc := range cases {
		got := parseMihomoLine(tc.input)
		if got != tc.want {
			t.Errorf("parseMihomoLine(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestReadMihomoVersionMissing(t *testing.T) {
	// Path that definitely doesn't exist.
	got := readMihomoVersion("/tmp/definitely_does_not_exist_vpnkit_test_12345")
	if got != "" {
		t.Errorf("expected empty for missing binary, got %q", got)
	}
}

// --- writeJSON error path ---

func TestWriteJSONUnmarshalable(t *testing.T) {
	var buf bytes.Buffer
	// A channel is not JSON-serializable.
	err := writeJSON(&buf, make(chan int))
	if err == nil {
		t.Error("expected marshal error for non-serializable type")
	}
}

// --- runSubsList empty ---

func TestSubsListEmpty(t *testing.T) {
	// With an empty list, output should be empty but not error.
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	var buf bytes.Buffer
	err := runSubsList(&buf, st, false)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got: %s", buf.String())
	}
}

// --- loadClient with a real store in temp HOME ---

func TestLoadClientFromRealStore(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	// Create a fresh store in the temp HOME so loadClient can read it.
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	client, st, err := loadClient()
	if err != nil {
		t.Fatalf("loadClient: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
	if st == nil {
		t.Error("expected non-nil store")
	}
	if st.Cfg.ControllerPort == 0 {
		t.Error("expected non-zero controller port")
	}
}

// --- runLocalRules edge cases ---

func TestLocalRulesListEmpty(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	var buf bytes.Buffer
	if err := runLocalRulesList(&buf, st, false); err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty rules: %s", buf.String())
	}
}

func TestLocalNodesListEmpty(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	var buf bytes.Buffer
	if err := runLocalNodesList(&buf, st, false); err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty nodes: %s", buf.String())
	}
}

// --- runUninstall interactive prompt (n aborts) ---

func TestUninstallInteractiveAbort(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()
	_ = p

	// Redirect stdin to simulate user answering 'n'.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var buf bytes.Buffer
	opts := uninstallOptions{Yes: false, BackupDir: t.TempDir()}
	err = runUninstall(&buf, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "aborted") {
		t.Errorf("expected 'aborted' in output: %s", buf.String())
	}
}

func TestUninstallInteractiveConfirm(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()
	_ = p

	// Redirect stdin to simulate user answering 'y'.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.WriteString("y\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var buf bytes.Buffer
	opts := uninstallOptions{Yes: false, BackupDir: t.TempDir()}
	err = runUninstall(&buf, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should NOT say "aborted".
	if strings.Contains(buf.String(), "aborted") {
		t.Errorf("should not abort on 'y': %s", buf.String())
	}
}

func TestUninstallInteractiveKeepMihomo(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	_ = os.MkdirAll(p.LocalBin, 0o755)
	_ = os.WriteFile(filepath.Join(p.LocalBin, "mihomo"), []byte("fake"), 0o755)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.WriteString("n\n") // abort, just test the listing output
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var buf bytes.Buffer
	opts := uninstallOptions{Yes: false, KeepMihomo: true, BackupDir: t.TempDir()}
	_ = runUninstall(&buf, opts)
	// With KeepMihomo=true, mihomo path should NOT be listed.
	out := buf.String()
	if strings.Contains(out, filepath.Join(p.LocalBin, "mihomo")) {
		t.Errorf("mihomo path listed despite KeepMihomo: %s", out)
	}
}

func TestUninstallInteractivePurgeLists(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var buf bytes.Buffer
	opts := uninstallOptions{Yes: false, Purge: true, BackupDir: t.TempDir()}
	_ = runUninstall(&buf, opts)
	out := buf.String()
	if !strings.Contains(out, "--purge") {
		t.Errorf("purge warning not shown: %s", out)
	}
}

// --- dispatch function tests (use panicOnDie to avoid os.Exit) ---

// mustNotPanic runs f and fails if f panics.
func mustNotPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	f()
}

// mustPanicWith runs f and expects a panic whose string contains want.
func mustPanicWith(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic containing %q, but no panic", want)
			return
		}
		if !strings.Contains(fmt.Sprint(r), want) {
			t.Errorf("panic %q does not contain %q", r, want)
		}
	}()
	f()
}

// --- dispatchTarget ---

func TestDispatchTargetShow(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// No args → shows current global_target (empty string is fine).
	mustNotPanic(t, func() { dispatchTarget(nil) })
}

func TestDispatchTargetSet(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// rc.7+: target value must resolve to a known source / DIRECT /
	// REJECT. DIRECT is the safe sentinel that always passes validation
	// even with an empty store (this is the post-init shape).
	mustNotPanic(t, func() { dispatchTarget([]string{"DIRECT"}) })
}

// TestDispatchTargetRejectsUnknown — rc.7 validation: an arbitrary
// string (typo, path traversal, garbage) must error out, not silently
// persist as a broken target the assembler can't satisfy.
func TestDispatchTargetRejectsUnknown(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "doesn't match", func() { dispatchTarget([]string{"PROXY"}) })
	mustPanicWith(t, "doesn't match", func() { dispatchTarget([]string{"../../etc/passwd"}) })
	mustPanicWith(t, "cannot be empty", func() { dispatchTarget([]string{""}) })
}

// --- dispatchSubs happy paths ---

func TestDispatchSubsList(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() { dispatchSubs([]string{"list"}) })
}

func TestDispatchSubsListJSON(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() { dispatchSubs([]string{"ls", "--json"}) })
}

func TestDispatchSubsAdd(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() {
		dispatchSubs([]string{"add", "mylist", "https://example.com/sub"})
	})
}

func TestDispatchSubsRm(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Add then remove.
	mustNotPanic(t, func() {
		dispatchSubs([]string{"add", "sub1", "https://example.com/1"})
	})
	mustNotPanic(t, func() {
		dispatchSubs([]string{"rm", "sub1"})
	})
}

func TestDispatchSubsEnable(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() {
		dispatchSubs([]string{"add", "sub2", "https://example.com/2"})
	})
	mustNotPanic(t, func() { dispatchSubs([]string{"disable", "sub2"}) })
	mustNotPanic(t, func() { dispatchSubs([]string{"enable", "sub2"}) })
}

func TestDispatchSubsUnknownVerb(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() {
		dispatchSubs([]string{"bogus"})
	})
}

func TestDispatchSubsNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	mustPanicWith(t, "dieUserErr", func() {
		dispatchSubs([]string{})
	})
}

// --- dispatchLocalNodes happy paths ---

func TestDispatchLocalNodesList(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"list"}) })
}

func TestDispatchLocalNodesAdd(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	const testURI = "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-A"
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"add", testURI}) })
}

func TestDispatchLocalNodesRm(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	const testURI = "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-B"
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"add", testURI}) })
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"rm", "HK-B"}) })
}

func TestDispatchLocalNodesEdit(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	const testURI = "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-C"
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"add", testURI}) })
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"edit", "HK-C", "server=5.6.7.8"}) })
}

func TestDispatchLocalNodesUnknown(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() {
		dispatchLocalNodes([]string{"bogus"})
	})
}

// --- dispatchLocalRules happy paths ---

func TestDispatchLocalRulesList(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() { dispatchLocalRules([]string{"list"}) })
}

func TestDispatchLocalRulesAdd(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() {
		dispatchLocalRules([]string{"add", "DOMAIN", "example.com", "PROXY"})
	})
}

func TestDispatchLocalRulesRm(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustNotPanic(t, func() {
		dispatchLocalRules([]string{"add", "DOMAIN", "example.com", "PROXY"})
	})
	mustNotPanic(t, func() {
		dispatchLocalRules([]string{"rm", "0"})
	})
}

func TestDispatchLocalRulesMove(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Add two rules then move.
	mustNotPanic(t, func() {
		dispatchLocalRules([]string{"add", "DOMAIN", "a.com", "PROXY"})
	})
	mustNotPanic(t, func() {
		dispatchLocalRules([]string{"add", "DOMAIN", "b.com", "DIRECT"})
	})
	mustNotPanic(t, func() {
		dispatchLocalRules([]string{"move", "0", "1"})
	})
}

func TestDispatchLocalRulesRmBadIndex(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() {
		dispatchLocalRules([]string{"rm", "notanumber"})
	})
}

func TestDispatchLocalRulesUnknown(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() {
		dispatchLocalRules([]string{"bogus"})
	})
}

// --- readMihomoVersion with fake binary ---

func TestReadMihomoVersionFakeBinary(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "fakemihomo")
	script := "#!/bin/sh\necho 'Mihomo Meta v1.99.0 linux amd64'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	got := readMihomoVersion(bin)
	if got != "v1.99.0" {
		t.Errorf("readMihomoVersion = %q, want v1.99.0", got)
	}
}

// --- runVersion smoke test ---

func TestRunVersionSmoke(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	// runVersion prints to os.Stdout; just make sure it doesn't panic.
	mustNotPanic(t, func() { runVersion() })
}

func TestRunVersionLongCommit(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	// Override commit so the truncation branch (len > 7) is hit.
	origCommit := commit
	commit = "abcdef1234567890"
	defer func() { commit = origCommit }()
	mustNotPanic(t, func() { runVersion() })
}

func TestRunVersionWithMihomoBinary(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	// Write a fake mihomo binary so the "mihomo binary: ... bytes" branch is hit.
	if err := os.MkdirAll(filepath.Dir(p.MihomoBinary()), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p.MihomoBinary(), []byte("fake"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustNotPanic(t, func() { runVersion() })
}

// --- runEnv smoke test ---

func TestRunEnvSmoke(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	// runEnv renders env vars from store; just make sure it doesn't panic.
	// With no store file the function degrades gracefully.
	mustNotPanic(t, func() { runEnv([]string{}) })
}

func TestRunEnvUnset(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	mustNotPanic(t, func() { runEnv([]string{"--unset"}) })
}

func TestRunEnvZsh(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	mustNotPanic(t, func() { runEnv([]string{"--shell=zsh"}) })
}

func TestRunEnvFish(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	mustNotPanic(t, func() { runEnv([]string{"--shell=fish"}) })
}

func TestRunEnvFunctions(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	mustNotPanic(t, func() { runEnv([]string{"--functions"}) })
}

// --- asString nil case ---

func TestAsStringNil(t *testing.T) {
	got := asString(nil)
	if got != "" {
		t.Errorf("asString(nil) = %q, want empty", got)
	}
}

func TestAsStringInt(t *testing.T) {
	got := asString(42)
	if got != "" {
		t.Errorf("asString(42) = %q, want empty (not a string)", got)
	}
}

// --- runSubsList with subs present ---

func TestRunSubsListWithSubs(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		Subscriptions: []store.Subscription{
			{Name: "test", URL: "https://example.com", Enabled: true, NodeCount: 5},
			{Name: "disabled", URL: "https://example.com/2", Enabled: false, NodeCount: 0},
		},
	}}
	var buf bytes.Buffer
	if err := runSubsList(&buf, st, false); err != nil {
		t.Fatalf("list: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "test") || !strings.Contains(out, "disabled") {
		t.Errorf("expected both subs in output: %s", out)
	}
}

// --- main.go dispatch functions (error path via dieRuntime, or success for non-client ones) ---

func TestDispatchInitFromMain(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	// dispatchInit should succeed (just calls runInit internally).
	mustNotPanic(t, func() { dispatchInit([]string{}) })
}

func TestDispatchInitForce(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	mustNotPanic(t, func() { dispatchInit([]string{"--force"}) })
}

func TestDispatchUninstallFromMain(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	// Pipe stdin to answer "n" so it aborts cleanly.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	mustNotPanic(t, func() {
		dispatchUninstall([]string{"--backup-dir=" + t.TempDir()})
	})
}

func TestDispatchUninstallYes(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	mustNotPanic(t, func() {
		dispatchUninstall([]string{"--yes", "--backup-dir=" + t.TempDir()})
	})
}

// dispatchStatus/IP/Mode/Groups require a running mihomo → loadClient fails → dieRuntime.
// We use mustPanicWith to cover the 3-stmt loadClient-error block.

func TestDispatchStatusNoClient(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	// Store must exist for loadClient to even try.
	var buf bytes.Buffer
	_ = runInit(&buf, runInitOpts{})
	mustPanicWith(t, "dieRuntime", func() { dispatchStatus([]string{}) })
}

func TestDispatchIPNoClient(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	_ = runInit(&buf, runInitOpts{})
	mustPanicWith(t, "dieRuntime", func() { dispatchIP([]string{}) })
}

func TestDispatchModeNoClient(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	_ = runInit(&buf, runInitOpts{})
	// rc.7: when mihomo is unreachable, mode persists to disk and warns
	// to stderr but returns rc=0 (consistent with `subs add`). Used to
	// dieUserErr; that was inconsistent with the rest of the mutators.
	mustNotPanic(t, func() { dispatchMode([]string{"rule"}) })
}

func TestDispatchGroupsNoClient(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	_ = runInit(&buf, runInitOpts{})
	mustPanicWith(t, "dieRuntime", func() { dispatchGroups([]string{}) })
}

func TestDispatchNodesNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	// Missing group arg → dieUserErr.
	mustPanicWith(t, "dieUserErr", func() { dispatchNodes([]string{}) })
}

func TestDispatchNodesNoClient(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	_ = runInit(&buf, runInitOpts{})
	// runNodes calls client → connection refused → dieUserErr.
	mustPanicWith(t, "die", func() { dispatchNodes([]string{"mygroup"}) })
}

func TestDispatchUseNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	mustPanicWith(t, "dieUserErr", func() { dispatchUse([]string{}) })
}

func TestDispatchUseNoClient(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	var buf bytes.Buffer
	_ = runInit(&buf, runInitOpts{})
	// runUse calls client → connection refused → dieUserErr.
	mustPanicWith(t, "die", func() { dispatchUse([]string{"grp", "node"}) })
}

// TestDispatchUpdateNetworkFail exercises dispatchUpdate when the store
// exists but the network check fails → runUpdate errors → dieRuntime (line 257).
func TestDispatchUpdateNetworkFail(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()

	// Unset proxy so SmartClient doesn't route through live proxy.
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(k, "")
	}
	// Valid store.
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Dead API port → network error → dieRuntime.
	origBase := updateAPIBase
	updateAPIBase = "http://127.0.0.1:1"
	defer func() { updateAPIBase = origBase }()

	mustPanicWith(t, "dieRuntime", func() {
		dispatchUpdate([]string{"--check"})
	})
}


func TestDispatchSubsDisable(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchSubs([]string{"add", "doge", "https://example.invalid/s"})
	dispatchSubs([]string{"disable", "doge"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	if st.Cfg.Subscriptions[0].Enabled {
		t.Errorf("disable did not flip enabled flag")
	}
}

func TestDispatchSubsUpdateUnknown(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "die", func() { dispatchSubs([]string{"update", "no-such-sub"}) })
}

func TestDispatchLocalNodesListJSON(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalNodes([]string{"list", "--json"})
}

func TestDispatchLocalRulesListJSON(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalRules([]string{"list", "--json"})
}

// --- dispatchLocalGroups coverage ---

func TestDispatchLocalGroupsNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalGroups([]string{}) })
}

func TestDispatchLocalGroupsUnknownVerb(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalGroups([]string{"bogus"}) })
}

func TestDispatchLocalGroupsListJSON(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "myhome"})
	mustNotPanic(t, func() { dispatchLocalGroups([]string{"list", "--json"}) })
}

func TestDispatchLocalGroupsRm(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "todelete"})
	mustNotPanic(t, func() { dispatchLocalGroups([]string{"rm", "todelete"}) })
}

func TestDispatchLocalGroupsRmNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalGroups([]string{"rm"}) })
}

func TestDispatchLocalGroupsEnableAlreadyEnabled(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// add creates group with Enabled=true; enabling again is a no-op
	dispatchLocalGroups([]string{"add", "grpA"})
	mustNotPanic(t, func() { dispatchLocalGroups([]string{"enable", "grpA"}) })
}

func TestDispatchLocalGroupsDisableAlreadyDisabled(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "grpB"})
	dispatchLocalGroups([]string{"disable", "grpB"})
	// disabling again is a no-op
	mustNotPanic(t, func() { dispatchLocalGroups([]string{"disable", "grpB"}) })
}

func TestDispatchLocalGroupsEnableNotFound(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalGroups([]string{"enable", "nosuchgroup"}) })
}

func TestDispatchLocalGroupsEnableNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalGroups([]string{"enable"}) })
}

func TestDispatchLocalGroupsRenameNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalGroups([]string{"rename", "only-one"}) })
}

func TestRunLocalGroupsListDisabledMark(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "grpC"})
	dispatchLocalGroups([]string{"disable", "grpC"})
	st, _ := store.Load(paths.Resolve().VpnkitConfigFile())
	var out bytes.Buffer
	runLocalGroupsList(&out, st, false)
	if !strings.Contains(out.String(), "grpC") {
		t.Errorf("list missing 'grpC': %q", out.String())
	}
}

// --- dispatchLocalNodes mv coverage ---

func TestDispatchLocalNodesMvVerb(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	dispatchLocalGroups([]string{"add", "src"})
	dispatchLocalGroups([]string{"add", "dst"})
	const uri = "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#MV-Node"
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"add", uri, "--group=src"}) })
	mustNotPanic(t, func() { dispatchLocalNodes([]string{"mv", "MV-Node", "dst"}) })
}

func TestDispatchLocalNodesMvNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalNodes([]string{"mv", "only-one"}) })
}

func TestDispatchLocalNodesMvNotFound(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalNodes([]string{"mv", "nosuchnode", "grp"}) })
}

func TestDispatchLocalGroupsAddNoArgs(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()
	restoreDie := panicOnDie(t)
	defer restoreDie()
	var buf bytes.Buffer
	if err := runInit(&buf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	mustPanicWith(t, "dieUserErr", func() { dispatchLocalGroups([]string{"add"}) })
}
