package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetConfigs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs" || r.Method != http.MethodGet {
			t.Fatalf("got %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode":       "rule",
			"log-level":  "info",
			"mixed-port": 7890,
			"allow-lan":  false,
			"secret":     "abc",
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	cfg, err := c.GetConfigs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "rule" || cfg.MixedPort != 7890 || cfg.LogLevel != "info" {
		t.Errorf("got %+v", cfg)
	}
}
