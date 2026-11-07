package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/filelock"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/modelclient"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
	"kbase/internal/secret"
)

// scoreRepo is a KB the score verb can walk: one claim recorded and marked in a
// leaf, refreshed so its register page stands. It is the smallest KB a pass has an item over; the fixture
// under testdata carries the graph every pass is exercised on.
func scoreRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	rigor := 0.5
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": maintenanceEntryPoint,
		maintenanceStampFile:     maintenanceFormatStamp,
		"kb-root/a.md":           "[↑ KB](entry-point.md)\n\n# A\n\nThe claim's own paragraph.<!-- kbase-id: clm-aaaaaa -->\n",
	}
	maps.Copy(files, recordFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A Claim", Rigor: &rigor, Rationale: "Shown."}}}))
	writeRepo(t, repo, files)
	updateFormatKB(t, repo)
	return repo
}

// fixtureRepo stages a copy of the fixture KB under name — a directory under
// testdata/ — as a repository, and nothing else: the caller converts it, or
// leaves the KB in the format it was authored in.
func fixtureRepo(t *testing.T, name string) string {
	t.Helper()
	return stageRepo(t, filepath.Join("testdata", name))
}

// writeRepo writes files under repo, making each directory they name.
func writeRepo(t *testing.T, repo string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// scoreProvider is the pool and configuration a score run resolves against: one
// entry in the pool, and the scoring role naming it — the role's own reference
// is what chooses the endpoint, so no --provider and no config.toml provider
// key is involved. The text role names a different model, so a run that fell
// back to it would say so.
func scoreProvider() providerOptions {
	return providerOptions{
		Providers: config.Providers{"solo": provider("solo", "http://solo.example/v1")},
		Config: config.Config{Models: config.ModelMap{
			Decision: "solo/decision-model", Text: "solo/text-model", Scoring: "solo/scoring-model"}},
	}
}

// scoreResult is one runScore invocation: its result, its streams, its exit
// code, and how many clients the verb asked its factory for.
type scoreResult struct {
	doc     parsedResult
	stdout  string
	stderr  string
	code    int
	dialled int
	// endpoints are the endpoints the verb's factory was asked to build, in
	// the order it was asked: one per client, so a run reaching two providers
	// says which.
	endpoints []model.Endpoint
}

// scoreIn drives one score invocation over repo with its state under state. A
// client the verb dials comes from client; a run that dials with no client
// installed fails the test, so a dry run cannot reach a provider unnoticed. A
// caller's own Ctx drives the run, and its absence runs it uncancelled.
func scoreIn(t *testing.T, repo, state string, opts scoreOptions, client func() model.Client) scoreResult {
	t.Helper()
	return scoreDrive(t, repo, state, opts, func(model.Endpoint) model.Client {
		if client == nil {
			t.Fatal("the run dialled a provider")
		}
		return client()
	})
}

// scoreDrive drives one score invocation over repo with its state under state,
// newClient being the registry's client seam. It is scoreIn's body for a test
// that reads the endpoint the verb built — what the registry put on it is what
// the wire carries.
func scoreDrive(t *testing.T, repo, state string, opts scoreOptions, newClient modelclient.NewClientFunc) scoreResult {
	t.Helper()
	if opts.Providers == nil {
		opts.providerOptions = scoreProvider()
	}
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}
	if opts.AskConcurrency == nil {
		// The scripted providers answer in the order calls reach them, which
		// only one call in flight at a time fixes to the walk's order.
		opts.AskConcurrency = new(1)
	}
	var stdout, stderr bytes.Buffer
	opts.WorkDir, opts.StateDir = repo, state
	opts.Stdout, opts.Stderr, opts.Logger = &stdout, &stderr, log.Discard()
	var dialled int
	var endpoints []model.Endpoint
	opts.NewClient = func(e model.Endpoint) model.Client {
		dialled++
		endpoints = append(endpoints, e)
		return newClient(e)
	}
	code, err := runScore(opts)
	if err != nil {
		t.Fatalf("runScore: %v", err)
	}
	got := scoreResult{doc: checkResult(t, "score", stdout.String()), stdout: stdout.String(), stderr: stderr.String(),
		code: code, dialled: dialled, endpoints: endpoints}
	if want := toolresult.ExitCode(got.doc.Outcome); got.code != want {
		t.Errorf("outcome %s with exit %d, want %d\n%s", got.doc.Outcome, got.code, want, got.stdout)
	}
	return got
}

// scoring is a scripted provider serving one reply per call, in order.
func scoring(replies ...string) model.Client { return scriptedScoring(replies...).Client() }

// scriptedScoring is scoring's script, for a double that wraps it.
func scriptedScoring(replies ...string) *modeltest.MockClient {
	responses := make([]model.Response, len(replies))
	for i, reply := range replies {
		responses[i] = model.Response{Content: reply}
	}
	return modeltest.NewScriptedMockPerConsult(responses)
}

// cancelOnSecond is a provider that cancels the run's context as the second
// call arrives: the first item's question is answered and lands, and the item
// after it meets a cancelled context, which is where the walk stops.
type cancelOnSecond struct {
	MockClient *modeltest.MockClient
	cancel     context.CancelFunc
	calls      int
}

func (c *cancelOnSecond) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	c.calls++
	if c.calls == 2 {
		c.cancel()
	}
	return c.MockClient.ConsultStream(ctx, req)
}

// Client is the model.Client whose calls c answers.
func (c *cancelOnSecond) Client() model.Client {
	return &model.ClientFuncs{Stream: c.ConsultStream, Decide: c.MockClient.Decision, List: c.MockClient.ListModels}
}

// rigorOf is the index's rigor for the node carrying id.
func rigorOf(t *testing.T, repo, id string) any {
	t.Helper()
	src, err := kbload.Open(filepath.Join(repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := query.ShowPayload(ix, id)
	if !ok {
		t.Fatalf("the index holds no node %s", id)
	}
	for _, f := range rec {
		if f.Key == "rigor" {
			return f.Value
		}
	}
	return nil
}

// formatOf is the metadata format the KB at repo stands at, as its loader reads
// it.
func formatOf(t *testing.T, repo string) string {
	t.Helper()
	src, err := kbload.Open(filepath.Join(repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	return src.Version()
}

// scoreFixture is the fixture KB the scoring passes are exercised on: the
// testdata/score-kb copy, committed and converted by update-format, so its
// conversion is saved, its index stands and it is stamped current.
// internal/score's invokeKB is the same KB in that package's own form: a
// package test reads only its own package's testdata.
func scoreFixture(t *testing.T) string {
	t.Helper()
	repo := fixtureRepo(t, "score-kb")
	updateFormatKB(t, repo)
	return repo
}

// TestScoreDocumentsARun: a run resolves the scoring role — never the text one —
// dials it once, and states the run in its result: the pass asked for, the
// role's provider and model, the run and its ledger, and the counts and the items
// scored that the run reported.
func TestScoreDocumentsARun(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client {
		return scoring("none", "none", "none", "none")
	})
	if got.doc.Outcome != "done" || got.dialled != 1 {
		t.Fatalf("the rigor run: outcome %s, %d dials\n%s", got.doc.Outcome, got.dialled, got.stdout)
	}
	if want := []string{"outcome", "kb-root", "state-dir", "pass", "roles",
		"run", "ledger", "counts", "scored", "warnings"}; !slices.Equal(got.doc.Keys, want) {
		t.Errorf("keys %q, want %q", got.doc.Keys, want)
	}
	if role := roleOf(got.doc, "scoring"); got.doc.Values["pass"] != query.PassRigor || role == nil ||
		role["model"] != "scoring-model" || role["provider"] != "solo" {
		t.Errorf("roles %v, pass %v; want the scoring role's model and provider and the pass asked for",
			got.doc.Values["roles"], got.doc.Values["pass"])
	}
	if strings.Contains(got.stdout, "text-model") {
		t.Errorf("the run names the text model:\n%s", got.stdout)
	}
	if counts, _ := got.doc.Values["counts"].(map[string]any); counts["total"] != 4 || counts["done"] != 4 ||
		len(anyStrings(got.doc.Values["scored"])) != 4 {
		t.Errorf("counts %v, scored %v; want the run's four items counted and named\n%s",
			got.doc.Values["counts"], got.doc.Values["scored"], got.stdout)
	}
}

// jsonLines is one of a run's JSONL files as its records, in file order.
func jsonLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("%s: %q does not parse: %v", path, line, err)
		}
		out = append(out, rec)
	}
	return out
}

// TestScoreWalksEveryPassByDefault: a run naming no pass walks both sets and
// says so in its result, a finished run carries no resume, and a second run
// over the KB it scored asks nothing: every value it landed recorded the hash
// of what it was judged against, so nothing is dirty.
func TestScoreWalksEveryPassByDefault(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{}, func() model.Client {
		return scoring("none", "none", "none", "none", "none", "none", "none", "none")
	})
	if got.doc.Outcome != "done" || got.dialled != 1 {
		t.Fatalf("the run: outcome %s, %d dials\n%s", got.doc.Outcome, got.dialled, got.stdout)
	}
	if got.doc.Values["pass"] != query.PassAll {
		t.Errorf("the result's pass = %v, want the mixed set %q", got.doc.Values["pass"], query.PassAll)
	}
	if counts, _ := got.doc.Values["counts"].(map[string]any); counts["total"] != 8 {
		t.Errorf("counts %v, want the eight items of both sets\n%s", got.doc.Values["counts"], got.stdout)
	}
	if _, resumes := got.doc.Values["resume"]; resumes {
		t.Errorf("a finished run carries resume:\n%s", got.stdout)
	}
	again := scoreIn(t, repo, state, scoreOptions{}, func() model.Client { return scoring() })
	if counts, _ := again.doc.Values["counts"].(map[string]any); again.doc.Outcome != "done" || counts["total"] != 0 {
		t.Errorf("the second run: outcome %s, counts %v; want done over no dirty item\n%s", again.doc.Outcome, counts, again.stdout)
	}
}

// TestScoreResumesAMixedRun: a run whose mixed set fails after its first five
// items leaves the rest for the call that continues it, which carries no
// argument naming the scope — the header is what says which set the run walks
// — and continuing it walks the rest to the end.
func TestScoreResumesAMixedRun(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	partial := scoreIn(t, repo, state, scoreOptions{}, func() model.Client {
		return scoring("none", "none", "none", "none", "none")
	})
	if partial.doc.Outcome != "failed" {
		t.Fatalf("the failing run: outcome %s\n%s", partial.doc.Outcome, partial.stdout)
	}
	run, _ := partial.doc.Values["run"].(string)
	if resume, _ := partial.doc.Values["resume"].(string); resume != "kbase score --resume --state-dir "+state {
		t.Errorf("resume %q, want the most recent run continued with no argument naming its scope", resume)
	}
	again := scoreIn(t, repo, state, scoreOptions{Resume: true}, func() model.Client { return scoring("none", "none", "none") })
	counts, _ := again.doc.Values["counts"].(map[string]any)
	if again.doc.Outcome != "done" || again.doc.Values["run"] != run || again.doc.Values["pass"] != query.PassAll ||
		counts["total"] != 8 || counts["done"] != 8 {
		t.Errorf("the resume: outcome %s, run %v, pass %v, counts %v; want run %s over the mixed set, every item done\n%s",
			again.doc.Outcome, again.doc.Values["run"], again.doc.Values["pass"], counts, run, again.stdout)
	}
}

// lineRecord is a run's ledger line, parsed.
func lineRecord(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(b), &rec); err != nil {
		t.Fatalf("%s does not parse: %v", path, err)
	}
	return rec
}

// TestScoreCancelledClosesOnTheItemItStoppedBefore: a run stopped by a cancelled
// context closes its result on the cancelled record, which names the item the
// run stopped before, and keeps resume before that record. The result passes
// the contract's own key check, which scoreIn ran.
func TestScoreCancelledClosesOnTheItemItStoppedBefore(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Ctx: ctx}, func() model.Client {
		return (&cancelOnSecond{MockClient: scriptedScoring("none"), cancel: cancel}).Client()
	})
	if got.doc.Outcome != "cancelled" || got.code != toolresult.ExitCode("cancelled") {
		t.Fatalf("the cancelled run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	keys := got.doc.Keys
	if keys[len(keys)-1] != "cancelled" || keys[len(keys)-2] != "warnings" || keys[len(keys)-3] != "resume" {
		t.Errorf("keys %q, want resume, then warnings, before the cancelled record the outcome closes on", keys)
	}
	if stopped, _ := got.doc.Values["cancelled"].(map[string]any); stopped["item"] == nil {
		t.Errorf("cancelled %v, want the item the run stopped before\n%s", stopped, got.stdout)
	}
}

// TestScoreFailedRunCarriesResumeBeforeFailures: a run whose items failed
// after one landed reports the call that continues it ahead of the record its
// outcome closes on, one failure per failed item.
func TestScoreFailedRunCarriesResumeBeforeFailures(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none") })
	if got.doc.Outcome != "failed" || got.code != toolresult.ExitCode("failed") {
		t.Fatalf("the run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	keys := got.doc.Keys
	if keys[len(keys)-1] != "failures" || keys[len(keys)-2] != "warnings" || keys[len(keys)-3] != "resume" {
		t.Fatalf("keys %q, want resume, then warnings, before the record the outcome closes on", keys)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["total"] != 4 || counts["done"] != 1 || counts["failed"] != 3 {
		t.Errorf("counts %v, want one item landed and three failed\n%s", counts, got.stdout)
	}
	if items := got.doc.items("failures"); len(items) != 3 || items[0]["check"] != checkError {
		t.Errorf("failures %v, want the three failed items'\n%s", items, got.stdout)
	}
}

// TestScoreFailedRunClosesOnFailures: a run whose item cannot be written ends
// `failed` and closes its result on the failures record the run reported, not
// on nothing.
func TestScoreFailedRunClosesOnFailures(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	// The records' directory refuses every new file, which is where a landing
	// writes; the page stands current from scoreRepo's refresh, so the run's
	// start refresh has no changed file to write and the walk reaches its one
	// item.
	dir := filepath.Join(repo, kb.KBDir, kb.IndexDir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none") })
	if got.doc.Outcome != "failed" || got.code != toolresult.ExitCode("failed") {
		t.Fatalf("the unwritable KB: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if keys := got.doc.Keys; keys[len(keys)-1] != "failures" {
		t.Errorf("keys %q, want failures last", keys)
	}
	if items := got.doc.items("failures"); len(items) != 1 {
		t.Errorf("failures %v, want the one item the run failed\n%s", items, got.stdout)
	}
}

// TestScoreDryRunIsInert: a dry run reports done, dials no provider, resolves no
// role — no roles record, which is where the four provider keys were — and
// carries no resume.
func TestScoreDryRunIsInert(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, DryRun: true}, nil)
	if got.doc.Outcome != "done" || got.dialled != 0 {
		t.Fatalf("the dry run: outcome %s, %d dials\n%s", got.doc.Outcome, got.dialled, got.stdout)
	}
	for _, key := range []string{"provider", "provider-rule", "provider-layer", "model", "roles", "resume"} {
		if v, ok := got.doc.Values[key]; ok {
			t.Errorf("a dry run carries %s: %v\n%s", key, v, got.stdout)
		}
	}
}

// TestScoreRefusesADryRunOnAnOlderKB: a dry run over the fixture's 1.0.0 KB
// is refused as a real run is, remedy kbase update-format, with the migration
// record after kb-root, and the KB left as it stands.
func TestScoreRefusesADryRunOnAnOlderKB(t *testing.T) {
	repo, state := fixtureRepo(t, "score-kb-1.0.0"), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, DryRun: true}, nil)
	if got.doc.Outcome != "refused" || got.code != toolresult.ExitCode("refused") {
		t.Fatalf("the dry run over 1.0.0: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if items := got.doc.items("refusals"); len(items) != 1 || items[0]["check"] != index.CheckIndex || items[0]["remedy"] != atTheCommandLine(index.UpdateFormatRemedy) {
		t.Errorf("refusals %v, want the pending conversion's, remedy %s", items, atTheCommandLine(index.UpdateFormatRemedy))
	}
	if got.doc.Keys[1] != "kb-root" || got.doc.Keys[2] != "migration" {
		t.Errorf("keys %q, want the migration record after kb-root", got.doc.Keys)
	}
	if format := formatOf(t, repo); format != "1.0.0" {
		t.Errorf("the KB stands at %s, want the dry run to have written nothing", format)
	}
}

// TestScoreSaysTheKeyTravelsInCleartext: a run whose endpoint is http:// to a
// host off the local network, with a key configured, says so in one line on stderr
// and proceeds — the same line `build` writes, naming the host and never the key.
// A local-network endpoint says nothing of it: on a PATH without dot, stderr holds
// only the lines saying no sheet was drawn.
func TestScoreSaysTheKeyTravelsInCleartext(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		warn      bool
	}{
		{"a plain-http host", "http://lab.example/v1", true},
		{"loopback", "http://127.0.0.1:8080/v1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.warn {
				t.Setenv("PATH", gitOnlyPath(t))
			}
			opts := scoreOptions{Pass: query.PassRigor}
			opts.providerOptions = providerOptions{
				Providers: config.Providers{"solo": provider("solo", tc.url)},
				Config:    config.Config{Models: config.ModelMap{Scoring: "solo/scoring-model"}},
			}
			got := scoreIn(t, scoreRepo(t), t.TempDir(), opts, func() model.Client { return scoring("none") })
			if got.doc.Outcome != "done" {
				t.Fatalf("the run: outcome %s\n%s", got.doc.Outcome, got.stdout)
			}
			if !tc.warn {
				// The run refreshes at its start, saying that dot is absent;
				// its landing's derivation says nothing on stderr.
				var noDot bytes.Buffer
				sayNoDot(&noDot, "score")
				if want := noDot.String(); got.stderr != want {
					t.Errorf("stderr = %q, want %q alone for %s", got.stderr, want, tc.url)
				}
				return
			}
			line := config.CleartextWarning(tc.url, secret.NewKey(testAPIKey))
			if line == "" || strings.Count(got.stderr, line) != 1 {
				t.Errorf("stderr (%d bytes), want one cleartext warning for %s", len(got.stderr), tc.url)
			}
			if strings.Contains(got.stderr, testAPIKey) {
				t.Error("stderr names the key")
			}
		})
	}
}

// standRuns stands n runs of kind in the store at state, named kind-000000 on
// and started a minute apart from the minute at in January, so every run a test
// invocation starts is newer. It returns their ids, the oldest first.
func standRuns(t *testing.T, state, kind string, n, at int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%06d", kind, i)
		started := time.Date(2026, 1, 1, 0, at+i, 0, 0, time.UTC).Format(time.RFC3339)
		l, err := score.Create(state, score.RunHeader{Run: ids[i], KBRoot: "/repo/kb-root", Pass: query.PassRigor, Started: started})
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

// TestScoreRefusals: an unknown pass is a usage refusal, naming the
// passes the flag takes; a run that meets the lock another run holds is retry,
// naming the directory held rather than a file that is not there.
func TestScoreRefusals(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: "other"}, nil)
	if got.doc.Outcome != "refused" || got.code != toolresult.ExitCode("refused") {
		t.Fatalf("--pass other: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items := got.doc.items("refusals")
	if len(items) != 1 || items[0]["check"] != toolresult.CheckUsage || items[0]["key"] != "--pass" ||
		!slices.Equal(anyStrings(items[0]["allowed"]), []string{query.PassRigor, query.PassBearing, query.PassAll}) {
		t.Errorf("refusals %v, want one usage refusal naming the passes the flag takes\n%s", items, got.stdout)
	}

	runs := filepath.Join(state, "score")
	if err := os.MkdirAll(runs, 0o777); err != nil {
		t.Fatal(err)
	}
	// A build holds the KB: the operation lock and the store behind it. The run
	// is refused at once, naming the holder's verb and its pid, before it asks
	// anything.
	buildState := t.TempDir()
	release, err := build.Hold(filepath.Join(repo, kb.KBDir), buildState, buildstore.KindBuild, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	got = scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, nil)
	if got.doc.Outcome != "refused" || got.code != toolresult.ExitCode("refused") {
		t.Fatalf("a run over a KB a build holds: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items = got.doc.items("refusals")
	want := fmt.Sprintf("a build (pid %d) is running; its state-dir is %s", os.Getpid(), buildState)
	if len(items) != 1 || items[0]["check"] != checkLock || items[0]["path"] != buildState || items[0]["detail"] != want {
		t.Errorf("refusals %v, want one lock refusal naming %q\n%s", items, want, got.stdout)
	}
	if got.dialled != 0 {
		t.Errorf("a refused run dialled %d clients", got.dialled)
	}
	if live, h, err := buildstore.HolderOf(state); err != nil || live || h.Kind != "" {
		t.Errorf("the refused run took the store: live %v, record %+v, %v", live, h, err)
	}
}

// TestScoreRetriesWhileAnotherRunHoldsTheStore: a store another run holds is a
// retry naming it — the run takes the operation lock first and lets it go on
// the way out.
func TestScoreRetriesWhileAnotherRunHoldsTheStore(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := filelock.Acquire(buildstore.HolderPath(state), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	record, err := json.Marshal(buildstore.Holder{PID: 1 << 30, Started: "2026-01-01T00:00:00Z", Kind: buildstore.KindBuild})
	if err != nil {
		t.Fatal(err)
	}
	if err := held.Write(record); err != nil {
		t.Fatal(err)
	}

	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, nil)
	if got.doc.Outcome != "retry" || got.code != toolresult.ExitCode("retry") {
		t.Fatalf("a run behind a held store: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items := got.doc.items("refusals")
	if len(items) != 1 || items[0]["check"] != checkLock || items[0]["path"] != buildstore.HolderPath(state) ||
		items[0]["detail"] != "another build (pid 1073741824, started 2026-01-01T00:00:00Z) holds the state store; "+toolresult.RetryAsk {
		t.Errorf("refusals %v, want one lock retry naming the store's holder\n%s", items, got.stdout)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "kbase-operation.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the operation lock the retried run took stands: %v", err)
	}
}

// TestScoreRunsOverAnOlderKBOnceUpdateFormatSavesIt: a real run over the
// fixture's 1.0.0 KB is refused, asking nothing and writing nothing; once
// update-format saves the conversion a run is done, a value lands as a node of
// the converted KB, and no run carries a migration record.
func TestScoreRunsOverAnOlderKBOnceUpdateFormatSavesIt(t *testing.T) {
	repo, state := fixtureRepo(t, "score-kb-1.0.0"), t.TempDir()
	if format := formatOf(t, repo); format != "1.0.0" {
		t.Fatalf("the fixture stands at %s, want the 1.0.0 KB it is", format)
	}
	refusedRun := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none", "none") })
	if refusedRun.doc.Outcome != "refused" || refusedRun.dialled != 0 || formatOf(t, repo) != "1.0.0" {
		t.Fatalf("the run over 1.0.0: outcome %s, %d dials, want refused before any ask or write\n%s", refusedRun.doc.Outcome, refusedRun.dialled, refusedRun.stdout)
	}
	updateFormatKB(t, repo)
	if format := formatOf(t, repo); format != kb.FormatVersion {
		t.Fatalf("the KB stands at %s after update-format, want %s", format, kb.FormatVersion)
	}
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none", "none") })
	if got.doc.Outcome != "done" {
		t.Fatalf("the run over the converted KB: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if _, migrated := got.doc.Values["migration"]; migrated {
		t.Errorf("a run over the converted KB carries a migration record:\n%s", got.stdout)
	}
	if v := rigorOf(t, repo, "clm-aaaaaa"); v != 1.0 {
		t.Errorf("the claim's rigor = %v, want the maximum the reply `none` lands", v)
	}
	if v := rigorOf(t, repo, "ana-aaaaaa"); v != 1.0 {
		t.Errorf("the analysis the split minted = %v, want it scored as a node of the converted KB", v)
	}
	again := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none", "none") })
	if again.doc.Outcome != "done" {
		t.Fatalf("the second run: outcome %s\n%s", again.doc.Outcome, again.stdout)
	}
	if _, migrated := again.doc.Values["migration"]; migrated {
		t.Errorf("a run over a current KB carries a migration record:\n%s", again.stdout)
	}
}

// statusProvider answers every request with code, counting the requests it
// read, and serves as the endpoint a score run dials.
func statusProvider(t *testing.T, code int) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	var seen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen++
		mu.Unlock()
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error":{"message":"unknown model: m"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() int {
		mu.Lock()
		defer mu.Unlock()
		return seen
	}
}

// scoreProviderAt is scoreProvider with its pool entry's base URL at baseURL:
// the one entry a scoring run resolves its role to, dialling a test server.
func scoreProviderAt(baseURL string) providerOptions {
	p := scoreProvider()
	p.Providers["solo"] = provider("solo", baseURL)
	return p
}

// scoringWire is the endpoint a scoring run's asks reach: every request body it
// read, in the order it read them, each answered with the charge list `none` —
// but, where refuseFrom is above zero, a request from that one on, counted
// from 1, which it refuses with a 400.
type scoringWire struct {
	mu         sync.Mutex
	bodies     [][]byte
	refuseFrom int
}

// handler reads each request's body and answers it with the charge list `none`
// as a stream: the reply a run prices to the maximum, so every item lands.
func (w *scoringWire) handler() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.mu.Lock()
		w.bodies = append(w.bodies, body)
		refused := w.refuseFrom > 0 && len(w.bodies) >= w.refuseFrom
		w.mu.Unlock()
		if refused {
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte(`{"error":{"message":"refused"}}`))
			return
		}
		rw.Header().Set("Content-Type", "text/event-stream")
		_, _ = rw.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"none\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":1,\"total_tokens\":41}}\n\n" +
			"data: [DONE]\n\n"))
	}
}

// promptCacheField is one body's `prompt_cache_key`, and whether the body
// carried the field at all: absent and empty are different requests, and only
// the first is "sends no key".
type promptCacheField struct {
	key     string
	present bool
}

// promptCacheFields is what every body the endpoint read carried, in the order
// it read them.
func (w *scoringWire) promptCacheFields(t *testing.T) []promptCacheField {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]promptCacheField, 0, len(w.bodies))
	for _, body := range w.bodies {
		var wire struct {
			Key *string `json:"prompt_cache_key"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		f := promptCacheField{present: wire.Key != nil}
		if wire.Key != nil {
			f.key = *wire.Key
		}
		out = append(out, f)
	}
	return out
}

// TestScoreSendsTheRunAsThePromptCacheKey: every ask of a scoring run carries
// the run's id as `prompt_cache_key`, so a run's asks reach one upstream and
// share one cache entry. Two items of one run, one server, one key — the id the
// result names.
func TestScoreSendsTheRunAsThePromptCacheKey(t *testing.T) {
	repo := scoreFixture(t)
	wire := &scoringWire{}
	srv := httptest.NewServer(wire.handler())
	defer srv.Close()

	got := scoreDrive(t, repo, t.TempDir(), scoreOptions{Pass: query.PassRigor, providerOptions: scoreProviderAt(srv.URL)},
		func(e model.Endpoint) model.Client { return model.NewOpenAIClient(e) })
	if got.doc.Outcome != "done" {
		t.Fatalf("the run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	run, _ := got.doc.Values["run"].(string)
	if run == "" {
		t.Fatalf("the result names no run: %v", got.doc.Values)
	}
	fields := wire.promptCacheFields(t)
	counts, _ := got.doc.Values["counts"].(map[string]any)
	total, _ := counts["total"].(int)
	if total < 2 || len(fields) != total {
		t.Fatalf("%d asks for %v items, want one per item and at least two\n%s", len(fields), counts["total"], got.stdout)
	}
	for i, f := range fields {
		if !f.present || f.key != run {
			t.Errorf("ask %d carried prompt_cache_key %q (present %v), want the run id %q", i+1, f.key, f.present, run)
		}
	}
}

// TestScoreResumeSendsTheSamePromptCacheKey: a resumed run continues the most
// recent run's id rather than minting one, so its asks carry the key the first
// invocation's did — one provider and one cache entry for the run, across
// processes.
func TestScoreResumeSendsTheSamePromptCacheKey(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	wire := &scoringWire{refuseFrom: 2}
	srv := httptest.NewServer(wire.handler())
	defer srv.Close()
	overHTTP := func(e model.Endpoint) model.Client { return model.NewOpenAIClient(e) }

	first := scoreDrive(t, repo, state, scoreOptions{Pass: query.PassRigor, providerOptions: scoreProviderAt(srv.URL)}, overHTTP)
	run, _ := first.doc.Values["run"].(string)
	if run == "" || first.doc.Outcome != "failed" {
		t.Fatalf("the first invocation: outcome %s, run %q; want failed naming its run\n%s", first.doc.Outcome, run, first.stdout)
	}
	firstAsks := wire.promptCacheFields(t)
	if len(firstAsks) != 4 {
		t.Fatalf("%d asks for the four items of the rigor set\n%s", len(firstAsks), first.stdout)
	}
	for i, f := range firstAsks {
		if !f.present || f.key != run {
			t.Errorf("the first invocation's ask %d carried prompt_cache_key %q (present %v), want the run id %q", i+1, f.key, f.present, run)
		}
	}

	wire.mu.Lock()
	wire.refuseFrom = 0
	wire.mu.Unlock()
	resumed := scoreDrive(t, repo, state, scoreOptions{Resume: true, providerOptions: scoreProviderAt(srv.URL)}, overHTTP)
	if got := resumed.doc.Values["run"]; got != run {
		t.Fatalf("the resumed run's result names %v, want %q", got, run)
	}
	all := wire.promptCacheFields(t)
	if len(all) <= len(firstAsks) {
		t.Fatalf("the resumed run asked nothing: %d asks over both invocations\n%s", len(all), resumed.stdout)
	}
	for i, f := range all {
		if !f.present || f.key != run {
			t.Errorf("ask %d carried prompt_cache_key %q (present %v), want the run id %q", i+1, f.key, f.present, run)
		}
	}
}

// TestScoreAsksOnceOnARefusal: a 400 will not change, so the item's ask is one
// request and fails its item at once, naming the status. The attempt count on
// a retryable status is internal/model's, pinned there.
func TestScoreAsksOnceOnARefusal(t *testing.T) {
	srv, asks := statusProvider(t, http.StatusBadRequest)
	got := scoreIn(t, scoreRepo(t), t.TempDir(), scoreOptions{Pass: query.PassRigor},
		func() model.Client { return model.NewOpenAIClient(model.Endpoint{BaseURL: srv.URL}) })
	if n := asks(); n != 1 {
		t.Errorf("%d requests, want 1\n%s", n, got.stdout)
	}
	if got.doc.Outcome != "failed" || got.code != toolresult.ExitCode("failed") {
		t.Fatalf("the run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["total"] != 1 || counts["failed"] != 1 || counts["done"] != 0 {
		t.Errorf("counts %v, want the one item failed\n%s", got.doc.Values["counts"], got.stdout)
	}
	items := got.doc.items("failures")
	if len(items) != 1 || items[0]["check"] != checkError {
		t.Fatalf("failures %v, want the one item's\n%s", items, got.stdout)
	}
	detail, _ := items[0]["detail"].(string)
	if !strings.HasPrefix(detail, "clm-aaaaaa: ") || !strings.Contains(detail, fmt.Sprintf("http %d", http.StatusBadRequest)) {
		t.Errorf("failure %q, want the item and the status that refused it\n%s", detail, got.stdout)
	}
}
