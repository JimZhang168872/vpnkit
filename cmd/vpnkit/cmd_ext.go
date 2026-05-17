package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"vpnkit/internal/paths"
	"vpnkit/internal/profiles"
	"vpnkit/internal/store"
)

type runExtApplyDeps struct {
	ExtensionsPath string
	ActiveProfile  string
	Reassemble     func() error // typically profMgr.Update(ctx, active)
	Reload         func() error // typically client.ReloadConfig(ctx, "")
}

func runExtApply(out io.Writer, d runExtApplyDeps) error {
	if d.ActiveProfile == "" {
		return fmt.Errorf("no active profile — set one with `vpnkit use <group> <node>` (or the TUI) and try again")
	}
	if err := d.Reassemble(); err != nil {
		return fmt.Errorf("reassemble: %w", err)
	}
	if err := d.Reload(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}
	fmt.Fprintln(out, "applied: subscription reassembled with extensions and mihomo reloaded")
	return nil
}

func dispatchExt(args []string) {
	if len(args) < 1 || args[0] != "apply" {
		dieUserErr("vpnkit ext: usage: vpnkit ext apply")
	}
	p := paths.Resolve()
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit ext apply: %v", err)
	}
	if st.Cfg.LegacyActiveProfile == "" {
		dieUserErr("vpnkit ext apply: no active profile — set one first")
	}
	mgr := profiles.New(profiles.Config{
		ConfigYAMLPath:   p.MihomoConfigFile(),
		ExtensionsPath:   extensionsPath(),
		ControllerPort:   st.Cfg.ControllerPort,
		ControllerSecret: st.Cfg.ControllerSecret,
		MixedPort:        st.Cfg.MixedPort,
		RuleTemplate:     st.Cfg.LegacyRuleTemplate,
		ProxyUser:        st.Cfg.ProxyUser,
		ProxyPass:        st.Cfg.ProxyPass,
	})
	mgr.Load(toProfilesProfilesCLI(st.Cfg.LegacyProfiles), st.Cfg.LegacyActiveProfile)

	client, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit ext apply: mihomo not reachable: %v", err)
	}

	deps := runExtApplyDeps{
		ExtensionsPath: extensionsPath(),
		ActiveProfile:  st.Cfg.LegacyActiveProfile,
		Reassemble: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := mgr.Update(ctx, st.Cfg.LegacyActiveProfile)
			return err
		},
		Reload: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return client.ReloadConfig(ctx, "")
		},
	}
	if err := runExtApply(os.Stdout, deps); err != nil {
		dieRuntime("vpnkit ext apply: %v", err)
	}
}

func toProfilesProfilesCLI(in []store.Profile) []profiles.Profile {
	out := make([]profiles.Profile, len(in))
	for i, x := range in {
		out[i] = profiles.Profile{
			Name: x.Name, URL: x.URL, UserAgent: x.UserAgent, LastUpdated: x.LastUpdated,
		}
	}
	return out
}
