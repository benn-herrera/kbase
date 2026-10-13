package claimgraph

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"kbase/internal/buildrecords"
)

// readAt places ids in reading order, one line apart in one document.
func readAt(ids ...string) map[string]position {
	out := map[string]position{}
	for i, id := range ids {
		out[id] = position{0, i}
	}
	return out
}

func idSet(ids ...string) map[string]bool {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// addsNothing is the answer naming culled as adding nothing to into.
func addsNothing(culled, into string) cullAnswer {
	l := linkOf(culled, into)
	if culled == l.a {
		return cullAnswer{l, buildrecords.VerdictAAddsNothing}
	}
	return cullAnswer{l, buildrecords.VerdictBAddsNothing}
}

func distinctSteps(x, y string) cullAnswer {
	return cullAnswer{linkOf(x, y), buildrecords.VerdictDistinct}
}

func TestCullSurvivorRule(t *testing.T) {
	tied := readAt("p", "x")
	tied["q"] = tied["p"]
	for _, tc := range []struct {
		name    string
		answers []cullAnswer
		prose   map[string]bool
		pos     map[string]position
		targets map[string]string
		roots   map[string]string
	}{
		{"the earliest partner in reading order is the target",
			[]cullAnswer{addsNothing("x", "q"), addsNothing("x", "p")}, idSet("p", "q", "x"), readAt("p", "q", "x"),
			map[string]string{"x": "p"}, map[string]string{"x": "p"}},
		{"partners tied in reading order go by id",
			[]cullAnswer{addsNothing("x", "q"), addsNothing("x", "p")}, idSet("p", "q", "x"), tied,
			map[string]string{"x": "p"}, map[string]string{"x": "p"}},
		{"a block claim named as adding nothing culls the prose claim into it",
			[]cullAnswer{addsNothing("k", "x")}, idSet("x"), readAt("k", "x"),
			map[string]string{"x": "k"}, map[string]string{"x": "k"}},
		{"a prose claim named culls into its non-prose partner",
			[]cullAnswer{addsNothing("x", "e")}, idSet("x"), readAt("x", "e"),
			map[string]string{"x": "e"}, map[string]string{"x": "e"}},
		{"two non-prose claims cull nothing",
			[]cullAnswer{addsNothing("k", "e")}, idSet(), readAt("k", "e"),
			map[string]string{}, map[string]string{}},
		{"distinct culls nothing",
			[]cullAnswer{distinctSteps("x", "y")}, idSet("x", "y"), readAt("x", "y"),
			map[string]string{}, map[string]string{}},
		{"fan-in: many claims share a root",
			[]cullAnswer{addsNothing("x", "r"), addsNothing("y", "r"), addsNothing("z", "x")}, idSet("r", "x", "y", "z"), readAt("r", "x", "y", "z"),
			map[string]string{"x": "r", "y": "r", "z": "x"}, map[string]string{"x": "r", "y": "r", "z": "r"}},
		{"a cycle among targets is broken at its earliest claim, which roots the rest",
			[]cullAnswer{addsNothing("x", "y"), addsNothing("y", "z"), addsNothing("z", "x")}, idSet("x", "y", "z"), readAt("x", "y", "z"),
			map[string]string{"y": "z", "z": "x"}, map[string]string{"y": "x", "z": "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := cullTargets(tc.answers, tc.prose, tc.pos)
			if !reflect.DeepEqual(targets, tc.targets) {
				t.Errorf("targets = %v, want %v", targets, tc.targets)
			}
			if roots := cullRoots(targets); !reflect.DeepEqual(roots, tc.roots) {
				t.Errorf("roots = %v, want %v", roots, tc.roots)
			}
		})
	}
}

// TestCullChainConfirms: in the chain c → b → a only c's root differs from
// its target, so one pair is confirmed, and a yes there culls both into a.
func TestCullChainConfirms(t *testing.T) {
	targets := cullTargets([]cullAnswer{addsNothing("b", "a"), addsNothing("c", "b")}, idSet("a", "b", "c"), readAt("a", "b", "c"))
	roots := cullRoots(targets)
	if got, want := confirmPairs(targets, roots), []link{linkOf("a", "c")}; !slices.Equal(got, want) {
		t.Fatalf("confirmPairs = %v, want %v", got, want)
	}
	final := finalCulls(targets, roots, confirmedCulls([]cullAnswer{addsNothing("c", "a")}, roots))
	if want := map[string]string{"b": "a", "c": "a"}; !reflect.DeepEqual(final, want) {
		t.Errorf("finalCulls = %v, want %v", final, want)
	}
}

// TestCullShielding runs pass 2 over the chain d → c → b → a: c and d are each
// judged against the root a alone, whatever the other's answer.
func TestCullShielding(t *testing.T) {
	targets := cullTargets([]cullAnswer{addsNothing("b", "a"), addsNothing("c", "b"), addsNothing("d", "c")}, idSet("a", "b", "c", "d"), readAt("a", "b", "c", "d"))
	roots := cullRoots(targets)
	if got, want := confirmPairs(targets, roots), []link{linkOf("a", "c"), linkOf("a", "d")}; !slices.Equal(got, want) {
		t.Fatalf("confirmPairs = %v, want %v", got, want)
	}
	for _, tc := range []struct {
		name  string
		pass2 []cullAnswer
		want  map[string]string
	}{
		{"a claim past a kept claim still culls into the root",
			[]cullAnswer{distinctSteps("c", "a"), addsNothing("d", "a")}, map[string]string{"b": "a", "d": "a"}},
		{"a no to the root keeps a claim that said yes to its partner",
			[]cullAnswer{addsNothing("c", "a"), distinctSteps("d", "a")}, map[string]string{"b": "a", "c": "a"}},
		{"naming the root as adding nothing keeps the claim and the root",
			[]cullAnswer{addsNothing("a", "c"), addsNothing("a", "d")}, map[string]string{"b": "a"}},
		{"an unanswered pair keeps its claim",
			nil, map[string]string{"b": "a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := finalCulls(targets, roots, confirmedCulls(tc.pass2, roots)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("finalCulls = %v, want %v", got, tc.want)
			}
		})
	}
}

func hl(x, y, harvest string) harvestedLink { return harvestedLink{linkOf(x, y), harvest} }

func cl(x, y, harvest string) buildrecords.CullLink {
	l := linkOf(x, y)
	return buildrecords.CullLink{A: l.a, B: l.b, Harvest: harvest}
}

func TestPlanCull(t *testing.T) {
	held := idSet("a", "b", "p", "q", "r", "x", "y")
	holds := func(id string) bool { return held[id] }
	for _, tc := range []struct {
		name    string
		links   []harvestedLink
		into    map[string]string
		want    CullPlan
		wantErr bool
	}{
		{"a link is re-pointed on both ends",
			[]harvestedLink{hl("x", "y", harvestUnmarked)}, map[string]string{"x": "p", "y": "q"},
			CullPlan{Culls: []string{"x", "y"}, Links: []buildrecords.CullLink{cl("p", "q", harvestCulled)}}, false},
		{"a self-link is dropped",
			[]harvestedLink{hl("x", "p", harvestAdjacent)}, map[string]string{"x": "p"},
			CullPlan{Culls: []string{"x"}}, false},
		{"re-pointed links meeting on one pair are one link",
			[]harvestedLink{hl("x", "r", harvestUnmarked), hl("y", "r", harvestAdjacent)}, map[string]string{"x": "p", "y": "p"},
			CullPlan{Culls: []string{"x", "y"}, Links: []buildrecords.CullLink{cl("p", "r", harvestCulled)}}, false},
		{"an untouched link keeps its harvest over a re-pointed one",
			[]harvestedLink{hl("x", "r", harvestUnmarked), hl("p", "r", harvestAdjacent)}, map[string]string{"x": "p"},
			CullPlan{Culls: []string{"x"}, Links: []buildrecords.CullLink{cl("p", "r", harvestAdjacent)}}, false},
		{"an untouched marked pair stands and a link re-pointed onto it is dropped",
			[]harvestedLink{hl("p", "r", harvestRef), hl("x", "r", harvestUnmarked)}, map[string]string{"x": "p"},
			CullPlan{Culls: []string{"x"}}, false},
		{"a marked link with a culled end comes out inferred, harvest culled",
			[]harvestedLink{hl("x", "r", harvestNamed)}, map[string]string{"x": "p"},
			CullPlan{Culls: []string{"x"}, Links: []buildrecords.CullLink{cl("p", "r", harvestCulled)}}, false},
		{"unmarked takes precedence over adjacent on a coinciding pair",
			[]harvestedLink{hl("a", "b", harvestAdjacent), hl("b", "a", harvestUnmarked)}, nil,
			CullPlan{Links: []buildrecords.CullLink{cl("a", "b", harvestUnmarked)}}, false},
		{"marked takes precedence over unmarked on a coinciding pair",
			[]harvestedLink{hl("a", "b", harvestUnmarked), hl("a", "b", harvestRef)}, nil,
			CullPlan{}, false},
		{"an into value that is itself culled is an error",
			[]harvestedLink{hl("x", "r", harvestUnmarked)}, map[string]string{"x": "y", "y": "p"},
			CullPlan{}, true},
		{"an id the graph does not hold is an error",
			[]harvestedLink{hl("x", "r", harvestUnmarked)}, map[string]string{"x": "gone"},
			CullPlan{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PlanCull(tc.links, tc.into, holds)
			if (err != nil) != tc.wantErr {
				t.Fatalf("PlanCull error = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PlanCull = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestCullOrderIndependent runs the whole pure cull — targets, roots, the
// confirm pairs, the final culls and the plan — over the same answers and
// links in shuffled orders, and requires one outcome.
func TestCullOrderIndependent(t *testing.T) {
	pos := readAt("k", "a", "b", "c", "d", "x", "y", "z", "e", "f")
	prose := idSet("a", "b", "c", "d", "x", "y", "z", "f")
	pass1 := []cullAnswer{
		addsNothing("b", "a"), addsNothing("c", "b"), addsNothing("d", "c"), addsNothing("c", "a"),
		addsNothing("k", "f"),
		addsNothing("x", "y"), addsNothing("y", "z"), addsNothing("z", "x"),
		distinctSteps("a", "k"), addsNothing("e", "d"),
	}
	pass2 := []cullAnswer{distinctSteps("d", "a"), addsNothing("y", "x"), addsNothing("a", "z")}
	links := []harvestedLink{
		hl("a", "b", harvestUnmarked), hl("b", "c", harvestAdjacent), hl("c", "d", harvestUnmarked),
		hl("a", "c", harvestRef), hl("k", "f", harvestAdjacent), hl("x", "y", harvestUnmarked),
		hl("y", "z", harvestAdjacent), hl("z", "x", harvestNamed), hl("a", "k", harvestUnmarked),
		hl("d", "e", harvestUnmarked), hl("b", "e", harvestAdjacent), hl("d", "b", harvestUnmarked),
		hl("b", "e", harvestUnmarked),
	}
	held := idSet("k", "a", "b", "c", "d", "x", "y", "z", "e", "f")
	run := func(pass1, pass2 []cullAnswer, links []harvestedLink) (map[string]string, CullPlan) {
		targets := cullTargets(pass1, prose, pos)
		roots := cullRoots(targets)
		final := finalCulls(targets, roots, confirmedCulls(pass2, roots))
		plan, err := PlanCull(links, final, func(id string) bool { return held[id] })
		if err != nil {
			t.Fatalf("PlanCull: %v", err)
		}
		return final, plan
	}
	wantFinal, wantPlan := run(pass1, pass2, links)
	if len(wantFinal) == 0 || len(wantPlan.Links) == 0 {
		t.Fatalf("the fixture culls nothing or plans no link: %v %+v", wantFinal, wantPlan)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 50 {
		p1, p2, ls := slices.Clone(pass1), slices.Clone(pass2), slices.Clone(links)
		r.Shuffle(len(p1), func(i, j int) { p1[i], p1[j] = p1[j], p1[i] })
		r.Shuffle(len(p2), func(i, j int) { p2[i], p2[j] = p2[j], p2[i] })
		r.Shuffle(len(ls), func(i, j int) { ls[i], ls[j] = ls[j], ls[i] })
		final, plan := run(p1, p2, ls)
		if !reflect.DeepEqual(final, wantFinal) {
			t.Fatalf("shuffle %d: finalCulls = %v, want %v", i, final, wantFinal)
		}
		if !reflect.DeepEqual(plan, wantPlan) {
			t.Fatalf("shuffle %d: CullPlan = %+v, want %+v", i, plan, wantPlan)
		}
	}
}
