package proto

import (
	"encoding/base64"
	"net/url"
	"testing"
)

func TestSSRParse(t *testing.T) {
	pwd := base64.RawURLEncoding.EncodeToString([]byte("password"))
	remarks := base64.RawURLEncoding.EncodeToString([]byte("HK-SSR"))
	obfsp := base64.RawURLEncoding.EncodeToString([]byte("obfs.example"))
	body := "1.2.3.4:8388:auth_aes128_md5:aes-256-cfb:tls1.2_ticket_auth:" + pwd +
		"/?obfsparam=" + obfsp + "&remarks=" + remarks
	uri := "ssr://" + base64.RawURLEncoding.EncodeToString([]byte(body))
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "ssr" || p["server"] != "1.2.3.4" || p["port"] != 8388 ||
		p["cipher"] != "aes-256-cfb" || p["password"] != "password" ||
		p["protocol"] != "auth_aes128_md5" || p["obfs"] != "tls1.2_ticket_auth" {
		t.Errorf("got %+v", p)
	}
	if p["obfs-param"] != "obfs.example" {
		t.Errorf("obfs-param: %v", p["obfs-param"])
	}
	if p["name"] != "HK-SSR" {
		t.Errorf("name: %v", p["name"])
	}
	_ = url.QueryEscape
}
