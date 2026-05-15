package store

import (
	"os"
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

func TestLoadExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `
controller_secret = "custom_secret_1234567890ab"
controller_port = 8080
rule_template = "custom"
ui_theme = "dark"
active_profile = "home"

[[profiles]]
name = "home"
url = "https://example.com/sub1"
user_agent = "custom-agent"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.ControllerSecret != "custom_secret_1234567890ab" {
		t.Errorf("secret: %s", s.Cfg.ControllerSecret)
	}
	if s.Cfg.ControllerPort != 8080 {
		t.Errorf("port: %d", s.Cfg.ControllerPort)
	}
	if s.Cfg.RuleTemplate != "custom" {
		t.Errorf("rule template: %s", s.Cfg.RuleTemplate)
	}
	if s.Cfg.UITheme != "dark" {
		t.Errorf("theme: %s", s.Cfg.UITheme)
	}
	if s.Cfg.ActiveProfile != "home" {
		t.Errorf("active profile: %s", s.Cfg.ActiveProfile)
	}
	if len(s.Cfg.Profiles) != 1 || s.Cfg.Profiles[0].Name != "home" {
		t.Errorf("profiles: %+v", s.Cfg.Profiles)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `
controller_secret = ""
controller_port = 0
rule_template = ""
ui_theme = ""
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Cfg.ControllerPort != 9090 {
		t.Errorf("port default: %d", s.Cfg.ControllerPort)
	}
	if s.Cfg.RuleTemplate != "loyalsoldier" {
		t.Errorf("rule template default: %s", s.Cfg.RuleTemplate)
	}
	if s.Cfg.UITheme != "default" {
		t.Errorf("ui theme default: %s", s.Cfg.UITheme)
	}
	if len(s.Cfg.ControllerSecret) < 16 {
		t.Errorf("secret default too short: %q", s.Cfg.ControllerSecret)
	}
}

func TestSaveConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Simulate concurrent saves (Save() should have mutex protection)
	done := make(chan error, 2)
	go func() {
		s1, _ := Load(path)
		s1.Cfg.ControllerPort = 8080
		done <- s1.Save()
	}()
	go func() {
		s2, _ := Load(path)
		s2.Cfg.ControllerPort = 9090
		done <- s2.Save()
	}()

	if err := <-done; err != nil {
		t.Fatalf("concurrent save 1: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("concurrent save 2: %v", err)
	}

	s3, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if s3.Cfg.ControllerPort != 9090 && s3.Cfg.ControllerPort != 8080 {
		t.Errorf("unexpected port: %d", s3.Cfg.ControllerPort)
	}
}
