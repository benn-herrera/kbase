package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// The SystemOne wire surface: the decision-model endpoint hangs off
// Endpoint.BaseURL beside chat/completions and models.
const pathSystemOne = "systemone"

// DecisionQuestion is one typed question of a SystemOne request.
//
// Key is kbase-chosen and opaque to the API; the answer lookup keys on its
// echo. Type is "noul" | "choice" — "score" is deliberately absent until an
// ask uses it: its criteria is an ordered ARRAY on the wire, a different
// shape, not a map. Criteria carries the per-option guidance: "true"/"false"
// for a noul question, option → guidance for a choice question.
type DecisionQuestion struct {
	Key          string
	Type         string
	Instructions string
	Criteria     map[string]string
}

// DecisionRequest is one SystemOne call. kbase sends exactly one question.
//
// Wire form: `questions` is a JSON OBJECT keyed by question key, not an
// array — one question, serialized under its own Key. `answers` comes back
// keyed the same way.
type DecisionRequest struct {
	Model     string
	State     string
	Questions []DecisionQuestion
}

// DecisionAnswer is one answer of a SystemOne response, its Key filled from
// the key the answer arrived under.
//
// Which fields carry the answer depends on Type: a noul answer carries only
// Noul (P(true)); a choice answer carries Choice and MAY carry Confidence
// and Probabilities — the schema requires only type and choice, so an
// absent optional field decodes to its zero value. Score stays at zero
// until the score question type exists.
type DecisionAnswer struct {
	Key, Type, Choice       string
	Noul, Score, Confidence float64
	Probabilities           map[string]float64
}

// DecisionResponse is one SystemOne result, the keyed `answers` object
// decoded into Answers with each answer's key preserved.
type DecisionResponse struct {
	Model   string
	Answers []DecisionAnswer
	Usage   Usage // input_tokens → PromptTokens, output_tokens → CompletionTokens
}

// SystemOne performs one decision-model round-trip: the request re-issued
// identically up to ChatAttempts on a transport error or a retry-class
// status (429 or 5xx), with no pause between attempts; a non-retryable
// non-2xx (any other 4xx) returns at once as StatusError. There is no
// streaming and no ErrChatIncomplete path — the re-issue loop is this
// method's own, not Chat's. A cancelled ctx returns its error at once.
func (c *HTTPClient) SystemOne(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	if c.endpoint.BaseURL == "" {
		return DecisionResponse{}, fmt.Errorf("endpoint BaseURL is empty")
	}
	body, err := encodeDecisionRequest(req)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("encode request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= ChatAttempts; attempt++ {
		out, err := c.systemOneOnce(ctx, body)
		if err == nil {
			if err := validateDecisionAnswers(req, out); err != nil {
				return DecisionResponse{}, err
			}
			return out, nil
		}
		lastErr = err
		var se StatusError
		if errors.As(err, &se) && !retryableStatus(se.Code) {
			return DecisionResponse{}, err
		}
		// As Chat does: a ctx cancelled during an attempt ends the call
		// with the ctx's own error instead of a re-issue.
		if err := ctx.Err(); err != nil {
			return DecisionResponse{}, err
		}
	}
	return DecisionResponse{}, fmt.Errorf("systemone: %d attempts, the last error: %w", ChatAttempts, lastErr)
}

// systemOneOnce is one SystemOne POST of the pre-encoded body. A transport
// failure, an undecodable body and a non-2xx all surface as errors; the
// retry loop above decides which of them deserve a re-issue.
func (c *HTTPClient) systemOneOnce(ctx context.Context, body []byte) (DecisionResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, pathSystemOne, mimeJSON, body)
	if err != nil {
		return DecisionResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("read response: %w", err)
	}
	out, err := decodeDecisionResponse(respBody)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

// retryableStatus reports whether a non-2xx status is worth an identical
// re-issue: 429 (later) and the 5xx class. Any other 4xx is the request
// itself being wrong and will be just as wrong three times.
func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code/100 == 5
}

// validateDecisionAnswers refuses an answer that arrived under a question
// key the request did not ask: an answer to an unasked question is server
// misbehavior, refused rather than ignored — the rule is loosened only by a
// deliberate change here. A MISSING answer is not refused: the seam owns
// that direction (re-ask once, then default), so a question unanswered in
// the response passes here.
func validateDecisionAnswers(req DecisionRequest, resp DecisionResponse) error {
	asked := make(map[string]bool, len(req.Questions))
	keys := make([]string, 0, len(req.Questions))
	for _, q := range req.Questions {
		asked[q.Key] = true
		keys = append(keys, q.Key)
	}
	for _, a := range resp.Answers {
		if !asked[a.Key] {
			return fmt.Errorf("systemone: answer under unasked question key %q (asked: %s)", a.Key, strings.Join(keys, ", "))
		}
	}
	return nil
}

// encodeDecisionRequest serializes a DecisionRequest to the SystemOne wire
// format: model, state (a JSON string) and the questions as a keyed object.
// Nothing else — no temperature, no chat_template_kwargs, no stream fields.
func encodeDecisionRequest(req DecisionRequest) ([]byte, error) {
	w := wireDecisionRequest{
		Model:     req.Model,
		State:     req.State,
		Questions: make(map[string]wireDecisionQuestion, len(req.Questions)),
	}
	for _, q := range req.Questions {
		w.Questions[q.Key] = wireDecisionQuestion{
			Type:         q.Type,
			Instructions: q.Instructions,
			Criteria:     q.Criteria,
		}
	}
	return json.Marshal(w)
}

// decodeDecisionResponse parses a SystemOne response. Unknown keys in the
// answers object and unknown fields on an answer are ignored — read whole,
// act only on what the mapping needs. Answers come out in sorted key order
// so the decode is deterministic whatever order the object arrived in.
func decodeDecisionResponse(body []byte) (DecisionResponse, error) {
	var w wireDecisionResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return DecisionResponse{}, err
	}
	out := DecisionResponse{
		Model: w.Model,
		Usage: Usage{PromptTokens: w.Usage.InputTokens, CompletionTokens: w.Usage.OutputTokens},
	}
	keys := make([]string, 0, len(w.Answers))
	for key := range w.Answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		a := w.Answers[key]
		out.Answers = append(out.Answers, DecisionAnswer{
			Key:           key,
			Type:          a.Type,
			Choice:        a.Choice,
			Noul:          a.Noul,
			Confidence:    a.Confidence,
			Probabilities: a.Probabilities,
		})
	}
	return out, nil
}

// --- wire format (SystemOne) ---

type wireDecisionRequest struct {
	Model     string                          `json:"model"`
	State     string                          `json:"state"`
	Questions map[string]wireDecisionQuestion `json:"questions"`
}

type wireDecisionQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

type wireDecisionResponse struct {
	Model   string                        `json:"model"`
	Answers map[string]wireDecisionAnswer `json:"answers"`
	Usage   wireDecisionUsage             `json:"usage"`
}

type wireDecisionAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Noul          float64            `json:"noul,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type wireDecisionUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
