package service

import (
	"context"
	"embed"
	"fmt"
	"io"
	"net/url"
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
	// 0o600 because Environment= directives may contain proxy URLs with
	// embedded credentials (e.g. socks5h://user:token@host). systemd accepts
	// user-mode unit files at 0o600; world-readable is unnecessary.
	f, err := os.OpenFile(m.cfg.UnitPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := renderUnit(f, m.cfg.BinaryPath, m.cfg.ConfigDir, m.cfg.MixedPort); err != nil {
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

func renderUnit(w io.Writer, binary, configDir string, mixedPort int) error {
	tmpl, err := template.ParseFS(unitFS, "templates/mihomo.service.tmpl")
	if err != nil {
		return err
	}
	return tmpl.Execute(w, map[string]any{
		"Binary":    binary,
		"ConfigDir": configDir,
		"ProxyEnv":  collectProxyEnv(mixedPort),
	})
}

// proxyEnvKeysForUnit is the fixed set of env vars systemd should forward to
// mihomo. Both case variants exist because some tools (curl, mihomo itself)
// read upper-case while others read lower-case; preserving both keeps mihomo's
// downloads consistent with the user's shell.
var proxyEnvKeysForUnit = []string{
	"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "NO_PROXY",
	"https_proxy", "http_proxy", "all_proxy", "no_proxy",
}

// collectProxyEnv reads the current process env, returns "KEY=VALUE" strings
// ready for the unit template (which wraps each in double quotes), and
// suppresses any value that points to a loopback host at mixedPort — that
// would deadlock mihomo on startup (it would try to download MMDB through
// itself before it's listening). NO_PROXY is exempt: it's a bypass list, not
// a proxy target.
func collectProxyEnv(mixedPort int) []string {
	var out []string
	for _, k := range proxyEnvKeysForUnit {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		if !isNoProxyKey(k) && isSelfReferential(v, mixedPort) {
			fmt.Fprintf(os.Stderr,
				"vpnkit: dropping %s=%q from mihomo unit — points at our own mixed-port %d (would deadlock)\n",
				k, v, mixedPort)
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func isNoProxyKey(k string) bool { return k == "NO_PROXY" || k == "no_proxy" }

// isSelfReferential reports whether a proxy URL targets a loopback host on
// the given port. Accepts socks5://, http://, https:// — any scheme really,
// because mihomo (and most http clients) parse them via url.Parse and read
// scheme+host:port. URLs missing a scheme or unparseable are treated as
// non-self-referential (safer to inject than drop).
func isSelfReferential(rawURL string, port int) bool {
	if port == 0 {
		return false
	}
	// url.Parse needs a scheme to populate Host reliably. Many users write
	// shells exports without one ("HTTPS_PROXY=127.0.0.1:7897"); also handle
	// the scheme-relative form ("//host:port") so it isn't double-prefixed.
	candidate := rawURL
	if strings.HasPrefix(candidate, "//") {
		candidate = "http:" + candidate
	} else if !strings.Contains(candidate, "://") {
		candidate = "http://" + candidate
	}
	u, err := url.Parse(candidate)
	if err != nil {
		return false
	}
	hostname, portStr := u.Hostname(), u.Port()
	if portStr == "" {
		return false
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p != port {
		return false
	}
	switch hostname {
	case "127.0.0.1", "localhost", "::1", "0.0.0.0":
		return true
	}
	return false
}
