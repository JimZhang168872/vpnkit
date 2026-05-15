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
		"name":    raw.PS,
		"type":    "vmess",
		"server":  raw.Add,
		"port":    port,
		"uuid":    raw.ID,
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
