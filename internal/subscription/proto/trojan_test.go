package proto

import "testing"

func TestTrojanBasic(t *testing.T) {
	p, err := Parse("trojan://secret@example.com:443?sni=relay.example.com#T1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "trojan" || p["server"] != "example.com" || p["port"] != 443 ||
		p["password"] != "secret" || p["sni"] != "relay.example.com" || p["name"] != "T1" {
		t.Errorf("got %+v", p)
	}
}

func TestTrojanWS(t *testing.T) {
	p, err := Parse("trojan://pw@h:443?type=ws&path=%2Fws&host=h.example#W")
	if err != nil {
		t.Fatal(err)
	}
	if p["network"] != "ws" {
		t.Errorf("network: %v", p["network"])
	}
	wsOpts, ok := p["ws-opts"].(map[string]any)
	if !ok || wsOpts["path"] != "/ws" {
		t.Errorf("ws-opts: %v", p["ws-opts"])
	}
}

func TestTrojanAllowInsecure(t *testing.T) {
	p, err := Parse("trojan://pw@h:443?allowInsecure=1#X")
	if err != nil {
		t.Fatal(err)
	}
	if p["skip-cert-verify"] != true {
		t.Errorf("skip-cert-verify: %v", p["skip-cert-verify"])
	}
}
