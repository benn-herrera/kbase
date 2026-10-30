//go:build kbasedev

package build

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kbase/internal/asks"
)

// groupRecords is every group record of kind the build in f wrote, in the
// order the stage launched the groups.
func groupRecords(t *testing.T, f fixture, kind asks.Kind) []asks.GroupRecord {
	t.Helper()
	out, err := asks.ReadGroupRecords(asks.GroupRecordsPath(filepath.Join(f.startTrailDir(t), asks.AskRecordsDir), kind))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// itemAsk is the ask the i-th item of g made.
func itemAsk(t *testing.T, g asks.GroupRecord, i int) string {
	t.Helper()
	items, err := asks.RecordedItems(g)
	if err != nil {
		t.Fatal(err)
	}
	state, err := items[i].Compose()
	if err != nil {
		t.Fatal(err)
	}
	return askName(state, g.Items[i].Item)
}

// TestABuildAsksInSubmissionOrder: at N=1 each stage's multichoice asks reach the
// wire in the order the stage launched them — its groups in order, each
// group's items in order, an item's re-ask right behind it.
func TestABuildAsksInSubmissionOrder(t *testing.T) {
	w := newWire(func(string) time.Duration { return 0 })
	f := builtAt(t, w, 1)
	for _, kind := range asks.Kinds {
		var launched []string
		for _, g := range groupRecords(t, f, kind) {
			for i, item := range g.Items {
				for range item.Calls {
					launched = append(launched, itemAsk(t, g, i))
				}
			}
		}
		var arrived []string
		for _, e := range w.events {
			if !e.returned && slices.Contains(launched, e.ask) {
				arrived = append(arrived, e.ask)
			}
		}
		if len(launched) == 0 || !slices.Equal(arrived, launched) {
			t.Errorf("%s: %d asks arrived in an order other than the %d launched", kind, len(arrived), len(launched))
		}
	}
}

// TestABuildAsksAGroupsFirstAlone: at N=4 each group's first ask returns
// before the group's second arrives.
func TestABuildAsksAGroupsFirstAlone(t *testing.T) {
	w := newWire(func(string) time.Duration { return 10 * time.Millisecond })
	f := builtAt(t, w, 4)
	groups := 0
	for _, kind := range asks.Kinds {
		for _, g := range groupRecords(t, f, kind) {
			if len(g.Items) < 2 {
				continue
			}
			groups++
			first, second := itemAsk(t, g, 0), itemAsk(t, g, 1)
			// The first item's last return: a re-ask returns twice.
			returned := -1
			for i, e := range w.events {
				if e == (wireEvent{ask: first, returned: true}) {
					returned = i
				}
			}
			arrived := slices.Index(w.events, wireEvent{ask: second})
			if returned < 0 || arrived < 0 || returned > arrived {
				t.Errorf("%s group %s: its second ask arrived before its first returned", kind, g.Group)
			}
		}
	}
	if groups == 0 {
		t.Fatal("no group asked two items")
	}
}
