package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/query"
)

// backEndScript is a command over the host's shell whose behaviour is one
// script: the script is written to a file and read by sh, so a body holding a
// space is still one element of the override's whitespace-split value.
func backEndScript(t *testing.T, script string, args ...string) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ask.sh")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	return append([]string{"sh", path}, args...)
}

// backEndRun is a score run on the harness back end whose command answers each
// item with script: the back end override names it, so no back end of kbase's is
// composed or probed. The configuration is file mode with the scoring role
// named, which is what every harness run resolves its model from.
func backEndRun(t *testing.T, script string, args ...string) scoreOptions {
	t.Helper()
	t.Setenv(backEndCommandEnv, strings.Join(backEndScript(t, script, args...), " "))
	opts := scoreOptions{Pass: query.PassRigor, Harness: opencodeHarness}
	opts.providerOptions = scoreProvider()
	return opts
}

// TestScoreHarnessAsksOneProcessPerItem: a harness run asks one item per
// process in the run's own working directory, empties that directory before
// each item, and lands the value the command's stdout carries.
func TestScoreHarnessAsksOneProcessPerItem(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	// A command that fails where the previous item's file is still visible: the
	// run only completes four items if each one's directory was emptied.
	opts := backEndRun(t, `if [ -e seen ]; then printf 'the previous item is visible\n' >&2; exit 4; fi
touch seen; printf 'none\n'`)
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the harness run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if got.dialled != 0 {
		t.Errorf("a harness run dialled a provider %d time(s)", got.dialled)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["total"] != 4 || counts["done"] != 4 || counts["failed"] != 0 {
		t.Errorf("counts %v, want the run's four items done by their own processes\n%s", counts, got.stdout)
	}
	dir, _ := got.doc.Values["ledger"].(string)
	work := filepath.Join(dir, harnessDirName)
	if _, err := os.Stat(work); err != nil {
		t.Errorf("the run left no working directory for its commands: %v", err)
	}
	// The command's own file survives the last item alone: it is the run's one
	// directory, emptied per item rather than grown per item.
	if _, err := os.Stat(filepath.Join(work, "seen")); err != nil {
		t.Errorf("the last item left no artifact: %v", err)
	}
}

// TestScoreHarnessRecordsWhatAnswered: the record names the back end and the
// scoring role's model, and the run's header names the argv that invoked each
// item and the variable that supplied it — the record a run makes of a command
// nobody chose through kbase's own interface.
func TestScoreHarnessRecordsWhatAnswered(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	opts := backEndRun(t, "printf 'none\\n'")
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the harness run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	role := roleOf(got.doc, "scoring")
	if role == nil || role["back-end"] != "harness" || role["model"] != "solo/scoring-model" {
		t.Fatalf("roles = %v, want the harness back end and the role's model\n%s", got.doc.Values["roles"], got.stdout)
	}
	for _, key := range []string{"provider", "provider-rule", "provider-layer", "harness", "harness-layer"} {
		if _, ok := role[key]; ok {
			t.Errorf("a harness role names %s: %v", key, role)
		}
	}

	dir, _ := got.doc.Values["ledger"].(string)
	header, err := os.ReadFile(filepath.Join(dir, "run.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"backend: harness", "model: solo/scoring-model", "- sh", "command_override: " + backEndCommandEnv,
	} {
		if !strings.Contains(string(header), want) {
			t.Errorf("run.yaml does not carry %q:\n%s", want, header)
		}
	}
	for _, unwanted := range []string{"harness:", "harness_layer:"} {
		if strings.Contains(string(header), unwanted) {
			t.Errorf("run.yaml carries %q, which names a profile:\n%s", unwanted, header)
		}
	}
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"backend":"harness"`, `"model":"solo/scoring-model"`} {
		if !strings.Contains(string(ledger), want) {
			t.Errorf("the ledger's lines do not carry %s:\n%s", want, ledger)
		}
	}
}

// TestScoreHarnessOverrideIsAnnouncedOnce: a run whose command the override
// named says so on stderr once, naming the variable and the argv, so a stray
// inherited variable can never score a KB with a command nobody chose and
// nothing said.
func TestScoreHarnessOverrideIsAnnouncedOnce(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	opts := backEndRun(t, "printf 'none\\n'")
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the harness run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	argv := strings.Fields(os.Getenv(backEndCommandEnv))
	quoted := `["` + strings.Join(argv, `" "`) + `"]`
	want := backEndCommandEnv + " names the command " + quoted +
		" — kbase runs it in place of its own back end's command\n"
	if got.stderr != want {
		t.Errorf("stderr = %q, want the one line naming the variable and the argv:\n%s", got.stderr, want)
	}
}

// TestScoreHarnessOverrideIsNotInherited: the command's own process does not
// carry the variable that chose it, so nothing a run executes — kbase itself
// among them — inherits the substitution.
func TestScoreHarnessOverrideIsNotInherited(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	script := `if [ -n "${` + backEndCommandEnv + `:-}" ]; then printf 'inherited\n' >&2; exit 9; fi
printf 'none\n'`
	got := scoreIn(t, repo, state, backEndRun(t, script), nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the run whose child inherited the variable: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
}

// TestScoreHarnessRefusals: a --harness value kbase ships no back end for is
// refused before the KB is opened, naming the one it takes, and so is a command
// the override named and nothing can start.
func TestScoreHarnessRefusals(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	opts := scoreOptions{Pass: query.PassRigor, Harness: "nosuch"}
	opts.providerOptions = scoreProvider()
	got := scoreIn(t, repo, state, opts, nil)
	items := got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != config.CheckHarness ||
		items[0]["key"] != "--harness" || !strings.Contains(items[0]["detail"].(string), "no harness profile") {
		t.Fatalf("--harness nosuch = %s, %v; want a harness refusal naming the back ends kbase runs\n%s",
			got.doc.Outcome, items, got.stdout)
	}
	if allowed := anyStrings(items[0]["allowed"]); len(allowed) != 1 || allowed[0] != opencodeHarness {
		t.Errorf("allowed = %v, want the one back end kbase ships", items[0]["allowed"])
	}
	if _, opened := got.doc.Values["run"]; opened {
		t.Errorf("the refusal opened a run:\n%s", got.stdout)
	}
	if _, err := os.Stat(filepath.Join(state, "score")); !os.IsNotExist(err) {
		t.Errorf("the refusal left a score directory: %v", err)
	}

	t.Setenv(backEndCommandEnv, "kbase-no-such-back-end-program --ask")
	got = scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Harness: opencodeHarness,
		providerOptions: scoreProvider()}, nil)
	items = got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != config.CheckHarness ||
		items[0]["key"] != "--harness" || !strings.Contains(items[0]["detail"].(string), "cannot be started") {
		t.Errorf("a command that cannot start = %s, %v; want a harness refusal naming --harness\n%s",
			got.doc.Outcome, items, got.stdout)
	}
}

// TestScoreHarnessFailureIsTheItem: a command exiting non-zero fails its item
// and the run goes on — the next item is asked, the line carries the exit code
// and the stderr, and the run closes on failures.
func TestScoreHarnessFailureIsTheItem(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	opts := backEndRun(t, "printf 'the stub refuses\\n' >&2; exit 3")
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "failed" {
		t.Fatalf("the failing run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["failed"] != 1 || counts["done"] != 0 {
		t.Errorf("counts %v, want the item failed and counted\n%s", counts, got.stdout)
	}
	items := got.doc.items("failures")
	if len(items) != 1 || items[0]["check"] != checkError {
		t.Fatalf("failures = %v, want one item\n%s", items, got.stdout)
	}
	detail, _ := items[0]["detail"].(string)
	for _, want := range []string{"exit status 3", "the stub refuses"} {
		if !strings.Contains(detail, want) {
			t.Errorf("the failure's detail = %q, want it to carry %q", detail, want)
		}
	}
	dir, _ := got.doc.Values["ledger"].(string)
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), `"status":"failed"`) || !strings.Contains(string(ledger), "the stub refuses") {
		t.Errorf("the line does not record the failure with its stderr:\n%s", ledger)
	}
}

// TestScoreHarnessFailureReasonIsASummary: a back end's stderr is arbitrary
// bytes, so what reaches the document's detail — and the log line beside it — is
// bounded and stripped of control characters, while the ledger's line keeps it
// whole. A terminal obeying a child's escape and a document carrying 64 KiB of
// it are both what the one channel prevents.
func TestScoreHarnessFailureReasonIsASummary(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	opts := backEndRun(t, `printf 'the stub refuses\n\033[2J\033]0;owned\007' >&2; printf 'x%.0s' $(seq 1 2000) >&2; exit 3`)
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "failed" {
		t.Fatalf("the failing run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items := got.doc.items("failures")
	if len(items) != 1 {
		t.Fatalf("failures = %v, want one item\n%s", items, got.stdout)
	}
	detail, _ := items[0]["detail"].(string)
	for _, banned := range []string{"\x1b", "\n", "\r", "\x07"} {
		if strings.Contains(detail, banned) {
			t.Errorf("the failure's detail carries %q: %q", banned, detail)
		}
	}
	if !strings.Contains(detail, "the stub refuses") || !strings.Contains(detail, "exit status 3") {
		t.Errorf("the failure's detail = %q, want the exit code and the stderr's own words", detail)
	}
	if len(detail) > 600 {
		t.Errorf("the failure's detail is %d bytes, want a bounded summary: %q", len(detail), detail)
	}
	// The one place a reply lands whole is the ledger's line.
	dir, _ := got.doc.Values["ledger"].(string)
	raw, _ := lineRecord(t, filepath.Join(dir, "ledger.jsonl"))["reply"].(string)
	if !strings.Contains(raw, "\x1b[2J") || !strings.Contains(raw, strings.Repeat("x", 2000)) {
		t.Errorf("the ledger's reply = %q, want the stderr whole", raw)
	}
}

// TestScoreHarnessIgnoresTheMode: the mode is a fact about the provider back
// end, so a base URL and role variables set beside --harness are neither a
// mixed mode nor a refusal.
func TestScoreHarnessIgnoresTheMode(t *testing.T) {
	unsetInferenceEnv(t)
	t.Setenv(config.EnvAPIBaseURL, "http://env.example/v1")
	t.Setenv(config.EnvDecision, "env-decision")
	t.Setenv(config.EnvText, "env-text")
	t.Setenv(config.EnvScoring, "env-scoring")
	repo := scoreFixture(t)
	// The files name a scoring reference as well, which is what the mixed-mode
	// refusal would otherwise be about: the environment's id is the one a
	// harness run is handed, so the model the record names says which of the two
	// sources was read.
	opts := backEndRun(t, "printf 'none\\n'")
	opts.ScoringModel = ""
	got := scoreIn(t, repo, t.TempDir(), opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the harness run with the environment set: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if role := roleOf(got.doc, "scoring"); role == nil || role["model"] != "env-scoring" {
		t.Errorf("roles = %v, want the environment's id under the harness\n%s", got.doc.Values["roles"], got.stdout)
	}
	// The announcement is the back end override's, and nothing else is said: no
	// mode refusal and no superseded-provider line.
	if lines := strings.Count(got.stderr, "\n"); lines != 1 {
		t.Errorf("stderr = %q, want the override's one line and nothing else", got.stderr)
	}
}

// TestScoreDryRunAsksNothing: a dry run asks nothing, so --harness changes
// nothing — no back end is set up, no command probed, no line written, and the
// document carries no roles record.
func TestScoreDryRunAsksNothing(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	opts := scoreOptions{Pass: query.PassRigor, DryRun: true, Harness: opencodeHarness}
	opts.providerOptions = scoreWithoutScoring()
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the dry run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	for _, key := range []string{"roles", "resume"} {
		if v, ok := got.doc.Values[key]; ok {
			t.Errorf("a dry run carries %s: %v\n%s", key, v, got.stdout)
		}
	}
	dir, _ := got.doc.Values["ledger"].(string)
	if _, err := os.Stat(filepath.Join(dir, harnessDirName)); err == nil {
		t.Error("a dry run left a working directory for a command it never ran")
	}
}

// TestScoreResumeCarriesTheBackEnd: the command that continues a run carries the
// back end flag it carried, and an environment-mode run carries the mode's
// settings as assignments before the program.
func TestScoreResumeCarriesTheBackEnd(t *testing.T) {
	opts := scoreOptions{Pass: query.PassRigor, ScoringModel: "org/judge", Harness: opencodeHarness}
	got := scoreResume(opts, "run-1", "/state", "", config.Overrides{})
	want := "kbase score --pass rigor --resume run-1 --state-dir /state --scoring-model org/judge --harness opencode"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
	env := config.Overrides{BaseURL: "http://env.example/v1", Decision: "d", Text: "t", Scoring: "s", APIKeyFile: "/keys/k.txt"}
	got = scoreResume(scoreOptions{Pass: query.PassRigor}, "run-2", "/state", config.ModeEnvironment, env)
	want = "KBASE_API_BASE_URL=http://env.example/v1 KBASE_DECISION=d KBASE_TEXT=t KBASE_SCORING=s " +
		"KBASE_API_KEY_FILE=/keys/k.txt kbase score --pass rigor --resume run-2 --state-dir /state"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
	// A --config-dir the invocation was given is carried on: the resumed run
	// reads that directory and no other, so it resolves the reference the first
	// run did under the same run id.
	got = scoreResume(scoreOptions{Pass: query.PassRigor, ConfigDir: "/cfg dir"}, "run-3", "/state", config.ModeFile, config.Overrides{})
	want = "kbase score --pass rigor --resume run-3 --state-dir /state --config-dir '/cfg dir'"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
}
