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

func TestToProxyMapEmitsDialerProxy(t *testing.T) {
	n := Node{
		Name:   "HK-A",
		Group:  "home",
		Via:    "doge:JP-1",
		Proto:  "hysteria2",
		Server: "1.2.3.4",
		Port:   443,
		Fields: map[string]any{"password": "x"},
	}
	m := ToProxyMap(n)
	if m["dialer-proxy"] != "doge:JP-1" {
		t.Errorf("dialer-proxy: got %v", m["dialer-proxy"])
	}
}

func TestToProxyMapOmitsDialerProxyWhenEmpty(t *testing.T) {
	n := Node{Name: "HK-A", Proto: "ss", Server: "1.2.3.4", Port: 8388}
	m := ToProxyMap(n)
	if _, ok := m["dialer-proxy"]; ok {
		t.Errorf("dialer-proxy should not be set when Via is empty: %v", m)
	}
}
