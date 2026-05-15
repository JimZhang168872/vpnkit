package proto

import "testing"

func TestTUICv5(t *testing.T) {
	p, err := Parse("tuic://uuid-x:password-y@h:443?alpn=h3&congestion_control=bbr&disable_sni=0&sni=h.example#T")
	if err != nil {
		t.Fatal(err)
	}
	if p["type"] != "tuic" || p["uuid"] != "uuid-x" || p["password"] != "password-y" ||
		p["port"] != 443 || p["congestion-controller"] != "bbr" {
		t.Errorf("got %+v", p)
	}
}
