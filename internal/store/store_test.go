package store

import (
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.ControllerPort != 9090 {
		t.Errorf("default port: %d", s.Cfg.ControllerPort)
	}
	if s.Cfg.RuleTemplate != "loyalsoldier" {
		t.Errorf("default rule template: %s", s.Cfg.RuleTemplate)
	}
	if s.Cfg.ServiceMode != "" {
		t.Errorf("service_mode must remain empty until detected: %s", s.Cfg.ServiceMode)
	}
	if len(s.Cfg.ControllerSecret) < 16 {
		t.Errorf("secret too short: %q", s.Cfg.ControllerSecret)
	}
	if s.Cfg.MixedPort != 7890 {
		t.Errorf("default mixed_port: %d", s.Cfg.MixedPort)
	}
	if len(s.Cfg.ProxyUser) < 8 {
		t.Errorf("proxy_user too short / not generated: %q", s.Cfg.ProxyUser)
	}
	if len(s.Cfg.ProxyPass) < 16 {
		t.Errorf("proxy_pass too short / not generated: %q", s.Cfg.ProxyPass)
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
	s.Cfg.ActiveProfile = "airport-A"
	s.Cfg.Profiles = []Profile{{Name: "airport-A", URL: "https://example.com/sub"}}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if s2.Cfg.ActiveProfile != "airport-A" {
		t.Errorf("active not persisted: %s", s2.Cfg.ActiveProfile)
	}
	if len(s2.Cfg.Profiles) != 1 || s2.Cfg.Profiles[0].Name != "airport-A" {
		t.Errorf("profiles not persisted: %+v", s2.Cfg.Profiles)
	}
}
