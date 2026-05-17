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
	inputs  []textinput.Model
	focused int
	proto   string
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

// supportedProtos is the ordered list for [p] cycling.
var supportedProtos = []string{"ss", "vmess", "vless", "trojan", "hysteria2", "tuic"}

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
	rows := []string{lipgloss.NewStyle().Bold(true).Render("Add Local Node — " + f.proto), ""}
	for i, d := range defs {
		mark := "  "
		if i == f.focused {
			mark = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ ")
		}
		rows = append(rows, mark+d.label+" "+f.inputs[i].View())
	}
	rows = append(rows, "", lipgloss.NewStyle().Faint(true).Render(
		"[Tab/↑↓] navigate  [Enter] save  [Esc] cancel  [p] change proto  [u] URI mode"))
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
	n.Port = p

	if n.Group == "" {
		n.Group = "local"
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
