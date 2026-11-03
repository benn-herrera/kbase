package query

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/result"
)

func f(v float64) *float64 { return &v }

// The node records the fixtures are built of. A scored claim rests on
// scoringAxiom at bearing 1.0, so its strength is its rigor.
const scoringAxiom = "axm-s00000"

func claim(id, title string) kb.ClaimNode { return kb.ClaimNode{ID: id, Title: title} }

// scored is g with the claim id at rigor (pending where nil), resting on
// scoringAxiom, which g gains where it lacks it.
func scored(g kb.Graph, id string, rigor *float64) kb.Graph {
	if !slices.ContainsFunc(g.Axioms, func(a kb.AxiomNode) bool { return a.ID == scoringAxiom }) {
		g.Axioms = append(g.Axioms, kb.AxiomNode{ID: scoringAxiom, Label: "S0", Title: "The scoring rule"})
	}
	for i := range g.Claims {
		if g.Claims[i].ID == id {
			g.Claims[i].Rigor = rigor
		}
	}
	for i := range g.Analyses {
		if g.Analyses[i].ID == id {
			g.Analyses[i].Rigor = rigor
		}
	}
	g.Founds = append(g.Founds, kb.BearingEdge{Source: scoringAxiom, Target: id, Bearing: kb.Bearing{Value: 1}})
	return g
}

func scorable(relation, source, target string, bearing float64) kb.ScorableEdge {
	return kb.ScorableEdge{Relation: relation, Source: source, Target: target, Bearing: kb.Bearing{Value: bearing}}
}

// leafDoc is a leaf of the main volume holding body.
func leafDoc(body string) string { return "[↑ Up](index.md)\n\n# Leaf\n\n" + body }

// writeRecordKB writes a KB of g's records and docs, each at its kb-root
// relative path, and returns its kb-root; an entry point docs does not supply
// is stamped and lists the main volume, whose index lists main/leaf.md.
func writeRecordKB(t *testing.T, g kb.Graph, docs map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	files := map[string]string{
		kb.EntryPointFile: "---\nkb-format: \"" + kb.FormatVersion + "\"\n---\n# KB\n\n- [Main](main/index.md)\n",
		"main/index.md":   "[↑ Up](../entry-point.md)\n\n# Main\n\n- [Leaf](leaf.md)\n",
		kb.GraphNodesFile: string(kb.EncodeNodes(g)),
		kb.GraphEdgesFile: string(kb.EncodeEdges(g)),
		"main/leaf.md":    leafDoc(""),
	}
	maps.Copy(files, docs)
	for rel, text := range files {
		writeFile(t, filepath.Join(root, rel), text)
	}
	return root
}

// load is the index Load answers over a KB of g's records, every claim,
// analysis and experiment but those unhosted names hosted by a statement
// marker in main/leaf.md.
func load(t *testing.T, g kb.Graph, unhosted ...string) *Index {
	t.Helper()
	var body strings.Builder
	host := func(id string) {
		if !slices.Contains(unhosted, id) {
			fmt.Fprintf(&body, "The statement of %s. %s\n\n", id, kb.StatementMarker(id))
		}
	}
	for _, n := range g.Claims {
		host(n.ID)
	}
	for _, n := range g.Analyses {
		host(n.ID)
	}
	for _, n := range g.Experiments {
		host(n.ID)
	}
	return loadKB(t, writeRecordKB(t, g, map[string]string{"main/leaf.md": leafDoc(body.String())}))
}

func loadKB(t *testing.T, root string) *Index {
	t.Helper()
	ix, err := Load(kb.OnDisk(root))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return ix
}

func ids(nodes []kb.NodeRow) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

// everyKindIndex holds a node of every kind: clm-aaaaaa (0.9, one dependent,
// clm-cccccc) and ana-aaaaaa (0.7) hosted in main/leaf.md; clm-bbbbbb (0.5)
// and exp-aaaaaa (rigor 0.5) in main/two.md; clm-cccccc pending and unhosted;
// wrk-a00000 pending; and scoringAxiom, founding the three scored claims and the
// analysis, so with three dependents.
func everyKindIndex(t *testing.T) *Index {
	t.Helper()
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Alpha result", Rationale: "Shown."}, claim("clm-bbbbbb", "Beta result"),
			claim("clm-cccccc", "Gamma result")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "Alpha method", Rationale: "Built."}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "A run", Status: kb.ExperimentRun, Rigor: f(0.5)}},
		Works:       []kb.WorkNode{{ID: "wrk-a00000", Label: "A", Title: "Alpha book", Rationale: "Cited."}},
		Scorable:    []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-aaaaaa", "clm-cccccc", 1)},
	}
	g = scored(scored(scored(g, "clm-aaaaaa", f(0.9)), "clm-bbbbbb", f(0.5)), "ana-aaaaaa", f(0.7))
	two := "[↑ Up](index.md)\n\n# Two\n\nThe run. " + kb.StatementMarker("exp-aaaaaa") + "\n\nBeta. " + kb.StatementMarker("clm-bbbbbb") + "\n"
	return loadKB(t, writeRecordKB(t, g, map[string]string{
		"main/index.md": "[↑ Up](../entry-point.md)\n\n# Main\n\n- [Leaf](leaf.md)\n- [Two](two.md)\n",
		"main/leaf.md":  leafDoc("Alpha. " + kb.StatementMarker("clm-aaaaaa") + "\n\nThe method. " + kb.StatementMarker("ana-aaaaaa") + "\n"),
		"main/two.md":   two,
	}))
}

func foundIDs(nodes []FoundNode) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.Node.ID)
	}
	return out
}

func sp(s string) *string { return &s }

func ip(n int) *int { return &n }

// TestFindFilters: each filter alone keeps the nodes it names, across every
// kind, in id order; min 0 keeps exactly the scored nodes.
func TestFindFilters(t *testing.T) {
	ix := everyKindIndex(t)
	all := []string{"ana-aaaaaa", scoringAxiom, "clm-aaaaaa", "clm-bbbbbb", "clm-cccccc", "exp-aaaaaa", "wrk-a00000"}
	hostedInMain := []string{"ana-aaaaaa", "clm-aaaaaa", "clm-bbbbbb", "exp-aaaaaa"}
	for _, tc := range []struct {
		name string
		sel  FindSelection
		want []string
	}{
		{"no filter", FindSelection{}, all},
		{"kind claim", FindSelection{Kinds: []string{kb.NodeKindClaim}}, []string{"clm-aaaaaa", "clm-bbbbbb", "clm-cccccc"}},
		{"kind analysis", FindSelection{Kinds: []string{kb.NodeKindAnalysis}}, []string{"ana-aaaaaa"}},
		{"kind experiment", FindSelection{Kinds: []string{kb.NodeKindExperiment}}, []string{"exp-aaaaaa"}},
		{"kind work", FindSelection{Kinds: []string{kb.NodeKindWork}}, []string{"wrk-a00000"}},
		{"kind axiom", FindSelection{Kinds: []string{kb.NodeKindAxiom}}, []string{scoringAxiom}},
		{"two kinds", FindSelection{Kinds: []string{kb.NodeKindWork, kb.NodeKindAxiom}}, []string{scoringAxiom, "wrk-a00000"}},
		{"title, case-insensitively", FindSelection{Title: "ALPHA"}, []string{"ana-aaaaaa", "clm-aaaaaa", "wrk-a00000"}},
		{"title matching none", FindSelection{Title: "no-such-node"}, nil},
		{"path a leaf", FindSelection{Path: sp("main/leaf.md")}, []string{"ana-aaaaaa", "clm-aaaaaa"}},
		{"path a directory", FindSelection{Path: sp("main")}, hostedInMain},
		{"path a directory with its slash", FindSelection{Path: sp("main/")}, hostedInMain},
		{"path empty, kb-root", FindSelection{Path: sp("")}, hostedInMain},
		{"path dot, kb-root", FindSelection{Path: sp(".")}, hostedInMain},
		{"path holding no host", FindSelection{Path: sp("mai")}, nil},
		{"min", FindSelection{Min: f(0.7)}, []string{"ana-aaaaaa", scoringAxiom, "clm-aaaaaa"}},
		{"max", FindSelection{Max: f(0.7)}, []string{"clm-bbbbbb", "exp-aaaaaa"}},
		{"min 0, the scored nodes", FindSelection{Min: f(0)}, []string{"ana-aaaaaa", scoringAxiom, "clm-aaaaaa", "clm-bbbbbb", "exp-aaaaaa"}},
		{"pending", FindSelection{Pending: true}, []string{"clm-cccccc", "wrk-a00000"}},
		{"min-dependents", FindSelection{MinDependents: 1}, []string{scoringAxiom, "clm-aaaaaa"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.sel.Order = OrderAscending
			if got := foundIDs(ix.Find(tc.sel)); !slices.Equal(got, tc.want) {
				t.Errorf("Find = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFindSortsAndLimits: a pending strength sorts below every number,
// either order flips only the key, ties broken by ascending strength then
// id, and a limit keeps the first or, negative, the last N in the sort's
// order.
func TestFindSortsAndLimits(t *testing.T) {
	ix := everyKindIndex(t)
	for _, tc := range []struct {
		name        string
		sort, order string
		limit       *int
		want        []string
	}{
		{"id descending", SortID, OrderDescending, nil,
			[]string{"wrk-a00000", "exp-aaaaaa", "clm-cccccc", "clm-bbbbbb", "clm-aaaaaa", scoringAxiom, "ana-aaaaaa"}},
		{"strength ascending, pending first", SortStrength, OrderAscending, nil,
			[]string{"clm-cccccc", "wrk-a00000", "clm-bbbbbb", "exp-aaaaaa", "ana-aaaaaa", "clm-aaaaaa", scoringAxiom}},
		{"strength descending, pending last", SortStrength, OrderDescending, nil,
			[]string{scoringAxiom, "clm-aaaaaa", "ana-aaaaaa", "clm-bbbbbb", "exp-aaaaaa", "clm-cccccc", "wrk-a00000"}},
		{"dependents descending, ties by ascending strength", SortDependents, OrderDescending, nil,
			[]string{scoringAxiom, "clm-aaaaaa", "clm-cccccc", "wrk-a00000", "clm-bbbbbb", "exp-aaaaaa", "ana-aaaaaa"}},
		{"the first two", SortStrength, OrderAscending, ip(2), []string{"clm-cccccc", "wrk-a00000"}},
		{"the last two, in the sort's order", SortStrength, OrderAscending, ip(-2), []string{"clm-aaaaaa", scoringAxiom}},
		{"a limit past the answer", SortStrength, OrderDescending, ip(-20),
			[]string{scoringAxiom, "clm-aaaaaa", "ana-aaaaaa", "clm-bbbbbb", "exp-aaaaaa", "clm-cccccc", "wrk-a00000"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := foundIDs(ix.Find(FindSelection{Sort: sp(tc.sort), Order: tc.order, Limit: tc.limit})); !slices.Equal(got, tc.want) {
				t.Errorf("Find = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCompareRanksPutsANullLast: a pending value sorts below every number,
// and a null after every value whichever the direction.
func TestCompareRanksPutsANullLast(t *testing.T) {
	values := []rank{{null: true}, {value: 0.5}, {pending: true}, {value: 0}}
	for _, tc := range []struct {
		descending bool
		want       []rank
	}{
		{false, []rank{{pending: true}, {value: 0}, {value: 0.5}, {null: true}}},
		{true, []rank{{value: 0.5}, {value: 0}, {pending: true}, {null: true}}},
	} {
		got := slices.Clone(values)
		slices.SortStableFunc(got, func(a, b rank) int { return compareRanks(a, b, tc.descending) })
		if !slices.Equal(got, tc.want) {
			t.Errorf("descending %v: %v, want %v", tc.descending, got, tc.want)
		}
	}
}

// TestFindRows: a find row is its node's id, node_type, title, label (null for a
// hosted kind), host (null where it has none), strength (*pending* where
// pending) and dependents.
func TestFindRows(t *testing.T) {
	rows, usage := FindPayload(everyKindIndex(t), FindSelection{Kinds: []string{kb.NodeKindClaim, kb.NodeKindWork}, Order: OrderAscending})
	if usage != nil {
		t.Fatalf("usage refusals %v", usage)
	}
	row := func(id, kind, title string, label, host, strength any, dependents int) result.Record {
		return result.Record{{Key: "id", Value: id}, {Key: "node_type", Value: kind}, {Key: "title", Value: result.Verbatim(title)}, {Key: "label", Value: label},
			{Key: "host", Value: host}, {Key: "strength", Value: strength}, {Key: "dependents", Value: dependents}}
	}
	want := []result.Record{
		row("clm-aaaaaa", kb.NodeKindClaim, "Alpha result", nil, "main/leaf.md", 0.9, 1),
		row("clm-bbbbbb", kb.NodeKindClaim, "Beta result", nil, "main/two.md", 0.5, 0),
		row("clm-cccccc", kb.NodeKindClaim, "Gamma result", nil, nil, kb.PendingLiteral, 0),
		row("wrk-a00000", kb.NodeKindWork, "Alpha book", result.Verbatim("A"), nil, kb.PendingLiteral, 0),
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("find rows = %v, want %v", rows, want)
	}
}

// TestFindByLabel: label keeps the works and axioms whose label contains it,
// case-folded, and never a node of a hosted kind, which carries none.
func TestFindByLabel(t *testing.T) {
	ix := everyKindIndex(t)
	for _, tc := range []struct {
		label string
		want  []string
	}{
		{"a", []string{"wrk-a00000"}},
		{"s0", []string{scoringAxiom}},
		{"alpha", nil},
	} {
		var got []string
		for _, n := range ix.Find(FindSelection{Label: tc.label, Order: OrderAscending}) {
			got = append(got, n.Node.ID)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("label %q: %v, want %v", tc.label, got, tc.want)
		}
	}
}

// TestFindRefusesItsUsage: a kind, sort or order outside its vocabulary,
// pending beside a bound, a limit beside no sort and a limit of 0 are each
// refused as usage, naming the flag.
func TestFindRefusesItsUsage(t *testing.T) {
	ix := everyKindIndex(t)
	for _, tc := range []struct {
		name    string
		sel     FindSelection
		key     string
		allowed []string
	}{
		{"an unknown kind", FindSelection{Kinds: []string{"lemma"}, Order: OrderAscending}, argmark.Flag("kind"), kb.NodeKinds},
		{"an unknown sort", FindSelection{Sort: sp("title"), Order: OrderAscending}, argmark.Flag("sort"), FindSorts},
		{"an unknown order", FindSelection{Order: "up"}, argmark.Flag("order"), Orders},
		{"pending beside min", FindSelection{Pending: true, Min: f(0), Order: OrderAscending}, argmark.Flag("pending"), nil},
		{"a limit without sort", FindSelection{Limit: ip(1), Order: OrderAscending}, argmark.Flag("limit"), nil},
		{"a limit of 0", FindSelection{Sort: sp(SortStrength), Limit: ip(0), Order: OrderAscending}, argmark.Flag("limit"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, usage := FindPayload(ix, tc.sel)
			if rows != nil || len(usage) != 1 || usage[0].Check != result.CheckUsage || usage[0].Key != tc.key ||
				!slices.Equal(usage[0].Allowed, tc.allowed) || usage[0].Detail == "" {
				t.Errorf("FindPayload = %v, %+v; want one usage item at %s allowing %v", rows, usage, tc.key, tc.allowed)
			}
		})
	}
}

// TestNoNodeSendsToFind: show refuses an id no node carries with the one
// no-node refusal, whose remedy names find, and find lists a node of a kind
// other than claim.
func TestNoNodeSendsToFind(t *testing.T) {
	ix := everyKindIndex(t)
	if _, ok := ShowPayload(ix, "axm-z99999"); ok {
		t.Fatal("show axm-z99999 answered")
	}
	if item := NoNode(argmark.Positional("id"), "axm-z99999"); item.Check != kb.CheckUnknownID || !strings.HasPrefix(item.Remedy, kb.NodeLister) {
		t.Errorf("no-node refusal %+v, want check %s and a remedy opening %q", item, kb.CheckUnknownID, kb.NodeLister)
	}
	rows, _ := FindPayload(ix, FindSelection{Kinds: []string{kb.NodeKindAxiom}, Order: OrderAscending})
	if len(rows) != 1 || rows[0][0].Value != scoringAxiom {
		t.Errorf("find --kind axiom = %v, want %s", rows, scoringAxiom)
	}
}

// TestCitationsOf: every citation of a node, read by its link title wherever
// the link points, ordered by path then line, its link text as written; a
// citation inside code, one of another node and the summary pages are not
// read; an id no node carries has no answer.
func TestCitationsOf(t *testing.T) {
	cite := func(text string) string { return "[" + text + `](../main/leaf.md#leaf "clm-aaaaaa")` }
	root := writeRecordKB(t, kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A Result"), claim("clm-bbbbbb", "Another")}},
		map[string]string{
			"main/leaf.md": leafDoc("It holds. " + kb.StatementMarker("clm-aaaaaa") + "\n\nOther. " + kb.StatementMarker("clm-bbbbbb") + "\n"),
			"z/note.md":    "# Note\n\nAs " + cite("the *result*") + " says.\n",
			"b/note.md": "# Note\n\nFirst " + cite("A Result") + " and " + cite("again") + ".\n\n```\n" + cite("code") + "\n```\n\n" +
				"Then `" + cite("inline") + "` and [another](../main/leaf.md#leaf \"clm-bbbbbb\") and " + cite("last") + ".\n",
			kb.RegisterFile: "# KB summary\n\n" + cite("summary") + "\n",
		})
	rows, ok, err := CitationsOfPayload(loadKB(t, root), "clm-aaaaaa")
	if err != nil || !ok {
		t.Fatalf("CitationsOfPayload: %v, %t", err, ok)
	}
	var got []string
	for _, r := range rows {
		got = append(got, fmt.Sprintf("%v:%v %v", r[0].Value, r[1].Value, r[2].Value))
	}
	want := []string{"b/note.md:3 A Result", "b/note.md:3 again", "b/note.md:9 last", "z/note.md:3 the *result*"}
	if !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
	if _, ok, _ := CitationsOfPayload(loadKB(t, root), "clm-zzzzzz"); ok {
		t.Error("an id no node carries was answered")
	}
}

// TestCitationsOfReadsALeafNamedLikeASummaryPage: a document named
// kb-summary.md where no summary page stands is read like any other; the
// volume's own page is not.
func TestCitationsOfReadsALeafNamedLikeASummaryPage(t *testing.T) {
	cite := `[A Result](../leaf.md#leaf "clm-aaaaaa")`
	root := writeRecordKB(t, kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A Result")}},
		map[string]string{
			"main/leaf.md":                leafDoc("It holds. " + kb.StatementMarker("clm-aaaaaa") + "\n"),
			"main/sub/" + kb.RegisterFile: "# Sub\n\nAs " + cite + " says.\n",
			"main/" + kb.RegisterFile:     "# KB summary\n\n" + cite + "\n",
		})
	cites, err := loadKB(t, root).CitationsOf("clm-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	want := []Citation{{Path: "main/sub/" + kb.RegisterFile, Line: 3, Text: "A Result"}}
	if !slices.Equal(cites, want) {
		t.Errorf("CitationsOf = %+v, want %+v", cites, want)
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func leaf(body string) string {
	return "[↑ Up](../index.md)\n\n# Leaf\n\n" + body + "\n"
}

// TestDependentCounts: a node's dependents are the distinct targets of its
// outgoing scoring edges; a references or demoted edge makes neither end one.
func TestDependentCounts(t *testing.T) {
	demoted := func(source, target string) kb.DemotedEdge {
		return kb.DemotedEdge{Source: source, Target: target, ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}}
	}
	ix := load(t, kb.Graph{
		Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C"), claim("clm-dddddd", "D"),
			claim("clm-eeeeee", "E"), claim("clm-ffffff", "F"), claim("clm-gggggg", "G"), claim("clm-hhhhhh", "H")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentRun}},
		References:  []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-ffffff"}, {Source: "clm-bbbbbb", Target: "clm-aaaaaa"}},
		Demoted:     []kb.DemotedEdge{demoted("clm-aaaaaa", "clm-gggggg"), demoted("clm-cccccc", "clm-aaaaaa")},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-aaaaaa", "clm-dddddd", 1),
			scorable(kb.RelationSupports, "clm-aaaaaa", "clm-eeeeee", 1),
			scorable(kb.RelationRefutes, "clm-aaaaaa", "clm-dddddd", 1),
			scorable(kb.RelationLifts, "exp-aaaaaa", "clm-aaaaaa", 1),
			scorable(kb.RelationLifts, "ana-aaaaaa", "clm-aaaaaa", 1),
			scorable(kb.RelationRefutes, "clm-aaaaaa", "clm-hhhhhh", 1),
		},
	})
	for id, want := range map[string]int{"clm-aaaaaa": 3, "exp-aaaaaa": 1, "ana-aaaaaa": 1,
		"clm-bbbbbb": 0, "clm-cccccc": 0, "clm-ffffff": 0, "clm-gggggg": 0} {
		if got := ix.dependentCounts()[id]; got != want {
			t.Errorf("dependents of %s = %d, want %d", id, got, want)
		}
	}
}

func TestNodeFieldsTotalOverKinds(t *testing.T) {
	for _, kind := range kb.NodeKinds {
		if nodeFields[kind] == nil {
			t.Errorf("no field list for node kind %q", kind)
		}
	}
	if len(nodeFields) != len(kb.NodeKinds) {
		t.Errorf("nodeFields has %d kinds, kb.NodeKinds %d", len(nodeFields), len(kb.NodeKinds))
	}
}

// TestAnalysisBandAndRecord: an analysis's strength band is derived from its
// strength as the index loads, and show's record carries its kind, rigor and
// strength.
func TestAnalysisBandAndRecord(t *testing.T) {
	ix := load(t, scored(kb.Graph{Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}}}, "ana-aaaaaa", f(0.7)))
	n, ok := ix.Node("ana-aaaaaa")
	if !ok || n.StrengthBand != "ok-with-caveats" {
		t.Errorf("analysis node = %+v, %v; want strength_band ok-with-caveats", n, ok)
	}
	rec, ok := ShowPayload(ix, "ana-aaaaaa")
	if !ok {
		t.Fatal("show ana-aaaaaa: no record")
	}
	want := map[string]any{"node_type": kb.NodeKindAnalysis, "id": "ana-aaaaaa", "rigor": 0.7, "strength": 0.7, "strength_band": "ok-with-caveats"}
	got := map[string]any{}
	for _, f := range rec {
		got[f.Key] = f.Value
	}
	for key, v := range want {
		if !reflect.DeepEqual(got[key], v) {
			t.Errorf("show %s = %v, want %v", key, got[key], v)
		}
	}
}

// TestShowDisplaysAStrength: show's strength field is the display rounding of
// a value the walk computed, while the index row keeps its exact value.
func TestShowDisplaysAStrength(t *testing.T) {
	bearing, premise := 0.2, 0.9
	exact := bearing * premise
	g := kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", bearing)}}
	ix := load(t, scored(scored(g, "clm-bbbbbb", f(premise)), "clm-aaaaaa", f(1)))
	rec, ok := ShowPayload(ix, "clm-aaaaaa")
	if !ok {
		t.Fatal("show clm-aaaaaa: no record")
	}
	var got any
	for _, f := range rec {
		if f.Key == "strength" {
			got = f.Value
		}
	}
	if got != 0.18 {
		t.Errorf("show strength = %v, want the display rounding 0.18", got)
	}
	if n, ok := ix.Node("clm-aaaaaa"); !ok || n.Strength == nil || *n.Strength != exact {
		t.Errorf("the index row's strength = %v, want the exact %v", n.Strength, exact)
	}
}

// TestShowEveryKind: each kind's record is its identity — host, host_anchor
// and summary, each null where the node has none — then its kind's own
// fields, an axiom's rigor its 1.0 by rule, a pending rigor or strength
// *pending*.
func TestShowEveryKind(t *testing.T) {
	ix := everyKindIndex(t)
	idFields := func(kind, id, title string, host, hostAnchor, summary any) result.Record {
		return result.Record{{Key: "node_type", Value: kind}, {Key: "id", Value: id}, {Key: "title", Value: result.Verbatim(title)},
			{Key: "host", Value: host}, {Key: "host_anchor", Value: hostAnchor}, {Key: "summary", Value: summary}}
	}
	page := "main/" + kb.RegisterFile + "#"
	for _, tc := range []struct {
		id   string
		want result.Record
	}{
		{"clm-aaaaaa", append(idFields(kb.NodeKindClaim, "clm-aaaaaa", "Alpha result", "main/leaf.md", "leaf", page+"alpha-result"),
			result.Record{{Key: "rigor", Value: 0.9}, {Key: manuallyScoredKey, Value: false}, {Key: "strength", Value: 0.9},
				{Key: "build_status", Value: "ok to build on"}, {Key: "strength_band", Value: "ok-to-build"}, {Key: "rationale", Value: result.Verbatim("Shown.")},
				{Key: "premise_count", Value: 1}}...)},
		{"clm-cccccc", append(idFields(kb.NodeKindClaim, "clm-cccccc", "Gamma result", nil, nil, nil),
			result.Record{{Key: "rigor", Value: kb.PendingLiteral}, {Key: manuallyScoredKey, Value: false}, {Key: "strength", Value: kb.PendingLiteral},
				{Key: "build_status", Value: nil}, {Key: "strength_band", Value: kb.UnknownBandSlug}, {Key: "rationale", Value: result.Verbatim("")},
				{Key: "premise_count", Value: 1}}...)},
		{"ana-aaaaaa", append(idFields(kb.NodeKindAnalysis, "ana-aaaaaa", "Alpha method", "main/leaf.md", "leaf", page+"alpha-method"),
			result.Record{{Key: "rigor", Value: 0.7}, {Key: manuallyScoredKey, Value: false}, {Key: "strength", Value: 0.7},
				{Key: "strength_band", Value: "ok-with-caveats"}, {Key: "rationale", Value: result.Verbatim("Built.")}}...)},
		{"exp-aaaaaa", append(idFields(kb.NodeKindExperiment, "exp-aaaaaa", "A run", "main/two.md", "two", page+"a-run"),
			result.Record{{Key: "status", Value: kb.ExperimentRun}, {Key: "rigor", Value: 0.5}, {Key: manuallyScoredKey, Value: false}}...)},
		{"wrk-a00000", append(idFields(kb.NodeKindWork, "wrk-a00000", "Alpha book", nil, nil, kb.RegisterFile+"#alpha-book"),
			result.Record{{Key: "label", Value: result.Verbatim("A")}, {Key: "rigor", Value: kb.PendingLiteral}, {Key: "rationale", Value: result.Verbatim("Cited.")}}...)},
		{scoringAxiom, append(idFields(kb.NodeKindAxiom, scoringAxiom, "The scoring rule", nil, nil, nil),
			result.Field{Key: "label", Value: result.Verbatim("S0")}, result.Field{Key: "rigor", Value: kb.AxiomRigor})},
	} {
		got, ok := ShowPayload(ix, tc.id)
		if !ok {
			t.Fatalf("show %s: no record", tc.id)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("show %s = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestAnUnhostedNodeHasNoHostOrSummary: show writes an unhosted node's host,
// host anchor and summary as null, and a hosted node's as its host leaf, the section it is
// stated in and its summary-page entry.
func TestAnUnhostedNodeHasNoHostOrSummary(t *testing.T) {
	g := kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentRun}}}
	ix := load(t, scored(scored(scored(g, "clm-aaaaaa", f(0.3)), "clm-bbbbbb", f(0.3)), "ana-aaaaaa", f(0.7)),
		"clm-aaaaaa", "ana-aaaaaa", "exp-aaaaaa")
	placed := func(rec result.Record) [3]any {
		var out [3]any
		for _, field := range rec {
			switch field.Key {
			case "host":
				out[0] = field.Value
			case "host_anchor":
				out[1] = field.Value
			case "summary":
				out[2] = field.Value
			}
		}
		return out
	}
	hosted := [3]any{"main/leaf.md", "leaf", "main/" + kb.RegisterFile + "#b"}
	for _, tc := range []struct {
		id   string
		want [3]any
	}{{"clm-aaaaaa", [3]any{}}, {"clm-bbbbbb", hosted}, {"ana-aaaaaa", [3]any{}}, {"exp-aaaaaa", [3]any{}}} {
		rec, ok := ShowPayload(ix, tc.id)
		if !ok {
			t.Fatalf("show %s: no record", tc.id)
		}
		if got := placed(rec); got != tc.want {
			t.Errorf("show %s host, host anchor and summary = %#v, want %#v", tc.id, got, tc.want)
		}
	}
}

// TestStatsSubtreeAggregatesPerVolume: stats' subtree_aggregates holds one
// record per volume, its claims and its experiments stated in a leaf under it.
func TestStatsSubtreeAggregatesPerVolume(t *testing.T) {
	got, _ := fieldAt(StatsPayload(everyKindIndex(t)), "subtree_aggregates")
	want := result.Record{{Key: "main", Value: result.Record{{Key: "claims", Value: 2}, {Key: "experiments", Value: 1}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stats subtree_aggregates = %v, want %v", got, want)
	}
}

// TestStatsRanksTheScoredClaims: stats' weakest is the scored claims, weakest
// first, a pending claim and every other kind left out, and stats carries no
// citation count.
func TestStatsRanksTheScoredClaims(t *testing.T) {
	stats := StatsPayload(everyKindIndex(t))
	if _, at := fieldAt(stats, "citation_edges"); at != -1 {
		t.Errorf("stats carries citation_edges: %v", stats)
	}
	weakest, _ := fieldAt(stats, "weakest")
	var got []string
	for _, rec := range weakest.([]result.Record) {
		got = append(got, rec[0].Value.(string))
	}
	if want := []string{"clm-bbbbbb", "clm-aaaaaa"}; !slices.Equal(got, want) {
		t.Errorf("stats weakest = %v, want %v", got, want)
	}
}

func TestStatsCensusPartitionsClaims(t *testing.T) {
	g := kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentRun}},
		Works:       []kb.WorkNode{{ID: "wrk-a00000", Label: "A", Title: "W"}},
		Axioms:      []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "T"}}}
	ix := load(t, g)
	census := StatsPayload(ix)[:len(kb.NodeKinds)]
	total := 0
	var keys []string
	for _, field := range census {
		total += field.Value.(int)
		keys = append(keys, field.Key)
	}
	if want := len(index.GraphKinds(g)); total != want {
		t.Errorf("census sums to %d, want %d", total, want)
	}
	if want := []string{"claims", "analyses", "experiments", "axioms", "works"}; !slices.Equal(keys, want) {
		t.Errorf("census keys %q, want %q", keys, want)
	}
}

func TestStatsCountsDemotedEdges(t *testing.T) {
	demoted := func(source, relation string) kb.DemotedEdge {
		return kb.DemotedEdge{Source: source, Target: "clm-aaaaaa", ReplacedRelation: relation, Bearing: kb.Bearing{Pending: true}}
	}
	ix := load(t, kb.Graph{
		Claims:     []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C")},
		Scorable:   []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 1)},
		Demoted:    []kb.DemotedEdge{demoted("clm-bbbbbb", kb.RelationRefutes), demoted("clm-cccccc", kb.RelationSupports)},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-cccccc"}},
	})
	counts := map[string]any{}
	for _, field := range StatsPayload(ix) {
		counts[field.Key] = field.Value
	}
	if counts["edges"] != 4 || counts["demoted_edges"] != 2 {
		t.Errorf("edges %v, demoted_edges %v; want 4 and 2", counts["edges"], counts["demoted_edges"])
	}
	demotedCensus, _ := fieldAt(counts["relations"].(result.Record), kb.RelationDemoted)
	if want := (result.Record{{Key: "edges", Value: 2}, {Key: "walked", Value: false}}); !reflect.DeepEqual(demotedCensus, want) {
		t.Errorf("stats' relations demoted = %v, want %v", demotedCensus, want)
	}
}

// relationsIndex holds both bearing classes on supports — a number, one whose
// text sorts after another's (1e-05 after 0.5), and pending — one row each on
// founds, lifts and references, and none on refutes or demoted. One supports
// row carries a context.
func relationsIndex(t *testing.T) *Index {
	t.Helper()
	withContext := scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 0.5)
	withContext.Context = "step 2"
	pending := scorable(kb.RelationSupports, "clm-dddddd", "clm-aaaaaa", 0)
	pending.Bearing = kb.Bearing{Pending: true}
	return load(t, kb.Graph{
		Claims:   []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C"), claim("clm-dddddd", "D")},
		Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S", Rigor: f(0.8)}},
		Works:    []kb.WorkNode{{ID: "wrk-b00000", Label: "B", Title: "B"}},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-cccccc", "clm-bbbbbb", 0.5),
			withContext,
			scorable(kb.RelationSupports, "clm-cccccc", "clm-aaaaaa", 1e-05),
			scorable(kb.RelationLifts, "ana-aaaaaa", "clm-aaaaaa", 0.8),
			pending,
		},
		Founds:     []kb.BearingEdge{{Source: "wrk-b00000", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}}},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb"}},
	})
}

func bearingCounts(pairs ...any) []result.Record {
	out := []result.Record{}
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, result.Record{{Key: "bearing", Value: pairs[i]}, {Key: "edges", Value: pairs[i+1]}})
	}
	return out
}

// TestStatsRelations: stats' relations stands after bearing_model and before
// strength_bands, one record per relation in kb.Relations order, its bearings
// numeric, then pending.
func TestStatsRelations(t *testing.T) {
	stats := StatsPayload(relationsIndex(t))
	got, at := fieldAt(stats, "relations")
	if _, model := fieldAt(stats, "bearing_model"); at != model+1 || stats[at+1].Key != "strength_bands" {
		t.Errorf("relations at %d, want right after bearing_model at %d and before strength_bands", at, model)
	}
	want := result.Record{
		{Key: kb.RelationSupports, Value: result.Record{{Key: "edges", Value: 4}, {Key: "walked", Value: true},
			{Key: "bearings", Value: bearingCounts("1e-05", 1, "0.5", 2, kb.PendingLiteral, 1)}}},
		{Key: kb.RelationFounds, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: false},
			{Key: "bearings", Value: bearingCounts("0.5", 1)}}},
		{Key: kb.RelationLifts, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: true},
			{Key: "bearings", Value: bearingCounts("0.8", 1)}}},
		{Key: kb.RelationRefutes, Value: result.Record{{Key: "edges", Value: 0}, {Key: "walked", Value: true},
			{Key: "bearings", Value: []result.Record{}}}},
		{Key: kb.RelationReferences, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: false}}},
		{Key: kb.RelationDemoted, Value: result.Record{{Key: "edges", Value: 0}, {Key: "walked", Value: false}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stats' relations = %v, want %v", got, want)
	}
}

// edgeRows is an edges answer as "source relation target=bearing context
// replaced_relation" lines, checking each row's keys.
func edgeRows(t *testing.T, records []result.Record) []string {
	t.Helper()
	var out []string
	for _, r := range records {
		var keys []string
		for _, f := range r {
			keys = append(keys, f.Key)
		}
		if want := []string{"source", "target", "relation", "bearing", "context", manuallyScoredKey, "replaced_relation"}; !slices.Equal(keys, want) {
			t.Fatalf("edges record keys %q, want %q", keys, want)
		}
		out = append(out, fmt.Sprintf("%v %v %v=%v %v %v", r[0].Value, r[2].Value, r[1].Value, r[3].Value, r[4].Value, r[6].Value))
	}
	return out
}

func TestEdges(t *testing.T) {
	ix := relationsIndex(t)
	every := []string{
		"clm-bbbbbb supports clm-aaaaaa=0.5 step 2 <nil>",
		"clm-cccccc supports clm-aaaaaa=1e-05 <nil> <nil>",
		"clm-cccccc supports clm-bbbbbb=0.5 <nil> <nil>",
		"clm-dddddd supports clm-aaaaaa=*pending* <nil> <nil>",
		"wrk-b00000 founds clm-aaaaaa=0.5 <nil> <nil>",
		"ana-aaaaaa lifts clm-aaaaaa=0.8 <nil> <nil>",
		"clm-aaaaaa references clm-bbbbbb=<nil> <nil> <nil>",
	}
	node := "clm-bbbbbb"
	for _, tc := range []struct {
		name string
		sel  EdgesSelection
		want []string
	}{
		{"every relation", EdgesSelection{}, every},
		{"founds", EdgesSelection{Relations: []string{kb.RelationFounds}}, every[4:5]},
		{"two relations", EdgesSelection{Relations: []string{kb.RelationReferences, kb.RelationLifts}}, every[5:]},
		{"a relation with no rows", EdgesSelection{Relations: []string{kb.RelationRefutes}}, nil},
		{"a node, both ways by default", EdgesSelection{ID: &node}, []string{every[0], every[2], every[6]}},
		{"a node, in", EdgesSelection{ID: &node, Direction: DirectionIn}, []string{every[2], every[6]}},
		{"a node, out", EdgesSelection{ID: &node, Direction: DirectionOut}, every[0:1]},
		{"a node, both, of a relation", EdgesSelection{ID: &node, Direction: DirectionBoth, Relations: []string{kb.RelationReferences}}, every[6:]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, usage := EdgesPayload(ix, tc.sel)
			if usage != nil {
				t.Fatalf("refused: %v", usage)
			}
			if got := edgeRows(t, rows); !slices.Equal(got, tc.want) {
				t.Errorf("edges %+v = %q, want %q", tc.sel, got, tc.want)
			}
		})
	}
}

// TestEdgesNameTheReplacedRelation: a demoted row names the relation its
// record replaced, two records joining one pair each their own, ordered by
// it; every other row's replaced_relation is null. A demoted row carries the
// bearing and the mark its record keeps of the replaced edge.
func TestEdgesNameTheReplacedRelation(t *testing.T) {
	demoted := func(source, relation string) kb.DemotedEdge {
		return kb.DemotedEdge{Source: source, Target: "clm-aaaaaa", ReplacedRelation: relation, Bearing: kb.Bearing{Pending: true}}
	}
	scoredCut := demoted("clm-bbbbbb", kb.RelationRefutes)
	scoredCut.Bearing, scoredCut.ManuallyScored = kb.Bearing{Value: 0.3}, true
	ix := load(t, kb.Graph{
		Claims:   []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C")},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-cccccc", "clm-aaaaaa", 1)},
		Demoted:  []kb.DemotedEdge{scoredCut, demoted("clm-cccccc", kb.RelationSupports), demoted("clm-bbbbbb", kb.RelationSupports)},
	})
	rows, usage := EdgesPayload(ix, EdgesSelection{})
	if usage != nil {
		t.Fatalf("refused: %v", usage)
	}
	want := []string{
		"clm-cccccc supports clm-aaaaaa=1.0 <nil> <nil>",
		"clm-bbbbbb demoted clm-aaaaaa=*pending* <nil> supports",
		"clm-bbbbbb demoted clm-aaaaaa=0.3 <nil> refutes",
		"clm-cccccc demoted clm-aaaaaa=*pending* <nil> supports",
	}
	if got := edgeRows(t, rows); !slices.Equal(got, want) {
		t.Errorf("edges = %q, want %q", got, want)
	}
	var marks []any
	for _, r := range rows {
		marks = append(marks, r[5].Value)
	}
	if want := []any{false, false, true, false}; !slices.Equal(marks, want) {
		t.Errorf("manually_scored = %v, want %v", marks, want)
	}
}

func TestEdgesRefusesItsUsage(t *testing.T) {
	ix := relationsIndex(t)
	node, missing := "clm-aaaaaa", "clm-ffffff"
	for _, tc := range []struct {
		name string
		sel  EdgesSelection
		want []string
	}{
		{"an unknown relation, once", EdgesSelection{Relations: []string{"nope", "nope"}}, []string{"usage --relation"}},
		{"an unknown direction", EdgesSelection{ID: &node, Direction: "sideways"}, []string{"usage --direction"}},
		{"a direction without a node", EdgesSelection{Direction: DirectionBoth}, []string{"usage --direction"}},
		{"an id no node carries", EdgesSelection{ID: &missing}, []string{kb.CheckUnknownID + " --id"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, usage := EdgesPayload(ix, tc.sel)
			var got []string
			for _, item := range usage {
				got = append(got, item.Check+" "+argmark.Spelling{}.Spell(item.Key))
			}
			if rows != nil || !slices.Equal(got, tc.want) {
				t.Errorf("refusals %q (rows %v), want %q", got, rows, tc.want)
			}
		})
	}
}

// sheetFixture is the 3.0.0 KB the sheet goldens are drawn from.
var sheetFixture = filepath.Join("..", "sheet", "testdata", "adjagent-sheet", "kb-root")

// convertedSheetKB is a copy of the sheet fixture converted to the current
// format by update-format's save.
func convertedSheetKB(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "kb-root")
	if err := os.CopyFS(root, os.DirFS(sheetFixture)); err != nil {
		t.Fatal(err)
	}
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := index.UpdateFormat(src, nil, nil, log.Discard()); err != nil {
		t.Fatal(err)
	}
	return root
}

// editedSheetKB is a converted copy of the sheet fixture whose records edit
// has changed, written through kb's record encoders.
func editedSheetKB(t *testing.T, edit func(*kb.Graph)) *kb.Source {
	t.Helper()
	src := kb.OnDisk(convertedSheetKB(t))
	g, err := kb.LoadGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	edit(&g)
	for rel, data := range map[string][]byte{kb.GraphNodesFile: kb.EncodeNodes(g), kb.GraphEdgesFile: kb.EncodeEdges(g)} {
		if err := os.WriteFile(src.KBPath(rel), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

// TestLoadAnswersFromTheRecords: Load holds every node the KB's records hold.
func TestLoadAnswersFromTheRecords(t *testing.T) {
	src := kb.OnDisk(convertedSheetKB(t))
	g, err := kb.LoadGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Sorted(maps.Keys(index.GraphKinds(g)))
	ix, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := ids(ix.Nodes()); len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("Load holds %v, want the records' %v", got, want)
	}
}

// TestLoadAnswersAConvertedKB: a KB converted at load and not yet saved
// answers, holding the nodes it holds read as current.
func TestLoadAnswersAConvertedKB(t *testing.T) {
	current, err := Load(kb.OnDisk(convertedSheetKB(t)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := kbload.Open(sheetFixture)
	if err != nil {
		t.Fatal(err)
	}
	converted, err := Load(src)
	if err != nil {
		t.Fatalf("Load of a converted, unsaved KB: %v", err)
	}
	if got, want := ids(converted.Nodes()), ids(current.Nodes()); !slices.Equal(got, want) {
		t.Errorf("converted KB holds %v, want %v", got, want)
	}
}

// TestLoadRefusesALoadError: a KB whose load fails — here on a premise cycle
// — is refused, one check: metadata item per fault naming the edge file, each
// with the remedy in git.
func TestLoadRefusesALoadError(t *testing.T) {
	src := editedSheetKB(t, func(g *kb.Graph) {
		g.Scorable = append(g.Scorable, kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-aaa003", Target: "clm-aaa002",
			Bearing: kb.Bearing{Value: 1}})
	})
	_, err := Load(src)
	var r result.Refusal
	if !errors.As(err, &r) || len(r) == 0 {
		t.Fatalf("Load over a cycle = %v, want a refusal", err)
	}
	for _, item := range r {
		if item.Check != kb.CheckMetadata || item.Remedy != kb.ConsistentRecordsRemedy || item.Path != kb.GraphEdgesFile || item.Detail == "" {
			t.Errorf("refusal item %+v, want check %s, path %s, remedy %q and a detail", item, kb.CheckMetadata, kb.GraphEdgesFile, kb.ConsistentRecordsRemedy)
		}
	}
}

// manualSheetKB is the sheet fixture with clm-aaa001, exp-aaa008 and the
// supports edge clm-aaa001 → clm-aaa003 manually scored, and a second demoted
// record of clm-aaa004 → clm-aaa001, replacing a refutation, manually scored:
// a pair two demoted records show alike.
func manualSheetKB(t *testing.T) *Index {
	t.Helper()
	src := editedSheetKB(t, func(g *kb.Graph) {
		for i := range g.Claims {
			g.Claims[i].ManuallyScored = g.Claims[i].ID == "clm-aaa001"
		}
		for i := range g.Experiments {
			g.Experiments[i].ManuallyScored = g.Experiments[i].ID == "exp-aaa008"
		}
		for i, e := range g.Scorable {
			g.Scorable[i].ManuallyScored = e.Relation == kb.RelationSupports && e.Source == "clm-aaa001" && e.Target == "clm-aaa003"
		}
		g.Demoted = append(g.Demoted, kb.DemotedEdge{Source: "clm-aaa004", Target: "clm-aaa001", ReplacedRelation: kb.RelationRefutes,
			Bearing: kb.Bearing{Value: 0.5}, ManuallyScored: true})
	})
	ix, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// fieldAt is the value of rec's field key and its position, -1 where rec has
// none.
func fieldAt(rec result.Record, key string) (any, int) {
	for i, f := range rec {
		if f.Key == key {
			return f.Value, i
		}
	}
	return nil, -1
}

// TestShowCarriesManuallyScored: a claim's, an analysis's and an experiment's
// record carry manually_scored right after rigor; a work's and an axiom's
// carry none.
func TestShowCarriesManuallyScored(t *testing.T) {
	ix := manualSheetKB(t)
	for _, tc := range []struct {
		id   string
		want any
	}{
		{"clm-aaa001", true},
		{"clm-aaa002", false},
		{"ana-aaa007", false},
		{"exp-aaa008", true},
		// The conversion's minted ids for the 3.0.0 wrk-smith2020 and axm-S1.
		{"wrk-omcjwo", nil},
		{"axm-7b65tg", nil},
	} {
		rec, ok := ShowPayload(ix, tc.id)
		if !ok {
			t.Fatalf("show %s: no record", tc.id)
		}
		got, at := fieldAt(rec, manuallyScoredKey)
		_, rigor := fieldAt(rec, "rigor")
		switch {
		case tc.want == nil && at != -1:
			t.Errorf("show %s carries manually_scored: %v", tc.id, rec)
		case tc.want != nil && (got != tc.want || at != rigor+1):
			t.Errorf("show %s: manually_scored %v at %d, want %v right after rigor at %d", tc.id, got, at, tc.want, rigor)
		}
	}
}

// TestEdgesCarryManuallyScored: an edge record carries its record's
// manually-scored value after its context, null on a relation whose records
// carry none.
func TestEdgesCarryManuallyScored(t *testing.T) {
	ix := manualSheetKB(t)
	manual := map[string]any{}
	rows, usage := EdgesPayload(ix, EdgesSelection{Relations: []string{kb.RelationSupports, kb.RelationFounds}})
	if usage != nil {
		t.Fatalf("refused: %v", usage)
	}
	for _, rec := range rows {
		if rec[5].Key != manuallyScoredKey {
			t.Fatalf("edges record %v: field 6 is %s, want %s", rec, rec[5].Key, manuallyScoredKey)
		}
		manual[fmt.Sprint(rec[0].Value, " ", rec[2].Value, " ", rec[1].Value)] = rec[5].Value
	}
	for name, want := range map[string]any{
		"clm-aaa001 supports clm-aaa003": true,
		"clm-aaa001 supports clm-bbb001": false,
		"axm-7b65tg founds clm-aaa001":   nil, // the conversion's minted id for the 3.0.0 axm-S1
	} {
		if got, ok := manual[name]; !ok || got != want {
			t.Errorf("edges %s: manually_scored %v (present %v), want %v", name, got, ok, want)
		}
	}
}

// TestStatsCountsManualAndModelScores: the fixture's flagged records are its
// nine claims, one analysis and one experiment, and its seven scorable and
// three demoted edges; two of each are manually scored.
func TestStatsCountsManualAndModelScores(t *testing.T) {
	stats := StatsPayload(manualSheetKB(t))
	_, at := fieldAt(stats, "subtree_aggregates")
	var got []string
	for _, f := range stats[at+1 : at+5] {
		got = append(got, fmt.Sprint(f.Key, "=", f.Value))
	}
	if want := []string{"rigor_manual=2", "rigor_model=9", "bearing_manual=2", "bearing_model=8"}; !slices.Equal(got, want) {
		t.Errorf("stats after subtree_aggregates = %q, want %q", got, want)
	}
}
