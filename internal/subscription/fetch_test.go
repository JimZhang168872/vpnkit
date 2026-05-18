package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestFetchDefaultUAIsMihomoFamily — empty UA must fall back to a UA that
// real-world subscription backends accept. We hit a regression where
// doggygosubs.com (and similar "客户端版本太老" gating providers) returned
// 4 fake "❗您的客户端版本太老❗" nodes when the UA was the old
// `clash-verge/v1.4.0` default. Switching to `mihomo/<ver>` makes them
// return the real proxy list. Don't pin the exact version (it'll age
// faster than the codebase), just enforce the `mihomo/` prefix.
func TestFetchDefaultUAIsMihomoFamily(t *testing.T) {
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	if _, err := Fetch(context.Background(), srv.URL, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(seenUA, "mihomo/") {
		t.Errorf("default UA must start with mihomo/, got %q", seenUA)
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

// TestFetch_NonHTTPSchemePassthrough verifies that single-URI subscriptions
// (vmess://, ss://, hysteria2://, hy2://, trojan://, vless://, tuic://, ssr://,
// hysteria://) are returned verbatim as the body without an HTTP request,
// because those schemes are self-contained and not fetchable.
func TestFetch_NonHTTPSchemePassthrough(t *testing.T) {
	cases := []string{
		"hysteria2://CBAI0bv97b21KRjXw3fDArlnW/ymWTur@jim.gulujili.xyz:8443?security=tls&fp=chrome&alpn=h3&sni=jim.gulujili.xyz#Hy2-entrance-CN-jim-hy2",
		"hy2://pass@example.com:443#node",
		"hysteria://example.com:443?protocol=udp&auth=secret&peer=peer.example#H1",
		"vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOiI0NDMifQ==",
		"ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@example.com:8388#node",
		"ssr://ZXhhbXBsZS5jb206ODM4ODphZWFkLWNoYWNoYTIwLWlldGYtcG9seTEzMDU6cGFzcw",
		"trojan://password@example.com:443?sni=example.com#node",
		"vless://uuid@example.com:443?security=tls&sni=example.com#node",
		"tuic://uuid:password@example.com:443?congestion_control=bbr#node",
	}
	for _, uri := range cases {
		t.Run(uri[:min(len(uri), 20)], func(t *testing.T) {
			body, err := Fetch(context.Background(), uri, "")
			if err != nil {
				t.Fatalf("non-HTTP scheme should not error, got: %v", err)
			}
			if string(body) != uri {
				t.Errorf("body should be URL verbatim\n  want: %s\n  got:  %s", uri, body)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
