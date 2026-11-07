package from30to40

import (
	"strings"

	"kbase/internal/formats/format30"
	"kbase/internal/formats/format40"
)

// mintedIDRE is a minted node id, as a 3.0.0 marker names one.
var mintedIDRE = format30.PyRE(format30.IDBody())

// lift is an analysis and a claim it lifts.
type lift struct{ analysis, claim string }

// convertMarkers is text with its 3.0.0 statement markers in the 4.0.0 form.
// An analysis marker standing alone on the line directly after a line ending
// in the marker of a claim it lifts moves to directly after that marker, and
// its emptied line goes; then every marker is rewritten as one kbase-id
// marker per id it names, attached with no space to the text preceding it on
// its line; a marker alone on its line keeps its indentation. A comment in a
// marker's form naming no id is left as it stands.
func convertMarkers(text string, lifts map[lift]bool) string {
	if !strings.Contains(text, format30.StatementMarkerOpener) {
		return text
	}
	lines := strings.Split(text, "\n")
	kept := lines[:0:0]
	for i, line := range lines {
		if i > 0 {
			if analysis, ok := aloneAnalysis(line); ok {
				above := kept[len(kept)-1]
				content := strings.TrimRight(above, " \t\r")
				if lifts[lift{analysis, lastMarkedID(content)}] {
					kept[len(kept)-1] = content + strings.TrimSpace(line) + above[len(content):]
					continue
				}
			}
		}
		kept = append(kept, line)
	}
	text = strings.Join(kept, "\n")
	var b strings.Builder
	last := 0
	for _, m := range format30.StatementMarkerRE.FindAllStringSubmatchIndex(text, -1) {
		ids := mintedIDRE.FindAllString(text[m[2]:m[3]], -1)
		if len(ids) == 0 {
			continue
		}
		start := last + len(strings.TrimRight(text[last:m[0]], " \t"))
		if start == 0 || text[start-1] == '\n' {
			start = m[0]
		}
		b.WriteString(text[last:start])
		for _, id := range ids {
			b.WriteString(format40.StatementMarker(id))
		}
		last = m[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

// aloneAnalysis is the analysis a line holding one 3.0.0 statement marker and
// nothing else names; ok is false for any other line.
func aloneAnalysis(line string) (string, bool) {
	t := strings.TrimSpace(line)
	m := format30.StatementMarkerRE.FindStringSubmatchIndex(t)
	if m == nil || m[0] != 0 || m[1] != len(t) {
		return "", false
	}
	ids := mintedIDRE.FindAllString(t[m[2]:m[3]], -1)
	if len(ids) != 1 || !strings.HasPrefix(ids[0], format30.AnalysisPrefix+"-") {
		return "", false
	}
	return ids[0], true
}

// lastMarkedID is the last id the 3.0.0 statement marker content ends in
// names, "" where content does not end in a marker.
func lastMarkedID(content string) string {
	all := format30.StatementMarkerRE.FindAllStringSubmatchIndex(content, -1)
	if len(all) == 0 || all[len(all)-1][1] != len(content) {
		return ""
	}
	m := all[len(all)-1]
	ids := mintedIDRE.FindAllString(content[m[2]:m[3]], -1)
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}
