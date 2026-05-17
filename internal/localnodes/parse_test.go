package localnodes

import (
	"encoding/base64"
	"testing"
)

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

func TestParseURIVmess(t *testing.T) {
	// vmess://BASE64({"v":"2","ps":"node-name","add":"1.2.3.4","port":"8443","id":"uuid-here","aid":"0","net":"ws","type":"none","host":"","path":"/path","tls":"tls"})
	payload := `{"v":"2","ps":"JP-Tokyo","add":"1.2.3.4","port":"8443","id":"11111111-2222-3333-4444-555555555555","aid":"0","net":"ws","type":"none","host":"example.com","path":"/path","tls":"tls"}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "vmess" || n.Server != "1.2.3.4" || n.Port != 8443 {
		t.Errorf("basic fields: %+v", n)
	}
	if n.Name != "JP-Tokyo" {
		t.Errorf("name from ps: %q", n.Name)
	}
	if n.Fields["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid: %v", n.Fields["uuid"])
	}
	if n.Fields["network"] != "ws" {
		t.Errorf("network: %v", n.Fields["network"])
	}
	if ws, _ := n.Fields["ws-opts"].(map[string]any); ws["path"] != "/path" {
		t.Errorf("ws-opts.path: %v", ws)
	}
}

func TestParseURITrojan(t *testing.T) {
	uri := "trojan://password123@1.2.3.4:8443?sni=example.com&alpn=h2,http/1.1#TR-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "trojan" || n.Server != "1.2.3.4" || n.Port != 8443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Name != "TR-1" {
		t.Errorf("name: %q", n.Name)
	}
	if n.Fields["password"] != "password123" {
		t.Errorf("password: %v", n.Fields["password"])
	}
	if n.Fields["sni"] != "example.com" {
		t.Errorf("sni: %v", n.Fields["sni"])
	}
	if alpn, _ := n.Fields["alpn"].([]string); len(alpn) != 2 || alpn[0] != "h2" {
		t.Errorf("alpn: %v", n.Fields["alpn"])
	}
}

func TestParseURIVless(t *testing.T) {
	// vless://UUID@host:port?encryption=none&security=reality&pbk=KEY&sni=...&type=tcp#name
	uri := "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?encryption=none&security=reality&pbk=publicKeyBase64&sni=example.com&type=tcp&flow=xtls-rprx-vision#VL-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "vless" || n.Server != "1.2.3.4" || n.Port != 443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Fields["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid: %v", n.Fields["uuid"])
	}
	if n.Fields["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow: %v", n.Fields["flow"])
	}
	if n.Fields["tls"] != true {
		t.Errorf("tls: %v", n.Fields["tls"])
	}
	if r, _ := n.Fields["reality-opts"].(map[string]any); r["public-key"] != "publicKeyBase64" {
		t.Errorf("reality public-key: %v", n.Fields["reality-opts"])
	}
}
