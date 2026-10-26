package write

import (
	"maps"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// This repository's frozen failure corpus, its 23 claims and their premises
// held as records. The replay writes the same population through this package
// and requires all of it to read back.

const regressionClaims = 23

type recorded struct {
	nodeID, title, rationale string
	rigor                    *float64
	premises                 []string
}

// loadRecorded reads the population out of the corpus's records, in record
// order.
func loadRecorded(t *testing.T) []recorded {
	t.Helper()
	g, err := kb.LoadAuthoredGraph(kb.OnDisk(filepath.Join(requireReferenceFixture(t, "writeapi-regression"), kb.KBDir)))
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

// TestRegressionReplayOps drives the corpus's values through the insert op
// into the records, minting fresh ids, in dependency order; the graph must
// come back isomorphic under the renaming.
func TestRegressionReplayOps(t *testing.T) {
	recs := loadRecorded(t)
	if len(recs) != regressionClaims {
		t.Fatalf("recovered %d claims, want %d", len(recs), regressionClaims)
	}
	root := recordKB(t, kb.Graph{})
	minted := map[string]string{}
	remaining := slices.Clone(recs)
	for len(remaining) > 0 {
		var ready, rest []recorded
		for _, r := range remaining {
			blocked := false
			for _, d := range r.premises {
				blocked = blocked || slices.ContainsFunc(remaining, func(o recorded) bool { return o.nodeID == d })
			}
			if blocked {
				rest = append(rest, r)
			} else {
				ready = append(ready, r)
			}
		}
		if len(ready) == 0 {
			t.Fatal("the recorded premise graph has a cycle")
		}
		var values []map[string]any
		for _, r := range ready {
			// The corpus predates the rule that 1.00 is an axiom's alone: the one
			// entry recording it is replayed at the rigor tables' maximum, the most
			// the insert op takes.
			rigor := min(*r.rigor, kb.RigorMaximum)
			v := map[string]any{"title": r.title, "rigor": rigor, "rationale": r.rationale}
			var deps []map[string]any
			for _, d := range r.premises {
				deps = append(deps, map[string]any{"id": minted[d], "bearing": 1.0})
			}
			if len(deps) > 0 {
				v[kb.ListRestsOn] = deps
			}
			values = append(values, v)
		}
		data, err := yaml.Marshal(map[string]any{entryKey: values})
		if err != nil {
			t.Fatal(err)
		}
		res := Run("insert-claim-entry", Options{KBRoot: root, Values: data, NoRefresh: true})
		if res.Outcome != result.Done {
			t.Fatalf("insert-claim-entry: %+v", res)
		}
		for i, r := range ready {
			minted[r.nodeID] = res.IDs[i]
		}
		remaining = rest
	}
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
