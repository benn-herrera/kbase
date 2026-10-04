package kb

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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

// RenderIDListField is a frontmatter "<field>: [id, id]" line.
func RenderIDListField(field string, ids []string) string {
	return field + ": [" + strings.Join(ids, ", ") + "]"
}

// ReplaceOrInsertFrontmatterField replaces field's value in the document's
// frontmatter block with ids — over the whole span the reader joins — or
// inserts it after the line opening with anchorPrefix, else at the block's
// top. Every other byte, terminators included, is kept. A document with no
// block is returned unchanged.
func ReplaceOrInsertFrontmatterField(document, field string, ids []string, anchorPrefix string) string {
	m := FrontmatterRE.FindStringSubmatchIndex(document)
	if m == nil {
		return document
	}
	body := document[m[2]:m[3]]
	newLine := RenderIDListField(field, ids)
	lines := SplitLines(body)
	parts := SplitLinesKeepEnds(body)
	eol := "\n"
	if len(parts) > 0 && terminator(parts[0]) != "" {
		eol = terminator(parts[0])
	}
	var out []string
	replaced := false
	for i := 0; i < len(lines); {
		if strings.HasPrefix(Strip(lines[i]), field+":") {
			indent := lines[i][:len(lines[i])-len(LStrip(lines[i]))]
			out = append(out, indent+newLine+eol)
			replaced = true
			i = FrontmatterFieldEnd(lines, i)
			continue
		}
		out = append(out, parts[i])
		i++
	}
	if !replaced {
		out = nil
		inserted := false
		for i, text := range lines {
			out = append(out, parts[i])
			if !inserted && strings.HasPrefix(Strip(text), anchorPrefix) {
				out = append(out, newLine+eol)
				inserted = true
			}
		}
		if !inserted {
			out = append([]string{newLine + eol}, out...)
		}
	}
	for i := 0; i < len(out)-1; i++ {
		if terminator(out[i]) == "" {
			out[i] += eol
		}
	}
	if n := len(out); n > 0 && strings.HasSuffix(out[n-1], "\n") {
		out[n-1] = out[n-1][:len(out[n-1])-1]
	}
	return document[:m[2]] + strings.Join(out, "") + document[m[3]:]
}
