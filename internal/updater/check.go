// Package updater implements vpnkit's self-update flow: query GitHub for the
// latest vpnkit + mihomo releases, download them through the configured
// release mirror, atomically replace the on-disk binaries, and (when invoked
// from a running TUI) re-exec the current process to pick up the new binary.
//
// All network calls are short-timeout and respect store.Cfg.ReleaseMirror so
// behind-GFW users get the same coverage as `INSTALL_MIRROR` in install.sh.
package updater

import (
	"fmt"
	"regexp"
	"strings"

	"vpnkit/internal/installer"
)

// Opts configures Check.
type Opts struct {
	// VpnkitCurrent is the current vpnkit version (typically main.version
	// set via ldflags). Empty or *-dev → no check.
	VpnkitCurrent string
	// MihomoCurrent is the version string from `mihomo -v` (or "" if mihomo
	// is not yet installed).
	MihomoCurrent string
	// APIBase overrides https://api.github.com (used by tests + when a
	// mirror routes api.github.com through itself).
	APIBase string
	// Repo / MihomoRepo override the default owner/repo paths. Tests use
	// these; production callers leave them empty for defaults.
	Repo       string
	MihomoRepo string
	// Token is an optional GitHub token (avoids the 60 req/hr unauth limit).
	Token string
}

// Info reports what's available.
type Info struct {
	VpnkitCurrent     string
	VpnkitLatest      string
	VpnkitNeedsUpdate bool
	MihomoCurrent     string
	MihomoLatest      string
	MihomoNeedsUpdate bool
}

// HasUpdate is true when anything is upgradable.
func (i Info) HasUpdate() bool {
	return i.VpnkitNeedsUpdate || i.MihomoNeedsUpdate
}

const (
	defaultRepo       = "JimZhang168872/vpnkit"
	defaultMihomoRepo = "MetaCubeX/mihomo"
)

// Check queries GitHub for the latest vpnkit + mihomo releases and compares
// with the current versions. Returns Info even when nothing needs updating;
// network errors bubble up so callers can decide whether to surface them.
func Check(opts Opts) (Info, error) {
	if opts.Repo == "" {
		opts.Repo = defaultRepo
	}
	if opts.MihomoRepo == "" {
		opts.MihomoRepo = defaultMihomoRepo
	}
	info := Info{
		VpnkitCurrent: opts.VpnkitCurrent,
		MihomoCurrent: opts.MihomoCurrent,
	}
	if isDevBuild(opts.VpnkitCurrent) {
		// Local build — skip the vpnkit half. mihomo half still works.
		opts.VpnkitCurrent = ""
	}
	rc := installer.NewReleaseClient(opts.APIBase, opts.Token)

	if opts.VpnkitCurrent != "" {
		rel, err := rc.LatestForRepo(opts.Repo)
		if err != nil {
			return info, fmt.Errorf("check vpnkit latest: %w", err)
		}
		info.VpnkitLatest = rel.Tag
		info.VpnkitNeedsUpdate = isNewer(opts.VpnkitCurrent, rel.Tag)
	}

	rel, err := rc.LatestForRepo(opts.MihomoRepo)
	if err != nil {
		return info, fmt.Errorf("check mihomo latest: %w", err)
	}
	info.MihomoLatest = rel.Tag
	info.MihomoNeedsUpdate = isNewer(opts.MihomoCurrent, rel.Tag)

	return info, nil
}

// devSuffix matches go ldflags conventions for unreleased builds:
//   "dev", "0.8.4-dev", "0.8.4-dev+abc123"
var devSuffix = regexp.MustCompile(`(?i)(^$|^dev$|-dev($|[+.-]))`)

func isDevBuild(v string) bool { return devSuffix.MatchString(v) }

// isNewer is a lightweight semver-ish compare that's good enough for our
// monotonically-increasing tags. Format: optional "v" prefix, dot-separated
// integer components, optional `-<pre>` suffix. Pre-release versions sort
// less than the same base without a pre-release suffix.
//
// This intentionally doesn't pull in a full semver library; it would be the
// single largest dep added to vpnkit.
func isNewer(current, latest string) bool {
	if current == latest {
		return false
	}
	if current == "" {
		return latest != ""
	}
	return compareVersion(latest, current) > 0
}

func compareVersion(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	aBase, aPre := splitPre(a)
	bBase, bPre := splitPre(b)
	aParts := splitInts(aBase)
	bParts := splitInts(bBase)
	for i := 0; i < max(len(aParts), len(bParts)); i++ {
		ai, bi := 0, 0
		if i < len(aParts) {
			ai = aParts[i]
		}
		if i < len(bParts) {
			bi = bParts[i]
		}
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	// Bases equal: no pre > pre.
	switch {
	case aPre == "" && bPre != "":
		return 1
	case aPre != "" && bPre == "":
		return -1
	case aPre < bPre:
		return -1
	case aPre > bPre:
		return 1
	}
	return 0
}

func splitPre(s string) (base, pre string) {
	if i := strings.IndexAny(s, "-"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func splitInts(s string) []int {
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// parseMihomoVersion extracts the version token from `mihomo -v` output.
// First line is typically: "Mihomo Meta v1.19.16 linux amd64 with go1.25.3 …"
func parseMihomoVersion(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// take first line
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	for _, f := range strings.Fields(s) {
		if strings.HasPrefix(f, "v") && len(f) > 1 && (f[1] >= '0' && f[1] <= '9') {
			return f
		}
	}
	return ""
}
