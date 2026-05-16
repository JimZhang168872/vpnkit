package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/service"
)

func TestUninstallRemovesAllVpnkitOwnedPaths(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	// Lay down a complete set of files vpnkit would normally own.
	_ = os.MkdirAll(p.VpnkitConfig, 0o755)
	_ = os.MkdirAll(p.MihomoConfig, 0o755)
	_ = os.MkdirAll(p.VpnkitState, 0o755)
	_ = os.MkdirAll(p.VpnkitCache, 0o755)
	_ = os.MkdirAll(p.LocalBin, 0o755)
	_ = os.WriteFile(p.VpnkitConfigFile(), []byte(`controller_port = 9090
[[profiles]]
  name = "airport-A"
  url = "https://example.com/sub"
`), 0o600)
	_ = os.WriteFile(p.MihomoConfigFile(), []byte("mixed-port: 7890\n"), 0o600)
	_ = os.WriteFile(filepath.Join(p.LocalBin, "vpnkit"), []byte("fake"), 0o755)
	_ = os.WriteFile(filepath.Join(p.LocalBin, "mihomo"), []byte("fake"), 0o755)
	_ = os.MkdirAll(p.SystemdUserDir, 0o755)
	_ = os.WriteFile(p.SystemdUnit(), []byte("[Unit]\n"), 0o644)

	var out bytes.Buffer
	opts := uninstallOptions{Yes: true, KeepMihomo: false}
	if err := runUninstall(&out, opts); err != nil {
		t.Fatalf("runUninstall: %v", err)
	}

	for _, path := range []string{
		p.VpnkitConfig,
		p.MihomoConfig,
		p.VpnkitState,
		p.VpnkitCache,
		p.VpnkitConfigFile(),
		filepath.Join(p.LocalBin, "vpnkit"),
		filepath.Join(p.LocalBin, "mihomo"),
		p.SystemdUnit(),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("still exists after uninstall: %s", path)
		}
	}
}

func TestUninstallBacksUpProfilesByDefault(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	_ = os.MkdirAll(p.VpnkitConfig, 0o755)
	_ = os.WriteFile(p.VpnkitConfigFile(), []byte(`controller_port = 9090
[[profiles]]
  name = "airport-A"
  url = "https://example.com/sub"
`), 0o600)

	var out bytes.Buffer
	opts := uninstallOptions{Yes: true, BackupDir: t.TempDir()}
	if err := runUninstall(&out, opts); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "backed up profiles") {
		t.Errorf("missing backup line in output:\n%s", got)
	}

	// Find the backup file via the output path.
	entries, _ := os.ReadDir(opts.BackupDir)
	if len(entries) == 0 {
		t.Fatalf("no backup file written in %s", opts.BackupDir)
	}
	data, _ := os.ReadFile(filepath.Join(opts.BackupDir, entries[0].Name()))
	if !strings.Contains(string(data), "airport-A") {
		t.Errorf("backup missing profile:\n%s", string(data))
	}
}

func TestUninstallPurgeSkipsBackup(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	_ = os.MkdirAll(p.VpnkitConfig, 0o755)
	_ = os.WriteFile(p.VpnkitConfigFile(), []byte(`[[profiles]]
  name = "airport-A"
  url = "u"
`), 0o600)

	var out bytes.Buffer
	opts := uninstallOptions{Yes: true, Purge: true, BackupDir: t.TempDir()}
	if err := runUninstall(&out, opts); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(opts.BackupDir)
	if len(entries) != 0 {
		t.Errorf("purge mode should not write a backup, found: %v", entries)
	}
	if strings.Contains(out.String(), "backed up") {
		t.Errorf("output should not mention backup:\n%s", out.String())
	}
}

func TestUninstallKeepMihomo(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	_ = os.MkdirAll(p.LocalBin, 0o755)
	mihomoPath := filepath.Join(p.LocalBin, "mihomo")
	_ = os.WriteFile(mihomoPath, []byte("fake"), 0o755)
	_ = os.WriteFile(filepath.Join(p.LocalBin, "vpnkit"), []byte("fake"), 0o755)

	var out bytes.Buffer
	opts := uninstallOptions{Yes: true, KeepMihomo: true}
	if err := runUninstall(&out, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mihomoPath); err != nil {
		t.Errorf("mihomo binary deleted despite --keep-mihomo: %v", err)
	}
}

func TestIsNotRunningRecognizesWrappedSentinel(t *testing.T) {
	wrapped := fmt.Errorf("svc: %w", service.ErrNotRunning)
	if !isNotRunning(wrapped) {
		t.Error("wrapped ErrNotRunning should be detected via errors.Is")
	}
	if !isNotRunning(service.ErrNotRunning) {
		t.Error("bare ErrNotRunning should be detected")
	}
	if isNotRunning(errors.New("something else entirely")) {
		t.Error("unrelated error should not match")
	}
	if isNotRunning(nil) {
		t.Error("nil error should not match")
	}
}

func TestUninstallRejectsEmptyHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })
	os.Setenv("HOME", "")
	var out bytes.Buffer
	err := runUninstall(&out, uninstallOptions{Yes: true})
	if err == nil {
		t.Error("expected error when HOME empty")
	}
	if !strings.Contains(err.Error(), "HOME") {
		t.Errorf("error message should mention HOME: %v", err)
	}
}

func TestUninstallReturnsBackupPath(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	_ = os.MkdirAll(p.VpnkitConfig, 0o755)
	_ = os.WriteFile(p.VpnkitConfigFile(), []byte(`[[profiles]]
  name = "A"
  url = "u"
`), 0o600)

	var out bytes.Buffer
	opts := uninstallOptions{Yes: true, BackupDir: t.TempDir()}
	if err := runUninstall(&out, opts); err != nil {
		t.Fatal(err)
	}
	// Output should reference the backup path so install.sh can grep it.
	output := out.String()
	if !strings.Contains(output, "BACKUP=") {
		t.Errorf("output missing BACKUP= marker for scripts to parse:\n%s", output)
	}
}
