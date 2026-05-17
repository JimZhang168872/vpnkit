package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCreatesDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Ports are randomized into the IANA dynamic/private range to avoid
	// multi-user same-host collisions (see store.randomHighPort).
	if !inHighRange(s.Cfg.ControllerPort) {
		t.Errorf("controller_port outside [30000,60000]: %d", s.Cfg.ControllerPort)
	}
	if !inHighRange(s.Cfg.MixedPort) {
		t.Errorf("mixed_port outside [30000,60000]: %d", s.Cfg.MixedPort)
	}
	if s.Cfg.MixedPort == s.Cfg.ControllerPort {
		t.Errorf("mixed and controller collided on same port: %d", s.Cfg.MixedPort)
	}
	if s.Cfg.LegacyRuleTemplate != "" {
		t.Errorf("legacy_rule_template should be empty on new store: %s", s.Cfg.LegacyRuleTemplate)
	}
	if s.Cfg.ServiceMode != "" {
		t.Errorf("service_mode must remain empty until detected: %s", s.Cfg.ServiceMode)
	}
	if len(s.Cfg.ControllerSecret) < 16 {
		t.Errorf("secret too short: %q", s.Cfg.ControllerSecret)
	}
	if len(s.Cfg.ProxyUser) < 8 {
		t.Errorf("proxy_user too short / not generated: %q", s.Cfg.ProxyUser)
	}
	if len(s.Cfg.ProxyPass) < 16 {
		t.Errorf("proxy_pass too short / not generated: %q", s.Cfg.ProxyPass)
	}
}

// TestDefaultPortsAreDistinctAcrossLoads ensures two fresh installs on the
// same machine almost never pick identical port pairs. We can't prove
// non-collision (crypto/rand could theoretically return the same value), but
// across N stores the pair-collision rate should be negligible.
func TestDefaultPortsAreDistinctAcrossLoads(t *testing.T) {
	const n = 50
	seen := make(map[[2]int]int, n)
	for i := 0; i < n; i++ {
		path := filepath.Join(t.TempDir(), "config.toml")
		s, err := Load(path)
		if err != nil {
			t.Fatalf("Load #%d: %v", i, err)
		}
		key := [2]int{s.Cfg.MixedPort, s.Cfg.ControllerPort}
		seen[key]++
	}
	// With 30001 possible values and 50 stores, expected duplicate pair count is
	// vanishingly small (<10^-5). Any duplicate signals the randomization is
	// broken (e.g. seeded math/rand sharing state).
	for k, c := range seen {
		if c > 1 {
			t.Errorf("port pair %v repeated %d times across %d loads — randomization not independent", k, c, n)
		}
	}
}

// TestZeroPortBackfillUsesHighRange exercises the upgrade path: an existing
// store.toml that was authored with the (now-removed) zero-default codepath
// must be backfilled with high-range ports, not 7890/9090.
func TestZeroPortBackfillUsesHighRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	// Construct a TOML deliberately missing both port keys so Load's
	// zero-port backfill branch fires.
	if err := os.WriteFile(path, []byte(`schema_version = 2
controller_secret = "deadbeef"
proxy_user = "vpnkit-abcd"
proxy_pass = "0123456789abcdef"
ui_theme = "default"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !inHighRange(s.Cfg.MixedPort) {
		t.Errorf("mixed_port backfill outside [30000,60000]: %d", s.Cfg.MixedPort)
	}
	if !inHighRange(s.Cfg.ControllerPort) {
		t.Errorf("controller_port backfill outside [30000,60000]: %d", s.Cfg.ControllerPort)
	}
	if s.Cfg.MixedPort == s.Cfg.ControllerPort {
		t.Errorf("backfilled mixed/controller collided: %d", s.Cfg.MixedPort)
	}
}

// TestExistingPortsArePreserved guards against a regression where an upgrade
// would clobber a working user's hand-edited / pre-existing port choice.
func TestExistingPortsArePreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`schema_version = 2
controller_secret = "deadbeef"
controller_port = 7890
mixed_port = 7891
proxy_user = "vpnkit-abcd"
proxy_pass = "0123456789abcdef"
ui_theme = "default"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.MixedPort != 7891 || s.Cfg.ControllerPort != 7890 {
		t.Errorf("existing ports clobbered: got mixed=%d controller=%d",
			s.Cfg.MixedPort, s.Cfg.ControllerPort)
	}
}

func inHighRange(p int) bool { return p >= 30000 && p <= 60000 }

func TestSchemaV2Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.SchemaVersion != 2 {
		t.Errorf("new store schema_version: got %d, want 2", s.Cfg.SchemaVersion)
	}
	if s.Cfg.Mode != "rule" {
		t.Errorf("default mode: got %q, want \"rule\"", s.Cfg.Mode)
	}
	if s.Cfg.GlobalTarget != "🚀 Proxy" {
		t.Errorf("default global_target: got %q, want \"🚀 Proxy\"", s.Cfg.GlobalTarget)
	}
	if s.Cfg.Subscriptions == nil {
		t.Error("Subscriptions must be empty slice, not nil")
	}
	if s.Cfg.LocalNodes == nil {
		t.Error("LocalNodes must be empty slice, not nil")
	}
	if s.Cfg.LocalRules == nil {
		t.Error("LocalRules must be empty slice, not nil")
	}
}

func TestProxyCredsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	u, p := s.Cfg.ProxyUser, s.Cfg.ProxyPass
	if u == "" || p == "" {
		t.Fatal("creds not generated")
	}
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if s2.Cfg.ProxyUser != u || s2.Cfg.ProxyPass != p {
		t.Errorf("creds regenerated on reload: got %q/%q want %q/%q",
			s2.Cfg.ProxyUser, s2.Cfg.ProxyPass, u, p)
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Cfg.LegacyActiveProfile = "airport-A"
	s.Cfg.LegacyProfiles = []Profile{{Name: "airport-A", URL: "https://example.com/sub"}}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if s2.Cfg.LegacyActiveProfile != "airport-A" {
		t.Errorf("active not persisted: %s", s2.Cfg.LegacyActiveProfile)
	}
	if len(s2.Cfg.LegacyProfiles) != 1 || s2.Cfg.LegacyProfiles[0].Name != "airport-A" {
		t.Errorf("profiles not persisted: %+v", s2.Cfg.LegacyProfiles)
	}
}

func TestSchemaV2HasLocalNodeGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.LocalNodeGroups == nil {
		t.Error("LocalNodeGroups must be initialized to empty slice, not nil")
	}
	if len(s.Cfg.LocalNodeGroups) != 0 {
		t.Errorf("fresh store should have 0 local groups, got %d", len(s.Cfg.LocalNodeGroups))
	}
}

func TestLoadRejectsV1Store(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	v1 := `controller_secret = "deadbeef"
controller_port = 9090
mixed_port = 7890
rule_template = "loyalsoldier"
active_profile = "doge"
[[profiles]]
name = "doge"
url = "https://example.invalid/sub"
`
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected v1 store rejection, got nil")
	}
	if !strings.Contains(err.Error(), "schema") || !strings.Contains(err.Error(), "vpnkit init --force") {
		t.Errorf("error should mention schema upgrade + remedy command, got: %v", err)
	}
}
