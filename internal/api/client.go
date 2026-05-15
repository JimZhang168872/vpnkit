// Package api talks to mihomo's external-controller HTTP/SSE/WS endpoints.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a thread-safe mihomo external-controller client.
type Client struct {
	BaseURL string
	Secret  string
	HTTP    *http.Client
}

// New builds a Client. baseURL like "http://127.0.0.1:9090". secret optional.
func New(baseURL, secret string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		BaseURL: baseURL,
		Secret:  secret,
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
}

// VersionInfo mirrors mihomo's /version response.
type VersionInfo struct {
	Version string `json:"version"`
	Meta    bool   `json:"meta"`
}

// Version queries /version.
func (c *Client) Version(ctx context.Context) (VersionInfo, error) {
	var v VersionInfo
	err := c.do(ctx, http.MethodGet, "/version", nil, &v)
	return v, err
}

// SetMode PATCHes /configs to switch mihomo's mode: rule|global|direct.
func (c *Client) SetMode(ctx context.Context, mode string) error {
	body := map[string]string{"mode": mode}
	return c.do(ctx, http.MethodPatch, "/configs", body, nil)
}

// ReloadConfig PUTs /configs with a path (mihomo reloads from disk).
func (c *Client) ReloadConfig(ctx context.Context, path string) error {
	body := map[string]string{"path": path}
	return c.do(ctx, http.MethodPut, "/configs", body, nil)
}

// do performs a JSON HTTP request, decoding into `into` if non-nil.
func (c *Client) do(ctx context.Context, method, path string, body any, into any) error {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mihomo %s %s: %d %s", method, path, resp.StatusCode, string(bytes.TrimSpace(buf)))
	}
	if into != nil {
		return json.NewDecoder(resp.Body).Decode(into)
	}
	return nil
}
