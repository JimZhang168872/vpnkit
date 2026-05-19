package localnodes

import (
	"encoding/base64"
	"testing"
)

func TestParseURIShadowsocks(t *testing.T) {
	// Format: ss://BASE64(method:password)@host:port#name
	// Pre-computed: base64("aes-256-gcm:secret") == "YWVzLTI1Ni1nY206c2VjcmV0"
	uri := "ss://YWVzLTI1Ni1nY206c2VjcmV0@1.2.3.4:8388#HK-A"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "ss" {
		t.Errorf("proto: %q", n.Proto)
	}
	if n.Server != "1.2.3.4" || n.Port != 8388 {
		t.Errorf("server/port: %q/%d", n.Server, n.Port)
	}
	if n.Name != "HK-A" {
		t.Errorf("name: %q", n.Name)
	}
	if n.Fields["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher: %v", n.Fields["cipher"])
	}
	if n.Fields["password"] != "secret" {
		t.Errorf("password: %v", n.Fields["password"])
	}
}

func TestParseURIVmess(t *testing.T) {
	// vmess://BASE64({"v":"2","ps":"node-name","add":"1.2.3.4","port":"8443","id":"uuid-here","aid":"0","net":"ws","type":"none","host":"","path":"/path","tls":"tls"})
	payload := `{"v":"2","ps":"JP-Tokyo","add":"1.2.3.4","port":"8443","id":"11111111-2222-3333-4444-555555555555","aid":"0","net":"ws","type":"none","host":"example.com","path":"/path","tls":"tls"}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "vmess" || n.Server != "1.2.3.4" || n.Port != 8443 {
		t.Errorf("basic fields: %+v", n)
	}
	if n.Name != "JP-Tokyo" {
		t.Errorf("name from ps: %q", n.Name)
	}
	if n.Fields["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid: %v", n.Fields["uuid"])
	}
	if n.Fields["network"] != "ws" {
		t.Errorf("network: %v", n.Fields["network"])
	}
	if ws, _ := n.Fields["ws-opts"].(map[string]any); ws["path"] != "/path" {
		t.Errorf("ws-opts.path: %v", ws)
	}
}

func TestParseURITrojan(t *testing.T) {
	uri := "trojan://password123@1.2.3.4:8443?sni=example.com&alpn=h2,http/1.1#TR-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "trojan" || n.Server != "1.2.3.4" || n.Port != 8443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Name != "TR-1" {
		t.Errorf("name: %q", n.Name)
	}
	if n.Fields["password"] != "password123" {
		t.Errorf("password: %v", n.Fields["password"])
	}
	if n.Fields["sni"] != "example.com" {
		t.Errorf("sni: %v", n.Fields["sni"])
	}
	if alpn, _ := n.Fields["alpn"].([]string); len(alpn) != 2 || alpn[0] != "h2" {
		t.Errorf("alpn: %v", n.Fields["alpn"])
	}
}

func TestParseURIVless(t *testing.T) {
	// vless://UUID@host:port?encryption=none&security=reality&pbk=KEY&sni=...&type=tcp#name
	uri := "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?encryption=none&security=reality&pbk=publicKeyBase64&sni=example.com&type=tcp&flow=xtls-rprx-vision#VL-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "vless" || n.Server != "1.2.3.4" || n.Port != 443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Fields["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid: %v", n.Fields["uuid"])
	}
	if n.Fields["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow: %v", n.Fields["flow"])
	}
	if n.Fields["tls"] != true {
		t.Errorf("tls: %v", n.Fields["tls"])
	}
	if r, _ := n.Fields["reality-opts"].(map[string]any); r["public-key"] != "publicKeyBase64" {
		t.Errorf("reality public-key: %v", n.Fields["reality-opts"])
	}
}

func TestParseURIHysteria2(t *testing.T) {
	uri := "hysteria2://password@1.2.3.4:443?sni=example.com&insecure=1&up=100&down=200&obfs=salamander&obfs-password=ofuscatekey#HY2-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "hysteria2" || n.Server != "1.2.3.4" || n.Port != 443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Fields["password"] != "password" {
		t.Errorf("password: %v", n.Fields["password"])
	}
	if n.Fields["up"] != "100 Mbps" || n.Fields["down"] != "200 Mbps" {
		t.Errorf("up/down: %v/%v", n.Fields["up"], n.Fields["down"])
	}
	if n.Fields["obfs"] != "salamander" || n.Fields["obfs-password"] != "ofuscatekey" {
		t.Errorf("obfs: %v / %v", n.Fields["obfs"], n.Fields["obfs-password"])
	}
	if n.Fields["skip-cert-verify"] != true {
		t.Errorf("skip-cert-verify: %v", n.Fields["skip-cert-verify"])
	}
}

// TestParseURIHysteria2_SlashInPassword regresses a real-world hysteria2
// URI where the password contains a literal '/'. Per RFC 3986 the '/' should
// be percent-encoded as %2F in userinfo, but many proxy providers emit the
// lenient form and other clients (Shadowrocket, Clash, NekoBox) accept it.
// Without preprocessing, Go's net/url.Parse would treat the '/' as the start
// of the path, mangling host/userinfo. Reported in:
//   hysteria2://CBAI0bv97b21KRjXw3fDArlnW/ymWTur@jim.gulujili.xyz:8443?...
func TestParseURIHysteria2_SlashInPassword(t *testing.T) {
	uri := "hysteria2://CBAI0bv97b21KRjXw3fDArlnW/ymWTur@jim.gulujili.xyz:8443?security=tls&fp=chrome&alpn=h3&sni=jim.gulujili.xyz#Hy2-entrance-CN-jim-hy2"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "hysteria2" {
		t.Errorf("proto: %q", n.Proto)
	}
	if n.Server != "jim.gulujili.xyz" {
		t.Errorf("server: %q want jim.gulujili.xyz", n.Server)
	}
	if n.Port != 8443 {
		t.Errorf("port: %d want 8443", n.Port)
	}
	if n.Fields["password"] != "CBAI0bv97b21KRjXw3fDArlnW/ymWTur" {
		t.Errorf("password: %q (must preserve raw / not percent-encoded)", n.Fields["password"])
	}
	if n.Fields["sni"] != "jim.gulujili.xyz" {
		t.Errorf("sni: %v", n.Fields["sni"])
	}
	if n.Name != "Hy2-entrance-CN-jim-jim-hy2" && n.Name != "Hy2-entrance-CN-jim-hy2" {
		// fragment is the visual name; some normalizers may dedupe hy2 → hy2
		// but the literal fragment text should land as the name.
		t.Errorf("name: %q want Hy2-entrance-CN-jim-hy2", n.Name)
	}
}

// TestParseURITrojan_SlashInPassword covers the same lenient-userinfo
// behavior for trojan URIs (also seen in the wild from various providers).
func TestParseURITrojan_SlashInPassword(t *testing.T) {
	uri := "trojan://abc/def@example.com:443?sni=example.com#tj-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Server != "example.com" || n.Port != 443 {
		t.Errorf("server:port = %s:%d, want example.com:443", n.Server, n.Port)
	}
	if n.Fields["password"] != "abc/def" {
		t.Errorf("password = %q, want %q", n.Fields["password"], "abc/def")
	}
}

// TestEscapeUserinfoSlashes_NoOpWhenSafe asserts the helper never changes
// strings that don't need fixing (no userinfo, no slashes, etc.).
func TestEscapeUserinfoSlashes_NoOpWhenSafe(t *testing.T) {
	cases := []string{
		"ss://YWVzOnB3@host:8388#name",                                       // no slash in userinfo
		"https://example.com/path/with/slashes?q=1",                          // no userinfo, slashes in path are fine
		"vless://uuid@host:443/?type=ws&path=/abc",                           // userinfo has no slash; path has them
		"trojan://pw@host:443/?sni=x#trojan-1",                               // trailing-only path, no userinfo slash
		"hysteria2://password@host:443?sni=x#hy2-1",                          // plain password
	}
	for _, c := range cases {
		got := escapeUserinfoSlashes(c)
		if got != c {
			t.Errorf("escapeUserinfoSlashes(%q) = %q, want unchanged", c, got)
		}
	}
}

// TestEscapeUserinfoSlashes_EncodesOnlyInUserinfo asserts only the userinfo
// segment is touched — path / query / fragment slashes are preserved.
func TestEscapeUserinfoSlashes_EncodesOnlyInUserinfo(t *testing.T) {
	in := "hy2://ab/cd@host:443/some/path?p=/q#frag/with/slash"
	got := escapeUserinfoSlashes(in)
	want := "hy2://ab%2Fcd@host:443/some/path?p=/q#frag/with/slash"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// Also support the hy2:// alias.
func TestParseURIHy2Alias(t *testing.T) {
	uri := "hy2://password@1.2.3.4:443"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "hysteria2" {
		t.Errorf("proto should normalize to hysteria2, got %q", n.Proto)
	}
}

func TestParseURITuic(t *testing.T) {
	uri := "tuic://UUID:PASSWORD@1.2.3.4:443?sni=example.com&congestion_control=bbr&udp_relay_mode=native&alpn=h3#TUIC-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "tuic" || n.Server != "1.2.3.4" || n.Port != 443 {
		t.Errorf("basic: %+v", n)
	}
	if n.Fields["uuid"] != "UUID" || n.Fields["password"] != "PASSWORD" {
		t.Errorf("uuid/password: %v/%v", n.Fields["uuid"], n.Fields["password"])
	}
	if n.Fields["congestion-controller"] != "bbr" {
		t.Errorf("congestion-controller: %v", n.Fields["congestion-controller"])
	}
	if n.Fields["sni"] != "example.com" {
		t.Errorf("sni: %v", n.Fields["sni"])
	}
}

// Error path coverage tests.

func TestParseURIMissingScheme(t *testing.T) {
	if _, err := ParseURI("no-scheme-here"); err == nil {
		t.Error("expected error for missing scheme")
	}
}

func TestParseURIUnsupportedScheme(t *testing.T) {
	if _, err := ParseURI("ftp://example.com"); err == nil {
		t.Error("expected error for unsupported scheme")
	}
}

func TestParseSS_MissingUserInfo(t *testing.T) {
	if _, err := ParseURI("ss://1.2.3.4:8388"); err == nil {
		t.Error("expected error for ss:// without userinfo")
	}
}

func TestParseSS_BadBase64(t *testing.T) {
	if _, err := ParseURI("ss://not!valid!base64@1.2.3.4:8388"); err == nil {
		t.Error("expected error for bad base64 in ss userinfo")
	}
}

func TestParseVmess_NoTLSNoWS(t *testing.T) {
	// vmess with tcp transport and no tls — covers the else branches
	payload := `{"v":"2","ps":"plain","add":"5.5.5.5","port":"1234","id":"aaaabbbb-cccc-dddd-eeee-ffffaaaabbbb","aid":"0","net":"tcp","type":"none","host":"","path":"","tls":""}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "vmess" || n.Server != "5.5.5.5" || n.Port != 1234 {
		t.Errorf("basic: %+v", n)
	}
	if _, hasTLS := n.Fields["tls"]; hasTLS {
		t.Error("should not have tls field for non-tls vmess")
	}
}

func TestParseVmess_TLSWithSNI(t *testing.T) {
	// tls=tls with explicit sni — covers servername=sni path
	payload := `{"v":"2","ps":"tls-sni","add":"6.6.6.6","port":"8443","id":"11111111-2222-3333-4444-555555555555","aid":"0","net":"tcp","type":"none","host":"fallback.com","path":"","tls":"tls","sni":"explicit.com"}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Fields["servername"] != "explicit.com" {
		t.Errorf("servername should prefer sni over host: %v", n.Fields["servername"])
	}
}

func TestParseTrojan_SkipCertVerify(t *testing.T) {
	uri := "trojan://pass@1.2.3.4:443?allowInsecure=1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Fields["skip-cert-verify"] != true {
		t.Errorf("skip-cert-verify: %v", n.Fields["skip-cert-verify"])
	}
}

func TestParseVless_TLSSecurity(t *testing.T) {
	// security=tls (not reality) covers the tls case branch
	uri := "vless://uuid-here@1.2.3.4:443?security=tls&sni=tls.example.com&type=tcp"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Fields["tls"] != true {
		t.Errorf("tls: %v", n.Fields["tls"])
	}
	if n.Fields["servername"] != "tls.example.com" {
		t.Errorf("servername: %v", n.Fields["servername"])
	}
}

func TestParseVless_RealityWithSID(t *testing.T) {
	// reality with short-id
	uri := "vless://uuid-here@1.2.3.4:443?security=reality&pbk=KEY&sid=SHORTID&sni=sni.example.com&type=tcp"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ro, ok := n.Fields["reality-opts"].(map[string]any); !ok || ro["short-id"] != "SHORTID" {
		t.Errorf("reality short-id: %v", n.Fields["reality-opts"])
	}
}

func TestParseHy2_NoFragment(t *testing.T) {
	// URI without fragment — nameOrFallback uses host
	uri := "hysteria2://pw@example.com:443"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Name != "example.com:443" {
		t.Errorf("name (fallback): %q", n.Name)
	}
}

func TestParseHy2WithExplicitUnit(t *testing.T) {
	uri := "hysteria2://pw@1.2.3.4:443?up=100%20Mbps&down=200%20Gbps#HY2-unit"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Fields["up"] != "100 Mbps" {
		t.Errorf("up should preserve explicit unit, got %v", n.Fields["up"])
	}
	if n.Fields["down"] != "200 Gbps" {
		t.Errorf("down should preserve explicit unit, got %v", n.Fields["down"])
	}
}

func TestToProxyMapSS(t *testing.T) {
	n := Node{Name: "SS-1", Proto: "ss", Server: "1.1.1.1", Port: 8388, Fields: map[string]any{"cipher": "aes-256-gcm", "password": "pw"}}
	m := ToProxyMap(n)
	if m["type"] != "ss" || m["cipher"] != "aes-256-gcm" {
		t.Errorf("ToProxyMap ss: %v", m)
	}
}

func TestParseURISocks5(t *testing.T) {
	uri := "socks5://user:pa%2Fss@1.2.3.4:1080#sock-1"
	n, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "socks5" {
		t.Errorf("proto = %q, want socks5", n.Proto)
	}
	if n.Server != "1.2.3.4" || n.Port != 1080 {
		t.Errorf("host:port = %s:%d", n.Server, n.Port)
	}
	if n.Name != "sock-1" {
		t.Errorf("name = %q, want sock-1", n.Name)
	}
	if n.Fields["username"] != "user" {
		t.Errorf("username = %v, want user", n.Fields["username"])
	}
	if got := n.Fields["password"]; got != "pa/ss" {
		t.Errorf("password = %v, want pa/ss (slash should round-trip)", got)
	}
}

func TestParseURISocks5_NoAuth(t *testing.T) {
	n, err := ParseURI("socks5://1.2.3.4:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Proto != "socks5" {
		t.Errorf("proto = %q, want socks5", n.Proto)
	}
	if _, has := n.Fields["username"]; has {
		t.Errorf("no auth should not emit username; got %v", n.Fields)
	}
}
