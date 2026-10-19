package model

import (
	"encoding/json"
	"sort"
)

// The Decisions wire surface: `input` carrying the composed prompt, questions
// an ARRAY of named questions each holding its choices as value/description
// pairs, and `answers` an array whose entries each carry the name they answer.
const pathDecisions = "decisions"

// encodeDecisionsRequest serializes a DecisionRequest to the Decisions wire
// format: model, input and one named question per — nothing else.
func encodeDecisionsRequest(req DecisionRequest) ([]byte, error) {
	w := wireDecisionsRequest{
		Model:     req.Model,
		Input:     req.State,
		Questions: make([]wireDecisionsQuestion, 0, len(req.Questions)),
	}
	for _, q := range req.Questions {
		wq := wireDecisionsQuestion{
			Type:         decisionsQuestionType(q.Type),
			Name:         q.Key,
			Instructions: q.Instructions,
		}
		if wq.Type == "predicate" {
			wq.Instructions = predicateInstructions(q)
		} else {
			wq.Choices = decisionsChoices(q.Criteria)
		}
		w.Questions = append(w.Questions, wq)
	}
	return json.Marshal(w)
}

// predicateInstructions is a noul question's instructions with its criterion
// guidance folded in: the Decisions surface gives a predicate no choices
// field, so the true/false guidance compose requires of a noul question has
// nowhere else on the wire to go.
func predicateInstructions(q DecisionQuestion) string {
	whenTrue, whenFalse := q.Criteria["true"], q.Criteria["false"]
	if whenTrue == "" || whenFalse == "" {
		return q.Instructions
	}
	guidance := "The condition is true when: " + whenTrue + "\nIt is false when: " + whenFalse
	if q.Instructions == "" {
		return guidance
	}
	return q.Instructions + "\n\n" + guidance
}

// decisionsQuestionType maps kbase's question type onto the Decisions
// surface's: a noul question asks for one probability, which the surface asks
// as a predicate. Every other type keeps its own name.
func decisionsQuestionType(questionType string) string {
	if questionType == "noul" {
		return "predicate"
	}
	return questionType
}

// decisionsChoices is a question's criteria as the Decisions surface's
// choices, sorted by value so one question always serializes to the same
// bytes.
func decisionsChoices(criteria map[string]string) []wireDecisionsChoice {
	values := make([]string, 0, len(criteria))
	for value := range criteria {
		values = append(values, value)
	}
	sort.Strings(values)
	out := make([]wireDecisionsChoice, 0, len(values))
	for _, value := range values {
		out = append(out, wireDecisionsChoice{Value: value, Description: criteria[value]})
	}
	return out
}

// decodeDecisionsResponse parses a Decisions response: one answer per entry of
// the answers array, in the order it arrived, each keyed by the name it
// carries. Unknown fields are ignored, and a question the response leaves
// unanswered simply has no entry — the seam owns that direction.
func decodeDecisionsResponse(body []byte) (DecisionResponse, error) {
	var w wireDecisionsResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return DecisionResponse{}, err
	}
	out := DecisionResponse{
		Model: w.Model,
		Usage: Usage{PromptTokens: w.Usage.InputTokens, CompletionTokens: w.Usage.OutputTokens},
	}
	for _, a := range w.Answers {
		out.Answers = append(out.Answers, decisionAnswer(a))
	}
	return out, nil
}

// decisionAnswer maps one Decisions answer onto the seam's answer: a predicate
// is a noul answer's P(true), a choice carries its choice, confidence and
// per-value probabilities, and a score its score. A refusal — and any type the
// seam does not know — keeps its own type and carries no value, which the seam
// reads as no answer at all.
func decisionAnswer(a wireDecisionsAnswer) DecisionAnswer {
	switch a.Type {
	case "predicate":
		return DecisionAnswer{Key: a.Name, Type: "noul", Noul: a.Probability}
	case "choice":
		answer := DecisionAnswer{Key: a.Name, Type: "choice", Choice: a.Choice, Confidence: a.Confidence}
		for _, p := range a.Probabilities {
			if answer.Probabilities == nil {
				answer.Probabilities = make(map[string]float64, len(a.Probabilities))
			}
			answer.Probabilities[p.Value] = p.Probability
		}
		return answer
	case "score":
		return DecisionAnswer{Key: a.Name, Type: "score", Score: a.Score}
	default:
		return DecisionAnswer{Key: a.Name, Type: a.Type}
	}
}

// --- wire format (Decisions) ---

type wireDecisionsRequest struct {
	Model     string                  `json:"model"`
	Input     string                  `json:"input"`
	Questions []wireDecisionsQuestion `json:"questions"`
}

type wireDecisionsQuestion struct {
	Type         string                `json:"type"`
	Name         string                `json:"name"`
	Instructions string                `json:"instructions,omitempty"`
	Choices      []wireDecisionsChoice `json:"choices,omitempty"`
}

type wireDecisionsChoice struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type wireDecisionsResponse struct {
	Model   string                `json:"model"`
	Answers []wireDecisionsAnswer `json:"answers"`
	Usage   wireDecisionUsage     `json:"usage"`
}

type wireDecisionsAnswer struct {
	Type          string                     `json:"type"`
	Name          string                     `json:"name"`
	Probability   float64                    `json:"probability"`
	Choice        string                     `json:"choice"`
	Confidence    float64                    `json:"confidence"`
	Score         float64                    `json:"score"`
	Probabilities []wireDecisionsProbability `json:"probabilities"`
}

type wireDecisionsProbability struct {
	Value       string  `json:"value"`
	Probability float64 `json:"probability"`
}
