package proto

import "testing"

func TestHysteria2(t *testing.T) {
	p, err := Parse("hysteria2://password@h:443?obfs=salamander&obfs-password=salt&sni=h.example#H2")
	if err != nil {
		t.Fatal(err)
	}
	if p["type"] != "hysteria2" || p["password"] != "password" || p["port"] != 443 || p["obfs"] != "salamander" {
		t.Errorf("got %+v", p)
	}
}

func TestHysteriaV1(t *testing.T) {
	p, err := Parse("hysteria://h:443?protocol=udp&auth=secret&peer=peer.example#H1")
	if err != nil {
		t.Fatal(err)
	}
	if p["type"] != "hysteria" || p["auth_str"] != "secret" || p["protocol"] != "udp" {
		t.Errorf("got %+v", p)
	}
}
