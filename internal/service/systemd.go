package service

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/mihomo.service.tmpl
var unitFS embed.FS

// Runner abstracts `systemctl --user` invocation for testability.
type Runner func(args ...string) (string, error)

// SystemdManager implements Manager using `systemctl --user`.
type SystemdManager struct {
	cfg Config
	run Runner
}

// NewSystemd constructs the systemd backend. If runner is nil, real systemctl is used.
func NewSystemd(cfg Config, runner Runner) *SystemdManager {
	if runner == nil {
		runner = defaultSystemctl
	}
	return &SystemdManager{cfg: cfg, run: runner}
}

func defaultSystemctl(args ...string) (string, error) {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (*SystemdManager) Mode() Mode { return ModeSystemdUser }

// Install writes the unit, runs daemon-reload, and enables it.
func (m *SystemdManager) Install(_ context.Context) error {
	if err := os.MkdirAll(filepath.Dir(m.cfg.UnitPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(m.cfg.UnitPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := renderUnit(f, m.cfg.BinaryPath, m.cfg.ConfigDir); err != nil {
		return err
	}
	if _, err := m.run("--user", "daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	if _, err := m.run("--user", "enable", "--now", "mihomo.service"); err != nil {
		return fmt.Errorf("enable: %w", err)
	}
	return nil
}

func (m *SystemdManager) Uninstall(ctx context.Context) error {
	_, _ = m.run("--user", "disable", "--now", "mihomo.service")
	if err := os.Remove(m.cfg.UnitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, _ = m.run("--user", "daemon-reload")
	return nil
}

func (m *SystemdManager) Start(_ context.Context) error {
	_, err := m.run("--user", "start", "mihomo.service")
	return err
}

func (m *SystemdManager) Stop(_ context.Context) error {
	_, err := m.run("--user", "stop", "mihomo.service")
	return err
}

func (m *SystemdManager) Restart(_ context.Context) error {
	_, err := m.run("--user", "restart", "mihomo.service")
	return err
}

func (m *SystemdManager) Status(_ context.Context) (Status, error) {
	out, _ := m.run("--user", "show", "mihomo.service",
		"--property=ActiveState,MainPID,ActiveEnterTimestamp")
	st := Status{Mode: ModeSystemdUser}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "ActiveState":
			st.Running = strings.TrimSpace(v) == "active"
		case "MainPID":
			st.PID, _ = strconv.Atoi(strings.TrimSpace(v))
		case "ActiveEnterTimestamp":
			st.Since, _ = time.Parse("Mon 2006-01-02 15:04:05 MST", strings.TrimSpace(v))
		}
	}
	return st, nil
}

func (m *SystemdManager) Logs(ctx context.Context, follow bool) (io.ReadCloser, error) {
	args := []string{"--user", "-u", "mihomo.service", "--no-pager"}
	if follow {
		args = append(args, "-f", "-n", "200")
	} else {
		args = append(args, "-n", "30")
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
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

func renderUnit(w io.Writer, binary, configDir string) error {
	tmpl, err := template.ParseFS(unitFS, "templates/mihomo.service.tmpl")
	if err != nil {
		return err
	}
	return tmpl.Execute(w, map[string]string{"Binary": binary, "ConfigDir": configDir})
}
