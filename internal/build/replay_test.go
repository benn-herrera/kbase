//go:build kbasedev

package build

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/asks"
	"kbase/internal/asks/call"
	"kbase/internal/buildstore"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/result"
)

// askedTrail is a fixture whose build of askedPaper asked the scripted
// provider, and the directory of the trail it stands on.
func askedTrail(t *testing.T) (fixture, string) {
	t.Helper()
	f := newFixture(t)
	storetest.WriteFile(t, filepath.Join(f.repo, "paper.tex"), askedPaper)
	if outcome, fields := f.build(t, f.asking(newAnswering(script).Client())); outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	return f, f.startTrailDir(t)
}

// replayProvider is client in both roles, under the models the fixture's
// build named, on a queue of one.
func replayProvider(client model.Client) Provider {
	return Provider{MultichoiceClient: client, Multichoice: "decision", OverviewClient: client, Overview: "text", Queue: model.NewQueue(1)}
}

// openReplay opens a replay of the store at state's current trail, every row.
func openReplay(t *testing.T, state string) *Replay {
	t.Helper()
	rp, err := OpenReplay(state, "", "", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	return rp
}

func readComparison(t *testing.T, rp *Replay) []comparisonRow {
	t.Helper()
	f, err := os.Open(filepath.Join(rp.Dir(), comparisonFile))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []comparisonRow
	for dec := json.NewDecoder(f); dec.More(); {
		var row comparisonRow
		if err := dec.Decode(&row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

// recordedItems is the trail's recorded multichoice items in walk order, each
// with its kind and group.
func recordedItems(t *testing.T, trail string) []selectedRow {
	t.Helper()
	var out []selectedRow
	for _, k := range asks.Kinds {
		recs, err := asks.ReadGroupRecords(asks.GroupRecordsPath(filepath.Join(trail, asks.AskRecordsDir), k))
		if err != nil {
			t.Fatal(err)
		}
		for _, rec := range recs {
			for _, it := range rec.Items {
				out = append(out, selectedRow{kind: string(rec.Kind), group: rec.Group, item: it.Item})
			}
		}
	}
	return out
}

// trailRecords is every file under the trail's answers/, asks/ and captures/,
// by trail-relative path.
func trailRecords(t *testing.T, trail string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, dir := range []string{asks.AnswersDir, asks.AskRecordsDir, asks.CapturesDir} {
		err := filepath.WalkDir(filepath.Join(trail, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(p)
			rel, _ := filepath.Rel(trail, p)
			files[rel] = string(b)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// porcelain is git status --porcelain in repo. The fixture's paper is never
// committed, so a build leaves it untracked.
func porcelain(t *testing.T, repo string) string {
	t.Helper()
	status := exec.Command("git", "status", "--porcelain")
	status.Dir = repo
	out, err := status.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	return string(out)
}

// TestBuildReplayOfAnUnchangedTrail: the current trail of a build asked
// through the scripted provider, replayed through the same script, re-renders
// every prompt as it was sent and gets every recorded answer back, across
// every kind and the overview, and leaves the trail, its stamp and the
// repository as they were.
func TestBuildReplayOfAnUnchangedTrail(t *testing.T) {
	f, trail := askedTrail(t)
	before, repoBefore := trailRecords(t, trail), porcelain(t, f.repo)
	stamped, err := os.Stat(trail)
	if err != nil {
		t.Fatal(err)
	}

	rp := openReplay(t, f.state)
	res, err := rp.Walk(context.Background(), replayProvider(newAnswering(script).Client()), log.Discard())
	if err != nil || res.Outcome != result.Done {
		t.Fatalf("replay = %+v, %v; want done", res, err)
	}
	rows := readComparison(t, rp)
	if len(rows) == 0 || len(rows) != res.Counts.Total || res.Counts.Done != len(rows) {
		t.Fatalf("%d rows, counts %+v; want every selected row replayed", len(rows), res.Counts)
	}
	kinds, outcomes := map[string]bool{}, map[asks.Outcome]bool{}
	for _, row := range rows {
		kinds[row.Kind] = true
		outcomes[row.Original.Outcome] = true
		if row.PromptChanged {
			t.Errorf("%s %s %s: the prompt changed", row.Kind, row.Group, row.Item)
		}
		if row.Replay.Outcome != row.Original.Outcome || !sameChoice(row.Replay.Choice, row.Original.Choice) {
			t.Errorf("%s %s %s: replayed %+v, original %+v", row.Kind, row.Group, row.Item, row.Replay, row.Original)
		}
	}
	for _, k := range asks.Kinds {
		if !kinds[string(k)] {
			t.Errorf("no %s row", k)
		}
	}
	if !kinds[ReplayOverview] {
		t.Error("no overview row")
	}
	for _, o := range []asks.Outcome{asks.Answered, asks.Reasked, asks.Defaulted} {
		if !outcomes[o] {
			t.Errorf("no row recorded %s", o)
		}
	}
	if res.Changed != 0 {
		t.Errorf("changed = %d, want 0", res.Changed)
	}
	if after := trailRecords(t, trail); !maps.Equal(after, before) {
		t.Errorf("the trail's records moved: %d files before, %d after", len(before), len(after))
	}
	if after := porcelain(t, f.repo); after != repoBefore {
		t.Errorf("git status --porcelain = %q after the replay, %q before", after, repoBefore)
	}
	if info, err := os.Stat(trail); err != nil || !info.ModTime().Equal(stamped.ModTime()) {
		t.Errorf("the trail's modification time moved from %v: %v, %v", stamped.ModTime(), info, err)
	}
}

// TestBuildReplayFlagsAnEditedTemplateInput: one template input edited in the
// trail's paragraph records changes that item's prompt and no other.
func TestBuildReplayFlagsAnEditedTemplateInput(t *testing.T) {
	f, trail := askedTrail(t)
	dir := filepath.Join(trail, asks.AskRecordsDir)
	recs, err := asks.ReadGroupRecords(asks.GroupRecordsPath(dir, asks.Paragraph))
	if err != nil {
		t.Fatal(err)
	}
	var edited selectedRow
	for _, rec := range recs {
		if i := slices.IndexFunc(rec.Items, func(it asks.ItemRecord) bool { return len(it.Inputs) > 0 }); i >= 0 {
			inputs := rec.Items[i].Inputs
			slot := slices.Sorted(maps.Keys(inputs))[0]
			inputs[slot] += " (edited)"
			edited = selectedRow{kind: string(rec.Kind), group: rec.Group, item: rec.Items[i].Item}
			break
		}
	}
	if edited.item == "" {
		t.Fatal("no paragraph item records inputs of its own")
	}
	if err := asks.WriteGroupRecords(dir, recs); err != nil {
		t.Fatal(err)
	}

	rp := openReplay(t, f.state)
	if res, err := rp.Walk(context.Background(), replayProvider(newAnswering(script).Client()), log.Discard()); err != nil || res.Outcome != result.Done {
		t.Fatalf("replay = %+v, %v; want done", res, err)
	}
	for _, row := range readComparison(t, rp) {
		if want := rowOf(row) == edited; row.PromptChanged != want {
			t.Errorf("%s %s %s: prompt_changed = %t, want %t", row.Kind, row.Group, row.Item, row.PromptChanged, want)
		}
	}
}

// failingKey is the scripted provider, save that every call asking key never
// completes.
type failingKey struct {
	*answering
	key string
}

func (a failingKey) Client() model.Client {
	return &model.ClientFuncs{Stream: a.ConsultStream, Decide: a.Decision}
}

func (a failingKey) Decision(ctx context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
	if req.Questions[0].Key == a.key {
		return model.DecisionResponse{}, errors.New("connection refused")
	}
	return a.answering.Decision(ctx, req)
}

// TestBuildReplayCarriesOnPastACallThatNeverCompletes: the replayed item
// whose every call fails is a failed row, and the replay asks every other row
// and ends done.
func TestBuildReplayCarriesOnPastACallThatNeverCompletes(t *testing.T) {
	f, trail := askedTrail(t)
	// An item's name keys its question, and groups reuse names: the lost item
	// is the first whose name no other item shares.
	items := recordedItems(t, trail)
	i := slices.IndexFunc(items, func(a selectedRow) bool {
		return !slices.ContainsFunc(items, func(b selectedRow) bool { return b.item == a.item && b != a })
	})
	if i < 0 {
		t.Fatal("every recorded item shares its name")
	}
	lost := items[i]

	rp := openReplay(t, f.state)
	res, err := rp.Walk(context.Background(), replayProvider(failingKey{newAnswering(script), lost.item}.Client()), log.Discard())
	if err != nil || res.Outcome != result.Done {
		t.Fatalf("replay = %+v, %v; want done", res, err)
	}
	if res.Counts.Failed != 1 {
		t.Errorf("counts = %+v, want one failed", res.Counts)
	}
	rows := readComparison(t, rp)
	if len(rows) != res.Counts.Total {
		t.Errorf("%d rows of %d selected; want every row replayed", len(rows), res.Counts.Total)
	}
	for _, row := range rows {
		if failed := row.Replay.Outcome == asks.Failed; failed != (rowOf(row) == lost) {
			t.Errorf("%s %s %s replayed %s", row.Kind, row.Group, row.Item, row.Replay.Outcome)
		}
	}
}

// TestBuildReplayOverviewThatNeverCompletes: a replayed overview call whose
// stream never completes is a failed row, and the replay asks every
// multichoice row as the trail recorded it and ends done.
func TestBuildReplayOverviewThatNeverCompletes(t *testing.T) {
	f, _ := askedTrail(t)
	a := newAnswering(script)
	cutOff := func(ctx context.Context, req model.Request) (model.StreamReader, error) {
		return modeltest.NewScriptedMock([]model.Response{{FinishReason: model.FinishLength}}, nil).ConsultStream(ctx, req)
	}
	rp := openReplay(t, f.state)
	res, err := rp.Walk(context.Background(), replayProvider(&model.ClientFuncs{Stream: cutOff, Decide: a.Decision}), log.Discard())
	if err != nil || res.Outcome != result.Done {
		t.Fatalf("replay = %+v, %v; want done", res, err)
	}
	overviews := 0
	for _, row := range readComparison(t, rp) {
		if row.Kind == ReplayOverview {
			overviews++
			if row.Replay.Outcome != asks.Failed {
				t.Errorf("overview attempt %d replayed %s, want %s", row.Attempt, row.Replay.Outcome, asks.Failed)
			}
		} else if row.Replay.Outcome != row.Original.Outcome || !sameChoice(row.Replay.Choice, row.Original.Choice) {
			t.Errorf("%s %s %s: replayed %+v, original %+v", row.Kind, row.Group, row.Item, row.Replay, row.Original)
		}
	}
	if overviews == 0 {
		t.Error("no overview row")
	}
}

// TestBuildReplayPrunedMidWalkFails: a replay whose directory is removed
// while it asks ends failed, and leaves no directory behind, though its
// in-flight calls' files recreated parts of it.
func TestBuildReplayPrunedMidWalkFails(t *testing.T) {
	f, _ := askedTrail(t)
	rp := openReplay(t, f.state)
	a := newAnswering(script)
	var once sync.Once
	client := &model.ClientFuncs{Stream: a.ConsultStream, Decide: func(ctx context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
		once.Do(func() {
			if err := os.RemoveAll(rp.Dir()); err != nil {
				t.Error(err)
			}
		})
		return a.Decision(ctx, req)
	}}
	if res, err := rp.Walk(context.Background(), replayProvider(client), log.Discard()); err == nil || res.Outcome != result.Failed {
		t.Fatalf("replay = %+v, %v; want failed naming the removed directory", res, err)
	}
	if _, err := os.Stat(rp.Dir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the replay directory stands: %v", err)
	}
}

// TestBuildReplayWholeTrailPrunedMidWalk: a prune that removes the whole
// trail under a walk leaves no trail standing, though an in-flight call
// recreated the replay's directory.
func TestBuildReplayWholeTrailPrunedMidWalk(t *testing.T) {
	f, trail := askedTrail(t)
	rp := openReplay(t, f.state)
	a := newAnswering(script)
	var once sync.Once
	client := &model.ClientFuncs{Stream: a.ConsultStream, Decide: func(ctx context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
		once.Do(func() {
			if err := os.RemoveAll(trail); err != nil {
				t.Error(err)
			}
		})
		return a.Decision(ctx, req)
	}}
	if res, err := rp.Walk(context.Background(), replayProvider(client), log.Discard()); err == nil || res.Outcome != result.Failed {
		t.Fatalf("replay = %+v, %v; want failed naming the removed directory", res, err)
	}
	trails, err := listTrails(f.state)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(trails, trail) {
		t.Errorf("the pruned trail %s stands", trail)
	}
}

// TestBuildReplayKeepsKeptRunsReplays: a replay over KeptRuns+1 standing
// replays of its trail keeps itself and the newest KeptRuns-1 of them, by
// replay.yaml's start.
func TestBuildReplayKeepsKeptRunsReplays(t *testing.T) {
	f, trail := askedTrail(t)
	now := time.Now()
	var staged []string
	for i := range buildstore.KeptRuns + 1 {
		name := replayKind + "-staged-" + string(rune('a'+i))
		started := stamp(now.Add(-time.Duration(buildstore.KeptRuns+1-i) * time.Hour))
		storetest.WriteFile(t, filepath.Join(trail, replaysDir, name, replayFile), "started: "+started+"\n")
		staged = append(staged, name)
	}

	rp := openReplay(t, f.state)
	if res, err := rp.Walk(context.Background(), replayProvider(newAnswering(script).Client()), log.Discard()); err != nil || res.Outcome != result.Done {
		t.Fatalf("replay = %+v, %v; want done", res, err)
	}
	entries, err := os.ReadDir(filepath.Join(trail, replaysDir))
	if err != nil {
		t.Fatal(err)
	}
	var have []string
	for _, e := range entries {
		have = append(have, e.Name())
	}
	want := append(slices.Clone(staged[len(staged)-(buildstore.KeptRuns-1):]), filepath.Base(rp.Dir()))
	slices.Sort(want)
	if !slices.Equal(have, want) {
		t.Errorf("replays/ holds %q, want the replay and the newest %d staged: %q", have, buildstore.KeptRuns-1, want)
	}
}

// TestBuildReplayCancelled: a replay whose context ends once its first item
// is asked is cancelled, names the second as the first not asked, and writes
// the first's row.
func TestBuildReplayCancelled(t *testing.T) {
	f, trail := askedTrail(t)
	order := recordedItems(t, trail)
	rp := openReplay(t, f.state)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := newAnswering(script)
	var mu sync.Mutex
	first := ""
	client := &model.ClientFuncs{Stream: a.ConsultStream, Decide: func(c context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
		mu.Lock()
		if key := req.Questions[0].Key; first == "" {
			first = key
		} else if key != first {
			cancel()
		}
		mu.Unlock()
		return a.Decision(c, req)
	}}
	res, err := rp.Walk(ctx, replayProvider(client), log.Discard())
	if err != nil || res.Outcome != result.Cancelled {
		t.Fatalf("replay = %+v, %v; want cancelled", res, err)
	}
	if c := res.Cancelled; (selectedRow{kind: c.Kind, group: c.Group, item: c.Item}) != order[1] {
		t.Errorf("cancelled names %+v, want the second row, %+v", c, order[1])
	}
	rows := readComparison(t, rp)
	if len(rows) != 1 || rowOf(rows[0]) != order[0] {
		t.Errorf("rows = %+v, want the first, %+v, alone", rows, order[0])
	}
}

// writeTrail writes a trail named start in the store at state as a build's
// asks write one, through call.New: one paragraph group whose items are
// answered, defaulted and failed, each named for its outcome, and an overview
// call asked at attempt 1, whose heading the overview stage rejects, and at
// attempt 2, answered.
func writeTrail(t *testing.T, state, start string) string {
	t.Helper()
	trail := filepath.Join(state, buildstore.ScratchDir, buildstore.TrailsDir, start)
	const heading = "# A heading"
	passages := modeltest.NewScriptedMockPerConsult([]model.Response{
		{Content: heading, FinishReason: "stop"},
		{Content: "A small paper, again.", FinishReason: "stop"},
	})
	client := &model.ClientFuncs{Stream: passages.ConsultStream, Decide: func(_ context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
		q := req.Questions[0]
		noul := 0.9
		switch q.Key {
		case string(asks.Failed):
			return model.DecisionResponse{}, errors.New("connection refused")
		case string(asks.Defaulted):
			noul = 2
		}
		return model.DecisionResponse{Answers: []model.DecisionAnswer{{Key: q.Key, Type: q.Type, Noul: noul}}}, nil
	}}
	caller := call.New(client, "solo", func() string { return trail }, log.Discard())
	queue := model.NewQueue(1)
	ctx := context.Background()
	batch := asks.NewReplayBatch(ctx, caller.Launch(queue, "decision"))
	var items []asks.ParagraphItem
	for _, o := range []asks.Outcome{asks.Answered, asks.Defaulted, asks.Failed} {
		items = append(items, asks.ParagraphItem{Paragraph: string(o), Text: "The paragraph " + string(o) + "."})
	}
	if err := batch.AskGroup(asks.Paragraph, "leaf", asks.ParagraphAsks("leaf.md", "The leaf.", items)); err != nil {
		t.Fatal(err)
	}
	recs, err := batch.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if err := asks.WriteGroupRecords(filepath.Join(trail, asks.AskRecordsDir), recs); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	if err := caller.PassageLaunch(queue, "text")(ctx, &wg, func(ask func(context.Context, string, []string, int) (string, error)) func(error) {
		for attempt, rejected := range [][]string{nil, {heading}} {
			if _, err := ask(ctx, "The excerpts.", rejected, attempt+1); err != nil {
				t.Error(err)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	return trail
}

// selectedRow is one row a replay selected: an overview call's by its
// attempt.
type selectedRow struct {
	kind, group, item string
	attempt           int
}

func rowOf(row comparisonRow) selectedRow {
	return selectedRow{kind: row.Kind, group: row.Group, item: row.Item, attempt: row.Attempt}
}

func selection(rp *Replay) []selectedRow {
	var out []selectedRow
	for _, g := range rp.groups {
		for _, it := range g.rec.Items {
			out = append(out, selectedRow{kind: string(g.rec.Kind), group: g.rec.Group, item: it.Item})
		}
	}
	for _, c := range rp.overviews {
		out = append(out, selectedRow{kind: ReplayOverview, item: ReplayOverview, attempt: c.Attempt})
	}
	return out
}

func TestBuildReplaySelection(t *testing.T) {
	state := t.TempDir()
	writeTrail(t, state, strings.Repeat("a", 40))
	row := func(o asks.Outcome) selectedRow {
		return selectedRow{kind: string(asks.Paragraph), group: "leaf", item: string(o)}
	}
	overview := func(attempt int) selectedRow {
		return selectedRow{kind: ReplayOverview, item: ReplayOverview, attempt: attempt}
	}
	for _, tc := range []struct {
		name       string
		kind       string
		failedOnly bool
		limit      int
		want       []selectedRow
	}{
		{name: "every row", want: []selectedRow{row(asks.Answered), row(asks.Defaulted), row(asks.Failed), overview(1), overview(2)}},
		{name: "failed only", failedOnly: true, want: []selectedRow{row(asks.Defaulted), row(asks.Failed), overview(1)}},
		{name: "overview", kind: ReplayOverview, want: []selectedRow{overview(1), overview(2)}},
		{name: "paragraph", kind: string(asks.Paragraph), want: []selectedRow{row(asks.Answered), row(asks.Defaulted), row(asks.Failed)}},
		{name: "limit", limit: 2, want: []selectedRow{row(asks.Answered), row(asks.Defaulted)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rp, err := OpenReplay(state, "", tc.kind, tc.failedOnly, tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if got := selection(rp); !slices.Equal(got, tc.want) {
				t.Errorf("selected %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestBuildReplayTrailChoice(t *testing.T) {
	state := t.TempDir()
	older, newer := strings.Repeat("b", 40), strings.Repeat("a", 40)
	writeTrail(t, state, newer)
	olderDir := writeTrail(t, state, older)
	trails := filepath.Join(state, buildstore.ScratchDir, buildstore.TrailsDir)
	then := time.Now().Add(-time.Hour)
	if err := os.Chtimes(olderDir, then, then); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ start, want string }{{"", newer}, {"b", older}, {older, older}} {
		rp, err := OpenReplay(state, tc.start, "", false, 0)
		if err != nil {
			t.Fatalf("start %q: %v", tc.start, err)
		}
		if want := filepath.Join(trails, tc.want, replaysDir) + string(filepath.Separator); !strings.HasPrefix(rp.Dir(), want) {
			t.Errorf("start %q replays %s, want under %s", tc.start, rp.Dir(), want)
		}
	}

	// A third trail sharing newer's first character.
	if err := os.MkdirAll(filepath.Join(trails, "a"+strings.Repeat("0", 39)), 0o777); err != nil {
		t.Fatal(err)
	}
	noInputs := filepath.Join(t.TempDir(), "state")
	noInputsTrail := filepath.Join(noInputs, buildstore.ScratchDir, buildstore.TrailsDir, newer)
	if err := asks.WriteGroupRecords(filepath.Join(noInputsTrail, asks.AskRecordsDir), []asks.GroupRecord{{Kind: asks.Paragraph, Group: "leaf",
		Items: []asks.ItemRecord{{Item: string(asks.Answered), Offered: []string{asks.LetterClaim, asks.LetterNotClaim}, Outcome: asks.Answered}}}}); err != nil {
		t.Fatal(err)
	}
	noExcerpts := filepath.Join(t.TempDir(), "state")
	noExcerptsCapture := filepath.Join(noExcerpts, buildstore.ScratchDir, buildstore.TrailsDir, newer, asks.CapturesDir, "overview-h-1.capture.jsonl")
	storetest.WriteFile(t, noExcerptsCapture, `{"type":"request","ask-attempt":1,"prompt-sha256":"h"}`+"\n"+`{"type":"attempt","outcome":"ok","reply":"r"}`+"\n")
	empty := t.TempDir()
	for _, tc := range []struct {
		name, state, start, kind, path string
	}{
		{"a prefix matching none", state, "c", "", trails},
		{"a prefix matching two", state, "a", "", trails},
		{"a store with no trail", empty, "", "", filepath.Join(empty, buildstore.ScratchDir, buildstore.TrailsDir)},
		{"a group record without inputs", noInputs, "", "", asks.GroupRecordsPath(filepath.Join(noInputsTrail, asks.AskRecordsDir), asks.Paragraph)},
		{"an overview capture without excerpts", noExcerpts, "", "", noExcerptsCapture},
		{name: "an empty selection", state: state, start: older, kind: string(asks.Colinear), path: filepath.Join(trails, older)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := OpenReplay(tc.state, tc.start, tc.kind, false, 0)
			assertReplayRefusal(t, err, tc.path)
		})
	}
}

// TestBuildReplayRefusesANoInferenceTrail: a trail a --no-inference build
// stands on records no ask, and is refused naming it. A fresh build makes no
// trail directory until it asks, so the trail is bounded, then resumed, which
// makes one.
func TestBuildReplayRefusesANoInferenceTrail(t *testing.T) {
	f := newFixture(t)
	bounded := f.options()
	bounded.Through = buildstore.StageDocumentGraph
	if outcome, fields := f.build(t, bounded); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if outcome, fields := f.build(t, resuming(f.options())); outcome != result.Done {
		t.Fatalf("resume = %s %v", outcome, fields)
	}
	_, err := OpenReplay(f.state, "", "", false, 0)
	assertReplayRefusal(t, err, f.startTrailDir(t))
}

func assertReplayRefusal(t *testing.T, err error, path string) {
	t.Helper()
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Check != result.CheckUsage || refusal[0].Key != buildstore.ReplayKey || refusal[0].Path != path {
		t.Errorf("OpenReplay = %v, want one %s refusal keyed %s naming %s", err, result.CheckUsage, buildstore.ReplayKey, path)
	}
}
