package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectViaSocket(t *testing.T) {
	tmp := t.TempDir()
	sd := filepath.Join(tmp, "systemd")
	if err := os.MkdirAll(sd, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(sd, "private"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if got := Detect(nil); got != ModeSystemdUser {
		t.Errorf("got %s want systemd-user", got)
	}
}

func TestDetectFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	runner := func(args ...string) (string, error) {
		return "", &execError{}
	}
	if got := Detect(runner); got != ModePID {
		t.Errorf("got %s want pid", got)
	}
}

type execError struct{}

func (*execError) Error() string { return "exit 1" }
