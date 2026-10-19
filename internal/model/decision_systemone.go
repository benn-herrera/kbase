package model

import (
	"encoding/json"
	"sort"
)

// The SystemOne wire surface: `state` and `questions` a JSON OBJECT keyed by
// kbase's question key — one question, serialized under its own Key — with
// `answers` coming back keyed the same way.
const pathSystemOne = "systemone"

// encodeSystemOneRequest serializes a DecisionRequest to the SystemOne wire
// format: model, state (a JSON string) and the questions as a keyed object.
// Nothing else — no temperature, no chat_template_kwargs, no stream fields.
func encodeSystemOneRequest(req DecisionRequest) ([]byte, error) {
	w := wireSystemOneRequest{
		Model:     req.Model,
		State:     req.State,
		Questions: make(map[string]wireSystemOneQuestion, len(req.Questions)),
	}
	for _, q := range req.Questions {
		w.Questions[q.Key] = wireSystemOneQuestion{
			Type:         q.Type,
			Instructions: q.Instructions,
			Criteria:     q.Criteria,
		}
	}
	return json.Marshal(w)
}

// decodeSystemOneResponse parses a SystemOne response. Unknown keys in the
// answers object and unknown fields on an answer are ignored — read whole,
// act only on what the mapping needs. Answers come out in sorted key order
// so the decode is deterministic whatever order the object arrived in.
func decodeSystemOneResponse(body []byte) (DecisionResponse, error) {
	var w wireSystemOneResponse
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

type wireSystemOneRequest struct {
	Model     string                           `json:"model"`
	State     string                           `json:"state"`
	Questions map[string]wireSystemOneQuestion `json:"questions"`
}

type wireSystemOneQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

type wireSystemOneResponse struct {
	Model   string                         `json:"model"`
	Answers map[string]wireSystemOneAnswer `json:"answers"`
	Usage   wireDecisionUsage              `json:"usage"`
}

type wireSystemOneAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Noul          float64            `json:"noul,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}
