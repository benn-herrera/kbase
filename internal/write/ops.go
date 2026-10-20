package write

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
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

// AddBuildEdges is the build's edge write: add-rests-on's lists and the
// demoted list. Only the build issues it, so it is no subcommand.
const AddBuildEdges = "add-build-edges"

// DemoteClaimEdge is the build's ring cut: an existing rests-on edge turned
// into a demoted row with its origin. Only the build issues it, so it is no
// subcommand.
const DemoteClaimEdge = "demote-claim-edge"

// ResolveDemoted removes a demoted edge or restores it to the rests-on list.
const ResolveDemoted = "resolve-demoted"

// The actions resolve-demoted takes, and what each reports having done.
const (
	actionRemove    = "remove"
	actionRestore   = "restore"
	resolvedRemove  = "removed"
	resolvedRestore = "restored"
)

var resolveActions = []string{actionRemove, actionRestore}

// checkDependencyCycle is the refusal class of a restore that would close a
// cycle.
const checkDependencyCycle = "dependency-cycle"

// opSpec is one write op: its planner, whether it may create a register,
// whether it inserts, and whether only the build issues it.
type opSpec struct {
	plan      func(*opContext, []entry) ([]intent, error)
	creates   bool
	inserts   bool
	buildOnly bool
}

var writeOps = map[string]opSpec{
	AddBuildEdges:             {plan: planAddRestsOn, buildOnly: true},
	ResolveDemoted:            {plan: planResolveDemoted},
	"insert-claim-entry":      {plan: planInserts("clm"), creates: true, inserts: true},
	"insert-analysis-entry":   {plan: planInserts("ana"), creates: true, inserts: true},
	"insert-experiment-entry": {plan: planExperiments, inserts: true},
	"insert-work-entry":       {plan: planWorkInserts, creates: true, inserts: true},
	"set-bearing":             {plan: planSetBearing},
	"set-rigor":               {plan: planSetRigor},
	"set-rationale":           {plan: planFieldUpdate("rationale", func(string) string { return "rationale" })},
	"add-rests-on":            {plan: planAddRestsOn},
	"set-frontmatter":         {plan: planSetFrontmatter},
	"mark-claim-in-leaf":      {plan: planMarkClaim},
	"drop-claim-edges":        {plan: planDropClaimEdges},
	"drop-inbound-edges":      {plan: planDropInboundEdges},
	DemoteClaimEdge:           {plan: planDemoteClaimEdge, buildOnly: true},
	"drop-claim-node":         {plan: planDropClaimNode},
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

// CreatesRegister is whether op takes --create.
func CreatesRegister(op string) bool { return writeOps[op].creates }

// OpField is one key of an op's values entry.
type OpField struct {
	Name     string
	Required bool
}

// OpVocabulary is a caller-issued op's values-entry keys, in vocabulary
// order, and whether it takes --create; ok is false for any other name.
func OpVocabulary(op string) (fields []OpField, create, ok bool) {
	vocabulary, known := opFields[op]
	spec := writeOps[op]
	if !known || spec.buildOnly {
		return nil, false, false
	}
	for _, f := range vocabulary {
		fields = append(fields, OpField{Name: f.name, Required: f.required})
	}
	return fields, spec.creates, true
}

// Inserts is whether op enters nodes, reporting their ids.
func Inserts(op string) bool { return writeOps[op].inserts }

// Options is one op invocation over the KB at KBRoot.
type Options struct {
	KBRoot    string
	Values    []byte
	Create    bool
	NoRefresh bool
	Logger    log.Logger
}

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

// Resolution is one demoted edge resolved: its ends and what was done,
// removed or restored.
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

// readbackFailed is a leaf write whose candidate read back other than
// intended.
type readbackFailed struct {
	subject    string
	mismatched []string
}

func (e *readbackFailed) Error() string { return e.subject }

// Run executes one op over the values in opts. It takes no lock: a caller
// other than the build, which holds the run lock, holds LockKB across it.
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
	// The annotation gate runs before any op's plan: a bullet the reader
	// refuses is a fault in the KB, and no write lands over one.
	if _, err := kb.Discover(src, opts.Logger); err != nil {
		var ann *kb.AnnotationError
		if errors.As(err, &ann) {
			return Result{Outcome: result.Refused, Refusals: index.AnnotationItems(ann)}
		}
		return errorResult(err)
	}
	if opts.NoRefresh && src.Migrated() {
		return refusedResult(result.Item{Check: index.CheckIndex, Path: src.KBPath(kb.EntryPointFile), Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("the KB's metadata format is %s and a write lands in %s only through the refresh that rewrites the whole KB, "+
				"which --no-refresh skips; no save mixes two formats", src.Version(), kb.FormatVersion)})
	}
	ctx, err := openStore(src)
	if err != nil {
		return errorResult(err)
	}
	ctx.create = opts.Create && spec.creates
	intents, err := spec.plan(ctx, entries)
	if err != nil {
		return errorResult(err)
	}
	out, err := applyEdits(src, batch(intents))
	res := Result{IDs: ctx.ids, Minted: ctx.minted, Adopted: ctx.adopted}
	if err != nil {
		var ie *interruptedError
		if errors.As(err, &ie) {
			res.Outcome, res.Written = result.Failed, ie.written
			res.Failures = []result.Item{{Check: checkWrite, Detail: ie.Error()}}
			return res
		}
		return errorResult(err)
	}
	switch out.status {
	case statusRefused:
		return refusedResult(result.Item{Key: out.subject, Remedy: "correct the values or repair the named file, then re-run kbase " + op,
			Detail: fmt.Sprintf("%s (%s)", out.detail, out.reason)})
	case statusRetry:
		return Result{Outcome: result.Retry, Refusals: []result.Item{{Check: "lock", Key: out.subject,
			Remedy: RetryRemedy(op),
			Detail: fmt.Sprintf("%s (%s). The values were correct and nothing was written", out.detail, out.reason)}}}
	}
	res.Written, res.Resolved = out.written, ctx.resolved
	res.Outcome = result.Unchanged
	if out.status == statusWritten {
		res.Outcome = result.Done
	}
	if opts.NoRefresh {
		return res
	}
	refreshed, removed, _, err := index.RefreshReporting(src, opts.Logger)
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

func errorResult(err error) Result {
	var refused result.Refusal
	var rb *readbackFailed
	var mf kb.MalformedError
	var ann *kb.AnnotationError
	var sym *kb.SymlinkError
	switch {
	case errors.As(err, &refused):
		return Result{Outcome: result.Refused, Refusals: refused}
	case errors.As(err, &ann):
		return Result{Outcome: result.Refused, Refusals: index.AnnotationItems(ann)}
	case errors.As(err, &sym):
		return refusedResult(result.Item{Check: kb.CheckSymlink, Path: sym.Path, Remedy: symlinkRemedy,
			Detail: err.Error()})
	case errors.As(err, &rb):
		return refusedResult(result.Item{Key: rb.subject, Remedy: "this is a renderer defect, not a value defect — report it; nothing on disk changed",
			Detail: "read back from the composed candidate with a different " + strings.Join(rb.mismatched, ", ") +
				" than the values supplied, so the live file was never written"})
	case errors.As(err, &mf):
		return refusedResult(result.Item{Key: "frontmatter", Remedy: "repair the named document's frontmatter, then re-run", Detail: mf.Msg})
	}
	return failedResult(err.Error())
}

// opContext is one invocation's view of the authored store. create is the
// --create acknowledgement, on an op that admits it.
type opContext struct {
	src      *kb.Source
	root     string
	create   bool
	known    map[string]kb.IDRecord
	axioms   map[string]bool
	ids      []string
	minted   []string
	adopted  []Adoption
	resolved []Resolution
	titles   map[string]string
	read     map[string]bool
}

// openStore reads the authored-id and axiom inventory, refusing a KB that keys
// one id from two register entries: the inventory keeps the first and would
// report the collision as clean.
func openStore(src *kb.Source) (*opContext, error) {
	known, duplicates, err := kb.AuthoredIDs(src)
	if err != nil {
		return nil, err
	}
	if len(duplicates) > 0 {
		ids := make([]string, 0, len(duplicates))
		for id := range duplicates {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		id := ids[0]
		return nil, opRefusal(id, fmt.Sprintf("is keyed by %d canonical register entries (%s); every later reading "+
			"would be taken over the inventory that keeps only the first", len(duplicates[id]), strings.Join(duplicates[id], ", ")),
			fmt.Sprintf("restore: delete the duplicate '<!-- id: %s -->' entry from all but one register, then re-run this op unchanged", id))
	}
	axioms, err := kb.KnownAxiomIDs(src)
	if err != nil {
		return nil, err
	}
	return &opContext{src: src, root: src.Root(), known: known, axioms: axioms, titles: map[string]string{}, read: map[string]bool{}}, nil
}

// mint is a fresh id of kind, clear of every authored id and of every id this
// invocation has minted.
func (c *opContext) mint(kind string) string {
	id := kb.MintID(kind, func(id string) bool {
		_, taken := c.known[id]
		return taken || slices.Contains(c.minted, id)
	})
	c.minted = append(c.minted, id)
	return id
}

// titleOf is an already-resolved node's register title.
func (c *opContext) titleOf(nodeID string) (string, error) {
	rec, ok := c.known[nodeID]
	if !ok || rec.RegisterPath == "" {
		return "", nil
	}
	if !c.read[rec.RegisterPath] {
		c.read[rec.RegisterPath] = true
		text, err := c.src.ReadText(c.src.KBPath(rec.RegisterPath))
		if err != nil {
			return "", err
		}
		r, err := readRegister(text)
		if err != nil {
			return "", err
		}
		for id, e := range r.claims {
			c.titles[id] = e.Title
		}
		for id, a := range r.analyses {
			c.titles[id] = a.Title
		}
		for id, w := range r.works {
			c.titles[id] = w.Title
		}
	}
	return c.titles[nodeID], nil
}

// contained resolves a kb-root-relative path value inside kb-root.
func (c *opContext) contained(rel, key string) (string, error) {
	return containedIn(c.root, rel, key)
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

// resolve is an id-valued field's reference closure. A clm-, ana- or wrk- id
// must have its register entry; an exp- id lives in its hosting document.
func (c *opContext) resolve(nodeID, key string, needsRegister bool) (kb.IDRecord, error) {
	rec, ok := c.known[nodeID]
	if !ok {
		return rec, opRefusal(key+"="+nodeID, "does not resolve against the authored-id inventory. Every id-valued "+
			"field must name a node that already exists at write time",
			fmt.Sprintf("restore: create %s's node with the insert op for its kind, or correct %s, then re-run", nodeID, key))
	}
	if needsRegister && rec.RegisterPath == "" {
		return rec, opRefusal(key+"="+nodeID, "is authored in document frontmatter but has no canonical register "+
			"entry, so it is not a node any consumer can read a title, a rigor or a rationale from",
			fmt.Sprintf("restore: insert %s's register entry, or correct %s, then re-run", nodeID, key))
	}
	return rec, nil
}

// premiseTargets resolves requested premises into bullet values; an axiom
// token must name an axiom the KB declares, and an exp- id an experiment its
// documents declare.
func (c *opContext) premiseTargets(values []restsOnValue) ([]premiseTarget, error) {
	var out []premiseTarget
	for _, v := range values {
		if !isClaimID(v.ID) && !isWorkID(v.ID) {
			if recKind(v.ID) == "exp" {
				if err := c.resolveExperiment(v.ID, kb.ListRestsOn+".id"); err != nil {
					return nil, err
				}
				out = append(out, premiseTarget{Target: v.ID, Context: v.Context, Bearing: v.Bearing})
				continue
			}
			if err := c.resolveAxiom(v.ID, kb.ListRestsOn+".id"); err != nil {
				return nil, err
			}
			out = append(out, premiseTarget{Target: v.ID, Context: v.Context, Bearing: v.Bearing})
			continue
		}
		if _, err := c.resolve(v.ID, kb.ListRestsOn+".id", true); err != nil {
			return nil, err
		}
		title, err := c.titleOf(v.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, premiseTarget{Target: v.ID, Title: title, Context: v.Context, Bearing: v.Bearing, Origin: v.Origin})
	}
	return out, nil
}

// resolveExperiment refuses a premise naming an experiment no document
// declares: the bullet would author an edge into no node, and the index build
// would refuse the KB it left behind.
func (c *opContext) resolveExperiment(token, key string) error {
	if rec, ok := c.known[token]; ok && rec.Kind == "exp" {
		return nil
	}
	return opRefusal(key, fmt.Sprintf("%q is an experiment no document declares. The bullet authors an edge into no "+
		"node, and the index build refuses a KB carrying one", token),
		"restore: declare it with insert-experiment-entry, or correct "+key+", then re-run")
}

// resolveAxiom refuses a premise naming an axiom the axiom source does not
// declare: the bullet would author an edge into no node, and the index build
// would refuse the KB it left behind.
func (c *opContext) resolveAxiom(token, key string) error {
	if c.axioms[axiomEdgeID(token)] {
		return nil
	}
	source := kb.AxiomsFile
	if p := kb.AxiomSource(c.src); p != "" {
		source = filepath.Base(p)
	}
	return opRefusal(key, fmt.Sprintf("%q names no axiom %s declares. The bullet authors an edge into no node, "+
		"and the index build refuses a KB carrying one", token, source),
		fmt.Sprintf("restore: declare it in %s as \"### %s: <title>\", or correct %s, then re-run", source, token, key))
}

// intent is one entry's change to one file. expect is stated outright;
// expectCurrent is derived from the baseline the splice receives; prove
// reads a document write back.
type intent struct {
	path, target                         string
	splice                               func(string) (string, error)
	expect                               []expectedEntry
	expectCurrent                        func(registerRead) ([]expectedEntry, error)
	prove                                func(string) ([]string, error)
	subject                              string
	claimDelta, analysisDelta, workDelta int
	create                               bool
}

// batch folds every intent into one edit per resolved file, in first-named
// order: two edits of one file would splice from one baseline.
func batch(intents []intent) []edit {
	var order []string
	groups := map[string][]intent{}
	for _, in := range intents {
		if _, ok := groups[in.target]; !ok {
			order = append(order, in.target)
		}
		groups[in.target] = append(groups[in.target], in)
	}
	var edits []edit
	for _, target := range order {
		group := groups[target]
		expected := &[]expectedEntry{}
		e := edit{path: group[0].path, expect: expected}
		for _, in := range group {
			*expected = append(*expected, in.expect...)
			e.claimDelta += in.claimDelta
			e.analysisDelta += in.analysisDelta
			e.workDelta += in.workDelta
			e.create = e.create || in.create
		}
		e.splice = func(document string) (string, error) {
			read, err := readRegister(document)
			if err != nil {
				return "", err
			}
			for _, in := range group {
				if in.expectCurrent == nil {
					continue
				}
				more, err := in.expectCurrent(read)
				if err != nil {
					return "", err
				}
				*expected = append(*expected, more...)
			}
			for _, in := range group {
				var err error
				if document, err = in.splice(document); err != nil {
					return "", err
				}
			}
			for _, in := range group {
				if in.prove == nil {
					continue
				}
				wrong, err := in.prove(kb.TranslateNewlines(document))
				if err != nil {
					return "", err
				}
				if len(wrong) > 0 {
					return "", &readbackFailed{subject: in.subject, mismatched: wrong}
				}
			}
			return document, nil
		}
		edits = append(edits, e)
	}
	return edits
}

// currentEntry is the record as the baseline holds it, as an expectation:
// an update's intended record is this with one field changed, so the proof
// also shows nothing else moved.
func currentEntry(read registerRead, nodeID, register string) (expectedEntry, error) {
	e, ok := read.record(nodeID)
	if !ok {
		return e, opRefusal("id="+nodeID, "has no canonical entry in "+register+" that the production parser returns",
			"restore: correct id, or repair "+nodeID+"'s entry, then re-run")
	}
	return e, nil
}

func planInserts(kind string) func(*opContext, []entry) ([]intent, error) {
	return func(c *opContext, entries []entry) ([]intent, error) {
		var intents []intent
		for _, e := range entries {
			rel := e.str("register")
			target, err := c.contained(rel, "register")
			if err != nil {
				return nil, err
			}
			premises, err := c.premiseTargets(e.restsOn(kb.ListRestsOn))
			if err != nil {
				return nil, err
			}
			lifts, refutes := e.pairs(kb.LiftsKey), e.pairs(kb.RefutesKey)
			for _, list := range []struct {
				key   string
				pairs []pair
			}{{kb.LiftsKey, lifts}, {kb.RefutesKey, refutes}} {
				for _, p := range list.pairs {
					if _, err := c.resolve(p.ID, list.key+".id", true); err != nil {
						return nil, err
					}
				}
			}
			want := expectedEntry{Title: e.str("title"), Rigor: e.score("rigor"), Rationale: e.str("rationale"),
				Lifts: lifts, Refutes: refutes}
			for _, t := range premises {
				want.Premises = append(want.Premises, expectedEdge{Source: t.Target, Context: t.Context, Bearing: bearingOf(t.Bearing)})
			}
			if kind == "clm" {
				want.StrengthenBy = e.list("strengthen-by")
			}
			if adopted, differs := adoptRegisterEntry(c.src, target, kind, want); adopted != "" {
				c.adopt(e.Index, adopted, differs)
				continue
			}
			nodeID := c.mint(kind)
			c.ids = append(c.ids, nodeID)
			want.NodeID = nodeID
			for i := range want.Premises {
				want.Premises[i].Target = nodeID
			}
			rendered := registerEntry{NodeID: nodeID, Title: want.Title, Rigor: want.Rigor, Rationale: want.Rationale,
				Premises: premises, NoEdge: e.str("no-edge"), StrengthenBy: want.StrengthenBy, Lifts: lifts, Refutes: refutes}
			in := intent{path: rel, target: target, splice: insertAll(renderEntry(rendered)), expect: []expectedEntry{want}, create: c.create}
			if kind == "clm" {
				in.claimDelta = 1
			} else {
				in.analysisDelta = 1
			}
			intents = append(intents, in)
		}
		return intents, nil
	}
}

// adoptRegisterEntry is the entry of kind the register already keys under
// want's title — the first that reads back as want, else the first — and the
// fields on which it differs from want; "" where no entry carries the title.
func adoptRegisterEntry(src *kb.Source, target, kind string, want expectedEntry) (string, []string) {
	text, err := src.ReadText(target)
	if err != nil {
		return "", nil
	}
	read, err := readRegister(text)
	if err != nil {
		return "", nil
	}
	title := collapse(want.Title)
	first, firstDiffers := "", []string(nil)
	for _, loc := range kb.LocateRegisterEntries(text) {
		if loc.Kind != kind || !loc.Bound() || loc.HeadingTitle != title {
			continue
		}
		got, ok := read.record(loc.NodeID)
		if !ok {
			continue
		}
		w := want
		w.NodeID = loc.NodeID
		for i := range w.Premises {
			w.Premises[i].Target = loc.NodeID
		}
		differs := mismatchedFields(w, got)
		if len(differs) == 0 {
			return loc.NodeID, nil
		}
		if first == "" {
			first, firstDiffers = loc.NodeID, differs
		}
	}
	return first, firstDiffers
}

// adopt records an insert entry that landed on an entry already there, with
// the supplied values that entry does not carry: an insert does not update.
func (c *opContext) adopt(entry int, nodeID string, differs []string) {
	c.ids = append(c.ids, nodeID)
	c.adopted = append(c.adopted, Adoption{Entry: entry, ID: nodeID, Differs: append([]string{}, differs...)})
}

func insertAll(rendered string) func(string) (string, error) {
	return func(document string) (string, error) { return insertEntry(document, rendered), nil }
}

func planExperiments(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		rel := e.str("document")
		target, err := c.contained(rel, "document")
		if err != nil {
			return nil, err
		}
		rigor := e.score("rigor")
		decl := experimentDecl{Status: e.str("status"), Rigor: rigor, Supports: e.pairs(kb.SupportsKey),
			Lifts: e.pairs(kb.LiftsKey), Refutes: e.pairs(kb.RefutesKey)}
		for _, list := range []struct {
			key   string
			pairs []pair
		}{{kb.SupportsKey, decl.Supports}, {kb.LiftsKey, decl.Lifts}, {kb.RefutesKey, decl.Refutes}} {
			for _, p := range list.pairs {
				if _, err := c.resolve(p.ID, list.key+".id", true); err != nil {
					return nil, err
				}
			}
		}
		if adopted := adoptExperiment(c.src, target, rel, decl); adopted != "" {
			c.adopt(e.Index, adopted, nil)
			continue
		}
		expID := c.mint("exp")
		c.ids = append(c.ids, expID)
		decl.ExpID = expID
		lines, err := renderExperimentNodes([]experimentDecl{decl})
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent{path: rel, target: target, splice: appendDeclaration(kb.ExperimentNodesKey, lines),
			prove: experimentProver(decl, rel), subject: rel + ":" + expID})
	}
	return intents, nil
}

// adoptExperiment is an experiment the document already hosts with this
// status, rigor and pair lists, or "".
func adoptExperiment(src *kb.Source, target, rel string, want experimentDecl) string {
	text, err := src.ReadText(target)
	if err != nil {
		return ""
	}
	nodes, err := kb.ParseExperimentLeaf(text, rel)
	if err != nil {
		return ""
	}
	for _, n := range nodes {
		got := experimentDecl{ExpID: n.ID, Status: n.Status, Rigor: n.Rigor, Supports: pairsOf(n.Supports),
			Lifts: pairsOf(n.Lifts), Refutes: pairsOf(n.Refutes)}
		if got.Status == want.Status && scoreEqual(got.Rigor, want.Rigor) && pairsEqual(got.Supports, want.Supports) &&
			pairsEqual(got.Lifts, want.Lifts) && pairsEqual(got.Refutes, want.Refutes) {
			return n.ID
		}
	}
	return ""
}

func experimentProver(decl experimentDecl, rel string) func(string) ([]string, error) {
	return func(candidate string) ([]string, error) {
		nodes, err := kb.ParseExperimentLeaf(candidate, rel)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			if n.ID != decl.ExpID {
				continue
			}
			got := experimentDecl{ExpID: n.ID, Status: n.Status, Rigor: n.Rigor, Supports: pairsOf(n.Supports),
				Lifts: pairsOf(n.Lifts), Refutes: pairsOf(n.Refutes)}
			var wrong []string
			if got.Status != decl.Status {
				wrong = append(wrong, "status")
			}
			if !scoreEqual(got.Rigor, decl.Rigor) {
				wrong = append(wrong, "rigor")
			}
			for _, list := range []struct {
				key       string
				got, want []pair
			}{{kb.SupportsKey, got.Supports, decl.Supports}, {kb.LiftsKey, got.Lifts, decl.Lifts}, {kb.RefutesKey, got.Refutes, decl.Refutes}} {
				if !pairsEqual(list.got, list.want) {
					wrong = append(wrong, list.key)
				}
			}
			return wrong, nil
		}
		return []string{"exp-id"}, nil
	}
}

func planWorkInserts(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	created := map[string]bool{}
	for _, e := range entries {
		rel := e.str("register")
		target, err := c.contained(rel, "register")
		if err != nil {
			return nil, err
		}
		key := e.str("key")
		nodeID := kb.WorkPrefix + "-" + key
		want := expectedEntry{NodeID: nodeID, Title: e.str("title"), Rigor: e.score("rigor"), Rationale: e.str("rationale")}
		if created[nodeID] {
			return nil, opRefusal("key="+key, fmt.Sprintf("is named twice in this batch (%s). A work's id is derived from its "+
				"citation key, so a second entry would be a second node for one work", nodeID),
				"restore: drop one of the two entries, then re-run")
		}
		if held, ok := c.known[nodeID]; ok && held.RegisterPath != "" {
			text, err := c.src.ReadText(c.src.KBPath(held.RegisterPath))
			if err != nil {
				return nil, err
			}
			read, err := readRegister(text)
			if err != nil {
				return nil, err
			}
			got, _ := read.record(nodeID)
			c.adopt(e.Index, nodeID, mismatchedFields(want, got))
			continue
		}
		created[nodeID] = true
		c.ids = append(c.ids, nodeID)
		intents = append(intents, intent{path: rel, target: target,
			splice: insertAll(renderWorkEntry(nodeID, want.Title, want.Rigor, want.Rationale)),
			expect: []expectedEntry{want}, workDelta: 1, create: c.create})
	}
	return intents, nil
}

// planFieldUpdate rewrites one authored field in place; the whole record is
// the proof.
func planFieldUpdate(key string, fieldOf func(string) string) func(*opContext, []entry) ([]intent, error) {
	return func(c *opContext, entries []entry) ([]intent, error) {
		var intents []intent
		for _, e := range entries {
			nodeID := e.str("id")
			rec, err := c.resolve(nodeID, "id", true)
			if err != nil {
				return nil, err
			}
			register, err := c.contained(rec.RegisterPath, "id")
			if err != nil {
				return nil, err
			}
			name := fieldOf(nodeID)
			rendered := formatScore(e.score(key))
			if key == "rationale" {
				rendered = collapse(e.str(key))
			}
			line := "- " + name + ": " + rendered
			intents = append(intents, intent{path: rec.RegisterPath, target: register,
				splice: func(document string) (string, error) { return replaceFieldLine(document, nodeID, name, line) },
				expectCurrent: func(read registerRead) ([]expectedEntry, error) {
					cur, err := currentEntry(read, nodeID, rec.RegisterPath)
					if err != nil {
						return nil, err
					}
					if key == "rationale" {
						cur.Rationale = e.str(key)
					} else {
						cur.Rigor = e.score(key)
					}
					return []expectedEntry{cur}, nil
				}})
		}
		return intents, nil
	}
}

// planSetRigor rewrites one node's local rigor wherever its kind authors it: a
// claim's, an analysis's or a work's register line, or an experiment's rigor in
// its host leaf's experiment-nodes mapping.
func planSetRigor(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		nodeID := e.str("id")
		rec, err := c.resolve(nodeID, "id", recKind(nodeID) != "exp")
		if err != nil {
			return nil, err
		}
		if rec.Kind == "exp" {
			if rec.HostingLeaf == "" {
				return nil, opRefusal("id="+nodeID, "is an experiment no document declares, so there is no rigor to rewrite",
					"restore: declare it with insert-experiment-entry, or correct id, then re-run")
			}
			leaf, err := c.contained(rec.HostingLeaf, "id")
			if err != nil {
				return nil, err
			}
			rigor := e.score("rigor")
			intents = append(intents, intent{path: rec.HostingLeaf, target: leaf,
				splice: replaceDeclarationRigor(nodeID, rigor),
				prove:  declarationRigorProver(nodeID, rigor, rec.HostingLeaf), subject: rec.HostingLeaf + ":" + nodeID})
			continue
		}
		register, err := c.contained(rec.RegisterPath, "id")
		if err != nil {
			return nil, err
		}
		rigor := e.score("rigor")
		line := "- rigor: " + formatScore(rigor)
		intents = append(intents, intent{path: rec.RegisterPath, target: register,
			splice: func(document string) (string, error) { return replaceFieldLine(document, nodeID, "rigor", line) },
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, nodeID, rec.RegisterPath)
				if err != nil {
					return nil, err
				}
				cur.Rigor = rigor
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// recKind is the kind an id's prefix names.
func recKind(nodeID string) string {
	kind, _, _ := strings.Cut(nodeID, "-")
	return kind
}

// nodeKind is the node_type an id's prefix names.
func nodeKind(nodeID string) string {
	switch recKind(nodeID) {
	case "clm":
		return kb.NodeKindClaim
	case "ana":
		return kb.NodeKindAnalysis
	case "exp":
		return kb.NodeKindExperiment
	case "axm":
		return kb.NodeKindAxiom
	case kb.WorkPrefix:
		return kb.NodeKindWork
	}
	return ""
}

// planSetBearing rewrites one existing scoring edge's bearing where it is
// authored and never inserts a token: a premise edge's on the target entry's
// rests-on bullet naming the source, a lift, refutation or experiment support
// on the source's pair line — in its leaf, or in the register's staged copy,
// which is a claim's own refutation list where the source is a claim.
func planSetBearing(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		source, target, relation := e.str("id"), e.str("target"), e.str("relation")
		bearing := bearingOf(e.score("bearing"))
		// An axiom declares no register entry: the rests-on bullet naming it
		// is where its edge stands and the proof it does.
		if !strings.HasPrefix(source, axiomIDPrefix) {
			if _, err := c.resolve(source, "id", false); err != nil {
				return nil, err
			}
		}
		if _, err := c.resolve(target, "target", false); err != nil {
			return nil, err
		}
		if !kb.BearingLegal(relation, nodeKind(source), nodeKind(target)) {
			return nil, opRefusal("relation="+relation, fmt.Sprintf("does not join a %s source to a %s target: no bearing "+
				"judges that pairing", nodeKind(source), nodeKind(target)),
				"restore: correct relation or the ids, then re-run")
		}
		in, err := c.bearingHomes(source, target, relation, bearing)
		if err != nil {
			return nil, err
		}
		intents = append(intents, in...)
	}
	return intents, nil
}

// bearingHomes is every intent that rewrites one edge's bearing where it is
// authored. A premise edge has one home — the target entry's rests-on bullet.
// A lift or refutation has one or two: its source's declaration in the hosting
// leaf, and the register's staged copy, which are two records of one edge and
// must be judged alike or the fan-out disagrees.
func (c *opContext) bearingHomes(source, target, relation string, bearing kb.Bearing) ([]intent, error) {
	src := c.known[source]
	if relation == kb.RelationLifts || relation == kb.RelationRefutes || src.Kind == "exp" {
		return c.pairHomes(source, target, relation, bearing)
	}
	rec, ok := c.known[target]
	if !ok || rec.RegisterPath == "" {
		return nil, opRefusal("target="+target, "has no canonical register entry, so it authors no premise bullet "+
			"this edge could be judged on", "restore: correct target, then re-run")
	}
	register, err := c.contained(rec.RegisterPath, "target")
	if err != nil {
		return nil, err
	}
	return []intent{{path: rec.RegisterPath, target: register,
		splice: replacePremiseBearing(target, source, relation, bearing),
		expectCurrent: func(read registerRead) ([]expectedEntry, error) {
			cur, err := currentEntry(read, target, rec.RegisterPath)
			if err != nil {
				return nil, err
			}
			at := slices.IndexFunc(cur.Premises, func(d expectedEdge) bool { return d.Source == source })
			if at < 0 {
				return nil, opRefusal("id="+source, "is not a premise of "+target+", so there is no bullet carrying a "+
					"bearing to rewrite. This op re-scores an edge; it does not create one",
					"restore: add the edge with add-rests-on, or correct id, then re-run")
			}
			cur.Premises[at].Bearing = bearing
			return []expectedEntry{cur}, nil
		}}}, nil
}

// scoreOf is a bearing as a values score: nil is pending.
func scoreOf(b kb.Bearing) *float64 {
	if b.Pending {
		return nil
	}
	v := b.Value
	return &v
}

// pairHomes is every home a pair's bearing is authored in: the hosting leaf's
// declaration, and the register's staged copy, each where it records the pair.
// A claim authors its refutations on its register entry's own list, which is
// the staged home and the only one it has.
func (c *opContext) pairHomes(source, target, relation string, bearing kb.Bearing) ([]intent, error) {
	rec, ok := c.known[source]
	if !ok {
		return nil, opRefusal("id="+source, "resolves to no node", "restore: correct id, then re-run")
	}
	key := relation
	if relation == kb.RelationSupports {
		key = kb.SupportsKey
	}
	score := scoreOf(bearing)
	var intents []intent
	if rec.HostingLeaf != "" {
		text, err := c.src.ReadText(c.src.KBPath(rec.HostingLeaf))
		if err != nil {
			return nil, err
		}
		if holdsDeclarationPair(text, source, key, target) {
			leaf, err := c.contained(rec.HostingLeaf, "id")
			if err != nil {
				return nil, err
			}
			intents = append(intents, intent{path: rec.HostingLeaf, target: leaf,
				splice: replaceDeclarationPair(source, key, target, score),
				prove:  pairBearingProver(source, key, target, bearing, rec.HostingLeaf), subject: rec.HostingLeaf + ":" + source})
		}
	}
	if rec.RegisterPath != "" && c.stagesPair(rec.RegisterPath, source, relation, target) {
		register, err := c.contained(rec.RegisterPath, "id")
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent{path: rec.RegisterPath, target: register,
			splice: editStagedPair(source, relation, func(id string) (string, bool, bool) {
				if id != target {
					return "", false, false
				}
				return renderPairLine(id, score), true, false
			}),
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, source, rec.RegisterPath)
				if err != nil {
					return nil, err
				}
				field := &cur.Lifts
				if relation == kb.RelationRefutes {
					field = &cur.Refutes
				}
				at := slices.IndexFunc(*field, func(p pair) bool { return p.ID == target })
				if at < 0 {
					return nil, opRefusal("id="+source, fmt.Sprintf("stages no %s pair for %s in this register", key, target),
						"restore: author the pair, then re-run")
				}
				(*field)[at] = pair{ID: target, Score: score}
				return []expectedEntry{cur}, nil
			}})
	}
	if len(intents) == 0 {
		return nil, opRefusal("id="+source, fmt.Sprintf("authors no %s pair for %s in either home — it is declared in "+
			"no document's frontmatter and its register entry stages no such pair", key, target),
			"restore: author the pair, then re-run")
	}
	return intents, nil
}

// holdsDeclarationPair is whether nodeID's declaration in text holds a pair
// naming target under key.
func holdsDeclarationPair(text, nodeID, key, target string) bool {
	decl, err := observedDeclaration(text, nodeID, "")
	if err != nil || decl == nil {
		return false
	}
	for _, list := range decl.lists() {
		if list.key != key {
			continue
		}
		for _, p := range list.pairs {
			if p.ID == target {
				return true
			}
		}
	}
	return false
}

// replacePremiseBearing rewrites the (bearing …) annotation on target's
// rests-on bullet naming source.
func replacePremiseBearing(target, source, relation string, bearing kb.Bearing) func(string) (string, error) {
	annotation := kb.RenderBearingAnnotation(bearing)
	return func(document string) (string, error) {
		loc, err := locate(document, target)
		if err != nil {
			return "", err
		}
		lines := kb.SplitLines(document)
		for i := loc.RigorStart; i < min(loc.RigorEnd, len(lines)); i++ {
			if !headNamesSource(kb.BulletHead(kb.TrimBulletLead(kb.Strip(lines[i]))), source) {
				continue
			}
			m := kb.BearingRE.FindStringIndex(lines[i])
			if m == nil {
				return "", spliceFailed("%s's bullet for %s carries no %s annotation", target, source, annotation)
			}
			replaced := lines[i][:m[0]] + annotation + lines[i][m[1]:]
			return spliceLines(document, i, i+1, []string{replaced}), nil
		}
		return "", spliceFailed("%s carries no rests-on bullet naming %s", target, source)
	}
}

// headNamesSource is whether a rests-on bullet's head names source: the id as
// bullet heads write it, or the "Axiom <label>" citation an axm- id stands
// for.
func headNamesSource(head, source string) bool {
	if head == source {
		return true
	}
	return strings.HasPrefix(source, axiomIDPrefix) && head == "Axiom "+strings.TrimPrefix(source, axiomIDPrefix)
}

// planDropClaimEdges removes each entry's outgoing edge bullets of one class,
// or both, from its register entry, as dropListBullets does. Demoted bullets
// are resolve-demoted's to remove.
func planDropClaimEdges(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		nodeID := e.str("id")
		rec, err := c.resolve(nodeID, "id", true)
		if err != nil {
			return nil, err
		}
		register, err := c.contained(rec.RegisterPath, "id")
		if err != nil {
			return nil, err
		}
		both := !e.has("edge-class")
		class := e.str("edge-class")
		classes := []string{kb.ListRestsOn, kb.RelationReferences}
		if !both {
			classes = []string{class}
		}
		var splices []func(string) (string, error)
		for _, list := range classes {
			splices = append(splices, dropListBullets(nodeID, list, func([]string) bool { return true }))
		}
		intents = append(intents, intent{path: rec.RegisterPath, target: register,
			splice: spliceAll(splices...),
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, nodeID, rec.RegisterPath)
				if err != nil {
					return nil, err
				}
				if both || class == kb.RelationReferences {
					cur.References = nil
				}
				if both || class == kb.ListRestsOn {
					cur.Premises = nil
				}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// planDropInboundEdges removes every authored edge naming id: each register
// entry's rests-on bullet naming it, each entry's references bullet pointing at
// it, each claim's refutation of it, and each analysis's or experiment's lift,
// refutation or support of it — authored in a leaf's declaration or a
// register's staged copy. An entry authoring no such edge is left as it stands.
func planDropInboundEdges(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		target := e.str("id")
		if _, err := c.resolve(target, "id", true); err != nil {
			return nil, err
		}
		st, err := kb.Discover(c.src, log.Discard())
		if err != nil {
			return nil, err
		}
		claims := st.ClaimEntries[:0:0]
		for _, ce := range st.ClaimEntries {
			if namesSource(ce.Premises, target) || edgesTarget(ce.References, target) || edgesTarget(ce.Refutes, target) {
				claims = append(claims, ce)
			}
		}
		for _, ce := range claims {
			register, err := c.contained(ce.CanonicalPath, "id")
			if err != nil {
				return nil, err
			}
			intents = append(intents, intent{path: ce.CanonicalPath, target: register,
				splice: dropInboundEdges(ce.ID, target),
				expectCurrent: func(read registerRead) ([]expectedEntry, error) {
					cur, err := currentEntry(read, ce.ID, ce.CanonicalPath)
					if err != nil {
						return nil, err
					}
					cur.Premises = slices.DeleteFunc(cur.Premises, func(d expectedEdge) bool { return d.Source == target })
					cur.References = slices.DeleteFunc(cur.References, func(d expectedEdge) bool { return d.Target == target })
					cur.Refutes = slices.DeleteFunc(cur.Refutes, func(p pair) bool { return p.ID == target })
					return []expectedEntry{cur}, nil
				}})
		}
		for _, ana := range st.Analyses {
			if namesSource(ana.Premises, target) {
				register, err := c.contained(ana.CanonicalPath, "id")
				if err != nil {
					return nil, err
				}
				id := ana.ID
				intents = append(intents, intent{path: ana.CanonicalPath, target: register,
					splice: dropInboundEdges(id, target),
					expectCurrent: func(read registerRead) ([]expectedEntry, error) {
						cur, err := currentEntry(read, id, ana.CanonicalPath)
						if err != nil {
							return nil, err
						}
						cur.Premises = slices.DeleteFunc(cur.Premises, func(d expectedEdge) bool { return d.Source == target })
						return []expectedEntry{cur}, nil
					}})
			}
			for _, list := range []struct {
				key   string
				edges []kb.Edge
			}{{kb.LiftsKey, ana.Lifts}, {kb.RefutesKey, ana.Refutes}} {
				if !edgesTarget(list.edges, target) {
					continue
				}
				leaf, err := c.contained(ana.CanonicalPath, "id")
				if err != nil {
					return nil, err
				}
				id := ana.ID
				intents = append(intents, intent{path: ana.CanonicalPath, target: leaf,
					splice: dropDeclarationPairs(id, list.key, target),
					prove:  declarationPairAbsent(id, list.key, target, ana.CanonicalPath), subject: ana.CanonicalPath + ":" + id})
			}
			if rec, ok := c.known[ana.ID]; ok && rec.RegisterPath != "" && c.stagesPair(rec.RegisterPath, ana.ID, "", target) {
				register, err := c.contained(rec.RegisterPath, "id")
				if err != nil {
					return nil, err
				}
				id := ana.ID
				intents = append(intents, intent{path: rec.RegisterPath, target: register,
					splice: spliceAll(editStagedPair(id, kb.RelationLifts, dropPair(target)),
						editStagedPair(id, kb.RelationRefutes, dropPair(target))),
					expectCurrent: func(read registerRead) ([]expectedEntry, error) {
						cur, err := currentEntry(read, id, rec.RegisterPath)
						if err != nil {
							return nil, err
						}
						cur.Lifts = slices.DeleteFunc(cur.Lifts, func(p pair) bool { return p.ID == target })
						cur.Refutes = slices.DeleteFunc(cur.Refutes, func(p pair) bool { return p.ID == target })
						return []expectedEntry{cur}, nil
					}})
			}
		}
		for _, x := range st.Experiments {
			for _, list := range []struct {
				key   string
				edges []kb.Edge
			}{{kb.SupportsKey, x.Supports}, {kb.LiftsKey, x.Lifts}, {kb.RefutesKey, x.Refutes}} {
				if !edgesTarget(list.edges, target) {
					continue
				}
				leaf, err := c.contained(x.CanonicalPath, "id")
				if err != nil {
					return nil, err
				}
				id := x.ID
				intents = append(intents, intent{path: x.CanonicalPath, target: leaf,
					splice: dropDeclarationPairs(id, list.key, target),
					prove:  declarationPairAbsent(id, list.key, target, x.CanonicalPath), subject: x.CanonicalPath + ":" + id})
			}
		}
	}
	return intents, nil
}

// stagesPair is whether a register entry stages a lift or refutation of
// target under relation; an empty relation asks either list. A claim's
// refutation list is staged the same way.
func (c *opContext) stagesPair(register, source, relation, target string) bool {
	text, err := c.src.ReadText(c.src.KBPath(register))
	if err != nil {
		return false
	}
	read, err := readRegister(text)
	if err != nil {
		return false
	}
	rec, ok := read.record(source)
	if !ok {
		return false
	}
	staged := func(pairs []pair) bool {
		return slices.ContainsFunc(pairs, func(p pair) bool { return p.ID == target })
	}
	switch relation {
	case kb.RelationLifts:
		return staged(rec.Lifts)
	case kb.RelationRefutes:
		return staged(rec.Refutes)
	}
	return staged(rec.Lifts) || staged(rec.Refutes)
}

// namesSource is whether any edge of edges is authored from source: a premise
// bullet naming it, or a pair list owned by it.
func namesSource(edges []kb.Edge, source string) bool {
	return slices.ContainsFunc(edges, func(e kb.Edge) bool { return e.Source == source })
}

// dropPair is an editStagedPair callback that removes the pair naming target.
func dropPair(target string) func(string) (string, bool, bool) {
	return func(id string) (string, bool, bool) {
		if id != target {
			return "", false, false
		}
		return "", true, true
	}
}

// declarationPairAbsent proves a leaf declaration no longer holds the pair
// naming target under key.
func declarationPairAbsent(nodeID, key, target, rel string) func(string) ([]string, error) {
	return func(candidate string) ([]string, error) {
		if holdsDeclarationPair(candidate, nodeID, key, target) {
			return []string{key}, nil
		}
		return nil, nil
	}
}

// edgesTarget is whether any edge of edges names target.
func edgesTarget(edges []kb.Edge, target string) bool {
	return slices.ContainsFunc(edges, func(d kb.Edge) bool { return d.Target == target })
}

// dropInboundEdges deletes every bullet of nodeID's rests-on, refutes and
// references lists naming target, as dropListBullets does.
func dropInboundEdges(nodeID, target string) func(string) (string, error) {
	return spliceAll(dropListBullets(nodeID, kb.ListRestsOn, bulletNames(target)),
		dropListBullets(nodeID, kb.RefutesKey, bulletNames(target)),
		dropListBullets(nodeID, kb.RelationReferences, bulletNames(target)))
}

// bulletNames matches a bullet whose head names any of targets.
func bulletNames(targets ...string) func(bulletIDs []string) bool {
	return func(bulletIDs []string) bool {
		return slices.ContainsFunc(bulletIDs, func(id string) bool { return slices.Contains(targets, id) })
	}
}

// spliceAll is splices applied in order, each to the last one's result.
func spliceAll(splices ...func(string) (string, error)) func(string) (string, error) {
	return func(document string) (string, error) {
		for _, sp := range splices {
			var err error
			if document, err = sp(document); err != nil {
				return "", err
			}
		}
		return document, nil
	}
}

// planDemoteClaimEdge turns nodeID's existing premise edge to target into a
// demoted row of nodeID's own entry, carrying the given origin: the bullet
// goes from the rests-on list and a demoted bullet naming target lands in its
// demoted list. Where the demoted row already stands with that origin, a
// premise bullet still naming target is removed and otherwise the pair is
// left as it stands; a row of another origin is refused.
func planDemoteClaimEdge(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		nodeID, target := e.str("id"), e.str("target")
		rec, err := c.resolve(nodeID, "id", true)
		if err != nil {
			return nil, err
		}
		if _, err := c.resolve(target, "target", true); err != nil {
			return nil, err
		}
		register, err := c.contained(rec.RegisterPath, "id")
		if err != nil {
			return nil, err
		}
		read, err := c.src.ReadText(c.src.KBPath(rec.RegisterPath))
		if err != nil {
			return nil, err
		}
		read_, err := readRegister(read)
		if err != nil {
			return nil, err
		}
		cur, err := currentEntry(read_, nodeID, rec.RegisterPath)
		if err != nil {
			return nil, err
		}
		origin := e.str("origin")
		depends := slices.IndexFunc(cur.Premises, func(d expectedEdge) bool { return d.Source == target })
		if demoted := slices.IndexFunc(cur.Demoted, func(d expectedEdge) bool { return d.Target == target }); demoted >= 0 {
			if standing := cur.Demoted[demoted].Origin; standing != origin {
				return nil, opRefusal("origin="+origin, fmt.Sprintf("%s's demoted row naming %s stands with origin %s, not %s",
					nodeID, target, standing, origin),
					"restore: resolve the standing row with kbase resolve-demoted, then re-run")
			}
			if depends < 0 {
				continue
			}
			intents = append(intents, intent{path: rec.RegisterPath, target: register,
				splice: dropListBullets(nodeID, kb.ListRestsOn, bulletNames(target)),
				expectCurrent: func(read registerRead) ([]expectedEntry, error) {
					cur, err := currentEntry(read, nodeID, rec.RegisterPath)
					if err != nil {
						return nil, err
					}
					cur.Premises = slices.DeleteFunc(cur.Premises, func(d expectedEdge) bool { return d.Source == target })
					return []expectedEntry{cur}, nil
				}})
			continue
		}
		if depends < 0 {
			return nil, opRefusal("target="+target, fmt.Sprintf("%s carries no rests-on bullet naming %s, and this op demotes "+
				"an existing edge, not adds a demoted row; only the build writes a demoted edge and add-rests-on adds the premise bullet", nodeID, target),
				"restore: add the edge with kbase add-rests-on, or correct target, then re-run")
		}
		t := premiseTarget{Target: target, Origin: origin}
		if t.Title, err = c.titleOf(target); err != nil {
			return nil, err
		}
		if e.has("context") {
			t.Context = e.str("context")
		}
		intents = append(intents, intent{path: rec.RegisterPath, target: register,
			splice: demoteClaimEdge(nodeID, target, t),
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, nodeID, rec.RegisterPath)
				if err != nil {
					return nil, err
				}
				cur.Premises = slices.DeleteFunc(cur.Premises, func(d expectedEdge) bool { return d.Source == target })
				cur.Demoted = append(cur.Demoted, expectedEdge{Source: nodeID, Target: target, Context: t.Context, Origin: t.Origin})
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// demoteClaimEdge deletes nodeID's rests-on bullet naming target, with its
// continuation lines and the list's header where no bullet is left, and
// appends the demoted bullet to the entry's demoted list, its header created
// where absent.
func demoteClaimEdge(nodeID, target string, t premiseTarget) func(string) (string, error) {
	return spliceAll(dropListBullets(nodeID, kb.ListRestsOn, bulletNames(target)),
		addBullets(&edgeAddition{nodeID: nodeID, section: kb.RelationDemoted, requested: []premiseTarget{t}, added: []premiseTarget{t}}, kb.RelationDemoted))
}

// planDropClaimNode deletes claim nodes entirely: each one's register entry
// block, its claim-rigor marker and claims entry in every leaf carrying
// either, and every other entry's demoted row naming it. An entry still
// naming a node in its edges is refused — the caller drops edges first — and
// so is a leaf whose claims list would empty, an empty claims list being
// no-claim's decision to make, and any line still naming a node once the
// removals are composed. A node already gone has only what remains of it
// removed.
func planDropClaimNode(c *opContext, entries []entry) ([]intent, error) {
	st, err := kb.Discover(c.src, log.Discard())
	if err != nil {
		return nil, err
	}
	var ids []string
	dropping := map[string]bool{}
	for _, e := range entries {
		ids = append(ids, e.str("id"))
		dropping[e.str("id")] = true
	}
	for _, nodeID := range ids {
		var refusals result.Refusal
		stranded := func(owner, authoring string) {
			refusals = append(refusals, result.Item{Key: "id=" + owner,
				Remedy: "run kbase drop-claim-edges and kbase drop-inbound-edges first, then re-run",
				Detail: fmt.Sprintf("%s's %s still names %s, and dropping the node would strand the edge; "+
					"run drop-claim-edges and drop-inbound-edges first", owner, authoring, nodeID)})
		}
		for _, ce := range st.ClaimEntries {
			if namesSource(ce.Premises, nodeID) {
				stranded(ce.ID, "rests-on list")
			}
			if edgesTarget(ce.References, nodeID) {
				stranded(ce.ID, "references list")
			}
			if edgesTarget(ce.Refutes, nodeID) {
				stranded(ce.ID, "refutes list")
			}
		}
		for _, ana := range st.Analyses {
			if namesSource(ana.Premises, nodeID) {
				stranded(ana.ID, "rests-on list")
			}
			for _, edges := range [][]kb.Edge{ana.Lifts, ana.Refutes} {
				if edgesTarget(edges, nodeID) {
					stranded(ana.ID, "pair list")
				}
			}
		}
		for _, x := range st.Experiments {
			for _, edges := range [][]kb.Edge{x.Supports, x.Lifts, x.Refutes} {
				if edgesTarget(edges, nodeID) {
					stranded(x.ID, "pair list")
				}
			}
		}
		if len(refusals) > 0 {
			return nil, refusals
		}
	}
	var emptied result.Refusal
	for _, leaf := range st.Leaves {
		if len(leaf.Claims) > 0 && !slices.ContainsFunc(leaf.Claims, func(id string) bool { return !dropping[id] }) {
			emptied = append(emptied, result.Item{Key: leaf.Path,
				Remedy: "restate the leaf's claims with kbase set-frontmatter, no-claim included, then re-run",
				Detail: fmt.Sprintf("%s's claims list holds only %s, which this call drops, and an empty claims list is "+
					"a no-claim decision set-frontmatter makes, not this op's", leaf.Path, strings.Join(leaf.Claims, ", "))})
		}
	}
	if len(emptied) > 0 {
		return nil, emptied
	}
	var intents []intent
	for _, nodeID := range ids {
		if rec, ok := c.known[nodeID]; ok && rec.RegisterPath != "" {
			register, err := c.contained(rec.RegisterPath, "id")
			if err != nil {
				return nil, err
			}
			intents = append(intents, intent{path: rec.RegisterPath, target: register,
				splice: dropEntryBlock(nodeID), claimDelta: -1})
		}
		for _, leaf := range st.Leaves {
			text, err := c.src.ReadText(c.src.KBPath(leaf.Path))
			if err != nil {
				return nil, err
			}
			if !slices.Contains(leaf.Claims, nodeID) && !strings.Contains(text, renderTier2Marker(nodeID)) {
				continue
			}
			target, err := c.contained(leaf.Path, "document")
			if err != nil {
				return nil, err
			}
			intents = append(intents, intent{path: leaf.Path, target: target,
				splice: dropLeafClaim(nodeID, leaf.Path),
				prove:  claimNodeProver(nodeID, leaf.Path), subject: leaf.Path + ":" + nodeID})
		}
	}
	for _, ce := range st.ClaimEntries {
		if dropping[ce.ID] || !slices.ContainsFunc(ce.Demoted, func(d kb.Edge) bool { return dropping[d.Target] }) {
			continue
		}
		register, err := c.contained(ce.CanonicalPath, "id")
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent{path: ce.CanonicalPath, target: register,
			splice: dropListBullets(ce.ID, kb.RelationDemoted, bulletNames(ids...)),
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, ce.ID, ce.CanonicalPath)
				if err != nil {
					return nil, err
				}
				cur.Demoted = slices.DeleteFunc(cur.Demoted, func(d expectedEdge) bool { return dropping[d.Target] })
				return []expectedEntry{cur}, nil
			}})
	}
	if err := refuseRemainingMentions(c, intents, ids); err != nil {
		return nil, err
	}
	return intents, nil
}

// refuseRemainingMentions refuses where, with intents composed, any line
// under kb-root still names one of ids: verify's claim-id link check reads
// every such line, so a drop leaving one leaves an unknown id. Refresh's
// roll-ups are not read, the trailing refresh rewriting them. An edit that
// does not compose is left to the store, which composes it again and reports
// it.
func refuseRemainingMentions(c *opContext, intents []intent, ids []string) error {
	candidates := map[string]string{}
	for _, e := range batch(intents) {
		text, err := c.src.ReadText(c.src.KBPath(e.path))
		if err != nil {
			return nil
		}
		if candidates[e.path], err = e.splice(text); err != nil {
			return nil
		}
	}
	files, err := kb.MarkdownFiles(c.src)
	if err != nil {
		return err
	}
	var refusals result.Refusal
	for _, file := range files {
		rel, err := filepath.Rel(c.root, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		text, composed := candidates[rel]
		if !composed {
			if text, err = c.src.ReadText(file); err != nil {
				continue
			}
		}
		for n, line := range kb.SplitLines(kb.StripCodeSplitLines(withoutRollups(text))) {
			named := kb.NodeIDs(line)
			for _, id := range ids {
				if !slices.Contains(named, id) {
					continue
				}
				refusals = append(refusals, result.Item{Key: "id=" + id, Path: rel, Line: n + 1,
					Remedy: "remove the mention — through the op that wrote it, or by hand where it was authored — then re-run",
					Detail: fmt.Sprintf("%s:%d names %s, a mention this op does not remove, and once the node is gone "+
						"verify's claim-id link check reports it as an unknown id", rel, n+1, id)})
			}
		}
	}
	if len(refusals) > 0 {
		return refusals
	}
	return nil
}

// withoutRollups is text with refresh's frontmatter roll-ups blanked, every
// line kept where it was.
func withoutRollups(text string) string {
	for _, d := range derivedFrontmatterFields {
		_, start, end, ok := kb.FrontmatterKeySpan(text, d[0])
		if !ok {
			continue
		}
		text = kb.EditFrontmatterLines(text, func(lines []string) []string {
			for i := start; i < end; i++ {
				lines[i] = ""
			}
			return lines
		})
	}
	return text
}

// dropEntryBlock deletes nodeID's whole register entry: from its heading, or
// its marker where it binds none, back over the separating rule, to the next
// entry's start, or the file's end.
func dropEntryBlock(nodeID string) func(string) (string, error) {
	return func(document string) (string, error) {
		located := kb.LocateRegisterEntries(document)
		idx := slices.IndexFunc(located, func(e kb.RegisterEntry) bool { return e.NodeID == nodeID })
		if idx < 0 {
			return "", spliceFailed("no canonical entry for %s in this register", nodeID)
		}
		e := located[idx]
		start := e.MarkerLine
		if e.HeadingLine >= 0 {
			start = e.HeadingLine
		}
		end := len(kb.SplitLines(document))
		for _, n := range located[idx+1:] {
			at := n.HeadingLine
			if at < 0 {
				at = n.MarkerLine
			}
			if at > start {
				end = at
				break
			}
		}
		fmEnd := 0
		if m := kb.FindFrontmatter(document); m != nil {
			fmEnd = strings.Count(document[:m[1]], "\n") + 1
		}
		for start > fmEnd {
			s := kb.Strip(kb.SplitLines(document)[start-1])
			if s != "" && s != "---" {
				break
			}
			start--
		}
		return spliceLines(document, start, end, nil), nil
	}
}

// dropLeafClaim removes nodeID's claim-rigor marker from every line it
// names it on, and nodeID from the frontmatter claims list. A line the
// marker leaves without prose goes with it.
func dropLeafClaim(nodeID, rel string) func(string) (string, error) {
	marker := renderTier2Marker(nodeID)
	return func(document string) (string, error) {
		if strings.Contains(document, marker) {
			var kept []string
			for _, l := range kb.SplitLinesKeepEnds(document) {
				if !strings.Contains(l, marker) {
					kept = append(kept, l)
					continue
				}
				body := strings.ReplaceAll(strings.ReplaceAll(l, " "+marker, ""), marker, "")
				if kb.Strip(body) == "" {
					continue
				}
				kept = append(kept, body)
			}
			document = strings.Join(kept, "")
		}
		if _, _, _, ok := kb.FrontmatterKeySpan(document, "claims"); !ok {
			return document, nil
		}
		fm, err := kb.ParseFrontmatter(document)
		if err != nil {
			return "", kb.MalformedError{Msg: rel + ": " + err.Error()}
		}
		claims, err := fm.ListOrEmpty("claims")
		if err != nil {
			return "", kb.MalformedError{Msg: rel + ": claims: " + err.Error()}
		}
		kept := slices.DeleteFunc(slices.Clone(claims), func(id string) bool { return id == nodeID })
		return kb.ReplaceOrInsertFrontmatterField(document, "claims", kept, "kind:"), nil
	}
}

// claimNodeProver reads the written leaf back: the marker and the claims
// entry must both have gone, and a claims list left empty must stand beside
// no-claim.
func claimNodeProver(nodeID, rel string) func(string) ([]string, error) {
	marker := renderTier2Marker(nodeID)
	return func(candidate string) ([]string, error) {
		fm, err := kb.ParseFrontmatter(candidate)
		if err != nil {
			return nil, kb.MalformedError{Msg: rel + ": " + err.Error()}
		}
		var wrong []string
		if strings.Contains(kb.StripFrontmatter(candidate), marker) {
			wrong = append(wrong, "claim-rigor marker")
		}
		claims, err := fm.ListOrEmpty("claims")
		if err != nil {
			return nil, kb.MalformedError{Msg: rel + ": claims: " + err.Error()}
		}
		_, listed := fm.Get("claims")
		_, noClaim := fm.Get("no-claim")
		if slices.Contains(claims, nodeID) || listed && len(claims) == 0 && !noClaim {
			wrong = append(wrong, "claims")
		}
		return wrong, nil
	}
}

// edgeAddition is one entry's requested edges of one list and the subset it
// does not already hold, decided once from the baseline.
type edgeAddition struct {
	nodeID, section string
	requested       []premiseTarget
	added           []premiseTarget
}

// narrow keeps the requested edges present lacks; a target named twice is
// kept once.
func (a *edgeAddition) narrow(present map[string]bool) {
	held := map[string]bool{}
	for k := range present {
		held[k] = true
	}
	a.added = nil
	for _, t := range a.requested {
		edge := readerTarget(a.nodeID, a.section, t)
		if held[edge] {
			continue
		}
		held[edge] = true
		a.added = append(a.added, t)
	}
}

// readerTarget is the id the reader keys this edge under once it is a
// bullet: the rendered bullet put through the reader that will read it.
func readerTarget(nodeID, section string, t premiseTarget) string {
	if section == kb.ListRestsOn {
		for _, e := range kb.ParsePremisesBullet(renderRestsOnBullet(t), nodeID, kb.NodeKindClaim) {
			return e.Source
		}
	}
	return t.Target
}

var (
	bulletRenderers = map[string]func(premiseTarget) string{
		kb.ListRestsOn:     renderRestsOnBullet,
		"references":       renderReferencesBullet,
		kb.RelationDemoted: renderDemotedBullet,
	}
	// sectionAnchors are the lines a missing section is placed above, the
	// first present winning, so the reader's field order survives any order
	// of writing. The derived heading and its line are both anchors: an entry
	// written before the two headings carries the line alone.
	sectionAnchors = map[string][]string{
		kb.ListRestsOn:     {"- references:", "- " + kb.RelationDemoted + ":", "- strengthen-by:", "### Strength", "- strength:"},
		"references":       {"- " + kb.RelationDemoted + ":", "- strengthen-by:", "### Strength", "- strength:"},
		kb.RelationDemoted: {"- strengthen-by:", "### Strength", "- strength:"},
	}
	// edgeSections is each list an entry's edges are added to, in field
	// order, with the edges of the entry's record it holds, the id a bullet of
	// that list names, and the expectation adding one writes.
	edgeSections = []struct {
		name     string
		held     func(*expectedEntry) *[]expectedEdge
		named    func(expectedEdge) string
		expected func(nodeID string, t premiseTarget) expectedEdge
	}{
		{kb.ListRestsOn,
			func(e *expectedEntry) *[]expectedEdge { return &e.Premises },
			func(e expectedEdge) string { return e.Source },
			func(nodeID string, t premiseTarget) expectedEdge {
				return expectedEdge{Source: t.Target, Target: nodeID, Context: t.Context, Bearing: bearingOf(t.Bearing)}
			}},
		{"references",
			func(e *expectedEntry) *[]expectedEdge { return &e.References },
			func(e expectedEdge) string { return e.Target },
			func(nodeID string, t premiseTarget) expectedEdge {
				return expectedEdge{Source: nodeID, Target: t.Target, Context: t.Context}
			}},
		{kb.RelationDemoted,
			func(e *expectedEntry) *[]expectedEdge { return &e.Demoted },
			func(e expectedEdge) string { return e.Target },
			func(nodeID string, t premiseTarget) expectedEdge {
				return expectedEdge{Source: nodeID, Target: t.Target, Context: t.Context, Origin: t.Origin}
			}},
	}
)

// addBullets appends an entry's new bullets below the last existing one of
// a list, so no line above moves and every derived annotation stays put.
func addBullets(a *edgeAddition, section string) func(string) (string, error) {
	header := "- " + section + ":"
	breakRE := foldBreakFor(a.nodeID)
	return func(document string) (string, error) {
		var bullets []string
		for _, t := range a.added {
			bullets = append(bullets, bulletRenderers[section](t))
		}
		if len(bullets) == 0 {
			return document, nil
		}
		loc, err := locate(document, a.nodeID)
		if err != nil {
			return "", err
		}
		lines := kb.SplitLines(document)
		limit := min(loc.RigorEnd, len(lines))
		first := func(prefix string) int {
			for i := loc.RigorStart; i < limit; i++ {
				if strings.HasPrefix(kb.Strip(lines[i]), prefix) {
					return i
				}
			}
			return -1
		}
		if head := first(header); head >= 0 {
			i := head + 1
			for i < limit && kb.Strip(lines[i]) != "" && !atHeading(kb.Strip(lines[i])) && !breakRE.MatchString(kb.Strip(lines[i])) {
				i++
			}
			return spliceLines(document, i, i, bullets), nil
		}
		for _, anchor := range sectionAnchors[section] {
			at := first(anchor)
			if at < 0 {
				continue
			}
			start := at
			added := append([]string{header}, bullets...)
			if strings.HasPrefix(anchor, "#") {
				// The heading's own blank line moves above the new list, so the
				// entry keeps one blank between a list and the heading below it.
				added = append(added, "")
				if at > loc.RigorStart && kb.Strip(lines[at-1]) == "" {
					start = at - 1
				}
			}
			return spliceLines(document, start, at, added), nil
		}
		return "", spliceFailed("%s has no %s line to anchor a %s section above", a.nodeID, strings.Join(sectionAnchors[section], " or "), section)
	}
}

func planAddRestsOn(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		nodeID := e.str("id")
		rec, err := c.resolve(nodeID, "id", true)
		if err != nil {
			return nil, err
		}
		register, err := c.contained(rec.RegisterPath, "id")
		if err != nil {
			return nil, err
		}
		var sections []int
		additions := map[int]*edgeAddition{}
		for i, s := range edgeSections {
			targets, err := c.premiseTargets(e.restsOn(s.name))
			if err != nil {
				return nil, err
			}
			if len(targets) == 0 {
				continue
			}
			if s.name != kb.ListRestsOn && !isClaimID(nodeID) {
				return nil, opRefusal(nodeID, "carries a "+s.name+" list, and a reference is one claim of this corpus naming another",
					"restore: drop the "+s.name+" list, or name a claim entry as the id, then re-run")
			}
			sections = append(sections, i)
			additions[i] = &edgeAddition{nodeID: nodeID, section: s.name, requested: targets, added: targets}
		}
		if len(sections) == 0 {
			return nil, opRefusal(nodeID, "this op adds an entry's outgoing edges and neither list names one",
				"restore: supply a rests-on list, a references list, or both, then re-run")
		}
		var splices []func(string) (string, error)
		for _, i := range sections {
			splices = append(splices, addBullets(additions[i], edgeSections[i].name))
		}
		intents = append(intents, intent{path: rec.RegisterPath, target: register,
			splice: func(document string) (string, error) {
				for _, sp := range splices {
					var err error
					if document, err = sp(document); err != nil {
						return "", err
					}
				}
				return document, nil
			},
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, nodeID, rec.RegisterPath)
				if err != nil {
					return nil, err
				}
				for _, i := range sections {
					a := additions[i]
					held := edgeSections[i].held(&cur)
					present := map[string]bool{}
					for _, edge := range *held {
						present[edgeSections[i].named(edge)] = true
					}
					a.narrow(present)
					for _, t := range a.added {
						*held = append(*held, edgeSections[i].expected(nodeID, t))
					}
				}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// demotedResolution is one entry's changes to its demoted list: the targets
// whose demoted bullet goes, and of them the ones gaining a rests-on bullet.
type demotedResolution struct {
	register, target string
	drop             map[string]bool
	restore          []premiseTarget
}

// planResolveDemoted removes each named demoted edge, or rewrites it as a
// rests-on bullet where that closes no cycle of the authored premise graph,
// the batch's earlier restores included. A pair already as asked — no
// demoted edge to remove, or a premise edge and no demoted one to restore —
// is left as it stands.
func planResolveDemoted(c *opContext, entries []entry) ([]intent, error) {
	st, err := kb.Discover(c.src, log.Discard())
	if err != nil {
		return nil, err
	}
	graph := index.AuthoredDependsGraph(st)
	claims := map[string]kb.ClaimEntry{}
	for _, e := range st.ClaimEntries {
		claims[e.ID] = e
	}
	var order []string
	byNode := map[string]*demotedResolution{}
	named := map[[2]string]int{}
	for _, e := range entries {
		nodeID, target, action := e.str("id"), e.str("target"), e.str("action")
		pair := [2]string{nodeID, target}
		if first, ok := named[pair]; ok {
			return nil, result.Refusal{{Entry: e.Index, Key: "target", Remedy: "drop one of the two entries, then re-run",
				Detail: fmt.Sprintf("names %s → %s, which entry %d already names; a pair takes one action per batch", nodeID, target, first)}}
		}
		named[pair] = e.Index
		rec, err := c.resolve(nodeID, "id", true)
		if err != nil {
			return nil, err
		}
		if _, err := c.resolve(target, "target", true); err != nil {
			return nil, err
		}
		cur := claims[nodeID]
		demoted := slices.IndexFunc(cur.Demoted, func(d kb.Edge) bool { return d.Target == target })
		depends := slices.ContainsFunc(cur.Premises, func(d kb.Edge) bool { return d.Relation == kb.RelationSupports && d.Source == target })
		r := byNode[nodeID]
		if demoted < 0 {
			if action == actionRemove || depends {
				continue
			}
			return nil, opRefusal("target="+target, fmt.Sprintf("%s carries no demoted edge to %s, so there is nothing to restore; "+
				"only the build writes a demoted edge", nodeID, target), "restore: add the edge with kbase add-rests-on, or correct target, then re-run")
		}
		if r == nil {
			register, err := c.contained(rec.RegisterPath, "id")
			if err != nil {
				return nil, err
			}
			r = &demotedResolution{register: rec.RegisterPath, target: register, drop: map[string]bool{}}
			byNode[nodeID] = r
			order = append(order, nodeID)
		}
		r.drop[target] = true
		done := resolvedRemove
		if action == actionRestore {
			if path := graph.Path(target, nodeID); path != nil {
				return nil, result.Refusal{{Check: checkDependencyCycle, Entry: e.Index, Key: "target",
					Remedy: "kbase resolve-demoted with action remove for this pair, or remove a premise edge on the named path first",
					Detail: fmt.Sprintf("restoring %s → %s as a premise edge would close the cycle %s", nodeID, target,
						strings.Join(append(path, target), " → "))}}
			}
			graph.Add(nodeID, target)
			done = resolvedRestore
			if !depends {
				d := cur.Demoted[demoted]
				t := premiseTarget{Target: target}
				if d.Context != nil {
					t.Context = *d.Context
				}
				if t.Title, err = c.titleOf(target); err != nil {
					return nil, err
				}
				r.restore = append(r.restore, t)
			}
		}
		c.resolved = append(c.resolved, Resolution{Source: nodeID, Target: target, Action: done})
	}
	var intents []intent
	for _, nodeID := range order {
		r := byNode[nodeID]
		splices := []func(string) (string, error){dropListBullets(nodeID, kb.RelationDemoted, bulletNames(slices.Collect(maps.Keys(r.drop))...))}
		if len(r.restore) > 0 {
			splices = append(splices, addBullets(&edgeAddition{nodeID: nodeID, section: kb.ListRestsOn, requested: r.restore, added: r.restore}, kb.ListRestsOn))
		}
		intents = append(intents, intent{path: r.register, target: r.target,
			splice: spliceAll(splices...),
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, nodeID, r.register)
				if err != nil {
					return nil, err
				}
				cur.Demoted = slices.DeleteFunc(cur.Demoted, func(d expectedEdge) bool { return r.drop[d.Target] })
				for _, t := range r.restore {
					cur.Premises = append(cur.Premises, expectedEdge{Source: t.Target, Target: nodeID, Context: t.Context, Bearing: kb.Bearing{Pending: true}})
				}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// dropListBullets deletes, from nodeID's list headed "- list:", every bullet
// whose head's claim ids match accepts, each with its continuation lines. A
// list left with no bullet loses its header too, unless it still holds other
// text, which keeps the header so a re-issue finds the list as it left it. An
// absent list leaves the document as it stands.
func dropListBullets(nodeID, list string, match func(bulletIDs []string) bool) func(string) (string, error) {
	header := "- " + list + ":"
	breakRE := foldBreakFor(nodeID)
	return func(document string) (string, error) {
		loc, err := locate(document, nodeID)
		if err != nil {
			return "", err
		}
		lines := kb.SplitLines(document)
		limit := min(loc.RigorEnd, len(lines))
		head := slices.IndexFunc(lines[loc.RigorStart:limit], func(l string) bool { return strings.HasPrefix(kb.Strip(l), header) })
		if head < 0 {
			return document, nil
		}
		head += loc.RigorStart
		end := head + 1
		for end < limit && !breakRE.MatchString(kb.Strip(lines[end])) && kb.Strip(lines[end]) != "---" && !atHeading(kb.Strip(lines[end])) {
			end++
		}
		for end > head+1 && kb.Strip(lines[end-1]) == "" {
			end--
		}
		var kept []string
		bullets, text, dropping := 0, false, false
		for _, l := range lines[head+1 : end] {
			if l != kb.LStrip(l) && strings.HasPrefix(kb.LStrip(l), "- ") {
				dropping = match(kb.ClaimIDs(kb.BulletHead(kb.Strip(kb.TrimBulletLead(l)))))
				if !dropping {
					bullets++
				}
			}
			if dropping {
				continue
			}
			kept = append(kept, l)
			text = text || kb.Strip(l) != ""
		}
		if bullets == 0 && !text {
			return spliceLines(document, head, end, nil), nil
		}
		return spliceLines(document, head+1, end, kept), nil
	}
}

// knownFrontmatterKeys is every key a frontmatter block this API composes
// may carry; derivedFrontmatterFields are refresh's roll-ups, carried over
// verbatim at refresh's own anchors.
var (
	knownFrontmatterKeys = []string{"kind", "path-stable", "claims", "no-claim", "experiments", kb.ExperimentNodesKey,
		kb.AnalysisNodesKey, kb.FormatKey}
	derivedFrontmatterFields = [][2]string{{"subtree-claims", "kind:"}, {"subtree-experiments", "subtree-claims:"}}
)

func planSetFrontmatter(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		rel := e.str("document")
		target, err := c.contained(rel, "document")
		if err != nil {
			return nil, err
		}
		claims, experiments := e.list("claims"), e.list("experiments")
		expNodes, analyses := e.experimentNodes(), e.analyses()
		for _, id := range claims {
			if _, err := c.resolve(id, "claims", true); err != nil {
				return nil, err
			}
		}
		for _, id := range experiments {
			if _, err := c.resolve(id, "experiments", false); err != nil {
				return nil, err
			}
		}
		var redeclared []string
		for _, n := range expNodes {
			if _, err := c.resolve(n.ExpID, "experiment-node.exp-id", false); err != nil {
				return nil, err
			}
			for _, list := range []struct {
				key   string
				pairs []pair
			}{{kb.SupportsKey, n.Supports}, {kb.LiftsKey, n.Lifts}, {kb.RefutesKey, n.Refutes}} {
				for _, p := range list.pairs {
					if _, err := c.resolve(p.ID, "experiment-node."+list.key+".id", true); err != nil {
						return nil, err
					}
				}
			}
			redeclared = append(redeclared, n.ExpID)
		}
		for _, n := range analyses {
			if _, err := c.resolve(n.AnaID, "analysis-node.ana-id", true); err != nil {
				return nil, err
			}
			for _, list := range []struct {
				key   string
				pairs []pair
			}{{kb.LiftsKey, n.Lifts}, {kb.RefutesKey, n.Refutes}} {
				for _, p := range list.pairs {
					if _, err := c.resolve(p.ID, "analysis-node."+list.key+".id", true); err != nil {
						return nil, err
					}
				}
			}
			redeclared = append(redeclared, n.AnaID)
		}
		intended := frontmatterValues{Kind: e.str("kind"), Claims: claims, Experiments: experiments, ExperimentNodes: expNodes, Analyses: analyses}
		if e.has("path-stable") {
			s := e.str("path-stable")
			intended.PathStable = &s
		}
		if e.has("no-claim") {
			s := e.str("no-claim")
			intended.NoClaim = &s
		}
		intents = append(intents, intent{path: rel, target: target,
			splice: replaceBlock(intended, rel, redeclared),
			prove:  frontmatterProver(intended, rel), subject: rel})
	}
	return intents, nil
}

// replaceBlock replaces a document's whole frontmatter with intended's, or
// opens the document with a first one, carrying refresh's roll-ups and the
// format stamp over. What the replace would lose is read off the document
// the splice receives.
func replaceBlock(intended frontmatterValues, rel string, redeclared []string) func(string) (string, error) {
	return func(document string) (string, error) {
		existing, err := existingFrontmatter(document, rel, redeclared)
		if err != nil {
			return "", err
		}
		block, err := renderFrontmatterBlock(intended)
		if err != nil {
			return "", err
		}
		var candidate string
		if m := kb.FindFrontmatter(document); m == nil {
			candidate = block + "\n" + document
		} else {
			candidate = document[:m[0]] + block + document[m[1]:]
		}
		for _, d := range derivedFrontmatterFields {
			v, ok := existing.Get(d[0])
			if !ok {
				continue
			}
			ids, err := v.Items()
			if err != nil {
				return "", kb.MalformedError{Msg: rel + ": " + d[0] + ": " + err.Error()}
			}
			candidate = kb.ReplaceOrInsertFrontmatterField(candidate, d[0], ids, d[1])
		}
		if stamp := existing.Str(kb.FormatKey); stamp != "" {
			return kb.StampFormat(candidate, stamp)
		}
		return candidate, nil
	}
}

// existingFrontmatter is the document's current fields, refusing a replace
// that would drop a key this API cannot render or a hosted node declaration
// the values do not restate.
func existingFrontmatter(text, rel string, redeclared []string) (kb.Frontmatter, error) {
	fields, err := kb.ParseFrontmatter(text)
	if err != nil {
		return nil, kb.MalformedError{Msg: rel + ": " + err.Error()}
	}
	if fields == nil {
		return nil, nil
	}
	var unknown []string
	for key := range fields {
		if !slices.Contains(knownFrontmatterKeys, key) && !slices.ContainsFunc(derivedFrontmatterFields, func(d [2]string) bool { return d[0] == key }) {
			unknown = append(unknown, key)
		}
	}
	slices.Sort(unknown)
	if len(unknown) > 0 {
		return nil, opRefusal(rel, fmt.Sprintf("carries frontmatter key(s) %v that this API cannot render, so replacing the block would drop them", unknown),
			fmt.Sprintf("restore: remove %q from the block by hand, or leave this document's frontmatter alone, then re-run", unknown[0]))
	}
	var dropped []string
	for _, id := range kb.DeclaredNodeIDs(fields) {
		if !slices.Contains(redeclared, id) && !slices.Contains(dropped, id) {
			dropped = append(dropped, id)
		}
	}
	slices.Sort(dropped)
	if len(dropped) > 0 {
		return nil, opRefusal(rel, fmt.Sprintf("declares hosted node(s) %v that these values do not restate, and replacing the block would destroy the declaration rather than edit it", dropped),
			fmt.Sprintf("restore: restate %s's block in the values, or leave this document's frontmatter alone, then re-run", dropped[0]))
	}
	return fields, nil
}

// FrontmatterValues is document's current block as one set-frontmatter
// values entry, every key it carries restated, so a writer replacing the
// block changes only the keys it sets over this; nil where there is no block.
func FrontmatterValues(text, document string) (map[string]any, error) {
	v, err := observedFrontmatter(text, document)
	if err != nil || v == nil {
		return nil, err
	}
	out := map[string]any{"document": document, "kind": v.Kind}
	if v.PathStable != nil {
		out["path-stable"] = *v.PathStable
	}
	if len(v.Claims) > 0 {
		out["claims"] = v.Claims
	}
	if v.NoClaim != nil {
		out["no-claim"] = *v.NoClaim
	}
	if len(v.Experiments) > 0 {
		out["experiments"] = v.Experiments
	}
	score := func(s *float64) any {
		if s == nil {
			return kb.PendingLiteral
		}
		return *s
	}
	var exps []map[string]any
	for _, n := range v.ExperimentNodes {
		e := map[string]any{"exp-id": n.ExpID, "status": n.Status}
		if n.Rigor != nil {
			e["rigor"] = score(n.Rigor)
		}
		for _, list := range []struct {
			key   string
			pairs []pair
		}{{kb.SupportsKey, n.Supports}, {kb.LiftsKey, n.Lifts}, {kb.RefutesKey, n.Refutes}} {
			var rows []map[string]any
			for _, p := range list.pairs {
				rows = append(rows, map[string]any{"id": p.ID, "bearing": score(p.Score)})
			}
			if len(rows) > 0 {
				e[list.key] = rows
			}
		}
		exps = append(exps, e)
	}
	if len(exps) > 0 {
		out["experiment-node"] = exps
	}
	var anas []map[string]any
	for _, n := range v.Analyses {
		e := map[string]any{"ana-id": n.AnaID}
		for _, list := range []struct {
			key   string
			pairs []pair
		}{{kb.LiftsKey, n.Lifts}, {kb.RefutesKey, n.Refutes}} {
			var rows []map[string]any
			for _, p := range list.pairs {
				rows = append(rows, map[string]any{"id": p.ID, "bearing": score(p.Score)})
			}
			if len(rows) > 0 {
				e[list.key] = rows
			}
		}
		anas = append(anas, e)
	}
	if len(anas) > 0 {
		out["analysis-node"] = anas
	}
	return out, nil
}

// observedFrontmatter is the candidate's block as the production readers
// return it.
func observedFrontmatter(text, rel string) (*frontmatterValues, error) {
	fields, err := kb.ParseFrontmatter(text)
	if err != nil {
		return nil, kb.MalformedError{Msg: rel + ": " + err.Error()}
	}
	if fields == nil {
		return nil, nil
	}
	v := &frontmatterValues{Kind: fields.Kind()}
	for _, f := range []struct {
		key  string
		into **string
	}{{"path-stable", &v.PathStable}, {"no-claim", &v.NoClaim}} {
		if val, ok := fields.Get(f.key); ok {
			s := val.Str
			if val.IsList {
				s = "[" + strings.Join(val.List, ", ") + "]"
			}
			*f.into = &s
		}
	}
	if v.Claims, err = fields.ListOrEmpty("claims"); err != nil {
		return nil, err
	}
	if v.Experiments, err = fields.ListOrEmpty("experiments"); err != nil {
		return nil, err
	}
	exps, err := kb.ParseExperimentLeaf(text, rel)
	if err != nil {
		return nil, err
	}
	for _, n := range exps {
		d := experimentDecl{ExpID: n.ID, Status: n.Status, Rigor: n.Rigor, Supports: pairsOf(n.Supports),
			Lifts: pairsOf(n.Lifts), Refutes: pairsOf(n.Refutes)}
		v.ExperimentNodes = append(v.ExperimentNodes, d)
	}
	anas, err := kb.ParseAnalysisLeaf(text, rel)
	if err != nil {
		return nil, err
	}
	for _, n := range anas {
		v.Analyses = append(v.Analyses, analysisDecl{AnaID: n.ID, Lifts: pairsOf(n.Lifts), Refutes: pairsOf(n.Refutes)})
	}
	return v, nil
}

func optionalStringEqual(a, b *string) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

// frontmatterProver reads the whole intended block back, field by field.
func frontmatterProver(intended frontmatterValues, rel string) func(string) ([]string, error) {
	return func(candidate string) ([]string, error) {
		got, err := observedFrontmatter(candidate, rel)
		if err != nil {
			return nil, err
		}
		if got == nil {
			return []string{"frontmatter"}, nil
		}
		want := intended
		if want.PathStable != nil {
			s := collapse(*want.PathStable)
			want.PathStable = &s
		}
		if want.NoClaim != nil {
			s := collapse(*want.NoClaim)
			want.NoClaim = &s
		}
		var wrong []string
		for _, f := range []struct {
			key  string
			same bool
		}{
			{"kind", got.Kind == want.Kind},
			{"path-stable", optionalStringEqual(got.PathStable, want.PathStable)},
			{"claims", slices.Equal(got.Claims, want.Claims)},
			{"no-claim", optionalStringEqual(got.NoClaim, want.NoClaim)},
			{"experiments", slices.Equal(got.Experiments, want.Experiments)},
			{"experiment-node", slices.EqualFunc(got.ExperimentNodes, want.ExperimentNodes, func(a, b experimentDecl) bool {
				return a.ExpID == b.ExpID && a.Status == b.Status && scoreEqual(a.Rigor, b.Rigor) &&
					pairsEqual(a.Supports, b.Supports) && pairsEqual(a.Lifts, b.Lifts) && pairsEqual(a.Refutes, b.Refutes)
			})},
			{"analysis-node", slices.EqualFunc(got.Analyses, want.Analyses, func(a, b analysisDecl) bool {
				return a.AnaID == b.AnaID && pairsEqual(a.Lifts, b.Lifts) && pairsEqual(a.Refutes, b.Refutes)
			})},
		} {
			if !f.same {
				wrong = append(wrong, f.key)
			}
		}
		return wrong, nil
	}
}

// appendDeclaration appends one node declaration to the list under key in
// the document's frontmatter, rendered is that list's key and the one entry;
// a document without the list gains it at the end of its frontmatter.
func appendDeclaration(key string, rendered []string) func(string) (string, error) {
	return func(document string) (string, error) {
		if kb.FindFrontmatter(document) == nil {
			return "", spliceFailed("the document has no frontmatter to declare a node in")
		}
		if _, _, end, ok := kb.FrontmatterKeySpan(document, key); ok {
			return kb.EditFrontmatterLines(document, func(lines []string) []string {
				return slices.Concat(lines[:end], rendered[1:], lines[end:])
			}), nil
		}
		return kb.SetFrontmatterKey(document, key, rendered, ""), nil
	}
}

func planMarkClaim(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		rel, nodeID := e.str("document"), e.str("id")
		target, err := c.contained(rel, "document")
		if err != nil {
			return nil, err
		}
		if _, err := c.resolve(nodeID, "id", true); err != nil {
			return nil, err
		}
		if !c.src.IsFile(target) {
			return nil, opRefusal(rel, "does not exist. A marker is placed beside the prose it anchors, and this API never composes a document body",
				"restore: correct document, or author the document first, then re-run")
		}
		intents = append(intents, intent{path: rel, target: target,
			splice: insertMarker(renderTier2Marker(nodeID), e.str("locator"), nodeID, rel),
			prove:  markerProver(nodeID, rel), subject: rel + ":" + nodeID})
	}
	return intents, nil
}

// insertMarker appends a claim's marker to the end of the located line,
// inside whatever block that line is in and ahead of its trailing
// whitespace. A line already carrying the marker is left as it is.
func insertMarker(marker, locator, nodeID, rel string) func(string) (string, error) {
	return func(document string) (string, error) {
		fm, err := kb.ParseFrontmatter(document)
		if err != nil {
			return "", kb.MalformedError{Msg: rel + ": " + err.Error()}
		}
		claims, err := fm.ListOrEmpty("claims")
		if err != nil {
			return "", kb.MalformedError{Msg: rel + ": claims: " + err.Error()}
		}
		if !slices.Contains(claims, nodeID) {
			return "", opRefusal("id="+nodeID, "is not in "+rel+"'s own claims: list, so the reader would harvest the "+
				"marker and drop it — markers are intersected with the document's claim membership",
				"restore: add "+nodeID+" to "+rel+"'s claims: with set-frontmatter, then re-run")
		}
		at, err := locateExcerpt(document, locator)
		if err != nil {
			return "", err
		}
		line := kb.SplitLines(document)[at]
		content := strings.TrimRight(line, " \t")
		if strings.Contains(content, marker) {
			return document, nil
		}
		return spliceLines(document, at, at+1, []string{content + " " + marker + line[len(content):]}), nil
	}
}

func markerProver(nodeID, rel string) func(string) ([]string, error) {
	return func(candidate string) ([]string, error) {
		leaf, err := kb.ParseLeaf(candidate, rel)
		if err != nil {
			return nil, err
		}
		if leaf == nil || !leaf.Tier2Marked[nodeID] {
			return []string{"claim-rigor marker"}, nil
		}
		return nil, nil
	}
}

// declarationPairs is one node declaration's pair lists as the leaf reader
// returns them.
type declarationPairs struct {
	ID                       string
	Rigor                    *float64
	Supports, Lifts, Refutes []pair
}

// lists is the declaration's pair lists in field order.
func (d declarationPairs) lists() []namedPairs {
	return []namedPairs{{kb.SupportsKey, d.Supports}, {kb.LiftsKey, d.Lifts}, {kb.RefutesKey, d.Refutes}}
}

// namedPairs is one pair list under its frontmatter key.
type namedPairs struct {
	key   string
	pairs []pair
}

// observedDeclaration is nodeID's declaration in text as the production leaf
// readers return it, or nil where the document declares no such node.
func observedDeclaration(text, nodeID, rel string) (*declarationPairs, error) {
	if strings.HasPrefix(nodeID, "ana-") {
		nodes, err := kb.ParseAnalysisLeaf(text, rel)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			if n.ID == nodeID {
				return &declarationPairs{ID: n.ID, Rigor: n.Rigor, Lifts: pairsOf(n.Lifts), Refutes: pairsOf(n.Refutes)}, nil
			}
		}
		return nil, nil
	}
	nodes, err := kb.ParseExperimentLeaf(text, rel)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.ID == nodeID {
			return &declarationPairs{ID: n.ID, Rigor: n.Rigor, Supports: pairsOf(n.Supports), Lifts: pairsOf(n.Lifts), Refutes: pairsOf(n.Refutes)}, nil
		}
	}
	return nil, nil
}

// pairBearingProver reads a rewritten declaration back: the pair naming
// target under key must carry bearing.
func pairBearingProver(nodeID, key, target string, bearing kb.Bearing, rel string) func(string) ([]string, error) {
	return func(candidate string) ([]string, error) {
		decl, err := observedDeclaration(candidate, nodeID, rel)
		if err != nil {
			return nil, err
		}
		if decl == nil {
			return []string{"id"}, nil
		}
		for _, list := range decl.lists() {
			if list.key != key {
				continue
			}
			for _, p := range list.pairs {
				if p.ID == target {
					if !scoreEqual(p.Score, scoreOf(bearing)) {
						return []string{key}, nil
					}
					return nil, nil
				}
			}
		}
		return []string{key}, nil
	}
}

var claimPairRE = kb.PyRE(`^-\s*(` + kb.IDBody("clm") + `)\s*:`)

// replaceDeclarationPair rewrites the bearing of nodeID's pair naming target
// under key, in its hosting leaf's declaration. An absent pair is a splice
// failure: the op re-scores an edge, it does not create one.
func replaceDeclarationPair(nodeID, key, target string, score *float64) func(string) (string, error) {
	return func(document string) (string, error) {
		next, touched, err := editDeclarationPairs(document, nodeID, key, func(id string) (string, bool, bool) {
			if id != target {
				return "", false, false
			}
			line, err := kb.RenderFrontmatterField(id, scoreNode(score))
			if err != nil {
				return "", false, false
			}
			return line[0], true, false
		})
		if err != nil {
			return "", err
		}
		if !touched {
			return "", spliceFailed("%s declares no %s pair for %s", nodeID, key, target)
		}
		return next, nil
	}
}

// dropDeclarationPairs removes nodeID's pairs naming target under key from its
// hosting leaf's declaration, appending the paths it changed to *paths; a
// declaration with no such pair is left as it stands.
func dropDeclarationPairs(nodeID, key, target string) func(string) (string, error) {
	return func(document string) (string, error) {
		next, _, err := editDeclarationPairs(document, nodeID, key, func(id string) (string, bool, bool) {
			if id != target {
				return "", false, false
			}
			return "", true, true
		})
		return next, err
	}
}

// editDeclarationPairs rewrites nodeID's pair list under key in the document's
// frontmatter: edit returns a pair's replacement line, whether it touched it,
// and whether the pair goes. A list left with no pair loses its key line; a
// document declaring no such node or list is a splice failure.
func editDeclarationPairs(document, nodeID, key string, edit func(id string) (line string, touched, drop bool)) (string, bool, error) {
	nodeKey, listKey := kb.ExpIDKey, kb.ExperimentNodesKey
	if strings.HasPrefix(nodeID, "ana-") {
		nodeKey, listKey = kb.AnaIDKey, kb.AnalysisNodesKey
	}
	lines, start, end, ok := kb.FrontmatterKeySpan(document, listKey)
	if !ok {
		return "", false, spliceFailed("%s is not declared in this document's frontmatter", nodeID)
	}
	entry, indent := -1, ""
	for i := start; i < end; i++ {
		if strings.TrimPrefix(kb.Strip(lines[i]), "- ") == nodeKey+": "+nodeID {
			entry, indent = i, lines[i][:len(lines[i])-len(kb.LStrip(lines[i]))]
			break
		}
	}
	if entry < 0 {
		return "", false, spliceFailed("%s is not declared in this document's frontmatter", nodeID)
	}
	next := end
	for i := entry + 1; i < end; i++ {
		if strings.HasPrefix(lines[i], indent+"- ") {
			next = i
			break
		}
	}
	head, keyIndent := -1, ""
	for i := entry + 1; i < next; i++ {
		if strings.TrimPrefix(kb.Strip(lines[i]), "- ") == key+":" {
			head, keyIndent = i, lines[i][:len(lines[i])-len(kb.LStrip(lines[i]))]
			break
		}
	}
	if head < 0 {
		return "", false, spliceFailed("%s declares no %s list in this document", nodeID, key)
	}
	blockEnd := head + 1
	for blockEnd < next {
		line := lines[blockEnd]
		if kb.Strip(line) != "" && len(line)-len(kb.LStrip(line)) <= len(keyIndent) {
			break
		}
		blockEnd++
	}
	var body []string
	kept, touched := 0, false
	for _, line := range lines[head+1 : blockEnd] {
		m := claimPairRE.FindStringSubmatch(kb.Strip(line))
		if m == nil {
			body = append(body, line)
			continue
		}
		replacement, t, drop := edit(m[1])
		if !t {
			body = append(body, line)
			kept++
			continue
		}
		touched = true
		if drop {
			continue
		}
		body = append(body, line[:len(line)-len(kb.LStrip(line))]+"- "+replacement)
		kept++
	}
	if !touched {
		return document, false, nil
	}
	out := kb.EditFrontmatterLines(document, func(all []string) []string {
		if kept == 0 {
			return append(slices.Clone(all[:head]), all[blockEnd:]...)
		}
		return append(append(slices.Clone(all[:head+1]), body...), all[blockEnd:]...)
	})
	return out, true, nil
}

// editStagedPair rewrites a register-staged pair list under relation: edit
// returns a pair's replacement line, whether it touched it, and whether the
// pair goes. A list left with no pair loses its header; a register entry
// staging no such list is a splice failure.
func editStagedPair(nodeID, relation string, edit func(id string) (line string, touched, drop bool)) func(string) (string, error) {
	return func(document string) (string, error) {
		loc, err := locate(document, nodeID)
		if err != nil {
			return "", err
		}
		lines := kb.SplitLines(document)
		limit := min(loc.RigorEnd, len(lines))
		head := -1
		for i := loc.RigorStart; i < limit; i++ {
			if kb.Strip(lines[i]) == "- "+relation+":" {
				head = i
				break
			}
		}
		if head < 0 {
			return document, nil
		}
		breakRE := foldBreakFor(nodeID)
		end := head + 1
		for end < limit && !breakRE.MatchString(kb.Strip(lines[end])) && kb.Strip(lines[end]) != "---" && !atHeading(kb.Strip(lines[end])) {
			end++
		}
		for end > head+1 && kb.Strip(lines[end-1]) == "" {
			end--
		}
		var body []string
		kept, touched := 0, false
		for _, line := range lines[head+1 : end] {
			p, ok, err := kb.ParsePair(line, nodeID, relation)
			if err != nil {
				return "", err
			}
			if ok {
				replacement, t, drop := edit(p.Target)
				if t {
					touched = true
					if drop {
						continue
					}
					body = append(body, replacement)
					kept++
					continue
				}
			}
			body = append(body, line)
			if ok {
				kept++
			}
		}
		if !touched {
			return document, nil
		}
		if kept == 0 {
			return spliceLines(document, head, end, nil), nil
		}
		return spliceLines(document, head+1, end, body), nil
	}
}

// replaceDeclarationRigor rewrites one experiment's rigor in its host leaf's
// experiment-nodes mapping, placing the key after status where the mapping
// carries none.
func replaceDeclarationRigor(nodeID string, rigor *float64) func(string) (string, error) {
	return func(document string) (string, error) {
		lines, start, end, ok := kb.FrontmatterKeySpan(document, kb.ExperimentNodesKey)
		if !ok {
			return "", spliceFailed("%s is not declared in this document's frontmatter", nodeID)
		}
		entry, indent := -1, ""
		for i := start; i < end; i++ {
			if strings.TrimPrefix(kb.Strip(lines[i]), "- ") == kb.ExpIDKey+": "+nodeID {
				entry, indent = i, lines[i][:len(lines[i])-len(kb.LStrip(lines[i]))]
				break
			}
		}
		if entry < 0 {
			return "", spliceFailed("%s is not declared in this document's frontmatter", nodeID)
		}
		next := end
		for i := entry + 1; i < end; i++ {
			if strings.HasPrefix(lines[i], indent+"- ") {
				next = i
				break
			}
		}
		fieldIndent := indent + "  "
		rendered, err := kb.RenderFrontmatterField(kb.RigorKey, scoreNode(rigor))
		if err != nil {
			return "", err
		}
		for i := entry + 1; i < next; i++ {
			key, _, ok := strings.Cut(kb.Strip(lines[i]), ":")
			if ok && key == kb.RigorKey {
				return kb.EditFrontmatterLines(document, func(body []string) []string {
					body[i] = fieldIndent + rendered[0]
					return body
				}), nil
			}
		}
		for i := entry + 1; i < next; i++ {
			key, _, ok := strings.Cut(kb.Strip(lines[i]), ":")
			if ok && key == kb.StatusKey {
				return kb.EditFrontmatterLines(document, func(body []string) []string {
					return slices.Insert(body, i+1, fieldIndent+rendered[0])
				}), nil
			}
		}
		return "", spliceFailed("%s declares no status line to place its rigor beside", nodeID)
	}
}

// declarationRigorProver reads a rewritten experiment declaration back: its
// rigor must be the value supplied.
func declarationRigorProver(nodeID string, rigor *float64, rel string) func(string) ([]string, error) {
	return func(candidate string) ([]string, error) {
		nodes, err := kb.ParseExperimentLeaf(candidate, rel)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			if n.ID != nodeID {
				continue
			}
			if !scoreEqual(n.Rigor, rigor) {
				return []string{"rigor"}, nil
			}
			return nil, nil
		}
		return []string{"exp-id"}, nil
	}
}
