// Package result owns the YAML document a subcommand writes to stdout: keys
// in the order given, strings quoted, outcome first.
package result

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
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

// Field is one key of a result document. Value is nil, a string, a bool, an
// int, a float64, a []string, a Record, a []Record, an Item or an []Item.
type Field struct {
	Key   string
	Value any
}

// Record is one mapping in a list, its keys in the order given.
type Record []Field

// The keys that close a document, chosen by its outcome.
const (
	RefusalsKey  = "refusals"
	FailuresKey  = "failures"
	CancelledKey = "cancelled"
)

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
		case int:
			if v == 0 {
				continue
			}
		case []string:
			if v == nil {
				continue
			}
		}
		r = append(r, f)
	}
	return append(r, Field{"detail", it.Detail})
}

// Items is items raised as an error.
type Items []Item

func (it Items) Error() string {
	parts := make([]string, len(it))
	for i, item := range it {
		parts[i] = item.Detail
	}
	return strings.Join(parts, "; ")
}

// Refusal is wrong input or state carried as an error, every offending item
// named.
type Refusal []Item

func (r Refusal) Error() string { return Items(r).Error() }

// Emit writes one result document: outcome, then fields in order.
func Emit(w io.Writer, outcome string, fields ...Field) error {
	return Write(w, append([]Field{{"outcome", outcome}}, fields...)...)
}

// Write writes fields as one YAML mapping, in order, strings quoted.
func Write(w io.Writer, fields ...Field) error {
	doc, err := mapping(fields)
	if err != nil {
		return err
	}
	enc := yaml.NewEncoder(w)
	if err := enc.Encode(doc); err != nil {
		return err
	}
	return enc.Close()
}

func mapping(fields []Field) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, f := range fields {
		value, err := encode(f.Value)
		if err != nil {
			return nil, fmt.Errorf("result key %s: %w", f.Key, err)
		}
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: f.Key}, value)
	}
	return node, nil
}

func encode(v any) (*yaml.Node, error) {
	switch x := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(x)}, nil
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: floatText(x)}, nil
	case Record:
		return mapping(x)
	case Item:
		return mapping(x.record())
	case []Item:
		records := make([]Record, len(x))
		for i, it := range x {
			records[i] = it.record()
		}
		return encode(records)
	case Items:
		return encode([]Item(x))
	case []Record:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, r := range x {
			m, err := mapping(r)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, m)
		}
		return seq, nil
	case string:
		return quoted(x), nil
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprint(x)}, nil
	case []string:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, s := range x {
			seq.Content = append(seq.Content, quoted(s))
		}
		return seq, nil
	}
	return nil, fmt.Errorf("unsupported value type %T", v)
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
