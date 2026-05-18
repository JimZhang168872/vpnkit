package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"vpnkit/internal/config"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// defaultRuleTemplate is what new v2 stores get when no rule template is set.
// Matches the curated template list in internal/rules/templates/.
const defaultRuleTemplate = "loyalsoldier"

// runInitOpts groups the optional inputs to runInit.
type runInitOpts struct {
	Force bool // back up any existing store before regenerating (v1 → v2 recovery)
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

	// Step 0: if --force, back up the existing store AND remove the
	// stale mihomo config.yaml. Without the YAML wipe, the next launch
	// re-uses the OLD proxy-groups even though the store is empty —
	// users get "I reset everything but mihomo still routes through
	// my deleted subscription." The Step-2 branch below will then
	// rebuild a fresh skeleton from the (now empty) store.
	if opts.Force {
		if _, err := os.Stat(p.VpnkitConfigFile()); err == nil {
			// UnixNano so two init --force calls within the same second
			// don't produce identical backup names and clobber each
			// other. QA found `bak.<unix-seconds>` collisions silently
			// losing the first backup when users panic-ran init twice.
			bak := fmt.Sprintf("%s.bak.%d", p.VpnkitConfigFile(), time.Now().UnixNano())
			if err := os.Rename(p.VpnkitConfigFile(), bak); err != nil {
				return fmt.Errorf("back up existing store: %w", err)
			}
			fmt.Fprintf(out, "🗄️  backed up old store to %s\n", bak)
		}
		// Also stash the old mihomo config.yaml alongside, so a freshly
		// generated skeleton can land cleanly without inheriting stale
		// proxy/proxy-groups/rules sections.
		if _, err := os.Stat(p.MihomoConfigFile()); err == nil {
			bak := fmt.Sprintf("%s.bak.%d", p.MihomoConfigFile(), time.Now().UnixNano())
			if err := os.Rename(p.MihomoConfigFile(), bak); err != nil {
				return fmt.Errorf("back up old mihomo config.yaml: %w", err)
			}
			fmt.Fprintf(out, "🗄️  backed up old mihomo config.yaml to %s\n", bak)
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

	// Step 2: generate mihomo config.yaml skeleton if missing.
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
