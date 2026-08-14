package dissect

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/log/logtest"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/pipeline/crashpoint"
	"kbase/internal/survey"
)

// The fallback-backed seam driven end to end through the REAL orchestrator:
// Coordinator → CallRunner → model.MockClient. Nothing here stubs the
// pipeline, because what is being tested is the seam's behaviour under the
// pipeline's own policies — one informed retry, then the mechanical fallback
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

// testEffort stands in for the registration site's declaration. Thinking is ON
// here, deliberately opposite to what the real registration declares
// (cmd.devRefineEffort): these tests assert that the stage passes its caller's
// value through, which a fixture agreeing with the default could not show.
var testEffort = model.DeclareEffort(model.RequestEffort{Thinking: true})

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
// refiner over it, and the boundary itself. Its call is also the one that
// writes the composed cut list, since it is the last (and only) boundary.
func oneBoundary(t *testing.T, lg log.Logger) (*Refiner, []byte, Boundary) {
	t.Helper()
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	// Room for the first two blocks but not the third, so the split lands on
	// the heading between them and the short block sits inside the window.
	p := params(tokensOf(string(src[:cands[1].Offset])) + 10)

	cuts, err := Split(src, span, cands, p)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	r, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, lg)
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	bs := boundariesOf(t, r)
	if len(bs) != 1 {
		t.Fatalf("%d boundaries, want exactly one", len(bs))
	}
	if len(bs[0].Menu) < 2 {
		t.Fatalf("menu = %+v, want an alternative to the mechanical cut", bs[0].Menu)
	}
	return r, src, bs[0]
}

// boundariesOf reads a refiner's adjudications, which is legal only between
// runs — Boundaries refuses while the fold is busy.
func boundariesOf(t *testing.T, r *Refiner) []Boundary {
	t.Helper()
	bs, err := r.Boundaries()
	if err != nil {
		t.Fatalf("Boundaries: %v", err)
	}
	return bs
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

// runStage runs the refinement stage over a fresh job directory and returns
// the result and the directory.
func runStage(t *testing.T, r *Refiner, client model.Client, lg log.Logger) (pipeline.JobResult, string, error) {
	t.Helper()
	dir := t.TempDir()
	res, err := runStageIn(t, dir, r, client, lg)
	return res, dir, err
}

// runStageIn is runStage over a job directory the caller owns — a resume runs
// twice over one directory, which is the whole point of it.
func runStageIn(t *testing.T, dir string, r *Refiner, client model.Client, lg log.Logger) (pipeline.JobResult, error) {
	t.Helper()
	cfg := config.Config{Models: config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"}}
	coord := pipeline.NewCoordinator(storeIn(t, dir, lg), pipeline.NewCallRunner(client, cfg, lg), 1, lg)

	plan := pipeline.Plan{
		JobFrame: jobFrame,
		Stages: []*pipeline.StagePlan{r.StagePlan(stageName, []pipeline.Input{
			{Name: "corpus", Hash: pipeline.HashBytes([]byte("fixture"))},
		})},
	}
	return coord.Run(context.Background(), plan, pipeline.ModeResume)
}

// cutListIn reads the stage's composed artifact back as the cut list it
// records — through the store's own reader, which is how the stage that
// consumes it will get at it.
func cutListIn(t *testing.T, dir, unit string) []survey.Span {
	t.Helper()
	data, err := storeIn(t, dir, log.Discard()).Get(unit)
	if err != nil {
		t.Fatalf("read %s: %v", unit, err)
	}
	// Through DecodeCutList rather than a Sscanf of its own: a test that
	// re-implements the format cannot fail when the format and its decoder
	// disagree, which is the failure worth catching.
	cuts, err := DecodeCutList(data)
	if err != nil {
		t.Fatalf("artifact %s does not decode: %v", unit, err)
	}
	return cuts
}

// storeIn opens the store of an output directory where a real run roots it:
// the temp-work tree kbase creates inside it.
func storeIn(t *testing.T, dir string, lg log.Logger) *pipeline.ArtifactStore {
	t.Helper()
	work, err := pipeline.OpenTempWork(dir, lg)
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	return work.ArtifactStore()
}

// storePath is the on-disk path of a store-relative unit under an output dir.
func storePath(dir, unit string) string {
	return filepath.Join(dir, pipeline.TempWorkDirName, filepath.FromSlash(unit))
}

// cutAt is the offset of boundary i in a composed cut list.
func cutAt(t *testing.T, cuts []survey.Span, i int) int {
	t.Helper()
	if i < 1 || i >= len(cuts) {
		t.Fatalf("boundary %d is not interior to a %d-section list", i, len(cuts))
	}
	return cuts[i].Start
}

// TestRefinementAcceptsAChoice: the clean path. The model answers with a menu
// number, the verifier maps it back to a byte offset, and the artifact is the
// offset it chose — not the mechanical one it was given.
func TestRefinementAcceptsAChoice(t *testing.T) {
	lg := &logtest.Capture{}
	r, src, b := oneBoundary(t, lg)
	number, offset := alternative(t, b)
	client := model.NewScriptedMock([]model.Response{{Content: fmt.Sprint(number), FinishReason: "stop"}}, nil)
	client.RecordCalls = true

	res, dir, err := runStage(t, r, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || res.FallbackCount != 0 || len(res.Failures) != 0 {
		t.Fatalf("result = %+v, want one clean unit", res)
	}
	if !res.DeliveryReady() {
		t.Error("a clean refinement run must be emit-ready")
	}
	if got := cutAt(t, cutListIn(t, dir, b.Unit), b.Index); got != offset {
		t.Errorf("the composed list records %d, want the chosen %d (mechanical was %d)", got, offset, b.Cut)
	}
	// The metrics the fold owes: one adjudication, accepted, and a composed
	// list that passed its whole-list check before it was written.
	if !lg.Has(t, "info", "outcome", "accepted") {
		t.Error("the accepted boundary was not recorded")
	}
	if !lg.Has(t, "info", "verify", "ok") {
		t.Error("the composed list's verification was not recorded")
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
	// §5: the model emits no raw offsets, so it is shown none. Every offset
	// the seam holds for this call — each menu candidate's, the window's own
	// ends, and the span's — must be absent from the prompt as a number.
	// The predecessor of this assertion looked for "offset %d", a string form
	// nothing in the package ever emitted, so it could not fail whatever the
	// code did.
	assertNoOffsets(t, prompt, offsetsOf(b.Menu)...)
	assertNoOffsets(t, prompt, b.Cut, b.Window.Lo, b.Window.Hi, r.span.Start, r.span.End)
}

// TestRefinementRetriesWithTheReason: a rejected answer is retried ONCE, and
// the retry carries the mechanical failure reason. A blind resend would be
// hoping temperature fixes it, which was ruled out (§12).
func TestRefinementRetriesWithTheReason(t *testing.T) {
	lg := &logtest.Capture{}
	r, _, b := oneBoundary(t, lg)
	number, offset := alternative(t, b)
	client := model.NewScriptedMockPerConsult([]model.Response{
		{Content: "I would put it somewhere else entirely.", FinishReason: "stop"},
		{Content: fmt.Sprint(number), FinishReason: "stop"},
	})
	client.RecordCalls = true

	res, dir, err := runStage(t, r, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || res.FallbackCount != 0 {
		t.Fatalf("result = %+v, want the retry to have been accepted", res)
	}
	if got := cutAt(t, cutListIn(t, dir, b.Unit), b.Index); got != offset {
		t.Errorf("the composed list records %d, want the retry's choice %d", got, offset)
	}
	calls := client.Calls()
	if len(calls) != 2 {
		t.Fatalf("%d calls, want the first attempt and one informed retry", len(calls))
	}
	retry := calls[1].Request.Messages[0].Content
	if !strings.Contains(retry, "previous attempt rejected") {
		t.Error("the retry did not carry the retry note")
	}
	// The note is model-facing text, so the two things that made it useless
	// are asserted here: the operator prefix (four of the note's twelve
	// budgeted words, spent before any content) and the raw byte offset it
	// carried into the prompt of the very next call.
	if strings.Contains(retry, "dissect: cut at") {
		t.Error("the retry note carries the operator-facing prefix")
	}
	assertNoOffsets(t, retry, offsetsOf(b.Menu)...)
	assertNoOffsets(t, retry, b.Cut, b.Window.Lo, b.Window.Hi)
	if !lg.Has(t, "warn", "outcome", "rejected") {
		t.Error("the rejection was not recorded")
	}
}

// TestRefinementFallsBackToTheMechanicalCut: two rejections exhaust the
// model attempts, and a fallback-backed seam has somewhere valid to stand. The
// unit is produced, marked as fallen back, and the artifact is the mechanical cut —
// model failure costs quality, never correctness (§3).
func TestRefinementFallsBackToTheMechanicalCut(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"unparseable", "I do not think either of those is right."},
		{"outside the menu", "99"},
		// The mis-mapping case. A response that OPENS with a number and then
		// says something else is not a choice; reading the first digit run out
		// of it would have yielded 2 — a legal menu entry, which then passes
		// membership and the clamp, verifies, and is written as a cut nobody
		// chose. A visible fallback is the correct outcome.
		{"a number in a sentence", "Boundary 2 of 3, so I would move it to 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lg := &logtest.Capture{}
			r, _, b := oneBoundary(t, lg)
			client := model.NewScriptedMock([]model.Response{{Content: tc.content, FinishReason: "stop"}}, nil)

			res, dir, err := runStage(t, r, client, lg)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Produced != 1 || res.FallbackCount != 1 || len(res.Failures) != 0 {
				t.Fatalf("result = %+v, want one degraded unit", res)
			}
			if !res.DeliveryReady() {
				t.Error("a degraded refinement is still correct; the job must be emit-ready")
			}
			if got := cutAt(t, cutListIn(t, dir, b.Unit), b.Index); got != b.Cut {
				t.Errorf("the composed list records %d, want the mechanical cut %d", got, b.Cut)
			}
			if !lg.Has(t, "warn", "unit", b.Unit) {
				t.Error("the fallback was not logged")
			}
			if !lg.Has(t, "info", "outcome", "fallback") {
				t.Error("the fold did not record the fallback in its own metrics")
			}
		})
	}
}

// TestRefinementTripwireAbortsTheWorker is §5's loud-abort case wired through
// the runner. The candidate set here claims an offset in the middle of a word
// — the shape a rebasing bug has — and the model chooses it. It is a legal
// menu entry, so nothing about the model's answer is wrong; the offsets are.
//
// So it must NOT be retried and must NOT fall back: the fallback comes from
// the same derivation. The worker aborts and the job reports a defect.
func TestRefinementTripwireAbortsTheWorker(t *testing.T) {
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	p := params(tokensOf(string(src[:cands[1].Offset])) + 10)
	cuts, err := Split(src, span, cands, p)
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

	r, err := NewRefiner(src, span, poisoned, cuts, stageName, p, testEffort, log.Discard())
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	b := boundariesOf(t, r)[0]
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
	if res.DeliveryReady() {
		t.Error("a job that aborted on a defect must not be emit-ready")
	}
	if n := len(client.Calls()); n != 1 {
		t.Errorf("%d calls; a defect is never retried", n)
	}
	if _, err := os.Stat(storePath(dir, b.Unit)); err == nil {
		t.Error("a defect must not fall back to the fallback; it comes from the same offsets")
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
// document order, each carrying its own window. One serial lane is what
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
	p := params(250)
	cuts, err := Split(src, span, cands, p)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	lg := &logtest.Capture{}
	r, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, lg)
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	bs := boundariesOf(t, r)
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

	res, dir, err := runStage(t, r, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// One artifact, however many boundaries: the calls are the fold's steps
	// and the composed cut list is the only thing the stage owes.
	if res.Produced != 1 || res.FallbackCount != 0 {
		t.Fatalf("result = %+v, want the one composed cut list", res)
	}
	calls := client.Calls()
	if len(calls) != len(bs) {
		t.Fatalf("%d calls for %d boundaries; the scan is one call per boundary", len(calls), len(bs))
	}
	list := cutListIn(t, dir, bs[len(bs)-1].Unit)
	for i, b := range bs {
		prompt := calls[i].Request.Messages[0].Content
		if window := strings.TrimSpace(string(src[b.Window.Lo:b.Window.Hi])); !strings.Contains(prompt, window) {
			t.Errorf("call %d does not carry boundary %d's window", i, b.Index)
		}
		if got := cutAt(t, list, b.Index); got != b.Cut {
			t.Errorf("the composed list puts boundary %d at %d, want %d", b.Index, got, b.Cut)
		}
	}
	// Every boundary but the last writes nothing at all: a fold that was
	// interrupted leaves no half-list for a resume to mistake for a whole one.
	for _, b := range bs[:len(bs)-1] {
		if _, err := os.Stat(storePath(dir, b.Unit)); err == nil {
			t.Errorf("%s exists; a boundary call produces stage state, not an artifact", b.Unit)
		}
	}
	if n := lg.Count("info", "outcome", "accepted"); n != len(bs) {
		t.Errorf("%d boundaries recorded as accepted, want %d", n, len(bs))
	}
}

// TestNewRefinerVerifiesItsBaseline: the mechanical list is what every
// failure falls back to, so a list that does not verify is a fallback that
// would emit unverified material. It is refused at stage setup, before a
// single call is spent.
func TestNewRefinerVerifiesItsBaseline(t *testing.T) {
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	broken := []survey.Span{{Start: 0, End: 10}, {Start: 20, End: len(src)}}

	if _, err := NewRefiner(src, span, cands, broken, stageName, params(250), testEffort, log.Discard()); err == nil {
		t.Fatal("a cut list that does not tile must not become a stage's fallback")
	} else if !strings.Contains(err.Error(), "fall back") {
		t.Errorf("error = %v, want it to name what the list was going to be", err)
	}
}

// denseBlocks is a run of short paragraphs: the shape dense prose (no lists,
// no fences) actually has, and the one that puts far more candidates in a
// boundary's window than a menu may show.
//
// The filler tag is one letter, so every number in the document text is two
// digits and cannot collide with the three-digit byte offsets the
// no-raw-offsets guard looks for.
func denseBlocks(n int) []blk {
	bs := make([]blk, 0, n)
	for range n {
		bs = append(bs, blk{survey.CutParagraph, words("q", 20)})
	}
	return bs
}

// TestMenuIsCappedAroundTheIncumbent is §9's menuCap: three candidates before
// the mechanical cut, the cut itself, three after.
//
// The cap is what closes the last unbounded input to this seam's prompt — a
// dense window can offer a hundred candidates, which is a reference buffer the
// size of the content it annotates and a hundred-way question for a 26B model.
// The incumbent is always in the menu because confirming the boundary is the
// most common correct answer at a fallback-backed seam.
func TestMenuIsCappedAroundTheIncumbent(t *testing.T) {
	src, cands := buildDoc(denseBlocks(60)...)
	span := wholeSpan(src)
	p := params(tokensOf(string(src[:cands[29].Offset])) + 5)

	cuts, err := Split(src, span, cands, p)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	r, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, log.Discard())
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	b := boundariesOf(t, r)[0]

	// The window really does hold more than the cap, or the case is vacuous.
	inWindow := 0
	for _, c := range cands {
		if b.Window.Contains(c.Offset) {
			inWindow++
		}
	}
	if inWindow <= menuCap {
		t.Fatalf("the window holds %d candidates; the cap is only tested above %d", inWindow, menuCap)
	}
	if len(b.Menu) != menuCap {
		t.Errorf("menu holds %d entries, want the cap of %d", len(b.Menu), menuCap)
	}
	if got := menuNumber(b, b.Cut); got != menuSide+1 {
		t.Errorf("the mechanical cut is entry %d, want %d — %d before it and %d after",
			got, menuSide+1, menuSide, menuSide)
	}
	for i, c := range b.Menu {
		if !b.Window.Contains(c.Offset) {
			t.Errorf("menu entry %d is outside the clamp window", i+1)
		}
		if !isCandidate(cands, c.Offset) {
			t.Errorf("menu entry %d is not an enumerated candidate", i+1)
		}
	}

	// The incumbent is marked, so "kept the mechanical cut" is a choice the
	// model can deliberately make rather than one it can only stumble into.
	rendered := renderMenu(src, b)
	if n := strings.Count(rendered, incumbentMark); n != 1 {
		t.Errorf("the rendered menu marks %d entries as current, want exactly 1:\n%s", n, rendered)
	}
	if !strings.Contains(rendered, fmt.Sprintf("%d. ", menuSide+1)) {
		t.Errorf("the rendered menu is missing the incumbent's entry:\n%s", rendered)
	}
	assertNoOffsets(t, rendered, offsetsOf(cands)...)
}

// TestMenuShortSideContributesWhatItHas: a boundary whose left neighbour holds
// no interior candidate at all still gets a menu, and it is the incumbent plus
// whatever the other side offers. A short side lends nothing to the long one —
// the menu is a neighbourhood of the boundary, not a nearest-seven list that
// would reach seven candidates deep into one section.
func TestMenuShortSideContributesWhatItHas(t *testing.T) {
	src, cands := buildDoc(append(
		[]blk{{survey.CutHeading, "# One\n\n" + words("q", 400)}},
		denseBlocks(40)...)...)
	span := wholeSpan(src)
	cuts := []survey.Span{{Start: 0, End: cands[0].Offset}, {Start: cands[0].Offset, End: span.End}}
	p := params(1_000_000)

	r, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, log.Discard())
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	b := boundariesOf(t, r)[0]

	before := 0
	for _, c := range b.Menu {
		if c.Offset < b.Cut {
			before++
		}
	}
	if before != 0 {
		t.Errorf("%d menu entries before the cut; the left neighbour holds no candidates", before)
	}
	if menuNumber(b, b.Cut) != 1 {
		t.Errorf("menu = %+v, want the incumbent first when nothing precedes it", b.Menu)
	}
	if len(b.Menu) != menuSide+1 {
		t.Errorf("menu holds %d entries, want the incumbent plus %d after it", len(b.Menu), menuSide)
	}
}

// TestParseChoiceRequiresANumber: the response must BE a menu number.
//
// The forgiving reading — first run of digits anywhere — looks kinder and is
// not: the prompt is full of numbers the model can echo, and a mis-mapped
// choice is a legal menu entry, so it verifies and is written. That is the one
// failure the two nets cannot see, because both look at the offset and the
// mis-mapping happened before the offset existed.
func TestParseChoiceRequiresANumber(t *testing.T) {
	const size = 5
	for _, tc := range []struct {
		name     string
		response string
		want     int
	}{
		{"a bare number", "3", 3},
		{"surrounded by whitespace", "  3\n", 3},
		{"with a full stop", "3.", 3},
		{"with closing punctuation", "3)", 3},
		{"a number in a sentence", "GroupingAnswer: 3", 0},
		{"a leading count the model echoed", "Boundary 2 of 3 — I would move it to 1", 0},
		{"prose with no number", "I do not think either is right.", 0},
		{"empty", "", 0},
		{"below the menu", "0", 0},
		{"past the menu", "99", 0},
		{"two numbers", "3 4", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseChoice(tc.response, size)
			switch {
			case tc.want == 0 && err == nil:
				t.Fatalf("parseChoice(%q) = %d, want a rejection", tc.response, got)
			case tc.want == 0:
				assertNoOffsets(t, err.Error(), 1234, 8392)
				assertNoMenuNumbers(t, err.Error())
			case err != nil:
				t.Fatalf("parseChoice(%q): %v", tc.response, err)
			case got != tc.want:
				t.Errorf("parseChoice(%q) = %d, want %d", tc.response, got, tc.want)
			}
		})
	}
}

// TestBoundaryArtifactsAreBoundToTheirParameters is §12's one prohibited
// failure closed at this seam: a stamp must not prove something it does not
// prove.
//
// A boundary artifact is an offset adjudicated for ONE question — this span,
// this budget, this mechanical cut list. A re-plan that changes any of them
// asks a different question at the same unit path, and without the parameter
// digest the resume scan would verdict the stale answer Valid and the run
// would reuse an offset adjudicated for a boundary that no longer exists.
func TestBoundaryArtifactsAreBoundToTheirParameters(t *testing.T) {
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	p := params(tokensOf(string(src[:cands[1].Offset])) + 10)
	cuts, err := Split(src, span, cands, p)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	// A run that adjudicates the boundary and leaves its artifact behind.
	first, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, log.Discard())
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	number, _ := alternative(t, boundariesOf(t, first)[0])
	client := model.NewScriptedMock([]model.Response{{Content: fmt.Sprint(number), FinishReason: "stop"}}, nil)
	res, dir, err := runStage(t, first, client, log.Discard())
	if err != nil || res.Produced != 1 {
		t.Fatalf("the first run did not produce the boundary: %+v %v", res, err)
	}

	// A second cut list over the same document with the same boundary count,
	// so the unit PATHS collide exactly. Only the parameters differ.
	moved := []survey.Span{{Start: 0, End: cands[0].Offset}, {Start: cands[0].Offset, End: span.End}}

	for _, tc := range []struct {
		name  string
		cuts  []survey.Span
		p     Params
		reuse bool
	}{
		{"the same parameters", cuts, p, true},
		{"a different budget", cuts, params(p.BudgetTokens + 1), false},
		{"a different mechanical cut list", moved, p, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRefiner(src, span, cands, tc.cuts, stageName, tc.p, testEffort, log.Discard())
			if err != nil {
				t.Fatalf("NewRefiner: %v", err)
			}
			scan := scanOf(t, dir, r)
			if tc.reuse {
				// A stage whose every unit is proven is complete, so the scan
				// reports it as reused rather than verdicting it unit by unit.
				if !scan.Complete() || scan.Reused != 1 {
					t.Errorf("scan = %+v, want the unchanged boundary reused", scan)
				}
				return
			}
			if len(scan.Verdicts) != 1 {
				t.Fatalf("%d verdicts, want the one boundary", len(scan.Verdicts))
			}
			v := scan.Verdicts[0]
			if v.Verdict != pipeline.VerdictInvalid {
				t.Errorf("verdict = %s, want the stale boundary redone", v.Verdict)
			}
			if !strings.Contains(v.Reason, paramsInput) {
				t.Errorf("reason = %q, want it to name the parameters that changed", v.Reason)
			}
		})
	}
}

// TestStagePlanCarriesTheDeclaredEffort: the ask this stage registers asks
// with the effort its CONSTRUCTOR was given.
//
// The stage has no standing to hold an opinion here — the effort belongs to
// the definition, and the definition is registered outside this package — so
// what is asserted is pass-through, not a value. testEffort is deliberately
// the opposite of what the real registration declares, which is what makes
// pass-through distinguishable from a default.
func TestStagePlanCarriesTheDeclaredEffort(t *testing.T) {
	r, _, _ := oneBoundary(t, log.Discard())
	got := r.StagePlan(stageName, nil).Ask.Effort
	if got != testEffort {
		t.Errorf("ask effort = %+v, want the declaration NewRefiner was given (%+v)", got, testEffort)
	}
}

// scanOf runs the resume scan a job over this refiner would run.
func scanOf(t *testing.T, dir string, r *Refiner) pipeline.ResumeScanResult {
	t.Helper()
	plan := pipeline.Plan{JobFrame: jobFrame, Stages: []*pipeline.StagePlan{
		r.StagePlan(stageName, []pipeline.Input{{Name: "corpus", Hash: pipeline.HashBytes([]byte("fixture"))}}),
	}}
	scan, err := storeIn(t, dir, log.Discard()).ResumeScan(plan.StageChain(), pipeline.ModeResume)
	if err != nil {
		t.Fatalf("ResumeScan: %v", err)
	}
	return scan
}

// TestVerifyBlamesTheModelOnlyForRejections: the two-class taxonomy is
// exhaustive by construction rather than by enumeration.
//
// RejectionError is the only class a model's answer can be responsible for, so
// everything else is ours. The classes are not symmetric in cost: a defect
// routed to the model burns a retry and then emits a degraded unit derived
// from a broken derivation, while a model failure routed to the defect path
// stops the job loudly. The default therefore has to be self-blame.
// A fresh Refiner per case, which is not incidental: composing a cut list
// spends the Refiner (Refiner.enter), so the accept case cannot be followed by
// another adjudication on the same instance. That is the constraint under test
// in TestARefinerIsOneRuns.
func TestVerifyBlamesTheModelOnlyForRejections(t *testing.T) {
	accepted, _, ab := oneBoundary(t, log.Discard())
	number, _ := alternative(t, ab)
	if _, err := accepted.verify(ab.Unit, fmt.Sprint(number)); err != nil {
		t.Fatalf("a legal choice must verify: %v", err)
	}

	// A rejection: the model's answer, retried once and then discarded for the
	// fallback. It must NOT be a defect, or a bad answer would stop the job.
	r, _, b := oneBoundary(t, log.Discard())
	_, err := r.verify(b.Unit, "99")
	if err == nil {
		t.Fatal("a choice outside the menu must be rejected")
	}
	if errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Errorf("err = %v, want a rejection rather than a defect", err)
	}
	var rej RejectionError
	if !errors.As(err, &rej) {
		t.Errorf("err = %v (%T), want the operator-facing RejectionError underneath", err, err)
	}

	// A unit this stage does not own. No response could cause it — the unit
	// table and the task list are built from one loop — so it is ours.
	if _, err := r.verify("cuts/9999"+boundarySuffix, fmt.Sprint(number)); !errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Errorf("err = %v, want a verifier defect for a unit this stage does not own", err)
	}

	// A plain error out of Verify — a window count that does not match the
	// boundaries. Unreachable by construction today, which is exactly why the
	// classification must not depend on anyone enumerating it.
	r.windows = append(r.windows, MoveWindow{})
	if _, err := r.verify(b.Unit, fmt.Sprint(number)); !errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Errorf("err = %v, want a plain verifier error classified as our defect", err)
	}
}

// TestNoNoteHandsTheModelALegalAnswer: every retry note the fold can send
// back is a PROMPT, and the model's whole vocabulary here is a menu number.
//
// The rejection reasons the parser produces are the ones with a number in
// reach — a message naming how many positions were listed names one of the
// legal answers — so this drives them through the fold's own reject path and
// reads the Note the retry would carry. Verify's own reasons are covered where
// they are constructed (TestVerifyRejections and its siblings).
func TestNoNoteHandsTheModelALegalAnswer(t *testing.T) {
	for _, response := range []string{"", "somewhere in the middle", "0", "99", "3 4", "GroupingAnswer: 2"} {
		t.Run(response, func(t *testing.T) {
			r, _, b := oneBoundary(t, log.Discard())
			_, err := r.verify(b.Unit, response)
			var rej RejectionError
			if !errors.As(err, &rej) {
				t.Fatalf("err = %v (%T), want a rejection for %q", err, err, response)
			}
			assertNoMenuNumbers(t, rej.Note())
			assertNoOffsets(t, rej.Note(), b.Cut, b.Window.Lo, b.Window.Hi)
		})
	}
}

// TestARefinerIsOneRuns: a Refiner is per RUN, and this is what makes that
// structural rather than a convention the tests happen to keep.
//
// Its working list, windows and counters are seeded once and never reset, so a
// second fold over the same instance would restart from a half-folded list and
// re-adjudicate boundary 1 against positions later boundaries had already
// moved to. The composed list would still verify — which is exactly the
// problem: nothing would be loud about an answer to a question nobody asked.
func TestARefinerIsOneRuns(t *testing.T) {
	r, _, b := oneBoundary(t, log.Discard())
	number, _ := alternative(t, b)
	if _, err := r.verify(b.Unit, fmt.Sprint(number)); err != nil {
		t.Fatalf("the first run must adjudicate: %v", err)
	}

	_, err := r.verify(b.Unit, fmt.Sprint(number))
	if !errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Fatalf("err = %v, want a defect: a composed Refiner is spent", err)
	}
	if !strings.Contains(err.Error(), "new Refiner per run") {
		t.Errorf("err = %v, want the remedy named", err)
	}
}

// TestTheFoldRefusesABoundaryOutOfTurn: the ordinal half of the single-owner
// assertion, at the seam it exists for.
//
// The exclusivity CAS alone covers the WRITE path. The read that builds a
// boundary's prompt happens earlier, in the worker goroutine, so two lanes
// over one Refiner would race on the working list before any CAS could fire
// and the menu they produced would already be derived from a torn read. One
// int catches it from either side.
func TestTheFoldRefusesABoundaryOutOfTurn(t *testing.T) {
	src, cands := scanDoc()
	span := wholeSpan(src)
	p := params(250)
	cuts, err := Split(src, span, cands, p)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	r, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, log.Discard())
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	bs := boundariesOf(t, r)
	if len(bs) < 2 {
		t.Fatalf("%d boundaries; the case needs a fold with a second step", len(bs))
	}

	// The second boundary asked first: a legal answer to a question the fold
	// has not reached.
	second := bs[1]
	number := menuNumber(second, second.Cut)
	if number == 0 {
		t.Fatalf("boundary %+v does not offer its own cut", second)
	}
	_, err = r.verify(second.Unit, fmt.Sprint(number))
	if !errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Fatalf("err = %v, want a defect for a boundary out of turn", err)
	}

	// And the retry does NOT trip it: a rejection is not terminal, so the fold
	// still expects the same boundary.
	fresh, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, log.Discard())
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	first := boundariesOf(t, fresh)[0]
	if _, err := fresh.verify(first.Unit, "not a number"); errors.Is(err, pipeline.ErrVerifierDefect) {
		t.Fatalf("err = %v, want a plain rejection", err)
	}
	if _, err := fresh.verify(first.Unit, fmt.Sprint(menuNumber(first, first.Cut))); err != nil {
		t.Errorf("the informed retry must re-ask the same boundary: %v", err)
	}
}

// TestCutListRoundTrip: one encoder, one decoder, and the composed list a
// later run reads is the one this run wrote.
func TestCutListRoundTrip(t *testing.T) {
	src, cands := scanDoc()
	span := wholeSpan(src)
	cuts, err := Split(src, span, cands, params(250))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	data, err := encodeCutList(CutList{Unit: "cuts/cutlist.txt", Cuts: cuts})
	if err != nil {
		t.Fatalf("encodeCutList: %v", err)
	}
	got, err := DecodeCutList(data)
	if err != nil {
		t.Fatalf("DecodeCutList: %v", err)
	}
	if !slices.Equal(got, cuts) {
		t.Errorf("round trip = %+v, want %+v", got, cuts)
	}

	for _, tc := range []struct{ name, data string }{
		{"empty", ""},
		{"one number on a line", "0 10\n20\n"},
		{"a word where an offset belongs", "0 ten\n"},
		{"a third column", "0 10 20\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeCutList([]byte(tc.data)); err == nil {
				t.Errorf("DecodeCutList(%q) succeeded; the format is the contract between two runs", tc.data)
			}
		})
	}
}

// TestNewRefinerRefusesAnEmptyUnitDirectory: every unit path is built from it,
// so an empty one yields "/0001.cut" — refused by the store much later, naming
// the artifact rather than the caller who never said where the work goes.
func TestNewRefinerRefusesAnEmptyUnitDirectory(t *testing.T) {
	src, cands := nearBoundaryDoc()
	span := wholeSpan(src)
	p := params(tokensOf(string(src[:cands[1].Offset])) + 10)
	cuts, err := Split(src, span, cands, p)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if _, err := NewRefiner(src, span, cands, cuts, "", p, testEffort, log.Discard()); err == nil {
		t.Fatal("a stage with nowhere to write its boundaries must be refused")
	}
}

// scanDoc is the multi-boundary fixture: five blocks, so a scan has more than
// one step and a kill can land in the middle of one.
func scanDoc() ([]byte, []survey.CutCandidate) {
	return buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 200)},
		blk{survey.CutParagraph, words("b", 30)},
		blk{survey.CutHeading, "# Three\n\n" + words("c", 200)},
		blk{survey.CutParagraph, words("d", 30)},
		blk{survey.CutHeading, "# Five\n\n" + words("e", 200)},
	)
}

// slackDoc is §5's counterexample as a document: a section only just over the
// minimum, with a candidate a few tokens inside each of its two ends. Both
// moves are legal against the mechanical list; together they are not.
func slackDoc() ([]byte, []survey.CutCandidate) {
	return buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 300)},
		blk{survey.CutParagraph, words("b", 6)},
		blk{survey.CutParagraph, words("c", 58)},
		blk{survey.CutParagraph, words("d", 6)},
		blk{survey.CutHeading, "# Five\n\n" + words("e", 300)},
	)
}

// tokensIn is the estimated size of a section of src.
func tokensIn(src []byte, r survey.Span) int { return params(0).estimate(src[r.Start:r.End]) }

// logField returns the value the first record at level carrying key holds.
// logtest matches key/value pairs; this reads one out, for the metrics whose
// exact value is the assertion's subject rather than its predicate.
func logField(t *testing.T, lg *logtest.Capture, level, key string) any {
	t.Helper()
	for _, rec := range lg.Snapshot() {
		if rec.Level != level {
			continue
		}
		for i := 0; i+1 < len(rec.Args); i += 2 {
			if k, ok := rec.Args[i].(string); ok && k == key {
				return rec.Args[i+1]
			}
		}
	}
	t.Fatalf("no %s record carries %q", level, key)
	return nil
}

// TestFoldRefusesTheSecondMoveIntoSpentSlack is the counterexample the ruling
// turned on (ARCHITECTURE.md §5, W2).
//
// The middle section is 70-odd tokens against a 64-token floor. Its left
// boundary moves a few tokens in and its right boundary is then offered a
// symmetric move back. Against the MECHANICAL list — the stateless design —
// each move verifies on its own and the composition is under the floor. The
// fold judges the second one against the list the first one produced, so the
// slack is spent, the move is refused, and the boundary stays where it is.
//
// Nothing is rationed and no freedom is halved: the first boundary adjudicated
// gets the whole of the slack, and the second is told, in the retry note,
// exactly what the answer was wrong about.
func TestFoldRefusesTheSecondMoveIntoSpentSlack(t *testing.T) {
	src, cands := slackDoc()
	span := wholeSpan(src)
	// A stated cut list rather than a searched one: the case is about the
	// sizes, and Split choosing differently would make it about Split.
	p := params(1_000_000)
	cuts := []survey.Span{
		{Start: 0, End: cands[0].Offset},
		{Start: cands[0].Offset, End: cands[3].Offset},
		{Start: cands[3].Offset, End: span.End},
	}

	// The fixture IS the counterexample, or the test proves nothing. Each move
	// alone leaves the middle section over the floor; together they do not.
	left := []survey.Span{{Start: 0, End: cands[1].Offset}, {Start: cands[1].Offset, End: cands[3].Offset}, cuts[2]}
	right := []survey.Span{cuts[0], {Start: cands[0].Offset, End: cands[2].Offset}, {Start: cands[2].Offset, End: span.End}}
	both := []survey.Span{{Start: 0, End: cands[1].Offset}, {Start: cands[1].Offset, End: cands[2].Offset}, {Start: cands[2].Offset, End: span.End}}
	if got := tokensIn(src, cuts[1]); got < minTokens {
		t.Fatalf("the middle section is %d tokens; the fixture must start over the %d-token floor", got, minTokens)
	}
	for _, one := range []struct {
		name string
		cuts []survey.Span
	}{{"the left move alone", left}, {"the right move alone", right}} {
		if err := Verify(src, span, cands, one.cuts, nil, p); err != nil {
			t.Fatalf("%s must be legal on its own, or the case is not the counterexample: %v", one.name, err)
		}
	}
	err := Verify(src, span, cands, both, nil, p)
	if err == nil {
		t.Fatalf("both moves compose to a %d-token section; the fixture is not the counterexample", tokensIn(src, both[1]))
	}
	// And for the RIGHT reason: the minimum is the rule that does not survive
	// composition, and a fixture that failed tiling or membership instead
	// would be testing a different claim.
	if !strings.Contains(err.Error(), "minimum") {
		t.Fatalf("the composition fails with %v, want the minimum section size", err)
	}

	lg := &logtest.Capture{}
	r, rerr := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, lg)
	if rerr != nil {
		t.Fatalf("NewRefiner: %v", rerr)
	}
	bs := boundariesOf(t, r)
	if len(bs) != 2 {
		t.Fatalf("%d boundaries, want the two ends of the middle section", len(bs))
	}
	first := menuNumber(bs[0], cands[1].Offset)
	second := menuNumber(bs[1], cands[2].Offset)
	if first == 0 || second == 0 {
		t.Fatalf("the moves are not on the menus (%+v, %+v)", bs[0].Menu, bs[1].Menu)
	}

	// The left boundary takes the slack; the right one asks for it twice and
	// is refused twice, which exhausts its attempts and keeps it where it is.
	client := model.NewScriptedMockPerConsult([]model.Response{
		{Content: fmt.Sprint(first), FinishReason: "stop"},
		{Content: fmt.Sprint(second), FinishReason: "stop"},
		{Content: fmt.Sprint(second), FinishReason: "stop"},
	})
	res, dir, err := runStage(t, r, client, lg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || res.FallbackCount != 1 || len(res.Failures) != 0 {
		t.Fatalf("result = %+v, want one composed list with one degraded boundary", res)
	}
	if !res.DeliveryReady() {
		t.Error("a refused move is a degraded refinement, not a broken job")
	}

	got := cutListIn(t, dir, bs[1].Unit)
	if len(got) != len(left) || got[1] != left[1] {
		t.Errorf("composed list = %+v, want the left move kept and the right one refused (%+v)", got, left)
	}
	assertVerifies(t, src, span, cands, got, p)
	if n := lg.Count("info", "outcome", "rejected"); n != 2 {
		t.Errorf("%d rejections recorded, want the two attempts at the spent slack", n)
	}
	if !lg.Has(t, "info", "outcome", "fallback") {
		t.Error("the refused boundary was not recorded as a fallback")
	}
}

// TestAnInterruptedFoldIsRedoneWhole is the resume ruling made falsifiable: a
// kill in the middle of the fold leaves NOTHING behind, and the next run
// re-adjudicates every boundary and produces a cut list an uninterrupted run
// would have produced.
//
// The absence is the design. A per-boundary artifact would be individually
// provable and individually reusable, and boundary i+1 was judged against
// boundary i's accepted position — a dependency no stamp carries — so a
// partial reuse would compose an answer to a question nobody asked. Stage
// granularity is what makes that unrepresentable rather than merely avoided,
// and the cost of it is in the log: the fold records what it re-spent.
func TestAnInterruptedFoldIsRedoneWhole(t *testing.T) {
	src, cands := scanDoc()
	span := wholeSpan(src)
	p := params(250)
	cuts, err := Split(src, span, cands, p)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	// The answer for every boundary is its own incumbent, so an uninterrupted
	// run and a resumed one agree on the whole list and the subject stays the
	// resume.
	script := func(r *Refiner) []model.Response {
		var out []model.Response
		for _, b := range boundariesOf(t, r) {
			n := menuNumber(b, b.Cut)
			if n == 0 {
				t.Fatalf("boundary %+v does not offer its own cut", b)
			}
			out = append(out, model.Response{Content: fmt.Sprint(n), FinishReason: "stop"})
		}
		return out
	}

	dir := t.TempDir()
	first, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, log.Discard())
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	boundaries := boundariesOf(t, first)
	if len(boundaries) < 2 {
		t.Fatalf("%d boundaries; a mid-fold kill needs a fold with a middle", len(boundaries))
	}
	unit := boundaries[len(boundaries)-1].Unit

	// Hit 2 is the second call's transport: the first boundary is adjudicated
	// and folded in, and the run dies before the second one is asked.
	disarm := crashpoint.Arm("pipeline.runner.pre-transport", crashpoint.ArmOnHit(2))
	_, err = runStageIn(t, dir, first, model.NewScriptedMockPerConsult(script(first)), log.Discard())
	disarm()
	var crash *crashpoint.Crash
	if !errors.As(err, &crash) {
		t.Fatalf("err = %v, want the armed crash", err)
	}
	if _, err := storeIn(t, dir, log.Discard()).Get(unit); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the killed fold left %s behind (err = %v); a half-fold must not be on disk", unit, err)
	}
	// The in-process crash unwinds through the lock's release, so this is the
	// remedy a real kill would need rather than one this test always uses.
	if err := os.Remove(storePath(dir, pipeline.LockFileName)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("clear the lockfile: %v", err)
	}

	// The resume: a new process's refiner over the same span, same directory.
	lg := &logtest.Capture{}
	second, err := NewRefiner(src, span, cands, cuts, stageName, p, testEffort, lg)
	if err != nil {
		t.Fatalf("NewRefiner: %v", err)
	}
	client := model.NewScriptedMockPerConsult(script(second))
	client.RecordCalls = true
	res, err := runStageIn(t, dir, second, client, lg)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.Produced != 1 || !res.DeliveryReady() {
		t.Fatalf("result = %+v, want the composed cut list", res)
	}
	list := cutListIn(t, dir, unit)
	assertVerifies(t, src, span, cands, list, p)
	if len(list) != len(cuts) {
		t.Errorf("composed list holds %d sections, want the %d an uninterrupted run produces", len(list), len(cuts))
	}

	// The redo's cost, which is the whole of what finer-grained salvage would
	// have saved: every boundary asked again, and the tokens that went with
	// them. It is one number per fold precisely because resume is
	// stage-granular — there is no prefix to price separately.
	if n := len(client.Calls()); n != len(boundaries) {
		t.Errorf("the resumed run made %d calls, want all %d boundaries re-adjudicated", n, len(boundaries))
	}
	if got := logField(t, lg, "info", "boundaries"); fmt.Sprint(got) != fmt.Sprint(len(boundaries)) {
		t.Errorf("the fold recorded %v boundaries, want %d", got, len(boundaries))
	}
	spent, ok := logField(t, lg, "info", "adjudicated_tokens").(int)
	if !ok || spent <= 0 {
		t.Errorf("adjudicated_tokens = %v; the redo's cost must be recorded", spent)
	}
}

// TestComposedCutListFailureIsADefect: the whole-list check before the write
// is unreachable by construction — every accepted move verified the same list
// — so what it guards is the fold's own bookkeeping. If it ever fires, no
// answer the model gave is responsible for it, so it must abort rather than
// retry or fall back.
//
// It is reached here by corrupting the working list directly, which is the
// only way to reach it and is exactly the class of bug it exists for.
func TestComposedCutListFailureIsADefect(t *testing.T) {
	t.Run("through the verified path", func(t *testing.T) {
		r, _, b := oneBoundary(t, log.Discard())
		r.work[0].End--
		if _, err := r.compose(b.Unit); !errors.Is(err, pipeline.ErrVerifierDefect) {
			t.Errorf("err = %v, want the composed list's failure classified as our defect", err)
		}
	})
	t.Run("through the fallback path", func(t *testing.T) {
		r, _, b := oneBoundary(t, log.Discard())
		r.work[0].End--
		// MechanicalFallback cannot refuse, so it refuses by producing nothing the
		// encoder will write — a loud write failure rather than a cut list
		// nobody verified.
		if _, err := encodeCutList(r.fallback(b.Unit)); err == nil {
			t.Error("an unverifiable composition must not encode into an artifact")
		}
	})
}
