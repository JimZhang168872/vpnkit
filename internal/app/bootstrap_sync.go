package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"vpnkit/internal/config"
	"vpnkit/internal/installer"
	"vpnkit/internal/paths"
	"vpnkit/internal/rules"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
)

// NewServiceManager builds a service.Manager for the given paths + store,
// picking systemd-user or PID mode via service.Detect when the store has no
// explicit choice. The detected mode is persisted back into the store.
//
// Shared by app.Run (TUI startup) and dispatchInit (CLI bootstrap) so the
// two paths can't drift on backend selection.
func NewServiceManager(p paths.XDG, st *store.Store) service.Manager {
	if st.Cfg.ServiceMode == "" {
		mode := service.Detect(nil)
		st.Cfg.ServiceMode = string(mode)
		_ = st.Save()
	}
	return service.New(service.Mode(st.Cfg.ServiceMode), service.Config{
		BinaryPath:  p.MihomoBinary(),
		ConfigDir:   p.MihomoConfig,
		PIDFilePath: p.PIDFile(),
		LogFilePath: p.MihomoLog(),
		UnitPath:    p.SystemdUnit(),
		MixedPort:   st.Cfg.MixedPort,
	})
}

// RunBootstrap performs first-run setup synchronously and writes progress
// lines to `out`. It is safe to call repeatedly — every step is idempotent
// (skips when the artifact already exists).
//
// Steps:
//  1. ensure XDG dirs
//  2. download mihomo binary if missing
//  3. write mihomo config.yaml skeleton if missing
//  4. pre-seed GeoIP / GeoSite (non-fatal on error)
//  5. pre-seed rule-set text files (non-fatal on error)
//  6. install service (systemd unit or PID no-op)
//  7. start the service
//  8. wait briefly + verify mihomo is actually running
//
// Failure at steps 1/2/6/8 returns an error. Steps 4/5 emit warnings.
func RunBootstrap(ctx context.Context, d BootstrapDeps, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}

	// Step 1: ensure dirs.
	if err := d.Paths.Ensure(); err != nil {
		return fmt.Errorf("paths: %w", err)
	}

	// Step 2: install mihomo if missing.
	if _, err := os.Stat(d.Paths.MihomoBinary()); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(out, "📥 downloading mihomo (one-time, can take a minute on slow links)…")
		install := d.InstallFunc
		if install == nil {
			install = installer.Install
		}
		res, err := install(installer.Options{
			Dst:     d.Paths.MihomoBinary(),
			APIBase: "",
		}, nil)
		if err != nil {
			return fmt.Errorf("install mihomo: %w", err)
		}
		fmt.Fprintf(out, "✅ mihomo %s installed → %s\n", res.Version, d.Paths.MihomoBinary())
	} else {
		fmt.Fprintf(out, "✅ mihomo binary already present (%s)\n", d.Paths.MihomoBinary())
	}

	// Step 3: config.yaml skeleton if missing. cmd_init usually wrote this
	// already, but TUI-first-launch and "I rm'd config.yaml" paths still
	// need this fallback.
	if _, err := os.Stat(d.Paths.MihomoConfigFile()); errors.Is(err, fs.ErrNotExist) {
		data, err := config.BuildSkeleton(config.SkeletonInput{
			MixedPort:        d.Store.Cfg.MixedPort,
			ControllerPort:   d.Store.Cfg.ControllerPort,
			ControllerSecret: d.Store.Cfg.ControllerSecret,
			RuleTemplate:     d.Store.Cfg.LegacyRuleTemplate,
			ProxyUser:        d.Store.Cfg.ProxyUser,
			ProxyPass:        d.Store.Cfg.ProxyPass,
		})
		if err != nil {
			return fmt.Errorf("build skeleton: %w", err)
		}
		if err := config.AtomicWrite(d.Paths.MihomoConfigFile(), data, 0o600); err != nil {
			return fmt.Errorf("write mihomo config: %w", err)
		}
		fmt.Fprintf(out, "✅ %s (created)\n", d.Paths.MihomoConfigFile())
	}

	// Step 4: pre-seed GeoIP / GeoSite. Non-fatal — mihomo can fetch on its
	// own at startup. We pre-fetch here because mihomo's built-in downloader
	// has a hardcoded 90s deadline and ignores HTTP(S)_PROXY, so on China
	// networks the first launch deadlocks. SmartClient respects env proxies.
	if _, err := installer.EnsureGeo(d.Paths.MihomoConfig, nil); err != nil {
		fmt.Fprintf(out, "⚠️  geo pre-seed had errors (non-fatal — mihomo will retry on launch): %v\n", err)
	} else {
		fmt.Fprintln(out, "✅ geo files seeded")
	}

	// Step 5: pre-seed embedded rule-set snapshots. Non-fatal.
	if _, err := rules.WriteRulesetsTo(filepath.Join(d.Paths.MihomoConfig, "ruleset")); err != nil {
		fmt.Fprintf(out, "⚠️  ruleset seed had errors (non-fatal — mihomo will fetch on demand): %v\n", err)
	} else {
		fmt.Fprintln(out, "✅ rulesets seeded")
	}

	// Step 6: install the service (writes systemd unit, or no-op for PID).
	fmt.Fprintf(out, "🔧 installing %s service backend…\n", d.Service.Mode())
	if err := d.Service.Install(ctx); err != nil {
		return fmt.Errorf("service install: %w", err)
	}

	// Step 7: start the service. systemd Install already did `enable --now`,
	// so a subsequent Start is either a no-op or a harmless "already started".
	// PID Install is a no-op so Start is the actual launch.
	if err := d.Service.Start(ctx); err != nil {
		// Treat "already running" as success: systemd-user's `enable --now`
		// in step 6 may have already started the unit, so a second `start`
		// is allowed to noop-fail.
		fmt.Fprintf(out, "ℹ️  service start: %v (continuing — checking status next)\n", err)
	}

	// Step 8: wait, then verify.
	time.Sleep(3 * time.Second)
	status, _ := d.Service.Status(ctx)
	if !status.Running {
		var tail string
		if r, err := d.Service.Logs(ctx, false); err == nil && r != nil {
			if data, derr := io.ReadAll(io.LimitReader(r, 4096)); derr == nil {
				tail = string(data)
			}
			_ = r.Close()
		}
		return fmt.Errorf("mihomo not running after start (mode=%s). Last log:\n%s", d.Service.Mode(), tail)
	}
	fmt.Fprintf(out, "✅ mihomo running (mode=%s, pid=%d)\n", status.Mode, status.PID)
	return nil
}
