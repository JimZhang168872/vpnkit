package subscription

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSIP008Parse(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"version": 1,
		"servers": []map[string]any{
			{"id": "1", "remarks": "HK", "server": "h1", "server_port": 8388, "method": "aes-128-gcm", "password": "pw1"},
			{"id": "2", "remarks": "SG", "server": "h2", "server_port": 8389, "method": "chacha20-ietf-poly1305", "password": "pw2"},
		},
	})
	got, err := parseSIP008(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
	if got[0]["name"] != "HK" || got[0]["server"] != "h1" {
		t.Errorf("server 0: %+v", got[0])
	}
}

func TestSIP008Invalid(t *testing.T) {
	if _, err := parseSIP008([]byte(`not json`)); err == nil || !strings.Contains(err.Error(), "json") {
		t.Errorf("expected JSON error, got %v", err)
	}
}
