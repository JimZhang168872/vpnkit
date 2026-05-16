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

// TestTUIC_PasswordContainsSlash covers password values with "/".
// url.Parse misinterprets "/" in authority; parser must tolerate it.
func TestTUIC_PasswordContainsSlash(t *testing.T) {
	uri := "tuic://5b8da5ad-7c8f-4c5e-9e44-aa00aa00aa00:CBAI/ymWTur@jim.gulujili.xyz:443?congestion_control=bbr&sni=jim.gulujili.xyz#Tuic-slash"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "tuic" ||
		p["server"] != "jim.gulujili.xyz" ||
		p["port"] != 443 ||
		p["uuid"] != "5b8da5ad-7c8f-4c5e-9e44-aa00aa00aa00" ||
		p["password"] != "CBAI/ymWTur" ||
		p["congestion-controller"] != "bbr" ||
		p["name"] != "Tuic-slash" {
		t.Errorf("got %+v", p)
	}
}
