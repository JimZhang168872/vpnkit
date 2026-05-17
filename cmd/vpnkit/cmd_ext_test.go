package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/extensions"
)

// TestRunExtApplyErrorsWhenNoActiveProfile removed in Phase 6:
// ActiveProfile concept replaced by Pipeline.Assemble in v1.
// TODO(v1-phase7): add test that covers "assemble fails" path.

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
