package claimgraph

import (
	"encoding/json"
	"fmt"
	"maps"
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

// writer lands values through the write API's ops as the build's own, each
// batch followed by the derivation.
type writer struct {
	kbRoot, repoRoot string
	lg               log.Logger
}

// options is one op invocation of the writer's over data.
func (w writer) options(data []byte) write.Options {
	return write.Options{KBRoot: w.kbRoot, Values: data, Provenance: write.ProvenanceMachine, Logger: w.lg}
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

// uniqueTitle is title where taken does not hold it, else title followed by
// the first counter, " (2)" on, that makes it a title taken does not hold;
// taken then holds what it returns. A node's title is unique within its kind.
func uniqueTitle(title string, taken map[string]bool) string {
	unique := title
	for n := 2; taken[unique]; n++ {
		unique = fmt.Sprintf("%s (%d)", title, n)
	}
	taken[unique] = true
	return unique
}

// hostedTitle is a claim title in the leaf that hosts it.
type hostedTitle struct{ document, title string }

// addClaims lands one claim per entry through add-claim, each marked at the
// one line of its document its locator names, and returns the id each took,
// in order. Title and rationale are one line each, as add-claim stores them.
// An entry whose title a claim its document hosts already carries — one an
// earlier entry adds included — takes that claim's id and writes nothing, as
// the same block or sentence read twice is one claim. Any other entry's title
// is made unique among the KB's claims.
func (w writer) addClaims(name string, entries []entry) ([]string, error) {
	src := builtTree(w.kbRoot)
	g, err := kb.LoadGraph(src)
	if err != nil {
		return nil, err
	}
	taken, titles := map[string]bool{}, map[string]string{}
	for _, n := range g.Claims {
		taken[n.Title], titles[n.ID] = true, n.Title
	}
	texts := map[string][]string{}
	hosted := map[hostedTitle]string{}
	adding := map[hostedTitle]int{}
	ids := make([]string, len(entries))
	repeats := map[int]int{}
	var vs values
	var minting []int
	for i, e := range entries {
		lines, read := texts[e.document]
		if !read {
			text, err := src.ReadText(src.KBPath(e.document))
			if err != nil {
				return nil, err
			}
			lines = kb.SplitLines(text)
			texts[e.document] = lines
			for _, id := range slices.Sorted(maps.Keys(kb.MarkerLines(text))) {
				site := hostedTitle{e.document, titles[id]}
				if _, held := hosted[site]; !held && titles[id] != "" {
					hosted[site] = id
				}
			}
		}
		site := hostedTitle{e.document, kb.NormalizeSpace(e.title)}
		if id, ok := hosted[site]; ok {
			ids[i] = id
			continue
		}
		if j, ok := adding[site]; ok {
			repeats[i] = j
			continue
		}
		at := write.ExcerptLines(strings.Join(lines, "\n"), e.locator)
		if len(at) != 1 {
			return nil, stopf(name, "the locator %q names %d lines of %s, and a claim's marker goes on exactly one", e.locator, len(at), e.document)
		}
		adding[site] = i
		minting = append(minting, i)
		vs = append(vs, map[string]any{"title": uniqueTitle(site.title, taken), "rationale": kb.NormalizeSpace(e.rationale),
			"document": e.document, "line": at[0] + 1, "snippet": kb.NormalizeSpace(lines[at[0]])})
	}
	if len(vs) > 0 {
		res, err := w.run(name, write.AddClaim, vs)
		if err != nil {
			return nil, err
		}
		for k, id := range res.IDs() {
			ids[minting[k]] = id
		}
	}
	for i, j := range repeats {
		ids[i] = ids[j]
	}
	return ids, nil
}

// writePlan lands the declared pass's plan in the order the ops'
// preconditions force: the claims, each marked at its display line, the works,
// the off-graph edges.
func (w writer) writePlan(p plan) ([]Finding, error) {
	var findings []Finding
	var ids []string
	if len(p.entries) > 0 {
		var err error
		if ids, err = w.addClaims("pass-1", p.entries); err != nil {
			return findings, err
		}
	}
	docs := map[string]bool{}
	for _, e := range p.entries {
		docs[e.document] = true
	}
	findings = append(findings, pass("pass-1-add-claim", field("entries", len(ids)), field("documents", len(docs))))

	works, f, err := w.writeWorks(p.works)
	if err != nil {
		return findings, err
	}
	findings = append(findings, f)

	var edges []pair
	for _, r := range p.restsOn {
		edges = append(edges, pair{ids[r.entry], works[r.workKey]})
	}
	f, err = w.writeEdges("pass-3-rests-on", edges, nil)
	if err != nil {
		return findings, err
	}
	return append(findings, f), nil
}

// writeWorks adds one work record per external work through add-work, its
// citation key its label, every rigor the pending literal and every title
// unique among the KB's works, and returns the id add-work minted each, by
// citation key.
func (w writer) writeWorks(works []CitedWork) (map[string]string, Finding, error) {
	if len(works) == 0 {
		return nil, fact("pass-1b-add-work", field("works", 0), field("named", 0), field("key-only", 0)), nil
	}
	g, err := kb.LoadGraph(builtTree(w.kbRoot))
	if err != nil {
		return nil, Finding{}, err
	}
	taken := map[string]bool{}
	for _, n := range g.Works {
		taken[n.Title] = true
	}
	vs := make(values, len(works))
	named := 0
	for i, wk := range works {
		vs[i] = map[string]any{"label": wk.Key, "title": uniqueTitle(kb.NormalizeSpace(wk.Title), taken), "rigor": kb.PendingLiteral,
			"rationale": kb.NormalizeSpace(workRationale(wk))}
		if wk.Named {
			named++
		}
	}
	res, err := w.run("pass-1b", write.AddWork, vs)
	if err != nil {
		return nil, Finding{}, err
	}
	ids := map[string]string{}
	for i, e := range res.Entries {
		ids[works[i].Key] = e.ID
	}
	return ids, pass("pass-1b-add-work", field("works", len(works)), field("named", named), field("key-only", len(works)-named)), nil
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

// landLeaf lands one leaf's new claims in one batch and returns the id each
// took, in order, as addClaims lands them: a claim of that title the leaf
// already hosts is taken instead.
func (w writer) landLeaf(document string, claims []newClaim) ([]string, error) {
	if len(claims) == 0 {
		return nil, nil
	}
	entries := make([]entry, len(claims))
	for i, c := range claims {
		entries[i] = entry{title: c.title, rationale: c.rationale, document: document, locator: c.locator}
	}
	return w.addClaims("land-"+document, entries)
}

// landLeaves is landLeaf over several leaves in one batch, each entry naming
// its leaf, and returns the id each entry took, in order: exactly what landing
// the leaves one by one in entry order would.
func (w writer) landLeaves(entries []entry) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	return w.addClaims("land-leaves", entries)
}
