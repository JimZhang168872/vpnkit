package env

import (
	"strings"
	"testing"
)

func TestRenderBash(t *testing.T) {
	got := Render(Options{Shell: "bash", Port: 7890, NoProxy: "localhost,127.0.0.1"})
	for _, want := range []string{
		"export http_proxy=http://127.0.0.1:7890",
		"export https_proxy=http://127.0.0.1:7890",
		"export all_proxy=socks5h://127.0.0.1:7890",
		"export no_proxy=localhost,127.0.0.1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderFish(t *testing.T) {
	got := Render(Options{Shell: "fish", Port: 7890})
	if !strings.Contains(got, "set -gx http_proxy http://127.0.0.1:7890") {
		t.Errorf("fish output: %s", got)
	}
}

func TestRenderUnset(t *testing.T) {
	got := Render(Options{Shell: "bash", Unset: true})
	for _, want := range []string{"unset http_proxy", "unset https_proxy", "unset all_proxy", "unset no_proxy"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
}
