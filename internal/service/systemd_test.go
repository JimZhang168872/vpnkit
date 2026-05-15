package service

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderUnit(t *testing.T) {
	var buf bytes.Buffer
	err := renderUnit(&buf, "/home/u/.local/bin/mihomo", "/home/u/.config/mihomo")
	if err != nil {
		t.Fatalf("renderUnit: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "ExecStart=/home/u/.local/bin/mihomo -d /home/u/.config/mihomo") {
		t.Errorf("missing ExecStart: %s", s)
	}
	if !strings.Contains(s, "WantedBy=default.target") {
		t.Error("missing WantedBy")
	}
}

func TestFakeSystemctlInvocations(t *testing.T) {
	calls := []string{}
	runner := func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}
	m := &SystemdManager{cfg: Config{BinaryPath: "/x", ConfigDir: "/c", UnitPath: t.TempDir() + "/mihomo.service"}, run: runner}
	if err := m.Install(nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(nil); err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"--user daemon-reload",
		"--user enable --now mihomo.service",
		"--user start mihomo.service",
		"--user stop mihomo.service",
	}
	for _, e := range expected {
		found := false
		for _, c := range calls {
			if c == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected call %q not seen; got %v", e, calls)
		}
	}
}
