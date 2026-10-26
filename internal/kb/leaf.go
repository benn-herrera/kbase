package kb

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/formats/format300"
)

// MalformedError is authored metadata no reader can take: refresh and verify
// stop on it.
type MalformedError struct{ Msg string }

func (e MalformedError) Error() string { return e.Msg }

func malformed(format string, args ...any) error {
	return MalformedError{Msg: fmt.Sprintf(format, args...)}
}

var (
	statementMarkerRE = format300.StatementMarkerRE
	headingLineRE     = PyRE(`^(#{1,6})\s+(.*)$`)
	axiomHeading      = PyRE(`^### Axiom (` + AxiomLabel + `):\s*(.+)$`)
	anyHeadingRE      = PyRE(`^(#{1,6})\s`)
)

// The frontmatter keys a leaf's node declaration carries besides its id, its
// status and its local value (RigorKey): the pair lists an analysis or
// experiment authors on the source.
const (
	LiftsKey   = "lifts"
	RefutesKey = "refutes"
)

// statementMarker is one node id a statement marker names, and the 0-based
// line the marker opens on.
type statementMarker struct {
	id   string
	line int
}

// statementMarkers is every node id the statement markers in text name, in
// text order.
func statementMarkers(text string) []statementMarker {
	var out []statementMarker
	for _, m := range statementMarkerRE.FindAllStringSubmatchIndex(text, -1) {
		line := strings.Count(text[:m[0]], "\n")
		for _, id := range NodeIDs(text[m[2]:m[3]]) {
			out = append(out, statementMarker{id, line})
		}
	}
	return out
}

// MarkerLines is, for every node id a statement marker in text names, the
// 0-based line the first such marker opens on.
func MarkerLines(text string) map[string]int {
	out := map[string]int{}
	for _, m := range statementMarkers(text) {
		if _, seen := out[m.id]; !seen {
			out[m.id] = m.line
		}
	}
	return out
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

// AxiomHeading is one "### Axiom <label>: <title>" heading of axioms.md: the
// label, the title, and the heading's 1-based line.
type AxiomHeading struct {
	Label, Title string
	Line         int
}

// AxiomHeadings is every axiom heading of text, outside code, in text order.
func AxiomHeadings(text string) []AxiomHeading {
	var out []AxiomHeading
	for i, line := range scrubbedLines(text) {
		if m := axiomHeading.FindStringSubmatch(line); m != nil {
			out = append(out, AxiomHeading{Label: m[1], Title: Strip(m[2]), Line: i + 1})
		}
	}
	return out
}

// AnchorSection is the code-stripped body of the section whose heading
// slugifies to anchor, up to the next heading as deep or shallower; ok is
// false where no heading anchors there.
func AnchorSection(text, anchor string) (string, bool) {
	lines := strings.Split(StripCodeSplitLines(text), "\n")
	for i, line := range lines {
		h := anyHeadingRE.FindStringSubmatch(line)
		if h == nil || Slugify(Strip(line[len(h[1]):])) != anchor {
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
