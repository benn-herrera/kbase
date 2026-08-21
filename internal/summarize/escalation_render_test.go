package summarize

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/treeplan"
)

// The empty-retry investigation (2026-08-18): summaries/entry-point.md.json
// attempt 1 was rejected "too long", and the escalated retry returned one byte —
// a newline. This test renders BOTH attempts of the entry-point's summary ask
// exactly as a live run does — real definition, real Effort and Retry
// declarations, the real coordinator and the real CallRunner — and pins the
// three facts a hostile read of those bytes turns up:
//
//  1. the cap the "too long" verifier enforces (Budgets.SummaryTokens) is never
//     stated to the model, in either attempt;
//  2. the escalated retry differs from the first attempt in the trailer alone —
//     the machine note is appended to the acceptance criteria and nothing else
//     in the prompt moves;
//  3. on the wire the escalation flips the two thinking kwargs and changes
//     NOTHING ELSE: MaxTokens is the same DefaultMaxTokens on the attempt that
//     is asked to reason as on the attempt that is not.
//
// Fact 3 is the one that costs an artifact. MaxTokens bounds reasoning and
// completion TOGETHER on an OpenAI-compatible endpoint, and Final() accumulates
// the content channel alone (model.httpStreamReader) — so an escalated retry
// that spends the window reasoning returns a truncated stream whose Content is
// empty. Nothing downstream reads FinishReason, so that truncation arrives at
// the verifier dressed as a badly-formed answer.
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

	// (a) The ask never states the cap its verifier enforces. The model is told
	// the framing is "one to three sentences" and told nothing at all about the
	// conclusions' length, while check() rejects on the SUM of the two against
	// Budgets.SummaryTokens. Neither attempt carries the number, so the retry
	// note "too long" is the first and only news the model gets of a budget —
	// and it still never learns what the budget IS.
	cap := strconv.Itoa(treeplan.DefaultBudgets().SummaryTokens)
	for name, turn := range map[string]string{"attempt 1": firstTurn, "attempt 2": retryTurn} {
		if strings.Contains(turn, cap) {
			t.Errorf("%s states the summary cap %s; this test pins that it does not — if the ask now "+
				"declares its own budget, the finding it was written for is fixed and the test should say so", name, cap)
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

	// (c) The escalation on the wire. This is the finding: the ONLY fields that
	// move are the two thinking kwargs. The completion window does not.
	if thinking(t, first.Request) {
		t.Error("attempt 1 asked with thinking on; the summary definition declares Thinking: false")
	}
	if !thinking(t, retry.Request) {
		t.Error("the escalated retry asked with thinking off; the Retry declaration escalates")
	}
	if first.Request.MaxTokens != retry.Request.MaxTokens {
		t.Errorf("MaxTokens moved between attempts (%d → %d); this test pins that it does NOT — the escalated "+
			"retry buys reasoning out of the same window the answer must come from",
			first.Request.MaxTokens, retry.Request.MaxTokens)
	}
	if retry.Request.MaxTokens != model.DefaultMaxTokens {
		t.Errorf("the escalated retry's MaxTokens is %d, want DefaultMaxTokens %d",
			retry.Request.MaxTokens, model.DefaultMaxTokens)
	}
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
// (AGENTS.md). The messages are omitted from the wire records because they are
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
