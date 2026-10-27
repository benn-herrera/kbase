package model_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// sseServer answers each request with the next body of bodies, the last one
// repeating, and keeps every request body it read.
func sseServer(t *testing.T, bodies ...string) (*httptest.Server, func() [][]byte) {
	t.Helper()
	var mu sync.Mutex
	var seen [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, b)
		body := bodies[min(len(seen), len(bodies))-1]
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		return append([][]byte{}, seen...)
	}
}

// statusServer answers every request with code and a provider-shaped error
// body, counting the requests it read.
func statusServer(t *testing.T, code int) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	var seen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen++
		mu.Unlock()
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error":{"message":"unknown model: m"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() int {
		mu.Lock()
		defer mu.Unlock()
		return seen
	}
}

// slotChat makes one chat call to model "m" as a consumer makes it: through the
// Slot of the one item on a queue of one, ctx being the call's own.
func slotChat(t *testing.T, ctx context.Context, c model.Client, system, prompt string, backoff []time.Duration) (model.ChatResult, error) {
	t.Helper()
	var res model.ChatResult
	var err error
	var wg sync.WaitGroup
	if goErr := model.NewQueue(1).Go(context.Background(), &wg, func(s *model.Slot) func() {
		res, err = s.Chat(ctx, c, model.ChatRequest{Model: "m", System: system, Prompt: prompt, Backoff: backoff})
		return nil
	}); goErr != nil {
		t.Fatalf("Go: %v", goErr)
	}
	wg.Wait()
	return res, err
}

// lastAttempt is the call's last attempt, its outcome; zero where it made none.
func lastAttempt(r model.ChatResult) model.ChatAttempt {
	if len(r.Attempts) == 0 {
		return model.ChatAttempt{}
	}
	return r.Attempts[len(r.Attempts)-1]
}

// attemptOutcomes are how the call's attempts ended, in order.
func attemptOutcomes(r model.ChatResult) []string {
	var out []string
	for _, a := range r.Attempts {
		out = append(out, a.Outcome)
	}
	return out
}

// openAIAt is an OpenAI-surface client of the server at url.
func openAIAt(url string) model.Client { return model.NewOpenAIClient(model.Endpoint{BaseURL: url}) }

const (
	sseAnswered = "data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":1,\"total_tokens\":41}}\n\n" +
		"data: [DONE]\n\n"
	sseShortOfDone   = "data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n"
	sseServedNothing = "data: {\"choices\":[{\"delta\":{\"content\":\"out of memory\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0}}\n\n" +
		"data: [DONE]\n\n"
)

func TestChatReissuesWhatDidNotComplete(t *testing.T) {
	for _, tc := range []struct {
		name        string
		bodies      []string
		requests    int
		ok          bool
		wantOutcome string
	}{
		{"answered at once", []string{sseAnswered}, 1, true, model.ChatOK},
		{"short of [DONE], then answered", []string{sseShortOfDone, sseAnswered}, 2, true, model.ChatOK},
		{"served nothing, then answered", []string{sseServedNothing, sseAnswered}, 2, true, model.ChatOK},
		{"never complete", []string{sseShortOfDone}, model.ChatAttempts, false, model.ChatTransportFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := sseServer(t, tc.bodies...)
			res, err := slotChat(t, context.Background(), openAIAt(srv.URL), "the system", "the prompt", nil)
			got, observed := lastAttempt(res), attemptOutcomes(res)
			if tc.ok != (err == nil) || (!tc.ok && !errors.Is(err, model.ErrChatIncomplete)) {
				t.Fatalf("Chat = %+v, %v; want ok %t", got, err, tc.ok)
			}
			if tc.ok && (got.Reply != "A" || got.Outcome != model.ChatOK || got.Usage.PromptTokens != 40) {
				t.Errorf("Chat = %+v", got)
			}
			if got.Outcome != tc.wantOutcome {
				t.Errorf("outcome = %q, want %q", got.Outcome, tc.wantOutcome)
			}
			requests := seen()
			if len(requests) != tc.requests || len(observed) != tc.requests {
				t.Fatalf("requests %d, observed %q; want %d", len(requests), observed, tc.requests)
			}
			for _, b := range requests[1:] {
				if string(b) != string(requests[0]) {
					t.Errorf("a re-issue differed from the first request:\n%s\n%s", b, requests[0])
				}
			}
		})
	}
}

// TestChatNamesAProviderRefusal: a provider that answers with a status that is
// not success read the request and declined it, which is not the transport
// failing — so the attempt says it refused, and the reason carries the status
// and the body's start rather than sending a reader to the path between kbase
// and a provider that is answering. A 400 will be as wrong on the third
// attempt as on the first, so the call is put on the wire once.
func TestChatNamesAProviderRefusal(t *testing.T) {
	srv, requests := statusServer(t, http.StatusBadRequest)

	res, err := slotChat(t, context.Background(), openAIAt(srv.URL), "the system", "the prompt", nil)
	got, observed := lastAttempt(res), attemptOutcomes(res)
	if !errors.Is(err, model.ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if got.Outcome != model.ChatProviderRefused {
		t.Errorf("outcome = %q, want %q", got.Outcome, model.ChatProviderRefused)
	}
	for _, reason := range []string{got.Error, err.Error()} {
		if !strings.Contains(reason, "http 400") || !strings.Contains(reason, "unknown model") {
			t.Errorf("reason = %q, want the status and the provider's own words", reason)
		}
		if strings.Contains(reason, model.ChatTransportFailure) {
			t.Errorf("reason = %q, want no transport failure where the provider answered", reason)
		}
	}
	if !strings.Contains(err.Error(), "1 attempt") {
		t.Errorf("reason = %q, want the one attempt it took", err)
	}
	if n := requests(); n != 1 || len(observed) != 1 {
		t.Fatalf("%d requests, %d attempts observed; want one: a refusal is not re-issued", n, len(observed))
	}
	for _, o := range observed {
		if o != model.ChatProviderRefused {
			t.Errorf("an attempt = %q, want %q", o, model.ChatProviderRefused)
		}
	}
}

// TestChatReissuesOnlyARetryableStatus: a status that may be the provider's
// moment — 429 and the 5xx class — is re-issued to the attempt count; every
// other answered status ends the call on the one attempt that met it.
func TestChatReissuesOnlyARetryableStatus(t *testing.T) {
	for _, tc := range []struct {
		code     int
		requests int
	}{
		{http.StatusBadRequest, 1},
		{http.StatusUnauthorized, 1},
		{http.StatusNotFound, 1},
		{http.StatusTooManyRequests, model.ChatAttempts},
		{http.StatusServiceUnavailable, model.ChatAttempts},
	} {
		t.Run(fmt.Sprintf("%d", tc.code), func(t *testing.T) {
			srv, requests := statusServer(t, tc.code)

			res, err := slotChat(t, context.Background(), openAIAt(srv.URL), "the system", "the prompt", nil)
			got, observed := lastAttempt(res), res.Attempts
			if !errors.Is(err, model.ErrChatIncomplete) {
				t.Fatalf("Chat = %v, want an incomplete call", err)
			}
			if got.Outcome != model.ChatProviderRefused {
				t.Errorf("outcome = %q, want %q", got.Outcome, model.ChatProviderRefused)
			}
			for _, reason := range []string{got.Error, err.Error()} {
				if want := fmt.Sprintf("http %d", tc.code); !strings.Contains(reason, want) {
					t.Errorf("reason = %q, want it to name %q", reason, want)
				}
			}
			if n := requests(); n != tc.requests || len(observed) != tc.requests {
				t.Fatalf("%d requests, %d attempts observed; want %d", n, len(observed), tc.requests)
			}
			if tc.requests == 1 {
				if got.Attempt != 1 || !strings.Contains(err.Error(), "1 attempt") {
					t.Errorf("attempt %d, reason %q; want the one attempt it took", got.Attempt, err)
				}
				return
			}
			if got.Attempt != model.ChatAttempts {
				t.Errorf("attempt = %d, want the last of %d", got.Attempt, model.ChatAttempts)
			}
		})
	}
}

// TestChatIncompleteReasonWithoutAttemptText: an attempt can end without an
// error text of its own — a server that served nothing says so through its
// usage alone — and the reason then ends on the outcome, with no separator
// left dangling after it.
func TestChatIncompleteReasonWithoutAttemptText(t *testing.T) {
	srv, _ := sseServer(t, sseServedNothing)

	res, err := slotChat(t, context.Background(), openAIAt(srv.URL), "the system", "the prompt", nil)
	got := lastAttempt(res)
	if !errors.Is(err, model.ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if got.Outcome != model.ChatServedNothing || got.Error != "" {
		t.Fatalf("last attempt = %+v, want an outcome of %q and no error text", got, model.ChatServedNothing)
	}
	if want := "the last ending " + got.Outcome; !strings.HasSuffix(err.Error(), want) {
		t.Errorf("reason = %q, want it to end %q", err, want)
	}
}

// TestChatNamesAnUnreachableEndpoint: nothing answering at all is the
// transport's failure — the case the refusal above must not be read as, and
// what a closed port looks like. No status refused it, so it is re-issued to
// the attempt count like an incomplete stream.
func TestChatNamesAnUnreachableEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	res, err := slotChat(t, context.Background(), openAIAt(url), "the system", "the prompt", nil)
	got, observed := lastAttempt(res), attemptOutcomes(res)
	if !errors.Is(err, model.ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if got.Outcome != model.ChatTransportFailure {
		t.Errorf("outcome = %q, want %q", got.Outcome, model.ChatTransportFailure)
	}
	if len(observed) != model.ChatAttempts {
		t.Errorf("%d attempts observed, want %d: a transport failure is re-issued", len(observed), model.ChatAttempts)
	}
}

func TestChatRequestShape(t *testing.T) {
	srv, seen := sseServer(t, sseAnswered)
	if _, err := slotChat(t, context.Background(), openAIAt(srv.URL), "the system", "the prompt", nil); err != nil {
		t.Fatal(err)
	}
	var body struct {
		MaxTokens   *int             `json:"max_tokens"`
		Model       string           `json:"model"`
		Messages    []map[string]any `json:"messages"`
		Temperature *float64         `json:"temperature"`
		Kwargs      map[string]any   `json:"chat_template_kwargs"`
		Reasoning   map[string]any   `json:"reasoning"`
		Stream      bool             `json:"stream"`
		Options     map[string]any   `json:"stream_options"`
		Tools       any              `json:"tools"`
	}
	if err := json.Unmarshal(seen()[0], &body); err != nil {
		t.Fatal(err)
	}
	if body.Model != "m" || len(body.Messages) != 2 || body.Messages[0]["role"] != "system" || body.Messages[0]["content"] != "the system" ||
		body.Messages[1]["role"] != "user" || body.Messages[1]["content"] != "the prompt" {
		t.Errorf("messages = %+v", body.Messages)
	}
	if body.Temperature == nil || *body.Temperature != 0 || body.Kwargs["enable_thinking"] != false || len(body.Kwargs) != 1 {
		t.Errorf("temperature %v, chat_template_kwargs %v", body.Temperature, body.Kwargs)
	}
	if len(body.Reasoning) != 1 || body.Reasoning["enabled"] != false {
		t.Errorf("reasoning = %v, want the provider's control with enabled false", body.Reasoning)
	}
	if !body.Stream || body.Options["include_usage"] != true || body.Tools != nil {
		t.Errorf("stream %t, stream_options %v, tools %v", body.Stream, body.Options, body.Tools)
	}
	if body.MaxTokens != nil {
		t.Errorf("max_tokens %d sent; the request must carry none", *body.MaxTokens)
	}
}

// TestChatRecordsReasoned: a call's attempt says it reasoned from either
// source the wire offers — a reasoning delta on the stream, or a usage block
// reporting reasoning tokens spent. The second is what makes a call that
// thought silently visible: no delta carried the thought, and the token count
// is the only trace of it. A call with neither source did not reason.
func TestChatRecordsReasoned(t *testing.T) {
	const answered = "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"
	usage := func(reasoningTokens int) string {
		return fmt.Sprintf("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":900,\"total_tokens\":940,\"completion_tokens_details\":{\"reasoning_tokens\":%d}}}\n\n", reasoningTokens)
	}
	const (
		thought = "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"weighing it\"}}]}\n\n"
		done    = "data: [DONE]\n\n"
	)
	for _, tc := range []struct {
		name string
		sse  string
		want bool
	}{
		{"usage reports reasoning tokens, no delta", answered + usage(880) + done, true},
		{"neither a delta nor reasoning tokens", answered + usage(0) + done, false},
		{"a reasoning delta, usage reports none", thought + answered + usage(0) + done, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := sseServer(t, tc.sse)
			res, err := slotChat(t, context.Background(), openAIAt(srv.URL), "the system", "the prompt", nil)
			if err != nil {
				t.Fatalf("Chat: %v", err)
			}
			got := lastAttempt(res)
			if n := len(seen()); n != 1 {
				t.Fatalf("%d requests; want the one it took", n)
			}
			if got.Reasoned != tc.want {
				t.Errorf("Reasoned = %t, want %t (usage %+v)", got.Reasoned, tc.want, got.Usage)
			}
		})
	}
}

// TestChatCarriesAnEmptyMessageContent: a message's content field reaches
// the wire even when empty — `"content":""`, never an absent field. A
// provider reads `{"role":"system"}` as a message with no text part and
// declines the whole request, so this is the shape of kbase's empty system
// prompt being sent at all.
func TestChatCarriesAnEmptyMessageContent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		system string
		prompt string
		want   []string
	}{
		{"empty system", "", "the prompt", []string{
			`{"role":"system","content":""}`,
			`{"role":"user","content":"the prompt"}`,
		}},
		{"empty prompt", "the system", "", []string{
			`{"role":"system","content":"the system"}`,
			`{"role":"user","content":""}`,
		}},
		{"neither empty", "the system", "the prompt", []string{
			`{"role":"system","content":"the system"}`,
			`{"role":"user","content":"the prompt"}`,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := sseServer(t, sseAnswered)
			if _, err := slotChat(t, context.Background(), openAIAt(srv.URL), tc.system, tc.prompt, nil); err != nil {
				t.Fatal(err)
			}
			raw := string(seen()[0])
			for _, w := range tc.want {
				if !strings.Contains(raw, w) {
					t.Errorf("body = %s, want it to carry %s", raw, w)
				}
			}
		})
	}
}

func TestChatStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := slotChat(t, ctx, modeltest.NewScriptedMock([]model.Response{{Content: "A"}}, nil).Client(), "s", "p", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Chat on a cancelled context = %v", err)
	}
}

// TestChatTakesAnEmptyReply: a generation that emitted nothing — its whole
// budget spent on reasoning — is a completed call with an empty reply, not a
// failure.
func TestChatTakesAnEmptyReply(t *testing.T) {
	m := modeltest.NewScriptedMock([]model.Response{{Content: "", FinishReason: "stop"}}, nil)
	res, err := slotChat(t, context.Background(), m.Client(), "s", "p", nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got := lastAttempt(res); got.Outcome != model.ChatOK || got.Reply != "" || got.FinishReason != "stop" || len(res.Attempts) != 1 {
		t.Errorf("attempts = %+v, want one completed attempt with an empty reply", res.Attempts)
	}
}

// TestChatAccumulatesTheStream: over the real transport, the reply accumulates
// across content deltas with no reasoning in it, and the include_usage chunk
// lands in the attempt's usage.
func TestChatAccumulatesTheStream(t *testing.T) {
	srv, _ := sseServer(t, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"scratch\"}}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":2,\"total_tokens\":6}}\n\n"+
		"data: [DONE]\n\n")
	res, err := slotChat(t, context.Background(), openAIAt(srv.URL), "s", "p", nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	got := lastAttempt(res)
	if got.Reply != "Hello" {
		t.Errorf("Reply = %q, want %q (reasoning must not leak in)", got.Reply, "Hello")
	}
	if got.FinishReason != "stop" || got.Usage.TotalTokens != 6 {
		t.Errorf("finish reason %q, usage %+v; want stop and 6 tokens in all", got.FinishReason, got.Usage)
	}
}

// TestChatEndsOnACancelMidStream: a ctx cancelled while the call waits on a
// stalled provider's next delta ends the call with the ctx's error, and the
// attempt it cut short is not among the call's attempts.
func TestChatEndsOnACancelMidStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	defer cancel()
	res, err := slotChat(t, ctx, openAIAt(srv.URL), "s", "p", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Chat = %v, want context.Canceled", err)
	}
	if len(res.Attempts) != 0 {
		t.Errorf("attempts = %+v, want none: the cancel cut the only one short", res.Attempts)
	}
}

// TestChatBacksOffBetweenReissues: each re-issue waits the next pause of the
// backoff, its last repeating, and a cancel during a pause ends the call.
func TestChatBacksOffBetweenReissues(t *testing.T) {
	srv, seen := sseServer(t, sseShortOfDone)
	started := time.Now()
	if _, err := slotChat(t, context.Background(), openAIAt(srv.URL), "s", "p",
		[]time.Duration{30 * time.Millisecond}); !errors.Is(err, model.ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want incomplete", err)
	}
	if took := time.Since(started); len(seen()) != model.ChatAttempts || took < time.Duration(model.ChatAttempts-1)*30*time.Millisecond {
		t.Errorf("%d requests over %v, want %d with a 30ms pause before each re-issue", len(seen()), took, model.ChatAttempts)
	}
	// The pause fits the budget (there is none), so a cancel arriving during
	// it is what ends the call.
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	defer cancel()
	if _, err := slotChat(t, ctx, openAIAt(srv.URL), "s", "p", []time.Duration{time.Hour}); !errors.Is(err, context.Canceled) {
		t.Errorf("Chat cancelled in its pause = %v", err)
	}
}

// TestChatGivesUpWhereThePauseOutlivesTheDeadline: a pause the deadline cannot
// cover is not taken. The attempt after it is impossible, and waiting it out
// would spend the whole budget dying on the deadline instead of on the attempt
// it made — the caller's real scoring backoff, 5 s then 30 s, inside a 2 s
// deadline.
func TestChatGivesUpWhereThePauseOutlivesTheDeadline(t *testing.T) {
	srv, requests := statusServer(t, http.StatusServiceUnavailable)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	res, err := slotChat(t, ctx, openAIAt(srv.URL), "the system", "the prompt",
		[]time.Duration{5 * time.Second, 30 * time.Second})
	got := lastAttempt(res)
	if !errors.Is(err, model.ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("reason = %q, want the attempts it made rather than the deadline", err)
	}
	if !strings.Contains(err.Error(), "1 attempt") || !strings.Contains(err.Error(), model.ChatProviderRefused) {
		t.Errorf("reason = %q, want the one attempt it made and how it ended", err)
	}
	if got.Attempt != 1 {
		t.Errorf("attempt = %d, want the one it made", got.Attempt)
	}
	if n := requests(); n != 1 {
		t.Errorf("%d requests, want the one attempt", n)
	}
	if took := time.Since(started); took >= 2*time.Second {
		t.Errorf("gave up after %v, want well inside the 2s deadline", took)
	}
}

// TestChatSpendsItsBudgetOnAttempts: a deadline the pauses fit is spent on
// attempts, not on giving up early. A 30 s deadline with a short injected
// backoff still puts the call on the wire model.ChatAttempts times.
func TestChatSpendsItsBudgetOnAttempts(t *testing.T) {
	srv, requests := statusServer(t, http.StatusServiceUnavailable)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := slotChat(t, ctx, openAIAt(srv.URL), "the system", "the prompt",
		[]time.Duration{time.Millisecond, time.Millisecond})
	got := lastAttempt(res)
	if !errors.Is(err, model.ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if got.Attempt != model.ChatAttempts {
		t.Errorf("attempt = %d, want the last of %d", got.Attempt, model.ChatAttempts)
	}
	if n := requests(); n != model.ChatAttempts {
		t.Errorf("%d requests, want %d", n, model.ChatAttempts)
	}
}

// TestChatTakesAPauseTheDeadlineCovers: a deadline longer than the whole
// sequence leaves it unaffected — every attempt is made and every pause is
// waited out rather than cut short.
func TestChatTakesAPauseTheDeadlineCovers(t *testing.T) {
	const pause = 30 * time.Millisecond
	srv, requests := statusServer(t, http.StatusServiceUnavailable)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	started := time.Now()
	if _, err := slotChat(t, ctx, openAIAt(srv.URL), "the system", "the prompt",
		[]time.Duration{pause}); !errors.Is(err, model.ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	took := time.Since(started)
	if n := requests(); n != model.ChatAttempts {
		t.Errorf("%d requests, want %d", n, model.ChatAttempts)
	}
	if want := time.Duration(model.ChatAttempts-1) * pause; took < want || took >= time.Second {
		t.Errorf("took %v, want the %d pauses it waited out inside the deadline", took, model.ChatAttempts-1)
	}
}

// TestChatRecordsWhenTheFirstDeltaArrived: first-token time is the moment the
// first non-empty content or reasoning delta arrived, and zero where the stream
// carried neither — a reasoning delta counts, an empty one does not.
func TestChatRecordsWhenTheFirstDeltaArrived(t *testing.T) {
	const (
		thought = "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"weighing it\"}}]}\n\n"
		empty   = "data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n"
		done    = "data: [DONE]\n\n"
		usage   = "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":1,\"total_tokens\":41}}\n\n"
	)
	for _, tc := range []struct {
		name     string
		sse      string
		answered bool
	}{
		{"content", sseAnswered, true},
		{"reasoning alone", thought + sseAnswered, true},
		{"empty deltas only", empty + usage + done, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := sseServer(t, tc.sse)
			res, err := slotChat(t, context.Background(), openAIAt(srv.URL), "the system", "the prompt", nil)
			if err != nil {
				t.Fatalf("Chat: %v", err)
			}
			got := lastAttempt(res)
			if !tc.answered {
				if got.FirstToken != 0 {
					t.Errorf("FirstToken = %v, want zero: the stream carried no delta", got.FirstToken)
				}
				return
			}
			if got.FirstToken <= 0 || got.FirstToken > got.Duration {
				t.Errorf("FirstToken = %v over a call of %v, want a moment inside it", got.FirstToken, got.Duration)
			}
		})
	}
}
