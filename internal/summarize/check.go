package summarize

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"kbase/internal/treeplan"
)

// §6.4's verifier post-conditions: deterministic, cheap, and about SHAPE
// rather than quality.
//
// Every message here is a retry note before it is anything else — the runner
// caps it at twelve words (pipeline.retryNoteWords) and puts it in the next
// prompt — so each one states the mechanical fact and names no path, no node
// and no number the model could echo back as an answer.
const (
	// linkSyntax is the whole of what "no link syntax" means mechanically: a
	// Markdown link and an image both carry it, and nothing else does. A bare
	// URL is not link syntax — it is source material a summary is allowed to
	// quote (§4.5.5).
	linkSyntax = "]("

	// headingCap bounds the conclusions heading. It is a plain title, so a cap
	// in words is what says "a title, not a sentence".
	headingCap = 10
)

// The answer grammar: three labelled blocks of free text, in a fixed order,
// with no terminator. The labels are the ONE statement of the format — the
// definition renders them (stubDefinition), the acceptance criteria name them,
// and parseSummary reads them — so the format the model is shown and the format
// the machine enforces cannot drift apart.
//
// The three values this ask returns are prose. Asking for them inside a JSON
// object made the model the author of a document as well as of the summary, and
// those are two different failure modes: an unescaped quote in a sentence that
// legitimately quotes source material is a document-construction failure, not a
// summary that is wrong. The artifact kbase writes for itself stays JSON
// (summary.go) — machine-to-machine encoding is not this seam.
const (
	labelFraming     = "FRAMING:"
	labelHeading     = "HEADING:"
	labelConclusions = "CONCLUSIONS:"
)

// parseSummary reads a response into the artifact's three fields.
//
// Forgiving of noise, strict on substance. Anything before the first label is
// ignored, a fence line is ignored wherever it appears, and a block runs to the
// next label or to the end of the response — so a preamble sentence, a fenced
// answer or a closing remark costs nothing. What is refused is genuinely
// ambiguous: a label given twice (two values for one field) and a response with
// no label at all (an answer to some other question). Everything ELSE about the
// values is check's, unchanged, and it stays the real net — an empty framing, an
// over-long summary, a link, a heading that is a sentence.
//
// The ORDER is fixed in the declaration and tolerated in the reading: the
// labels are what makes each value unambiguous, so a permuted answer is a
// correct answer written oddly, and rejecting it would cost a retry to learn
// nothing.
func parseSummary(response string) (Summary, error) {
	blocks := map[string]*strings.Builder{
		labelFraming:     {},
		labelHeading:     {},
		labelConclusions: {},
	}
	seen := make(map[string]bool, len(blocks))
	var cur *strings.Builder
	for _, line := range strings.Split(response, "\n") {
		if label, rest, ok := blockLabel(line); ok {
			if seen[label] {
				return Summary{}, errors.New(label + " given twice; one block each")
			}
			seen[label] = true
			cur = blocks[label]
			cur.WriteString(rest)
			continue
		}
		if cur == nil || fenceLine(line) {
			continue
		}
		cur.WriteString("\n" + line)
	}
	if cur == nil {
		return Summary{}, errors.New("answer in the " + labelFraming + " " + labelHeading +
			" and " + labelConclusions + " blocks")
	}
	return Summary{
		Framing:            strings.TrimSpace(blocks[labelFraming].String()),
		ConclusionsHeading: strings.TrimSpace(blocks[labelHeading].String()),
		Conclusions:        strings.TrimSpace(blocks[labelConclusions].String()),
	}, nil
}

// blockLabel reports whether a line opens a block, and returns the label it
// opens with whatever the model wrote after it on the same line.
//
// A label is matched through the decoration a model wraps it in — a bullet, a
// bold run, a heading marker — because none of that changes which value the
// block holds, and case-insensitively for the same reason. What follows the
// label on the line is the block's first line: a model that writes
// "FRAMING: this section holds…" has answered.
func blockLabel(line string) (label, rest string, ok bool) {
	trimmed := trimMarkup(line)
	for _, l := range []string{labelFraming, labelHeading, labelConclusions} {
		if len(trimmed) >= len(l) && strings.EqualFold(trimmed[:len(l)], l) {
			return l, trimMarkup(trimmed[len(l):]), true
		}
	}
	return "", "", false
}

// trimMarkup strips the decoration around a label — list bullets, emphasis,
// code spans, heading markers — leaving the label itself. It touches label
// matching only: a line of a block's own text is that block's text, verbatim.
func trimMarkup(s string) string { return strings.Trim(s, " \t\r-*`#>_") }

// fenceLine reports a line that is nothing but a code fence. It is the one
// piece of a wrapper that would otherwise land INSIDE a value rather than
// before the first label, and a fence in a delivered summary is a defect no
// content rule below would catch.
func fenceLine(line string) bool {
	t := strings.TrimSpace(line)
	return len(t) >= 3 && (strings.Trim(t, "`") == "" || strings.Trim(t, "~") == "")
}

// check holds one summary to §6.4.
//
// Empty conclusions are LEGAL and stay legal (§4.5.6): an index over purely
// navigational material has nothing to conclude, and an invented conclusion is
// worse than a missing one — routing lives in the down-link list, which no
// model writes. What is not legal is an empty framing: the node would then
// render as a heading and a list, and the page has nothing to say for itself.
func (s *Summarizer) check(u *unit, sum Summary) error {
	if sum.Framing == "" {
		return errors.New("say in a line or two what this section holds")
	}
	if got, limit := s.job.Params.Est.Estimate(sum.Framing+"\n\n"+sum.Conclusions),
		s.job.Params.Budgets.SummaryTokens; got > limit {
		// The cap is stated over the SUM because that is what the level above
		// pays for this node: G-2's digest cost per index child is one number
		// (treeplan's summary cap), and two separately-capped halves could
		// together cost twice what the parent's bound was proved against.
		return errors.New("too long; shorten it to a short paragraph and its conclusions")
	}
	if sum.Conclusions != "" && sum.ConclusionsHeading == "" {
		return errors.New("the conclusions need a heading of their own")
	}
	if err := plainHeading(sum.ConclusionsHeading); err != nil {
		return err
	}
	for _, field := range []string{sum.Framing, sum.Conclusions, sum.ConclusionsHeading} {
		if strings.Contains(field, linkSyntax) {
			return errors.New("no links; the navigation is added mechanically")
		}
		if err := s.noNodePath(u, field); err != nil {
			return err
		}
	}
	if err := headingDepth(sum.Framing); err != nil {
		return err
	}
	return headingDepth(sum.Conclusions)
}

// noNodePath is F-11's check, and it is NODE-path matching rather than
// path-SHAPE matching.
//
// "No path-like token" would contradict §4.5.5: on the pinned validation
// corpora a discipline-conforming summary quotes API signatures, config keys
// and file paths verbatim, and would be rejected outright. What I-2 forbids is
// narrower and mechanically checkable — the text may name no node of THIS
// knowledge base. A path-shaped string that names no node is source material
// and is legal.
//
// The node being summarised is exempt: a summary is allowed to be about
// itself. Index basenames are not checked at all, because every index in the
// tree is called `index.md` — the string names no particular node, so matching
// it would reject a summary for naming nothing.
func (s *Summarizer) noNodePath(u *unit, field string) error {
	if field == "" {
		return nil
	}
	s.mu.Lock()
	paths := s.paths
	s.mu.Unlock()
	own := path.Base(u.node.Path)
	for _, p := range paths {
		if p == u.node.Path {
			continue
		}
		if strings.Contains(field, p) {
			return errors.New("do not name a file of this knowledge base")
		}
		base := path.Base(p)
		if base == indexBase || base == own {
			continue
		}
		if strings.Contains(field, base) {
			return errors.New("do not name a file of this knowledge base")
		}
	}
	return nil
}

// indexBase is what every index node's file is called, so it names no
// particular node and is not a node reference.
const indexBase = "index.md"

// plainHeading refuses a conclusions heading that is not a title: a heading
// marker (stage 8 supplies the `##`), more than one line, or a sentence.
func plainHeading(heading string) error {
	if heading == "" {
		return nil
	}
	if strings.ContainsAny(heading, "#\n") {
		return errors.New("the heading is a plain title, without markup")
	}
	if len(strings.Fields(heading)) > headingCap {
		return errors.New("the heading is a title, not a sentence")
	}
	return nil
}

// headingDepth refuses a heading above `##`: the node's own H1 is the tree
// plan's title, rendered by stage 8, and a second one inside the prose would
// give the page two titles.
func headingDepth(field string) error {
	for _, line := range strings.Split(field, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			return errors.New("no top-level heading; this section already has its title")
		}
	}
	return nil
}

// Read reads one node's summary artifact back, for stage 8's render path.
//
// It is the ONE reader of this artifact family: the level above uses it for
// its children's material and internal/assemble uses it for the conclusions
// block, so what the two render is the same value read the same way.
//
// ABSENCE is legal and UNREADABILITY is not [ARCH F6]. A node with no summary
// is the O-1 grammar's ordinary case — the no-model build delivers one, and so
// does a level with no stage — but a summary that exists and cannot be read is
// a different event, and folding the two together meant no gate could see a
// systematically missing summary: the render and gate 5's re-render both go
// through here, so they agreed with each other about nothing being there. The
// prefix now holds two artifact families distinguished only by suffix (§3.5),
// which is exactly where that silence would cost the most.
func Read(store ArtifactReader, summariesDir, nodePath string) (Summary, bool, error) {
	data, err := store.Get(summariesDir + "/" + nodePath + unitSuffix)
	if errors.Is(err, fs.ErrNotExist) {
		return Summary{}, false, nil
	}
	if err != nil {
		return Summary{}, false, err
	}
	s, err := ReadJSON(strings.NewReader(string(data)))
	if err != nil {
		return Summary{}, false, err
	}
	return s, true, nil
}

// All reads every index node's summary for one tree plan, keyed by node path.
//
// Stage 8 renders from it and stage 9 re-renders from it (check 5 compares
// bytes), so both take the same map from the same reader — a second read path
// would be a second thing to keep true.
func All(store ArtifactReader, summariesDir string, plan treeplan.TreePlan) (map[string]Summary, error) {
	out := map[string]Summary{}
	for _, n := range plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			continue
		}
		s, ok, err := Read(store, summariesDir, n.Path)
		if err != nil {
			return nil, fmt.Errorf("summarize: the summary of %s: %w", n.Path, err)
		}
		if ok {
			out[n.Path] = s
		}
	}
	return out, nil
}
