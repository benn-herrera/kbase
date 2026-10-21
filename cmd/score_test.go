package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/filelock"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/query"
)

// scoreRepo is a KB the score verb can walk: one claim declared in a leaf and
// authored in a register, refreshed so the index stands and the entry point is
// stamped current. It is the smallest KB a pass has an item over; the fixture
// under testdata carries the graph every pass is exercised on.
func scoreRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": "---\nkind: entry-point\nkb-format: \"" + kb.FormatVersion + "\"\n---\n\n# KB\n\n- [a](a.md)\n",
		"kb-root/a.md": "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ KB](entry-point.md)\n\n# A\n\n" +
			"The claim's own paragraph. <!-- claim-rigor: clm-aaaaaa -->\n",
		"kb-root/claim-rigor.md": "# Register\n\n## A Claim\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n" +
			"- rigor: 0.50\n- strength: *pending*\n- rationale: Shown.\n",
	}
	writeRepo(t, repo, files)
	if out, code := kbase(t, repo, "refresh"); code != 0 {
		t.Fatalf("refresh: exit %d\n%s", code, out)
	}
	return repo
}

// fixtureRepo stages a copy of the fixture KB under name — a directory under
// testdata/ — as a repository, and nothing else: the caller refreshes, or
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

// scoreResult is one runScore invocation: its document, its streams, its exit
// code, and how many clients the verb asked its factory for.
type scoreResult struct {
	doc     resultDoc
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
	if opts.Providers == nil {
		opts.providerOptions = scoreProvider()
	}
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}
	var stdout, stderr bytes.Buffer
	opts.WorkDir, opts.StateDir = repo, state
	opts.Stdout, opts.Stderr, opts.Logger = &stdout, &stderr, log.Discard()
	var dialled int
	var endpoints []model.Endpoint
	opts.NewClient = func(e model.Endpoint) model.Client {
		dialled++
		endpoints = append(endpoints, e)
		if client == nil {
			t.Fatal("the run dialled a provider")
		}
		return client()
	}
	code, err := runScore(opts)
	if err != nil {
		t.Fatalf("runScore: %v", err)
	}
	got := scoreResult{doc: checkDocument(t, "score", stdout.String()), stdout: stdout.String(), stderr: stderr.String(),
		code: code, dialled: dialled, endpoints: endpoints}
	if want := exitCodes[got.doc.Outcome]; got.code != want {
		t.Errorf("outcome %s with exit %d, want %d\n%s", got.doc.Outcome, got.code, want, got.stdout)
	}
	return got
}

// scoring is a scripted provider serving one reply per call, in order.
func scoring(replies ...string) *model.MockClient {
	responses := make([]model.Response, len(replies))
	for i, reply := range replies {
		responses[i] = model.Response{Content: reply}
	}
	return model.NewScriptedMockPerConsult(responses)
}

// cancelOnSecond is a provider that cancels the run's context as the second
// call arrives: the first item's question is answered and lands, and the item
// after it meets a cancelled context, which is where the walk stops.
type cancelOnSecond struct {
	*model.MockClient
	cancel context.CancelFunc
	calls  int
}

func (c *cancelOnSecond) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	c.calls++
	if c.calls == 2 {
		c.cancel()
	}
	return c.MockClient.ConsultStream(ctx, req)
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

// bearingOf is the index's bearing for the edge e names, as the column records
// it: the value's spelling, or the pending literal.
func bearingOf(t *testing.T, repo string, e query.ScoringEdge) string {
	t.Helper()
	src, err := kbload.Open(filepath.Join(repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	row, ok := ix.Edge(e)
	if !ok {
		t.Fatalf("the index holds no %s edge %s → %s", e.Relation, e.Source, e.Target)
	}
	text, _ := row.Bearing.(string)
	return text
}

// scoreFixture is the fixture KB the scoring passes are exercised on: the
// testdata/score-kb copy, refreshed, so its index stands and its entry point is
// stamped current.
func scoreFixture(t *testing.T) string {
	t.Helper()
	repo := fixtureRepo(t, "score-kb")
	if _, err := index.Refresh(kb.OnDisk(filepath.Join(repo, kb.KBDir)), log.Discard()); err != nil {
		t.Fatalf("refresh the fixture: %v", err)
	}
	return repo
}

// TestScoreRunLandsAndResumes: a rigor run asks one item per claim, analysis and
// experiment, and a bearing run one per scoring edge, each landing the reply's
// value through the write API; both founds bearings land, a work's own rigor is
// no item and stays as authored; a --resume of a finished run lands nothing and
// asks nothing; a malformed reply lands nothing for its item and is counted.
func TestScoreRunLandsAndResumes(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()

	// The rigor pass, by id: the claims, the analysis and the experiment.
	rigor := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client {
		return scoring(
			"none",
			"flt-overreach: Theorem 1",
			"flt-underspecified: Every exchange clears",
			"none")
	})
	if rigor.doc.Outcome != "done" || rigor.dialled != 1 {
		t.Fatalf("the rigor run: outcome %s, %d dials\n%s", rigor.doc.Outcome, rigor.dialled, rigor.stdout)
	}
	if want := []string{"outcome", "kb-root", "state-dir", "pass", "roles",
		"run", "ledger", "counts", "scored"}; !slices.Equal(rigor.doc.Keys, want) {
		t.Errorf("keys %q, want %q", rigor.doc.Keys, want)
	}
	if role := roleOf(rigor.doc, "scoring"); rigor.doc.Values["pass"] != query.PassRigor || role == nil ||
		role["model"] != "scoring-model" || role["provider"] != "solo" {
		t.Errorf("roles %v, pass %v; want the scoring role's model and provider and the pass asked for",
			rigor.doc.Values["roles"], rigor.doc.Values["pass"])
	}
	if strings.Contains(rigor.stdout, "text-model") {
		t.Errorf("the run names the text model:\n%s", rigor.stdout)
	}
	if counts, _ := rigor.doc.Values["counts"].(map[string]any); counts["total"] != 4 || counts["done"] != 4 || counts["remaining"] != 0 {
		t.Errorf("counts %v, want the four items done\n%s", rigor.doc.Values["counts"], rigor.stdout)
	}
	rigors := []struct {
		id   string
		want float64
	}{{"ana-aaaaaa", 1.00}, {"clm-aaaaaa", 0.65}, {"clm-bbbbbb", 0.95}, {"exp-aaaaaa", 1.00}}
	if scored := anyStrings(rigor.doc.Values["scored"]); !slices.Equal(scored, []string{"ana-aaaaaa", "clm-aaaaaa", "clm-bbbbbb", "exp-aaaaaa"}) {
		t.Errorf("scored %q, want the four nodes in id order", scored)
	}
	for _, tc := range rigors {
		if got := rigorOf(t, repo, tc.id); got != tc.want {
			t.Errorf("%s's rigor = %v, want the reply's %v", tc.id, got, tc.want)
		}
	}
	// A work is no rigor subject: its value is the one neither pass asks, and
	// the run leaves the authored one standing.
	if got := rigorOf(t, repo, "wrk-aaaaaa"); got != 0.9 {
		t.Errorf("the work's rigor = %v, want the authored 0.9", got)
	}

	// The bearing pass, by relation, then source, then target.
	bearing := scoreIn(t, repo, state, scoreOptions{Pass: query.PassBearing}, func() model.Client {
		return scoring("flt-transfer: The First Result", "none", "none", "none", "none", "none")
	})
	if bearing.doc.Outcome != "done" || bearing.dialled != 1 {
		t.Fatalf("the bearing run: outcome %s, %d dials\n%s", bearing.doc.Outcome, bearing.dialled, bearing.stdout)
	}
	if counts, _ := bearing.doc.Values["counts"].(map[string]any); counts["total"] != 6 || counts["done"] != 6 || counts["remaining"] != 0 {
		t.Errorf("counts %v, want the six edges done\n%s", bearing.doc.Values["counts"], bearing.stdout)
	}
	bearings := []struct {
		edge query.ScoringEdge
		want string
	}{
		{query.ScoringEdge{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationSupports}, "0.65"},
		{query.ScoringEdge{Source: "exp-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationSupports}, "1.0"},
		{query.ScoringEdge{Source: "axm-S1", Target: "ana-aaaaaa", Relation: kb.RelationFounds}, "1.0"},
		{query.ScoringEdge{Source: "wrk-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationFounds}, "1.0"},
		{query.ScoringEdge{Source: "ana-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationLifts}, "1.0"},
		{query.ScoringEdge{Source: "ana-aaaaaa", Target: "clm-aaaaaa", Relation: kb.RelationRefutes}, "1.0"},
	}
	for _, tc := range bearings {
		if got := bearingOf(t, repo, tc.edge); got != tc.want {
			t.Errorf("the %s edge %s → %s bears %q, want the reply's %q", tc.edge.Relation, tc.edge.Source, tc.edge.Target, got, tc.want)
		}
	}
	// The run's records stand beside each other: the run.yaml that names it and
	// the line per attempt. A real run writes no items.jsonl — its lines are the
	// list — which a dry run alone leaves.
	dir, _ := bearing.doc.Values["ledger"].(string)
	for _, name := range []string{"run.yaml", "ledger.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the run left no %s: %v", name, err)
		}
	}
	// Every line names the model asked for, and it is the scoring role's:
	// the text model appears nowhere in the run.
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if !strings.Contains(string(ledger), `"model":"scoring-model"`) || strings.Contains(string(ledger), "text-model") {
		t.Errorf("the ledger's lines do not name the scoring model alone:\n%s", ledger)
	}

	// A resume of a finished run asks nothing: the client errors on any call,
	// and the run reports done with no call made.
	unreachable := model.NewScriptedMockPerConsult(nil)
	unreachable.SetError(errors.New("the run asked a provider it was told is unreachable"))
	unreachable.RecordCalls = true
	run, _ := bearing.doc.Values["run"].(string)
	again := scoreIn(t, repo, state, scoreOptions{Pass: query.PassBearing, Resume: run}, func() model.Client { return unreachable })
	if calls := unreachable.Calls(); len(calls) != 0 {
		t.Errorf("the resumed run asked %d time(s), want none", len(calls))
	}
	if again.doc.Outcome != "done" || again.doc.Values["run"] != run {
		t.Fatalf("the resumed run: outcome %s, run %v\n%s", again.doc.Outcome, again.doc.Values["run"], again.stdout)
	}
	if counts, _ := again.doc.Values["counts"].(map[string]any); counts["done"] != 6 || counts["failed"] != 0 {
		t.Errorf("the resumed run: counts %v, want the six items already done", counts)
	}
	if scored := anyStrings(again.doc.Values["scored"]); len(scored) != 0 {
		t.Errorf("the resumed run landed %q", scored)
	}
	if _, resumes := again.doc.Values["resume"]; resumes {
		t.Errorf("a finished run carries resume:\n%s", again.stdout)
	}
	if again.doc.Values["ledger"] != bearing.doc.Values["ledger"] {
		t.Errorf("the resumed run's ledger %v, want the run's own %v", again.doc.Values["ledger"], bearing.doc.Values["ledger"])
	}

	// A reply the parse refuses lands nothing for its item, fails it, and the
	// run goes on: the other three land as they would, and the closing record
	// names the failed item with the reason its line recorded.
	partial := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client {
		return scoring("none", "about half", "none", "none")
	})
	if partial.doc.Outcome != "failed" {
		t.Fatalf("the run with a malformed reply: outcome %s\n%s", partial.doc.Outcome, partial.stdout)
	}
	counts, _ := partial.doc.Values["counts"].(map[string]any)
	if counts["total"] != 4 || counts["done"] != 3 || counts["failed"] != 1 {
		t.Errorf("counts %v, want three done and one failed\n%s", partial.doc.Values["counts"], partial.stdout)
	}
	if scored := anyStrings(partial.doc.Values["scored"]); slices.Contains(scored, "clm-aaaaaa") {
		t.Errorf("the refused item's value landed anyway: %q", scored)
	}
	if got := rigorOf(t, repo, "clm-aaaaaa"); got != 0.65 {
		t.Errorf("the refused item's rigor = %v, want the value the last run landed", got)
	}
	failures := partial.doc.items("failures")
	if len(failures) != 1 || failures[0]["check"] != checkError {
		t.Fatalf("failures %v, want the one refused item\n%s", failures, partial.stdout)
	}
	if detail, _ := failures[0]["detail"].(string); !strings.HasPrefix(detail, "clm-aaaaaa: ") ||
		!strings.Contains(detail, "not a charge line") {
		t.Errorf("the failure %q, want the item and the reason its line recorded", detail)
	}
}

// TestScoreResumeAppliesAnUpdate: a resume after the software changed continues
// rather than refusing. The value the running table moves is re-derived from the
// ledger's ids and lands through the write a landing uses, so the KB's stored
// value and its published index both move; the run's directory records from what
// to what; the header keeps the ask the run's own judgements answered; and
// nothing asks the model.
func TestScoreResumeAppliesAnUpdate(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	first := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client {
		return scoring("flt-overreach: The claim's own paragraph.")
	})
	if first.doc.Outcome != "done" {
		t.Fatalf("the run: outcome %s\n%s", first.doc.Outcome, first.stdout)
	}
	if got := rigorOf(t, repo, "clm-aaaaaa"); got != 0.65 {
		t.Fatalf("the claim's rigor = %v, want the reply's 0.65", got)
	}
	run, _ := first.doc.Values["run"].(string)
	dir, _ := first.doc.Values["ledger"].(string)

	// What a run written under the earlier table and ask left behind: the KB
	// holds the value that table landed, the line records it, and the header
	// names the instruction the run's judgements answered.
	claim := query.Item{Node: &query.ScoringNode{Kind: kb.NodeKindClaim, ID: "clm-aaaaaa"}}
	if err := landScore(filepath.Join(repo, kb.KBDir), claim, kb.Bearing{Value: 0.3}, log.Discard()); err != nil {
		t.Fatal(err)
	}
	if _, err := index.Refresh(kb.OnDisk(filepath.Join(repo, kb.KBDir)), log.Discard()); err != nil {
		t.Fatal(err)
	}
	if got := rigorOf(t, repo, "clm-aaaaaa"); got != 0.3 {
		t.Fatalf("the claim's rigor = %v, want the value the earlier table landed", got)
	}
	rewriteLine(t, filepath.Join(dir, "ledger.jsonl"), func(rec map[string]any) { rec["value"] = "0.30" })
	header := moveTheAsk(t, filepath.Join(dir, "run.yaml"))

	unreachable := model.NewScriptedMockPerConsult(nil)
	unreachable.SetError(errors.New("the resumed run asked a provider"))
	unreachable.RecordCalls = true
	again := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Resume: run}, func() model.Client { return unreachable })
	if again.doc.Outcome != "done" || again.doc.Values["run"] != run {
		t.Fatalf("the resumed run: outcome %s, run %v\n%s", again.doc.Outcome, again.doc.Values["run"], again.stdout)
	}
	if calls := unreachable.Calls(); len(calls) != 0 {
		t.Errorf("the resumed run asked %d time(s), want none: a re-price re-derives from the ids", len(calls))
	}
	if got := rigorOf(t, repo, "clm-aaaaaa"); got != 0.65 {
		t.Errorf("the claim's rigor = %v, want the running table's 0.65: the moved value landed", got)
	}
	rec := lineRecord(t, filepath.Join(dir, "ledger.jsonl"))
	if rec["value"] != "0.65" {
		t.Errorf("the ledger's value = %v, want the value the table gives its ids", rec["value"])
	}
	record, err := os.ReadFile(filepath.Join(dir, "repriced.yaml"))
	if err != nil {
		t.Fatalf("the re-price left no record: %v", err)
	}
	// The record names the item as the result does — one spelling for both, so a
	// repriced.yaml line joins to the run's scored without a translation.
	scored := anyStrings(first.doc.Values["scored"])
	if len(scored) != 1 {
		t.Fatalf("the first run scored %q, want the one item it landed", scored)
	}
	for _, want := range []string{"item: " + scored[0], `from: "0.30"`, `to: "0.65"`} {
		if !strings.Contains(string(record), want) {
			t.Errorf("the record does not carry %q:\n%s", want, record)
		}
	}
	if got, err := os.ReadFile(filepath.Join(dir, "run.yaml")); err != nil || string(got) != header {
		t.Errorf("the resume rewrote run.yaml:\n%s\n---\n%s", header, got)
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

// rewriteLine rewrites a run's ledger line through f, which is how a test states
// what a run under an earlier table recorded.
func rewriteLine(t *testing.T, path string, f func(map[string]any)) {
	t.Helper()
	rec := lineRecord(t, path)
	f(rec)
	line, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o666); err != nil {
		t.Fatal(err)
	}
}

// moveTheAsk replaces the digest run.yaml records with another, which is the
// header an update that moved the ask leaves behind, and returns the file's
// bytes as it wrote them.
func moveTheAsk(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if strings.HasPrefix(line, "ask_sha256: ") {
			line = "ask_sha256: " + strings.Repeat("0", 64)
		}
		out = append(out, line)
	}
	written := strings.Join(out, "\n") + "\n"
	if err := os.WriteFile(path, []byte(written), 0o666); err != nil {
		t.Fatal(err)
	}
	return written
}

// TestScoreCancelledClosesOnTheItemItStoppedBefore: a run stopped by a cancelled
// context closes its document on the cancelled record, naming the set's first
// item the run recorded no attempt for — the one it stopped before — and keeps
// resume before that record. The document passes the contract's own key check,
// which scoreIn ran.
func TestScoreCancelledClosesOnTheItemItStoppedBefore(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Ctx: ctx}, func() model.Client {
		return &cancelOnSecond{MockClient: scoring("none"), cancel: cancel}
	})
	if got.doc.Outcome != "cancelled" || got.code != exitCodes["cancelled"] {
		t.Fatalf("the cancelled run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	stopped, _ := got.doc.Values["cancelled"].(map[string]any)
	if stopped["item"] != "clm-aaaaaa" {
		t.Errorf("cancelled %v, want the claim the run stopped before\n%s", stopped, got.stdout)
	}
	if keys := got.doc.Keys; keys[len(keys)-2] != "resume" {
		t.Errorf("keys %q, want resume before the record the outcome closes on", keys)
	}
	if counts, _ := got.doc.Values["counts"].(map[string]any); counts["total"] != 4 || counts["done"] != 1 || counts["remaining"] != 3 {
		t.Errorf("counts %v, want the one item answered and the three remaining\n%s", got.doc.Values["counts"], got.stdout)
	}
	if scored := anyStrings(got.doc.Values["scored"]); !slices.Equal(scored, []string{"ana-aaaaaa"}) {
		t.Errorf("scored %q, want the one item that landed", scored)
	}
}

// TestScoreStopAfterAPartialWalkCarriesResume: a run that stops inside its end
// refresh — here with the index it must publish left unwritable — still reports
// the command that continues it, ahead of the record its outcome closes on, and
// the failure the document names is the refresh's, not an item's.
func TestScoreStopAfterAPartialWalkCarriesResume(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	// The index stands current, so the start refresh has no changed file to
	// write; the one item --limit leaves the walk at changes it, and that change
	// cannot be published.
	index := filepath.Join(repo, kb.KBDir, kb.IndexDir)
	if err := os.Chmod(index, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(index, 0o755) })
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Limit: 1}, func() model.Client { return scoring("none") })
	if got.doc.Outcome != "failed" || got.code != exitCodes["failed"] {
		t.Fatalf("the run whose refresh could not publish: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	keys := got.doc.Keys
	if keys[len(keys)-1] != "failures" || keys[len(keys)-2] != "resume" {
		t.Fatalf("keys %q, want resume before the record the outcome closes on", keys)
	}
	run, _ := got.doc.Values["run"].(string)
	if resumed, _ := got.doc.Values["resume"].(string); !strings.HasPrefix(resumed, "kbase score --pass "+query.PassRigor+" --resume "+run) {
		t.Errorf("resume %q, want the command continuing run %s\n%s", resumed, run, got.stdout)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["total"] != 4 || counts["done"] != 1 || counts["remaining"] != 3 || counts["failed"] != 0 {
		t.Errorf("counts %v, want one item landed, three remaining and no item failed\n%s", counts, got.stdout)
	}
	if items := got.doc.items("failures"); len(items) != 1 || items[0]["check"] != checkError {
		t.Errorf("failures %v, want the refresh's own failure\n%s", items, got.stdout)
	}
}

// TestScoreFailedRunClosesOnFailures: a run whose item cannot be written ends
// `failed` and closes its document on the failures record — one item naming the
// item and the reason the ledger recorded for it — not on nothing.
func TestScoreFailedRunClosesOnFailures(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	// The KB's register directory refuses every new file, which is where a
	// landing writes; the index stands current from scoreRepo's refresh, so the
	// run's start refresh has no changed file to write and the walk reaches its
	// one item.
	dir := filepath.Join(repo, kb.KBDir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none") })
	if got.doc.Outcome != "failed" || got.code != exitCodes["failed"] {
		t.Fatalf("the unwritable KB: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if keys := got.doc.Keys; keys[len(keys)-1] != "failures" {
		t.Errorf("keys %q, want failures last", keys)
	}
	items := got.doc.items("failures")
	if len(items) != 1 {
		t.Fatalf("failures %v, want one item naming the claim\n%s", items, got.stdout)
	}
	detail, _ := items[0]["detail"].(string)
	if items[0]["check"] != checkError || !strings.HasPrefix(detail, "clm-aaaaaa: ") || len(detail) == len("clm-aaaaaa: ") {
		t.Errorf("failures %v, want check %s, the item's name and the reason its line recorded\n%s", items, checkError, got.stdout)
	}
	if v := rigorOf(t, repo, "clm-aaaaaa"); v != 0.5 {
		t.Errorf("the claim's rigor = %v, want the authored 0.5: nothing landed", v)
	}
}

// TestScoreDryRunIsInert: a dry run reports done with every item remaining,
// dials no provider, lands no value, resolves no role — no roles record, which
// is where the four provider keys were — and carries no resume, leaving the
// run's item list where the ledger's lines would go.
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
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["total"] != 1 || counts["done"] != 0 || counts["remaining"] != 1 {
		t.Errorf("counts %v, want one item asked about and none done", got.doc.Values["counts"])
	}
	if scored := anyStrings(got.doc.Values["scored"]); len(scored) != 0 {
		t.Errorf("a dry run landed %q", scored)
	}
	if v := rigorOf(t, repo, "clm-aaaaaa"); v != 0.5 {
		t.Errorf("a dry run changed the claim's rigor to %v", v)
	}
	dir, _ := got.doc.Values["ledger"].(string)
	for _, name := range []string{"run.yaml", "items.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the dry run left no %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "ledger.jsonl")); err == nil {
		t.Error("a dry run wrote a ledger line: it asked nothing")
	}
	// No back end answered it, so its header names none: the key is absent rather
	// than holding a value that is none of the three a run records.
	header, err := os.ReadFile(filepath.Join(dir, "run.yaml"))
	if err != nil {
		t.Fatalf("read the dry run's header: %v", err)
	}
	if strings.Contains(string(header), "backend") {
		t.Errorf("a dry run's header names a back end:\n%s", header)
	}
}

// TestScoreRefusesADryRunOnAnOlderKB: a dry run writes nothing, so the refresh
// that would convert the fixture's 1.0.0 KB does not run: it refuses the index
// the load cannot answer, naming kbase refresh, and leaves the KB as it stands.
func TestScoreRefusesADryRunOnAnOlderKB(t *testing.T) {
	repo, state := fixtureRepo(t, "score-kb-1.0.0"), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, DryRun: true}, nil)
	if got.doc.Outcome != "refused" || got.code != exitCodes["refused"] {
		t.Fatalf("the dry run over 1.0.0: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items := got.doc.items("refusals")
	if len(items) == 0 || items[0]["check"] != "index" || items[0]["remedy"] != "kbase refresh" {
		t.Errorf("refusals %v, want the index one naming kbase refresh\n%s", items, got.stdout)
	}
	if got.doc.Keys[1] != "kb-root" || got.doc.Keys[2] != "migration" {
		t.Errorf("keys %q, want the migration record after kb-root", got.doc.Keys)
	}
	if format := formatOf(t, repo); format != "1.0.0" {
		t.Errorf("the KB stands at %s, want the dry run to have written nothing", format)
	}
}

// TestScoreSaysTheKeyTravelsInCleartext: a run whose endpoint is http:// to a
// host other than loopback, with a key configured, says so in one line on stderr
// and proceeds — the same line `build` writes, naming the host and never the key.
// A loopback endpoint says nothing.
func TestScoreSaysTheKeyTravelsInCleartext(t *testing.T) {
	unsetInferenceEnv(t)
	for _, tc := range []struct {
		name, url string
		warn      bool
	}{
		{"a plain-http host", "http://lab.example/v1", true},
		{"loopback", "http://127.0.0.1:8080/v1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
				if got.stderr != "" {
					t.Errorf("stderr = %q, want nothing for %s", got.stderr, tc.url)
				}
				return
			}
			line := config.CleartextWarning(tc.url, testAPIKey)
			if line == "" || strings.Count(got.stderr, line) != 1 {
				t.Errorf("stderr = %q, want one line %q", got.stderr, line)
			}
			if strings.Contains(got.stderr, testAPIKey) {
				t.Errorf("stderr names the key: %q", got.stderr)
			}
		})
	}
}

// TestScoreResumeRefusesAPath: --resume names a run id kbase minted, never a
// path — the value is joined into the state store, and a directory under it is
// removed recursively — so a value of any other shape is a usage refusal naming
// the flag, and nothing is opened. The shape is MintID's own: what it mints,
// --resume takes.
func TestScoreResumeRefusesAPath(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	for _, resume := range []string{"../../..", "run-aaaaaa/../../..", "run-1", "..", "/", "run-AAAAAA", "run-aaaaaaa", " run-aaaaaa"} {
		got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Resume: resume}, nil)
		items := got.doc.items("refusals")
		if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkUsage || items[0]["key"] != "--resume" {
			t.Errorf("--resume %q = %s, %v; want one usage refusal naming --resume\n%s",
				resume, got.doc.Outcome, items, got.stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(state, "score")); !os.IsNotExist(err) {
		t.Errorf("a refused --resume left a score directory: %v", err)
	}
	for range 100 {
		if id := kb.MintID("run", func(string) bool { return false }); !runIDRE.MatchString(id) {
			t.Fatalf("MintID minted %q, which --resume refuses", id)
		}
	}
}

// TestScoreRefusals: a pass no document names is a usage refusal, naming both
// passes the flag takes; a run that meets the lock another run holds is retry,
// naming the directory held rather than a file that is not there.
func TestScoreRefusals(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: "other"}, nil)
	if got.doc.Outcome != "refused" || got.code != exitCodes["refused"] {
		t.Fatalf("--pass other: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items := got.doc.items("refusals")
	if len(items) != 1 || items[0]["check"] != checkUsage || items[0]["key"] != "--pass" ||
		!slices.Equal(anyStrings(items[0]["allowed"]), []string{query.PassRigor, query.PassBearing}) {
		t.Errorf("refusals %v, want one usage refusal naming both passes\n%s", items, got.stdout)
	}

	runs := filepath.Join(state, "score")
	if err := os.MkdirAll(runs, 0o777); err != nil {
		t.Fatal(err)
	}
	held, err := filelock.Acquire(runs, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	got = scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, nil)
	if got.doc.Outcome != "retry" || got.code != exitCodes["retry"] {
		t.Fatalf("a run behind a held lock: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items = got.doc.items("refusals")
	if len(items) != 1 || items[0]["check"] != checkLock || items[0]["path"] != runs {
		t.Errorf("refusals %v, want one lock refusal naming %s\n%s", items, runs, got.stdout)
	}
}

// TestScoreMigratesAnOlderKB: a real run over the fixture's 1.0.0 KB is done —
// the start refresh is the write that saves the conversion — a value lands, the
// KB is current afterwards with exactly one migration record after kb-root, and
// the next run carries none.
func TestScoreMigratesAnOlderKB(t *testing.T) {
	repo, state := fixtureRepo(t, "score-kb-1.0.0"), t.TempDir()
	if format := formatOf(t, repo); format != "1.0.0" {
		t.Fatalf("the fixture stands at %s, want the 1.0.0 KB it is", format)
	}
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none") })
	if got.doc.Outcome != "done" {
		t.Fatalf("the run over 1.0.0: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if got.doc.Keys[1] != "kb-root" || got.doc.Keys[2] != "migration" {
		t.Fatalf("keys %q, want one migration record after kb-root", got.doc.Keys)
	}
	if m, _ := got.doc.Values["migration"].(map[string]any); m["from"] != "1.0.0" || m["to"] != kb.FormatVersion {
		t.Errorf("migration %v, want 1.0.0 → %s", m, kb.FormatVersion)
	}
	if format := formatOf(t, repo); format != kb.FormatVersion {
		t.Errorf("the KB stands at %s after a done run, want %s", format, kb.FormatVersion)
	}
	if v := rigorOf(t, repo, "clm-aaaaaa"); v != 1.0 {
		t.Errorf("the claim's rigor = %v, want the maximum the reply `none` lands", v)
	}
	again := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none") })
	if again.doc.Outcome != "done" {
		t.Fatalf("the second run: outcome %s\n%s", again.doc.Outcome, again.stdout)
	}
	if _, migrated := again.doc.Values["migration"]; migrated {
		t.Errorf("a run over a current KB carries a migration record:\n%s", again.stdout)
	}
}
