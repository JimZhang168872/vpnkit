package proto

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestVmessParse(t *testing.T) {
	payload := map[string]any{
		"v": "2", "ps": "HK-01", "add": "1.2.3.4", "port": "443",
		"id": "11111111-2222-3333-4444-555555555555", "aid": "0",
		"net": "ws", "tls": "tls", "path": "/path", "host": "example.com",
		"scy": "auto",
	}
	b, _ := json.Marshal(payload)
	uri := "vmess://" + base64.StdEncoding.EncodeToString(b)
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "vmess" {
		t.Errorf("type: %v", p["type"])
	}
	if p["name"] != "HK-01" {
		t.Errorf("name: %v", p["name"])
	}
	if p["server"] != "1.2.3.4" {
		t.Errorf("server: %v", p["server"])
	}
	if p["port"] != 443 {
		t.Errorf("port: %v", p["port"])
	}
	if p["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid: %v", p["uuid"])
	}
	if p["network"] != "ws" {
		t.Errorf("network: %v", p["network"])
	}
	if p["tls"] != true {
		t.Errorf("tls: %v", p["tls"])
	}
	wsOpts, ok := p["ws-opts"].(map[string]any)
	if !ok || wsOpts["path"] != "/path" {
		t.Errorf("ws-opts: %v", p["ws-opts"])
	}
}

func TestVmessUrlBase64URLEncoding(t *testing.T) {
	payload := `{"v":"2","ps":"X","add":"a.com","port":"1","id":"00000000-0000-0000-0000-000000000000","aid":"0","net":"tcp"}`
	uri := "vmess://" + base64.RawURLEncoding.EncodeToString([]byte(payload))
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["name"] != "X" {
		t.Errorf("name: %v", p["name"])
	}
}

func TestVmessInvalidBase64(t *testing.T) {
	if _, err := Parse("vmess://!@#$"); err == nil {
		t.Error("expected error")
	}
}

func TestVmessMissingField(t *testing.T) {
	b, _ := json.Marshal(map[string]any{"v": "2"})
	uri := "vmess://" + base64.StdEncoding.EncodeToString(b)
	if _, err := Parse(uri); err == nil {
		t.Error("expected error for missing fields")
	}
}

func TestVmessGrpcOpts(t *testing.T) {
	payload := map[string]any{
		"v": "2", "ps": "G", "add": "g.example", "port": "443",
		"id": "u", "aid": "0", "net": "grpc", "path": "svc1",
	}
	b, _ := json.Marshal(payload)
	uri := "vmess://" + base64.StdEncoding.EncodeToString(b)
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	grpc, ok := p["grpc-opts"].(map[string]any)
	if !ok || grpc["grpc-service-name"] != "svc1" {
		t.Errorf("grpc-opts: %v", p["grpc-opts"])
	}
}
