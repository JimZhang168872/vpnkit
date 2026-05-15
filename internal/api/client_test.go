package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("auth header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.19.16", "meta": true})
	}))
	defer srv.Close()
	c := New(srv.URL, "secret")
	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v.Version != "1.19.16" || !v.Meta {
		t.Errorf("got %+v", v)
	}
}

func TestSetMode(t *testing.T) {
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method: %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.SetMode(context.Background(), "global"); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "global" {
		t.Errorf("body: %v", body)
	}
}

func TestErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	_, err := c.Version(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got %v", err)
	}
}
