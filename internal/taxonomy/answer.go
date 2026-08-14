package taxonomy

import (
	"encoding/json"
	"strings"

	"kbase/internal/text"
	"kbase/internal/treeplan"
)

// The answer transport: the model returns one JSON object, and this turns it
// into the typed answer the verifier checks (treeplan.GroupingAnswer).
//
// The schema is documented in the definition the call carries (see
// stubDefinition) and stated once, here, in the types that read it:
//
//	{"groups":[{"title":"…","scope":"…","kind":"page","members":[1,2]}]}
//
// # Model-facing vocabulary
//
// `kind` is "page" or "section", not the artifact's "leaf" and "index". It is
// the vocabulary every rejection note already uses ("these must be pages, not
// sections"), and a model told one word in the prompt and corrected with
// another is being asked to translate on the retry.
//
// # Members are 1-based
//
// The candidate list is numbered from 1, so the answer is too; the mapping to
// the 0-based indices treeplan.CheckAnswer partitions over happens here, in the
// one place that knows both spellings. An out-of-range number survives the
// mapping deliberately — the partition check is what refuses it, with a note.
const (
	// kindPage and kindSection are the model-facing flag values. The set is
	// closed at two: `annex` is not a model-facing flag, because a model that
	// can place material out of scope can hide its own failures (O-12, §2.8).
	kindPage    = "page"
	kindSection = "section"
)

// answer is the wire shape of one container call's response.
type answer struct {
	Groups []answerGroup `json:"groups"`
}

type answerGroup struct {
	Title   string `json:"title"`
	Scope   string `json:"scope"`
	Kind    string `json:"kind"`
	Members []int  `json:"members"`
}

// parseAnswer reads a response into the typed answer.
//
// Unknown fields are tolerated and missing ones are not. The post-condition
// this seam owns is the partition and the caps (§3.3), not JSON hygiene: a
// model that added a field has still answered the question, while a model that
// omitted `members` has not. What is NOT tolerated is prose instead of an
// object — the object is extracted from the first `{` to the last `}` so a
// fenced block or a trailing sentence still parses, and anything else is a
// rejection carrying the one-line correction.
func parseAnswer(response string) (treeplan.GroupingAnswer, error) {
	obj, ok := text.JSONObject(response)
	if !ok {
		return treeplan.GroupingAnswer{}, treeplan.RejectionError{
			Reason: "answer with the JSON object described and nothing else"}
	}
	var a answer
	if err := json.Unmarshal([]byte(obj), &a); err != nil {
		return treeplan.GroupingAnswer{}, treeplan.RejectionError{
			Reason: "the answer is not the JSON object described"}
	}
	out := treeplan.GroupingAnswer{Groups: make([]treeplan.AnswerGroup, 0, len(a.Groups))}
	for _, g := range a.Groups {
		out.Groups = append(out.Groups, treeplan.AnswerGroup{
			Title:   g.Title,
			Scope:   g.Scope,
			Kind:    nodeKind(g.Kind),
			Members: zeroBased(g.Members),
		})
	}
	return out, nil
}

// nodeKind maps the model's flag onto the artifact's node kind. An unknown
// value maps to itself, so the rejection comes from CheckAnswer's own closed
// set ("a group is neither a page nor a section") rather than from a second
// spelling of that rule here.
func nodeKind(flag string) treeplan.Kind {
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case kindPage:
		return treeplan.KindLeaf
	case kindSection:
		return treeplan.KindIndex
	default:
		return treeplan.Kind(flag)
	}
}

// zeroBased shifts the answer's 1-based entry numbers onto the candidate
// slice's own indices. Out-of-range values pass through to the partition
// check, which is the one place an entry number is adjudicated.
func zeroBased(members []int) []int {
	out := make([]int, 0, len(members))
	for _, m := range members {
		out = append(out, m-1)
	}
	return out
}
