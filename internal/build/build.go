// Package build walks kb_tools' stage table: each stage's rows in order,
// each stage's boundary a commit, bounded by --through, with the rows that
// spend inference dropped under --no-inference. Whether an invocation opens
// a build or resumes one is read off the commit trail alone.
package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"kbase/internal/asks"
	"kbase/internal/asks/call"
	"kbase/internal/claimgraph"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/ledger"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/result"
)

// Stage is one stage of the build: its id and what it is for.
type Stage struct {
	ID, Display string
}

// Stages is the build's stage vocabulary in walk order. kb_tools' retired
// phase-5 is never given to a stage: its ledgers would read it as recorded.
var Stages = []Stage{
	{"start", "build started"},
	{"document-graph", "document tree derived"},
	{"spine-seed", "claim-graph spine seeded"},
	{"claims-declared", "declared claim graph"},
	{"claims-discovered", "claim discovery"},
	{"equations-minted", "equation nodes minted"},
	{"references-found", "unmarked references found"},
	{"depends-attributed", "dependency attribution"},
	{"phase-3a", "validation gate"},
	{"overview-drafted", "overview drafted"},
}

const (
	stageStart         = "start"
	stageDocumentGraph = "document-graph"
	stageSpineSeed     = "spine-seed"
)

// charterFile is where a build keeps its charter, at the repository root.
const charterFile = "kb-build-charter.md"

// ownedPaths are the root-relative paths a build commits and restores: the
// build records under every spelling, so a migration's removal of the old
// one is committed with the stage that saved it.
var ownedPaths = append([]string{kb.KBDir, charterFile}, kbload.RecordFiles()...)

// The start boundary's body: the charter it was given, or the absence stated.
const (
	charterField = "charter:"
	noCharter    = "charter: none — this build was given none and runs on its sources"
)

// Every boundary's body ends with the build's inputs, one line per path.
const (
	volumeRootField   = "volume-root:"
	bibliographyField = "bibliography:"
)

// The refusal and failure classes a build names in its items.
const (
	checkUsage      = "usage"
	checkWorktree   = "worktree"
	checkLock       = "lock"
	checkDirtyPaths = "dirty-paths"
	checkPandoc     = "pandoc"
	checkSource     = "source"
	checkInclude    = "include"
	checkMetadata   = "metadata-key"
	checkProvider   = "provider"
	checkKBRoot     = "kb-root"
	checkCharter    = "charter"
	checkInference  = "inference"
	checkRecords    = "records"
	checkStateDir   = "state-dir"
	checkInputs     = "inputs"
	checkError      = "error"
)

// row is one step of a stage, and one unit of its progress. A row spends
// inference where it makes a model call under a system prompt, or where its
// whole work is a model call made inside the tool it runs; it calls a model,
// too, where part of its work does.
type row struct {
	id, stage             string
	system                string
	spendsOwnInference    bool
	spendsInferenceInPart bool
	run                   func(*walk) (result.Record, error)
}

func (r row) spendsInference() bool { return r.spendsOwnInference || r.system != "" }

func (r row) callsAModel() bool { return r.spendsInference() || r.spendsInferenceInPart }

// rows is the step table in walk order.
var rows = []row{
	{id: "pre.preflight", stage: stageStart, run: preflight},
	{id: "pre.charter", stage: stageStart, run: charter},
	{id: "dg.build", stage: stageDocumentGraph, run: documentGraph},
	{id: "seed.graph-init", stage: "spine-seed", run: seedSpine},
	{id: "declared.build", stage: "claims-declared", run: claimGraph(mechanical(claimgraph.Declared))},
	{id: "discover.build", stage: "claims-discovered", spendsOwnInference: true, run: claimGraph(claimgraph.Discovered)},
	{id: "equations.build", stage: "equations-minted", run: claimGraph(mechanical(claimgraph.Equations))},
	{id: "unmarked.build", stage: "references-found", spendsOwnInference: true, run: claimGraph(claimgraph.References)},
	{id: "depends.attribute", stage: "depends-attributed", spendsInferenceInPart: true, run: claimGraph(claimgraph.Depends)},
	{id: "p3a.gate", stage: "phase-3a", run: validationGate},
	{id: "p3a.stamp", stage: "phase-3a", run: stampReadiness},
	{id: "ov.docs", stage: "overview-drafted", system: asks.OverviewSystem, run: overview},
}

// applies is whether a row runs: a run spending no inference drops every row
// that would spend it and walks the rest.
func (r row) applies(noInference bool) bool { return !noInference || !r.spendsInference() }

// stageRows is a stage's rows that run in this build, and those it drops.
func stageRows(stage string, noInference bool) (run []row, dropped []string) {
	for _, r := range rows {
		switch {
		case r.stage != stage:
		case r.applies(noInference):
			run = append(run, r)
		default:
			dropped = append(dropped, r.id)
		}
	}
	return run, dropped
}

// ResolveStage is the stage name names, by id or display name, whitespace
// collapsed and case ignored.
func ResolveStage(name string) (Stage, bool) {
	wanted := strings.ToLower(strings.Join(strings.Fields(name), " "))
	for _, s := range Stages {
		if wanted == strings.ToLower(s.ID) || wanted == strings.ToLower(s.Display) {
			return s, true
		}
	}
	return Stage{}, false
}

func stageIDList() []string {
	ids := make([]string, len(Stages))
	for i, s := range Stages {
		ids[i] = s.ID
	}
	return ids
}

// Options is one build.
type Options struct {
	// VolumeRoots are the papers' top .tex files, one volume each.
	VolumeRoots    []string
	Bibliographies []string
	// Charter is a file stating the build's scope, kept at the repository
	// root as the build's charter when it opens.
	Charter string
	// Through is the last stage to walk; empty walks them all.
	Through string
	// StateDir overrides the state store's location.
	StateDir    string
	NoInference bool
	// ConfigDir is the configuration directory the invocation named, if it
	// named one; the resume command names it too.
	ConfigDir string
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	// Provider is where the build's model calls go, or what the configuration
	// lacks for them.
	Provider func() (Provider, error)
	Logger   log.Logger

	// beforeUnit runs as each unit starts.
	beforeUnit func(stage, unit string)
	// now is the clock the progress record reads.
	now func() time.Time
}

// Provider is where a build's model calls go: the client, the model the
// claim graph's letter asks and the overview passage are each put to, and
// how many asks of one group are in flight at once — 0 for the default.
type Provider struct {
	Client            model.Client
	Letters, Overview string
	ReaderConcurrency int
}

// walk is one build's state as its rows pass it along, and what its result
// reports of it.
type walk struct {
	ctx      context.Context
	opts     Options
	repo     *ledger.Repo
	kbRoot   string
	stateDir string
	progress *progress
	// trail is the boundary commits, newest first, as this invocation
	// extends it.
	trail []ledger.Entry
	// letters puts the claim graph's letter asks, nil where this build calls
	// no model; passage asks the overview passage from excerpts, rejected
	// holding the lines a previous reply was refused for and attempt
	// counting the asks from 1.
	letters           asks.LetterReader
	readerConcurrency int
	passage           func(ctx context.Context, excerpts string, rejected []string, attempt int) (string, error)

	// inStage is the stage whose rows are running.
	inStage string

	// through is the resolved bound; resumed, restored and walked are set as
	// the walk reaches them, and stay nil where it never did.
	through  string
	resumed  *bool
	restored []string
	walked   []result.Record
}

// retry is a live holder of what the build needs.
type retry []result.Item

func (r retry) Error() string { return result.Items(r).Error() }

// failure is a row whose own checks found its products disagreeing, or a
// product a later step found wrong, every finding named. A row that leaves it
// empty has its stage name the report holding the findings.
type failure []result.Item

func (f failure) Error() string {
	if len(f) == 0 {
		return "a check failed"
	}
	return result.Items(f).Error()
}

// cancelled is a build stopped on request, the unit in flight abandoned.
type cancelled struct{ stage, unit string }

func (c cancelled) Error() string { return fmt.Sprintf("cancelled in %s at %s", c.stage, c.unit) }

// ending is how a build ended: its outcome, and the items or the unit in
// flight that say why where it did not succeed.
type ending struct {
	outcome string
	items   []result.Item
	at      cancelled
}

// ended classifies a walk's error; success is the outcome given.
func ended(success string, err error) ending {
	var (
		r result.Refusal
		t retry
		f failure
		c cancelled
	)
	switch {
	case err == nil:
		return ending{outcome: success}
	case errors.As(err, &r):
		return ending{outcome: result.Refused, items: r}
	case errors.As(err, &t):
		return ending{outcome: result.Retry, items: t}
	case errors.As(err, &c):
		return ending{outcome: result.Cancelled, at: c}
	case errors.As(err, &f):
		return ending{outcome: result.Failed, items: f}
	}
	return ending{outcome: result.Failed, items: []result.Item{{Check: checkError, Detail: err.Error()}}}
}

// fields is the key that closes the result document, none on success.
func (e ending) fields() []result.Field {
	switch e.outcome {
	case result.Refused, result.Retry:
		return []result.Field{{Key: result.RefusalsKey, Value: e.items}}
	case result.Failed:
		return []result.Field{{Key: result.FailuresKey, Value: e.items}}
	case result.Cancelled:
		return []result.Field{{Key: result.CancelledKey, Value: result.Record{{Key: "stage", Value: e.at.stage}, {Key: "unit", Value: e.at.unit}}}}
	}
	return nil
}

// event is the progress record's end of the invocation that ended so.
func (e ending) event() event {
	return event{Event: eventEnd, Outcome: e.outcome, Items: e.items, Stage: e.at.stage, Unit: e.at.unit}
}

// Run walks the stages from where the commit trail stands through
// opts.Through and returns the result's outcome and fields.
func Run(ctx context.Context, opts Options) (string, []result.Field) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	if opts.now == nil {
		opts.now = time.Now
	}
	w := &walk{ctx: ctx, opts: opts}
	through := Stages[len(Stages)-1]
	if opts.Through != "" {
		var ok bool
		if through, ok = ResolveStage(opts.Through); !ok {
			return w.result(ended("", result.Refusal{{Check: checkUsage, Key: "--through", Allowed: stageIDList(),
				Detail: fmt.Sprintf("--through %q names no stage, by id or display name", opts.Through)}}))
		}
	}
	w.through = through.ID
	if err := w.open(); err != nil {
		return w.result(ended("", err))
	}
	runLock, err := takeRunLock(w.kbRoot, w.stateDir)
	if err != nil {
		return w.result(ended("", err))
	}
	defer func() {
		if err := runLock.ReleaseRemoving(); err != nil {
			opts.Logger.Error("the run lock was released and its file not removed", "error", err)
		}
	}()
	lock, err := takeLock(w.stateDir, opts.now())
	if err != nil {
		return w.result(ended("", err))
	}
	defer lock.Release()
	if err := readyStore(w.stateDir); err != nil {
		return w.result(ended("", err))
	}
	if w.progress, err = openProgress(w.stateDir, opts.now); err != nil {
		return w.result(ended("", err))
	}
	defer w.progress.close()
	if err := w.progress.emit(event{Event: eventRun, PID: os.Getpid(), NoInference: opts.NoInference, Resume: w.resumeCommand()}); err != nil {
		return w.result(ended("", err))
	}
	var outcome string
	err = pruneCaptures(w.stateDir)
	if err == nil {
		err = awaitWriters(w.kbRoot)
	}
	if err == nil {
		outcome, err = w.run(through)
	}
	end := ended(outcome, err)
	if err := w.progress.emit(end.event()); err != nil {
		opts.Logger.Error("the progress record lost the build's end", "error", err)
	}
	return w.result(end)
}

// result is the build's document: what the walk reached, in the order the
// contract gives, then why it ended where it did not succeed.
func (w *walk) result(end ending) (string, []result.Field) {
	var fields []result.Field
	if w.kbRoot != "" {
		fields = append(fields, result.Field{Key: "kb-root", Value: w.kbRoot}, result.Field{Key: "state-dir", Value: w.stateDir})
	}
	if w.through != "" {
		fields = append(fields, result.Field{Key: "through", Value: w.through})
	}
	fields = append(fields, result.Field{Key: "no-inference", Value: w.opts.NoInference})
	if w.resumed != nil {
		fields = append(fields, result.Field{Key: "resumed", Value: *w.resumed})
	}
	if w.restored != nil {
		fields = append(fields, result.Field{Key: "restored", Value: w.restored})
	}
	if w.walked != nil {
		fields = append(fields, result.Field{Key: "stages", Value: w.walked})
	}
	if w.kbRoot != "" && end.outcome != result.Done && end.outcome != result.Unchanged {
		fields = append(fields, result.Field{Key: "resume", Value: shellJoin(w.resumeCommand())})
	}
	return end.outcome, append(fields, end.fields()...)
}

// open finds the repository, kb-root/ and the state store.
func (w *walk) open() error {
	repo, err := ledger.Open(w.opts.WorkDir, ownedPaths, w.opts.Logger)
	var nw ledger.NotWorktreeError
	if errors.As(err, &nw) {
		return result.Refusal{{Check: checkWorktree, Path: nw.Dir, Remedy: "git init", Detail: err.Error()}}
	}
	if err != nil {
		return err
	}
	w.repo, w.kbRoot = repo, filepath.Join(repo.Root, kb.KBDir)
	if w.stateDir, err = stateDir(w.opts.StateDir, w.kbRoot); err != nil {
		return err
	}
	if _, err := storeVersion(w.stateDir); err != nil {
		return err
	}
	return os.MkdirAll(w.stateDir, 0o777)
}

// run walks the stages the trail leaves unrecorded, through the bound, and
// returns the outcome it succeeded with.
func (w *walk) run(through Stage) (string, error) {
	trail, err := w.repo.Trail()
	if err != nil {
		return "", err
	}
	w.trail = trail
	if len(trail) > 0 {
		if was, ok := recordedInputs(trail[0].Body); ok {
			if items := inputsRefusal(w.repo.Root, was, w.inputs()); len(items) > 0 {
				return "", items
			}
		}
	}
	recorded := w.recorded()
	last := slices.IndexFunc(Stages, func(s Stage) bool { return s.ID == through.ID })
	var remaining []Stage
	for _, s := range Stages[:last+1] {
		if recorded[s.ID] == "" {
			remaining = append(remaining, s)
		}
	}
	resumed := recorded[stageStart] != ""
	w.resumed = &resumed
	if err := w.launch(remaining); err != nil {
		return "", err
	}
	if len(remaining) == 0 {
		if len(recorded) == len(Stages) {
			return result.Unchanged, nil
		}
		return result.Bounded, nil
	}
	restored, err := w.prepare(remaining[0], recorded)
	if err != nil {
		return "", err
	}
	w.restored = restored
	w.walked = []result.Record{}
	for _, s := range remaining {
		rec, err := w.stage(s)
		w.walked = append(w.walked, rec)
		if err != nil {
			return "", err
		}
	}
	if last == len(Stages)-1 {
		return result.Done, nil
	}
	return result.Bounded, nil
}

// recorded is each stage the trail records, with its newest boundary
// commit. An id outside the vocabulary reads as no stage.
func (w *walk) recorded() map[string]string {
	recorded := map[string]string{}
	for _, e := range w.trail {
		known := slices.ContainsFunc(Stages, func(s Stage) bool { return s.ID == e.Stage })
		if known && recorded[e.Stage] == "" {
			recorded[e.Stage] = e.Commit
		}
	}
	return recorded
}

// namedSetting is a provider error naming the setting the configuration
// lacks: the variable that would supply it and the file that otherwise does.
type namedSetting interface{ Named() (key, path string) }

// launch refuses, before any stage is walked, a walk that cannot finish: a
// volume root that is not a file, or a model call with no provider
// configured. Where a row left to walk calls a model it readies the asks,
// their evidence and answers kept under the state store's scratch.
func (w *walk) launch(remaining []Stage) error {
	var items result.Refusal
	for _, root := range w.opts.VolumeRoots {
		if info, err := os.Stat(w.absolute(root)); err != nil || !info.Mode().IsRegular() {
			items = append(items, result.Item{Check: checkUsage, Path: root, Key: "<volume-root>",
				Detail: fmt.Sprintf("volume root %s is not a file", root)})
		}
	}
	var calling *row
	for _, s := range remaining {
		run, _ := stageRows(s.ID, w.opts.NoInference)
		for i := range run {
			if calling == nil && run[i].callsAModel() && !w.opts.NoInference {
				calling = &run[i]
			}
		}
	}
	if calling != nil {
		var p Provider
		err := errors.New("no provider is configured")
		if w.opts.Provider != nil {
			p, err = w.opts.Provider()
		}
		if err != nil {
			it := result.Item{Check: checkProvider, Remedy: "kbase configure",
				Detail: fmt.Sprintf("%s (%s) calls a model, and %v; configure one, or run with --no-inference", calling.id, calling.stage, err)}
			var lack namedSetting
			if errors.As(err, &lack) {
				it.Key, it.Path = lack.Named()
			}
			items = append(items, it)
		} else {
			caller := call.New(p.Client, filepath.Join(w.stateDir, scratchDir), w.opts.Logger)
			w.letters, w.readerConcurrency = caller.Letters(p.Letters), p.ReaderConcurrency
			w.passage = func(ctx context.Context, excerpts string, rejected []string, attempt int) (string, error) {
				return caller.Passage(ctx, p.Overview, excerpts, rejected, attempt)
			}
		}
	}
	if len(items) > 0 {
		return items
	}
	return nil
}

// prepare readies the worktree for the walk: a fresh build refuses a
// kb-root/ something is in; a lock git left in the repository is refused by
// name; dirt in the paths the build owns is restored to the last boundary
// where it is the interrupted stage's own uncommitted work, and refused
// otherwise; a resume regenerates missing records.
func (w *walk) prepare(next Stage, recorded map[string]string) ([]string, error) {
	if recorded[stageStart] == "" {
		if err := kbRootGuard(w.kbRoot); err != nil {
			return nil, err
		}
	}
	if lock, err := w.repo.IndexLock(); err != nil {
		return nil, err
	} else if lock != "" {
		return nil, result.Refusal{{Check: checkLock, Path: lock,
			Detail: "git's index lock stands, so no boundary can be committed; a git process killed mid-commit leaves it — remove it once no git process is running in this repository"}}
	}
	dirty, err := w.repo.Dirty()
	if err != nil {
		return nil, err
	}
	if len(dirty) > 0 {
		events, err := readProgress(w.stateDir)
		if err != nil {
			return nil, err
		}
		if !interrupted(events, next.ID) {
			items := make(result.Refusal, len(dirty))
			for i, p := range dirty {
				items[i] = result.Item{Check: checkDirtyPaths, Path: p,
					Detail: "a path this build owns is dirty and no interrupted stage of it accounts for it; commit it, or restore it to the last kb-build: commit"}
			}
			return nil, items
		}
		last := ""
		if len(w.trail) > 0 {
			last = w.trail[0].Commit
		}
		w.opts.Logger.Info("restoring the interrupted stage's paths to the last boundary", "stage", next.ID, "commit", last, "paths", len(dirty))
		if err := w.repo.Restore(last); err != nil {
			return nil, err
		}
	}
	if recorded[stageDocumentGraph] != "" {
		if err := w.ensureRecords(); err != nil {
			return nil, err
		}
		if err := w.loadFormat(recorded); err != nil {
			return nil, err
		}
	}
	return append([]string{}, dirty...), nil
}

// loadFormat reads the format of the KB a resume walks on: one newer than
// this kbase is refused; one older, past the stage that stamps the spine,
// is saved in the current form before any later stage reads it, the save
// landing with the next boundary.
func (w *walk) loadFormat(recorded map[string]string) error {
	src, err := kbload.Open(w.kbRoot)
	if err != nil || !src.Migrated() || recorded[stageSpineSeed] == "" {
		return err
	}
	w.opts.Logger.Info("saving the KB in the current metadata format", "from", src.Version(), "to", kb.FormatVersion)
	_, err = index.Refresh(src, w.opts.Logger)
	return err
}

// stage runs one stage's rows and records its boundary: its entry in the
// result names the boundary, the rows dropped and the report holding each
// row's record, written whether or not the rows held.
func (w *walk) stage(s Stage) (result.Record, error) {
	run, dropped := stageRows(s.ID, w.opts.NoInference)
	report := reportPath(w.stateDir, s.ID)
	entry := result.Record{{Key: "stage", Value: s.ID}, {Key: "commit", Value: nil},
		{Key: "dropped", Value: append([]string{}, dropped...)}, {Key: "report", Value: report}}
	ran, err := w.rows(s, run)
	if werr := writeReport(report, result.Record{{Key: "stage", Value: s.ID}, {Key: "display", Value: s.Display},
		{Key: "rows", Value: ran}, {Key: "dropped", Value: append([]string{}, dropped...)}}); werr != nil && err == nil {
		err = werr
	}
	var f failure
	if errors.As(err, &f) && len(f) == 0 {
		err = failure{{Check: s.ID, Path: report, Detail: "a check of this stage found its products disagreeing; the report names each finding"}}
	}
	if err != nil {
		return entry, err
	}
	body := w.body(s.ID, dropped)
	commit, err := w.repo.Record(s.ID, s.Display, body)
	if err != nil {
		return entry, err
	}
	entry[1].Value = commit
	w.trail = append([]ledger.Entry{{Stage: s.ID, Commit: commit, Body: body}}, w.trail...)
	if err := w.progress.emit(event{Event: eventRecorded, Stage: s.ID, Commit: commit}); err != nil {
		return entry, err
	}
	return entry, nil
}

// rows runs a stage's rows in order, each one's record kept under its id.
func (w *walk) rows(s Stage, run []row) ([]result.Record, error) {
	ran := []result.Record{}
	w.inStage = s.ID
	if err := w.progress.emit(event{Event: eventStage, Stage: s.ID, Total: len(run)}); err != nil {
		return ran, err
	}
	for i, r := range run {
		if w.ctx.Err() != nil {
			return ran, cancelled{s.ID, r.id}
		}
		if w.opts.beforeUnit != nil {
			w.opts.beforeUnit(s.ID, r.id)
		}
		out, err := r.run(w)
		ran = append(ran, append(result.Record{{Key: "row", Value: r.id}}, out...))
		if w.ctx.Err() != nil {
			return ran, cancelled{s.ID, r.id}
		}
		if err != nil {
			return ran, err
		}
		if err := w.progress.emit(event{Event: eventUnit, Stage: s.ID, Unit: r.id, Done: i + 1, Total: len(run)}); err != nil {
			return ran, err
		}
	}
	return ran, nil
}

// body is a boundary commit's body: the start boundary names its charter, a
// stage that dropped rows names them, and every boundary ends with the
// build's inputs.
func (w *walk) body(stage string, dropped []string) string {
	note := ""
	switch {
	case stage == stageStart && kb.IsFile(filepath.Join(w.repo.Root, charterFile)):
		note = charterField + " " + charterFile
	case stage == stageStart:
		note = noCharter
	case len(dropped) > 0:
		note = "--no-inference: this build spent no model call, so " + strings.Join(dropped, ", ") + " did not run. " +
			"Every row of this stage that costs none ran; what the dropped rows would have authored is absent from the KB, " +
			"and whatever the stage before them wrote about it stands."
	}
	in := w.inputs().body()
	if note == "" {
		return in
	}
	return note + "\n\n" + in
}

// inputs is what a build is built from: the volume roots and the
// bibliographies given, each in the order given.
type inputs struct{ roots, bibliographies []string }

// inputs is this run's, each path repository-relative with forward slashes.
func (w *walk) inputs() inputs {
	var in inputs
	root := kb.ResolvePath(w.repo.Root)
	for _, p := range w.opts.VolumeRoots {
		in.roots = append(in.roots, repoRelative(root, w.absolute(p)))
	}
	for _, p := range w.opts.Bibliographies {
		in.bibliographies = append(in.bibliographies, repoRelative(root, w.absolute(p)))
	}
	return in
}

// repoRelative is the absolute path abs relative to the symlink-resolved
// repository root, abs resolved as far as it exists. A path that does not
// resolve is compared as given, so launch refuses it only on a fresh build:
// a resume compares inputs before launch and refuses it as an input the
// trail does not record.
func repoRelative(root, abs string) string {
	abs = kb.ResolvePath(abs)
	if rel, err := filepath.Rel(root, abs); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(abs)
}

func (in inputs) body() string {
	var lines []string
	for _, p := range in.roots {
		lines = append(lines, volumeRootField+" "+p)
	}
	for _, p := range in.bibliographies {
		lines = append(lines, bibliographyField+" "+p)
	}
	return strings.Join(lines, "\n")
}

// recordedInputs is the inputs a boundary's body records; ok is false where
// it records none, as a boundary written before bodies carried them does not.
func recordedInputs(body string) (in inputs, ok bool) {
	for _, line := range strings.Split(body, "\n") {
		if p, found := strings.CutPrefix(line, volumeRootField+" "); found {
			in.roots = append(in.roots, p)
		} else if p, found := strings.CutPrefix(line, bibliographyField+" "); found {
			in.bibliographies = append(in.bibliographies, p)
		}
	}
	return in, len(in.roots) > 0
}

// inputsRefusal is one item per input this run was given otherwise than the
// trail records it, order included; none where every input matches.
func inputsRefusal(repoRoot string, recorded, given inputs) result.Refusal {
	var items result.Refusal
	for _, c := range []struct {
		key, noun string
		was, now  []string
	}{
		{"<volume-root>", "volume roots", recorded.roots, given.roots},
		{"--bibliography", "bibliographies", recorded.bibliographies, given.bibliographies},
	} {
		if slices.Equal(c.was, c.now) {
			continue
		}
		change := "the same paths in another order"
		var parts []string
		if added := missingFrom(c.was, c.now); len(added) > 0 {
			parts = append(parts, "added "+strings.Join(added, ", "))
		}
		if removed := missingFrom(c.now, c.was); len(removed) > 0 {
			parts = append(parts, "removed "+strings.Join(removed, ", "))
		}
		if len(parts) > 0 {
			change = strings.Join(parts, "; ")
		}
		items = append(items, result.Item{Check: checkInputs, Path: repoRoot, Key: c.key,
			Remedy: "resume with the inputs the trail records, or start over from a commit before the trail",
			Detail: fmt.Sprintf("the kb-build: trail records %s [%s] and this run was given [%s]: %s",
				c.noun, strings.Join(c.was, ", "), strings.Join(c.now, ", "), change)})
	}
	return items
}

// missingFrom is every path of paths that set lacks, in paths' order.
func missingFrom(set, paths []string) []string {
	var out []string
	for _, p := range paths {
		if !slices.Contains(set, p) {
			out = append(out, p)
		}
	}
	return out
}

// absolute is p as the invocation's working directory resolves it.
func (w *walk) absolute(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(w.opts.WorkDir, p)
}

// resumeCommand is the invocation that continues this build: every flag the
// launch carried but the bound, since resuming past it is what resuming is
// for.
func (w *walk) resumeCommand() []string {
	argv := []string{"kbase", "build"}
	for _, root := range w.opts.VolumeRoots {
		argv = append(argv, w.absolute(root))
	}
	for _, b := range w.opts.Bibliographies {
		argv = append(argv, "--bibliography", w.absolute(b))
	}
	if w.opts.Charter != "" {
		argv = append(argv, "--charter", w.absolute(w.opts.Charter))
	}
	if w.opts.NoInference {
		argv = append(argv, "--no-inference")
	}
	if w.opts.StateDir != "" {
		argv = append(argv, "--state-dir", w.stateDir)
	}
	if w.opts.ConfigDir != "" {
		argv = append(argv, "--config-dir", w.absolute(w.opts.ConfigDir))
	}
	return argv
}
