package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "x.yaml")
	if err := AtomicWrite(target, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "hello\n" {
		t.Errorf("got %q err %v", got, err)
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm: %v", info.Mode().Perm())
	}
}

func TestAtomicWriteReplacesExisting(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "x.yaml")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(target, []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new\n" {
		t.Errorf("got %q", got)
	}
}
