//go:build kbasedev

package asks

import (
	"errors"
	"os"
	"slices"
	"testing"
)

// TestReadGroupRecordsReadsAnEarlierRecord: a record written before inputs
// were recorded, or carrying a field this kbase does not know, reads with
// empty inputs.
func TestReadGroupRecordsReadsAnEarlierRecord(t *testing.T) {
	path := GroupRecordsPath(t.TempDir(), Paragraph)
	earlier := "kind: paragraph\ngroup: v/a.md\nprefix: \"the leaf\\n\"\nretired: true\nitems:\n  - item: S1\n    outcome: answered\n"
	if err := os.WriteFile(path, []byte(earlier), 0o666); err != nil {
		t.Fatal(err)
	}
	got, err := ReadGroupRecords(path)
	if err != nil || len(got) != 1 || len(got[0].Items) != 1 || got[0].Inputs != nil || got[0].Items[0].Inputs != nil || got[0].Items[0].Outcome != Answered {
		t.Errorf("ReadGroupRecords = %+v, %v; want the one group, its item answered, no inputs", got, err)
	}
}

// TestRecordedItemsReproduceTheAskedState: for every kind, the items
// RecordedItems makes of a group's record compose the very state each was
// asked with, and that state with the item's question digests to the
// record's prompt-sha256.
func TestRecordedItemsReproduceTheAskedState(t *testing.T) {
	for _, kind := range Kinds {
		t.Run(string(kind), func(t *testing.T) {
			h := &heard{}
			rec, err := askedBatch(t, NewBatch, readerLaunch(h.read), kind, itemsOf(t, kind))
			if err != nil {
				t.Fatal(err)
			}
			again, err := RecordedItems(rec)
			if err != nil || len(again) != len(rec.Items) {
				t.Fatalf("RecordedItems = %d items, %v; want %d", len(again), err, len(rec.Items))
			}
			for i, it := range rec.Items {
				state := h.stateOf(t, it.Item)
				if got, err := again[i].Compose(); err != nil || got != state {
					t.Errorf("%s re-composes %v:\n%s\nwant the asked state:\n%s", it.Item, err, got, state)
				}
				if AskDigest(state, again[i].Question) != it.PromptSHA256 {
					t.Errorf("%s: the asked state and the re-made question do not digest to the recorded prompt-sha256", it.Item)
				}
			}
		})
	}
}

// TestAReplayBatchCarriesOnPastAFailedItem: one item at a time, the second's
// call never completing. A replay's batch records the second failed with its
// inputs and digest and no call, and carries on to answer the third.
func TestAReplayBatchCarriesOnPastAFailedItem(t *testing.T) {
	h := &heard{scripted: scripted{replies: map[string][]string{"S1": {LetterClaim}, "S3": {LetterClaim}}}}
	rec, err := askedBatch(t, NewReplayBatch, serialLaunch(h.read), Paragraph, paragraphItems("S1", "S2", "S3"))
	var incomplete IncompleteError
	if errors.As(err, &incomplete) {
		t.Fatalf("Wait = %v; want no stop", err)
	}
	var got []Outcome
	for _, it := range rec.Items {
		got = append(got, it.Outcome)
	}
	if want := []Outcome{Answered, Failed, Answered}; !slices.Equal(got, want) {
		t.Fatalf("recorded %q, want %q", got, want)
	}
	failed := rec.Items[1]
	if failed.Choice != nil || failed.Confidence != nil || len(failed.Calls) != 0 || len(failed.Inputs) == 0 ||
		failed.PromptSHA256 != AskDigest(h.stateOf(t, failed.Item), paragraphItems(failed.Item)[0].Question) {
		t.Errorf("the failed item = %+v; want no choice, no confidence, no call, its inputs and its prompt-sha256", failed)
	}
}
