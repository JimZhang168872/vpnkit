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

// TestHysteria2_PasswordContainsSlash covers real-world URLs where the
// password contains "/". RFC 3986 disallows "/" in authority, so a naive
// url.Parse misinterprets the authority section. Parser must tolerate it.
func TestHysteria2_PasswordContainsSlash(t *testing.T) {
	uri := "hysteria2://CBAI0bv97b21KRjXw3fDArlnW/ymWTur@jim.gulujili.xyz:8443?alpn=h3&sni=jim.gulujili.xyz#Hy2-entrance-CN-jim-hy2"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := p["type"]; got != "hysteria2" {
		t.Errorf("type: got %v want hysteria2", got)
	}
	if got := p["server"]; got != "jim.gulujili.xyz" {
		t.Errorf("server: got %v want jim.gulujili.xyz", got)
	}
	if got := p["port"]; got != 8443 {
		t.Errorf("port: got %v (%T) want 8443", got, got)
	}
	if got := p["password"]; got != "CBAI0bv97b21KRjXw3fDArlnW/ymWTur" {
		t.Errorf("password: got %v want CBAI0bv97b21KRjXw3fDArlnW/ymWTur", got)
	}
	if got := p["sni"]; got != "jim.gulujili.xyz" {
		t.Errorf("sni: got %v", got)
	}
	if got := p["name"]; got != "Hy2-entrance-CN-jim-hy2" {
		t.Errorf("name: got %v", got)
	}
}

// TestHy2Alias covers the hy2:// shorthand scheme.
func TestHy2Alias(t *testing.T) {
	p, err := Parse("hy2://pw@h:443?obfs=salamander&obfs-password=salt#N")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "hysteria2" || p["password"] != "pw" || p["port"] != 443 ||
		p["server"] != "h" || p["obfs"] != "salamander" || p["obfs-password"] != "salt" {
		t.Errorf("got %+v", p)
	}
}
