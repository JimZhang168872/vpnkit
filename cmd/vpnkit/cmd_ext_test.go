package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/extensions"
)

func TestRunExtApplyErrorsWhenNoActiveProfile(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	err := runExtApply(&buf, runExtApplyDeps{
		ExtensionsPath: filepath.Join(dir, "extensions.toml"),
		ActiveProfile:  "",
		Reassemble:     func() error { return nil },
		Reload:         func() error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("want 'no active profile' error, got %v", err)
	}
}

func TestRunExtApplyHappy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	var buf bytes.Buffer
	reassembled, reloaded := false, false
	err := runExtApply(&buf, runExtApplyDeps{
		ExtensionsPath: path,
		ActiveProfile:  "p",
		Reassemble:     func() error { reassembled = true; return nil },
		Reload:         func() error { reloaded = true; return nil },
	})
	if err != nil {
		t.Fatalf("runExtApply: %v", err)
	}
	if !reassembled || !reloaded {
		t.Fatalf("expected reassemble+reload, got reassemble=%v reload=%v", reassembled, reloaded)
	}
	if !strings.Contains(buf.String(), "applied") {
		t.Fatalf("expected 'applied' line, got %q", buf.String())
	}
}
