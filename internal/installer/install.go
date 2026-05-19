package installer

import (
	"fmt"
)

// Options control an Install call.
type Options struct {
	APIBase     string // override GitHub API base (for tests / enterprise)
	Token       string // GITHUB_TOKEN
	Dst         string // absolute destination path for mihomo binary
	Version     string // empty = latest
	ForceCompat *bool  // nil = autodetect; true/false = override
	// NoProxy forces the GitHub HTTP traffic to bypass the user's env proxy
	// (HTTP_PROXY / HTTPS_PROXY / …). Set this for the BOOTSTRAP path
	// (vpnkit init's first-time mihomo download): if the env proxy is
	// vpnkit's own mihomo (the v0.9.x `proxy_on` case), trying to use it
	// would deadlock — mihomo doesn't exist yet, we're trying to install it.
	// Update / upgrade callers should leave this false so the user's
	// explicit HTTPS_PROXY (often set via `vpnkit env` after bootstrap) is
	// honored.
	NoProxy bool
}

// Result describes a successful install.
type Result struct {
	Version    string
	Compatible bool
}

// Install runs the full flow: resolve release → choose asset → download →
// unpack → rename. No mirror layer; network errors propagate unchanged so
// the caller can surface them with a "configure HTTPS_PROXY" hint.
func Install(opts Options, progress ProgressFunc) (Result, error) {
	if opts.Dst == "" {
		return Result{}, fmt.Errorf("install: Dst is required")
	}
	rc := NewReleaseClient(opts.APIBase, opts.Token)
	if opts.NoProxy {
		// Bootstrap path: never honor env proxy (it likely points at our own
		// mihomo, which doesn't exist yet → chicken-and-egg).
		rc.HTTP = noProxyHTTPClient()
	}
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

	if err := Download(url, "", opts.Dst, progress, opts.NoProxy); err != nil {
		return Result{}, fmt.Errorf("install: download: %w", err)
	}
	return Result{Version: rel.Tag, Compatible: compat}, nil
}

// currentArch wraps runtime.GOARCH so tests can override via build tags later if needed.
func currentArch() string {
	return runtimeGOARCH()
}
