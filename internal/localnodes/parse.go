package localnodes

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseURI dispatches on the URI's scheme to one of the protocol-specific
// parsers. Names are taken from the URI fragment (#name) when present,
// otherwise a stable fallback derived from server:port is used.
func ParseURI(raw string) (Node, error) {
	if i := strings.Index(raw, "://"); i < 0 {
		return Node{}, errors.New("parse: missing scheme")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Node{}, fmt.Errorf("parse: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "ss":
		return parseSS(u, raw)
	case "vmess":
		return parseVmess(u, raw)
	case "vless":
		return parseVless(u, raw)
	case "trojan":
		return parseTrojan(u, raw)
	case "hysteria2", "hy2":
		return parseHy2(u, raw)
	case "tuic":
		return parseTuic(u, raw)
	default:
		return Node{}, fmt.Errorf("parse: unsupported scheme %q", u.Scheme)
	}
}

func nameOrFallback(u *url.URL) string {
	if u.Fragment != "" {
		// URI fragment is already decoded by url.Parse.
		return u.Fragment
	}
	return u.Host
}

func parseSS(u *url.URL, raw string) (Node, error) {
	// ss://BASE64(method:password)@host:port#name  (SIP002)
	// Some sources use the older ss://BASE64(method:password@host:port)#name form;
	// we cover only SIP002 here (current mihomo standard).
	userInfo := u.User.String()
	if userInfo == "" {
		return Node{}, errors.New("parse(ss): missing userinfo")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(userInfo)
	if err != nil {
		// Some sources pad; try StdEncoding too.
		decoded, err = base64.StdEncoding.DecodeString(userInfo)
		if err != nil {
			return Node{}, fmt.Errorf("parse(ss): bad base64 userinfo: %w", err)
		}
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return Node{}, errors.New("parse(ss): userinfo must be method:password")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, fmt.Errorf("parse(ss): bad port: %w", err)
	}
	return Node{
		Name:   nameOrFallback(u),
		Proto:  "ss",
		Server: u.Hostname(),
		Port:   port,
		Fields: map[string]any{
			"cipher":   parts[0],
			"password": parts[1],
		},
	}, nil
}

func parseVmess(_ *url.URL, raw string) (Node, error) {
	// vmess://BASE64(json) — the JSON is the canonical clash node minus the
	// type/name keys; convert to mihomo-style fields here.
	const prefix = "vmess://"
	if !strings.HasPrefix(raw, prefix) {
		return Node{}, errors.New("parse(vmess): missing prefix")
	}
	b64 := strings.TrimPrefix(raw, prefix)
	if i := strings.IndexAny(b64, "#?"); i >= 0 {
		b64 = b64[:i]
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(b64)
		if err != nil {
			return Node{}, fmt.Errorf("parse(vmess): bad base64: %w", err)
		}
	}
	var raw_ struct {
		PS   string `json:"ps"`
		Add  string `json:"add"`
		Port any    `json:"port"`
		ID   string `json:"id"`
		Aid  any    `json:"aid"`
		Net  string `json:"net"`
		Type string `json:"type"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
	}
	if err := json.Unmarshal(decoded, &raw_); err != nil {
		return Node{}, fmt.Errorf("parse(vmess): bad json: %w", err)
	}
	port, err := anyToInt(raw_.Port)
	if err != nil {
		return Node{}, fmt.Errorf("parse(vmess): bad port: %w", err)
	}
	aid, _ := anyToInt(raw_.Aid)
	fields := map[string]any{
		"uuid":    raw_.ID,
		"alterId": aid,
		"cipher":  "auto",
		"network": raw_.Net,
	}
	if raw_.TLS == "tls" {
		fields["tls"] = true
		if raw_.SNI != "" {
			fields["servername"] = raw_.SNI
		} else if raw_.Host != "" {
			fields["servername"] = raw_.Host
		}
	}
	if raw_.Net == "ws" {
		wsOpts := map[string]any{}
		if raw_.Path != "" {
			wsOpts["path"] = raw_.Path
		}
		if raw_.Host != "" {
			wsOpts["headers"] = map[string]any{"Host": raw_.Host}
		}
		fields["ws-opts"] = wsOpts
	}
	return Node{
		Name:   raw_.PS,
		Proto:  "vmess",
		Server: raw_.Add,
		Port:   port,
		Fields: fields,
	}, nil
}

func anyToInt(v any) (int, error) {
	switch x := v.(type) {
	case float64:
		return int(x), nil
	case int:
		return x, nil
	case string:
		return strconv.Atoi(x)
	default:
		return 0, fmt.Errorf("anyToInt: unsupported %T", v)
	}
}

// stub the other parsers so the package compiles before we implement them.
func parseVless(u *url.URL, raw string) (Node, error)  { return Node{}, errors.New("vless: not implemented yet") }
func parseTrojan(u *url.URL, raw string) (Node, error) { return Node{}, errors.New("trojan: not implemented yet") }
func parseHy2(u *url.URL, raw string) (Node, error)    { return Node{}, errors.New("hy2: not implemented yet") }
func parseTuic(u *url.URL, raw string) (Node, error)   { return Node{}, errors.New("tuic: not implemented yet") }
