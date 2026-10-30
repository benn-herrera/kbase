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
	"time"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/records"
	"kbase/internal/result"
	"kbase/internal/write"
)

// Options is one claim-graph stage over the tree at KBRoot, the repository
// whose root holds the build records, and the document graph's records.
type Options struct {
	KBRoot   string
	RepoRoot string
	Records  string
	Logger   log.Logger
	// Launch starts each multichoice ask's item. It is nil only for a build
	// spending no inference, which drops the node pass, the unmarked-reference
	// asks and the co-linearity cull and classifies every candidate as its
	// draft.
	Launch asks.Launch
	// AskRecords is where the stage's multichoice-ask group records land, as one
	// file once its asks are done; none is kept where it is "".
	AskRecords string
	// Progress hears the units of a stage that asks: the node pass's leaves,
	// the unmarked-reference asks' and classification's source groups, and the
	// cull's groups of each pass.
	Progress Progress
}

// asking is whether the stage puts its multichoice asks to a model.
func (o Options) asking() bool { return o.Launch != nil }

// batch is a new ask batch of the stage, launched through Launch.
func (o Options) batch(ctx context.Context) *asks.Batch {
	return asks.NewBatch(ctx, o.Launch)
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

// askStopWriting is askStop(err), the group records the stopped batch
// returned written to dir first. An error writing them rides on the stop as
// its fault and never replaces it.
func askStopWriting(dir string, records []asks.GroupRecord, err error) error {
	st := askStop(err)
	if werr := asks.WriteGroupRecords(dir, records); werr != nil {
		var s *stop
		if errors.As(st, &s) {
			s.fault = werr
			return st
		}
		return errors.Join(st, werr)
	}
	return st
}

// askTotals is a stage's multichoice asks in the zero form: wall-seconds sums its
// calls' wall time, stage-wall-seconds is wall, the ask batch's elapsed time.
func askTotals(records []asks.GroupRecord, wall time.Duration) result.Record {
	t := asks.Totals(records)
	perCall := 0.0
	if t.Calls > 0 {
		perCall = t.WallSeconds / float64(t.Calls)
	}
	return recordOf(field("items", t.Items), field("answered", t.Answered), field("re-asked", t.Reasked), field("defaulted", t.Defaulted), field("failed", t.Failed),
		field("calls", t.Calls), field("unmeasured", t.Unmeasured), field("thinking-blocks", t.ThinkingBlocks), field("output-tokens", t.OutputTokens),
		field("seconds-per-call", fmt.Sprintf("%.2f", perCall)), field("wall-seconds", fmt.Sprintf("%.1f", t.WallSeconds)),
		field("stage-wall-seconds", fmt.Sprintf("%.1f", wall.Seconds())), field("api-seconds", fmt.Sprintf("%.1f", t.APISeconds)), field("warm-cache-read", fmt.Sprintf("%d/%d", t.WarmCacheReadTokens, t.WarmPromptTokens)))
}

// Discovered is the node pass. Per leaf the record holds unlanded, in sorted
// order, an unread leaf's paragraph asks go as one group sharing its render,
// and its whole outcome is held; a planned leaf's comes from the record
// without an ask. Once every ask is done, every held claim lands in one insert
// batch, in sorted leaf order, and the record is written once, every leaf
// landed. It exits on every obligated paragraph of every leaf carrying a
// recorded verdict, recomputed over the tree it wrote.
func Discovered(ctx context.Context, opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		record, ok, err := buildrecords.ReadNodePass(recordsAt(w.repoRoot))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, stopf("node-pass-record", "no node-pass record stands at %s. The declared pass writes one listing every leaf it stamped, and this pass reads its scope from nothing else", buildrecords.NodePassFile)
		}
		if err := passTwoGate(t); err != nil {
			return nil, err
		}
		inv, err := scanInventory(t, recs)
		if err != nil {
			return nil, err
		}
		if _, err := readGraph(t, &inv); err != nil {
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
		var groups []asks.GroupRecord
		var defaulted []string
		var landing []entry
		causes := map[string]int{}
		if err := opts.Progress.units(len(paths)); err != nil {
			return findings, err
		}
		type leafAsks struct {
			leaf  *leafReading
			asked []askedParagraph
			group int
		}
		unread := map[string]leafAsks{}
		batch, launched := opts.batch(ctx), 0
		for _, p := range paths {
			if record.Leaves[p].State != ReadUnread {
				continue
			}
			leaf := readLeaf(t.Documents[p], &inv)
			a := leafAsks{leaf: leaf, asked: leaf.askedParagraphs(leaf.obligated(&inv)), group: -1}
			if len(a.asked) > 0 {
				if err := askLeaf(batch, leaf, a.asked); err != nil {
					break
				}
				a.group, launched = launched, launched+1
			}
			unread[p] = a
		}
		groups, err = batch.Wait()
		if err != nil {
			return findings, askStopWriting(opts.AskRecords, groups, err)
		}
		for _, p := range paths {
			held := record.Leaves[p]
			if held.State == ReadLanded {
				if err := opts.Progress.unit(p); err != nil {
					return findings, err
				}
				continue
			}
			if held.State == ReadUnread {
				a := unread[p]
				nothing := OutcomeNothingToRead
				held = buildrecords.LeafEntry{State: ReadPlanned, Outcome: &nothing}
				if a.group >= 0 {
					claims, verdicts := judgeLeaf(a.leaf, a.asked, groups[a.group])
					outcome := OutcomeNoClaim
					if len(claims) > 0 {
						outcome = OutcomeMinted
					}
					held = buildrecords.LeafEntry{State: ReadPlanned, Outcome: &outcome, Verdicts: verdicts}
					for _, c := range claims {
						held.Claims = append(held.Claims, buildrecords.PlannedClaim{Title: c.title, Locator: c.excerpt})
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
			}
			for _, c := range held.Claims {
				landing = append(landing, entry{title: t.registerTitle(c.Title, p), rationale: proseRationale(p), document: p, locator: c.Locator})
			}
			held.State = ReadLanded
			record.Leaves[p] = held
			if err := opts.Progress.unit(p); err != nil {
				return findings, err
			}
		}
		if err := asks.WriteGroupRecords(opts.AskRecords, groups); err != nil {
			return findings, err
		}
		ids, err := w.landLeaves(landing)
		if err != nil {
			return findings, err
		}
		if states[ReadLanded] < len(paths) {
			if err := WriteNodePass(w.repoRoot, record); err != nil {
				return findings, err
			}
		}
		minted := len(ids)
		findings = append(findings,
			fact("stage-C-asks", append(recordOf(field("leaves", len(groups))), askTotals(groups, batch.Elapsed())...)...),
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

// askLeaf launches one leaf's paragraph asks into batch, as one group sharing
// its render.
func askLeaf(batch *asks.Batch, leaf *leafReading, asked []askedParagraph) error {
	items := make([]asks.ParagraphItem, len(asked))
	for i, a := range asked {
		items[i] = asks.ParagraphItem{Paragraph: a.name(), Text: a.labelled()}
	}
	return batch.AskGroup(asks.Paragraph, leaf.document, asks.ParagraphAsks(leaf.document, leaf.render.text, items))
}

// judgeLeaf is what one leaf's answered paragraph asks come to.
func judgeLeaf(leaf *leafReading, asked []askedParagraph, group asks.GroupRecord) ([]proseClaim, []buildrecords.ParagraphVerdict) {
	answers := make([]paragraphAnswer, len(asked))
	for i, a := range asked {
		answers[i] = paragraphAnswer{asked: a}
		if l := group.Items[i].Choice; l != nil {
			yes := *l == asks.LetterClaim
			answers[i].statesResult = &yes
		}
	}
	return leaf.judge(answers)
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
	return runStage(opts, true, body)
}

// runStage is run, ending on the gate only where gated: a stage writing
// nothing under kb-root/ has nothing for refresh or verify to judge.
func runStage(opts Options, gated bool, body func(t *Tree, recs []records.Record, w writer) ([]Finding, error)) (Report, error) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	var r Report
	if info, err := os.Stat(filepath.Join(opts.KBRoot, kb.IndexDir)); err != nil || !info.IsDir() {
		r.Findings = append(r.Findings, (&stop{check: "spine", detail: filepath.Base(opts.KBRoot) + "/" + kb.IndexDir + "/ does not exist; the spine seed creates it"}).finding())
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
		return r, s.fault
	}
	if err != nil {
		return r, err
	}
	if gated {
		r.Findings = append(r.Findings, gate(opts.KBRoot, opts.Logger)...)
	}
	return r, nil
}

// Declared is the declared pass: conformance with the double-run guard, the
// inventory, the claims the author marked, their assembly with the endcap,
// the writes, and the build records written fresh — every declaring leaf
// unread, no candidate classified, no shortlist planned, nothing culled —
// then the gate.
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
			pass("stage-E-assemble", field("claims", len(p.entries))),
			fact("stage-E-endcap", field("works", len(p.works)), field("off-graph-edges", len(p.restsOn)), field("citing-claims", len(sources))))
		written, err := w.writePlan(p)
		findings = append(findings, written...)
		if err != nil {
			return findings, err
		}
		leaves := map[string]buildrecords.LeafEntry{}
		for _, path := range t.Paths {
			if declaringKinds[t.kind(path)] {
				leaves[path] = buildrecords.LeafEntry{State: ReadUnread}
			}
		}
		if err := WriteNodePass(w.repoRoot, buildrecords.NodePassRecord{Leaves: leaves}); err != nil {
			return findings, err
		}
		if err := WriteClassification(w.repoRoot, buildrecords.ClassificationRecord{}); err != nil {
			return findings, err
		}
		if err := WriteUnmarked(w.repoRoot, buildrecords.UnmarkedRecord{}); err != nil {
			return findings, err
		}
		if err := WriteCull(w.repoRoot, buildrecords.CullRecord{}); err != nil {
			return findings, err
		}
		return append(findings, fact("stage-F-node-pass", field("leaves-unread", len(leaves)))), nil
	})
}

// counting is every resolving reference that counts: all of them, less those
// in prose the node pass judged not a claim.
func counting(t *Tree, inv *Inventory, rec buildrecords.NodePassRecord) []Anchor {
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
			var claims []newClaim
			for _, e := range byDoc[d] {
				claims = append(claims, newClaim{title: e.title, rationale: e.rationale, locator: e.locator})
			}
			if _, err := w.landLeaf(d, claims); err != nil {
				return nil, err
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
// stage D's narrowing over the node pass's verdicts and the cull record's
// links, oriented — read, never recomputed, and none where no cull record
// stands — every candidate classified — with no reader, every one taking its
// draft — into the classification record, one add-build-edges batch, and then
// the ring row over the graph that batch left.
func Depends(ctx context.Context, opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		record, err := requireNodePass(w.repoRoot)
		if err != nil {
			return nil, err
		}
		cull, _, err := buildrecords.ReadCull(recordsAt(w.repoRoot))
		if err != nil {
			return nil, err
		}
		if err := passTwoGate(t); err != nil {
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
			fact("authored-graph", field("claims", len(g.Nodes)), field("hosting-documents", g.documents())),
			fact("stage-D-anchors", field("anchors", len(inv.Anchors)), field("resolving", resolving), field("in-proofs", inProof)))
		a := narrow(t, g, &inv, record, orientLinks(cull.Links, g, claimPositions(t, g, &inv)))
		findings = append(findings, candidateFindings(a)...)
		c, err := classify(ctx, w.repoRoot, a.candidates, statements(t, g, &inv), opts)
		if err != nil {
			return findings, err
		}
		findings = append(findings, classificationFindings(c, opts.asking())...)
		f, err := w.writeEdges("pass-3-add-rests-on", c.edges, c.refs)
		findings = append(findings, f)
		if err != nil {
			return findings, err
		}
		cut, warnings, err := ringRow(w)
		if err != nil {
			return findings, err
		}
		return append(findings, pass("stage-D-ring", field("demoted", pairs(cut)), field("warnings", warnings))), nil
	})
}

// ringRow breaks every cycle of the records the edge batch left, cutting the
// edges index.BreakCycles' walk cuts in the tree's hosted order, all through
// one demote-claim-edge batch in cut order: the walk computes every cut over
// one graph before any is issued. It returns each cut as the authored pair —
// the claim the edge ran into, then the premise — in cut order, and the walk's
// warning for each.
func ringRow(w writer) ([]pair, []string, error) {
	src := builtTree(w.kbRoot)
	g, err := kb.LoadGraph(src)
	if err != nil {
		return nil, nil, err
	}
	hosted, err := index.HostedOrder(src, g)
	if err != nil {
		return nil, nil, err
	}
	_, cuts, items := index.BreakCycles(g, hosted)
	cut := make([]pair, 0, len(items))
	warnings := make([]string, 0, len(items))
	entries := make([]map[string]any, 0, len(cuts))
	for i, d := range cuts {
		entries = append(entries, map[string]any{"id": d.Target, "target": d.Source})
		cut = append(cut, pair{d.Target, d.Source})
		warnings = append(warnings, items[i].Detail)
	}
	if len(entries) > 0 {
		if _, err := issue(w.raw, write.DemoteClaimEdge, entries); err != nil {
			return nil, nil, stopf("ring-row", "%v; the ring row cannot proceed", err)
		}
	}
	return cut, warnings, nil
}

func requireNodePass(repoRoot string) (buildrecords.NodePassRecord, error) {
	rec, ok, err := buildrecords.ReadNodePass(recordsAt(repoRoot))
	if err != nil {
		return rec, err
	}
	if !ok {
		return rec, stopf("node-pass-record", "no node-pass record stands at %s; the declared pass is what writes it", buildrecords.NodePassFile)
	}
	return rec, nil
}

func candidateFindings(a attribution) []Finding {
	sources := map[string]bool{}
	var harvested, offeredLetters, drafts []string
	for _, c := range a.candidates {
		sources[c.source.ID] = true
		harvested = append(harvested, strings.Join(c.harvests, "+"))
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
			field("by-harvest", tally(harvested)), field("by-offered", tally(offeredLetters)), field("own-equations-dropped", len(a.ownEquations))),
		fact("stage-D-drafts", field("by-relation", counts([]string{string(MentionedBy), string(SupportedBy)}, countOf(drafts))), field("directed-by-route", routes)),
		fact("stage-D-word-filtered", field("pairs", len(a.wordDropped))),
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
		fact("stage-D-asks", askTotals(c.asked, c.wall)...),
		pass("stage-D-classify", field("dependency-edges", len(c.edges)), field("references", len(c.refs))),
	}
}
