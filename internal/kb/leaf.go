package kb

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// MalformedError is authored metadata no reader can take: refresh and verify
// stop on it.
type MalformedError struct{ Msg string }

func (e MalformedError) Error() string { return e.Msg }

func malformed(format string, args ...any) error {
	return MalformedError{Msg: fmt.Sprintf(format, args...)}
}

var (
	headingLineRE = PyRE(`^(#{1,6})\s+(.*)$`)
	anyHeadingRE  = PyRE(`^(#{1,6})\s`)
)

// statementMarker is one node id a statement marker names, the 0-based line
// the marker opens on, and the byte offset in its text it opens at.
type statementMarker struct {
	id           string
	line, offset int
}

// statementMarkers is every hosted node id the statement markers in text
// name, in text order: a work's or an axiom's id in a marker hosts nothing.
func statementMarkers(text string) []statementMarker {
	var out []statementMarker
	for _, m := range StatementMarkerRE.FindAllStringSubmatchIndex(text, -1) {
		line := strings.Count(text[:m[0]], "\n")
		for _, id := range hostedIDs(text[m[2]:m[3]]) {
			out = append(out, statementMarker{id, line, m[0]})
		}
	}
	return out
}

// MarkerLines is, for every node id a statement marker in text names, the
// 0-based line the first such marker opens on.
func MarkerLines(text string) map[string]int {
	return firstMarkers(text, func(m statementMarker) int { return m.line })
}

// MarkerOffsets is, for every node id a statement marker in text names, the
// byte offset the first such marker opens at: the order markers are read in,
// two on one line among them.
func MarkerOffsets(text string) map[string]int {
	return firstMarkers(text, func(m statementMarker) int { return m.offset })
}

// firstMarkers is at of the first statement marker naming each node id of
// text.
func firstMarkers(text string, at func(statementMarker) int) map[string]int {
	out := map[string]int{}
	for _, m := range statementMarkers(text) {
		if _, seen := out[m.id]; !seen {
			out[m.id] = at(m)
		}
	}
	return out
}

// LineMarker is one node id a statement marker on a line names, and the byte
// span [Start, End) of the whole marker. Ids named by one marker share its span.
type LineMarker struct {
	ID         string
	Start, End int
}

// LineMarkers is every statement marker on one line naming a hosted node, in
// order.
func LineMarkers(line string) []LineMarker {
	var out []LineMarker
	for _, m := range StatementMarkerRE.FindAllStringSubmatchIndex(line, -1) {
		for _, id := range hostedIDs(line[m[2]:m[3]]) {
			out = append(out, LineMarker{id, m[0], m[1]})
		}
	}
	return out
}

// markedSpanRE is one statement marker with the spaces and tabs before it.
var markedSpanRE = regexp.MustCompile(`[ \t]*` + StatementMarkerRE.String())

// StripLineMarkers is line with every statement marker removed, each taking
// the spaces and tabs before it; nothing else changes. A marker naming several
// ids is removed in one cut.
func StripLineMarkers(line string) string {
	return markedSpanRE.ReplaceAllString(line, "")
}

// MarkerSite is one statement marker: the kb-root-relative path of the leaf
// holding it and the 0-based line it opens on.
type MarkerSite struct {
	Path string
	Line int
}

// HostFault is a claim, analysis or experiment that does not carry exactly one
// statement marker: Markers is empty where it carries none, and otherwise
// every marker naming it, in path order, the first its host's.
type HostFault struct {
	ID      string
	Markers []MarkerSite
}

// Hosts is the host of each claim, analysis and experiment of g: the leaf
// holding its statement marker, and where it has several the first in path
// order. leaves is each leaf's kb-root-relative path and text. A node with no
// marker has no host; it and a node with several markers are each one
// HostFault, in g's order.
func Hosts(g Graph, leaves map[string]string) (map[string]string, []HostFault) {
	sites := map[string][]MarkerSite{}
	for _, p := range slices.Sorted(maps.Keys(leaves)) {
		for _, m := range statementMarkers(leaves[p]) {
			sites[m.id] = append(sites[m.id], MarkerSite{Path: p, Line: m.line})
		}
	}
	var ids []string
	for _, n := range g.Claims {
		ids = append(ids, n.ID)
	}
	for _, n := range g.Analyses {
		ids = append(ids, n.ID)
	}
	for _, n := range g.Experiments {
		ids = append(ids, n.ID)
	}
	hosts := map[string]string{}
	var faults []HostFault
	for _, id := range ids {
		s := sites[id]
		if len(s) > 0 {
			hosts[id] = s[0].Path
		}
		if len(s) != 1 {
			faults = append(faults, HostFault{ID: id, Markers: s})
		}
	}
	return hosts, faults
}

// FirstHeading is the text of a document's first heading at any level.
func FirstHeading(text string) string {
	for _, line := range SplitLines(text) {
		if m := headingLineRE.FindStringSubmatch(line); m != nil {
			return Strip(m[2])
		}
	}
	return ""
}

// AxiomHeading is one axiom heading of axioms.md: the axiom id its marker
// names, "" where it carries none; its text after its #s, its markers removed;
// and its 1-based line.
type AxiomHeading struct {
	ID, Text string
	Line     int
}

// AxiomHeadingLine is the heading of the axiom id of label and title, its
// marker ending it.
func AxiomHeadingLine(label, title, id string) string {
	return "### " + AxiomHeadingText(label, title) + StatementMarker(id)
}

// AxiomHeadingText is the text of the axiom heading of label and title, after
// its `###`.
func AxiomHeadingText(label, title string) string { return "Axiom " + label + ": " + title }

var axiomIDRE = PyRE(IDBody(AxiomPrefix))

// AxiomHeadings is every axiom heading of text, outside code, in text order:
// one per axiom id a heading's markers name, and every unmarked heading
// opening "### Axiom ", whose ID is "".
func AxiomHeadings(text string) []AxiomHeading {
	var out []AxiomHeading
	for i, line := range scrubbedLines(text) {
		m := headingLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		heading := Strip(StripLineMarkers(m[2]))
		var ids []string
		for _, mk := range StatementMarkerRE.FindAllStringSubmatch(line, -1) {
			ids = append(ids, findAllBounded(axiomIDRE, mk[1])...)
		}
		for _, id := range ids {
			out = append(out, AxiomHeading{ID: id, Text: heading, Line: i + 1})
		}
		if len(ids) == 0 && strings.HasPrefix(line, "### Axiom ") {
			out = append(out, AxiomHeading{Text: heading, Line: i + 1})
		}
	}
	return out
}

// AxiomAnchor is the anchor of the first heading of text, axioms.md, carrying
// the axiom id's marker; ok false where none does.
func AxiomAnchor(text, id string) (string, bool) {
	for _, h := range AxiomHeadings(text) {
		if h.ID == id {
			return HeadingAnchors(text)[h.Line-1], true
		}
	}
	return "", false
}

// HeadingAnchors is the anchor of every heading of text, by 0-based line of
// its str.splitlines() lines, as GitHub's slugger gives them in document
// order: a line of a fenced block is no heading, and an HTML comment — a
// statement marker among them — is no part of a heading's text.
func HeadingAnchors(text string) map[int]string {
	var s Slugger
	out := map[int]string{}
	for i, line := range blankFenced(SplitLines(text)) {
		if m := headingLineRE.FindStringSubmatch(line); m != nil {
			out[i] = s.Slug(m[2])
		}
	}
	return out
}

// AnchorSection is the code-stripped body of the first section whose heading
// anchors at anchor (HeadingAnchors), up to the next heading as deep or
// shallower; ok is false where no heading anchors there.
func AnchorSection(text, anchor string) (string, bool) {
	anchors := HeadingAnchors(text)
	lines := strings.Split(StripCodeSplitLines(text), "\n")
	for i, line := range lines {
		h := anyHeadingRE.FindStringSubmatch(line)
		if a, ok := anchors[i]; h == nil || !ok || a != anchor {
			continue
		}
		var body []string
		for _, next := range lines[i+1:] {
			if n := anyHeadingRE.FindStringSubmatch(next); n != nil && len(n[1]) <= len(h[1]) {
				break
			}
			body = append(body, next)
		}
		return strings.Join(body, "\n"), true
	}
	return "", false
}

// UnmigratedAgentsCheck names UnmigratedAgentsFile's refusal.
const UnmigratedAgentsCheck = "agents-redirect"

// UnmigratedAgentsFile is the refusal for a KB whose CLAUDE.md is not the
// one-line redirect to AGENTS.md, or "" where it is or is absent.
func UnmigratedAgentsFile(src *Source) (string, error) {
	kbRoot := src.Root()
	redirect := filepath.Join(kbRoot, AgentsRedirectFile)
	if !src.IsFile(redirect) {
		return "", nil
	}
	text, err := src.ReadText(redirect)
	if err != nil {
		return "", err
	}
	if Strip(text) == AgentsRedirect {
		return "", nil
	}
	return fmt.Sprintf("%s is not the one-line redirect '%s' — this KB predates the %s split, or the file was edited. "+
		"Nothing here converts it: move its content into %s (append if that file exists), make %s the single line '%s', then rerun.",
		redirect, AgentsRedirect, AgentsFile, filepath.Join(kbRoot, AgentsFile), redirect, AgentsRedirect), nil
}
