package write

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// addLeaf is the leaf the add tests name: a heading, two paragraphs holding
// "bound holds", and a fenced block between them.
const addLeaf = "[↑ Entry](entry-point.md)\n\n# Main Result\n\nThe bound holds for every n.\n\n" +
	"```math\nx = y\n```\n\nThe bound holds again later.\n"

// addKB is a KB holding g's records and one leaf, leaf.md, holding addLeaf.
func addKB(t *testing.T, g kb.Graph) string {
	t.Helper()
	root := recordKB(t, g)
	for rel, text := range map[string]string{kb.EntryPointFile: "# Entry\n\n- [Leaf](leaf.md)\n", "leaf.md": addLeaf} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// mintedIDRE is a whole minted id of kind prefix.
func mintedIDRE(prefix string) *regexp.Regexp {
	return regexp.MustCompile(`^` + kb.IDBody(prefix) + `$`)
}

// onlyEntry is res's one entry, failing where res is not done with one entry.
func onlyEntry(t *testing.T, res Result) EntryResult {
	t.Helper()
	if res.Outcome != result.Done || len(res.Entries) != 1 {
		t.Fatalf("result = %+v, want done with one entry", res)
	}
	return res.Entries[0]
}

// TestAddsWriteTheirKeys: each add mints its id, writes its record from its
// keys — rigor pending, not manually scored and never judged where left out,
// manually scored and judged over the statement its marker gives it where
// supplied, *pending* included — and a hosted node's marker at the end of the
// named line.
func TestAddsWriteTheirKeys(t *testing.T) {
	place := "  document: leaf.md\n  line: 5\n  snippet: bound   holds\n"
	cases := []struct {
		name, op, values string
		check            func(t *testing.T, root string, g kb.Graph, id string)
	}{
		{"claim, rigor left out", "add-claim", "- title: A   Bound\n  rationale: ''\n" + place, func(t *testing.T, root string, g kb.Graph, id string) {
			want := kb.ClaimNode{ID: id, Title: "A Bound", Rationale: ""}
			if len(g.Claims) != 1 || g.Claims[0] != want {
				t.Errorf("claims = %+v, want [%+v]", g.Claims, want)
			}
		}},
		{"claim, rigor supplied", "add-claim", "- title: A Bound\n  rationale: Shown.\n  rigor: 0.5\n" + place, func(t *testing.T, root string, g kb.Graph, id string) {
			if n := g.Claims[0]; n.Rigor == nil || *n.Rigor != 0.5 || !n.ManuallyScored || n.Rationale != "Shown." || n.JudgedHash != nodeJudgedNow(t, root, id) {
				t.Errorf("claim = %+v, want rigor 0.5, manually scored, rationale Shown., judged over the statement its marker gives it", n)
			}
		}},
		{"analysis, pending supplied", "add-analysis", "- title: A Proof\n  rationale: Shown.\n  rigor: '*pending*'\n" + place, func(t *testing.T, root string, g kb.Graph, id string) {
			want := kb.AnalysisNode{ID: id, Title: "A Proof", ManuallyScored: true, Rationale: "Shown.", JudgedHash: nodeJudgedNow(t, root, id)}
			if len(g.Analyses) != 1 || g.Analyses[0] != want {
				t.Errorf("analyses = %+v, want [%+v]", g.Analyses, want)
			}
		}},
		{"experiment", "add-experiment", "- title: A Trial\n  status: planned\n" + place, func(t *testing.T, root string, g kb.Graph, id string) {
			want := kb.ExperimentNode{ID: id, Title: "A Trial", Status: kb.ExperimentPlanned}
			if len(g.Experiments) != 1 || g.Experiments[0] != want {
				t.Errorf("experiments = %+v, want [%+v]", g.Experiments, want)
			}
		}},
		{"work", "add-work", "- label: Smith2020\n  title: A Work\n  rationale: Cited.\n  rigor: 0.7\n", func(t *testing.T, root string, g kb.Graph, id string) {
			if !mintedIDRE(kb.WorkPrefix).MatchString(id) || len(g.Works) != 1 || g.Works[0].Label != "Smith2020" ||
				g.Works[0].Title != "A Work" || *g.Works[0].Rigor != 0.7 {
				t.Errorf("id %s, works = %+v, want a minted id labelled Smith2020", id, g.Works)
			}
		}},
		{"work, rigor left out", "add-work", "- label: Jones2021\n  title: Another Work\n  rationale: Cited.\n", func(t *testing.T, root string, g kb.Graph, id string) {
			if len(g.Works) != 1 || g.Works[0].Rigor != nil {
				t.Errorf("works = %+v, want one whose rigor is %s", g.Works, kb.PendingLiteral)
			}
		}},
		{"axiom", "add-axiom", "- label: '$\\alpha$:  the first'\n  title: Closure\n", func(t *testing.T, root string, g kb.Graph, id string) {
			want := kb.AxiomNode{ID: id, Label: `$\alpha$: the first`, Title: "Closure"}
			if !mintedIDRE(kb.AxiomPrefix).MatchString(id) || len(g.Axioms) != 1 || g.Axioms[0] != want {
				t.Errorf("axioms = %+v, want [%+v], its id minted", g.Axioms, want)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := addKB(t, kb.Graph{})
			e := onlyEntry(t, run(t, root, c.op, "entry:\n"+c.values))
			prefix := map[string]string{AddClaim: kb.ClaimPrefix, AddAnalysis: kb.AnalysisPrefix, AddExperiment: kb.ExperimentPrefix,
				AddWork: kb.WorkPrefix, AddAxiom: kb.AxiomPrefix}[c.op]
			if !mintedIDRE(prefix).MatchString(e.ID) {
				t.Fatalf("entry = %+v, want a minted id", e)
			}
			c.check(t, root, records(t, root), e.ID)
			if c.op == "add-work" || c.op == "add-axiom" {
				return
			}
			line := kb.SplitLines(recordFileText(t, root, "leaf.md"))[4]
			if want := "The bound holds for every n." + kb.StatementMarker(e.ID); line != want {
				t.Errorf("line 5 = %q, want %q", line, want)
			}
		})
	}
}

// TestAnAddWritesTheFlagItsCallerEarns: a minted node's rigor, and the
// bearings of the edges a second batch gives it, land manually scored where
// the caller supplied them, and pending and not manually scored where it left
// them out; under machine provenance nothing lands manually scored. A supplied
// number is judged over the statements it was set against under either
// provenance, and a left-out value is never judged. A work's edge founds the
// node, and carries no mark.
func TestAnAddWritesTheFlagItsCallerEarns(t *testing.T) {
	g := kb.Graph{
		Claims:      []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Hosted", Rigor: f64(0.5), Rationale: "Seeded."}, {ID: "clm-bbbbbb", Title: "Loose", Rigor: f64(0.5), Rationale: "Seeded."}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Hosted", Status: "run", Rigor: f64(0.5)}},
		Works:       []kb.WorkNode{{ID: "wrk-k00000", Label: "k", Title: "W", Rigor: f64(0.5), Rationale: "Cited."}},
	}
	for _, c := range []struct {
		name       string
		provenance Provenance
		manual     bool
	}{{"caller", ProvenanceCaller, true}, {"machine", ProvenanceMachine, false}} {
		t.Run(c.name, func(t *testing.T) {
			root := addKB(t, g)
			id := onlyEntry(t, runRecordOp(t, root, AddClaim,
				"entry:\n- title: New\n  rationale: Shown.\n  rigor: 0.4\n  document: leaf.md\n  line: 5\n  snippet: bound\n", c.provenance)).ID
			edges := runRecordOp(t, root, Batch, "entry:\n"+
				edgeEntry(AddEdge, "clm-aaaaaa", id, kb.RelationSupports, "bearing: 0.5")+
				edgeEntry(AddEdge, "exp-aaaaaa", id, kb.RelationSupports)+
				edgeEntry(AddEdge, "wrk-k00000", id, kb.RelationFounds, "context: its bound")+
				edgeEntry(AddEdge, id, "clm-bbbbbb", kb.RelationRefutes), c.provenance)
			if edges.Outcome != result.Done {
				t.Fatalf("the new claim's edges: %+v", edges)
			}
			got := records(t, root)
			n := got.Claims[slices.IndexFunc(got.Claims, func(n kb.ClaimNode) bool { return n.ID == id })]
			if n.Rigor == nil || *n.Rigor != 0.4 || n.ManuallyScored != c.manual || n.JudgedHash != nodeJudgedNow(t, root, id) {
				t.Errorf("claim = %+v, want rigor 0.4 manually-scored %v, judged over its statement", n, c.manual)
			}
			want := []kb.ScorableEdge{
				{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: id, Bearing: kb.Bearing{Value: 0.5}, ManuallyScored: c.manual,
					JudgedHash: edgeJudgedNow(t, root, "clm-aaaaaa", id)},
				{Relation: kb.RelationSupports, Source: "exp-aaaaaa", Target: id, Bearing: kb.Bearing{Pending: true}},
				{Relation: kb.RelationRefutes, Source: id, Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}},
			}
			for _, w := range want {
				if !slices.Contains(got.Scorable, w) {
					t.Errorf("no record %+v among %+v", w, got.Scorable)
				}
			}
			founds := kb.BearingEdge{Source: "wrk-k00000", Target: id, Context: "its bound", Bearing: kb.Bearing{Pending: true}}
			if !slices.Equal(got.Founds, []kb.BearingEdge{founds}) {
				t.Errorf("founds = %+v, want %+v", got.Founds, founds)
			}
		})
	}
}

// TestAnAddRefusesItsKeysOutOfShape: a missing document, line or snippet is a
// shape refusal naming the key; a rigor above 1, a status outside run and
// planned, and a title holding a line break are refused at their key.
func TestAnAddRefusesItsKeysOutOfShape(t *testing.T) {
	full := map[string]string{"title": "A Trial", "status": "run", "document": "leaf.md", "line": "5", "snippet": "bound"}
	entryOf := func(over map[string]string, drop string) string {
		v := "entry:\n-"
		for _, k := range []string{"title", "status", "document", "line", "snippet"} {
			value, ok := over[k]
			if !ok {
				value = full[k]
			}
			if k != drop {
				v += " " + k + ": " + value + "\n "
			}
		}
		return v
	}
	cases := []struct {
		name, values, key string
	}{
		{"no document", entryOf(nil, "document"), "document"},
		{"no line", entryOf(nil, "line"), "line"},
		{"no snippet", entryOf(nil, "snippet"), "snippet"},
		{"rigor above 1", "entry:\n- title: T\n  status: run\n  rigor: 1.5\n  document: leaf.md\n  line: 5\n  snippet: bound\n", "rigor"},
		{"status outside", entryOf(map[string]string{"status": "pending"}, ""), "status"},
		{"title with a line break", entryOf(map[string]string{"title": `"Two\nlines"`}, ""), "title"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, refusals := parseValues([]byte(c.values), "add-experiment")
			if len(refusals) != 1 || refusals[0].Key != c.key {
				t.Errorf("refusals = %+v, want one at %s", refusals, c.key)
			}
		})
	}
}

// TestAnAddRefusesATitleItsKindHolds: an add whose title a node of its kind
// carries is refused naming that node, writing nothing, and so is a work or an
// axiom whose label another of its kind carries; the same title on another
// kind is accepted.
func TestAnAddRefusesATitleItsKindHolds(t *testing.T) {
	g := kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Main Result", Rationale: ""}},
		Works:  []kb.WorkNode{{ID: "wrk-o00000", Label: "Old1", Title: "Main Result"}},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "Main Result"}}}
	place := "  document: leaf.md\n  line: 3\n  snippet: Main\n"
	refused := []struct{ op, values, key, id string }{
		{"add-claim", "- title: Main   Result\n  rationale: ''\n" + place, "title", "clm-aaaaaa"},
		{"add-work", "- label: New2\n  title: Main Result\n  rationale: ''\n", "title", "wrk-o00000"},
		{"add-work", "- label: Old1\n  title: Another\n  rationale: ''\n", "label", "wrk-o00000"},
		{"add-axiom", "- label: S2\n  title: Main Result\n", "title", "axm-s10000"},
		{"add-axiom", "- label: S1\n  title: Another\n", "label", "axm-s10000"},
	}
	for _, c := range refused {
		t.Run(c.op+" "+c.key, func(t *testing.T) {
			root := addKB(t, g)
			before := snapshot(t, root)
			res := run(t, root, c.op, "entry:\n"+c.values)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.key ||
				res.Refusals[0].Detail != "node exists: "+c.id || res.Refusals[0].Entry != 1 {
				t.Errorf("result = %+v, want one refusal at %s, node exists: %s", res, c.key, c.id)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused add wrote")
			}
		})
	}
	t.Run("another kind", func(t *testing.T) {
		root := addKB(t, g)
		onlyEntry(t, run(t, root, "add-analysis", "entry:\n- title: Main Result\n  rationale: ''\n"+place))
	})
}

// TestAnAddPlacesItsMarkerOnTheNamedLine: the marker attaches with no space,
// a heading's anchor unchanged by it; a line past the end, a fence's opening
// or closing line, and a line not containing the snippet are refused, the
// last naming the nearby lines that do.
func TestAnAddPlacesItsMarkerOnTheNamedLine(t *testing.T) {
	root := addKB(t, kb.Graph{})
	before := kb.HeadingAnchors(addLeaf)[2]
	e := onlyEntry(t, run(t, root, "add-claim", "entry:\n- title: T\n  rationale: ''\n  document: leaf.md\n  line: 3\n  snippet: Main Result\n"))
	text := recordFileText(t, root, "leaf.md")
	if line := kb.SplitLines(text)[2]; line != "# Main Result"+kb.StatementMarker(e.ID) {
		t.Errorf("line 3 = %q, want the marker attached with no space", line)
	}
	if after := kb.HeadingAnchors(text)[2]; after != before || before != "main-result" {
		t.Errorf("anchor %q before, %q after, want main-result both", before, after)
	}

	for _, c := range []struct {
		name, line, snippet, key string
		detail                   []string
	}{
		{"past the end", "40", "bound", "line", []string{"line 40", "past the end"}},
		{"fence opening", "7", "math", "line", []string{"line 7", "fenced block"}},
		{"fence closing", "9", "`", "line", []string{"line 9", "fenced block"}},
		{"no snippet", "3", "bound holds", "snippet", []string{"line 3", "lines 5, 11"}},
		{"snippet nowhere", "3", "absent words", "snippet", []string{"no line"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := addKB(t, kb.Graph{})
			before := snapshot(t, root)
			res := run(t, root, "add-analysis", "entry:\n- title: T\n  rationale: ''\n  document: leaf.md\n  line: "+c.line+
				"\n  snippet: '"+c.snippet+"'\n")
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.key {
				t.Fatalf("result = %+v, want one refusal at %s", res, c.key)
			}
			for _, want := range c.detail {
				if !strings.Contains(res.Refusals[0].Detail, want) {
					t.Errorf("detail %q does not say %q", res.Refusals[0].Detail, want)
				}
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused add wrote")
			}
		})
	}
}

// TestARepeatedAddIsRefused: an add re-issued is refused, node exists, with
// kb-root byte-identical; a batch adding one node twice is refused at the
// repeat, writing nothing.
func TestARepeatedAddIsRefused(t *testing.T) {
	adds := map[string]string{
		"add-claim":      "- title: T\n  rationale: ''\n  document: leaf.md\n  line: 5\n  snippet: bound\n",
		"add-analysis":   "- title: T\n  rationale: ''\n  document: leaf.md\n  line: 5\n  snippet: bound\n",
		"add-experiment": "- title: T\n  status: run\n  document: leaf.md\n  line: 5\n  snippet: bound\n",
		"add-work":       "- label: Smith2020\n  title: T\n  rationale: ''\n",
		"add-axiom":      "- label: S1\n  title: T\n",
	}
	for op, values := range adds {
		t.Run(op, func(t *testing.T) {
			root := addKB(t, kb.Graph{})
			id := onlyEntry(t, run(t, root, op, "entry:\n"+values)).ID
			before := snapshot(t, root)
			res := run(t, root, op, "entry:\n"+values)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Detail != "node exists: "+id {
				t.Errorf("re-issue = %+v, want refused, node exists: %s", res, id)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a re-issued add changed kb-root")
			}

			fresh := addKB(t, kb.Graph{})
			before = snapshot(t, fresh)
			res = run(t, fresh, op, "entry:\n"+values+values)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Entry != 2 ||
				!strings.HasPrefix(res.Refusals[0].Detail, "node exists: ") {
				t.Errorf("one batch adding twice = %+v, want refused at entry 2, node exists", res)
			}
			if !maps.Equal(before, snapshot(t, fresh)) {
				t.Error("a refused batch wrote")
			}
		})
	}
}

// TestAddAxiomGivesItsMarkedHeading: add-axiom marks the one unmarked heading
// of exactly its text in axioms.md, reporting heading marked, and appends its
// marked heading otherwise, creating the file where absent, reporting heading
// appended; a heading of its text another axiom's marker ends, and one of
// other text, are not its own.
func TestAddAxiomGivesItsMarkedHeading(t *testing.T) {
	for _, c := range []struct{ name, before, want, heading string }{
		{"absent", "", "### Axiom S2: A Rule@\n", "appended"},
		{"prose above", "# Axioms\n\n### Axiom S1: Closure<!-- kbase-id: axm-s10000 -->\n\nIt closes.\n",
			"# Axioms\n\n### Axiom S1: Closure<!-- kbase-id: axm-s10000 -->\n\nIt closes.\n\n### Axiom S2: A Rule@\n", "appended"},
		{"unmarked heading stands", "### Axiom S2: A Rule  \n\nAuthored first.\n", "### Axiom S2: A Rule@\n\nAuthored first.\n", "marked"},
		{"only a heading of other text", "### Axiom S2: A Rul\n", "### Axiom S2: A Rul\n\n### Axiom S2: A Rule@\n", "appended"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := addKB(t, kb.Graph{Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "Closure"}}})
			if c.before != "" {
				if err := os.WriteFile(filepath.Join(root, kb.AxiomsFile), []byte(c.before), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			e := onlyEntry(t, run(t, root, "add-axiom", "entry:\n- label: S2\n  title: A   Rule\n"))
			want := strings.Replace(c.want, "@", kb.StatementMarker(e.ID), 1)
			if got := recordFileText(t, root, kb.AxiomsFile); got != want {
				t.Errorf("axioms.md = %q, want %q", got, want)
			}
			if e.Heading != c.heading {
				t.Errorf("heading = %q, want %q", e.Heading, c.heading)
			}
		})
	}
}

// TestAddAxiomRefusesTwoUnmarkedHeadingsOfItsText: two unmarked headings of
// the axiom's text are refused, naming their lines, and nothing is written.
func TestAddAxiomRefusesTwoUnmarkedHeadingsOfItsText(t *testing.T) {
	root := addKB(t, kb.Graph{})
	if err := os.WriteFile(filepath.Join(root, kb.AxiomsFile), []byte("### Axiom S2: A Rule\n\nOne.\n\n### Axiom S2: A Rule\n\nTwo.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	res := run(t, root, "add-axiom", "entry:\n- label: S2\n  title: A Rule\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "label" ||
		res.Refusals[0].Path != kb.AxiomsFile || !strings.Contains(res.Refusals[0].Detail, "lines 1, 5") {
		t.Errorf("result = %+v, want one refusal at label naming lines 1 and 5 of %s", res, kb.AxiomsFile)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused add-axiom wrote")
	}
}

// TestSummaryMayShift: an entry can move another's anchor only where a slug,
// its repeat suffixes stripped, is shared — with another node's title or with
// the fixed rigor and strength headings every entry carries; the node's own
// title is no other entry.
func TestSummaryMayShift(t *testing.T) {
	g := kb.Graph{Works: []kb.WorkNode{{ID: "wrk-s00000", Label: "S", Title: "Old Work"}}}
	for _, c := range []struct {
		name, id, title string
		titled          string
		want            bool
	}{
		{"an unrelated title", "", "New Result", "", false},
		{"the same slug", "", "Old Work!", "", true},
		{"a suffixed slug", "", "Old Work 2", "", true},
		{"the node's own title", "wrk-s00000", "Old Work", "", false},
		{"a fixed heading's slug", "", "Rigor", "", true},
		{"a standing title of a fixed heading's slug", "", "New Result", "Strength 3", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := &recordBatch{g: g}
			if c.titled != "" {
				b.g.Claims = []kb.ClaimNode{{ID: "clm-aaaaaa", Title: c.titled}}
			}
			if got := b.summaryMayShift(c.id, c.title); got != c.want {
				t.Errorf("summaryMayShift(%q, %q) = %v, want %v", c.id, c.title, got, c.want)
			}
		})
	}
}

// TestAnAddShiftingARepeatSuffixRepointsTheLinksIntoIt: an add whose summary
// entry takes a slug a standing entry held, so that entry takes the next
// repeat suffix, re-points every link into the standing entry to its new
// anchor in the same write, an ordinary link or a reference definition.
func TestAnAddShiftingARepeatSuffixRepointsTheLinksIntoIt(t *testing.T) {
	const other = "[↑ Entry](entry-point.md)\n\n# Other\n\nSee [Smith](kb-summary.md#old-work).\n\n[smith]: kb-summary.md#old-work\n"
	for _, c := range []struct{ name, op, values string }{
		{"add-work", AddWork, "- label: Adams2019\n  title: Old Work!\n  rationale: Cited.\n"},
		{"add-claim", AddClaim, "- title: Old Work?\n  rationale: Shown.\n  document: leaf.md\n  line: 15\n  snippet: By\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := editKB(t, editGraph())
			writeDocs(t, root, map[string]string{"other.md": other})
			onlyEntry(t, run(t, root, c.op, "entry:\n"+c.values))
			want := "[↑ Entry](entry-point.md)\n\n# Other\n\nSee [Smith](kb-summary.md#old-work-1).\n\n[smith]: kb-summary.md#old-work-1\n"
			if got := recordFileText(t, root, "other.md"); got != want {
				t.Errorf("other.md =\n%s\nwant\n%s", got, want)
			}
			if dead, err := kb.DeadLinksIn(kb.OnDisk(root), root, summarySlugs(t, root)); err != nil || len(dead) != 0 {
				t.Errorf("dead links %v, %v; want none", dead, err)
			}
		})
	}
}
