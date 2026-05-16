package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestNodesHumanShowsCurrentAndDelays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01", "JP-02", "KR-04"}},
				"HK-01": map[string]any{
					"type":    "Shadowsocks",
					"history": []map[string]any{{"time": "t1", "delay": 45}},
				},
				"JP-02": map[string]any{
					"type":    "Shadowsocks",
					"history": []map[string]any{{"time": "t1", "delay": 87}},
				},
				"KR-04": map[string]any{"type": "Shadowsocks"},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runNodes(&buf, c, "🚀 Proxy", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"✓ HK-01", "JP-02", "45", "87", "(no test)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestNodesGroupNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{"P": map[string]any{"type": "Selector", "all": []string{}}},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runNodes(&buf, c, "NotExist", false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestNodesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"P":  map[string]any{"type": "Selector", "now": "n1", "all": []string{"n1"}},
				"n1": map[string]any{"type": "Shadowsocks", "history": []map[string]any{{"time": "t", "delay": 12}}},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runNodes(&buf, c, "P", true); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if got["group"] != "P" || got["current"] != "n1" {
		t.Errorf("got %v", got)
	}
}
