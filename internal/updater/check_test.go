package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseMihomoVersionOutput(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Mihomo Meta v1.19.16 linux amd64 with go1.25.3", "v1.19.16"},
		{"Mihomo Meta v1.20.0-alpha linux amd64", "v1.20.0-alpha"},
		{"garbage", ""},
		{"", ""},
		{"v1.19.16", "v1.19.16"}, // also accept bare version line
	}
	for _, tc := range tests {
		if got := parseMihomoVersion(tc.in); got != tc.want {
			t.Errorf("parseMihomoVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsDevBuild(t *testing.T) {
	cases := []struct {
		v   string
		dev bool
	}{
		{"", true},
		{"dev", true},
		{"0.8.4-dev", true},
		{"0.8.4-dev+abc123", true},
		{"0.8.4", false},
		{"v0.8.4", false},
		{"0.9.0-rc.1", false}, // pre-release, but not dev
	}
	for _, tc := range cases {
		if got := isDevBuild(tc.v); got != tc.dev {
			t.Errorf("isDevBuild(%q) = %v, want %v", tc.v, got, tc.dev)
		}
	}
}

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		current, latest string
		newer           bool
	}{
		{"0.8.4", "0.9.0", true},
		{"v0.8.4", "v0.9.0", true},
		{"0.9.0", "0.9.0", false},
		{"0.9.0", "0.8.4", false},
		{"v1.19.16", "v1.19.24", true},
		{"v1.19.24", "v1.19.16", false},
	}
	for _, tc := range cases {
		got := isNewer(tc.current, tc.latest)
		if got != tc.newer {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.newer)
		}
	}
}

func TestCheckHappyPath(t *testing.T) {
	// Mock the GitHub API endpoint for vpnkit's repo.
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/JimZhang168872/vpnkit/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.9.0",
		})
	})
	mux.HandleFunc("/repos/MetaCubeX/mihomo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.19.30",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	info, err := Check(Opts{
		VpnkitCurrent:  "v0.8.4",
		MihomoCurrent:  "v1.19.16",
		APIBase:        srv.URL,
		Repo:           "JimZhang168872/vpnkit",
		MihomoRepo:     "MetaCubeX/mihomo",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.VpnkitLatest != "v0.9.0" || !info.VpnkitNeedsUpdate {
		t.Errorf("vpnkit: %+v", info)
	}
	if info.MihomoLatest != "v1.19.30" || !info.MihomoNeedsUpdate {
		t.Errorf("mihomo: %+v", info)
	}
	if !info.HasUpdate() {
		t.Errorf("HasUpdate should be true: %+v", info)
	}
}

func TestCheckSkipsDevBuild(t *testing.T) {
	// dev build → no API hit, no update
	info, err := Check(Opts{
		VpnkitCurrent: "0.8.4-dev",
		MihomoCurrent: "v1.19.16",
		// no APIBase / Repo — would fail if it tried to hit network
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.VpnkitNeedsUpdate {
		t.Errorf("dev build should never need update: %+v", info)
	}
}

func TestCheckEmptyMihomoCurrent(t *testing.T) {
	// Empty mihomo version (binary not installed yet) — should still check
	// vpnkit and signal mihomo update wanted (any version > "" is newer).
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/x/y/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.9.0"})
	})
	mux.HandleFunc("/repos/m/m/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.0.0"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	info, err := Check(Opts{
		VpnkitCurrent: "v0.8.0", MihomoCurrent: "",
		APIBase: srv.URL, Repo: "x/y", MihomoRepo: "m/m",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.MihomoNeedsUpdate {
		t.Errorf("empty mihomo current → should signal update: %+v", info)
	}
}
