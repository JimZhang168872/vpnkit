package netx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BuiltinGitHubMirrors is the fallback chain of public github-acceleration
// services tried when neither the user-preferred mirror nor direct github.com
// works. Order matters — first entry tried first.
//
// These services come and go; the chain is intentionally short and the
// onAttempt callback at the call site reports each attempt's outcome in real
// time so users can pick a working one themselves when the list rots.
// SHA256 verification at the call site is the safety belt against compromise.
//
// Last verified (probe from inside China): see commit message of the change
// that touched this list. If most entries here time out, look at the failure
// log printed by `vpnkit update` to discover which is currently up and set
// release_mirror manually.
var BuiltinGitHubMirrors = []string{
	"https://github.91chi.fun/", // 1.7s HEAD direct → 302 (verified 2026-05-16)
	"https://ghproxy.com/",      // 5s HEAD → 301 (verified 2026-05-16)
	"https://mirror.ghproxy.com/",
	"https://ghps.cc/",
	"https://hub.gitmirror.com/",
}

// OnAttempt is called once per chain entry as it succeeds or fails. mirror is
// "" for direct github; err is nil on success. Useful for streaming progress
// to the user so a long fallback chain doesn't look like a hang.
type OnAttempt func(mirror string, err error)

// OpenWithFallback fetches githubURL through a chain of candidate endpoints
// and returns the body of the first one that responds with a 2xx status.
//
// Order:
//  1. preferredMirror (if non-empty) — the user's or last-success cached value
//  2. direct githubURL — fastest path outside the GFW
//  3. each entry of builtinMirrors — public accelerator fallback
//
// onAttempt (optional) receives every attempt's outcome in real time.
//
// On total failure the returned error is `errors.Join`-aggregated so every
// individual attempt's reason is visible — single-lastErr was a footgun that
// hid 3 timeouts and surfaced only "ghp.ci: no such host" to users.
//
// Per-attempt timeout caps how long we hang on any single dead mirror.
//
// Encoded principle: GitHub artifact downloads must transparently fall through
// a chain of known-good endpoints; report each attempt; cache the winner.
// Adding a new artifact download means using this helper, not inventing a new
// naked http.Get.
func OpenWithFallback(
	ctx context.Context,
	githubURL string,
	preferredMirror string,
	builtinMirrors []string,
	perAttemptTimeout time.Duration,
	onAttempt OnAttempt,
) (io.ReadCloser, string, error) {
	chain := buildChain(preferredMirror, builtinMirrors)
	var attemptErrs []error
	for _, mirror := range chain {
		url := applyMirrorPrefix(githubURL, mirror)
		body, err := fetch(ctx, url, perAttemptTimeout)
		if onAttempt != nil {
			onAttempt(mirror, err)
		}
		if err == nil {
			return body, mirror, nil
		}
		attemptErrs = append(attemptErrs, fmt.Errorf("%s: %w", labelMirror(mirror), err))
	}
	if len(attemptErrs) == 0 {
		return nil, "", errors.New("OpenWithFallback: empty chain")
	}
	return nil, "", errors.Join(attemptErrs...)
}

// buildChain merges preferred + direct + builtins, dedupes preserving order.
// preferred goes first when present; direct ("") always present unless already
// in preferred slot. Builtins follow, each only once.
func buildChain(preferred string, builtins []string) []string {
	seen := map[string]bool{}
	chain := []string{}
	add := func(m string) {
		key := strings.TrimRight(m, "/")
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = true
		chain = append(chain, m)
	}
	if preferred != "" {
		add(preferred)
	}
	add("") // direct
	for _, m := range builtins {
		add(m)
	}
	return chain
}

func fetch(ctx context.Context, url string, timeout time.Duration) (io.ReadCloser, error) {
	rctx, cancel := context.WithTimeout(ctx, timeout)
	// NOTE: do not defer cancel here — caller owns body lifecycle. We give it
	// to a small wrapper below that cancels on Close.
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	// Use SmartClient so a running mihomo (env proxy alive) is the preferred
	// path, while bootstrap with a dead proxy gracefully degrades to direct
	// + mirror chain. See SmartClient docstring for the full rule.
	client := SmartClient(0) // request-scoped ctx already bounds the attempt
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return &cancelCloser{ReadCloser: resp.Body, cancel: cancel}, nil
}

// cancelCloser cancels the per-attempt context when the body is closed, so
// keep-alives + transport bookkeeping wind down cleanly.
type cancelCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// applyMirrorPrefix wraps githubURL with mirror prefix. Empty mirror returns
// URL unchanged. Trailing slash on mirror is normalized.
func applyMirrorPrefix(githubURL, mirror string) string {
	if mirror == "" {
		return githubURL
	}
	if !strings.HasSuffix(mirror, "/") {
		mirror += "/"
	}
	return mirror + githubURL
}

func labelMirror(m string) string {
	if m == "" {
		return "direct"
	}
	return strings.TrimSuffix(strings.TrimPrefix(m, "https://"), "/")
}
