// Package tui drives the vpnkit TUI inside a tmux session and asserts on
// captured panes. Use newTUISession(t) to spin up an isolated HOME, build
// the binary once per test run, and start a detached tmux session. Tests
// SKIP gracefully if tmux is not installed.
package tui

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	buildOnce   sync.Once
	binaryPath  string
	buildErr    error
	originalEnv []string // captured at init, before any test mutates HOME
)

func init() {
	originalEnv = os.Environ()
}

func vpnkitBinary(t *testing.T) string {
	buildOnce.Do(func() {
		repoRoot, err := repoRootDir()
		if err != nil {
			buildErr = err
			return
		}
		out := filepath.Join(os.TempDir(), "vpnkit-tui-harness")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/vpnkit")
		cmd.Dir = repoRoot
		// Use the original environment (captured before any test mutates HOME)
		// so the Go module cache is not placed inside a test TempDir, which
		// would contain read-only files that fail TempDir cleanup.
		cmd.Env = originalEnv
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("go build: %v: %s", err, stderr.String())
			return
		}
		binaryPath = out
	})
	if buildErr != nil {
		t.Fatalf("vpnkit binary build failed: %v", buildErr)
	}
	return binaryPath
}

func repoRootDir() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		next := filepath.Dir(d)
		if next == d {
			return "", fmt.Errorf("could not find go.mod walking up from cwd")
		}
		d = next
	}
}

type isoEnv struct {
	home string
}

func newIsolatedHome(t *testing.T) *isoEnv {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h, ".cache"))
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TERM", "xterm-256color")
	return &isoEnv{home: h}
}

type tuiSession struct {
	t    *testing.T
	name string
	iso  *isoEnv
}

func newTUISession(t *testing.T, iso *isoEnv) *tuiSession {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available — skipping TUI integration test")
	}
	binary := vpnkitBinary(t)
	name := "vpnkit-tui-" + randHex(4)
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name,
		"-x", "130", "-y", "36",
		fmt.Sprintf("HOME=%s XDG_CONFIG_HOME=%s XDG_STATE_HOME=%s XDG_CACHE_HOME=%s TERM=xterm-256color %s",
			iso.home,
			filepath.Join(iso.home, ".config"),
			filepath.Join(iso.home, ".local", "state"),
			filepath.Join(iso.home, ".cache"),
			binary))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("tmux new-session: %v: %s", err, stderr.String())
	}
	sess := &tuiSession{t: t, name: name, iso: iso}
	t.Cleanup(sess.Kill)
	time.Sleep(2 * time.Second)
	return sess
}

func (s *tuiSession) SendKeys(keys ...string) {
	s.t.Helper()
	for _, k := range keys {
		cmd := exec.Command("tmux", "send-keys", "-t", s.name, k)
		if err := cmd.Run(); err != nil {
			s.t.Fatalf("send-keys %q: %v", k, err)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func (s *tuiSession) SendLiteral(text string) {
	s.t.Helper()
	cmd := exec.Command("tmux", "send-keys", "-l", "-t", s.name, text)
	if err := cmd.Run(); err != nil {
		s.t.Fatalf("send-keys -l: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
}

func (s *tuiSession) Capture() string {
	s.t.Helper()
	cmd := exec.Command("tmux", "capture-pane", "-t", s.name, "-p")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		s.t.Fatalf("capture-pane: %v", err)
	}
	return stdout.String()
}

func (s *tuiSession) MustContain(want string) {
	s.t.Helper()
	frame := s.Capture()
	if !strings.Contains(frame, want) {
		s.t.Fatalf("frame missing %q:\n%s", want, frame)
	}
}

func (s *tuiSession) MustNotContain(want string) {
	s.t.Helper()
	frame := s.Capture()
	if strings.Contains(frame, want) {
		s.t.Fatalf("frame should not contain %q:\n%s", want, frame)
	}
}

func (s *tuiSession) Kill() {
	exec.Command("tmux", "kill-session", "-t", s.name).Run()
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
