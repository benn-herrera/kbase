package claimgraph

import (
	"context"
	"fmt"
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
// ascending, each source's targets best first. excluded counts the ordered
// pairs left out of the pools as existing candidates, ownEquations those left
// out as a source's own equation.
type unmarkedPlan struct {
	sources      []string
	pairs        []pair
	excluded     int
	ownEquations int
}

// splitByKind is targets as two lists, those in blocks and the rest, each
// keeping targets' order.
func splitByKind(targets []string, blocks map[string]bool) (inBlocks, rest []string) {
	for _, t := range targets {
		if blocks[t] {
			inBlocks = append(inBlocks, t)
		} else {
			rest = append(rest, t)
		}
	}
	return inBlocks, rest
}

// planUnmarked is the stage's shortlist over the whole node set. Per source —
// every node but a minted equation — its pool is every other node less every
// pair already a candidate and every equation the source states, split into
// block claims and the rest, each best first; it plans the first
// shortlistRestK of the rest and, where the source is itself a block claim,
// the first shortlistBlockK of the block claims, those ahead of the rest. A
// pure function of the statements, keyed by node id, the block claims and the
// excluded pairs.
func planUnmarked(nodes []ClaimNode, statements map[string]string, blocks map[string]bool, candidatePairs []pair, own map[pair]bool) unmarkedPlan {
	var sources []string
	for _, n := range nodes {
		if n.Equation == "" {
			sources = append(sources, n.ID)
		}
	}
	slices.Sort(sources)
	ranked := rankShortlist(statements, sources, candidatePairs)
	out := unmarkedPlan{sources: sources}
	pool := len(statements) - 1
	for _, s := range sources {
		var kept []string
		for _, t := range ranked[s] {
			if !own[pair{s, t}] {
				kept = append(kept, t)
			}
		}
		blockTargets, rest := splitByKind(kept, blocks)
		var chosen []string
		if blocks[s] {
			chosen = blockTargets[:min(shortlistBlockK, len(blockTargets))]
		}
		for _, t := range slices.Concat(chosen, rest[:min(shortlistRestK, len(rest))]) {
			out.pairs = append(out.pairs, pair{s, t})
		}
		out.excluded += pool - len(ranked[s])
		out.ownEquations += len(ranked[s]) - len(kept)
	}
	return out
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
		if _, err := passTwoGate(t); err != nil {
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
		a := narrow(t, g, &inv, nodePass, nil)
		existing := make([]pair, len(a.candidates))
		for i, c := range a.candidates {
			existing[i] = c.pair()
		}
		blocks := map[string]bool{}
		for _, s := range blockClaimSites(g, &inv) {
			blocks[s.node.ID] = true
		}
		planned := planUnmarked(g.Nodes, statement, blocks, existing, ownEquations(t, g, &inv))
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
		source := g.node(id)
		items := make([]asks.UnmarkedItem, len(bySource[id]))
		for i, target := range bySource[id] {
			items[i] = asks.UnmarkedItem{Target: askClaim(g.node(target)), Statement: statement(g.node(target))}
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
		localities[unmarkedLocality(g.node(p.source).Document, g.node(p.target).Document)]++
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
	return []Finding{
		fact("stage-U-shortlist", field("sources", len(planned.sources)), field("rest-k", shortlistRestK), field("block-k", shortlistBlockK), field("planned", len(planned.pairs)),
			field("excluded-as-candidates", planned.excluded), field("excluded-as-own-equations", planned.ownEquations),
			field("held", held), field("asked", len(planned.pairs)-held)),
		fact("stage-U-asks", append(recordOf(field("sources", len(groups))), askTotals(groups)...)...),
		fact("stage-U-yeses", field("found", len(yeses)), field("by-locality", counts(unmarkedLocalities, localities))),
		fact("stage-U-defaulted", field("pairs", pairs(defaulted))),
	}
}
