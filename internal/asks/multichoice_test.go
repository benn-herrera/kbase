package asks

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

// scripted answers each item's asks from its script, one reply per ask in
// order; an item whose script runs out never completes.
type scripted struct {
	mu      sync.Mutex
	replies map[string][]string
	asked   map[string][]string
}

func (s *scripted) read(_ context.Context, q MultichoiceQuestion) (Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asked == nil {
		s.asked = map[string][]string{}
	}
	s.asked[q.Item] = append(s.asked[q.Item], q.State)
	n := len(s.asked[q.Item])
	if n > len(s.replies[q.Item]) {
		return Reply{}, IncompleteError{q.Item, "no reply arrived"}
	}
	return Reply{Text: s.replies[q.Item][n-1]}, nil
}

func paragraphItems(names ...string) []MultichoiceItem {
	items := make([]ParagraphItem, len(names))
	for i, n := range names {
		items[i] = ParagraphItem{Paragraph: n, Text: n + ": text"}
	}
	return ParagraphAsks("v/leaf.md", "the leaf", items)
}

// readerLaunch is a Launch that starts every item at once on a goroutine of
// its own, its calls going through read, with no file to write.
func readerLaunch(read MultichoiceReader) Launch {
	return func(ctx context.Context, wg *sync.WaitGroup, item func(MultichoiceReader) func(error)) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if after := item(read); after != nil {
				after(nil)
			}
		}()
		return nil
	}
}

// askGroup asks items as the one group of a batch launched over read, and
// returns its record.
func askGroup(read MultichoiceReader, group string, items []MultichoiceItem) (GroupRecord, error) {
	b := NewBatch(context.Background(), readerLaunch(read))
	if err := b.AskGroup(Paragraph, group, items); err != nil {
		_, err = b.Wait()
		return GroupRecord{}, err
	}
	recs, err := b.Wait()
	if err != nil {
		return GroupRecord{}, err
	}
	return recs[0], nil
}

func TestDecide(t *testing.T) {
	ab := []string{"A", "B"}
	for _, tc := range []struct {
		reply  Reply
		letter string
		ok     bool
	}{
		{Reply{Text: " A\n"}, "A", true},
		{Reply{Text: "C"}, "", false},
		{Reply{Text: "A."}, "", false},
		{Reply{Text: "The answer is B"}, "", false},
		{Reply{Text: ""}, "", false},
		{Reply{Confidence: map[string]float64{"A": 0.4, "B": 0.4, "C": 0.9}}, "A", true},
		{Reply{Confidence: map[string]float64{"C": 0.9}}, "", false},
	} {
		if l, ok := Decide(tc.reply, ab); l != tc.letter || ok != tc.ok {
			t.Errorf("Decide(%+v) = %q, %t; want %q, %t", tc.reply, l, ok, tc.letter, tc.ok)
		}
	}
}

func TestAskGroupDecisionRules(t *testing.T) {
	s := &scripted{replies: map[string][]string{
		"S1":    {"A"},
		"S2-S3": {"I think it states a result.", "B"},
		"S4":    {"maybe", "neither"},
	}}
	rec, err := askGroup(s.read, "v/leaf.md", paragraphItems("S1", "S2-S3", "S4"))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		letter  string
		outcome Outcome
		calls   int
	}{{"A", Answered, 1}, {"B", Reasked, 2}, {"", Defaulted, 2}}
	for i, w := range want {
		it := rec.Items[i]
		got := ""
		if it.Choice != nil {
			got = *it.Choice
		}
		if got != w.letter || it.Outcome != w.outcome || len(it.Calls) != w.calls {
			t.Errorf("%s = letter %q, %s, %d calls; want %q, %s, %d", it.Item, got, it.Outcome, len(it.Calls), w.letter, w.outcome, w.calls)
		}
	}
	reask := s.asked["S2-S3"][1]
	if reask != s.asked["S2-S3"][0] {
		t.Errorf("the re-ask is not the identical question: %q then %q", s.asked["S2-S3"][0], reask)
	}
	if !strings.HasSuffix(rec.Prefix, "\n") || !strings.Contains(rec.Prefix, "the leaf") || strings.Contains(rec.Prefix, ": text") {
		t.Errorf("the group's prefix is not its shared context: %q", rec.Prefix)
	}
	if tot := Totals([]GroupRecord{rec}); tot.Items != 3 || tot.Answered != 1 || tot.Reasked != 1 || tot.Defaulted != 1 || tot.Calls != 5 || tot.Unmeasured != 5 {
		t.Errorf("totals = %+v", tot)
	}
}

func TestAskGroupStopsOnACallThatNeverCompletes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		replies map[string][]string
	}{
		{"the first item", map[string][]string{}},
		{"a later item", map[string][]string{"S1": {"A"}, "S2": {"B"}, "S4": {"A"}}},
		{"a re-ask", map[string][]string{"S1": {"A"}, "S2": {"B"}, "S3": {"?"}, "S4": {"A"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &scripted{replies: tc.replies}
			_, err := askGroup(s.read, "v/leaf.md", paragraphItems("S1", "S2", "S3", "S4"))
			var incomplete IncompleteError
			if !errors.As(err, &incomplete) {
				t.Fatalf("AskGroup = %v, want the incomplete call", err)
			}
		})
	}
}

// TestABatchReportsTheFailingItemNotACancelledSibling: an item whose call
// fails while siblings are in flight stops the batch, and the batch reports
// that failure — not a sibling's error from the cancellation it caused, even
// where the sibling stands earlier in the group.
func TestABatchReportsTheFailingItemNotACancelledSibling(t *testing.T) {
	var inFlight sync.WaitGroup
	inFlight.Add(2)
	read := func(ctx context.Context, q MultichoiceQuestion) (Reply, error) {
		switch q.Item {
		case "S2", "S4":
			inFlight.Done()
			<-ctx.Done()
			return Reply{}, IncompleteError{q.Item, "cancelled"}
		case "S3":
			inFlight.Wait()
			return Reply{}, IncompleteError{q.Item, "no reply arrived"}
		}
		return Reply{Text: "A"}, nil
	}
	_, err := askGroup(read, "v/leaf.md", paragraphItems("S1", "S2", "S3", "S4"))
	var incomplete IncompleteError
	if !errors.As(err, &incomplete) || incomplete.Subject != "S3" {
		t.Fatalf("the batch = %v, want S3's call that never completed", err)
	}
}

// TestAOneItemGroupDoesNotHoldUpTheNextGroup: a group of one has no later
// item to reuse its prefix, so the next group launches while its ask is still
// in flight.
func TestAOneItemGroupDoesNotHoldUpTheNextGroup(t *testing.T) {
	secondStarted := make(chan struct{})
	read := func(_ context.Context, q MultichoiceQuestion) (Reply, error) {
		if q.Item == "S2" {
			close(secondStarted)
			return Reply{Text: "A"}, nil
		}
		select {
		case <-secondStarted:
			return Reply{Text: "A"}, nil
		case <-time.After(2 * time.Second):
			return Reply{}, IncompleteError{q.Item, "the second group never started while this ask was in flight"}
		}
	}
	b := NewBatch(context.Background(), readerLaunch(read))
	for _, g := range []struct{ leaf, item string }{{"v/a.md", "S1"}, {"v/b.md", "S2"}} {
		if err := b.AskGroup(Paragraph, g.leaf, paragraphItems(g.item)); err != nil {
			break
		}
	}
	if _, err := b.Wait(); err != nil {
		t.Fatal(err)
	}
}

// TestWriteGroupRecordsIsOneFile: a stage's group records land as one file,
// every group in the order given, and nothing lands where there is no record
// or no directory.
func TestWriteGroupRecordsIsOneFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), AskRecordsDir)
	s := &scripted{replies: map[string][]string{"S1": {"A"}, "S2": {"B"}}}
	b := NewBatch(context.Background(), readerLaunch(s.read))
	for _, g := range []struct{ leaf, item string }{{"v/b.md", "S1"}, {"v/a.md", "S2"}} {
		if err := b.AskGroup(Paragraph, g.leaf, paragraphItems(g.item)); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := b.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteGroupRecords(dir, recs); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || filepath.Join(dir, entries[0].Name()) != GroupRecordsPath(dir, Paragraph) {
		t.Fatalf("the records dir holds %v, %v; want the one file of the stage's kind", entries, err)
	}
	f, err := os.Open(GroupRecordsPath(dir, Paragraph))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	var got []GroupRecord
	for {
		var rec GroupRecord
		if err := dec.Decode(&rec); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		got = append(got, rec)
	}
	letter := func(r GroupRecord) string {
		if len(r.Items) != 1 || r.Items[0].Choice == nil {
			return ""
		}
		return *r.Items[0].Choice
	}
	if len(got) != 2 || got[0].Group != "v/b.md" || letter(got[0]) != "A" || got[1].Group != "v/a.md" || letter(got[1]) != "B" {
		t.Errorf("the file holds %+v, want both groups in the order given", got)
	}

	for _, tc := range []struct {
		name, dir string
		recs      []GroupRecord
	}{{"no records", filepath.Join(t.TempDir(), "none"), nil}, {"no directory", "", recs}} {
		if err := WriteGroupRecords(tc.dir, tc.recs); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
		if _, err := os.Stat(GroupRecordsPath(tc.dir, Paragraph)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: a records file stands (%v), want nothing written", tc.name, err)
		}
	}
}
