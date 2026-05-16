package main

import (
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"
	"vpnkit/internal/config"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// runInit creates ~/.config/vpnkit/config.toml and ~/.config/mihomo/config.yaml
// when missing, then optionally restores a profiles section from a backup TOML
// at restorePath (produced by `vpnkit uninstall --keep-profiles`). Idempotent.
func runInit(out io.Writer, restorePath string) error {
	p := paths.Resolve()
	if err := p.Ensure(); err != nil {
		return fmt.Errorf("ensure dirs: %w", err)
	}

	fmt.Fprintln(out, "🛠️  vpnkit init")

	// Step 1: load (creates with defaults if missing).
	tomlExisted := fileExists(p.VpnkitConfigFile())
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	if tomlExisted {
		fmt.Fprintf(out, "✅ %s (kept)\n", p.VpnkitConfigFile())
	} else {
		fmt.Fprintf(out, "✅ %s (created)\n", p.VpnkitConfigFile())
	}

	// Step 2: restore profiles if a backup was passed AND store has none yet.
	// We do not overwrite a user's existing profiles to avoid double-counting.
	if restorePath != "" && len(st.Cfg.Profiles) == 0 {
		var backup struct {
			Profiles []store.Profile `toml:"profiles"`
		}
		data, rerr := os.ReadFile(restorePath)
		if rerr != nil {
			fmt.Fprintf(out, "⚠️  failed to read backup %s: %v\n", restorePath, rerr)
		} else if err := toml.Unmarshal(data, &backup); err != nil {
			fmt.Fprintf(out, "⚠️  failed to parse backup %s: %v\n", restorePath, err)
		} else if len(backup.Profiles) > 0 {
			st.Cfg.Profiles = backup.Profiles
			if err := st.Save(); err != nil {
				return fmt.Errorf("save profiles: %w", err)
			}
			fmt.Fprintf(out, "📋 restored %d profile(s) from %s\n", len(backup.Profiles), restorePath)
		}
	}

	// Step 3: generate mihomo config.yaml skeleton if missing.
	if !fileExists(p.MihomoConfigFile()) {
		data, err := config.BuildSkeleton(config.SkeletonInput{
			MixedPort:        st.Cfg.MixedPort,
			ControllerPort:   st.Cfg.ControllerPort,
			ControllerSecret: st.Cfg.ControllerSecret,
			RuleTemplate:     st.Cfg.RuleTemplate,
			ReleaseMirror:    st.Cfg.ReleaseMirror,
			ProxyUser:        st.Cfg.ProxyUser,
			ProxyPass:        st.Cfg.ProxyPass,
		})
		if err != nil {
			return fmt.Errorf("build skeleton: %w", err)
		}
		if err := config.AtomicWrite(p.MihomoConfigFile(), data, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", p.MihomoConfigFile(), err)
		}
		fmt.Fprintf(out, "✅ %s (created)\n", p.MihomoConfigFile())
	} else {
		// Already exists — sync the security-owned fields in case toml was just
		// regenerated (same logic as app.Run startup path).
		changed, err := config.EnsureSecurityFields(p.MihomoConfigFile(), config.SecurityFields{
			MixedPort:        st.Cfg.MixedPort,
			ControllerPort:   st.Cfg.ControllerPort,
			ControllerSecret: st.Cfg.ControllerSecret,
			ProxyUser:        st.Cfg.ProxyUser,
			ProxyPass:        st.Cfg.ProxyPass,
		})
		if err != nil {
			fmt.Fprintf(out, "⚠️  could not reconcile %s: %v\n", p.MihomoConfigFile(), err)
		} else if changed {
			fmt.Fprintf(out, "🔄 %s (security fields synced)\n", p.MihomoConfigFile())
		} else {
			fmt.Fprintf(out, "✅ %s (kept)\n", p.MihomoConfigFile())
		}
	}

	fmt.Fprintln(out, "🎉 ready — run `vpnkit` to start")
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
