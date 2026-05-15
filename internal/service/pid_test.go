package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPIDLifecycle(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not on PATH")
	}
	tmp := t.TempDir()
	cfg := Config{
		BinaryPath:  sleep,
		ConfigDir:   tmp,
		PIDFilePath: filepath.Join(tmp, "x.pid"),
		LogFilePath: filepath.Join(tmp, "x.log"),
	}
	m := NewPID(cfg, []string{"60"}) // override args for test
	ctx := context.Background()

	// Status before start: not running.
	st, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status pre: %v", err)
	}
	if st.Running {
		t.Fatal("expected not running")
	}

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	st, err = m.Status(ctx)
	if err != nil {
		t.Fatalf("Status post: %v", err)
	}
	if !st.Running || st.PID == 0 {
		t.Fatalf("expected running, got %+v", st)
	}

	if err := m.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, _ = m.Status(ctx)
	if st.Running {
		t.Fatal("still running after stop")
	}

	// Stop again should report not running.
	if err := m.Stop(ctx); !errors.Is(err, ErrNotRunning) {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	// PID file should be cleaned up.
	if _, err := os.Stat(cfg.PIDFilePath); !os.IsNotExist(err) {
		t.Errorf("PID file left behind")
	}
}

func TestPIDRestart(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not on PATH")
	}
	tmp := t.TempDir()
	cfg := Config{
		BinaryPath:  sleep,
		PIDFilePath: filepath.Join(tmp, "x.pid"),
		LogFilePath: filepath.Join(tmp, "x.log"),
	}
	m := NewPID(cfg, []string{"60"})
	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	st1, _ := m.Status(ctx)
	if err := m.Restart(ctx); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	st2, _ := m.Status(ctx)
	if !st2.Running || st1.PID == st2.PID {
		t.Errorf("restart PID unchanged: %d -> %d", st1.PID, st2.PID)
	}
	_ = m.Stop(ctx)
}
