package subscription

import (
	"encoding/base64"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatClash      Format = "clash"
	FormatSIP008     Format = "sip008"
	FormatBase64List Format = "base64-list"
	FormatURI        Format = "uri"
)

// Detect identifies the subscription's wire format from its bytes.
func Detect(body []byte) Format {
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		return FormatSIP008
	}
	var probe map[string]any
	if err := yaml.Unmarshal(body, &probe); err == nil {
		if _, ok := probe["proxies"]; ok {
			return FormatClash
		}
		if _, ok := probe["proxy-groups"]; ok {
			return FormatClash
		}
	}
	if dec, err := tolerantB64(trimmed); err == nil && strings.Contains(string(dec), "://") {
		return FormatBase64List
	}
	if strings.Contains(trimmed, "://") {
		return FormatURI
	}
	return FormatURI
}

func tolerantB64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.URLEncoding,
		base64.RawStdEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, base64.CorruptInputError(0)
}
