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

// TestPlanUnmarked: a source's pool is every other node less the pairs
// already candidates either way round and the equations it states, ranked
// best first with ties to the lower id; a minted equation is a target and
// never a source.
func TestPlanUnmarked(t *testing.T) {
	nodes := []ClaimNode{{ID: "clm-a"}, {ID: "clm-b"}, {ID: "clm-c"}, {ID: "clm-d"}, {ID: "clm-e", Equation: "eq:e"}}
	statements := map[string]string{
		"clm-a": "The contraction bound holds for the operator $`\\lambda_{c}`$.",
		"clm-b": "The contraction bound for the operator.",
		"clm-c": "The contraction bound for the operator.",
		"clm-d": "Unrelated widgets gadgets.",
		"clm-e": "``` math\n\\lambda_c = 1\n```",
	}
	p := planUnmarked(nodes, statements, []pair{{"clm-d", "clm-a"}}, map[pair]bool{{"clm-a", "clm-e"}: true})
	if !slices.Equal(p.sources, []string{"clm-a", "clm-b", "clm-c", "clm-d"}) {
		t.Errorf("sources = %q, want every node but the equation", p.sources)
	}
	for source, want := range map[string][]string{
		"clm-a": {"clm-b", "clm-c"},
		"clm-b": {"clm-c", "clm-a", "clm-d", "clm-e"},
		"clm-d": {"clm-b", "clm-c", "clm-e"},
	} {
		if got := targetsOf(p, source); !slices.Equal(got, want) {
			t.Errorf("%s's shortlist = %q, want %q", source, got, want)
		}
	}
	if p.excluded != 2 || p.ownEquations != 1 {
		t.Errorf("excluded %d as candidates and %d as own equations, want 2 and 1", p.excluded, p.ownEquations)
	}
	if got := shortlistTerms(statements["clm-a"]); !slices.Equal(got, []string{"contraction", "bound", "holds", "operator", "lambda", `m:\lambda_c`}) {
		t.Errorf("terms = %q, want the folded words and the maths symbol with its subscript unbraced", got)
	}

	var many []ClaimNode
	same := map[string]string{}
	for _, id := range []string{"clm-1", "clm-2", "clm-3", "clm-4", "clm-5", "clm-6", "clm-7"} {
		many = append(many, ClaimNode{ID: id})
		same[id] = "alpha beta gamma"
	}
	if got := targetsOf(planUnmarked(many, same, nil, nil), "clm-1"); !slices.Equal(got, []string{"clm-2", "clm-3", "clm-4", "clm-5", "clm-6"}) {
		t.Errorf("a pool of six ties = %q, want the first %d by id", got, shortlistK)
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
		{Source: "clm-b", Target: "clm-c", Offered: []string{a, b2}, Letter: &b2, Outcome: ClassifyReasked},
		{Source: "clm-a", Target: "clm-c", Offered: []string{a, b2}, Letter: &a, Outcome: ClassifyAnswered},
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
		got.Pairs[0].Source != "clm-a" || *got.Pairs[0].Letter != a || got.Pairs[1].Outcome != ClassifyReasked {
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
