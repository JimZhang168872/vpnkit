package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesToFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "vpnkit.log")
	lg, err := New(path, LevelDebug)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Info("hello", "k", 1)
	lg.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "hello") || !strings.Contains(string(data), "k=1") {
		t.Errorf("log file missing content: %q", data)
	}
}

func TestLevelFiltering(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "vpnkit.log")
	lg, err := New(path, LevelWarn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Debug("debug-msg")
	lg.Info("info-msg")
	lg.Warn("warn-msg")
	lg.Close()

	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, "debug-msg") || strings.Contains(out, "info-msg") {
		t.Errorf("level filter let through too much: %q", out)
	}
	if !strings.Contains(out, "warn-msg") {
		t.Errorf("warn missing: %q", out)
	}
}
