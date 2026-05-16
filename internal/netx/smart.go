package netx

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// SmartClient returns an HTTP client that *adaptively* honors the env proxy:
//
//   - If HTTP_PROXY/HTTPS_PROXY/ALL_PROXY (or their lowercase variants) point
//     at a host that responds to a 500 ms TCP dial, return a client that uses
//     the stdlib's http.ProxyFromEnvironment. Rationale: vpnkit IS often the
//     proxy, and a running mihomo is the most reliable path out for users
//     behind the GFW. Bypassing the user's own working proxy makes no sense.
//
//   - Otherwise return NoProxyClient: env says go through a proxy, but the
//     proxy isn't listening (typically mihomo hasn't been started yet — the
//     v0.9.1 bootstrap-deadlock case). Bypass env so we don't deadlock.
//
// The probe is one-shot per call. Long-lived callers should cache the
// returned client across multiple requests.
//
// This is the right helper for HTTP that fetches GitHub artifacts. The
// controller API (api/client.go, talking to 127.0.0.1:9090) should keep
// NoProxyClient — looping it through env proxy would route loopback traffic
// back through itself.
func SmartClient(timeout time.Duration) *http.Client {
	if envProxyHost() == "" {
		return NoProxyClient(timeout)
	}
	if !proxyReachable(envProxyHost(), 500*time.Millisecond) {
		return NoProxyClient(timeout)
	}
	// Don't use http.ProxyFromEnvironment — stdlib caches the proxy URL via
	// sync.Once at first call, so subsequent env changes are invisible
	// (breaks tests, and breaks our probe-then-reconfigure logic). Use a
	// fresh closure that reads env on every request instead.
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
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

// proxyReachable returns true if a TCP dial to hostPort succeeds within
// timeout. Used to detect whether the env-declared proxy is actually up.
func proxyReachable(hostPort string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", hostPort, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
