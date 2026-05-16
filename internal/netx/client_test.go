package netx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The regression we're guarding against: a user running vpnkit with
// HTTP_PROXY set (e.g. via `proxy_on`) must NOT have control-plane HTTP
// silently routed through that proxy. We assert that NoProxyClient
// dials the target directly even when the env says go through a dead
// proxy.
func TestNoProxyClientIgnoresEnvProxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hi"))
	}))
	defer target.Close()

	// Point env proxy at a dead loopback address. If the client honors
	// it, the request would die with a "connection refused" — exactly
	// the v0.9.0 bug.
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("http_proxy", "http://127.0.0.1:1")
	t.Setenv("https_proxy", "http://127.0.0.1:1")

	c := NoProxyClient(5 * time.Second)
	resp, err := c.Get(target.URL)
	if err != nil {
		t.Fatalf("NoProxyClient routed through env proxy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestNoProxyClientHasNilProxyOnTransport(t *testing.T) {
	c := NoProxyClient(time.Second)
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", c.Transport)
	}
	if tr.Proxy != nil {
		t.Error("Transport.Proxy is set, want nil")
	}
}
