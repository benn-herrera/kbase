package dissect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/log/logtest"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/survey"
)

// The refinement seam driven end to end through the REAL orchestrator:
// Coordinator → CallRunner → model.MockClient. Nothing here stubs the
// pipeline, because what is being tested is the seam's behaviour under the
// pipeline's own policies — one informed retry, then the mechanical baseline
// (§12) — and a stub would be a second implementation of exactly the rules in
// question.
//
// The model side is a mock and the prompt text is a stub (see stubDefinition):
// per the burst's ruling, the live gemma-4 definition and its eval belong to
// the embedded-definitions burst. The seam does not depend on the wording.

const (
	// stageName is the stage under test; also the domain and the unit
	// directory, so a failure names one thing rather than three.
	stageName = "cuts"
	// jobFrame is slot 1 for these runs. The refinement stage has no opinion
	// about it — it is the job's, and one job here means one frame.
	jobFrame = "Job: dissect the fixture span."
)

// nearBoundaryDoc is a document whose blocks are sized so that a SECOND
// candidate lands inside the boundary's overlap window: a big block, a short
// one, a big one. Without the short block every menu would hold exactly the
// mechanical cut, and a seam where the model has no alternative to choose
// tests nothing.
func nearBoundaryDoc() ([]byte, []survey.CutCandidate) {
	return buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 200)},
		blk{survey.CutParagraph, words("b", 30)},
		blk{survey.CutHeading, "# Three\n\n" + words("c", 200)},
	)
}

// oneBoundary builds the single-boundary fixture: the mechanical list, the
// refiner over it, and the boundary itself.
func oneBoundary(t *testing.T) (*Refiner, []byte, Boundary) {
	t.Helper()
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	// Room for the first two blocks but not the third, so the split lands on
	// the heading between them and the short block sits inside the window.
	budget := tokensOf(string(src[:cands[1].Offset])) + 10

	cuts, err := Split(src, span, cands, budget)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	r, err := NewRefiner(src, span, cands, cuts, stageName)
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	bs := r.Boundaries()
	if len(bs) != 1 {
		t.Fatalf("%d boundaries, want exactly one", len(bs))
	}
	if len(bs[0].Menu) < 2 {
		t.Fatalf("menu = %+v, want an alternative to the mechanical cut", bs[0].Menu)
	}
	return r, src, bs[0]
}

// menuNumber is the 1-based menu position of an offset, or 0.
func menuNumber(b Boundary, off int) int {
	for i, c := range b.Menu {
		if c.Offset == off {
			return i + 1
		}
	}
	return 0
}

// alternative is a menu number that is NOT the mechanical cut — the answer
// that proves the model's choice was taken rather than ignored.
func alternative(t *testing.T, b Boundary) (number, offset int) {
	t.Helper()
	for i, c := range b.Menu {
		if c.Offset != b.Cut {
			return i + 1, c.Offset
		}
	}
	t.Fatalf("boundary %+v offers no alternative to its mechanical cut", b)
	return 0, 0
}

// runStage runs the refinement stage over a job directory and returns the
// result and the directory.
func runStage(t *testing.T, r *Refiner, client model.Client, lg log.Logger) (pipeline.JobResult, string, error) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{Models: config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"}}
	coord := pipeline.NewCoordinator(pipeline.NewStore(dir, lg), pipeline.NewCallRunner(client, cfg, lg), 1, lg)

	plan := pipeline.Plan{
		SystemFrame: jobFrame,
		Stages: []*pipeline.StagePlan{r.StagePlan(stageName, []pipeline.Input{
			{Name: "corpus", Hash: pipeline.HashBytes([]byte("fixture"))},
		})},
	}
	res, err := coord.Run(context.Background(), plan, pipeline.ModeResume)
	return res, dir, err
}

// cutIn reads a boundary artifact back as the offset it recorded.
func cutIn(t *testing.T, dir, unit string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(unit)))
	if err != nil {
		t.Fatalf("read %s: %v", unit, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("artifact %s is %q, not an offset", unit, data)
	}
	return n
}

// TestRefinementAcceptsAChoice: the clean path. The model answers with a menu
// number, the verifier maps it back to a byte offset, and the artifact is the
// offset it chose — not the mechanical one it was given.
func TestRefinementAcceptsAChoice(t *testing.T) {
	r, src, b := oneBoundary(t)
	number, offset := alternative(t, b)
	client := model.NewScriptedMock([]model.Response{{Content: fmt.Sprint(number), FinishReason: "stop"}}, nil)
	client.RecordCalls = true
	lg := &logtest.Capture{}

	res, dir, err := runStage(t, r, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || res.Degraded != 0 || len(res.Failures) != 0 {
		t.Fatalf("result = %+v, want one clean unit", res)
	}
	if !res.EmitReady() {
		t.Error("a clean refinement run must be emit-ready")
	}
	if got := cutIn(t, dir, b.Unit); got != offset {
		t.Errorf("artifact records %d, want the chosen %d (mechanical was %d)", got, offset, b.Cut)
	}

	// The call carried the window in the content slot and the menu beside it,
	// which is what the model's number is a choice among.
	prompt := client.Calls()[0].Request.Messages[0].Content
	// Trimmed, because the builder normalizes a slot's edges (prompt
	// .normalizeSlot); the window's own text is what has to be there.
	if window := strings.TrimSpace(string(src[b.Window.Lo:b.Window.Hi])); !strings.Contains(prompt, window) {
		t.Error("the call did not carry the boundary's overlap window")
	}
	for i, c := range b.Menu {
		if !strings.Contains(prompt, fmt.Sprintf("%d. %s:", i+1, c.Kind)) {
			t.Errorf("the call's menu is missing entry %d (%s)", i+1, c.Kind)
		}
	}
	if strings.Contains(prompt, fmt.Sprintf("offset %d", b.Cut)) {
		t.Error("the call showed a raw byte offset; the model's vocabulary is menu numbers")
	}
}

// TestRefinementRetriesWithTheReason: a rejected answer is retried ONCE, and
// the retry carries the mechanical failure reason. A blind resend would be
// hoping temperature fixes it, which was ruled out (§12).
func TestRefinementRetriesWithTheReason(t *testing.T) {
	r, _, b := oneBoundary(t)
	number, offset := alternative(t, b)
	client := model.NewScriptedMockPerConsult([]model.Response{
		{Content: "I would put it somewhere else entirely.", FinishReason: "stop"},
		{Content: fmt.Sprint(number), FinishReason: "stop"},
	})
	client.RecordCalls = true
	lg := &logtest.Capture{}

	res, dir, err := runStage(t, r, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || res.Degraded != 0 {
		t.Fatalf("result = %+v, want the retry to have been accepted", res)
	}
	if got := cutIn(t, dir, b.Unit); got != offset {
		t.Errorf("artifact records %d, want the retry's choice %d", got, offset)
	}
	calls := client.Calls()
	if len(calls) != 2 {
		t.Fatalf("%d calls, want the first attempt and one informed retry", len(calls))
	}
	if !strings.Contains(calls[1].Request.Messages[0].Content, "previous attempt rejected") {
		t.Error("the retry did not carry the corrective note")
	}
	if !lg.Has(t, "warn", "outcome", "rejected") {
		t.Error("the rejection was not recorded")
	}
}

// TestRefinementFallsBackToTheMechanicalCut: two rejections exhaust the
// semantic attempts, and a refinement seam has somewhere valid to stand. The
// unit is produced, marked degraded, and the artifact is the mechanical cut —
// model failure costs quality, never correctness (§3).
func TestRefinementFallsBackToTheMechanicalCut(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"unparseable", "I do not think either of those is right."},
		{"outside the menu", "99"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, b := oneBoundary(t)
			client := model.NewScriptedMock([]model.Response{{Content: tc.content, FinishReason: "stop"}}, nil)
			lg := &logtest.Capture{}

			res, dir, err := runStage(t, r, client, lg)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Produced != 1 || res.Degraded != 1 || len(res.Failures) != 0 {
				t.Fatalf("result = %+v, want one degraded unit", res)
			}
			if !res.EmitReady() {
				t.Error("a degraded refinement is still correct; the job must be emit-ready")
			}
			if got := cutIn(t, dir, b.Unit); got != b.Cut {
				t.Errorf("artifact records %d, want the mechanical cut %d", got, b.Cut)
			}
			if !lg.Has(t, "warn", "unit", b.Unit) {
				t.Error("the fallback was not logged")
			}
		})
	}
}

// TestRefinementTripwireAbortsTheWorker is §5's loud-abort case wired through
// the runner. The candidate set here claims an offset in the middle of a word
// — the shape a rebasing bug has — and the model chooses it. It is a legal
// menu entry, so nothing about the model's answer is wrong; the offsets are.
//
// So it must NOT be retried and must NOT fall back: the baseline comes from
// the same derivation. The worker aborts and the job reports a defect.
func TestRefinementTripwireAbortsTheWorker(t *testing.T) {
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	cuts, err := Split(src, span, cands, tokensOf(string(src[:cands[1].Offset]))+10)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// A candidate a few bytes past the mechanical cut, inside a word: exactly
	// what an off-by-a-rebase would enumerate, and what Assemble would have
	// refused if these bytes and these offsets had ever met.
	bad := cuts[0].End + 4
	if survey.WhitespaceAdjacent(src, bad) {
		t.Fatalf("the fixture's offset %d is not mid-word; the case would not fire", bad)
	}
	poisoned := insertCandidate(cands, survey.CutCandidate{Offset: bad, Kind: survey.CutParagraph})

	r, err := NewRefiner(src, span, poisoned, cuts, stageName)
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	b := r.Boundaries()[0]
	number := menuNumber(b, bad)
	if number == 0 {
		t.Fatalf("the poisoned candidate is not in the menu %+v", b.Menu)
	}

	client := model.NewScriptedMock([]model.Response{{Content: fmt.Sprint(number), FinishReason: "stop"}}, nil)
	client.RecordCalls = true
	res, dir, runErr := runStage(t, r, client, log.Discard())

	var abort pipeline.WorkerAbortError
	if !errors.As(runErr, &abort) {
		t.Fatalf("err = %v (%T), want a WorkerAbortError", runErr, runErr)
	}
	var defect OffsetDefectError
	if !errors.As(runErr, &defect) || defect.Offset != bad {
		t.Errorf("err = %v, want the tripwire's own diagnosis at %d", runErr, bad)
	}
	if res.EmitReady() {
		t.Error("a job that aborted on a defect must not be emit-ready")
	}
	if n := len(client.Calls()); n != 1 {
		t.Errorf("%d calls; a defect is never retried", n)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(b.Unit))); err == nil {
		t.Error("a defect must not fall back to the baseline; it comes from the same offsets")
	}
}

// insertCandidate puts one candidate into a sorted set, keeping it sorted —
// the artifact's own contract, so the poisoned set is refused for the reason
// under test and not for being malformed.
func insertCandidate(cands []survey.CutCandidate, c survey.CutCandidate) []survey.CutCandidate {
	out := make([]survey.CutCandidate, 0, len(cands)+1)
	for _, existing := range cands {
		if c.Offset > 0 && existing.Offset > c.Offset {
			out = append(out, c)
			c.Offset = 0
		}
		out = append(out, existing)
	}
	if c.Offset > 0 {
		out = append(out, c)
	}
	return out
}

// TestRefinementScansSerially is §5's shape: one call per boundary, in
// document order, each carrying its own window. One domain stream is what
// makes that true — a worker owns a domain and runs it serially, so the
// windows overwrite each other instead of accumulating.
func TestRefinementScansSerially(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 200)},
		blk{survey.CutParagraph, words("b", 30)},
		blk{survey.CutHeading, "# Three\n\n" + words("c", 200)},
		blk{survey.CutParagraph, words("d", 30)},
		blk{survey.CutHeading, "# Five\n\n" + words("e", 200)},
	)
	span := wholeSpan(src)
	cuts, err := Split(src, span, cands, 250)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	r, err := NewRefiner(src, span, cands, cuts, stageName)
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	bs := r.Boundaries()
	if len(bs) < 2 {
		t.Fatalf("%d boundaries, want a scan with more than one step", len(bs))
	}

	// One scripted answer per boundary, in order: the mechanical cut's own
	// menu number, so every unit verifies and the subject stays the scan.
	var script []model.Response
	for _, b := range bs {
		n := menuNumber(b, b.Cut)
		if n == 0 {
			t.Fatalf("boundary %+v does not offer its own mechanical cut", b)
		}
		script = append(script, model.Response{Content: fmt.Sprint(n), FinishReason: "stop"})
	}
	client := model.NewScriptedMockPerConsult(script)
	client.RecordCalls = true

	res, dir, err := runStage(t, r, client, &logtest.Capture{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != len(bs) || res.Degraded != 0 {
		t.Fatalf("result = %+v, want one unit per boundary", res)
	}
	calls := client.Calls()
	if len(calls) != len(bs) {
		t.Fatalf("%d calls for %d boundaries; the scan is one call per boundary", len(calls), len(bs))
	}
	for i, b := range bs {
		prompt := calls[i].Request.Messages[0].Content
		if window := strings.TrimSpace(string(src[b.Window.Lo:b.Window.Hi])); !strings.Contains(prompt, window) {
			t.Errorf("call %d does not carry boundary %d's window", i, b.Index)
		}
		if got := cutIn(t, dir, b.Unit); got != b.Cut {
			t.Errorf("%s records %d, want %d", b.Unit, got, b.Cut)
		}
	}
}

// TestNewRefinerVerifiesItsBaseline: the mechanical list is what every
// failure falls back to, so a list that does not verify is a fallback that
// would emit unverified material. It is refused at stage setup, before a
// single call is spent.
func TestNewRefinerVerifiesItsBaseline(t *testing.T) {
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	broken := []survey.Range{{Start: 0, End: 10}, {Start: 20, End: len(src)}}

	if _, err := NewRefiner(src, span, cands, broken, stageName); err == nil {
		t.Fatal("a cut list that does not tile must not become a stage's baseline")
	} else if !strings.Contains(err.Error(), "fall back") {
		t.Errorf("error = %v, want it to name what the list was going to be", err)
	}
}
