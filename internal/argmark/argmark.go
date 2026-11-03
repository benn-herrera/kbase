// Package argmark is the one marker by which text names a tool argument or
// holds a kbase call: the marked form, which each surface spells as it takes
// the argument — --name, <name> or a kbase command line at the command line;
// name, or the tool's call, over MCP. It imports no kbase package, so every
// producer of text an MCP reader can receive reaches it.
package argmark

import (
	"bytes"
	"encoding/json"
	"strings"
)

// A marker is delimited by two private-use code points, and its kind is the
// byte after the opening one. Spelling rewrites only text kbase composes —
// help, refusals, remedies — and never KB content a result passes through,
// which may hold the same code points and is written as it stands.
const (
	open           = string(rune(0xE000))
	close          = string(rune(0xE001))
	flagKind       = '-'
	flagValueKind  = '='
	positionalKind = '<'
	callKind       = '!'
)

// Flag is the marked form of the flag name: --name at the command line, name
// over MCP.
func Flag(name string) string { return open + string(flagKind) + name + close }

// Positional is the marked form of the positional argument name: <name> at
// the command line, as its usage line spells it, and name over MCP.
func Positional(name string) string { return open + string(positionalKind) + name + close }

// FlagValue is the marked form of the flag name given value: --name value at
// the command line, name: value over MCP.
func FlagValue(name, value string) string {
	return open + string(flagValueKind) + name + " " + value + close
}

// Call is the marked form of a call of command with args, its tool's
// arguments mapping: a positional's value under its name, a flag's under its
// own. args holds only what JSON encodes — strings, bools, numbers, lists of
// them, and lists of mappings, as a write batch's entries are.
func Call(command string, args map[string]any) string {
	if args == nil {
		args = map[string]any{}
	}
	return call(command, args)
}

// Recorded is the marked form of a call recorded as the words after its
// command, which the spelling reads with the command's own flags.
func Recorded(command string, argv []string) string {
	if argv == nil {
		argv = []string{}
	}
	return call(command, argv)
}

// call panics where args does not encode: only a value no producer passes,
// a channel or a NaN, fails to.
func call(command string, args any) string {
	text, err := JSON(args)
	if err != nil {
		panic("argmark: a call's arguments are no JSON value: " + err.Error())
	}
	return open + string(callKind) + command + " " + text + close
}

// JSON is v as compact JSON, its object keys sorted and nothing HTML-escaped.
func JSON(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// Argument is a tool argument a text names: a flag, or a positional.
type Argument struct {
	Name       string
	Positional bool
}

// Invocation is a call a text holds: its command, and its arguments as a
// mapping, or, where it was recorded, as the words after the command.
type Invocation struct {
	Command string
	Args    map[string]any
	Argv    []string
}

// Spelling is how one surface spells marked text: a flag and a positional by
// MCP, and a call by Call.
type Spelling struct {
	// MCP spells an argument as MCP takes it, name; otherwise as the command
	// line does, --name or <name>.
	MCP bool
	// Call spells a call; where it is nil, a call stays marked.
	Call func(Invocation) string
}

// Spell is text with every marker spelled by s. A marker that is the whole
// text stays bare. One inside other text is set off in backticks where its
// spelling has no delimiter of its own: over MCP every one, at the command
// line a call alone, --name and <name> being delimited already.
func (s Spelling) Spell(text string) string {
	if !strings.Contains(text, open) {
		return text
	}
	markers, prose := 0, false
	scan(text, func(literal string) { prose = prose || literal != "" }, func(marker) { markers++ })
	whole := markers == 1 && !prose
	var b strings.Builder
	scan(text, func(literal string) { b.WriteString(literal) }, func(m marker) {
		spelled, delimited := s.spell(m)
		if !whole && !delimited {
			spelled = "`" + spelled + "`"
		}
		b.WriteString(spelled)
	})
	return b.String()
}

// spell is m as s spells it, and whether that spelling is delimited without
// backticks: a call left marked is.
func (s Spelling) spell(m marker) (string, bool) {
	switch {
	case m.call != nil && s.Call != nil:
		return s.Call(*m.call), false
	case m.call != nil:
		return m.text, true
	case s.MCP && m.hasValue:
		return m.arg.Name + ": " + m.value, false
	case s.MCP:
		return m.arg.Name, false
	case m.hasValue:
		return "--" + m.arg.Name + " " + m.value, true
	case m.arg.Positional:
		return "<" + m.arg.Name + ">", true
	}
	return "--" + m.arg.Name, true
}

// Arguments is every argument text names through a marker, in order.
func Arguments(text string) []Argument {
	var out []Argument
	scan(text, func(string) {}, func(m marker) {
		if m.call == nil {
			out = append(out, m.arg)
		}
	})
	return out
}

// Calls is every call text holds, in order.
func Calls(text string) []Invocation {
	var out []Invocation
	scan(text, func(string) {}, func(m marker) {
		if m.call != nil {
			out = append(out, *m.call)
		}
	})
	return out
}

// Marked is whether text carries a marker's opening, well formed or not.
func Marked(text string) bool { return strings.Contains(text, open) }

type marker struct {
	text     string
	arg      Argument
	value    string
	hasValue bool
	call     *Invocation
}

// scan hands each stretch of text outside a marker to literal and each marker
// to marked. An opening that begins no well-formed marker is literal.
func scan(text string, literal func(string), marked func(marker)) {
	for {
		i := strings.Index(text, open)
		if i < 0 {
			literal(text)
			return
		}
		literal(text[:i])
		rest := text[i+len(open):]
		j := strings.Index(rest, close)
		if j < 0 {
			literal(text[i:])
			return
		}
		m, ok := parse(rest[:j])
		if !ok {
			literal(open)
			text = rest
			continue
		}
		m.text = text[i : i+len(open)+j+len(close)]
		marked(m)
		text = rest[j+len(close):]
	}
}

// parse is the marker whose body, between its delimiters, is body.
func parse(body string) (marker, bool) {
	if body == "" {
		return marker{}, false
	}
	kind, payload := body[0], body[1:]
	switch kind {
	case flagKind, positionalKind:
		return marker{arg: Argument{Name: payload, Positional: kind == positionalKind}}, isName(payload)
	case flagValueKind:
		name, value, ok := strings.Cut(payload, " ")
		return marker{arg: Argument{Name: name}, value: value, hasValue: true}, ok && isName(name)
	case callKind:
		command, args, ok := strings.Cut(payload, " ")
		if !ok || !isName(command) {
			return marker{}, false
		}
		inv := Invocation{Command: command}
		var err error
		if strings.HasPrefix(args, "[") {
			err = json.Unmarshal([]byte(args), &inv.Argv)
		} else {
			err = json.Unmarshal([]byte(args), &inv.Args)
		}
		return marker{call: &inv}, err == nil
	}
	return marker{}, false
}

// isName is whether s is a command's or an argument's name: lower-case words
// joined by hyphens.
func isName(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	return strings.Trim(s, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}
