package claimgraph

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// relationLetter is the classify ask's letter for each relation, the one map
// between the two.
var relationLetter = map[Relation]string{SupportedBy: asks.LetterSupportedBy, InSupportOf: asks.LetterInSupportOf, MentionedBy: asks.LetterMention}

func letterRelation(letter string) (Relation, bool) {
	for r, l := range relationLetter {
		if l == letter {
			return r, true
		}
	}
	return "", false
}

// classification is what classification decided and what it writes: every
// candidate's relation and outcome, the depends edges and references records,
// less the depends edges a cycle of the classified set demoted, and this
// run's ask records.
type classification struct {
	relations map[pair]Relation
	outcomes  map[pair]string
	edges     []pair
	refs      []pair
	demoted   []pair
	asked     []asks.GroupRecord
}

// held is whether the record already answers for a candidate: a draft does
// not, to a run that can ask.
func held(e buildrecords.CandidateEntry, ok, asking bool) bool {
	return ok && !(asking && e.Outcome == ClassifyDrafted)
}

// classify classifies every candidate, one letter ask each, grouped by source
// in ascending id and offered only the letters of its offered relations;
// with no reader every candidate takes its draft. Each group lands in the
// classification record as it completes, and a resumed run asks only what the
// record does not hold. A call that never completes stops it with every
// completed group recorded. A run that asks reports each source's group as
// a unit of progress, and each candidate defaulted as a fallback. opts
// supplies the reader and how it asks.
func classify(ctx context.Context, repoRoot string, candidates []candidate, statement func(ClaimNode) string, opts Options) (classification, error) {
	reader, progress := opts.Reader, opts.Progress
	out := classification{relations: map[pair]Relation{}, outcomes: map[pair]string{}}
	rec, _, err := buildrecords.ReadClassification(recordsAt(repoRoot))
	if err != nil {
		return out, err
	}
	entries := map[pair]buildrecords.CandidateEntry{}
	for _, e := range rec.Candidates {
		entries[pair{e.Source, e.Target}] = e
	}
	bySource := map[string][]candidate{}
	var sources []string
	for _, c := range candidates {
		if bySource[c.source.ID] == nil {
			sources = append(sources, c.source.ID)
		}
		bySource[c.source.ID] = append(bySource[c.source.ID], c)
	}
	slices.Sort(sources)
	pendingOf := map[string][]candidate{}
	for _, s := range sources {
		for _, c := range bySource[s] {
			if e, ok := entries[c.pair()]; !held(e, ok, reader != nil) {
				pendingOf[s] = append(pendingOf[s], c)
			}
		}
	}
	if reader != nil {
		if err := progress.units(len(pendingOf)); err != nil {
			return out, err
		}
	}
	for _, s := range sources {
		pending := pendingOf[s]
		if len(pending) == 0 {
			continue
		}
		if reader == nil {
			for _, c := range pending {
				entries[c.pair()] = buildrecords.CandidateEntry{Source: c.source.ID, Target: c.target.ID, Offered: offeredLetters(c), Outcome: ClassifyDrafted}
			}
		} else {
			items := make([]asks.ClassifyItem, len(pending))
			for i, c := range pending {
				items[i] = asks.ClassifyItem{Target: askClaim(c.target), Statement: statement(c.target), Passages: c.passages, Offered: offeredLetters(c)}
			}
			source := pending[0].source
			group, err := asks.AskGroup(ctx, asks.Classify, source.ID, asks.ClassifyAsks(askClaim(source), statement(source), items), reader, opts.ReaderConcurrency, opts.AskRecords)
			if err != nil {
				return out, err
			}
			out.asked = append(out.asked, group)
			for i, item := range group.Items {
				c := pending[i]
				e := buildrecords.CandidateEntry{Source: source.ID, Target: c.target.ID, Offered: item.Offered, Letter: item.Letter, Outcome: string(item.Outcome)}
				if item.Confidence != nil {
					e.Confidence = &item.Confidence
				}
				entries[c.pair()] = e
				if item.Outcome == asks.Defaulted {
					if err := progress.fallback(result.Item{Check: checkDefaulted, Path: source.Document,
						Detail: fmt.Sprintf("%s -> %s was answered with no offered letter twice: it takes its drafted relation, %s", source.ID, c.target.ID, c.draft)}); err != nil {
						return out, err
					}
				}
			}
			if err := progress.unit(source.ID); err != nil {
				return out, err
			}
		}
		rec.Candidates = rec.Candidates[:0]
		for _, e := range entries {
			rec.Candidates = append(rec.Candidates, e)
		}
		if err := WriteClassification(repoRoot, rec); err != nil {
			return out, err
		}
	}
	for _, c := range candidates {
		e := entries[c.pair()]
		rel := c.draft
		if e.Letter != nil {
			if r, ok := letterRelation(*e.Letter); ok {
				rel = r
			}
		}
		out.relations[c.pair()], out.outcomes[c.pair()] = rel, e.Outcome
	}
	out.edges, out.refs, out.demoted = writtenRecords(candidates, out.relations)
	return out, nil
}

func offeredLetters(c candidate) []string {
	letters := make([]string, len(c.offered))
	for i, r := range c.offered {
		letters[i] = relationLetter[r]
	}
	return letters
}

func askClaim(n ClaimNode) asks.Claim {
	return asks.Claim{ID: n.ID, Title: n.Title, Document: n.Document, Locator: n.Locator}
}

// statements is each claim's statement as an ask shows it: its block's extent
// or its prose paragraph, or its equation fence; a claim none reaches is
// shown by its title.
func statements(t *Tree, g *AuthoredGraph, inv *Inventory) func(ClaimNode) string {
	found := map[string]string{}
	for _, b := range claimBodies(t, g, inv) {
		found[b.node.ID] = b.text
	}
	lines := map[string][]string{}
	for id, f := range equationFences(g, inv) {
		if lines[f.Document] == nil {
			lines[f.Document] = pageLines(t, f.Document)
		}
		found[id] = strings.Join(lines[f.Document][f.Start:min(f.End, len(lines[f.Document]))], "\n")
	}
	return func(n ClaimNode) string {
		if s, ok := found[n.ID]; ok {
			return s
		}
		return n.Title
	}
}

// pageLines is a document's lines markers off and unquoted, line for line.
func pageLines(t *Tree, document string) []string {
	return kb.SplitLines(unquote(stripMarkers(t.Documents[document].Text)))
}

// cutEdges is every references record the build's cycle breaking cut from
// depends, with its origin: an edge a cycle of the classified set demoted,
// and a containment-directed pair a ring demoted that took its drafted
// mention rather than a letter. Its origin is inferred where the unmarked
// record answered the pair yes, cited otherwise.
func cutEdges(ring []pair, c classification, unmarkedYes []pair) map[pair]string {
	out := map[pair]string{}
	cut := func(p pair) {
		out[p] = kb.OriginCited
		if slices.Contains(unmarkedYes, p) {
			out[p] = kb.OriginInferred
		}
	}
	for _, p := range c.demoted {
		cut(p)
	}
	for _, p := range ring {
		if c.relations[p] == MentionedBy && (c.outcomes[p] == ClassifyDrafted || c.outcomes[p] == ClassifyDefaulted) {
			cut(p)
		}
	}
	return out
}

// writtenRecords is the depends edges, references records and cycle-demoted
// edges the classified candidates write: every depends edge on a cycle is
// demoted, so what remains is acyclic by construction.
func writtenRecords(candidates []candidate, relations map[pair]Relation) (edges, refs, demoted []pair) {
	depends := map[pair]bool{}
	references := map[pair]bool{}
	for _, c := range candidates {
		if d, p := written(c, relations[c.pair()]); d {
			depends[p] = true
		} else {
			references[p] = true
		}
	}
	var all []pair
	for p := range depends {
		all = append(all, p)
	}
	slices.SortFunc(all, comparePairs)
	demoted = cycleEdges(all)
	for _, p := range all {
		if !slices.Contains(demoted, p) {
			edges = append(edges, p)
		}
	}
	for _, p := range demoted {
		references[p] = true
	}
	for p := range references {
		refs = append(refs, p)
	}
	slices.SortFunc(refs, comparePairs)
	return edges, refs, demoted
}
