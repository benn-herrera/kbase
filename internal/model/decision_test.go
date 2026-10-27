package model_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// decisionRequest is the one-question request the tests send.
func decisionRequest() model.DecisionRequest {
	return model.DecisionRequest{
		Model: "clef",
		State: "the composed state",
		Questions: []model.DecisionQuestion{{
			Key:          "q1",
			Type:         "choice",
			Instructions: "pick one",
			Criteria:     map[string]string{"A": "first", "B": "second"},
		}},
	}
}

// slotDecision makes one decision call as a consumer makes it: through the Slot
// of the one item on a queue of one, ctx being the call's own.
func slotDecision(t *testing.T, ctx context.Context, c model.Client, req model.DecisionRequest) (model.DecisionResponse, error) {
	t.Helper()
	var resp model.DecisionResponse
	var err error
	var wg sync.WaitGroup
	if goErr := model.NewQueue(1).Go(context.Background(), &wg, func(s *model.Slot) func() {
		resp, err = s.Decision(ctx, c, req)
		return nil
	}); goErr != nil {
		t.Fatalf("Go: %v", goErr)
	}
	wg.Wait()
	return resp, err
}

// decisionServer serves the given responses in order: an element of the
// form "status:<code>" is sent as that status with a short body; the
// sentinel nilBody kills the connection unread, a transport error; anything
// else is sent as a 200 body. The handler records method, path, auth header
// and body of every request it saw. The handler runs on the server's
// goroutine, so every field it touches is guarded by mu.
type decisionServer struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests int
	method   string
	path     string
	auth     string
	bodies   []string
}

const decisionNilBody = "\x00nil"

func decisionStatus(code int) string { return fmt.Sprintf("status:%d", code) }

func newDecisionServer(t *testing.T, responses ...string) *decisionServer {
	t.Helper()
	s := &decisionServer{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.requests++
		s.method, s.path, s.auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		s.bodies = append(s.bodies, string(body))
		resp := responses[min(s.requests-1, len(responses)-1)]
		switch {
		case resp == decisionNilBody:
			// Kill the connection without a response: the transport
			// error shape.
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
		case strings.HasPrefix(resp, "status:"):
			code := 0
			_, _ = fmt.Sscanf(resp, "status:%d", &code)
			w.WriteHeader(code)
			_, _ = w.Write([]byte("server error: token=" + testAPIKey))
		default:
			_, _ = w.Write([]byte(resp))
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

// saw reports how many requests the server has handled.
func (s *decisionServer) saw() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

// snapshot returns the recorded method, auth header and request bodies.
func (s *decisionServer) snapshot() (method, auth string, bodies []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.method, s.auth, append([]string(nil), s.bodies...)
}

// TestDecisionSurfacePath: the decision call hangs off BaseURL at the path the
// endpoint's resolved surface names — systemone where it names none — with no
// doubled slash on a trailing-slash base.
func TestDecisionSurfacePath(t *testing.T) {
	cases := []struct {
		name      string
		decisions bool
		base      func(string) string
		want      string
		comment   string
	}{
		{"bare host", false, func(u string) string { return u }, "/systemone", ""},
		{"v1 base", false, func(u string) string { return u + "/v1" }, "/v1/systemone", ""},
		{"trailing slash", false, func(u string) string { return u + "/" }, "/systemone", "no doubled slash"},
		{"decisions, bare host", true, func(u string) string { return u }, "/decisions", ""},
		{"decisions, v1 base", true, func(u string) string { return u + "/v1" }, "/v1/decisions", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte(`{"model":"clef"}`))
			}))
			defer srv.Close()

			c := model.NewOpenAIClient(model.Endpoint{Name: "t", BaseURL: tc.base(srv.URL), APIKey: testAPIKey, Decisions: tc.decisions})
			if _, err := slotDecision(t, context.Background(), c, decisionRequest()); err != nil {
				t.Fatalf("Decision: %v", err)
			}
			if gotPath != tc.want {
				t.Errorf("path: got %q, want %q (%s)", gotPath, tc.want, tc.comment)
			}
		})
	}
}

func TestSystemOnePayloadShape(t *testing.T) {
	s := newDecisionServer(t, `{"model":"clef","answers":{},"usage":{}}`)
	c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
	if _, err := slotDecision(t, context.Background(), c, decisionRequest()); err != nil {
		t.Fatalf("Decision: %v", err)
	}
	gotMethod, gotAuth, bodies := s.snapshot()
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer "+testAPIKey {
		t.Errorf("auth: got %q, want the bearer key", gotAuth)
	}
	if len(bodies) != 1 {
		t.Fatalf("requests: got %d, want 1", len(bodies))
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(bodies[0]), &body); err != nil {
		t.Fatalf("unmarshal body %s: %v", bodies[0], err)
	}
	if body["model"] != "clef" {
		t.Errorf("model: got %v, want clef", body["model"])
	}
	if body["state"] != "the composed state" {
		t.Errorf("state: got %v, want the composed state", body["state"])
	}
	qs, ok := body["questions"].(map[string]any)
	if !ok {
		t.Fatalf("questions is %T, want a keyed object: %s", body["questions"], bodies[0])
	}
	q, ok := qs["q1"].(map[string]any)
	if !ok {
		t.Fatalf("questions[q1] missing or not an object: %s", bodies[0])
	}
	if q["type"] != "choice" || q["instructions"] != "pick one" {
		t.Errorf("questions[q1] = %v, want type choice and the instructions", q)
	}
	criteria, ok := q["criteria"].(map[string]any)
	if !ok || criteria["A"] != "first" || criteria["B"] != "second" {
		t.Errorf("criteria = %v, want the per-option guidance", q["criteria"])
	}
	for _, forbidden := range []string{"temperature", "stream", "stream_options", "chat_template_kwargs", "messages", "input"} {
		if _, ok := body[forbidden]; ok {
			t.Errorf("field %q not on the SystemOne body was sent: %s", forbidden, bodies[0])
		}
	}
}

func TestSystemOneAnswerDecoding(t *testing.T) {
	const body = `{
		"model": "clef",
		"answers": {
			"choice-full": {"type": "choice", "choice": "A", "confidence": 0.9,
				"probabilities": {"A": 0.9, "B": 0.1}, "extra_field": true},
			"choice-bare": {"type": "choice", "choice": "B"},
			"noul-q": {"type": "noul", "noul": 0.96}
		},
		"usage": {"input_tokens": 100, "output_tokens": 5}
	}`
	s := newDecisionServer(t, body)
	c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
	req := model.DecisionRequest{
		Model: "clef",
		State: "the composed state",
		Questions: []model.DecisionQuestion{
			{Key: "choice-full", Type: "choice"},
			{Key: "choice-bare", Type: "choice"},
			{Key: "noul-q", Type: "noul"},
		},
	}
	resp, err := slotDecision(t, context.Background(), c, req)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}

	if resp.Model != "clef" {
		t.Errorf("model: got %q, want clef", resp.Model)
	}
	if resp.Usage.PromptTokens != 100 || resp.Usage.CompletionTokens != 5 {
		t.Errorf("usage: got %+v, want 100/5", resp.Usage)
	}

	// Answers decode in sorted key order, each keyed by the key it arrived
	// under; an unknown FIELD on an answer is tolerated (read whole, act
	// only on what the mapping needs) — an unknown KEY is refused elsewhere.
	byKey := map[string]model.DecisionAnswer{}
	for _, a := range resp.Answers {
		byKey[a.Key] = a
	}
	if len(resp.Answers) != 3 {
		t.Fatalf("answers: got %d (%v), want 3 — the whole object read", len(resp.Answers), resp.Answers)
	}
	full := byKey["choice-full"]
	if full.Type != "choice" || full.Choice != "A" || full.Confidence != 0.9 ||
		full.Probabilities["A"] != 0.9 || full.Probabilities["B"] != 0.1 {
		t.Errorf("choice-full = %+v", full)
	}
	bare := byKey["choice-bare"]
	if bare.Type != "choice" || bare.Choice != "B" || bare.Confidence != 0 || bare.Probabilities != nil {
		t.Errorf("choice-bare = %+v, want the optional fields at their zero values", bare)
	}
	noul := byKey["noul-q"]
	if noul.Type != "noul" || noul.Noul != 0.96 || noul.Probabilities != nil {
		t.Errorf("noul-q = %+v, want noul 0.96 and no probabilities", noul)
	}
}

func TestSystemOneRetryTransportErrorThenSuccess(t *testing.T) {
	s := newDecisionServer(t, decisionNilBody, `{"model":"clef","answers":{"q1":{"type":"choice","choice":"A"}},"usage":{"input_tokens":1,"output_tokens":1}}`)
	c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
	resp, err := slotDecision(t, context.Background(), c, decisionRequest())
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if s.saw() != 2 {
		t.Errorf("requests: got %d, want 2 (one transport failure, one re-issue)", s.saw())
	}
	if len(resp.Answers) != 1 || resp.Answers[0].Key != "q1" || resp.Answers[0].Choice != "A" {
		t.Errorf("answers = %+v, want the re-issued call's answer", resp.Answers)
	}
}

func TestSystemOneRetry500ThenSuccess(t *testing.T) {
	s := newDecisionServer(t, decisionStatus(500), `{"model":"clef","answers":{},"usage":{"input_tokens":7,"output_tokens":2}}`)
	c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
	resp, err := slotDecision(t, context.Background(), c, decisionRequest())
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if s.saw() != 2 {
		t.Errorf("requests: got %d, want 2 (one 500, one re-issue)", s.saw())
	}
	if resp.Usage.PromptTokens != 7 || resp.Usage.CompletionTokens != 2 {
		t.Errorf("usage: got %+v, want the successful call's figures", resp.Usage)
	}
}

func TestSystemOne4xxNoRetry(t *testing.T) {
	s := newDecisionServer(t, decisionStatus(400))
	c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
	_, err := slotDecision(t, context.Background(), c, decisionRequest())
	if err == nil {
		t.Fatal("expected error from 400, got nil")
	}
	var se model.StatusError
	if !errors.As(err, &se) || se.Code != http.StatusBadRequest {
		t.Errorf("err = %v, want a model.StatusError with code 400", err)
	}
	if s.saw() != 1 {
		t.Errorf("requests: got %d, want 1 — a 4xx is not re-issued", s.saw())
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("API key leaked into the error: %v", err)
	}
}

func TestSystemOneAttemptsExhausted(t *testing.T) {
	s := newDecisionServer(t, decisionNilBody, decisionNilBody, decisionNilBody)
	c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
	_, err := slotDecision(t, context.Background(), c, decisionRequest())
	if err == nil {
		t.Fatal("expected error after exhausted attempts, got nil")
	}
	if s.saw() != model.ChatAttempts {
		t.Errorf("requests: got %d, want %d", s.saw(), model.ChatAttempts)
	}
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("error does not name the attempt count: %v", err)
	}
}

func TestSystemOneContextCancellation(t *testing.T) {
	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	}))
	defer func() { close(stop); srv.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-time.After(50 * time.Millisecond); cancel() }()

	c := model.NewOpenAIClient(newTestEndpoint(srv.URL))
	_, err := slotDecision(t, ctx, c, decisionRequest())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestMockDecisionScripted(t *testing.T) {
	m := modeltest.NewScriptedMockWithDecisions([]model.DecisionAnswer{
		{Key: "q1", Type: "choice", Choice: "A", Probabilities: map[string]float64{"A": 0.9, "B": 0.1}},
		{Key: "q2", Type: "noul", Noul: 0.96},
	})
	ctx := context.Background()
	first, err := m.Decision(ctx, model.DecisionRequest{Model: "clef"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(first.Answers) != 1 || first.Answers[0].Key != "q1" || first.Answers[0].Choice != "A" ||
		first.Answers[0].Probabilities["B"] != 0.1 {
		t.Errorf("first = %+v, want the first scripted answer with probabilities", first)
	}
	second, err := m.Decision(ctx, model.DecisionRequest{})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(second.Answers) != 1 || second.Answers[0].Noul != 0.96 {
		t.Errorf("second = %+v, want the noul answer", second)
	}
	if _, err := m.Decision(ctx, model.DecisionRequest{}); !errors.Is(err, modeltest.ErrMockExhausted) {
		t.Errorf("past the script: got %v, want ErrMockExhausted", err)
	}
}

func TestMockDecisionUnscriptedExhausted(t *testing.T) {
	m := modeltest.NewScriptedMock(nil, nil)
	if _, err := m.Decision(context.Background(), model.DecisionRequest{}); !errors.Is(err, modeltest.ErrMockExhausted) {
		t.Errorf("unscripted: got %v, want ErrMockExhausted", err)
	}
}

// TestSystemOneUnaskedKeyRefused: an answer under a question key the
// request did not ask is server misbehavior — refused with a descriptive
// error, not ignored, and not retried (one HTTP request, however the
// unasked key arrives).
func TestSystemOneUnaskedKeyRefused(t *testing.T) {
	cases := []struct {
		name    string
		answers string
	}{
		{"extra key beside the asked one", `{"q1":{"type":"choice","choice":"A"},"sneaky":{"type":"noul","noul":0.5}}`},
		{"unasked key only", `{"sneaky":{"type":"choice","choice":"A"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newDecisionServer(t, `{"model":"clef","answers":`+tc.answers+`,"usage":{"input_tokens":1,"output_tokens":1}}`)
			c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
			_, err := slotDecision(t, context.Background(), c, decisionRequest())
			if err == nil {
				t.Fatal("expected error for the unasked key, got nil")
			}
			if !strings.Contains(err.Error(), `unasked question key "sneaky"`) {
				t.Errorf("error does not name the offending key: %v", err)
			}
			if !strings.Contains(err.Error(), "asked: q1") {
				t.Errorf("error does not name the asked keys: %v", err)
			}
			if s.saw() != 1 {
				t.Errorf("requests: got %d, want 1 — the refusal is not retried", s.saw())
			}
		})
	}
}

// TestSystemOneMissingAnswerTolerated: a question with no answer in the
// response is the seam's recoverable path, not a transport failure — the
// transport returns what arrived, whatever keys went unanswered.
func TestSystemOneMissingAnswerTolerated(t *testing.T) {
	s := newDecisionServer(t, `{"model":"clef","answers":{"q2":{"type":"noul","noul":0.9}},"usage":{}}`)
	c := model.NewOpenAIClient(newTestEndpoint(s.srv.URL))
	req := model.DecisionRequest{
		Model: "clef",
		State: "state",
		Questions: []model.DecisionQuestion{
			{Key: "q1", Type: "choice", Instructions: "pick"},
			{Key: "q2", Type: "noul", Instructions: "judge"},
		},
	}
	resp, err := slotDecision(t, context.Background(), c, req)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if len(resp.Answers) != 1 || resp.Answers[0].Key != "q2" || resp.Answers[0].Noul != 0.9 {
		t.Errorf("answers = %+v, want q2's answer alone, q1's absence left to the seam", resp.Answers)
	}
}

// decisionsWireRequest is the Decisions request body as the wire carries it:
// questions and choices arrays, kept in the order the bytes hold them.
type decisionsWireRequest struct {
	Model     string `json:"model"`
	Input     string `json:"input"`
	Questions []struct {
		Type         string `json:"type"`
		Name         string `json:"name"`
		Instructions string `json:"instructions"`
		Choices      []struct {
			Value       string `json:"value"`
			Description string `json:"description"`
		} `json:"choices"`
	} `json:"questions"`
}

// TestDecisionsRequestShape: the Decisions surface's body is model, input and
// an array of named questions — a noul question asked as a predicate, a choice
// question as a choice, its criteria the choices sorted by value so one
// question serializes the same way every time. A predicate carries no choices
// at all (the schema gives it none), so its criterion guidance is written into
// its instructions instead. None of the SystemOne body's fields appear.
func TestDecisionsRequestShape(t *testing.T) {
	for _, tc := range []struct {
		name             string
		question         model.DecisionQuestion
		wantType         string
		wantChoices      []string
		wantInstructions []string
	}{
		{"predicate", model.DecisionQuestion{Key: "S1", Type: "noul", Instructions: "judge the step",
			Criteria: map[string]string{"true": "the step follows from the rules", "false": "it does not"}},
			"predicate", nil,
			[]string{"judge the step", "The condition is true when: the step follows from the rules",
				"It is false when: it does not"}},
		{"choice", model.DecisionQuestion{Key: "S2", Type: "choice", Instructions: "pick the letter",
			Criteria: map[string]string{"D": "fourth", "C": "third"}},
			"choice", []string{"C", "D"}, []string{"pick the letter"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newDecisionServer(t, `{"model":"clef","answers":[]}`)
			c := model.NewOpenAIClient(model.Endpoint{Name: "t", BaseURL: s.srv.URL, APIKey: testAPIKey, Decisions: true})
			req := model.DecisionRequest{Model: "clef", State: "the composed state", Questions: []model.DecisionQuestion{tc.question}}
			if _, err := slotDecision(t, context.Background(), c, req); err != nil {
				t.Fatalf("Decision: %v", err)
			}
			_, _, bodies := s.snapshot()
			if len(bodies) != 1 {
				t.Fatalf("requests: got %d, want 1", len(bodies))
			}
			var body decisionsWireRequest
			if err := json.Unmarshal([]byte(bodies[0]), &body); err != nil {
				t.Fatalf("unmarshal body %s: %v", bodies[0], err)
			}
			if body.Model != "clef" || body.Input != "the composed state" {
				t.Errorf("model/input = %q/%q, want the model and the composed state", body.Model, body.Input)
			}
			if len(body.Questions) != 1 {
				t.Fatalf("questions = %v, want the one asked", body.Questions)
			}
			q := body.Questions[0]
			if q.Type != tc.wantType || q.Name != tc.question.Key {
				t.Errorf("question = %+v, want type %s and its key as the name", q, tc.wantType)
			}
			for _, want := range tc.wantInstructions {
				if !strings.Contains(q.Instructions, want) {
					t.Errorf("instructions = %q, want them to carry %q", q.Instructions, want)
				}
			}
			var values []string
			for _, choice := range q.Choices {
				values = append(values, choice.Value)
				if want := tc.question.Criteria[choice.Value]; choice.Description != want {
					t.Errorf("choice %s description = %q, want %q", choice.Value, choice.Description, want)
				}
			}
			if !slices.Equal(values, tc.wantChoices) {
				t.Errorf("choices = %v, want %v", values, tc.wantChoices)
			}
			// A predicate has no choices field on the schema, and the endpoint
			// rejects one it does not know.
			if tc.wantType == "predicate" && strings.Contains(bodies[0], `"choices"`) {
				t.Errorf("a predicate carried choices: %s", bodies[0])
			}
			for _, forbidden := range []string{`"state"`, `"answers"`, `"temperature"`, `"messages"`} {
				if strings.Contains(bodies[0], forbidden) {
					t.Errorf("SystemOne field %s sent on the Decisions body: %s", forbidden, bodies[0])
				}
			}
			// `questions` is the Decisions array, never SystemOne's keyed object.
			if !strings.Contains(bodies[0], `"questions":[`) {
				t.Errorf("questions is not a JSON array: %s", bodies[0])
			}
		})
	}
}

// TestDecisionsAnswerDecoding: each Decisions answer type maps onto the seam's
// answer — a predicate is a noul P(true), a choice carries its choice,
// confidence and probabilities (an array of value/probability, read as a
// map), a refusal its type alone, and a score its score — every one keyed by
// the name it answered.
func TestDecisionsAnswerDecoding(t *testing.T) {
	const body = `{
		"model": "clef",
		"answers": [
			{"type": "predicate", "name": "q1", "probability": 0.96},
			{"type": "choice", "name": "q2", "choice": "B", "confidence": 0.8,
			 "probabilities": [{"value": "B", "probability": 0.8}, {"value": "A", "probability": 0.2}]},
			{"type": "refusal", "name": "q3"},
			{"type": "score", "name": "q4", "score": 0.4}
		],
		"usage": {"input_tokens": 100, "output_tokens": 5}
	}`
	s := newDecisionServer(t, body)
	c := model.NewOpenAIClient(model.Endpoint{Name: "t", BaseURL: s.srv.URL, APIKey: testAPIKey, Decisions: true})
	req := model.DecisionRequest{Model: "clef", State: "the composed state", Questions: []model.DecisionQuestion{
		{Key: "q1", Type: "noul"}, {Key: "q2", Type: "choice"}, {Key: "q3", Type: "choice"}, {Key: "q4", Type: "score"},
	}}
	resp, err := slotDecision(t, context.Background(), c, req)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if resp.Model != "clef" || resp.Usage.PromptTokens != 100 || resp.Usage.CompletionTokens != 5 {
		t.Errorf("model/usage = %q/%+v, want clef and 100/5", resp.Model, resp.Usage)
	}
	if len(resp.Answers) != 4 {
		t.Fatalf("answers = %+v, want the four the response carried", resp.Answers)
	}
	byKey := map[string]model.DecisionAnswer{}
	for _, a := range resp.Answers {
		byKey[a.Key] = a
	}
	if a := byKey["q1"]; a.Type != "noul" || a.Noul != 0.96 {
		t.Errorf("q1 = %+v, want a noul answer carrying the predicate's probability", a)
	}
	if a := byKey["q2"]; a.Type != "choice" || a.Choice != "B" || a.Confidence != 0.8 ||
		a.Probabilities["B"] != 0.8 || a.Probabilities["A"] != 0.2 {
		t.Errorf("q2 = %+v, want the choice, its confidence and the probabilities as a map", a)
	}
	if a := byKey["q3"]; a.Type != "refusal" || a.Choice != "" || a.Noul != 0 || a.Probabilities != nil {
		t.Errorf("q3 = %+v, want the refusal's type alone", a)
	}
	if a := byKey["q4"]; a.Type != "score" || a.Score != 0.4 {
		t.Errorf("q4 = %+v, want the score", a)
	}
}

// TestDecisionsUnaskedNameRefused: an answer naming a question the request did
// not ask is refused on this surface too, by the name it echoes back.
func TestDecisionsUnaskedNameRefused(t *testing.T) {
	s := newDecisionServer(t, `{"model":"clef","answers":[{"type":"predicate","name":"sneaky","probability":0.5}]}`)
	c := model.NewOpenAIClient(model.Endpoint{Name: "t", BaseURL: s.srv.URL, APIKey: testAPIKey, Decisions: true})
	_, err := slotDecision(t, context.Background(), c, decisionRequest())
	if err == nil {
		t.Fatal("expected error for the unasked name, got nil")
	}
	if !strings.Contains(err.Error(), `unasked question key "sneaky"`) || !strings.Contains(err.Error(), "asked: q1") {
		t.Errorf("error names neither the offending name nor the asked keys: %v", err)
	}
	if s.saw() != 1 {
		t.Errorf("requests: got %d, want 1 — the refusal is not retried", s.saw())
	}
}
