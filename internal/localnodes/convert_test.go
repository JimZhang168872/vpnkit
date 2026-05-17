package localnodes

import (
	"testing"
)

func TestToProxyMapHysteria2(t *testing.T) {
	n := Node{
		Name: "HK-A", Proto: "hysteria2", Server: "1.2.3.4", Port: 443,
		Fields: map[string]any{"password": "x", "up": "100 Mbps", "down": "200 Mbps", "sni": "example.com"},
	}
	m := ToProxyMap(n)
	if m["name"] != "HK-A" || m["type"] != "hysteria2" || m["server"] != "1.2.3.4" || m["port"] != 443 {
		t.Errorf("basic: %v", m)
	}
	if m["password"] != "x" || m["up"] != "100 Mbps" || m["sni"] != "example.com" {
		t.Errorf("fields not flattened: %v", m)
	}
}
