package subscription

import (
	"strings"
	"testing"

	"vpnkit/internal/extensions"
	"vpnkit/internal/subscription/proto"
)

func TestAssembleMergesEverything(t *testing.T) {
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
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(yamlBytes)
	for _, want := range []string{
		"mixed-port: 7890",
		"HK-01",
		"GEOIP,CN",
		"🚀 Proxy",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestAssembleGeoxURLPointsAtGitHubDirectly(t *testing.T) {
	out, err := Assemble(AssembleInput{
		Result:           Result{Source: "uri", Proxies: nil},
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		RuleTemplate:     "minimal",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest") {
		t.Errorf("expected direct GitHub geox-url, got:\n%s", s)
	}
	if strings.Contains(s, "jsdelivr") || strings.Contains(s, "ghproxy") {
		t.Errorf("found mirror reference in geox-url:\n%s", s)
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

func TestAssembleAppliesExtensions(t *testing.T) {
	in := AssembleInput{
		Result: Result{
			Source: "uri",
			Proxies: []proto.Proxy{
				{"name": "US-1", "type": "ss", "server": "x", "port": 1, "cipher": "c", "password": "p"},
				{"name": "JP-Relay", "type": "ss", "server": "y", "port": 2, "cipher": "c", "password": "p"},
			},
		},
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "s",
		RuleTemplate:     "minimal",
		Extensions: extensions.Extensions{
			Chains: []extensions.Chain{
				{Node: "US-1", Via: "JP-Relay"},
			},
			Groups: []extensions.Group{
				{Name: "Stream", Type: "select", Proxies: []string{"US-1", "DIRECT"}},
			},
		},
	}
	out, err := Assemble(in)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "dialer-proxy: JP-Relay") {
		t.Fatalf("expected dialer-proxy line, got:\n%s", s)
	}
	if !strings.Contains(s, "name: Stream") {
		t.Fatalf("expected custom group Stream, got:\n%s", s)
	}
}
