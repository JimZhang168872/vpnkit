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
