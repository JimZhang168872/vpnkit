package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEnsureSecurityFieldsOnStaleConfigInjectsAuth(t *testing.T) {
	// Pre-v0.7.0 style config: no authentication, no bind-address.
	pre := `mixed-port: 7890
allow-lan: false
mode: rule
external-controller: 127.0.0.1:9090
secret: oldsecret
proxies:
  - name: HK-01
    type: ss
    server: 1.1.1.1
    port: 8388
    cipher: aes-128-gcm
    password: x
proxy-groups:
  - name: "🚀 Proxy"
    type: select
    proxies: [HK-01]
rules:
  - MATCH,🚀 Proxy
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureSecurityFields(path, SecurityFields{
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "oldsecret",
		ProxyUser:        "alice",
		ProxyPass:        "p4ss",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true (auth and bind-address were missing)")
	}

	out, _ := os.ReadFile(path)
	s := string(out)
	for _, want := range []string{
		"authentication:",
		"alice:p4ss",
		"bind-address: 127.0.0.1",
		"allow-lan: false",
		"HK-01",                  // proxy preserved
		"🚀 Proxy",                // group preserved
		"MATCH,🚀 Proxy",          // rule preserved
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q after reconcile:\n%s", want, s)
		}
	}

	// Re-running should be a no-op.
	changed2, err := EnsureSecurityFields(path, SecurityFields{
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "oldsecret",
		ProxyUser:        "alice",
		ProxyPass:        "p4ss",
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Error("expected idempotent (no change on second call)")
	}
}

func TestEnsureSecurityFieldsAlignsDrifts(t *testing.T) {
	// Config has wrong port/secret/auth — store wins.
	pre := `mixed-port: 7890
external-controller: 127.0.0.1:9090
secret: oldsecret
authentication:
  - bob:old
allow-lan: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureSecurityFields(path, SecurityFields{
		MixedPort:        7891,
		ControllerPort:   9091,
		ControllerSecret: "newsecret",
		ProxyUser:        "alice",
		ProxyPass:        "p4ss",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true")
	}

	out, _ := os.ReadFile(path)
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["mixed-port"] != 7891 {
		t.Errorf("mixed-port = %v, want 7891", doc["mixed-port"])
	}
	if doc["external-controller"] != "127.0.0.1:9091" {
		t.Errorf("external-controller = %v", doc["external-controller"])
	}
	if doc["secret"] != "newsecret" {
		t.Errorf("secret = %v", doc["secret"])
	}
	if doc["bind-address"] != "127.0.0.1" {
		t.Errorf("bind-address = %v", doc["bind-address"])
	}
	if doc["allow-lan"] != false {
		t.Errorf("allow-lan = %v", doc["allow-lan"])
	}
	authList, _ := doc["authentication"].([]any)
	if len(authList) != 1 || authList[0] != "alice:p4ss" {
		t.Errorf("authentication = %v", doc["authentication"])
	}
}

func TestEnsureSecurityFieldsMissingFileIsError(t *testing.T) {
	_, err := EnsureSecurityFields(filepath.Join(t.TempDir(), "no-such-file.yaml"), SecurityFields{
		MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "s", ProxyUser: "u", ProxyPass: "p",
	})
	if err == nil {
		t.Error("expected error on missing file")
	}
}

func TestEnsureSecurityFieldsInjectsGeoxUrlWhenMissing(t *testing.T) {
	// Pre-v0.8.3 config.yaml: no geox-url at all → mihomo would try to download
	// MMDB from github.com on boot and hang inside the GFW. We backfill the
	// jsdelivr default so existing users self-heal on next launch.
	pre := `mixed-port: 7890
external-controller: 127.0.0.1:9090
secret: s
allow-lan: false
bind-address: 127.0.0.1
proxies:
  - {name: HK, type: ss, server: 1.1.1.1, port: 1, cipher: aes-128-gcm, password: p}
rules:
  - GEOIP,CN,DIRECT
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsureSecurityFields(path, SecurityFields{
		MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "s",
		ProxyUser: "u", ProxyPass: "p",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true (geox-url was missing)")
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	if !strings.Contains(s, "geox-url") || !strings.Contains(s, "github.com/MetaCubeX/meta-rules-dat/releases/download/latest") {
		t.Errorf("geox-url not injected:\n%s", s)
	}
	if !strings.Contains(s, "HK") {
		t.Errorf("existing proxies lost:\n%s", s)
	}
}

func TestEnsureSecurityFieldsPreservesExistingGeoxUrl(t *testing.T) {
	// User customized geox-url via patch.yaml or by hand — don't clobber it.
	pre := `mixed-port: 7890
external-controller: 127.0.0.1:9090
secret: s
allow-lan: false
bind-address: 127.0.0.1
geox-url:
  geoip: https://custom.example.com/geoip.metadb
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsureSecurityFields(path, SecurityFields{
		MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "s",
		ProxyUser: "u", ProxyPass: "p",
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		// auth is being inserted (no ProxyUser/Pass in pre), so changed will be true.
		// We still need to verify the geox-url survived.
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "custom.example.com/geoip.metadb") {
		t.Errorf("user-customized geox-url got clobbered:\n%s", string(out))
	}
}

func TestEnsureSecurityFieldsKeepsFileMode0600(t *testing.T) {
	pre := "mixed-port: 7890\nexternal-controller: 127.0.0.1:9090\nsecret: s\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureSecurityFields(path, SecurityFields{
		MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "s",
		ProxyUser: "u", ProxyPass: "p",
	}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", info.Mode().Perm())
	}
}
