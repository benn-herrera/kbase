package main

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/model"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// scopeRepo is a KB whose scope arguments have each case to name: clm-aaaaaa,
// a run's to judge, and clm-bbbbbb, manually scored, both never judged; a
// supports edge between them; a work founding clm-aaaaaa and an axiom.
func scopeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	rigor := 0.5
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": maintenanceEntryPoint,
		maintenanceStampFile:     maintenanceFormatStamp,
		"kb-root/a.md": "[↑ KB](entry-point.md)\n\n# A\n\nThe first claim's own paragraph.<!-- kbase-id: clm-aaaaaa -->\n\n" +
			"The second claim's own paragraph.<!-- kbase-id: clm-bbbbbb -->\n",
	}
	maps.Copy(files, recordFiles(kb.Graph{
		Claims: []kb.ClaimNode{
			{ID: "clm-aaaaaa", Title: "A Claim", Rationale: "Shown."},
			{ID: "clm-bbbbbb", Title: "A Judged Claim", Rigor: &rigor, ManuallyScored: true, Rationale: "Judged by hand."},
		},
		Works:    []kb.WorkNode{{ID: "wrk-aaaaaa", Label: "Work2020", Title: "A Work", Rationale: "Cited."}},
		Axioms:   []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "Closure"}},
		Founds:   []kb.BearingEdge{{Source: "wrk-aaaaaa", Target: "clm-aaaaaa", Bearing: kb.Bearing{Pending: true}}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}}},
	}))
	writeRepo(t, repo, files)
	updateFormatKB(t, repo)
	return repo
}

// refusedAt is the one refusal a result carries, failing t unless the
// result is refused with exactly one item, keyed at key under check.
func refusedAt(t *testing.T, got scoreResult, check, key string) map[string]any {
	t.Helper()
	items := got.doc.items(toolresult.RefusalsKey)
	if got.doc.Outcome != toolresult.Refused || len(items) != 1 || items[0]["check"] != check || items[0]["key"] != key {
		t.Fatalf("outcome %s, refusals %v; want one %s refusal at %s\n%s", got.doc.Outcome, items, check, key, got.stdout)
	}
	if got.dialled != 0 {
		t.Errorf("a refused run dialled %d clients", got.dialled)
	}
	return items[0]
}

// TestScoreRefusesANamedItemNoPassAsks: --node on a work or an axiom and
// --edge on a founds edge name a value no pass asks, and an id or edge that
// does not stand names nothing: each is refused at its argument, nothing
// asked.
func TestScoreRefusesANamedItemNoPassAsks(t *testing.T) {
	repo := scopeRepo(t)
	for _, c := range []struct {
		name        string
		opts        scoreOptions
		check, key  string
		detailOpens string
	}{
		{"a work", scoreOptions{Node: "wrk-aaaaaa"}, toolresult.CheckUsage, "--node", "wrk-aaaaaa is a work"},
		{"an axiom", scoreOptions{Node: "axm-s10000"}, toolresult.CheckUsage, "--node", "axm-s10000 is an axiom"},
		{"a node that does not stand", scoreOptions{Node: "clm-zzzzzz"}, kb.CheckUnknownID, "--node", "no node clm-zzzzzz"},
		{"a founds edge", scoreOptions{Edge: "wrk-aaaaaa,clm-aaaaaa,founds"}, toolresult.CheckUsage, "--edge", "a founds edge's bearing"},
		{"an edge that does not stand", scoreOptions{Edge: "clm-bbbbbb,clm-aaaaaa,supports"}, kb.CheckUnknownID, "--edge", "no edge supports"},
	} {
		t.Run(c.name, func(t *testing.T) {
			item := refusedAt(t, scoreIn(t, repo, t.TempDir(), c.opts, nil), c.check, c.key)
			if detail, _ := item["detail"].(string); len(detail) < len(c.detailOpens) || detail[:len(c.detailOpens)] != c.detailOpens {
				t.Errorf("detail %q, want it to open %q", detail, c.detailOpens)
			}
		})
	}
}

// TestScoreRefusesAScopeArgumentBesideAnother: --pass beside --node or --edge
// is refused — a named item's pass follows from its kind — and so is --resume
// beside an argument naming a scope, and an --edge that names no edge.
func TestScoreRefusesAScopeArgumentBesideAnother(t *testing.T) {
	repo := scopeRepo(t)
	for _, c := range []struct {
		name string
		opts scoreOptions
		key  string
	}{
		{"--pass beside --node", scoreOptions{Pass: query.PassRigor, Node: "clm-aaaaaa"}, "--pass"},
		{"--pass beside --edge", scoreOptions{Pass: query.PassBearing, Edge: "clm-aaaaaa,clm-bbbbbb,supports"}, "--pass"},
		{"--resume beside --node", scoreOptions{Resume: true, Node: "clm-aaaaaa"}, "--node"},
		{"--resume beside --rescore-manual", scoreOptions{Resume: true, RescoreManual: true}, "--rescore-manual"},
		{"an edge of two parts", scoreOptions{Edge: "clm-aaaaaa,clm-bbbbbb"}, "--edge"},
	} {
		t.Run(c.name, func(t *testing.T) {
			refusedAt(t, scoreIn(t, repo, t.TempDir(), c.opts, nil), toolresult.CheckUsage, c.key)
		})
	}
}

// TestScoreRefusesAManuallyScoredItemWithoutRescore: --node naming a manually
// scored claim is refused without --rescore-manual; with it the run asks that
// one node and lands the run's value unmarked.
func TestScoreRefusesAManuallyScoredItemWithoutRescore(t *testing.T) {
	repo := scopeRepo(t)
	refusedAt(t, scoreIn(t, repo, t.TempDir(), scoreOptions{Node: "clm-bbbbbb"}, nil), toolresult.CheckUsage, "--node")
	got := scoreIn(t, repo, t.TempDir(), scoreOptions{Node: "clm-bbbbbb", RescoreManual: true}, func() model.Client { return scoring("none") })
	if got.doc.Outcome != toolresult.Done || got.doc.Values["pass"] != query.PassRigor || !slices.Equal(anyStrings(got.doc.Values["scored"]), []string{"clm-bbbbbb"}) {
		t.Fatalf("the rescoring run: outcome %s, pass %v, scored %v; want the one node scored\n%s",
			got.doc.Outcome, got.doc.Values["pass"], got.doc.Values["scored"], got.stdout)
	}
	if v := rigorOf(t, repo, "clm-bbbbbb"); v != 1.0 {
		t.Errorf("the claim's rigor = %v, want the run's 1.0", v)
	}
	src, err := kbload.Open(filepath.Join(repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	if ix.ManuallyScored(query.Item{Node: &query.ScoringNode{Kind: kb.NodeKindClaim, ID: "clm-bbbbbb"}}) {
		t.Error("the run's value landed manually scored, want the mark false")
	}
}

// TestScoreResumeRefusals: --resume is refused where no run stands, where the
// most recent run finished, and where a build started after it; each refusal
// names --resume, its remedy a fresh score, which asks only what is dirty.
func TestScoreResumeRefusals(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	remedy := atTheCommandLine("start a fresh " + argmark.Call("score", nil) + ", which asks only what is dirty")
	check := func(t *testing.T, got scoreResult, detailOpens string) {
		t.Helper()
		item := refusedAt(t, got, toolresult.CheckUsage, "--resume")
		detail, _ := item["detail"].(string)
		if item["remedy"] != remedy || len(detail) < len(detailOpens) || detail[:len(detailOpens)] != detailOpens {
			t.Errorf("refusal %v, want remedy %q and a detail opening %q", item, remedy, detailOpens)
		}
	}
	check(t, scoreIn(t, repo, state, scoreOptions{Resume: true}, nil), "no score run stands here")

	partial := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none") })
	if partial.doc.Outcome != toolresult.Failed {
		t.Fatalf("the failing run: outcome %s\n%s", partial.doc.Outcome, partial.stdout)
	}
	buildRan(t, state, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	check(t, scoreIn(t, repo, state, scoreOptions{Resume: true}, nil), "a build started at")

	one, finished := scoreRepo(t), t.TempDir()
	if got := scoreIn(t, one, finished, scoreOptions{}, func() model.Client { return scoring("none") }); got.doc.Outcome != toolresult.Done {
		t.Fatalf("the finishing run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	check(t, scoreIn(t, one, finished, scoreOptions{Resume: true}, nil), "the most recent score run")
}

// TestScoreDryRunListsTheDirtyManualValues: a dry run lists what a run would
// ask — every dirty item that is not manually scored — with the count, and
// the dirty manually scored values it would skip, asking nothing.
func TestScoreDryRunListsTheDirtyManualValues(t *testing.T) {
	got := scoreIn(t, scopeRepo(t), t.TempDir(), scoreOptions{DryRun: true}, nil)
	if got.doc.Outcome != toolresult.Done || got.dialled != 0 {
		t.Fatalf("the dry run: outcome %s, %d dials\n%s", got.doc.Outcome, got.dialled, got.stdout)
	}
	ask, skip := anyStrings(got.doc.Values["ask"]), anyStrings(got.doc.Values["skip"])
	if want := []string{"clm-aaaaaa", "supports clm-aaaaaa → clm-bbbbbb"}; !slices.Equal(ask, want) {
		t.Errorf("ask %q, want %q", ask, want)
	}
	if want := []string{"clm-bbbbbb"}; !slices.Equal(skip, want) {
		t.Errorf("skip %q, want the dirty manually scored %q", skip, want)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["total"] != 3 || counts["remaining"] != 2 || counts["skipped"] != 1 {
		t.Errorf("counts %v, want three items: two to ask and one skipped", counts)
	}
}

// TestScoreCancelledAfterALandingLeavesTheDerivedFilesCurrent: a run cancelled
// after its first item answered lands that value with its derivation, so the
// derived files the KB holds are what its records derive — the derivation
// over them would change nothing.
func TestScoreCancelledAfterALandingLeavesTheDerivedFilesCurrent(t *testing.T) {
	t.Setenv("PATH", gitOnlyPath(t))
	repo, state := scoreFixture(t), t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Ctx: ctx}, func() model.Client {
		return (&cancelOnSecond{MockClient: scriptedScoring("flt-overreach: 1"), cancel: cancel}).Client()
	})
	if got.doc.Outcome != toolresult.Cancelled || len(anyStrings(got.doc.Values["scored"])) != 1 {
		t.Fatalf("the cancelled run: outcome %s, scored %v; want cancelled with one value landed\n%s",
			got.doc.Outcome, got.doc.Values["scored"], got.stdout)
	}
	src, err := kbload.Open(filepath.Join(repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	derived, err := index.PlanDerivation(src)
	if err != nil {
		t.Fatal(err)
	}
	var stale []string
	for _, f := range derived.Files {
		stale = append(stale, f.Rel)
	}
	if len(stale) != 0 || len(derived.Removed) != 0 {
		t.Errorf("the derivation over the records would write %q and remove %q, want the derived files current", stale, derived.Removed)
	}
}
