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
	"vpnkit/internal/netx"
	"vpnkit/internal/paths"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
	"vpnkit/internal/updater"
)

// updateOptions configures runUpdate.
type updateOptions struct {
	Check       bool // print plan + exit 0
	VpnkitOnly  bool
	MihomoOnly  bool
	Yes         bool // skip interactive confirm
	NoExec      bool // don't syscall.Exec after vpnkit upgrade (tests)
}

// runUpdate is the body of `vpnkit update`. Exit codes are encoded as
// errors that dispatcher maps to sys.Exit.
//   nil  → success (or already up to date)
//   err  → caller dies with code 2 (runtime)
func runUpdate(out io.Writer, opts updateOptions, st *store.Store, currentVpnkitVer string) error {
	p := paths.Resolve()

	// 1. Determine current mihomo version.
	mihomoCur := readMihomoVersion(p.MihomoBinary())

	// 2. Check latest.
	fmt.Fprintln(out, "🔎 checking for updates …")
	info, err := updater.Check(updater.Opts{
		VpnkitCurrent: currentVpnkitVer,
		MihomoCurrent: mihomoCur,
		APIBase:       prefixedAPIBase(st.Cfg.ReleaseMirror),
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
		if err := upgradeVpnkit(out, p, st, info.VpnkitLatest); err != nil {
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
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wasRunning := false
	if status, err := svc.Status(ctx); err == nil && status.Running {
		wasRunning = true
		fmt.Fprintln(out, "🛑 stopping mihomo for binary swap …")
		_ = svc.Stop(ctx)
	}
	res, err := installer.Install(installer.Options{
		Dst:       p.MihomoBinary(),
		Mirror:    st.Cfg.ReleaseMirror,
		Version:   version,
		OnAttempt: mirrorAttemptPrinter(out),
	}, nil)
	if err != nil {
		return err
	}
	cacheWinningMirror(out, st, res.Mirror)
	if wasRunning {
		fmt.Fprintln(out, "▶️  restarting mihomo …")
		if err := svc.Start(ctx); err != nil {
			return fmt.Errorf("restart: %w", err)
		}
	}
	fmt.Fprintf(out, "✅ mihomo upgraded to %s\n", version)
	return nil
}

func upgradeVpnkit(out io.Writer, p paths.XDG, st *store.Store, version string) error {
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
	winningMirror, err := updater.DownloadAndApplyVpnkit(githubURL, "", dst, st.Cfg.ReleaseMirror, mirrorAttemptPrinter(out))
	if err != nil {
		return err
	}
	cacheWinningMirror(out, st, winningMirror)
	fmt.Fprintf(out, "✅ vpnkit %s → %s\n", version, dst)
	return nil
}

// mirrorAttemptPrinter returns an OnAttempt callback that prints each chain
// attempt's outcome to out as the download walks its fallback chain. So a
// user staring at "📥 downloading mihomo …" sees in real time which mirror
// is being tried, why each is failing, and which one finally worked —
// instead of waiting silently for ~60 s and getting a single misleading
// "ghp.ci: no such host" at the end.
func mirrorAttemptPrinter(out io.Writer) netx.OnAttempt {
	return func(mirror string, err error) {
		label := mirror
		if label == "" {
			label = "github direct"
		} else {
			label = strings.TrimSuffix(strings.TrimPrefix(label, "https://"), "/")
		}
		if err == nil {
			fmt.Fprintf(out, "   ✓ %s\n", label)
			return
		}
		// Trim verbose net error wrapping for a readable single line.
		msg := err.Error()
		if i := strings.Index(msg, ": "); i >= 0 && i < 60 {
			// drop one layer of wrapping if any
		}
		if len(msg) > 100 {
			msg = msg[:100] + "…"
		}
		fmt.Fprintf(out, "   ✗ %s — %s\n", label, msg)
	}
}

// cacheWinningMirror remembers a non-empty mirror that just served a download
// when the user hadn't configured one. Saves the next launch's bootstrap from
// waiting on the same dead direct connection. Never overwrites a user-set
// value — they might be deliberately pinning a specific mirror.
func cacheWinningMirror(out io.Writer, st *store.Store, mirror string) {
	if mirror == "" || st.Cfg.ReleaseMirror != "" {
		return
	}
	st.Cfg.ReleaseMirror = mirror
	if err := st.Save(); err != nil {
		fmt.Fprintf(out, "⚠️  cache mirror %s: %v\n", mirror, err)
		return
	}
	fmt.Fprintf(out, "💾 cached release_mirror = %s (next download starts here)\n", mirror)
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
	// Use the exported helper; it strips to the first version-looking token.
	return parseMihomoLine(string(out))
}

// parseMihomoLine is a tiny wrapper around updater.parseMihomoVersion that
// lives here because the updater pkg keeps that helper unexported. We
// inline the logic to avoid widening the updater public surface.
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

// prefixedAPIBase returns "https://api.github.com" wrapped in the configured
// release mirror, if any. Returns "" when no mirror so updater uses its default.
func prefixedAPIBase(mirror string) string {
	if mirror == "" {
		return ""
	}
	m := strings.TrimSuffix(mirror, "/")
	return m + "/" + "https://api.github.com"
}
