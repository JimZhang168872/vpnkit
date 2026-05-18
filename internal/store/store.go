// Package store reads and writes vpnkit's own config file (~/.config/vpnkit/config.toml).
package store

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// Profile records one subscription entry (legacy v1 schema).
type Profile struct {
	Name        string    `toml:"name"`
	URL         string    `toml:"url"`
	UserAgent   string    `toml:"user_agent,omitempty"`
	LastUpdated time.Time `toml:"last_updated,omitempty"`
}

// Subscription is a remote proxy subscription feed (v2 schema).
type Subscription struct {
	Name        string    `toml:"name"`
	URL         string    `toml:"url"`
	UserAgent   string    `toml:"user_agent,omitempty"`
	Enabled     bool      `toml:"enabled"`
	LastUpdated time.Time `toml:"last_updated,omitempty"`
	NodeCount   int       `toml:"node_count,omitempty"`
}

// LocalNodeGroup is a named collection of local nodes (v2 schema, rc.3+).
type LocalNodeGroup struct {
	Name    string `toml:"name"`
	Enabled bool   `toml:"enabled"`
}

// LocalNode is a manually configured proxy node (v2 schema).
//
// Group is REQUIRED — callers that construct LocalNode values for Save must
// populate Group with a name that exists in Config.LocalNodeGroups. The
// `omitempty` tag is intentional: it keeps rc.2 stores (which had no Group
// field) round-trippable, and it lets Load() detect un-migrated nodes by
// the empty-Group sentinel. Leaving Group="" on Save is treated as
// "needs migration to default 'local' group" and silently corrected on
// next Load.
//
// Via is optional and writes mihomo's dialer-proxy field on this node at
// assemble time.
type LocalNode struct {
	Name   string         `toml:"name"`
	Group  string         `toml:"group,omitempty"`
	Via    string         `toml:"via,omitempty"`
	Proto  string         `toml:"proto"`
	Server string         `toml:"server"`
	Port   int            `toml:"port"`
	Fields map[string]any `toml:"fields,omitempty"`
}

// LocalRule is a manually configured routing rule (v2 schema).
type LocalRule struct {
	Type    string `toml:"type"`
	Payload string `toml:"payload"`
	Target  string `toml:"target"`
}

// Config is vpnkit's persisted configuration.
type Config struct {
	SchemaVersion int `toml:"schema_version"`

	ControllerSecret string `toml:"controller_secret"`
	ControllerPort   int    `toml:"controller_port"`
	MixedPort        int    `toml:"mixed_port"`
	ProxyUser        string `toml:"proxy_user"`
	ProxyPass        string `toml:"proxy_pass"`
	UITheme          string `toml:"ui_theme"`
	ServiceMode      string `toml:"service_mode,omitempty"`

	Mode string `toml:"mode"`
	// GlobalTarget overrides which 🚀 Proxy Selector member becomes the
	// "now" default. With the rc.7+ active-source model, this is normally
	// derived from ActiveSource ("<active>-auto"). Kept as a separate
	// field for advanced overrides and backwards-compat migration.
	GlobalTarget string `toml:"global_target"`
	// ActiveSource (rc.7+) names the one source (subscription OR local-
	// node group) whose rules drive routing AND whose nodes back 🚀 Proxy.
	// Empty falls back to "first enabled subscription, else first enabled
	// local group". User intent "选谁用谁" — flip the active source and
	// the entire rules + proxy graph follows. See assembler.Input for
	// details.
	ActiveSource string `toml:"active_source,omitempty"`

	Subscriptions   []Subscription   `toml:"subscriptions"`
	LocalNodes      []LocalNode      `toml:"local_nodes"`
	LocalNodeGroups []LocalNodeGroup `toml:"local_node_groups"`
	LocalRules      []LocalRule      `toml:"local_rules"`

	// Legacy fields below are detection-only for LegacyActiveProfile and
	// LegacyProfiles (read in Load to detect v1 stores; not consumed by any
	// post-Phase-1 logic). LegacyRuleTemplate is the exception: it is still
	// actively written and read through Phase 5 — see comment below.
	LegacyActiveProfile string    `toml:"active_profile,omitempty"`
	LegacyProfiles      []Profile `toml:"profiles,omitempty"`
	// LegacyRuleTemplate kept as a *live* field through Phase 5 of the v1
	// migration: internal/tabs/settings/rules.go writes the user's chosen
	// template here, and internal/app/{run,bootstrap}.go + cmd/vpnkit/cmd_init.go
	// read it. Phase 6 will move this to a v2 home (likely on Subscription or a
	// new global setting); when deleting this field, redirect those callers.
	LegacyRuleTemplate string `toml:"rule_template,omitempty"`
}

// Store wraps a Config and its on-disk location.
type Store struct {
	path string
	mu   sync.Mutex
	Cfg  Config
}

// Load reads `path`. If the file does not exist, defaults are written and returned.
func Load(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		s.Cfg = defaults()
		if err := s.Save(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, &s.Cfg); err != nil {
		return nil, err
	}
	// Apply defaults for any zero-value fields the caller relies on. Persist
	// the result so generated creds survive across launches (otherwise users
	// upgrading from a pre-auth version would see fresh creds every run).
	changed := false
	if s.Cfg.SchemaVersion == 0 && (s.Cfg.LegacyActiveProfile != "" || len(s.Cfg.LegacyProfiles) > 0 || s.Cfg.LegacyRuleTemplate != "") {
		return nil, fmt.Errorf("store at %s uses schema v1 (vpnkit ≤ v0.10.x); "+
			"v1.0.0 changed the data model. Back up the file, then run "+
			"`vpnkit init --force` to regenerate", path)
	}
	// Reject schema versions we don't understand. Without this, an older
	// vpnkit binary happily eats a future v3 store, backfills defaults
	// into unknown fields, and writes back a v2-shaped frankenstein that
	// loses the v3 information silently. Better to fail loud.
	if s.Cfg.SchemaVersion > 2 {
		return nil, fmt.Errorf("store at %s has schema_version=%d which this vpnkit doesn't understand "+
			"(supported up to v2). Upgrade vpnkit or restore an older backup", path, s.Cfg.SchemaVersion)
	}
	if s.Cfg.SchemaVersion == 0 {
		s.Cfg.SchemaVersion = 2
		changed = true
	}
	if s.Cfg.ControllerPort == 0 {
		s.Cfg.ControllerPort = randomHighPort()
		changed = true
	}
	if s.Cfg.MixedPort == 0 {
		s.Cfg.MixedPort = randomHighPort()
		// Guard against the (vanishingly small) chance the two random draws
		// landed on the same port. portutil.FindFree at runtime would shift
		// one of them anyway, but it's cleaner to start distinct.
		for s.Cfg.MixedPort == s.Cfg.ControllerPort {
			s.Cfg.MixedPort = randomHighPort()
		}
		changed = true
	}
	if s.Cfg.UITheme == "" {
		s.Cfg.UITheme = "default"
		changed = true
	}
	if s.Cfg.ControllerSecret == "" {
		s.Cfg.ControllerSecret = randHex(16)
		changed = true
	}
	if s.Cfg.ProxyUser == "" {
		s.Cfg.ProxyUser = "vpnkit-" + randHex(4)
		changed = true
	}
	if s.Cfg.ProxyPass == "" {
		s.Cfg.ProxyPass = randHex(16)
		changed = true
	}
	if s.Cfg.Mode == "" {
		s.Cfg.Mode = "rule"
		changed = true
	}
	// GlobalTarget is the *member* the top-level "🚀 Proxy" Selector
	// defaults to. Layered migration:
	//
	//   1. Empty or "🚀 Proxy" (rc.5- self-loop) → "DIRECT" as the safe
	//      fallback. Done unconditionally.
	//   2. "DIRECT" + at least one enabled proxy source → first source's
	//      `-auto`. Catches the rc.6 upgrade case where users with
	//      existing subs end up stuck on DIRECT (rule (1) bumped them
	//      there) and MATCH,🚀 Proxy then resolves to direct connections.
	//      AddSubscription / AddLocalGroup have a similar nudge for new
	//      runtime additions, but they don't fire on already-loaded
	//      stores — Load() is the only place that catches the upgrade.
	//
	// If the user explicitly wants MATCH → DIRECT despite having proxies,
	// the right knob is `mode = "direct"` (emits MATCH,🎯 Direct
	// directly), not `global_target = "DIRECT"`.
	if s.Cfg.GlobalTarget == "" || s.Cfg.GlobalTarget == "🚀 Proxy" {
		s.Cfg.GlobalTarget = "DIRECT"
		changed = true
	}
	// The "DIRECT" → first-source bump must run AFTER the lazy
	// local-node-group migration below — otherwise a first-load on an
	// rc.2 store (ungrouped node + no group entry yet) would miss the
	// freshly-synthesized "local" group and stay on DIRECT, then bump
	// only on the second load, producing churn.
	if s.Cfg.Subscriptions == nil {
		s.Cfg.Subscriptions = []Subscription{}
		changed = true
	}
	if s.Cfg.LocalNodes == nil {
		s.Cfg.LocalNodes = []LocalNode{}
		changed = true
	}
	if s.Cfg.LocalRules == nil {
		s.Cfg.LocalRules = []LocalRule{}
		changed = true
	}
	if s.Cfg.LocalNodeGroups == nil {
		s.Cfg.LocalNodeGroups = []LocalNodeGroup{}
		changed = true
	}
	defaultGroupName := "local"
	needsDefaultGroup := false
	for i := range s.Cfg.LocalNodes {
		if s.Cfg.LocalNodes[i].Group == "" {
			s.Cfg.LocalNodes[i].Group = defaultGroupName
			needsDefaultGroup = true
			changed = true
		}
	}
	if needsDefaultGroup {
		hasDefault := false
		for _, g := range s.Cfg.LocalNodeGroups {
			if g.Name == defaultGroupName {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			s.Cfg.LocalNodeGroups = append(s.Cfg.LocalNodeGroups, LocalNodeGroup{
				Name:    defaultGroupName,
				Enabled: true,
			})
			changed = true
		}
	}
	// Deferred from above: now that lazy migration may have synthesized
	// a "local" group, re-evaluate the DIRECT bump so existing-source
	// users (including rc.2 lazy-migrated ones) hop to first-source-auto
	// in a single Load() pass.
	if s.Cfg.GlobalTarget == "DIRECT" {
		if first := firstEnabledProxySource(&s.Cfg); first != "" {
			s.Cfg.GlobalTarget = first + "-auto"
			changed = true
		}
	}
	// ActiveSource (rc.7) — auto-derive on first launch under the new
	// model. Two cases trigger:
	//   1. Fresh install: nothing in the store yet → pick first enabled
	//      source.
	//   2. Upgrading from rc.6 with `global_target = "<name>-auto"` →
	//      strip the suffix to back-derive the source name. Guarantees
	//      rc.6 users land on the same active source they were already
	//      routing through, no manual config touch needed.
	if s.Cfg.ActiveSource == "" {
		if derived := deriveActiveFromGlobalTarget(s.Cfg.GlobalTarget); derived != "" && sourceExists(&s.Cfg, derived) {
			s.Cfg.ActiveSource = derived
			changed = true
		} else if first := firstEnabledProxySource(&s.Cfg); first != "" {
			s.Cfg.ActiveSource = first
			changed = true
		}
	}
	if changed {
		if err := s.Save(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// deriveActiveFromGlobalTarget converts rc.6's `global_target =
// "<name>-auto"` form back to the source name. Returns "" if the target
// doesn't have the `-auto` suffix (e.g. user pointed at a specific node
// like "boost:HK-01" — no clean derivation, leave to firstEnabled).
func deriveActiveFromGlobalTarget(gt string) string {
	const suffix = "-auto"
	if !strings.HasSuffix(gt, suffix) {
		return ""
	}
	return strings.TrimSuffix(gt, suffix)
}

// sourceExists checks whether `name` matches any enabled subscription or
// local-node group. Used to validate a derived ActiveSource before
// trusting it — otherwise a rc.6 user who removed the original sub but
// kept the stale `global_target` could end up with an ActiveSource that
// no longer corresponds to anything routable.
func sourceExists(cfg *Config, name string) bool {
	for _, s := range cfg.Subscriptions {
		if s.Enabled && s.Name == name {
			return true
		}
	}
	for _, g := range cfg.LocalNodeGroups {
		if g.Enabled && g.Name == name {
			return true
		}
	}
	return false
}

// firstEnabledProxySource returns the name of the first enabled proxy
// source (subscription preferred, then local-node group) in insertion
// order. Used by Load() for the GlobalTarget="DIRECT"→first-source
// migration, and conceptually mirrors app.firstProxySource — kept here
// to avoid a store→app import cycle.
func firstEnabledProxySource(cfg *Config) string {
	for _, s := range cfg.Subscriptions {
		if s.Enabled {
			return s.Name
		}
	}
	for _, g := range cfg.LocalNodeGroups {
		if g.Enabled {
			return g.Name
		}
	}
	return ""
}

// Save serializes Cfg to disk atomically (tmp + rename).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "config-*.toml.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Belt-and-suspenders: tighten perms (os.CreateTemp is 0600 by default but
	// some umask paths historically left this looser). Rename preserves perms.
	_ = tmp.Chmod(0o600)
	// Cleanup on any path that doesn't successfully rename. After Rename the
	// source no longer exists, so the deferred Remove becomes a no-op.
	defer os.Remove(tmpName)
	if err := toml.NewEncoder(tmp).Encode(s.Cfg); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func defaults() Config {
	cp := randomHighPort()
	mp := randomHighPort()
	for mp == cp {
		mp = randomHighPort()
	}
	return Config{
		SchemaVersion:    2,
		ControllerSecret: randHex(16),
		ControllerPort:   cp,
		MixedPort:        mp,
		ProxyUser:        "vpnkit-" + randHex(4),
		ProxyPass:        randHex(16),
		UITheme:          "default",
		Mode:             "rule",
		GlobalTarget:     "DIRECT",
		Subscriptions:   []Subscription{},
		LocalNodes:      []LocalNode{},
		LocalNodeGroups: []LocalNodeGroup{},
		LocalRules:      []LocalRule{},
	}
}

func randHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// randomHighPort returns a uniformly-distributed TCP port in [30000, 60000],
// drawn from crypto/rand so that two concurrent first-launches by different
// users on the same host pick independent ports — the historical 7890/9090
// defaults guaranteed collisions in multi-user setups (mihomo could not bind).
// portutil.FindFree still runs at startup as a safety net, but starting from
// a random seed reduces the post-randomization scan distance to typically zero.
//
// Rejection sampling is used to remove modulo bias: uint16 (65536 values) mod
// 30001 would otherwise over-sample the low ~18% of the range by 2x.
func randomHighPort() int {
	const count = 30001
	const limit = 65536 - (65536 % count) // largest multiple of count fitting in uint16
	var b [2]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			// crypto/rand on Linux/macOS does not fail post-boot. If it does,
			// log to stderr so two simultaneous fallbacks remain observable,
			// then return a fixed mid-range port; FindFree will scan from there.
			fmt.Fprintln(os.Stderr, "vpnkit: crypto/rand failed in randomHighPort; using fallback 45000:", err)
			return 45000
		}
		v := int(binary.BigEndian.Uint16(b[:]))
		if v < limit {
			return 30000 + v%count
		}
		// Reject the biased tail (the upper 5534 values of uint16 space) and redraw.
	}
}
