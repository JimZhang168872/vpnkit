package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestInstallerIgnoresEnvProxy is the regression for the v0.9.0 deadlock:
// a user with `proxy_on` set in their shell would inherit HTTP_PROXY pointing
// at 127.0.0.1:7890. vpnkit's installer (bootstrap path that brings mihomo up
// in the first place) would then try to go through that proxy, but mihomo
// wasn't running yet, so the request died with "connection refused".
//
// The fix wires installer's HTTP client through internal/netx.NoProxyClient,
// which explicitly sets Transport.Proxy = nil. This test guards it: an env
// proxy pointing at a dead port must not break installer release fetches.
func TestInstallerIgnoresEnvProxy(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.2.3",
			"assets":   []any{},
		})
	}))
	defer api.Close()

	// Dead proxy on a high loopback port — if the client honors env, requests
	// will fail with "connection refused" exactly like the user's bug report.
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("http_proxy", "http://127.0.0.1:1")
	t.Setenv("https_proxy", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")

	rc := NewReleaseClient(api.URL, "")
	rel, err := rc.LatestForRepo("MetaCubeX/mihomo")
	if err != nil {
		t.Fatalf("installer hit env proxy (regression): %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("tag = %q, want v1.2.3", rel.Tag)
	}
}
