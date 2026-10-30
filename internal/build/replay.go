//go:build kbasedev

package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/asks"
	"kbase/internal/asks/call"
	"kbase/internal/atomicfile"
	"kbase/internal/buildstore"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/result"
)

// A development replay's layout: one directory per replay under its trail's
// replays/, holding replay.yaml, the replay's own answers/, captures/ and
// asks/, and the comparison of each row it re-asked.
const (
	replaysDir     = "replays"
	replayFile     = "replay.yaml"
	comparisonFile = "comparison.jsonl"
	replayKind     = "replay"
)

// ReplayOverview is the kind selecting a trail's overview calls, and the kind
// and item of an overview call's comparison row.
const ReplayOverview = "overview"

// Replay is a re-asking of one build trail's recorded asks under this
// binary's templates: the selected items of each group record, in walk order,
// then the selected overview calls.
type Replay struct {
	id, trail, dir string
	groups         []replayGroup
	overviews      []call.OverviewCapture
}

// replayGroup is one group record narrowed to its selected items, and those
// items as the running binary asks them.
type replayGroup struct {
	rec   asks.GroupRecord
	items []asks.MultichoiceItem
}

// ReplayResult is what a replay reports of itself: how it stopped — result.Done
// where every selected row was asked, result.Cancelled where the context
// stopped it first, result.Failed for a fault of the replay itself — its rows
// by replayed outcome, how many differ from the original in outcome or
// choice, and, where it was cancelled, the first row it did not ask.
type ReplayResult struct {
	Outcome   string
	Counts    struct{ Total, Done, Failed int }
	Changed   int
	Cancelled struct{ Kind, Group, Item string }
}

// OpenReplay selects what a replay of a trail in the state store at stateDir
// re-asks. The trail is the one start names by hex prefix, or, where start is
// "", the trail directory last stamped. It selects, in walk order — the
// multichoice kinds in asks.Kinds order, groups in file order and items in
// record order, then the overview calls by ask attempt — the rows of kind,
// every kind where it is ""; with failedOnly, only the items recorded failed
// or defaulted and the overview calls that failed or were rejected; and,
// where limit is above zero, the first limit of those. Nothing is written.
func OpenReplay(stateDir, start, kind string, failedOnly bool, limit int) (*Replay, error) {
	stateDir, err := filepath.Abs(stateDir)
	if err != nil {
		return nil, err
	}
	trail, err := replayTrail(stateDir, start)
	if err != nil {
		return nil, err
	}
	rp := &Replay{trail: trail}
	taken := 0
	full := func() bool { return limit > 0 && taken >= limit }
	recorded := false
	for _, k := range asks.Kinds {
		path := asks.GroupRecordsPath(filepath.Join(trail, asks.AskRecordsDir), k)
		recs, err := asks.ReadGroupRecords(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		recorded = true
		if kind != "" && kind != string(k) {
			continue
		}
		for _, rec := range recs {
			selected := rec
			selected.Items = nil
			for _, it := range rec.Items {
				if full() {
					break
				}
				if failedOnly && it.Outcome != asks.Failed && it.Outcome != asks.Defaulted {
					continue
				}
				if len(rec.Inputs)+len(it.Inputs) == 0 {
					return nil, result.Refusal{{Check: result.CheckUsage, Key: buildstore.ReplayKey, Path: path,
						Detail: fmt.Sprintf("%s group %s, item %s records no template inputs: an earlier kbase wrote it", rec.Kind, rec.Group, it.Item)}}
				}
				selected.Items = append(selected.Items, it)
				taken++
			}
			if len(selected.Items) == 0 {
				continue
			}
			items, err := asks.RecordedItems(selected)
			if err != nil {
				return nil, err
			}
			rp.groups = append(rp.groups, replayGroup{rec: selected, items: items})
		}
	}
	captures, err := call.ReadOverviewCaptures(trail)
	if err != nil {
		return nil, err
	}
	if !recorded && len(captures) == 0 {
		return nil, result.Refusal{{Check: result.CheckUsage, Key: buildstore.ReplayKey, Path: trail,
			Detail: "the trail records no ask: it was built without inference, or asked nothing"}}
	}
	for _, c := range captures {
		if kind != "" && kind != ReplayOverview || full() {
			break
		}
		if o := overviewOutcome(c); failedOnly && o != asks.Failed && o != overviewRejected {
			continue
		}
		if !c.HasInputs {
			return nil, result.Refusal{{Check: result.CheckUsage, Key: buildstore.ReplayKey, Path: c.Path,
				Detail: "the overview call's capture records no excerpts: an earlier kbase wrote it"}}
		}
		rp.overviews = append(rp.overviews, c)
		taken++
	}
	if len(rp.groups) == 0 && len(rp.overviews) == 0 {
		return nil, result.Refusal{{Check: result.CheckUsage, Key: buildstore.ReplayKey, Path: trail,
			Detail: "the selection is empty: no recorded ask matches the kind, --failed and --limit given"}}
	}
	parent := filepath.Join(trail, replaysDir)
	rp.id = kb.MintID(replayKind, func(id string) bool {
		_, err := os.Stat(filepath.Join(parent, id))
		return err == nil
	})
	rp.dir = filepath.Join(parent, rp.id)
	return rp, nil
}

// replayTrail is the trail directory start names by hex prefix in the store at
// stateDir, or the one last stamped where start is "".
func replayTrail(stateDir, start string) (string, error) {
	trails, err := listTrails(stateDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(stateDir, buildstore.ScratchDir, buildstore.TrailsDir)
	if len(trails) == 0 {
		return "", result.Refusal{{Check: result.CheckUsage, Key: buildstore.ReplayKey, Path: path, Detail: "no build trail stands in the state store"}}
	}
	if start == "" {
		return trails[0], nil
	}
	var matches []string
	for _, t := range trails {
		if strings.HasPrefix(filepath.Base(t), start) {
			matches = append(matches, filepath.Base(t))
		}
	}
	switch len(matches) {
	case 0:
		return "", result.Refusal{{Check: result.CheckUsage, Key: buildstore.ReplayKey, Path: path, Detail: "no build trail's start commit begins " + start}}
	case 1:
		return filepath.Join(path, matches[0]), nil
	}
	return "", result.Refusal{{Check: result.CheckUsage, Key: buildstore.ReplayKey, Path: path,
		Detail: fmt.Sprintf("%s begins more than one build trail's start commit: %s", start, strings.Join(matches, ", "))}}
}

// overviewOutcome is how the overview call c recorded ended: failed where its
// last attempt did not complete, else the overview stage's judgement of its
// reply.
func overviewOutcome(c call.OverviewCapture) asks.Outcome {
	if c.Outcome != model.ChatOK {
		return asks.Failed
	}
	_, _, o := judgeOverview(c.Reply)
	return o
}

// ID is the replay's id, minted replay-<id>.
func (rp *Replay) ID() string { return rp.id }

// Dir is the replay's directory, absolute, under its trail's replays/.
func (rp *Replay) Dir() string { return rp.dir }

// Trail is the start commit of the trail the replay re-asks.
func (rp *Replay) Trail() string { return filepath.Base(rp.trail) }

// replayHeader is replay.yaml: the replay, the trail's start commit, the
// models that answered each role, and when it started.
type replayHeader struct {
	Replay        string `yaml:"replay"`
	Trail         string `yaml:"trail"`
	DecisionModel string `yaml:"decision_model"`
	TextModel     string `yaml:"text_model"`
	Started       string `yaml:"started"`
}

// comparisonRow is one line of comparison.jsonl: the row, its original and
// replayed side, and whether the replay's rendered prompt differs from the
// one the trail records. Attempt is an overview row's alone.
type comparisonRow struct {
	Kind          string         `json:"kind"`
	Group         string         `json:"group"`
	Item          string         `json:"item"`
	Attempt       int            `json:"attempt,omitempty"`
	Original      comparisonSide `json:"original"`
	Replay        comparisonSide `json:"replay"`
	PromptChanged bool           `json:"prompt_changed"`
}

// comparisonSide is one side of a row: its outcome, and the choice and
// confidence of a multichoice item, null where none is recorded.
type comparisonSide struct {
	Outcome    asks.Outcome       `json:"outcome"`
	Choice     *string            `json:"choice"`
	Confidence map[string]float64 `json:"confidence"`
}

func sideOf(it asks.ItemRecord) comparisonSide {
	return comparisonSide{Outcome: it.Outcome, Choice: it.Choice, Confidence: it.Confidence}
}

// overviewReplayed is one overview call as the replay asked it: whether it
// was, how it ended, and the digest of the prompt it rendered.
type overviewReplayed struct {
	asked        bool
	outcome      asks.Outcome
	promptSHA256 string
}

// Walk re-asks the selected rows through p — the multichoice items as one
// replay batch on p's decision role, then each overview call as its own item
// on its text role, every call on p.Queue — recording the replay under its
// directory alone and taking no lock. Once replay.yaml is written it keeps
// itself and the newest buildstore.KeptRuns-1 other replays of its trail's
// replays/, by start. Once the walk has drained it checks the
// replay directory still stands, then writes the replay's group records and
// comparison.jsonl. A call that never completes is a failed row, not the
// replay's error: the error is a fault of the replay itself — a trail that no
// longer stands, a compose or write error, or a replay directory removed under
// the walk, which is then removed whole.
func (rp *Replay) Walk(ctx context.Context, p Provider, lg log.Logger) (ReplayResult, error) {
	res := ReplayResult{Outcome: result.Failed}
	for _, g := range rp.groups {
		res.Counts.Total += len(g.rec.Items)
	}
	res.Counts.Total += len(rp.overviews)
	header, err := yaml.Marshal(replayHeader{Replay: rp.id, Trail: rp.Trail(),
		DecisionModel: p.Multichoice, TextModel: p.Overview, Started: stamp(time.Now())})
	if err != nil {
		return res, err
	}
	if err := rp.setUp(header); err != nil {
		return res, fmt.Errorf("start replay %s: %w", rp.id, err)
	}
	if err := buildstore.PruneByStart(filepath.Dir(rp.dir), replayFile, rp.id, buildstore.KeptRuns, func(string) bool { return true }); err != nil {
		return res, fmt.Errorf("prune the replays: %w", err)
	}

	batch := asks.NewReplayBatch(ctx, call.New(p.MultichoiceClient, rp.Dir, lg).Launch(p.Queue, p.Multichoice))
	for _, g := range rp.groups {
		if err := batch.AskGroup(g.rec.Kind, g.rec.Group, g.items); err != nil {
			break
		}
	}
	recs, walkErr := batch.Wait()
	overviews := make([]overviewReplayed, len(rp.overviews))
	if walkErr == nil {
		walkErr = rp.askOverviews(ctx, call.New(p.OverviewClient, rp.Dir, lg).PassageLaunch(p.Queue, p.Overview), overviews)
	}

	// An item in flight when a prune removed the replay may have recreated
	// parts of its directory through call's own writes; none of the replay's
	// own writes lands until the directory is known to stand.
	if _, err := os.Stat(filepath.Join(rp.dir, replayFile)); err != nil {
		rmErr := os.RemoveAll(rp.dir)
		// Removing an entry moves a directory's modification time, so the
		// trail's replays/ goes only where the trail holds nothing else.
		if entries, readErr := os.ReadDir(rp.trail); readErr == nil && len(entries) == 1 && entries[0].Name() == replaysDir {
			_ = os.Remove(filepath.Join(rp.trail, replaysDir))
			_ = os.Remove(rp.trail)
		}
		return res, errors.Join(fmt.Errorf("the replay directory %s was removed under the walk: %w", rp.dir, err), rmErr)
	}

	rows, unasked := rp.compare(recs, overviews, &res)
	if len(rows) > 0 {
		if err := rp.write(recs, rows); err != nil {
			return res, err
		}
	}
	switch {
	case walkErr != nil && ctx.Err() == nil:
		return res, walkErr
	case len(rows) < res.Counts.Total:
		res.Outcome = result.Cancelled
		res.Cancelled.Kind, res.Cancelled.Group, res.Cancelled.Item = unasked.Kind, unasked.Group, unasked.Item
	default:
		res.Outcome = result.Done
	}
	return res, nil
}

// setUp creates the replay's directory and writes replay.yaml, leaving the
// trail directory's modification time — the current-trail marker and the
// prune's order — as it found it. No step creates a parent, so a trail the
// prune removed fails here rather than standing again.
func (rp *Replay) setUp(header []byte) error {
	info, err := os.Stat(rp.trail)
	if err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Dir(rp.dir), 0o777); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	if err := os.Chtimes(rp.trail, time.Time{}, info.ModTime()); err != nil {
		return err
	}
	for _, dir := range []string{rp.dir, filepath.Join(rp.dir, asks.AnswersDir), filepath.Join(rp.dir, asks.CapturesDir), filepath.Join(rp.dir, asks.AskRecordsDir)} {
		if err := os.Mkdir(dir, 0o777); err != nil {
			return err
		}
	}
	return atomicfile.Write(filepath.Join(rp.dir, replayFile), header, nil)
}

// askOverviews asks each selected overview call as its own item through
// launch, with its recorded inputs and attempt, into out. It stops launching
// where the context ends.
func (rp *Replay) askOverviews(ctx context.Context, launch asks.PassageLaunch, out []overviewReplayed) error {
	var wg sync.WaitGroup
	errs := make([]error, len(rp.overviews))
	var launchErr error
	for i, c := range rp.overviews {
		prompt, err := asks.OverviewPrompt(c.Excerpts, c.Rejected)
		if err != nil {
			launchErr = err
			break
		}
		out[i].promptSHA256 = asks.Digest(prompt)
		if err := launch(ctx, &wg, func(ask func(context.Context, string, []string, int) (string, error)) func(error) {
			reply, err := ask(ctx, c.Excerpts, c.Rejected, c.Attempt)
			var incomplete asks.IncompleteError
			switch {
			case errors.As(err, &incomplete):
				out[i].outcome = asks.Failed
			case err != nil:
				errs[i] = err
				return nil
			default:
				_, _, out[i].outcome = judgeOverview(reply)
			}
			out[i].asked = true
			return func(err error) { errs[i] = err }
		}); err != nil {
			launchErr = err
			break
		}
	}
	wg.Wait()
	return errors.Join(append(errs, launchErr)...)
}

// compare is the comparison's rows in walk order, multichoice rows joined to
// the replay's records on kind, group and item, and overview rows to their
// capture, counted into res; and the first row in walk order not asked.
func (rp *Replay) compare(recs []asks.GroupRecord, overviews []overviewReplayed, res *ReplayResult) (rows []comparisonRow, unasked comparisonRow) {
	type key struct {
		kind        asks.Kind
		group, item string
	}
	replayed := map[key]asks.ItemRecord{}
	for _, rec := range recs {
		for _, it := range rec.Items {
			replayed[key{rec.Kind, rec.Group, it.Item}] = it
		}
	}
	add := func(row comparisonRow, asked bool) {
		if !asked {
			if unasked.Item == "" {
				unasked = row
			}
			return
		}
		if row.Replay.Outcome == asks.Failed {
			res.Counts.Failed++
		} else {
			res.Counts.Done++
		}
		if row.Replay.Outcome != row.Original.Outcome || !sameChoice(row.Replay.Choice, row.Original.Choice) {
			res.Changed++
		}
		rows = append(rows, row)
	}
	for _, g := range rp.groups {
		for _, it := range g.rec.Items {
			got, ok := replayed[key{g.rec.Kind, g.rec.Group, it.Item}]
			add(comparisonRow{Kind: string(g.rec.Kind), Group: g.rec.Group, Item: it.Item, Original: sideOf(it), Replay: sideOf(got),
				PromptChanged: got.PromptSHA256 != it.PromptSHA256}, ok)
		}
	}
	for i, c := range rp.overviews {
		r := overviews[i]
		add(comparisonRow{Kind: ReplayOverview, Item: ReplayOverview, Attempt: c.Attempt,
			Original: comparisonSide{Outcome: overviewOutcome(c)}, Replay: comparisonSide{Outcome: r.outcome},
			PromptChanged: r.promptSHA256 != c.PromptSHA256}, r.asked)
	}
	return rows, unasked
}

func sameChoice(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// write writes the replay's group records, one file per kind, and its
// comparison.jsonl whole, into the directories setUp created.
func (rp *Replay) write(recs []asks.GroupRecord, rows []comparisonRow) error {
	for _, k := range asks.Kinds {
		var of []asks.GroupRecord
		for _, rec := range recs {
			if rec.Kind == k {
				of = append(of, rec)
			}
		}
		if err := asks.WriteGroupRecords(filepath.Join(rp.dir, asks.AskRecordsDir), of); err != nil {
			return fmt.Errorf("record the replay: %w", err)
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	if err := atomicfile.Write(filepath.Join(rp.dir, comparisonFile), buf.Bytes(), nil); err != nil {
		return fmt.Errorf("compare the replay: %w", err)
	}
	return nil
}
