package installer

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallLatestE2E(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("only amd64/arm64 supported")
	}
	payload := []byte("#!/bin/sh\necho mihomo v0.0.0-fake\n")
	gzPayload := mustGzip(t, payload)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/MetaCubeX/mihomo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.0.0-fake",
			"assets": []map[string]any{
				{"name": "mihomo-linux-" + runtime.GOARCH + "-compatible-v0.0.0-fake.gz", "browser_download_url": baseURL + "/dl.gz"},
				{"name": "mihomo-linux-" + runtime.GOARCH + "-v0.0.0-fake.gz", "browser_download_url": baseURL + "/dl.gz"},
			},
		})
	})
	mux.HandleFunc("/dl.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gzPayload)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "mihomo")
	opts := Options{
		APIBase:     srv.URL,
		Mirror:      "",
		Dst:         dst,
		Version:     "",
		ForceCompat: nil,
	}
	res, err := Install(opts, nil)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Version != "v0.0.0-fake" {
		t.Errorf("version: %s", res.Version)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch")
	}
}

func mustGzip(t *testing.T, p []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(p)
	_ = gw.Close()
	return buf.Bytes()
}
