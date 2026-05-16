package netx

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// clearProxyEnv wipes every proxy-related env var. Tests should call this
// before setting up their own env so leftovers from the parent shell
// (a user with `proxy_on` active) don't bleed into the test.
func clearProxyEnv(t *testing.T) {
	for _, k := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
		"http_proxy", "https_proxy", "all_proxy", "no_proxy",
	} {
		t.Setenv(k, "")
	}
}

func TestSmartClientUsesEnvProxyWhenAlive(t *testing.T) {
	clearProxyEnv(t)
	// A live "proxy" that always 200s a sentinel body — when SmartClient
	// honors env proxy, the request to any URL is routed through this
	// server, so the response body comes from here.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("VIA_PROXY"))
	}))
	defer proxy.Close()

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)

	c := SmartClient(3 * time.Second)
	resp, err := c.Get("http://example.invalid/never-reached")
	if err != nil {
		t.Fatalf("SmartClient should have routed through env proxy: %v", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 16)
	n, _ := resp.Body.Read(buf)
	if string(buf[:n]) != "VIA_PROXY" {
		t.Errorf("body = %q, want VIA_PROXY (proxy hit)", buf[:n])
	}
}

func TestSmartClientFallsBackWhenEnvProxyDead(t *testing.T) {
	clearProxyEnv(t)
	// Find a free port → close it → env proxy points there (dead).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadAddr := ln.Addr().String()
	_ = ln.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("DIRECT"))
	}))
	defer target.Close()

	t.Setenv("HTTP_PROXY", "http://"+deadAddr)
	t.Setenv("HTTPS_PROXY", "http://"+deadAddr)

	c := SmartClient(3 * time.Second)
	resp, err := c.Get(target.URL)
	if err != nil {
		t.Fatalf("dead env proxy should not break the request: %v", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 16)
	n, _ := resp.Body.Read(buf)
	if string(buf[:n]) != "DIRECT" {
		t.Errorf("body = %q, want DIRECT (fell back to no-proxy)", buf[:n])
	}
}

func TestSmartClientNoEnvProxy(t *testing.T) {
	clearProxyEnv(t)

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}))
	defer target.Close()

	c := SmartClient(3 * time.Second)
	resp, err := c.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestEnvProxyHostFromEnv(t *testing.T) {
	cases := []struct {
		envs map[string]string
		want string
	}{
		{map[string]string{"HTTP_PROXY": "http://127.0.0.1:7890"}, "127.0.0.1:7890"},
		{map[string]string{"http_proxy": "http://user:pw@127.0.0.1:7890"}, "127.0.0.1:7890"},
		{map[string]string{"HTTPS_PROXY": "http://10.0.0.1:8080"}, "10.0.0.1:8080"},
		{map[string]string{"ALL_PROXY": "socks5://127.0.0.1:1080"}, "127.0.0.1:1080"},
		{map[string]string{}, ""},
		{map[string]string{"HTTP_PROXY": "not-a-url"}, ""},
	}
	for i, c := range cases {
		for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
			t.Setenv(k, "")
		}
		for k, v := range c.envs {
			t.Setenv(k, v)
		}
		got := envProxyHost()
		if got != c.want {
			t.Errorf("case %d: got %q, want %q", i, got, c.want)
		}
	}
	_ = url.Parse // keep import alive
	_ = strings.HasPrefix
}
