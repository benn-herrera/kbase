package summarize

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"kbase/internal/log/logtest"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/prompt"
	"kbase/internal/treeplan"
)

// TestLevelStagesSummariseEverySection: the clean path over the whole chain.
// One artifact per section node, written deepest first, and the levels the tree
// does not reach cost nothing.
func TestLevelStagesSummariseEverySection(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newScene(t, lg, pageBody)
	client := model.NewScriptedMock([]model.Response{
		answer("What this section holds.", "What it settles", "Alpha is configured in the manifest."),
	}, nil)

	res, err := sc.run(t, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := len(sc.indexNodes())
	if res.Produced != want || len(res.Failures) != 0 || !res.DeliveryReady() {
		t.Fatalf("result = %+v, want %d clean summaries", res, want)
	}
	if res.FallbackCount != 0 {
		t.Errorf("a no-fallback seam reported %d fallbacks", res.FallbackCount)
	}
	// The static stage list is four levels deep; this tree is two, so two
	// stages resolved to zero lanes and the coordinator skipped them. The unit
	// count is what proves they cost nothing.
	if res.Units != want {
		t.Errorf("%d units over %d stages, want one per section", res.Units, res.Stages)
	}
	for _, n := range sc.indexNodes() {
		s := sc.summaryIn(t, n.Path)
		if s.Schema != SchemaVersion || s.Framing == "" {
			t.Errorf("%s carries %+v, want a schema and a framing", n.Path, s)
		}
	}
}

// TestSummaryReadsPerChildKind is F-10: a leaf child is read as a BODY, an
// index child as its own framing and conclusions. It is a property of the NODE
// and not of the level, which is why the two are asserted on two calls of one
// run.
func TestSummaryReadsPerChildKind(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newScene(t, lg, pageBody)
	// One response per call, so the entry-point's call reads summaries that
	// are distinguishable from the pages under them.
	sections := len(sc.indexNodes())
	responses := make([]model.Response, 0, sections)
	for i := range sections {
		responses = append(responses, answer(
			"framing-"+string(rune('a'+i)), "Heading "+string(rune('A'+i)),
			"conclusion-"+string(rune('a'+i))))
	}
	client := model.NewScriptedMockPerConsult(responses)
	client.RecordCalls = true

	if _, err := sc.run(t, client, lg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	calls := client.Calls()
	if len(calls) != sections {
		t.Fatalf("%d calls, want one per section", len(calls))
	}
	// The first calls are the level-2 sections: each reads its pages' bodies.
	first := calls[0].Request.Messages[0].Content
	if !strings.Contains(first, "The page body of") {
		t.Errorf("a section over pages did not read their bodies:\n%s", first)
	}
	// The last call is the entry-point: it reads its children's SUMMARIES and
	// not their pages (ARCHITECTURE §4 row 6, read per child kind).
	last := calls[len(calls)-1].Request.Messages[0].Content
	if !strings.Contains(last, "framing-a") || !strings.Contains(last, "conclusion-a") {
		t.Errorf("the entry-point did not read its children's summaries:\n%s", last)
	}
	if strings.Contains(last, "The page body of") {
		t.Errorf("the entry-point read a page body through an index child:\n%s", last)
	}
	// Routing context is the tree plan's scope lines, beside the material.
	if !strings.Contains(last, "What a reader finds under this section:") {
		t.Errorf("the call carried no routing context:\n%s", last)
	}
}

// TestSummaryRetriesThenFailsAndCascades: the no-fallback seam end to end. A
// section that never verifies fails its unit, and the entry-point above it is
// CASCADE-failed — not attempted, no call spent, reported beside its cause.
func TestSummaryRetriesThenFailsAndCascades(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newScene(t, lg, pageBody)
	client := model.NewScriptedMock([]model.Response{{Content: "I could not summarise that.", FinishReason: "stop"}}, nil)
	client.RecordCalls = true

	res, err := sc.run(t, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DeliveryReady() {
		t.Fatal("a run whose summaries never verified was reported emit-ready")
	}
	if res.Produced != 0 {
		t.Errorf("%d summaries were written for a run where none verified", res.Produced)
	}
	var cascades, verifications int
	for _, f := range res.Failures {
		switch f.Kind {
		case pipeline.FailureCascade:
			cascades++
		case pipeline.FailureVerification:
			verifications++
		}
	}
	if verifications != 2 || cascades != 1 {
		t.Fatalf("failures = %+v, want two failed sections and the entry-point cascaded", res.Failures)
	}
	// Cascade-failure spends nothing: two sections × two attempts, and no call
	// at all for the unit above them.
	if got := len(client.Calls()); got != 4 {
		t.Errorf("%d calls, want two attempts per failed section and none for the cascade", got)
	}
}

// TestSummaryInputIsBoundedByG2: the whole input one call may read is
// summaryInputTokens, and the tree plan proved that before this stage ran. A
// breach is therefore OUR arithmetic — refuse-and-split, inventoried against
// the node whose fan-out is wrong, never a truncated prompt.
func TestSummaryInputIsBoundedByG2(t *testing.T) {
	lg := &logtest.Capture{}
	huge := func(n treeplan.Node) string {
		return strings.Repeat("oversized page body. ", 2000)
	}
	sc := newScene(t, lg, huge)
	client := model.NewScriptedMock([]model.Response{
		answer("What this section holds.", "What it settles", ""),
	}, nil)

	res, err := sc.run(t, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DeliveryReady() {
		t.Fatal("a call over the G-2 bound was allowed to go out")
	}
	found := false
	for _, f := range res.Failures {
		if f.Kind == pipeline.FailureBudget {
			found = true
			var over prompt.ErrOverBudget
			if !errorsAs(f.Err, &over) {
				t.Fatalf("the budget failure is %v, want a slot refusal", f.Err)
			}
			if over.Slot != prompt.SlotContent {
				t.Errorf("the refusal names %s, want the content buffer the children fill", over.Slot)
			}
			if over.Budget != testBudgets().SummaryInputTokens {
				t.Errorf("the refusal is against a budget of %d, want G-2's %d",
					over.Budget, testBudgets().SummaryInputTokens)
			}
		}
	}
	if !found {
		t.Fatalf("failures = %+v, want a budget refusal", res.Failures)
	}
}

// TestVerifierPostConditions is §6.4, table-driven over the one function that
// states them. Empty conclusions are legal and stay legal; everything else here
// is a rejection the model is asked to fix once.
func TestVerifierPostConditions(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newScene(t, lg, pageBody)
	if _, err := sc.sum.resolve(); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	entry := sc.plan.Nodes[0]
	unitPath := sc.sum.Unit(entry.Path)
	// A leaf's own path, for the node-path check to catch.
	var leaf treeplan.Node
	for _, n := range sc.plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			leaf = n
			break
		}
	}

	for _, tc := range []struct {
		name     string
		response string
		want     string // a substring of the rejection, or "" for accepted
	}{
		{"accepted", mustSummary("What this holds.", "What it settles", "It settles this."), ""},
		{"empty conclusions are legal", mustSummary("Only navigation lives here.", "", ""), ""},
		{"fenced object", "```json\n" + mustSummary("Fenced.", "", "") + "\n```", ""},
		{"prose", "I would summarise it as follows.", "JSON object"},
		{"no framing", mustSummary("", "What it settles", "It settles this."), "say in a line or two"},
		{"over the summary cap", mustSummary(strings.Repeat("long ", 400), "", ""), "too long"},
		{"conclusions without a heading", mustSummary("Framing.", "", "It settles this."), "need a heading"},
		{"a link", mustSummary("See [the page](one.md).", "", ""), "no links"},
		{"a node path", mustSummary("Read "+leaf.Path+" for detail.", "", ""), "do not name a file"},
		{"a top-level heading", mustSummary("Framing.\n\n# Contents\n\nmore", "", ""), "no top-level heading"},
		{"a marked-up heading", mustSummary("Framing.", "## Results", "It settles this."), "plain title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sc.sum.verify(unitPath, tc.response)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("verify rejected a legal answer: %v", err)
				}
				s, ok := got.(Summary)
				if !ok || s.Unit != unitPath || s.Schema != SchemaVersion {
					t.Fatalf("verify returned %+v, want a stamped summary", got)
				}
				return
			}
			if err == nil {
				t.Fatalf("verify accepted %q", tc.response)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("rejection %q does not carry %q", err, tc.want)
			}
			if strings.Contains(err.Error(), unitPath) {
				t.Error("the rejection note names a store path; a note carries no path")
			}
		})
	}
}

// TestSummaryQuotingASourcePathIsLegal is F-11's point: the check is NODE-path
// matching, not path-shape matching. A summary that quotes a source file, an
// API path or a configuration key verbatim is conforming to §4.5's discipline,
// not breaking I-2.
func TestSummaryQuotingASourcePathIsLegal(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newScene(t, lg, pageBody)
	if _, err := sc.sum.resolve(); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	unitPath := sc.sum.Unit(sc.plan.Nodes[0].Path)
	legal := mustSummary("Projects are described by default.project.json under src/shared.", "", "")
	if _, err := sc.sum.verify(unitPath, legal); err != nil {
		t.Fatalf("a summary quoting source paths was rejected: %v", err)
	}
}

func mustSummary(framing, heading, conclusions string) string {
	data, err := json.Marshal(Summary{Framing: framing, ConclusionsHeading: heading, Conclusions: conclusions})
	if err != nil {
		panic(err)
	}
	return string(data)
}

// errorsAs is errors.As with the test's own name, so the assertion above reads
// as one line rather than as two.
func errorsAs(err error, target any) bool { return errors.As(err, target) }
