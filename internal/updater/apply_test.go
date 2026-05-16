package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Clear any proxy env vars that may have leaked in from the parent shell
// (a developer with `proxy_on` active). Without this, SmartClient inside
// DownloadAndApplyVpnkit would route httptest URLs through 127.0.0.1:7890
// and produce garbled "responses" or sha mismatches.
func init() {
	for _, k := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
		"http_proxy", "https_proxy", "all_proxy",
	} {
		_ = os.Unsetenv(k)
	}
}

// makeTarGz wraps a single "vpnkit" file with `body` content into a tarball
// matching the layout produced by goreleaser.
func makeTarGz(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name: "vpnkit",
		Mode: 0o755,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestApplyVpnkitReplacesBinary(t *testing.T) {
	dir := t.TempDir()

	// Existing binary at dst (pretend it's the old version).
	dst := filepath.Join(dir, "vpnkit")
	if err := os.WriteFile(dst, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	body := []byte("NEWBINARYCONTENT")
	tarball := makeTarGz(t, body)
	sum := sha256.Sum256(tarball)
	sumHex := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarball)
	}))
	defer srv.Close()

	if _, err := DownloadAndApplyVpnkit(srv.URL, sumHex, dst, "", nil); err != nil {
		t.Fatalf("DownloadAndApplyVpnkit: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, body) {
		t.Errorf("binary not replaced: got %q want %q", got, body)
	}
	info, _ := os.Stat(dst)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("perm = %v, want 0755", info.Mode().Perm())
	}
}

func TestApplyVpnkitRejectsBadSHA(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "vpnkit")
	if err := os.WriteFile(dst, []byte("KEEPME"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("NEW")
	tarball := makeTarGz(t, body)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarball)
	}))
	defer srv.Close()

	_, err := DownloadAndApplyVpnkit(srv.URL, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", dst, "", nil)
	if err == nil {
		t.Fatal("expected SHA mismatch error")
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "KEEPME" {
		t.Errorf("existing binary was clobbered on SHA mismatch: %q", got)
	}
}

func TestApplyVpnkitWithoutSHACheck(t *testing.T) {
	// Empty expected sha → skip verify (CLI path: SHA256SUMS already checked
	// separately, or user opted out).
	dir := t.TempDir()
	dst := filepath.Join(dir, "vpnkit")
	body := []byte("INSTALLED")
	tarball := makeTarGz(t, body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarball)
	}))
	defer srv.Close()
	if _, err := DownloadAndApplyVpnkit(srv.URL, "", dst, "", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, body) {
		t.Errorf("body mismatch: got %q", got)
	}
}

func TestParseSHA256Sums(t *testing.T) {
	sums := `abc123  vpnkit_0.9.0_linux_amd64.tar.gz
def456  vpnkit_0.9.0_linux_arm64.tar.gz
fff999  SHA256SUMS
`
	got, err := parseSHA256Sums(sums, "vpnkit_0.9.0_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc123" {
		t.Errorf("got %q want abc123", got)
	}
	if _, err := parseSHA256Sums(sums, "missing.tar.gz"); err == nil {
		t.Error("expected error for missing entry")
	}
}

// Just smoke-test the URL builder.
func TestVpnkitAssetName(t *testing.T) {
	got := vpnkitAssetName("v0.9.0", "amd64")
	want := "vpnkit_0.9.0_linux_amd64.tar.gz"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

var _ = fmt.Sprintf
