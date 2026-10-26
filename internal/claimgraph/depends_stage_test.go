package claimgraph

import (
	"context"
	"slices"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/records"
)

// TestDependsAsksEveryLinkOnce: over a leaf holding prose A, B and C, block
// D (Theorem 1, which B and C name) and equation node E, in that reading
// order, the cull record's links are each asked once from their later claim
// to their earlier, an equation always the target: offered two letters where
// the target is the equation and three otherwise, never one. A pair the
// unmarked record answered yes both ways is one entry, and a link whose pair
// a marked harvest reached is not asked from the link's side. An in-support-of
// answer writes target to source and a mention a references row. The ring the
// answers close, A → D → C → A, loses the edge the walk closes it on, A's
// premise C.
func TestDependsAsksEveryLinkOnce(t *testing.T) {
	const (
		a = "clm-aaaaaa"
		b = "clm-bbbbbb"
		c = "clm-cccccc"
		d = "clm-dddddd"
		e = "clm-eeeeee"
	)
	m := func(id string) string { return " <!-- claim-rigor: " + id + " -->" }
	root, repo, opts := seedTheoremLeaf(t,
		map[string]string{a: "Result A", b: "Result B", c: "Result C", d: "Theorem 1", e: "Equation (`eq:one`) — x equals one"},
		"Alpha holds here."+m(a)+"\n\nBeta holds here, by Theorem 1."+m(b)+"\n\nGamma holds here, by Theorem 1."+m(c)+"\n\n"+
			"> **Theorem**\n>\n> **Theorem 1.** Delta holds here."+m(d)+"\n\n``` math\nx = 1 \\label{eq:one}"+m(e)+"\n```\n",
		records.Record{Kind: records.KindFence, Order: 2, Document: cullLeaf, EquationLabels: []string{"eq:one"}})
	yes := asks.LetterPoints
	if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{Pairs: []buildrecords.CandidateEntry{
		{Source: a, Target: b, Offered: []string{asks.LetterPoints, asks.LetterDoesNotPoint}, Letter: &yes, Outcome: string(asks.Answered)},
		{Source: b, Target: a, Offered: []string{asks.LetterPoints, asks.LetterDoesNotPoint}, Letter: &yes, Outcome: string(asks.Answered)},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteCull(repo, buildrecords.CullRecord{Links: []buildrecords.CullLink{
		{A: a, B: b, Harvest: harvestUnmarked},
		{A: a, B: c, Harvest: harvestAdjacent},
		{A: a, B: d, Harvest: harvestUnmarked},
		{A: b, B: d, Harvest: harvestCulled},
		{A: b, B: e, Harvest: harvestAdjacent},
	}}); err != nil {
		t.Fatal(err)
	}
	supported, inSupport, mention := asks.LetterSupportedBy, asks.LetterInSupportOf, asks.LetterMention
	reader := &letters{by: map[string]string{
		b + "->" + a: inSupport,
		c + "->" + a: inSupport,
		d + "->" + a: supported,
		b + "->" + e: mention,
		b + "->" + d: mention,
		c + "->" + d: supported,
	}}
	opts.Reader = reader.read
	report, err := Depends(context.Background(), opts)
	requireStagePassed(t, report, err)

	three := []string{supported, inSupport, mention}
	type entry struct {
		pair    pair
		offered []string
		letter  string
	}
	want := []entry{
		{pair{b, a}, three, inSupport},
		{pair{b, d}, three, mention},
		{pair{b, e}, []string{supported, mention}, mention},
		{pair{c, a}, three, inSupport},
		{pair{c, d}, three, supported},
		{pair{d, a}, three, supported},
	}
	rec, _, err := buildrecords.ReadClassification(recordsAt(repo))
	if err != nil {
		t.Fatal(err)
	}
	var got []entry
	for _, ce := range rec.Candidates {
		letter := ""
		if ce.Letter != nil {
			letter = *ce.Letter
		}
		got = append(got, entry{pair{ce.Source, ce.Target}, ce.Offered, letter})
	}
	slices.SortFunc(got, func(x, y entry) int { return comparePairs(x.pair, y.pair) })
	if !slices.EqualFunc(got, want, func(x, y entry) bool {
		return x.pair == y.pair && slices.Equal(x.offered, y.offered) && x.letter == y.letter
	}) {
		t.Errorf("classification entries = %+v, want %+v", got, want)
	}

	g := loadRecords(t, root)
	var rows []string
	for _, edge := range g.Scorable {
		if !edge.Bearing.Pending || edge.ManuallyScored {
			t.Errorf("the build wrote a judged bearing %+v on %s → %s", edge.Bearing, edge.Source, edge.Target)
		}
		rows = append(rows, edge.Target+" "+edge.Relation+" "+edge.Source)
	}
	for _, edge := range g.References {
		rows = append(rows, edge.Source+" "+kb.RelationReferences+" "+edge.Target)
	}
	for _, edge := range g.Demoted {
		rows = append(rows, edge.Target+" "+kb.RelationDemoted+" "+edge.Source)
	}
	slices.Sort(rows)
	wantRows := []string{
		a + " " + kb.RelationDemoted + " " + c,
		a + " " + kb.RelationSupports + " " + b,
		b + " " + kb.RelationReferences + " " + d,
		b + " " + kb.RelationReferences + " " + e,
		c + " " + kb.RelationSupports + " " + d,
		d + " " + kb.RelationSupports + " " + a,
	}
	if !slices.Equal(rows, wantRows) {
		t.Errorf("edges = %q, want %q", rows, wantRows)
	}

	var ring []string
	for _, f := range report.Findings {
		for _, fd := range f.Fields {
			if f.Check == "stage-D-ring" && fd.Key == "demoted" {
				ring, _ = fd.Value.([]string)
			}
		}
	}
	if want := []string{a + " -> " + c}; !slices.Equal(ring, want) {
		t.Errorf("stage-D-ring demoted = %q, want the walk's one cut %q", ring, want)
	}
}
