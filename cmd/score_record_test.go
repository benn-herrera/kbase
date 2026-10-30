package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/query"
)

// TestScoreRecordNamesWhatAnswered: the record names the provider the scoring
// role named and the model it asked, in the roles record, in the run's header
// and in every ledger line, the header and lines naming the back end too — and
// no field an item's own command once carried stands anywhere.
func TestScoreRecordNamesWhatAnswered(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client {
		return scoring("none", "none", "none", "none")
	})
	if got.doc.Outcome != "done" {
		t.Fatalf("the run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	role := roleOf(got.doc, "scoring")
	if role == nil || role["provider"] != "solo" ||
		role["provider-rule"] != config.RuleConfig || role["model"] != "scoring-model" {
		t.Fatalf("roles = %v, want the pool entry the role named and the model it asked\n%s", got.doc.Values["roles"], got.stdout)
	}
	for _, key := range []string{"harness", "harness-layer", "back-end"} {
		if _, ok := role[key]; ok {
			t.Errorf("the role record names %s: %v", key, role)
		}
	}

	dir, _ := got.doc.Values["ledger"].(string)
	header, err := os.ReadFile(filepath.Join(dir, "run.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"backend: provider", "provider: solo", "model: scoring-model"} {
		if !strings.Contains(string(header), want) {
			t.Errorf("run.yaml does not carry %q:\n%s", want, header)
		}
	}
	for _, gone := range []string{"command:", "command_override:"} {
		if strings.Contains(string(header), gone) {
			t.Errorf("run.yaml carries %q, which named a command nothing asks now:\n%s", gone, header)
		}
	}
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"backend":"provider"`, `"model":"scoring-model"`} {
		if !strings.Contains(string(ledger), want) {
			t.Errorf("the ledger's lines do not carry %s:\n%s", want, ledger)
		}
	}
}

// TestScoreFailureReasonIsASummary: what reaches the document's detail — and the
// log line beside it — is bounded and stripped of control characters, while the
// ledger's line keeps the reason whole. A terminal obeying an escape and a
// document carrying 64 KiB of one are both what the one channel prevents.
func TestScoreFailureReasonIsASummary(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	// The stub's failure is the transport's, so it is re-issued; the pause is
	// injected short because what this test reads is the reason, not the wait.
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Backoff: []time.Duration{time.Millisecond}}, func() model.Client {
		m := modeltest.NewScriptedMock(nil, nil)
		m.SetError(errors.New("the stub refuses\n\x1b[2J\x1b]0;owned\a" + strings.Repeat("x", 2000)))
		return m.Client()
	})
	if got.doc.Outcome != "failed" {
		t.Fatalf("the failing run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items := got.doc.items("failures")
	if len(items) != 1 {
		t.Fatalf("failures = %v, want one item\n%s", items, got.stdout)
	}
	detail, _ := items[0]["detail"].(string)
	for _, banned := range []string{"\x1b", "\n", "\r", "\a"} {
		if strings.Contains(detail, banned) {
			t.Errorf("the failure's detail carries %q: %q", banned, detail)
		}
	}
	if !strings.Contains(detail, "the stub refuses") {
		t.Errorf("the failure's detail = %q, want the reason's own words", detail)
	}
	if len(detail) > 600 {
		t.Errorf("the failure's detail is %d bytes, want a bounded summary: %q", len(detail), detail)
	}
	// The one place a reason lands whole is the ledger's line.
	dir, _ := got.doc.Values["ledger"].(string)
	raw, _ := lineRecord(t, filepath.Join(dir, "ledger.jsonl"))["reply"].(string)
	if !strings.Contains(raw, "\x1b[2J") || !strings.Contains(raw, strings.Repeat("x", 2000)) {
		t.Errorf("the ledger's reply = %q, want the reason whole", raw)
	}
}

// TestScoreResumeCarriesTheInvocation: the command that continues a run carries
// the scoring model and the configuration directory the invocation carried.
func TestScoreResumeCarriesTheInvocation(t *testing.T) {
	opts := scoreOptions{Pass: query.PassRigor, ScoringModel: "org/judge"}
	got := scoreResume(opts, "run-1", "/state")
	want := "kbase score --pass rigor --resume run-1 --state-dir /state --scoring-model org/judge"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
	// A --config-dir the invocation was given is carried on: the resumed run
	// reads that directory and no other, so it resolves the reference the first
	// run did under the same run id.
	got = scoreResume(scoreOptions{Pass: query.PassRigor, ConfigDir: "/cfg dir"}, "run-3", "/state")
	want = "kbase score --pass rigor --resume run-3 --state-dir /state --config-dir '/cfg dir'"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
	// --project-dir is carried where the invocation named one and only then.
	got = scoreResume(scoreOptions{ProjectDir: "/the/project"}, "run-5", "/state")
	want = "kbase score --resume run-5 --state-dir /state --project-dir /the/project"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
	// A run that named no pass walks both, and its resume command names none
	// either: the header is what says which set the run walks.
	got = scoreResume(scoreOptions{}, "run-4", "/state")
	want = "kbase score --resume run-4 --state-dir /state"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
}
