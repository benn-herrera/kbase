package asks

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
		return Reply{}, IncompleteError{Subject: q.Item, Detail: "no reply arrived"}
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
			return Reply{}, IncompleteError{Subject: q.Item, Detail: "cancelled"}
		case "S3":
			inFlight.Wait()
			return Reply{}, IncompleteError{Subject: q.Item, Detail: "no reply arrived"}
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
			return Reply{}, IncompleteError{Subject: q.Item, Detail: "the second group never started while this ask was in flight"}
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
	var got []GroupRecord
	for dec := yaml.NewDecoder(f); ; {
		var r GroupRecord
		if err := dec.Decode(&r); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
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

// serialLaunch is a Launch over read holding one item in flight: an item
// launches once the one before it has returned, and not at all where the
// batch stopped meanwhile.
func serialLaunch(read MultichoiceReader) Launch {
	token := make(chan struct{}, 1)
	return func(ctx context.Context, wg *sync.WaitGroup, item func(MultichoiceReader) func(error)) error {
		select {
		case token <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		if err := ctx.Err(); err != nil {
			<-token
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			after := item(read)
			<-token
			if after != nil {
				after(nil)
			}
		}()
		return nil
	}
}

// heard is a reader double that keeps every question it is put and answers
// each by script, as scripted does, or with the first offered letter where
// it has no script.
type heard struct {
	scripted
	mu    sync.Mutex
	asked []MultichoiceQuestion
}

func (h *heard) read(ctx context.Context, q MultichoiceQuestion) (Reply, error) {
	h.mu.Lock()
	h.asked = append(h.asked, q)
	h.mu.Unlock()
	if h.replies == nil {
		return Reply{Text: q.Offered[0]}, nil
	}
	return h.scripted.read(ctx, q)
}

// stateOf is the state of the first question h was put for item.
func (h *heard) stateOf(t *testing.T, item string) string {
	t.Helper()
	for _, q := range h.asked {
		if q.Item == item {
			return q.State
		}
	}
	t.Fatalf("%s was never asked", item)
	return ""
}

// askedBatch is the one group of items, of kind, asked through a batch new
// makes over launch, and Wait's error.
func askedBatch(t *testing.T, newBatch func(context.Context, Launch) *Batch, launch Launch, kind Kind, items []MultichoiceItem) (GroupRecord, error) {
	t.Helper()
	b := newBatch(context.Background(), launch)
	_ = b.AskGroup(kind, "g", items) // a stop is Wait's to report
	recs, err := b.Wait()
	if len(recs) != 1 {
		t.Fatalf("Wait returned %d group records, %v; want the one group", len(recs), err)
	}
	return recs[0], err
}

// itemsOf is a group of asks of kind, of more than one item.
func itemsOf(t *testing.T, kind Kind) []MultichoiceItem {
	t.Helper()
	src := Claim{ID: "clm-aaaaaa", Title: "Theorem 1", Document: "v/a.md", Locator: "**Theorem 1**."}
	lemma := Claim{ID: "clm-bbbbbb", Title: "Lemma 2", Document: "v/b.md"}
	eq := Claim{ID: "clm-cccccc", Title: "(1)", Document: "v/b.md"}
	switch kind {
	case Paragraph:
		return paragraphItems("S1", "S2", "S3")
	case Unmarked:
		return UnmarkedAsks("v/a.md", "S1: We study the map.\n", src, "The map has a fixed point.", []UnmarkedItem{
			{Target: lemma, Statement: "The map is a contraction."}, {Target: eq, Statement: "$$ x = y $$"}})
	case Colinear:
		return ColinearAsks([]ColinearItem{
			{Key: lemma.ID, First: src, FirstText: "The map has a fixed point.", Second: lemma, SecondText: "So it is unique."},
			{Key: eq.ID, First: lemma, FirstText: "The map is a contraction.", Second: eq, SecondText: "x = y"}})
	case Classify:
		return ClassifyAsks(src, "The map has a fixed point.", []ClassifyItem{
			{Target: lemma, Statement: "lemma", Passages: []string{"zeta", "alpha"}, Offered: []string{LetterSupportedBy, LetterInSupportOf, LetterMention}},
			{Target: eq, Statement: "x = y", Passages: []string{"zeta"}, Offered: []string{LetterSupportedBy, LetterMention}}})
	}
	t.Fatalf("no items of kind %s", kind)
	return nil
}

// TestAGroupRecordsSharedInputsOnce: a leaf's paragraph asks record the
// leaf's inputs once, in the group's, and each paragraph's in its item's.
func TestAGroupRecordsSharedInputsOnce(t *testing.T) {
	h := &heard{}
	rec, err := askedBatch(t, NewBatch, readerLaunch(h.read), Paragraph, paragraphItems("S1", "S2", "S3"))
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(rec.Inputs); !slices.Equal(got, []string{"body", "document"}) {
		t.Errorf("the group's inputs are keyed %q, want body and document", got)
	}
	for _, it := range rec.Items {
		if got := keys(it.Inputs); !slices.Equal(got, []string{"paragraph", "paragraph-text"}) {
			t.Errorf("%s's inputs are keyed %q, want paragraph and paragraph-text", it.Item, got)
		}
	}
}

func TestAskDigestCoversTheQuestionAndTheState(t *testing.T) {
	q := Question{Key: "S1", Type: "noul", TrueLetter: LetterClaim, Instructions: "decide", Criteria: map[string]string{"true": "yes", "false": "no"}}
	base := AskDigest("state", q)
	instructions, trueLetter, key := q, q, q
	instructions.Instructions = "decide again"
	trueLetter.TrueLetter = LetterNotClaim
	key.Key = "S2"
	for _, tc := range []struct {
		name   string
		digest string
		differ bool
	}{
		{"the instructions", AskDigest("state", instructions), true},
		{"the true letter", AskDigest("state", trueLetter), true},
		{"the state", AskDigest("another state", q), true},
		{"the key", AskDigest("state", key), false},
	} {
		if (tc.digest != base) != tc.differ {
			t.Errorf("changing only %s: digest differs %t, want %t", tc.name, tc.digest != base, tc.differ)
		}
	}
}

// TestAStoppedBatchRecordsWhatItAsked: one item at a time, the second's call
// never completing. A build's batch stops on it and records the first
// answered, the second failed with its inputs and digest and no call, and
// the third, never launched, not at all.
func TestAStoppedBatchRecordsWhatItAsked(t *testing.T) {
	h := &heard{scripted: scripted{replies: map[string][]string{"S1": {LetterClaim}, "S3": {LetterClaim}}}}
	rec, err := askedBatch(t, NewBatch, serialLaunch(h.read), Paragraph, paragraphItems("S1", "S2", "S3"))
	var incomplete IncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("Wait = %v; want a stop", err)
	}
	var got []Outcome
	for _, it := range rec.Items {
		got = append(got, it.Outcome)
	}
	if want := []Outcome{Answered, Failed}; !slices.Equal(got, want) {
		t.Fatalf("recorded %q, want %q", got, want)
	}
	failed := rec.Items[1]
	if failed.Choice != nil || failed.Confidence != nil || len(failed.Calls) != 0 || len(failed.Inputs) == 0 ||
		failed.PromptSHA256 != AskDigest(h.stateOf(t, failed.Item), paragraphItems(failed.Item)[0].Question) {
		t.Errorf("the failed item = %+v; want no choice, no confidence, no call, its inputs and its prompt-sha256", failed)
	}
	if tot := Totals([]GroupRecord{rec}); tot.Items != 2 || tot.Answered != 1 || tot.Failed != 1 {
		t.Errorf("Totals = items %d, answered %d, failed %d; want 2, 1, 1", tot.Items, tot.Answered, tot.Failed)
	}
}

// TestAFailedReAskKeepsItsCompletedCall: an item whose first ask decided no
// letter and whose re-ask never completed is failed, with the one call that
// completed.
func TestAFailedReAskKeepsItsCompletedCall(t *testing.T) {
	h := &heard{scripted: scripted{replies: map[string][]string{"S1": {"?"}}}}
	rec, err := askedBatch(t, NewBatch, serialLaunch(h.read), Paragraph, paragraphItems("S1"))
	var incomplete IncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("Wait = %v, want the re-ask that never completed", err)
	}
	if it := rec.Items[0]; it.Outcome != Failed || len(it.Calls) != 1 || it.Choice != nil {
		t.Errorf("the item = %s with %d calls, choice %v; want failed with one call and no choice", it.Outcome, len(it.Calls), it.Choice)
	}
}

func TestAFinishedBatchRecordsEveryGroup(t *testing.T) {
	read := (&scripted{replies: map[string][]string{"P1": {"A"}, "P3": {"A"}}}).read
	b := NewBatch(context.Background(), readerLaunch(read))
	for _, g := range []struct {
		leaf  string
		items []MultichoiceItem
	}{{"g1", paragraphItems("P1")}, {"g2", nil}, {"g3", paragraphItems("P3")}} {
		if err := b.AskGroup(Paragraph, g.leaf, g.items); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := b.Wait()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range recs {
		got = append(got, r.Group)
	}
	if !slices.Equal(got, []string{"g1", "g2", "g3"}) {
		t.Fatalf("records for groups %v, want [g1 g2 g3]", got)
	}
	if len(recs[1].Items) != 0 {
		t.Fatalf("g2 holds %d items, want none", len(recs[1].Items))
	}
}
