//go:build kbasedev

package main

import (
	"bytes"
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/asks/call"
	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	toolresult "kbase/internal/result"
)

// replayStart is the fixture trail's start commit.
var replayStart = strings.Repeat("ab", 20)

// replayGroup and replayItem are the fixture trail's one paragraph group and
// its one item.
const replayGroup, replayItem = "leaf", "1-2"

// replayFixtureClient answers every multichoice ask the same way each time it
// is asked — a choice question with its first offered letter, a noul question
// with 0.9 — and every overview call with one passage.
func replayFixtureClient() *model.ClientFuncs {
	return &model.ClientFuncs{
		Stream: func(ctx context.Context, req model.Request) (model.StreamReader, error) {
			return modeltest.NewScriptedMock([]model.Response{{Content: "A small paper.", FinishReason: "stop"}}, nil).ConsultStream(ctx, req)
		},
		Decide: func(_ context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
			q := req.Questions[0]
			first := slices.Min(slices.Collect(maps.Keys(q.Criteria)))
			return model.DecisionResponse{Answers: []model.DecisionAnswer{{Key: q.Key, Type: q.Type, Noul: 0.9, Choice: first, Confidence: 0.9}}}, nil
		},
	}
}

// writeReplayTrail writes a trail under the store at state as a build's asks
// write one, through call.New over replayFixtureClient: one paragraph group of
// one item and one overview call. It returns the trail directory and its row
// count. It stands beside internal/build's writeTrail because package main
// cannot reach that package's test helpers.
func writeReplayTrail(t *testing.T, state string) (string, int) {
	t.Helper()
	trail := filepath.Join(state, buildstore.ScratchDir, buildstore.TrailsDir, replayStart)
	caller := call.New(replayFixtureClient(), func() string { return trail }, log.Discard())
	queue := model.NewQueue(1)
	ctx := context.Background()
	batch := asks.NewBatch(ctx, caller.Launch(queue, "decision"))
	paragraphs := asks.ParagraphAsks("leaf.md", "The leaf.", []asks.ParagraphItem{
		{Paragraph: replayItem, Text: "Every bounded sequence has a convergent subsequence."}})
	if err := batch.AskGroup(asks.Paragraph, replayGroup, paragraphs); err != nil {
		t.Fatal(err)
	}
	recs, err := batch.Wait()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range asks.Kinds {
		var of []asks.GroupRecord
		for _, r := range recs {
			if r.Kind == k {
				of = append(of, r)
			}
		}
		if len(of) == 0 {
			continue
		}
		if err := asks.WriteGroupRecords(filepath.Join(trail, asks.AskRecordsDir), of); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	if err := caller.PassageLaunch(queue, "text")(ctx, &wg, func(ask func(context.Context, string, []string, int) (string, error)) func(error) {
		if _, err := ask(ctx, "The excerpts.", nil, 1); err != nil {
			t.Error(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	return trail, 2
}

// buildReplayIn drives one build replay from workDir over the store stateDir
// names, every role answered by replayFixtureClient.
func buildReplayIn(t *testing.T, workDir, stateDir string) (resultDoc, string) {
	t.Helper()
	return buildReplayWith(t, context.Background(), replayFixtureClient(), workDir, stateDir)
}

// buildReplayWith drives one build replay under ctx from workDir over the
// store stateDir names, every role answered by client.
func buildReplayWith(t *testing.T, ctx context.Context, client model.Client, workDir, stateDir string) (resultDoc, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	opts := buildReplayOptions{Replay: true, Given: []string{"replay", "state-dir"}}
	opts.WorkDir, opts.StateDir = workDir, stateDir
	opts.Stdout, opts.Stderr, opts.Logger = &stdout, &stderr, log.Discard()
	opts.AskConcurrency = new(1)
	opts.NewClient = func(model.Endpoint) model.Client { return client }
	opts.Provider = buildProvider(scoreProvider(), &stderr)
	code, err := runBuildReplay(ctx, opts)
	if err != nil {
		t.Fatalf("runBuildReplay: %v", err)
	}
	doc := parseResult(t, stdout.String())
	if want := toolresult.ExitCode(doc.Outcome); code != want {
		t.Errorf("outcome %s with exit %d, want %d\n%s", doc.Outcome, code, want, stdout.String())
	}
	return doc, stdout.String()
}

// TestBuildReplayVerb: a replay of an unchanged trail through the verb asks
// every recorded row, reproduces each, and reports its document's keys with
// its directory under the trail's.
func TestBuildReplayVerb(t *testing.T) {
	state := t.TempDir()
	trail, rows := writeReplayTrail(t, state)
	doc, stdout := buildReplayIn(t, t.TempDir(), state)
	if doc.Outcome != toolresult.Done {
		t.Fatalf("the replay: outcome %s, want done\n%s", doc.Outcome, stdout)
	}
	if want := []string{"outcome", "state-dir", "trail", "replay", "dir", "counts", "changed"}; !slices.Equal(doc.Keys, want) {
		t.Errorf("keys %q, want %q", doc.Keys, want)
	}
	if got := mappingKeys(doc.Nodes["counts"]); !slices.Equal(got, []string{"total", "done", "failed"}) {
		t.Errorf("counts keys %q, want total, done, failed", got)
	}
	counts, _ := doc.Values["counts"].(map[string]any)
	if counts["total"] != rows || counts["done"] != rows || counts["failed"] != 0 || doc.Values["changed"] != 0 {
		t.Errorf("counts %v, changed %v; want %d rows all done and none changed\n%s", counts, doc.Values["changed"], rows, stdout)
	}
	if doc.Values["state-dir"] != state || doc.Values["trail"] != replayStart {
		t.Errorf("state-dir %v, trail %v; want %s and %s", doc.Values["state-dir"], doc.Values["trail"], state, replayStart)
	}
	dir, _ := doc.Values["dir"].(string)
	if replay, _ := doc.Values["replay"].(string); filepath.Base(dir) != replay || filepath.Dir(filepath.Dir(dir)) != trail {
		t.Errorf("dir %q, replay %v; want the replay's own directory under the trail's replays", dir, doc.Values["replay"])
	}
}

// TestBuildReplayCancelledDocument: a replay whose context ends during its
// first call leaves the trail's one paragraph item unasked, ends cancelled,
// and names that item in its cancelled record.
func TestBuildReplayCancelledDocument(t *testing.T) {
	state := t.TempDir()
	writeReplayTrail(t, state)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := replayFixtureClient()
	decide := client.Decide
	var once sync.Once
	client.Decide = func(c context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
		once.Do(cancel)
		return decide(c, req)
	}
	doc, stdout := buildReplayWith(t, ctx, client, t.TempDir(), state)
	stopped, _ := doc.Values[toolresult.CancelledKey].(map[string]any)
	if doc.Outcome != toolresult.Cancelled || stopped["kind"] != string(asks.Paragraph) || stopped["group"] != replayGroup || stopped["item"] != replayItem {
		t.Fatalf("the replay: outcome %s, cancelled %v; want cancelled before %s %s %s\n%s", doc.Outcome, stopped, asks.Paragraph, replayGroup, replayItem, stdout)
	}
}

// TestBuildReplayNeedsNoSourceTree: from a directory holding no repository,
// a relative --state-dir resolves against that directory and the replay
// completes.
func TestBuildReplayNeedsNoSourceTree(t *testing.T) {
	work := t.TempDir()
	state := filepath.Join(work, "store")
	writeReplayTrail(t, state)
	doc, stdout := buildReplayIn(t, work, "store")
	if doc.Outcome != toolresult.Done || doc.Values["state-dir"] != state {
		t.Fatalf("the replay: outcome %s, state-dir %v; want done over %s\n%s", doc.Outcome, doc.Values["state-dir"], state, stdout)
	}
}

// TestBuildReplayRefusals: each usage refusal of build --replay, naming its
// key; the flags a replay takes pass to the trail's selection.
func TestBuildReplayRefusals(t *testing.T) {
	empty := t.TempDir()
	for _, tc := range []struct {
		name, key string
		args      []string
	}{
		{"--kind without --replay", "--kind", []string{"--kind", string(asks.Paragraph)}},
		{"--failed without --replay", "--failed", []string{"--failed"}},
		{"--limit without --replay", "--limit", []string{"--limit", "1"}},
		{"--resume beside it", buildstore.ReplayKey, []string{"--replay", "--resume"}},
		{"--through beside it", buildstore.ReplayKey, []string{"--replay", "--through", "start"}},
		{"two positionals", buildstore.ReplayKey, []string{"--replay", "ab", "cd"}},
		{"a path", buildstore.ReplayKey, []string{"--replay", "../" + replayStart}},
		{"an unknown kind", "--kind", []string{"--replay", "--kind", "bogus"}},
		{"a negative limit", "--limit", []string{"--replay", "--limit=-1"}},
		{"no store", "--state-dir", []string{"--replay"}},
		{"no trail, beside every flag a replay takes", buildstore.ReplayKey, []string{"--replay", "ab", "--state-dir", empty,
			"--kind", build.ReplayOverview, "--failed", "--limit", "1", "--ask-concurrency", "2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := kbase(t, t.TempDir(), append([]string{"build"}, tc.args...)...)
			doc := parseResult(t, out)
			items := doc.items(toolresult.RefusalsKey)
			if doc.Outcome != toolresult.Refused || code != toolresult.ExitCode(toolresult.Refused) || len(items) != 1 ||
				items[0]["check"] != toolresult.CheckUsage || items[0]["key"] != tc.key {
				t.Fatalf("%q: outcome %s, exit %d, refusals %v; want one usage refusal naming %s\n%s", tc.args, doc.Outcome, code, items, tc.key, out)
			}
		})
	}
}

// TestBuildReplayFlagsAreNoMCPArgument: the replay's flags are excluded from
// the build tool, whose schema derives none of them.
func TestBuildReplayFlagsAreNoMCPArgument(t *testing.T) {
	names := []string{"replay", "kind", "failed", "limit"}
	for _, name := range names {
		if f := buildCmd.Flags().Lookup(name); f == nil || !mcp.ExcludedFlag(f) {
			t.Errorf("build --%s is not excluded from MCP", name)
		}
	}
}
