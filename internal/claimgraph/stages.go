package claimgraph

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/records"
	"kbase/internal/result"
)

// Options is one claim-graph stage over the tree at KBRoot, the repository
// whose root holds the build records, and the document graph's records.
type Options struct {
	KBRoot   string
	RepoRoot string
	Records  string
	Logger   log.Logger
	// Reader puts each letter ask. It is nil only for a build spending no
	// inference, which drops the node pass and classifies every candidate as
	// its draft.
	Reader asks.LetterReader
	// ReaderConcurrency is how many asks of one group the reader has in
	// flight once its first has returned; 0 is asks.DefaultReaderConcurrency.
	ReaderConcurrency int
	// AskRecords is where each letter-ask group's record lands; none is kept
	// where it is "".
	AskRecords string
	// Progress hears the units of a stage that asks: the node pass's leaves
	// and classification's ask groups.
	Progress Progress
}

// Progress hears a stage's units as they go; a nil func hears nothing.
// Units is how many the stage has, once it knows; Unit is one done; Fallback
// is an item that took its mechanical draft for want of a readable answer.
type Progress struct {
	Units    func(total int) error
	Unit     func(name string) error
	Fallback func(result.Item) error
}

func (p Progress) units(total int) error {
	if p.Units == nil {
		return nil
	}
	return p.Units(total)
}

func (p Progress) unit(name string) error {
	if p.Unit == nil {
		return nil
	}
	return p.Unit(name)
}

func (p Progress) fallback(it result.Item) error {
	if p.Fallback == nil {
		return nil
	}
	return p.Fallback(it)
}

// checkDefaulted names a fallback: an item that took its draft.
const checkDefaulted = "defaulted"

// askStop is the stage stopping on a call that never completed, or on any
// other error a reader returned.
func askStop(err error) error {
	var incomplete asks.IncompleteError
	if errors.As(err, &incomplete) {
		return stopf("inference-failed", "%s", incomplete.Error())
	}
	return err
}

// askTotals is a stage's letter asks in the zero form.
func askTotals(records []asks.GroupRecord) result.Record {
	t := asks.Totals(records)
	perCall := 0.0
	if t.Calls > 0 {
		perCall = t.WallSeconds / float64(t.Calls)
	}
	return recordOf(field("items", t.Items), field("answered", t.Answered), field("re-asked", t.Reasked), field("defaulted", t.Defaulted),
		field("calls", t.Calls), field("unmeasured", t.Unmeasured), field("thinking-blocks", t.ThinkingBlocks), field("output-tokens", t.OutputTokens),
		field("seconds-per-call", fmt.Sprintf("%.2f", perCall)), field("wall-seconds", fmt.Sprintf("%.1f", t.WallSeconds)),
		field("api-seconds", fmt.Sprintf("%.1f", t.APISeconds)), field("warm-cache-read", fmt.Sprintf("%d/%d", t.WarmCacheReadTokens, t.WarmPromptTokens)))
}

// Discovered is the node pass. Per leaf the record holds unlanded, in sorted
// order: an unread leaf's paragraph asks as one group sharing its render, its
// whole outcome into the record as planned, the leaf landed in one act, and
// the leaf marked landed — so a stop costs at most one leaf's asks and a
// planned leaf completes from the record without an ask. It exits on every
// obligated paragraph of every leaf carrying a recorded verdict, recomputed
// over the tree it wrote.
func Discovered(ctx context.Context, opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		record, ok, err := ReadNodePass(w.repoRoot)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, stopf("node-pass-record", "no node-pass record stands at %s. The declared pass writes one listing every leaf it stamped, and this pass reads its scope from nothing else", NodePassFile)
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
		paths := slices.Sorted(maps.Keys(record.Leaves))
		states := map[string]int{}
		for _, p := range paths {
			states[record.Leaves[p].State]++
			if _, ok := t.Documents[p]; !ok {
				return nil, stopf("node-pass-record", "the record names %s, which is no document of this tree", p)
			}
		}
		findings := []Finding{fact("stage-C-scope", field("unread", states[ReadUnread]), field("planned", states[ReadPlanned]), field("landed", states[ReadLanded]))}
		hosted := map[string]map[string]bool{}
		for _, n := range g.Nodes {
			if hosted[n.Document] == nil {
				hosted[n.Document] = map[string]bool{}
			}
			hosted[n.Document][n.ID] = true
		}
		var groups []asks.GroupRecord
		var defaulted []string
		causes := map[string]int{}
		minted := 0
		if err := opts.Progress.units(len(paths)); err != nil {
			return findings, err
		}
		for _, p := range paths {
			entry := record.Leaves[p]
			if entry.State == ReadLanded {
				if err := opts.Progress.unit(p); err != nil {
					return findings, err
				}
				continue
			}
			if entry.State == ReadUnread {
				leaf := readLeaf(t.Documents[p], &inv)
				asked := leaf.askedParagraphs(leaf.obligated(&inv))
				nothing := OutcomeNothingToRead
				entry = LeafEntry{State: ReadPlanned, Outcome: &nothing}
				if len(asked) > 0 {
					group, claims, verdicts, err := askLeaf(ctx, leaf, asked, opts)
					if err != nil {
						return findings, askStop(err)
					}
					groups = append(groups, group)
					outcome := OutcomeNoClaim
					if len(claims) > 0 {
						outcome = OutcomeMinted
					}
					entry = LeafEntry{State: ReadPlanned, Outcome: &outcome, Verdicts: verdicts}
					for _, c := range claims {
						entry.Claims = append(entry.Claims, PlannedClaim{Title: c.title, Locator: c.excerpt})
					}
					for _, v := range verdicts {
						if v.Verdict == JudgementDefaulted {
							causes[*v.Cause]++
							defaulted = append(defaulted, fmt.Sprintf("%s line %d (%s)", p, v.Line+1, *v.Cause))
							if err := opts.Progress.fallback(result.Item{Check: checkDefaulted, Path: p, Line: v.Line + 1,
								Detail: fmt.Sprintf("the paragraph's verdict was defaulted (%s): it takes its mechanical draft", *v.Cause)}); err != nil {
								return findings, err
							}
						}
					}
				}
				record.Leaves[p] = entry
				if err := WriteNodePass(w.repoRoot, record); err != nil {
					return findings, err
				}
			}
			elsewhere := map[string]bool{}
			for other, ids := range hosted {
				if other != p {
					for id := range ids {
						elsewhere[id] = true
					}
				}
			}
			blocks := map[string]string{}
			for _, n := range g.hostedBy(p) {
				if n.Locator != "" {
					blocks[n.ID] = n.Locator
				}
			}
			claims := make([]newClaim, len(entry.Claims))
			for i, c := range entry.Claims {
				claims[i] = newClaim{title: c.Title, rationale: proseRationale(p), locator: c.Locator}
			}
			ids, err := w.landLeaf(p, t.kind(p), claims, blocks, elsewhere)
			if err != nil {
				return findings, err
			}
			if hosted[p] == nil {
				hosted[p] = map[string]bool{}
			}
			for _, id := range ids {
				hosted[p][id] = true
			}
			entry.State = ReadLanded
			record.Leaves[p] = entry
			if err := WriteNodePass(w.repoRoot, record); err != nil {
				return findings, err
			}
			minted += len(ids)
			if err := opts.Progress.unit(p); err != nil {
				return findings, err
			}
		}
		findings = append(findings,
			fact("stage-C-asks", append(recordOf(field("leaves", len(groups))), askTotals(groups)...)...),
			fact("stage-C-defaulted", field("by-cause", counts(DefaultCauses, causes)), field("paragraphs", nonNilStrings(defaulted))))

		after, err := readTree(opts.KBRoot)
		if err != nil {
			return findings, err
		}
		inv2, err := scanInventory(after, recs)
		if err != nil {
			return findings, err
		}
		var wrong []string
		for _, p := range paths {
			judged := map[int]bool{}
			for _, v := range record.Leaves[p].Verdicts {
				judged[v.Line] = true
			}
			var missing []int
			for _, para := range readLeaf(after.Documents[p], &inv2).obligated(&inv2) {
				if !judged[para.start] {
					missing = append(missing, para.start+1)
				}
			}
			if len(missing) > 0 {
				wrong = append(wrong, fmt.Sprintf("%s: paragraphs owed a verdict begin on lines %v, and the record judges none of them", p, missing))
			}
		}
		if len(wrong) > 0 {
			return findings, stopf("verdict-coverage", "%d leaf/leaves leave a paragraph owed a verdict without one: %q", len(wrong), wrong[:min(5, len(wrong))])
		}
		return append(findings, pass("stage-C-identify", field("prose-claims-minted", minted), field("leaves", len(paths)), field("leaves-asked", len(groups)))), nil
	})
}

// askLeaf is one leaf's paragraph asks, as one group sharing its render, and
// what they come to.
func askLeaf(ctx context.Context, leaf *leafReading, asked []askedParagraph, opts Options) (asks.GroupRecord, []proseClaim, []ParagraphVerdict, error) {
	items := make([]asks.ParagraphItem, len(asked))
	for i, a := range asked {
		items[i] = asks.ParagraphItem{Paragraph: a.name(), Text: a.labelled()}
	}
	group, err := asks.AskGroup(ctx, asks.Paragraph, leaf.document, asks.ParagraphAsks(leaf.document, leaf.render.text, items), opts.Reader, opts.ReaderConcurrency, opts.AskRecords)
	if err != nil {
		return group, nil, nil, err
	}
	answers := make([]paragraphAnswer, len(asked))
	for i, a := range asked {
		answers[i] = paragraphAnswer{asked: a}
		if l := group.Items[i].Letter; l != nil {
			yes := *l == asks.LetterClaim
			answers[i].statesResult = &yes
		}
	}
	claims, verdicts := leaf.judge(answers)
	return group, claims, verdicts, nil
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// run reads the stage's input — the spine, the tree, the records — and runs
// body over it; a stop ends the report on its finding, and a report with no
// failure ends on the gate.
func run(opts Options, body func(t *Tree, recs []records.Record, w writer) ([]Finding, error)) (Report, error) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	var r Report
	if info, err := os.Stat(filepath.Join(opts.KBRoot, kb.IndexDir)); err != nil || !info.IsDir() {
		r.Findings = append(r.Findings, (&stop{"spine", filepath.Base(opts.KBRoot) + "/" + kb.IndexDir + "/ does not exist; the spine seed creates it"}).finding())
		return r, nil
	}
	recs, err := records.Read(opts.Records)
	if err != nil {
		return r, err
	}
	t, err := readTree(opts.KBRoot)
	if err != nil {
		return r, err
	}
	found, err := body(t, recs, writer{opts.KBRoot, opts.RepoRoot, opts.Logger})
	r.Findings = append(r.Findings, found...)
	var s *stop
	if errors.As(err, &s) {
		r.Findings = append(r.Findings, s.finding())
		return r, nil
	}
	if err != nil {
		return r, err
	}
	r.Findings = append(r.Findings, gate(opts.KBRoot, opts.Logger)...)
	return r, nil
}

// Declared is the declared pass: conformance with the double-run guard, the
// inventory, the claims the author marked, their assembly with the endcap,
// the writes, and the build records written fresh — every declaring leaf
// unread, no candidate classified — then the gate.
func Declared(opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		if err := conformanceGate(t); err != nil {
			return nil, err
		}
		findings := []Finding{pass("stage-A-conformance", field("documents", len(t.Paths)))}
		inv, err := scanInventory(t, recs)
		if err != nil {
			return findings, err
		}
		findings = append(findings, inv.census()...)
		claims := blockClaims(&inv)
		findings = append(findings, pass("stage-C-identify", field("block-hosted-claims", len(claims)),
			field("leaves-unmarked", len(unmarkedDocuments(t, &inv)))))
		p := assemblePlan(t, &inv, claims)
		sources := map[int]bool{}
		for _, r := range p.restsOn {
			sources[r.entry] = true
		}
		findings = append(findings,
			pass("stage-E-assemble", field("register-entries", len(p.entries)), field("registers", len(p.registers())),
				field("frontmatter-records", len(p.documents)), field("markers", len(p.markers))),
			fact("stage-E-endcap", field("works", len(p.works)), field("off-graph-edges", len(p.restsOn)), field("citing-claims", len(sources))))
		written, err := w.writePlan(p)
		findings = append(findings, written...)
		if err != nil {
			return findings, err
		}
		leaves := map[string]LeafEntry{}
		for _, d := range p.documents {
			if declaringKinds[d.kind] {
				leaves[d.path] = LeafEntry{State: ReadUnread}
			}
		}
		if err := WriteNodePass(w.repoRoot, NodePassRecord{Leaves: leaves}); err != nil {
			return findings, err
		}
		if err := WriteClassification(w.repoRoot, ClassificationRecord{}); err != nil {
			return findings, err
		}
		return append(findings, fact("stage-F-node-pass", field("leaves-unread", len(leaves)))), nil
	})
}

// counting is every resolving reference that counts: all of them, less those
// in prose the node pass judged not a claim.
func counting(t *Tree, inv *Inventory, rec NodePassRecord) []Anchor {
	leaves := map[string]*leafReading{}
	var out []Anchor
	for _, a := range inv.Anchors {
		if a.Target == "" {
			continue
		}
		if leaves[a.Document] == nil {
			leaves[a.Document] = readLeaf(t.Documents[a.Document], inv)
		}
		if leaves[a.Document].standing(a, rec.Leaves[a.Document]) != standingNotAClaim {
			out = append(out, a)
		}
	}
	return out
}

// Equations mints every equation a counting reference names that nothing
// else holds and is not yet minted, landing each leaf in one act, and exits
// on the comparison of the equation nodes the tree then holds with the
// counting set. With no verdict recorded every reference counts.
func Equations(opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		record, err := requireNodePass(w.repoRoot)
		if err != nil {
			return nil, err
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
		counted := counting(t, &inv, record)
		type label struct{ target, label string }
		all, kept := map[label]bool{}, map[label]bool{}
		for _, a := range inv.Anchors {
			if a.Target != "" {
				all[label{a.Target, a.Label}] = true
			}
		}
		for _, a := range counted {
			kept[label{a.Target, a.Label}] = true
		}
		wanted := unheld(&inv, counted)
		var pending []referencedEquation
		for _, e := range wanted {
			if _, ok := g.equationNode(e.document, e.label); !ok {
				pending = append(pending, e)
			}
		}
		entries, err := equationEntries(t, pending)
		if err != nil {
			return nil, err
		}
		hosted := map[string]map[string]bool{}
		for _, n := range g.Nodes {
			if hosted[n.Document] == nil {
				hosted[n.Document] = map[string]bool{}
			}
			hosted[n.Document][n.ID] = true
		}
		byDoc := map[string][]entry{}
		var docs []string
		for _, e := range entries {
			if byDoc[e.document] == nil {
				docs = append(docs, e.document)
			}
			byDoc[e.document] = append(byDoc[e.document], e)
		}
		slices.Sort(docs)
		for _, d := range docs {
			elsewhere := map[string]bool{}
			for other, ids := range hosted {
				if other != d {
					for id := range ids {
						elsewhere[id] = true
					}
				}
			}
			var claims []newClaim
			for _, e := range byDoc[d] {
				claims = append(claims, newClaim{title: e.title, rationale: e.rationale})
			}
			ids, err := w.landLeaf(d, t.kind(d), claims, nil, elsewhere)
			if err != nil {
				return nil, err
			}
			if hosted[d] == nil {
				hosted[d] = map[string]bool{}
			}
			for _, id := range ids {
				hosted[d][id] = true
			}
		}
		after, err := readTree(opts.KBRoot)
		if err != nil {
			return nil, err
		}
		inv2, err := scanInventory(after, recs)
		if err != nil {
			return nil, err
		}
		g2, err := readGraph(after, &inv2)
		if err != nil {
			return nil, err
		}
		type key struct{ document, label string }
		minted, expected := map[key]bool{}, map[key]bool{}
		for _, n := range g2.Nodes {
			if n.Equation != "" {
				minted[key{n.Document, n.Equation}] = true
			}
		}
		documents := map[string]bool{}
		for _, e := range wanted {
			expected[key{e.document, e.label}] = true
			documents[e.document] = true
		}
		var extra, missing []string
		for k := range minted {
			if !expected[k] {
				extra = append(extra, k.document+" "+k.label)
			}
		}
		for k := range expected {
			if !minted[k] {
				missing = append(missing, k.document+" "+k.label)
			}
		}
		if len(extra) > 0 || len(missing) > 0 {
			slices.Sort(extra)
			slices.Sort(missing)
			return nil, stopf("equation-set", "the equation nodes the tree holds are not the counting set: minted and not counted %q, counted and not minted %q",
				extra[:min(5, len(extra))], missing[:min(5, len(missing))])
		}
		return []Finding{pass("stage-E-equations", field("minted-this-run", len(entries)), field("equation-nodes", len(expected)),
			field("documents", len(documents)), field("labels-named-only-from-non-claims", len(all)-len(kept)))}, nil
	})
}

// Depends is dependency attribution: the entry condition, the authored graph,
// stage D's narrowing over the node pass's verdicts, every candidate
// classified — with no reader, every one taking its draft — into the
// classification record, and one add-depends-on batch.
func Depends(ctx context.Context, opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		record, err := requireNodePass(w.repoRoot)
		if err != nil {
			return nil, err
		}
		state, err := passTwoGate(t)
		if err != nil {
			return nil, err
		}
		findings := []Finding{pass("stage-A-conformance", field("documents", len(t.Paths)))}
		inv, err := scanInventory(t, recs)
		if err != nil {
			return findings, err
		}
		g, err := readGraph(t, &inv)
		if err != nil {
			return findings, err
		}
		resolving, inProof := 0, 0
		for _, a := range inv.Anchors {
			if a.Target != "" {
				resolving++
			}
			if strings.EqualFold(a.HostingEnvironment, proofEnvironment) {
				inProof++
			}
		}
		findings = append(findings,
			fact("stage-A-determinations", field("hosting", len(state.hosting)), field("no-claim", len(state.determined)), field("undeclared", len(state.undeclared))),
			fact("authored-graph", field("claims", len(g.Nodes)), field("hosting-documents", g.documents())),
			fact("stage-D-anchors", field("anchors", len(inv.Anchors)), field("resolving", resolving), field("in-proofs", inProof)))
		a := narrow(t, g, &inv, record)
		findings = append(findings, candidateFindings(a)...)
		c, err := classify(ctx, w.repoRoot, a.candidates, statements(t, g, &inv), opts)
		if err != nil {
			return findings, askStop(err)
		}
		findings = append(findings, classificationFindings(c, opts.Reader != nil)...)
		f, err := w.writeEdges("pass-3-add-depends-on", c.edges, c.refs)
		findings = append(findings, f)
		return findings, err
	})
}

func requireNodePass(repoRoot string) (NodePassRecord, error) {
	rec, ok, err := ReadNodePass(repoRoot)
	if err != nil {
		return rec, err
	}
	if !ok {
		return rec, stopf("node-pass-record", "no node-pass record stands at %s; the declared pass is what writes it", NodePassFile)
	}
	return rec, nil
}

func candidateFindings(a attribution) []Finding {
	sources := map[string]bool{}
	var harvests, offeredLetters, drafts []string
	for _, c := range a.candidates {
		sources[c.source.ID] = true
		harvests = append(harvests, strings.Join(c.harvests, "+"))
		names := make([]string, len(c.offered))
		for i, r := range c.offered {
			names[i] = string(r)
		}
		offeredLetters = append(offeredLetters, strings.Join(names, ","))
		drafts = append(drafts, string(c.draft))
	}
	routes := counts([]string{byEquation, byIdentifier, bySoleClaim}, a.routes)
	return []Finding{
		fact("stage-D-candidates", field("candidates", len(a.candidates)), field("sources", len(sources)),
			field("by-harvest", tally(harvests)), field("by-offered", tally(offeredLetters))),
		fact("stage-D-drafts", field("by-relation", counts([]string{string(MentionedBy), string(SupportedBy)}, countOf(drafts))), field("directed-by-route", routes)),
		fact("stage-D-word-filtered", field("pairs", len(a.wordDropped))),
		fact("stage-D-containment-ring", field("demoted", pairs(a.demoted))),
	}
}

func countOf(values []string) map[string]int {
	out := map[string]int{}
	for _, v := range values {
		out[v]++
	}
	return out
}

func classificationFindings(c classification, asked bool) []Finding {
	var rels, outcomes []string
	var defaulted []pair
	for p, r := range c.relations {
		rels = append(rels, string(r))
		outcomes = append(outcomes, c.outcomes[p])
		if c.outcomes[p] == ClassifyDefaulted {
			defaulted = append(defaulted, p)
		}
	}
	slices.SortFunc(defaulted, comparePairs)
	names := make([]string, len(relations))
	for i, r := range relations {
		names[i] = string(r)
	}
	return []Finding{
		fact("stage-D-classified", field("by-relation", counts(names, countOf(rels))), field("by-outcome", tally(outcomes)), field("asked", asked)),
		fact("stage-D-defaulted", field("candidates", pairs(defaulted))),
		fact("stage-D-asks", askTotals(c.asked)...),
		pass("stage-D-classify", field("dependency-edges", len(c.edges)), field("references", len(c.refs)), field("demoted", pairs(c.demoted))),
	}
}
