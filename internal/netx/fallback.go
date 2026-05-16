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
// These services come and go; the chain is intentionally short so we fail
// fast if everything is down. SHA256 verification at the call site is the
// real safety belt against compromise.
var BuiltinGitHubMirrors = []string{
	"https://mirror.ghproxy.com/",
	"https://ghproxy.com/",
	"https://ghp.ci/",
}

// OpenWithFallback fetches githubURL through a chain of candidate endpoints
// and returns the body of the first one that responds with a 2xx status.
//
// Order:
//  1. preferredMirror (if non-empty) — the user's or last-success cached value
//  2. direct githubURL — fastest path outside the GFW
//  3. each entry of builtinMirrors — public accelerator fallback
//
// Returns the body, the mirror prefix that won ("" for direct), and any error
// from the LAST attempt. Callers should persist the winning mirror so future
// downloads start with the known-good endpoint.
//
// Per-attempt connect timeout caps how long we hang on any single dead mirror.
//
// Encoded principle: GitHub artifact downloads must transparently fall through
// a chain of known-good endpoints; cache the winner. Adding a new artifact
// download means using this helper, not inventing a new naked http.Get.
func OpenWithFallback(
	ctx context.Context,
	githubURL string,
	preferredMirror string,
	builtinMirrors []string,
	perAttemptTimeout time.Duration,
) (io.ReadCloser, string, error) {
	chain := buildChain(preferredMirror, builtinMirrors)
	var lastErr error
	for _, mirror := range chain {
		url := applyMirrorPrefix(githubURL, mirror)
		body, err := fetch(ctx, url, perAttemptTimeout)
		if err == nil {
			return body, mirror, nil
		}
		lastErr = fmt.Errorf("%s: %w", labelMirror(mirror), err)
	}
	if lastErr == nil {
		lastErr = errors.New("OpenWithFallback: empty chain")
	}
	return nil, "", lastErr
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
	client := NoProxyClient(0) // request-scoped ctx already bounds the attempt
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
