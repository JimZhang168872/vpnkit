package patch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyMissingFile(t *testing.T) {
	target := map[string]any{"port": 7890}
	if err := Apply(filepath.Join(t.TempDir(), "patch.yaml"), target); err != nil {
		t.Errorf("missing file should be a no-op: %v", err)
	}
	if target["port"] != 7890 {
		t.Errorf("target mutated: %v", target)
	}
}

func TestApplyDeepMerge(t *testing.T) {
	dir := t.TempDir()
	patchPath := filepath.Join(dir, "patch.yaml")
	_ = os.WriteFile(patchPath, []byte(`
port: 7891
dns:
  enable: true
  nameserver: [8.8.8.8]
`), 0o600)
	target := map[string]any{
		"port": 7890,
		"dns": map[string]any{
			"enhanced-mode": "fake-ip",
		},
	}
	if err := Apply(patchPath, target); err != nil {
		t.Fatal(err)
	}
	if target["port"] != 7891 {
		t.Errorf("port: %v", target["port"])
	}
	dns := target["dns"].(map[string]any)
	if dns["enable"] != true || dns["enhanced-mode"] != "fake-ip" {
		t.Errorf("dns merge: %+v", dns)
	}
	ns, _ := dns["nameserver"].([]any)
	if len(ns) != 1 || ns[0] != "8.8.8.8" {
		t.Errorf("nameserver: %v", ns)
	}
}
