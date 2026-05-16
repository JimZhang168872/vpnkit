package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultUA = "clash-verge/v1.4.0"

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
	resp, err := http.DefaultClient.Do(req)
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
