package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/build"
	"kbase/internal/config"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
)

// scoreToolHarness writes a harness stub under base and names it through the
// back end override: slow answers after a pause, fail refuses the item, and
// value answers at once. The stub is the executable a scoring run's harness back
// end runs, so a tool call's child is the shipped binary and the run is real.
func scoreToolHarness(t *testing.T, base, mode string) {
	t.Helper()
	script := filepath.Join(base, "score-stub.sh")
	body := `#!/usr/bin/env bash
set -euo pipefail
case "${1:-value}" in
slow) sleep 5; printf 'none\n' ;;
fail) sleep 1; printf 'the stub refuses\n' >&2; exit 3 ;;
*) printf 'none\n' ;;
esac
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KBASE_SCORE_BACK_END_COMMAND", script+" "+mode)
	t.Setenv(config.EnvScoring, "stub/model")
}

// scoreToolServer is the MCP tool table over repo, with the child's state and
// configuration isolated under base.
func scoreToolServer(t *testing.T, repo, base string) testServer {
	t.Helper()
	isolateChildBuild(t, base)
	tools, err := mcpTools(rootCmd, repo, &bytes.Buffer{}, log.Discard(), &buildLaunch{served: repo})
	if err != nil {
		t.Fatal(err)
	}
	return testServer{tools: tools}
}

// scoreToolStart calls the score tool and requires the start document: the run's
// id, ledger directory, pid and the wait command.
func scoreToolStart(t *testing.T, srv testServer, state string, args map[string]any) (run string, pid int, ledger string) {
	t.Helper()
	args = maps.Clone(args)
	args["state-dir"] = state
	start, isError := callTool(t, srv, "score", args)
	pidFloat, _ := start["pid"].(float64)
	run, _ = start["run"].(string)
	ledger, _ = start["ledger"].(string)
	if isError || start["outcome"] != toolresult.Done || pidFloat <= 0 || run == "" ||
		ledger != filepath.Join(state, "score", run) {
		t.Fatalf("score start: isError %v, %v", isError, start)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wantWait := build.ShellJoin([]string{exe, "wait", "--state-dir", state, "--pid", strconv.Itoa(int(pidFloat))})
	if wait, _ := start["wait"].(string); wait != wantWait {
		t.Errorf("score: wait = %q, want the exact command %q", wait, wantWait)
	}
	return run, int(pidFloat), ledger
}

// TestMCPScoreStartsDetached: a score tool call returns while the run is still
// walking, naming a pid that wait accepts; status reports the run in flight;
// wait then reports the run's counts and the item it landed.
func TestMCPScoreStartsDetached(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.EvalSymlinks(scoreRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	gitRepo(t, repo)
	state := filepath.Join(base, "state")
	scoreToolHarness(t, base, "slow")
	srv := scoreToolServer(t, repo, base)

	run, pid, ledger := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor", "harness": "opencode"})
	t.Cleanup(func() { callTool(t, srv, "cancel", map[string]any{"state-dir": state}) })

	if status, _ := callTool(t, srv, "status", map[string]any{"state-dir": state}); status["state"] != "running" || status["pid"] != float64(pid) {
		t.Errorf("status after the start: %v, want running under pid %d", status, pid)
	}
	out, code := kbase(t, repo, "wait", "--state-dir", state, "--pid", strconv.Itoa(pid))
	doc := parseResult(t, out)
	if doc.Outcome != toolresult.Done || code != 0 {
		t.Fatalf("wait: outcome %s, exit %d\n%s", doc.Outcome, code, out)
	}
	if doc.Values["run"] != run {
		t.Errorf("wait run = %v, want %s", doc.Values["run"], run)
	}
	counts, _ := doc.Values["counts"].(map[string]any)
	if counts["total"] != 1 || counts["done"] != 1 || counts["failed"] != 0 || counts["remaining"] != 0 {
		t.Errorf("wait counts %v, want the one item done\n%s", counts, out)
	}
	if scored := anyStrings(doc.Values["scored"]); !slices.Equal(scored, []string{"clm-aaaaaa"}) {
		t.Errorf("wait scored %q, want the claim the run landed", scored)
	}
	if _, err := os.Stat(filepath.Join(ledger, "ledger.jsonl")); err != nil {
		t.Errorf("the run left no ledger: %v", err)
	}
	if got := rigorOf(t, repo, "clm-aaaaaa"); got != 1.0 {
		t.Errorf("the claim's rigor = %v, want the reply's 1.0", got)
	}
}

// TestMCPScoreWaitReportsFailures: a finished run's wait document counts the
// failed item and carries its reason, as the run's own document does.
func TestMCPScoreWaitReportsFailures(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.EvalSymlinks(scoreRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	gitRepo(t, repo)
	state := filepath.Join(base, "state")
	scoreToolHarness(t, base, "fail")
	srv := scoreToolServer(t, repo, base)

	_, pid, _ := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor", "harness": "opencode"})
	t.Cleanup(func() { callTool(t, srv, "cancel", map[string]any{"state-dir": state}) })
	out, code := kbase(t, repo, "wait", "--state-dir", state, "--pid", strconv.Itoa(pid))
	doc := parseResult(t, out)
	if doc.Outcome != toolresult.Failed || code != exitCodes[toolresult.Failed] {
		t.Fatalf("wait: outcome %s, exit %d\n%s", doc.Outcome, code, out)
	}
	counts, _ := doc.Values["counts"].(map[string]any)
	if counts["total"] != 1 || counts["done"] != 0 || counts["failed"] != 1 || counts["remaining"] != 0 {
		t.Errorf("wait counts %v, want the one item failed\n%s", counts, out)
	}
	items := doc.items(toolresult.FailuresKey)
	if len(items) != 1 || items[0]["check"] != checkError ||
		!strings.HasPrefix(items[0]["detail"].(string), "clm-aaaaaa: ") ||
		!strings.Contains(items[0]["detail"].(string), "the stub refuses") {
		t.Errorf("failures %v, want the item's name and the reason its line recorded\n%s", items, out)
	}
}

// TestMCPScoreCancelLeavesResumableLedger: cancel stops the run in flight and
// leaves its ledger and resume command; re-issuing the resume with a harness
// that answers finishes the run.
func TestMCPScoreCancelLeavesResumableLedger(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.EvalSymlinks(scoreRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	gitRepo(t, repo)
	state := filepath.Join(base, "state")
	scoreToolHarness(t, base, "slow")
	srv := scoreToolServer(t, repo, base)

	run, pid, ledger := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor", "harness": "opencode"})
	cancel, isError := callTool(t, srv, "cancel", map[string]any{"state-dir": state})
	if isError || cancel["outcome"] != toolresult.Done || cancel["pid"] != float64(pid) {
		t.Fatalf("cancel: isError %v, %v", isError, cancel)
	}
	if resume, _ := cancel["resume"].(string); !strings.Contains(resume, "--resume "+run) {
		t.Errorf("cancel resume = %q, want the command that continues run %s", resume, run)
	}
	if _, err := os.Stat(filepath.Join(ledger, "run.yaml")); err != nil {
		t.Errorf("the cancelled run left no run.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ledger, score.ResultFile)); err != nil {
		t.Errorf("the cancelled run left no result: %v", err)
	}

	// The cancelled run's ledger continues: a harness that answers lands the
	// item the cancellation left unasked.
	scoreToolHarness(t, base, "value")
	out, code := kbase(t, repo, "score", "--pass", "rigor", "--resume", run, "--harness", "opencode", "--state-dir", state)
	doc := checkDocument(t, "score", out)
	if doc.Outcome != toolresult.Done || code != 0 {
		t.Fatalf("resume: outcome %s, exit %d\n%s", doc.Outcome, code, out)
	}
	counts, _ := doc.Values["counts"].(map[string]any)
	if counts["done"] != 1 {
		t.Errorf("resume counts %v, want the item done\n%s", counts, out)
	}
	if got := rigorOf(t, repo, "clm-aaaaaa"); got != 1.0 {
		t.Errorf("the claim's rigor = %v, want the resumed reply's 1.0", got)
	}
}
