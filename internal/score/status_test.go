package score

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
	"kbase/internal/statedir"
)

// heldRun opens a run in the store at state whose process is pid, records its
// two items and the line the first landed, and returns the closed ledger — the
// run a waiter finds a result document for, or finds abandoned. cmd's status
// tests stage the same run through the verb's package, which cannot reach this
// file's helpers.
func heldRun(t *testing.T, state, run string, pid int) *Ledger {
	t.Helper()
	const started = "2026-10-01T10:00:00Z"
	l, err := Create(state, RunHeader{Run: run, KBRoot: t.TempDir(), Pass: query.PassRigor, Started: started})
	if err != nil {
		t.Fatal(err)
	}
	items := []query.Item{node(kb.NodeKindClaim, "clm-aaaaaa"), node(kb.NodeKindClaim, "clm-bbbbbb")}
	if err := l.WriteItems(items); err != nil {
		t.Fatal(err)
	}
	landed := Entry{Status: StatusDone, Value: "0.65", Faults: []Charge{}, Started: started, Ended: started}
	if err := l.Append(items[0], landed); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := l.WritePID(pid); err != nil {
		t.Fatal(err)
	}
	return l
}

// holdStore takes the store at state for the run named, its holder record
// naming this process, over a worktree of its own, and returns the release. The
// returned function releases once however often it is called, so a test that
// must let a polling waiter unwind can release it itself and still register it
// for cleanup. cmd's status tests hold the store the same way, for the same
// reason heldRun is restated.
func holdStore(t *testing.T, state, run string) func() {
	t.Helper()
	repo := t.TempDir()
	for _, d := range []string{".git/objects", ".git/refs/heads", kb.KBDir} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	release, err := build.Hold(filepath.Join(repo, kb.KBDir), state, buildstore.KindScore, run, time.Now())
	if err != nil {
		t.Fatalf("hold the store for a score run: %v", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if err := release(); err != nil {
				t.Errorf("release the store: %v", err)
			}
		})
	}
}

// decoded is a result document's keys.
func decoded(t *testing.T, doc string) map[string]any {
	t.Helper()
	var d map[string]any
	if err := yaml.Unmarshal([]byte(doc), &d); err != nil {
		t.Fatalf("decoding the document: %v\n%s", err, doc)
	}
	return d
}

// TestStatusFollowsTheOperationLock: given no state-dir, status reads the
// store the operation lock names, else the default store; a state-dir given
// is read as given.
func TestStatusFollowsTheOperationLock(t *testing.T) {
	base := t.TempDir()
	storetest.GitEnv(t, base)
	repo := filepath.Join(base, "repo")
	storetest.MakeRepo(t, repo)
	kbRoot := filepath.Join(repo, kb.KBDir)
	defaultStore, err := statedir.Dir("", kbRoot)
	if err != nil {
		t.Fatal(err)
	}
	hold := func(t *testing.T) string {
		t.Helper()
		other := t.TempDir()
		release, err := build.Hold(kbRoot, other, buildstore.KindBuild, "", time.Now())
		if err != nil {
			t.Fatalf("hold a non-default store: %v", err)
		}
		t.Cleanup(func() {
			if err := release(); err != nil {
				t.Errorf("release the store: %v", err)
			}
		})
		return other
	}
	for _, tc := range []struct {
		name      string
		stage     func(t *testing.T) (want string)
		stateDir  bool
		wantState string
	}{
		{"follows", hold, false, buildstore.StateRunning},
		{"explicit wins", func(t *testing.T) string { hold(t); return defaultStore }, true, buildstore.StateNone},
		{"empty lock", func(t *testing.T) string {
			storetest.WriteFile(t, buildstore.OperationLockPath(kbRoot), "")
			return defaultStore
		}, false, buildstore.StateNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.stage(t)
			opts := buildstore.MonitorOptions{WorkDir: repo}
			if tc.stateDir {
				opts.StateDir = want
			}
			outcome, fields := Status(opts)
			if got, state := storetest.Field(fields, "state-dir"), storetest.Field(fields, "state"); got != want || state != tc.wantState {
				t.Errorf("status = %s, state-dir %v, state %v; want %s, %s\n%v", outcome, got, state, want, tc.wantState, fields)
			}
		})
	}
}

// refusedNote stands for the note cmd supplies with a refused key's remedy.
const refusedNote = "a note on how the key reaches the store"

// TestReadRunOverlaysTheResultFilesOnTheLedger: a run that was killed before its
// flush has result files and no lines for them. They count under the status
// they record: a result overrides the ledger's last status for its item, and an
// item outside the item list counts too.
func TestReadRunOverlaysTheResultFilesOnTheLedger(t *testing.T) {
	state := t.TempDir()
	l := heldRun(t, state, "run-unflushed", 0)
	landedAgain := node(kb.NodeKindClaim, "clm-aaaaaa")
	answered, outside := node(kb.NodeKindClaim, "clm-bbbbbb"), node(kb.NodeKindClaim, "clm-cccccc")
	at := "2026-10-01T10:00:00Z"
	for i, r := range []struct {
		it query.Item
		e  Entry
	}{
		{landedAgain, Entry{Status: StatusFailed, Reply: "the provider closed the connection", Started: at, Ended: at}},
		{answered, Entry{Status: StatusDone, Value: "0.40", Faults: []Charge{}, Started: at, Ended: at}},
		{outside, Entry{Status: StatusFailed, Reply: "http 401: " + strings.Repeat("a long provider body ", 40), Remedy: "the command that lets the call through", Note: refusedNote, Started: at, Ended: at}},
	} {
		if err := l.writeResult(i, resultRecord{Line: l.line(r.it, r.e)}); err != nil {
			t.Fatal(err)
		}
	}

	st, err := ReadRun(state, "run-unflushed")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Counts{Total: 3, Done: 1, Failed: 2}); st.Counts != want {
		t.Errorf("counts %+v, want %+v", st.Counts, want)
	}
	if want := []string{"clm-bbbbbb"}; !slices.Equal(st.Scored, want) {
		t.Errorf("scored %q, want %q", st.Scored, want)
	}
	if len(st.Failures) != 2 {
		t.Errorf("failures %+v, want the overridden item and the outside one", st.Failures)
	}
	// A failure item carries the remedy its line recorded, and one whose line
	// names none carries none.
	remedies := map[string]string{}
	for _, f := range st.Failures {
		remedies[strings.SplitN(f.Detail, ":", 2)[0]] = f.Remedy
	}
	// The reply is longer than the summary's bound; the note the line recorded
	// still ends the detail.
	for _, f := range st.Failures {
		if f.Remedy != "" && !strings.HasSuffix(f.Detail, "; "+refusedNote) {
			t.Errorf("failure detail %q does not end with the note its line recorded", f.Detail)
		}
	}
	if want := map[string]string{"clm-aaaaaa": "", "clm-cccccc": "the command that lets the call through"}; !maps.Equal(remedies, want) {
		t.Errorf("failure remedies %v, want %v", remedies, want)
	}
}

// TestReadResultsSkipsAFileThatNoLongerStands: a result file the flush removes
// between the listing and the read is no result, not an error. A dangling link
// is listed and fails to read the same way.
func TestReadResultsSkipsAFileThatNoLongerStands(t *testing.T) {
	runDir := t.TempDir()
	dir := filepath.Join(runDir, resultsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "0"+resultSuffix)); err != nil {
		t.Fatal(err)
	}
	got, err := readResults(runDir)
	if err != nil || len(got) != 0 {
		t.Errorf("readResults = %v, %v; want none and no error", got, err)
	}
}

// TestWaitRunReturnsTheRunsDocument: the run wrote its result document, so the
// waiter reads that document rather than inventing one, and exits on the outcome
// it names — cancelled here, so the code 4 is the document's own.
func TestWaitRunReturnsTheRunsDocument(t *testing.T) {
	state, pid := t.TempDir(), os.Getpid()
	l := heldRun(t, state, "run-cancelled", pid)
	if err := WriteResultDocument(l.Dir(), result.Cancelled, nil); err != nil {
		t.Fatal(err)
	}

	doc, code, err := WaitRun(state, pid, time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(l.Dir(), ResultFile))
	if err != nil {
		t.Fatal(err)
	}
	if doc != string(want) {
		t.Errorf("wait reported\n%s\nwant the run's own document\n%s", doc, want)
	}
	if code != result.ExitCode(result.Cancelled) {
		t.Errorf("exit %d, want the cancelled outcome's %d", code, result.ExitCode(result.Cancelled))
	}
}

// TestWaitRunWaitsWhileTheRunHoldsTheStore: a holder record naming the run's
// own process is a run still in flight, so the waiter polls rather than
// reporting the fallback, and returns the document the run writes while it
// waits — a document that did not stand when the first pass read there.
func TestWaitRunWaitsWhileTheRunHoldsTheStore(t *testing.T) {
	state := t.TempDir()
	l := heldRun(t, state, "run-live", os.Getpid())
	t.Cleanup(holdStore(t, state, "run-live"))

	written := make(chan error, 1)
	go func() {
		time.Sleep(5 * time.Millisecond)
		written <- WriteResultDocument(l.Dir(), result.Cancelled, nil)
	}()

	doc, code, err := WaitRun(state, os.Getpid(), time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if err := <-written; err != nil {
		t.Fatalf("the run's document: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(l.Dir(), ResultFile))
	if err != nil {
		t.Fatalf("reading the document the run wrote: %v", err)
	}
	if doc != string(want) {
		t.Errorf("wait reported\n%s\nwant the document the run wrote\n%s", doc, want)
	}
	if code != result.ExitCode(result.Cancelled) {
		t.Errorf("exit %d, want the cancelled outcome's %d", code, result.ExitCode(result.Cancelled))
	}
}

// TestWaitRunStopsBehindAnotherProcessHolder: a holder record naming a
// process other than the run's is a later run holding the store — a build
// started after this one ended — and is no reason to wait, so the waiter
// reports the run's end at once. A waiter that polled behind it would never
// return, so the watchdog turns that into a failure rather than a hung suite,
// releasing the store first so the polling goroutine unwinds.
func TestWaitRunStopsBehindAnotherProcessHolder(t *testing.T) {
	state := t.TempDir()
	release := holdStore(t, state, "run-live")
	t.Cleanup(release)
	mine := os.Getpid() + 1
	heldRun(t, state, "run-superseded", mine)

	type waited struct {
		doc  string
		code int
		err  error
	}
	got := make(chan waited, 1)
	go func() {
		doc, code, err := WaitRun(state, mine, time.Millisecond)
		got <- waited{doc, code, err}
	}()

	var res waited
	select {
	case res = <-got:
	case <-time.After(2 * time.Second):
		release()
		t.Fatalf("WaitRun is still polling behind a holder of another process")
	}
	if res.err != nil {
		t.Fatalf("wait: %v", res.err)
	}
	if res.code != result.ExitCode(result.Failed) || res.code != 3 {
		t.Errorf("exit %d, want the run's end, which is failed's %d (3)", res.code, result.ExitCode(result.Failed))
	}
	if d := decoded(t, res.doc); d["outcome"] != result.Failed || d["run"] != "run-superseded" {
		t.Errorf("document names outcome %v, run %v; want failed, run-superseded\n%s", d["outcome"], d["run"], res.doc)
	}
}

// TestWaitRunReportsARunItsProcessLeft: no document and no live holder is a
// run whose process ended without writing its result. The waiter reports it from
// the run's own ledger — the counts and the one item that landed — with the
// failure naming the end, and exits on failed, which is 3.
func TestWaitRunReportsARunItsProcessLeft(t *testing.T) {
	state, pid := t.TempDir(), os.Getpid()
	heldRun(t, state, "run-abandoned", pid)

	doc, code, err := WaitRun(state, pid, time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if code != result.ExitCode(result.Failed) || code != 3 {
		t.Errorf("exit %d, want failed's %d (3)", code, result.ExitCode(result.Failed))
	}
	d := decoded(t, doc)
	if d["outcome"] != result.Failed || d["run"] != "run-abandoned" || d["state-dir"] != state || d["pass"] != query.PassRigor {
		t.Errorf("document names outcome %v, run %v, state-dir %v, pass %v; want failed, run-abandoned, %s, %s\n%s",
			d["outcome"], d["run"], d["state-dir"], d["pass"], state, query.PassRigor, doc)
	}
	counts, _ := d["counts"].(map[string]any)
	if counts["total"] != 2 || counts["done"] != 1 || counts["failed"] != 0 || counts["remaining"] != 1 {
		t.Errorf("counts = %v, want the two items with one landed\n%s", d["counts"], doc)
	}
	var scored []string
	if list, ok := d["scored"].([]any); ok {
		for _, s := range list {
			name, _ := s.(string)
			scored = append(scored, name)
		}
	}
	if !slices.Equal(scored, []string{"clm-aaaaaa"}) {
		t.Errorf("scored = %q, want the one item whose value landed", scored)
	}
	failures, _ := d[result.FailuresKey].([]any)
	if len(failures) != 1 {
		t.Fatalf("failures = %v, want the one item naming the process's end\n%s", failures, doc)
	}
	item, _ := failures[0].(map[string]any)
	if detail, _ := item["detail"].(string); !strings.Contains(detail, "ended without writing its result") {
		t.Errorf("failure = %q, want it to name the process's end", detail)
	}
}
