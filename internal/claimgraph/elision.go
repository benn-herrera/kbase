package claimgraph

import (
	"encoding/json"
	"fmt"
	"slices"

	"kbase/internal/kb"
	"kbase/internal/result"
	"kbase/internal/write"
)

// IssueCulls removes each culled claim of the KB at kbRoot through
// remove-node — which takes its record, every edge and demoted record naming
// it, and its statement marker — as one batch through run, and returns the
// outcome. An outcome neither done nor unchanged stops the issue: no retry, no
// re-planning.
func IssueCulls(kbRoot string, ids []string, run func(op string, values []byte) write.Result) (write.Result, error) {
	if len(ids) == 0 {
		return write.Result{}, nil
	}
	g, err := kb.LoadGraph(builtTree(kbRoot))
	if err != nil {
		return write.Result{}, err
	}
	res, err := issue(run, write.Batch, cullEntries(g, ids))
	if err != nil {
		return res, fmt.Errorf("%v: %w", ids, err)
	}
	return res, nil
}

// cullEntries is the batch removing ids from g, each removal valid where it
// stands: remove-node refuses while a surviving record mentions the node, so
// a culled claim whose rationale mentions another culled claim is removed
// before it, the rest in plan order. Where every claim left is mentioned by
// another, an edit-claim entry clears the rationale of each claim mentioning
// the first of them before that one's removal.
func cullEntries(g kb.Graph, ids []string) []map[string]any {
	rationale := map[string]string{}
	for _, n := range g.Claims {
		rationale[n.ID] = n.Rationale
	}
	left := slices.Clone(ids)
	mentioned := func(m string) bool {
		return slices.ContainsFunc(left, func(n string) bool { return n != m && write.Mentions(rationale[n], m) })
	}
	var entries []map[string]any
	for len(left) > 0 {
		i := slices.IndexFunc(left, func(m string) bool { return !mentioned(m) })
		if i < 0 {
			i = 0
			for _, n := range left[1:] {
				if write.Mentions(rationale[n], left[0]) {
					entries = append(entries, map[string]any{"operation": write.EditClaim, "id": n, "rationale": ""})
					rationale[n] = ""
				}
			}
		}
		entries = append(entries, map[string]any{"operation": write.RemoveNode, "id": left[i]})
		left = slices.Delete(left, i, i+1)
	}
	return entries
}

// issue lands one op over its values entries in one batch, refusing an
// outcome the caller cannot proceed from.
func issue(run func(op string, values []byte) write.Result, op string, entries []map[string]any) (write.Result, error) {
	data, err := json.Marshal(map[string]any{"entry": entries})
	if err != nil {
		return write.Result{}, err
	}
	res := run(op, data)
	switch res.Outcome {
	case result.Done, result.Unchanged:
		return res, nil
	default:
		return res, fmt.Errorf("%s ended %s: %s", op, res.Outcome, refusalText(res))
	}
}
