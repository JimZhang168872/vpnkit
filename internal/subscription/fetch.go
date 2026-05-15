package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultUA = "clash-verge/v1.4.0"

// Fetch retrieves a subscription body. ua is optional (defaults to clash-verge UA).
func Fetch(ctx context.Context, url, ua string) ([]byte, error) {
	if ua == "" {
		ua = defaultUA
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription fetch %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}
