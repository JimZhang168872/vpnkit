package proto

import "testing"

func TestVlessBasic(t *testing.T) {
	p, err := Parse("vless://uuid-here@server:443?security=tls&sni=example.com&type=tcp#V1")
	if err != nil {
		t.Fatal(err)
	}
	if p["type"] != "vless" || p["uuid"] != "uuid-here" || p["server"] != "server" || p["port"] != 443 ||
		p["tls"] != true || p["servername"] != "example.com" || p["name"] != "V1" {
		t.Errorf("got %+v", p)
	}
}

func TestVlessReality(t *testing.T) {
	p, err := Parse("vless://u@h:443?security=reality&pbk=PUBKEY&sid=SHORTID&fp=chrome&sni=www.example.com&flow=xtls-rprx-vision#R")
	if err != nil {
		t.Fatal(err)
	}
	if p["client-fingerprint"] != "chrome" || p["flow"] != "xtls-rprx-vision" {
		t.Errorf("got %+v", p)
	}
	rr, _ := p["reality-opts"].(map[string]any)
	if rr["public-key"] != "PUBKEY" || rr["short-id"] != "SHORTID" {
		t.Errorf("reality-opts: %v", rr)
	}
}

func TestVlessWS(t *testing.T) {
	p, err := Parse("vless://u@h:443?type=ws&path=%2Fpath&host=h.example&security=tls#W")
	if err != nil {
		t.Fatal(err)
	}
	wsOpts, _ := p["ws-opts"].(map[string]any)
	if wsOpts["path"] != "/path" {
		t.Errorf("ws-opts: %v", wsOpts)
	}
}
