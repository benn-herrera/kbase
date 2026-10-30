package call

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/model"
)

// firstLine is the first line of the one capture file under dir matching glob,
// decoded as a JSON object.
func firstLine(t *testing.T, dir, glob string) map[string]json.RawMessage {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(dir, glob))
	if len(paths) != 1 {
		t.Fatalf("captures %s = %q, want one", glob, paths)
	}
	b, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var line map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.SplitN(string(b), "\n", 2)[0]), &line); err != nil {
		t.Fatal(err)
	}
	return line
}

// TestADecisionCaptureRecordsTheTrueLetter: the capture's question carries the
// true letter the question offered, which the answer-cache key and the reading
// of a noul answer depend on.
func TestADecisionCaptureRecordsTheTrueLetter(t *testing.T) {
	scratch := t.TempDir()
	served := &decisions{answers: []model.DecisionAnswer{noulAnswer("S1", 0.9)}}
	if _, err := askGroup(letters(newCaller(served, scratch)), paragraphItems("S1")); err != nil {
		t.Fatal(err)
	}
	want := paragraphItems("S1")[0].Question.TrueLetter
	if want == "" {
		t.Fatal("the paragraph question has no true letter to record")
	}
	var question struct {
		TrueLetter string `json:"true-letter"`
	}
	if err := json.Unmarshal(firstLine(t, trailCaptures(scratch), "*.capture.jsonl")["question"], &question); err != nil {
		t.Fatal(err)
	}
	if question.TrueLetter != want {
		t.Errorf("true-letter = %q, want %q", question.TrueLetter, want)
	}
}
