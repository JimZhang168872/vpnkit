package api

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/coder/websocket"
	"vpnkit/internal/netx"
)

// ConnectionsSnapshot is one /connections tick.
type ConnectionsSnapshot struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
}

// Connection is one active connection entry.
type Connection struct {
	ID       string
	Host     string
	Port     string
	Network  string
	Type     string
	Rule     string
	Chains   []string
	Upload   int64
	Download int64
}

type rawConnection struct {
	ID       string          `json:"id"`
	Metadata json.RawMessage `json:"metadata"`
	Rule     string          `json:"rule"`
	Chains   []string        `json:"chains"`
	Upload   int64           `json:"upload"`
	Download int64           `json:"download"`
}

type rawConnsResp struct {
	DownloadTotal int64           `json:"downloadTotal"`
	UploadTotal   int64           `json:"uploadTotal"`
	Connections   []rawConnection `json:"connections"`
}

type rawMetadata struct {
	Host    string `json:"host"`
	DstIP   string `json:"destinationIP"`
	DstPort string `json:"destinationPort"`
	Network string `json:"network"`
	Type    string `json:"type"`
}

// Connections opens a WebSocket to /connections and emits snapshots until ctx cancels.
func (c *Client) Connections(ctx context.Context) (<-chan ConnectionsSnapshot, <-chan error) {
	out := make(chan ConnectionsSnapshot, 8)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		wsURL := strings.Replace(c.BaseURL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		header := map[string][]string{}
		if c.Secret != "" {
			header["Authorization"] = []string{"Bearer " + c.Secret}
		}
		// Control-plane: explicit no-proxy HTTPClient — coder/websocket would
		// otherwise use http.DefaultTransport which honors HTTP_PROXY env.
		conn, _, err := websocket.Dial(ctx, wsURL+"/connections", &websocket.DialOptions{
			HTTPHeader: header,
			HTTPClient: netx.NoProxyClient(0),
		})
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				if ctx.Err() == nil {
					errCh <- err
				}
				return
			}
			var raw rawConnsResp
			if err := json.Unmarshal(data, &raw); err != nil {
				continue
			}
			snap := ConnectionsSnapshot{
				DownloadTotal: raw.DownloadTotal,
				UploadTotal:   raw.UploadTotal,
			}
			for _, rc := range raw.Connections {
				var meta rawMetadata
				_ = json.Unmarshal(rc.Metadata, &meta)
				host := meta.Host
				if host == "" {
					host = meta.DstIP
				}
				snap.Connections = append(snap.Connections, Connection{
					ID:       rc.ID,
					Host:     host,
					Port:     meta.DstPort,
					Network:  meta.Network,
					Type:     meta.Type,
					Rule:     rc.Rule,
					Chains:   rc.Chains,
					Upload:   rc.Upload,
					Download: rc.Download,
				})
			}
			select {
			case out <- snap:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errCh
}

// CloseConnection sends DELETE /connections/{id} to mihomo.
func (c *Client) CloseConnection(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/connections/"+id, nil, nil)
}
