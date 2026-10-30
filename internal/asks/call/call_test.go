package call

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kbase/internal/asks"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// trailDir is the one trail directory a test's scratch holds.
func trailDir(scratch string) string {
	return filepath.Join(scratch, "trails", "t")
}

// trailAnswers is the answers directory of the one trail a test's scratch holds.
func trailAnswers(scratch string) string {
	return filepath.Join(trailDir(scratch), asks.AnswersDir)
}

// trailCaptures is the captures directory of the one trail a test's scratch holds.
func trailCaptures(scratch string) string {
	return filepath.Join(trailDir(scratch), asks.CapturesDir)
}

// double is a test client, answering through the model.Client it gives.
type double interface{ Client() model.Client }

// newCaller is a Caller over d whose answers and captures go to scratch's one trail.
func newCaller(d double, scratch string) *Caller {
	return New(d.Client(), func() string { return trailDir(scratch) }, log.Discard())
}

func paragraphItems(names ...string) []asks.MultichoiceItem {
	items := make([]asks.ParagraphItem, len(names))
	for i, n := range names {
		items[i] = asks.ParagraphItem{Paragraph: n, Text: n + ": text"}
	}
	return asks.ParagraphAsks("v/leaf.md", "the leaf", items)
}

// letters is c's Launch for the decision model m-decision, on a queue of two.
func letters(c *Caller) asks.Launch { return c.Launch(model.NewQueue(2), "m-decision") }

// askGroup asks items as the one paragraph group of a batch launched through
// launch, and returns its record.
func askGroup(launch asks.Launch, items []asks.MultichoiceItem) (asks.GroupRecord, error) {
	b := asks.NewBatch(context.Background(), launch)
	if err := b.AskGroup(asks.Paragraph, "v/leaf.md", items); err != nil {
		_, err = b.Wait()
		return asks.GroupRecord{}, err
	}
	recs, err := b.Wait()
	if err != nil {
		return asks.GroupRecord{}, err
	}
	return recs[0], nil
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

func (d *decisions) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	return nil, errors.New("the decision asks go over Decision")
}

// Client is the model.Client whose calls d answers.
func (d *decisions) Client() model.Client {
	return &model.ClientFuncs{Decide: d.Decision, Stream: d.ConsultStream}
}

// noulAnswer is a noul answer of p, keyed as the seam keys its questions: by
// the item's name.
func noulAnswer(key string, p float64) model.DecisionAnswer {
	return model.DecisionAnswer{Key: key, Type: "noul", Noul: p}
}

// ask asks the two paragraph items of one group over d and returns the record.
func ask(t *testing.T, d double, scratch string) asks.GroupRecord {
	t.Helper()
	rec, err := askGroup(letters(newCaller(d, scratch)), paragraphItems("S1", "S2"))
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestCallerMapsAnswersToConfidenceAndAsksNothingTwice(t *testing.T) {
	scratch := t.TempDir()
	served := &decisions{answers: []model.DecisionAnswer{noulAnswer("S1", 0.96), noulAnswer("S2", 0.04)}}
	rec := ask(t, served, scratch)
	if *rec.Items[0].Choice != "A" || *rec.Items[1].Choice != "B" {
		t.Fatalf("letters = %v, %v", *rec.Items[0].Choice, *rec.Items[1].Choice)
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
	captures, _ := filepath.Glob(filepath.Join(trailCaptures(scratch), "paragraph-v_leaf.md-S*.capture.jsonl"))
	if len(captures) != 2 {
		t.Fatalf("captures = %q", captures)
	}
	b, _ := os.ReadFile(captures[0])
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); len(lines) != 2 || !strings.Contains(lines[0], `"type":"request"`) ||
		!strings.Contains(lines[0], `"state-sha256"`) || !strings.Contains(lines[1], `"outcome":"ok"`) {
		t.Errorf("capture = %s", b)
	}
	answers, _ := filepath.Glob(filepath.Join(trailAnswers(scratch), "*.json"))
	if len(answers) != 2 {
		t.Errorf("cached answers = %q, want one per completed call", answers)
	}

	// A resume over the cache makes no request and decides identically.
	again := ask(t, &decisions{}, scratch)
	if *again.Items[0].Choice != "A" || *again.Items[1].Choice != "B" {
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
	rec, err := askGroup(letters(newCaller(served, scratch)), paragraphItems("S1", "S2"))
	if err != nil {
		t.Fatal(err)
	}
	if *rec.Items[0].Choice != "A" {
		t.Errorf("S1 = %s, want A over its probabilities", *rec.Items[0].Choice)
	}
	if *rec.Items[1].Choice != "B" {
		t.Errorf("S2 = %s, want B, its own confidence under the chosen letter", *rec.Items[1].Choice)
	}
	if c := rec.Items[1].Confidence; c["B"] != 0.8 || len(c) != 1 {
		t.Errorf("S2's confidence = %v, want B alone", c)
	}

	empty := &decisions{answers: []model.DecisionAnswer{{Key: "S1", Type: "choice"}, {Key: "S1", Type: "choice"}}}
	rec, err = askGroup(letters(newCaller(empty, t.TempDir())), paragraphItems("S1"))
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
	rec, err := askGroup(letters(newCaller(same, scratch)), paragraphItems("S1"))
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
	cached, _ := filepath.Glob(filepath.Join(trailAnswers(scratch), "*.json"))
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
	rec, err := askGroup(letters(newCaller(refusals, t.TempDir())), paragraphItems("S1"))
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
	_, err := askGroup(letters(newCaller(served, scratch)), paragraphItems("S1"))
	var incomplete asks.IncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("AskGroup = %v, want the incomplete call", err)
	}
	if answers, _ := filepath.Glob(filepath.Join(trailAnswers(scratch), "*")); len(answers) != 0 {
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

// probed is a client whose calls run during over the scratch tree, so a test
// sees what exists on disk while a call is in flight.
type probed struct {
	decisions
	during func()
}

func (p *probed) Decision(ctx context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
	p.during()
	return p.decisions.Decision(ctx, req)
}

func (p *probed) ConsultStream(context.Context, model.Request) (model.StreamReader, error) {
	p.during()
	return doneStream{}, nil
}

// Client is the model.Client whose calls p answers.
func (p *probed) Client() model.Client {
	return &model.ClientFuncs{Decide: p.Decision, Stream: p.ConsultStream}
}

type doneStream struct{}

func (doneStream) Next() (model.Chunk, error) { return model.Chunk{}, io.EOF }
func (doneStream) Final() model.Response {
	return model.Response{Content: "the passage", FinishReason: "stop", StreamDone: true}
}
func (doneStream) Close() error { return nil }

// question is paragraph item's ask as a reader receives it, at attempt.
func question(item string, attempt int) asks.MultichoiceQuestion {
	it := paragraphItems(item)[0]
	q := it.Question
	q.Key = it.Item
	return asks.MultichoiceQuestion{Kind: asks.Paragraph, Group: "v/leaf.md", Item: it.Item, State: it.Item + " state",
		Question: q, Offered: it.Offered, Attempt: attempt}
}

// TestAnItemsFilesExistOnlyOnceItsAfterRuns pins the write rule through the
// seam: while any call of an item runs, none of the files its calls leave
// exists — its earlier call's included — and every one of them exists by the
// time the item's own after runs.
func TestAnItemsFilesExistOnlyOnceItsAfterRuns(t *testing.T) {
	files := func(scratch string) int {
		n := 0
		for _, dir := range []string{trailCaptures(scratch), trailAnswers(scratch)} {
			entries, _ := os.ReadDir(dir)
			n += len(entries)
		}
		return n
	}
	ctx := context.Background()
	// Each item makes two calls, as a re-ask or the overview's second ask
	// does, and hands its after back.
	items := map[string]func(t *testing.T, c *Caller, wg *sync.WaitGroup, after func(error)) error{
		"decide": func(t *testing.T, c *Caller, wg *sync.WaitGroup, after func(error)) error {
			return c.Launch(model.NewQueue(1), "m")(ctx, wg, func(read asks.MultichoiceReader) func(error) {
				for attempt := 1; attempt <= 2; attempt++ {
					if _, err := read(ctx, question("S1", attempt)); err != nil {
						t.Error(err)
					}
				}
				return after
			})
		},
		"ask": func(t *testing.T, c *Caller, wg *sync.WaitGroup, after func(error)) error {
			return c.PassageLaunch(model.NewQueue(1), "m")(ctx, wg, func(passage func(context.Context, string, []string, int) (string, error)) func(error) {
				for attempt := 1; attempt <= 2; attempt++ {
					if _, err := passage(ctx, "excerpts", nil, attempt); err != nil {
						t.Error(err)
					}
				}
				return after
			})
		},
	}
	for name, launch := range items {
		t.Run(name, func(t *testing.T) {
			scratch := t.TempDir()
			var during []int
			client := &probed{decisions: decisions{answers: []model.DecisionAnswer{noulAnswer("S1", 0.9), noulAnswer("S1", 0.9)}}}
			client.during = func() { during = append(during, files(scratch)) }
			atAfter := -1
			var wg sync.WaitGroup
			err := launch(t, newCaller(client, scratch), &wg, func(writeErr error) {
				if writeErr != nil {
					t.Error(writeErr)
				}
				atAfter = files(scratch)
			})
			if err != nil {
				t.Fatal(err)
			}
			wg.Wait()
			if !slices.Equal(during, []int{0, 0}) {
				t.Errorf("files during the item's calls = %v, want none during either", during)
			}
			if atAfter != 4 {
				t.Errorf("%d files when the item's after ran, want both calls' capture and answer", atAfter)
			}
		})
	}
}

// TestAnOverviewCutOffBeforeAnyTextIsNotAskedAgain: an overview answer that
// reached the completion cap with no text fails the ask on its one request,
// and leaves its capture, with the attempt's outcome, but no answer.
func TestAnOverviewCutOffBeforeAnyTextIsNotAskedAgain(t *testing.T) {
	scratch := t.TempDir()
	mock := modeltest.NewScriptedMock([]model.Response{{Content: "", FinishReason: "length"}}, nil)
	mock.RecordCalls = true
	var got error
	var wg sync.WaitGroup
	err := newCaller(mock, scratch).PassageLaunch(model.NewQueue(1), "m")(context.Background(), &wg, func(passage func(context.Context, string, []string, int) (string, error)) func(error) {
		_, got = passage(context.Background(), "excerpts", nil, 1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	var incomplete asks.IncompleteError
	if !errors.As(got, &incomplete) {
		t.Fatalf("ask = %v, want an asks.IncompleteError", got)
	}
	if n := len(mock.Calls()); n != 1 {
		t.Errorf("the model saw %d requests, want 1", n)
	}
	if answers, _ := filepath.Glob(filepath.Join(trailAnswers(scratch), "*")); len(answers) != 0 {
		t.Errorf("answers held = %v, want none", answers)
	}
	paths, _ := filepath.Glob(filepath.Join(trailCaptures(scratch), "*.capture.jsonl"))
	if len(paths) != 1 {
		t.Fatalf("captures = %q, want one", paths)
	}
	b, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var attempt struct {
		Type    string `json:"type"`
		Outcome string `json:"outcome"`
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("capture = %s, want a request line and one attempt line", b)
	}
	if err := json.Unmarshal([]byte(lines[1]), &attempt); err != nil || attempt.Type != "attempt" || attempt.Outcome != model.ChatTruncatedEmpty {
		t.Errorf("attempt line = %s, want outcome %q (%v)", lines[1], model.ChatTruncatedEmpty, err)
	}
}

// TestDecideReturnsItsContextsEnd: a call whose ctx ends while it is in
// flight — its batch stopped by a sibling — returns that ending, never an
// incomplete call that would read as its own inference failure.
func TestDecideReturnsItsContextsEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &probed{}
	client.during = cancel
	var got error
	var wg sync.WaitGroup
	scratch := t.TempDir()
	err := newCaller(client, scratch).Launch(model.NewQueue(1), "m")(ctx, &wg, func(read asks.MultichoiceReader) func(error) {
		_, got = read(ctx, question("S1", 1))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	var incomplete asks.IncompleteError
	if !errors.Is(got, context.Canceled) || errors.As(got, &incomplete) {
		t.Errorf("a cancelled call returned %v, want its context's error", got)
	}
	if left, err := filepath.Glob(filepath.Join(trailCaptures(scratch), "*")); err != nil || len(left) != 0 {
		t.Errorf("a cancelled call left captures %v (%v), want nothing written for it", left, err)
	}
}

// TestACaptureRecordsItsTokenWait: an item launched behind another on a queue
// of one records, in its capture, the wait for the token the first held; the
// first, which took a free token, records next to none.
func TestACaptureRecordsItsTokenWait(t *testing.T) {
	const hold = 100 * time.Millisecond
	scratch := t.TempDir()
	release := make(chan struct{})
	var calls atomic.Int32
	client := &probed{decisions: decisions{answers: []model.DecisionAnswer{noulAnswer("S1", 0.9), noulAnswer("S2", 0.9)}}}
	client.during = func() {
		if calls.Add(1) == 1 {
			<-release
		}
	}
	ctx := context.Background()
	launch := newCaller(client, scratch).Launch(model.NewQueue(1), "m")
	item := func(name string) func(asks.MultichoiceReader) func(error) {
		return func(read asks.MultichoiceReader) func(error) {
			if _, err := read(ctx, question(name, 1)); err != nil {
				t.Error(err)
			}
			return func(error) {}
		}
	}
	var wg sync.WaitGroup
	if err := launch(ctx, &wg, item("S1")); err != nil {
		t.Fatal(err)
	}
	var releasedAt time.Time
	go func() {
		time.Sleep(hold)
		releasedAt = time.Now()
		close(release)
	}()
	before := time.Now()
	if err := launch(ctx, &wg, item("S2")); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	waited := func(item string) int64 {
		t.Helper()
		paths, _ := filepath.Glob(filepath.Join(trailCaptures(scratch), "paragraph-v_leaf.md-"+item+"-*.capture.jsonl"))
		if len(paths) != 1 {
			t.Fatalf("%s's captures = %q, want one", item, paths)
		}
		b, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		var request struct {
			Type        string `json:"type"`
			TokenWaitMS *int64 `json:"token-wait-ms"`
		}
		if err := json.Unmarshal([]byte(strings.SplitN(string(b), "\n", 2)[0]), &request); err != nil || request.Type != "request" || request.TokenWaitMS == nil {
			t.Fatalf("%s's capture opens %s, want a request carrying token-wait-ms (%v)", item, b, err)
		}
		return *request.TokenWaitMS
	}
	first, second := waited("S1"), waited("S2")
	if floor := (releasedAt.Sub(before) - 10*time.Millisecond).Milliseconds(); second < floor {
		t.Errorf("S2 waited out S1's call and records %dms, want at least %dms", second, floor)
	}
	if first >= second {
		t.Errorf("S1 took a free token and records a %dms wait, S2 records %dms", first, second)
	}
}

// overview asks the overview of "excerpts" through client, its answers and
// captures in scratch's one trail, and returns the passage.
func overview(t *testing.T, client model.Client, scratch string) (string, error) {
	t.Helper()
	var passage string
	var got error
	var wg sync.WaitGroup
	launch := New(client, func() string { return trailDir(scratch) }, log.Discard()).PassageLaunch(model.NewQueue(1), "m")
	err := launch(context.Background(), &wg, func(ask func(context.Context, string, []string, int) (string, error)) func(error) {
		passage, got = ask(context.Background(), "excerpts", nil, 1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	return passage, got
}

// sampled is a scripted client answering "the passage" that sends sampling s.
func sampled(s model.Sampling) (*modeltest.MockClient, model.Client) {
	mock := modeltest.NewScriptedMock([]model.Response{{Content: "the passage", FinishReason: "stop"}}, nil)
	mock.RecordCalls = true
	client := mock.Client().(*model.ClientFuncs)
	client.Sampling = s
	return mock, client
}

// TestACaptureRecordsTheSamplingSent: a chat capture's request line carries the
// effective temperature and thinking level — kbase's defaults for a client
// stating none, a Messages client's included.
func TestACaptureRecordsTheSamplingSent(t *testing.T) {
	defaultTemperature, err := json.Marshal(model.DefaultTemperature)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name            string
		client          func(t *testing.T) model.Client
		wantTemperature string
		wantThinking    string
	}{
		{"stated", func(*testing.T) model.Client {
			_, c := sampled(model.Sampling{Temperature: new(0.7), Thinking: new(model.ThinkingLow)})
			return c
		}, "0.7", `"low"`},
		{"unset", func(*testing.T) model.Client {
			_, c := sampled(model.Sampling{})
			return c
		}, string(defaultTemperature), `"off"`},
		{"messages with thinking high", func(t *testing.T) model.Client {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, modeltest.MessagesStream("", "the passage", "end_turn"))
			}))
			t.Cleanup(srv.Close)
			c := model.NewAnthropicClient(model.Endpoint{BaseURL: srv.URL, Protocol: model.ProtocolAnthropic,
				Sampling: model.Sampling{Thinking: new(model.ThinkingHigh)}})
			t.Cleanup(func() { _ = c.Close() })
			return c
		}, string(defaultTemperature), `"high"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scratch := t.TempDir()
			if _, err := overview(t, tc.client(t), scratch); err != nil {
				t.Fatal(err)
			}
			paths, _ := filepath.Glob(filepath.Join(trailCaptures(scratch), "*.capture.jsonl"))
			if len(paths) != 1 {
				t.Fatalf("captures = %q, want one", paths)
			}
			b, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			var request map[string]json.RawMessage
			if err := json.Unmarshal([]byte(strings.SplitN(string(b), "\n", 2)[0]), &request); err != nil {
				t.Fatal(err)
			}
			if got := string(request["temperature"]); got != tc.wantTemperature {
				t.Errorf("temperature = %q, want %s", got, tc.wantTemperature)
			}
			if got := string(request["thinking"]); got != tc.wantThinking {
				t.Errorf("thinking = %q, want %s", got, tc.wantThinking)
			}
		})
	}
}

// TestAnAnswerCachedBeforeSamplingStillAnswersAnUnsetAsk: an answer at the
// default-sampling key — the digest of model, system, prompt and attempt —
// answers, with no request, an ask whose sampling is unset or the stated
// default; a non-default level misses it and asks.
func TestDecisionDigestIsStable(t *testing.T) {
	q := asks.MultichoiceQuestion{
		State:   "st",
		Attempt: 1,
		Question: asks.Question{
			Key: "k", Type: "ty", Instructions: "ins", TrueLetter: "T",
			Criteria: map[string]string{"b": "two", "a": "one"},
		},
	}
	want := asks.Digest("m", "k", "ty", "ins", "T", "a", "one", "b", "two", "st", "1")
	if got := decisionDigest("m", q); got != want {
		t.Fatalf("decisionDigest = %s, want %s", got, want)
	}
}

func TestAnAnswerCachedBeforeSamplingStillAnswersAnUnsetAsk(t *testing.T) {
	scratch := t.TempDir()
	system, err := asks.System(asks.OverviewSystem)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := asks.OverviewPrompt("excerpts", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(trailAnswers(scratch), 0o777); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(trailAnswers(scratch), asks.Digest("m", system, prompt, "1")+".txt")
	if err := os.WriteFile(cached, []byte("the cached passage"), 0o666); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		sampling  model.Sampling
		want      string
		wantCalls int
	}{
		{"unset", model.Sampling{}, "the cached passage", 0},
		{"the default stated", model.Sampling{Temperature: new(0.0), Thinking: new(model.ThinkingOff)}, "the cached passage", 0},
		{"thinking low", model.Sampling{Thinking: new(model.ThinkingLow)}, "the passage", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock, client := sampled(tc.sampling)
			got, err := overview(t, client, scratch)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want || len(mock.Calls()) != tc.wantCalls {
				t.Errorf("passage %q after %d request(s), want %q after %d", got, len(mock.Calls()), tc.want, tc.wantCalls)
			}
		})
	}
}
