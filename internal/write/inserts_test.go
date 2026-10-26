package write

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

const (
	hostedLeaf = "a.md"
	looseLeaf  = "b.md"
	looseLine  = "A loose statement stands here"
)

// insertGraph holds, of each kind an insert places, a node titled Hosted that
// a.md hosts and one titled Loose that no leaf marks; each experiment supports
// clm-aaaaaa.
func insertGraph() kb.Graph {
	trial := kb.Bearing{Value: 0.6}
	return kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Hosted", Rigor: f64(0.5), Rationale: "Seeded."},
			{ID: "clm-bbbbbb", Title: "Loose", Rigor: f64(0.5), Rationale: "Seeded."}},
		Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "Hosted", Rigor: f64(0.5), Rationale: "Seeded."},
			{ID: "ana-bbbbbb", Title: "Loose", Rigor: f64(0.5), Rationale: "Seeded."}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Hosted", Status: "run", Rigor: f64(0.5)},
			{ID: "exp-bbbbbb", Title: "Loose", Status: "run", Rigor: f64(0.5)}},
		Works:  []kb.WorkNode{{ID: "wrk-k", Title: "W", Rigor: f64(0.5), Rationale: "Cited."}},
		Axioms: []kb.AxiomNode{{ID: "axm-S1", Title: "Closure"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: "exp-aaaaaa", Target: "clm-aaaaaa", Bearing: trial},
			{Relation: kb.RelationSupports, Source: "exp-bbbbbb", Target: "clm-aaaaaa", Bearing: trial},
		},
	}
}

// insertKB is a KB holding g's records and two leaves: a.md, hosting the
// Hosted nodes, and b.md, hosting none.
func insertKB(t *testing.T, g kb.Graph) string {
	t.Helper()
	root := recordKB(t, g)
	for rel, text := range map[string]string{
		hostedLeaf: "# A\n\nThe hosted claim holds. <!-- claim-rigor: clm-aaaaaa -->\n\n## Hosted parts\n" +
			"<!-- claim-rigor: ana-aaaaaa -->\n<!-- claim-rigor: exp-aaaaaa -->\n\nThe parts.\n",
		looseLeaf: "# B leaf\n\n" + looseLine + ".\n\nThe other paragraph.\n",
	} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// placedNode is the node row of id the KB at root derives, and its host.
func placedNode(t *testing.T, root, id string) (kb.NodeRow, string) {
	t.Helper()
	rows, err := index.Derive(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(rows.Nodes, func(n kb.NodeRow) bool { return n.ID == id })
	if i < 0 {
		t.Fatalf("no node row for %s", id)
	}
	return rows.Nodes[i], rows.Hosts[id]
}

// placingInsert is one insert that places a marker: its op, its id prefix, and
// its values for a title and a document, "" for none.
type placingInsert struct {
	op, prefix string
	values     func(title, document string) string
}

var placingInserts = []placingInsert{
	{"insert-claim-entry", "clm", func(title, document string) string {
		v := "entry:\n- title: " + title + "\n  rationale: Seeded.\n  rigor: 0.5\n"
		if document != "" {
			v += "  document: " + document + "\n  locator: " + looseLine + "\n"
		}
		return v
	}},
	{"insert-analysis-entry", "ana", func(title, document string) string {
		v := "entry:\n- title: " + title + "\n  rationale: Seeded.\n  rigor: 0.5\n"
		if document != "" {
			v += "  document: " + document + "\n"
		}
		return v
	}},
	{"insert-experiment-entry", "exp", func(title, document string) string {
		return "entry:\n- title: " + title + "\n  document: " + document + "\n  status: run\n  rigor: 0.5\n" +
			"  supports:\n  - id: clm-aaaaaa\n    bearing: 0.6\n"
	}},
}

// TestAnInsertAdoptsByTitleAndHost: given a document, an insert adopts the
// node of its title hosted there, else one of its title no leaf marks, which
// it marks there; given none, only one no leaf marks; otherwise it mints. An
// adoption that writes nothing is unchanged, and one that marks is done,
// writing the leaf alone; a minted node is marked where a document is given,
// an analysis's or an experiment's statement then being its host's first
// heading's section, and a re-issue adopts it.
func TestAnInsertAdoptsByTitleAndHost(t *testing.T) {
	for _, in := range placingInserts {
		for _, c := range []struct {
			name, title, document, adopts string
			outcome                       string
			leafWritten                   bool
			host                          string
		}{
			{"hosted in the document", "Hosted", hostedLeaf, "aaaaaa", result.Unchanged, false, hostedLeaf},
			{"unmarked, marked in the document", "Loose", looseLeaf, "bbbbbb", result.Done, true, looseLeaf},
			{"unmarked, no document", "Loose", "", "bbbbbb", result.Unchanged, false, ""},
			{"hosted elsewhere, minted in the document", "Hosted", looseLeaf, "", result.Done, true, looseLeaf},
			{"hosted elsewhere, minted with no document", "Hosted", "", "", result.Done, false, ""},
			{"none, minted in the document", "New", looseLeaf, "", result.Done, true, looseLeaf},
		} {
			if in.prefix == "exp" && c.document == "" {
				continue
			}
			t.Run(in.op+" "+c.name, func(t *testing.T) {
				root := insertKB(t, insertGraph())
				before := snapshot(t, root)
				values := in.values(c.title, c.document)
				res := runRecordOp(t, root, in.op, values, ProvenanceCaller)
				if res.Outcome != c.outcome || len(res.IDs) != 1 {
					t.Fatalf("%s: %+v, want %s and one id", in.op, res, c.outcome)
				}
				id := res.IDs[0]
				if c.adopts != "" {
					if want := in.prefix + "-" + c.adopts; id != want || len(res.Minted) != 0 || len(res.Adopted) != 1 || res.Adopted[0].ID != want {
						t.Fatalf("%s: %+v, want %s adopted", in.op, res, want)
					}
				} else if !slices.Equal(res.Minted, res.IDs) || !strings.HasPrefix(id, in.prefix+"-") || len(res.Adopted) != 0 {
					t.Fatalf("%s: %+v, want one %s- id minted", in.op, res, in.prefix)
				}
				after := snapshot(t, root)
				if got := after[looseLeaf] != before[looseLeaf]; got != c.leafWritten || slices.Contains(res.Written, looseLeaf) != c.leafWritten {
					t.Errorf("%s wrote %v, %s moved %v; want it moved %v", in.op, res.Written, looseLeaf, got, c.leafWritten)
				}
				if c.adopts != "" && after[kb.GraphNodesFile] != before[kb.GraphNodesFile] {
					t.Errorf("an adopting %s wrote the node file", in.op)
				}
				if c.leafWritten && strings.Count(after[looseLeaf], kb.StatementMarker(id)) != 1 {
					t.Errorf("%s carries %s's marker other than once:\n%s", looseLeaf, id, after[looseLeaf])
				}
				row, host := placedNode(t, root, id)
				if host != c.host {
					t.Errorf("%s is hosted at %q, want %q", id, host, c.host)
				}
				if in.prefix != "clm" && c.host == looseLeaf && row.CanonicalAnchor != kb.Slugify("B leaf") {
					t.Errorf("%s's statement anchors at %q, want its host's first heading", id, row.CanonicalAnchor)
				}
				settled := snapshot(t, root)
				again := runRecordOp(t, root, in.op, values, ProvenanceCaller)
				if again.Outcome != result.Unchanged || !slices.Equal(again.IDs, res.IDs) || len(again.Minted) != 0 {
					t.Errorf("re-issued %s: %+v, want unchanged adopting %s", in.op, again, id)
				}
				if !maps.Equal(settled, snapshot(t, root)) {
					t.Errorf("a re-issued %s wrote the KB", in.op)
				}
			})
		}
	}
}

// TestAnExperimentAdoptsOnlyOnEqualValues: an experiment of the title and
// host is adopted only where its status, and the rigor and pair lists the
// values supply, equal the supplied ones — a key or a pair's bearing left out
// is not compared; otherwise one is minted.
func TestAnExperimentAdoptsOnlyOnEqualValues(t *testing.T) {
	for _, c := range []struct {
		name, values string
		adopted      bool
	}{
		{"another rigor", "  status: run\n  rigor: 0.4\n  supports:\n  - id: clm-aaaaaa\n    bearing: 0.6\n", false},
		{"another status", "  status: pending\n  rigor: 0.5\n  supports:\n  - id: clm-aaaaaa\n    bearing: 0.6\n", false},
		{"another bearing", "  status: run\n  rigor: 0.5\n  supports:\n  - id: clm-aaaaaa\n    bearing: 0.5\n", false},
		{"another pair", "  status: run\n  rigor: 0.5\n  supports:\n  - id: clm-bbbbbb\n    bearing: 0.6\n", false},
		{"no pairs", "  status: run\n  rigor: 0.5\n", true},
		{"no rigor", "  status: run\n  supports:\n  - id: clm-aaaaaa\n    bearing: 0.6\n", true},
		{"no pair bearing", "  status: run\n  rigor: 0.5\n  supports:\n  - id: clm-aaaaaa\n", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := insertKB(t, insertGraph())
			res := runRecordOp(t, root, "insert-experiment-entry", "entry:\n- title: Hosted\n  document: "+hostedLeaf+"\n"+c.values, ProvenanceCaller)
			if c.adopted {
				if res.Outcome != result.Unchanged || !slices.Equal(res.IDs, []string{"exp-aaaaaa"}) || len(res.Minted) != 0 {
					t.Errorf("insert-experiment-entry: %+v, want exp-aaaaaa adopted, nothing written", res)
				}
				return
			}
			if res.Outcome != result.Done || len(res.Minted) != 1 || len(res.Adopted) != 0 {
				t.Errorf("insert-experiment-entry: %+v, want one experiment minted", res)
			}
		})
	}
}

// TestAClaimIsPlacedOnlyWithBothKeys: insert-claim-entry given a document
// without a locator, or a locator without a document, is refused naming the
// missing key, and writes nothing.
func TestAClaimIsPlacedOnlyWithBothKeys(t *testing.T) {
	for _, c := range []struct{ given, missing string }{
		{"  document: " + looseLeaf + "\n", "locator"},
		{"  locator: " + looseLine + "\n", "document"},
	} {
		t.Run(c.missing, func(t *testing.T) {
			root := insertKB(t, insertGraph())
			before := snapshot(t, root)
			res := runRecordOp(t, root, "insert-claim-entry", "entry:\n- title: New\n  rationale: Shown.\n"+c.given, ProvenanceCaller)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.missing || res.Refusals[0].Entry != 1 {
				t.Fatalf("insert-claim-entry: %+v, want one refusal naming %s at entry 1", res, c.missing)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused insert wrote the KB")
			}
		})
	}
}

// TestAnInsertWritesTheFlagItsCallerEarns: a minted node's rigor and its
// edges' bearings land manually scored where the caller supplied them, and
// pending and not manually scored where it left them out; under machine
// provenance nothing lands manually scored. A premise from a work founds the
// node, and any other supports it.
func TestAnInsertWritesTheFlagItsCallerEarns(t *testing.T) {
	values := "entry:\n- title: New\n  rationale: Shown.\n  rigor: 0.4\n  rests-on:\n  - id: clm-aaaaaa\n    bearing: 0.5\n" +
		"  - id: exp-aaaaaa\n  - id: wrk-k\n    context: its bound\n  refutes:\n  - id: clm-bbbbbb\n"
	for _, c := range []struct {
		name       string
		provenance Provenance
		manual     bool
	}{{"caller", ProvenanceCaller, true}, {"machine", ProvenanceMachine, false}} {
		t.Run(c.name, func(t *testing.T) {
			root := insertKB(t, insertGraph())
			res := runRecordOp(t, root, "insert-claim-entry", values, c.provenance)
			if res.Outcome != result.Done || len(res.Minted) != 1 {
				t.Fatalf("insert-claim-entry: %+v", res)
			}
			id := res.Minted[0]
			g := records(t, root)
			n := g.Claims[slices.IndexFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == id })]
			if formatScore(n.Rigor) != "0.4" || n.ManuallyScored != c.manual {
				t.Errorf("claim = %+v, want rigor 0.4 manually-scored %v", n, c.manual)
			}
			want := []kb.ScorableEdge{
				{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: id, Bearing: kb.Bearing{Value: 0.5}, ManuallyScored: c.manual},
				{Relation: kb.RelationSupports, Source: "exp-aaaaaa", Target: id, Bearing: kb.Bearing{Pending: true}},
				{Relation: kb.RelationRefutes, Source: id, Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}},
			}
			for _, w := range want {
				if !slices.Contains(g.Scorable, w) {
					t.Errorf("no record %+v among %+v", w, g.Scorable)
				}
			}
			founds := kb.BearingEdge{Source: "wrk-k", Target: id, Context: "its bound", Bearing: kb.Bearing{Pending: true}}
			if !slices.Equal(g.Founds, []kb.BearingEdge{founds}) {
				t.Errorf("founds = %+v, want %+v", g.Founds, founds)
			}
		})
	}
	t.Run("omitted rigor", func(t *testing.T) {
		root := insertKB(t, insertGraph())
		res := runRecordOp(t, root, "insert-analysis-entry", "entry:\n- title: New\n  rationale: Shown.\n", ProvenanceCaller)
		if res.Outcome != result.Done || len(res.Minted) != 1 {
			t.Fatalf("insert-analysis-entry: %+v", res)
		}
		g := records(t, root)
		if n := g.Analyses[slices.IndexFunc(g.Analyses, func(n kb.AnalysisNode) bool { return n.ID == res.Minted[0] })]; n.Rigor != nil || n.ManuallyScored {
			t.Errorf("analysis = %+v, want rigor pending and not manually scored", n)
		}
	})
}

// TestAnInsertsPairBearingIsOptional: an analysis's and an experiment's pair
// list entry left without a bearing lands pending and not manually scored, and
// one given a bearing, *pending* included, lands it manually scored.
func TestAnInsertsPairBearingIsOptional(t *testing.T) {
	for _, in := range []struct {
		op, values string
		lists      []string
	}{
		{"insert-analysis-entry", "entry:\n- title: New\n  rationale: Shown.\n", []string{kb.LiftsKey, kb.RefutesKey}},
		{"insert-experiment-entry", "entry:\n- title: New\n  document: " + looseLeaf + "\n  status: run\n",
			[]string{kb.SupportsKey, kb.LiftsKey, kb.RefutesKey}},
	} {
		for _, c := range []struct {
			name, line string
			want       kb.Bearing
			manual     bool
		}{
			{"omitted", "", kb.Bearing{Pending: true}, false},
			{"a number", "    bearing: 0.5\n", kb.Bearing{Value: 0.5}, true},
			{"pending", "    bearing: \"" + kb.PendingLiteral + "\"\n", kb.Bearing{Pending: true}, true},
		} {
			for _, list := range in.lists {
				t.Run(in.op+" "+list+" "+c.name, func(t *testing.T) {
					root := insertKB(t, insertGraph())
					res := runRecordOp(t, root, in.op, in.values+"  "+list+":\n  - id: clm-bbbbbb\n"+c.line, ProvenanceCaller)
					if res.Outcome != result.Done || len(res.Minted) != 1 {
						t.Fatalf("%s: %+v", in.op, res)
					}
					want := kb.ScorableEdge{Relation: insertLists[list].relations[0], Source: res.Minted[0], Target: "clm-bbbbbb",
						Bearing: c.want, ManuallyScored: c.manual}
					if g := records(t, root); !slices.Contains(g.Scorable, want) {
						t.Errorf("no record %+v among %+v", want, g.Scorable)
					}
				})
			}
		}
	}
}

// TestAnInsertRefusesAnEdgeClosingACycle: a claim resting on clm-bbbbbb and
// refuting clm-aaaaaa, which supports clm-bbbbbb, would close a cycle; the
// insert is refused, check dependency-cycle, naming the path, and writes
// nothing.
func TestAnInsertRefusesAnEdgeClosingACycle(t *testing.T) {
	g := insertGraph()
	g.Scorable = append(g.Scorable, kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb",
		Bearing: kb.Bearing{Value: 0.5}})
	root := insertKB(t, g)
	before := snapshot(t, root)
	res := runRecordOp(t, root, "insert-claim-entry",
		"entry:\n- title: New\n  rationale: Shown.\n  rests-on:\n  - id: clm-bbbbbb\n  refutes:\n  - id: clm-aaaaaa\n", ProvenanceCaller)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("insert-claim-entry: %+v, want one refusal", res)
	}
	if r := res.Refusals[0]; r.Check != checkDependencyCycle || r.Key != kb.RefutesKey || r.Entry != 1 {
		t.Errorf("refusal = %+v, want check %s on entry 1's refutes", r, checkDependencyCycle)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused insert wrote the KB")
	}
}

// TestAnInsertReportsWhatItAdoptedDiffersIn: an adopted node's report names
// each supplied value it does not carry; a work is adopted by its key and an
// axiom by its label.
func TestAnInsertReportsWhatItAdoptedDiffersIn(t *testing.T) {
	for _, c := range []struct {
		op, values, id string
		differs        []string
	}{
		{"insert-claim-entry", "entry:\n- title: Loose\n  rationale: Seeded.\n  rigor: 0.1\n  rests-on:\n  - id: clm-aaaaaa\n", "clm-bbbbbb",
			[]string{"rigor", kb.ListRestsOn}},
		{"insert-work-entry", "entry:\n- key: k\n  title: Another\n  rationale: Cited.\n", "wrk-k", []string{"title"}},
		{"insert-axiom-entry", "entry:\n- label: S1\n  title: Another\n", "axm-S1", []string{"title"}},
		{"insert-axiom-entry", "entry:\n- label: S1\n  title: Closure\n", "axm-S1", []string{}},
	} {
		t.Run(c.op+" "+c.id, func(t *testing.T) {
			root := insertKB(t, insertGraph())
			before := snapshot(t, root)
			res := runRecordOp(t, root, c.op, c.values, ProvenanceCaller)
			if res.Outcome != result.Unchanged || len(res.Adopted) != 1 || res.Adopted[0].ID != c.id || !slices.Equal(res.Adopted[0].Differs, c.differs) {
				t.Fatalf("%s: %+v, want %s adopted differing in %v", c.op, res, c.id, c.differs)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("an adopting %s wrote the KB", c.op)
			}
		})
	}
}

// TestAWorkAndAnAxiomAreEnteredByTheirKeys: a work's id is its citation key's
// and an axiom's its label's, neither drawn; a work's rigor left out is
// pending, and a new key or label named twice in one batch is refused at the
// repeat, nothing written.
func TestAWorkAndAnAxiomAreEnteredByTheirKeys(t *testing.T) {
	root := insertKB(t, insertGraph())
	res := runRecordOp(t, root, "insert-work-entry", "entry:\n- key: Smith2020\n  title: Smith (2020)\n  rationale: Cited.\n", ProvenanceCaller)
	if res.Outcome != result.Done || !slices.Equal(res.IDs, []string{"wrk-Smith2020"}) || len(res.Minted) != 0 {
		t.Fatalf("insert-work-entry: %+v", res)
	}
	res = runRecordOp(t, root, "insert-axiom-entry", "entry:\n- label: S2\n  title: Openness\n", ProvenanceCaller)
	if res.Outcome != result.Done || !slices.Equal(res.IDs, []string{"axm-S2"}) || len(res.Minted) != 0 {
		t.Fatalf("insert-axiom-entry: %+v", res)
	}
	g := records(t, root)
	if !slices.Contains(g.Works, kb.WorkNode{ID: "wrk-Smith2020", Title: "Smith (2020)", Rationale: "Cited."}) {
		t.Errorf("works = %+v, want wrk-Smith2020 pending", g.Works)
	}
	if !slices.Contains(g.Axioms, kb.AxiomNode{ID: "axm-S2", Title: "Openness"}) {
		t.Errorf("axioms = %+v, want axm-S2", g.Axioms)
	}
	for _, c := range []struct{ op, key, values string }{
		{"insert-work-entry", "key", "entry:\n- key: Doe2021\n  title: Doe\n  rationale: Cited.\n- key: Doe2021\n  title: Doe\n  rationale: Cited.\n"},
		{"insert-axiom-entry", "label", "entry:\n- label: S3\n  title: Order\n- label: S3\n  title: Order\n"},
	} {
		before := snapshot(t, root)
		res := runRecordOp(t, root, c.op, c.values, ProvenanceCaller)
		if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.key || res.Refusals[0].Entry != 2 {
			t.Errorf("%s naming one new %s twice: %+v, want one refusal at %s, entry 2", c.op, c.key, res, c.key)
		}
		if !maps.Equal(before, snapshot(t, root)) {
			t.Errorf("a refused %s wrote the KB", c.op)
		}
	}
}
