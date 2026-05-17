package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"vpnkit/internal/config"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// defaultRuleTemplate is what new v2 stores get when no rule template is set.
// Matches the curated template list in internal/rules/templates/.
const defaultRuleTemplate = "loyalsoldier"

// runInitOpts groups the optional inputs to runInit.
type runInitOpts struct {
	RestorePath string // optional backup TOML to merge profiles from
	Force       bool   // back up any existing store before regenerating (v1 → v2 recovery)
}

// runInit creates ~/.config/vpnkit/config.toml and ~/.config/mihomo/config.yaml
// when missing and optionally restores a profiles section from a backup TOML.
// Idempotent.
func runInit(out io.Writer, opts runInitOpts) error {
	p := paths.Resolve()
	if err := p.Ensure(); err != nil {
		return fmt.Errorf("ensure dirs: %w", err)
	}

	fmt.Fprintln(out, "🛠️  vpnkit init")

	// Step 0: if --force, back up any existing store before re-creating it.
	if opts.Force {
		if _, err := os.Stat(p.VpnkitConfigFile()); err == nil {
			bak := fmt.Sprintf("%s.bak.%d", p.VpnkitConfigFile(), time.Now().Unix())
			if err := os.Rename(p.VpnkitConfigFile(), bak); err != nil {
				return fmt.Errorf("back up existing store: %w", err)
			}
			fmt.Fprintf(out, "🗄️  backed up old store to %s\n", bak)
		}
	}

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
	storeDirty := false
	if opts.RestorePath != "" && len(st.Cfg.LegacyProfiles) == 0 {
		var backup struct {
			Profiles []store.Profile `toml:"profiles"`
		}
		data, rerr := os.ReadFile(opts.RestorePath)
		if rerr != nil {
			fmt.Fprintf(out, "⚠️  failed to read backup %s: %v\n", opts.RestorePath, rerr)
		} else if err := toml.Unmarshal(data, &backup); err != nil {
			fmt.Fprintf(out, "⚠️  failed to parse backup %s: %v\n", opts.RestorePath, err)
		} else if len(backup.Profiles) > 0 {
			st.Cfg.LegacyProfiles = backup.Profiles
			storeDirty = true
			fmt.Fprintf(out, "📋 restored %d profile(s) from %s\n", len(backup.Profiles), opts.RestorePath)
		}
	}
	if storeDirty {
		if err := st.Save(); err != nil {
			return fmt.Errorf("save store: %w", err)
		}
	}

	// Step 3: generate mihomo config.yaml skeleton if missing.
	if !fileExists(p.MihomoConfigFile()) {
		ruleTemplate := st.Cfg.LegacyRuleTemplate
		if ruleTemplate == "" {
			ruleTemplate = defaultRuleTemplate // default for v2 stores that no longer persist this choice
		}
		data, err := config.BuildSkeleton(config.SkeletonInput{
			MixedPort:        st.Cfg.MixedPort,
			ControllerPort:   st.Cfg.ControllerPort,
			ControllerSecret: st.Cfg.ControllerSecret,
			RuleTemplate:     ruleTemplate,
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
