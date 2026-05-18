package sources

import (
	"strings"
	"testing"

	"vpnkit/internal/localnodes"
)

// TestNewLocalNodeFieldFormFromNode_PrefillsHy2 verifies that creating an
// edit form from an existing hysteria2 node fills the proto/common/proto-
// specific fields with the node's stored values. Up/Down are stored as
// "100 Mbps" strings and must be unwrapped to plain ints for the input.
func TestNewLocalNodeFieldFormFromNode_PrefillsHy2(t *testing.T) {
	node := localnodes.Node{
		Name:   "JP-hy2",
		Group:  "home",
		Via:    "doge-auto",
		Proto:  "hysteria2",
		Server: "jp.example.com",
		Port:   443,
		Fields: map[string]any{
			"password":         "pa$$w0rd",
			"sni":              "fake.example.com",
			"up":               "200 Mbps",
			"down":             "1000 Mbps",
			"obfs":             "salamander",
			"obfs-password":    "obfspw",
			"skip-cert-verify": true,
		},
	}
	f := newLocalNodeFieldFormFromNode(node)
	if f.editingName != "JP-hy2" {
		t.Errorf("editingName = %q, want JP-hy2", f.editingName)
	}
	if f.proto != "hysteria2" {
		t.Errorf("proto = %q, want hysteria2", f.proto)
	}
	got := collectValues(f)
	want := map[string]string{
		"proto":            "hysteria2",
		"name":             "JP-hy2",
		"group":            "home",
		"server":           "jp.example.com",
		"port":             "443",
		"password":         "pa$$w0rd",
		"sni":              "fake.example.com",
		"up":               "200",
		"down":             "1000",
		"obfs":             "salamander",
		"obfs-password":    "obfspw",
		"skip-cert-verify": "true",
		"via":              "doge-auto",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("input %q = %q, want %q", k, got[k], v)
		}
	}
}

// TestNewLocalNodeFieldFormFromNode_PrefillsVmessNestedWSOpts checks that
// nested fields (ws-opts.host, ws-opts.path) round-trip through the form.
func TestNewLocalNodeFieldFormFromNode_PrefillsVmessNestedWSOpts(t *testing.T) {
	node := localnodes.Node{
		Name: "vm-1", Group: "office", Proto: "vmess",
		Server: "vm.example.com", Port: 8080,
		Fields: map[string]any{
			"uuid":    "uuid-1234",
			"alterId": 0,
			"cipher":  "auto",
			"network": "ws",
			"ws-opts": map[string]any{"host": "cdn.example.com", "path": "/wsx"},
			"tls":     false,
		},
	}
	f := newLocalNodeFieldFormFromNode(node)
	got := collectValues(f)
	if got["ws-opts.host"] != "cdn.example.com" {
		t.Errorf("ws-opts.host = %q", got["ws-opts.host"])
	}
	if got["ws-opts.path"] != "/wsx" {
		t.Errorf("ws-opts.path = %q", got["ws-opts.path"])
	}
	if got["tls"] != "false" {
		t.Errorf("tls = %q, want false", got["tls"])
	}
	if got["alterId"] != "0" {
		t.Errorf("alterId = %q, want 0", got["alterId"])
	}
}

// TestNewLocalNodeFieldFormFromNode_PrefillsTrojanAlpnCSV verifies an alpn
// []any slice unwraps back to comma-separated form.
func TestNewLocalNodeFieldFormFromNode_PrefillsTrojanAlpnCSV(t *testing.T) {
	node := localnodes.Node{
		Name: "tj-1", Group: "home", Proto: "trojan",
		Server: "tj.example.com", Port: 443,
		Fields: map[string]any{
			"password":         "secret",
			"sni":              "tj.example.com",
			"alpn":             []any{"h2", "http/1.1"},
			"skip-cert-verify": false,
		},
	}
	f := newLocalNodeFieldFormFromNode(node)
	got := collectValues(f)
	if got["alpn"] != "h2,http/1.1" {
		t.Errorf("alpn = %q, want csv", got["alpn"])
	}
	if got["skip-cert-verify"] != "false" {
		t.Errorf("skip-cert-verify = %q, want false", got["skip-cert-verify"])
	}
}

// TestEditFormRoundTrip prefills from a node, commits, and verifies the
// resulting Node matches the original (same Fields, name, group, etc).
func TestEditFormRoundTrip(t *testing.T) {
	orig := localnodes.Node{
		Name: "ss-1", Group: "home", Proto: "ss",
		Server: "ss.example.com", Port: 8388,
		Fields: map[string]any{
			"cipher":   "aes-256-gcm",
			"password": "hunter2",
		},
	}
	f := newLocalNodeFieldFormFromNode(orig)
	got, err := f.commitFieldForm()
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got.Name != orig.Name || got.Group != orig.Group ||
		got.Proto != orig.Proto || got.Server != orig.Server || got.Port != orig.Port {
		t.Errorf("scalar fields drifted: got %+v want %+v", got, orig)
	}
	if got.Fields["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher drifted: %v", got.Fields["cipher"])
	}
	if got.Fields["password"] != "hunter2" {
		t.Errorf("password drifted: %v", got.Fields["password"])
	}
}

// collectValues maps each input's logical key → its current value.
func collectValues(f *localNodeForm) map[string]string {
	defs := f.formFieldDefs()
	out := make(map[string]string, len(defs))
	for i, d := range defs {
		out[d.key] = strings.TrimSpace(f.inputs[i].Value())
	}
	return out
}
