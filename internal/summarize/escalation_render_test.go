package summarize

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/log/logtest"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// The empty-retry investigation (2026-08-18): summaries/entry-point.md.json
// attempt 1 was rejected "too long", and the escalated retry returned one byte —
// a newline. This test renders BOTH attempts of the entry-point's summary ask
// exactly as a live run does — real definition, real Effort and Retry
// declarations, the real coordinator and the real CallRunner — and pins the
// three facts the fixes for it turn on:
//
//  1. the cap the "too long" verifier enforces (Budgets.SummaryTokens) is still
//     stated to the model in NEITHER attempt, and the ask bounds its conclusions
//     block all the same — as a shape, the way it bounds its framing (ruled
//     2026-08-21). TestTheConclusionsShapeFitsTheSummaryCap holds that shape to
//     the cap it was calibrated against;
//  2. the escalated retry differs from the first attempt in the trailer alone —
//     the machine note is appended to the acceptance criteria and nothing else
//     in the prompt moves;
//  3. on the wire the escalation moves the two thinking kwargs AND the
//     completion window: the attempt asked to reason gets ThinkingMaxTokens
//     where the first attempt got DefaultMaxTokens.
//
// Fact 3 is the one that cost an artifact, and it is the pin worth keeping.
// MaxTokens bounds reasoning and completion TOGETHER on an OpenAI-compatible
// endpoint, and Final() accumulates the content channel alone
// (model.httpStreamReader) — so an escalated retry that inherits the first
// attempt's window spends it reasoning and returns a truncated stream whose
// Content is empty. An inherited window here is that event again.
//
// The rendered bytes and the two wire records land under
// test_data/transient/summarize-escalation-render/ so the reading is
// reproducible rather than reported.
func TestEscalatedRetryRenderAndWire(t *testing.T) {
	lg := log.Discard()

	// The shipped ask, not the fixture's: the fixture's retry declaration
	// deliberately does not escalate, and the escalation is the subject here.
	shipped := func(sc *scene) {
		s, err := New(Job{
			TreePlan:     func() (treeplan.TreePlan, error) { return sc.plan, nil },
			Store:        sc.store,
			LeavesDir:    leavesDir,
			SummariesDir: summariesDir,
			Params:       treeplan.Params{Budgets: treeplan.DefaultBudgets()},
			Effort:       Effort,
			Retry:        Retry,
		}, lg)
		if err != nil {
			t.Fatalf("summarize.New at the shipped operating point: %v", err)
		}
		sc.sum = s
	}

	// Pass 1 counts the stage's calls. The entry-point is the LAST of them —
	// the level stages run deepest-first and it sits at level 1 — which is what
	// lets pass 2 script a rejection onto that call by position without
	// hardcoding a tree shape.
	countScene := newScene(t, lg, pageBody)
	shipped(countScene)
	counter := model.NewScriptedMock([]model.Response{
		answer("This section collects the fixture documents.", "What it establishes", "Nothing in particular."),
	}, nil)
	counter.RecordCalls = true
	if _, err := countScene.run(t, counter, lg); err != nil {
		t.Fatalf("the counting pass should summarise cleanly: %v", err)
	}
	calls := len(counter.Calls())
	if calls < 2 {
		t.Fatalf("the fixture made %d summary calls; this test needs the entry-point to sit behind at least one other", calls)
	}

	// Pass 2: every call but the last verifies, and the entry-point's two
	// attempts are both over the summary cap — so attempt 1 is rejected with
	// "too long" and attempt 2 is the escalated retry the event saw.
	script := make([]model.Response, 0, calls+1)
	for range calls - 1 {
		script = append(script, answer("This section collects the fixture documents.", "What it establishes", "Nothing in particular."))
	}
	overLong := answer("This section collects the fixture documents.", "What it establishes", filler("conclusion", 400))
	script = append(script, overLong, overLong)

	sc := newScene(t, lg, pageBody)
	shipped(sc)
	client := model.NewScriptedMockPerConsult(script)
	client.RecordCalls = true
	// The entry-point fails its unit and the job is not emit-ready (the
	// no-fallback seam), which is the correct outcome and not what is under
	// test — it is here to prove the two captured attempts really are the
	// rejected pair.
	res, err := sc.run(t, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	entryUnit := sc.sum.Unit("entry-point.md")
	if len(res.Failures) != 1 || res.Failures[0].Path != entryUnit {
		t.Fatalf("failures = %+v, want the entry-point unit %q rejected twice", res.Failures, entryUnit)
	}
	if res.Failures[0].ModelAttempts != Retry.Attempts {
		t.Fatalf("the entry-point made %d attempts, want the declared %d",
			res.Failures[0].ModelAttempts, Retry.Attempts)
	}
	got := client.Calls()
	if len(got) != calls+1 {
		t.Fatalf("expected %d consults (%d clean, then the entry-point twice), got %d", calls+1, calls-1, len(got))
	}
	first, retry := got[calls-1], got[calls]

	turn := func(c model.MockCall) string {
		if len(c.Request.Messages) != 1 {
			t.Fatalf("a call carried %d messages; the appliance sends one user turn", len(c.Request.Messages))
		}
		return c.Request.Messages[0].Content
	}
	firstTurn, retryTurn := turn(first), turn(retry)

	// The two attempts are the entry-point's: its status line is the one the
	// builder writes only for the root (callInput).
	const rootLine = "This is the whole knowledge base"
	if !strings.Contains(firstTurn, rootLine) {
		t.Fatalf("the rejected attempt is not the entry-point's call; it lacks %q", rootLine)
	}

	writeEvidence(t, firstTurn, retryTurn, first.Request, retry.Request)

	// (a) The ask bounds both of the blocks its verifier measures, and it
	// bounds them in SHAPE rather than in tokens. check() rejects on the SUM of
	// framing and conclusions against Budgets.SummaryTokens; the model is told
	// how many sentences and how many conclusions, and is told no number it
	// could echo back as an answer. Both halves are load-bearing, so both are
	// pinned: the number must be absent from every attempt, and the bound must
	// be present in every attempt.
	cap := strconv.Itoa(treeplan.DefaultBudgets().SummaryTokens)
	for name, turn := range map[string]string{"attempt 1": firstTurn, "attempt 2": retryTurn} {
		if strings.Contains(turn, cap) {
			t.Errorf("%s states the summary cap %s; the ruling is that the verifier owns the number and the "+
				"ask owns the shape, and check()'s messages stay number-free for the same reason", name, cap)
		}
		for _, bound := range []string{framingBound, conclusionsBound} {
			if !strings.Contains(turn, bound) {
				t.Errorf("%s does not bound its blocks: want the guidance %q on the wire", name, bound)
			}
		}
	}

	// (b) The escalated retry is the first attempt plus a trailer line. Every
	// byte before the trailer is identical, which is what withRetryNote
	// promises and what the prefix cache is being held to.
	note := "- previous attempt rejected: too long; shorten it to a short paragraph and its conclusions"
	if !strings.Contains(retryTurn, note) {
		t.Errorf("the retry note is not in the escalated turn; want a line %q", note)
	}
	if strings.Contains(firstTurn, "previous attempt rejected") {
		t.Error("the first attempt carries a retry note; the note is built from the ORIGINAL input")
	}
	if cut := strings.Index(retryTurn, note); cut >= 0 {
		// Everything the retry says before its note must be what attempt 1 said
		// in the same position.
		if !strings.HasPrefix(firstTurn, retryTurn[:cut-len("\n")]) {
			t.Error("the escalated retry changed bytes before its retry note; the informed retry is meant to be " +
				"the same call with one line added")
		}
	}

	// (c) The escalation on the wire. The thinking kwargs move AND the window
	// moves with them: an escalation that reasons out of the answer's own
	// budget is the 2026-08-18 one-byte retry, so the two must never come apart
	// again.
	if thinking(t, first.Request) {
		t.Error("attempt 1 asked with thinking on; the summary definition declares Thinking: false")
	}
	if !thinking(t, retry.Request) {
		t.Error("the escalated retry asked with thinking off; the Retry declaration escalates")
	}
	if first.Request.MaxTokens != model.DefaultMaxTokens {
		t.Errorf("attempt 1's MaxTokens is %d, want the default window %d",
			first.Request.MaxTokens, model.DefaultMaxTokens)
	}
	if retry.Request.MaxTokens != model.ThinkingMaxTokens {
		t.Errorf("the escalated retry's MaxTokens is %d, want ThinkingMaxTokens %d — reasoning is spent out of "+
			"the answer's window, so the attempt asked to reason is the attempt that needs a bigger one",
			retry.Request.MaxTokens, model.ThinkingMaxTokens)
	}
	if retry.Request.MaxTokens <= first.Request.MaxTokens {
		t.Errorf("the escalated retry asks harder in %d tokens where the first attempt had %d; an escalation "+
			"that does not widen the window makes the harder ask the smaller one",
			retry.Request.MaxTokens, first.Request.MaxTokens)
	}
}

// The shape the ask states for the two blocks its verifier measures. They are
// the definition's own words (stubDefinition) because both ends of the
// calibration have to move together: the prompt states the shape, this test
// computes the worst answer that shape permits, and TestTheConclusionsShapeFitsTheSummaryCap
// holds that answer to the cap. A reworded bound is a recalibration, and it
// fails here until the numbers below say the same thing the prompt does.
const (
	framingBound     = "one to three sentences"
	conclusionsBound = "at most four and each a sentence or two"

	framingSentences   = 3
	conclusionsCap     = 4
	sentencesPerPoint  = 2
	wordsPerSentence   = 24 // a long sentence; the shape must survive a verbose model
	averageWordLetters = 5
)

// TestTheConclusionsShapeFitsTheSummaryCap is WP3's substance: the ask now
// bounds its conclusions, and the whole point of the bound is that an answer
// obeying it is an answer check() accepts.
//
// The ruling forbids the prompt from stating the cap, which means the ask and
// the verifier agree by CALIBRATION rather than by saying the same number. A
// calibration nobody checks drifts the first time either side is edited — the
// cap is provisional (§9) and the definition is a stub — so the agreement is
// held here, on the worst answer the stated shape permits.
func TestTheConclusionsShapeFitsTheSummaryCap(t *testing.T) {
	sentence := strings.TrimSpace(strings.Repeat(strings.Repeat("w", averageWordLetters)+" ", wordsPerSentence)) + "."
	sentences := func(n int) string { return strings.TrimSpace(strings.Repeat(sentence+" ", n)) }

	budgets := treeplan.DefaultBudgets()
	est := tokens.Estimator{}
	worst := Summary{
		Framing:            sentences(framingSentences),
		ConclusionsHeading: "What it establishes",
		Conclusions:        strings.Join(slices.Repeat([]string{sentences(sentencesPerPoint)}, conclusionsCap), "\n\n"),
	}
	got := est.Estimate(worst.Framing + "\n\n" + worst.Conclusions)
	if got > budgets.SummaryTokens {
		t.Errorf("the worst answer the ask's shape permits estimates at %d tokens, over the %d the verifier "+
			"allows: %d framing sentences plus %d conclusions of %d, at %d words each. The ask would be "+
			"engineering its own rejection again — tighten the shape in stubDefinition and here together",
			got, budgets.SummaryTokens, framingSentences, conclusionsCap, sentencesPerPoint, wordsPerSentence)
	}

	// The other half: the cap is still what binds. An answer that ignores the
	// shape must still be rejected, or the calibration above would be passing
	// because nothing measures anything.
	loose := worst
	loose.Conclusions = strings.Join(slices.Repeat([]string{sentences(sentencesPerPoint)}, conclusionsCap*3), "\n\n")
	if got := est.Estimate(loose.Framing + "\n\n" + loose.Conclusions); got <= budgets.SummaryTokens {
		t.Errorf("three times the permitted conclusions still estimates at %d tokens, inside the %d cap; the "+
			"shape is not calibrated to anything", got, budgets.SummaryTokens)
	}
}

// TestAOneByteAnswerIsATruncationNotABadSummary is the classification at the
// seam the 2026-08-18 event happened at. The response IS that artifact: one
// byte, a newline, finish_reason "length".
//
// Put through this ask's verifier it reads as "answer in the FRAMING: HEADING:
// and CONCLUSIONS: blocks" — a compliance rejection the model never earned,
// against a summary it never finished writing. The class and the record are the
// pipeline's, and TestATruncatedAttemptIsNotARejection over there holds them;
// what is held here is that the summary seam gets the same answer with its own
// definition, its own verifier and its own retry in the loop.
func TestAOneByteAnswerIsATruncationNotABadSummary(t *testing.T) {
	const oneByte = "\n"
	truncated := model.Response{Content: oneByte, FinishReason: model.FinishLength}
	good := answer("This section collects the fixture documents.", "What it establishes", "Nothing in particular.")

	t.Run("the retry recovers it and the record names it", func(t *testing.T) {
		lg := &logtest.Capture{}
		sc := newScene(t, lg, pageBody)
		// Every unit is truncated on its first attempt and answers on its
		// second, so every unit consumes exactly two consults and the
		// alternation holds however many units the fixture's tree has. The
		// assertions are then about the CLASS of the event rather than about
		// which unit happened to draw it.
		client := model.NewScriptedMockPerConsult(alternating(sc, truncated, good))
		client.RecordCalls = true

		res, err := sc.run(t, client, lg)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(res.Failures) != 0 {
			t.Fatalf("failures = %+v; a truncated attempt whose retry answered is a recovered unit", res.Failures)
		}
		if !lg.Has(t, "warn", "outcome", pipeline.OutcomeTruncated) {
			t.Error("no record calls the event a truncation; the finish reason is the one field that could")
		}
		if lg.Has(t, "warn", "outcome", pipeline.OutcomeRejected) {
			t.Error("a truncated summary was recorded as a rejection; the model did not answer wrongly, it did not finish")
		}

		// The retry carries no note: there is no mechanical fact about the
		// ANSWER to feed back, and a note would correct a model that never
		// finished speaking.
		for _, c := range client.Calls() {
			if strings.Contains(c.Request.Messages[0].Content, "previous attempt rejected") {
				t.Error("the retry after a truncation carries a rejection note; the remedy is the window, not a correction")
			}
		}
	})

	t.Run("exhausting the attempts fails the unit as a truncation", func(t *testing.T) {
		lg := &logtest.Capture{}
		sc := newScene(t, lg, pageBody)
		client := model.NewScriptedMockPerConsult(alternating(sc, truncated, truncated))

		res, err := sc.run(t, client, lg)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(res.Failures) == 0 {
			t.Fatal("every attempt was truncated and no unit failed")
		}
		for _, f := range res.Failures {
			if f.Kind == pipeline.FailureUpstream || f.Kind == pipeline.FailureCascade {
				continue // the levels above, which never got their inputs
			}
			if f.Kind != pipeline.FailureTruncation {
				t.Errorf("%s failed as %q, want %q: a unit that never received a complete answer is not a unit "+
					"whose answers were wrong, and the two take opposite remedies",
					f.Path, f.Kind, pipeline.FailureTruncation)
			}
		}
	})
}

// alternating scripts first, second, first, second… for as many consults as
// sc's tree could possibly make: two calls per index node (§4 row 6) and two
// model attempts each, plus slack. The mock walks the queue one entry per
// consult, so a pair per unit is what puts each unit's first attempt on `first`
// — and a queue longer than the run needs costs nothing, since the entries past
// the end are never served.
func alternating(sc *scene, first, second model.Response) []model.Response {
	out := make([]model.Response, 0, 4*len(sc.indexNodes())+4)
	for range 2*len(sc.indexNodes()) + 2 {
		out = append(out, first, second)
	}
	return out
}

// thinking reads the effort a request actually put on the wire — both spellings,
// which DefaultRequest always sends together.
func thinking(t *testing.T, req model.Request) bool {
	t.Helper()
	a, aok := req.ChatTemplateKwargs["thinking"].(bool)
	b, bok := req.ChatTemplateKwargs["enable_thinking"].(bool)
	if !aok || !bok {
		t.Fatalf("a request carried chat_template_kwargs %v; both thinking keys are always sent", req.ChatTemplateKwargs)
	}
	if a != b {
		t.Fatalf("the two thinking keys disagree: thinking=%t enable_thinking=%t", a, b)
	}
	return a
}

// writeEvidence lands the rendered turns and the two wire records where they can
// be read: a measurement worth taking is worth being able to look at afterwards
// (CONVENTIONS.md). The messages are omitted from the wire records because they are
// the turns, written beside them in full.
func writeEvidence(t *testing.T, firstTurn, retryTurn string, first, retry model.Request) {
	t.Helper()
	dir := filepath.Join(moduleRootOf(t), "test_data", "transient", "summarize-escalation-render")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("make the evidence directory: %v", err)
	}
	type wire struct {
		Attempt            int            `json:"attempt"`
		Model              string         `json:"model"`
		Temperature        float64        `json:"temperature"`
		MaxTokens          int            `json:"maxTokens"`
		ChatTemplateKwargs map[string]any `json:"chatTemplateKwargs"`
		TurnBytes          int            `json:"turnBytes"`
	}
	records := []wire{
		{1, first.Model, first.Temperature, first.MaxTokens, first.ChatTemplateKwargs, len(firstTurn)},
		{2, retry.Model, retry.Temperature, retry.MaxTokens, retry.ChatTemplateKwargs, len(retryTurn)},
	}
	blob, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatalf("encode the wire records: %v", err)
	}
	for name, content := range map[string][]byte{
		"attempt-1.prompt.txt": []byte(firstTurn),
		"attempt-2.prompt.txt": []byte(retryTurn),
		"wire.json":            blob,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// moduleRootOf walks up to the directory holding go.mod, so evidence lands at
// the repo-relative path the rest of the suite uses regardless of the package
// the test runs from.
func moduleRootOf(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package under test")
		}
		dir = parent
	}
}
