package proto

import (
	"encoding/base64"
	"testing"
)

func TestSSStandardSIP002(t *testing.T) {
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password"))
	uri := "ss://" + userinfo + "@1.2.3.4:8388#HK-Server"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "ss" || p["server"] != "1.2.3.4" || p["port"] != 8388 ||
		p["cipher"] != "aes-128-gcm" || p["password"] != "password" || p["name"] != "HK-Server" {
		t.Errorf("got %+v", p)
	}
}

func TestSSLegacyForm(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pwd@example.com:8388"))
	uri := "ss://" + encoded + "#Legacy"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["server"] != "example.com" || p["port"] != 8388 || p["cipher"] != "aes-256-gcm" {
		t.Errorf("got %+v", p)
	}
}

func TestSSPluginV2rayPlugin(t *testing.T) {
	userinfo := base64.StdEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:secret"))
	uri := "ss://" + userinfo + "@a.b:443?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Btls%3Bhost%3Dexample.com#X"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["plugin"] != "v2ray-plugin" {
		t.Errorf("plugin: %v", p["plugin"])
	}
	po, ok := p["plugin-opts"].(map[string]any)
	if !ok {
		t.Fatalf("plugin-opts missing")
	}
	if po["mode"] != "websocket" || po["host"] != "example.com" || po["tls"] != true {
		t.Errorf("plugin-opts: %+v", po)
	}
}

func TestSSURLEncodedPassword(t *testing.T) {
	uri := "ss://aes-128-gcm:p%40ss@h.example:8388#x"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["password"] != "p@ss" {
		t.Errorf("password: %v", p["password"])
	}
}

func TestSSInvalid(t *testing.T) {
	if _, err := Parse("ss://"); err == nil {
		t.Error("expected error for empty body")
	}
}
