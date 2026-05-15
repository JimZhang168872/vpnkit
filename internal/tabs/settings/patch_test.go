package settings

import (
	"os"
	"path/filepath"
	"testing"

	"vpnkit/internal/paths"
)

func TestPatchLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	mihomoDir := filepath.Join(dir, "mihomo")
	_ = os.MkdirAll(mihomoDir, 0o755)
	patchPath := filepath.Join(mihomoDir, "patch.yaml")
	_ = os.WriteFile(patchPath, []byte("log-level: debug\n"), 0o600)

	p := paths.XDG{MihomoConfig: mihomoDir}
	m := newPatch(p)
	if got := m.Content(); got != "log-level: debug\n" {
		t.Errorf("loaded: %q", got)
	}
	m.SetContent("log-level: warn\n")
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(patchPath)
	if string(data) != "log-level: warn\n" {
		t.Errorf("saved: %s", data)
	}
}
