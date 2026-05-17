package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MetaCubeX/mihomo/releases/latest" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.19.16",
			"assets": []map[string]any{
				{"name": "mihomo-linux-amd64-v1.19.16.gz", "browser_download_url": "https://example.com/x.gz"},
				{"name": "mihomo-linux-amd64-compatible-v1.19.16.gz", "browser_download_url": "https://example.com/c.gz"},
			},
		})
	}))
	defer srv.Close()
	rc := NewReleaseClient(srv.URL, "")
	rel, err := rc.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.Tag != "v1.19.16" {
		t.Errorf("tag: %s", rel.Tag)
	}
	if len(rel.Assets) != 2 {
		t.Errorf("assets: %d", len(rel.Assets))
	}
}

func TestFindAssetURL(t *testing.T) {
	rel := Release{
		Tag: "v1.19.16",
		Assets: []Asset{
			{Name: "mihomo-linux-amd64-v1.19.16.gz", URL: "https://example.com/a.gz"},
			{Name: "mihomo-linux-amd64-compatible-v1.19.16.gz", URL: "https://example.com/b.gz"},
		},
	}
	url, err := rel.AssetURL("mihomo-linux-amd64-compatible-v1.19.16.gz")
	if err != nil {
		t.Fatalf("AssetURL: %v", err)
	}
	if url != "https://example.com/b.gz" {
		t.Errorf("wrong url: %s", url)
	}
	if _, err := rel.AssetURL("missing.gz"); err == nil {
		t.Errorf("missing asset: expected error")
	}
}

