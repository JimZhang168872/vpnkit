package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"

	"vpnkit/internal/netx"
)

// Traffic is one /traffic sample from mihomo (bytes per second since last tick).
type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// Traffic opens an SSE-ish line-delimited JSON stream of /traffic and pushes events
// to the returned channel. Errors land on errCh and close both channels.
// The stream stops when ctx is cancelled.
func (c *Client) Traffic(ctx context.Context) (<-chan Traffic, <-chan error) {
	out := make(chan Traffic, 16)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		// Control-plane: bypass env proxy. Timeout=0 because the stream lives
		// as long as ctx is alive.
		client := netx.NoProxyClient(0)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/traffic", nil)
		if err != nil {
			errCh <- err
			return
		}
		if c.Secret != "" {
			req.Header.Set("Authorization", "Bearer "+c.Secret)
		}
		resp, err := client.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 4096), 1<<20)
		for scanner.Scan() {
			var t Traffic
			if err := json.Unmarshal(scanner.Bytes(), &t); err != nil {
				continue
			}
			select {
			case out <- t:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()
	return out, errCh
}
