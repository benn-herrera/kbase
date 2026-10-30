//go:build kbasedev

package call

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// askOverview asks the overview passage once through c, from excerpts and
// rejected at attempt.
func askOverview(t *testing.T, c *Caller, excerpts string, rejected []string, attempt int) {
	t.Helper()
	var wg sync.WaitGroup
	err := c.PassageLaunch(model.NewQueue(1), "m")(context.Background(), &wg, func(ask func(context.Context, string, []string, int) (string, error)) func(error) {
		_, _ = ask(context.Background(), excerpts, rejected, attempt)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

// TestOverviewCapturesRecordTheirInputsAndReadBack: an overview call's capture
// holds the inputs its prompt was rendered from, and ReadOverviewCaptures
// returns them in attempt order with the digest of the prompt they render.
func TestOverviewCapturesRecordTheirInputsAndReadBack(t *testing.T) {
	scratch := t.TempDir()
	mock := modeltest.NewScriptedMockPerConsult([]model.Response{
		{Content: "reply one", FinishReason: "stop"},
		{Content: "reply two", FinishReason: "stop"},
	})
	c := newCaller(mock, scratch)
	askOverview(t, c, "E1", nil, 1)
	askOverview(t, c, "E", []string{"r1"}, 2)

	caps, err := ReadOverviewCaptures(trailDir(scratch))
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) != 2 || caps[0].Attempt != 1 || caps[1].Attempt != 2 {
		t.Fatalf("captures = %+v, want attempts 1, 2", caps)
	}
	for i, want := range []struct {
		excerpts string
		rejected []string
		reply    string
	}{{"E1", nil, "reply one"}, {"E", []string{"r1"}, "reply two"}} {
		got := caps[i]
		prompt, err := asks.OverviewPrompt(want.excerpts, want.rejected)
		if err != nil {
			t.Fatal(err)
		}
		if !got.HasInputs || got.Excerpts != want.excerpts || !slices.Equal(got.Rejected, want.rejected) {
			t.Errorf("capture %d inputs = %q %q (has %v), want %q %q", i, got.Excerpts, got.Rejected, got.HasInputs, want.excerpts, want.rejected)
		}
		if got.PromptSHA256 != asks.Digest(prompt) {
			t.Errorf("capture %d prompt-sha256 = %s, want the digest of the prompt its inputs render", i, got.PromptSHA256)
		}
		if got.Outcome != model.ChatOK || got.Reply != want.reply {
			t.Errorf("capture %d outcome, reply = %q, %q, want %q, %q", i, got.Outcome, got.Reply, model.ChatOK, want.reply)
		}
	}
	if _, ok := firstLine(t, trailCaptures(scratch), "overview-*-1.capture.jsonl")["rejected-lines"]; ok {
		t.Error("an attempt with no rejected lines recorded a rejected-lines key")
	}
	if _, ok := firstLine(t, trailCaptures(scratch), "overview-*-2.capture.jsonl")["excerpts"]; !ok {
		t.Error("the retry's capture has no excerpts key")
	}
}

// TestAnOverviewCaptureWithoutExcerptsHasNoInputs: a capture written without
// inputs reads back as having none, and a trail with no captures directory
// has no captures.
func TestAnOverviewCaptureWithoutExcerptsHasNoInputs(t *testing.T) {
	trail := t.TempDir()
	captures := filepath.Join(trail, asks.CapturesDir)
	if err := os.MkdirAll(captures, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"request","ask-attempt":1,"prompt-sha256":"h"}` + "\n" + `{"type":"attempt","outcome":"ok","reply":"r"}` + "\n"
	if err := os.WriteFile(filepath.Join(captures, overviewName+"-h-1.capture.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	caps, err := ReadOverviewCaptures(trail)
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) != 1 || caps[0].HasInputs {
		t.Errorf("captures = %+v, want one with no inputs", caps)
	}
	if none, err := ReadOverviewCaptures(t.TempDir()); err != nil || len(none) != 0 {
		t.Errorf("a trail with no captures directory read %v, %v, want none", none, err)
	}
}

// TestAFailedOverviewCallReadsBackAsNotOK: a call that never completes leaves
// a capture whose outcome is an attempt's failure.
func TestAFailedOverviewCallReadsBackAsNotOK(t *testing.T) {
	scratch := t.TempDir()
	mock := modeltest.NewScriptedMock([]model.Response{{Content: "", FinishReason: "length"}}, nil)
	askOverview(t, newCaller(mock, scratch), "E", nil, 1)
	caps, err := ReadOverviewCaptures(trailDir(scratch))
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) != 1 || caps[0].Outcome == "" || caps[0].Outcome == model.ChatOK {
		t.Errorf("captures = %+v, want one whose outcome is an attempt's failure", caps)
	}
}
