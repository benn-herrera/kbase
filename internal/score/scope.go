package score

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/argmark"
	"kbase/internal/asks"
	"kbase/internal/buildstore"
	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
)

// recordedHashes is the judged hash each scored record of src carries, by the
// key the ledger names its item by.
func recordedHashes(src *kb.Source) (map[string]kb.JudgedHash, error) {
	g, err := kb.LoadGraph(src)
	if err != nil {
		return nil, err
	}
	out := map[string]kb.JudgedHash{}
	node := func(kind, id string, h kb.JudgedHash) {
		out[itemKey(query.Item{Node: &query.ScoringNode{Kind: kind, ID: id}})] = h
	}
	for _, n := range g.Claims {
		node(kb.NodeKindClaim, n.ID, n.JudgedHash)
	}
	for _, n := range g.Analyses {
		node(kb.NodeKindAnalysis, n.ID, n.JudgedHash)
	}
	for _, n := range g.Experiments {
		node(kb.NodeKindExperiment, n.ID, n.JudgedHash)
	}
	for _, e := range g.Scorable {
		out[itemKey(query.Item{Edge: &query.ScoringEdge{Source: e.Source, Target: e.Target, Relation: e.Relation}})] = e.JudgedHash
	}
	return out, nil
}

// judgedHash is the judged hash of it over g's statements as they stand: what
// a value landed now is judged against.
func judgedHash(g graph, it query.Item) (kb.JudgedHash, error) {
	if it.Node != nil {
		return asks.NodeJudgedHash(it.Node.Kind, g.statements[it.Node.ID])
	}
	return asks.EdgeJudgedHash(g.statements[it.Edge.Source], g.statements[it.Edge.Target])
}

// dirty is whether it's recorded hash differs from the hash of what it would be
// judged against now. A value never judged records all zeros, which no
// computed hash is, so it is dirty.
func dirty(g graph, recorded map[string]kb.JudgedHash, it query.Item) (bool, error) {
	h, err := judgedHash(g, it)
	if err != nil {
		return false, err
	}
	return recorded[itemKey(it)] != h, nil
}

// scopeOf is the set a fresh invocation walks: the node or the edge it names,
// dirty or not, or, for a full run, each item of its pass that is dirty or,
// under rescore-manual, manually scored. A named item no pass asks, one that
// does not stand, and a manually scored one without rescore-manual are
// refused.
func scopeOf(ix *query.Index, g graph, recorded map[string]kb.JudgedHash, inv Invocation) ([]query.Item, error) {
	switch {
	case inv.Node != "":
		it, err := namedNode(ix, inv.Node)
		if err != nil {
			return nil, err
		}
		return namedItem(ix, it, argmark.Flag("node"), inv.Rescore)
	case inv.Edge != nil:
		it, err := namedEdge(ix, *inv.Edge)
		if err != nil {
			return nil, err
		}
		return namedItem(ix, it, argmark.Flag("edge"), inv.Rescore)
	}
	candidates, err := query.ScoringSet(ix, inv.Pass)
	if err != nil {
		return nil, err
	}
	var items []query.Item
	for _, it := range candidates {
		stale, err := dirty(g, recorded, it)
		if err != nil {
			return nil, err
		}
		if stale || inv.Rescore && ix.ManuallyScored(it) {
			items = append(items, it)
		}
	}
	return items, nil
}

// namedItem is the one item a run naming it at key walks, refused where its
// value is manually scored and rescore does not say to re-judge it.
func namedItem(ix *query.Index, it query.Item, key string, rescore bool) ([]query.Item, error) {
	if !rescore && ix.ManuallyScored(it) {
		args := map[string]any{"rescore-manual": true}
		if it.Edge != nil {
			args["edge"] = map[string]any{"source": it.Edge.Source, "target": it.Edge.Target, "relation": it.Edge.Relation}
		} else {
			args["node"] = it.Node.ID
		}
		return nil, result.Refusal{{Check: result.CheckUsage, Key: key,
			Detail: ItemName(it) + "'s value is manually scored, a person's judgement, which a run re-judges only where " +
				argmark.Flag("rescore-manual") + " says to",
			Remedy: "re-run as " + argmark.Call("score", args) + " to re-judge it, the run's value replacing the person's; or leave the value standing"}}
	}
	return []query.Item{it}, nil
}

// namedNode is the rigor item of the node id, refused where no node stands
// under it and where it is a work or an axiom, which no pass asks.
func namedNode(ix *query.Index, id string) (query.Item, error) {
	key := argmark.Flag("node")
	row, ok := ix.Node(id)
	if !ok {
		detail, remedy := kb.NoNode(key, id)
		return query.Item{}, result.Refusal{{Check: kb.CheckUnknownID, Key: key, Detail: detail, Remedy: remedy}}
	}
	var why string
	switch row.NodeType {
	case kb.NodeKindWork:
		why = id + " is a work, whose rigor is judged by hand with the document in hand"
	case kb.NodeKindAxiom:
		why = id + " is an axiom, whose rigor is 1.0 by rule"
	default:
		return query.Item{Node: &query.ScoringNode{Kind: row.NodeType, ID: id}}, nil
	}
	return query.Item{}, result.Refusal{{Check: result.CheckUsage, Key: key, Detail: why + ": no pass asks it",
		Remedy: "name a claim, an analysis or an experiment in " + key}}
}

// namedEdge is the bearing item of the edge e names, refused where its
// relation is one no pass asks and where no such edge stands.
func namedEdge(ix *query.Index, e query.ScoringEdge) (query.Item, error) {
	key := argmark.Flag("edge")
	it := query.Item{Edge: &e}
	var why string
	switch e.Relation {
	case kb.RelationFounds:
		why = "a founds edge's bearing is judged by hand, the work or axiom it rests on lying outside the corpus"
	case kb.RelationReferences:
		why = "a references edge carries no bearing"
	case kb.RelationDemoted:
		why = "a demoted record carries the replaced edge's bearing until it is restored"
	}
	walked := kb.BearingWalkRelations
	if why != "" {
		return query.Item{}, result.Refusal{{Check: result.CheckUsage, Key: key, Detail: why + ": no pass asks it",
			Remedy: "name a " + strings.Join(walked[:len(walked)-1], ", ") + " or " + walked[len(walked)-1] + " edge in " + key}}
	}
	if _, ok := ix.Edge(e); !ok || !slices.Contains(walked, e.Relation) {
		return query.Item{}, result.Refusal{{Check: kb.CheckUnknownID, Key: key, Detail: "no edge " + ItemName(it),
			Remedy: "list the edge records that stand with " + argmark.Call("edges", nil) + ", correct " + key + ", then re-run"}}
	}
	return it, nil
}

// ResumeTarget is the header of the run a resume continues in the state
// directory dir: its most recent real run, which the resume continues with the
// scope that run recorded. It is refused where no real run stands, where that
// run finished — no item failed or was left unreached — and where a build has
// started since it; each refusal's remedy is a fresh run, which asks only what
// is dirty.
func ResumeTarget(dir string) (RunHeader, error) {
	refuse := func(detail string) error {
		return result.Refusal{{Check: result.CheckUsage, Key: argmark.Flag("resume"), Path: RunsDir(dir), Detail: detail,
			Remedy: "start a fresh " + argmark.Call("score", nil) + ", which asks only what is dirty"}}
	}
	run, started, err := newestRun(dir, func(name string) bool { return runKind(name) == RunKindReal })
	if err != nil {
		return RunHeader{}, err
	}
	if run == "" {
		return RunHeader{}, refuse("no score run stands here to resume")
	}
	st, err := ReadRun(dir, run)
	if err != nil {
		return RunHeader{}, err
	}
	if st.Counts.Failed == 0 && st.Counts.Remaining == 0 {
		return RunHeader{}, refuse("the most recent score run, " + run + ", finished: no item of it failed or was left unreached")
	}
	build, err := buildstore.ReadState(dir)
	if err != nil {
		return RunHeader{}, err
	}
	if startedAfter(build.Started, started) {
		return RunHeader{}, refuse(fmt.Sprintf("a build started at %s, after the most recent score run, %s, started at %s",
			build.Started, run, started))
	}
	return readHeader(filepath.Join(RunsDir(dir), run))
}

// readItems is the item list items.jsonl records in the run directory runDir:
// the set the run enumerated, in its order.
func readItems(runDir string) ([]query.Item, error) {
	recorded, err := readAttempts(filepath.Join(runDir, itemsFile))
	if err != nil {
		return nil, err
	}
	items := make([]query.Item, len(recorded))
	for i, a := range recorded {
		items[i] = a.asItem()
	}
	return items, nil
}
