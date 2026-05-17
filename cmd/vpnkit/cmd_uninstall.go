package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
)

// uninstallOptions configures runUninstall.
type uninstallOptions struct {
	Yes        bool   // skip interactive prompt
	Purge      bool   // also delete profiles (no backup)
	KeepMihomo bool   // do not delete ~/.local/bin/mihomo
	BackupDir  string // where to write the profiles backup (default: /tmp)
}

// runUninstall stops the service and removes vpnkit-owned files. Always
// best-effort: a single failure logs but does not stop the rest. The output
// includes a "BACKUP=<path>" line when a profiles backup was created, for
// install.sh to grep.
func runUninstall(out io.Writer, opts uninstallOptions) error {
	p := paths.Resolve()
	// Safety guard: refuse to operate when HOME is unset or relative — the
	// uninstall does os.RemoveAll on each XDG-derived path and we never want
	// to wipe a relative path from cwd.
	if p.Home == "" || !filepath.IsAbs(p.Home) {
		return fmt.Errorf("HOME is unset or not absolute (%q); refusing to run uninstall", p.Home)
	}
	if opts.BackupDir == "" {
		opts.BackupDir = "/tmp"
	}

	if !opts.Yes {
		fmt.Fprintln(out, "🗑️  vpnkit uninstall — will remove:")
		fmt.Fprintln(out, "    🛑 mihomo service")
		fmt.Fprintf(out, "    %s\n", p.SystemdUnit())
		fmt.Fprintf(out, "    %s\n", p.MihomoConfig)
		fmt.Fprintf(out, "    %s\n", p.VpnkitConfig)
		fmt.Fprintf(out, "    %s\n", p.VpnkitState)
		fmt.Fprintf(out, "    %s\n", p.VpnkitCache)
		fmt.Fprintf(out, "    %s\n", filepath.Join(p.LocalBin, "vpnkit"))
		if !opts.KeepMihomo {
			fmt.Fprintf(out, "    %s\n", filepath.Join(p.LocalBin, "mihomo"))
		}
		if opts.Purge {
			fmt.Fprintln(out, "    ⚠️  including profiles (--purge)")
		} else {
			fmt.Fprintln(out, "    📦 profiles will be backed up to "+opts.BackupDir)
		}
		fmt.Fprint(out, "continue? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintf(out, "❌ failed to read confirmation: %v\n", err)
			return nil
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	// Step 1: backup profiles unless purging.
	backupPath := ""
	if !opts.Purge && fileExists(p.VpnkitConfigFile()) {
		bp, err := backupProfiles(p.VpnkitConfigFile(), opts.BackupDir)
		if err != nil {
			fmt.Fprintf(out, "⚠️  could not back up profiles: %v\n", err)
		} else if bp != "" {
			backupPath = bp
			fmt.Fprintf(out, "📦 backed up profiles → %s\n", bp)
		}
	}

	// Step 2: stop the systemd-user / PID service if running.
	// Determine mode from store (best-effort).
	mode := service.ModeSystemdUser
	if st, err := store.Load(p.VpnkitConfigFile()); err == nil && st.Cfg.ServiceMode != "" {
		mode = service.Mode(st.Cfg.ServiceMode)
	}
	svc := service.New(mode, service.Config{
		BinaryPath:  p.MihomoBinary(),
		ConfigDir:   p.MihomoConfig,
		PIDFilePath: p.PIDFile(),
		LogFilePath: p.MihomoLog(),
		UnitPath:    p.SystemdUnit(),
		// MixedPort intentionally omitted: uninstall only Stop/Uninstalls,
		// never renders a fresh unit, so the value is unused.
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := svc.Stop(ctx); err != nil && !isNotRunning(err) {
		fmt.Fprintf(out, "⚠️  stop mihomo: %v\n", err)
	} else {
		fmt.Fprintln(out, "🛑 stopped mihomo")
	}
	if err := svc.Uninstall(ctx); err != nil {
		fmt.Fprintf(out, "⚠️  uninstall service: %v\n", err)
	} else {
		fmt.Fprintln(out, "🧹 removed systemd unit")
	}

	// Step 3: remove file paths.
	toRemove := []string{
		p.MihomoConfig,
		p.VpnkitConfig,
		p.VpnkitState,
		p.VpnkitCache,
		filepath.Join(p.LocalBin, "vpnkit"),
	}
	if !opts.KeepMihomo {
		toRemove = append(toRemove, filepath.Join(p.LocalBin, "mihomo"))
	}
	for _, path := range toRemove {
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintf(out, "⚠️  remove %s: %v\n", path, err)
		} else {
			fmt.Fprintf(out, "🗑️  removed %s\n", path)
		}
	}

	if backupPath != "" {
		// Machine-parseable line for install.sh.
		fmt.Fprintf(out, "BACKUP=%s\n", backupPath)
	}
	fmt.Fprintln(out, "🎉 uninstalled")
	return nil
}

// backupProfiles parses the vpnkit config.toml, extracts the [[profiles]]
// array, and writes a standalone TOML file containing only the profiles
// section. Returns the backup path, or empty string when there are no
// profiles to back up. Uses the same toml lib as the rest of the project
// so non-trivial values (URLs containing brackets, multi-line strings, etc.)
// round-trip correctly.
func backupProfiles(tomlPath, dir string) (string, error) {
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return "", err
	}
	var src struct {
		Profiles []store.Profile `toml:"profiles"`
	}
	if err := toml.Unmarshal(data, &src); err != nil {
		return "", fmt.Errorf("parse %s: %w", tomlPath, err)
	}
	if len(src.Profiles) == 0 {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(dir, fmt.Sprintf("vpnkit-profiles-%s.toml", time.Now().Format("20060102-150405")))
	f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(src); err != nil {
		return "", err
	}
	return out, nil
}

func isNotRunning(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, service.ErrNotRunning) || strings.Contains(err.Error(), "not running")
}
