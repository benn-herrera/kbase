package main

import (
	"bytes"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"kbase/internal/build"
	"kbase/internal/config"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
)

// scoreToolProvider starts the inference endpoint a tool call's scoring run
// reaches and names it in the environment: slow holds each request for a moment
// so the run stays in flight, fail refuses it with a reason of its own, and
// value answers at once. The tool call's child is the shipped binary and the run
// is real, so what answers it is an endpoint rather than a stub process.
func scoreToolProvider(t *testing.T, mode string) {
	t.Helper()
	unsetInferenceEnv(t)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode {
		case "slow":
			select {
			case <-release:
			case <-time.After(2 * time.Second):
			}
		case "fail":
			// A moment's pause so the run's ledger and pid stand before the
			// item fails: the tool call reports the start, and the failure is
			// read from the run's own record.
			time.Sleep(time.Second)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"the stub refuses"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"none\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":1,\"total_tokens\":41}}\n\n" +
			"data: [DONE]\n\n"))
	}))
	t.Cleanup(srv.Close)
	// Registered after Close so it runs before it: a request the run left
	// hanging must wake before the server waits for its handlers.
	t.Cleanup(func() { close(release) })
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
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
	scoreToolProvider(t, "slow")
	srv := scoreToolServer(t, repo, base)

	run, pid, ledger := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor"})
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
	scoreToolProvider(t, "fail")
	srv := scoreToolServer(t, repo, base)

	_, pid, _ := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor"})
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

// TestBuildRefusedWhileAScoreRunHoldsTheLock: a scoring run holds the KB's
// operation lock while it walks, so a build over the same repository is
// refused at once, naming the score run and its pid.
func TestBuildRefusedWhileAScoreRunHoldsTheLock(t *testing.T) {
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
	scoreToolProvider(t, "slow")
	srv := scoreToolServer(t, repo, base)

	_, pid, _ := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor"})
	t.Cleanup(func() { callTool(t, srv, "cancel", map[string]any{"state-dir": state}) })

	out, code := kbase(t, repo, "build", "paper.tex", "--no-inference", "--state-dir", filepath.Join(base, "build-state"))
	doc := parseResult(t, out)
	items := doc.items(toolresult.RefusalsKey)
	want := fmt.Sprintf("a score run (pid %d) is running; its state-dir is %s", pid, state)
	if doc.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] || len(items) != 1 ||
		items[0]["check"] != checkLock || items[0]["path"] != state || items[0]["detail"] != want {
		t.Errorf("a build under a score run: outcome %s, exit %d, refusals %v; want one lock refusal %q\n%s",
			doc.Outcome, code, items, want, out)
	}
	if _, err := os.Stat(filepath.Join(base, "build-state", "progress.jsonl")); err == nil {
		t.Error("the refused build wrote a progress record")
	}
}

// TestStatusReportsAKilledScoreRun: a score run killed in flight leaves the
// store's holder record naming it, and status reports it interrupted under its
// own run id and counts rather than falling back to the build.
func TestStatusReportsAKilledScoreRun(t *testing.T) {
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
	scoreToolProvider(t, "slow")
	srv := scoreToolServer(t, repo, base)

	run, pid, ledger := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor"})
	// The set the run enumerated stands beside the ledger, written after the pid
	// the launch waits on; a killed run's status counts what it left.
	set := filepath.Join(ledger, "items.jsonl")
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(set); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the run wrote no items.jsonl")
		}
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Kill(); err != nil {
		t.Fatal(err)
	}
	// The record stands until a clean exit removes it, so a killed run's is
	// read: the pid is gone and the lock is no longer held.
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		live, h, err := build.HolderOf(state)
		if err != nil {
			t.Fatal(err)
		}
		if !live && h.PID == pid {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the killed run's record: live %v, %+v", live, h)
		}
	}
	status, isError := callTool(t, srv, "status", map[string]any{"state-dir": state})
	if isError || status["state"] != "interrupted" || status["run"] != run || status["pid"] != float64(pid) {
		t.Fatalf("status of a killed score run = %v, want interrupted, run %s, pid %d", status, run, pid)
	}
	if counts, _ := status["counts"].(map[string]any); counts["total"] != float64(1) || counts["done"] != float64(0) {
		t.Errorf("counts %v, want the run's one item unlanded", status["counts"])
	}
	if _, err := os.Stat(filepath.Join(ledger, "run.yaml")); err != nil {
		t.Errorf("the killed run left no run.yaml: %v", err)
	}
	// The record names a run that is gone, so nothing holds the store: cancel
	// finds no holder to signal.
	cancel, _ := callTool(t, srv, "cancel", map[string]any{"state-dir": state})
	items, _ := cancel[toolresult.RefusalsKey].([]any)
	if cancel["outcome"] != toolresult.Refused || len(items) != 1 {
		t.Fatalf("cancel of a killed score run = %v, want one refusal", cancel)
	}
	if item, _ := items[0].(map[string]any); item["check"] != checkLock || item["detail"] != "no run holds the state store" {
		t.Errorf("cancel's refusal = %v, want the lock check naming no holder", items[0])
	}
}

// TestMCPScoreCancelLeavesResumableLedger: cancel stops the run in flight and
// leaves its ledger and resume command; re-issuing the resume against an
// endpoint that answers finishes the run.
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
	scoreToolProvider(t, "slow")
	srv := scoreToolServer(t, repo, base)

	run, pid, ledger := scoreToolStart(t, srv, state, map[string]any{"pass": "rigor"})
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

	// The cancelled run's ledger continues: an endpoint that answers lands the
	// item the cancellation left unasked.
	scoreToolProvider(t, "value")
	out, code := kbase(t, repo, "score", "--pass", "rigor", "--resume", run, "--state-dir", state)
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
