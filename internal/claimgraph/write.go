package claimgraph

import (
	"encoding/json"
	"slices"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/write"
)

// retryLimit bounds the identical re-issues of an op that answers retry.
const retryLimit = 3

// values is one op's values entries.
type values []map[string]any

// writer lands values through the write API's ops as the build's own, with no
// trailing refresh: kb_claimgraph stage G refreshes once the writes are in.
type writer struct {
	kbRoot, repoRoot string
	lg               log.Logger
}

// options is one op invocation of the writer's over data.
func (w writer) options(data []byte) write.Options {
	return write.Options{KBRoot: w.kbRoot, Values: data, NoRefresh: true, Provenance: write.ProvenanceMachine, Logger: w.lg}
}

// run issues op over vs, re-issuing an identical call on retry. Any outcome
// but done or unchanged stops the stage: the values are this stage's own.
func (w writer) run(name, op string, vs values) (write.Result, error) {
	data, err := json.Marshal(map[string]any{"entry": vs})
	if err != nil {
		return write.Result{}, err
	}
	var res write.Result
	for range retryLimit {
		res = write.Run(op, w.options(data))
		if res.Outcome != result.Retry {
			break
		}
	}
	switch res.Outcome {
	case result.Done, result.Unchanged:
		return res, nil
	case result.Retry:
		return res, stopf("concurrent-writer", "%s returned retry %d times over identical values; another writer holds the files this run needs: %s", op, retryLimit, refusalText(res))
	}
	return res, stopf(name, "%s ended %s and wrote nothing this stage can proceed from: %s", op, res.Outcome, refusalText(res))
}

// raw issues op over data once and returns its result whatever the outcome:
// the caller judges it.
func (w writer) raw(op string, data []byte) write.Result {
	return write.Run(op, w.options(data))
}

func refusalText(res write.Result) string {
	parts := make([]string, 0, len(res.Refusals)+len(res.Failures))
	for _, r := range res.Refusals {
		parts = append(parts, r.Key+": "+r.Detail)
	}
	for _, f := range res.Failures {
		parts = append(parts, f.Detail)
	}
	return strings.Join(parts, "; ")
}

// insertClaims inserts one claim per entry, each marked in its document at its
// locator in the call that mints it, and returns the id each took, in order.
func (w writer) insertClaims(name string, entries []entry) ([]string, error) {
	vs := make(values, len(entries))
	for i, e := range entries {
		vs[i] = map[string]any{"title": e.title, "rationale": e.rationale, "document": e.document, "locator": e.locator}
	}
	res, err := w.run(name, "insert-claim-entry", vs)
	return res.IDs, err
}

// writePlan lands the declared pass's plan in the order the ops'
// preconditions force: the claims, each marked at its display line, the works,
// the off-graph edges.
func (w writer) writePlan(p plan) ([]Finding, error) {
	var findings []Finding
	var ids []string
	if len(p.entries) > 0 {
		var err error
		if ids, err = w.insertClaims("pass-1", p.entries); err != nil {
			return findings, err
		}
	}
	docs := map[string]bool{}
	for _, e := range p.entries {
		docs[e.document] = true
	}
	findings = append(findings, pass("pass-1-insert-claim-entry", field("entries", len(ids)), field("documents", len(docs))))

	f, err := w.writeWorks(p.works)
	if err != nil {
		return findings, err
	}
	findings = append(findings, f)

	var edges []pair
	for _, r := range p.restsOn {
		edges = append(edges, pair{ids[r.entry], r.workID})
	}
	f, err = w.writeEdges("pass-3-rests-on", edges, nil)
	if err != nil {
		return findings, err
	}
	return append(findings, f), nil
}

// writeWorks inserts one work record per external work, every rigor the
// pending literal.
func (w writer) writeWorks(works []CitedWork) (Finding, error) {
	if len(works) == 0 {
		return fact("pass-1b-insert-work-entry", field("works", 0), field("named", 0), field("key-only", 0)), nil
	}
	vs := make(values, len(works))
	named := 0
	for i, wk := range works {
		vs[i] = map[string]any{"key": wk.Key, "title": wk.Title, "rigor": kb.PendingLiteral, "rationale": workRationale(wk)}
		if wk.Named {
			named++
		}
	}
	if _, err := w.run("pass-1b", "insert-work-entry", vs); err != nil {
		return Finding{}, err
	}
	return pass("pass-1b-insert-work-entry", field("works", len(works)), field("named", named), field("key-only", len(works)-named)), nil
}

// writeEdges lands every edge in one batch of the build's edge write, one
// entry per source carrying its premises and its references, so a refusal
// cannot land half of one claim's edges. Every premise is written pending: the
// build judges no edge, and a bearing is a human's to set.
func (w writer) writeEdges(name string, edges, references []pair) (Finding, error) {
	if len(edges) == 0 && len(references) == 0 {
		return fact(name, field("dependency-edges", 0), field("dependency-sources", 0), field("references", 0), field("reference-sources", 0)), nil
	}
	premises := map[string][]map[string]any{}
	refs := map[string][]map[string]any{}
	var sources []string
	for _, e := range edges {
		premises[e.source] = append(premises[e.source], map[string]any{"id": e.target, "bearing": kb.PendingLiteral})
		sources = append(sources, e.source)
	}
	for _, r := range references {
		refs[r.source] = append(refs[r.source], map[string]any{"id": r.target})
		sources = append(sources, r.source)
	}
	slices.Sort(sources)
	sources = slices.Compact(sources)
	vs := make(values, len(sources))
	for i, s := range sources {
		vs[i] = map[string]any{"id": s}
		if d, ok := premises[s]; ok {
			vs[i][kb.ListRestsOn] = d
		}
		if r, ok := refs[s]; ok {
			vs[i]["references"] = r
		}
	}
	if _, err := w.run(name, write.AddBuildEdges, vs); err != nil {
		return Finding{}, err
	}
	return pass(name, field("dependency-edges", len(edges)), field("dependency-sources", len(premises)),
		field("references", len(references)), field("reference-sources", len(refs))), nil
}

// newClaim is one claim a leaf gains after the declared pass: its title as its
// register page carries it, and the locator its marker is placed by.
type newClaim struct {
	title, rationale, locator string
}

// landLeaf lands one leaf's new claims in one act and returns the id each
// took, in order. Each is inserted with the leaf and its locator, so it is
// marked there in the call that mints it; a claim of that title the leaf
// already hosts, or one no leaf hosts, is adopted instead, by the insert's own
// rule.
func (w writer) landLeaf(document string, claims []newClaim) ([]string, error) {
	if len(claims) == 0 {
		return nil, nil
	}
	entries := make([]entry, len(claims))
	for i, c := range claims {
		entries[i] = entry{title: c.title, rationale: c.rationale, document: document, locator: c.locator}
	}
	return w.insertClaims("land-"+document, entries)
}

// landLeaves is landLeaf over several leaves in one insert batch, each entry
// naming its leaf, and returns the id each entry took, in order. It adopts
// exactly what landing the leaves one by one in entry order would: within the
// batch a minted claim joins the batch's graph and its marker the batch's
// hosts, which adoption reads.
func (w writer) landLeaves(entries []entry) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	return w.insertClaims("land-leaves", entries)
}
