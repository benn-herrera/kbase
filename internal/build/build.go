// Package build walks the stage table: each stage's rows in order,
// each stage's boundary a commit, bounded by --through, with the rows that
// spend inference dropped under --no-inference. A fresh build orders its
// inputs once and every boundary records them; --resume continues the commit
// trail with the inputs it records.
package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"kbase/internal/asks"
	"kbase/internal/asks/call"
	"kbase/internal/buildstore"
	"kbase/internal/claimgraph"
	"kbase/internal/config"
	"kbase/internal/gitledger"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/result"
	"kbase/internal/statedir"
)

// startSubject is the subject of the empty commit a fresh build on a branch
// with no commit starts from: outside gitledger.Prefix, so no part of the
// trail.
const startSubject = "kb-start: the repository before the build"

// The refusal and failure classes a build names in its items, beside those
// buildstore names.
const (
	checkDirtyPaths = "dirty-paths"
	checkPandoc     = "pandoc"
	checkSource     = "source"
	checkInclude    = "include"
	checkMetadata   = "metadata-key"
	checkKBRoot     = kb.CheckKBRoot
	checkInference  = "inference"
	checkRecords    = "records"
	checkInputs     = "inputs"
)

// stepRuns is what each step of buildstore.Steps does, keyed by its id.
var stepRuns = map[string]func(*walk) (result.Record, error){
	"pre.preflight":     preflight,
	"dg.build":          documentGraph,
	"seed.graph-init":   seedSpine,
	"declared.build":    claimGraph(mechanical(claimgraph.Declared)),
	"discover.build":    claimGraph(claimgraph.Discovered),
	"equations.build":   claimGraph(mechanical(claimgraph.Equations)),
	"unmarked.build":    claimGraph(claimgraph.References),
	"cull.build":        claimGraph(claimgraph.Culled),
	"depends.attribute": claimGraph(claimgraph.Depends),
	"split.build":       claimGraph(mechanical(claimgraph.Split)),
	"p3a.gate":          validationGate,
	"p3a.stamp":         stampReadiness,
	"ov.docs":           overview,
}

func stageIDList() []string {
	ids := make([]string, len(buildstore.Stages))
	for i, s := range buildstore.Stages {
		ids[i] = s.ID
	}
	return ids
}

// Options is one build.
type Options struct {
	// VolumeRoots are the papers' top .tex files, one volume each. A relative
	// one, and a relative bibliography, resolves against the repository root.
	VolumeRoots    []string
	Bibliographies []string
	// Resume continues the trail with the inputs it records, and is given
	// none.
	Resume bool
	// Through is the last stage to walk; empty walks them all.
	Through string
	// StateDir overrides the state store's location.
	StateDir    string
	NoInference bool
	// ConfigDir is the configuration directory the invocation named, if it
	// named one, absolute; the resume command names it as given.
	ConfigDir string
	// ProjectDir is the --project-dir the invocation named, if it named one,
	// absolute; the resume command names it.
	ProjectDir string
	// AskConcurrency is the --ask-concurrency value the invocation named, 0
	// where it named none; the resume command names it too.
	AskConcurrency int
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

// Provider is where a build's model calls go: one client and the model it is
// asked for, per role — the multichoice asks and the overview passage reach their
// own endpoints independently — the invocation's one queue every call of
// every role takes a slot of, and the provenance its result states, one
// record per role in role order.
type Provider struct {
	MultichoiceClient model.Client
	Multichoice       string
	OverviewClient    model.Client
	Overview          string
	Queue             *model.Queue

	// Roles is the build's result record of where each role's asks went, one
	// per role the build resolved, in role order.
	Roles []result.Record
}

// walk is one build's state as its rows pass it along, and what its result
// reports of it.
type walk struct {
	ctx      context.Context
	opts     Options
	repo     *gitledger.Repo
	kbRoot   string
	stateDir string
	progress *progress
	// trail is the boundary commits, newest first, as this invocation
	// extends it.
	trail []gitledger.Entry
	// trailDir is the current trail's directory under the store's scratch,
	// empty until the trail holds a start entry; it is set on the stage
	// goroutine, before any later stage launches an item.
	trailDir string
	// multichoice launches the claim graph's multichoice asks, nil where this build
	// calls no model; passage launches the overview's one item.
	multichoice asks.Launch
	passage     asks.PassageLaunch

	// inStage is the stage whose rows are running.
	inStage string

	// through is the resolved bound; resumed, restored and walked are set as
	// the walk reaches them, and stay nil where it never did.
	through  string
	resumed  *bool
	restored []string
	walked   []result.Record

	// recordedBoundary is whether this invocation recorded a stage boundary.
	recordedBoundary bool

	// startedAt is the commit a fresh build began from, which its start
	// boundary records; empty on a resume.
	startedAt string

	// roles is where launch's resolved endpoints went, one record per role the
	// build resolved, in role order. Empty where no endpoint was resolved at
	// all — a build spending no inference names no role.
	roles []result.Record
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
	through := buildstore.Stages[len(buildstore.Stages)-1]
	if opts.Through != "" {
		var ok bool
		if through, ok = buildstore.ResolveStage(opts.Through); !ok {
			return w.result(buildstore.EndingOf("", result.Refusal{{Check: buildstore.CheckUsage, Key: "--through", Allowed: stageIDList(),
				Detail: fmt.Sprintf("--through %q names no stage, by id or display name", opts.Through)}}))
		}
	}
	if items := inputsUsage(opts); len(items) > 0 {
		return w.result(buildstore.EndingOf("", items))
	}
	w.through = through.ID
	if err := w.open(); err != nil {
		return w.result(buildstore.EndingOf("", err))
	}
	if !opts.Resume {
		w.opts.VolumeRoots = inRepository(w.repo.Root, opts.VolumeRoots)
		w.opts.Bibliographies = inRepository(w.repo.Root, opts.Bibliographies)
	}
	// The run's locks, in the order every run takes them: the KB's operation
	// lock, then the state store's holder lock, whose record names this run.
	release, err := Hold(w.kbRoot, w.stateDir, buildstore.KindBuild, "", opts.now())
	if err != nil {
		return w.result(buildstore.EndingOf("", err))
	}
	defer func() {
		if err := release(); err != nil {
			opts.Logger.Error("the run's locks were released and their files not removed", "error", err)
		}
	}()
	if err := readyStore(w.stateDir); err != nil {
		return w.result(buildstore.EndingOf("", err))
	}
	if w.progress, err = openProgress(w.stateDir, opts.now); err != nil {
		return w.result(buildstore.EndingOf("", err))
	}
	defer w.progress.close()
	if err := w.progress.emit(buildstore.Event{Event: buildstore.EventRun, PID: os.Getpid(), NoInference: opts.NoInference, Resume: w.resumeCommand()}); err != nil {
		return w.result(buildstore.EndingOf("", err))
	}
	var outcome string
	err = pruneBuildCaptures(w.stateDir)
	if err == nil {
		err = awaitWriters(w.kbRoot)
	}
	if err == nil {
		outcome, err = w.run(through)
	}
	end := buildstore.EndingOf(outcome, err)
	if err := w.progress.emit(end.Event()); err != nil {
		opts.Logger.Error("the progress record lost the build's end", "error", err)
	}
	return w.result(end)
}

// result is the build's document: what the walk reached, in the order the
// contract gives, then why it ended where it did not succeed.
func (w *walk) result(end buildstore.Ending) (string, []result.Field) {
	var fields []result.Field
	// walked is whether this invocation recorded a boundary: one that recorded
	// none walked no stage, and its document names neither the bound nor the
	// stages.
	walked := w.walked != nil
	if w.kbRoot != "" {
		fields = append(fields, result.Field{Key: "kb-root", Value: w.kbRoot}, result.Field{Key: "state-dir", Value: w.stateDir})
	}
	// The roles record stands where the endpoint's provenance did: one record
	// per role the build resolved, in role order. A build whose endpoints no
	// pool entry supplied — no inference at all — names none.
	if len(w.roles) > 0 {
		fields = append(fields, result.Field{Key: "roles", Value: w.roles})
	}
	if walked {
		fields = append(fields, result.Field{Key: "through", Value: w.through})
	}
	fields = append(fields, result.Field{Key: "no-inference", Value: w.opts.NoInference})
	if w.resumed != nil {
		fields = append(fields, result.Field{Key: "resumed", Value: *w.resumed})
	}
	if w.restored != nil {
		fields = append(fields, result.Field{Key: "restored", Value: w.restored})
	}
	if walked {
		fields = append(fields, result.Field{Key: "stages", Value: w.walked})
	}
	// resume names the command that continues this build, and is absent where
	// an identical re-issue continues nothing: a bounded run stops at the bound
	// its own command repeats, and a retry never walks a stage. A cancelled or
	// failed run is continued wherever it walked a stage; a refusal's only
	// where it recorded a boundary, since a refusal that recorded none wrote
	// nothing to continue.
	switch end.Outcome {
	case result.Cancelled, result.Failed, result.Refused:
		continues := walked
		if end.Outcome == result.Refused {
			continues = w.recordedBoundary
		}
		if command, confirmed := buildstore.RecordedCommand(w.resumeCommand()); continues && confirmed {
			fields = append(fields, result.Field{Key: "resume", Value: command})
		}
	}
	return end.Outcome, append(fields, end.Fields()...)
}

// open finds the repository, kb-root/ and the state store.
func (w *walk) open() error {
	repo, err := gitledger.Open(w.opts.WorkDir, buildstore.OwnedPaths, w.opts.Logger)
	var nw gitledger.NotWorktreeError
	if errors.As(err, &nw) {
		return result.Refusal{{Check: buildstore.CheckWorktree, Path: nw.Dir, Remedy: "git init", Detail: err.Error()}}
	}
	if err != nil {
		return err
	}
	w.repo, w.kbRoot = repo, filepath.Join(repo.Root, kb.KBDir)
	if w.stateDir, err = statedir.Dir(w.opts.StateDir, w.kbRoot); err != nil {
		return err
	}
	if _, err := buildstore.StoreVersion(w.stateDir); err != nil {
		return err
	}
	return os.MkdirAll(w.stateDir, 0o777)
}

// run walks the stages the trail leaves unrecorded, through the bound, and
// returns the outcome it succeeded with.
func (w *walk) run(through buildstore.Stage) (string, error) {
	trail, err := w.repo.Trail()
	if err != nil {
		return "", err
	}
	w.trail = trail
	w.setTrailDir()
	if err := w.takeInputs(); err != nil {
		return "", err
	}
	recorded := buildstore.RecordedStages(w.trail)
	if items := buildstore.TrailRefusal(w.repo.Root, recorded); len(items) > 0 {
		return "", items
	}
	last := slices.IndexFunc(buildstore.Stages, func(s buildstore.Stage) bool { return s.ID == through.ID })
	var remaining []buildstore.Stage
	for _, s := range buildstore.Stages[:last+1] {
		if recorded[s.ID] == "" {
			remaining = append(remaining, s)
		}
	}
	resumed := recorded[buildstore.StageStart] != ""
	w.resumed = &resumed
	if err := w.launch(remaining); err != nil {
		return "", err
	}
	if len(remaining) > 0 {
		if err := pruneTrails(w.stateDir, w.trailDir); err != nil {
			return "", err
		}
		if !resumed {
			if err := w.progress.rotateProgress(w.stateDir); err != nil {
				return "", err
			}
		}
	}
	if len(remaining) == 0 {
		if len(recorded) == len(buildstore.Stages) {
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
	if last == len(buildstore.Stages)-1 {
		return result.Done, nil
	}
	return result.Bounded, nil
}

// launch refuses, before any stage is walked, a walk that cannot finish: a
// volume root that is not a file, or a model call with no provider
// configured. Where a row left to walk calls a model it readies the asks,
// their evidence and answers kept under the state store's scratch.
func (w *walk) launch(remaining []buildstore.Stage) error {
	var items result.Refusal
	for _, root := range w.opts.VolumeRoots {
		if info, err := os.Stat(root); err != nil || !info.Mode().IsRegular() {
			items = append(items, result.Item{Check: buildstore.CheckUsage, Path: root, Key: "<volume-root>",
				Detail: fmt.Sprintf("volume root %s is not a file", root)})
		}
	}
	var calling *buildstore.Step
	for _, s := range remaining {
		run, _ := buildstore.StageSteps(s.ID, w.opts.NoInference)
		for i := range run {
			if calling == nil && run[i].CallsAModel() && !w.opts.NoInference {
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
			it := result.Item{Check: config.CheckProvider,
				Detail: fmt.Sprintf("%s (%s) calls a model, and %v; run kbase show-config to see the resolved configuration, or run with --no-inference", calling.ID, calling.Stage, err)}
			var lack result.Refusal
			if errors.As(err, &lack) && len(lack) == 1 {
				it.Key, it.Path = lack[0].Key, lack[0].Path
			}
			items = append(items, it)
		} else {
			w.roles = p.Roles
			trail := func() string { return w.trailDir }
			w.multichoice = call.New(p.MultichoiceClient, trail, w.opts.Logger).Launch(p.Queue, p.Multichoice)
			w.passage = call.New(p.OverviewClient, trail, w.opts.Logger).PassageLaunch(p.Queue, p.Overview)
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
func (w *walk) prepare(next buildstore.Stage, recorded map[string]string) ([]string, error) {
	if recorded[buildstore.StageStart] == "" {
		if err := kbRootGuard(w.kbRoot); err != nil {
			return nil, err
		}
	}
	if lock, err := w.repo.IndexLock(); err != nil {
		return nil, err
	} else if lock != "" {
		return nil, result.Refusal{{Check: buildstore.CheckLock, Path: lock,
			Detail: "git's index lock stands, so no boundary can be committed; a git process killed mid-commit leaves it — remove it once no git process is running in this repository"}}
	}
	dirty, err := w.repo.Dirty()
	if err != nil {
		return nil, err
	}
	if len(dirty) > 0 {
		events, err := buildstore.ReadProgress(w.stateDir)
		if err != nil {
			return nil, err
		}
		if !uncommittedIn(events, next.ID) {
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
	if recorded[buildstore.StageDocumentGraph] != "" {
		if err := w.ensureRecords(); err != nil {
			return nil, err
		}
		if err := w.loadFormat(recorded); err != nil {
			return nil, err
		}
	}
	return append([]string{}, dirty...), nil
}

// loadFormat reads the format of the KB a resume walks on past the stage that
// seeds the spine: one newer than this kbase or a degraded one is refused, and
// one older is saved in the current form before any later stage reads it, the
// save landing with the next boundary. Before that stage the KB carries no
// stamp, and the seeding stage reads the format itself.
func (w *walk) loadFormat(recorded map[string]string) error {
	if recorded[buildstore.StageSpineSeed] == "" {
		return nil
	}
	src, err := kbload.Open(w.kbRoot)
	if err != nil {
		return err
	}
	if err := index.RefuseDegraded(src); err != nil || !src.Migrated() {
		return err
	}
	w.opts.Logger.Info("saving the KB in the current metadata format", "from", src.Version(), "to", kb.FormatVersion)
	_, err = w.refresh(src)
	return err
}

// refresh is index.Refresh over src, the stamped documents a migration save
// writes put in it first.
func (w *walk) refresh(src *kb.Source) ([]string, error) {
	if err := kbdocs.Restamp(src, filepath.Base(w.repo.Root)); err != nil {
		return nil, err
	}
	return index.Refresh(src, w.opts.Logger)
}

// stage runs one stage's rows and records its boundary: its entry in the
// result names the boundary, the rows dropped and the report holding each
// row's record, written whether or not the rows held.
func (w *walk) stage(s buildstore.Stage) (result.Record, error) {
	run, dropped := buildstore.StageSteps(s.ID, w.opts.NoInference)
	report := reportPath(w.stateDir, s.ID)
	entry := result.Record{{Key: "stage", Value: s.ID}, {Key: "commit", Value: nil},
		{Key: "dropped", Value: append([]string{}, dropped...)}, {Key: "report", Value: report}}
	ran, err := w.rows(s, run)
	if werr := writeReport(report, result.Record{{Key: "stage", Value: s.ID}, {Key: "display", Value: s.Display},
		{Key: "rows", Value: ran}, {Key: "dropped", Value: append([]string{}, dropped...)}}); werr != nil && err == nil {
		err = werr
	}
	var f buildstore.Failure
	if errors.As(err, &f) && len(f) == 0 {
		err = buildstore.Failure{{Check: s.ID, Path: report, Detail: "a check of this stage found its products disagreeing; the report names each finding"}}
	}
	if err != nil {
		return entry, err
	}
	// The commit a fresh build starts from is made only once start's rows,
	// the last that can refuse, have held; only a fresh build walks start.
	if s.ID == buildstore.StageStart {
		if err := w.startFrom(); err != nil {
			return entry, err
		}
	}
	body := w.body(s.ID, dropped)
	commit, err := w.repo.Record(s.ID, s.Display, body)
	if err != nil {
		return entry, err
	}
	entry[1].Value = commit
	w.trail = append([]gitledger.Entry{{Stage: s.ID, Commit: commit, Body: body}}, w.trail...)
	w.setTrailDir()
	if err := w.progress.emit(buildstore.Event{Event: buildstore.EventRecorded, Stage: s.ID, Commit: commit}); err != nil {
		return entry, err
	}
	w.recordedBoundary = true
	return entry, nil
}

// setTrailDir points the walk at the directory of the trail's newest start
// entry, where the trail holds one.
func (w *walk) setTrailDir() {
	if i := slices.IndexFunc(w.trail, func(e gitledger.Entry) bool { return e.Stage == buildstore.StageStart }); i >= 0 {
		w.trailDir = filepath.Join(w.stateDir, buildstore.ScratchDir, buildstore.TrailsDir, w.trail[i].Commit)
	}
}

// rows runs a stage's rows in order, each one's record kept under its id.
func (w *walk) rows(s buildstore.Stage, run []buildstore.Step) ([]result.Record, error) {
	ran := []result.Record{}
	w.inStage = s.ID
	if err := w.progress.emit(buildstore.Event{Event: buildstore.EventStage, Stage: s.ID, Total: len(run)}); err != nil {
		return ran, err
	}
	for i, r := range run {
		if w.ctx.Err() != nil {
			return ran, buildstore.Cancelled{Stage: s.ID, Unit: r.ID}
		}
		if w.opts.beforeUnit != nil {
			w.opts.beforeUnit(s.ID, r.ID)
		}
		out, err := stepRuns[r.ID](w)
		ran = append(ran, append(result.Record{{Key: "row", Value: r.ID}}, out...))
		if w.ctx.Err() != nil {
			return ran, buildstore.Cancelled{Stage: s.ID, Unit: r.ID}
		}
		if err != nil {
			return ran, err
		}
		if err := w.progress.emit(buildstore.Event{Event: buildstore.EventUnit, Stage: s.ID, Unit: r.ID, Done: i + 1, Total: len(run)}); err != nil {
			return ran, err
		}
	}
	return ran, nil
}

// body is a boundary commit's body: a stage that dropped rows names them, the
// start boundary records the commit the build began from, and every boundary
// ends with the build's inputs.
func (w *walk) body(stage string, dropped []string) string {
	note := ""
	if len(dropped) > 0 {
		note = "--no-inference: this build spent no model call, so " + strings.Join(dropped, ", ") + " did not run. " +
			"Every row of this stage that costs none ran; what the dropped rows would have authored is absent from the KB, " +
			"and whatever the stage before them wrote about it stands."
	}
	startedAt := ""
	if stage == buildstore.StageStart {
		startedAt = w.startedAt
	}
	in := buildstore.Inputs{VolumeRoots: w.opts.VolumeRoots, Bibliographies: w.opts.Bibliographies}.Body(startedAt)
	if note == "" {
		return in
	}
	return note + "\n\n" + in
}

// inputsUsage refuses, before any store is resolved, an invocation whose
// inputs contradict how it continues the trail: a resume given inputs, or a
// build given neither volume roots nor --resume.
func inputsUsage(opts Options) result.Refusal {
	var items result.Refusal
	switch {
	case opts.Resume:
		if len(opts.VolumeRoots) > 0 {
			items = append(items, result.Item{Check: buildstore.CheckUsage, Key: "<volume-root>",
				Detail: "--resume continues with the volume roots the trail records, and takes none"})
		}
		if len(opts.Bibliographies) > 0 {
			items = append(items, result.Item{Check: buildstore.CheckUsage, Key: "--bibliography",
				Detail: "--resume continues with the --bibliography files the trail records, and takes none"})
		}
	case len(opts.VolumeRoots) == 0:
		items = append(items, result.Item{Check: buildstore.CheckUsage, Key: "<volume-root>",
			Detail: "a build names its volume roots, or continues the trail with --resume"})
	}
	return items
}

// inRepository is paths made absolute against the repository root where
// relative, an absolute one cleaned, and ordered as text: the one order every
// stage reads them in. It is nil where paths is, so a build given no
// bibliography discovers them.
func inRepository(root string, paths []string) []string {
	if paths == nil {
		return nil
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = kb.ProjectPath(root, p)
	}
	slices.Sort(out)
	return out
}

// takeInputs reads the trail against the invocation: a build given inputs
// over a standing trail is refused, as is a resume over an empty trail or one
// whose newest boundary records no inputs; otherwise a resume takes the
// inputs that boundary records, in the order recorded.
func (w *walk) takeInputs() error {
	recorded, ok := buildstore.Inputs{}, false
	if len(w.trail) > 0 {
		recorded, ok = buildstore.RecordedInputs(w.trail[0].Body)
	}
	switch {
	case !w.opts.Resume && len(w.trail) == 0:
		return nil
	case !w.opts.Resume:
		remedy, err := w.startOverRemedy(true, ok)
		if err != nil {
			return err
		}
		detail := "a kb-build: trail stands, written before boundaries recorded their inputs"
		if ok {
			detail = "a kb-build: trail stands, recording volume roots [" + strings.Join(recorded.VolumeRoots, ", ") + "]"
			if recorded.Bibliographies != nil {
				detail += " and bibliographies [" + strings.Join(recorded.Bibliographies, ", ") + "]"
			}
		}
		return result.Refusal{{Check: checkInputs, Key: "<volume-root>", Path: w.repo.Root, Remedy: remedy,
			Detail: detail + "; a build over other inputs starts over"}}
	case len(w.trail) == 0:
		return result.Refusal{{Check: buildstore.CheckTrail, Key: "--resume", Path: w.repo.Root,
			Remedy: "kbase build <volume-root>...", Detail: "no kb-build: trail to resume"}}
	case !ok:
		remedy, err := w.startOverRemedy(false, false)
		if err != nil {
			return err
		}
		return result.Refusal{{Check: checkInputs, Key: "--resume", Path: w.repo.Root, Remedy: remedy,
			Detail: "the trail's newest boundary records no inputs: it was written before boundaries carried them"}}
	}
	root := kb.ResolvePath(w.repo.Root)
	w.opts.VolumeRoots = recordedPaths(root, recorded.VolumeRoots)
	w.opts.Bibliographies = recordedPaths(root, recorded.Bibliographies)
	return nil
}

// recordedPaths is paths as a trail records them: an absolute one as written,
// a relative one — written before boundaries recorded them absolute — joined
// to the repository root.
func recordedPaths(root string, paths []string) []string {
	if paths == nil {
		return nil
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = kb.ProjectPath(root, filepath.FromSlash(p))
	}
	return out
}

// startOverRemedy is the remedy naming the trail's start-over commit: the
// started-at its start boundary records; else that boundary's parent; else
// none, and a fresh clone is the way to start over. rewindable names
// rewind-build's started-at target beside a recorded started-at, and
// resumable adds the resume where the trail can be continued.
func (w *walk) startOverRemedy(rewindable, resumable bool) (string, error) {
	start, startedAt := buildstore.StartedAt(w.trail)
	remedy := "rebuild in a fresh clone"
	switch {
	case startedAt != "" && rewindable:
		remedy = "start over: `rewind-build`'s `started-at` target, or `git reset --hard " + startedAt + "`"
	case startedAt != "":
		remedy = "start over: `git reset --hard " + startedAt + "`"
	case start.Commit != "":
		parent, err := w.repo.Parent(start.Commit)
		if err != nil {
			return "", err
		}
		if parent != "" {
			remedy = "start over: `git reset --hard " + parent + "`"
		}
	}
	if resumable {
		return "continue with `kbase build --resume`, or " + remedy, nil
	}
	return remedy, nil
}

// startFrom is where a fresh build begins: the commit HEAD names, or, on a
// branch with no commit, an empty commit made for it, so its start boundary
// has a commit to record and a start over has one to return to.
func (w *walk) startFrom() error {
	head, err := w.repo.Head()
	if err == nil && head == "" {
		head, err = w.repo.Commit(startSubject, "")
	}
	w.startedAt = head
	return err
}

// resumeCommand is the invocation that continues this build: --resume and
// every flag the launch carried but the bound, since resuming past it is what
// resuming is for, and the inputs, which the trail records.
func (w *walk) resumeCommand() []string {
	argv := []string{"kbase", "build", "--resume"}
	if w.opts.NoInference {
		argv = append(argv, "--no-inference")
	}
	if w.opts.AskConcurrency != 0 {
		argv = append(argv, "--ask-concurrency", strconv.Itoa(w.opts.AskConcurrency))
	}
	if w.opts.StateDir != "" {
		argv = append(argv, "--state-dir", w.stateDir)
	}
	if w.opts.ConfigDir != "" {
		argv = append(argv, "--config-dir", w.opts.ConfigDir)
	}
	if w.opts.ProjectDir != "" {
		argv = append(argv, "--project-dir", w.opts.ProjectDir)
	}
	return argv
}
