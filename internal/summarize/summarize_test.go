package summarize

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// TestSummaryReadsPerChildKind: a leaf child is read as a BODY and an index
// child as its own framing and conclusions. It is a property of the NODE and not
// of the level, which is why the two are asserted on two calls of one run —
// here over a tree whose every node holds children of ONE kind, which is the
// case that costs one call (the mixed case is the test below).
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
	// not their pages (ARCHITECTURE §4 row 6).
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

// TestMixedContainerBlendsTheLeafGroupCard is the B-5 scheme: a node holding
// both pages and sections costs TWO calls — its pages summarised as a group in
// isolation, then its own call over that group's CARD and its sections'
// summaries. No page body appears in the second call, so a copious root-level
// page can no longer outweigh whole domains at the entry point.
func TestMixedContainerBlendsTheLeafGroupCard(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newMixedScene(t, lg, pageBody)
	// One response per call, so every call's material is traceable to the call
	// that produced it. One worker, so the order is the stage order: the two
	// domains, then the entry-point's card, then the entry-point.
	responses := make([]model.Response, 0, 4)
	for i := range 4 {
		responses = append(responses, answer(
			"framing-"+string(rune('a'+i)), "Heading "+string(rune('A'+i)),
			"conclusion-"+string(rune('a'+i))))
	}
	client := model.NewScriptedMockPerConsult(responses)
	client.RecordCalls = true

	res, err := sc.run(t, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Three section nodes and one card: the card is a unit of its own, which is
	// what makes it resumable and what keeps it out of its reader's stage.
	if res.Produced != 4 || len(res.Failures) != 0 || !res.DeliveryReady() {
		t.Fatalf("result = %+v, want three summaries and one card", res)
	}
	calls := client.Calls()
	if len(calls) != 4 {
		t.Fatalf("%d calls, want one per section plus the entry-point's card", len(calls))
	}

	group := calls[2].Request.Messages[0].Content
	own := calls[3].Request.Messages[0].Content
	// The group call is the leaves-only call shape: the pages directly under the
	// entry-point, in full, and nothing else.
	if !strings.Contains(group, "The page body of Epsilon") {
		t.Errorf("the leaf-group call did not read the page under the entry-point:\n%s", group)
	}
	if strings.Contains(group, "framing-a") {
		t.Errorf("the leaf-group call read a section summary; it is over the pages alone:\n%s", group)
	}
	// The entry-point's own call is summary-class throughout: the card in the
	// pages' place, and one summary per domain.
	if !strings.Contains(own, cardTitle) || !strings.Contains(own, "framing-c") {
		t.Errorf("the entry-point's call did not read the leaf-group card:\n%s", own)
	}
	if !strings.Contains(own, "framing-a") || !strings.Contains(own, "framing-b") {
		t.Errorf("the entry-point's call did not read its domains' summaries:\n%s", own)
	}
	if strings.Contains(own, "The page body of") {
		t.Errorf("the entry-point's call read a raw page body:\n%s", own)
	}
	// The pages keep their own routing lines: what is bounded is the material,
	// not the reader's knowledge of what sits under it.
	if !strings.Contains(own, "- Epsilon — ") {
		t.Errorf("the entry-point's call lost the page's routing line:\n%s", own)
	}

	// The card is an INPUT and never a delivered page's summary block: the
	// entry-point's own artifact is the blended answer, not the card.
	if got := sc.summaryIn(t, "entry-point.md").Framing; got != "framing-d" {
		t.Errorf("the entry-point's summary is %q, want the blended answer", got)
	}
	cardPath := sc.sum.Card("entry-point.md")
	if _, err := sc.store.Get(cardPath); err != nil {
		t.Fatalf("the leaf-group card was not written: %v", err)
	}

	// And it resumes: a second run over the same store proves every unit from
	// its own stamp, the card included, and asks nothing. A client with an empty
	// script fails any call it is given, so "asks nothing" is asserted rather
	// than inferred from a count.
	sc.writePages(t)
	again, err := sc.run(t, model.NewScriptedMockPerConsult(nil), lg)
	if err != nil {
		t.Fatalf("the resumed run: %v", err)
	}
	if again.Reused != 4 || again.Produced != 0 {
		t.Errorf("the resumed run = %+v, want four units reused and nothing re-asked", again)
	}
}

// TestLeavesOnlyContainerCostsOneCall is the other half of the ruling: the group
// call and the summary call are the SAME call wherever a node holds no section,
// so today's behaviour is unchanged and no card exists to be read or swept.
func TestLeavesOnlyContainerCostsOneCall(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newScene(t, lg, pageBody)
	client := model.NewScriptedMock([]model.Response{
		answer("What this section holds.", "What it settles", "Alpha is configured in the manifest."),
	}, nil)
	if _, err := sc.run(t, client, lg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, n := range sc.indexNodes() {
		if _, err := sc.store.Get(sc.sum.Card(n.Path)); err == nil {
			t.Errorf("%s holds children of one kind and still wrote a leaf-group card", n.Path)
		}
	}
}

// TestMixedContainerCardFailureCascades is the stage-6 rework's headline
// invariant, asked directly [GO M-4]: a mixed node's summary is downstream of
// its OWN card, so a card that never verifies cascades into it.
//
// The existing cascade test runs over a non-mixed tree, so it exercises the
// pre-existing section→parent edge and never card→node. This one fails the card
// and asserts the node above it was never attempted.
func TestMixedContainerCardFailureCascades(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newMixedScene(t, lg, pageBody)
	// The stage order is the two domains, then the entry-point's card, then the
	// entry-point. The domains verify; the card never does, on either attempt.
	bad := model.Response{Content: "I could not summarise those pages.", FinishReason: "stop"}
	client := model.NewScriptedMockPerConsult([]model.Response{
		answer("framing-a", "Heading A", "conclusion-a"),
		answer("framing-b", "Heading B", "conclusion-b"),
		bad, bad,
	})
	client.RecordCalls = true

	res, err := sc.run(t, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DeliveryReady() {
		t.Fatal("a run whose leaf-group card never verified was reported emit-ready")
	}
	var cascaded, verification []string
	for _, f := range res.Failures {
		switch f.Kind {
		case pipeline.FailureCascade:
			cascaded = append(cascaded, f.Path)
		case pipeline.FailureVerification:
			verification = append(verification, f.Path)
		}
	}
	card := sc.sum.Card("entry-point.md")
	if len(verification) != 1 || verification[0] != card {
		t.Fatalf("verification failures = %q, want the entry-point's card %q", verification, card)
	}
	unit := sc.sum.Unit("entry-point.md")
	if len(cascaded) != 1 || cascaded[0] != unit {
		t.Fatalf("cascade failures = %q, want the node whose card failed (%q)", cascaded, unit)
	}
	// Cascade-failure spends nothing: two domains, two attempts at the card,
	// and no call at all for the node above it.
	if got := len(client.Calls()); got != 4 {
		t.Errorf("%d calls, want the two domains and two attempts at the card", got)
	}
	if _, err := sc.store.Get(unit); err == nil {
		t.Error("the mixed node's summary was written over a card that never verified")
	}
}

// Absence and unreadability are different events [ARCH F6]. A node with no
// summary is legal — the O-1 grammar renders an index without a conclusions
// block — but a summary that exists and cannot be read is a failure, and
// answering "there is none" for it made a systematically missing summary
// invisible to every gate, since the render and the re-render both read it
// here.
func TestReadDistinguishesAbsenceFromUnreadable(t *testing.T) {
	lg := &logtest.Capture{}
	sc := newScene(t, lg, pageBody)

	s, ok, err := Read(sc.store, summariesDir, "entry-point.md")
	if err != nil || ok || s.Framing != "" {
		t.Fatalf("an absent summary read as (%+v, %t, %v), want the legal empty state", s, ok, err)
	}

	// A directory where the artifact goes: it exists, and it is not a summary.
	blocked := filepath.Join(sc.dir, pipeline.TempWorkDirName, summariesDir, "entry-point.md"+unitSuffix)
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("plant an unreadable artifact: %v", err)
	}
	if _, _, err := Read(sc.store, summariesDir, "entry-point.md"); err == nil {
		t.Error("an unreadable summary read as an absent one")
	}
	if _, err := All(sc.store, summariesDir, sc.plan); err == nil {
		t.Error("All read an unreadable summary as an absent one")
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
