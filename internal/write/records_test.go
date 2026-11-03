package write

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// recordKB writes a KB at the current format holding g's records and returns
// its kb-root.
func recordKB(t *testing.T, g kb.Graph) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	for rel, data := range map[string][]byte{kb.EntryPointFile: []byte("# KB\n"),
		kb.FormatFile:     kb.EncodeFormatStamp(kb.FormatVersion),
		kb.GraphNodesFile: kb.EncodeNodes(g), kb.GraphEdgesFile: kb.EncodeEdges(g)} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// derive brings the derived files of the KB at root current, as an empty batch
// does, so that an op run after it writes only the files its records change.
func derive(t *testing.T, root string) {
	t.Helper()
	if res := Run(Batch, Options{KBRoot: root, Values: []byte("entry: []\n")}); res.Outcome != result.Done && res.Outcome != result.Unchanged {
		t.Fatalf("deriving the seed: %+v", res)
	}
}

// runRecordOp runs op over root, as provenance supplies.
func runRecordOp(t *testing.T, root, op, values string, provenance Provenance) Result {
	t.Helper()
	return Run(op, Options{KBRoot: root, Values: []byte(values), Provenance: provenance})
}

// records is the records root's files hold.
func records(t *testing.T, root string) kb.Graph {
	t.Helper()
	g, err := kb.LoadGraph(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func recordFileText(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// scoredGraph holds one record of every type a rigor or a bearing is written
// on, each scored as manual says, and a demoted record standing for a cut
// supports edge.
func scoredGraph(manual bool) kb.Graph {
	half := kb.Bearing{Value: 0.5}
	return kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f64(0.5), ManuallyScored: manual, Rationale: "Seeded value."},
			{ID: "clm-bbbbbb", Title: "B", Rigor: f64(0.5), ManuallyScored: manual}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "An", Rigor: f64(0.5), ManuallyScored: manual}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "E", Status: "run", Rigor: f64(0.5), ManuallyScored: manual}},
		Works:       []kb.WorkNode{{ID: "wrk-k00000", Label: "k", Title: "W", Rigor: f64(0.5)}},
		Founds:      []kb.BearingEdge{{Source: "wrk-k00000", Target: "clm-aaaaaa", Bearing: half}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: half, ManuallyScored: manual},
			{Relation: kb.RelationLifts, Source: "exp-aaaaaa", Target: "clm-aaaaaa", Bearing: half, ManuallyScored: manual},
			{Relation: kb.RelationRefutes, Source: "ana-aaaaaa", Target: "clm-bbbbbb", Bearing: half, ManuallyScored: manual},
		},
		Demoted: []kb.DemotedEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Bearing: half}},
	}
}

// rigorText is a rigor as its values entry spells it: its number, or the
// pending literal.
func rigorText(r *float64) string {
	if r == nil {
		return kb.PendingLiteral
	}
	return kb.PyFloatRepr(*r)
}

// scored is the value and the manually-scored flag w's record carries in g,
// the flag false on a type that carries none.
func scored(t *testing.T, g kb.Graph, w scoreWrite) (value string, manual bool) {
	t.Helper()
	if w.relation == "" {
		for _, n := range g.Claims {
			if n.ID == w.id {
				return rigorText(n.Rigor), n.ManuallyScored
			}
		}
		for _, n := range g.Analyses {
			if n.ID == w.id {
				return rigorText(n.Rigor), n.ManuallyScored
			}
		}
		for _, n := range g.Experiments {
			if n.ID == w.id {
				return rigorText(n.Rigor), n.ManuallyScored
			}
		}
		for _, n := range g.Works {
			if n.ID == w.id {
				return rigorText(n.Rigor), false
			}
		}
	}
	for _, r := range g.Founds {
		if w.relation == kb.RelationFounds && r.Source == w.id && r.Target == w.target {
			return kb.BearingText(r.Bearing), false
		}
	}
	for _, r := range g.Scorable {
		if r.Relation == w.relation && r.Source == w.id && r.Target == w.target {
			return kb.BearingText(r.Bearing), r.ManuallyScored
		}
	}
	t.Fatalf("no record of %s %s %s %s", w.op, w.id, w.relation, w.target)
	return "", false
}

// scoreWrite is one rigor or bearing write: the record it names — a node's
// rigor where relation is "", an edge's bearing otherwise — the value it
// supplies, and whether the record's type carries the flag.
type scoreWrite struct {
	name, op, id, target, relation, value string
	flagged                               bool
}

func (w scoreWrite) values() string {
	value := w.value
	if value == kb.PendingLiteral {
		value = `"` + value + `"`
	}
	if w.relation == "" {
		return "entry:\n  - id: " + w.id + "\n    rigor: " + value + "\n"
	}
	return "entry:\n  - source: " + w.id + "\n    target: " + w.target + "\n    relation: " + w.relation + "\n    bearing: " + value + "\n"
}

// scoreWrites is a write of value onto every record type a rigor or a bearing
// lands on.
func scoreWrites(value string) []scoreWrite {
	return []scoreWrite{
		{"claim rigor", EditClaim, "clm-aaaaaa", "", "", value, true},
		{"analysis rigor", EditAnalysis, "ana-aaaaaa", "", "", value, true},
		{"experiment rigor", EditExperiment, "exp-aaaaaa", "", "", value, true},
		{"work rigor", EditWork, "wrk-k00000", "", "", value, false},
		{"supports bearing", EditEdge, "clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, value, true},
		{"lifts bearing", EditEdge, "exp-aaaaaa", "clm-aaaaaa", kb.RelationLifts, value, true},
		{"refutes bearing", EditEdge, "ana-aaaaaa", "clm-bbbbbb", kb.RelationRefutes, value, true},
		{"founds bearing", EditEdge, "wrk-k00000", "clm-aaaaaa", kb.RelationFounds, value, false},
	}
}

// TestACallerSuppliedValueIsManuallyScored: a public write op's rigor or
// bearing, a number or *pending*, lands with manually-scored true on every
// type that carries the flag, and as the bare value on a work's rigor and a
// founds bearing, which carry none.
func TestACallerSuppliedValueIsManuallyScored(t *testing.T) {
	for _, value := range []string{"0.3", kb.PendingLiteral} {
		for _, w := range scoreWrites(value) {
			t.Run(w.name+" "+value, func(t *testing.T) {
				root := recordKB(t, scoredGraph(false))
				if res := runRecordOp(t, root, w.op, w.values(), ProvenanceCaller); res.Outcome != result.Done {
					t.Fatalf("%s: %+v", w.op, res)
				}
				got, manual := scored(t, records(t, root), w)
				if got != value || manual != w.flagged {
					t.Errorf("record carries %s, manually-scored %v; want %s, %v", got, manual, value, w.flagged)
				}
			})
		}
	}
}

// TestAMachineValueIsNotManuallyScored: a value a run supplies, under
// ProvenanceMachine, lands with manually-scored false, over a record a person
// scored.
func TestAMachineValueIsNotManuallyScored(t *testing.T) {
	for _, w := range scoreWrites("0.3") {
		t.Run(w.name, func(t *testing.T) {
			root := recordKB(t, scoredGraph(true))
			if res := runRecordOp(t, root, w.op, w.values(), ProvenanceMachine); res.Outcome != result.Done {
				t.Fatalf("%s: %+v", w.op, res)
			}
			if got, manual := scored(t, records(t, root), w); got != "0.3" || manual {
				t.Errorf("record carries %s, manually-scored %v; want 0.3, false", got, manual)
			}
		})
	}
}

// TestReassertingTheValueMarksItManual: a rigor or a bearing edit given the
// value a record carries, on a record not manually scored, lands the flag true
// with outcome done; re-issued, it is unchanged, as is an edit-claim given a
// rationale whose whitespace collapses to the one the record carries.
func TestReassertingTheValueMarksItManual(t *testing.T) {
	for _, w := range scoreWrites("0.5") {
		if !w.flagged {
			continue
		}
		t.Run(w.name, func(t *testing.T) {
			root := recordKB(t, scoredGraph(false))
			derive(t, root)
			if res := runRecordOp(t, root, w.op, w.values(), ProvenanceCaller); res.Outcome != result.Done {
				t.Fatalf("first %s: %+v", w.op, res)
			}
			if got, manual := scored(t, records(t, root), w); got != "0.5" || !manual {
				t.Errorf("record carries %s, manually-scored %v; want 0.5, true", got, manual)
			}
			if res := runRecordOp(t, root, w.op, w.values(), ProvenanceCaller); res.Outcome != result.Unchanged || len(res.Written) > 0 {
				t.Errorf("re-issued %s: %+v, want unchanged", w.op, res)
			}
		})
	}
	t.Run("collapsed rationale", func(t *testing.T) {
		root := recordKB(t, scoredGraph(false))
		derive(t, root)
		res := runRecordOp(t, root, EditClaim, "entry:\n  - id: clm-aaaaaa\n    rationale: \"  Seeded \\t value. \"\n", ProvenanceCaller)
		if res.Outcome != result.Unchanged || len(res.Written) > 0 {
			t.Errorf("edit-claim: %+v, want unchanged", res)
		}
	})
}

// TestAnOpLeavesTheFlagOfAValueItDoesNotWrite: an edit of a claim's rationale
// and of an experiment's status writes its field and leaves the record's
// manually-scored as it stood, and an op on a node record leaves the edge file
// unwritten.
func TestAnOpLeavesTheFlagOfAValueItDoesNotWrite(t *testing.T) {
	for _, manual := range []bool{false, true} {
		root := recordKB(t, scoredGraph(manual))
		derive(t, root)
		edges := recordFileText(t, root, kb.GraphEdgesFile)
		for _, c := range []struct{ op, values string }{
			{EditClaim, "entry:\n  - id: clm-aaaaaa\n    rationale: Rewritten.\n"},
			{EditExperiment, "entry:\n  - id: exp-aaaaaa\n    status: planned\n"},
		} {
			res := runRecordOp(t, root, c.op, c.values, ProvenanceMachine)
			var recordsWritten []string
			for _, p := range res.Written {
				if strings.HasPrefix(p, kb.IndexDir+"/") {
					recordsWritten = append(recordsWritten, p)
				}
			}
			if res.Outcome != result.Done || strings.Join(recordsWritten, ",") != kb.GraphNodesFile {
				t.Fatalf("%s: %+v, want done writing %s alone of the record files", c.op, res, kb.GraphNodesFile)
			}
		}
		g := records(t, root)
		if c := g.Claims[0]; c.Rationale != "Rewritten." || c.ManuallyScored != manual {
			t.Errorf("claim = %+v, want the new rationale and manually-scored %v", c, manual)
		}
		if x := g.Experiments[0]; x.Status != kb.ExperimentPlanned || x.ManuallyScored != manual {
			t.Errorf("experiment = %+v, want status planned and manually-scored %v", x, manual)
		}
		if got := recordFileText(t, root, kb.GraphEdgesFile); got != edges {
			t.Errorf("edge file moved under node ops:\n%s\nwant\n%s", got, edges)
		}
	}
}

// TestAWriteOverALoadErrorIsRefused: where the load fails — here on an edge
// naming a node no record carries — a write op is refused, check metadata,
// naming the fault and the remedy in git, and writes nothing.
func TestAWriteOverALoadErrorIsRefused(t *testing.T) {
	g := scoredGraph(false)
	g.References = []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-cccccc"}}
	root := recordKB(t, g)
	before := snapshot(t, root)
	res := runRecordOp(t, root, EditClaim, "entry:\n  - id: clm-aaaaaa\n    rigor: 0.3\n", ProvenanceCaller)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("edit-claim over a load error: %+v, want one refusal", res)
	}
	if r := res.Refusals[0]; r.Check != kb.CheckMetadata || r.Remedy != kb.ConsistentRecordsRemedy || !strings.Contains(r.Detail, "clm-cccccc") {
		t.Errorf("refusal = %+v, want check %s naming clm-cccccc, remedy %q", r, kb.CheckMetadata, kb.ConsistentRecordsRemedy)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Errorf("a refused write wrote the KB")
	}
}
