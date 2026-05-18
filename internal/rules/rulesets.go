package rules

import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rulesetFS is the embedded snapshot of Loyalsoldier rule-set text files,
// gzipped to keep the binary ~6 MB lighter (7.9 MB raw → 2.1 MB gz).
// Refreshed periodically; mihomo refreshes at runtime per the
// `interval` set in each rule-provider so a stale embed is bounded
// to 24h of drift after the first launch reaches the CDN.
//
//go:embed rulesets/*.txt.gz
var rulesetFS embed.FS

// EmbeddedRulesetNames returns the rule-set names (without extension)
// that vpnkit ships with. Order is filesystem walk order — alphabetical
// within Go's embed.FS guarantee.
func EmbeddedRulesetNames() ([]string, error) {
	entries, err := fs.ReadDir(rulesetFS, "rulesets")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".txt.gz")
		out = append(out, name)
	}
	return out, nil
}

// WriteRulesetsTo seeds the mihomo ruleset directory (typically
// ~/.config/mihomo/ruleset/) with the bundled snapshot. Each embedded
// <name>.txt.gz is decompressed and written as <dir>/<name>.txt.
//
// Idempotent and non-destructive: files that already exist on disk are
// left alone. mihomo's rule-provider loop refreshes them on `interval`
// (24h in the loyalsoldier template), so over time the user's local copy
// drifts toward the upstream CDN's. If the bundled snapshot is newer
// than the local file (e.g. fresh vpnkit install on top of an old
// mihomo cache), the bundled copy wins.
//
// Returns the count of files actually written. Failure on any one file
// surfaces as an error but does not stop processing of the rest — the
// caller (bootstrap) can tolerate partial seeding because mihomo will
// fetch any missing file on its first load.
func WriteRulesetsTo(dir string) (int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("ruleset dir: %w", err)
	}
	entries, err := fs.ReadDir(rulesetFS, "rulesets")
	if err != nil {
		return 0, fmt.Errorf("read embed: %w", err)
	}
	var errs []error
	written := 0
	for _, e := range entries {
		gzName := e.Name() // "<name>.txt.gz"
		txtName := strings.TrimSuffix(gzName, ".gz")
		dst := filepath.Join(dir, txtName)
		// Idempotent: skip if a copy already exists. mihomo may have
		// updated it from upstream; overwriting would undo that work.
		if _, statErr := os.Stat(dst); statErr == nil {
			continue
		} else if !errors.Is(statErr, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("%s: stat: %w", txtName, statErr))
			continue
		}
		if err := writeOneRuleset(gzName, dst); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", txtName, err))
			continue
		}
		written++
	}
	if len(errs) > 0 {
		return written, errors.Join(errs...)
	}
	return written, nil
}

func writeOneRuleset(embedName, dstPath string) error {
	src, err := rulesetFS.Open("rulesets/" + embedName)
	if err != nil {
		return fmt.Errorf("open embed: %w", err)
	}
	defer src.Close()
	gz, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("gunzip: %w", err)
	}
	defer gz.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dstPath), ".ruleset.*.tmp")
	if err != nil {
		return fmt.Errorf("tmpfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, gz); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("copy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dstPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
