// Package result owns the YAML document a subcommand writes to stdout: keys
// in the order given, strings quoted, outcome first.
package result

import (
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"kbase/internal/argmark"
)

// Outcomes.
const (
	Done      = "done"
	Unchanged = "unchanged"
	Bounded   = "bounded"
	Refused   = "refused"
	Retry     = "retry"
	Failed    = "failed"
	Cancelled = "cancelled"
)

var exitCodes = map[string]int{
	Done: 0, Unchanged: 0, Bounded: 0,
	Refused: 1, Retry: 2, Failed: 3, Cancelled: 4,
}

// ExitCode is the process exit code an outcome stands for. An outcome outside
// the table reads 0.
func ExitCode(outcome string) int { return exitCodes[outcome] }

// IsError reports whether the outcome is an error: its exit code is not 0.
func IsError(outcome string) bool { return ExitCode(outcome) != 0 }

// Field is one key of a result. Value is nil, a string, a Verbatim,
// a bool, an int, a float64, a []string, a Record, a []Record, an Item or an
// []Item.
type Field struct {
	Key   string
	Value any
}

// Verbatim is text a result carries as it stands on every surface: KB
// content a result passes through — a title, a rationale, a context, a line of
// a leaf. Only the text kbase composes is spelled; KB text holding what reads
// as a marker is the author's, and is never rewritten.
type Verbatim string

// Record is one mapping in a list, its keys in the order given.
type Record []Field

// The keys that close a result, chosen by its outcome.
const (
	RefusalsKey  = "refusals"
	FailuresKey  = "failures"
	CancelledKey = "cancelled"
)

var closingKeys = []string{RefusalsKey, FailuresKey, CancelledKey}

// WarningsKey is the key of a verb's warnings: one item per fault the verb
// noted and carried on past.
const WarningsKey = "warnings"

// WithWarnings is a copy of fields with warnings placed last among the verb's
// own keys: before the key that closes the result, where fields end on one.
func WithWarnings(fields []Field, warnings Field) []Field {
	at := len(fields)
	if at > 0 && slices.Contains(closingKeys, fields[at-1].Key) {
		at--
	}
	return slices.Insert(slices.Clone(fields), at, warnings)
}

// Item is one refusal, failure or verify finding. A zero key is omitted;
// Detail is always written, and an item names at least one of Check, Path
// and Key. Entry, Line and Column are 1-based.
type Item struct {
	Check   string   `json:"check,omitempty"`
	Path    string   `json:"path,omitempty"`
	Entry   int      `json:"entry,omitempty"`
	Key     string   `json:"key,omitempty"`
	Line    int      `json:"line,omitempty"`
	Column  int      `json:"column,omitempty"`
	Allowed []string `json:"allowed,omitempty"`
	Remedy  string   `json:"remedy,omitempty"`
	Detail  string   `json:"detail"`
}

func (it Item) record() Record {
	var r Record
	for _, f := range []Field{{"check", it.Check}, {"path", it.Path}, {"entry", it.Entry}, {"key", it.Key},
		{"line", it.Line}, {"column", it.Column}, {"allowed", it.Allowed}, {"remedy", it.Remedy}} {
		switch v := f.Value.(type) {
		case string:
			if v == "" {
				continue
			}
		case []string:
			if v == nil {
				continue
			}
		default:
			if text, ok := integerText(v); ok && text == "0" {
				continue
			}
		}
		r = append(r, f)
	}
	return append(r, Field{"detail", it.Detail})
}

// Items is items raised as an error.
type Items []Item

// Error is the details joined, each argument they name spelled as the command
// line takes it: an error's text is read on stderr and in logs.
func (it Items) Error() string {
	parts := make([]string, len(it))
	for i, item := range it {
		parts[i] = argmark.Spelling{}.Spell(item.Detail)
	}
	return strings.Join(parts, "; ")
}

// CheckUsage names a refusal of the invocation itself: a request whose own
// form is wrong.
const CheckUsage = "usage"

// Refusal is wrong input or state carried as an error, every offending item
// named.
type Refusal []Item

func (r Refusal) Error() string { return Items(r).Error() }

// What a retry and a failed outcome ask of the caller: each ends the detail of
// every item that closes a result with that outcome.
const (
	RetryAsk  = "re-issue the identical call later"
	FailedAsk = "re-issuing the identical call will not fix it"
)

// closingAsks is, per outcome whose items end on an ask, the closing key and
// the ask.
var closingAsks = map[string]struct{ key, ask string }{
	Retry:  {RefusalsKey, RetryAsk},
	Failed: {FailuresKey, FailedAsk},
}

// Emit writes one result: outcome, then fields in order. Under retry
// and failed, every closing item's detail ends with the outcome's ask; the
// caller's fields and items are left as given.
func Emit(w io.Writer, outcome string, fields ...Field) error {
	doc := append([]Field{{"outcome", outcome}}, fields...)
	if c, ok := closingAsks[outcome]; ok {
		for i, f := range doc {
			if f.Key == c.key {
				doc[i].Value = withAsk(f.Value, c.ask)
			}
		}
	}
	return Write(w, doc...)
}

// withAsk is a copy of the items v holds, each detail ended with ask; any other
// value is returned as given.
func withAsk(v any, ask string) any {
	var items []Item
	switch x := v.(type) {
	case []Item:
		items = x
	case Items:
		items = x
	default:
		return v
	}
	out := make([]Item, len(items))
	for i, it := range items {
		it.Detail = endWithAsk(it.Detail, ask)
		out[i] = it
	}
	return out
}

// endWithAsk joins ask to detail after "; ", dropping the detail's trailing
// period so the detail stays one sentence.
func endWithAsk(detail, ask string) string {
	detail = strings.TrimSuffix(detail, ".")
	if detail == "" {
		return ask
	}
	return detail + "; " + ask
}

// Spelled is w spelling, by s, every string of a result Write puts on it —
// the surface the call w answers came through — but a Verbatim, and any other
// text written on it, such as what cobra prints from a command's summary and
// flag usages. A w already spelled keeps its spelling, settled where the call
// came in. A result written on any other writer keeps the marked form, as a
// record does until it is emitted again.
func Spelled(w io.Writer, s argmark.Spelling) io.Writer {
	if _, ok := w.(spelledWriter); ok {
		return w
	}
	return spelledWriter{w, s}
}

type spelledWriter struct {
	w        io.Writer
	spelling argmark.Spelling
}

// Underlying is the writer a Spelled writer w wraps, else w: where a verb
// writes bytes no spelling may alter.
func Underlying(w io.Writer) io.Writer {
	if sw, ok := w.(spelledWriter); ok {
		return sw.w
	}
	return w
}

// Write spells p's text where it carries a marker, which a writer of whole
// lines, as cobra is, never splits. It reports p written whole.
func (sw spelledWriter) Write(p []byte) (int, error) {
	if !argmark.Marked(string(p)) {
		return sw.w.Write(p)
	}
	if _, err := io.WriteString(sw.w, sw.spelling.Spell(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// spellingOf is the spelling of w's results, none where it is no Spelled
// writer, and the writer a result spelled by it is written on: a Spelled
// writer's own, so the encoded result, its strings spelled already, is not
// spelled again whole and a Verbatim keeps its text.
func spellingOf(w io.Writer) (func(string) string, io.Writer) {
	if sw, ok := w.(spelledWriter); ok {
		return sw.spelling.Spell, sw.w
	}
	return func(s string) string { return s }, w
}

// Write writes fields as one YAML mapping, in order, strings quoted and
// spelled as w spells them, a Verbatim as it stands.
func Write(w io.Writer, fields ...Field) error {
	spell, out := spellingOf(w)
	doc, err := mapping(fields, spell)
	if err != nil {
		return err
	}
	return encodeNode(out, doc)
}

// Reemit writes on w the result doc, which Write wrote on a writer that
// spelled nothing — a result recorded for a later reader — with each string
// spelled as w spells it.
func Reemit(w io.Writer, doc []byte) error {
	var node yaml.Node
	if err := yaml.Unmarshal(doc, &node); err != nil {
		return fmt.Errorf("reading the stored result: %w", err)
	}
	spell, out := spellingOf(w)
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
			n.Value = spell(n.Value)
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&node)
	return encodeNode(out, &node)
}

func encodeNode(w io.Writer, n *yaml.Node) error {
	enc := yaml.NewEncoder(w)
	if err := enc.Encode(n); err != nil {
		return err
	}
	return enc.Close()
}

func mapping(fields []Field, spell func(string) string) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, f := range fields {
		value, err := encode(f.Value, spell)
		if err != nil {
			return nil, fmt.Errorf("result key %s: %w", f.Key, err)
		}
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: f.Key}, value)
	}
	return node, nil
}

func encode(v any, spell func(string) string) (*yaml.Node, error) {
	switch x := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(x)}, nil
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: floatText(x)}, nil
	case Record:
		return mapping(x, spell)
	case Item:
		return mapping(x.record(), spell)
	case []Item:
		records := make([]Record, len(x))
		for i, it := range x {
			records[i] = it.record()
		}
		return encode(records, spell)
	case Items:
		return encode([]Item(x), spell)
	case []Record:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, r := range x {
			m, err := mapping(r, spell)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, m)
		}
		return seq, nil
	case string:
		return quoted(spell(x)), nil
	case Verbatim:
		return quoted(string(x)), nil
	case []string:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, s := range x {
			seq.Content = append(seq.Content, quoted(spell(s)))
		}
		return seq, nil
	}
	if text, ok := integerText(v); ok {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: text}, nil
	}
	return nil, fmt.Errorf("unsupported value type %T", v)
}

// integerText is the decimal text of v when its kind is any signed or unsigned
// integer, a named type included.
func integerText(v any) (string, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10), true
	}
	return "", false
}

// floatText spells a float so YAML 1.1 and 1.2 readers both take it for one:
// positional notation with a decimal point, never an exponent.
func floatText(f float64) string {
	switch {
	case math.IsNaN(f):
		return ".nan"
	case math.IsInf(f, 1):
		return ".inf"
	case math.IsInf(f, -1):
		return "-.inf"
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func quoted(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: s}
}
