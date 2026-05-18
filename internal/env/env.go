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
	Shell     string // bash|zsh|fish; empty = bash
	Port      int    // mihomo mixed-port; default 7890
	User      string // proxy basic-auth user (empty = no auth)
	Pass      string // proxy basic-auth password
	NoProxy   string // optional no_proxy value
	Unset     bool   // emit unset/erase instead of export/set
	Functions bool   // emit proxy_on / proxy_off function definitions
}

// Render returns a snippet suitable for `eval "$(vpnkit env)"`.
//
// In Functions mode it emits shell function DEFINITIONS — `proxy_on` /
// `proxy_off` — which the user appends to their rc file once and then
// invokes by name. The functions internally `eval` `vpnkit env` so they
// always pick up the current store creds, not a frozen snapshot.
func Render(o Options) string {
	if o.Port == 0 {
		o.Port = 7890
	}
	if o.Shell == "" {
		o.Shell = "bash"
	}
	if o.Functions {
		return renderFunctions(o.Shell)
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
	// Two cases per variable so Go programs (http.ProxyFromEnvironment) and
	// uppercase-only readers also pick the proxy up. Cost is 4 extra lines.
	pairs := []struct {
		name string
		val  string
	}{
		{"http_proxy", httpURL}, {"HTTP_PROXY", httpURL},
		{"https_proxy", httpURL}, {"HTTPS_PROXY", httpURL},
		{"all_proxy", socksURL}, {"ALL_PROXY", socksURL},
	}
	if o.NoProxy != "" {
		pairs = append(pairs,
			struct {
				name string
				val  string
			}{"no_proxy", o.NoProxy},
			struct {
				name string
				val  string
			}{"NO_PROXY", o.NoProxy},
		)
	}
	var b strings.Builder
	for _, p := range pairs {
		// Single-quote the value so shell metacharacters (`$`, backtick,
		// etc.) inside the URL-encoded basic-auth pass aren't expanded
		// when the user runs `eval "$(vpnkit env)"`. url.UserPassword
		// only encodes URL-reserved chars, NOT shell-active ones, so a
		// password like `f$ss` would silently lose the `$ss` portion
		// (shell expands $ss to "").
		quoted := shellQuote(p.val, o.Shell)
		switch o.Shell {
		case "fish":
			fmt.Fprintf(&b, "set -gx %s %s\n", p.name, quoted)
		default:
			fmt.Fprintf(&b, "export %s=%s\n", p.name, quoted)
		}
	}
	return b.String()
}

// shellQuote wraps val in single quotes for bash/zsh and POSIX-compatible
// shells. Any embedded single quote becomes `'\''` (close-quote, escaped
// quote, reopen). fish uses the same single-quote behavior. Output is
// always safe to embed verbatim in `export NAME=...` / `set -gx NAME ...`.
func shellQuote(val, shell string) string {
	// Single-quote escaping is uniform across bash/zsh/fish for this
	// pattern. fish actually accepts double-quotes with `\$` escaping
	// too, but single-quote is simpler and works in every case we emit.
	if !strings.ContainsAny(val, "'") {
		return "'" + val + "'"
	}
	return "'" + strings.ReplaceAll(val, "'", `'\''`) + "'"
}

func renderUnset(shell string) string {
	vars := []string{
		"http_proxy", "HTTP_PROXY",
		"https_proxy", "HTTPS_PROXY",
		"all_proxy", "ALL_PROXY",
		"no_proxy", "NO_PROXY",
	}
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

// renderFunctions emits proxy_on / proxy_off as shell function definitions.
// Suggested use:
//
//	vpnkit env --shell zsh --functions >> ~/.zshrc
//
// Then in any new shell:  proxy_on   (turn on)   /   proxy_off  (turn off)
func renderFunctions(shell string) string {
	switch shell {
	case "fish":
		return `function proxy_on
  eval (vpnkit env --shell fish)
  echo "🟢 proxy on"
end
function proxy_off
  eval (vpnkit env --shell fish --unset)
  echo "🔴 proxy off"
end
`
	default: // bash, zsh
		return `proxy_on() {
  eval "$(vpnkit env --shell ` + shell + `)"
  echo "🟢 proxy on"
}
proxy_off() {
  eval "$(vpnkit env --shell ` + shell + ` --unset)"
  echo "🔴 proxy off"
}
`
	}
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
