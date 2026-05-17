package assembler

import (
	"strings"
	"testing"
)

func TestAssembleEmitsBaseConfig(t *testing.T) {
	out, err := Assemble(Input{
		Mode:             ModeRule,
		GlobalTarget:     "🚀 Proxy",
		MixedPort:        50595,
		ControllerPort:   32645,
		ControllerSecret: "secret",
		ProxyUser:        "vpnkit-user",
		ProxyPass:        "pass",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"mixed-port: 50595",
		"external-controller: 127.0.0.1:32645",
		"secret: secret",
		"bind-address: 127.0.0.1",
		"allow-lan: false",
		"mode: rule",
		"vpnkit-",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}
