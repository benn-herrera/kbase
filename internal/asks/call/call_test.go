package call

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/log"
	"kbase/internal/model"
)

func paragraphItems(names ...string) []asks.LetterItem {
	items := make([]asks.ParagraphItem, len(names))
	for i, n := range names {
		items[i] = asks.ParagraphItem{Paragraph: n, Text: n + ": text"}
	}
	return asks.ParagraphAsks("v/leaf.md", "the leaf", items)
}

// decisions serves the answers scripted per question key, one per call.
type decisions struct {
	mu      sync.Mutex
	answers []model.DecisionAnswer
	calls   []model.DecisionRequest
}

func (d *decisions) Decision(_ context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, req)
	if len(d.answers) == 0 {
		return model.DecisionResponse{}, errors.New("no answer scripted")
	}
	a := d.answers[0]
	d.answers = d.answers[1:]
	return model.DecisionResponse{Answers: []model.DecisionAnswer{a}}, nil
}

func (d *decisions) Consult(context.Context, model.Request) (model.Response, error) {
	return model.Response{}, errors.New("the decision asks go over Decision")
}

func (d *decisions) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	return nil, errors.New("the decision asks go over Decision")
}

func (d *decisions) ListModels(context.Context) ([]model.ModelInfo, error) { return nil, nil }

// noulAnswer is a noul answer of p, keyed as the seam keys its questions: by
// the item's name.
func noulAnswer(key string, p float64) model.DecisionAnswer {
	return model.DecisionAnswer{Key: key, Type: "noul", Noul: p}
}

// ask asks the two paragraph items of one group over client and returns the
// record.
func ask(t *testing.T, client model.Client, scratch string) asks.GroupRecord {
	t.Helper()
	read := New(client, scratch, log.Discard()).Letters("m-decision")
	rec, err := asks.AskGroup(context.Background(), asks.Paragraph, "v/leaf.md", paragraphItems("S1", "S2"), read, 0, filepath.Join(scratch, asks.AskRecordsDir))
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestCallerMapsAnswersToConfidenceAndAsksNothingTwice(t *testing.T) {
	scratch := t.TempDir()
	served := &decisions{answers: []model.DecisionAnswer{noulAnswer("S1", 0.96), noulAnswer("S2", 0.04)}}
	rec := ask(t, served, scratch)
	if *rec.Items[0].Letter != "A" || *rec.Items[1].Letter != "B" {
		t.Fatalf("letters = %v, %v", *rec.Items[0].Letter, *rec.Items[1].Letter)
	}
	// The noul figure sits under the true letter, the remainder under the
	// false one — keyed by which criterion the letter names, never by order.
	if c := rec.Items[0].Confidence; c["A"] != 0.96 || math.Abs(c["B"]-0.04) > 1e-9 {
		t.Errorf("S1's confidence = %v, want A 0.96 and B 0.04", c)
	}
	if len(served.calls) != 2 {
		t.Fatalf("%d requests, want 2", len(served.calls))
	}
	req := served.calls[0]
	if req.Model != "m-decision" || len(req.Questions) != 1 {
		t.Fatalf("request = %+v", req)
	}
	q := req.Questions[0]
	if q.Key != "S1" || q.Type != "noul" || q.Criteria["true"] == "" || q.Criteria["false"] == "" {
		t.Errorf("question = %+v, want one noul question keyed by the item with both criteria", q)
	}
	if req.State == "" {
		t.Error("the composed state is not the request's state")
	}
	captures, _ := filepath.Glob(filepath.Join(scratch, CapturesDir, "paragraph-v_leaf.md-S*.capture.jsonl"))
	if len(captures) != 2 {
		t.Fatalf("captures = %q", captures)
	}
	b, _ := os.ReadFile(captures[0])
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); len(lines) != 2 || !strings.Contains(lines[0], `"type":"request"`) ||
		!strings.Contains(lines[0], `"state-sha256"`) || !strings.Contains(lines[1], `"outcome":"ok"`) {
		t.Errorf("capture = %s", b)
	}
	answers, _ := filepath.Glob(filepath.Join(scratch, AnswersDir, "*.json"))
	if len(answers) != 2 {
		t.Errorf("cached answers = %q, want one per completed call", answers)
	}

	// A resume over the cache makes no request and decides identically.
	again := ask(t, &decisions{}, scratch)
	if *again.Items[0].Letter != "A" || *again.Items[1].Letter != "B" {
		t.Error("the cached answers decided differently")
	}

	// A different model's answers are not another's.
	other := &decisions{answers: []model.DecisionAnswer{noulAnswer("S1", 0.96), noulAnswer("S2", 0.04)}}
	ask(t, other, scratch)
}

// TestCallerMapsAChoiceAnswerAndItsFallbacks: the probabilities map carries
// whole; without it the answer's own confidence sits under the chosen
// letter; with neither, the answer is unreadable and defaults.
func TestCallerMapsAChoiceAnswerAndItsFallbacks(t *testing.T) {
	scratch := t.TempDir()
	served := &decisions{answers: []model.DecisionAnswer{
		{Key: "S1", Type: "choice", Choice: "A", Probabilities: map[string]float64{"A": 0.9, "B": 0.1}},
		{Key: "S2", Type: "choice", Choice: "B", Confidence: 0.8},
	}}
	read := New(served, scratch, log.Discard()).Letters("m-decision")
	rec, err := asks.AskGroup(context.Background(), asks.Paragraph, "v/leaf.md", paragraphItems("S1", "S2"), read, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if *rec.Items[0].Letter != "A" {
		t.Errorf("S1 = %s, want A over its probabilities", *rec.Items[0].Letter)
	}
	if *rec.Items[1].Letter != "B" {
		t.Errorf("S2 = %s, want B, its own confidence under the chosen letter", *rec.Items[1].Letter)
	}
	if c := rec.Items[1].Confidence; c["B"] != 0.8 || len(c) != 1 {
		t.Errorf("S2's confidence = %v, want B alone", c)
	}

	empty := &decisions{answers: []model.DecisionAnswer{{Key: "S1", Type: "choice"}, {Key: "S1", Type: "choice"}}}
	read = New(empty, t.TempDir(), log.Discard()).Letters("m-decision")
	rec, err = asks.AskGroup(context.Background(), asks.Paragraph, "v/leaf.md", paragraphItems("S1"), read, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Items[0].Outcome != asks.Defaulted {
		t.Errorf("S1 = %s, want the unreadable answer defaulted", rec.Items[0].Outcome)
	}
}

// TestAReaskReachesTheModel: a re-ask of an unreadable answer is the
// identical question, its attempt ordinal keeping it out of the answer
// cache, and a deterministic second answer defaults the item.
func TestAReaskReachesTheModel(t *testing.T) {
	scratch := t.TempDir()
	// The transport re-serves the same unreadable answer: the re-ask reads
	// it again and the item defaults on the second pass.
	same := &decisions{answers: []model.DecisionAnswer{{Key: "S1", Type: "choice"}, {Key: "S1", Type: "choice"}}}
	read := New(same, scratch, log.Discard()).Letters("m-decision")
	rec, err := asks.AskGroup(context.Background(), asks.Paragraph, "v/leaf.md", paragraphItems("S1"), read, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(same.calls) != 2 || rec.Items[0].Outcome != asks.Defaulted {
		t.Fatalf("%d calls, outcome %s; want the re-ask to reach the model and default", len(same.calls), rec.Items[0].Outcome)
	}
	if same.calls[1].State != same.calls[0].State || same.calls[0].Questions[0].Key != same.calls[1].Questions[0].Key {
		t.Error("the re-ask is not the identical question")
	}
	resumed := ask(t, &decisions{answers: []model.DecisionAnswer{noulAnswer("S1", 0.96), noulAnswer("S2", 0.04)}}, scratch)
	if len(resumed.Items[0].Calls) != 2 {
		t.Errorf("the resumed item holds %d calls, want both its asks", len(resumed.Items[0].Calls))
	}
	cached, _ := filepath.Glob(filepath.Join(scratch, AnswersDir, "*.json"))
	if len(cached) != 4 {
		t.Errorf("cached answers = %q, want one per ask including the re-asks", cached)
	}
}

// TestARefusalTakesTheReaskThenTheDefault: the Decisions surface's refusal
// answer says the model declined and carries no value at all, so the seam
// reads it as unreadable — the identical question asked once more, then the
// item defaulted when that is refused too.
func TestARefusalTakesTheReaskThenTheDefault(t *testing.T) {
	refusals := &decisions{answers: []model.DecisionAnswer{
		{Key: "S1", Type: "refusal"},
		{Key: "S1", Type: "refusal"},
	}}
	read := New(refusals, t.TempDir(), log.Discard()).Letters("m-decision")
	rec, err := asks.AskGroup(context.Background(), asks.Paragraph, "v/leaf.md", paragraphItems("S1"), read, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(refusals.calls) != 2 {
		t.Fatalf("%d calls, want the refusal re-asked once", len(refusals.calls))
	}
	first, again := refusals.calls[0].Questions[0], refusals.calls[1].Questions[0]
	if refusals.calls[1].State != refusals.calls[0].State ||
		again.Key != first.Key || again.Type != first.Type || again.Instructions != first.Instructions {
		t.Error("the re-ask is not the identical question")
	}
	if rec.Items[0].Outcome != asks.Defaulted {
		t.Errorf("S1 = %s, want the refused answer defaulted", rec.Items[0].Outcome)
	}
}

func TestCallerNamesTheCallThatNeverCompleted(t *testing.T) {
	scratch := t.TempDir()
	served := &decisions{}
	read := New(served, scratch, log.Discard()).Letters("m-decision")
	_, err := asks.AskGroup(context.Background(), asks.Paragraph, "v/leaf.md", paragraphItems("S1"), read, 0, "")
	var incomplete asks.IncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("AskGroup = %v, want the incomplete call", err)
	}
	if answers, _ := filepath.Glob(filepath.Join(scratch, AnswersDir, "*")); len(answers) != 0 {
		t.Errorf("a failed call cached %q", answers)
	}
}

// TestDecideHoldsTheLettersTheQuestionOffers: the question's true-letter is
// which offered criterion "true" is, and the composed state carries the slot
// values the record's readers name.
func TestDecideHoldsTheLettersTheQuestionOffers(t *testing.T) {
	items := paragraphItems("S1")
	if items[0].Question.Type != "noul" || items[0].Question.TrueLetter != asks.LetterClaim {
		t.Fatalf("question = %+v, want noul with %s true", items[0].Question, asks.LetterClaim)
	}
	state, err := items[0].Compose()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state, "S1: text") || strings.Contains(state, "@!") {
		t.Errorf("the composed state = %q, want its paragraph's text and no unfilled slot", state)
	}
}
