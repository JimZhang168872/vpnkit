# vpnkit Phase 2 Implementation Plan — Profiles + Proxies

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make vpnkit daily-usable. Add full subscription management (add/list/remove/use/update), multi-format protocol parsing (9 protocols + Base64 + Clash YAML passthrough), group synthesis, patch overlay merging, the Profiles tab UI, and the Proxies tab UI (list/switch/delay-test).

**Architecture:** Three new `internal/` packages — `subscription` (fetch + format detection + per-protocol parsers + assemble), `patch` (overlay merge), and tab implementations for `tabs/profiles` and `tabs/proxies`. Reuses Phase 1's `config`, `rules`, `store`, `api`, and `app` packages. mihomo gets hot-reloaded via `PUT /configs` after each subscription update.

**Tech Stack:** Same as Phase 1 (Go 1.22, bubbletea v0.25, lipgloss, yaml.v3). No new third-party deps.

**Spec reference:** [`docs/superpowers/specs/2026-05-15-vpnkit-tui-design.md`](../specs/2026-05-15-vpnkit-tui-design.md) §4.5 (Profiles/Proxies tabs), §5 (subscription conversion), §6 (rule sets), §8 (REST API).

**Order of work:** Profiles first (T1–T15), then Proxies (T16–T23), then smoke + tag (T24).

---

## File Map

| Path | Responsibility |
|---|---|
| `internal/subscription/fetch.go` | HTTP fetch with custom UA, redirect handling, 30s timeout |
| `internal/subscription/detect.go` | Detect format (Clash YAML / SIP008 JSON / Base64 / single-URI) |
| `internal/subscription/proto/vmess.go` | `vmess://` parser → `mihomo.Proxy` |
| `internal/subscription/proto/ss.go` | `ss://` SIP002 parser (with legacy base64 variant) |
| `internal/subscription/proto/ssr.go` | `ssr://` ShadowsocksR parser |
| `internal/subscription/proto/trojan.go` | `trojan://` parser |
| `internal/subscription/proto/vless.go` | `vless://` parser including REALITY fields |
| `internal/subscription/proto/hysteria.go` | `hysteria://` v1 + `hysteria2://` parsers |
| `internal/subscription/proto/tuic.go` | `tuic://` v5 parser |
| `internal/subscription/proto/sip008.go` | SIP008 JSON list parser |
| `internal/subscription/proto/proxy.go` | Shared `Proxy` map[string]any type, `Parse(uri)` dispatcher |
| `internal/subscription/convert.go` | Top-level: bytes → `[]Proxy` (handles clash, base64, list, single-URI) |
| `internal/subscription/groups.go` | Synthesize default `proxy-groups` when subscription has none |
| `internal/subscription/assemble.go` | Assemble final config (base skeleton + subscription proxies + rules + patch) |
| `internal/patch/patch.go` | Load `~/.config/mihomo/patch.yaml`, deep-merge over a target map |
| `internal/profiles/manager.go` | Profile lifecycle: add/remove/update/activate (uses store + subscription + patch + api) |
| `internal/api/proxies.go` | Extend `Client` with `GetProxies`, `PutProxy`, `Delay`, `GroupDelay` |
| `internal/tabs/profiles/profiles.go` | Profiles tab Model with table + add/edit popup |
| `internal/tabs/profiles/form.go` | Add/edit popup form (textinput-based) |
| `internal/tabs/proxies/proxies.go` | Proxies tab Model with tree view |
| `internal/tabs/proxies/delay.go` | Delay-test orchestration (group + table render) |
| `internal/app/model.go` (MODIFY) | Wire new tabs replacing two `stub.Model` entries |
| `internal/app/update.go` (MODIFY) | Route messages to new tabs |
| `internal/app/run.go` (MODIFY) | Poll `/proxies` every 5s; expose `*profiles.Manager` to tabs |
| `internal/msg/msg.go` (MODIFY) | Add `ProxiesSnapshot`, `ProfileUpdated`, `ProfileError` messages |

---

## Task 1: Subscription Proxy type + URI dispatcher

**Files:**
- Create: `internal/subscription/proto/proxy.go`
- Create: `internal/subscription/proto/proxy_test.go`

- [ ] **Step 1: Write failing test**

`internal/subscription/proto/proxy_test.go`:
```go
package proto

import "testing"

func TestParseDispatchesScheme(t *testing.T) {
	tests := []struct {
		uri    string
		scheme string
	}{
		{"vmess://abc", "vmess"},
		{"ss://abc", "ss"},
		{"ssr://abc", "ssr"},
		{"trojan://abc", "trojan"},
		{"vless://abc", "vless"},
		{"hysteria://abc", "hysteria"},
		{"hysteria2://abc", "hysteria2"},
		{"tuic://abc", "tuic"},
	}
	for _, tt := range tests {
		got, _, _ := schemeOf(tt.uri)
		if got != tt.scheme {
			t.Errorf("schemeOf(%s) = %s, want %s", tt.uri, got, tt.scheme)
		}
	}
}

func TestParseUnknownScheme(t *testing.T) {
	_, err := Parse("ftp://example.com")
	if err == nil {
		t.Error("expected error for unknown scheme")
	}
}
```

- [ ] **Step 2: Write impl**

`internal/subscription/proto/proxy.go`:
```go
// Package proto holds per-protocol parsers turning subscription URIs into
// mihomo.Proxy values (a generic map[string]any matching mihomo's YAML schema).
package proto

import (
	"fmt"
	"strings"
)

// Proxy is a mihomo proxy entry, deliberately untyped to mirror the YAML schema.
// Required keys: name, type, server, port. Other keys depend on protocol.
type Proxy map[string]any

// Parser converts a single proxy URI (e.g. "vmess://...") to a Proxy.
type Parser func(uri string) (Proxy, error)

// registered parsers, populated by per-protocol init() calls.
var registry = map[string]Parser{}

// Register adds a parser for a scheme (call from proto package init).
func Register(scheme string, p Parser) { registry[scheme] = p }

// Parse dispatches the URI to the matching scheme parser.
func Parse(uri string) (Proxy, error) {
	scheme, _, err := schemeOf(uri)
	if err != nil {
		return nil, err
	}
	parser, ok := registry[scheme]
	if !ok {
		return nil, fmt.Errorf("proto: unsupported scheme %q", scheme)
	}
	return parser(uri)
}

func schemeOf(uri string) (string, string, error) {
	i := strings.Index(uri, "://")
	if i <= 0 {
		return "", "", fmt.Errorf("proto: not a URI: %q", uri)
	}
	return uri[:i], uri[i+3:], nil
}
```

- [ ] **Step 3: Run test, commit**

```bash
export PATH="$HOME/.local/go/bin:$PATH"
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/
git commit -m "feat(subscription): proxy type and URI dispatcher skeleton"
```

---

## Task 2: vmess parser

**Files:**
- Create: `internal/subscription/proto/vmess.go`
- Create: `internal/subscription/proto/vmess_test.go`

`vmess://` is base64-encoded JSON of the V2RayN spec.

- [ ] **Step 1: Test**

`internal/subscription/proto/vmess_test.go`:
```go
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
	// Try URL-safe base64 (RFC 4648 §5) as alternate encoding.
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
```

- [ ] **Step 2: Implementation**

`internal/subscription/proto/vmess.go`:
```go
package proto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
)

func init() { Register("vmess", parseVmess) }

func parseVmess(uri string) (Proxy, error) {
	_, body, err := schemeOf(uri)
	if err != nil {
		return nil, err
	}
	decoded, err := tolerantBase64(body)
	if err != nil {
		return nil, fmt.Errorf("vmess: %w", err)
	}
	var raw struct {
		PS   string `json:"ps"`
		Add  string `json:"add"`
		Port any    `json:"port"`
		ID   string `json:"id"`
		Aid  any    `json:"aid"`
		Scy  string `json:"scy"`
		Net  string `json:"net"`
		Type string `json:"type"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
	}
	if err := json.Unmarshal(decoded, &raw); err != nil {
		return nil, fmt.Errorf("vmess: bad json: %w", err)
	}
	if raw.Add == "" || raw.Port == nil || raw.ID == "" {
		return nil, fmt.Errorf("vmess: missing required fields")
	}
	port, err := asInt(raw.Port)
	if err != nil {
		return nil, fmt.Errorf("vmess: port: %w", err)
	}
	aid, _ := asInt(raw.Aid)
	cipher := raw.Scy
	if cipher == "" {
		cipher = "auto"
	}
	p := Proxy{
		"name":   raw.PS,
		"type":   "vmess",
		"server": raw.Add,
		"port":   port,
		"uuid":   raw.ID,
		"alterId": aid,
		"cipher":  cipher,
	}
	if raw.Net != "" {
		p["network"] = raw.Net
	}
	if raw.TLS == "tls" {
		p["tls"] = true
		if raw.SNI != "" {
			p["servername"] = raw.SNI
		}
	}
	switch raw.Net {
	case "ws":
		opts := map[string]any{}
		if raw.Path != "" {
			opts["path"] = raw.Path
		}
		if raw.Host != "" {
			opts["headers"] = map[string]any{"Host": raw.Host}
		}
		p["ws-opts"] = opts
	case "grpc":
		p["grpc-opts"] = map[string]any{"grpc-service-name": raw.Path}
	case "h2":
		opts := map[string]any{}
		if raw.Path != "" {
			opts["path"] = raw.Path
		}
		if raw.Host != "" {
			opts["host"] = []string{raw.Host}
		}
		p["h2-opts"] = opts
	}
	return p, nil
}

// asInt converts a JSON value (number or string) to int.
func asInt(v any) (int, error) {
	switch x := v.(type) {
	case float64:
		return int(x), nil
	case string:
		return strconv.Atoi(x)
	case int:
		return x, nil
	}
	return 0, fmt.Errorf("not a number: %v", v)
}

// tolerantBase64 tries standard, URL, and raw variants.
func tolerantBase64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.URLEncoding,
		base64.RawStdEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("base64: not decodable")
}
```

- [ ] **Step 3: Test + commit**

```bash
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/vmess.go internal/subscription/proto/vmess_test.go
git commit -m "feat(subscription): vmess:// parser"
```

---

## Task 3: ss (Shadowsocks) parser

**Files:**
- Create: `internal/subscription/proto/ss.go`
- Create: `internal/subscription/proto/ss_test.go`

`ss://` supports SIP002 form `ss://base64(method:password)@host:port#name` and legacy `ss://base64(method:password@host:port)#name`. Plugin support via query string.

- [ ] **Test**

`internal/subscription/proto/ss_test.go`:
```go
package proto

import (
	"encoding/base64"
	"testing"
)

func TestSSStandardSIP002(t *testing.T) {
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:password"))
	uri := "ss://" + userinfo + "@1.2.3.4:8388#HK-Server"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "ss" || p["server"] != "1.2.3.4" || p["port"] != 8388 ||
		p["cipher"] != "aes-128-gcm" || p["password"] != "password" || p["name"] != "HK-Server" {
		t.Errorf("got %+v", p)
	}
}

func TestSSLegacyForm(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pwd@example.com:8388"))
	uri := "ss://" + encoded + "#Legacy"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["server"] != "example.com" || p["port"] != 8388 || p["cipher"] != "aes-256-gcm" {
		t.Errorf("got %+v", p)
	}
}

func TestSSPluginV2rayPlugin(t *testing.T) {
	userinfo := base64.StdEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:secret"))
	uri := "ss://" + userinfo + "@a.b:443?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Btls%3Bhost%3Dexample.com#X"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["plugin"] != "v2ray-plugin" {
		t.Errorf("plugin: %v", p["plugin"])
	}
	po, ok := p["plugin-opts"].(map[string]any)
	if !ok {
		t.Fatalf("plugin-opts missing")
	}
	if po["mode"] != "websocket" || po["host"] != "example.com" || po["tls"] != true {
		t.Errorf("plugin-opts: %+v", po)
	}
}

func TestSSURLEncodedPassword(t *testing.T) {
	// URL-encoded password (no base64 wrap) per Shadowsocks 2022 spec
	uri := "ss://aes-128-gcm:p%40ss@h.example:8388#x"
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["password"] != "p@ss" {
		t.Errorf("password: %v", p["password"])
	}
}

func TestSSInvalid(t *testing.T) {
	if _, err := Parse("ss://"); err == nil {
		t.Error("expected error for empty body")
	}
}
```

- [ ] **Implementation**

`internal/subscription/proto/ss.go`:
```go
package proto

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func init() { Register("ss", parseSS) }

func parseSS(uri string) (Proxy, error) {
	_, rest, err := schemeOf(uri)
	if err != nil {
		return nil, err
	}
	if rest == "" {
		return nil, fmt.Errorf("ss: empty body")
	}
	// Split off fragment (name) and query (plugin etc).
	frag := ""
	if i := strings.Index(rest, "#"); i >= 0 {
		frag, _ = url.QueryUnescape(rest[i+1:])
		rest = rest[:i]
	}
	query := ""
	if i := strings.Index(rest, "?"); i >= 0 {
		query = rest[i+1:]
		rest = rest[:i]
	}

	var method, password, host string
	var port int

	if at := strings.LastIndex(rest, "@"); at >= 0 {
		// SIP002: userinfo@host:port. userinfo may be base64 or plain method:password.
		userinfo := rest[:at]
		hostPort := rest[at+1:]
		if decoded, err := tolerantBase64(userinfo); err == nil && strings.Contains(string(decoded), ":") {
			parts := strings.SplitN(string(decoded), ":", 2)
			method, password = parts[0], parts[1]
		} else {
			parts := strings.SplitN(userinfo, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("ss: malformed userinfo")
			}
			method = parts[0]
			password, _ = url.QueryUnescape(parts[1])
		}
		host, port, err = splitHostPort(hostPort)
		if err != nil {
			return nil, err
		}
	} else {
		// Legacy: base64(method:password@host:port)
		decoded, err := tolerantBase64(rest)
		if err != nil {
			return nil, fmt.Errorf("ss legacy: %w", err)
		}
		s := string(decoded)
		at := strings.LastIndex(s, "@")
		if at < 0 {
			return nil, fmt.Errorf("ss legacy: missing @")
		}
		credParts := strings.SplitN(s[:at], ":", 2)
		if len(credParts) != 2 {
			return nil, fmt.Errorf("ss legacy: bad creds")
		}
		method = credParts[0]
		password = credParts[1]
		host, port, err = splitHostPort(s[at+1:])
		if err != nil {
			return nil, err
		}
	}

	p := Proxy{
		"name":     frag,
		"type":     "ss",
		"server":   host,
		"port":     port,
		"cipher":   method,
		"password": password,
	}

	if query != "" {
		q, _ := url.ParseQuery(query)
		if pluginRaw := q.Get("plugin"); pluginRaw != "" {
			pluginRaw, _ = url.QueryUnescape(pluginRaw)
			plugin, opts := parseSSPlugin(pluginRaw)
			p["plugin"] = plugin
			p["plugin-opts"] = opts
		}
	}
	return p, nil
}

// parseSSPlugin splits "plugin;k=v;k2=v2;flag" into name + map.
func parseSSPlugin(s string) (string, map[string]any) {
	parts := strings.Split(s, ";")
	name := parts[0]
	opts := map[string]any{}
	for _, kv := range parts[1:] {
		if eq := strings.Index(kv, "="); eq >= 0 {
			opts[kv[:eq]] = kv[eq+1:]
		} else if kv != "" {
			opts[kv] = true
		}
	}
	return name, opts
}

func splitHostPort(s string) (string, int, error) {
	c := strings.LastIndex(s, ":")
	if c < 0 {
		return "", 0, fmt.Errorf("missing port in %q", s)
	}
	port, err := strconv.Atoi(s[c+1:])
	if err != nil {
		return "", 0, fmt.Errorf("port: %w", err)
	}
	return s[:c], port, nil
}

var _ = base64.StdEncoding // tolerantBase64 is in vmess.go but lives in this package
```

- [ ] **Commit**

```bash
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/ss.go internal/subscription/proto/ss_test.go
git commit -m "feat(subscription): ss:// (Shadowsocks SIP002 + legacy) parser"
```

---

## Task 4: ssr (ShadowsocksR) parser

`ssr://base64(host:port:protocol:method:obfs:base64pass/?obfsparam=base64&protoparam=base64&remarks=base64&group=base64)`

- [ ] **Test** in `internal/subscription/proto/ssr_test.go`:
```go
package proto

import (
	"encoding/base64"
	"net/url"
	"testing"
)

func TestSSRParse(t *testing.T) {
	pwd := base64.RawURLEncoding.EncodeToString([]byte("password"))
	remarks := base64.RawURLEncoding.EncodeToString([]byte("HK-SSR"))
	obfsp := base64.RawURLEncoding.EncodeToString([]byte("obfs.example"))
	body := "1.2.3.4:8388:auth_aes128_md5:aes-256-cfb:tls1.2_ticket_auth:" + pwd +
		"/?obfsparam=" + obfsp + "&remarks=" + remarks
	uri := "ssr://" + base64.RawURLEncoding.EncodeToString([]byte(body))
	p, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p["type"] != "ssr" || p["server"] != "1.2.3.4" || p["port"] != 8388 ||
		p["cipher"] != "aes-256-cfb" || p["password"] != "password" ||
		p["protocol"] != "auth_aes128_md5" || p["obfs"] != "tls1.2_ticket_auth" {
		t.Errorf("got %+v", p)
	}
	if p["obfs-param"] != "obfs.example" {
		t.Errorf("obfs-param: %v", p["obfs-param"])
	}
	if p["name"] != "HK-SSR" {
		t.Errorf("name: %v", p["name"])
	}
	_ = url.QueryEscape // silence import
}
```

- [ ] **Implementation** `internal/subscription/proto/ssr.go`:
```go
package proto

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func init() { Register("ssr", parseSSR) }

func parseSSR(uri string) (Proxy, error) {
	_, rest, err := schemeOf(uri)
	if err != nil {
		return nil, err
	}
	decoded, err := tolerantBase64(rest)
	if err != nil {
		return nil, fmt.Errorf("ssr: %w", err)
	}
	s := string(decoded)
	// Split path from query.
	idx := strings.Index(s, "/?")
	main := s
	query := ""
	if idx >= 0 {
		main = s[:idx]
		query = s[idx+2:]
	}
	parts := strings.Split(main, ":")
	if len(parts) != 6 {
		return nil, fmt.Errorf("ssr: expected 6 colon parts, got %d", len(parts))
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("ssr: port: %w", err)
	}
	password, err := tolerantBase64(parts[5])
	if err != nil {
		return nil, fmt.Errorf("ssr: password: %w", err)
	}
	p := Proxy{
		"type":     "ssr",
		"server":   parts[0],
		"port":     port,
		"protocol": parts[2],
		"cipher":   parts[3],
		"obfs":     parts[4],
		"password": string(password),
	}
	if query != "" {
		q, _ := url.ParseQuery(query)
		decode := func(k string) string {
			if v := q.Get(k); v != "" {
				if b, err := tolerantBase64(v); err == nil {
					return string(b)
				}
			}
			return ""
		}
		if v := decode("obfsparam"); v != "" {
			p["obfs-param"] = v
		}
		if v := decode("protoparam"); v != "" {
			p["protocol-param"] = v
		}
		if v := decode("remarks"); v != "" {
			p["name"] = v
		}
	}
	_ = base64.StdEncoding
	return p, nil
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/ssr.go internal/subscription/proto/ssr_test.go
git commit -m "feat(subscription): ssr:// parser"
```

---

## Task 5: trojan parser

`trojan://password@host:port?security=tls&sni=&allowInsecure=&type=ws&path=&host=#name`

- [ ] **Test** `internal/subscription/proto/trojan_test.go`:
```go
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
```

- [ ] **Implementation** `internal/subscription/proto/trojan.go`:
```go
package proto

import (
	"fmt"
	"net/url"
	"strconv"
)

func init() { Register("trojan", parseTrojan) }

func parseTrojan(uri string) (Proxy, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("trojan: %w", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("trojan: port: %w", err)
	}
	pw, _ := url.PathUnescape(u.User.Username())
	q := u.Query()
	p := Proxy{
		"name":     u.Fragment,
		"type":     "trojan",
		"server":   u.Hostname(),
		"port":     port,
		"password": pw,
	}
	if sni := q.Get("sni"); sni != "" {
		p["sni"] = sni
	} else if peer := q.Get("peer"); peer != "" {
		p["sni"] = peer
	}
	if ai := q.Get("allowInsecure"); ai == "1" || ai == "true" {
		p["skip-cert-verify"] = true
	}
	switch q.Get("type") {
	case "ws":
		p["network"] = "ws"
		opts := map[string]any{}
		if path := q.Get("path"); path != "" {
			opts["path"] = path
		}
		if host := q.Get("host"); host != "" {
			opts["headers"] = map[string]any{"Host": host}
		}
		p["ws-opts"] = opts
	case "grpc":
		p["network"] = "grpc"
		p["grpc-opts"] = map[string]any{"grpc-service-name": q.Get("serviceName")}
	}
	return p, nil
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/trojan.go internal/subscription/proto/trojan_test.go
git commit -m "feat(subscription): trojan:// parser"
```

---

## Task 6: vless parser (XTLS / REALITY)

- [ ] **Test** `internal/subscription/proto/vless_test.go`:
```go
package proto

import "testing"

func TestVlessBasic(t *testing.T) {
	p, err := Parse("vless://uuid-here@server:443?security=tls&sni=example.com&type=tcp#V1")
	if err != nil {
		t.Fatal(err)
	}
	if p["type"] != "vless" || p["uuid"] != "uuid-here" || p["server"] != "server" || p["port"] != 443 ||
		p["tls"] != true || p["servername"] != "example.com" || p["name"] != "V1" {
		t.Errorf("got %+v", p)
	}
}

func TestVlessReality(t *testing.T) {
	p, err := Parse("vless://u@h:443?security=reality&pbk=PUBKEY&sid=SHORTID&fp=chrome&sni=www.example.com&flow=xtls-rprx-vision#R")
	if err != nil {
		t.Fatal(err)
	}
	if p["client-fingerprint"] != "chrome" || p["flow"] != "xtls-rprx-vision" {
		t.Errorf("got %+v", p)
	}
	rr, _ := p["reality-opts"].(map[string]any)
	if rr["public-key"] != "PUBKEY" || rr["short-id"] != "SHORTID" {
		t.Errorf("reality-opts: %v", rr)
	}
}

func TestVlessWS(t *testing.T) {
	p, err := Parse("vless://u@h:443?type=ws&path=%2Fpath&host=h.example&security=tls#W")
	if err != nil {
		t.Fatal(err)
	}
	wsOpts, _ := p["ws-opts"].(map[string]any)
	if wsOpts["path"] != "/path" {
		t.Errorf("ws-opts: %v", wsOpts)
	}
}
```

- [ ] **Implementation** `internal/subscription/proto/vless.go`:
```go
package proto

import (
	"fmt"
	"net/url"
	"strconv"
)

func init() { Register("vless", parseVless) }

func parseVless(uri string) (Proxy, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("vless: %w", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("vless: port: %w", err)
	}
	q := u.Query()
	p := Proxy{
		"name":   u.Fragment,
		"type":   "vless",
		"server": u.Hostname(),
		"port":   port,
		"uuid":   u.User.Username(),
	}
	switch q.Get("security") {
	case "tls":
		p["tls"] = true
	case "reality":
		p["tls"] = true
		p["reality-opts"] = map[string]any{
			"public-key": q.Get("pbk"),
			"short-id":   q.Get("sid"),
		}
	}
	if sni := q.Get("sni"); sni != "" {
		p["servername"] = sni
	}
	if fp := q.Get("fp"); fp != "" {
		p["client-fingerprint"] = fp
	}
	if flow := q.Get("flow"); flow != "" {
		p["flow"] = flow
	}
	if net := q.Get("type"); net != "" && net != "tcp" {
		p["network"] = net
	}
	switch q.Get("type") {
	case "ws":
		opts := map[string]any{}
		if path := q.Get("path"); path != "" {
			opts["path"] = path
		}
		if host := q.Get("host"); host != "" {
			opts["headers"] = map[string]any{"Host": host}
		}
		p["ws-opts"] = opts
	case "grpc":
		p["grpc-opts"] = map[string]any{"grpc-service-name": q.Get("serviceName")}
	}
	return p, nil
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/vless.go internal/subscription/proto/vless_test.go
git commit -m "feat(subscription): vless:// parser (TLS + REALITY)"
```

---

## Task 7: hysteria + hysteria2 parsers

Two parsers in one file (similar structure).

- [ ] **Test** `internal/subscription/proto/hysteria_test.go`:
```go
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
```

- [ ] **Implementation** `internal/subscription/proto/hysteria.go`:
```go
package proto

import (
	"fmt"
	"net/url"
	"strconv"
)

func init() {
	Register("hysteria", parseHysteriaV1)
	Register("hysteria2", parseHysteria2)
	Register("hy2", parseHysteria2)
}

func parseHysteria2(uri string) (Proxy, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, err
	}
	q := u.Query()
	p := Proxy{
		"name":     u.Fragment,
		"type":     "hysteria2",
		"server":   u.Hostname(),
		"port":     port,
		"password": u.User.Username(),
	}
	if v := q.Get("obfs"); v != "" {
		p["obfs"] = v
		if pwd := q.Get("obfs-password"); pwd != "" {
			p["obfs-password"] = pwd
		}
	}
	if sni := q.Get("sni"); sni != "" {
		p["sni"] = sni
	}
	if q.Get("insecure") == "1" {
		p["skip-cert-verify"] = true
	}
	return p, nil
}

func parseHysteriaV1(uri string) (Proxy, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, err
	}
	q := u.Query()
	p := Proxy{
		"name":     u.Fragment,
		"type":     "hysteria",
		"server":   u.Hostname(),
		"port":     port,
		"auth_str": q.Get("auth"),
	}
	if v := q.Get("protocol"); v != "" {
		p["protocol"] = v
	}
	if v := q.Get("peer"); v != "" {
		p["sni"] = v
	}
	if v := q.Get("up"); v != "" {
		p["up"] = v
	}
	if v := q.Get("down"); v != "" {
		p["down"] = v
	}
	if q.Get("insecure") == "1" {
		p["skip-cert-verify"] = true
	}
	return p, nil
}

var _ = fmt.Errorf
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/hysteria.go internal/subscription/proto/hysteria_test.go
git commit -m "feat(subscription): hysteria + hysteria2 parsers"
```

---

## Task 8: tuic parser

- [ ] **Test** `internal/subscription/proto/tuic_test.go`:
```go
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
```

- [ ] **Implementation** `internal/subscription/proto/tuic.go`:
```go
package proto

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func init() { Register("tuic", parseTUIC) }

func parseTUIC(uri string) (Proxy, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("tuic: %w", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, err
	}
	uuid := u.User.Username()
	pw, _ := u.User.Password()
	q := u.Query()
	p := Proxy{
		"name":     u.Fragment,
		"type":     "tuic",
		"server":   u.Hostname(),
		"port":     port,
		"uuid":     uuid,
		"password": pw,
	}
	if v := q.Get("congestion_control"); v != "" {
		p["congestion-controller"] = v
	}
	if v := q.Get("sni"); v != "" {
		p["sni"] = v
	}
	if v := q.Get("alpn"); v != "" {
		p["alpn"] = strings.Split(v, ",")
	}
	if q.Get("disable_sni") == "1" {
		p["disable-sni"] = true
	}
	return p, nil
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/proto/ -v
git add internal/subscription/proto/tuic.go internal/subscription/proto/tuic_test.go
git commit -m "feat(subscription): tuic:// v5 parser"
```

---

## Task 9: SIP008 JSON parser (multi-proxy)

SIP008 is a JSON list format from Shadowsocks ecosystem. Different from single-URI parsers: it returns multiple proxies.

- [ ] **Test** `internal/subscription/sip008_test.go`:
```go
package subscription

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSIP008Parse(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"version": 1,
		"servers": []map[string]any{
			{"id": "1", "remarks": "HK", "server": "h1", "server_port": 8388, "method": "aes-128-gcm", "password": "pw1"},
			{"id": "2", "remarks": "SG", "server": "h2", "server_port": 8389, "method": "chacha20-ietf-poly1305", "password": "pw2"},
		},
	})
	got, err := parseSIP008(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
	if got[0]["name"] != "HK" || got[0]["server"] != "h1" {
		t.Errorf("server 0: %+v", got[0])
	}
}

func TestSIP008Invalid(t *testing.T) {
	if _, err := parseSIP008([]byte(`not json`)); err == nil || !strings.Contains(err.Error(), "json") {
		t.Errorf("expected JSON error, got %v", err)
	}
}
```

- [ ] **Implementation** `internal/subscription/sip008.go`:
```go
package subscription

import (
	"encoding/json"
	"fmt"

	"vpnkit/internal/subscription/proto"
)

func parseSIP008(body []byte) ([]proto.Proxy, error) {
	var doc struct {
		Version int                 `json:"version"`
		Servers []map[string]any    `json:"servers"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("sip008: json: %w", err)
	}
	out := make([]proto.Proxy, 0, len(doc.Servers))
	for _, s := range doc.Servers {
		p := proto.Proxy{
			"type":     "ss",
			"server":   s["server"],
			"port":     toInt(s["server_port"]),
			"cipher":   s["method"],
			"password": s["password"],
		}
		if r, ok := s["remarks"].(string); ok {
			p["name"] = r
		}
		out = append(out, p)
	}
	return out, nil
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/ -v
git add internal/subscription/sip008.go internal/subscription/sip008_test.go
git commit -m "feat(subscription): SIP008 JSON list parser"
```

---

## Task 10: Convert dispatcher (format detection + bulk parse)

**Files:**
- Create: `internal/subscription/detect.go`
- Create: `internal/subscription/convert.go`
- Create: `internal/subscription/convert_test.go`

- [ ] **Test** `internal/subscription/convert_test.go`:
```go
package subscription

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestConvertClashYAML(t *testing.T) {
	body := []byte(`port: 7890
proxies:
  - {name: A, type: ss, server: 1.1.1.1, port: 8388, cipher: aes-128-gcm, password: x}
`)
	r, err := Convert(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "clash" {
		t.Errorf("source: %s", r.Source)
	}
	if len(r.Proxies) != 1 || r.Proxies[0]["name"] != "A" {
		t.Errorf("proxies: %+v", r.Proxies)
	}
}

func TestConvertBase64List(t *testing.T) {
	list := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:pwd")) + "@a.b:8388#X\n" +
		"trojan://pw@host:443?sni=s#Y"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(list)))
	r, err := Convert(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "base64-list" || len(r.Proxies) != 2 {
		t.Errorf("source=%s proxies=%d", r.Source, len(r.Proxies))
	}
}

func TestConvertSingleURI(t *testing.T) {
	r, err := Convert([]byte("trojan://pw@host:443#One"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != "uri" || len(r.Proxies) != 1 {
		t.Errorf("source=%s proxies=%d", r.Source, len(r.Proxies))
	}
}

func TestConvertSkipsMalformed(t *testing.T) {
	list := "vmess://bad\nss://" + base64.StdEncoding.EncodeToString([]byte("m:p")) + "@h:1#OK"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(list)))
	r, err := Convert(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Proxies) != 1 || len(r.Errors) != 1 {
		t.Errorf("proxies=%d errors=%d", len(r.Proxies), len(r.Errors))
	}
	if !strings.Contains(r.Errors[0].Error(), "vmess") {
		t.Errorf("error: %v", r.Errors[0])
	}
}
```

- [ ] **Implementation** `internal/subscription/detect.go`:
```go
package subscription

import (
	"encoding/base64"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatClash      Format = "clash"
	FormatSIP008     Format = "sip008"
	FormatBase64List Format = "base64-list"
	FormatURI        Format = "uri"
)

// Detect identifies the subscription's wire format from its bytes.
func Detect(body []byte) Format {
	trimmed := strings.TrimSpace(string(body))
	// 1. JSON?
	if strings.HasPrefix(trimmed, "{") {
		return FormatSIP008
	}
	// 2. Clash YAML?
	var probe map[string]any
	if err := yaml.Unmarshal(body, &probe); err == nil {
		if _, ok := probe["proxies"]; ok {
			return FormatClash
		}
		if _, ok := probe["proxy-groups"]; ok {
			return FormatClash
		}
	}
	// 3. Base64 list?
	if dec, err := tolerantB64(trimmed); err == nil && strings.Contains(string(dec), "://") {
		return FormatBase64List
	}
	// 4. Single-URI fallback.
	if strings.Contains(trimmed, "://") {
		return FormatURI
	}
	return FormatURI
}

func tolerantB64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.URLEncoding,
		base64.RawStdEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, base64.CorruptInputError(0)
}
```

`internal/subscription/convert.go`:
```go
// Package subscription fetches, detects format, and converts subscriptions into
// mihomo-compatible proxy lists.
package subscription

import (
	"bufio"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/subscription/proto"
)

// Result is the outcome of converting a subscription body.
type Result struct {
	Source  string         // "clash" | "sip008" | "base64-list" | "uri"
	Proxies []proto.Proxy
	// Raw is set when Source == "clash" so the assembler can preserve proxy-groups/rules.
	Raw map[string]any
	// Errors collects per-node parsing failures (subscription proceeds with whatever parsed OK).
	Errors []error
}

// Convert dispatches on detected format and returns Result.
func Convert(body []byte) (Result, error) {
	switch Detect(body) {
	case FormatClash:
		return convertClash(body)
	case FormatSIP008:
		px, err := parseSIP008(body)
		return Result{Source: string(FormatSIP008), Proxies: px}, err
	case FormatBase64List:
		dec, _ := tolerantB64(strings.TrimSpace(string(body)))
		return convertList(string(dec), string(FormatBase64List))
	default:
		return convertList(string(body), string(FormatURI))
	}
}

func convertClash(body []byte) (Result, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return Result{}, fmt.Errorf("clash yaml: %w", err)
	}
	r := Result{Source: string(FormatClash), Raw: doc}
	if px, ok := doc["proxies"].([]any); ok {
		for _, x := range px {
			if m, ok := x.(map[string]any); ok {
				r.Proxies = append(r.Proxies, proto.Proxy(m))
			}
		}
	}
	return r, nil
}

func convertList(s, source string) (Result, error) {
	r := Result{Source: source}
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.Contains(line, "://") {
			continue
		}
		p, err := proto.Parse(line)
		if err != nil {
			r.Errors = append(r.Errors, err)
			continue
		}
		r.Proxies = append(r.Proxies, p)
	}
	return r, sc.Err()
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/... -v
git add internal/subscription/detect.go internal/subscription/convert.go internal/subscription/convert_test.go
git commit -m "feat(subscription): format detection + bulk converter"
```

---

## Task 11: Group synthesis

**Files:**
- Create: `internal/subscription/groups.go`
- Create: `internal/subscription/groups_test.go`

- [ ] **Test** `internal/subscription/groups_test.go`:
```go
package subscription

import (
	"testing"

	"vpnkit/internal/subscription/proto"
)

func TestSynthesizeGroups(t *testing.T) {
	proxies := []proto.Proxy{
		{"name": "HK-01", "type": "ss"},
		{"name": "JP-02", "type": "vmess"},
	}
	g := SynthesizeGroups(proxies)
	if len(g) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(g))
	}
	names := map[string]bool{}
	for _, grp := range g {
		names[grp["name"].(string)] = true
	}
	for _, want := range []string{"🚀 Proxy", "♻️ Auto", "🎯 Direct", "🛑 Reject"} {
		if !names[want] {
			t.Errorf("missing group %s", want)
		}
	}
	// Proxy group should include both nodes after the special entries.
	for _, grp := range g {
		if grp["name"] == "🚀 Proxy" {
			members := grp["proxies"].([]string)
			if !contains(members, "HK-01") || !contains(members, "JP-02") {
				t.Errorf("Proxy group members: %v", members)
			}
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
```

- [ ] **Implementation** `internal/subscription/groups.go`:
```go
package subscription

import "vpnkit/internal/subscription/proto"

// SynthesizeGroups builds the default 4-group set (Proxy / Auto / Direct / Reject)
// for subscriptions that don't supply their own proxy-groups.
func SynthesizeGroups(proxies []proto.Proxy) []map[string]any {
	names := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if n, ok := p["name"].(string); ok && n != "" {
			names = append(names, n)
		}
	}
	proxyGroup := append([]string{"♻️ Auto", "🎯 Direct"}, names...)
	return []map[string]any{
		{"name": "🚀 Proxy", "type": "select", "proxies": proxyGroup},
		{"name": "♻️ Auto", "type": "url-test", "proxies": names,
			"url": "https://www.gstatic.com/generate_204", "interval": 300, "tolerance": 50},
		{"name": "🎯 Direct", "type": "select", "proxies": []string{"DIRECT"}},
		{"name": "🛑 Reject", "type": "select", "proxies": []string{"REJECT", "DIRECT"}},
	}
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/ -v
git add internal/subscription/groups.go internal/subscription/groups_test.go
git commit -m "feat(subscription): default group synthesis"
```

---

## Task 12: Patch overlay loader

**Files:**
- Create: `internal/patch/patch.go`
- Create: `internal/patch/patch_test.go`

- [ ] **Test** `internal/patch/patch_test.go`:
```go
package patch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyMissingFile(t *testing.T) {
	target := map[string]any{"port": 7890}
	if err := Apply(filepath.Join(t.TempDir(), "patch.yaml"), target); err != nil {
		t.Errorf("missing file should be a no-op: %v", err)
	}
	if target["port"] != 7890 {
		t.Errorf("target mutated: %v", target)
	}
}

func TestApplyDeepMerge(t *testing.T) {
	dir := t.TempDir()
	patchPath := filepath.Join(dir, "patch.yaml")
	_ = os.WriteFile(patchPath, []byte(`
port: 7891
dns:
  enable: true
  nameserver: [8.8.8.8]
`), 0o600)
	target := map[string]any{
		"port": 7890,
		"dns": map[string]any{
			"enhanced-mode": "fake-ip",
		},
	}
	if err := Apply(patchPath, target); err != nil {
		t.Fatal(err)
	}
	if target["port"] != 7891 {
		t.Errorf("port: %v", target["port"])
	}
	dns := target["dns"].(map[string]any)
	if dns["enable"] != true || dns["enhanced-mode"] != "fake-ip" {
		t.Errorf("dns merge: %+v", dns)
	}
	// Array replace: nameserver came in as [8.8.8.8]
	ns, _ := dns["nameserver"].([]any)
	if len(ns) != 1 || ns[0] != "8.8.8.8" {
		t.Errorf("nameserver: %v", ns)
	}
}
```

- [ ] **Implementation** `internal/patch/patch.go`:
```go
// Package patch loads a user-edited overlay YAML and deep-merges it into a mihomo
// config map. Maps merge recursively; arrays in the patch replace target arrays.
package patch

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Apply reads patchPath (no-op if missing) and merges its contents into target.
func Apply(patchPath string, target map[string]any) error {
	data, err := os.ReadFile(patchPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var overlay map[string]any
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return err
	}
	deepMerge(target, overlay)
	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if existingMap, ok := dst[k].(map[string]any); ok {
			if newMap, ok := v.(map[string]any); ok {
				deepMerge(existingMap, newMap)
				continue
			}
		}
		// Arrays and scalars: replace.
		dst[k] = v
	}
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/patch/ -v
git add internal/patch/
git commit -m "feat(patch): user overlay deep-merge"
```

---

## Task 13: Assemble pipeline (subscription + rules + patch → final config)

**Files:**
- Create: `internal/subscription/assemble.go`
- Create: `internal/subscription/assemble_test.go`

- [ ] **Test** `internal/subscription/assemble_test.go`:
```go
package subscription

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/subscription/proto"
)

func TestAssembleMergesEverything(t *testing.T) {
	dir := t.TempDir()
	patchPath := filepath.Join(dir, "patch.yaml")
	_ = os.WriteFile(patchPath, []byte("log-level: debug\n"), 0o600)

	r := Result{
		Source: "uri",
		Proxies: []proto.Proxy{
			{"name": "HK-01", "type": "ss", "server": "1.1.1.1", "port": 8388, "cipher": "aes-128-gcm", "password": "x"},
		},
	}
	yamlBytes, err := Assemble(AssembleInput{
		Result:           r,
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "secret",
		RuleTemplate:     "minimal",
		PatchPath:        patchPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(yamlBytes)
	for _, want := range []string{
		"mixed-port: 7890",
		"HK-01",
		"GEOIP,CN",
		"log-level: debug",
		"🚀 Proxy",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestAssembleKeepsExistingGroupsFromClash(t *testing.T) {
	r := Result{
		Source: "clash",
		Raw: map[string]any{
			"proxy-groups": []any{
				map[string]any{"name": "MyGroup", "type": "select", "proxies": []any{"DIRECT"}},
			},
		},
		Proxies: []proto.Proxy{{"name": "n1", "type": "ss", "server": "h", "port": 1, "cipher": "c", "password": "p"}},
	}
	out, err := Assemble(AssembleInput{Result: r, MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "s", RuleTemplate: "minimal"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "MyGroup") {
		t.Errorf("custom group lost:\n%s", out)
	}
}
```

- [ ] **Implementation** `internal/subscription/assemble.go`:
```go
package subscription

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/patch"
	"vpnkit/internal/rules"
	"vpnkit/internal/subscription/proto"
)

// AssembleInput drives a single Assemble call.
type AssembleInput struct {
	Result           Result
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	LogLevel         string
	RuleTemplate     string // loyalsoldier|minimal
	PatchPath        string // optional; empty = skip
}

// Assemble produces the final config.yaml bytes by combining:
//   base skeleton + subscription proxies + groups (synthesized or from clash) + rules + patch overlay.
func Assemble(in AssembleInput) ([]byte, error) {
	if in.MixedPort == 0 {
		in.MixedPort = 7890
	}
	if in.ControllerPort == 0 {
		in.ControllerPort = 9090
	}
	if in.LogLevel == "" {
		in.LogLevel = "info"
	}
	ruleYAML, err := rules.Load(in.RuleTemplate)
	if err != nil {
		return nil, err
	}
	var ruleDoc map[string]any
	if err := yaml.Unmarshal(ruleYAML, &ruleDoc); err != nil {
		return nil, fmt.Errorf("rule template parse: %w", err)
	}

	doc := map[string]any{
		"mixed-port":          in.MixedPort,
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           in.LogLevel,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", in.ControllerPort),
		"secret":              in.ControllerSecret,
	}

	// Proxies.
	rawProxies := make([]any, 0, len(in.Result.Proxies))
	for _, p := range in.Result.Proxies {
		rawProxies = append(rawProxies, map[string]any(p))
	}
	doc["proxies"] = rawProxies

	// Proxy-groups: keep subscription-supplied; otherwise synthesize.
	if in.Result.Source == "clash" && in.Result.Raw != nil {
		if g, ok := in.Result.Raw["proxy-groups"]; ok {
			doc["proxy-groups"] = g
		}
	}
	if _, has := doc["proxy-groups"]; !has {
		doc["proxy-groups"] = groupsToAny(SynthesizeGroups(in.Result.Proxies))
	}

	// Rule template fields (overrides any pre-existing).
	for k, v := range ruleDoc {
		doc[k] = v
	}

	// User patch overlay (last write wins).
	if in.PatchPath != "" {
		if err := patch.Apply(in.PatchPath, doc); err != nil {
			return nil, fmt.Errorf("patch: %w", err)
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

func groupsToAny(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, g := range in {
		out[i] = g
	}
	return out
}

var _ proto.Proxy // keep import
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/ -v
git add internal/subscription/assemble.go internal/subscription/assemble_test.go
git commit -m "feat(subscription): final config assembler"
```

---

## Task 14: Subscription HTTP fetch

**Files:**
- Create: `internal/subscription/fetch.go`
- Create: `internal/subscription/fetch_test.go`

- [ ] **Test** `internal/subscription/fetch_test.go`:
```go
package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSendsUA(t *testing.T) {
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	body, err := Fetch(context.Background(), srv.URL, "clash-verge/1.0")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "body" {
		t.Errorf("body: %s", body)
	}
	if seenUA != "clash-verge/1.0" {
		t.Errorf("UA: %s", seenUA)
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := Fetch(context.Background(), srv.URL, ""); err == nil {
		t.Error("expected error")
	}
}
```

- [ ] **Implementation** `internal/subscription/fetch.go`:
```go
package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultUA = "clash-verge/v1.4.0"

// Fetch retrieves a subscription body. ua is optional (defaults to clash-verge UA).
func Fetch(ctx context.Context, url, ua string) ([]byte, error) {
	if ua == "" {
		ua = defaultUA
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription fetch %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20)) // 32 MiB safety cap
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/subscription/ -v
git add internal/subscription/fetch.go internal/subscription/fetch_test.go
git commit -m "feat(subscription): HTTP fetcher with UA + 32MiB cap"
```

---

## Task 15: Profiles manager (full lifecycle)

**Files:**
- Create: `internal/profiles/manager.go`
- Create: `internal/profiles/manager_test.go`

- [ ] **Test** `internal/profiles/manager_test.go`:
```go
package profiles

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndUpdate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("trojan://pw@h.example:443#node1"))
	}))
	defer srv.Close()

	m := New(Config{
		ConfigYAMLPath:   configPath,
		PatchPath:        filepath.Join(dir, "patch.yaml"),
		ControllerPort:   9090,
		ControllerSecret: "x",
		RuleTemplate:     "minimal",
	})
	if err := m.Add(Profile{Name: "main", URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Update(context.Background(), "main"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "node1") || !strings.Contains(string(got), "GEOIP,CN") {
		t.Errorf("config missing expected content:\n%s", got)
	}
}

func TestActivateSwitchesActive(t *testing.T) {
	dir := t.TempDir()
	m := New(Config{
		ConfigYAMLPath:   filepath.Join(dir, "config.yaml"),
		PatchPath:        filepath.Join(dir, "patch.yaml"),
		ControllerPort:   9090,
		ControllerSecret: "x",
		RuleTemplate:     "minimal",
	})
	_ = m.Add(Profile{Name: "a"})
	_ = m.Add(Profile{Name: "b"})
	m.SetActive("b")
	if m.Active() != "b" {
		t.Errorf("active: %s", m.Active())
	}
	if names := m.List(); names[0] != "a" || names[1] != "b" {
		t.Errorf("list: %v", names)
	}
}
```

- [ ] **Implementation** `internal/profiles/manager.go`:
```go
// Package profiles is the high-level facade combining subscription fetch + convert +
// assemble + config write, plus storing profile metadata in memory.
package profiles

import (
	"context"
	"errors"
	"sync"
	"time"

	"vpnkit/internal/config"
	"vpnkit/internal/subscription"
)

// Profile is one subscription entry tracked in-memory by the manager.
type Profile struct {
	Name        string
	URL         string
	UserAgent   string
	LastUpdated time.Time
	NodeCount   int
}

// Config configures Manager.
type Config struct {
	ConfigYAMLPath   string // path to write the assembled mihomo config
	PatchPath        string // user overlay path (may not exist)
	ControllerPort   int
	ControllerSecret string
	MixedPort        int    // default 7890 if zero
	RuleTemplate     string // loyalsoldier|minimal
}

// Manager holds the active profile list and writes config files.
type Manager struct {
	cfg      Config
	mu       sync.Mutex
	profiles []Profile
	active   string
}

// New constructs a Manager. ListSeed lets tests/restore preload profiles.
func New(cfg Config) *Manager { return &Manager{cfg: cfg} }

// Load replaces the profile list (e.g. from store.Cfg.Profiles).
func (m *Manager) Load(list []Profile, active string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles = list
	m.active = active
}

// List returns profile names in insertion order.
func (m *Manager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, len(m.profiles))
	for i, p := range m.profiles {
		names[i] = p.Name
	}
	return names
}

// All returns a copy of all profile entries.
func (m *Manager) All() []Profile {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Profile, len(m.profiles))
	copy(out, m.profiles)
	return out
}

// Active returns the currently-active profile name.
func (m *Manager) Active() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// SetActive marks a profile name as active.
func (m *Manager) SetActive(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = name
}

// Add registers a new profile.
func (m *Manager) Add(p Profile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.profiles {
		if e.Name == p.Name {
			return errors.New("profiles: duplicate name")
		}
	}
	m.profiles = append(m.profiles, p)
	if m.active == "" {
		m.active = p.Name
	}
	return nil
}

// Remove deletes a profile by name.
func (m *Manager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.profiles[:0]
	for _, p := range m.profiles {
		if p.Name != name {
			out = append(out, p)
		}
	}
	m.profiles = out
	if m.active == name {
		m.active = ""
	}
}

// Update fetches the named profile's URL, converts, assembles, and writes config.yaml.
// Returns the number of proxies parsed.
func (m *Manager) Update(ctx context.Context, name string) (int, error) {
	m.mu.Lock()
	var p *Profile
	for i := range m.profiles {
		if m.profiles[i].Name == name {
			p = &m.profiles[i]
			break
		}
	}
	cfg := m.cfg
	m.mu.Unlock()
	if p == nil {
		return 0, errors.New("profiles: not found")
	}

	body, err := subscription.Fetch(ctx, p.URL, p.UserAgent)
	if err != nil {
		return 0, err
	}
	res, err := subscription.Convert(body)
	if err != nil {
		return 0, err
	}
	yamlBytes, err := subscription.Assemble(subscription.AssembleInput{
		Result:           res,
		MixedPort:        cfg.MixedPort,
		ControllerPort:   cfg.ControllerPort,
		ControllerSecret: cfg.ControllerSecret,
		RuleTemplate:     cfg.RuleTemplate,
		PatchPath:        cfg.PatchPath,
	})
	if err != nil {
		return 0, err
	}
	if err := config.AtomicWrite(cfg.ConfigYAMLPath, yamlBytes, 0o600); err != nil {
		return 0, err
	}

	m.mu.Lock()
	p.LastUpdated = time.Now()
	p.NodeCount = len(res.Proxies)
	m.mu.Unlock()
	return len(res.Proxies), nil
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/profiles/ -v
git add internal/profiles/
git commit -m "feat(profiles): lifecycle manager (add/remove/activate/update)"
```

---

## Task 16: API client extension — proxies & delay

**Files:**
- Create: `internal/api/proxies.go`
- Create: `internal/api/proxies_test.go`

- [ ] **Test** `internal/api/proxies_test.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProxies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"GLOBAL": map[string]any{"type": "Selector", "now": "DIRECT", "all": []string{"DIRECT", "REJECT"}},
				"DIRECT": map[string]any{"type": "Direct"},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	out, err := c.GetProxies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	g, ok := out["GLOBAL"]
	if !ok || g.Type != "Selector" || g.Now != "DIRECT" {
		t.Errorf("GLOBAL: %+v", g)
	}
	if len(g.All) != 2 {
		t.Errorf("All: %v", g.All)
	}
}

func TestPutProxy(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.PutProxy(context.Background(), "🚀 Proxy", "HK-01"); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "HK-01" {
		t.Errorf("body: %v", got)
	}
}

func TestDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{"delay": 123})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	d, err := c.Delay(context.Background(), "HK-01", "https://www.gstatic.com/generate_204", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if d != 123 {
		t.Errorf("delay: %d", d)
	}
}
```

- [ ] **Implementation** `internal/api/proxies.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ProxyInfo mirrors one entry in /proxies' "proxies" map.
type ProxyInfo struct {
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

type proxiesResponse struct {
	Proxies map[string]ProxyInfo `json:"proxies"`
}

// GetProxies fetches the /proxies snapshot.
func (c *Client) GetProxies(ctx context.Context) (map[string]ProxyInfo, error) {
	var resp proxiesResponse
	if err := c.do(ctx, http.MethodGet, "/proxies", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Proxies, nil
}

// PutProxy selects `node` as the current member of group `group`.
func (c *Client) PutProxy(ctx context.Context, group, node string) error {
	return c.do(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group),
		map[string]string{"name": node}, nil)
}

// Delay queries a single node's delay (ms) against `testURL` with timeout in ms.
func (c *Client) Delay(ctx context.Context, node, testURL string, timeoutMs int) (int, error) {
	q := url.Values{}
	q.Set("url", testURL)
	q.Set("timeout", fmt.Sprintf("%d", timeoutMs))
	path := "/proxies/" + url.PathEscape(node) + "/delay?" + q.Encode()
	var out struct {
		Delay int `json:"delay"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return 0, err
	}
	return out.Delay, nil
}

// GroupDelay tests every member of a group at once.
func (c *Client) GroupDelay(ctx context.Context, group, testURL string, timeoutMs int) (map[string]int, error) {
	q := url.Values{}
	q.Set("url", testURL)
	q.Set("timeout", fmt.Sprintf("%d", timeoutMs))
	path := "/group/" + url.PathEscape(group) + "/delay?" + q.Encode()
	out := map[string]int{}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// json import kept just in case of future direct json decoding paths.
var _ = json.NewDecoder
```

- [ ] **Commit**:
```bash
go test -race ./internal/api/ -v
git add internal/api/proxies.go internal/api/proxies_test.go
git commit -m "feat(api): /proxies snapshot + selector + delay test"
```

---

## Task 17: New shared messages for Profiles & Proxies

**Files:**
- Modify: `internal/msg/msg.go`

Append to the file:
```go
// Profile-related messages emitted by the profiles manager driver.

// ProxiesSnapshot is one /proxies tick.
type ProxiesSnapshot struct {
	Groups map[string]ProxyGroup
}

// ProxyGroup is the dashboard-friendly form of an entry in /proxies.
type ProxyGroup struct {
	Name string
	Type string
	Now  string
	All  []string
}

// ProfileUpdated is dispatched when a profile finishes updating.
type ProfileUpdated struct {
	Name      string
	NodeCount int
}

// ProfileError is dispatched when a profile operation fails.
type ProfileError struct {
	Name string
	Err  error
}

// DelayResults is dispatched after a group delay test.
type DelayResults struct {
	Group   string
	Results map[string]int
}
```

- [ ] **Test:** None directly (types only); next tasks consume them.

- [ ] **Commit**:
```bash
go build ./...
git add internal/msg/msg.go
git commit -m "feat(msg): profile, proxies, delay messages"
```

---

## Task 18: Profiles tab Model + table

**Files:**
- Create: `internal/tabs/profiles/profiles.go`
- Create: `internal/tabs/profiles/profiles_test.go`

This tab shows a table of profiles with these columns: Active marker, Name, URL (truncated), NodeCount, LastUpdated. Keys: `a` open add form, `u` update selected, `Enter` activate, `d` delete (with confirm), `↑↓` navigate.

For Phase 2 simplicity the add form is a multi-line `textinput`-driven popup; full bubbles/form features land later.

- [ ] **Test** `internal/tabs/profiles/profiles_test.go`:
```go
package profiles

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
	"vpnkit/internal/profiles"
)

func TestRendersProfiles(t *testing.T) {
	m := New(profiles.New(profiles.Config{}))
	m.SetProfiles([]profiles.Profile{
		{Name: "main", URL: "https://example.com/sub", NodeCount: 3},
	}, "main")
	view := m.View(80, 24)
	if !strings.Contains(view, "main") || !strings.Contains(view, "★") {
		t.Errorf("missing label or active marker:\n%s", view)
	}
}

func TestSelectionAdvances(t *testing.T) {
	mgr := profiles.New(profiles.Config{})
	_ = mgr.Add(profiles.Profile{Name: "a"})
	_ = mgr.Add(profiles.Profile{Name: "b"})
	m := New(mgr)
	m.SetProfiles(mgr.All(), "")
	if m.Selected().Name != "a" {
		t.Errorf("first: %v", m.Selected())
	}
	m, _ = m.Update(msg.ProfileUpdated{Name: "ignored"}) // arbitrary msg, no-op
	m.MoveDown()
	if m.Selected().Name != "b" {
		t.Errorf("after down: %v", m.Selected())
	}
}
```

- [ ] **Implementation** `internal/tabs/profiles/profiles.go`:
```go
// Package profiles implements the Profiles tab (subscription CRUD).
package profiles

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/profiles"
)

// Model is the Profiles tab.
type Model struct {
	mgr      *profiles.Manager
	list     []profiles.Profile
	active   string
	cursor   int
}

// New builds an empty Profiles tab. The owner injects a *profiles.Manager.
func New(mgr *profiles.Manager) Model {
	return Model{mgr: mgr}
}

// SetProfiles refreshes the rendered list (called when manager state changes).
func (m *Model) SetProfiles(list []profiles.Profile, active string) {
	m.list = list
	m.active = active
	if m.cursor >= len(m.list) {
		m.cursor = 0
	}
}

// Selected returns the currently-highlighted profile.
func (m Model) Selected() profiles.Profile {
	if m.cursor >= len(m.list) {
		return profiles.Profile{}
	}
	return m.list[m.cursor]
}

// MoveDown / MoveUp control the cursor (exposed for tests & key handlers).
func (m *Model) MoveDown() {
	if m.cursor < len(m.list)-1 {
		m.cursor++
	}
}
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update absorbs tea.Msg. Profile manager events drive the displayed state.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	switch ev := message.(type) {
	case msg.ProfileUpdated:
		// Refresh list from manager after a successful update.
		if m.mgr != nil {
			m.list = m.mgr.All()
			m.active = m.mgr.Active()
		}
	case msg.ProfileError:
		// Surface errors via flash bar in the parent app; tab itself is read-only.
		_ = ev
	}
	return m, nil
}

// View renders the tab.
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Profiles")
	var rows []string
	rows = append(rows, header, "")
	if len(m.list) == 0 {
		rows = append(rows, "  No subscriptions yet — press 'a' to add")
	}
	for i, p := range m.list {
		marker := "  "
		if p.Name == m.active {
			marker = "★ "
		}
		row := fmt.Sprintf("%s%-12s  %-40s  nodes=%d", marker, p.Name, truncate(p.URL, 40), p.NodeCount)
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ " + row)
		} else {
			row = "  " + row
		}
		rows = append(rows, row)
	}
	rows = append(rows, "", "[a] add  [u] update  [Enter] activate  [d] delete  [↑↓] navigate")
	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/tabs/profiles/ -v
git add internal/tabs/profiles/
git commit -m "feat(tabs): profiles tab with table view"
```

---

## Task 19: Profiles add/edit popup form

**Files:**
- Create: `internal/tabs/profiles/form.go`
- Create: `internal/tabs/profiles/form_test.go`

Lightweight form using `bubbles/textinput`. Two fields: Name, URL. `Tab` to switch field, `Enter` to submit, `Esc` to cancel.

- [ ] **Test** `internal/tabs/profiles/form_test.go`:
```go
package profiles

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFormCollectsFields(t *testing.T) {
	f := newForm()
	for _, r := range "main" {
		f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "https://x" {
		f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if f.Name() != "main" {
		t.Errorf("name: %q", f.Name())
	}
	if f.URL() != "https://x" {
		t.Errorf("url: %q", f.URL())
	}
}
```

- [ ] **Implementation** `internal/tabs/profiles/form.go`:
```go
package profiles

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type formField int

const (
	fieldName formField = iota
	fieldURL
)

// form is a 2-field popup for add/edit.
type form struct {
	name   textinput.Model
	url    textinput.Model
	active formField
}

func newForm() form {
	n := textinput.New()
	n.Placeholder = "profile name"
	n.Focus()
	u := textinput.New()
	u.Placeholder = "https://example.com/sub"
	return form{name: n, url: u, active: fieldName}
}

func (f form) Name() string { return strings.TrimSpace(f.name.Value()) }
func (f form) URL() string  { return strings.TrimSpace(f.url.Value()) }

func (f form) Update(msg tea.Msg) (form, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.Type {
		case tea.KeyTab, tea.KeyShiftTab:
			if f.active == fieldName {
				f.active = fieldURL
				f.name.Blur()
				f.url.Focus()
			} else {
				f.active = fieldName
				f.url.Blur()
				f.name.Focus()
			}
			return f, nil
		}
	}
	var cmd tea.Cmd
	if f.active == fieldName {
		f.name, cmd = f.name.Update(msg)
	} else {
		f.url, cmd = f.url.Update(msg)
	}
	return f, cmd
}

func (f form) View() string {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)
	return style.Render("Add Profile\n\nName:\n" + f.name.View() + "\n\nURL:\n" + f.url.View() + "\n\n[Tab] switch  [Enter] save  [Esc] cancel")
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/tabs/profiles/ -v
git add internal/tabs/profiles/form.go internal/tabs/profiles/form_test.go
git commit -m "feat(tabs): profiles add/edit popup form"
```

---

## Task 20: Wire Profiles tab into app

**Files:**
- Modify: `internal/app/model.go` (replace Profiles stub with real tab)
- Modify: `internal/app/view.go` (render Profiles tab when active)
- Modify: `internal/app/update.go` (dispatch profile messages)
- Modify: `internal/app/run.go` (instantiate profiles.Manager, wire callbacks)

This is the largest stitch task in Phase 2. Follow Phase 1's pattern: introduce new struct fields, replace the `stubs[TabProfiles]` indirection with `profiles.Model`, send tea.Cmds for `Update` action.

### Step 1 — Add `profiles.Manager` to `Model`

Edit `internal/app/model.go`:
- Add import `"vpnkit/internal/profiles"` and `tabprofiles "vpnkit/internal/tabs/profiles"`
- Add fields:
  ```go
  profilesMgr  *profiles.Manager
  profilesTab  tabprofiles.Model
  showAddForm  bool
  addForm      tabprofiles.Form
  ```

Wait — `tabprofiles.Form` is unexported (lowercase `form` in T19). Re-export it: rename `form` → `Form`, `newForm` → `NewForm`, in `internal/tabs/profiles/form.go` BEFORE this task starts.

(Add that rename to the task as Step 0.)

### Step 0 — rename form to Form

In `internal/tabs/profiles/form.go`, replace `form` → `Form` everywhere, `newForm` → `NewForm`.

### Step 2 — Update `NewModel` to take a `*profiles.Manager`

`internal/app/model.go`:
```go
func NewModel(client *api.Client, mgr *profiles.Manager) Model {
    stubs := [NumTabs]stub.Model{}
    for i := TabProxies; i < NumTabs; i++ {
        if i == TabProfiles {
            continue
        }
        stubs[i] = stub.New(TabNames[i])
    }
    return Model{
        keys:        DefaultKeys(),
        activeTab:   TabDashboard,
        dashboard:   dashboard.New(),
        profilesTab: tabprofiles.New(mgr),
        profilesMgr: mgr,
        stubs:       stubs,
        apiClient:   client,
    }
}
```

### Step 3 — Update `view.go` switch to render `profilesTab` when active

```go
case TabProfiles:
    body = m.profilesTab.View(bodyWidth, bodyHeight)
```

And when `m.showAddForm` is true, overlay the form (centered).

### Step 4 — Update `update.go` to handle Profiles-specific keys

When `activeTab == TabProfiles` and not in form mode, intercept `a/u/d/Enter/↑/↓`. When `showAddForm`, route keys to `addForm.Update`, and on Enter submit + dispatch a tea.Cmd to call `profilesMgr.Add` + trigger Update.

### Step 5 — Update `run.go` to construct `profilesMgr`, wire ticker for `/proxies` polling, and ensure `NewModel` is updated

```go
profMgr := profiles.New(profiles.Config{
    ConfigYAMLPath:   p.MihomoConfigFile(),
    PatchPath:        filepath.Join(p.MihomoConfig, "patch.yaml"),
    ControllerPort:   st.Cfg.ControllerPort,
    ControllerSecret: st.Cfg.ControllerSecret,
    RuleTemplate:     st.Cfg.RuleTemplate,
})
profMgr.Load(toProfilesProfiles(st.Cfg.Profiles), st.Cfg.ActiveProfile)
model := NewModel(client, profMgr)
```

Add helper:
```go
func toProfilesProfiles(in []store.Profile) []profiles.Profile {
    out := make([]profiles.Profile, len(in))
    for i, p := range in {
        out[i] = profiles.Profile{Name: p.Name, URL: p.URL, UserAgent: p.UserAgent, LastUpdated: p.LastUpdated}
    }
    return out
}
```

After Update succeeds, persist back to store and emit `msg.ProfileUpdated`. After every change, also call `client.ReloadConfig(ctx, p.MihomoConfigFile())`.

### Step 6 — Tests

Existing `internal/app/model_test.go` still passes (tab navigation unchanged). Add a new test verifying that pressing `3` shows the Profiles tab not the stub.

### Build + commit

```bash
go build ./...
go test -race ./... -v
git add internal/app/ internal/tabs/profiles/form.go
git commit -m "feat(app): wire profiles tab + manager + reload pipeline"
```

This task SHOULD complete with all packages still green. If the form/wiring proves more involved, split into 20a (form rename + Model field) and 20b (Update/Run wiring) commits.

---

## Task 21: Proxies tab Model

**Files:**
- Create: `internal/tabs/proxies/proxies.go`
- Create: `internal/tabs/proxies/proxies_test.go`

Renders mihomo's proxy groups as a tree. Top-level entries are groups (Selector / URLTest / Direct types); when expanded, members are listed under them.

For Phase 2 keep it simple: flat list of "group → current selection" with an expand toggle (`Enter` on a group expands inline to show members).

- [ ] **Test** `internal/tabs/proxies/proxies_test.go`:
```go
package proxies

import (
	"strings"
	"testing"

	"vpnkit/internal/msg"
)

func TestRendersGroups(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ProxiesSnapshot{
		Groups: map[string]msg.ProxyGroup{
			"GLOBAL": {Name: "GLOBAL", Type: "Selector", Now: "DIRECT", All: []string{"DIRECT", "REJECT"}},
		},
	})
	view := m.View(80, 24)
	if !strings.Contains(view, "GLOBAL") || !strings.Contains(view, "DIRECT") {
		t.Errorf("view:\n%s", view)
	}
}

func TestRendersDelayResults(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ProxiesSnapshot{
		Groups: map[string]msg.ProxyGroup{
			"G": {Name: "G", Type: "Selector", Now: "n1", All: []string{"n1", "n2"}},
		},
	})
	m, _ = m.Update(msg.DelayResults{Group: "G", Results: map[string]int{"n1": 42, "n2": 99}})
	view := m.View(80, 24)
	if !strings.Contains(view, "42") || !strings.Contains(view, "99") {
		t.Errorf("delays not rendered:\n%s", view)
	}
}
```

- [ ] **Implementation** `internal/tabs/proxies/proxies.go`:
```go
// Package proxies implements the Proxies tab (group/node selection + delay tests).
package proxies

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
)

// Model is the Proxies tab.
type Model struct {
	groups  map[string]msg.ProxyGroup
	delays  map[string]map[string]int // group → node → ms
	order   []string                  // group names in render order
	cursor  int                       // index into order
	expanded map[string]bool
}

// New returns an empty Proxies tab model.
func New() Model {
	return Model{
		groups:   map[string]msg.ProxyGroup{},
		delays:   map[string]map[string]int{},
		expanded: map[string]bool{},
	}
}

func (Model) Init() tea.Cmd { return nil }

// Update absorbs ProxiesSnapshot + DelayResults.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	switch ev := message.(type) {
	case msg.ProxiesSnapshot:
		m.groups = ev.Groups
		m.order = sortedSelectableGroups(ev.Groups)
		if m.cursor >= len(m.order) {
			m.cursor = 0
		}
	case msg.DelayResults:
		m.delays[ev.Group] = ev.Results
	}
	return m, nil
}

// Cursor utilities used by parent's key dispatcher.
func (m *Model) MoveDown() {
	if m.cursor < len(m.order)-1 {
		m.cursor++
	}
}
func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}
func (m Model) SelectedGroup() string {
	if m.cursor >= len(m.order) {
		return ""
	}
	return m.order[m.cursor]
}
func (m *Model) ToggleExpand() {
	g := m.SelectedGroup()
	if g != "" {
		m.expanded[g] = !m.expanded[g]
	}
}

// View renders the tab.
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Proxies")
	var rows []string
	rows = append(rows, header, "")
	if len(m.order) == 0 {
		rows = append(rows, "  No proxy groups (mihomo not yet running or no subscription active)")
	}
	for i, g := range m.order {
		group := m.groups[g]
		expanded := m.expanded[g]
		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ ")
		}
		rows = append(rows, fmt.Sprintf("%s%-20s  %-12s  → %s", prefix, group.Name, group.Type, group.Now))
		if expanded {
			for _, node := range group.All {
				delayStr := ""
				if d, ok := m.delays[g][node]; ok {
					delayStr = fmt.Sprintf("%d ms", d)
				}
				marker := "   "
				if node == group.Now {
					marker = "  ✓"
				}
				rows = append(rows, fmt.Sprintf("    %s %-30s  %s", marker, node, delayStr))
			}
		}
	}
	rows = append(rows, "", "[Enter] expand/switch  [t] delay test  [↑↓] navigate")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}

func sortedSelectableGroups(in map[string]msg.ProxyGroup) []string {
	var names []string
	for k, g := range in {
		// Hide special built-ins (single-node groups, Direct/Reject).
		if g.Type == "Direct" || g.Type == "Reject" || g.Type == "Pass" {
			continue
		}
		if len(g.All) <= 0 {
			continue
		}
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
```

- [ ] **Commit**:
```bash
go test -race ./internal/tabs/proxies/ -v
git add internal/tabs/proxies/
git commit -m "feat(tabs): proxies tab with group tree view"
```

---

## Task 22: Delay test workflow

**Files:**
- Create: `internal/tabs/proxies/delay.go`
- Modify: `internal/tabs/proxies/proxies.go` (add `DelayCmd` builder)

Provides a `tea.Cmd` constructor that hits `/group/{name}/delay` and produces a `DelayResults` message. The parent app's `Update` handles key `t` by invoking this.

- [ ] **Implementation** `internal/tabs/proxies/delay.go`:
```go
package proxies

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/msg"
)

// DelayCmd returns a tea.Cmd that performs a group delay test and emits
// msg.DelayResults on completion (or empty results on error).
func DelayCmd(client *api.Client, group string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := client.GroupDelay(ctx, group, "https://www.gstatic.com/generate_204", 5000)
		if err != nil {
			return msg.DelayResults{Group: group, Results: map[string]int{}}
		}
		return msg.DelayResults{Group: group, Results: res}
	}
}
```

- [ ] **Commit**:
```bash
go build ./...
git add internal/tabs/proxies/delay.go
git commit -m "feat(tabs): delay test command for Proxies tab"
```

---

## Task 23: Wire Proxies tab into app

**Files:**
- Modify: `internal/app/model.go` (add `proxiesTab tabproxies.Model`)
- Modify: `internal/app/view.go` (render proxies when active)
- Modify: `internal/app/update.go` (handle keys: `Enter`/`t`, dispatch `DelayCmd` + `PutProxy`)
- Modify: `internal/app/run.go` (poll `/proxies` every 5s; convert `api.ProxyInfo` → `msg.ProxyGroup`)

Add `pollProxies` goroutine to `run.go`:

```go
func pollProxies(prog *tea.Program, client *api.Client) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    for {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        proxies, err := client.GetProxies(ctx)
        cancel()
        if err == nil {
            groups := map[string]msg.ProxyGroup{}
            for name, info := range proxies {
                groups[name] = msg.ProxyGroup{Name: name, Type: info.Type, Now: info.Now, All: info.All}
            }
            prog.Send(msg.ProxiesSnapshot{Groups: groups})
        }
        <-ticker.C
    }
}
```

Call `go pollProxies(prog, client)` in `Run()`.

In `update.go`:
```go
case TabProxies:
    switch m := msg.(type) {
    case tea.KeyMsg:
        switch m.String() {
        case "up": model.proxiesTab.MoveUp()
        case "down": model.proxiesTab.MoveDown()
        case "enter": model.proxiesTab.ToggleExpand()
        case "t":
            grp := model.proxiesTab.SelectedGroup()
            if grp != "" {
                cmd = proxies.DelayCmd(model.apiClient, grp)
            }
        }
    }
```

(Wire it into the existing tea.KeyMsg case in update.go.)

- [ ] **Commit**:
```bash
go build ./...
go test -race ./... -v
git add internal/app/
git commit -m "feat(app): wire proxies tab + /proxies polling + delay test"
```

---

## Task 24: Smoke check + README Phase 2 quickstart + tag

- [ ] **Step 1: rebuild + install**
```bash
export PATH="$HOME/.local/go/bin:$PATH"
make install
```

- [ ] **Step 2: full test sweep**
```bash
go test -race ./...
```
All packages green.

- [ ] **Step 3: manual smoke**

(coordinate with user — TUI smoke requires a running mihomo without conflicts)

If mihomo is running and reachable:
1. Launch `vpnkit`.
2. Press `3` to enter Profiles. Press `a` to add a subscription. Save.
3. Press `u` to update; verify the flash shows "N nodes parsed".
4. Press `2` to enter Proxies. Verify groups + Now/All render.
5. Press `Enter` on a group to expand; `t` to run delay test.
6. Press `q` to quit; verify mihomo is still running and `~/.config/mihomo/config.yaml` reflects the subscription.

- [ ] **Step 4: README update**

Append to `README.md`:
```markdown

## Phase 2 features

Profiles tab supports:
- `a` add subscription (popup form: name + URL)
- `u` update selected subscription (fetches, parses, writes `config.yaml`, reloads mihomo)
- `d` delete; `Enter` activate; `↑↓` navigate

Proxies tab supports:
- Live group/node view from mihomo's `/proxies` (polled every 5 s)
- `Enter` to expand a group and pick a node
- `t` to run a delay test against the highlighted group

Supported subscription formats: Clash YAML, SIP008 JSON, Base64-encoded URI list,
and single-URI variants of `vmess://`, `ss://`, `ssr://`, `trojan://`, `vless://`,
`hysteria://`, `hysteria2://`, `tuic://`.

Default rule template: Loyalsoldier (changeable via `~/.config/vpnkit/config.toml`).
User overlay: `~/.config/mihomo/patch.yaml` is deep-merged on every update.
```

- [ ] **Step 5: commit + tag**
```bash
git add README.md
git commit -m "docs: Phase 2 quickstart and supported formats"
git tag v0.2.0-phase2
```

---

## Self-Review Notes

- Spec §5 (subscription conversion): 9 protocol parsers (T2–T9) + format detection (T10) + group synthesis (T11) — covered.
- Spec §6 (default rule set): reused from Phase 1; patch overlay added (T12); assembled together (T13).
- Spec §8 (REST API): T16 adds `/proxies` + `PutProxy` + delay tests. `/rules` and `/connections` deferred to Phase 3 per phase plan.
- Spec §4.5 (tabs): Profiles tab (T18–T20), Proxies tab (T21–T23). Connections / Logs / Rules / Settings stay stubbed.
- Spec §10 (error handling): subscription parser tolerates per-node failures via `Result.Errors` (T10); fetch returns errors normally; profile update errors surface via `msg.ProfileError`.

No placeholders, all references to types/functions are defined in earlier tasks before consumed.

Phase 2 task count: 24. End state: vpnkit becomes daily-usable — users can add a subscription, switch nodes, and run delay tests entirely in the TUI.

End of Phase 2 plan.
