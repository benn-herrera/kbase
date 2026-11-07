package model

import "testing"

// correctionQuestions are a noul and a choice question, each with criteria.
var correctionQuestions = map[string]DecisionQuestion{
	"noul": {Key: "S1", Type: "noul", Instructions: "judge the step",
		Criteria: map[string]string{"true": "the step follows", "false": "it does not"}},
	"choice": {Key: "S2", Type: "choice", Instructions: "pick the letter",
		Criteria: map[string]string{"D": "fourth", "C": "third"}},
}

// TestDecisionBodiesWithoutCorrection pins each shape's body with no
// correction byte for byte, which the shape tests, checking field by field,
// do not: the server's prefix cache depends on these bytes. The answer cache
// does not; it is keyed by the question's decisionDigest.
func TestDecisionBodiesWithoutCorrection(t *testing.T) {
	for _, tc := range []struct {
		name, question string
		encode         func(DecisionRequest) ([]byte, error)
		want           string
	}{
		{"systemone noul", "noul", encodeSystemOneRequest,
			`{"model":"clef","state":"the state","questions":{"S1":{"type":"noul","instructions":"judge the step","criteria":{"false":"it does not","true":"the step follows"}}}}`},
		{"systemone choice", "choice", encodeSystemOneRequest,
			`{"model":"clef","state":"the state","questions":{"S2":{"type":"choice","instructions":"pick the letter","criteria":{"C":"third","D":"fourth"}}}}`},
		{"decisions predicate", "noul", encodeDecisionsRequest,
			`{"model":"clef","input":"the state","questions":[{"type":"predicate","name":"S1","instructions":"judge the step\n\nThe condition is true when: the step follows\nIt is false when: it does not"}]}`},
		{"decisions choice", "choice", encodeDecisionsRequest,
			`{"model":"clef","input":"the state","questions":[{"type":"choice","name":"S2","instructions":"pick the letter","choices":[{"value":"C","description":"third"},{"value":"D","description":"fourth"}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := DecisionRequest{Model: "clef", State: "the state", Questions: []DecisionQuestion{correctionQuestions[tc.question]}}
			got, err := tc.encode(req)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("body:\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestACorrectionEndsWhereEachShapePlacesIt: a corrected body is the
// uncorrected body with the correction appended verbatim to one field — the
// question's instructions on the Decisions surface, after a predicate's
// guidance; the state on the SystemOne surface — and nothing else moved.
func TestACorrectionEndsWhereEachShapePlacesIt(t *testing.T) {
	const correction = "\n\nAsked again."
	for _, tc := range []struct {
		name, question string
		encode         func(DecisionRequest) ([]byte, error)
		// placed is the uncorrected request with the correction already in
		// the field it belongs to.
		placed func(DecisionRequest) DecisionRequest
	}{
		{"systemone noul", "noul", encodeSystemOneRequest, func(r DecisionRequest) DecisionRequest {
			r.State += correction
			return r
		}},
		{"systemone choice", "choice", encodeSystemOneRequest, func(r DecisionRequest) DecisionRequest {
			r.State += correction
			return r
		}},
		{"decisions predicate", "noul", encodeDecisionsRequest, func(r DecisionRequest) DecisionRequest {
			q := r.Questions[0]
			q.Instructions = predicateInstructions(q) + correction
			q.Criteria = nil
			r.Questions = []DecisionQuestion{q}
			return r
		}},
		{"decisions choice", "choice", encodeDecisionsRequest, func(r DecisionRequest) DecisionRequest {
			q := r.Questions[0]
			q.Instructions += correction
			r.Questions = []DecisionQuestion{q}
			return r
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := DecisionRequest{Model: "clef", State: "the state", Questions: []DecisionQuestion{correctionQuestions[tc.question]}}
			want, err := tc.encode(tc.placed(req))
			if err != nil {
				t.Fatalf("encode placed: %v", err)
			}
			req.Correction = correction
			got, err := tc.encode(req)
			if err != nil {
				t.Fatalf("encode corrected: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("body:\n got %s\nwant %s", got, want)
			}
		})
	}
}
