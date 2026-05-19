package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestInstallerBootstrapIgnoresEnvProxy is the regression for the v0.9.0
// deadlock: a user with `proxy_on` set in their shell would inherit
// HTTP_PROXY pointing at 127.0.0.1:7890. vpnkit's installer (bootstrap path
// that brings mihomo up in the first place) would then try to go through
// that proxy, but mihomo wasn't running yet, so the request died with
// "connection refused".
//
// In rc.11+ this is controlled per-call via Options.NoProxy. Bootstrap
// callers (app/bootstrap_sync.go) set NoProxy=true → bypass env. Update
// callers (cmd_update.go) leave it false → honor env (user explicitly set
// HTTPS_PROXY = working GFW workaround).
//
// This test guards the bootstrap branch: even with a dead HTTP_PROXY env
// set, NoProxy=true must let the release fetch through.
func TestInstallerBootstrapIgnoresEnvProxy(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.2.3",
			"assets":   []any{},
		})
	}))
	defer api.Close()

	// Dead proxy on a high loopback port.
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("http_proxy", "http://127.0.0.1:1")
	t.Setenv("https_proxy", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")

	rc := NewReleaseClient(api.URL, "")
	rc.HTTP = noProxyHTTPClient() // simulates Options.NoProxy=true
	rel, err := rc.LatestForRepo("MetaCubeX/mihomo")
	if err != nil {
		t.Fatalf("bootstrap installer hit env proxy (regression): %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("tag = %q, want v1.2.3", rel.Tag)
	}
}

// TestInstallerUpdateHonorsEnvProxy is the dual: when NoProxy=false (the
// update / upgrade case), HTTPS_PROXY must be honored. A dead proxy here
// SHOULD surface an error rather than silently bypassing the user's intent.
func TestInstallerUpdateHonorsEnvProxy(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.2.3",
			"assets":   []any{},
		})
	}))
	defer api.Close()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	rc := NewReleaseClient(api.URL, "")
	// Default client honors env (no NoProxy override).
	_, err := rc.LatestForRepo("MetaCubeX/mihomo")
	if err == nil {
		t.Fatal("update path should surface dead-proxy error; got nil — env proxy was silently bypassed (regression)")
	}
}
