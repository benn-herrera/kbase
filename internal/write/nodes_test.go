package write

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// nodesGraph is a claim, clm-aaaaaa, with a supports edge into it, one out of
// it and a demoted record naming it, beside a work and an axiom.
func nodesGraph() kb.Graph {
	return kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Main Result", Rationale: "Shown."},
			{ID: "clm-bbbbbb", Title: "Lemma", Rationale: "Shown."}, {ID: "clm-cccccc", Title: "Corollary", Rationale: "Shown."}},
		Works:  []kb.WorkNode{{ID: "wrk-s00000", Label: "Smith2020", Title: "Old Work", Rationale: "Cited."}},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "Closure"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Context: "by the lemma", Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true},
			{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-cccccc", Bearing: kb.Bearing{Pending: true}},
		},
		Demoted: []kb.DemotedEdge{{Source: "clm-cccccc", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Value: 0.2}}},
	}
}

// nodesLeaf hosts clm-aaaaaa in its first section.
const nodesLeaf = "[↑ Entry](entry-point.md)\n\n# Leaf\n\nThe main result holds.<!-- kbase-id: clm-aaaaaa -->\n\n" +
	"## Later\n\nIt holds here too.\n"

// nodesKB is a KB holding g's records, nodesLeaf at leaf.md, and other.md
// holding other.
func nodesKB(t *testing.T, g kb.Graph, other string) string {
	t.Helper()
	root := recordKB(t, g)
	for rel, text := range map[string]string{kb.EntryPointFile: "# Entry\n\n- [Leaf](leaf.md)\n- [Other](other.md)\n",
		"leaf.md": nodesLeaf, "other.md": other} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const plainOther = "[↑ Entry](entry-point.md)\n\n# Other\n\nNothing cites here.\n"

// rowNames is each row's relation, source and target.
func rowNames(rows []kb.EdgeRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Relation+" "+r.Source+" "+r.Target)
	}
	return out
}

// TestRemoveNodeTakesEveryRecordNamingIt: remove-node removes the node's
// record, its edges at either end and the demoted record naming it, and its
// marker, reporting each edge record it removed as a full edge row.
func TestRemoveNodeTakesEveryRecordNamingIt(t *testing.T) {
	root := nodesKB(t, nodesGraph(), plainOther)
	res := run(t, root, "remove-node", "entry:\n- id: clm-aaaaaa\n")
	e := onlyEntry(t, res)
	if e.ID != "clm-aaaaaa" {
		t.Errorf("entry id = %q, want clm-aaaaaa", e.ID)
	}
	want := []string{"supports clm-aaaaaa clm-cccccc", "supports clm-bbbbbb clm-aaaaaa", "demoted clm-cccccc clm-aaaaaa"}
	if got := rowNames(e.RemovedEdges); !slices.Equal(got, want) {
		t.Fatalf("removed-edges = %v, want %v", got, want)
	}
	lemma := e.RemovedEdges[1]
	if lemma.Context == nil || *lemma.Context != "by the lemma" || lemma.Bearing == nil || lemma.ManuallyScored == nil || !*lemma.ManuallyScored {
		t.Errorf("the lemma's row = %+v, want its context, bearing and manual mark", lemma)
	}
	g := records(t, root)
	if len(g.Claims) != 2 || slices.ContainsFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == "clm-aaaaaa" }) {
		t.Errorf("claims = %+v, want clm-aaaaaa gone and the other two standing", g.Claims)
	}
	if len(g.Edges()) != 0 {
		t.Errorf("edge records = %+v, want none", g.Edges())
	}
	if got := recordFileText(t, root, "leaf.md"); strings.Contains(got, "clm-aaaaaa") || !strings.Contains(got, "The main result holds.\n") {
		t.Errorf("leaf.md = %q, want the marker gone and the prose standing", got)
	}
	if res := run(t, root, "remove-node", "entry:\n- id: clm-aaaaaa\n"); res.Outcome != result.Refused ||
		len(res.Refusals) != 1 || res.Refusals[0].Detail != "no node clm-aaaaaa" {
		t.Errorf("re-issued: %+v, want refused, no node", res)
	}
}

// TestRemoveNodeTakesAnAxiomsHeading: an axiom's removal takes the heading
// carrying its marker from axioms.md, and leaves an unmarked heading of the
// same text, the author's.
func TestRemoveNodeTakesAnAxiomsHeading(t *testing.T) {
	root := nodesKB(t, nodesGraph(), plainOther)
	if err := os.WriteFile(filepath.Join(root, kb.AxiomsFile), []byte("# Axioms\n\n"+kb.AxiomHeadingLine("S1", "Closure", "axm-s10000")+
		"\n\nIt closes.\n### Axiom S1: Closure\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	onlyEntry(t, run(t, root, "remove-node", "entry:\n- id: axm-s10000\n"))
	if got, want := recordFileText(t, root, kb.AxiomsFile), "# Axioms\n\n\nIt closes.\n### Axiom S1: Closure\n"; got != want {
		t.Errorf("axioms.md = %q, want %q", got, want)
	}
}

// TestACitationRefusesTheRemoval: a citation of the node, found by the id in
// its link title, refuses the removal, naming path:line; one inside code
// does not; nothing is written.
func TestACitationRefusesTheRemoval(t *testing.T) {
	other := "[↑ Entry](entry-point.md)\n\n# Other\n\n```\n[Main Result](leaf.md#leaf \"clm-aaaaaa\")\n```\n\n" +
		"As [the main result](leaf.md#leaf \"clm-aaaaaa\") shows.\n"
	root := nodesKB(t, nodesGraph(), other)
	before := snapshot(t, root)
	res := run(t, root, "remove-node", "entry:\n- id: clm-aaaaaa\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("result = %+v, want refused with one item", res)
	}
	r := res.Refusals[0]
	if r.Path != "other.md" || r.Line != 9 || r.Entry != 1 || !strings.Contains(r.Detail, "other.md:9") {
		t.Errorf("refusal = %+v, want other.md line 9 named, entry 1", r)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Error("a refused removal wrote to the KB")
	}
}

// TestABareIDRefusesTheRemoval: the node's id standing bare in a document's
// prose refuses the removal once per line holding it, naming path:line, a
// link whose whole text is the id included, and a line that also cites it
// named once; one inside code or only part of a link's text does not; nothing
// is written.
func TestABareIDRefusesTheRemoval(t *testing.T) {
	other := "[↑ Entry](entry-point.md)\n\n# Other\n\n`clm-aaaaaa` and [see clm-aaaaaa](leaf.md) are no mention.\n\n" +
		"Both clm-aaaaaa and clm-aaaaaa hold.\n\nSo does [clm-aaaaaa](leaf.md).\n\n" +
		"As [the main result](leaf.md#leaf \"clm-aaaaaa\") shows, clm-aaaaaa holds.\n"
	root := nodesKB(t, nodesGraph(), other)
	before := snapshot(t, root)
	res := run(t, root, "remove-node", "entry:\n- id: clm-aaaaaa\n")
	var got []string
	for _, r := range res.Refusals {
		got = append(got, fmt.Sprintf("%s:%d", r.Path, r.Line))
	}
	if want := []string{"other.md:7", "other.md:9", "other.md:11"}; res.Outcome != result.Refused || !slices.Equal(got, want) {
		t.Fatalf("result = %+v, want refused at %q", res, want)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused removal wrote to the KB")
	}
}

// TestASpanOnlyTheWorkRecordListsRefusesTheRemoval: a citation span carrying
// a work's label refuses the work's removal, naming its path:line, where no
// reference-list entry has that key; one inside code does not, and an entry of
// that key anywhere in the KB lets the removal through.
func TestASpanOnlyTheWorkRecordListsRefusesTheRemoval(t *testing.T) {
	const other = "[↑ Entry](entry-point.md)\n\n# Other\n\n```\n<span class=\"citation\" data-cites=\"Smith2020\">(Smith2020)</span>\n```\n\n" +
		"As <span class=\"citation\" data-cites=\"Jones2021 Smith2020\">(Jones 2021; Smith 2020)</span> shows.\n"
	root := nodesKB(t, nodesGraph(), other)
	before := snapshot(t, root)
	res := run(t, root, "remove-node", "entry:\n- id: wrk-s00000\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("result = %+v, want refused with one item", res)
	}
	if r := res.Refusals[0]; r.Path != "other.md" || r.Line != 9 || r.Key != "id" || !strings.Contains(r.Detail, "Smith2020") {
		t.Errorf("refusal = %+v, want other.md line 9 at id, naming the label", r)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused removal wrote to the KB")
	}
	if err := os.WriteFile(filepath.Join(root, "refs.md"), []byte("# Refs\n\n<div id=\"ref-Smith2020\" class=\"csl-entry\">Smith.</div>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	onlyEntry(t, run(t, root, "remove-node", "entry:\n- id: wrk-s00000\n"))
}

// TestARemovalShiftingARepeatSuffixRepointsTheLinksIntoIt: removing a node
// whose summary entry held a slug a later entry repeats moves that entry to
// the unsuffixed slug, and every link into it is re-pointed in the same write.
func TestARemovalShiftingARepeatSuffixRepointsTheLinksIntoIt(t *testing.T) {
	g := editGraph()
	g.Works = append(g.Works, kb.WorkNode{ID: "wrk-a00000", Label: "Adams2019", Title: "Old Work!", Rationale: "Cited."})
	root := editKB(t, g)
	writeDocs(t, root, map[string]string{"other.md": "[↑ Entry](entry-point.md)\n\n# Other\n\nSee [Smith](kb-summary.md#old-work-1).\n"})
	onlyEntry(t, run(t, root, "remove-node", "entry:\n- id: wrk-a00000\n"))
	if got, want := kb.SplitLines(recordFileText(t, root, "other.md"))[4], "See [Smith](kb-summary.md#old-work)."; got != want {
		t.Errorf("other.md line 5 = %q, want %q", got, want)
	}
	if dead, err := kb.DeadLinksIn(kb.OnDisk(root), root, summarySlugs(t, root)); err != nil || len(dead) != 0 {
		t.Errorf("dead links %v, %v; want none", dead, err)
	}
}

// TestAnAxiomRemovalShiftingARepeatSuffixRepointsTheLinksIntoIt: removing an
// axiom whose heading held a slug a later heading of axioms.md repeats moves
// that heading to the unsuffixed slug, and every link into it is re-pointed.
func TestAnAxiomRemovalShiftingARepeatSuffixRepointsTheLinksIntoIt(t *testing.T) {
	g := editGraph()
	g.Axioms = append(g.Axioms, kb.AxiomNode{ID: "axm-s20000", Label: "S1!", Title: "Closure"})
	root := editKB(t, g)
	writeDocs(t, root, map[string]string{
		kb.AxiomsFile: "# Axioms\n\n" + kb.AxiomHeadingLine("S1", "Closure", "axm-s10000") + "\n\nIt closes.\n\n" +
			kb.AxiomHeadingLine("S1!", "Closure", "axm-s20000") + "\n\nIt closes too.\n",
		"leaf.md":  strings.ReplaceAll(editLeaf, "[Closure](axioms.md#axiom-s1-closure \"axm-s10000\")", "the closure"),
		"other.md": "[↑ Entry](entry-point.md)\n\n# Other\n\nSee [the second](axioms.md#axiom-s1-closure-1 \"axm-s20000\").\n"})
	onlyEntry(t, run(t, root, "remove-node", "entry:\n- id: axm-s10000\n"))
	if got, want := kb.SplitLines(recordFileText(t, root, "other.md"))[4],
		"See [the second](axioms.md#axiom-s1-closure \"axm-s20000\")."; got != want {
		t.Errorf("other.md line 5 = %q, want %q", got, want)
	}
}

// TestACitationInAnAxiomsDocumentTheBatchCreatesRefusesTheRemoval: an
// axiom added earlier in the batch, its axioms.md not yet standing, whose
// title cites the claim, refuses the claim's removal with an item naming
// axioms.md and the heading's line, beside the one naming the title field.
func TestACitationInAnAxiomsDocumentTheBatchCreatesRefusesTheRemoval(t *testing.T) {
	root := nodesKB(t, nodesGraph(), plainOther)
	res := run(t, root, Batch, "entry:\n"+
		"- operation: add-axiom\n  label: S2\n  title: '[Main Result](leaf.md#leaf \"clm-aaaaaa\") closes'\n"+
		"- operation: remove-node\n  id: clm-aaaaaa\n")
	if res.Outcome != result.Refused {
		t.Fatalf("result = %+v, want refused", res)
	}
	if !slices.ContainsFunc(res.Refusals, func(r result.Item) bool { return r.Path == kb.AxiomsFile && r.Line == 1 && r.Entry == 2 }) {
		t.Errorf("refusals = %+v, want one naming %s line 1, entry 2", res.Refusals, kb.AxiomsFile)
	}
}

// TestAnEditEdgeEarlierInTheBatchClearsAContextMention: a surviving edge's
// context mentioning the node refuses its removal, naming the record and the
// field, remedy edit-edge; an edit-edge clearing it earlier in the same batch
// lets the removal apply.
func TestAnEditEdgeEarlierInTheBatchClearsAContextMention(t *testing.T) {
	g := nodesGraph()
	g.Scorable = append(g.Scorable, kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-cccccc",
		Context: "unlike clm-aaaaaa", Bearing: kb.Bearing{Pending: true}})
	root := nodesKB(t, g, plainOther)
	res := run(t, root, Batch, "entry:\n- operation: remove-node\n  id: clm-aaaaaa\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("result = %+v, want refused with one item", res)
	}
	if r := res.Refusals[0]; r.Path != kb.GraphEdgesFile || !strings.Contains(r.Detail, "context") ||
		!strings.Contains(r.Detail, "clm-bbbbbb") || !strings.Contains(r.Remedy, "edit-edge") {
		t.Errorf("refusal = %+v, want the edge's context named, remedy edit-edge", r)
	}
	res = run(t, root, Batch, "entry:\n"+edgeEntry("edit-edge", "clm-bbbbbb", "clm-cccccc", kb.RelationSupports, "context: independently")+
		"- operation: remove-node\n  id: clm-aaaaaa\n")
	if res.Outcome != result.Done || len(res.Entries) != 2 {
		t.Fatalf("result = %+v, want done with two entries", res)
	}
}

// TestADemotedRecordsContextRefusesTheRemoval: a demoted record's context
// mentioning the node refuses its removal, naming the edge file, its remedy
// the remove-edge entry that removes that record; that entry earlier in the
// same batch lets the removal land.
func TestADemotedRecordsContextRefusesTheRemoval(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Removed"}, {ID: "clm-bbbbbb", Title: "Other"}, {ID: "clm-cccccc", Title: "Seed"}},
		Demoted: []kb.DemotedEdge{{Source: "clm-cccccc", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports,
			Context: "as clm-aaaaaa shows", Bearing: kb.Bearing{Pending: true}}},
	}
	root := recordKB(t, g)
	before := snapshot(t, root)
	remove := "- operation: remove-node\n  id: clm-aaaaaa\n"
	res := run(t, root, Batch, "entry:\n"+remove)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("remove: %+v, want one refusal", res)
	}
	if it, want := res.Refusals[0], "remove-edge entry, source clm-cccccc, target clm-bbbbbb and relation "+kb.RelationDemoted; it.Path != kb.GraphEdgesFile ||
		!strings.Contains(it.Remedy, want) {
		t.Errorf("refusal = %+v, want path %s, remedy containing %q", it, kb.GraphEdgesFile, want)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused removal changed kb-root")
	}
	if res := run(t, root, Batch, "entry:\n"+edgeEntry(RemoveEdge, "clm-cccccc", "clm-bbbbbb", kb.RelationDemoted)+remove); res.Outcome != result.Done {
		t.Fatalf("the removal after the remedy: %+v", res)
	}
	if slices.ContainsFunc(records(t, root).Claims, func(n kb.ClaimNode) bool { return n.ID == "clm-aaaaaa" }) {
		t.Error("claims still hold clm-aaaaaa")
	}
}

// TestARecordMentionRefusesTheRemoval: a prose field of another record naming
// the node — which the derivation renders onto a register page — refuses its
// removal, naming the record file, the record and the field, and nothing is
// written.
func TestARecordMentionRefusesTheRemoval(t *testing.T) {
	const mention = "as clm-aaaaaa shows"
	for _, c := range []struct {
		name, file, record, field string
		stage                     func(g *kb.Graph)
	}{
		{"rationale", kb.GraphNodesFile, "clm-bbbbbb", "rationale", func(g *kb.Graph) { g.Claims[1].Rationale = mention }},
		{"work rationale", kb.GraphNodesFile, "wrk-k00000", "rationale", func(g *kb.Graph) { g.Works[0].Rationale = mention }},
		{"edge context", kb.GraphEdgesFile, kb.RelationSupports + " clm-cccccc → clm-bbbbbb", "context",
			func(g *kb.Graph) { g.Scorable[0].Context = mention }},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := kb.Graph{
				Claims:   []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Removed"}, {ID: "clm-bbbbbb", Title: "Other"}, {ID: "clm-cccccc", Title: "Seed"}},
				Works:    []kb.WorkNode{{ID: "wrk-k00000", Label: "k", Title: "W"}},
				Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-cccccc", Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}}},
			}
			c.stage(&g)
			root := recordKB(t, g)
			before := snapshot(t, root)
			res := run(t, root, RemoveNode, "entry:\n- id: clm-aaaaaa\n")
			if res.Outcome != result.Refused || len(res.Refusals) != 1 {
				t.Fatalf("remove: %+v, want one refusal", res)
			}
			if it := res.Refusals[0]; it.Key != "id" || it.Entry != 1 || it.Path != c.file || !strings.Contains(it.Detail, c.record+"'s "+c.field) {
				t.Errorf("refusal = %+v, want entry 1's id at %s naming %s's %s", it, c.file, c.record, c.field)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused removal changed kb-root")
			}
		})
	}
}

// TestRemoveNodeTakesEveryClaimALeafHosts: a batch removing every claim a
// leaf hosts lands, the leaf losing both markers and keeping its prose.
func TestRemoveNodeTakesEveryClaimALeafHosts(t *testing.T) {
	root := seedKB(t)
	b := onlyEntry(t, run(t, root, AddClaim, "entry:\n- title: Bound Result\n  rationale: Shown.\n  document: leaf.md\n  line: 7\n  snippet: names a bound\n")).ID
	if res := run(t, root, RemoveNode, "entry:\n- id: "+seedClaim+"\n- id: "+b+"\n"); res.Outcome != result.Done {
		t.Fatalf("remove both: %+v", res)
	}
	if leaf := snapshot(t, root)["leaf.md"]; strings.Contains(leaf, kb.StatementMarkerOpener) || !strings.Contains(leaf, "The first result holds here.\n") ||
		!strings.Contains(leaf, "A second paragraph names a bound.  \n") {
		t.Errorf("the leaf kept a marker, or lost its prose:\n%s", leaf)
	}
}

// TestADryRunPreviewsTheCascade: a dry run of a removal reports the edges the
// real run removes and writes nothing.
func TestADryRunPreviewsTheCascade(t *testing.T) {
	root := nodesKB(t, nodesGraph(), plainOther)
	before := snapshot(t, root)
	dry := run(t, root, "remove-node", "dry-run: true\nentry:\n- id: clm-aaaaaa\n")
	if !dry.DryRun || dry.Outcome != result.Done {
		t.Fatalf("dry run = %+v, want done and dry-run", dry)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Error("a dry run wrote to the KB")
	}
	real := onlyEntry(t, run(t, root, "remove-node", "entry:\n- id: clm-aaaaaa\n"))
	if got, want := rowNames(onlyEntry(t, dry).RemovedEdges), rowNames(real.RemovedEdges); len(want) != 3 || !slices.Equal(got, want) {
		t.Errorf("dry run's removed-edges = %v, the real run's %v", got, want)
	}
}

// TestMoveNodeRepointsItsCitations: move-node moves the marker to the end of
// the named line and re-points the node's citations to its new host and
// section, leaving the citing words; a move to where the marker stands is
// unchanged; a work, an axiom and an unknown id are refused.
func TestMoveNodeRepointsItsCitations(t *testing.T) {
	other := "[↑ Entry](entry-point.md)\n\n# Other\n\n## Later On\n\nIt holds here too.\n\n" +
		"See [Main Result](leaf.md#leaf \"clm-aaaaaa\") and [\"the  main result holds\"](leaf.md#leaf \"clm-aaaaaa\").\n"
	root := nodesKB(t, nodesGraph(), other)
	e := onlyEntry(t, run(t, root, "move-node", "entry:\n- id: clm-aaaaaa\n  document: other.md\n  line: 7\n  snippet: holds here\n"))
	if e.ID != "clm-aaaaaa" {
		t.Errorf("entry id = %q, want clm-aaaaaa", e.ID)
	}
	want := "[↑ Entry](entry-point.md)\n\n# Other\n\n## Later On\n\nIt holds here too.<!-- kbase-id: clm-aaaaaa -->\n\n" +
		"See [Main Result](other.md#later-on \"clm-aaaaaa\") and [\"the  main result holds\"](other.md#later-on \"clm-aaaaaa\").\n"
	if got := recordFileText(t, root, "other.md"); got != want {
		t.Errorf("other.md =\n%s\nwant\n%s", got, want)
	}
	if got := recordFileText(t, root, "leaf.md"); strings.Contains(got, "kbase-id") {
		t.Errorf("leaf.md = %q, want the marker taken from it", got)
	}
	moved := snapshot(t, root)
	again := run(t, root, "move-node", "entry:\n- id: clm-aaaaaa\n  document: other.md\n  line: 7\n  snippet: holds here\n")
	if again.Outcome != result.Unchanged || len(again.Entries) != 1 || again.Entries[0].Outcome != result.Unchanged {
		t.Errorf("a move to where the marker stands = %+v, want unchanged", again)
	}
	if after := snapshot(t, root); !maps.Equal(moved, after) {
		t.Error("an unchanged move wrote to the KB")
	}
	for _, tc := range []struct{ id, check, detail, remedy string }{
		{"wrk-s00000", "", "wrk-s00000 is a work, which has no statement marker to move", remedy(nothingToMove)},
		{"axm-s10000", "", "axm-s10000 is an axiom, which has no statement marker to move", remedy(nothingToMove)},
		{"clm-zzzzzz", kb.CheckUnknownID, "no node clm-zzzzzz", ""},
	} {
		res := run(t, root, "move-node", "entry:\n- id: "+tc.id+"\n  document: leaf.md\n  line: 5\n  snippet: main result\n")
		if res.Outcome != result.Refused || len(res.Refusals) != 1 {
			t.Errorf("move of %s = %+v, want one refusal", tc.id, res)
			continue
		}
		if r := res.Refusals[0]; r.Key != "id" || r.Check != tc.check || r.Detail != tc.detail {
			t.Errorf("move of %s refused at %q, check %q, detail %q; want id, %q, %q", tc.id, r.Key, r.Check, r.Detail, tc.check, tc.detail)
		} else if tc.remedy != "" && r.Remedy != tc.remedy {
			t.Errorf("move of %s: remedy %q, want %q", tc.id, r.Remedy, tc.remedy)
		}
	}
}

// TestALeafHoldingACitationIsStillALeaf: a link in a leaf's prose, a
// citation, makes it no index: an add places its marker there, and
// remove-node takes the marker out of it.
func TestALeafHoldingACitationIsStillALeaf(t *testing.T) {
	citing := func(t *testing.T) string {
		t.Helper()
		root := nodesKB(t, nodesGraph(), plainOther)
		leaf := nodesLeaf + "\nSee [Lemma](other.md#other \"clm-bbbbbb\").\n"
		if err := os.WriteFile(filepath.Join(root, "leaf.md"), []byte(leaf), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	root := citing(t)
	res := run(t, root, "add-claim", "entry:\n- title: New Result\n  rationale: Shown.\n  document: leaf.md\n  line: 9\n  snippet: holds here\n")
	if res.Outcome != result.Done || len(res.Entries) != 1 {
		t.Errorf("add-claim = %+v, want done", res)
	} else if id, got := res.Entries[0].ID, recordFileText(t, root, "leaf.md"); !strings.Contains(got, "It holds here too."+kb.StatementMarker(id)) {
		t.Errorf("leaf.md = %q, want %s's marker on line 9", got, id)
	}
	root = citing(t)
	res = run(t, root, RemoveNode, "entry:\n- id: clm-aaaaaa\n")
	if got := recordFileText(t, root, "leaf.md"); res.Outcome != result.Done || strings.Contains(got, "clm-aaaaaa") {
		t.Errorf("remove-node: %s, leaf.md = %q, want done with the marker taken out", res.Outcome, got)
	}
}

// TestMoveNodeRefusesAnUnknownKeyListingItsOwnKeys: an unknown key in a
// move-node entry is refused at that key, allowing move-node's own keys,
// the id included.
func TestMoveNodeRefusesAnUnknownKeyListingItsOwnKeys(t *testing.T) {
	root := nodesKB(t, nodesGraph(), plainOther)
	res := run(t, root, "move-node", "entry:\n- id: clm-aaaaaa\n  document: leaf.md\n  line: 9\n  snippet: It holds\n  bogus: 1\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "bogus" {
		t.Fatalf("result = %+v, want refused at bogus", res)
	}
	if want := []string{"document", "id", "line", "snippet"}; !slices.Equal(res.Refusals[0].Allowed, want) {
		t.Errorf("allowed = %q, want %q", res.Refusals[0].Allowed, want)
	}
}

// TestMoveNodeRefusesALineThatLacksTheSnippet: move-node keeps the add ops'
// placement refusals, naming the line that holds the snippet.
func TestMoveNodeRefusesALineThatLacksTheSnippet(t *testing.T) {
	root := nodesKB(t, nodesGraph(), plainOther)
	res := run(t, root, "move-node", "entry:\n- id: clm-aaaaaa\n  document: leaf.md\n  line: 9\n  snippet: main result\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "snippet" {
		t.Fatalf("result = %+v, want refused at snippet", res)
	}
	if d := res.Refusals[0].Detail; !strings.Contains(d, "it appears on line 5") {
		t.Errorf("detail = %q, want it to name line 5", d)
	}
}
