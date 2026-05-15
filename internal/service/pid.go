package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PIDManager runs mihomo via a detached child and tracks it through a PID file.
type PIDManager struct {
	cfg  Config
	args []string // mihomo args; defaults to ["-d", cfg.ConfigDir]
}

// NewPID constructs a PIDManager. If args is nil, default mihomo args are used.
func NewPID(cfg Config, args []string) *PIDManager {
	if args == nil {
		args = []string{"-d", cfg.ConfigDir}
	}
	return &PIDManager{cfg: cfg, args: args}
}

func (*PIDManager) Mode() Mode { return ModePID }

// Install is a no-op in PID mode.
func (*PIDManager) Install(context.Context) error { return nil }

// Uninstall removes the pid file if any.
func (m *PIDManager) Uninstall(ctx context.Context) error {
	_ = m.Stop(ctx)
	return nil
}

// Start launches the detached child and writes the PID file.
func (m *PIDManager) Start(ctx context.Context) error {
	if st, _ := m.Status(ctx); st.Running {
		return fmt.Errorf("service: already running pid=%d", st.PID)
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.LogFilePath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(m.cfg.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	cmd := exec.Command(m.cfg.BinaryPath, m.args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	logFile.Close()

	if err := os.MkdirAll(filepath.Dir(m.cfg.PIDFilePath), 0o755); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	if err := os.WriteFile(m.cfg.PIDFilePath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	// Detach so the child outlives this process.
	_ = cmd.Process.Release()
	return nil
}

// Stop sends SIGTERM, waits up to 5s, then SIGKILL.
func (m *PIDManager) Stop(ctx context.Context) error {
	pid, err := m.readPID()
	if err != nil {
		return ErrNotRunning
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(m.cfg.PIDFilePath)
		return ErrNotRunning
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		_ = os.Remove(m.cfg.PIDFilePath)
		return ErrNotRunning
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			_ = os.Remove(m.cfg.PIDFilePath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = proc.Signal(syscall.SIGKILL)
	_ = os.Remove(m.cfg.PIDFilePath)
	return nil
}

// Restart = Stop (best-effort) + Start.
func (m *PIDManager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil && !errors.Is(err, ErrNotRunning) {
		return err
	}
	return m.Start(ctx)
}

// Status reads the PID file and probes /proc.
func (m *PIDManager) Status(ctx context.Context) (Status, error) {
	pid, err := m.readPID()
	if err != nil || !processAlive(pid) {
		return Status{Mode: ModePID}, nil
	}
	since := time.Time{}
	if info, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
		since = info.ModTime()
	}
	return Status{Running: true, PID: pid, Since: since, Mode: ModePID}, nil
}

// Logs returns mihomo's combined output. follow=true uses tail-like behavior.
func (m *PIDManager) Logs(ctx context.Context, follow bool) (io.ReadCloser, error) {
	if !follow {
		return os.Open(m.cfg.LogFilePath)
	}
	cmd := exec.CommandContext(ctx, "tail", "-n", "200", "-F", m.cfg.LogFilePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &cmdReader{r: stdout, cmd: cmd}, nil
}

type cmdReader struct {
	r   io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReader) Read(p []byte) (int, error) { return c.r.Read(p) }
func (c *cmdReader) Close() error {
	_ = c.cmd.Process.Kill()
	return c.r.Close()
}

func (m *PIDManager) readPID() (int, error) {
	data, err := os.ReadFile(m.cfg.PIDFilePath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func processAlive(pid int) bool {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}
