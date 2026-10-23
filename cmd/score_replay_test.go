//go:build !kbasedist

package main

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// replayedReplies are the replies a run over the score fixture's rigor set is
// answered with, in its order — ana-aaaaaa, clm-aaaaaa, clm-bbbbbb, exp-aaaaaa:
// the second is no answer, so the run records one failed item.
var replayedReplies = []string{"none", "about half", "flt-underspecified: 2", "none"}

// replayedRun is a rigor run over the score fixture answered with
// replayedReplies: the repository, the state store and the run's id.
func replayedRun(t *testing.T) (repo, state, run string) {
	t.Helper()
	repo, state = scoreFixture(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring(replayedReplies...) })
	run, _ = got.doc.Values["run"].(string)
	if got.doc.Outcome != toolresult.Failed || run == "" {
		t.Fatalf("the run: outcome %s, run %q; want failed on its one malformed reply\n%s", got.doc.Outcome, run, got.stdout)
	}
	return repo, state, run
}

// replayIn drives one replay over repo with its state under state, its client
// from client. The flags it was given are --replay, and --failed and --limit
// where opts sets them.
func replayIn(t *testing.T, repo, state string, opts scoreReplayOptions, client func() model.Client) (resultDoc, string) {
	t.Helper()
	opts.Given = []string{"replay"}
	if opts.Failed {
		opts.Given = append(opts.Given, "failed")
	}
	if opts.Limit != 0 {
		opts.Given = append(opts.Given, "limit")
	}
	opts.providerOptions = scoreProvider()
	var stdout, stderr bytes.Buffer
	opts.WorkDir, opts.StateDir = repo, state
	opts.Stdout, opts.Stderr, opts.Logger = &stdout, &stderr, log.Discard()
	opts.NewClient = func(model.Endpoint) model.Client { return client() }
	code, err := runScoreReplay(opts)
	if err != nil {
		t.Fatalf("runScoreReplay: %v", err)
	}
	doc := parseResult(t, stdout.String())
	if want := exitCodes[doc.Outcome]; code != want {
		t.Errorf("outcome %s with exit %d, want %d\n%s", doc.Outcome, code, want, stdout.String())
	}
	return doc, stdout.String()
}

// comparisons is the comparison.jsonl of the replay doc reports.
func comparisons(t *testing.T, doc resultDoc) []map[string]any {
	t.Helper()
	dir, _ := doc.Values["dir"].(string)
	return jsonLines(t, filepath.Join(dir, "comparison.jsonl"))
}

// comparedItems is the items a replay's comparison names, in its order.
func comparedItems(t *testing.T, doc resultDoc) []string {
	t.Helper()
	var out []string
	for _, c := range comparisons(t, doc) {
		item, _ := c["item"].(string)
		out = append(out, item)
	}
	return out
}

// TestScoreReplayReproducesTheRun: a replay over an endpoint answering the run's
// own replies reproduces each item's status and value, renders every prompt the
// run's captures record, and reports nothing changed.
func TestScoreReplayReproducesTheRun(t *testing.T) {
	repo, state, run := replayedRun(t)
	doc, stdout := replayIn(t, repo, state, scoreReplayOptions{Run: run}, func() model.Client { return scoring(replayedReplies...) })
	if doc.Outcome != toolresult.Done {
		t.Fatalf("the replay: outcome %s, want done with its failure counted\n%s", doc.Outcome, stdout)
	}
	counts, _ := doc.Values["counts"].(map[string]any)
	if counts["total"] != 4 || counts["done"] != 3 || counts["failed"] != 1 || doc.Values["changed"] != 0 {
		t.Errorf("counts %v, changed %v; want four items, three done, one failed and none changed\n%s", counts, doc.Values["changed"], stdout)
	}
	compared := comparisons(t, doc)
	if len(compared) != 4 {
		t.Fatalf("the comparison holds %d lines, want one per item", len(compared))
	}
	for _, c := range compared {
		original, _ := c["original"].(map[string]any)
		replay, _ := c["replay"].(map[string]any)
		if original["status"] != replay["status"] || original["value"] != replay["value"] ||
			!slices.Equal(anyStrings(original["faults"]), anyStrings(replay["faults"])) || c["prompt_changed"] != false {
			t.Errorf("%v: the replay does not reproduce the run", c)
		}
	}
	if items := comparedItems(t, doc); !slices.Equal(items, []string{"ana-aaaaaa", "clm-aaaaaa", "clm-bbbbbb", "exp-aaaaaa"}) {
		t.Errorf("the comparison names %q, want the run's items in its order", items)
	}
}

// TestScoreReplayDocumentKeys: the replay's result document carries its keys in
// their order, the closing record its outcome names last.
func TestScoreReplayDocumentKeys(t *testing.T) {
	repo, state, run := replayedRun(t)
	doc, _ := replayIn(t, repo, state, scoreReplayOptions{Run: run}, func() model.Client { return scoring(replayedReplies...) })
	if want := []string{"outcome", "kb-root", "state-dir", "run", "replay", "dir", "counts", "changed"}; !slices.Equal(doc.Keys, want) {
		t.Errorf("keys %q, want %q", doc.Keys, want)
	}
	if got := mappingKeys(doc.Nodes["counts"]); !slices.Equal(got, []string{"total", "done", "failed"}) {
		t.Errorf("counts keys %q, want total, done, failed", got)
	}
	dir, _ := doc.Values["dir"].(string)
	if replay, _ := doc.Values["replay"].(string); !filepath.IsAbs(dir) || filepath.Base(dir) != replay ||
		filepath.Dir(dir) != filepath.Join(state, "score", run, "replays") {
		t.Errorf("dir %q, replay %v; want the replay's own directory under the run's replays/, absolute", dir, doc.Values["replay"])
	}
}

// TestScoreReplayWritesOnlyItsDirectory: a replay leaves kb-root/ and every file
// of the store byte-identical, the run's own records included, and writes
// its four files under replays/<id>/ and nothing else.
func TestScoreReplayWritesOnlyItsDirectory(t *testing.T) {
	repo, state, run := replayedRun(t)
	repoBefore, stateBefore := tree(t, repo), tree(t, state)
	doc, stdout := replayIn(t, repo, state, scoreReplayOptions{Run: run}, func() model.Client { return scoring(replayedReplies...) })
	if doc.Outcome != toolresult.Done {
		t.Fatalf("the replay: outcome %s\n%s", doc.Outcome, stdout)
	}
	repoAfter := tree(t, repo)
	if len(repoAfter) != len(repoBefore) {
		t.Errorf("the repository held %d files and holds %d", len(repoBefore), len(repoAfter))
	}
	for rel, before := range repoBefore {
		if repoAfter[rel] != before {
			t.Errorf("the replay changed %s in the repository", rel)
		}
	}
	replay, _ := doc.Values["replay"].(string)
	own := path.Join("score", run, "replays", replay)
	var written []string
	for rel, after := range tree(t, state) {
		before, stood := stateBefore[rel]
		switch {
		case stood && before != after:
			t.Errorf("the replay changed %s in the store", rel)
		case !stood && path.Dir(rel) != own:
			t.Errorf("the replay wrote %s, outside its directory %s", rel, own)
		case !stood:
			written = append(written, path.Base(rel))
		}
	}
	slices.Sort(written)
	if want := []string{"captures.jsonl", "comparison.jsonl", "ledger.jsonl", "replay.yaml"}; !slices.Equal(written, want) {
		t.Errorf("the replay wrote %q, want %q", written, want)
	}
}

// TestScoreReplayFailedSelectsTheFailedItems: --failed re-asks only the items
// the run's ledger last records failed.
func TestScoreReplayFailedSelectsTheFailedItems(t *testing.T) {
	repo, state, run := replayedRun(t)
	doc, stdout := replayIn(t, repo, state, scoreReplayOptions{Run: run, Failed: true}, func() model.Client { return scoring("about half") })
	if doc.Outcome != toolresult.Done {
		t.Fatalf("the replay: outcome %s\n%s", doc.Outcome, stdout)
	}
	if items := comparedItems(t, doc); !slices.Equal(items, []string{"clm-aaaaaa"}) {
		t.Errorf("--failed re-asked %q, want the one failed item", items)
	}
}

// TestScoreReplayLimitCutsInItemOrder: --limit takes the first items of the
// run's item list.
func TestScoreReplayLimitCutsInItemOrder(t *testing.T) {
	repo, state, run := replayedRun(t)
	doc, stdout := replayIn(t, repo, state, scoreReplayOptions{Run: run, scoreOptions: scoreOptions{Limit: 2}},
		func() model.Client { return scoring(replayedReplies[:2]...) })
	if doc.Outcome != toolresult.Done {
		t.Fatalf("the replay: outcome %s\n%s", doc.Outcome, stdout)
	}
	if items := comparedItems(t, doc); !slices.Equal(items, []string{"ana-aaaaaa", "clm-aaaaaa"}) {
		t.Errorf("--limit 2 re-asked %q, want the run's first two items", items)
	}
}

// TestScoreReplayRefusals: --replay takes a run's id and no path; a run with no
// captures — one pruned, a dry run, one that asked nothing — is refused naming
// --replay; a run's own flag beside --replay is refused naming --replay, and
// --failed without --replay naming --failed. The accepted flags pass.
func TestScoreReplayRefusals(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	stood := standRuns(t, state, "run", 16, 0)
	if got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor}, func() model.Client { return scoring("none") }); got.doc.Outcome != toolresult.Done {
		t.Fatalf("the run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if _, err := os.Stat(filepath.Join(state, "score", stood[0])); !os.IsNotExist(err) {
		t.Fatalf("the run did not prune %s: %v", stood[0], err)
	}
	dry := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, DryRun: true}, nil)
	dryRun, _ := dry.doc.Values["run"].(string)

	for _, tc := range []struct {
		name, key, detail string
		args              []string
	}{
		{"a path", "--replay", "not a path", []string{"--replay", "../../run-aaaaaa"}},
		{"a malformed id", "--replay", "not a path", []string{"--replay", "run-1"}},
		{"a pruned run", "--replay", "captures.jsonl", []string{"--replay", stood[0]}},
		{"a dry run", "--replay", "captures.jsonl", []string{"--replay", dryRun}},
		{"a run that asked nothing, beside every flag a replay takes", "--replay", "captures.jsonl",
			[]string{"--replay", stood[1], "--failed", "--limit", "1", "--timeout", "5s", "--scoring-model", "solo/other"}},
		{"--dry-run beside it", "--replay", "--dry-run is a run's", []string{"--replay", stood[1], "--dry-run"}},
		{"--pass beside it", "--replay", "--pass is a run's", []string{"--replay", stood[1], "--pass", "rigor"}},
		{"--resume beside it", "--replay", "--resume is a run's", []string{"--replay", stood[1], "--resume", stood[1]}},
		{"--failed alone", "--failed", "with --replay", []string{"--failed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := kbase(t, repo, append([]string{"score", "--state-dir", state}, tc.args...)...)
			doc := parseResult(t, out)
			items := doc.items(toolresult.RefusalsKey)
			if doc.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] || len(items) != 1 ||
				items[0]["check"] != checkUsage || items[0]["key"] != tc.key {
				t.Fatalf("%q: outcome %s, exit %d, refusals %v; want one usage refusal naming %s\n%s", tc.args, doc.Outcome, code, items, tc.key, out)
			}
			if detail, _ := items[0]["detail"].(string); !strings.Contains(detail, tc.detail) {
				t.Errorf("%q: detail %q, want it to name %q", tc.args, detail, tc.detail)
			}
		})
	}
}
