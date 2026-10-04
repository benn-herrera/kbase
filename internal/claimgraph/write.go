package claimgraph

import (
	"encoding/json"
	"path/filepath"
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

// writer lands values through the write API's ops, with no trailing refresh:
// kb_claimgraph stage G refreshes once the writes are in.
type writer struct {
	kbRoot, repoRoot string
	lg               log.Logger
}

// run issues op over vs, re-issuing an identical call on retry. Any outcome
// but done or unchanged stops the stage: the values are this stage's own.
func (w writer) run(name, op string, vs values, create bool) (write.Result, error) {
	data, err := json.Marshal(map[string]any{"entry": vs})
	if err != nil {
		return write.Result{}, err
	}
	var res write.Result
	for range retryLimit {
		res = write.Run(op, write.Options{KBRoot: w.kbRoot, Values: data, Create: create, NoRefresh: true, Logger: w.lg})
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

// claimEntries is every claim entry of the register at kbRoot/register; none
// where there is no such register.
func claimEntries(kbRoot, register string) ([]kb.ClaimEntry, error) {
	p := filepath.Join(kbRoot, filepath.FromSlash(register))
	if !kb.IsFile(p) {
		return nil, nil
	}
	text, err := kb.ReadText(p)
	if err != nil {
		return nil, err
	}
	return kb.ParseClaimEntries(text, register, nil, log.Discard()), nil
}

// insertClaims inserts one entry per title into register and returns the id
// each took, in order.
func (w writer) insertClaims(name, register string, titles, rationales []string) ([]string, error) {
	vs := make(values, len(titles))
	for i := range titles {
		vs[i] = map[string]any{"register": register, "title": titles[i], "rigor": kb.PendingLiteral, "rationale": rationales[i]}
	}
	res, err := w.run(name, "insert-claim-entry", vs, true)
	return res.IDs, err
}

// writePlan lands the declared pass's plan in the order the ops'
// preconditions force: the claim entries, the works, every document's
// frontmatter, the off-graph edges, the markers.
func (w writer) writePlan(p plan) ([]Finding, error) {
	var findings []Finding
	minted := map[int]string{}
	for _, register := range p.registers() {
		var positions []int
		var titles, rationales []string
		for i, e := range p.entries {
			if e.register == register {
				positions = append(positions, i)
				titles = append(titles, registerTitle(e.title, e.document))
				rationales = append(rationales, e.rationale)
			}
		}
		ids, err := w.insertClaims("pass-1", register, titles, rationales)
		if err != nil {
			return findings, err
		}
		for i, pos := range positions {
			minted[pos] = ids[i]
		}
		findings = append(findings, pass("pass-1-insert-claim-entry", field("register", register), field("entries", len(ids))))
	}

	f, err := w.writeWorks(p.works)
	if err != nil {
		return findings, err
	}
	findings = append(findings, f)

	var fm values
	for _, d := range p.documents {
		e := map[string]any{"document": d.path, "kind": d.kind}
		if len(d.claims) > 0 {
			ids := make([]string, len(d.claims))
			for i, pos := range d.claims {
				ids[i] = minted[pos]
			}
			e["claims"] = ids
		}
		if d.noClaim != "" {
			e["no-claim"] = d.noClaim
		}
		fm = append(fm, e)
	}
	if _, err := w.run("pass-2", "set-frontmatter", fm, false); err != nil {
		return findings, err
	}
	findings = append(findings, pass("pass-2-set-frontmatter", field("documents", len(fm))))

	var edges []pair
	for _, r := range p.restsOn {
		edges = append(edges, pair{minted[r.entry], r.workID})
	}
	f, err = w.writeEdges("pass-3-rests-on", edges, nil)
	if err != nil {
		return findings, err
	}
	findings = append(findings, f)

	if len(p.markers) > 0 {
		vs := make(values, len(p.markers))
		for i, m := range p.markers {
			vs[i] = map[string]any{"document": m.document, "id": minted[m.entry], "locator": m.locator}
		}
		if _, err := w.run("pass-4", "mark-claim-in-leaf", vs, false); err != nil {
			return findings, err
		}
	}
	docs := map[string]bool{}
	for _, m := range p.markers {
		docs[m.document] = true
	}
	findings = append(findings, pass("pass-4-mark-claim-in-leaf", field("markers", len(p.markers)), field("documents", len(docs))))
	return findings, nil
}

// writeWorks inserts one entry per external work into the KB root's register,
// every strength the pending literal.
func (w writer) writeWorks(works []CitedWork) (Finding, error) {
	if len(works) == 0 {
		return fact("pass-1b-insert-work-entry", field("works", 0), field("named", 0), field("key-only", 0)), nil
	}
	vs := make(values, len(works))
	named := 0
	for i, wk := range works {
		vs[i] = map[string]any{"register": kb.RegisterFile, "key": wk.Key, "title": wk.Title, "strength": kb.PendingLiteral, "rationale": workRationale(wk)}
		if wk.Named {
			named++
		}
	}
	if _, err := w.run("pass-1b", "insert-work-entry", vs, true); err != nil {
		return Finding{}, err
	}
	return pass("pass-1b-insert-work-entry", field("works", len(works)), field("named", named), field("key-only", len(works)-named)), nil
}

// writeEdges lands every edge in one add-depends-on batch, one entry per
// source carrying both its dependencies and its references, so a refusal
// cannot land half of one claim's edges. A work target's applicability is
// left for the op to render pending.
func (w writer) writeEdges(name string, edges, references []pair) (Finding, error) {
	if len(edges) == 0 && len(references) == 0 {
		return fact(name, field("dependency-edges", 0), field("dependency-sources", 0), field("references", 0), field("reference-sources", 0)), nil
	}
	depends := map[string][]map[string]any{}
	refs := map[string][]map[string]any{}
	var sources []string
	for _, e := range edges {
		depends[e.source] = append(depends[e.source], map[string]any{"id": e.target})
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
		if d, ok := depends[s]; ok {
			vs[i]["depends-on"] = d
		}
		if r, ok := refs[s]; ok {
			vs[i]["references"] = r
		}
	}
	if _, err := w.run(name, "add-depends-on", vs, false); err != nil {
		return Finding{}, err
	}
	return pass(name, field("dependency-edges", len(edges)), field("dependency-sources", len(depends)),
		field("references", len(references)), field("reference-sources", len(refs))), nil
}

// newClaim is one claim a leaf gains after the declared pass. Locator is what
// its marker is placed by, "" for an equation, which takes none.
type newClaim struct {
	title, rationale, locator string
}

// landLeaf is one leaf's final state after the declared pass, in one act,
// and returns the ids claims took. A claim whose title the leaf's register
// already carries under an id this leaf hosts, or under one no document
// hosts, takes that id. The frontmatter is the leaf's block as it stands with
// the new ids after its claims; markers cover a new claim with a locator however few the leaf declares, and
// a block claim where the leaf's claims other than equations number two or
// more — blocks maps each block claim's id to its display line.
func (w writer) landLeaf(document, kind string, claims []newClaim, blocks map[string]string, elsewhere map[string]bool) ([]string, error) {
	text, err := kb.ReadText(filepath.Join(w.kbRoot, filepath.FromSlash(document)))
	if err != nil {
		return nil, err
	}
	fm := kb.ParseFrontmatter(text)
	existing, err := fm.ListOrEmpty("claims")
	if err != nil {
		return nil, err
	}
	register := registerFor(document)
	held, err := claimEntries(w.kbRoot, register)
	if err != nil {
		return nil, err
	}
	landed := map[string]string{}
	for _, e := range held {
		if slices.Contains(existing, e.ID) || !elsewhere[e.ID] {
			landed[e.Title] = e.ID
		}
	}
	titles := make([]string, len(claims))
	var freshTitles, freshRationales []string
	for i, c := range claims {
		titles[i] = registerTitle(c.title, document)
		if _, ok := landed[titles[i]]; !ok {
			freshTitles = append(freshTitles, titles[i])
			freshRationales = append(freshRationales, c.rationale)
		}
	}
	stem := "land-" + document
	if len(freshTitles) > 0 {
		ids, err := w.insertClaims(stem, register, freshTitles, freshRationales)
		if err != nil {
			return nil, err
		}
		for i, t := range freshTitles {
			landed[t] = ids[i]
		}
		if held, err = claimEntries(w.kbRoot, register); err != nil {
			return nil, err
		}
	}
	ids := make([]string, len(titles))
	for i, t := range titles {
		ids[i] = landed[t]
	}
	final := slices.Clone(existing)
	for _, id := range ids {
		if !slices.Contains(final, id) {
			final = append(final, id)
		}
	}
	if !slices.Equal(final, existing) {
		e, err := write.FrontmatterValues(text, document)
		if err != nil {
			return nil, err
		}
		if e == nil {
			e = map[string]any{"document": document}
		}
		// A leaf declares its claims or a reason it has none, never both
		// (tier-1 coverage), so one gaining a claim drops its reason.
		delete(e, "no-claim")
		e["kind"], e["claims"] = kind, final
		if _, err := w.run(stem, "set-frontmatter", values{e}, false); err != nil {
			return nil, err
		}
		if text, err = kb.ReadText(filepath.Join(w.kbRoot, filepath.FromSlash(document))); err != nil {
			return nil, err
		}
	}
	heldTitles := map[string]string{}
	for _, e := range held {
		heldTitles[e.ID] = e.Title
	}
	counted := 0
	for _, id := range final {
		if _, isEquation := kb.EquationLabel(heldTitles[id]); !isEquation {
			counted++
		}
	}
	locators := map[string]string{}
	for id, l := range blocks {
		locators[id] = l
	}
	prose := map[string]bool{}
	for i, c := range claims {
		if c.locator != "" {
			locators[ids[i]], prose[ids[i]] = c.locator, true
		}
	}
	var wanted values
	for _, id := range final {
		l, ok := locators[id]
		if ok && (prose[id] || counted > 1) && !strings.Contains(text, write.Tier2Marker(id)) {
			wanted = append(wanted, map[string]any{"document": document, "id": id, "locator": l})
		}
	}
	if len(wanted) > 0 {
		if _, err := w.run(stem, "mark-claim-in-leaf", wanted, false); err != nil {
			return nil, err
		}
	}
	return ids, nil
}
