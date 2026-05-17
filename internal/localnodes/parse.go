package localnodes

import (
	"encoding/base64"
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

// stub the other parsers so the package compiles before we implement them.
func parseVmess(u *url.URL, raw string) (Node, error)  { return Node{}, errors.New("vmess: not implemented yet") }
func parseVless(u *url.URL, raw string) (Node, error)  { return Node{}, errors.New("vless: not implemented yet") }
func parseTrojan(u *url.URL, raw string) (Node, error) { return Node{}, errors.New("trojan: not implemented yet") }
func parseHy2(u *url.URL, raw string) (Node, error)    { return Node{}, errors.New("hy2: not implemented yet") }
func parseTuic(u *url.URL, raw string) (Node, error)   { return Node{}, errors.New("tuic: not implemented yet") }
