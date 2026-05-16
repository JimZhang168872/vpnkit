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

// TestUpdate_SingleHysteria2URI is an end-to-end check that a hysteria2://
// single-URI subscription (the kind users paste straight from a provider)
// flows through Fetch -> Convert -> Assemble -> AtomicWrite and produces a
// config.yaml containing the expected proxy. Regression guard for
// "unsupported protocol scheme \"hysteria2\"" reported on 2026-05-16.
func TestUpdate_SingleHysteria2URI(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	const hy2URI = "hysteria2://CBAI0bv97b21KRjXw3fDArlnW/ymWTur@jim.gulujili.xyz:8443?security=tls&fp=chrome&alpn=h3&sni=jim.gulujili.xyz#Hy2-entrance-CN-jim-hy2"

	m := New(Config{
		ConfigYAMLPath:   configPath,
		PatchPath:        filepath.Join(dir, "patch.yaml"),
		ControllerPort:   9090,
		ControllerSecret: "x",
		RuleTemplate:     "minimal",
	})
	if err := m.Add(Profile{Name: "hy2", URL: hy2URI}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	count, err := m.Update(context.Background(), "hy2")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if count != 1 {
		t.Errorf("node count: got %d want 1", count)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{
		"type: hysteria2",
		"server: jim.gulujili.xyz",
		"port: 8443",
		"password: CBAI0bv97b21KRjXw3fDArlnW/ymWTur",
		"sni: jim.gulujili.xyz",
		"name: Hy2-entrance-CN-jim-hy2",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("config missing %q\n---\n%s", want, s)
		}
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
