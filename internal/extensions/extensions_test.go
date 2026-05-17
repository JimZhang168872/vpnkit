package extensions

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(filepath.Join(dir, "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load missing: unexpected error %v", err)
	}
	if len(got.Chains) != 0 || len(got.Groups) != 0 {
		t.Fatalf("Load missing: want empty, got %+v", got)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	want := Extensions{
		SchemaVersion: 1,
		Chains: []Chain{
			{Node: "🇺🇸 US-1", Via: "🇯🇵 JP-Relay"},
			{Node: "🇰🇷 KR-Edge", Via: "🇯🇵 JP-Relay"},
		},
		Groups: []Group{
			{
				Name: "🎯 Stream", Type: "select",
				Proxies: []string{"🇺🇸 US-1", "🇯🇵 JP-1", "DIRECT"},
			},
			{
				Name: "♻️ Auto-US", Type: "url-test",
				Proxies:   []string{"🇺🇸 US-1", "🇺🇸 US-2"},
				URL:       "https://www.gstatic.com/generate_204",
				Interval:  300,
				Tolerance: 50,
			},
		},
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm: want 0600, got %o", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roundtrip mismatch:\nwant %+v\n got %+v", want, got)
	}
}
