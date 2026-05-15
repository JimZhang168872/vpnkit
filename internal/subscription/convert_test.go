package subscription

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestConvertClashYAML(t *testing.T) {
	body := []byte(`port: 7890
proxies:
  - {name: A, type: ss, server: 1.1.1.1, port: 8388, cipher: aes-128-gcm, password: x}
`)
	r, err := Convert(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "clash" {
		t.Errorf("source: %s", r.Source)
	}
	if len(r.Proxies) != 1 || r.Proxies[0]["name"] != "A" {
		t.Errorf("proxies: %+v", r.Proxies)
	}
}

func TestConvertBase64List(t *testing.T) {
	list := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:pwd")) + "@a.b:8388#X\n" +
		"trojan://pw@host:443?sni=s#Y"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(list)))
	r, err := Convert(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "base64-list" || len(r.Proxies) != 2 {
		t.Errorf("source=%s proxies=%d", r.Source, len(r.Proxies))
	}
}

func TestConvertSingleURI(t *testing.T) {
	r, err := Convert([]byte("trojan://pw@host:443#One"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "uri" || len(r.Proxies) != 1 {
		t.Errorf("source=%s proxies=%d", r.Source, len(r.Proxies))
	}
}

func TestConvertSkipsMalformed(t *testing.T) {
	list := "vmess://bad\nss://" + base64.StdEncoding.EncodeToString([]byte("m:p")) + "@h:1#OK"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(list)))
	r, err := Convert(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Proxies) != 1 || len(r.Errors) != 1 {
		t.Errorf("proxies=%d errors=%d", len(r.Proxies), len(r.Errors))
	}
	if !strings.Contains(r.Errors[0].Error(), "vmess") {
		t.Errorf("error: %v", r.Errors[0])
	}
}
