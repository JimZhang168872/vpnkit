package assembler

import (
	"strings"
	"testing"
)

func TestAssembleRejectsZeroPorts(t *testing.T) {
	cases := []Input{
		{ControllerPort: 32645, ControllerSecret: "s"},    // MixedPort=0
		{MixedPort: 50595, ControllerSecret: "s"},         // ControllerPort=0
	}
	for i, in := range cases {
		_, err := Assemble(in)
		if err == nil {
			t.Errorf("case %d: expected error for zero port, got nil", i)
		}
	}
}

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
