package subscription

import (
	"encoding/json"
	"fmt"

	"vpnkit/internal/subscription/proto"
)

func parseSIP008(body []byte) ([]proto.Proxy, error) {
	var doc struct {
		Version int              `json:"version"`
		Servers []map[string]any `json:"servers"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("sip008: json: %w", err)
	}
	out := make([]proto.Proxy, 0, len(doc.Servers))
	for _, s := range doc.Servers {
		p := proto.Proxy{
			"type":     "ss",
			"server":   s["server"],
			"port":     toInt(s["server_port"]),
			"cipher":   s["method"],
			"password": s["password"],
		}
		if r, ok := s["remarks"].(string); ok {
			p["name"] = r
		}
		out = append(out, p)
	}
	return out, nil
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}
