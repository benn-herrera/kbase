package summarize

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"kbase/internal/text"
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

// parseSummary reads a response into the artifact's three fields.
//
// Unknown fields are tolerated and missing ones are not, for the same reason
// the taxonomy answer tolerates them: the post-condition is about the prose,
// and a model that added a field has still written the summary.
func parseSummary(response string) (Summary, error) {
	obj, ok := text.JSONObject(response)
	if !ok {
		return Summary{}, errors.New("answer with the JSON object described and nothing else")
	}
	var s Summary
	if err := json.Unmarshal([]byte(obj), &s); err != nil {
		return Summary{}, errors.New("the answer is not the JSON object described")
	}
	s.Framing = strings.TrimSpace(s.Framing)
	s.ConclusionsHeading = strings.TrimSpace(s.ConclusionsHeading)
	s.Conclusions = strings.TrimSpace(s.Conclusions)
	return s, nil
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
func Read(store ArtifactReader, summariesDir, nodePath string) (Summary, bool, error) {
	data, err := store.Get(summariesDir + "/" + nodePath + unitSuffix)
	if err != nil {
		// A node with no summary is a legal state, not a failure: the O-1
		// grammar renders an index with no conclusions block, which is what
		// the no-model build delivers and what a level with no stage produces.
		return Summary{}, false, nil
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
