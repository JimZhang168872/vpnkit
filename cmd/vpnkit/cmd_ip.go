package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"

	"vpnkit/internal/api"
	"vpnkit/internal/store"
)

const defaultIPInfoURL = "https://ipinfo.io/json"

// runIP fetches ipinfoURL through mihomo's mixed-port proxy.
// If ipinfoURL is empty, defaultIPInfoURL is used.
func runIP(out io.Writer, c *api.Client, st *store.Store, ipinfoURL string, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if ipinfoURL == "" {
		ipinfoURL = defaultIPInfoURL
	}

	cfg, err := c.GetConfigs(ctx)
	if err != nil {
		return fmt.Errorf("mihomo not reachable: %w", err)
	}
	if cfg.MixedPort == 0 {
		return fmt.Errorf("mihomo mixed-port not configured")
	}
	authority := fmt.Sprintf("127.0.0.1:%d", cfg.MixedPort)
	if st != nil && st.Cfg.ProxyUser != "" && st.Cfg.ProxyPass != "" {
		authority = url.UserPassword(st.Cfg.ProxyUser, st.Cfg.ProxyPass).String() + "@" + authority
	}
	proxyURL, _ := url.Parse("http://" + authority)
	client := &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ipinfoURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ipinfo unreachable through proxy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("ipinfo status %d", resp.StatusCode)
	}
	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("decode ipinfo: %w", err)
	}

	via := pickVia(c, ctx)
	info["via"] = via

	if jsonOut {
		return writeJSON(out, info)
	}
	rows := []struct {
		k, v string
	}{
		{"ip", asString(info["ip"])},
		{"country", asString(info["country"])},
		{"region", asString(info["region"])},
		{"city", asString(info["city"])},
		{"org", asString(info["org"])},
		{"via", via},
	}
	for _, r := range rows {
		fmt.Fprintf(out, "%-8s %s\n", r.k, r.v)
	}
	return nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// pickVia returns "<group> → <now>" for the first user-selectable group.
// Returns "" if nothing matches or proxies fetch fails (best-effort).
func pickVia(c *api.Client, ctx context.Context) string {
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return ""
	}
	var names []string
	for n, info := range proxies {
		if isUserSelectableType(info.Type) && len(info.All) > 0 {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	g := proxies[names[0]]
	return fmt.Sprintf("%s → %s", names[0], g.Now)
}
