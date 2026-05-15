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

var _ = base64.StdEncoding
