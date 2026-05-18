// local_form.go — Local-node form types, constructors, and renderers.
// Extracted from sources.go (Task 6.1) and extended with the proto-driven
// multi-field form (Task 6.2).
package sources

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/localnodes"
)

// formMode distinguishes the kind of overlay that is currently open.
type formMode int

const (
	formModeURI         formMode = iota // single textinput — paste a proxy URI
	formModeNewGroup                    // single textinput — create a new group
	formModeRenameGroup                 // single textinput — rename existing group
	formModeNodeFields                  // multi-field proto-driven form
)

// localNodeForm is shared by URI / new-group / rename-group / multi-field inputs.
type localNodeForm struct {
	mode         formMode
	input        textinput.Model // used by URI / new-group / rename-group modes
	defaultGroup string
	oldName      string // rename mode: current name being changed

	// formModeNodeFields:
	inputs      []textinput.Model
	focused     int
	proto       string
	editingName string // non-empty when editing an existing node (used to call Update vs Add)
}

// ─── Single-input constructors ────────────────────────────────────────────────

func newLocalNodeURIForm() *localNodeForm {
	ti := newTextInput("proxy URI (e.g. vmess://...)", "")
	ti.Focus()
	return &localNodeForm{mode: formModeURI, input: ti}
}

func newGroupNameForm() *localNodeForm {
	ti := newTextInput("group name (e.g. home, office)", "")
	ti.Focus()
	return &localNodeForm{mode: formModeNewGroup, input: ti}
}

func newGroupRenameForm(current string) *localNodeForm {
	ti := newTextInput("new group name", current)
	ti.Focus()
	return &localNodeForm{mode: formModeRenameGroup, input: ti, oldName: current}
}

// ─── Multi-field form definitions ─────────────────────────────────────────────

// fieldDef describes one input field in the multi-field local-node form.
type fieldDef struct {
	key         string // logical key, e.g. "password"
	label       string // human label, e.g. "Password:"
	placeholder string
	intField    bool // render as number, validated on save
}

// commonFields appear at the top of every proto (after the proto field itself).
var commonFields = []fieldDef{
	{key: "name", label: "Name:", placeholder: "any name you want"},
	{key: "group", label: "Group:", placeholder: "home / office / local"},
	{key: "server", label: "Server:", placeholder: "1.2.3.4 or host.example.com"},
	{key: "port", label: "Port:", placeholder: "443", intField: true},
}

// protoFields maps proto string → its specific fields (after commonFields).
var protoFields = map[string][]fieldDef{
	"ss": {
		{key: "cipher", label: "Cipher:", placeholder: "aes-256-gcm | chacha20-ietf-poly1305 | ..."},
		{key: "password", label: "Password:", placeholder: ""},
	},
	"vmess": {
		{key: "uuid", label: "UUID:", placeholder: ""},
		{key: "alterId", label: "AlterId:", placeholder: "0", intField: true},
		{key: "cipher", label: "Cipher:", placeholder: "auto"},
		{key: "network", label: "Network:", placeholder: "tcp | ws | grpc"},
		{key: "ws-opts.host", label: "WS Host:", placeholder: "(only if Network=ws)"},
		{key: "ws-opts.path", label: "WS Path:", placeholder: "/path"},
		{key: "tls", label: "TLS (true/false):", placeholder: "false"},
		{key: "servername", label: "TLS SNI:", placeholder: ""},
	},
	"vless": {
		{key: "uuid", label: "UUID:", placeholder: ""},
		{key: "network", label: "Network:", placeholder: "tcp | ws | grpc"},
		{key: "flow", label: "Flow:", placeholder: "xtls-rprx-vision (optional)"},
		{key: "tls", label: "TLS (true/false):", placeholder: "false"},
		{key: "servername", label: "TLS SNI:", placeholder: ""},
		{key: "reality-opts.public-key", label: "Reality PubKey:", placeholder: ""},
		{key: "reality-opts.short-id", label: "Reality ShortID:", placeholder: ""},
	},
	"trojan": {
		{key: "password", label: "Password:", placeholder: ""},
		{key: "sni", label: "SNI:", placeholder: ""},
		{key: "alpn", label: "ALPN (csv):", placeholder: "h2,http/1.1"},
		{key: "skip-cert-verify", label: "Skip cert verify (true/false):", placeholder: "false"},
	},
	"hysteria2": {
		{key: "password", label: "Password:", placeholder: ""},
		{key: "sni", label: "SNI:", placeholder: ""},
		{key: "up", label: "Up (Mbps int):", placeholder: "100", intField: true},
		{key: "down", label: "Down (Mbps int):", placeholder: "200", intField: true},
		{key: "obfs", label: "Obfs:", placeholder: "salamander (optional)"},
		{key: "obfs-password", label: "Obfs Password:", placeholder: ""},
		{key: "skip-cert-verify", label: "Skip cert verify (true/false):", placeholder: "false"},
	},
	"tuic": {
		{key: "uuid", label: "UUID:", placeholder: ""},
		{key: "password", label: "Password:", placeholder: ""},
		{key: "sni", label: "SNI:", placeholder: ""},
		{key: "congestion-controller", label: "Congestion:", placeholder: "bbr | cubic"},
		{key: "udp-relay-mode", label: "UDP Relay Mode:", placeholder: "native | quic"},
		{key: "alpn", label: "ALPN (csv):", placeholder: "h3"},
	},
}

// supportedProtos is the ordered list for ←/→ cycling on the Proto field.
var supportedProtos = []string{"ss", "vmess", "vless", "trojan", "hysteria2", "tuic"}

// cycleProto returns a new form whose proto is shifted by dir (+1 or -1)
// relative to f.proto. Values for keys present in both old and new proto's
// field set are carried over; proto-specific fields are reset to defaults.
// editingName is preserved so commit still calls Update() for edit flows.
func cycleProto(f *localNodeForm, dir int) *localNodeForm {
	idx := 0
	for i, p := range supportedProtos {
		if p == f.proto {
			idx = i
			break
		}
	}
	next := supportedProtos[(idx+dir+len(supportedProtos))%len(supportedProtos)]
	nf := newLocalNodeFieldForm(next, f.defaultGroup)
	nf.editingName = f.editingName

	oldDefs := f.formFieldDefs()
	oldValues := make(map[string]string, len(oldDefs))
	for i, d := range oldDefs {
		oldValues[d.key] = f.inputs[i].Value()
	}
	newDefs := nf.formFieldDefs()
	for i, d := range newDefs {
		if d.key == "proto" {
			continue
		}
		if v, ok := oldValues[d.key]; ok {
			nf.inputs[i].SetValue(v)
		}
	}
	nf.focused = 0
	for i := range nf.inputs {
		nf.inputs[i].Blur()
	}
	nf.inputs[0].Focus()
	return nf
}

// viaField is appended after proto-specific fields.
var viaField = fieldDef{
	key:         "via",
	label:       "Via (optional):",
	placeholder: "doge-auto, doge:HK-A, ... (empty = none)",
}

// ─── Multi-field constructor ───────────────────────────────────────────────────

// newLocalNodeFieldForm builds a multi-field form for a given proto.
// defaultGroup pre-fills the Group field.
func newLocalNodeFieldForm(proto, defaultGroup string) *localNodeForm {
	defs := append([]fieldDef{
		{key: "proto", label: "Proto:", placeholder: proto},
	}, commonFields...)
	defs = append(defs, protoFields[proto]...)
	defs = append(defs, viaField)

	inputs := make([]textinput.Model, len(defs))
	for i, d := range defs {
		ti := newTextInput(d.placeholder, "")
		if d.key == "proto" {
			ti.SetValue(proto)
		}
		if d.key == "group" {
			ti.SetValue(defaultGroup)
		}
		// Mask credential fields with bullets so over-the-shoulder
		// reading of a TUI session doesn't leak passwords/UUIDs. Users
		// who need to verify the value can still copy from the store
		// via `vpnkit local-nodes list --json` over a private channel.
		if isCredentialField(d.key) {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		inputs[i] = ti
	}
	inputs[1].Focus() // skip proto (index 0), focus on Name (index 1)

	return &localNodeForm{
		mode:         formModeNodeFields,
		defaultGroup: defaultGroup,
		proto:        proto,
		inputs:       inputs,
		focused:      1,
	}
}

// newLocalNodeFieldFormFromNode builds a multi-field edit form pre-filled
// with the values from an existing node. Sets editingName so commit calls
// Update() instead of Add(). Proto is taken from the node; the form's field
// set matches the node's proto.
func newLocalNodeFieldFormFromNode(n localnodes.Node) *localNodeForm {
	f := newLocalNodeFieldForm(n.Proto, n.Group)
	f.editingName = n.Name
	defs := f.formFieldDefs()
	for i, d := range defs {
		v := valueForFieldFromNode(n, d)
		if v != "" {
			f.inputs[i].SetValue(v)
		} else if d.key == "proto" {
			// Always keep proto value visible even if blank check skipped it.
			f.inputs[i].SetValue(n.Proto)
		}
	}
	return f
}

// valueForFieldFromNode reads one fieldDef's value out of a Node and returns
// the string representation suitable for a textinput.
func valueForFieldFromNode(n localnodes.Node, d fieldDef) string {
	switch d.key {
	case "proto":
		return n.Proto
	case "name":
		return n.Name
	case "group":
		return n.Group
	case "server":
		return n.Server
	case "port":
		if n.Port > 0 {
			return strconv.Itoa(n.Port)
		}
		return ""
	case "via":
		return n.Via
	}
	// Nested keys like "ws-opts.host" — index into the outer map.
	if strings.Contains(d.key, ".") {
		parts := strings.SplitN(d.key, ".", 2)
		outer, inner := parts[0], parts[1]
		sub, _ := n.Fields[outer].(map[string]any)
		if sub == nil {
			return ""
		}
		s, _ := sub[inner].(string)
		return s
	}
	v, ok := n.Fields[d.key]
	if !ok {
		return ""
	}
	switch d.key {
	case "tls", "skip-cert-verify":
		if b, ok := v.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
	case "alpn":
		switch arr := v.(type) {
		case []string:
			return strings.Join(arr, ",")
		case []any:
			ss := make([]string, len(arr))
			for i, e := range arr {
				ss[i] = fmt.Sprint(e)
			}
			return strings.Join(ss, ",")
		}
	case "up", "down":
		if s, ok := v.(string); ok {
			return strings.TrimSpace(strings.TrimSuffix(s, "Mbps"))
		}
	}
	if d.intField {
		if i, ok := v.(int); ok {
			return strconv.Itoa(i)
		}
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// formFieldDefs returns the ordered field definitions for this form's proto.
func (f *localNodeForm) formFieldDefs() []fieldDef {
	defs := append([]fieldDef{{key: "proto", label: "Proto:", placeholder: f.proto}}, commonFields...)
	defs = append(defs, protoFields[f.proto]...)
	defs = append(defs, viaField)
	return defs
}

// ─── Renderers ────────────────────────────────────────────────────────────────

// renderLocalNodeForm dispatches to the appropriate renderer based on mode.
func renderLocalNodeForm(f *localNodeForm) string {
	switch f.mode {
	case formModeNewGroup:
		return lipgloss.NewStyle().Bold(true).Render("New Local Group") + "\n\n" +
			"  " + f.input.View() + "\n\n" +
			lipgloss.NewStyle().Faint(true).Render("[Enter] create  [Esc] cancel")
	case formModeRenameGroup:
		return lipgloss.NewStyle().Bold(true).Render("Rename Local Group") + "\n\n" +
			"  " + f.input.View() + "\n\n" +
			lipgloss.NewStyle().Faint(true).Render("[Enter] rename  [Esc] cancel")
	case formModeNodeFields:
		return renderLocalNodeFieldForm(f)
	default: // formModeURI
		return lipgloss.NewStyle().Bold(true).Render("Add Local Node (URI)") + "\n\n" +
			"  Enter proxy URI:\n  " + f.input.View() + "\n\n" +
			lipgloss.NewStyle().Faint(true).Render("[Enter] add  [Esc] cancel")
	}
}

// renderLocalNodeFieldForm renders the proto-driven multi-field form.
func renderLocalNodeFieldForm(f *localNodeForm) string {
	defs := f.formFieldDefs()
	title := "Add Local Node — " + f.proto
	if f.editingName != "" {
		title = "Edit Local Node — " + f.proto + " (" + f.editingName + ")"
	}
	rows := []string{lipgloss.NewStyle().Bold(true).Render(title), ""}
	for i, d := range defs {
		mark := "  "
		if i == f.focused {
			mark = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ ")
		}
		rows = append(rows, mark+d.label+" "+f.inputs[i].View())
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(
		"[Tab/↑↓] navigate  [Enter] save  [Esc] cancel  [←→ on Proto] cycle proto"))
	return strings.Join(rows, "\n")
}

// ─── URI helper ───────────────────────────────────────────────────────────────

// addNodeFromURI parses a proxy URI and adds it to the local nodes manager.
// defaultGroup sets node.Group when non-empty; falls back to "local".
func addNodeFromURI(pl PipelineFace, uri, defaultGroup string) error {
	n, err := localnodes.ParseURI(uri)
	if err != nil {
		return err
	}
	if defaultGroup != "" {
		n.Group = defaultGroup
	} else {
		n.Group = "local"
	}
	return pl.LocalNodes().Add(n)
}

// ─── Save logic ───────────────────────────────────────────────────────────────

// commitFieldForm builds a localnodes.Node from the multi-field form values.
// Returns an error if required fields are missing or non-integer int fields
// contain non-numeric text.
func (f *localNodeForm) commitFieldForm() (localnodes.Node, error) {
	defs := f.formFieldDefs()
	values := make(map[string]string, len(defs))
	for i, d := range defs {
		values[d.key] = strings.TrimSpace(f.inputs[i].Value())
	}

	n := localnodes.Node{
		Name:   values["name"],
		Group:  values["group"],
		Via:    values["via"],
		Proto:  f.proto,
		Server: values["server"],
		Fields: map[string]any{},
	}

	if n.Name == "" || n.Server == "" || values["port"] == "" {
		return n, fmt.Errorf("name, server, and port are required")
	}

	p, err := strconv.Atoi(values["port"])
	if err != nil {
		return n, fmt.Errorf("port must be int: %w", err)
	}
	if p < 1 || p > 65535 {
		return n, fmt.Errorf("port %d out of range (must be 1-65535)", p)
	}
	n.Port = p

	if n.Group == "" {
		n.Group = "local"
	}

	// Protocol-specific required fields. Without these checks the form
	// saves a node that mihomo will refuse to dial (e.g. an ss node with
	// no password just produces "unable to authenticate"). Catch it at
	// form-commit time so the user gets a meaningful error not a
	// downstream cryptic mihomo log.
	for _, req := range protoRequiredFields(f.proto) {
		if strings.TrimSpace(values[req]) == "" {
			return n, fmt.Errorf("%s requires %s", f.proto, req)
		}
	}

	for _, d := range defs {
		switch d.key {
		case "proto", "name", "group", "server", "port", "via":
			continue
		}
		v := values[d.key]
		if v == "" {
			continue
		}
		if d.intField {
			pv, perr := strconv.Atoi(v)
			if perr != nil {
				return n, fmt.Errorf("%s must be int: %w", d.key, perr)
			}
			if d.key == "up" || d.key == "down" {
				n.Fields[d.key] = fmt.Sprintf("%d Mbps", pv)
			} else {
				n.Fields[d.key] = pv
			}
			continue
		}
		if d.key == "tls" || d.key == "skip-cert-verify" {
			n.Fields[d.key] = v == "true" || v == "1" || v == "yes"
			continue
		}
		if strings.Contains(d.key, ".") {
			parts := strings.SplitN(d.key, ".", 2)
			outer, inner := parts[0], parts[1]
			sub, _ := n.Fields[outer].(map[string]any)
			if sub == nil {
				sub = map[string]any{}
			}
			sub[inner] = v
			n.Fields[outer] = sub
			continue
		}
		if d.key == "alpn" {
			n.Fields[d.key] = strings.Split(v, ",")
			continue
		}
		n.Fields[d.key] = v
	}
	return n, nil
}

// protoRequiredFields returns the form-field keys that must be non-empty
// for a given protocol. Mihomo dial-time validation produces opaque
// errors ("authentication failed") when these are missing, so catch
// at form-commit. Keys here match the form's field-def keys (see
// formFieldDefs in this file).
func protoRequiredFields(proto string) []string {
	switch proto {
	case "ss":
		return []string{"cipher", "password"}
	case "vmess":
		return []string{"uuid"}
	case "vless":
		return []string{"uuid"}
	case "trojan":
		return []string{"password"}
	case "hysteria2", "hy2":
		return []string{"password"}
	case "tuic":
		return []string{"uuid", "password"}
	}
	return nil
}

// isCredentialField reports whether a form-field key holds a value that
// shouldn't be rendered in plaintext on screen. Mirrors the keys that
// providers expect to be private; passwords, UUIDs (which are pre-shared
// secrets in mihomo's vmess/vless), and obfuscation passwords all
// qualify. Cipher / SNI / server are display-OK because they don't
// authenticate connections.
func isCredentialField(key string) bool {
	switch key {
	case "password", "obfs-password", "uuid":
		return true
	}
	return false
}
