package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTrafficStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"up": 100, "down": 200}`)
		flusher.Flush()
		fmt.Fprintln(w, `{"up": 300, "down": 400}`)
		flusher.Flush()
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, errCh := c.Traffic(ctx)
	got := []Traffic{}
	for i := 0; i < 2; i++ {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case err := <-errCh:
			t.Fatalf("err: %v", err)
		case <-ctx.Done():
			t.Fatal("timeout")
		}
	}
	if got[0].Up != 100 || got[1].Down != 400 {
		t.Errorf("got %+v", got)
	}
}
