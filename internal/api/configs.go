package api

import (
	"context"
	"net/http"
)

// Configs mirrors the subset of /configs we use.
type Configs struct {
	Mode      string `json:"mode"`
	LogLevel  string `json:"log-level"`
	MixedPort int    `json:"mixed-port"`
	AllowLAN  bool   `json:"allow-lan"`
	Secret    string `json:"secret"`
}

// GetConfigs fetches /configs.
func (c *Client) GetConfigs(ctx context.Context) (Configs, error) {
	var out Configs
	err := c.do(ctx, http.MethodGet, "/configs", nil, &out)
	return out, err
}
