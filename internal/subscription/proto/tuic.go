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
