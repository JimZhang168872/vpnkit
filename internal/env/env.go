// Package env renders shell-specific snippets that export (or unset) proxy variables.
package env

import (
	"fmt"
	"strings"
)

// Options drives the renderer.
type Options struct {
	Shell   string // bash|zsh|fish; empty = bash
	Port    int    // mihomo mixed-port; default 7890
	NoProxy string // optional no_proxy value
	Unset   bool   // emit unset/erase instead of export/set
}

// Render returns a snippet suitable for `eval "$(vpnkit env)"`.
func Render(o Options) string {
	if o.Port == 0 {
		o.Port = 7890
	}
	if o.Shell == "" {
		o.Shell = "bash"
	}
	if o.Unset {
		return renderUnset(o.Shell)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", o.Port)
	socks := fmt.Sprintf("socks5h://127.0.0.1:%d", o.Port)
	var b strings.Builder
	switch o.Shell {
	case "fish":
		fmt.Fprintf(&b, "set -gx http_proxy %s\n", url)
		fmt.Fprintf(&b, "set -gx https_proxy %s\n", url)
		fmt.Fprintf(&b, "set -gx all_proxy %s\n", socks)
		if o.NoProxy != "" {
			fmt.Fprintf(&b, "set -gx no_proxy %s\n", o.NoProxy)
		}
	default: // bash, zsh
		fmt.Fprintf(&b, "export http_proxy=%s\n", url)
		fmt.Fprintf(&b, "export https_proxy=%s\n", url)
		fmt.Fprintf(&b, "export all_proxy=%s\n", socks)
		if o.NoProxy != "" {
			fmt.Fprintf(&b, "export no_proxy=%s\n", o.NoProxy)
		}
	}
	return b.String()
}

func renderUnset(shell string) string {
	vars := []string{"http_proxy", "https_proxy", "all_proxy", "no_proxy"}
	var b strings.Builder
	for _, v := range vars {
		switch shell {
		case "fish":
			fmt.Fprintf(&b, "set -e %s\n", v)
		default:
			fmt.Fprintf(&b, "unset %s\n", v)
		}
	}
	return b.String()
}
