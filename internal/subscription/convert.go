// Package subscription fetches, detects format, and converts subscriptions into
// mihomo-compatible proxy lists.
package subscription

import (
	"bufio"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/subscription/proto"
)

// Result is the outcome of converting a subscription body.
type Result struct {
	Source  string
	Proxies []proto.Proxy
	Raw     map[string]any
	Errors  []error
}

// Convert dispatches on detected format and returns Result.
func Convert(body []byte) (Result, error) {
	switch Detect(body) {
	case FormatClash:
		return convertClash(body)
	case FormatSIP008:
		px, err := parseSIP008(body)
		return Result{Source: string(FormatSIP008), Proxies: px}, err
	case FormatBase64List:
		dec, _ := tolerantB64(strings.TrimSpace(string(body)))
		return convertList(string(dec), string(FormatBase64List))
	default:
		return convertList(string(body), string(FormatURI))
	}
}

func convertClash(body []byte) (Result, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return Result{}, fmt.Errorf("clash yaml: %w", err)
	}
	r := Result{Source: string(FormatClash), Raw: doc}
	if px, ok := doc["proxies"].([]any); ok {
		for _, x := range px {
			if m, ok := x.(map[string]any); ok {
				r.Proxies = append(r.Proxies, proto.Proxy(m))
			}
		}
	}
	return r, nil
}

func convertList(s, source string) (Result, error) {
	r := Result{Source: source}
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.Contains(line, "://") {
			continue
		}
		p, err := proto.Parse(line)
		if err != nil {
			r.Errors = append(r.Errors, err)
			continue
		}
		r.Proxies = append(r.Proxies, p)
	}
	return r, sc.Err()
}
