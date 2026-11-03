package write

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/result"
)

// This repository's frozen failure corpus, its 23 claims and their premises
// held as records. The replay writes the same population through this package
// and requires all of it to read back.

const regressionClaims = 23

// regressionFormat is the format the corpus's records are held in.
const regressionFormat = "3.0.0"

type recorded struct {
	nodeID, title, rationale string
	rigor                    *float64
	premises                 []string
}

// loadRecorded reads the population out of the corpus's records, in record
// order: a copy of its 3.0.0 records, stamped so, converted at load.
func loadRecorded(t *testing.T) []recorded {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	if err := os.CopyFS(root, os.DirFS(filepath.Join(requireReferenceFixture(t, "writeapi-regression"), kb.KBDir))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, kb.FormatFile), kb.EncodeFormatStamp(regressionFormat), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	g, err := kb.LoadGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	var out []recorded
	for _, n := range g.Claims {
		r := recorded{nodeID: n.ID, title: n.Title, rationale: n.Rationale, rigor: n.Rigor}
		for _, e := range g.Scorable {
			if e.Relation == kb.RelationSupports && e.Target == n.ID {
				r.premises = append(r.premises, e.Source)
			}
		}
		out = append(out, r)
	}
	known := map[string]bool{}
	for _, r := range out {
		known[r.nodeID] = true
	}
	for _, r := range out {
		if r.rigor == nil {
			t.Fatalf("%s: rigor is pending; the replay renders numbers only", r.nodeID)
		}
		for _, d := range r.premises {
			if !known[d] {
				t.Fatalf("%s rests on %s, which the frozen corpus does not declare", r.nodeID, d)
			}
		}
	}
	return out
}

func recordedEdges(recs []recorded) map[[2]string]bool {
	out := map[[2]string]bool{}
	for _, r := range recs {
		for _, d := range r.premises {
			out[[2]string{r.nodeID, d}] = true
		}
	}
	return out
}

// TestRegressionReplayOps drives the corpus's values through add-claim into
// the records, minting fresh ids, and its premises through add-edge in a
// second batch; the graph must come back isomorphic under the renaming.
func TestRegressionReplayOps(t *testing.T) {
	recs := loadRecorded(t)
	if len(recs) != regressionClaims {
		t.Fatalf("recovered %d claims, want %d", len(recs), regressionClaims)
	}
	root := recordKB(t, kb.Graph{})
	leaf := "# Replay\n"
	for i := range recs {
		leaf += fmt.Sprintf("\nResult %d holds.\n", i)
	}
	writeFile(t, filepath.Join(root, kb.EntryPointFile), "# KB\n\n- [Replay](replay.md)\n")
	writeFile(t, filepath.Join(root, "replay.md"), leaf)
	replay := func(op string, values []map[string]any) Result {
		t.Helper()
		data, err := yaml.Marshal(map[string]any{entryKey: values})
		if err != nil {
			t.Fatal(err)
		}
		res := Run(op, Options{KBRoot: root, Values: data})
		if res.Outcome != result.Done {
			t.Fatalf("%s: %+v", op, res)
		}
		return res
	}
	var adds []map[string]any
	for i, r := range recs {
		// The corpus predates the rule that 1.00 is an axiom's alone: the one
		// entry recording it is replayed at the rigor tables' maximum, the most
		// add-claim takes.
		rigor := min(*r.rigor, kb.RigorMaximum)
		adds = append(adds, map[string]any{"title": r.title, "rigor": rigor, "rationale": r.rationale,
			"document": "replay.md", "line": 3 + 2*i, "snippet": fmt.Sprintf("Result %d holds", i)})
	}
	minted := map[string]string{}
	for i, id := range replay(AddClaim, adds).IDs() {
		minted[recs[i].nodeID] = id
	}
	var edges []map[string]any
	for _, r := range recs {
		for _, d := range r.premises {
			edges = append(edges, map[string]any{"source": minted[d], "target": minted[r.nodeID], "relation": kb.RelationSupports, "bearing": 1.0})
		}
	}
	replay(AddEdge, edges)
	if len(minted) != len(recs) {
		t.Fatalf("minted %d ids, want %d", len(minted), len(recs))
	}
	back := map[string]string{}
	for old, fresh := range minted {
		back[fresh] = old
		if old == fresh {
			t.Errorf("%s was written under its recorded id", old)
		}
	}
	g := records(t, root)
	if len(g.Claims) != regressionClaims {
		t.Errorf("%d claim records, want %d", len(g.Claims), regressionClaims)
	}
	titles := map[string]string{}
	for _, n := range g.Claims {
		titles[n.ID] = n.Title
	}
	for _, r := range recs {
		if got := titles[minted[r.nodeID]]; got != r.title {
			t.Errorf("%s (was %s) reads back titled %q, want %q", minted[r.nodeID], r.nodeID, got, r.title)
		}
	}
	got := map[[2]string]bool{}
	for _, e := range g.Scorable {
		if e.Relation == kb.RelationSupports {
			got[[2]string{back[e.Target], back[e.Source]}] = true
		}
	}
	if want := recordedEdges(recs); !maps.Equal(got, want) {
		t.Errorf("premise graph under the renaming %v, want %v", got, want)
	}
}
