package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFlagsExtractsJSON(t *testing.T) {
	json, rest := parseFlags([]string{"--json", "Proxy", "HK-01"})
	if !json {
		t.Error("expected json=true")
	}
	if len(rest) != 2 || rest[0] != "Proxy" || rest[1] != "HK-01" {
		t.Errorf("rest=%v", rest)
	}
}

func TestParseFlagsJSONLast(t *testing.T) {
	json, rest := parseFlags([]string{"Proxy", "HK-01", "--json"})
	if !json {
		t.Error("expected json=true")
	}
	if len(rest) != 2 {
		t.Errorf("rest=%v", rest)
	}
}

func TestParseFlagsNoJSON(t *testing.T) {
	json, rest := parseFlags([]string{"Proxy"})
	if json {
		t.Error("expected json=false")
	}
	if len(rest) != 1 {
		t.Errorf("rest=%v", rest)
	}
}

func TestRenderTableAlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	renderTable(&buf, []string{"GROUP", "TYPE"}, [][]string{
		{"🚀 Proxy", "Selector"},
		{"♻️ Auto", "URLTest"},
	})
	out := buf.String()
	if !strings.Contains(out, "GROUP") || !strings.Contains(out, "TYPE") {
		t.Errorf("missing headers: %s", out)
	}
	if !strings.Contains(out, "🚀 Proxy") || !strings.Contains(out, "Selector") {
		t.Errorf("missing rows: %s", out)
	}
}

func TestWriteJSONCompactWithNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, map[string]any{"a": 1, "b": "x"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected trailing newline: %q", out)
	}
	if !strings.Contains(out, `"a":1`) || !strings.Contains(out, `"b":"x"`) {
		t.Errorf("output: %s", out)
	}
}
