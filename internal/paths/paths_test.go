package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXDGFallsBackToHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	p := Resolve()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"VpnkitConfig", p.VpnkitConfig, filepath.Join(tmp, ".config", "vpnkit")},
		{"MihomoConfig", p.MihomoConfig, filepath.Join(tmp, ".config", "mihomo")},
		{"VpnkitState", p.VpnkitState, filepath.Join(tmp, ".local", "state", "vpnkit")},
		{"VpnkitCache", p.VpnkitCache, filepath.Join(tmp, ".cache", "vpnkit")},
		{"LocalBin", p.LocalBin, filepath.Join(tmp, ".local", "bin")},
		{"SystemdUserDir", p.SystemdUserDir, filepath.Join(tmp, ".config", "systemd", "user")},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestXDGRespectsEnvOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	p := Resolve()
	if p.VpnkitConfig != filepath.Join(tmp, "cfg", "vpnkit") {
		t.Errorf("XDG_CONFIG_HOME not honored: %s", p.VpnkitConfig)
	}
	if p.VpnkitState != filepath.Join(tmp, "state", "vpnkit") {
		t.Errorf("XDG_STATE_HOME not honored: %s", p.VpnkitState)
	}
}

func TestEnsureCreatesAllDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	p := Resolve()
	if err := p.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	dirs := []string{p.VpnkitConfig, p.MihomoConfig, p.VpnkitState, p.VpnkitCache, p.LocalBin, p.SystemdUserDir}
	for _, d := range dirs {
		if info, err := os.Stat(d); err != nil || !info.IsDir() {
			t.Errorf("dir %s missing: %v", d, err)
		}
	}
}
