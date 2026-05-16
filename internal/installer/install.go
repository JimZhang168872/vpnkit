package installer

import (
	"fmt"

	"vpnkit/internal/netx"
)

// Options control an Install call.
type Options struct {
	APIBase     string         // override GitHub API base (for tests / enterprise)
	Token       string         // GITHUB_TOKEN
	Mirror      string         // optional URL prefix applied to release download URLs
	Dst         string         // absolute destination path for mihomo binary
	Version     string         // empty = latest
	ForceCompat *bool          // nil = autodetect; true/false = override
	OnAttempt   netx.OnAttempt // per-mirror callback; nil = silent
}

// Result describes a successful install.
type Result struct {
	Version    string
	Compatible bool
	// Mirror is the prefix that actually served the binary, "" for direct.
	// Callers may persist this back to store.Cfg.ReleaseMirror so the next
	// download starts with the known-good endpoint.
	Mirror string
}

// Install runs the full flow: resolve release → choose asset → download → verify → unpack → rename.
//
// opts.Mirror is the *preferred* mirror; if it doesn't work or is empty,
// Download falls through to direct github + builtin public mirrors. The
// mirror that actually served the bytes is returned in Result.Mirror.
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

	winningMirror, derr := Download(url, "", opts.Dst, opts.Mirror, opts.OnAttempt, progress)
	if derr != nil {
		return Result{}, fmt.Errorf("install: download: %w", derr)
	}
	return Result{Version: rel.Tag, Compatible: compat, Mirror: winningMirror}, nil
}

// currentArch wraps runtime.GOARCH so tests can override via build tags later if needed.
func currentArch() string {
	return runtimeGOARCH()
}
