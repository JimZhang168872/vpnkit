// Package paths resolves XDG base directories and standard vpnkit / mihomo locations.
package paths

import (
	"os"
	"path/filepath"
)

// XDG holds resolved absolute paths for all directories vpnkit reads or writes.
type XDG struct {
	Home           string
	VpnkitConfig   string // ~/.config/vpnkit
	MihomoConfig   string // ~/.config/mihomo
	VpnkitState    string // ~/.local/state/vpnkit
	VpnkitCache    string // ~/.cache/vpnkit
	LocalBin       string // ~/.local/bin
	SystemdUserDir string // ~/.config/systemd/user
	RuntimeDir     string // $XDG_RUNTIME_DIR (may be empty)
}

// Resolve reads XDG environment variables, applying spec-defined fallbacks.
func Resolve() XDG {
	home := os.Getenv("HOME")
	configHome := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	stateHome := envOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	cacheHome := envOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	return XDG{
		Home:           home,
		VpnkitConfig:   filepath.Join(configHome, "vpnkit"),
		MihomoConfig:   filepath.Join(configHome, "mihomo"),
		VpnkitState:    filepath.Join(stateHome, "vpnkit"),
		VpnkitCache:    filepath.Join(cacheHome, "vpnkit"),
		LocalBin:       filepath.Join(home, ".local", "bin"),
		SystemdUserDir: filepath.Join(configHome, "systemd", "user"),
		RuntimeDir:     runtime,
	}
}

// Ensure creates all vpnkit-owned directories with 0o755.
// mihomo-owned dirs (ruleset/, profiles/) are created lazily by their respective subsystems.
func (p XDG) Ensure() error {
	dirs := []string{
		p.VpnkitConfig, p.MihomoConfig, p.VpnkitState, p.VpnkitCache, p.LocalBin, p.SystemdUserDir,
		filepath.Join(p.VpnkitCache, "downloads"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// MihomoBinary returns ~/.local/bin/mihomo.
func (p XDG) MihomoBinary() string { return filepath.Join(p.LocalBin, "mihomo") }

// VpnkitConfigFile returns ~/.config/vpnkit/config.toml.
func (p XDG) VpnkitConfigFile() string { return filepath.Join(p.VpnkitConfig, "config.toml") }

// MihomoConfigFile returns ~/.config/mihomo/config.yaml.
func (p XDG) MihomoConfigFile() string { return filepath.Join(p.MihomoConfig, "config.yaml") }

// PIDFile returns ~/.local/state/vpnkit/mihomo.pid.
func (p XDG) PIDFile() string { return filepath.Join(p.VpnkitState, "mihomo.pid") }

// MihomoLog returns ~/.local/state/vpnkit/mihomo.log.
func (p XDG) MihomoLog() string { return filepath.Join(p.VpnkitState, "mihomo.log") }

// VpnkitLog returns ~/.local/state/vpnkit/vpnkit.log.
func (p XDG) VpnkitLog() string { return filepath.Join(p.VpnkitState, "vpnkit.log") }

// SystemdUnit returns ~/.config/systemd/user/mihomo.service.
func (p XDG) SystemdUnit() string { return filepath.Join(p.SystemdUserDir, "mihomo.service") }

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
