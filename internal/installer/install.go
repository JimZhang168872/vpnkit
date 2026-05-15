package installer

import (
	"fmt"
)

// Options control an Install call.
type Options struct {
	APIBase     string // override GitHub API base (for tests / enterprise)
	Token       string // GITHUB_TOKEN
	Mirror      string // optional URL prefix applied to release download URLs
	Dst         string // absolute destination path for mihomo binary
	Version     string // empty = latest
	ForceCompat *bool  // nil = autodetect; true/false = override
}

// Result describes a successful install.
type Result struct {
	Version    string
	Compatible bool
}

// Install runs the full flow: resolve release → choose asset → download → verify → unpack → rename.
func Install(opts Options, progress ProgressFunc) (Result, error) {
	if opts.Dst == "" {
		return Result{}, fmt.Errorf("install: Dst is required")
	}
	rc := NewReleaseClient(opts.APIBase, opts.Token)
	var rel Release
	var err error
	if opts.Version == "" {
		rel, err = rc.Latest()
	} else {
		rel, err = rc.ByTag(opts.Version)
	}
	if err != nil {
		return Result{}, fmt.Errorf("install: fetch release: %w", err)
	}

	compat := false
	if opts.ForceCompat != nil {
		compat = *opts.ForceCompat
	} else {
		compat = NeedsCompatibleBuild()
	}
	name := assetName(currentArch(), compat, rel.Tag)
	url, err := rel.AssetURL(name)
	if err != nil {
		// Fall back to the other variant if exact name missing.
		altName := assetName(currentArch(), !compat, rel.Tag)
		alt, altErr := rel.AssetURL(altName)
		if altErr != nil {
			return Result{}, fmt.Errorf("install: %w", err)
		}
		url = alt
		compat = !compat
	}
	url = ApplyMirror(url, opts.Mirror)

	if err := Download(url, "", opts.Dst, progress); err != nil {
		return Result{}, fmt.Errorf("install: download: %w", err)
	}
	return Result{Version: rel.Tag, Compatible: compat}, nil
}

// currentArch wraps runtime.GOARCH so tests can override via build tags later if needed.
func currentArch() string {
	return runtimeGOARCH()
}
