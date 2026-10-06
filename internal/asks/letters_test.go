package asks

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// scripted answers each item's asks from its script, one reply per ask in
// order; an item whose script runs out never completes.
type scripted struct {
	mu      sync.Mutex
	replies map[string][]string
	asked   map[string][]string
}

func (s *scripted) read(_ context.Context, q LetterQuestion) (Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asked == nil {
		s.asked = map[string][]string{}
	}
	s.asked[q.Item] = append(s.asked[q.Item], q.Prompt)
	n := len(s.asked[q.Item])
	if n > len(s.replies[q.Item]) {
		return Reply{}, IncompleteError{q.Item, "no reply arrived"}
	}
	return Reply{Text: s.replies[q.Item][n-1]}, nil
}

func paragraphItems(names ...string) []LetterItem {
	items := make([]ParagraphItem, len(names))
	for i, n := range names {
		items[i] = ParagraphItem{Paragraph: n, Text: n + ": text"}
	}
	return ParagraphAsks("v/leaf.md", "the leaf", items)
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
	dir := t.TempDir()
	rec, err := AskGroup(context.Background(), Paragraph, "v/leaf.md", paragraphItems("S1", "S2-S3", "S4"), s.read, 0, dir)
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
		if it.Letter != nil {
			got = *it.Letter
		}
		if got != w.letter || it.Outcome != w.outcome || len(it.Calls) != w.calls {
			t.Errorf("%s = letter %q, %s, %d calls; want %q, %s, %d", it.Item, got, it.Outcome, len(it.Calls), w.letter, w.outcome, w.calls)
		}
	}
	reask := s.asked["S2-S3"][1]
	if !strings.HasPrefix(reask, s.asked["S2-S3"][0]) || !strings.Contains(reask, "I think it states a result.") {
		t.Error("the re-ask is not the first ask with the reply quoted after it")
	}
	if !strings.HasSuffix(rec.Prefix, "\n") || !strings.Contains(rec.Prefix, "the leaf") || strings.Contains(rec.Prefix, ": text") {
		t.Errorf("the group's prefix is not its shared context: %q", rec.Prefix)
	}
	if _, err := os.Stat(GroupRecordPath(dir, Paragraph, "v/leaf.md")); err != nil {
		t.Errorf("no group record: %v", err)
	}
	if tot := Totals([]GroupRecord{rec}); tot.Items != 3 || tot.Answered != 1 || tot.Reasked != 1 || tot.Defaulted != 1 || tot.Calls != 5 || tot.Unmeasured != 5 {
		t.Errorf("totals = %+v", tot)
	}
}

// TestAskGroupHoldsItsConcurrency: after the first ask, as many asks are in
// flight as the group's concurrency and never more.
func TestAskGroupHoldsItsConcurrency(t *testing.T) {
	for _, concurrency := range []int{1, 3} {
		var mu sync.Mutex
		inFlight, most := 0, 0
		read := func(_ context.Context, q LetterQuestion) (Reply, error) {
			mu.Lock()
			inFlight++
			most = max(most, inFlight)
			mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			inFlight--
			mu.Unlock()
			return Reply{Text: "A"}, nil
		}
		names := []string{"S1", "S2", "S3", "S4", "S5", "S6"}
		if _, err := AskGroup(context.Background(), Paragraph, "v/leaf.md", paragraphItems(names...), read, concurrency, ""); err != nil {
			t.Fatal(err)
		}
		if most != concurrency {
			t.Errorf("concurrency %d: at most %d asks in flight, want %d", concurrency, most, concurrency)
		}
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
			dir := t.TempDir()
			s := &scripted{replies: tc.replies}
			_, err := AskGroup(context.Background(), Paragraph, "v/leaf.md", paragraphItems("S1", "S2", "S3", "S4"), s.read, 0, dir)
			var incomplete IncompleteError
			if !errors.As(err, &incomplete) {
				t.Fatalf("AskGroup = %v, want the incomplete call", err)
			}
			if _, err := os.Stat(GroupRecordPath(dir, Paragraph, "v/leaf.md")); err == nil {
				t.Error("a group that stopped recorded itself")
			}
		})
	}
}
