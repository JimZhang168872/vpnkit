package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// MeasureGroup probes connectivity for every member of `group` and returns
// a map keyed by the member's mihomo proxy name. It handles three different
// mihomo group topologies transparently:
//
//  1. `group` is itself a url-test / fallback / load-balance — mihomo's
//     /group/<name>/delay endpoint works directly. One round-trip.
//  2. `group` is a Selector built by vpnkit's assembler (every user-facing
//     subscription / local-nodes group). vpnkit emits a companion url-test
//     group named "<group>-auto" — try that first.
//  3. Neither variant exists (custom user-defined Selector, or a group
//     mihomo loaded from a hand-edited config.yaml). Fall back to
//     enumerating members via /proxies/<group> and calling
//     /proxies/<member>/delay in parallel for each.
//
// On HTTP 401/500/etc the original error from /group/<name>/delay is
// returned — only 404 triggers the fallback chain. mihomo encodes a failed
// measurement (timeout / dial error) as delay=0; that value is preserved
// in the result map so callers can render "timeout" however they want.
func (c *Client) MeasureGroup(ctx context.Context, group, testURL string, timeoutMs int) (map[string]int, error) {
	autoName := group + "-auto"
	results, err := c.GroupDelay(ctx, autoName, testURL, timeoutMs)
	if err == nil {
		return results, nil
	}
	if !isNotFound(err) {
		return nil, err
	}
	// `<group>-auto` doesn't exist — try `<group>` directly (covers
	// non-vpnkit url-test groups loaded from custom config).
	results, err = c.GroupDelay(ctx, group, testURL, timeoutMs)
	if err == nil {
		return results, nil
	}
	if !isNotFound(err) {
		return nil, err
	}
	// Both 404 — fall back to per-member iteration via /proxies.
	return c.measureMembers(ctx, group, testURL, timeoutMs)
}

// measureMembers reads `group`'s member list from /proxies and calls
// /proxies/<member>/delay for each in parallel. Used when neither
// /group/<group>/delay nor /group/<group>-auto/delay exists.
func (c *Client) measureMembers(ctx context.Context, group, testURL string, timeoutMs int) (map[string]int, error) {
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("get proxies: %w", err)
	}
	info, ok := proxies[group]
	if !ok {
		return nil, fmt.Errorf("group %q not found in /proxies", group)
	}
	if len(info.All) == 0 {
		// Selector with no members — return empty rather than erroring so
		// the TUI flash shows "tested 0 nodes" instead of an error toast.
		return map[string]int{}, nil
	}
	type result struct {
		name  string
		delay int
		err   error
	}
	ch := make(chan result, len(info.All))
	var wg sync.WaitGroup
	for _, name := range info.All {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			d, derr := c.Delay(ctx, name, testURL, timeoutMs)
			ch <- result{name: name, delay: d, err: derr}
		}(name)
	}
	wg.Wait()
	close(ch)
	out := make(map[string]int, len(info.All))
	for r := range ch {
		if r.err != nil {
			// Treat per-node failures as timeout (0) — same wire convention
			// as mihomo's group delay endpoint when a single member fails.
			out[r.name] = 0
			continue
		}
		out[r.name] = r.delay
	}
	return out, nil
}

// isNotFound reports whether err came from a mihomo 404 response. The
// api.Client wraps non-2xx responses as `mihomo <method> <path>: <code>
// <body>` strings, so we string-match on " 404 " — adequate until we
// refactor to a typed HTTPError. Returns false on nil or unrelated errors.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.StatusCode == 404
	}
	return strings.Contains(err.Error(), " 404 ")
}

// HTTPError represents a non-2xx mihomo response. Callers can errors.As
// this to inspect the status code. Today only MeasureGroup checks it;
// most paths still get the legacy fmt.Errorf string.
type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("mihomo %s %s: %d %s", e.Method, e.Path, e.StatusCode, e.Body)
}
