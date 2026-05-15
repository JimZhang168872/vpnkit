package settings

import (
	"os"
	"path/filepath"
	"testing"

	"vpnkit/internal/paths"
)

func TestCacheSizeAndClear(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "downloads"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "downloads", "x.gz"), []byte("hello world!"), 0o644)
	p := paths.XDG{VpnkitCache: dir}
	m := newCache(p)
	if m.Size() <= 0 {
		t.Errorf("size should be > 0")
	}
	if err := m.Clear(); err != nil {
		t.Errorf("Clear: %v", err)
	}
	if m.Size() != 0 {
		t.Errorf("size after clear: %d", m.Size())
	}
}
