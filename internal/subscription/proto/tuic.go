package proto

import (
	"fmt"
	"net/url"
	"strings"
)

func init() { Register("tuic", parseTUIC) }

func parseTUIC(uri string) (Proxy, error) {
	parts, err := splitProxyURI(uri)
	if err != nil {
		return nil, fmt.Errorf("tuic: %w", err)
	}
	// userinfo is "uuid:password"; split on the FIRST ':' so that ':' inside
	// the password stays with the password.
	uuid, pw := parts.UserInfo, ""
	if i := strings.Index(parts.UserInfo, ":"); i >= 0 {
		uuid = parts.UserInfo[:i]
		pw, _ = url.PathUnescape(parts.UserInfo[i+1:])
	}
	q := parts.Query
	p := Proxy{
		"name":     parts.Fragment,
		"type":     "tuic",
		"server":   parts.Host,
		"port":     parts.Port,
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
