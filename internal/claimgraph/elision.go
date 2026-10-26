package claimgraph

import (
	"encoding/json"
	"fmt"

	"kbase/internal/result"
	"kbase/internal/write"
)

// IssueCulls issues, for each id in order, drop-claim-edges, drop-inbound-edges
// and drop-claim-node — which takes the node's record and its statement
// marker — through run, and returns the outcomes. A run whose outcome is
// neither done nor unchanged stops the issue: no retry, no re-planning.
func IssueCulls(ids []string, run func(op string, values []byte) write.Result) ([]write.Result, error) {
	var out []write.Result
	for _, id := range ids {
		for _, op := range []string{"drop-claim-edges", "drop-inbound-edges", "drop-claim-node"} {
			res, err := issue(run, op, map[string]any{"id": id})
			if err != nil {
				return out, fmt.Errorf("%s %s: %w", op, id, err)
			}
			out = append(out, res)
		}
	}
	return out, nil
}

// issue lands one op over one values entry, refusing an outcome the caller
// cannot proceed from.
func issue(run func(op string, values []byte) write.Result, op string, e map[string]any) (write.Result, error) {
	data, err := json.Marshal(map[string]any{"entry": []map[string]any{e}})
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
