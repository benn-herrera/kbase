package claimgraph

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/records"
	"kbase/internal/result"
)

// harvestedLink is a link with the harvest that found it: a marked harvest's
// name, harvestUnmarked, harvestAdjacent or harvestCulled.
type harvestedLink struct {
	link
	harvest string
}

// cullAnswer is one co-linearity verdict over a link, as
// buildrecords.VerdictAAddsNothing, VerdictBAddsNothing or VerdictDistinct.
type cullAnswer struct {
	link
	verdict string
}

// named is the claim the answer names as adding nothing and its partner; ok
// is false for any other verdict.
func (c cullAnswer) named() (culled, into string, ok bool) {
	switch c.verdict {
	case buildrecords.VerdictAAddsNothing:
		return c.a, c.b, true
	case buildrecords.VerdictBAddsNothing:
		return c.b, c.a, true
	}
	return "", "", false
}

// cullTargets is each culled prose claim's target. Each answer naming a claim
// as adding nothing culls it into its partner, except that a claim outside
// prose is never culled: the prose claim of the pair is culled into it
// instead, and a pair with no prose claim culls nothing. A claim culled by
// several answers targets the earliest of their partners in reading order,
// ties by id. A cycle among targets is broken by removing the target of the
// cycle's earliest claim in reading order.
func cullTargets(answers []cullAnswer, prose map[string]bool, pos map[string]position) map[string]string {
	earlier := func(x, y string) bool {
		return cmp.Or(comparePositions(pos[x], pos[y]), cmp.Compare(x, y)) < 0
	}
	targets := map[string]string{}
	for _, a := range answers {
		culled, into, ok := a.named()
		if !ok {
			continue
		}
		if !prose[culled] {
			culled, into = into, culled
		}
		if !prose[culled] {
			continue
		}
		if t, ok := targets[culled]; !ok || earlier(into, t) {
			targets[culled] = into
		}
	}
	var breaks []string
	done := map[string]bool{}
	for _, start := range slices.Sorted(maps.Keys(targets)) {
		onPath := map[string]bool{}
		x, cycle := start, false
		for !done[x] {
			if onPath[x] {
				cycle = true
				break
			}
			onPath[x] = true
			next, ok := targets[x]
			if !ok {
				break
			}
			x = next
		}
		if cycle {
			first := x
			for y := targets[x]; y != x; y = targets[y] {
				if earlier(y, first) {
					first = y
				}
			}
			breaks = append(breaks, first)
		}
		for y := range onPath {
			done[y] = true
		}
	}
	for _, id := range breaks {
		delete(targets, id)
	}
	return targets
}

// cullRoots is each culled claim's root: its targets followed to a claim with
// none. targets must hold no cycle, as cullTargets returns them.
func cullRoots(targets map[string]string) map[string]string {
	roots := make(map[string]string, len(targets))
	for c, r := range targets {
		for next, ok := targets[r]; ok; next, ok = targets[r] {
			r = next
		}
		roots[c] = r
	}
	return roots
}

func compareLinks(x, y link) int {
	return cmp.Or(cmp.Compare(x.a, y.a), cmp.Compare(x.b, y.b))
}

// confirmPairs is the link (c, root(c)) for every culled c whose root is not
// its target, sorted.
func confirmPairs(targets, roots map[string]string) []link {
	var out []link
	for c, t := range targets {
		if r := roots[c]; r != t {
			out = append(out, linkOf(c, r))
		}
	}
	slices.SortFunc(out, compareLinks)
	return out
}

// confirmedCulls is the set of confirm links whose answer names the culled
// claim, not its root, as adding nothing.
func confirmedCulls(answers []cullAnswer, roots map[string]string) map[link]bool {
	out := map[link]bool{}
	for _, a := range answers {
		if culled, into, ok := a.named(); ok && roots[culled] == into {
			out[a.link] = true
		}
	}
	return out
}

// finalCulls maps each claim the cull drops to its root: a claim whose root is
// its target, and a claim whose confirm link is confirmed. Every other claim
// is kept.
func finalCulls(targets, roots map[string]string, confirmed map[link]bool) map[string]string {
	out := map[string]string{}
	for c, t := range targets {
		if r := roots[c]; r == t || confirmed[linkOf(c, r)] {
			out[c] = r
		}
	}
	return out
}

// CullPlan is the culled claims in id order and every inferred link that
// survives the cull, sorted.
type CullPlan struct {
	Culls []string
	Links []buildrecords.CullLink
}

// harvestRank orders a link's harvests, the lowest taking precedence: a marked
// harvest, then unmarked, adjacent, culled.
func harvestRank(h string) int {
	switch h {
	case harvestUnmarked:
		return 1
	case harvestAdjacent:
		return 2
	case harvestCulled:
		return 3
	}
	return 0
}

// PlanCull re-points every link through into on both ends. A link coinciding
// with another keeps the harvest of higher precedence. A self-link is dropped;
// an untouched marked link is withheld, as is a re-pointed link equal to one;
// an untouched inferred link keeps its harvest; every other re-pointed link,
// a marked one included, is inferred with harvest culled. into maps each
// culled claim to its root: an into value that is itself a key, or an id holds
// rejects, is an error.
func PlanCull(links []harvestedLink, into map[string]string, holds func(string) bool) (CullPlan, error) {
	culls := slices.Sorted(maps.Keys(into))
	for _, id := range culls {
		root := into[id]
		for _, x := range []string{id, root} {
			if !holds(x) {
				return CullPlan{}, fmt.Errorf("the cull of %s into %s names %s, which the claim graph does not hold", id, root, x)
			}
		}
		if next, ok := into[root]; ok {
			return CullPlan{}, fmt.Errorf("%s is culled into %s, which is itself culled into %s", id, root, next)
		}
	}
	best := map[link]string{}
	for _, l := range links {
		h, ok := best[l.link]
		if !ok || cmp.Or(cmp.Compare(harvestRank(l.harvest), harvestRank(h)), cmp.Compare(l.harvest, h)) < 0 {
			best[l.link] = l.harvest
		}
	}
	repoint := func(id string) string {
		if r, ok := into[id]; ok {
			return r
		}
		return id
	}
	marked := map[link]bool{}
	kept := map[link]string{}
	repointed := map[link]bool{}
	for l, h := range best {
		r := linkOf(repoint(l.a), repoint(l.b))
		switch {
		case r.a == r.b:
		case r != l:
			repointed[r] = true
		case harvestRank(h) == 0:
			marked[l] = true
		default:
			kept[l] = h
		}
	}
	for r := range repointed {
		if _, ok := kept[r]; !ok && !marked[r] {
			kept[r] = harvestCulled
		}
	}
	plan := CullPlan{Culls: culls}
	for _, l := range slices.SortedFunc(maps.Keys(kept), compareLinks) {
		plan.Links = append(plan.Links, buildrecords.CullLink{A: l.a, B: l.b, Harvest: kept[l]})
	}
	return plan, nil
}

// cullAsk is one co-linearity ask: its link, and the claim its item is named
// by.
type cullAsk struct {
	link
	key string
}

// Culled is the co-linearity cull. Every link with a prose end — the marked
// pairs, the unmarked record's yeses and the adjacency links, unordered — is
// asked once, grouped by its earlier claim in reading order and named by the
// later. Every culled claim whose root is not its target is asked once more
// against its root, grouped by the root and named by the claim. Once both
// passes' asks are done, the culls are issued as one batch per drop op, the
// inferred links that survive them re-pointed, and the cull record written
// once. A defaulted ask keeps both claims.
func Culled(ctx context.Context, opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		nodePass, err := requireNodePass(w.repoRoot)
		if err != nil {
			return nil, err
		}
		unmarked, _, err := buildrecords.ReadUnmarked(recordsAt(w.repoRoot))
		if err != nil {
			return nil, err
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
		pos := claimPositions(t, g, &inv)
		prose := proseClaims(g, &inv)
		say := statements(t, g, &inv)

		var links []harvestedLink
		marked := map[link]bool{}
		for _, c := range narrow(t, g, &inv, nodePass, nil).candidates {
			l := linkOf(c.source.ID, c.target.ID)
			marked[l] = true
			for _, h := range c.harvests {
				links = append(links, harvestedLink{l, h})
			}
		}
		for _, l := range unmarkedLinks(unmarked) {
			links = append(links, harvestedLink{l, harvestUnmarked})
		}
		for _, l := range adjacentLinks(t, g, &inv, pos, marked) {
			links = append(links, harvestedLink{l, harvestAdjacent})
		}

		seen := map[link]bool{}
		var candidates []link
		for _, l := range links {
			if (prose[l.a] || prose[l.b]) && !seen[l.link] {
				seen[l.link] = true
				candidates = append(candidates, l.link)
			}
		}
		slices.SortFunc(candidates, compareLinks)
		first := map[string][]cullAsk{}
		for _, l := range candidates {
			earlier, later := shownOrder(l, pos)
			first[earlier] = append(first[earlier], cullAsk{l, later})
		}
		answers1, entries, groups1, wall1, err := askCull(ctx, opts, 1, first, g, say, pos)
		if err != nil {
			return nil, askStop(err)
		}

		targets := cullTargets(answers1, prose, pos)
		roots := cullRoots(targets)
		confirm := confirmPairs(targets, roots)
		second := map[string][]cullAsk{}
		for _, l := range confirm {
			c, root := l.a, l.b
			if roots[c] != root {
				c, root = l.b, l.a
			}
			second[root] = append(second[root], cullAsk{l, c})
		}
		answers2, entries2, groups2, wall2, err := askCull(ctx, opts, 2, second, g, say, pos)
		if err != nil {
			return nil, askStop(err)
		}
		if err := asks.WriteGroupRecords(opts.AskRecords, slices.Concat(groups1, groups2)); err != nil {
			return nil, err
		}
		entries = append(entries, entries2...)

		into := finalCulls(targets, roots, confirmedCulls(answers2, roots))
		plan, err := PlanCull(links, into, func(id string) bool { _, ok := g.node(id); return ok })
		if err != nil {
			return nil, stopf("cull-plan", "%v", err)
		}
		if _, err := IssueCulls(plan.Culls, w.raw); err != nil {
			return nil, err
		}
		rec := buildrecords.CullRecord{Entries: entries, Links: plan.Links}
		intoRoots := make([]string, len(plan.Culls))
		for i, id := range plan.Culls {
			rec.Culls = append(rec.Culls, buildrecords.Cull{ID: id, Into: into[id]})
			intoRoots[i] = into[id]
		}
		if err := WriteCull(w.repoRoot, rec); err != nil {
			return nil, err
		}
		oneStep := 0
		for _, a := range answers1 {
			if _, _, ok := a.named(); ok {
				oneStep++
			}
		}
		return []Finding{
			fact("stage-cull-asks-1", askTotals(groups1, wall1)...),
			fact("stage-cull-asks-2", askTotals(groups2, wall2)...),
			pass("stage-cull", field("candidates", len(candidates)), field("one-step", oneStep), field("confirm-asked", len(confirm)),
				field("culled", len(plan.Culls)), field("by-root", tally(intoRoots)), field("links", len(plan.Links))),
		}, nil
	})
}

// shownOrder is l's two claims in reading order, ties by id.
func shownOrder(l link, pos map[string]position) (first, second string) {
	if cmp.Or(comparePositions(pos[l.b], pos[l.a]), cmp.Compare(l.b, l.a)) < 0 {
		return l.b, l.a
	}
	return l.a, l.b
}

// askCull is one pass of co-linearity asks, one group per key of groups in
// ascending order, its id the pass and the key, the pass one ask batch. Each
// answer is mapped from the order the claims were shown in onto the link's id
// order; a defaulted ask is distinct, and a fallback. Each group is a unit of
// progress. Answers, units and fallbacks are taken once the batch has
// drained. It returns the pass's answers, entries and group records, and the
// batch's elapsed time.
func askCull(ctx context.Context, opts Options, pass int, groups map[string][]cullAsk, g *AuthoredGraph, say func(ClaimNode) string, pos map[string]position) ([]cullAnswer, []buildrecords.CullEntry, []asks.GroupRecord, time.Duration, error) {
	var answers []cullAnswer
	var entries []buildrecords.CullEntry
	ids := slices.Sorted(maps.Keys(groups))
	if err := opts.Progress.units(len(ids)); err != nil {
		return nil, nil, nil, 0, err
	}
	addsNothing := func(l link, id string) string {
		if id == l.a {
			return buildrecords.VerdictAAddsNothing
		}
		return buildrecords.VerdictBAddsNothing
	}
	name := func(id string) string { return fmt.Sprintf("p%d-%s", pass, id) }
	batch := opts.batch(ctx)
	for _, id := range ids {
		items := make([]asks.ColinearItem, len(groups[id]))
		for i, a := range groups[id] {
			first, second := shownOrder(a.link, pos)
			fn, _ := g.node(first)
			sn, _ := g.node(second)
			items[i] = asks.ColinearItem{Key: a.key, First: askClaim(fn), Second: askClaim(sn), FirstText: say(fn), SecondText: say(sn)}
		}
		if err := batch.AskGroup(asks.Colinear, name(id), asks.ColinearAsks(items)); err != nil {
			break
		}
	}
	records, err := batch.Wait()
	if err != nil {
		return nil, nil, nil, 0, err
	}
	for k, id := range ids {
		for i, it := range records[k].Items {
			a := groups[id][i]
			first, second := shownOrder(a.link, pos)
			verdict := buildrecords.VerdictDistinct
			if it.Choice != nil {
				switch *it.Choice {
				case asks.LetterSecondAddsNothing:
					verdict = addsNothing(a.link, second)
				case asks.LetterFirstAddsNothing:
					verdict = addsNothing(a.link, first)
				}
			}
			answers = append(answers, cullAnswer{a.link, verdict})
			entries = append(entries, buildrecords.CullEntry{A: a.a, B: a.b, Pass: pass, Verdict: verdict, Outcome: string(it.Outcome)})
			if it.Outcome == asks.Defaulted {
				n, _ := g.node(a.key)
				if err := opts.Progress.fallback(result.Item{Check: checkDefaulted, Path: n.Document,
					Detail: fmt.Sprintf("the co-linearity ask over %s and %s was answered with no offered letter twice: it is recorded distinct and both claims keep", a.a, a.b)}); err != nil {
					return answers, entries, records, batch.Elapsed(), err
				}
			}
		}
		if err := opts.Progress.unit(name(id)); err != nil {
			return answers, entries, records, batch.Elapsed(), err
		}
	}
	return answers, entries, records, batch.Elapsed(), nil
}
