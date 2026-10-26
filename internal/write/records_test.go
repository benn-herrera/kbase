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

// runRecordOp runs op over root with --no-refresh, as provenance supplies.
func runRecordOp(t *testing.T, root, op, values string, provenance Provenance) Result {
	t.Helper()
	return Run(op, Options{KBRoot: root, Values: []byte(values), NoRefresh: true, Provenance: provenance})
}

// records is the records root's files hold.
func records(t *testing.T, root string) kb.Graph {
	t.Helper()
	g, err := kb.LoadAuthoredGraph(kb.OnDisk(root))
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
		Works:       []kb.WorkNode{{ID: "wrk-k", Title: "W", Rigor: f64(0.5)}},
		Founds:      []kb.BearingEdge{{Source: "wrk-k", Target: "clm-aaaaaa", Bearing: half}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: half, ManuallyScored: manual},
			{Relation: kb.RelationLifts, Source: "exp-aaaaaa", Target: "clm-aaaaaa", Bearing: half, ManuallyScored: manual},
			{Relation: kb.RelationRefutes, Source: "ana-aaaaaa", Target: "clm-bbbbbb", Bearing: half, ManuallyScored: manual},
		},
		Demoted: []kb.DemotedEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Bearing: half}},
	}
}

// scored is the value and the manually-scored flag the record op's values name
// carries in g, the flag false on a type that carries none.
func scored(t *testing.T, g kb.Graph, op, id, target, relation string) (value string, manual bool) {
	t.Helper()
	if op == "set-rigor" {
		for _, n := range g.Claims {
			if n.ID == id {
				return formatScore(n.Rigor), n.ManuallyScored
			}
		}
		for _, n := range g.Analyses {
			if n.ID == id {
				return formatScore(n.Rigor), n.ManuallyScored
			}
		}
		for _, n := range g.Experiments {
			if n.ID == id {
				return formatScore(n.Rigor), n.ManuallyScored
			}
		}
		for _, n := range g.Works {
			if n.ID == id {
				return formatScore(n.Rigor), false
			}
		}
	}
	for _, r := range g.Founds {
		if relation == kb.RelationFounds && r.Source == id && r.Target == target {
			return kb.BearingText(r.Bearing), false
		}
	}
	for _, r := range g.Scorable {
		if r.Relation == relation && r.Source == id && r.Target == target {
			return kb.BearingText(r.Bearing), r.ManuallyScored
		}
	}
	t.Fatalf("no record of %s %s %s %s", op, id, relation, target)
	return "", false
}

// scoreWrite is one rigor or bearing write: the record it names, the value it
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
	if w.op == "set-rigor" {
		return "entry:\n  - id: " + w.id + "\n    rigor: " + value + "\n"
	}
	return "entry:\n  - id: " + w.id + "\n    target: " + w.target + "\n    relation: " + w.relation + "\n    bearing: " + value + "\n"
}

// scoreWrites is a write of value onto every record type a rigor or a bearing
// lands on.
func scoreWrites(value string) []scoreWrite {
	return []scoreWrite{
		{"claim rigor", "set-rigor", "clm-aaaaaa", "", "", value, true},
		{"analysis rigor", "set-rigor", "ana-aaaaaa", "", "", value, true},
		{"experiment rigor", "set-rigor", "exp-aaaaaa", "", "", value, true},
		{"work rigor", "set-rigor", "wrk-k", "", "", value, false},
		{"supports bearing", "set-bearing", "clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, value, true},
		{"lifts bearing", "set-bearing", "exp-aaaaaa", "clm-aaaaaa", kb.RelationLifts, value, true},
		{"refutes bearing", "set-bearing", "ana-aaaaaa", "clm-bbbbbb", kb.RelationRefutes, value, true},
		{"founds bearing", "set-bearing", "wrk-k", "clm-aaaaaa", kb.RelationFounds, value, false},
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
				got, manual := scored(t, records(t, root), w.op, w.id, w.target, w.relation)
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
			if got, manual := scored(t, records(t, root), w.op, w.id, w.target, w.relation); got != "0.3" || manual {
				t.Errorf("record carries %s, manually-scored %v; want 0.3, false", got, manual)
			}
		})
	}
}

// TestReassertingTheValueMarksItManual: set-rigor and set-bearing given the
// value a record carries, on a record not manually scored, land the flag true
// with outcome done; re-issued, they are unchanged, as is set-rationale given a
// value whose whitespace collapses to the one the record carries.
func TestReassertingTheValueMarksItManual(t *testing.T) {
	for _, w := range scoreWrites("0.5") {
		if !w.flagged {
			continue
		}
		t.Run(w.name, func(t *testing.T) {
			root := recordKB(t, scoredGraph(false))
			if res := runRecordOp(t, root, w.op, w.values(), ProvenanceCaller); res.Outcome != result.Done {
				t.Fatalf("first %s: %+v", w.op, res)
			}
			if got, manual := scored(t, records(t, root), w.op, w.id, w.target, w.relation); got != "0.5" || !manual {
				t.Errorf("record carries %s, manually-scored %v; want 0.5, true", got, manual)
			}
			if res := runRecordOp(t, root, w.op, w.values(), ProvenanceCaller); res.Outcome != result.Unchanged || len(res.Written) > 0 {
				t.Errorf("re-issued %s: %+v, want unchanged", w.op, res)
			}
		})
	}
	t.Run("collapsed rationale", func(t *testing.T) {
		root := recordKB(t, scoredGraph(false))
		res := runRecordOp(t, root, "set-rationale", "entry:\n  - id: clm-aaaaaa\n    rationale: \"  Seeded \\t value. \"\n", ProvenanceCaller)
		if res.Outcome != result.Unchanged || len(res.Written) > 0 {
			t.Errorf("set-rationale: %+v, want unchanged", res)
		}
	})
}

// TestABatchWritesOneValueToAField: a batch naming one record's field twice
// with values that differ, compared collapsed, is refused naming both entries
// and writes nothing; the same value twice, spelled with other whitespace, is
// written once.
func TestABatchWritesOneValueToAField(t *testing.T) {
	root := recordKB(t, scoredGraph(false))
	before := snapshot(t, root)
	res := runRecordOp(t, root, "set-rigor",
		"entry:\n  - id: clm-aaaaaa\n    rigor: 0.3\n  - id: clm-bbbbbb\n    rigor: 0.3\n  - id: clm-aaaaaa\n    rigor: 0.4\n", ProvenanceCaller)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Entry != 3 || res.Refusals[0].Key != "rigor" {
		t.Fatalf("conflicting set-rigor: %+v, want one refusal of entry 3's rigor", res)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Errorf("a refused batch wrote the KB")
	}
	res = runRecordOp(t, root, "set-rationale",
		"entry:\n  - id: clm-aaaaaa\n    rationale: Rewritten once.\n  - id: clm-aaaaaa\n    rationale: \" Rewritten   once. \"\n", ProvenanceCaller)
	if res.Outcome != result.Done || records(t, root).Claims[0].Rationale != "Rewritten once." {
		t.Errorf("one rationale twice: %+v, want it written", res)
	}
}

// TestAnOpLeavesTheFlagOfAValueItDoesNotWrite: set-rationale and set-status
// write their field and leave the record's manually-scored as it stood, and an
// op on a node record leaves the edge file unwritten.
func TestAnOpLeavesTheFlagOfAValueItDoesNotWrite(t *testing.T) {
	for _, manual := range []bool{false, true} {
		root := recordKB(t, scoredGraph(manual))
		edges := recordFileText(t, root, kb.GraphEdgesFile)
		for _, c := range []struct{ op, values string }{
			{"set-rationale", "entry:\n  - id: clm-aaaaaa\n    rationale: Rewritten.\n"},
			{"set-status", "entry:\n  - id: exp-aaaaaa\n    status: pending\n"},
		} {
			res := runRecordOp(t, root, c.op, c.values, ProvenanceMachine)
			if res.Outcome != result.Done || strings.Join(res.Written, ",") != kb.GraphNodesFile {
				t.Fatalf("%s: %+v, want done writing %s alone", c.op, res, kb.GraphNodesFile)
			}
		}
		g := records(t, root)
		if c := g.Claims[0]; c.Rationale != "Rewritten." || c.ManuallyScored != manual {
			t.Errorf("claim = %+v, want the new rationale and manually-scored %v", c, manual)
		}
		if x := g.Experiments[0]; x.Status != "pending" || x.ManuallyScored != manual {
			t.Errorf("experiment = %+v, want status pending and manually-scored %v", x, manual)
		}
		if got := recordFileText(t, root, kb.GraphEdgesFile); got != edges {
			t.Errorf("edge file moved under node ops:\n%s\nwant\n%s", got, edges)
		}
	}
}

// TestSetBearingWritesOnlyAnEdgeThatStands: set-bearing refuses an edge no
// record carries, naming that it creates none, and an edge a demoted record
// stands for, naming the demotion; neither writes anything.
func TestSetBearingWritesOnlyAnEdgeThatStands(t *testing.T) {
	for _, c := range []struct {
		name, values, check, detail string
	}{
		{"no record", "entry:\n  - id: clm-bbbbbb\n    target: ana-aaaaaa\n    relation: supports\n    bearing: 0.2\n",
			"", "never creates one"},
		{"demoted", "entry:\n  - id: clm-aaaaaa\n    target: clm-bbbbbb\n    relation: supports\n    bearing: 0.2\n",
			"demoted", "is demoted"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := recordKB(t, scoredGraph(false))
			before := snapshot(t, root)
			res := runRecordOp(t, root, "set-bearing", c.values, ProvenanceCaller)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Check != c.check ||
				!strings.Contains(res.Refusals[0].Detail, c.detail) {
				t.Fatalf("set-bearing: %+v, want one refusal, check %q, naming %q", res, c.check, c.detail)
			}
			if after := snapshot(t, root); !maps.Equal(before, after) {
				t.Errorf("a refused set-bearing wrote the KB")
			}
		})
	}
}

// TestAWriteOverADegradedLoadIsRefused: where the load is degraded — here an
// edge naming a node no record carries — a write op is refused, check
// degraded, naming the fault and kbase refresh, and writes nothing.
func TestAWriteOverADegradedLoadIsRefused(t *testing.T) {
	g := scoredGraph(false)
	g.References = []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-cccccc"}}
	root := recordKB(t, g)
	before := snapshot(t, root)
	res := runRecordOp(t, root, "set-rigor", "entry:\n  - id: clm-aaaaaa\n    rigor: 0.3\n", ProvenanceCaller)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("set-rigor over a degraded load: %+v, want one refusal", res)
	}
	if r := res.Refusals[0]; r.Check != kb.CheckDegraded || r.Remedy != "kbase refresh" || !strings.Contains(r.Detail, "clm-cccccc") {
		t.Errorf("refusal = %+v, want check %s naming clm-cccccc, remedy kbase refresh", r, kb.CheckDegraded)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Errorf("a refused write wrote the KB")
	}
}

// TestARigorAboveTheMaximumIsKept: a write that re-renders the node file keeps
// a rigor it found above the maximum on a record it did not write.
func TestARigorAboveTheMaximumIsKept(t *testing.T) {
	g := scoredGraph(false)
	g.Works[0].Rigor = f64(1.2)
	root := recordKB(t, g)
	if res := runRecordOp(t, root, "set-rigor", "entry:\n  - id: clm-aaaaaa\n    rigor: 0.3\n", ProvenanceCaller); res.Outcome != result.Done {
		t.Fatalf("set-rigor: %+v", res)
	}
	if r := records(t, root).Works[0].Rigor; r == nil || *r != 1.2 {
		t.Errorf("the work's rigor = %v, want 1.2 as the file held it", r)
	}
}
