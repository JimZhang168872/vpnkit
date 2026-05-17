package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// initEnv wires a tmp HOME so paths.Resolve returns sandboxed dirs and
// returns the resolved XDG view.
func initEnv(t *testing.T) (paths.XDG, func()) {
	t.Helper()
	tmp := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldConfig := os.Getenv("XDG_CONFIG_HOME")
	oldState := os.Getenv("XDG_STATE_HOME")
	oldCache := os.Getenv("XDG_CACHE_HOME")
	os.Setenv("HOME", tmp)
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_STATE_HOME")
	os.Unsetenv("XDG_CACHE_HOME")
	return paths.Resolve(), func() {
		os.Setenv("HOME", oldHome)
		if oldConfig != "" {
			os.Setenv("XDG_CONFIG_HOME", oldConfig)
		}
		if oldState != "" {
			os.Setenv("XDG_STATE_HOME", oldState)
		}
		if oldCache != "" {
			os.Setenv("XDG_CACHE_HOME", oldCache)
		}
	}
}

func TestInitFromScratch(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	var out bytes.Buffer
	if err := runInit(&out, runInitOpts{}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	if _, err := os.Stat(p.VpnkitConfigFile()); err != nil {
		t.Errorf("config.toml not created: %v", err)
	}
	if _, err := os.Stat(p.MihomoConfigFile()); err != nil {
		t.Errorf("config.yaml not created: %v", err)
	}

	yaml, _ := os.ReadFile(p.MihomoConfigFile())
	s := string(yaml)
	for _, want := range []string{"authentication:", "bind-address: 127.0.0.1", "allow-lan: false"} {
		if !strings.Contains(s, want) {
			t.Errorf("config.yaml missing %q:\n%s", want, s)
		}
	}

	output := out.String()
	if !strings.Contains(output, p.VpnkitConfigFile()) || !strings.Contains(output, p.MihomoConfigFile()) {
		t.Errorf("output missing file paths:\n%s", output)
	}
}

func TestInitIdempotent(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	var out bytes.Buffer
	if err := runInit(&out, runInitOpts{}); err != nil {
		t.Fatal(err)
	}
	tomlBefore, _ := os.ReadFile(p.VpnkitConfigFile())
	yamlBefore, _ := os.ReadFile(p.MihomoConfigFile())

	if err := runInit(&out, runInitOpts{}); err != nil {
		t.Fatal(err)
	}
	tomlAfter, _ := os.ReadFile(p.VpnkitConfigFile())
	yamlAfter, _ := os.ReadFile(p.MihomoConfigFile())

	if string(tomlBefore) != string(tomlAfter) {
		t.Error("config.toml changed on second init (should be idempotent)")
	}
	if string(yamlBefore) != string(yamlAfter) {
		t.Error("config.yaml changed on second init")
	}
}

func TestInitRestoresProfiles(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	// Simulate a backup file with profiles.
	backup := filepath.Join(t.TempDir(), "vpnkit-profiles-backup.toml")
	backupContent := `[[profiles]]
  name = "airport-A"
  url = "https://example.com/sub"
`
	if err := os.WriteFile(backup, []byte(backupContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runInit(&out, runInitOpts{RestorePath: backup}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	toml, _ := os.ReadFile(p.VpnkitConfigFile())
	s := string(toml)
	for _, want := range []string{"airport-A", "https://example.com/sub"} {
		if !strings.Contains(s, want) {
			t.Errorf("profile not restored: missing %q in:\n%s", want, s)
		}
	}
	if !strings.Contains(out.String(), "restored") {
		t.Errorf("output should mention restoration:\n%s", out.String())
	}
}

func TestInitForceBacksUpV1Store(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	storePath := p.VpnkitConfigFile()
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storePath, []byte(`active_profile = "doge"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runInit(&out, runInitOpts{Force: true}); err != nil {
		t.Fatalf("init --force: %v", err)
	}

	matches, err := filepath.Glob(storePath + ".bak.*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected one .bak.* file, got %v", matches)
	}
	st, err := store.Load(storePath)
	if err != nil {
		t.Fatalf("load post-init: %v", err)
	}
	if st.Cfg.SchemaVersion != 2 {
		t.Errorf("post-init schema: %d", st.Cfg.SchemaVersion)
	}
}
