package claimgraph

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/result"
)

func graphOf(nodes ...ClaimNode) *AuthoredGraph {
	g := &AuthoredGraph{Nodes: nodes, byID: map[string]int{}}
	for i, n := range nodes {
		g.byID[n.ID] = i
	}
	return g
}

func targetsOf(p unmarkedPlan, source string) []string {
	var out []string
	for _, x := range p.pairs {
		if x.source == source {
			out = append(out, x.target)
		}
	}
	return out
}

// shortlistPaper is two papers' claims: p holds a directory s of two
// documents and a directory t of one, q a document alone. clm-z sits before
// clm-s in their document though its id sorts after; clm-e is an equation
// node. clm-s and clm-z state the same thing, clm-q most of it, clm-c1 a
// word of it, and the rest nothing of it.
func shortlistPaper() ([]ClaimNode, map[string]string, map[string]position) {
	nodes := []ClaimNode{
		{ID: "clm-z", Document: "p/s/a.md"}, {ID: "clm-s", Document: "p/s/a.md"},
		{ID: "clm-e", Document: "p/s/a.md", Equation: "eq:e"}, {ID: "clm-a1", Document: "p/s/a.md"},
		{ID: "clm-b1", Document: "p/s/b.md"}, {ID: "clm-c1", Document: "p/t/c.md"}, {ID: "clm-q", Document: "q/x.md"},
	}
	statements := map[string]string{
		"clm-z": "contraction operator bound holds", "clm-s": "contraction operator bound holds",
		"clm-e": "zebra", "clm-a1": "gadgets alpha", "clm-b1": "gizmos beta",
		"clm-c1": "contraction widgets", "clm-q": "contraction operator bound",
	}
	pos := map[string]position{
		"clm-z": {1, 5}, "clm-s": {1, 10}, "clm-e": {1, 20}, "clm-a1": {1, 30},
		"clm-b1": {2, 5}, "clm-c1": {3, 5}, "clm-q": {4, 5},
	}
	return nodes, statements, pos
}

func checkPicks(t *testing.T, name string, p unmarkedPlan, want map[string]unmarkedPicks) {
	t.Helper()
	for source, w := range want {
		got := p.picks[source]
		if !slices.Equal(got.proximity, w.proximity) || !slices.Equal(got.wildcard, w.wildcard) || !slices.Equal(got.adjacent, w.adjacent) {
			t.Errorf("%s: %s picks %+v, want %+v", name, source, got, w)
		}
		if targets := targetsOf(p, source); !slices.Equal(targets, slices.Concat(w.proximity, w.wildcard)) {
			t.Errorf("%s: %s is planned %q, want its proximity picks then its wildcards", name, source, targets)
		}
	}
}

// TestPlanUnmarkedProximityThenWildcards: a source's proximity slots take its
// nearest targets by locality, then by places apart in reading order, a tie
// to the earlier claim whatever the ids; another paper is never near. Its
// wildcards take the best by cosine less the proximity picks, so another
// paper is reached there. A minted equation is a target and never a source,
// and each source is planned apart, so a pair both ends pick is planned both
// ways.
func TestPlanUnmarkedProximityThenWildcards(t *testing.T) {
	nodes, statements, pos := shortlistPaper()
	p := planUnmarked(nodes, statements, pos, nil, nil)
	if !slices.Equal(p.sources, []string{"clm-a1", "clm-b1", "clm-c1", "clm-q", "clm-s", "clm-z"}) {
		t.Errorf("sources = %q, want every node but the equation", p.sources)
	}
	checkPicks(t, "unexcluded", p, map[string]unmarkedPicks{
		"clm-s":  {proximity: []string{"clm-z", "clm-e", "clm-a1"}, wildcard: []string{"clm-q", "clm-c1"}},
		"clm-b1": {proximity: []string{"clm-a1", "clm-e", "clm-s"}, wildcard: []string{"clm-c1", "clm-q"}},
		"clm-q":  {wildcard: []string{"clm-s", "clm-z"}},
	})
	for _, both := range []pair{{"clm-s", "clm-q"}, {"clm-s", "clm-z"}} {
		if !slices.Contains(p.pairs, both) || !slices.Contains(p.pairs, pair{both.target, both.source}) {
			t.Errorf("%v is not planned both ways: %v", both, p.pairs)
		}
	}
}

// TestPlanUnmarkedExclusions: a marked or adjacent link is out of both its
// ends' pools, and a source's own equation out of its own; a proximity slot
// the source's paper cannot fill stays empty, though another paper's claim
// stands in the pool. Each adjacency link is listed under both its ends.
func TestPlanUnmarkedExclusions(t *testing.T) {
	nodes, statements, pos := shortlistPaper()
	adjacent := linkOf("clm-s", "clm-a1")
	exclude := map[link]bool{linkOf("clm-z", "clm-s"): true, adjacent: true}
	p := planUnmarked(nodes, statements, pos, exclude, map[pair]bool{{"clm-s", "clm-e"}: true})
	p.addAdjacent([]link{adjacent})
	checkPicks(t, "excluded", p, map[string]unmarkedPicks{
		"clm-s":  {proximity: []string{"clm-b1", "clm-c1"}, wildcard: []string{"clm-q"}, adjacent: []string{"clm-a1"}},
		"clm-z":  {proximity: []string{"clm-e", "clm-a1", "clm-b1"}, wildcard: []string{"clm-q", "clm-c1"}},
		"clm-a1": {proximity: []string{"clm-e", "clm-z", "clm-b1"}, wildcard: []string{"clm-c1", "clm-q"}, adjacent: []string{"clm-s"}},
	})
	if p.adjacent != 1 {
		t.Errorf("adjacent = %d, want the one link", p.adjacent)
	}
}

// TestAYesFromEitherSideIsOneLink: a pair answered yes both ways, or yes one
// way and no the other, is one link; a pair answered no or defaulted both
// ways is none.
func TestAYesFromEitherSideIsOneLink(t *testing.T) {
	yes, no := asks.LetterPoints, asks.LetterDoesNotPoint
	rec := buildrecords.UnmarkedRecord{Pairs: []buildrecords.CandidateEntry{
		{Source: "clm-b", Target: "clm-a", Letter: &yes}, {Source: "clm-a", Target: "clm-b", Letter: &yes},
		{Source: "clm-d", Target: "clm-c", Letter: &yes}, {Source: "clm-c", Target: "clm-d", Letter: &no},
		{Source: "clm-e", Target: "clm-f", Letter: &no}, {Source: "clm-f", Target: "clm-e", Letter: &no},
		{Source: "clm-g", Target: "clm-h"}, {Source: "clm-h", Target: "clm-g"},
	}}
	if got := unmarkedLinks(rec); !slices.Equal(got, []link{{"clm-a", "clm-b"}, {"clm-c", "clm-d"}}) {
		t.Errorf("links = %v, want one per pair a yes reached from either side", got)
	}
}

func TestShortlistTerms(t *testing.T) {
	if got := shortlistTerms("The contraction bound holds for the operator $`\\lambda_{c}`$."); !slices.Equal(got, []string{"contraction", "bound", "holds", "operator", "lambda", `m:\lambda_c`}) {
		t.Errorf("terms = %q, want the folded words and the maths symbol with its subscript unbraced", got)
	}
}

func TestPySumIsCompensated(t *testing.T) {
	if got := pySum([]float64{1e100, 1, -1e100}); got != 1 {
		t.Errorf("pySum = %v, want 1 as Python's sum gives it", got)
	}
	if got := pySum(nil); got != 0 {
		t.Errorf("pySum of nothing = %v", got)
	}
}

func TestUnmarkedRecordRoundTrip(t *testing.T) {
	repo := t.TempDir()
	if err := WriteUnmarked(repo, buildrecords.UnmarkedRecord{}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(repo, buildrecords.UnmarkedFile))
	if !strings.HasSuffix(string(b), "\nplanned: null\npairs: []\n") {
		t.Errorf("the empty record = %q, want no plan and no pairs", b)
	}
	empty, ok, err := buildrecords.ReadUnmarked(recordsAt(repo))
	if err != nil || !ok || empty.Planned != nil || len(empty.Pairs) != 0 || empty.About != unmarkedAbout {
		t.Fatalf("read back = %+v, %t, %v", empty, ok, err)
	}

	a, b2 := asks.LetterPoints, asks.LetterDoesNotPoint
	plan := []buildrecords.PlannedPair{{"clm-b", "clm-c"}, {"clm-a", "clm-c"}}
	rec := buildrecords.UnmarkedRecord{Planned: &plan, Pairs: []buildrecords.CandidateEntry{
		{Source: "clm-b", Target: "clm-c", Offered: []string{a, b2}, Letter: &b2, Outcome: string(asks.Reasked)},
		{Source: "clm-a", Target: "clm-c", Offered: []string{a, b2}, Letter: &a, Outcome: string(asks.Answered)},
	}}
	if err := WriteUnmarked(repo, rec); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(repo, buildrecords.UnmarkedFile))
	if !strings.Contains(string(b), "\nplanned:\n  - - clm-b\n    - clm-c\n  - - clm-a\n    - clm-c\npairs:\n") {
		t.Errorf("the plan is not written in its order:\n%s", b)
	}
	got, _, err := buildrecords.ReadUnmarked(recordsAt(repo))
	if err != nil || got.Planned == nil || !slices.Equal(*got.Planned, plan) || len(got.Pairs) != 2 ||
		got.Pairs[0].Source != "clm-a" || *got.Pairs[0].Letter != a || got.Pairs[1].Outcome != string(asks.Reasked) {
		t.Errorf("read back = %+v, %v; want the plan as written and the pairs sorted", got, err)
	}
	if len(unanswered(got)) != 0 || !slices.Equal(unmarkedFound(got), []pair{{"clm-a", "clm-c"}}) {
		t.Errorf("unanswered %v, found %v", unanswered(got), unmarkedFound(got))
	}
}

// scriptedLetters answers each group->item from its script, one reply per
// ask in order; an item whose script runs out never completes.
type scriptedLetters struct {
	mu      sync.Mutex
	replies map[string][]string
	asked   map[string]int
}

func (s *scriptedLetters) read(_ context.Context, q asks.LetterQuestion) (asks.Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := q.Group + "->" + q.Item
	if s.asked == nil {
		s.asked = map[string]int{}
	}
	s.asked[key]++
	if s.asked[key] > len(s.replies[key]) {
		return asks.Reply{}, asks.IncompleteError{Subject: key, Detail: "no reply arrived"}
	}
	return asks.Reply{Text: s.replies[key][s.asked[key]-1]}, nil
}

func TestAskPlannedThroughTheLetterSeam(t *testing.T) {
	g := graphOf(
		ClaimNode{ID: "clm-s1", Document: "v/a.md", Title: "S1"}, ClaimNode{ID: "clm-s2", Document: "v/b.md", Title: "S2"},
		ClaimNode{ID: "clm-t1", Document: "v/a.md", Title: "T1"}, ClaimNode{ID: "clm-t2", Document: "v/b.md", Title: "T2"},
		ClaimNode{ID: "clm-t3", Document: "w/c.md", Title: "T3"}, ClaimNode{ID: "clm-t4", Document: "v/a.md", Title: "T4"},
	)
	plan := []buildrecords.PlannedPair{{"clm-s1", "clm-t1"}, {"clm-s1", "clm-t2"}, {"clm-s1", "clm-t3"}, {"clm-s2", "clm-t4"}}
	repo := t.TempDir()
	rec := buildrecords.UnmarkedRecord{Planned: &plan}
	if err := WriteUnmarked(repo, rec); err != nil {
		t.Fatal(err)
	}
	statement := func(n ClaimNode) string { return n.Title + " states it." }
	body := func(document string) string { return "the leaf " + document }
	var fallbacks []result.Item
	var units []string
	total := 0
	progress := Progress{
		Units:    func(n int) error { total = n; return nil },
		Unit:     func(name string) error { units = append(units, name); return nil },
		Fallback: func(it result.Item) error { fallbacks = append(fallbacks, it); return nil },
	}
	reader := &scriptedLetters{replies: map[string][]string{
		"clm-s1->clm-t1": {"A"},
		"clm-s1->clm-t2": {"It does not.", "B"},
		"clm-s1->clm-t3": {"?", "maybe"},
	}}
	_, held, groups, err := askPlanned(context.Background(), repo, rec, g, statement, body, Options{Reader: reader.read, Progress: progress})
	var incomplete asks.IncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("askPlanned = %v, want the call that never completed", err)
	}
	var s *stop
	if !errors.As(askStop(err), &s) || s.check != "inference-failed" {
		t.Errorf("the stage's stop = %v, want inference-failed", askStop(err))
	}
	if held != 0 || len(groups) != 1 || total != 2 || !slices.Equal(units, []string{"clm-s1"}) {
		t.Errorf("held %d, %d groups, units %q of %d; want s1's group done of two", held, len(groups), units, total)
	}
	landed, _, _ := buildrecords.ReadUnmarked(recordsAt(repo))
	want := map[string]string{"clm-t1": "A answered", "clm-t2": "B re-asked", "clm-t3": "- defaulted"}
	if len(landed.Pairs) != 3 {
		t.Fatalf("landed %+v, want s1's group whole and s2's nothing", landed.Pairs)
	}
	for _, e := range landed.Pairs {
		l := "-"
		if e.Letter != nil {
			l = *e.Letter
		}
		if got := l + " " + e.Outcome; e.Source != "clm-s1" || got != want[e.Target] || !slices.Equal(e.Offered, []string{"A", "B"}) {
			t.Errorf("%s -> %s = %s offered %q, want %s offered A and B", e.Source, e.Target, got, e.Offered, want[e.Target])
		}
	}
	if len(fallbacks) != 1 || fallbacks[0].Check != checkDefaulted || fallbacks[0].Path != "v/a.md" ||
		!strings.Contains(fallbacks[0].Detail, "clm-s1 -> clm-t3") || !strings.Contains(fallbacks[0].Detail, "no offered letter twice") {
		t.Errorf("fallbacks = %+v, want the defaulted pair named with its cause under its source's document", fallbacks)
	}

	again := &scriptedLetters{replies: map[string][]string{"clm-s2->clm-t4": {"A"}}}
	done, held, groups, err := askPlanned(context.Background(), repo, landed, g, statement, body, Options{Reader: again.read})
	if err != nil || held != 3 || len(groups) != 1 || len(again.asked) != 1 {
		t.Fatalf("the resume = held %d, %d groups, asked %v, %v; want only the pair the record lacks", held, len(groups), again.asked, err)
	}
	if got := unmarkedFound(done); !slices.Equal(got, []pair{{"clm-s1", "clm-t1"}, {"clm-s2", "clm-t4"}}) {
		t.Errorf("found = %v, want the two As and neither the B nor the default", got)
	}
	findings := unmarkedFindings(unmarkedPlan{sources: []string{"clm-s1", "clm-s2"}, pairs: []pair{{"clm-s1", "clm-t1"}, {"clm-s1", "clm-t2"}, {"clm-s1", "clm-t3"}, {"clm-s2", "clm-t4"}}}, done, g, held, groups)
	if f := findings[2].Record(); f[2].Value != 2 || f[3].Value.(result.Record)[0].Value != 1 || f[3].Value.(result.Record)[1].Value != 1 {
		t.Errorf("stage-U-yeses = %v, want one in the same document and one in the same directory", f)
	}
	if f := findings[3].Record(); !slices.Equal(f[2].Value.([]string), []string{"clm-s1 -> clm-t3"}) {
		t.Errorf("stage-U-defaulted = %v", f)
	}
}

// TestAnInferredLinkIsAskedThreeWay: an inferred claim-to-claim pair is
// offered all three relations and drafted mention, as a pair reached by a
// mark is, keeping its harvest; classification asks it with the marked pairs,
// and with no reader it is recorded drafted.
func TestAnInferredLinkIsAskedThreeWay(t *testing.T) {
	tree := &Tree{Documents: map[string]Document{
		"v/x.md": {Path: "v/x.md", Text: "We use [the bound](y.md).\n"},
		"v/y.md": {Path: "v/y.md", Text: "The bound holds.\n"},
	}}
	g := graphOf(
		ClaimNode{ID: "clm-a", Document: "v/x.md", Title: "A"},
		ClaimNode{ID: "clm-b", Document: "v/x.md", Title: "B"},
		ClaimNode{ID: "clm-c", Document: "v/y.md", Title: "C"},
		ClaimNode{ID: "clm-d", Document: "v/y.md", Title: "D"},
	)
	inv := &Inventory{Anchors: []Anchor{{Document: "v/x.md", Line: 0, Href: "y.md", Target: "v/y.md"}}}
	inferred := pair{"clm-d", "clm-c"}
	a := narrow(tree, g, inv, buildrecords.NodePassRecord{}, []inferredPair{{inferred, harvestAdjacent}})

	all := []Relation{SupportedBy, InSupportOf, MentionedBy}
	want := []pair{{"clm-a", "clm-c"}, {"clm-a", "clm-d"}, {"clm-b", "clm-c"}, {"clm-b", "clm-d"}, inferred}
	if len(a.candidates) != len(want) {
		t.Fatalf("candidates = %+v, want %v", a.candidates, want)
	}
	for _, c := range a.candidates {
		if !slices.Contains(want, c.pair()) || !slices.Equal(c.offered, all) || c.draft != MentionedBy {
			t.Errorf("%v offered %v drafted %s, want one of %v offered all three, drafted mention", c.pair(), c.offered, c.draft, want)
		}
		if c.pair() == inferred && !slices.Equal(c.harvests, []string{harvestAdjacent}) {
			t.Errorf("the inferred pair's harvests = %q, want its own", c.harvests)
		}
	}

	statement := func(n ClaimNode) string { return n.Title }
	run := func(reader *letters) map[pair]buildrecords.CandidateEntry {
		t.Helper()
		repo := t.TempDir()
		if err := WriteClassification(repo, buildrecords.ClassificationRecord{}); err != nil {
			t.Fatal(err)
		}
		opts := Options{}
		if reader != nil {
			opts.Reader = reader.read
		}
		if _, err := classify(context.Background(), repo, a.candidates, statement, opts); err != nil {
			t.Fatal(err)
		}
		rec, _, err := buildrecords.ReadClassification(recordsAt(repo))
		if err != nil {
			t.Fatal(err)
		}
		entries := map[pair]buildrecords.CandidateEntry{}
		for _, e := range rec.Candidates {
			entries[pair{e.Source, e.Target}] = e
		}
		if len(entries) != len(want) {
			t.Fatalf("record = %+v, want %d candidates", rec.Candidates, len(want))
		}
		return entries
	}

	mention := asks.LetterMention
	reader := &letters{by: map[string]string{}}
	for _, p := range want {
		reader.by[p.source+"->"+p.target] = mention
	}
	for p, e := range run(reader) {
		if e.Letter == nil || *e.Letter != mention || len(e.Offered) != len(all) || e.Outcome != string(asks.Answered) {
			t.Errorf("%v records %+v, want asked three letters and answered C", p, e)
		}
	}
	if len(reader.asked) != len(want) {
		t.Errorf("asked %q, want every pair once", reader.asked)
	}
	if e := run(nil)[inferred]; e.Letter != nil || e.Outcome != ClassifyDrafted {
		t.Errorf("with no reader the inferred pair records %+v, want drafted", e)
	}
}

func TestUnmarkedLocality(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{"p/s/a.md", "p/s/a.md", "same document"},
		{"p/s/a.md", "p/s/b.md", "same directory"},
		{"p/s/a.md", "p/t/b.md", "same paper"},
		{"p/s/a.md", "q/s/a.md", "cross paper"},
		{"index.md", "entry-point.md", "cross paper"},
	} {
		if got := unmarkedLocality(tc.a, tc.b); got != tc.want {
			t.Errorf("locality(%s, %s) = %s, want %s", tc.a, tc.b, got, tc.want)
		}
	}
}
