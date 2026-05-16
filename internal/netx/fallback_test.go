package netx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenWithFallbackPicksFirstWorkingMirror(t *testing.T) {
	// Direct github will "time out" by being a server we close before request.
	deadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang forever — caller's per-attempt timeout will trip.
		time.Sleep(10 * time.Second)
	}))
	deadServer.Close() // immediately closed → connect refused

	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("HELLO"))
	}))
	defer live.Close()

	body, winner, err := OpenWithFallback(context.Background(),
		deadServer.URL+"/asset.tar.gz",
		"", // no preferred
		[]string{
			live.URL + "/",
		},
		2*time.Second,
	)
	if err != nil {
		t.Fatalf("OpenWithFallback: %v", err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "HELLO" {
		t.Errorf("got %q want HELLO", data)
	}
	if !strings.HasPrefix(winner, live.URL) {
		t.Errorf("winner = %q, want prefix %s", winner, live.URL)
	}
}

func TestOpenWithFallbackPreferredFirst(t *testing.T) {
	preferred := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("PREFERRED"))
	}))
	defer preferred.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OTHER"))
	}))
	defer other.Close()

	body, winner, err := OpenWithFallback(context.Background(),
		"http://does-not-resolve.invalid/asset.gz",
		preferred.URL+"/",
		[]string{other.URL + "/"},
		2*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "PREFERRED" {
		t.Errorf("preferred should win, got %q", data)
	}
	if !strings.HasPrefix(winner, preferred.URL) {
		t.Errorf("winner = %q", winner)
	}
}

func TestOpenWithFallbackAllFailReturnsLastErr(t *testing.T) {
	_, _, err := OpenWithFallback(context.Background(),
		"http://nonexistent-a.invalid/x",
		"",
		[]string{
			"http://nonexistent-b.invalid/",
			"http://nonexistent-c.invalid/",
		},
		1*time.Second,
	)
	if err == nil {
		t.Fatal("expected error when all endpoints fail")
	}
}

func TestApplyMirrorPrefix(t *testing.T) {
	cases := []struct {
		mirror, url, want string
	}{
		{"", "https://github.com/a/b", "https://github.com/a/b"},
		{"https://ghproxy.com/", "https://github.com/a/b", "https://ghproxy.com/https://github.com/a/b"},
		{"https://ghproxy.com", "https://github.com/a/b", "https://ghproxy.com/https://github.com/a/b"},
	}
	for _, c := range cases {
		if got := applyMirrorPrefix(c.url, c.mirror); got != c.want {
			t.Errorf("applyMirrorPrefix(%q, %q) = %q, want %q", c.url, c.mirror, got, c.want)
		}
	}
}

func TestOpenWithFallbackHTTP404TriesNext(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", 404)
	}))
	defer notFound.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FOUND"))
	}))
	defer ok.Close()

	body, winner, err := OpenWithFallback(context.Background(),
		notFound.URL+"/asset",
		"",
		[]string{ok.URL + "/"},
		2*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "FOUND" {
		t.Errorf("got %q", data)
	}
	if !strings.HasPrefix(winner, ok.URL) {
		t.Errorf("winner = %q", winner)
	}
}
