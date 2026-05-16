package subscription

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/subscription/proto"
)

func TestAssembleMergesEverything(t *testing.T) {
	dir := t.TempDir()
	patchPath := filepath.Join(dir, "patch.yaml")
	_ = os.WriteFile(patchPath, []byte("log-level: debug\n"), 0o600)

	r := Result{
		Source: "uri",
		Proxies: []proto.Proxy{
			{"name": "HK-01", "type": "ss", "server": "1.1.1.1", "port": 8388, "cipher": "aes-128-gcm", "password": "x"},
		},
	}
	yamlBytes, err := Assemble(AssembleInput{
		Result:           r,
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "secret",
		RuleTemplate:     "minimal",
		PatchPath:        patchPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(yamlBytes)
	for _, want := range []string{
		"mixed-port: 7890",
		"HK-01",
		"GEOIP,CN",
		"log-level: debug",
		"🚀 Proxy",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestAssembleAppliesReleaseMirror(t *testing.T) {
	out, err := Assemble(AssembleInput{
		Result:           Result{Source: "uri", Proxies: nil},
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		RuleTemplate:     "minimal",
		ReleaseMirror:    "https://ghproxy.com/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "ghproxy.com/https://github.com") {
		t.Errorf("missing mirror-prefixed geox-url:\n%s", out)
	}
}

func TestAssembleEmitsAuthentication(t *testing.T) {
	out, err := Assemble(AssembleInput{
		Result:           Result{Source: "uri", Proxies: nil},
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		RuleTemplate:     "minimal",
		ProxyUser:        "alice",
		ProxyPass:        "p4ss",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "authentication:") || !strings.Contains(string(out), "alice:p4ss") {
		t.Errorf("missing authentication block:\n%s", out)
	}
}

func TestAssembleKeepsExistingGroupsFromClash(t *testing.T) {
	r := Result{
		Source: "clash",
		Raw: map[string]any{
			"proxy-groups": []any{
				map[string]any{"name": "MyGroup", "type": "select", "proxies": []any{"DIRECT"}},
			},
		},
		Proxies: []proto.Proxy{{"name": "n1", "type": "ss", "server": "h", "port": 1, "cipher": "c", "password": "p"}},
	}
	out, err := Assemble(AssembleInput{Result: r, MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "s", RuleTemplate: "minimal"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "MyGroup") {
		t.Errorf("custom group lost:\n%s", out)
	}
}
