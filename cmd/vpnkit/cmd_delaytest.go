package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"vpnkit/internal/api"
)

// defaultDelayTestURL is mihomo's stock health-check URL. 204 No Content
// keeps response payload small so the measurement reflects the proxy hop
// rather than upstream latency from a heavyweight HTML page.
const (
	defaultDelayTestURL   = "https://www.gstatic.com/generate_204"
	defaultDelayTimeoutMs = 5000
)

// runTest probes connectivity through mihomo's /proxies/<name>/delay
// (when node is set) or /group/<name>/delay (when only group is set).
//
// A delay of 0 means the test timed out — mihomo returns the raw int from
// its measurement loop, and we surface "timeout" in text output but keep
// 0 in JSON so machine consumers can decide for themselves.
func runTest(out io.Writer, c *api.Client, group, node, testURL string, timeoutMs int, jsonOut bool) error {
	if group == "" && node == "" {
		return errors.New("test: group or node required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs+2000)*time.Millisecond)
	defer cancel()

	if node != "" {
		d, err := c.Delay(ctx, node, testURL, timeoutMs)
		if err != nil {
			return fmt.Errorf("delay %s: %w", node, err)
		}
		if jsonOut {
			return writeJSON(out, map[string]any{
				"node":       node,
				"delay_ms":   d,
				"url":        testURL,
				"timeout_ms": timeoutMs,
			})
		}
		fmt.Fprintf(out, "%-24s  %s\n", node, formatDelay(d))
		return nil
	}

	// MeasureGroup transparently handles the Selector→url-test pivot and
	// the per-member fallback for groups mihomo doesn't accept on its
	// /group/<name>/delay endpoint. See api.MeasureGroup for the cascade.
	results, err := c.MeasureGroup(ctx, group, testURL, timeoutMs)
	if err != nil {
		return fmt.Errorf("delay %s: %w", group, err)
	}
	if jsonOut {
		return writeJSON(out, map[string]any{
			"group":      group,
			"url":        testURL,
			"timeout_ms": timeoutMs,
			"results":    results,
		})
	}
	names := make([]string, 0, len(results))
	for n := range results {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(out, "  %-24s  %s\n", n, formatDelay(results[n]))
	}
	return nil
}

// formatDelay turns a raw ms value into human text. mihomo encodes timeout
// as 0; anything else is the measured round-trip in milliseconds.
func formatDelay(ms int) string {
	if ms == 0 {
		return "timeout"
	}
	return fmt.Sprintf("%d ms", ms)
}

func dispatchTest(args []string) {
	jsonOut, rest := parseFlags(args)
	testURL := defaultDelayTestURL
	timeoutMs := defaultDelayTimeoutMs
	// In-place flag stripping for --url / --timeout-ms so the surviving
	// positional args are <group> [<node>] in order.
	rest = consumeStringFlag(rest, "--url", &testURL)
	rest = consumeIntFlag(rest, "--timeout-ms", &timeoutMs)

	if len(rest) < 1 {
		dieUserErr("vpnkit test: usage: vpnkit test <group> [<node>] [--url URL] [--timeout-ms MS] [--json]")
	}
	group := rest[0]
	node := ""
	if len(rest) >= 2 {
		node = rest[1]
	}
	c, _, err := loadClient()
	if err != nil {
		dieJSONOrText(jsonOut, "vpnkit test", err)
	}
	if err := runTest(os.Stdout, c, group, node, testURL, timeoutMs, jsonOut); err != nil {
		dieJSONOrText(jsonOut, "vpnkit test", err)
	}
}

// consumeStringFlag removes "--key" "value" or "--key=value" from args and
// writes the value into *dst. Returns the remaining args.
func consumeStringFlag(args []string, key string, dst *string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == key && i+1 < len(args) {
			*dst = args[i+1]
			i++
			continue
		}
		if len(a) > len(key)+1 && a[:len(key)+1] == key+"=" {
			*dst = a[len(key)+1:]
			continue
		}
		out = append(out, a)
	}
	return out
}

// consumeIntFlag is consumeStringFlag for int values; bad values are ignored
// (the default stays in *dst), keeping the flag opt-in and forgiving.
func consumeIntFlag(args []string, key string, dst *int) []string {
	var s string
	out := consumeStringFlag(args, key, &s)
	if s != "" {
		var n int
		_, _ = fmt.Sscanf(s, "%d", &n)
		if n > 0 {
			*dst = n
		}
	}
	return out
}
