package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmbeddedRulesetNamesIncludesLoyalsoldierSet asserts the bundled
// snapshot covers every rule-provider referenced by the loyalsoldier
// template — if a file is missing, mihomo will be forced to fetch it
// from the CDN on first launch, partially defeating the "offline-ready"
// purpose of the embed.
func TestEmbeddedRulesetNamesIncludesLoyalsoldierSet(t *testing.T) {
	got, err := EmbeddedRulesetNames()
	if err != nil {
		t.Fatalf("EmbeddedRulesetNames: %v", err)
	}
	want := []string{
		"apple", "cncidr", "direct", "gfw", "google", "greatfire",
		"icloud", "lancidr", "private", "proxy", "reject",
		"telegramcidr", "tld-not-cn",
	}
	gotSet := map[string]bool{}
	for _, n := range got {
		gotSet[n] = true
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("missing embedded ruleset %q (have %v)", w, got)
		}
	}
}

// TestWriteRulesetsToEmpty seeds a clean directory and verifies every
// embedded ruleset gets a decompressed .txt twin written.
func TestWriteRulesetsToEmpty(t *testing.T) {
	dir := t.TempDir()
	n, err := WriteRulesetsTo(dir)
	if err != nil {
		t.Fatalf("WriteRulesetsTo: %v", err)
	}
	names, _ := EmbeddedRulesetNames()
	if n != len(names) {
		t.Errorf("wrote %d files, want %d", n, len(names))
	}
	// Spot-check decompression actually happened (file content shouldn't
	// start with gzip magic bytes 0x1f 0x8b).
	data, err := os.ReadFile(filepath.Join(dir, "private.txt"))
	if err != nil {
		t.Fatalf("read private.txt: %v", err)
	}
	if len(data) < 2 || (data[0] == 0x1f && data[1] == 0x8b) {
		t.Errorf("private.txt looks like gzip, not decompressed text")
	}
	// And it should look like a clash domain rule file — first non-comment
	// line should be a hostname (no spaces, dots common).
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.ContainsAny(line, " \t") {
			t.Errorf("private.txt content not a hostname list (line=%q)", line)
		}
		break
	}
}

// TestWriteRulesetsToIdempotent verifies existing files are left alone —
// mihomo's runtime refresh may have written a newer version, and we
// don't want to roll it back to the embedded snapshot on every vpnkit
// launch.
func TestWriteRulesetsToIdempotent(t *testing.T) {
	dir := t.TempDir()
	// First seed: writes everything.
	if _, err := WriteRulesetsTo(dir); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Stomp the on-disk private.txt with a marker that mihomo "updated".
	marker := "USER_OR_MIHOMO_UPDATED\n"
	if err := os.WriteFile(filepath.Join(dir, "private.txt"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-seed: must skip private.txt because it exists.
	n2, err := WriteRulesetsTo(dir)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if n2 != 0 {
		t.Errorf("idempotent re-seed wrote %d files, want 0", n2)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "private.txt"))
	if string(data) != marker {
		t.Errorf("re-seed clobbered user/mihomo update; got %q", string(data))
	}
}
