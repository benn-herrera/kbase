package claimgraph

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/records"
	"kbase/internal/result"
)

// unmarkedLocalities is how near a pair's two documents sit, nearest first:
// a yes's locality is what the report counts it under.
var unmarkedLocalities = []string{"same document", "same directory", "same paper", "cross paper"}

// unmarkedPlan is the pairs the unmarked-reference stage asks: sources
// ascending, each source's proximity picks then its wildcard picks. picks
// holds every source's targets by the slot that chose them, and the claims
// adjacency links it to, which are never asked; adjacent counts those links.
type unmarkedPlan struct {
	sources  []string
	pairs    []pair
	picks    map[string]unmarkedPicks
	adjacent int
}

type unmarkedPicks struct{ proximity, wildcard, adjacent []string }

// planUnmarked is the stage's shortlist over the whole node set. Per source —
// every node but a minted equation — its pool is every other node less the
// excluded links and every equation the source states. Its proximity slots
// take the pool's nearest targets within its paper: nearest locality first,
// then fewest places apart in the sequence of every node by position and id,
// a tie to the earlier. Its wildcard slots take the pool's best by cosine,
// less the proximity picks, so a target in another paper is reached there
// alone. A slot left unfilled stays empty. Each source is planned apart from
// every other, so a pair both its ends pick is planned both ways.
func planUnmarked(nodes []ClaimNode, statements map[string]string, pos map[string]position, exclude map[link]bool, own map[pair]bool) unmarkedPlan {
	document := map[string]string{}
	sequence := make([]string, len(nodes))
	var sources []string
	for i, n := range nodes {
		document[n.ID] = n.Document
		sequence[i] = n.ID
		if n.Equation == "" {
			sources = append(sources, n.ID)
		}
	}
	slices.Sort(sources)
	slices.SortFunc(sequence, func(x, y string) int {
		return cmp.Or(comparePositions(pos[x], pos[y]), cmp.Compare(x, y))
	})
	index := make(map[string]int, len(sequence))
	for i, id := range sequence {
		index[id] = i
	}
	crossPaper := len(unmarkedLocalities) - 1
	ranked := rankShortlist(statements, sources, nil)
	out := unmarkedPlan{sources: sources, picks: map[string]unmarkedPicks{}}
	for _, s := range sources {
		inPool := func(t string) bool { return t != s && !exclude[linkOf(s, t)] && !own[pair{s, t}] }
		tier := func(t string) int {
			return slices.Index(unmarkedLocalities, unmarkedLocality(document[s], document[t]))
		}
		distance := func(t string) int { d := index[t] - index[s]; return max(d, -d) }
		var near []string
		for _, t := range sequence {
			if inPool(t) && tier(t) < crossPaper {
				near = append(near, t)
			}
		}
		slices.SortFunc(near, func(x, y string) int {
			return cmp.Or(cmp.Compare(tier(x), tier(y)), cmp.Compare(distance(x), distance(y)), cmp.Compare(index[x], index[y]))
		})
		p := unmarkedPicks{proximity: near[:min(shortlistProximityK, len(near))]}
		for _, t := range ranked[s] {
			if len(p.wildcard) == shortlistWildcardK {
				break
			}
			if inPool(t) && !slices.Contains(p.proximity, t) {
				p.wildcard = append(p.wildcard, t)
			}
		}
		out.picks[s] = p
		for _, t := range slices.Concat(p.proximity, p.wildcard) {
			out.pairs = append(out.pairs, pair{s, t})
		}
	}
	return out
}

// addAdjacent records the adjacency links among p's picks, each under both
// its ends.
func (p *unmarkedPlan) addAdjacent(adj []link) {
	p.adjacent += len(adj)
	for _, l := range adj {
		for _, end := range []pair{{l.a, l.b}, {l.b, l.a}} {
			picks := p.picks[end.source]
			picks.adjacent = append(picks.adjacent, end.target)
			p.picks[end.source] = picks
		}
	}
}

// unmarkedFound is every pair the record holds answered yes, sorted: the
// stage's yeses, a plan's or an earlier one's alike, since an answer is a
// fact about the text.
func unmarkedFound(r buildrecords.UnmarkedRecord) []pair {
	var out []pair
	for _, e := range r.Pairs {
		if e.Letter != nil && *e.Letter == asks.LetterPoints {
			out = append(out, pair{e.Source, e.Target})
		}
	}
	slices.SortFunc(out, comparePairs)
	return out
}

// unmarkedLinks is unmarkedFound unordered: one link per pair answered yes in
// either direction or both, sorted.
func unmarkedLinks(r buildrecords.UnmarkedRecord) []link {
	var out []link
	for _, p := range unmarkedFound(r) {
		out = append(out, linkOf(p.source, p.target))
	}
	slices.SortFunc(out, func(x, y link) int { return cmp.Or(cmp.Compare(x.a, y.a), cmp.Compare(x.b, y.b)) })
	return slices.Compact(out)
}

// unmarkedLocality is the nearest of unmarkedLocalities two kb-root-relative
// document paths share.
func unmarkedLocality(source, target string) string {
	dir := func(p string) string {
		if i := strings.LastIndex(p, "/"); i >= 0 {
			return p[:i]
		}
		return p
	}
	paper := func(p string) string { first, _, _ := strings.Cut(p, "/"); return first }
	switch {
	case source == target:
		return unmarkedLocalities[0]
	case dir(source) == dir(target):
		return unmarkedLocalities[1]
	case paper(source) == paper(target):
		return unmarkedLocalities[2]
	}
	return unmarkedLocalities[3]
}

// References is the references-found stage: the shortlist planned into the
// unmarked record before the first ask, then one letter ask per planned pair
// the record does not hold, grouped by source in ascending id, every ask of a
// group sharing the source's leaf and statement, each group's letters landing
// in the record as it completes. A yes is the only answer that yields
// anything, and it yields it to dependency attribution. It writes nothing
// under kb-root/, so no gate follows it; it exits on the record as it stands
// on disk: its plan against its outcomes, every planned pair carrying one.
func References(ctx context.Context, opts Options) (Report, error) {
	return runStage(opts, false, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		nodePass, err := requireNodePass(w.repoRoot)
		if err != nil {
			return nil, err
		}
		record, ok, err := buildrecords.ReadUnmarked(recordsAt(w.repoRoot))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, stopf("unmarked-record", "no unmarked record stands at %s. The declared pass writes one empty, and this stage resumes from nothing else", buildrecords.UnmarkedFile)
		}
		if err := passTwoGate(t); err != nil {
			return nil, err
		}
		inv, err := scanInventory(t, recs)
		if err != nil {
			return nil, err
		}
		g, err := readGraph(t, &inv)
		if err != nil {
			return nil, err
		}
		statementOf := statements(t, g, &inv)
		statement := map[string]string{}
		for _, n := range g.Nodes {
			statement[n.ID] = statementOf(n)
		}
		pos := claimPositions(t, g, &inv)
		marked := map[link]bool{}
		for _, c := range narrow(t, g, &inv, nodePass, nil).candidates {
			marked[linkOf(c.source.ID, c.target.ID)] = true
		}
		adj := adjacentLinks(t, g, &inv, pos, marked)
		exclude := maps.Clone(marked)
		for _, l := range adj {
			exclude[l] = true
		}
		planned := planUnmarked(g.Nodes, statement, pos, exclude, ownEquations(t, g, &inv))
		planned.addAdjacent(adj)
		plan := make([]buildrecords.PlannedPair, len(planned.pairs))
		for i, p := range planned.pairs {
			plan[i] = buildrecords.PlannedPair{p.source, p.target}
		}
		record.Planned = &plan
		if err := WriteUnmarked(w.repoRoot, record); err != nil {
			return nil, err
		}
		bodies := map[string]string{}
		body := func(document string) string {
			if _, ok := bodies[document]; !ok {
				bodies[document] = readLeaf(t.Documents[document], &inv).render.text
			}
			return bodies[document]
		}
		record, held, groups, err := askPlanned(ctx, w.repoRoot, record, g, statementOf, body, opts)
		findings := unmarkedFindings(planned, record, g, held, groups)
		if err != nil {
			return findings, askStop(err)
		}
		landed, _, err := buildrecords.ReadUnmarked(recordsAt(w.repoRoot))
		if err != nil {
			return findings, err
		}
		if missing := unanswered(landed); landed.Planned == nil || len(missing) > 0 {
			return findings, stopf("unmarked-coverage", "%d planned pair(s) of %s carry no outcome: %q", len(missing), buildrecords.UnmarkedFile, pairs(missing[:min(5, len(missing))]))
		}
		return append(findings, pass("stage-U-found", field("found", len(unmarkedFound(landed))), field("planned", len(*landed.Planned)))), nil
	})
}

// askPlanned asks every planned pair rec does not hold, one group per source
// in plan order, landing each group in the record as it completes: an
// answered or re-asked pair carries its letter, a defaulted one none. It
// returns the record, how many planned pairs it already held, and this run's
// groups. A call that never completes stops it with every completed group
// recorded. Each source's group is a unit of progress and each defaulted pair
// a fallback.
func askPlanned(ctx context.Context, repoRoot string, rec buildrecords.UnmarkedRecord, g *AuthoredGraph, statement func(ClaimNode) string, body func(document string) string, opts Options) (buildrecords.UnmarkedRecord, int, []asks.GroupRecord, error) {
	pending := unanswered(rec)
	held := 0
	if rec.Planned != nil {
		held = len(*rec.Planned) - len(pending)
	}
	var sources []string
	bySource := map[string][]string{}
	for _, p := range pending {
		if bySource[p.source] == nil {
			sources = append(sources, p.source)
		}
		bySource[p.source] = append(bySource[p.source], p.target)
	}
	var groups []asks.GroupRecord
	if err := opts.Progress.units(len(sources)); err != nil {
		return rec, held, groups, err
	}
	for _, id := range sources {
		source, _ := g.node(id)
		items := make([]asks.UnmarkedItem, len(bySource[id]))
		for i, targetID := range bySource[id] {
			target, _ := g.node(targetID)
			items[i] = asks.UnmarkedItem{Target: askClaim(target), Statement: statement(target)}
		}
		group, err := asks.AskGroup(ctx, asks.Unmarked, id,
			asks.UnmarkedAsks(source.Document, body(source.Document), askClaim(source), statement(source), items),
			opts.Reader, opts.ReaderConcurrency, opts.AskRecords)
		if err != nil {
			return rec, held, groups, err
		}
		groups = append(groups, group)
		for _, item := range group.Items {
			e := buildrecords.CandidateEntry{Source: id, Target: item.Item, Offered: item.Offered, Letter: item.Letter, Outcome: string(item.Outcome)}
			if item.Confidence != nil {
				e.Confidence = &item.Confidence
			}
			rec.Pairs = append(rec.Pairs, e)
			if item.Outcome == asks.Defaulted {
				if err := opts.Progress.fallback(result.Item{Check: checkDefaulted, Path: source.Document,
					Detail: fmt.Sprintf("%s -> %s was answered with no offered letter twice: it is recorded defaulted and yields no candidate", id, item.Item)}); err != nil {
					return rec, held, groups, err
				}
			}
		}
		if err := WriteUnmarked(repoRoot, rec); err != nil {
			return rec, held, groups, err
		}
		if err := opts.Progress.unit(id); err != nil {
			return rec, held, groups, err
		}
	}
	return rec, held, groups, nil
}

func unmarkedFindings(planned unmarkedPlan, rec buildrecords.UnmarkedRecord, g *AuthoredGraph, held int, groups []asks.GroupRecord) []Finding {
	yeses := unmarkedFound(rec)
	localities := map[string]int{}
	for _, p := range yeses {
		source, _ := g.node(p.source)
		target, _ := g.node(p.target)
		localities[unmarkedLocality(source.Document, target.Document)]++
	}
	outcome := map[pair]string{}
	for _, e := range rec.Pairs {
		outcome[pair{e.Source, e.Target}] = e.Outcome
	}
	var defaulted []pair
	for _, p := range planned.pairs {
		if outcome[p] == ClassifyDefaulted {
			defaulted = append(defaulted, p)
		}
	}
	picks := make(result.Record, len(planned.sources))
	for i, s := range planned.sources {
		p := planned.picks[s]
		picks[i] = field(s, recordOf(field("proximity", p.proximity), field("wildcard", p.wildcard), field("adjacent", p.adjacent)))
	}
	return []Finding{
		fact("stage-U-shortlist", field("sources", len(planned.sources)), field("proximity-k", shortlistProximityK), field("wildcard-k", shortlistWildcardK),
			field("adjacent", planned.adjacent), field("planned", len(planned.pairs)), field("held", held), field("asked", len(planned.pairs)-held)),
		fact("stage-U-asks", append(recordOf(field("sources", len(groups))), askTotals(groups)...)...),
		fact("stage-U-yeses", field("found", len(yeses)), field("by-locality", counts(unmarkedLocalities, localities))),
		fact("stage-U-defaulted", field("pairs", pairs(defaulted))),
		fact("stage-U-picks", picks...),
	}
}
