package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestConnectionsStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "bye")
		payload, _ := json.Marshal(map[string]any{
			"downloadTotal": 1024,
			"uploadTotal":   2048,
			"connections": []map[string]any{
				{"id": "abc", "metadata": map[string]any{"host": "example.com", "destinationPort": "443"}, "rule": "Match", "chains": []string{"🚀 Proxy"}, "upload": 100, "download": 200},
			},
		})
		_ = c.Write(r.Context(), websocket.MessageText, payload)
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(strings.Replace(srv.URL, "http://", "http://", 1), "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, errCh := c.Connections(ctx)
	select {
	case snap := <-ch:
		if snap.DownloadTotal != 1024 || len(snap.Connections) != 1 {
			t.Errorf("snap: %+v", snap)
		}
		if snap.Connections[0].Host != "example.com" {
			t.Errorf("host: %s", snap.Connections[0].Host)
		}
	case err := <-errCh:
		t.Fatalf("err: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}
