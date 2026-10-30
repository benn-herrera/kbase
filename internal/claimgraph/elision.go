package claimgraph

import (
	"encoding/json"
	"fmt"

	"kbase/internal/result"
	"kbase/internal/write"
)

// IssueCulls issues drop-claim-node — which takes each node's record, every
// edge and demoted record naming it, and its statement marker — through run,
// as one batch over ids in order, and returns the outcome. An outcome neither
// done nor unchanged stops the issue: no retry, no re-planning.
func IssueCulls(ids []string, run func(op string, values []byte) write.Result) (write.Result, error) {
	if len(ids) == 0 {
		return write.Result{}, nil
	}
	entries := make([]map[string]any, len(ids))
	for i, id := range ids {
		entries[i] = map[string]any{"id": id}
	}
	res, err := issue(run, "drop-claim-node", entries)
	if err != nil {
		return res, fmt.Errorf("%v: %w", ids, err)
	}
	return res, nil
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
