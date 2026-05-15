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
