package call

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kbase/internal/asks"
	"kbase/internal/log"
	"kbase/internal/model"
)

func paragraphItems(names ...string) []asks.LetterItem {
	items := make([]asks.ParagraphItem, len(names))
	for i, n := range names {
		items[i] = asks.ParagraphItem{Paragraph: n, Text: n + ": text"}
	}
	return asks.ParagraphAsks("v/leaf.md", "the leaf", items)
}

func TestCallerCapturesCachesAndAsksNothingTwice(t *testing.T) {
	scratch := t.TempDir()
	served := model.Usage{PromptTokens: 90, CompletionTokens: 1}
	mock := model.NewScriptedMockPerConsult([]model.Response{
		{Content: "A", Usage: served, UsageReported: true},
		{Content: "out of memory", UsageReported: true},
		{Content: "B", Usage: served, UsageReported: true},
	})
	mock.RecordCalls = true
	read := New(mock, scratch, log.Discard()).Letters("m-light")
	items := paragraphItems("S1", "S2")
	ask := func(read asks.LetterReader) asks.GroupRecord {
		t.Helper()
		rec, err := asks.AskGroup(context.Background(), asks.Paragraph, "v/leaf.md", items, read, 0, filepath.Join(scratch, asks.AskRecordsDir))
		if err != nil {
			t.Fatal(err)
		}
		return rec
	}
	first := ask(read)
	if *first.Items[0].Letter != "A" || *first.Items[1].Letter != "B" {
		t.Fatalf("letters = %v, %v", *first.Items[0].Letter, *first.Items[1].Letter)
	}
	calls := mock.Calls()
	if len(calls) != 3 {
		t.Fatalf("%d requests, want 3 (the second item re-issued once after a reply that served nothing)", len(calls))
	}
	req := calls[0].Request
	system, _ := asks.System(asks.ReaderSystem)
	if req.Model != "m-light" || len(req.Messages) != 2 || req.Messages[0].Content != system || req.Messages[0].Role != "system" {
		t.Errorf("request = %+v, want the reader-system fragment as the system message", req)
	}
	captures, _ := filepath.Glob(filepath.Join(scratch, CapturesDir, "paragraph-v_leaf.md-S*.capture.jsonl"))
	if len(captures) != 2 {
		t.Fatalf("captures = %q", captures)
	}
	b, _ := os.ReadFile(captures[1])
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); len(lines) != 3 || !strings.Contains(lines[0], `"type":"request"`) ||
		!strings.Contains(lines[1], `"outcome":"served-nothing"`) || !strings.Contains(lines[2], `"outcome":"ok"`) {
		t.Errorf("capture = %s", b)
	}
	answers, _ := filepath.Glob(filepath.Join(scratch, AnswersDir, "*.txt"))
	if len(answers) != 2 {
		t.Errorf("cached answers = %q, want one per completed call", answers)
	}

	resumed := model.NewScriptedMockPerConsult(nil)
	resumed.RecordCalls = true
	again := ask(New(resumed, scratch, log.Discard()).Letters("m-light"))
	if n := len(resumed.Calls()); n != 0 {
		t.Errorf("a resume over the cache made %d requests", n)
	}
	if *again.Items[0].Letter != "A" || *again.Items[1].Letter != "B" {
		t.Error("the cached answers decided differently")
	}
	other := model.NewScriptedMockPerConsult([]model.Response{{Content: "B"}, {Content: "B"}})
	other.RecordCalls = true
	ask(New(other, scratch, log.Discard()).Letters("m-other"))
	if n := len(other.Calls()); n != 2 {
		t.Errorf("a different model made %d requests, want 2: one model's answers are not another's", n)
	}
}

// TestAReaskOfTheSamePromptReachesTheModel: the overview passage re-asked
// after an empty reply composes the identical prompt, and its second ask is
// put to the model rather than answered from the first's cache entry.
func TestAReaskOfTheSamePromptReachesTheModel(t *testing.T) {
	scratch := t.TempDir()
	mock := model.NewScriptedMockPerConsult([]model.Response{{Content: "   "}, {Content: "A passage."}})
	mock.RecordCalls = true
	c := New(mock, scratch, log.Discard())
	first, err := c.Passage(context.Background(), "m-heavy", "==> Knowledge Base <==", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Passage(context.Background(), "m-heavy", "==> Knowledge Base <==", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if first != "   " || second != "A passage." || len(mock.Calls()) != 2 {
		t.Errorf("replies %q, %q over %d calls; want the re-ask answered by the model", first, second, len(mock.Calls()))
	}
	again, err := New(model.NewScriptedMockPerConsult(nil), scratch, log.Discard()).Passage(context.Background(), "m-heavy", "==> Knowledge Base <==", nil, 2)
	if err != nil || again != "A passage." {
		t.Errorf("the second ask resumed = %q, %v; want its own cached answer", again, err)
	}
}

func TestCallerNamesTheCallThatNeverCompleted(t *testing.T) {
	scratch := t.TempDir()
	mock := model.NewScriptedMockPerConsult(nil)
	mock.SetError(errors.New("connection refused"))
	mock.RecordCalls = true
	c := New(mock, scratch, log.Discard())
	if !slices.Equal(c.overviewBackoff, []time.Duration{5 * time.Second, 30 * time.Second}) {
		t.Errorf("overview backoff = %v, want kb_tools' driver's 5s then 30s", c.overviewBackoff)
	}
	c.overviewBackoff = []time.Duration{time.Millisecond}
	_, err := c.Passage(context.Background(), "m-heavy", "==> Knowledge Base <==", nil, 1)
	if n := len(mock.Calls()); n != model.ChatAttempts {
		t.Errorf("%d requests, want the call re-issued to %d attempts", n, model.ChatAttempts)
	}
	var incomplete asks.IncompleteError
	if !errors.As(err, &incomplete) || !strings.Contains(err.Error(), "connection refused") || !strings.Contains(err.Error(), CapturesDir) {
		t.Fatalf("Passage = %v, want the incomplete call naming its cause and capture", err)
	}
	if answers, _ := filepath.Glob(filepath.Join(scratch, AnswersDir, "*")); len(answers) != 0 {
		t.Errorf("an incomplete call cached %q", answers)
	}
}
