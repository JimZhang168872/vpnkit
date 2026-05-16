// Package env renders shell-specific snippets that export (or unset) proxy
// variables, and manages a matching ~/.netrc entry when proxy auth is enabled.
package env

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Options drives the renderer.
type Options struct {
	Shell   string // bash|zsh|fish; empty = bash
	Port    int    // mihomo mixed-port; default 7890
	User    string // proxy basic-auth user (empty = no auth)
	Pass    string // proxy basic-auth password
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
	authority := fmt.Sprintf("127.0.0.1:%d", o.Port)
	if o.User != "" && o.Pass != "" {
		authority = url.UserPassword(o.User, o.Pass).String() + "@" + authority
	}
	httpURL := "http://" + authority
	socksURL := "socks5h://" + authority
	var b strings.Builder
	switch o.Shell {
	case "fish":
		fmt.Fprintf(&b, "set -gx http_proxy %s\n", httpURL)
		fmt.Fprintf(&b, "set -gx https_proxy %s\n", httpURL)
		fmt.Fprintf(&b, "set -gx all_proxy %s\n", socksURL)
		if o.NoProxy != "" {
			fmt.Fprintf(&b, "set -gx no_proxy %s\n", o.NoProxy)
		}
	default: // bash, zsh
		fmt.Fprintf(&b, "export http_proxy=%s\n", httpURL)
		fmt.Fprintf(&b, "export https_proxy=%s\n", httpURL)
		fmt.Fprintf(&b, "export all_proxy=%s\n", socksURL)
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

// WriteNetrc creates or updates the netrc entry for machine `host` with the
// given login/password. Any existing entry for the same machine is replaced.
// The file is rewritten with mode 0600. Foreign entries are preserved.
func WriteNetrc(path, host, user, pass string) error {
	if path == "" {
		return fmt.Errorf("env: empty netrc path")
	}
	var existing string
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}
	kept := stripNetrcMachine(existing, host)
	if kept != "" && !strings.HasSuffix(kept, "\n") {
		kept += "\n"
	}
	entry := fmt.Sprintf("machine %s login %s password %s\n", host, user, pass)
	out := kept + entry

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return err
	}
	// os.WriteFile only applies the perm on create; if ~/.netrc already
	// existed with e.g. 0644, the password we just wrote would be world-readable.
	return os.Chmod(path, 0o600)
}

// stripNetrcMachine removes every machine block matching `host` from `body`,
// preserving every other line verbatim. The parser is line-oriented and
// conservative: a block begins at a line whose first non-blank token is
// `machine`, `default`, or `macdef`, and ends just before the next such line
// (or EOF). Unknown tokens inside a foreign block (e.g. `account`, `macdef`
// payloads) are kept untouched, so users with non-standard netrc files don't
// lose credentials for other services.
func stripNetrcMachine(body, host string) string {
	lines := strings.Split(body, "\n")
	var out []string
	skip := false
	for _, ln := range lines {
		first := firstToken(ln)
		switch first {
		case "machine":
			rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), "machine"))
			// "machine 127.0.0.1 ..." — match host as first token of rest.
			skip = firstToken(rest) == host
			if skip {
				continue
			}
		case "default", "macdef":
			skip = false
		}
		if !skip {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

// firstToken returns the first whitespace-separated token of s (or "").
func firstToken(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			s = s[i:]
			break
		}
	}
	for i, r := range s {
		if r == ' ' || r == '\t' {
			return s[:i]
		}
	}
	return s
}
