package netx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

var unsetenv = os.Unsetenv

// All fallback tests must run with no env proxy so SmartClient's
// "use env proxy if alive" branch doesn't redirect them through a
// leftover from the parent shell.
func init() {
	for _, k := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
		"http_proxy", "https_proxy", "all_proxy", "no_proxy",
	} {
		_ = unsetenv(k)
	}
}

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
		nil,
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
		nil,
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
	var attempts []string
	_, _, err := OpenWithFallback(context.Background(),
		"http://nonexistent-a.invalid/x",
		"",
		[]string{
			"http://nonexistent-b.invalid/",
			"http://nonexistent-c.invalid/",
		},
		1*time.Second,
		func(mirror string, e error) {
			attempts = append(attempts, mirror)
		},
	)
	if err == nil {
		t.Fatal("expected error when all endpoints fail")
	}
	// onAttempt fires once per chain entry; "" (direct) + 2 builtins = 3.
	if len(attempts) != 3 {
		t.Errorf("onAttempt fired %d times, want 3: %v", len(attempts), attempts)
	}
	// Aggregated error must mention each failed endpoint, not just the last.
	if !strings.Contains(err.Error(), "nonexistent-a") ||
		!strings.Contains(err.Error(), "nonexistent-b") ||
		!strings.Contains(err.Error(), "nonexistent-c") {
		t.Errorf("aggregated error missing entries:\n%s", err)
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
		nil,
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
