package write

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

const (
	dropped, other, refuted, isolated = "clm-drop00", "clm-other0", "clm-refut0", "clm-isol00"
	dropAnalysis, dropExperiment      = "ana-aaaaaa", "exp-aaaaaa"
)

// edgeName is an edge as the drop tests compare it.
type edgeName struct{ relation, source, target string }

// edgeNames is every edge record of g but the demoted ones.
func edgeNames(g kb.Graph) []edgeName {
	var out []edgeName
	for _, r := range g.References {
		out = append(out, edgeName{kb.RelationReferences, r.Source, r.Target})
	}
	for _, r := range g.Founds {
		out = append(out, edgeName{kb.RelationFounds, r.Source, r.Target})
	}
	for _, r := range g.Scorable {
		out = append(out, edgeName{r.Relation, r.Source, r.Target})
	}
	return out
}

// Edges naming the dropped claim, by whose list they are.
var (
	// ownEdges are the claim's own lists: its rests-on — a claim's, a work's
	// and an experiment's premise — and its references.
	ownEdges = []edgeName{
		{kb.RelationSupports, seedClaim, dropped}, {kb.RelationFounds, "wrk-k", dropped},
		{kb.RelationReferences, dropped, seedClaim},
	}
	// inboundEdges are every other node's edges naming it: another claim's
	// rests-on naming it, a references and a refutation of it, an analysis's
	// lift and refutation of it, and an experiment's support and lift of it.
	inboundEdges = []edgeName{
		{kb.RelationSupports, dropped, other}, {kb.RelationReferences, other, dropped}, {kb.RelationRefutes, refuted, dropped},
		{kb.RelationLifts, dropAnalysis, dropped}, {kb.RelationRefutes, dropAnalysis, dropped},
		{kb.RelationSupports, dropExperiment, dropped}, {kb.RelationLifts, dropExperiment, dropped},
	}
	// unrelatedEdges name the claim in neither list: its own refutation of
	// another, and an edge between two others.
	unrelatedEdges = []edgeName{{kb.RelationRefutes, dropped, other}, {kb.RelationSupports, seedClaim, other}}
)

// dropGraph holds a claim carrying every edge list.
func dropGraph() kb.Graph {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: seedClaim, Title: "Seed"}, {ID: dropped, Title: "Dropped"}, {ID: other, Title: "Other"},
			{ID: refuted, Title: "Refuting"}, {ID: isolated, Title: "Isolated"}},
		Analyses:    []kb.AnalysisNode{{ID: dropAnalysis, Title: "An"}},
		Experiments: []kb.ExperimentNode{{ID: dropExperiment, Title: "E", Status: "run"}},
		Works:       []kb.WorkNode{{ID: "wrk-k", Title: "W"}},
	}
	for _, e := range slices.Concat(ownEdges, inboundEdges, unrelatedEdges) {
		switch e.relation {
		case kb.RelationReferences:
			g.References = append(g.References, kb.UnscoredEdge{Source: e.source, Target: e.target})
		case kb.RelationFounds:
			g.Founds = append(g.Founds, kb.BearingEdge{Source: e.source, Target: e.target, Bearing: kb.Bearing{Pending: true}})
		default:
			g.Scorable = append(g.Scorable, kb.ScorableEdge{Relation: e.relation, Source: e.source, Target: e.target, Bearing: kb.Bearing{Pending: true}})
		}
	}
	return g
}

// TestDropEdges: drop-claim-edges removes a claim's own lists, the class named
// or both, and drop-inbound-edges every other node's edges naming it; an
// experiment's support of the claim is in both sets. Every other edge stands,
// a re-issue is unchanged and byte-identical, and a claim no such edge names
// is unchanged.
func TestDropEdges(t *testing.T) {
	isRestsOn := func(e edgeName) bool { return e.relation != kb.RelationReferences }
	isReference := func(e edgeName) bool { return e.relation == kb.RelationReferences }
	experimentSupport := edgeName{kb.RelationSupports, dropExperiment, dropped}
	for _, c := range []struct {
		name, op, values string
		gone             []edgeName
	}{
		{"both classes", "drop-claim-edges", "entry:\n- id: " + dropped + "\n", append(slices.Clone(ownEdges), experimentSupport)},
		{"rests-on", "drop-claim-edges", "entry:\n- id: " + dropped + "\n  edge-class: rests-on\n",
			append(slices.DeleteFunc(slices.Clone(ownEdges), isReference), experimentSupport)},
		{"references", "drop-claim-edges", "entry:\n- id: " + dropped + "\n  edge-class: references\n",
			slices.DeleteFunc(slices.Clone(ownEdges), isRestsOn)},
		{"inbound", "drop-inbound-edges", "entry:\n- id: " + dropped + "\n", inboundEdges},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := recordKB(t, dropGraph())
			want := slices.DeleteFunc(edgeNames(records(t, root)), func(e edgeName) bool { return slices.Contains(c.gone, e) })
			if res := runRecordOp(t, root, c.op, c.values, ProvenanceCaller); res.Outcome != result.Done {
				t.Fatalf("%s: %+v", c.op, res)
			}
			if got := edgeNames(records(t, root)); !slices.Equal(got, want) {
				t.Errorf("edges = %v, want %v", got, want)
			}
			before := snapshot(t, root)
			if again := runRecordOp(t, root, c.op, c.values, ProvenanceCaller); again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("again: %+v, want unchanged, byte-identical", again)
			}
			if res := runRecordOp(t, root, c.op, "entry:\n- id: "+isolated+"\n", ProvenanceCaller); res.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("a claim no such edge names: %+v, want unchanged, byte-identical", res)
			}
		})
	}
	root := recordKB(t, dropGraph())
	res := runRecordOp(t, root, "drop-claim-edges", "entry:\n- id: "+dropped+"\n  edge-class: demoted\n", ProvenanceCaller)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "edge-class" || res.Refusals[0].Entry != 1 {
		t.Errorf("edge-class demoted: %+v, want one refusal at entry 1 keyed edge-class", res)
	}
}

// seedSharedLeaf seeds a KB whose leaf hosts the seed claim and a second,
// "Bound Result", inserted with extra and marked in the leaf's prose; it
// returns kb-root and the second claim's id.
func seedSharedLeaf(t *testing.T, extra string) (string, string) {
	t.Helper()
	root := seedKB(t)
	b := insertClaim(t, root, "Bound Result", extra)
	if res := run(t, root, "mark-claim-in-leaf", "entry:\n- document: leaf.md\n  id: "+b+"\n  locator: a second paragraph names a bound\n"); res.Outcome != result.Done {
		t.Fatalf("mark-claim-in-leaf: %+v", res)
	}
	return root, b
}

// TestDropClaimNode deletes a claim node whole: its record and its marker go,
// the rest of each file standing, and a re-issue is unchanged. A premise the
// claim rests on goes with it, while a premise another claim rests on it
// refuses the drop, naming the edge and the two edge ops.
func TestDropClaimNode(t *testing.T) {
	root, b := seedSharedLeaf(t, "")
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n")
	if res.Outcome != result.Done {
		t.Fatalf("drop: %+v", res)
	}
	if g := records(t, root); slices.ContainsFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == b }) || len(g.Claims) != 1 {
		t.Errorf("claims = %+v, want the seed alone", g.Claims)
	}
	leaf := snapshot(t, root)["leaf.md"]
	if strings.Contains(leaf, b) || !strings.Contains(leaf, "A second paragraph names a bound.  \n") ||
		!strings.Contains(leaf, "holds here. "+kb.StatementMarker(seedClaim)) {
		t.Errorf("the leaf lost other than B's marker:\n%s", leaf)
	}
	before := snapshot(t, root)
	again := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n")
	if again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("again: %+v, want unchanged, byte-identical", again)
	}

	root, b = seedSharedLeaf(t, "  rests-on:\n  - id: "+seedClaim+"\n")
	before = snapshot(t, root)
	res = run(t, root, "drop-claim-node", "entry:\n- id: "+seedClaim+"\n")
	if !refusedAt(res, result.Item{Key: "id", Entry: 1}) {
		t.Errorf("drop the premise another claim rests on: %+v, want one refusal of entry 1's id", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused drop changed kb-root")
	}
	if res := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n"); res.Outcome != result.Done {
		t.Fatalf("drop the claim resting on the seed: %+v, want done", res)
	}
	if g := records(t, root); len(g.Scorable) != 0 || len(g.Claims) != 1 {
		t.Errorf("records = %+v, want the seed alone and no edge", g)
	}
}

// TestDropClaimNodeTakesItsRefutations: a claim refuting another is dropped
// with its refutation, while the claim it refutes cannot be dropped from under
// the refutation.
func TestDropClaimNodeTakesItsRefutations(t *testing.T) {
	root := seedKB(t)
	refuter := insertClaim(t, root, "Counter Result", "  refutes:\n  - id: "+seedClaim+"\n")
	before := snapshot(t, root)
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+seedClaim+"\n")
	if !refusedAt(res, result.Item{Key: "id", Entry: 1}) {
		t.Errorf("drop the refuted claim: %+v, want one refusal of entry 1's id", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused drop changed kb-root")
	}
	if res := run(t, root, "drop-claim-node", "entry:\n- id: "+refuter+"\n"); res.Outcome != result.Done {
		t.Fatalf("drop the refuter: %+v, want done", res)
	}
	g := records(t, root)
	if len(g.Scorable) != 0 || slices.ContainsFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == refuter }) {
		t.Errorf("records = %+v, want the refuter and its refutation gone", g)
	}
}

// TestDropClaimNodeTakesEveryClaimALeafHosts: a batch dropping every claim a
// leaf hosts lands, the leaf losing both markers.
func TestDropClaimNodeTakesEveryClaimALeafHosts(t *testing.T) {
	root, b := seedSharedLeaf(t, "")
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+seedClaim+"\n- id: "+b+"\n")
	if res.Outcome != result.Done {
		t.Fatalf("drop both: %+v", res)
	}
	if leaf := snapshot(t, root)["leaf.md"]; strings.Contains(leaf, "claim-rigor") || !strings.Contains(leaf, "The first result holds here.\n") {
		t.Errorf("the leaf kept a marker, or lost its prose:\n%s", leaf)
	}
}

// TestDropClaimNodeDemotedRecords: every demoted record naming the node goes
// with it, at either end, and the records of others stand.
func TestDropClaimNodeDemotedRecords(t *testing.T) {
	pending := kb.Bearing{Pending: true}
	root := recordKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: seedClaim, Title: "Seed"}, {ID: dropped, Title: "Dropped"}, {ID: other, Title: "Other"}},
		Demoted: []kb.DemotedEdge{
			{Source: dropped, Target: other, ReplacedRelation: kb.RelationSupports, Bearing: pending},
			{Source: other, Target: dropped, ReplacedRelation: kb.RelationRefutes, Bearing: pending},
			{Source: seedClaim, Target: other, ReplacedRelation: kb.RelationSupports, Bearing: pending},
		},
	})
	if res := runRecordOp(t, root, "drop-claim-node", "entry:\n- id: "+dropped+"\n", ProvenanceCaller); res.Outcome != result.Done {
		t.Fatalf("drop: %+v, want done", res)
	}
	want := []kb.DemotedEdge{{Source: seedClaim, Target: other, ReplacedRelation: kb.RelationSupports, Bearing: pending}}
	if got := records(t, root).Demoted; !slices.Equal(got, want) {
		t.Errorf("demoted = %+v, want %+v", got, want)
	}
}

// TestDropClaimNodeRecordMention: a prose field of another record naming the
// node — which the trailing refresh renders onto a register page — refuses the
// drop, naming the record file, the record and the field, and nothing is
// written.
func TestDropClaimNodeRecordMention(t *testing.T) {
	mention := "as " + dropped + " shows"
	for _, c := range []struct {
		name, file, record, field string
		stage                     func(g *kb.Graph)
	}{
		{"rationale", kb.GraphNodesFile, other, "rationale", func(g *kb.Graph) { g.Claims[1].Rationale = mention }},
		{"strengthen-by", kb.GraphNodesFile, other, "strengthen-by[1]", func(g *kb.Graph) { g.Claims[1].StrengthenBy = []string{mention} }},
		{"work rationale", kb.GraphNodesFile, "wrk-k", "rationale", func(g *kb.Graph) { g.Works[0].Rationale = mention }},
		{"edge context", kb.GraphEdgesFile, kb.RelationSupports + " " + seedClaim + " → " + other, "context",
			func(g *kb.Graph) { g.Scorable[0].Context = mention }},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := kb.Graph{
				Claims:   []kb.ClaimNode{{ID: dropped, Title: "Dropped"}, {ID: other, Title: "Other"}, {ID: seedClaim, Title: "Seed"}},
				Works:    []kb.WorkNode{{ID: "wrk-k", Title: "W"}},
				Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: seedClaim, Target: other, Bearing: kb.Bearing{Pending: true}}},
			}
			c.stage(&g)
			root := recordKB(t, g)
			before := snapshot(t, root)
			res := runRecordOp(t, root, "drop-claim-node", "entry:\n- id: "+dropped+"\n", ProvenanceCaller)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 {
				t.Fatalf("drop: %+v, want one refusal", res)
			}
			if it := res.Refusals[0]; it.Key != "id="+dropped || it.Path != c.file || !strings.Contains(it.Detail, c.record+"'s "+c.field) {
				t.Errorf("refusal = %+v, want id=%s at %s naming %s's %s", it, dropped, c.file, c.record, c.field)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused drop changed kb-root")
			}
		})
	}
}

// lineOf is the 1-based number of text's first line containing sub; the
// fixture failing where none does.
func lineOf(t *testing.T, text, sub string) int {
	t.Helper()
	for n, line := range strings.Split(text, "\n") {
		if strings.Contains(line, sub) {
			return n + 1
		}
	}
	t.Fatalf("no line contains %q:\n%s", sub, text)
	return 0
}

// refusedAt is whether res is refused with exactly the items want, compared on
// check, key, entry, path and line.
func refusedAt(res Result, want ...result.Item) bool {
	if res.Outcome != result.Refused || len(res.Refusals) != len(want) {
		return false
	}
	for i, it := range res.Refusals {
		w := want[i]
		if it.Check != w.Check || it.Key != w.Key || it.Entry != w.Entry || it.Path != w.Path || it.Line != w.Line {
			return false
		}
	}
	return true
}

// TestDropClaimNodeProseMention: a prose mention of the node in a leaf that
// does not host it refuses the drop, naming the mention's path:line, and
// nothing is written; the register page refresh renders is not read.
func TestDropClaimNodeProseMention(t *testing.T) {
	root, b := seedSharedLeaf(t, "")
	if !strings.Contains(snapshot(t, root)[kb.RegisterFile], b) {
		t.Fatalf("the register page refresh rendered does not name %s; the fixture needs it to", b)
	}
	leafPath := filepath.Join(root, "leaf.md")
	text, err := os.ReadFile(leafPath)
	if err != nil {
		t.Fatal(err)
	}
	staged := strings.Replace(string(text), "The bound is tight in the limit.", "The bound is tight in the limit, as "+b+" shows.", 1)
	if staged == string(text) {
		t.Fatal("no sentence to stage the mention in")
	}
	if err := os.WriteFile(leafPath, []byte(staged), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	line := lineOf(t, staged, "as "+b+" shows")
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n")
	if !refusedAt(res, result.Item{Key: "id=" + b, Path: "leaf.md", Line: line}) {
		t.Errorf("prose mention: %+v, want one refusal of id=%s at leaf.md:%d", res, b, line)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused drop changed kb-root")
	}
}
