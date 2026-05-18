package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"vpnkit/internal/netx"
)

// defaultUA mimics a recent mihomo build's outbound UA. Many subscription
// backends gate behind a name+version regex (e.g. doggygosubs returns 4
// dummy "❗您的客户端版本太老❗" nodes when the UA matches `clash-verge` or
// `ClashforWindows/0.20.x`). `mihomo/<ver>` and `clash.meta` both pass.
// We pick mihomo/ because it's the canonical name of our underlying core;
// version is concrete (not "X.Y.Z" placeholder) so naive regexes that
// extract a version succeed. Update periodically as mihomo releases major
// versions — outdated values still work but get less polite treatment.
const defaultUA = "mihomo/v1.19.25"

// Fetch retrieves a subscription body.
//
// HTTP(S) URLs are fetched. Self-contained single-URI schemes
// (vmess://, ss://, ssr://, trojan://, vless://, hysteria://, hysteria2://,
// hy2://, tuic://) are returned verbatim as the body — they ARE the
// subscription and don't need an HTTP round-trip. Downstream Convert/Parse
// will turn them into mihomo proxies.
//
// ua is optional (defaults to clash-verge UA) and is only sent for HTTP(S).
func Fetch(ctx context.Context, url, ua string) ([]byte, error) {
	if !isHTTPURL(url) {
		return []byte(url), nil
	}
	if ua == "" {
		ua = defaultUA
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	// Use SmartClient: it auto-falls-back to NoProxyClient when no env
	// proxy is set or when the env proxy is unreachable. Critical here
	// because users frequently have `HTTPS_PROXY=http://127.0.0.1:7890`
	// (mihomo's mixed-port) exported in their shell. With
	// http.DefaultClient, subscription fetches would route through
	// mihomo — but mihomo's first launch needs subscriptions to load,
	// and mihomo may be down / bootstrapping / waiting on the very
	// subscription we're fetching. Result: confusing "connection refused"
	// chain that's actually "loop through self." SmartClient probes the
	// proxy first and falls back to direct connection on failure.
	client := netx.SmartClient(0)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription fetch %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
