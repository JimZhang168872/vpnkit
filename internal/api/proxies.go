package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ProxyInfo mirrors one entry in /proxies' "proxies" map.
type ProxyInfo struct {
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

type proxiesResponse struct {
	Proxies map[string]ProxyInfo `json:"proxies"`
}

// GetProxies fetches the /proxies snapshot.
func (c *Client) GetProxies(ctx context.Context) (map[string]ProxyInfo, error) {
	var resp proxiesResponse
	if err := c.do(ctx, http.MethodGet, "/proxies", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Proxies, nil
}

// PutProxy selects `node` as the current member of group `group`.
func (c *Client) PutProxy(ctx context.Context, group, node string) error {
	return c.do(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group),
		map[string]string{"name": node}, nil)
}

// Delay queries a single node's delay (ms) against `testURL` with timeout in ms.
func (c *Client) Delay(ctx context.Context, node, testURL string, timeoutMs int) (int, error) {
	q := url.Values{}
	q.Set("url", testURL)
	q.Set("timeout", fmt.Sprintf("%d", timeoutMs))
	path := "/proxies/" + url.PathEscape(node) + "/delay?" + q.Encode()
	var out struct {
		Delay int `json:"delay"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return 0, err
	}
	return out.Delay, nil
}

// GroupDelay tests every member of a group at once.
func (c *Client) GroupDelay(ctx context.Context, group, testURL string, timeoutMs int) (map[string]int, error) {
	q := url.Values{}
	q.Set("url", testURL)
	q.Set("timeout", fmt.Sprintf("%d", timeoutMs))
	path := "/group/" + url.PathEscape(group) + "/delay?" + q.Encode()
	out := map[string]int{}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

var _ = json.NewDecoder
