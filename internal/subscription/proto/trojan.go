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
