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
	"kbase/internal/log"
	"kbase/internal/query"
	"kbase/internal/result"
)

// premiseOf is edgeID's premise edge naming source in the register text, as
// the production reader returns it.
func premiseOf(t *testing.T, text, entryID, source string) (kb.Edge, bool) {
	t.Helper()
	read, err := readRegister(text)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := read.claims[entryID]
	if !ok {
		t.Fatalf("%s has no entry", entryID)
	}
	for _, e := range entry.Premises {
		if e.Source == source {
			return e, true
		}
	}
	return kb.Edge{}, false
}

// TestSetBearingOnEveryRelation lands a bearing on a claim premise, a founds
// edge, a lift and a refutation, on an experiment's support and on a claim's
// own refutation list, and reads each back as the same state.
func TestSetBearingOnEveryRelation(t *testing.T) {
	root := seedKB(t)
	unrelated := insertClaim(t, root, "Unrelated Result", "")
	claim := insertClaim(t, root, "Bound Result", "  rests-on:\n  - id: "+seedClaim+"\n    bearing: 0.5\n  refutes:\n  - id: "+unrelated+"\n")
	work := func() string {
		res := run(t, root, "insert-work-entry", "entry:\n- register: claim-rigor.md\n  key: Smith2020\n  title: Smith (2020)\n  rigor: 0.8\n  rationale: Cited.\n", false)
		if res.Outcome != result.Done {
			t.Fatalf("insert work: %+v", res)
		}
		return res.IDs[0]
	}()
	if res := run(t, root, "add-rests-on", "entry:\n- id: "+claim+"\n  rests-on:\n  - id: "+work+"\n    bearing: 0.8\n", false); res.Outcome != result.Done {
		t.Fatalf("rest on the work: %+v", res)
	}
	analysis := func() string {
		res := run(t, root, "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n  lifts:\n  - id: "+claim+"\n    bearing: 0.5\n  refutes:\n  - id: "+seedClaim+"\n    bearing: 0.4\n", false)
		if res.Outcome != result.Done {
			t.Fatalf("insert analysis: %+v", res)
		}
		return res.IDs[0]
	}()
	experiment := func() string {
		res := run(t, root, "insert-experiment-entry", "entry:\n- document: leaf.md\n  status: run\n  rigor: 0.9\n  supports:\n  - id: "+claim+"\n    bearing: 0.6\n", false)
		if res.Outcome != result.Done {
			t.Fatalf("insert experiment: %+v", res)
		}
		return res.IDs[0]
	}()

	steps := []struct {
		name, values string
		check        func(t *testing.T, files map[string]string)
	}{
		{"claim premise", "entry:\n- id: " + seedClaim + "\n  target: " + claim + "\n  relation: supports\n  bearing: 0.25\n",
			func(t *testing.T, files map[string]string) {
				if e, ok := premiseOf(t, files[kb.RegisterFile], claim, seedClaim); !ok || e.Bearing != (kb.Bearing{Value: 0.25}) {
					t.Errorf("premise = %+v (set %v), want bearing 0.25", e, ok)
				}
			}},
		{"founds", "entry:\n- id: " + work + "\n  target: " + claim + "\n  relation: founds\n  bearing: 0.3\n",
			func(t *testing.T, files map[string]string) {
				if e, ok := premiseOf(t, files[kb.RegisterFile], claim, work); !ok || e.Bearing != (kb.Bearing{Value: 0.3}) {
					t.Errorf("founds premise = %+v (set %v), want bearing 0.3", e, ok)
				}
			}},
		{"lift", "entry:\n- id: " + analysis + "\n  target: " + claim + "\n  relation: lifts\n  bearing: \"*pending*\"\n",
			func(t *testing.T, files map[string]string) {
				read := readRegisterOrFail(t, files[kb.RegisterFile])
				if got := read.analyses[analysis].Lifts; len(got) != 1 || !got[0].Bearing.Pending {
					t.Errorf("lift = %+v, want one pending bearing", got)
				}
			}},
		{"refute", "entry:\n- id: " + analysis + "\n  target: " + seedClaim + "\n  relation: refutes\n  bearing: 0\n",
			func(t *testing.T, files map[string]string) {
				read := readRegisterOrFail(t, files[kb.RegisterFile])
				if got := read.analyses[analysis].Refutes; len(got) != 1 || got[0].Bearing != (kb.Bearing{Value: 0}) {
					t.Errorf("refutation = %+v, want bearing 0", got)
				}
			}},
		{"claim refutation", "entry:\n- id: " + claim + "\n  target: " + unrelated + "\n  relation: refutes\n  bearing: 0.6\n",
			func(t *testing.T, files map[string]string) {
				read := readRegisterOrFail(t, files[kb.RegisterFile])
				if got := read.claims[claim].Refutes; len(got) != 1 || got[0].Bearing != (kb.Bearing{Value: 0.6}) {
					t.Errorf("claim refutation = %+v, want bearing 0.6", got)
				}
			}},
		{"experiment support", "entry:\n- id: " + experiment + "\n  target: " + claim + "\n  relation: supports\n  bearing: 0.75\n",
			func(t *testing.T, files map[string]string) {
				decl, err := observedDeclaration(files["leaf.md"], experiment, "leaf.md")
				if err != nil || decl == nil {
					t.Fatalf("declaration = %v, %v", decl, err)
				}
				if len(decl.Supports) != 1 || !scoreEqual(decl.Supports[0].Score, f64(0.75)) {
					t.Errorf("experiment supports = %+v, want 0.75", decl.Supports)
				}
			}},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			res := run(t, root, "set-bearing", step.values, false)
			if res.Outcome != result.Done {
				t.Fatalf("set-bearing: %+v", res)
			}
			step.check(t, snapshot(t, root))
			before := snapshot(t, root)
			if again := run(t, root, "set-bearing", step.values, false); again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("again: %+v, want unchanged byte-identical", again)
			}
		})
	}
}

// TestSetBearingOnADoublyEnteredEdge judges a lift whose source records it in
// both authored ends — its leaf declaration and the register's staged copy —
// and requires both to move: one link left behind is a fan-out disagreement,
// and verify says so right after the op.
func TestSetBearingOnADoublyEnteredEdge(t *testing.T) {
	root := seedKB(t)
	res := run(t, root, "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n  lifts:\n  - id: "+seedClaim+"\n    bearing: 0.5\n", false)
	if res.Outcome != result.Done || len(res.IDs) != 1 {
		t.Fatalf("insert analysis: %+v", res)
	}
	analysis := res.IDs[0]
	declare := "entry:\n- document: leaf.md\n  kind: leaf\n  claims: [" + seedClaim + "]\n  analysis-node:\n  - ana-id: " + analysis +
		"\n    lifts:\n    - id: " + seedClaim + "\n      bearing: 0.5\n"
	if r := run(t, root, "set-frontmatter", declare, false); r.Outcome != result.Done {
		t.Fatalf("declare the same lift in the hosting leaf: %+v", r)
	}
	verifyDone := func(when string) {
		t.Helper()
		findings, err := index.Verify(kb.OnDisk(root))
		if err != nil || len(findings) > 0 {
			t.Fatalf("verify %s: %v %v", when, findings, err)
		}
	}
	verifyDone("with both ends recording 0.5")

	if r := run(t, root, "set-bearing", "entry:\n- id: "+analysis+"\n  target: "+seedClaim+"\n  relation: lifts\n  bearing: 0.25\n", false); r.Outcome != result.Done {
		t.Fatalf("set-bearing: %+v", r)
	}
	files := snapshot(t, root)
	read := readRegisterOrFail(t, files[kb.RegisterFile])
	if got := read.analyses[analysis].Lifts; len(got) != 1 || got[0].Bearing != (kb.Bearing{Value: 0.25}) {
		t.Errorf("staged copy = %+v, want bearing 0.25", got)
	}
	decl, err := observedDeclaration(files["leaf.md"], analysis, "leaf.md")
	if err != nil || decl == nil || len(decl.Lifts) != 1 || !scoreEqual(decl.Lifts[0].Score, f64(0.25)) {
		t.Errorf("leaf declaration = %+v, %v; want bearing 0.25", decl, err)
	}
	verifyDone("after judging both ends")

	before := snapshot(t, root)
	again := run(t, root, "set-bearing", "entry:\n- id: "+analysis+"\n  target: "+seedClaim+"\n  relation: lifts\n  bearing: 0.25\n", false)
	if again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("again: %+v, want unchanged, byte-identical", again)
	}
}

// TestARegisterStagedPairReachesDeps: a lift an analysis entry stages in the
// register, and whose hosting leaf declares no pair for, is an edge the wire
// carries — deps answers it as it answers a declared one.
func TestARegisterStagedPairReachesDeps(t *testing.T) {
	root := seedKB(t)
	res := run(t, root, "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n  lifts:\n  - id: "+seedClaim+"\n    bearing: 0.5\n", false)
	if res.Outcome != result.Done || len(res.IDs) != 1 {
		t.Fatalf("insert analysis: %+v", res)
	}
	analysis := res.IDs[0]
	declare := "entry:\n- document: leaf.md\n  kind: leaf\n  claims: [" + seedClaim + "]\n  analysis-node:\n  - ana-id: " + analysis + "\n"
	if r := run(t, root, "set-frontmatter", declare, false); r.Outcome != result.Done {
		t.Fatalf("declare the analysis in its hosting leaf: %+v", r)
	}
	ix, err := query.Load(kb.OnDisk(root))
	if err != nil {
		t.Fatalf("query.Load: %v", err)
	}
	if got, want := ix.DependentsOf(analysis), []string{seedClaim}; !slices.Equal(got, want) {
		t.Errorf("DependentsOf(%s) = %v, want the stage-only lift's target %v", analysis, got, want)
	}
}

// TestAPairNamesAnAnalysis: a pair list takes a claim or an analysis as its
// target, as the reader and BearingLegal do, so an analysis's lift of another
// analysis and a claim's refutation of one land as edges of that target kind; a
// work id is no pair target.
func TestAPairNamesAnAnalysis(t *testing.T) {
	root := seedKB(t)
	insertAnalysis := func(title, lifts string) string {
		t.Helper()
		res := run(t, root, "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: "+title+
			"\n  rigor: 0.9\n  rationale: Lifts.\n"+lifts, false)
		if res.Outcome != result.Done || len(res.IDs) != 1 {
			t.Fatalf("insert %s: %+v", title, res)
		}
		return res.IDs[0]
	}
	first := insertAnalysis("First Analysis", "  lifts:\n  - id: "+seedClaim+"\n    bearing: 0.5\n")
	second := insertAnalysis("Second Analysis", "  lifts:\n  - id: "+first+"\n    bearing: 0.25\n")
	refuter := insertClaim(t, root, "A Refuting Result", "  refutes:\n  - id: "+first+"\n    bearing: 0.4\n")
	declare := "entry:\n- document: leaf.md\n  kind: leaf\n  claims: [" + seedClaim + "]\n  analysis-node:\n" +
		"  - ana-id: " + first + "\n  - ana-id: " + second + "\n"
	if res := run(t, root, "set-frontmatter", declare, false); res.Outcome != result.Done {
		t.Fatalf("declare the analyses in their hosting leaf: %+v", res)
	}

	st, err := kb.Discover(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	lifts := func(id string) []kb.Edge {
		for _, a := range st.Analyses {
			if a.ID == id {
				return a.Lifts
			}
		}
		return nil
	}
	refutes := func(id string) []kb.Edge {
		for _, c := range st.ClaimEntries {
			if c.ID == id {
				return c.Refutes
			}
		}
		return nil
	}
	for _, tc := range []struct {
		name  string
		edges []kb.Edge
		want  kb.Edge
	}{
		{"a lift of an analysis", lifts(second),
			kb.Edge{Source: second, Target: first, Relation: kb.RelationLifts, TargetKind: kb.NodeKindAnalysis, Bearing: kb.Bearing{Value: 0.25}}},
		{"a refutation of an analysis", refutes(refuter),
			kb.Edge{Source: refuter, Target: first, Relation: kb.RelationRefutes, TargetKind: kb.NodeKindAnalysis, Bearing: kb.Bearing{Value: 0.4}}},
	} {
		if len(tc.edges) != 1 || tc.edges[0] != tc.want {
			t.Errorf("%s = %+v, want %+v", tc.name, tc.edges, tc.want)
		}
	}

	before := snapshot(t, root)
	res := run(t, root, "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: A Third Analysis\n  rigor: 0.9\n"+
		"  rationale: Lifts a work.\n  lifts:\n  - id: wrk-smith2020\n    bearing: 0.5\n", false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || !strings.Contains(res.Refusals[0].Detail, "is not an id matching clm-") {
		t.Errorf("a work id as a pair target: %+v, want one refusal naming the pair id grammar", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused insert-analysis-entry wrote")
	}
}

func readRegisterOrFail(t *testing.T, text string) registerRead {
	t.Helper()
	read, err := readRegister(text)
	if err != nil {
		t.Fatal(err)
	}
	return read
}

// TestSetBearingRefusals: an illegal pairing, an edge that exists in no home,
// and a value outside the domain are each refused with nothing written.
func TestSetBearingRefusals(t *testing.T) {
	root := seedKB(t)
	claim := insertClaim(t, root, "Bound Result", "  rests-on:\n  - id: "+seedClaim+"\n    bearing: 0.5\n")
	analysis := func() string {
		res := run(t, root, "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n", false)
		if res.Outcome != result.Done {
			t.Fatalf("insert analysis: %+v", res)
		}
		return res.IDs[0]
	}()
	before := snapshot(t, root)
	for _, tc := range []struct {
		name, values string
		refused      func(result.Item) bool
	}{
		{"invariant token", "entry:\n- id: INVARIANT-S1\n  target: " + claim + "\n  relation: supports\n  bearing: 0.5\n",
			func(it result.Item) bool {
				return strings.Contains(it.Detail, "not an id") || strings.Contains(it.Detail, "does not resolve")
			}},
		{"illegal pairing", "entry:\n- id: " + analysis + "\n  target: " + claim + "\n  relation: founds\n  bearing: 0.5\n",
			func(it result.Item) bool { return strings.Contains(it.Detail, "no bearing judges that pairing") }},
		{"unknown relation", "entry:\n- id: " + analysis + "\n  target: " + claim + "\n  relation: depends\n  bearing: 0.5\n",
			func(it result.Item) bool {
				return strings.Contains(it.Detail, "outside the closed relation vocabulary")
			}},
		{"out of domain", "entry:\n- id: " + seedClaim + "\n  target: " + claim + "\n  relation: supports\n  bearing: 1.5\n",
			func(it result.Item) bool { return strings.Contains(it.Detail, "outside [0, 1]") }},
		{"no such edge", "entry:\n- id: " + analysis + "\n  target: " + claim + "\n  relation: lifts\n  bearing: 0.5\n",
			func(it result.Item) bool {
				return strings.Contains(it.Detail, "stages no lifts pair") || strings.Contains(it.Detail, "authors no")
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, root, "set-bearing", tc.values, false)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || !tc.refused(res.Refusals[0]) {
				t.Errorf("got %+v, want one refusal matching", res)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused set-bearing wrote")
			}
		})
	}
}

// TestWriteRefusesAMalformedBullet: a bullet the reader refuses stops every op
// before its plan, at the bullet's file and line, and nothing is written.
func TestWriteRefusesAMalformedBullet(t *testing.T) {
	root := seedKB(t)
	path := filepath.Join(root, kb.RegisterFile)
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tokenless := string(text) + "\n---\n\n## Tokenless\n<!-- id: clm-bbbbbb -->\n\n### Rigor\n- rigor: 0.5\n- rests-on:\n  - clm-aaaaaa — Seed Result\n- rationale: Shown.\n"
	if err := os.WriteFile(path, []byte(tokenless), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	res := run(t, root, "set-rigor", "entry:\n- id: "+seedClaim+"\n  rigor: 0.2\n", false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("got %+v, want one refusal", res)
	}
	it := res.Refusals[0]
	if want := lineOf(t, tokenless, "- clm-aaaaaa — Seed Result"); it.Check != "metadata" || it.Path != kb.RegisterFile || it.Line != want || it.Key != kb.ListRestsOn {
		t.Errorf("refusal = %+v, want check metadata at %s:%d", it, kb.RegisterFile, want)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused write changed kb-root")
	}
}

// TestSetRigorOnEveryHome writes a claim's, an analysis's, a work's and an
// experiment's rigor, each in the home its kind authors it in.
func TestSetRigorOnEveryHome(t *testing.T) {
	root := seedKB(t)
	analysis := func() string {
		res := run(t, root, "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n", false)
		if res.Outcome != result.Done {
			t.Fatalf("insert analysis: %+v", res)
		}
		return res.IDs[0]
	}()
	work := func() string {
		res := run(t, root, "insert-work-entry", "entry:\n- register: claim-rigor.md\n  key: Smith2020\n  title: Smith (2020)\n  rigor: 0.8\n  rationale: Cited.\n", false)
		if res.Outcome != result.Done {
			t.Fatalf("insert work: %+v", res)
		}
		return res.IDs[0]
	}()
	experiment := func() string {
		res := run(t, root, "insert-experiment-entry", "entry:\n- document: leaf.md\n  status: run\n  rigor: 0.9\n", false)
		if res.Outcome != result.Done {
			t.Fatalf("insert experiment: %+v", res)
		}
		return res.IDs[0]
	}()
	for _, tc := range []struct {
		id   string
		want *float64
	}{
		{seedClaim, f64(0.31)}, {analysis, f64(0.32)}, {work, f64(0.33)}, {experiment, f64(0.34)},
	} {
		values := "entry:\n- id: " + tc.id + "\n  rigor: " + formatScore(tc.want) + "\n"
		if res := run(t, root, "set-rigor", values, false); res.Outcome != result.Done {
			t.Fatalf("set-rigor %s: %+v", tc.id, res)
		}
		files := snapshot(t, root)
		if strings.HasPrefix(tc.id, "exp-") {
			decl, err := observedDeclaration(files["leaf.md"], tc.id, "leaf.md")
			if err != nil || decl == nil || !scoreEqual(decl.Rigor, tc.want) {
				t.Errorf("%s rigor = %v, %v; want %v", tc.id, decl, err, tc.want)
			}
			continue
		}
		read := readRegisterOrFail(t, files[kb.RegisterFile])
		var got *float64
		switch {
		case strings.HasPrefix(tc.id, "ana-"):
			a := read.analyses[tc.id]
			got = a.Rigor
		case strings.HasPrefix(tc.id, kb.WorkPrefix+"-"):
			w := read.works[tc.id]
			got = w.Rigor
		default:
			c := read.claims[tc.id]
			got = c.Rigor
		}
		if !scoreEqual(got, tc.want) {
			t.Errorf("%s rigor = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestARigorWriteTakesTheTablesMaximum: 1.00 belongs to an axiom by rule, so
// every op that takes a node's own rigor refuses a value above the rigor
// tables' maximum, naming the key and the range, and the maximum itself is
// written and read back.
func TestARigorWriteTakesTheTablesMaximum(t *testing.T) {
	root := seedKB(t)
	before := snapshot(t, root)
	rangeText := "[0, " + kb.PyFloatRepr(kb.RigorMaximum) + "]"
	for _, tc := range []struct{ name, op, values string }{
		{"set-rigor", "set-rigor", "entry:\n- id: " + seedClaim + "\n  rigor: 1.0\n"},
		{"insert-claim-entry", "insert-claim-entry", "entry:\n- register: claim-rigor.md\n  title: A New Result\n  rigor: 1.0\n  rationale: Shown.\n"},
		{"insert-analysis-entry", "insert-analysis-entry", "entry:\n- register: claim-rigor.md\n  title: An Analysis\n  rigor: 1.0\n  rationale: Lifts.\n"},
		{"insert-work-entry", "insert-work-entry", "entry:\n- register: claim-rigor.md\n  key: Smith2020\n  title: Smith (2020)\n  rigor: 1.0\n  rationale: Cited.\n"},
		{"insert-experiment-entry", "insert-experiment-entry", "entry:\n- document: leaf.md\n  status: run\n  rigor: 1.0\n"},
		{"set-frontmatter's experiment node", "set-frontmatter", "entry:\n- document: leaf.md\n  kind: leaf\n  experiment-node:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 1.0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, root, tc.op, tc.values, false)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 {
				t.Fatalf("%s: %+v, want one refusal", tc.op, res)
			}
			if it := res.Refusals[0]; !strings.HasSuffix(it.Key, "rigor") ||
				!strings.Contains(it.Detail, "outside "+rangeText) || !strings.Contains(it.Detail, "axiom by rule") {
				t.Errorf("%s refusal = %+v, want key rigor and the range %s", tc.op, it, rangeText)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("a refused %s wrote", tc.op)
			}
		})
	}

	atMaximum := f64(kb.RigorMaximum)
	if res := run(t, root, "set-rigor", "entry:\n- id: "+seedClaim+"\n  rigor: "+formatScore(atMaximum)+"\n", false); res.Outcome != result.Done {
		t.Fatalf("set-rigor at the maximum: %+v", res)
	}
	if got := readRegisterOrFail(t, snapshot(t, root)[kb.RegisterFile]).claims[seedClaim].Rigor; !scoreEqual(got, atMaximum) {
		t.Errorf("rigor = %v, want the maximum %v", got, *atMaximum)
	}
}

// TestAnAuthoredOneIsRead: a KB carrying an authored `rigor: 1.0` — a converted
// experiment — is read, never refused, this rule being the write path's alone:
// refresh restamps it and verify reports nothing.
func TestAnAuthoredOneIsRead(t *testing.T) {
	root := seedKB(t)
	leaf := filepath.Join(root, "leaf.md")
	text, err := os.ReadFile(leaf)
	if err != nil {
		t.Fatal(err)
	}
	declared := "claims: [" + seedClaim + "]\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 1.0"
	converted := strings.Replace(string(text), "claims: ["+seedClaim+"]", declared, 1)
	if converted == string(text) {
		t.Fatalf("the seed leaf declares no claims list:\n%s", text)
	}
	if err := os.WriteFile(leaf, []byte(converted), 0o644); err != nil {
		t.Fatal(err)
	}
	src := kb.OnDisk(root)
	if _, err := index.Refresh(src, log.Discard()); err != nil {
		t.Fatalf("refresh over an authored 1.0: %v", err)
	}
	if findings, err := index.Verify(src); err != nil || len(findings) > 0 {
		t.Errorf("verify over an authored 1.0: %v, %v; want no finding", findings, err)
	}
	state, err := kb.Discover(src, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range state.Experiments {
		if e.ID == "exp-aaaaaa" {
			if e.Rigor == nil || !scoreEqual(e.Rigor, f64(kb.AxiomRigor)) {
				t.Errorf("the converted experiment reads rigor %v, want 1", e.Rigor)
			}
			return
		}
	}
	t.Errorf("the converted experiment is in no discovered leaf: %+v", state.Experiments)
}

// TestANewPremiseIsPendingAndMovesNoClaim: an inserted premise lands with a
// pending bearing, so it contributes nothing and the claim reads 0 until the
// bearing is judged; judging the edge afterwards is what moves it.
func TestANewPremiseIsPendingAndMovesNoClaim(t *testing.T) {
	root := seedKB(t)
	// The seed rests on an axiom, so it holds a value of its own for the
	// inserted claim's edge to multiply once that edge is judged.
	if err := os.WriteFile(filepath.Join(root, kb.AxiomsFile), []byte("# Axioms\n\n### Axiom S1: The rule\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := run(t, root, "add-rests-on", "entry:\n- id: "+seedClaim+"\n  rests-on:\n  - id: Axiom S1\n    bearing: 1.0\n", false); res.Outcome != result.Done {
		t.Fatalf("add-rests-on: %+v", res)
	}
	claim := insertClaim(t, root, "Bound Result", "  rests-on:\n  - id: "+seedClaim+"\n")
	strengthOf := func(id string) *float64 {
		t.Helper()
		_, rows, _, err := kb.ReadIndex[kb.NodeRow](kb.OnDisk(root), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ID == id {
				return r.Strength
			}
		}
		t.Fatalf("no index row for %s", id)
		return nil
	}
	if got := strengthOf(claim); got == nil || *got != 0 {
		t.Fatalf("a pending premise moved the claim: strength %v, want 0", got)
	}
	if res := run(t, root, "set-bearing", "entry:\n- id: "+seedClaim+"\n  target: "+claim+"\n  relation: supports\n  bearing: 0.2\n", false); res.Outcome != result.Done {
		t.Fatalf("set-bearing: %+v", res)
	}
	// 0.2 × the seed's 0.90 is 0.18, under the claim's 0.50.
	bearing, seedRigor := 0.2, 0.9
	got := strengthOf(claim)
	if got == nil {
		t.Fatal("a judged premise left the claim with no strength")
	}
	if want := bearing * seedRigor; *got != want {
		t.Errorf("a judged premise left the claim at %v, want %v", *got, want)
	}
}
