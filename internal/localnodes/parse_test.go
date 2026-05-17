package localnodes

import "testing"

func TestParseURIShadowsocks(t *testing.T) {
	// Format: ss://BASE64(method:password)@host:port#name
	// Pre-computed: base64("aes-256-gcm:secret") == "YWVzLTI1Ni1nY206c2VjcmV0"
	uri := "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-A"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "ss" {
		t.Errorf("proto: %q", n.Proto)
	}
	if n.Server != "1.2.3.4" || n.Port != 8388 {
		t.Errorf("server/port: %q/%d", n.Server, n.Port)
	}
	if n.Name != "HK-A" {
		t.Errorf("name: %q", n.Name)
	}
	if n.Fields["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher: %v", n.Fields["cipher"])
	}
	if n.Fields["password"] != "secret" {
		t.Errorf("password: %v", n.Fields["password"])
	}
}
