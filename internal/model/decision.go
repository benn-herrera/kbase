package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DecisionQuestion is one typed question of a decision call.
//
// Key is kbase-chosen and opaque to the API; the answer lookup keys on its
// echo — the SystemOne answer's own key, the Decisions answer's name. Type is
// "noul" | "choice" — "score" is deliberately absent until an ask uses it: its
// criteria is an ordered ARRAY on the wire, a different shape, not a map.
// Criteria carries the per-option guidance: "true"/"false" for a noul
// question, option → guidance for a choice question.
type DecisionQuestion struct {
	Key          string
	Type         string
	Instructions string
	Criteria     map[string]string
}

// DecisionRequest is one decision call. kbase sends exactly one question.
//
// State is the composed prompt, under the name each surface gives it: the
// SystemOne request's `state`, the Decisions request's `input`.
type DecisionRequest struct {
	Model     string
	State     string
	Questions []DecisionQuestion
}

// DecisionAnswer is one answer of a decision response, its Key filled from the
// key or name the answer arrived under.
//
// Which fields carry the answer depends on Type: a noul answer carries only
// Noul (P(true)); a choice answer carries Choice and MAY carry Confidence
// and Probabilities — the schema requires only type and choice, so an
// absent optional field decodes to its zero value. A refusal carries its type
// alone: the model declined, which the seam reads as no answer at all. Score
// is read from the Decisions surface's score answer and unread by the seam.
type DecisionAnswer struct {
	Key, Type, Choice       string
	Noul, Score, Confidence float64
	Probabilities           map[string]float64
}

// DecisionResponse is one decision result, its answers decoded into Answers
// with each answer's key preserved.
type DecisionResponse struct {
	Model   string
	Answers []DecisionAnswer
	Usage   Usage // input_tokens → PromptTokens, output_tokens → CompletionTokens
}

// Decision performs one decision-model round-trip over the surface the
// endpoint names: the request re-issued identically up to ChatAttempts on a
// transport error or a retry-class status (429 or 5xx), with no pause between
// attempts; a non-retryable non-2xx (any other 4xx) returns at once as
// StatusError. There is no streaming and no ErrChatIncomplete path — the
// re-issue loop is this method's own, not Chat's. A cancelled ctx returns its
// error at once.
func (c *OpenAIClient) Decision(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	if c.endpoint.BaseURL == "" {
		return DecisionResponse{}, fmt.Errorf("endpoint BaseURL is empty")
	}
	surface := decisionSurfaceOf(c.endpoint.Decisions)
	body, err := surface.encode(req)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("encode request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= ChatAttempts; attempt++ {
		out, err := c.decisionOnce(ctx, surface, body)
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
	return DecisionResponse{}, fmt.Errorf("%s: %d attempts, the last error: %w", surface.path, ChatAttempts, lastErr)
}

// decisionOnce is one POST of the pre-encoded body to the surface's path. A
// transport failure, an undecodable body and a non-2xx all surface as errors;
// the retry loop above decides which of them deserve a re-issue.
func (c *OpenAIClient) decisionOnce(ctx context.Context, surface decisionSurface, body []byte) (DecisionResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, surface.path, mimeJSON, body)
	if err != nil {
		return DecisionResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("read response: %w", err)
	}
	out, err := surface.decode(respBody)
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

// validateDecisionAnswers refuses an answer naming a question the request did
// not ask: an answer to an unasked question is server misbehavior, refused
// rather than ignored — the rule is loosened only by a deliberate change here.
// Each surface fills the answer's Key from what it echoes back, so one check
// serves both. A MISSING answer is not refused: the seam owns that direction
// (re-ask once, then default), so a question unanswered in the response passes
// here.
func validateDecisionAnswers(req DecisionRequest, resp DecisionResponse) error {
	asked := make(map[string]bool, len(req.Questions))
	keys := make([]string, 0, len(req.Questions))
	for _, q := range req.Questions {
		asked[q.Key] = true
		keys = append(keys, q.Key)
	}
	for _, a := range resp.Answers {
		if !asked[a.Key] {
			return fmt.Errorf("decision: answer under unasked question key %q (asked: %s)", a.Key, strings.Join(keys, ", "))
		}
	}
	return nil
}

// decisionSurface is one wire shape a decision endpoint speaks: the path it
// hangs off BaseURL at, and the codecs that write its request and read its
// response.
type decisionSurface struct {
	path   string
	encode func(DecisionRequest) ([]byte, error)
	decode func([]byte) (DecisionResponse, error)
}

// decisionSurfaceOf is the shape the endpoint's resolved surface names: the
// Decisions surface where the caller resolved that, the SystemOne surface
// otherwise — what an endpoint with no flavour named gets.
func decisionSurfaceOf(decisions bool) decisionSurface {
	if decisions {
		return decisionSurface{path: pathDecisions, encode: encodeDecisionsRequest, decode: decodeDecisionsResponse}
	}
	return decisionSurface{path: pathSystemOne, encode: encodeSystemOneRequest, decode: decodeSystemOneResponse}
}

// wireDecisionUsage is the token accounting both surfaces carry, under the
// same names.
type wireDecisionUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
