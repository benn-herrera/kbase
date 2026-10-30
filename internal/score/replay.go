//go:build kbasedev

package score

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"go.yaml.in/yaml/v3"

	"kbase/internal/atomicfile"
	"kbase/internal/buildstore"
	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
)

// A replay's layout: one directory per replay under its run's replays/, the
// replay.yaml that names it and the comparison of each item it re-asked; its
// ledger and captures take the run's own file names.
const (
	replaysDir     = "replays"
	replayFile     = "replay.yaml"
	comparisonFile = "comparison.jsonl"
	replayKind     = "replay"
)

// ReplayHeader is what replay.yaml records of a replay: which run it re-asks,
// what answered it, with what model, the pass it walked, the ask the binary
// sends, and when it started. Replay, Run and AskSHA256 are the replay's own and
// Walk stamps them, so a caller leaves them zero.
type ReplayHeader struct {
	Replay    string `yaml:"replay"`
	Run       string `yaml:"run"`
	BackEnd   string `yaml:"backend"`
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	Pass      string `yaml:"pass"`
	AskSHA256 string `yaml:"ask_sha256"`
	Started   string `yaml:"started"`
}

// Replay is a re-asking of a run's captured calls under this binary's
// templates: the calls selected, each item's last capture, in the order of the
// run's item list, and the outcome the run's ledger last records for each.
type Replay struct {
	id, run, dir string
	// pass is the run's pass, and selected the one this replay narrowed to; ""
	// where it took the run's set whole.
	pass     string
	selected string
	calls    []capturedCall
	original map[string]attempt
}

// askPass is the set of instructions this replay sends: the pass it narrowed to,
// or the run's own where it narrowed to nothing.
func (rp *Replay) askPass() string {
	if rp.selected != "" {
		return rp.selected
	}
	return rp.pass
}

// ReplayResult is what a replay reports of itself: how it stopped — result.Done
// where every selected item was asked, result.Cancelled where the context
// stopped it first — its items by the status it recorded for each, how many of
// them differ from the run's outcome in status or value, and, where it was
// cancelled, the first item it did not ask.
type ReplayResult struct {
	Outcome string
	Counts  Counts
	Changed int
	Stopped string
}

// OpenReplay selects the calls a replay of run, in the state directory dir,
// re-asks: every item with a capture, by its last, in the order of the run's
// item list; with pass naming one pass, only the calls that pass asked; with
// failedOnly, only those whose last ledger line is failed; and, where limit is
// above zero, the first limit of those. A run with no captures.jsonl — one
// standing no longer, one that asked nothing, a dry run — is refused naming
// --replay, and a pass the run did not walk one naming --pass. The run's records
// are read to their last complete line, so a run still appending may be
// replayed; nothing of it is written.
func OpenReplay(dir, run, pass string, failedOnly bool, limit int) (*Replay, error) {
	runDir := filepath.Join(RunsDir(dir), run)
	captured, err := readLastCaptures(runDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, result.Refusal{{Check: result.CheckUsage, Path: filepath.Join(runDir, capturesFile), Key: buildstore.ReplayKey,
			Detail: "run " + run + " has no " + capturesFile + ": it stands no longer, asked nothing, or was a dry run"}}
	}
	if err != nil {
		return nil, err
	}
	h, err := readHeader(runDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(runDir, runFile), err)
	}
	switch pass {
	case "", query.PassBoth:
		pass = ""
	case query.PassRigor, query.PassBearing:
		if h.Pass != query.PassBoth && h.Pass != pass {
			return nil, result.Refusal{{Check: result.CheckUsage, Key: "--pass",
				Detail: "run " + run + " walked " + h.Pass + ": --pass " + pass + " selects no call of it"}}
		}
	default:
		return nil, result.Refusal{{Check: result.CheckUsage, Key: "--pass",
			Detail: "--pass takes " + query.PassRigor + ", " + query.PassBearing + " or " + query.PassBoth + ", not " + strconv.Quote(pass)}}
	}
	items, err := readAttempts(filepath.Join(runDir, itemsFile))
	if err != nil {
		return nil, err
	}
	lines, err := readAttempts(filepath.Join(runDir, linesFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	rp := &Replay{run: run, pass: h.Pass, selected: pass, original: make(map[string]attempt, len(lines))}
	for _, a := range lines {
		rp.original[a.lineKey()] = a
	}
	for _, it := range items {
		key := it.lineKey()
		c, ok := captured[key]
		if !ok || pass != "" && c.Pass != pass {
			continue
		}
		if failedOnly && rp.original[key].Status != StatusFailed {
			continue
		}
		if limit > 0 && len(rp.calls) == limit {
			break
		}
		rp.calls = append(rp.calls, c)
	}
	parent := filepath.Join(runDir, replaysDir)
	rp.id = kb.MintID(replayKind, func(id string) bool {
		_, err := os.Stat(filepath.Join(parent, id))
		return err == nil
	})
	rp.dir = filepath.Join(parent, rp.id)
	return rp, nil
}

// capturedCall is one capture read back: the record, the class it names, and
// the closed text it carried, which a reply is parsed against.
type capturedCall struct {
	captureRecord
	class  class
	closed closedText
}

// readLastCaptures is, per item, the last call the captures.jsonl in the run
// directory runDir records, keyed as the ledger keys an item. The file is read to
// its last complete line, so a run still appending is read as far as it has
// landed. A complete line that does not read as a capture, names no class, or
// numbers bytes its text does not hold is an error: the record is what a reply
// is parsed against, and no part of it is guessed.
func readLastCaptures(runDir string) (map[string]capturedCall, error) {
	path := filepath.Join(runDir, capturesFile)
	b, _, err := readComplete(path)
	if err != nil {
		return nil, err
	}
	out := map[string]capturedCall{}
	for i, raw := range bytes.Split(b, []byte{'\n'}) {
		if len(raw) == 0 {
			continue
		}
		var rec captureRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		cl, ok := classNamed(rec.Class)
		if !ok {
			return nil, fmt.Errorf("%s line %d: no class is named %q", path, i+1, rec.Class)
		}
		closed, err := rec.closedText()
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		out[rec.lineKey()] = capturedCall{captureRecord: rec, class: cl, closed: closed}
	}
	return out, nil
}

// closedText is the closed text the record carries, refused where a line names
// bytes outside the text.
func (rec captureRecord) closedText() (closedText, error) {
	ct := closedText{text: rec.Text, lines: make([]line, len(rec.Lines))}
	for i, l := range rec.Lines {
		if l.From < 0 || l.From > l.To || l.To > len(rec.Text) {
			return closedText{}, fmt.Errorf("line %d names bytes %d-%d of a %d-byte text", i+1, l.From, l.To, len(rec.Text))
		}
		ct.lines[i] = line{no: l.No, end: l.End, from: l.From, to: l.To}
	}
	return ct, nil
}

// classNamed is the class a capture's name records, ok false where none is.
func classNamed(name string) (class, bool) {
	for _, cl := range []class{statementClass, methodClass, bearingClass} {
		if cl.name == name {
			return cl, true
		}
	}
	return class{}, false
}

// ID is the replay's id, minted as a run's is.
func (rp *Replay) ID() string { return rp.id }

// Dir is the replay's directory, under its run's replays/.
func (rp *Replay) Dir() string { return rp.dir }

// comparisonLine is one line of a replay's comparison.jsonl: the item, the
// outcome the run's ledger last records for it and the replay's, and whether the
// prompt the replay rendered differs from the one the run's capture records.
type comparisonLine struct {
	Item          string    `json:"item"`
	Before        string    `json:"before,omitempty"`
	Original      judgement `json:"original"`
	Replay        judgement `json:"replay"`
	PromptChanged bool      `json:"prompt_changed"`
}

// judgement is one side of a comparison: the status, the value and the fault ids
// in the order written, [] where none is recorded.
type judgement struct {
	Status string   `json:"status"`
	Value  string   `json:"value"`
	Faults []string `json:"faults"`
}

// judgementOf is e as a comparison states it.
func judgementOf(e Entry) judgement {
	ids := make([]string, 0, len(e.Faults))
	for _, c := range e.Faults {
		ids = append(ids, c.ID)
	}
	return judgement{Status: e.Status, Value: e.Value, Faults: ids}
}

// Walk re-asks the selected calls, launched in item order through launch,
// recording the replay under its directory alone: replay.yaml, a ledger line and
// a capture per call in the run's own shapes, its run the replay's id, and a
// comparison line per item. The three files are appended once each, in item
// order, after the walk has drained, on every exit after the walk asked
// anything.
// Each call renders its captured closed text through this binary's template for
// the capture's class and is parsed against that closed text, priced, and landed
// nowhere. At its start the replay keeps itself and the newest
// buildstore.KeptRuns-1 others of its run's replays/, as a run keeps runs. A
// per-item failure is counted, not returned: the error is a fault of the
// replay itself — among them a run
// directory that no longer stands, which the replay never recreates, and a
// replay directory removed under the walk.
func (rp *Replay) Walk(ctx context.Context, launch Launch, h ReplayHeader) (ReplayResult, error) {
	digest, err := askDigest(rp.askPass())
	if err != nil {
		return ReplayResult{}, err
	}
	h.Replay, h.Run, h.Pass, h.AskSHA256 = rp.id, rp.run, rp.askPass(), digest
	b, err := yaml.Marshal(h)
	if err != nil {
		return ReplayResult{}, err
	}
	// Mkdir and never MkdirAll: a run the prune removed fails here rather than
	// standing again as a directory no run.yaml names, which no prune removes.
	if err := os.Mkdir(filepath.Dir(rp.dir), 0o777); err != nil && !errors.Is(err, fs.ErrExist) {
		return ReplayResult{}, fmt.Errorf("start replay %s: %w", rp.id, err)
	}
	if err := os.Mkdir(rp.dir, 0o777); err != nil {
		return ReplayResult{}, fmt.Errorf("start replay %s: %w", rp.id, err)
	}
	if err := atomicfile.Write(filepath.Join(rp.dir, replayFile), b, nil); err != nil {
		return ReplayResult{}, err
	}
	if err := buildstore.PruneByStart(filepath.Dir(rp.dir), replayFile, rp.id, buildstore.KeptRuns, func(n string) bool { return runKind(n) == runKind(rp.id) }); err != nil {
		return ReplayResult{}, fmt.Errorf("prune the replays: %w", err)
	}

	l := &Ledger{dir: rp.dir, run: rp.id, backEnd: h.BackEnd, lines: map[string]Entry{}}
	defer l.Close()
	var comparisons *os.File
	defer func() {
		if comparisons != nil {
			comparisons.Close()
		}
	}()
	res := ReplayResult{Outcome: result.Done, Counts: Counts{Total: len(rp.calls)}}
	// Each call's place in the selection is its result cell, so the files take
	// their records in item order whatever order the answers came in; a cell
	// left nil was not answered.
	answers := make([]*Entry, len(rp.calls))
	calls := make([]*call, len(rp.calls))
	cut := make([]bool, len(rp.calls))
	var (
		wg      sync.WaitGroup
		walkErr error
		stopped bool
	)
	for i, captured := range rp.calls {
		c := &call{class: captured.class, closed: captured.closed}
		if c.prompt, err = c.class.render(c.closed); err != nil {
			walkErr = err
			break
		}
		calls[i] = c
		e := Entry{Model: h.Model, PromptSHA256: digestOf(c.prompt)}
		if err := launch(ctx, &wg, func(ask Ask) func() {
			answered, interrupted := c.attempt(ctx, ask, e)
			if interrupted {
				cut[i] = true
			} else {
				answers[i] = &answered
			}
			return nil
		}); err != nil {
			stopped = true
			break
		}
	}
	wg.Wait()
	if stopped || slices.Contains(cut, true) {
		res.Outcome = result.Cancelled
	}
	var recorded []attempt
	var captures, compared []any
	for i, answered := range answers {
		captured := rp.calls[i]
		it := captured.asItem()
		if answered == nil {
			if res.Outcome == result.Cancelled && res.Stopped == "" {
				res.Stopped = ItemName(it)
			}
			continue
		}
		e, c := *answered, calls[i]
		captures = append(captures, l.captureOf(it, e, *c))
		recorded = append(recorded, l.line(it, e))
		originalLine, hasLine := rp.original[captured.lineKey()]
		original := originalLine.entry()
		compared = append(compared, comparisonLine{Item: ItemName(it), Before: original.Before, Original: judgementOf(original),
			Replay: judgementOf(e), PromptChanged: e.PromptSHA256 != captured.PromptSHA256})
		switch e.Status {
		case StatusDone:
			res.Counts.Done++
		default:
			res.Counts.Failed++
		}
		// A capture with no ledger line behind it records no outcome to differ
		// from.
		if hasLine && (e.Status != original.Status || e.Value != original.Value) {
			res.Changed++
		}
	}
	res.Counts.Remaining = res.Counts.Total - res.Counts.Done - res.Counts.Failed
	// The three files take their records here, one append each in item order,
	// whatever ended the walk.
	if err := l.appendLines(&l.captures, capturesFile, captures); err != nil {
		res.Outcome = result.Failed
		return res, fmt.Errorf("capture the replay: %w", err)
	}
	if err := l.record(recorded); err != nil {
		res.Outcome = result.Failed
		return res, fmt.Errorf("record the replay: %w", err)
	}
	if err := l.appendLines(&comparisons, comparisonFile, compared); err != nil {
		res.Outcome = result.Failed
		return res, fmt.Errorf("compare the replay: %w", err)
	}
	if walkErr != nil {
		return res, walkErr
	}
	if _, err := os.Stat(rp.dir); err != nil {
		return res, fmt.Errorf("the replay directory %s no longer stands: %w", rp.dir, err)
	}
	return res, nil
}
