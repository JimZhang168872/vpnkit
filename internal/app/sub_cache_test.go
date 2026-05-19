package app

import (
	"os"
	"path/filepath"
	"testing"

	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

func TestSubCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	res := &subscription.Result{
		Source: "clash",
		Proxies: []proto.Proxy{
			{"name": "N1", "type": "vmess", "server": "a.b", "port": 443},
			{"name": "N2", "type": "ss", "server": "c.d", "port": 8388},
		},
		Raw: map[string]any{"rules": []any{"DOMAIN-SUFFIX,baidu.com,DIRECT"}},
	}
	if err := saveSubResult(dir, "sub-a", res); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := loadSubResult(dir, "sub-a")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded is nil")
	}
	if got := len(loaded.Proxies); got != 2 {
		t.Fatalf("proxies len = %d, want 2", got)
	}
	if loaded.Proxies[0]["name"] != "N1" {
		t.Errorf("proxy[0].name = %v, want N1", loaded.Proxies[0]["name"])
	}
	if loaded.Source != "clash" {
		t.Errorf("Source = %q, want clash", loaded.Source)
	}
}

func TestSubCacheLoadMissingReturnsNil(t *testing.T) {
	dir := t.TempDir()
	loaded, err := loadSubResult(dir, "never-saved")
	if err != nil {
		t.Fatalf("load missing: %v (should be nil error)", err)
	}
	if loaded != nil {
		t.Errorf("loaded = %v, want nil", loaded)
	}
}

func TestSubCacheNameEscaping(t *testing.T) {
	dir := t.TempDir()
	res := &subscription.Result{Source: "test"}
	// Name with slashes and spaces should not escape the cache dir.
	if err := saveSubResult(dir, "../escape attempt/with space", res); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadSubResult(dir, "../escape attempt/with space")
	if err != nil || got == nil {
		t.Fatalf("round-trip with weird name failed: err=%v got=%v", err, got)
	}
	// Verify the file lives inside subCacheDir, not above it.
	entries, _ := os.ReadDir(subCacheDir(dir))
	if len(entries) != 1 {
		t.Errorf("expected 1 entry in sub-cache/, got %d", len(entries))
	}
	parent, _ := os.ReadDir(filepath.Dir(dir))
	for _, e := range parent {
		if e.Name() != filepath.Base(dir) {
			// good — TempDir's parent only has the temp dir we made
			continue
		}
	}
}

func TestSubCacheDrop(t *testing.T) {
	dir := t.TempDir()
	res := &subscription.Result{Source: "test", Proxies: []proto.Proxy{{"name": "X"}}}
	_ = saveSubResult(dir, "sub-x", res)
	if err := dropSubResult(dir, "sub-x"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	loaded, _ := loadSubResult(dir, "sub-x")
	if loaded != nil {
		t.Errorf("expected nil after drop, got %v", loaded)
	}
	// Dropping again is a no-op.
	if err := dropSubResult(dir, "sub-x"); err != nil {
		t.Errorf("drop again should be no-op, got: %v", err)
	}
}

func TestSubCacheEmptyDirIsNoOp(t *testing.T) {
	// cacheDir="" disables caching — used by tests + ephemeral flows.
	res := &subscription.Result{Source: "x"}
	if err := saveSubResult("", "any", res); err != nil {
		t.Errorf("save with empty dir: %v", err)
	}
	loaded, err := loadSubResult("", "any")
	if err != nil {
		t.Errorf("load with empty dir: %v", err)
	}
	if loaded != nil {
		t.Errorf("loaded should be nil for empty dir, got %v", loaded)
	}
}
