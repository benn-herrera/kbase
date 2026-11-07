package write

import (
	"strings"
	"testing"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestAnEdgeRemedyCallsEdgesAsTheSurfaceDoes: the remedies of a cycle refusal
// and of an edge-exists refusal point at the edges query through the marked
// call, never a literal CLI command.
func TestAnEdgeRemedyCallsEdgesAsTheSurfaceDoes(t *testing.T) {
	root := recordKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}},
		},
	})
	for _, tc := range []struct{ name, entry string }{
		{"cycle", edgeEntry(AddEdge, "clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports)},
		{"edge exists", edgeEntry(AddEdge, "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, "context: by it")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := runEdges(t, root, tc.entry)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 {
				t.Fatalf("result = %+v, want one refusal", res)
			}
			remedy, call := res.Refusals[0].Remedy, argmark.Call("edges", nil)
			if !strings.Contains(remedy, call) || strings.Contains(remedy, "kbase edges") {
				t.Errorf("remedy = %q, want the call %s and no literal kbase edges", remedy, call)
			}
		})
	}
}

// TestANoNodeOrNoEdgeRemedyFollowsItsOp: a refusal of a node or an edge that
// does not stand takes its branch — a removal's, whose thing may already be
// gone; a minted id's, every kind's a work's and an axiom's included, named
// only in a later batch; or an edge reference's — at the key the entry named
// it under.
func TestANoNodeOrNoEdgeRemedyFollowsItsOp(t *testing.T) {
	branch := func(op, id string) string { return strings.ReplaceAll(noNodeRemedy(op, "id", id), id, "<id>") }
	removed, minted := branch(RemoveNode, "clm-zzzzzz"), branch(EditClaim, "clm-zzzzzz")
	for _, kind := range []string{branch(EditWork, "wrk-n00000"), branch(EditAxiom, "axm-s90000")} {
		if kind != minted {
			t.Errorf("a work's or an axiom's no-node remedy %q is not a minted id's, %q", kind, minted)
		}
	}
	if removed == minted {
		t.Error("a removal's no-node remedy is a minted id's")
	}
	if noEdgeRemedy(RemoveEdge) == noEdgeRemedy(EditEdge) {
		t.Error("a removal's no-edge remedy is a reference's")
	}
	const unknown = "clm-zzzzzz"
	place := "  document: leaf.md\n  line: 5\n  snippet: holds\n"
	for _, tc := range []struct {
		name, entry, want string
	}{
		{"remove-node", "- operation: remove-node\n  id: " + unknown + "\n", noNodeRemedy(RemoveNode, "id", unknown)},
		{"edit-claim", "- operation: edit-claim\n  id: " + unknown + "\n  title: T\n", noNodeRemedy(EditClaim, "id", unknown)},
		{"edit-axiom", "- operation: edit-axiom\n  id: axm-s90000\n  title: T\n", noNodeRemedy(EditAxiom, "id", "axm-s90000")},
		{"move-node", "- operation: move-node\n  id: " + unknown + "\n" + place, noNodeRemedy(MoveNode, "id", unknown)},
		{"add-citation", "- operation: add-citation\n  id: " + unknown + "\n" + place, noNodeRemedy(AddCitation, "id", unknown)},
		{"add-edge source", edgeEntry(AddEdge, unknown, "clm-aaaaaa", kb.RelationSupports), noNodeRemedy(AddEdge, "source", unknown)},
		{"add-edge work source", edgeEntry(AddEdge, "wrk-n00000", "clm-aaaaaa", kb.RelationFounds), noNodeRemedy(AddEdge, "source", "wrk-n00000")},
		{"remove-edge target", edgeEntry(RemoveEdge, "clm-aaaaaa", unknown, kb.RelationSupports), noNodeRemedy(RemoveEdge, "target", unknown)},
		{"edit-edge", edgeEntry(EditEdge, "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, "context: by it"), noEdgeRemedy(EditEdge)},
		{"remove-edge", edgeEntry(RemoveEdge, "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports), noEdgeRemedy(RemoveEdge)},
		{"restore-demoted", edgeEntry(RestoreDemoted, "clm-aaaaaa", "clm-bbbbbb", kb.RelationDemoted), noEdgeRemedy(RestoreDemoted)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := runEdges(t, recordKB(t, edgeOpsGraph()), tc.entry)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 {
				t.Fatalf("result = %+v, want one refusal", res)
			}
			if got := res.Refusals[0].Remedy; got != tc.want {
				t.Errorf("remedy = %q, want %q", got, tc.want)
			}
		})
	}
}
