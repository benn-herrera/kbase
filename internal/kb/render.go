package kb

import (
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"kbase/internal/formats/format400"
)

// RenderStrengthLine is an entry's derived "- strength:" line; a nil value is
// the pending placeholder, carrying no phrase and no trace.
func RenderStrengthLine(valueText *string, statusPhrase, trace string) string {
	if valueText == nil {
		return StrengthPendingLine
	}
	return "- strength: " + *valueText + " (" + statusPhrase + ")" + trace
}

// RenderStrengthAnnotation is a premise bullet's "(strength …)" annotation:
// the source's recorded derived strength.
func RenderStrengthAnnotation(valueText string) string { return "(strength " + valueText + ")" }

// BearingText is a bearing as a bullet writes it: its value, or the pending
// literal.
func BearingText(b Bearing) string {
	if b.Pending {
		return PendingLiteral
	}
	return PyFloatRepr(b.Value)
}

// RenderBearingAnnotation is a scoring edge's "(bearing …)" annotation.
func RenderBearingAnnotation(b Bearing) string { return "(bearing " + BearingText(b) + ")" }

// StatementMarkerOpener is the text a statement marker opens with, ahead of
// the node id.
const StatementMarkerOpener = format400.StatementMarkerOpener

// StatementMarker is the statement marker naming nodeID.
func StatementMarker(nodeID string) string { return format400.StatementMarker(nodeID) }

// StatementMarkerRE is one statement marker as it is read, a newline inside
// it included; its group is the text naming the node ids.
var StatementMarkerRE = format400.StatementMarkerRE

// AppendMarker is line, which carries no line break, with marker attached
// directly after its last character but its trailing spaces and tabs, with no
// space between; a line already carrying the marker is returned as it is.
func AppendMarker(line, marker string) string { return format400.AppendMarker(line, marker) }

var (
	// plainSafeRE is a string no YAML 1.1 or 1.2 reader takes for anything but
	// itself when written plain, less yaml11Words.
	plainSafeRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_./-]*$`)
	yaml11Words = map[string]bool{"y": true, "n": true, "yes": true, "no": true, "on": true, "off": true, "true": true, "false": true, "null": true}
)

// FrontmatterString is s as frontmatter writes a string: plain where no YAML
// reader could take it for anything else, double-quoted otherwise, which
// escapes every line break, so no string spans two lines.
func FrontmatterString(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if !plainSafeRE.MatchString(s) || yaml11Words[strings.ToLower(s)] {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

// FrontmatterNumber is a number as frontmatter writes it, its text as given.
func FrontmatterNumber(text string) *yaml.Node {
	tag := "!!int"
	if strings.ContainsAny(text, ".eE") {
		tag = "!!float"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: text}
}

// FrontmatterList is items as a block list.
func FrontmatterList(items []*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: items}
}

// FrontmatterMapping is keys and values, alternating, as a block mapping in
// the order given.
func FrontmatterMapping(kv ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: kv}
}
