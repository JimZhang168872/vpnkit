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

func TestGroupsFiltersBuiltinsAndRendersTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"DIRECT":   map[string]any{"type": "Direct"},
				"REJECT":   map[string]any{"type": "Reject"},
				"GLOBAL":   map[string]any{"type": "Selector", "now": "DIRECT", "all": []string{"DIRECT"}},
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01", "JP-02"}},
				"♻️ Auto":  map[string]any{"type": "URLTest", "now": "HK-01", "all": []string{"HK-01", "JP-02"}},
			},
		})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runGroups(&buf, c, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"🚀 Proxy", "♻️ Auto", "Selector", "URLTest", "HK-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "DIRECT") || strings.Contains(out, "REJECT") || strings.Contains(out, "GLOBAL") {
		t.Errorf("builtin not filtered:\n%s", out)
	}
}

func TestGroupsJSON(t *testing.T) {
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
	if err := runGroups(&buf, c, true); err != nil {
		t.Fatal(err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &arr); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if len(arr) != 1 || arr[0]["name"] != "P" {
		t.Errorf("got %v", arr)
	}
}
