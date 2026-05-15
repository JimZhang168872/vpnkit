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
