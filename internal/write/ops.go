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

// AddBuildEdges is the build's edge write: a node's rests-on and references
// lists, written as a run's and never refused for a cycle, which the build's
// walk cuts. Only the build issues it, so it is no subcommand.
const AddBuildEdges = "add-build-edges"

// DemoteClaimEdge is the build's ring cut: a standing scoring edge replaced by
// the demoted record that keeps it whole. Only the build issues it, so it is
// no subcommand.
const DemoteClaimEdge = "demote-claim-edge"

// SplitClaim is the build's analysis split: a claim's argument carried by a
// minted analysis that lifts it, with the claim's premises moved onto the
// analysis. Only the build issues it, so it is no subcommand.
const SplitClaim = "split-claim"

// CiteBareIDs is update-format's citing: every bare id of a node, in every
// document but the summary pages, replaced by the node's citation in the
// naming form. Only update-format issues it, so it is no subcommand.
const CiteBareIDs = "cite-bare-ids"

// The edge writes: each names one edge by its source, target and relation.
// RestoreDemoted restores the edge a demoted record replaced.
const (
	AddEdge        = "add-edge"
	EditEdge       = "edit-edge"
	RemoveEdge     = "remove-edge"
	RestoreDemoted = "restore-demoted"
)

// The node adds: each mints its node's id and writes its record, a hosted
// node's statement marker with it.
const (
	AddClaim      = "add-claim"
	AddAnalysis   = "add-analysis"
	AddExperiment = "add-experiment"
	AddWork       = "add-work"
	AddAxiom      = "add-axiom"
)

// The node edits: each changes the record fields its entry gives of the node
// its id names, keeping the node's citations live through a title change.
const (
	EditClaim      = "edit-claim"
	EditAnalysis   = "edit-analysis"
	EditExperiment = "edit-experiment"
	EditWork       = "edit-work"
	EditAxiom      = "edit-axiom"
)

// The node removal and move: each names its node by id.
const (
	RemoveNode = "remove-node"
	MoveNode   = "move-node"
)

// AddCitation replaces a span of a named line with a citation of a node.
const AddCitation = "add-citation"

// checkDependencyCycle is the refusal class of an edge that would close a
// cycle.
const checkDependencyCycle = "dependency-cycle"

// opSpec is one write op: its edit of the records, whether it adds (minting),
// edits or removes a node, or names one edge, whether only the build issues
// it, and whether it writes over records holding a cycle among the scoring
// edges — the build's, which writes cycles and then demotes them within one
// stage.
type opSpec struct {
	edit       *recordEdit
	adds       bool
	edits      bool
	removes    bool
	edge       bool
	buildOnly  bool
	overCycles bool
}

// removal is whether op takes away what stands, so that its "no node" or "no
// edge" refusal may be of a thing an earlier batch already removed. It is no
// opSpec field: the ops' edits read it, and an edit reading writeOps would
// make writeOps' initialization refer to itself.
func removal(op string) bool {
	return op == RemoveNode || op == RemoveEdge || op == RestoreDemoted
}

var writeOps = map[string]opSpec{
	AddBuildEdges:   {edit: &recordEdit{apply: addBuildEdges, machine: true}, buildOnly: true, overCycles: true},
	AddEdge:         {edit: &recordEdit{apply: addEdge}, edge: true},
	EditEdge:        {edit: &recordEdit{apply: editEdge, prior: refuseFieldless}, edge: true},
	RemoveEdge:      {edit: &recordEdit{apply: removeEdge}, edge: true},
	RestoreDemoted:  {edit: &recordEdit{apply: restoreDemoted}, edge: true},
	AddClaim:        {edit: &recordEdit{apply: addClaim}, adds: true},
	AddAnalysis:     {edit: &recordEdit{apply: addAnalysis}, adds: true},
	AddExperiment:   {edit: &recordEdit{apply: addExperiment}, adds: true},
	AddWork:         {edit: &recordEdit{apply: addWork}, adds: true},
	AddAxiom:        {edit: &recordEdit{apply: addAxiom}, adds: true},
	EditClaim:       {edit: &recordEdit{apply: editNode(kb.NodeKindClaim), prior: refuseFieldless}, edits: true},
	EditAnalysis:    {edit: &recordEdit{apply: editNode(kb.NodeKindAnalysis), prior: refuseFieldless}, edits: true},
	EditExperiment:  {edit: &recordEdit{apply: editNode(kb.NodeKindExperiment), prior: refuseFieldless}, edits: true},
	EditWork:        {edit: &recordEdit{apply: editNode(kb.NodeKindWork), prior: refuseFieldless}, edits: true},
	EditAxiom:       {edit: &recordEdit{apply: editNode(kb.NodeKindAxiom), prior: refuseFieldless}, edits: true},
	RemoveNode:      {edit: &recordEdit{apply: removeNode}, removes: true},
	MoveNode:        {edit: &recordEdit{apply: moveNode}, edits: true},
	AddCitation:     {edit: &recordEdit{apply: addCitation}},
	DemoteClaimEdge: {edit: &recordEdit{apply: demoteEdge}, buildOnly: true, overCycles: true},
	SplitClaim:      {edit: &recordEdit{apply: splitClaim, machine: true}, buildOnly: true},
	CiteBareIDs:     {edit: &recordEdit{apply: citeBareIDs}, buildOnly: true},
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

// OpField is one key of an op's values entry: its name, whether an entry
// requires it, and its type and description as the op's help lists them.
type OpField struct {
	Name        string
	Required    bool
	Type        string
	Description string
}

// OpVocabulary is a caller-issued op's values-entry keys, in vocabulary
// order; ok is false for any other name.
func OpVocabulary(op string) (fields []OpField, ok bool) {
	vocabulary, known := opFields[op]
	if !known || writeOps[op].buildOnly {
		return nil, false
	}
	for _, f := range vocabulary {
		fields = append(fields, OpField{Name: f.name, Required: f.required, Type: f.typ, Description: f.about})
	}
	return fields, true
}

// Adds is whether op mints one node per entry, reporting its id.
func Adds(op string) bool { return writeOps[op].adds }

// Edits is whether op edits the node each entry's id names, reporting the id.
func Edits(op string) bool { return writeOps[op].edits }

// Removes is whether op removes the node each entry's id names, reporting the
// id and the edge records it removed.
func Removes(op string) bool { return writeOps[op].removes }

// WritesEdge is whether op names one edge per entry, reporting its source,
// target and relation.
func WritesEdge(op string) bool { return writeOps[op].edge }

// Batch is the op of kbase write's mixed batch, whose entries each name their
// op under operation.
const Batch = "write"

// Options is one batch over the KB at KBRoot, the batch Values. DryRun does
// everything but write, as the batch's dry-run does. Commit makes the batch a
// committing one, as the batch's commit does; the
// commits are the caller's. Preflight runs once a committing batch's entries
// and derivation are known, before its first write, a dry run's included; it
// is told whether the batch is a dry run, on which the caller commits nothing.
// Its error stops the batch with nothing written, a result.Refusal refusing it
// and any other error failing it. Provenance is who supplied the rigor and
// bearing values it writes.
type Options struct {
	KBRoot     string
	Values     []byte
	DryRun     bool
	Commit     bool
	Preflight  func(dryRun bool) error
	Provenance Provenance
	Logger     log.Logger
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

// Result is what a batch did, or, under DryRun, would do. Entries is each
// entry's report, in order, where the batch applied. Written is every
// kb-root-relative path it wrote — records, leaves and derived files — and
// Removed every one the derivation removed. Derived is whether the derivation
// ran, and Drawn whether it drew the sheets, which it does not without
// Graphviz dot. Migration is what the load converted, nil on a KB at the
// current version. Commit is whether the batch commits, by Options.Commit or
// the batch's commit.
type Result struct {
	Outcome   string
	DryRun    bool
	Commit    bool
	Entries   []EntryResult
	Written   []string
	Removed   []string
	Derived   bool
	Drawn     bool
	Migration *kb.Migration
	Refusals  []result.Item
	Failures  []result.Item
}

// EntryResult is one batch entry's report: its 1-based position, its op, and
// whether it changed the KB, done or unchanged. An add's and an edit's entry
// also names the node it minted or edited. An add-axiom entry's Heading is marked or appended: how the axiom's
// marked heading came to stand. Names is the node ids the entry's values name
// — its id, source and target. An edge op's entry names its edge by Source,
// Target and Relation,
// and a restore-demoted entry's Restored is the relations it put back. A
// remove-node entry's RemovedEdges is every edge record it removed, as edge
// rows. An add-citation entry's ID is the node it cites, and Document, Line
// and Link the citing document, the 1-based line and the link it wrote there.
// An edit-work entry that changes the work's label carries Rewritten, every
// line it rewrote a reference on, [] where none; it is nil on every other
// entry. A cite-bare-ids entry's Cited is each bare id it cited, in document
// and line order.
type EntryResult struct {
	Entry        int
	Operation    string
	Outcome      string
	Names        []string
	ID           string
	Heading      string
	Source       string
	Target       string
	Relation     string
	Restored     []string
	RemovedEdges []kb.EdgeRow
	Document     string
	Line         int
	Link         string
	Rewritten    []Rewritten
	Cited        []Cited
}

// Cited is one bare id a cite-bare-ids entry cited: the kb-root-relative path
// of its document, its 1-based line and the id.
type Cited struct {
	Path string
	Line int
	ID   string
}

// Rewritten is one line a work's label change rewrote a reference on: its
// kb-root-relative path and its 1-based line.
type Rewritten struct {
	Path string
	Line int
}

// IDs is the node each entry naming one reports, in entry order.
func (r Result) IDs() []string {
	var ids []string
	for _, e := range r.Entries {
		if e.ID != "" {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// checkWrite is the class of a failure inside the write.
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

// Run runs the batch in opts.Values: op's entries alone, or, where op is
// Batch, entries each naming their op. Every entry's shape is checked first,
// every fault reported at once; then the entries apply in memory, in order,
// stopping at the first refusal; then the derivation is computed over what
// they left; and only then is every changed file written, unless the batch is
// a dry run. A batch that fails anywhere writes
// nothing. It takes no lock: a caller other than the build, which holds the
// operation lock, holds LockKB across it.
func Run(op string, opts Options) Result {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	if info, err := os.Stat(opts.KBRoot); err != nil || !info.IsDir() {
		return failedResult(opts.KBRoot + " is not a directory, so no KB can be resolved under it")
	}
	root := kb.ResolvePath(opts.KBRoot)
	b, refusals := parseValues(opts.Values, op)
	b.dryRun = b.dryRun || opts.DryRun
	b.commit = b.commit || opts.Commit
	if refusals != nil {
		return Result{Outcome: result.Refused, DryRun: b.dryRun, Commit: b.commit, Refusals: refusals}
	}
	src, err := kbload.Open(root)
	if err != nil {
		res := errorResult(err)
		res.DryRun, res.Commit = b.dryRun, b.commit
		return res
	}
	res := runOverSource(op, opts, src, b)
	res.DryRun, res.Commit = b.dryRun, b.commit
	if m, ok := src.Migration(); ok {
		res.Migration = &m
	}
	return res
}

// BatchDryRun is whether the batch of op says dry-run: true, as Run reads it.
// Refusals are ignored, so it is false where the batch does not parse as far
// as the option.
func BatchDryRun(op string, values []byte) bool {
	b, _ := parseValues(values, op)
	return b.dryRun
}

// runOverSource is Run's work over the KB its loader opened.
func runOverSource(op string, opts Options, src *kb.Source, b batch) Result {
	derive, overCycles := index.Derive, false
	for _, e := range b.entries {
		if writeOps[e.Op].overCycles {
			derive, overCycles = index.DeriveOverCycles, true
		}
	}
	rows, err := derive(src)
	if err != nil {
		return errorResult(err)
	}
	if err := index.PendingConversion(src); err != nil {
		return errorResult(err)
	}
	if err := index.Precheck(src); err != nil {
		return errorResult(err)
	}
	a, err := applyEntries(src, b.entries, opts.Provenance, rows.Hosts)
	if err != nil {
		return errorResult(err)
	}
	contended := func() *Result {
		for _, c := range a.changes {
			if !stillMatches(src, c.target, c.disk) {
				return &Result{Outcome: result.Retry, Refusals: []result.Item{{Check: kb.CheckLock, Path: src.Repo(), Key: c.subject,
					Remedy: RetryRemedy(op),
					Detail: "changed between the read and the replace; a concurrent writer intervened. The batch was valid and nothing was written"}}}
			}
		}
		return nil
	}
	if res := contended(); res != nil {
		return *res
	}
	for i := range a.entries {
		e := b.entries[a.entries[i].Entry-1]
		a.entries[i].Names = e.names()
		if WritesEdge(e.Op) {
			a.entries[i].Source, a.entries[i].Target, a.entries[i].Relation = e.str("source"), e.str("target"), e.str("relation")
		}
	}
	res := Result{Entries: a.entries}
	for _, c := range a.changes {
		src.Put(c.target, c.data)
	}
	derived, err := index.PlanDerivation(src)
	switch {
	case err == nil:
		res.Derived, res.Drawn = true, derived.Drawn
	case !overCycles || !holdsOnlyCycles(src):
		return errorResult(err)
	}
	changes := a.changes
	for _, f := range derived.Files {
		changes = append(changes, change{subject: f.Rel, target: src.KBPath(f.Rel), data: f.Data})
	}
	for _, c := range changes {
		res.Written = append(res.Written, c.subject)
	}
	res.Removed = append([]string{}, derived.Removed...)
	if b.commit && opts.Preflight != nil {
		if err := opts.Preflight(b.dryRun); err != nil {
			return errorResult(err)
		}
		if !b.dryRun {
			if res := contended(); res != nil {
				return *res
			}
		}
	}
	if !b.dryRun {
		if err := writeChanges(src, changes, derived.Removed); err != nil {
			return interruptedResult(res, err)
		}
	}
	slices.Sort(res.Written)
	res.Written = slices.Compact(res.Written)
	slices.Sort(res.Removed)
	res.Outcome = result.Unchanged
	if len(res.Written) > 0 || len(res.Removed) > 0 || slices.ContainsFunc(res.Entries, func(e EntryResult) bool { return e.Outcome == result.Done }) {
		res.Outcome = result.Done
	}
	return res
}

// holdsOnlyCycles is whether the derivation refuses the records src holds for
// a cycle among their scoring edges and for nothing else. The build's edge
// write closes such a cycle and its ring cut demotes it in the batch after,
// whose derivation brings the derived files current: a batch of a build-only
// op that leaves one writes its records without the derivation, which cannot
// read them.
func holdsOnlyCycles(src *kb.Source) bool {
	if _, err := index.Derive(src); err == nil {
		return false
	}
	_, err := index.DeriveOverCycles(src)
	return err == nil
}

// writeChanges writes each change in turn, then removes each of removed,
// kb-root-relative; a failure part-way is an interruptedError naming what
// already landed.
func writeChanges(src *kb.Source, changes []change, removed []string) error {
	var written []string
	for _, c := range changes {
		if err := index.WriteKBFile(src, c.target, c.data); err != nil {
			return &interruptedError{written: written, failed: c.subject, cause: err}
		}
		written = append(written, c.subject)
	}
	for _, rel := range removed {
		if err := index.RemoveKBFile(src, src.KBPath(rel)); err != nil {
			return &interruptedError{written: written, failed: rel, cause: err}
		}
	}
	return nil
}

// interruptedResult is res failed where err interrupted a batch after some of
// its files were written, and err's result otherwise.
func interruptedResult(res Result, err error) Result {
	var ie *interruptedError
	if errors.As(err, &ie) {
		res.Outcome, res.Written, res.Removed = result.Failed, ie.written, nil
		res.Entries = nil
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
