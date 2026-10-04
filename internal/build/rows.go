package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/atomicfile"
	"kbase/internal/claimgraph"
	"kbase/internal/docgraph"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/latex/pandoc"
	"kbase/internal/records"
	"kbase/internal/result"
)

// preflight is pandoc's version range.
func preflight(w *walk) (result.Record, error) {
	version, err := pandoc.Preflight(w.ctx)
	if err != nil {
		return nil, pandocFailure(err)
	}
	return result.Record{{Key: "pandoc", Value: version}}, nil
}

// pandocFailure is a missing or out-of-range pandoc refused, naming what to
// install; anything else as it is.
func pandocFailure(err error) error {
	var missing pandoc.MissingError
	var bad pandoc.VersionError
	if errors.As(err, &missing) || errors.As(err, &bad) {
		return refusal{{Check: checkPandoc, Detail: err.Error()}}
	}
	return err
}

// charter keeps the charter the build was given at the repository root,
// where the start boundary names it. Given none, a charter already standing
// there is the build's, as kb_tools reads its default charter path.
func charter(w *walk) (result.Record, error) {
	target := filepath.Join(w.repo.Root, charterFile)
	if w.opts.Charter != "" {
		text, err := os.ReadFile(w.opts.Charter)
		if err != nil {
			return nil, refusal{{Check: checkCharter, Path: w.opts.Charter, Key: "--charter", Detail: fmt.Sprintf("--charter %s cannot be read: %v", w.opts.Charter, err)}}
		}
		if err := atomicfile.Write(target, text, nil); err != nil {
			return nil, err
		}
	}
	if !kb.IsFile(target) {
		return result.Record{{Key: "charter", Value: nil}}, nil
	}
	return result.Record{{Key: "charter", Value: charterFile}}, nil
}

// The kb-root/ tri-state: absent, holding only the derived index, or holding
// something a build could destroy.
const (
	kbRootAbsent    = "absent"
	kbRootSpineOnly = "spine-only"
	kbRootPopulated = "populated"
)

func kbRootState(kbRoot string) (string, error) {
	entries, err := os.ReadDir(kbRoot)
	if errors.Is(err, os.ErrNotExist) {
		return kbRootAbsent, nil
	}
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Name() != kb.IndexDir {
			return kbRootPopulated, nil
		}
	}
	return kbRootSpineOnly, nil
}

// kbRootGuard refuses to open a build over a kb-root/ something is in.
func kbRootGuard(kbRoot string) error {
	state, err := kbRootState(kbRoot)
	if err != nil {
		return err
	}
	if state == kbRootPopulated {
		return refusal{{Check: checkKBRoot, Path: kbRoot,
			Detail: fmt.Sprintf("%s is %s: it holds documents and this repository has no kb-build: trail to resume; move that tree aside to build afresh", kbRoot, state)}}
	}
	return nil
}

// bibliographies is the set the reader is offered: the build's own, or, given
// none, every .bib in the volume root's directory in sorted path order.
func (w *walk) bibliographies() ([]string, error) {
	if w.opts.Bibliographies != nil {
		return w.opts.Bibliographies, nil
	}
	dir := filepath.Dir(w.opts.VolumeRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var beside []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".bib" && !strings.HasPrefix(e.Name(), ".") {
			beside = append(beside, filepath.Join(dir, e.Name()))
		}
	}
	slices.Sort(beside)
	return beside, nil
}

// documentGraph builds the tree and writes its records into the state store.
func documentGraph(w *walk) (result.Record, error) {
	bibs, err := w.bibliographies()
	if err != nil {
		return nil, err
	}
	report, err := docgraph.Build(w.ctx, w.opts.Logger, docgraph.Options{
		VolumeRoot: w.opts.VolumeRoot, Bibliographies: bibs, KBRoot: w.kbRoot,
	})
	if err != nil {
		return nil, documentGraphFailure(w.opts.VolumeRoot, err)
	}
	if err := docgraph.WriteRecords(w.stateDir, report); err != nil {
		return nil, err
	}
	rec := result.Record{{Key: "documents", Value: report.Documents}, {Key: "records", Value: len(report.Records)}}
	if len(report.UnreadBibliographies) > 0 {
		rec = append(rec, result.Field{Key: "unread-bibliographies", Value: report.UnreadBibliographies})
	}
	rec = append(rec, result.Field{Key: "checks", Value: checks(report)})
	if report.Failed() {
		return rec, failure(failedChecks(report))
	}
	return rec, nil
}

func checks(report docgraph.Report) []string {
	out := make([]string, len(report.Findings))
	for i, f := range report.Findings {
		out[i] = f.String()
	}
	return out
}

// failedChecks is each failing check of the document graph as an item.
func failedChecks(report docgraph.Report) []result.Item {
	var out []result.Item
	for _, f := range report.Findings {
		if f.Status == docgraph.StatusFail {
			out = append(out, result.Item{Check: f.Check, Detail: f.Detail})
		}
	}
	return out
}

// documentGraphFailure sorts a document-graph failure into a refusal of the
// input or a failure of the tool.
func documentGraphFailure(paper string, err error) error {
	var source pandoc.SourceError
	var include pandoc.IncludeError
	var metadata docgraph.MetadataKeyError
	switch {
	case errors.As(err, &source):
		return refusal{{Check: checkSource, Path: paper, Detail: fmt.Sprintf("%s: unparseable source: %s", paper, source.Complaint)}}
	case errors.As(err, &include):
		items := make(refusal, len(include.Includes))
		for i, inc := range include.Includes {
			items[i] = result.Item{Check: checkInclude, Path: inc.Target,
				Detail: fmt.Sprintf("%s: could not load include %s, named at %s", paper, inc.Target, inc.Origin)}
		}
		return items
	case errors.As(err, &metadata):
		items := make(refusal, len(metadata.Keys))
		for i, k := range metadata.Keys {
			items[i] = result.Item{Check: checkMetadata, Path: paper, Key: k,
				Detail: fmt.Sprintf("%s: metadata key %q is neither content nor apparatus", paper, k)}
		}
		return items
	}
	if p := pandocFailure(err); p != err {
		return p
	}
	return fmt.Errorf("%s: %w", paper, err)
}

// ensureRecords regenerates the document graph's records where the state
// store has lost them: the document graph re-run into scratch, its tree
// required byte-identical to the one the document-graph boundary committed.
func (w *walk) ensureRecords() error {
	if kb.IsFile(records.Path(w.stateDir)) {
		return nil
	}
	scratch := filepath.Join(w.stateDir, scratchDir, regenerateDir)
	if err := os.RemoveAll(scratch); err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	built := filepath.Join(scratch, kb.KBDir)
	bibs, err := w.bibliographies()
	if err != nil {
		return err
	}
	report, err := docgraph.Build(w.ctx, w.opts.Logger, docgraph.Options{
		VolumeRoot: w.opts.VolumeRoot, Bibliographies: bibs, KBRoot: built,
	})
	if err != nil {
		return documentGraphFailure(w.opts.VolumeRoot, err)
	}
	if report.Failed() {
		return failure(failedChecks(report))
	}
	committed, err := w.repo.Files(w.recorded()[stageDocumentGraph], kb.KBDir)
	if err != nil {
		return err
	}
	if path, differs, err := firstDifference(built, committed); err != nil {
		return err
	} else if differs {
		return failure{{Check: checkRecords, Path: kb.KBDir + "/" + path,
			Detail: fmt.Sprintf("the document graph's records are missing from %s, and the tree regenerated to replace them differs from the document-graph boundary's at %s/%s", w.stateDir, kb.KBDir, path)}}
	}
	w.opts.Logger.Info("regenerated the document graph's records", "state-dir", w.stateDir)
	return docgraph.WriteRecords(w.stateDir, report)
}

// firstDifference is the first path, in sorted order, whose bytes under dir
// and in files differ, or that only one of them holds.
func firstDifference(dir string, files map[string][]byte) (string, bool, error) {
	on := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		on[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		return "", false, err
	}
	paths := make([]string, 0, len(on)+len(files))
	for p := range on {
		paths = append(paths, p)
	}
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range slices.Compact(paths) {
		a, inA := on[p]
		b, inB := files[p]
		if inA != inB || !bytes.Equal(a, b) {
			return p, true, nil
		}
	}
	return "", false, nil
}

// mechanical is a claim-graph stage that asks nothing, as one that may.
func mechanical(stage func(claimgraph.Options) (claimgraph.Report, error)) func(context.Context, claimgraph.Options) (claimgraph.Report, error) {
	return func(_ context.Context, o claimgraph.Options) (claimgraph.Report, error) { return stage(o) }
}

// claimGraph runs one claim-graph stage over the tree, the build records at
// the repository root and the document graph's records, its letter asks put
// through the build's reader — none under --no-inference.
func claimGraph(stage func(context.Context, claimgraph.Options) (claimgraph.Report, error)) func(*walk) (result.Record, error) {
	return func(w *walk) (result.Record, error) {
		report, err := stage(w.ctx, claimgraph.Options{KBRoot: w.kbRoot, RepoRoot: w.repo.Root, Records: records.Path(w.stateDir), Logger: w.opts.Logger,
			Reader: w.letters, ReaderConcurrency: w.readerConcurrency, AskRecords: filepath.Join(w.stateDir, scratchDir, asks.AskRecordsDir),
			Progress: w.unitProgress()})
		if err != nil {
			return nil, err
		}
		rec := result.Record{{Key: "claimgraph", Value: report.Records()}}
		if report.Failed() {
			return rec, failure(report.Failures())
		}
		return rec, nil
	}
}

// unitProgress records a claim-graph stage's own units and fallbacks in the
// progress record, under the stage the walk is in.
func (w *walk) unitProgress() claimgraph.Progress {
	done, total := 0, 0
	return claimgraph.Progress{
		Units: func(n int) error {
			done, total = 0, n
			return w.progress.emit(event{Event: eventUnits, Stage: w.inStage, Total: n})
		},
		Unit: func(name string) error {
			done++
			return w.progress.emit(event{Event: eventUnit, Stage: w.inStage, Unit: name, Done: done, Total: total})
		},
		Fallback: func(it result.Item) error {
			return w.progress.emit(event{Event: eventFallback, Stage: w.inStage, Items: []result.Item{it}})
		},
	}
}

// validationGate is refresh, then verify green: every check compares one
// mechanical product with another, so a finding stops the build.
func validationGate(w *walk) (result.Record, error) {
	written, err := index.Refresh(w.kbRoot, w.opts.Logger)
	slices.Sort(written)
	rec := result.Record{{Key: "refreshed", Value: append([]string{}, slices.Compact(written)...)}}
	var refused index.Refusal
	if errors.As(err, &refused) {
		return rec, refusal(refused.Items)
	}
	if err != nil {
		return rec, err
	}
	found, err := index.Verify(w.kbRoot)
	if err != nil {
		return rec, err
	}
	items := index.Items(found)
	rec = append(rec, result.Field{Key: "verify", Value: items})
	if len(items) > 0 {
		return rec, failure(items)
	}
	return rec, nil
}

// stampReadiness writes the readiness documents, each only where none
// stands, AGENTS.md carrying the scope pin from the charter the start
// boundary names.
func stampReadiness(w *walk) (result.Record, error) {
	pin, err := w.scopePin()
	if err != nil {
		return nil, err
	}
	reports, err := kbdocs.Stamp(w.kbRoot, filepath.Base(w.repo.Root), pin)
	rec := result.Record{{Key: "stamped", Value: append([]string{}, reports...)}}
	var refused kbdocs.Refusal
	if errors.As(err, &refused) {
		return rec, refusal{refused.Item}
	}
	return rec, err
}

// scopePin is the charter the start boundary names, as written, or the
// stated absence of one. A charter the boundary names and the disk has lost
// is refused rather than stamped as absent: the two are different facts.
func (w *walk) scopePin() (string, error) {
	for _, e := range w.trail {
		if e.Stage != stageStart {
			continue
		}
		for _, line := range strings.Split(e.Body, "\n") {
			if line == noCharter || !strings.HasPrefix(line, charterField) {
				continue
			}
			rel := strings.TrimSpace(strings.TrimPrefix(line, charterField))
			text, err := kb.ReadText(filepath.Join(w.repo.Root, rel))
			if errors.Is(err, os.ErrNotExist) {
				return "", refusal{{Check: checkCharter, Path: rel,
					Detail: fmt.Sprintf("the start boundary records a charter at %s, and no file stands there; restore it so %s carries the scope pin", rel, kb.AgentsFile)}}
			}
			if err != nil {
				return "", err
			}
			if pin := kb.Strip(text); pin != "" {
				return pin, nil
			}
		}
		break
	}
	return kbdocs.NoCharterPin, nil
}

// overviewAsks is how many times the overview passage is asked for: once,
// and once more where the reply cannot be used.
const overviewAsks = 2

// overview writes the overview document: the passage asked for from excerpts
// of the tree, a reply holding a construct the document cannot take asked
// for once more with those lines quoted, and the template filled around it.
func overview(w *walk) (result.Record, error) {
	ex, err := kbdocs.ComposeExcerpts(w.kbRoot)
	if err != nil {
		return nil, err
	}
	for _, c := range ex.Cuts {
		w.opts.Logger.Warn("an excerpt was cut to fit its cap, at a paragraph boundary", "document", c.Title, "kept", c.Kept, "cut", c.Excluded)
	}
	rec := result.Record{{Key: "excerpt-chars", Value: len([]rune(ex.Text))}, {Key: "excerpt-cuts", Value: len(ex.Cuts)}}
	var rejected []string
	var passage string
	for attempt := 1; attempt <= overviewAsks; attempt++ {
		reply, err := w.passage(w.ctx, ex.Text, rejected, attempt)
		var incomplete asks.IncompleteError
		if errors.As(err, &incomplete) {
			return rec, failure{{Check: checkInference, Detail: "the overview passage was never answered: " + incomplete.Error()}}
		}
		if err != nil {
			return rec, err
		}
		passage, rejected = kb.Strip(reply), kbdocs.NotProse(reply)
		if passage != "" && len(rejected) == 0 {
			break
		}
	}
	switch {
	case len(rejected) > 0:
		return append(rec, result.Field{Key: "rejected", Value: rejected}),
			failure{{Check: checkInference, Path: kbdocs.OverviewFile, Detail: "the overview passage held lines a passage may not, twice"}}
	case passage == "":
		return rec, failure{{Check: checkInference, Path: kbdocs.OverviewFile, Detail: "the overview passage came back empty, twice"}}
	}
	text, err := kbdocs.Overview(filepath.Base(w.repo.Root), passage)
	if err != nil {
		return rec, err
	}
	return rec, atomicfile.Write(filepath.Join(w.kbRoot, kbdocs.OverviewFile), []byte(text), nil)
}
