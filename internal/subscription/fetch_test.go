package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSendsUA(t *testing.T) {
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	body, err := Fetch(context.Background(), srv.URL, "clash-verge/1.0")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "body" {
		t.Errorf("body: %s", body)
	}
	if seenUA != "clash-verge/1.0" {
		t.Errorf("UA: %s", seenUA)
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := Fetch(context.Background(), srv.URL, ""); err == nil {
		t.Error("expected error")
	}
}
