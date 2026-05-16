// Package netx provides HTTP client helpers for vpnkit's *control plane* —
// every outbound request whose purpose is to bring up or manage the proxy
// itself (binary downloads, controller API, traffic stream, …).
//
// These clients explicitly set `Transport.Proxy = nil`, so they ignore the
// caller's environment proxy variables. This is critical: vpnkit's
// bootstrap installer fetches GitHub before mihomo is up. If we honored
// `HTTP_PROXY=http://...@127.0.0.1:7890` (which users routinely set via
// `vpnkit env` / `proxy_on`), the installer would try to dial the mihomo
// port we haven't started yet — chicken-and-egg deadlock.
//
// The rule encoded here:
//
//	control-plane HTTP must never depend on the proxy it is bringing up.
//
// New control-plane HTTP call sites should use NoProxyClient(timeout)
// rather than constructing a bare http.Client. Anything that DOES want
// to honor the user's proxy (subscription fetch, the `vpnkit ip` test
// against ipinfo.io) keeps using the stdlib defaults.
package netx

import (
	"net"
	"net/http"
	"time"
)

// NoProxyClient returns an *http.Client whose transport is identical to
// http.DefaultTransport's behavior except Proxy is hard-set to nil. The
// caller still passes its own timeout.
//
// timeout=0 means no overall request timeout (suitable for long-lived
// streams; callers using a request-scoped context.WithTimeout don't need
// the client-level one anyway).
func NoProxyClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: NoProxyTransport(),
	}
}

// NoProxyTransport returns a fresh transport with Proxy explicitly nil.
// Other defaults match the stdlib's DefaultTransport so we don't lose
// connection pooling or DialContext behavior.
func NoProxyTransport() *http.Transport {
	return &http.Transport{
		Proxy: nil, // explicit — do not honor HTTP_PROXY env
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
