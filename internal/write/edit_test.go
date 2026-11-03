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

// editGraph is one node of each kind, the claim, analysis and experiment
// judged by a run (not manually scored) with a recorded hash.
func editGraph() kb.Graph {
	hash := kb.JudgedHash{1}
	return kb.Graph{
		Claims:      []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Main Result", Rigor: f64(0.5), Rationale: "Shown.", JudgedHash: hash}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "The Proof", Rigor: f64(0.5), Rationale: "Argued.", JudgedHash: hash}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "A Trial", Status: kb.ExperimentPlanned, Rigor: f64(0.5), JudgedHash: hash}},
		Works:       []kb.WorkNode{{ID: "wrk-s00000", Label: "Smith2020", Title: "Old Work", Rigor: f64(0.5), Rationale: "Cited."}},
		Axioms:      []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "Closure"}},
	}
}

// editLeaf hosts the claim, the analysis and the experiment, and cites the
// axiom, the work and the claim: naming forms, quotes, and one citation inside
// a fenced block.
const editLeaf = "[↑ Entry](entry-point.md)\n\n# Leaf\n\n" +
	"The main result holds.<!-- kbase-id: clm-aaaaaa -->\n\n" +
	"## Proof\n\nIt follows.<!-- kbase-id: ana-aaaaaa -->\n\n" +
	"## Trial\n\nWe ran it.<!-- kbase-id: exp-aaaaaa -->\n\n" +
	"By [Closure](axioms.md#axiom-s1-closure \"axm-s10000\"), as [Old Work](kb-summary.md#old-work \"wrk-s00000\") " +
	"and [\"the old work\"](kb-summary.md#old-work \"wrk-s00000\") show; see [Main Result](leaf.md#leaf \"clm-aaaaaa\") " +
	"and [\"The main result holds.\"](leaf.md#leaf \"clm-aaaaaa\").\n\n" +
	"```\n[Closure](axioms.md#axiom-s1-closure \"axm-s10000\")\n```\n"

// editKB is a KB holding g's records, editLeaf, and axioms.md holding the
// axiom's heading.
func editKB(t *testing.T, g kb.Graph) string {
	t.Helper()
	root := recordKB(t, g)
	for rel, text := range map[string]string{kb.EntryPointFile: "# Entry\n\n- [Leaf](leaf.md)\n", "leaf.md": editLeaf,
		kb.AxiomsFile: "# Axioms\n\n" + kb.AxiomHeadingLine("S1", "Closure", "axm-s10000") + "\n\nIt closes.\n"} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestEditsWriteTheirFields: each edit replaces the fields it is given and
// leaves the rest; a rigor set on a claim, an analysis or an experiment marks
// it manually scored and records the judged hash of its statement, and an edit
// setting no rigor keeps the hash; the entry reports the id.
func TestEditsWriteTheirFields(t *testing.T) {
	cases := []struct {
		name, op, values string
		check            func(t *testing.T, root string, g kb.Graph)
	}{
		{"claim", "edit-claim", "- id: clm-aaaaaa\n  rigor: 0.75\n  rationale: Shown   twice.\n", func(t *testing.T, root string, g kb.Graph) {
			want := kb.ClaimNode{ID: "clm-aaaaaa", Title: "Main Result", Rigor: f64(0.75), ManuallyScored: true, Rationale: "Shown twice.",
				JudgedHash: nodeJudgedNow(t, root, "clm-aaaaaa")}
			if n := g.Claims[0]; n.Title != want.Title || *n.Rigor != *want.Rigor || n.ManuallyScored != want.ManuallyScored ||
				n.Rationale != want.Rationale || n.JudgedHash != want.JudgedHash {
				t.Errorf("claim = %+v, want %+v", n, want)
			}
		}},
		{"analysis", "edit-analysis", "- id: ana-aaaaaa\n  title: The New Proof\n  rigor: '*pending*'\n", func(t *testing.T, root string, g kb.Graph) {
			if n := g.Analyses[0]; n.Title != "The New Proof" || n.Rigor != nil || !n.ManuallyScored || n.Rationale != "Argued." ||
				n.JudgedHash != nodeJudgedNow(t, root, "ana-aaaaaa") {
				t.Errorf("analysis = %+v, want the new title, rigor pending, manually scored and judged over its statement, the rest standing", n)
			}
		}},
		{"experiment", "edit-experiment", "- id: exp-aaaaaa\n  status: run\n", func(t *testing.T, root string, g kb.Graph) {
			if n := g.Experiments[0]; n.Status != kb.ExperimentRun || *n.Rigor != 0.5 || n.ManuallyScored || n.Title != "A Trial" || n.JudgedHash != (kb.JudgedHash{1}) {
				t.Errorf("experiment = %+v, want status run, its rigor, mark and hash standing", n)
			}
		}},
		{"work", "edit-work", "- id: wrk-s00000\n  rigor: 0.9\n  rationale: ''\n", func(t *testing.T, root string, g kb.Graph) {
			if n := g.Works[0]; *n.Rigor != 0.9 || n.Rationale != "" || n.Title != "Old Work" {
				t.Errorf("work = %+v, want rigor 0.9 and no rationale", n)
			}
		}},
		{"axiom", "edit-axiom", "- id: axm-s10000\n  title: Closure Law\n", func(t *testing.T, root string, g kb.Graph) {
			if n := g.Axioms[0]; n.Title != "Closure Law" {
				t.Errorf("axiom = %+v, want the new title", n)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := editKB(t, editGraph())
			e := onlyEntry(t, run(t, root, c.op, "entry:\n"+c.values))
			if want := strings.TrimPrefix(strings.SplitN(c.values, "\n", 2)[0], "- id: "); e.ID != want || e.Outcome != result.Done {
				t.Errorf("entry = %+v, want done, reporting id %s", e, want)
			}
			c.check(t, root, records(t, root))
		})
	}
}

// TestAManualEditRecordsTheCurrentJudgedHash: a rigor an edit-claim sets
// records the hash of the claim's statement as it stands when the value is
// set — a statement edited since the last judgement moves it — so the value
// is not dirty to the next score run; an edit that sets no rigor leaves the
// recorded hash standing.
func TestAManualEditRecordsTheCurrentJudgedHash(t *testing.T) {
	root := editKB(t, editGraph())
	leaf := filepath.Join(root, "leaf.md")
	edited := strings.Replace(editLeaf, "The main result holds.", "The main result holds everywhere.", 1)
	if err := os.WriteFile(leaf, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if statementNow(t, root, "clm-aaaaaa") != "The main result holds everywhere." {
		t.Fatalf("the claim states %q, want the edited statement", statementNow(t, root, "clm-aaaaaa"))
	}
	before := nodeJudgedNow(t, root, "clm-aaaaaa")
	if before == (kb.JudgedHash{}) || before == (kb.JudgedHash{1}) {
		t.Fatalf("the statement's hash is %v, which a record already carries", before)
	}
	onlyEntry(t, run(t, root, "edit-claim", "entry:\n- id: clm-aaaaaa\n  rigor: 0.75\n"))
	if got := records(t, root).Claims[0]; got.JudgedHash != before || !got.ManuallyScored {
		t.Errorf("the claim records hash %v, manually scored %v; want the edited statement's %v, marked", got.JudgedHash, got.ManuallyScored, before)
	}
	onlyEntry(t, run(t, root, "edit-claim", "entry:\n- id: clm-aaaaaa\n  rationale: Shown again.\n"))
	if got := records(t, root).Claims[0].JudgedHash; got != before {
		t.Errorf("an edit setting no rigor moved the hash to %v, want %v standing", got, before)
	}
}

// TestAnEditTakesItsKindsFields: each edit takes the fields its kind carries,
// and refuses at shape any other, naming the kind's fields as allowed; an
// entry giving no field is refused at shape too.
func TestAnEditTakesItsKindsFields(t *testing.T) {
	values := map[string]string{"title": "T", "rigor": "0.5", "rationale": "R", "status": "run", "label": "X"}
	ops := []struct {
		op, id string
		fields []string
	}{
		{"edit-claim", "clm-aaaaaa", []string{"title", "rigor", "rationale"}},
		{"edit-analysis", "ana-aaaaaa", []string{"title", "rigor", "rationale"}},
		{"edit-experiment", "exp-aaaaaa", []string{"title", "status", "rigor"}},
		{"edit-work", "wrk-s00000", []string{"label", "title", "rigor", "rationale"}},
		{"edit-axiom", "axm-s10000", []string{"label", "title"}},
	}
	for _, o := range ops {
		for _, key := range slices.Sorted(maps.Keys(values)) {
			_, refusals := parseValues([]byte("entry:\n- id: "+o.id+"\n  "+key+": "+values[key]+"\n"), o.op)
			carried := slices.Contains(o.fields, key)
			switch {
			case carried && len(refusals) != 0:
				t.Errorf("%s %s: refused %+v, want accepted", o.op, key, refusals)
			case !carried && (len(refusals) != 1 || refusals[0].Key != key || !slices.Equal(refusals[0].Allowed, slices.Sorted(slices.Values(o.fields)))):
				t.Errorf("%s %s: refusals %+v, want one at %s allowing exactly %v", o.op, key, refusals, key, o.fields)
			}
		}
		_, refusals := parseValues([]byte("entry:\n- operation: "+o.op+"\n  id: "+o.id+"\n  document: x.md\n"), Batch)
		if len(refusals) != 1 || refusals[0].Key != "document" || !slices.Equal(refusals[0].Allowed, slices.Sorted(slices.Values(o.fields))) {
			t.Errorf("%s in a mixed batch with document: refusals %+v, want one at document allowing exactly %v", o.op, refusals, o.fields)
		}
		_, refusals = parseValues([]byte("entry:\n- id: "+o.id+"\n"), o.op)
		if len(refusals) != 1 || refusals[0].Entry != 1 || !slices.Equal(refusals[0].Allowed, slices.Sorted(slices.Values(o.fields))) {
			t.Errorf("%s with no field: refusals %+v, want one allowing %v", o.op, refusals, o.fields)
		}
	}
	for _, c := range []struct{ name, values, key string }{
		{"rigor above 1", "- id: clm-aaaaaa\n  rigor: 1.5\n", "rigor"},
		{"title with a line break", "- id: clm-aaaaaa\n  title: \"Two\\nlines\"\n", "title"},
		{"empty title", "- id: clm-aaaaaa\n  title: '  '\n", "title"},
	} {
		if _, refusals := parseValues([]byte("entry:\n"+c.values), "edit-claim"); len(refusals) != 1 || refusals[0].Key != c.key {
			t.Errorf("%s: refusals %+v, want one at %s", c.name, refusals, c.key)
		}
	}
	if _, refusals := parseValues([]byte("entry:\n- id: exp-aaaaaa\n  status: pending\n"), "edit-experiment"); len(refusals) != 1 || refusals[0].Key != "status" {
		t.Errorf("status outside run and planned: refusals %+v, want one at status", refusals)
	}
}

// TestAnEditRefusesAnIDItDoesNotEdit: an id of another kind is refused, its
// remedy naming the edit op for that kind; an id no node carries is refused,
// no node <id>. Neither writes.
func TestAnEditRefusesAnIDItDoesNotEdit(t *testing.T) {
	cases := []struct{ op, id, detail, remedy string }{
		{"edit-claim", "ana-aaaaaa", "analysis", "edit-analysis"},
		{"edit-analysis", "clm-aaaaaa", "claim", "edit-claim"},
		{"edit-claim", "exp-aaaaaa", "experiment", "edit-experiment"},
		{"edit-experiment", "wrk-s00000", "work", "edit-work"},
		{"edit-work", "axm-s10000", "axiom", "edit-axiom"},
		{"edit-claim", "clm-zzzzzz", "no node clm-zzzzzz", kb.NodeLister},
	}
	for _, c := range cases {
		t.Run(c.op+" "+c.id, func(t *testing.T) {
			root := editKB(t, editGraph())
			before := snapshot(t, root)
			res := run(t, root, c.op, "entry:\n- id: "+c.id+"\n  title: Another\n")
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "id" ||
				!strings.Contains(res.Refusals[0].Detail, c.detail) || !strings.Contains(res.Refusals[0].Remedy, c.remedy) {
				t.Errorf("result = %+v, want one refusal at id, detail naming %q, remedy naming %q", res, c.detail, c.remedy)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused edit wrote")
			}
		})
	}
}

// TestAnEditRefusesATitleItsKindHolds: a title another node of the kind
// carries is refused naming that node, writing nothing; a title a node of
// another kind carries is taken.
func TestAnEditRefusesATitleItsKindHolds(t *testing.T) {
	g := editGraph()
	g.Claims = append(g.Claims, kb.ClaimNode{ID: "clm-bbbbbb", Title: "Second Result", Rationale: ""})
	g.Works = append(g.Works, kb.WorkNode{ID: "wrk-j00000", Label: "Jones2021", Title: "Other Work"})
	g.Axioms = append(g.Axioms, kb.AxiomNode{ID: "axm-s20000", Label: "S2", Title: "Openness"})
	for _, c := range []struct{ op, id, title, holder string }{
		{"edit-claim", "clm-bbbbbb", "Main   Result", "clm-aaaaaa"},
		{"edit-work", "wrk-j00000", "Old Work", "wrk-s00000"},
		{"edit-axiom", "axm-s20000", "Closure", "axm-s10000"},
	} {
		t.Run(c.op, func(t *testing.T) {
			root := editKB(t, g)
			before := snapshot(t, root)
			res := run(t, root, c.op, "entry:\n- id: "+c.id+"\n  title: "+c.title+"\n")
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "title" ||
				!strings.Contains(res.Refusals[0].Detail, c.holder) {
				t.Errorf("result = %+v, want one refusal at title naming %s", res, c.holder)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused edit wrote")
			}
		})
	}
	t.Run("another kind", func(t *testing.T) {
		root := editKB(t, g)
		onlyEntry(t, run(t, root, "edit-claim", "entry:\n- id: clm-bbbbbb\n  title: The Proof\n"))
	})
}

// TestATitleEditKeepsCitationsLive: a title edit rewrites an axiom's heading,
// keeping its marker, re-points the axiom's and the work's citations to where their link target
// moved, and gives each naming-form citation the new title, leaving a quote's
// text and a citation inside code as they stand.
func TestATitleEditKeepsCitationsLive(t *testing.T) {
	root := editKB(t, editGraph())
	res := run(t, root, Batch, "entry:\n"+
		"- operation: edit-axiom\n  id: axm-s10000\n  title: Closure   Law\n"+
		"- operation: edit-work\n  id: wrk-s00000\n  title: New Work\n"+
		"- operation: edit-claim\n  id: clm-aaaaaa\n  title: Main Theorem\n")
	if res.Outcome != result.Done || len(res.Entries) != 3 {
		t.Fatalf("result = %+v, want done with three entries", res)
	}
	want := strings.Replace(editLeaf, "By [Closure](axioms.md#axiom-s1-closure \"axm-s10000\"), as [Old Work](kb-summary.md#old-work \"wrk-s00000\") "+
		"and [\"the old work\"](kb-summary.md#old-work \"wrk-s00000\") show; see [Main Result](leaf.md#leaf \"clm-aaaaaa\")",
		"By [Closure Law](axioms.md#axiom-s1-closure-law \"axm-s10000\"), as [New Work](kb-summary.md#new-work \"wrk-s00000\") "+
			"and [\"the old work\"](kb-summary.md#new-work \"wrk-s00000\") show; see [Main Theorem](leaf.md#leaf \"clm-aaaaaa\")", 1)
	if got := recordFileText(t, root, "leaf.md"); got != want {
		t.Errorf("leaf.md =\n%s\nwant\n%s", got, want)
	}
	if got, want := recordFileText(t, root, kb.AxiomsFile), "# Axioms\n\n"+kb.AxiomHeadingLine("S1", "Closure Law", "axm-s10000")+"\n\nIt closes.\n"; got != want {
		t.Errorf("axioms.md = %q, want %q", got, want)
	}
	if anchors := kb.HeadingAnchors(recordFileText(t, root, kb.RegisterFile)); !slices.Contains(slices.Collect(maps.Values(anchors)), "new-work") {
		t.Errorf("the summary page's anchors %v hold no new-work, where the work's citations now point", anchors)
	}
	for _, rel := range res.Written {
		if rel == kb.EntryPointFile {
			t.Errorf("written %v: an edit writes no document beyond the citing ones and axioms.md", res.Written)
		}
	}
}

// TestAnEditOfAStandingValueIsUnchanged: an edit giving the values that stand
// is unchanged and leaves the records' bytes; re-asserting the rigor of a
// record a run judged marks it manually scored and records the judged hash of
// its statement, done.
func TestAnEditOfAStandingValueIsUnchanged(t *testing.T) {
	root := editKB(t, editGraph())
	// The claim's rigor set by hand over the statement that stands: its mark and
	// its hash are what the edit below gives again.
	onlyEntryOf(t, run(t, root, "edit-claim", "entry:\n- id: clm-aaaaaa\n  rigor: 0.5\n"))
	nodes := recordFileText(t, root, kb.GraphNodesFile)
	e := onlyEntryOf(t, run(t, root, "edit-claim", "entry:\n- id: clm-aaaaaa\n  title: Main   Result\n  rigor: 0.5\n  rationale: Shown.\n"))
	if e.Outcome != result.Unchanged || recordFileText(t, root, kb.GraphNodesFile) != nodes {
		t.Errorf("entry = %+v, want unchanged with the records as they stood", e)
	}
	axioms := recordFileText(t, root, kb.AxiomsFile)
	if e := onlyEntryOf(t, run(t, root, "edit-axiom", "entry:\n- id: axm-s10000\n  title: Closure\n")); e.Outcome != result.Unchanged ||
		recordFileText(t, root, kb.AxiomsFile) != axioms {
		t.Errorf("axiom entry = %+v, want unchanged with axioms.md as it stood", e)
	}

	e = onlyEntryOf(t, run(t, root, "edit-analysis", "entry:\n- id: ana-aaaaaa\n  rigor: 0.5\n"))
	if n := records(t, root).Analyses[0]; e.Outcome != result.Done || !n.ManuallyScored || *n.Rigor != 0.5 || n.JudgedHash != nodeJudgedNow(t, root, "ana-aaaaaa") {
		t.Errorf("entry %+v, analysis %+v: want done, manually scored, the rigor standing and judged over its statement", e, n)
	}
}

// headingKB is a KB holding claims' records and leaf.md holding leaf.
func headingKB(t *testing.T, leaf string, claims ...kb.ClaimNode) string {
	t.Helper()
	root := recordKB(t, kb.Graph{Claims: claims})
	for rel, text := range map[string]string{kb.EntryPointFile: "# Entry\n\n- [Leaf](leaf.md)\n", "leaf.md": leaf} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestATitleEditRepointsTheCitationsIntoAMovedHeading: a retitle that rewrites
// a naming-form citation inside a heading re-points every citation of that
// heading to its new anchor, and of a heading whose repeat suffix shifts with
// it, each from where it pointed before the edit.
func TestATitleEditRepointsTheCitationsIntoAMovedHeading(t *testing.T) {
	cases := []struct{ name, leaf, want string }{
		{"the heading's own anchor",
			"# Leaf\n\nIt holds.<!-- kbase-id: clm-aaaaaa -->\n\n## On [Old](#leaf \"clm-aaaaaa\")\n\n" +
				"It follows.<!-- kbase-id: clm-bbbbbb -->\n\nSee [the second](#on-old \"clm-bbbbbb\").\n",
			"# Leaf\n\nIt holds.<!-- kbase-id: clm-aaaaaa -->\n\n## On [New](#leaf \"clm-aaaaaa\")\n\n" +
				"It follows.<!-- kbase-id: clm-bbbbbb -->\n\nSee [the second](#on-new \"clm-bbbbbb\").\n"},
		{"a repeat suffix shifting",
			"# Leaf\n\nIt holds.<!-- kbase-id: clm-aaaaaa -->\n\n## On [Old](#leaf \"clm-aaaaaa\")\n\n## On Old\n\n" +
				"It follows.<!-- kbase-id: clm-bbbbbb -->\n\nSee [the first](leaf.md#on-old \"clm-bbbbbb\") and [the second](<#on-old-1> \"clm-bbbbbb\").\n",
			"# Leaf\n\nIt holds.<!-- kbase-id: clm-aaaaaa -->\n\n## On [New](#leaf \"clm-aaaaaa\")\n\n## On Old\n\n" +
				"It follows.<!-- kbase-id: clm-bbbbbb -->\n\nSee [the first](leaf.md#on-new \"clm-bbbbbb\") and [the second](<#on-old> \"clm-bbbbbb\").\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := headingKB(t, c.leaf, kb.ClaimNode{ID: "clm-aaaaaa", Title: "Old", Rationale: "Shown."},
				kb.ClaimNode{ID: "clm-bbbbbb", Title: "Second", Rationale: "Shown."})
			onlyEntry(t, run(t, root, EditClaim, "entry:\n- id: clm-aaaaaa\n  title: New\n"))
			if got := recordFileText(t, root, "leaf.md"); got != c.want {
				t.Errorf("leaf.md =\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}

// TestATitleANamingFormCannotReadIsRefused: a title under which a naming-form
// citation of the node would not read as one link is refused at title, naming
// the citing path:line, and writes nothing.
func TestATitleANamingFormCannotReadIsRefused(t *testing.T) {
	root := headingKB(t, "# Leaf\n\nIt holds.<!-- kbase-id: clm-aaaaaa -->\n\nSee [Old](#leaf \"clm-aaaaaa\").\n",
		kb.ClaimNode{ID: "clm-aaaaaa", Title: "Old", Rationale: "Shown."})
	before := snapshot(t, root)
	res := run(t, root, EditClaim, "entry:\n- id: clm-aaaaaa\n  title: \"A [draft\"\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "title" ||
		!strings.Contains(res.Refusals[0].Detail, "leaf.md:5") {
		t.Errorf("result = %+v, want one refusal at title naming leaf.md:5", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused edit wrote")
	}
}

// writeDocs writes each of docs, by kb-root-relative path, under root.
func writeDocs(t *testing.T, root string, docs map[string]string) {
	t.Helper()
	for rel, text := range docs {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestARenameRepointsEveryLinkIntoTheMovedAnchor: an edit that moves the
// node's own anchor — an axiom's heading, by its title or its label, or a
// work's summary entry — re-points every link into the old anchor, a citation
// of the node or an ordinary link, changing only the link target; the naming
// form takes a new title, never a new label.
func TestARenameRepointsEveryLinkIntoTheMovedAnchor(t *testing.T) {
	const other = "[↑ Entry](entry-point.md)\n\n# Other\n\n" +
		"See [the closure axiom](axioms.md#axiom-s1-closure), [Closure](axioms.md#axiom-s1-closure \"axm-s10000\") " +
		"and [the old work](kb-summary.md#old-work).\n"
	cases := []struct{ name, op, values, want string }{
		{"an axiom's title", EditAxiom, "- id: axm-s10000\n  title: Closure Law\n",
			"See [the closure axiom](axioms.md#axiom-s1-closure-law), [Closure Law](axioms.md#axiom-s1-closure-law \"axm-s10000\") " +
				"and [the old work](kb-summary.md#old-work).\n"},
		{"an axiom's label", EditAxiom, "- id: axm-s10000\n  label: T1\n",
			"See [the closure axiom](axioms.md#axiom-t1-closure), [Closure](axioms.md#axiom-t1-closure \"axm-s10000\") " +
				"and [the old work](kb-summary.md#old-work).\n"},
		{"a work's title", EditWork, "- id: wrk-s00000\n  title: New Work\n",
			"See [the closure axiom](axioms.md#axiom-s1-closure), [Closure](axioms.md#axiom-s1-closure \"axm-s10000\") " +
				"and [the old work](kb-summary.md#new-work).\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := editKB(t, editGraph())
			writeDocs(t, root, map[string]string{"other.md": other})
			onlyEntry(t, run(t, root, c.op, "entry:\n"+c.values))
			if got := kb.SplitLines(recordFileText(t, root, "other.md"))[4]; got != strings.TrimSuffix(c.want, "\n") {
				t.Errorf("other.md line 5 =\n%s\nwant\n%s", got, c.want)
			}
			dead, err := kb.DeadLinksIn(kb.OnDisk(root), root, summarySlugs(t, root))
			if err != nil || len(dead) != 0 {
				t.Errorf("dead links %v, %v; want none", dead, err)
			}
		})
	}
	t.Run("a reference definition", func(t *testing.T) {
		root := editKB(t, editGraph())
		writeDocs(t, root, map[string]string{"other.md": "[↑ Entry](entry-point.md)\n\n# Other\n\nSee [the axiom][ax].\n\n" +
			"[ax]: axioms.md#axiom-s1-closure \"The closure axiom\"\n\n```\n[ax]: axioms.md#axiom-s1-closure\n```\n"})
		onlyEntry(t, run(t, root, EditAxiom, "entry:\n- id: axm-s10000\n  title: Closure Law\n"))
		want := "[↑ Entry](entry-point.md)\n\n# Other\n\nSee [the axiom][ax].\n\n" +
			"[ax]: axioms.md#axiom-s1-closure-law \"The closure axiom\"\n\n```\n[ax]: axioms.md#axiom-s1-closure\n```\n"
		if got := recordFileText(t, root, "other.md"); got != want {
			t.Errorf("other.md =\n%s\nwant\n%s", got, want)
		}
	})
	t.Run("another work's repeat suffix", func(t *testing.T) {
		g := editGraph()
		g.Works = append(g.Works, kb.WorkNode{ID: "wrk-j00000", Label: "Jones2021", Title: "Old Work!", Rationale: "Cited."})
		root := editKB(t, g)
		derive(t, root)
		writeDocs(t, root, map[string]string{"other.md": "[↑ Entry](entry-point.md)\n\n# Other\n\n" +
			"See [Smith](kb-summary.md#old-work-1) and [Jones](kb-summary.md#old-work).\n"})
		onlyEntry(t, run(t, root, EditWork, "entry:\n- id: wrk-s00000\n  label: Adams2020\n"))
		if got, want := kb.SplitLines(recordFileText(t, root, "other.md"))[4],
			"See [Smith](kb-summary.md#old-work) and [Jones](kb-summary.md#old-work-1)."; got != want {
			t.Errorf("other.md line 5 = %q, want %q", got, want)
		}
		dead, err := kb.DeadLinksIn(kb.OnDisk(root), root, summarySlugs(t, root))
		if err != nil || len(dead) != 0 {
			t.Errorf("dead links %v, %v; want none", dead, err)
		}
	})
	t.Run("the axiom's heading", func(t *testing.T) {
		root := editKB(t, editGraph())
		onlyEntry(t, run(t, root, EditAxiom, "entry:\n- id: axm-s10000\n  label: T1\n  title: Closure Law\n"))
		if got, want := recordFileText(t, root, kb.AxiomsFile), "# Axioms\n\n"+kb.AxiomHeadingLine("T1", "Closure Law", "axm-s10000")+"\n\nIt closes.\n"; got != want {
			t.Errorf("axioms.md = %q, want %q", got, want)
		}
		if n := records(t, root).Axioms[0]; n.Label != "T1" || n.Title != "Closure Law" {
			t.Errorf("axiom = %+v, want label T1 and title Closure Law", n)
		}
	})
}

// summarySlugs is every summary page of the KB at root by kb-root-relative
// path, with the anchors of its headings, as the dead-link check reads a page's
// fragments.
func summarySlugs(t *testing.T, root string) map[string][]string {
	t.Helper()
	pages := map[string][]string{}
	for _, page := range []string{kb.RegisterFile} {
		pages[page] = slices.Collect(maps.Values(kb.HeadingAnchors(recordFileText(t, root, page))))
	}
	return pages
}

// keysDoc uses the work's citation key Smith2020 as a key every way a
// document does — spans, their key-only rendering, a reference-list entry, a
// link into it — inside a blockquote too, and as words and inside code, which
// a rekey leaves.
const keysDoc = "[↑ Entry](entry-point.md)\n\n# Refs\n\n" +
	"As <span class=\"citation\" data-cites=\"Smith2020\">(Smith2020)</span> and\n" +
	"<span class=\"citation\"\ndata-cites=\"Jones2021 Smith2020\">(Jones 2021; Smith 2020)</span> show; see [the entry](#ref-Smith2020).\n\n" +
	"> quoted <span class=\"citation\"\n> data-cites=\"Smith2020\">(Smith2020)</span>\n\n" +
	"Smith2020 in prose stays.\n\n" +
	"```\n<span class=\"citation\" data-cites=\"Smith2020\">(Smith2020)</span>\n```\n\n" +
	"<div id=\"ref-Smith2020\" class=\"csl-entry\">\nSmith. 2020. Old Work.\n</div>\n"

// TestAWorkLabelEditRewritesEveryKeyReference: a work's label change rewrites
// each use of the old key as a key — a span's data-cites token and its
// key-only rendering, a reference-list entry's id, a link into it — in the same
// write, and leaves a rendering that is not key-only, the key as words and
// anything inside code.
func TestAWorkLabelEditRewritesEveryKeyReference(t *testing.T) {
	root := editKB(t, editGraph())
	writeDocs(t, root, map[string]string{"refs.md": keysDoc})
	e := onlyEntry(t, run(t, root, EditWork, "entry:\n- id: wrk-s00000\n  label: Smith2021\n"))
	if want := []Rewritten{{"refs.md", 5}, {"refs.md", 7}, {"refs.md", 10}, {"refs.md", 18}}; !slices.Equal(e.Rewritten, want) {
		t.Errorf("rewritten = %v, want %v", e.Rewritten, want)
	}
	if e := onlyEntry(t, run(t, root, EditWork, "entry:\n- id: wrk-s00000\n  title: Newer Work\n")); e.Rewritten != nil {
		t.Errorf("a title edit reports rewritten %v, want none", e.Rewritten)
	}
	want := "[↑ Entry](entry-point.md)\n\n# Refs\n\n" +
		"As <span class=\"citation\" data-cites=\"Smith2021\">(Smith2021)</span> and\n" +
		"<span class=\"citation\"\ndata-cites=\"Jones2021 Smith2021\">(Jones 2021; Smith 2020)</span> show; see [the entry](#ref-Smith2021).\n\n" +
		"> quoted <span class=\"citation\"\n> data-cites=\"Smith2021\">(Smith2021)</span>\n\n" +
		"Smith2020 in prose stays.\n\n" +
		"```\n<span class=\"citation\" data-cites=\"Smith2020\">(Smith2020)</span>\n```\n\n" +
		"<div id=\"ref-Smith2021\" class=\"csl-entry\">\nSmith. 2020. Old Work.\n</div>\n"
	if got := recordFileText(t, root, "refs.md"); got != want {
		t.Errorf("refs.md =\n%s\nwant\n%s", got, want)
	}
	if n := records(t, root).Works[0]; n.Label != "Smith2021" {
		t.Errorf("work = %+v, want label Smith2021", n)
	}
}

// TestALabelEditIsRefusedWhereItWouldCollide: a label another node of the kind
// carries is refused naming it; a work label a citation span or a
// reference-list entry already uses as a key is refused naming each such
// path:line; a work label outside the citation-key grammar is refused at
// shape. None writes.
func TestALabelEditIsRefusedWhereItWouldCollide(t *testing.T) {
	g := editGraph()
	g.Works = append(g.Works, kb.WorkNode{ID: "wrk-j00000", Label: "Jones2021", Title: "Other Work"})
	g.Axioms = append(g.Axioms, kb.AxiomNode{ID: "axm-s20000", Label: "S2", Title: "Openness"})
	for _, c := range []struct{ name, op, values, detail string }{
		{"a work's label", EditWork, "- id: wrk-s00000\n  label: Jones2021\n", "wrk-j00000"},
		{"an axiom's label", EditAxiom, "- id: axm-s10000\n  label: S2\n", "axm-s20000"},
		{"a key a span uses", EditWork, "- id: wrk-s00000\n  label: Brown2019\n", "refs.md:22"},
		{"a key an entry uses", EditWork, "- id: wrk-s00000\n  label: Lee2018\n", "refs.md:21"},
		{"a key outside the grammar", EditWork, "- id: wrk-s00000\n  label: Smith 2020\n", "not a citation key"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := editKB(t, g)
			writeDocs(t, root, map[string]string{"refs.md": keysDoc + "<div id=\"ref-Lee2018\" class=\"csl-entry\">Lee.</div>\n" +
				"Also <span class=\"citation\" data-cites=\"Brown2019\">(Brown2019)</span>.\n"})
			before := snapshot(t, root)
			res := run(t, root, c.op, "entry:\n"+c.values)
			if res.Outcome != result.Refused || len(res.Refusals) == 0 || res.Refusals[0].Key != "label" || !strings.Contains(res.Refusals[0].Detail, c.detail) {
				t.Errorf("result = %+v, want refused at label naming %s", res, c.detail)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused edit wrote")
			}
		})
	}
}

// onlyEntryOf is res's one entry, failing where res did not apply one.
func onlyEntryOf(t *testing.T, res Result) EntryResult {
	t.Helper()
	if len(res.Entries) != 1 {
		t.Fatalf("result = %+v, want one entry", res)
	}
	return res.Entries[0]
}
