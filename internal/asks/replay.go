//go:build kbasedev

package asks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"

	"go.yaml.in/yaml/v3"
)

// NewReplayBatch is NewBatch, save that a call that never completes leaves
// its item failed and the batch launching the rest; any other error stops
// it.
func NewReplayBatch(ctx context.Context, launch Launch) *Batch {
	b := NewBatch(ctx, launch)
	b.carryOn = true
	return b
}

// RecordedItems is rec's items as the running binary asks them: each item's
// inputs its group's merged with its own, its recorded offered letters, and
// the question its kind's question file states for them.
func RecordedItems(rec GroupRecord) ([]MultichoiceItem, error) {
	out := make([]MultichoiceItem, len(rec.Items))
	for i, it := range rec.Items {
		inputs := make(map[string]string, len(rec.Inputs)+len(it.Inputs))
		maps.Copy(inputs, rec.Inputs)
		maps.Copy(inputs, it.Inputs)
		item, err := multichoiceItem(rec.Kind, it.Item, it.Offered, inputs)
		if err != nil {
			return nil, fmt.Errorf("%s group %s, item %s: %w", rec.Kind, rec.Group, it.Item, err)
		}
		out[i] = item
	}
	return out, nil
}

// ReadGroupRecords is the group records of the file at path, as
// WriteGroupRecords writes them. A field it does not know is ignored, and one
// it lacks is empty, so a record an earlier kbase wrote reads with no inputs.
func ReadGroupRecords(path string) ([]GroupRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []GroupRecord
	for dec := yaml.NewDecoder(f); ; {
		var rec GroupRecord
		if err := dec.Decode(&rec); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, rec)
	}
}
