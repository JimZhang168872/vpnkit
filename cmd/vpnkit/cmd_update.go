package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"vpnkit/internal/installer"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
	"vpnkit/internal/updater"
)

// updateOptions configures runUpdate.
type updateOptions struct {
	Check      bool // print plan + exit 0
	VpnkitOnly bool
	MihomoOnly bool
	Yes        bool // skip interactive confirm
	NoExec     bool // don't syscall.Exec after vpnkit upgrade (tests)
}

// updateAPIBase is overrideable in tests to point at a mock GitHub API server.
var updateAPIBase = ""

// runUpdate is the body of `vpnkit update`. Exit codes are encoded as
// errors that dispatcher maps to sys.Exit.
//
//	nil  → success (or already up to date)
//	err  → caller dies with code 2 (runtime)
func runUpdate(out io.Writer, opts updateOptions, st *store.Store, currentVpnkitVer string) error {
	p := paths.Resolve()

	// 1. Determine current mihomo version.
	mihomoCur := readMihomoVersion(p.MihomoBinary())

	// 2. Check latest.
	fmt.Fprintln(out, "🔎 checking for updates …")
	info, err := updater.Check(updater.Opts{
		VpnkitCurrent: currentVpnkitVer,
		MihomoCurrent: mihomoCur,
		APIBase:        updateAPIBase,
	})
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}

	// 3. Apply scope filter from flags.
	if opts.VpnkitOnly {
		info.MihomoNeedsUpdate = false
	}
	if opts.MihomoOnly {
		info.VpnkitNeedsUpdate = false
	}

	// 4. Print plan.
	printPlan(out, info)

	if opts.Check || !info.HasUpdate() {
		return nil
	}

	if !opts.Yes {
		fmt.Fprint(out, "proceed? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read confirmation: %w", err)
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	// 5. Do the upgrades.
	if info.MihomoNeedsUpdate {
		if err := upgradeMihomo(out, p, st, info.MihomoLatest); err != nil {
			return fmt.Errorf("mihomo upgrade: %w", err)
		}
	}
	if info.VpnkitNeedsUpdate {
		if err := upgradeVpnkit(out, p, info.VpnkitLatest); err != nil {
			return fmt.Errorf("vpnkit upgrade: %w", err)
		}
		fmt.Fprintln(out, "🎉 vpnkit upgraded — re-exec'ing …")
		if !opts.NoExec {
			// This call does not return on success.
			if err := updater.ExecSelf(); err != nil {
				fmt.Fprintf(out, "⚠️  re-exec failed (%v) — run `vpnkit` again to pick up the new binary\n", err)
			}
		}
		return nil
	}

	fmt.Fprintln(out, "🎉 done")
	return nil
}

func printPlan(out io.Writer, i updater.Info) {
	switch {
	case i.VpnkitNeedsUpdate && i.MihomoNeedsUpdate:
		fmt.Fprintf(out, "📦 vpnkit %s → %s\n", i.VpnkitCurrent, i.VpnkitLatest)
		fmt.Fprintf(out, "📦 mihomo %s → %s\n", i.MihomoCurrent, i.MihomoLatest)
	case i.VpnkitNeedsUpdate:
		fmt.Fprintf(out, "📦 vpnkit %s → %s\n", i.VpnkitCurrent, i.VpnkitLatest)
		if i.MihomoCurrent != "" {
			fmt.Fprintf(out, "✅ mihomo already at %s\n", i.MihomoCurrent)
		}
	case i.MihomoNeedsUpdate:
		fmt.Fprintf(out, "📦 mihomo %s → %s\n", i.MihomoCurrent, i.MihomoLatest)
		if i.VpnkitCurrent != "" {
			fmt.Fprintf(out, "✅ vpnkit already at %s\n", i.VpnkitCurrent)
		}
	default:
		fmt.Fprintln(out, "✅ already up to date")
	}
}

func upgradeMihomo(out io.Writer, p paths.XDG, st *store.Store, version string) error {
	fmt.Fprintf(out, "📥 downloading mihomo %s …\n", version)
	svc := service.New(service.Mode(st.Cfg.ServiceMode), service.Config{
		BinaryPath:  p.MihomoBinary(),
		ConfigDir:   p.MihomoConfig,
		PIDFilePath: p.PIDFile(),
		LogFilePath: p.MihomoLog(),
		UnitPath:    p.SystemdUnit(),
		MixedPort:   st.Cfg.MixedPort,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wasRunning := false
	if status, err := svc.Status(ctx); err == nil && status.Running {
		wasRunning = true
		fmt.Fprintln(out, "🛑 stopping mihomo for binary swap …")
		_ = svc.Stop(ctx)
	}
	_, err := installer.Install(installer.Options{
		Dst:     p.MihomoBinary(),
		Version: version,
		APIBase: updateAPIBase,
	}, nil)
	if err != nil {
		return err
	}
	if wasRunning {
		fmt.Fprintln(out, "▶️  restarting mihomo …")
		if err := svc.Start(ctx); err != nil {
			return fmt.Errorf("restart: %w", err)
		}
	}
	fmt.Fprintf(out, "✅ mihomo upgraded to %s\n", version)
	return nil
}

func upgradeVpnkit(out io.Writer, p paths.XDG, version string) error {
	dst, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate vpnkit binary: %w", err)
	}
	if resolved, err := os.Readlink(dst); err == nil && resolved != "" {
		dst = resolved
	}
	arch := updater.CurrentArch()
	tarball := fmt.Sprintf("vpnkit_%s_linux_%s.tar.gz", strings.TrimPrefix(version, "v"), arch)
	githubURL := fmt.Sprintf("https://github.com/JimZhang168872/vpnkit/releases/download/%s/%s", version, tarball)
	fmt.Fprintf(out, "📥 downloading vpnkit %s (%s) …\n", version, tarball)
	if err := updater.DownloadAndApplyVpnkit(githubURL, "", dst); err != nil {
		return err
	}
	fmt.Fprintf(out, "✅ vpnkit %s → %s\n", version, dst)
	return nil
}

// readMihomoVersion runs `mihomo -v` and parses the first line. Returns ""
// if the binary is missing or output unparseable.
func readMihomoVersion(binary string) string {
	if _, err := os.Stat(binary); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-v").Output()
	if err != nil {
		return ""
	}
	return parseMihomoLine(string(out))
}

// parseMihomoLine inlines the version-token parser; updater.parseMihomoVersion
// is unexported.
func parseMihomoLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	for _, f := range strings.Fields(s) {
		if strings.HasPrefix(f, "v") && len(f) > 1 && (f[1] >= '0' && f[1] <= '9') {
			return f
		}
	}
	return ""
}
