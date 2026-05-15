package proto

import "testing"

func TestParseDispatchesScheme(t *testing.T) {
	tests := []struct {
		uri    string
		scheme string
	}{
		{"vmess://abc", "vmess"},
		{"ss://abc", "ss"},
		{"ssr://abc", "ssr"},
		{"trojan://abc", "trojan"},
		{"vless://abc", "vless"},
		{"hysteria://abc", "hysteria"},
		{"hysteria2://abc", "hysteria2"},
		{"tuic://abc", "tuic"},
	}
	for _, tt := range tests {
		got, _, _ := schemeOf(tt.uri)
		if got != tt.scheme {
			t.Errorf("schemeOf(%s) = %s, want %s", tt.uri, got, tt.scheme)
		}
	}
}

func TestParseUnknownScheme(t *testing.T) {
	_, err := Parse("ftp://example.com")
	if err == nil {
		t.Error("expected error for unknown scheme")
	}
}
