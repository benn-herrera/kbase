package format300

import (
	"regexp"
	"strings"
)

// StatementMarkerOpener is the text a statement marker opens with, ahead of
// the node id.
const StatementMarkerOpener = "<!-- claim-rigor:"

// StatementMarker is the statement marker naming the node id.
func StatementMarker(id string) string { return StatementMarkerOpener + " " + id + " -->" }

// StatementMarkerRE is one statement marker as it is read, a newline inside
// it included; its group is the text naming the node ids.
var StatementMarkerRE = PyRE(`(?s)<!--\s*claim-rigor:\s*(.*?)\s*-->`)

// AppendMarker is line, which carries no line break, with marker appended
// ahead of its trailing spaces and tabs; a line already carrying the marker
// is returned as it is.
func AppendMarker(line, marker string) string {
	content := strings.TrimRight(line, " \t")
	if strings.Contains(content, marker) {
		return line
	}
	return content + " " + marker + line[len(content):]
}

// DisplayMathInfo is the info string of a displayed equation's fence.
const DisplayMathInfo = "math"

// EquationLocator is the text that locates the line inside an equation's
// display-maths fence carrying its label, where the equation's statement
// marker goes: the fence's closing line can carry none.
func EquationLocator(label string) string { return `\label{` + label + `}` }

var equationTitleRE = regexp.MustCompile("(?s)^Equation \\(`([^`]+)`\\) — .+$")

// EquationLabel is the \label an equation node's title carries, or "" with
// ok false for any other title.
func EquationLabel(title string) (string, bool) {
	m := equationTitleRE.FindStringSubmatch(title)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// LabelName is the name a labelled block's label line carries — letters, in
// words single-spaced — as a pattern fragment: a label line is read only where
// its name is this.
const LabelName = `[A-Za-z]+(?: [A-Za-z]+)*`

// labelLineRE is a labelled block's label line, bare or wrapped in the
// identifier the source's \label became.
var labelLineRE = PyRE(`^>\s*(?:<span id="[^"]*">)?\*\*(` + LabelName + `)\*\*(?:</span>)?\s*$`)

// LabelLine is the name line carries where it is a labelled block's label
// line as it is read; ok false for any other line.
func LabelLine(line string) (name string, ok bool) {
	m := labelLineRE.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}
