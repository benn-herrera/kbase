package write

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/result"
)

// RenderCitation is the one op that reads instead of writing.
const RenderCitation = "render-citation"

// AddBuildEdges is the build's edge write: add-rests-on's lists, written as a
// run's and never refused for a cycle, which the build's walk cuts. Only the
// build issues it, so it is no subcommand.
const AddBuildEdges = "add-build-edges"

// DemoteClaimEdge is the build's ring cut: a standing scoring edge replaced by
// the demoted record that keeps it whole. Only the build issues it, so it is
// no subcommand.
const DemoteClaimEdge = "demote-claim-edge"

// SplitClaim is the build's analysis split: a claim's argument carried by a
// minted analysis that lifts it, with the claim's premises moved onto the
// analysis. Only the build issues it, so it is no subcommand.
const SplitClaim = "split-claim"

// ResolveDemoted removes a demoted record or restores the edge it replaced.
const ResolveDemoted = "resolve-demoted"

// The actions resolve-demoted takes, and what each reports having done.
const (
	actionRemove    = "remove"
	actionRestore   = "restore"
	resolvedRemove  = "removed"
	resolvedRestore = "restored"
)

var resolveActions = []string{actionRemove, actionRestore}

// checkDependencyCycle is the refusal class of an edge that would close a
// cycle.
const checkDependencyCycle = "dependency-cycle"

// opSpec is one write op: its edit of the records, whether it inserts, whether
// only the build issues it, and whether it writes over a degraded load — the
// build's, which writes cycles and then cuts them.
type opSpec struct {
	edit         *recordEdit
	inserts      bool
	buildOnly    bool
	overDegraded bool
}

var writeOps = map[string]opSpec{
	AddBuildEdges:             {edit: &recordEdit{apply: addBuildEdges, machine: true}, buildOnly: true, overDegraded: true},
	ResolveDemoted:            {edit: &recordEdit{apply: resolveDemoted, prior: refuseRepeatedPairs}},
	"insert-claim-entry":      {edit: &recordEdit{apply: insertClaimNode}, inserts: true},
	"insert-analysis-entry":   {edit: &recordEdit{apply: insertAnalysisNode}, inserts: true},
	"insert-experiment-entry": {edit: &recordEdit{apply: insertExperimentNode}, inserts: true},
	"insert-work-entry":       {edit: &recordEdit{apply: insertWorkNode}, inserts: true},
	"insert-axiom-entry":      {edit: &recordEdit{apply: insertAxiomNode}, inserts: true},
	"set-bearing":             {edit: &recordEdit{field: "bearing", apply: setBearing}},
	"set-rigor":               {edit: &recordEdit{field: "rigor", apply: setRigor}},
	"set-rationale":           {edit: &recordEdit{field: "rationale", apply: setRationale}},
	"set-status":              {edit: &recordEdit{field: "status", apply: setStatus}},
	"add-rests-on":            {edit: &recordEdit{apply: addRestsOn}},
	"add-pairs":               {edit: &recordEdit{apply: addPairs}},
	"mark-claim-in-leaf":      {edit: &recordEdit{apply: markNode}},
	"drop-claim-edges":        {edit: &recordEdit{apply: dropClaimEdges}},
	"drop-inbound-edges":      {edit: &recordEdit{apply: dropInboundEdges}},
	DemoteClaimEdge:           {edit: &recordEdit{apply: demoteEdge}, buildOnly: true, overDegraded: true},
	SplitClaim:                {edit: &recordEdit{apply: splitClaim, machine: true}, buildOnly: true},
	"drop-claim-node":         {edit: &recordEdit{apply: refuseStrandedClaim, finish: dropClaimNodes}},
}

// Ops is every write op a caller may issue, by name, sorted.
func Ops() []string {
	names := make([]string, 0, len(writeOps))
	for name, spec := range writeOps {
		if !spec.buildOnly {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// OpField is one key of an op's values entry.
type OpField struct {
	Name     string
	Required bool
}

// OpVocabulary is a caller-issued op's values-entry keys, in vocabulary
// order; ok is false for any other name.
func OpVocabulary(op string) (fields []OpField, ok bool) {
	vocabulary, known := opFields[op]
	if !known || writeOps[op].buildOnly {
		return nil, false
	}
	for _, f := range vocabulary {
		fields = append(fields, OpField{Name: f.name, Required: f.required})
	}
	return fields, true
}

// Inserts is whether op enters nodes, reporting their ids.
func Inserts(op string) bool { return writeOps[op].inserts }

// Options is one op invocation over the KB at KBRoot. Provenance is who
// supplied the rigor and bearing values it writes. Restamp, where set, is
// called on the source before the trailing refresh, to put in it the stamped
// documents a migration save writes.
type Options struct {
	KBRoot     string
	Values     []byte
	NoRefresh  bool
	Provenance Provenance
	Logger     log.Logger
	Restamp    func(*kb.Source) error
}

// Provenance is who supplied a write's rigor and bearing values, which decides
// the manually-scored flag each is written with.
type Provenance int

const (
	// ProvenanceCaller marks every value it writes manually scored. It is the
	// zero value, so a caller that names none never lets a run overwrite a
	// person's value.
	ProvenanceCaller Provenance = iota
	// ProvenanceMachine is a run's own judgement, written not manually scored.
	ProvenanceMachine
)

// Result is what an op did. IDs is an insert's id per entry, minted or
// adopted; Minted the ones this call drew; Adopted the entries already
// there. Refreshed is what the trailing refresh wrote, nil where none ran,
// and Removed what it removed. Resolved is each demoted edge resolve-demoted
// changed. Migration is what the load converted, nil on a KB at the current
// version.
type Result struct {
	Outcome   string
	Written   []string
	IDs       []string
	Minted    []string
	Adopted   []Adoption
	Refreshed []string
	Removed   []string
	Resolved  []Resolution
	Citations []string
	Migration *kb.Migration
	Refusals  []result.Item
	Failures  []result.Item
}

// Resolution is one demoted edge resolved: the values' id and target, and
// what was done, removed or restored.
type Resolution struct {
	Source, Target, Action string
}

// Adoption is an insert entry that landed on an entry already there: the
// entry's 1-based index in the values, the id it adopted, and the values
// keys whose supplied value the existing entry does not carry.
type Adoption struct {
	Entry   int
	ID      string
	Differs []string
}

// checkWrite is the class of a failure inside the write or its refresh.
const checkWrite = "write"

// symlinkRemedy is what clears a document that is a symbolic link.
const symlinkRemedy = "replace the link with the document it should be, so nothing is read or written through it, then re-run"

// failedResult is a failure inside the write.
func failedResult(detail string) Result {
	return Result{Outcome: result.Failed, Failures: []result.Item{{Check: checkWrite, Detail: detail}}}
}

// opRefusal is a refusal composed by an op: the offending identity, why, and
// the corrective call.
func opRefusal(name, detail, restore string) error {
	return result.Refusal{{Key: name, Remedy: remedy(restore), Detail: detail}}
}

func refusedResult(r result.Item) Result {
	return Result{Outcome: result.Refused, Refusals: []result.Item{r}}
}

// remedy is a restore instruction without its "restore: " lead.
func remedy(restore string) string { return strings.TrimPrefix(restore, "restore: ") }

// readbackFailed is a write whose composed records or leaf read back other
// than intended.
type readbackFailed struct {
	subject    string
	mismatched []string
}

func (e *readbackFailed) Error() string { return e.subject }

// Run executes one op over the values in opts. It takes no lock: a caller
// other than the build, which holds the operation lock, holds LockKB across it.
func Run(op string, opts Options) Result {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	if info, err := os.Stat(opts.KBRoot); err != nil || !info.IsDir() {
		return failedResult(opts.KBRoot + " is not a directory, so no KB can be resolved under it")
	}
	root := kb.ResolvePath(opts.KBRoot)
	entries, refusals := parseValues(opts.Values, op)
	if refusals != nil {
		return Result{Outcome: result.Refused, Refusals: refusals}
	}
	src, err := kbload.Open(root)
	if err != nil {
		return errorResult(err)
	}
	res := runOverSource(op, opts, src, entries)
	if m, ok := src.Migration(); ok {
		res.Migration = &m
	}
	return res
}

// runOverSource is Run's work over the KB its loader opened.
func runOverSource(op string, opts Options, src *kb.Source, entries []entry) Result {
	if op == RenderCitation {
		return renderCitations(src, entries)
	}
	spec, ok := writeOps[op]
	if !ok {
		return failedResult("unknown op " + op)
	}
	rows, err := index.Derive(src)
	if err != nil {
		return errorResult(err)
	}
	if res, refused := degradedRefusal(rows); refused && !spec.overDegraded {
		return res
	}
	if opts.NoRefresh && src.Migrated() {
		return refusedResult(result.Item{Check: index.CheckIndex, Path: src.KBPath(kb.EntryPointFile), Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("the KB's metadata format is %s and a write lands in %s only through the refresh that rewrites the whole KB, "+
				"which --no-refresh skips; no save mixes two formats", src.Version(), kb.FormatVersion)})
	}
	if !opts.NoRefresh {
		if err := index.Precheck(src); err != nil {
			return errorResult(err)
		}
	}
	var res Result
	out, b, err := editRecords(src, spec.edit, entries, opts.Provenance, rows.Hosts)
	if b != nil {
		res = Result{IDs: b.ids, Minted: b.minted, Adopted: b.adopted, Resolved: b.resolved}
	}
	if err != nil {
		return interruptedResult(res, err)
	}
	if out.status == statusRetry {
		return Result{Outcome: result.Retry, Refusals: []result.Item{{Check: kb.CheckLock, Path: src.Repo(), Key: out.subject,
			Remedy: RetryRemedy(op),
			Detail: fmt.Sprintf("%s (%s). The values were correct and nothing was written", out.detail, out.reason)}}}
	}
	res.Written = out.written
	res.Outcome = result.Unchanged
	if out.status == statusWritten {
		res.Outcome = result.Done
	}
	if opts.NoRefresh {
		return res
	}
	var refreshed, removed []string
	if opts.Restamp != nil {
		err = opts.Restamp(src)
	}
	if err == nil {
		refreshed, removed, _, err = index.RefreshReporting(src, opts.Logger)
	}
	slices.Sort(refreshed)
	res.Refreshed = append([]string{}, slices.Compact(refreshed)...)
	res.Removed = append([]string{}, removed...)
	if err != nil {
		res.Outcome = result.Failed
		res.Failures = []result.Item{{Check: checkWrite, Detail: "the write landed and the trailing refresh did not: " + err.Error()}}
		return res
	}
	if len(res.Refreshed) > 0 || len(res.Removed) > 0 {
		res.Outcome = result.Done
	}
	return res
}

// interruptedResult is res failed where err interrupted a batch after some of
// its files were written, and err's result otherwise.
func interruptedResult(res Result, err error) Result {
	var ie *interruptedError
	if errors.As(err, &ie) {
		res.Outcome, res.Written = result.Failed, ie.written
		res.Failures = []result.Item{{Check: checkWrite, Detail: ie.Error()}}
		return res
	}
	return errorResult(err)
}

func errorResult(err error) Result {
	var refused result.Refusal
	var rb *readbackFailed
	var sym *kb.SymlinkError
	switch {
	case errors.As(err, &refused):
		return Result{Outcome: result.Refused, Refusals: refused}
	case errors.As(err, &sym):
		return refusedResult(result.Item{Check: kb.CheckSymlink, Path: sym.Path, Remedy: symlinkRemedy,
			Detail: err.Error()})
	case errors.As(err, &rb):
		return refusedResult(result.Item{Key: rb.subject, Remedy: "this is a renderer defect, not a value defect — report it; nothing on disk changed",
			Detail: "read back from the composed candidate with a different " + strings.Join(rb.mismatched, ", ") +
				" than the values supplied, so the live file was never written"})
	}
	return failedResult(err.Error())
}

func containedIn(root, rel, key string) (string, error) {
	target, err := resolveTarget(root, rel)
	var se *storeError
	if errors.As(err, &se) {
		return "", opRefusal(key, fmt.Sprintf("%q %s. Paths are relative to kb-root/, not to the repository root", rel, se.detail),
			"restore: correct "+key+" to a kb-root-relative path inside the KB and re-run")
	}
	return target, err
}
