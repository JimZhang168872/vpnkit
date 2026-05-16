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
	parts, err := splitProxyURI(uri)
	if err != nil {
		return nil, fmt.Errorf("hysteria2: %w", err)
	}
	password, _ := url.PathUnescape(parts.UserInfo)
	p := Proxy{
		"name":     parts.Fragment,
		"type":     "hysteria2",
		"server":   parts.Host,
		"port":     parts.Port,
		"password": password,
	}
	if v := parts.Query.Get("obfs"); v != "" {
		p["obfs"] = v
		if pwd := parts.Query.Get("obfs-password"); pwd != "" {
			p["obfs-password"] = pwd
		}
	}
	if sni := parts.Query.Get("sni"); sni != "" {
		p["sni"] = sni
	}
	if parts.Query.Get("insecure") == "1" {
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
