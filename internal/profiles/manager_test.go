package profiles

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndUpdate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("trojan://pw@h.example:443#node1"))
	}))
	defer srv.Close()

	m := New(Config{
		ConfigYAMLPath:   configPath,
		PatchPath:        filepath.Join(dir, "patch.yaml"),
		ControllerPort:   9090,
		ControllerSecret: "x",
		RuleTemplate:     "minimal",
	})
	if err := m.Add(Profile{Name: "main", URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Update(context.Background(), "main"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "node1") || !strings.Contains(string(got), "GEOIP,CN") {
		t.Errorf("config missing expected content:\n%s", got)
	}
}

func TestActivateSwitchesActive(t *testing.T) {
	dir := t.TempDir()
	m := New(Config{
		ConfigYAMLPath:   filepath.Join(dir, "config.yaml"),
		PatchPath:        filepath.Join(dir, "patch.yaml"),
		ControllerPort:   9090,
		ControllerSecret: "x",
		RuleTemplate:     "minimal",
	})
	_ = m.Add(Profile{Name: "a"})
	_ = m.Add(Profile{Name: "b"})
	m.SetActive("b")
	if m.Active() != "b" {
		t.Errorf("active: %s", m.Active())
	}
	if names := m.List(); names[0] != "a" || names[1] != "b" {
		t.Errorf("list: %v", names)
	}
}
