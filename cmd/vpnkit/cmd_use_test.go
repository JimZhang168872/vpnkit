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

func TestUseHappy(t *testing.T) {
	put := false
	mux := http.NewServeMux()
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "JP-02", "all": []string{"HK-01", "JP-02"}},
			},
		})
	})
	mux.HandleFunc("/proxies/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			put = true
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runUse(&buf, c, "🚀 Proxy", "HK-01", false); err != nil {
		t.Fatal(err)
	}
	if !put {
		t.Error("expected PUT to /proxies/{group}")
	}
	if !strings.Contains(buf.String(), "Proxy → HK-01") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestUseNodeNotInGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"P": map[string]any{"type": "Selector", "now": "n1", "all": []string{"n1"}},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runUse(&buf, c, "P", "n2", false)
	if err == nil || !strings.Contains(err.Error(), "not in group") {
		t.Errorf("got %v", err)
	}
}

func TestUseGroupNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{}})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runUse(&buf, c, "NoSuch", "n", false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("got %v", err)
	}
}
