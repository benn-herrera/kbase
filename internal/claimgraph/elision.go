package claimgraph

import (
	"encoding/json"
	"fmt"

	"kbase/internal/result"
	"kbase/internal/write"
)

// IssueCulls issues drop-claim-edges, drop-inbound-edges and drop-claim-node —
// which takes the node's record and its statement marker — through run, each
// as one batch over ids in order, and returns the outcomes. A batch whose
// outcome is neither done nor unchanged stops the issue: no retry, no
// re-planning.
func IssueCulls(ids []string, run func(op string, values []byte) write.Result) ([]write.Result, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	entries := make([]map[string]any, len(ids))
	for i, id := range ids {
		entries[i] = map[string]any{"id": id}
	}
	var out []write.Result
	for _, op := range []string{"drop-claim-edges", "drop-inbound-edges", "drop-claim-node"} {
		res, err := issue(run, op, entries)
		if err != nil {
			return out, fmt.Errorf("%s %v: %w", op, ids, err)
		}
		out = append(out, res)
	}
	return out, nil
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
