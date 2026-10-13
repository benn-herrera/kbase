package model

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
)

// systemOneRequest is the one-question request the tests send.
func systemOneRequest() DecisionRequest {
	return DecisionRequest{
		Model: "clef",
		State: "the composed state",
		Questions: []DecisionQuestion{{
			Key:          "q1",
			Type:         "choice",
			Instructions: "pick one",
			Criteria:     map[string]string{"A": "first", "B": "second"},
		}},
	}
}

// systemOneServer serves the given responses in order: an element of the
// form "status:<code>" is sent as that status with a short body; the
// sentinel nilBody kills the connection unread, a transport error; anything
// else is sent as a 200 body. The handler records method, path, auth header
// and body of every request it saw. The handler runs on the server's
// goroutine, so every field it touches is guarded by mu.
type systemOneServer struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests int
	method   string
	path     string
	auth     string
	bodies   []string
}

const systemOneNilBody = "\x00nil"

func systemOneStatus(code int) string { return fmt.Sprintf("status:%d", code) }

func newSystemOneServer(t *testing.T, responses ...string) *systemOneServer {
	t.Helper()
	s := &systemOneServer{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.requests++
		s.method, s.path, s.auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		s.bodies = append(s.bodies, string(body))
		resp := responses[min(s.requests-1, len(responses)-1)]
		switch {
		case resp == systemOneNilBody:
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
func (s *systemOneServer) saw() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

// snapshot returns the recorded method, auth header and request bodies.
func (s *systemOneServer) snapshot() (method, auth string, bodies []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.method, s.auth, append([]string(nil), s.bodies...)
}

func TestSystemOneURLSuffix(t *testing.T) {
	cases := []struct {
		name    string
		base    func(string) string
		want    string
		comment string
	}{
		{"bare host", func(u string) string { return u }, "/systemone", ""},
		{"v1 base", func(u string) string { return u + "/v1" }, "/v1/systemone", ""},
		{"trailing slash", func(u string) string { return u + "/" }, "/systemone", "no doubled slash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte(`{"model":"clef","answers":{},"usage":{}}`))
			}))
			defer srv.Close()

			c := NewHTTPClient(Endpoint{Name: "t", BaseURL: tc.base(srv.URL), APIKey: testAPIKey})
			if _, err := c.SystemOne(context.Background(), systemOneRequest()); err != nil {
				t.Fatalf("SystemOne: %v", err)
			}
			if gotPath != tc.want {
				t.Errorf("path: got %q, want %q (%s)", gotPath, tc.want, tc.comment)
			}
		})
	}
}

func TestSystemOnePayloadShape(t *testing.T) {
	s := newSystemOneServer(t, `{"model":"clef","answers":{},"usage":{}}`)
	c := NewHTTPClient(newTestEndpoint(s.srv.URL))
	if _, err := c.SystemOne(context.Background(), systemOneRequest()); err != nil {
		t.Fatalf("SystemOne: %v", err)
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
	for _, forbidden := range []string{"temperature", "stream", "stream_options", "chat_template_kwargs", "messages"} {
		if _, ok := body[forbidden]; ok {
			t.Errorf("chat-only field %q sent on the SystemOne body: %s", forbidden, bodies[0])
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
	s := newSystemOneServer(t, body)
	c := NewHTTPClient(newTestEndpoint(s.srv.URL))
	req := DecisionRequest{
		Model: "clef",
		State: "the composed state",
		Questions: []DecisionQuestion{
			{Key: "choice-full", Type: "choice"},
			{Key: "choice-bare", Type: "choice"},
			{Key: "noul-q", Type: "noul"},
		},
	}
	resp, err := c.SystemOne(context.Background(), req)
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
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
	byKey := map[string]DecisionAnswer{}
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
	s := newSystemOneServer(t, systemOneNilBody, `{"model":"clef","answers":{"q1":{"type":"choice","choice":"A"}},"usage":{"input_tokens":1,"output_tokens":1}}`)
	c := NewHTTPClient(newTestEndpoint(s.srv.URL))
	resp, err := c.SystemOne(context.Background(), systemOneRequest())
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if s.saw() != 2 {
		t.Errorf("requests: got %d, want 2 (one transport failure, one re-issue)", s.saw())
	}
	if len(resp.Answers) != 1 || resp.Answers[0].Key != "q1" || resp.Answers[0].Choice != "A" {
		t.Errorf("answers = %+v, want the re-issued call's answer", resp.Answers)
	}
}

func TestSystemOneRetry500ThenSuccess(t *testing.T) {
	s := newSystemOneServer(t, systemOneStatus(500), `{"model":"clef","answers":{},"usage":{"input_tokens":7,"output_tokens":2}}`)
	c := NewHTTPClient(newTestEndpoint(s.srv.URL))
	resp, err := c.SystemOne(context.Background(), systemOneRequest())
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if s.saw() != 2 {
		t.Errorf("requests: got %d, want 2 (one 500, one re-issue)", s.saw())
	}
	if resp.Usage.PromptTokens != 7 || resp.Usage.CompletionTokens != 2 {
		t.Errorf("usage: got %+v, want the successful call's figures", resp.Usage)
	}
}

func TestSystemOne4xxNoRetry(t *testing.T) {
	s := newSystemOneServer(t, systemOneStatus(400))
	c := NewHTTPClient(newTestEndpoint(s.srv.URL))
	_, err := c.SystemOne(context.Background(), systemOneRequest())
	if err == nil {
		t.Fatal("expected error from 400, got nil")
	}
	var se StatusError
	if !errors.As(err, &se) || se.Code != http.StatusBadRequest {
		t.Errorf("err = %v, want a StatusError with code 400", err)
	}
	if s.saw() != 1 {
		t.Errorf("requests: got %d, want 1 — a 4xx is not re-issued", s.saw())
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("API key leaked into the error: %v", err)
	}
}

func TestSystemOneAttemptsExhausted(t *testing.T) {
	s := newSystemOneServer(t, systemOneNilBody, systemOneNilBody, systemOneNilBody)
	c := NewHTTPClient(newTestEndpoint(s.srv.URL))
	_, err := c.SystemOne(context.Background(), systemOneRequest())
	if err == nil {
		t.Fatal("expected error after exhausted attempts, got nil")
	}
	if s.saw() != ChatAttempts {
		t.Errorf("requests: got %d, want %d", s.saw(), ChatAttempts)
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

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	_, err := c.SystemOne(ctx, systemOneRequest())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestMockSystemOneScripted(t *testing.T) {
	m := NewScriptedMockWithDecisions([]DecisionAnswer{
		{Key: "q1", Type: "choice", Choice: "A", Probabilities: map[string]float64{"A": 0.9, "B": 0.1}},
		{Key: "q2", Type: "noul", Noul: 0.96},
	})
	ctx := context.Background()
	first, err := m.SystemOne(ctx, DecisionRequest{Model: "clef"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(first.Answers) != 1 || first.Answers[0].Key != "q1" || first.Answers[0].Choice != "A" ||
		first.Answers[0].Probabilities["B"] != 0.1 {
		t.Errorf("first = %+v, want the first scripted answer with probabilities", first)
	}
	second, err := m.SystemOne(ctx, DecisionRequest{})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(second.Answers) != 1 || second.Answers[0].Noul != 0.96 {
		t.Errorf("second = %+v, want the noul answer", second)
	}
	if _, err := m.SystemOne(ctx, DecisionRequest{}); !errors.Is(err, ErrMockExhausted) {
		t.Errorf("past the script: got %v, want ErrMockExhausted", err)
	}
}

func TestMockSystemOneUnscriptedExhausted(t *testing.T) {
	m := NewScriptedMock(nil, nil)
	if _, err := m.SystemOne(context.Background(), DecisionRequest{}); !errors.Is(err, ErrMockExhausted) {
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
			s := newSystemOneServer(t, `{"model":"clef","answers":`+tc.answers+`,"usage":{"input_tokens":1,"output_tokens":1}}`)
			c := NewHTTPClient(newTestEndpoint(s.srv.URL))
			_, err := c.SystemOne(context.Background(), systemOneRequest())
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
	s := newSystemOneServer(t, `{"model":"clef","answers":{"q2":{"type":"noul","noul":0.9}},"usage":{}}`)
	c := NewHTTPClient(newTestEndpoint(s.srv.URL))
	req := DecisionRequest{
		Model: "clef",
		State: "state",
		Questions: []DecisionQuestion{
			{Key: "q1", Type: "choice", Instructions: "pick"},
			{Key: "q2", Type: "noul", Instructions: "judge"},
		},
	}
	resp, err := c.SystemOne(context.Background(), req)
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if len(resp.Answers) != 1 || resp.Answers[0].Key != "q2" || resp.Answers[0].Noul != 0.9 {
		t.Errorf("answers = %+v, want q2's answer alone, q1's absence left to the seam", resp.Answers)
	}
}
