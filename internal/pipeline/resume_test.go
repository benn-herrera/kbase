package pipeline

import (
	"errors"
	"testing"
)

// The synthetic chain every scan test runs against: three stages, shaped like
// the real one (a serial whole-corpus artifact, a per-unit fan-out, a
// roll-up), with each stage consuming the one before it. Stage shapes are
// what these tests need; no real stage implementations exist yet, and none
// would make the scan any more exercised.
const (
	relSurvey   = "1-survey/survey.json"
	relTreePlan = "2-treeplan/treeplan.json"
	relLeafA    = "3-leaves/a.md"
	relLeafB    = "3-leaves/b.md"
)

var sourceInput = Input{Name: "source", Hash: HashBytes([]byte("corpus bytes"))}

// fixedUnits is the OwedArtifactResolver for a stage whose description is known up
// front. Most test chains are this; the ones exercising the dynamic chain use
// a resolver that reads the store.
func fixedUnits(units ...OwedArtifact) OwedArtifactResolver {
	return func() ([]OwedArtifact, error) { return units, nil }
}

func testChain() StageChain {
	return StageChain{
		{Name: "survey", Units: fixedUnits(
			OwedArtifact{Path: relSurvey, Inputs: []Input{sourceInput}},
		)},
		{Name: "treeplan", Units: fixedUnits(
			OwedArtifact{Path: relTreePlan, Inputs: []Input{sourceInput}, Upstreams: []string{relSurvey}},
		)},
		{Name: "leaves", Units: fixedUnits(
			OwedArtifact{Path: relLeafA, Inputs: []Input{sourceInput}, Upstreams: []string{relTreePlan}},
			OwedArtifact{Path: relLeafB, Inputs: []Input{sourceInput}, Upstreams: []string{relTreePlan}},
		)},
	}
}

// build writes the named units of the chain, deriving each unit's inputs the
// same way the scan will — which is the point of ResolveInputs being one
// function rather than two.
func build(t *testing.T, s *ArtifactStore, chain StageChain, paths ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, p := range paths {
		want[p] = true
	}
	for _, stage := range chain {
		units, err := stage.Units()
		if err != nil {
			t.Fatalf("describe %s: %v", stage.Name, err)
		}
		for _, u := range units {
			if !want[u.Path] {
				continue
			}
			inputs, err := s.resolveInputs(u)
			if err != nil {
				t.Fatalf("resolveInputs(%s): %v", u.Path, err)
			}
			put(t, s, u.Path, "content of "+u.Path, inputs...)
		}
	}
}

func scan(t *testing.T, s *ArtifactStore, chain StageChain, mode Mode) ResumeScanResult {
	t.Helper()
	res, err := s.ResumeScan(chain, mode)
	if err != nil {
		t.Fatalf("ResumeScan: %v", err)
	}
	return res
}

func TestScanEmptyStore(t *testing.T) {
	s, _ := newArtifactStore(t)
	res := scan(t, s, testChain(), ModeResume)

	if res.ResumeStage != 0 || res.StageName != "survey" {
		t.Fatalf("resume at stage %d (%q), want the first stage", res.ResumeStage, res.StageName)
	}
	if res.Reused != 0 || res.Redo != 1 {
		t.Errorf("reused %d redo %d, want 0 and 1", res.Reused, res.Redo)
	}
	if len(res.Verdicts) != 1 || res.Verdicts[0].Verdict != VerdictAbsent {
		t.Errorf("verdicts %+v, want one absent", res.Verdicts)
	}
	if res.Complete() {
		t.Error("an empty store reported a complete chain")
	}
}

// TestScanDeepestValidPrefix is the resume boundary itself: two stages
// proven, a third half done, so the restart point is the third stage and only
// its unfinished unit is redone.
func TestScanDeepestValidPrefix(t *testing.T) {
	s, lg := newArtifactStore(t)
	chain := testChain()
	build(t, s, chain, relSurvey, relTreePlan, relLeafA)
	lg.Reset() // the assertions below are about the scan, not the setup

	res := scan(t, s, chain, ModeResume)
	if res.ResumeStage != 2 || res.StageName != "leaves" {
		t.Fatalf("resume at stage %d (%q), want stage 2 (leaves)", res.ResumeStage, res.StageName)
	}
	if res.Reused != 3 || res.Redo != 1 {
		t.Errorf("reused %d redo %d, want 3 and 1", res.Reused, res.Redo)
	}
	byPath := map[string]OwedArtifactVerdict{}
	for _, v := range res.Verdicts {
		byPath[v.Path] = v
	}
	if got := byPath[relLeafA].Verdict; got != VerdictValid {
		t.Errorf("%s is %s, want %s", relLeafA, got, VerdictValid)
	}
	if got := byPath[relLeafB].Verdict; got != VerdictAbsent {
		t.Errorf("%s is %s, want %s", relLeafB, got, VerdictAbsent)
	}

	// Forensics: one summary record with the counts, one debug record per
	// unit that will be redone, carrying why.
	if !lg.Has(t, "info", "reused", 3) || !lg.Has(t, "info", "redo", 1) {
		t.Error("the scan logged no summary inventory")
	}
	if n := lg.Count("debug", "path", relLeafB); n != 1 {
		t.Errorf("%d debug records for the redone unit, want 1", n)
	}
	if lg.Count("debug", "path", relLeafA) != 0 {
		t.Error("a reused unit produced a redo record")
	}
}

func TestScanCompleteChain(t *testing.T) {
	s, lg := newArtifactStore(t)
	chain := testChain()
	build(t, s, chain, relSurvey, relTreePlan, relLeafA, relLeafB)

	res := scan(t, s, chain, ModeResume)
	if !res.Complete() {
		t.Fatalf("resume at stage %d (%q), want a complete chain", res.ResumeStage, res.StageName)
	}
	if res.Reused != 4 || res.Redo != 0 {
		t.Errorf("reused %d redo %d, want 4 and 0", res.Reused, res.Redo)
	}
	if len(res.Verdicts) != 0 {
		t.Errorf("a complete chain returned %d verdicts", len(res.Verdicts))
	}
	if !lg.Has(t, "info", "reused", 4) {
		t.Error("the scan logged no summary inventory")
	}
}

// TestScanUpstreamChangeInvalidatesDownstream is what makes the chain a
// chain: rewriting an upstream artifact changes its output hash, and every
// unit derived from it loses its proof without anyone propagating an
// invalidation.
func TestScanUpstreamChangeInvalidatesDownstream(t *testing.T) {
	s, _ := newArtifactStore(t)
	chain := testChain()
	build(t, s, chain, relSurvey, relTreePlan, relLeafA, relLeafB)

	// The survey stage re-runs and produces different bytes from the same
	// inputs. It stays valid itself; everything downstream must not.
	put(t, s, relSurvey, "a different survey", sourceInput)

	res := scan(t, s, chain, ModeResume)
	if res.ResumeStage != 1 || res.StageName != "treeplan" {
		t.Fatalf("resume at stage %d (%q), want stage 1 (treeplan)", res.ResumeStage, res.StageName)
	}
	if res.Reused != 1 || res.Redo != 1 {
		t.Errorf("reused %d redo %d, want 1 and 1", res.Reused, res.Redo)
	}
	if v := res.Verdicts[0]; v.Verdict != VerdictInvalid {
		t.Errorf("%s is %s, want %s", v.Path, v.Verdict, VerdictInvalid)
	}
}

// TestScanStopsAtAnUnstampedArtifact: a stage whose output is present but
// carries no proof is not complete, so the walk stops there rather than
// reading the downstream artifacts derived from it.
func TestScanStopsAtAnUnstampedArtifact(t *testing.T) {
	s, _ := newArtifactStore(t)
	chain := testChain()
	build(t, s, chain, relSurvey, relTreePlan)
	rm(t, s, relSurvey+StampSuffix)

	res := scan(t, s, chain, ModeResume)
	if res.ResumeStage != 0 {
		t.Fatalf("resume at stage %d, want the survey stage", res.ResumeStage)
	}
	if res.Verdicts[0].Verdict != VerdictInvalid {
		t.Errorf("verdict %s, want %s", res.Verdicts[0].Verdict, VerdictInvalid)
	}
}

// TestScanFreshReadsTheStoreItWasPointedAt: what makes --fresh rebuild
// everything is the DISCARD its caller ran first (Coordinator.Run), not a scan
// that declines to look. So a fresh scan over a store nobody emptied reports
// exactly what is there — the measurement that makes a run's "0 reused" mean
// the directory was empty.
//
// This test is deliberately the un-discarded half: the discarded half, where
// the same walk reports nothing reused, is TestFreshsReusedCountMeasuresTheDiscard
// in fresh_test.go, and the two together are what fails when the discard goes.
func TestScanFreshReadsTheStoreItWasPointedAt(t *testing.T) {
	s, lg := newArtifactStore(t)
	chain := testChain()
	build(t, s, chain, relSurvey, relTreePlan, relLeafA, relLeafB)

	res := scan(t, s, chain, ModeFresh)
	if !res.Complete() || res.Reused != 4 || res.Redo != 0 {
		t.Errorf("a fresh scan over a store nobody discarded reported %d reused and %d to redo, "+
			"want the four units that are sitting there: %+v", res.Reused, res.Redo, res)
	}
	if !lg.Has(t, "info", "mode", ModeFresh) {
		t.Error("a fresh run did not say so in the log")
	}
}

func TestScanRefusesIncoherentStore(t *testing.T) {
	s, _ := newArtifactStore(t)
	chain := testChain()
	build(t, s, chain, relSurvey, relTreePlan)
	restamp(t, s, relSurvey, func(st *Stamp) { st.Path = "elsewhere.json" })

	_, err := s.ResumeScan(chain, ModeResume)
	assertIncoherent(t, err, remedyFresh)
}

// TestScanRefusesBadChains: a chain description defect refuses the run rather
// than being verdicted. Under the dynamic chain the refusal lands when the
// stage carrying the defect is DESCRIBED, which for a later stage is when the
// run reaches it — so these drive the description to the end of the chain, the
// way Coordinator.Run does stage by stage.
func TestScanRefusesBadChains(t *testing.T) {
	cases := []struct {
		name  string
		chain StageChain
	}{
		{"empty", StageChain{}},
		{
			name: "a path produced by two stages",
			chain: StageChain{
				{Name: "a", Units: fixedUnits(OwedArtifact{Path: "x.md"})},
				{Name: "b", Units: fixedUnits(OwedArtifact{Path: "x.md"})},
			},
		},
		{
			name: "an upstream nothing produces and nothing proves",
			chain: StageChain{
				{Name: "a", Units: fixedUnits(OwedArtifact{Path: "x.md"})},
				{Name: "b", Units: fixedUnits(OwedArtifact{Path: "y.md", Upstreams: []string{"nowhere.md"}})},
			},
		},
		{
			name: "an upstream from the same stage",
			chain: StageChain{
				{Name: "a", Units: fixedUnits(
					OwedArtifact{Path: "x.md"},
					OwedArtifact{Path: "y.md", Upstreams: []string{"x.md"}},
				)},
			},
		},
		{
			name:  "a path outside the job directory",
			chain: StageChain{{Name: "a", Units: fixedUnits(OwedArtifact{Path: "../x.md"})}},
		},
		{
			name:  "a description that cannot be resolved at all",
			chain: StageChain{{Name: "a", Units: func() ([]OwedArtifact, error) { return nil, errors.New("no tree plan to read") }}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newArtifactStore(t)
			if _, err := s.ResumeScan(tc.chain, ModeResume); err == nil && len(tc.chain) == 0 {
				t.Fatal("ResumeScan accepted an empty chain")
			}
			if len(tc.chain) == 0 {
				return
			}
			if _, err := s.resolveStage(tc.chain, len(tc.chain)-1); err == nil {
				t.Fatal("the chain description was accepted")
			}
		})
	}
}

func TestScanUnknownMode(t *testing.T) {
	s, _ := newArtifactStore(t)
	if _, err := s.ResumeScan(testChain(), Mode("maybe")); err == nil {
		t.Fatal("ResumeScan accepted an unknown mode")
	}
}

// TestChainMayConsumeAProvenArtifactItDoesNotProduce is the other half of the
// dynamic chain: a stage's upstream need not be produced by this chain at all
// — a re-plan consumes the tree plan a previous epoch left behind — but it must
// be PROVEN. So the store is asked for the stamp, and an upstream nothing
// proves refuses the run rather than being verdicted, since no amount of
// running this chain would ever produce it.
func TestChainMayConsumeAProvenArtifactItDoesNotProduce(t *testing.T) {
	const external = "epoch1/treeplan.json"
	chain := func() StageChain {
		return StageChain{{Name: "leaves", Units: fixedUnits(
			OwedArtifact{Path: relLeafA, Inputs: []Input{sourceInput}, Upstreams: []string{external}},
		)}}
	}

	// The one incoherence --fresh does not cure, and the refusal says so: an
	// empty job directory holds no stamp for that artifact either.
	t.Run("unproven is a refusal --fresh cannot cure", func(t *testing.T) {
		s, _ := newArtifactStore(t)
		for _, mode := range []Mode{ModeResume, ModeFresh} {
			_, err := s.ResumeScan(chain(), mode)
			assertIncoherent(t, err, remedyUnproduced)
		}
	})

	t.Run("stamped is legal, and its hash enters the unit", func(t *testing.T) {
		s, _ := newArtifactStore(t)
		put(t, s, external, "a previous epoch's tree plan", sourceInput)

		res := scan(t, s, chain(), ModeResume)
		if res.ResumeStage != 0 || res.Redo != 1 {
			t.Fatalf("scan = %+v, want the one stage to be redone", res)
		}
		units, err := chain()[0].Units()
		if err != nil {
			t.Fatal(err)
		}
		inputs, err := s.resolveInputs(units[0])
		if err != nil {
			t.Fatalf("resolveInputs: %v", err)
		}
		stamp, err := s.readStamp(external)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, in := range inputs {
			if in.Name == upstreamInputPrefix+external && in.Hash == stamp.Output {
				found = true
			}
		}
		if !found {
			t.Errorf("inputs %+v do not carry the external upstream's output hash", inputs)
		}
	})
}

// TestStageIsDescribedWhenItIsReached: the walk does not describe a stage
// past the resume boundary. That is what makes the dynamic chain possible —
// stage N+1's units may not be knowable until stage N has run — and it is a
// property of the walk, not an accident of the fixture.
func TestStageIsDescribedWhenItIsReached(t *testing.T) {
	s, _ := newArtifactStore(t)
	described := 0
	chain := StageChain{
		{Name: "survey", Units: fixedUnits(OwedArtifact{Path: relSurvey, Inputs: []Input{sourceInput}})},
		{Name: "treeplan", Units: func() ([]OwedArtifact, error) {
			described++
			return []OwedArtifact{{Path: relTreePlan, Inputs: []Input{sourceInput}, Upstreams: []string{relSurvey}}}, nil
		}},
	}

	// The first stage is absent, so the walk stops there.
	if _, err := s.ResumeScan(chain, ModeResume); err != nil {
		t.Fatalf("ResumeScan: %v", err)
	}
	if described != 0 {
		t.Errorf("the second stage was described %d times before the first was complete", described)
	}

	// With the first stage proven, the walk reaches the second and asks.
	// Written directly rather than through build, which describes every
	// stage of the chain it is handed and would be counted here.
	put(t, s, relSurvey, "content of "+relSurvey, sourceInput)
	if _, err := s.ResumeScan(chain, ModeResume); err != nil {
		t.Fatalf("ResumeScan: %v", err)
	}
	if described != 1 {
		t.Errorf("the second stage was described %d times, want once", described)
	}
}

// TestResolveInputsIsTheOneDerivation guards the property both sides of every
// proof depend on: what Put stamps and what Scan checks come from the same
// function, so an upstream's identity enters the stamp under a name that
// cannot drift.
func TestResolveInputsIsTheOneDerivation(t *testing.T) {
	s, _ := newArtifactStore(t)
	chain := testChain()
	build(t, s, chain, relSurvey)

	treePlan, err := chain[1].Units()
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := s.resolveInputs(treePlan[0])
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if len(inputs) != 2 {
		t.Fatalf("%d inputs, want the source plus one upstream", len(inputs))
	}
	surveyStamp, err := s.readStamp(relSurvey)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range inputs {
		if in.Name == upstreamInputPrefix+relSurvey {
			found = true
			if in.Hash != surveyStamp.Output {
				t.Errorf("upstream input carries %q, want the upstream's output hash", in.Hash)
			}
		}
	}
	if !found {
		t.Errorf("inputs %+v carry no entry for the upstream artifact", inputs)
	}

	// An upstream that was never written is doubt, not a refusal.
	leaves, err := chain[2].Units()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.resolveInputs(leaves[0])
	var inc IncoherentStoreError
	if err == nil || errors.As(err, &inc) {
		t.Errorf("ResolveInputs over an unwritten upstream = %v, want an ordinary error", err)
	}
}
