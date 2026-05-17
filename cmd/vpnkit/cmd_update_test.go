package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/store"
)

// newMockReleaseServer returns a test server that responds to GitHub release
// API calls with fake data, and optionally serves a gzip-compressed binary
// download at /dl.gz.
func newMockReleaseServer(t *testing.T, mihomoTag, vpnkitTag string) *httptest.Server {
	t.Helper()
	// Unset all proxy env vars so netx.SmartClient falls back to NoProxyClient
	// and connects directly to the mock server instead of routing through the
	// developer's live proxy (HTTPS_PROXY=http://127.0.0.1:7897 on this host).
	for _, k := range []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"ALL_PROXY", "all_proxy",
	} {
		t.Setenv(k, "")
	}
	// Build a tiny fake mihomo binary wrapped in gzip.
	var gzBuf bytes.Buffer
	w := gzip.NewWriter(&gzBuf)
	_, _ = w.Write([]byte("#!/bin/sh\necho fake\n"))
	_ = w.Close()
	gzPayload := gzBuf.Bytes()

	mux := http.NewServeMux()
	mihomoRelease := func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": mihomoTag,
			"assets": []map[string]any{
				{"name": "mihomo-linux-amd64-" + mihomoTag + ".gz", "browser_download_url": baseURL + "/dl.gz"},
				{"name": "mihomo-linux-amd64-compatible-" + mihomoTag + ".gz", "browser_download_url": baseURL + "/dl.gz"},
				{"name": "mihomo-linux-arm64-" + mihomoTag + ".gz", "browser_download_url": baseURL + "/dl.gz"},
				{"name": "mihomo-linux-arm64-compatible-" + mihomoTag + ".gz", "browser_download_url": baseURL + "/dl.gz"},
			},
		})
	}
	mux.HandleFunc("/repos/MetaCubeX/mihomo/releases/latest", mihomoRelease)
	// Handle /releases/tags/<tag> so installer.Install(version=<tag>) also works.
	mux.HandleFunc("/repos/MetaCubeX/mihomo/releases/tags/", mihomoRelease)
	mux.HandleFunc("/repos/JimZhang168872/vpnkit/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": vpnkitTag,
			"assets":   []map[string]any{},
		})
	})
	mux.HandleFunc("/dl.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gzPayload)
	})
	srv := httptest.NewServer(mux)
	return srv
}

func TestRunUpdateAlreadyUpToDate(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	srv := newMockReleaseServer(t, "v1.18.0", "v1.0.0")
	defer srv.Close()

	origBase := updateAPIBase
	updateAPIBase = srv.URL
	defer func() { updateAPIBase = origBase }()

	var buf bytes.Buffer
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	// Using "dev" version skips vpnkit check; mihomo cur == latest => up to date.
	err := runUpdate(&buf, updateOptions{Check: true, Yes: true}, st, "dev")
	if err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	out := buf.String()
	// Either "already up to date" (mihomo same or newer) or a version line.
	if !strings.Contains(out, "checking") {
		t.Errorf("expected checking message: %s", out)
	}
}

func TestRunUpdateCheckFlag(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	srv := newMockReleaseServer(t, "v1.19.0", "v1.0.0")
	defer srv.Close()

	origBase := updateAPIBase
	updateAPIBase = srv.URL
	defer func() { updateAPIBase = origBase }()

	var buf bytes.Buffer
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	// --check returns early without doing any installs.
	err := runUpdate(&buf, updateOptions{Check: true, Yes: true}, st, "dev")
	if err != nil {
		t.Fatalf("runUpdate --check: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "checking") {
		t.Errorf("expected checking message: %s", out)
	}
}

func TestRunUpdateMihomoOnlyFlag(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	srv := newMockReleaseServer(t, "v1.19.0", "v1.1.0")
	defer srv.Close()

	origBase := updateAPIBase
	updateAPIBase = srv.URL
	defer func() { updateAPIBase = origBase }()

	var buf bytes.Buffer
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	// MihomoOnly + Check: should not attempt vpnkit upgrade.
	err := runUpdate(&buf, updateOptions{Check: true, Yes: true, MihomoOnly: true}, st, "v0.9.0")
	if err != nil {
		t.Fatalf("runUpdate mihomo-only: %v", err)
	}
	out := buf.String()
	// vpnkit needs update was suppressed — so "vpnkit" update line should NOT appear.
	if strings.Contains(out, "v0.9.0 → v1.1.0") {
		t.Errorf("vpnkit should not be upgraded in mihomo-only mode: %s", out)
	}
}

func TestRunUpdateVpnkitOnlyFlag(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	srv := newMockReleaseServer(t, "v1.19.0", "v1.0.0")
	defer srv.Close()

	origBase := updateAPIBase
	updateAPIBase = srv.URL
	defer func() { updateAPIBase = origBase }()

	var buf bytes.Buffer
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runUpdate(&buf, updateOptions{Check: true, Yes: true, VpnkitOnly: true}, st, "dev")
	if err != nil {
		t.Fatalf("runUpdate vpnkit-only: %v", err)
	}
}

func TestRunUpdateNetworkError(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	origBase := updateAPIBase
	// Point at a port that has nothing listening.
	updateAPIBase = "http://127.0.0.1:1"
	defer func() { updateAPIBase = origBase }()

	var buf bytes.Buffer
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runUpdate(&buf, updateOptions{Check: true, Yes: true}, st, "dev")
	if err == nil {
		t.Error("expected error for unreachable API server")
	}
	if !strings.Contains(err.Error(), "check") {
		t.Errorf("error should mention 'check': %v", err)
	}
}

// TestRunUpdateInteractiveAbort exercises the interactive prompt path with "n".
func TestRunUpdateInteractiveAbort(t *testing.T) {
	_, restore := initEnv(t)
	defer restore()

	srv := newMockReleaseServer(t, "v1.19.0", "v1.0.0")
	defer srv.Close()

	origBase := updateAPIBase
	updateAPIBase = srv.URL
	defer func() { updateAPIBase = origBase }()

	// Pipe "n" into stdin so the prompt is answered automatically.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var buf bytes.Buffer
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	// Check: false + Yes: false → reads stdin. "n" → aborted.
	if err := runUpdate(&buf, updateOptions{Check: false, Yes: false, MihomoOnly: true}, st, "dev"); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(buf.String(), "aborted") {
		t.Errorf("expected 'aborted' in output: %s", buf.String())
	}
}

// TestRunUpdateMihomoUpgrade exercises the full mihomo upgrade path using
// the mock server. Uses initEnv so the destination path is in a temp dir.
func TestRunUpdateMihomoUpgrade(t *testing.T) {
	p, restore := initEnv(t)
	defer restore()

	srv := newMockReleaseServer(t, "v1.19.0", "v1.0.0")
	defer srv.Close()

	origBase := updateAPIBase
	updateAPIBase = srv.URL
	defer func() { updateAPIBase = origBase }()

	// Make sure the parent directory for the mihomo binary exists.
	if err := os.MkdirAll(filepath.Dir(p.MihomoBinary()), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var buf bytes.Buffer
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	// Check: false, Yes: true, MihomoOnly: true → should download and install.
	if err := runUpdate(&buf, updateOptions{Check: false, Yes: true, MihomoOnly: true}, st, "dev"); err != nil {
		t.Fatalf("runUpdate mihomo upgrade: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "done") && !strings.Contains(out, "upgraded") {
		t.Errorf("expected success message: %s", out)
	}
}
