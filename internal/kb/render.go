package kb

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// PyFloatRepr is Python's repr of a float: the shortest round-tripping
// digits, positional between 1e-4 and 1e16 with at least one fractional
// digit, else exponent form.
func PyFloatRepr(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64)
	sign := ""
	if strings.HasPrefix(e, "-") {
		sign, e = "-", e[1:]
	}
	mant, expText, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expText)
	point := exp + 1
	switch {
	case point < -3 || point > 16:
		out := digits[:1]
		if len(digits) > 1 {
			out += "." + digits[1:]
		}
		expSign := "+"
		if exp < 0 {
			expSign, exp = "-", -exp
		}
		return fmt.Sprintf("%s%se%s%02d", sign, out, expSign, exp)
	case point <= 0:
		return sign + "0." + strings.Repeat("0", -point) + digits
	case point >= len(digits):
		return sign + digits + strings.Repeat("0", point-len(digits)) + ".0"
	}
	return sign + digits[:point] + "." + digits[point:]
}

// RenderSolidityLine is an entry's derived "- solidity:" line; a nil value
// is the pending placeholder, carrying no phrase and no trace.
func RenderSolidityLine(valueText *string, statusPhrase, trace string) string {
	if valueText == nil {
		return SolidityPendingLine
	}
	return "- solidity: " + *valueText + " (" + statusPhrase + ")" + trace
}

// RenderSolidityAnnotation is a claim-target depends-on bullet's
// "(solidity …)" annotation.
func RenderSolidityAnnotation(valueText string) string { return "(solidity " + valueText + ")" }

// RenderOriginAnnotation is a demoted bullet's "(origin …)" annotation.
func RenderOriginAnnotation(origin string) string { return "(origin " + origin + ")" }

// RenderIDListField is a frontmatter "<field>: [id, id]" line.
func RenderIDListField(field string, ids []string) string {
	return field + ": [" + strings.Join(ids, ", ") + "]"
}

// frontmatterKeyOf is the key a frontmatter line opens at the top level, ok
// false where the line continues the key above it, is blank, or is a
// comment.
func frontmatterKeyOf(line string) (string, bool) {
	if line == "" || strings.ContainsRune(" \t-#", rune(line[0])) {
		return "", false
	}
	key, _, ok := strings.Cut(line, ":")
	return Strip(key), ok
}

// frontmatterKeyEnd is one past the last line of the top-level key opening
// at start: its line and every indented or list-item line below it.
func frontmatterKeyEnd(lines []string, start int) int {
	i := start + 1
	for i < len(lines) && lines[i] != "" && strings.ContainsRune(" \t-", rune(lines[i][0])) {
		i++
	}
	return i
}

// editFrontmatter is document with its frontmatter's body lines replaced by
// edit's result over them, each emitted line given the body's own line
// break; every other byte is kept. A document with no frontmatter is
// returned unchanged.
func editFrontmatter(document string, edit func(lines []string) []string) string {
	m := FindFrontmatter(document)
	if m == nil {
		return document
	}
	body := document[m[2]:m[3]]
	eol := "\n"
	if parts := SplitLinesKeepEnds(body); len(parts) > 0 && terminator(parts[0]) != "" {
		eol = terminator(parts[0])
	} else if m[3] > 0 && strings.HasSuffix(document[:m[2]], "\r\n") {
		eol = "\r\n"
	}
	lines := edit(SplitLines(body))
	joined := strings.Join(lines, eol)
	if m[2] == m[3] && len(lines) > 0 {
		joined += eol
	}
	return document[:m[2]] + joined + document[m[3]:]
}

// ReplaceOrInsertFrontmatterField replaces field's value in the document's
// frontmatter with ids — over the key's whole span — or inserts it after the
// span of the key anchorPrefix opens, else at the top. Every other byte,
// terminators included, is kept. A document with no frontmatter is returned
// unchanged.
func ReplaceOrInsertFrontmatterField(document, field string, ids []string, anchorPrefix string) string {
	return setFrontmatterKey(document, field, []string{RenderIDListField(field, ids)}, strings.TrimSuffix(anchorPrefix, ":"), true)
}

// SetFrontmatterKey replaces key's span in the document's frontmatter with
// lines, or inserts them after the span of the key after, else at the end.
// Every other byte is kept; a document with no frontmatter is returned
// unchanged.
func SetFrontmatterKey(document, key string, lines []string, after string) string {
	return setFrontmatterKey(document, key, lines, after, false)
}

// FrontmatterKeySpan is the body lines of the frontmatter and the span
// [start, end) of key's lines among them; ok false where the document has no
// frontmatter or the key is not in it.
func FrontmatterKeySpan(document, key string) (lines []string, start, end int, ok bool) {
	m := FindFrontmatter(document)
	if m == nil {
		return nil, 0, 0, false
	}
	lines = SplitLines(document[m[2]:m[3]])
	for i := range lines {
		if k, isKey := frontmatterKeyOf(lines[i]); isKey && k == key {
			return lines, i, frontmatterKeyEnd(lines, i), true
		}
	}
	return lines, 0, 0, false
}

// EditFrontmatterLines is document with its frontmatter body lines replaced
// by edit's result over them; every other byte is kept, and a document with
// no frontmatter is returned unchanged.
func EditFrontmatterLines(document string, edit func(lines []string) []string) string {
	return editFrontmatter(document, edit)
}

func setFrontmatterKey(document, key string, newLines []string, after string, atTop bool) string {
	return editFrontmatter(document, func(lines []string) []string {
		for i := range lines {
			if k, ok := frontmatterKeyOf(lines[i]); ok && k == key {
				return slices.Concat(lines[:i], newLines, lines[frontmatterKeyEnd(lines, i):])
			}
		}
		for i := range lines {
			if k, ok := frontmatterKeyOf(lines[i]); ok && k == after {
				end := frontmatterKeyEnd(lines, i)
				return slices.Concat(lines[:end], newLines, lines[end:])
			}
		}
		if atTop {
			return slices.Concat(newLines, lines)
		}
		at := len(lines)
		for i := range lines {
			if k, ok := frontmatterKeyOf(lines[i]); ok && k == FormatKey && frontmatterKeyEnd(lines, i) == len(lines) {
				at = i
			}
		}
		return slices.Concat(lines[:at], newLines, lines[at:])
	})
}

// StampFormat is document with version as its frontmatter's kb-format, the
// last key, any stamp it carried removed; a document with no frontmatter
// gains one holding the stamp alone.
func StampFormat(document, version string) (string, error) {
	stamp, err := RenderFrontmatterField(FormatKey, FrontmatterString(version))
	if err != nil {
		return "", err
	}
	if FindFrontmatter(document) == nil {
		return WrapFrontmatter(stamp) + "\n" + document, nil
	}
	return editFrontmatter(document, func(lines []string) []string {
		var out []string
		for i := 0; i < len(lines); {
			if key, ok := frontmatterKeyOf(lines[i]); ok && key == FormatKey {
				i = frontmatterKeyEnd(lines, i)
				continue
			}
			out = append(out, lines[i])
			i++
		}
		return append(out, stamp...)
	}), nil
}

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

// FrontmatterIDList is ids as one inline list.
func FrontmatterIDList(ids []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, id := range ids {
		n.Content = append(n.Content, FrontmatterString(id))
	}
	return n
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

// RenderFrontmatterField is one top-level key and its value as frontmatter
// lines, nested values indented two spaces.
func RenderFrontmatterField(key string, v *yaml.Node) ([]string, error) {
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(FrontmatterMapping(FrontmatterString(key), v)); err != nil {
		return nil, fmt.Errorf("frontmatter %s: %w", key, err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("frontmatter %s: %w", key, err)
	}
	return strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n"), nil
}

// WrapFrontmatter is body lines between the frontmatter's fences, the
// closing fence's line break left to the document.
func WrapFrontmatter(lines []string) string {
	return fence + "\n" + strings.Join(append(slices.Clone(lines), fence), "\n")
}
