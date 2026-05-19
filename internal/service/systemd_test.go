package service

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRenderUnit(t *testing.T) {
	// Isolate from the host's real proxy env so this test only asserts the
	// invariant skeleton bits. Proxy injection has its own dedicated cases.
	unsetAllProxyEnv(t)

	var buf bytes.Buffer
	err := renderUnit(&buf, "/home/u/.local/bin/mihomo", "/home/u/.config/mihomo", 0)
	if err != nil {
		t.Fatalf("renderUnit: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "ExecStart=/home/u/.local/bin/mihomo -d /home/u/.config/mihomo") {
		t.Errorf("missing ExecStart: %s", s)
	}
	if !strings.Contains(s, "WantedBy=default.target") {
		t.Error("missing WantedBy")
	}
	if !strings.Contains(s, "StartLimitBurst=3") {
		t.Error("missing StartLimitBurst")
	}
	if !strings.Contains(s, "StartLimitIntervalSec=30") {
		t.Error("missing StartLimitIntervalSec")
	}
	if strings.Contains(s, "Environment=") {
		t.Errorf("no proxy env set but Environment= appeared: %s", s)
	}
}

// proxyEnvKeys are the variables systemd should forward to mihomo so that
// mihomo's geox-url downloads (and any other outbound HTTP) honor the user's
// shell-level proxy setup. Both casings exist because clients vary.
var proxyEnvKeys = []string{
	"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "NO_PROXY",
	"https_proxy", "http_proxy", "all_proxy", "no_proxy",
}

func unsetAllProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range proxyEnvKeys {
		t.Setenv(k, "")
	}
}

func TestRenderUnitInjectsProxyEnv(t *testing.T) {
	unsetAllProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7897")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:7897")
	t.Setenv("NO_PROXY", "localhost,127.0.0.1,::1")

	var buf bytes.Buffer
	if err := renderUnit(&buf, "/x/mihomo", "/c", 50595); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{
		`Environment="HTTPS_PROXY=http://127.0.0.1:7897"`,
		`Environment="HTTP_PROXY=http://127.0.0.1:7897"`,
		`Environment="NO_PROXY=localhost,127.0.0.1,::1"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("unit missing %q\n--- unit ---\n%s", want, s)
		}
	}
}

func TestRenderUnitSkipsSelfReferentialProxy(t *testing.T) {
	// HTTPS_PROXY points at vpnkit's own mixed-port (50595). Injecting this
	// would deadlock mihomo on startup: it tries to download MMDB through
	// itself, but is not yet listening.
	tests := []struct {
		name  string
		value string
	}{
		{"loopback v4", "http://127.0.0.1:50595"},
		{"localhost", "http://localhost:50595"},
		{"loopback v6", "http://[::1]:50595"},
		{"socks5 v4", "socks5://127.0.0.1:50595"},
		{"with creds", "http://user:pass@127.0.0.1:50595"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAllProxyEnv(t)
			t.Setenv("HTTPS_PROXY", tt.value)

			var buf bytes.Buffer
			if err := renderUnit(&buf, "/x/mihomo", "/c", 50595); err != nil {
				t.Fatal(err)
			}
			s := buf.String()
			if strings.Contains(s, "Environment=") && strings.Contains(s, "HTTPS_PROXY") {
				t.Errorf("self-referential HTTPS_PROXY=%q was injected; would deadlock\n--- unit ---\n%s", tt.value, s)
			}
		})
	}
}

func TestRenderUnitDetectsSchemeRelativeSelfRef(t *testing.T) {
	// Rare but legal form: "//host:port" without a scheme. The guard must
	// still catch it, otherwise it slips into the unit and deadlocks mihomo.
	unsetAllProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "//127.0.0.1:50595")

	var buf bytes.Buffer
	if err := renderUnit(&buf, "/x/mihomo", "/c", 50595); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "HTTPS_PROXY") {
		t.Errorf("scheme-relative self-ref leaked into unit:\n%s", buf.String())
	}
}

func TestRenderUnitInjectsForeignLoopbackProxy(t *testing.T) {
	// HTTPS_PROXY points at 127.0.0.1 but a *different* port — another
	// proxy on the same host (e.g. clash-verge). Safe to inject.
	unsetAllProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7897")

	var buf bytes.Buffer
	if err := renderUnit(&buf, "/x/mihomo", "/c", 50595); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, `Environment="HTTPS_PROXY=http://127.0.0.1:7897"`) {
		t.Errorf("foreign loopback HTTPS_PROXY was incorrectly suppressed\n--- unit ---\n%s", s)
	}
}

func TestRenderUnitNoProxyAlwaysInjected(t *testing.T) {
	// NO_PROXY is not a proxy target — it's a bypass list — so the self-ref
	// guard must not apply, even if the user typed odd values.
	unsetAllProxyEnv(t)
	t.Setenv("NO_PROXY", "127.0.0.1:50595,example.com")

	var buf bytes.Buffer
	if err := renderUnit(&buf, "/x/mihomo", "/c", 50595); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, `Environment="NO_PROXY=127.0.0.1:50595,example.com"`) {
		t.Errorf("NO_PROXY should always be injected verbatim\n--- unit ---\n%s", s)
	}
}

func TestRenderUnitQuotesValuesWithSpaces(t *testing.T) {
	// systemd's Environment= directive requires quoting around values that
	// contain whitespace or special characters. Our quoting wraps every
	// KEY=VALUE pair so this is robust.
	unsetAllProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://proxy with space:80")

	var buf bytes.Buffer
	if err := renderUnit(&buf, "/x/mihomo", "/c", 50595); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, `Environment="HTTPS_PROXY=http://proxy with space:80"`) {
		t.Errorf("value with space not quoted as a single token\n--- unit ---\n%s", s)
	}
}

func TestSystemdUninstallRemovesDropInDir(t *testing.T) {
	dir := t.TempDir()
	unitPath := dir + "/mihomo.service"
	dropInDir := unitPath + ".d"

	if err := os.WriteFile(unitPath, []byte("[Unit]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dropInDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dropInDir+"/env.conf", []byte("[Service]\nEnvironment=X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := func(args ...string) (string, error) { return "", nil }
	m := &SystemdManager{cfg: Config{UnitPath: unitPath}, run: runner}

	if err := m.Uninstall(nil); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Errorf("unit file still exists: %v", err)
	}
	if _, err := os.Stat(dropInDir); !os.IsNotExist(err) {
		t.Errorf("drop-in dir still exists: %v (should have been removed)", err)
	}
}

// TestSystemdUninstallEmptyUnitPathGuard ensures the safety guard against a
// zero-value UnitPath skips the drop-in cleanup branch (otherwise it would
// resolve to ".d" and recursively wipe whichever directory the caller's cwd
// points at).
func TestSystemdUninstallEmptyUnitPathGuard(t *testing.T) {
	// Create a ".d" directory in a temp scratch space, then run Uninstall
	// with the cwd pointing there. If the guard were missing, the drop-in
	// branch would attempt RemoveAll(".d") and wipe it.
	scratch := t.TempDir()
	canary := scratch + "/.d/keep-me"
	if err := os.MkdirAll(scratch+"/.d", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canary, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir("/")
	}()
	if err := os.Chdir(scratch); err != nil {
		t.Fatal(err)
	}

	runner := func(args ...string) (string, error) { return "", nil }
	m := &SystemdManager{cfg: Config{UnitPath: ""}, run: runner}
	_ = m.Uninstall(nil) // intentionally ignore — Remove("") returns an error but the guard branch is what we test

	if _, err := os.Stat(canary); err != nil {
		t.Fatalf("canary file at %s was destroyed by Uninstall — guard missing? err=%v", canary, err)
	}
}

func TestFakeSystemctlInvocations(t *testing.T) {
	calls := []string{}
	runner := func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}
	m := &SystemdManager{cfg: Config{BinaryPath: "/x", ConfigDir: "/c", UnitPath: t.TempDir() + "/mihomo.service"}, run: runner}
	if err := m.Install(nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(nil); err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"--user daemon-reload",
		"--user enable --now mihomo.service",
		"--user start mihomo.service",
		"--user stop mihomo.service",
	}
	for _, e := range expected {
		found := false
		for _, c := range calls {
			if c == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected call %q not seen; got %v", e, calls)
		}
	}
}
