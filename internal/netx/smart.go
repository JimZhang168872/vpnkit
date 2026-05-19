package netx

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// SmartClient returns an HTTP client that honors the user's env proxy when
// one is set, and falls back to NoProxyClient when no proxy env var is
// configured.
//
// Pre-rc.11 this also probed the proxy's TCP port (500 ms / later 3 s) and
// silently fell back to NoProxyClient on failure, with the goal of unwedging
// the bootstrap-deadlock case (HTTPS_PROXY pointing at vpnkit's own mihomo
// that hadn't started yet — v0.9.1 anti-deadlock). In practice that probe
// was too clever for the common case: when a CN-network user explicitly set
// HTTPS_PROXY for `vpnkit update`, a momentarily-slow loopback or a slightly
// loaded mihomo failed the probe → fell through to direct github.com → GFW
// kills the connection 5 minutes later. The user's reasonable expectation
// (HTTPS_PROXY is set, please use it) was silently ignored.
//
// rc.11+ behavior: trust env unconditionally. If env points at a dead host,
// fail FAST with the underlying transport error (connection refused, DNS
// error, etc.) so the user gets an actionable message instead of a generic
// 5-min timeout. The bootstrap-deadlock case becomes the user's
// responsibility (unset HTTPS_PROXY before `vpnkit init`), which is the
// install.sh-documented workflow anyway.
//
// This is the right helper for HTTP that fetches GitHub artifacts. The
// controller API (api/client.go, talking to 127.0.0.1:9090) should keep
// NoProxyClient — looping it through env proxy would route loopback traffic
// back through itself.
func SmartClient(timeout time.Duration) *http.Client {
	if envProxyHost() == "" {
		return NoProxyClient(timeout)
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// Fresh closure (not http.ProxyFromEnvironment) so env changes
			// between requests are observable. http.ProxyFromEnvironment
			// caches via sync.Once at first call.
			Proxy: func(*http.Request) (*url.URL, error) { return envProxyURL(), nil },
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

// envProxyURL returns the parsed URL of the first non-empty env proxy var,
// or nil. Unlike http.ProxyFromEnvironment this re-reads env on every call.
func envProxyURL() *url.URL {
	for _, key := range []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"ALL_PROXY", "all_proxy",
	} {
		v := os.Getenv(key)
		if v == "" {
			continue
		}
		u, err := url.Parse(v)
		if err == nil && u.Host != "" {
			return u
		}
	}
	return nil
}

// envProxyHost extracts "host:port" from the first non-empty env proxy var.
// Returns "" when no proxy is configured or the URL is unparseable.
func envProxyHost() string {
	for _, key := range []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"ALL_PROXY", "all_proxy",
	} {
		v := os.Getenv(key)
		if v == "" {
			continue
		}
		u, err := url.Parse(v)
		if err != nil || u.Host == "" {
			continue
		}
		return u.Host
	}
	return ""
}

