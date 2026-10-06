package write

import (
	"errors"
	"fmt"
	"os"
	"regexp"
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

// AddBuildEdges is the build's edge write: add-depends-on's lists and the
// demoted list. Only the build issues it, so it is no subcommand.
const AddBuildEdges = "add-build-edges"

// ResolveDemoted removes a demoted edge or restores it to depends-on.
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
	AddBuildEdges:             {plan: planAddDependsOn, buildOnly: true},
	ResolveDemoted:            {plan: planResolveDemoted},
	"insert-claim-entry":      {plan: planInserts("clm"), creates: true, inserts: true},
	"insert-support-entry":    {plan: planInserts("sup"), creates: true, inserts: true},
	"insert-experiment-entry": {plan: planExperiments, inserts: true},
	"insert-work-entry":       {plan: planWorkInserts, creates: true, inserts: true},
	"set-work-strength":       {plan: planFieldUpdate("strength", func(string) string { return "strength" })},
	"set-applicability":       {plan: planSetApplicability},
	"set-rigor":               {plan: planFieldUpdate("rigor", rigorField)},
	"set-rationale":           {plan: planFieldUpdate("rationale", func(string) string { return "rationale" })},
	"add-depends-on":          {plan: planAddDependsOn},
	"set-frontmatter":         {plan: planSetFrontmatter},
	"mark-claim-in-leaf":      {plan: planMarkClaim},
	"set-on-point-fraction":   {plan: planSetOnPointFraction},
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
// changed.
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
	if op == RenderCitation {
		return renderCitations(src, entries)
	}
	spec, ok := writeOps[op]
	if !ok {
		return failedResult("unknown op " + op)
	}
	if opts.NoRefresh && src.Migrated() {
		return refusedResult(result.Item{Check: kbload.CheckFormat, Path: src.KBPath(kb.EntryPointFile), Remedy: index.RefreshRemedy,
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
	switch {
	case errors.As(err, &refused):
		return Result{Outcome: result.Refused, Refusals: refused}
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
	ids      []string
	minted   []string
	adopted  []Adoption
	resolved []Resolution
	titles   map[string]string
	read     map[string]bool
}

// openStore reads the authored-id inventory, refusing a KB that keys one id
// from two register entries: the inventory keeps the first and would report
// the collision as clean.
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
	return &opContext{src: src, root: src.Root(), known: known, titles: map[string]string{}, read: map[string]bool{}}, nil
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
		r := readRegister(text)
		for id, e := range r.claims {
			c.titles[id] = e.Title
		}
		for id, s := range r.supports {
			c.titles[id] = s.Title
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

// resolve is an id-valued field's reference closure. A clm-, sup- or work-
// id must have its register entry; an exp- id lives in its hosting document.
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

// dependsTargets resolves requested edges into bullet values; a framework
// token is passed through, its declaration being the framework source.
func (c *opContext) dependsTargets(values []dependsOnValue) ([]dependsOnTarget, error) {
	var out []dependsOnTarget
	for _, v := range values {
		if !isClaimID(v.ID) && !isWorkID(v.ID) {
			out = append(out, dependsOnTarget{Target: v.ID, Context: v.Context})
			continue
		}
		if _, err := c.resolve(v.ID, "depends-on.id", true); err != nil {
			return nil, err
		}
		title, err := c.titleOf(v.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, dependsOnTarget{Target: v.ID, Title: title, Context: v.Context, Applicability: v.Applicability, Origin: v.Origin})
	}
	return out, nil
}

// intent is one entry's change to one file. expect is stated outright;
// expectCurrent is derived from the baseline the splice receives; prove
// reads a document write back.
type intent struct {
	path, target                        string
	splice                              func(string) (string, error)
	expect                              []expectedEntry
	expectCurrent                       func(registerRead) ([]expectedEntry, error)
	prove                               func(string) ([]string, error)
	subject                             string
	claimDelta, supportDelta, workDelta int
	create                              bool
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
			e.supportDelta += in.supportDelta
			e.workDelta += in.workDelta
			e.create = e.create || in.create
		}
		e.splice = func(document string) (string, error) {
			read := readRegister(document)
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

// rigorField is the on-disk name of a node's local rigor, read off its kind.
func rigorField(nodeID string) string {
	if strings.HasPrefix(nodeID, "sup-") {
		return "quality"
	}
	return "confidence"
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
			depends, err := c.dependsTargets(e.dependsOn("depends-on"))
			if err != nil {
				return nil, err
			}
			supports := e.pairs("supports")
			for _, p := range supports {
				if _, err := c.resolve(p.ID, "supports.id", true); err != nil {
					return nil, err
				}
			}
			want := expectedEntry{Title: e.str("title"), Rigor: e.score("rigor"), Rationale: e.str("rationale"), Supports: supports}
			for _, t := range depends {
				want.DependsOn = append(want.DependsOn, expectedEdge{Target: t.Target, Context: t.Context, Applicability: t.Applicability})
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
			rendered := registerEntry{NodeID: nodeID, Title: want.Title, Score: want.Rigor, Rationale: want.Rationale,
				DependsOn: depends, NoEdge: e.str("no-edge"), ScoreField: rigorField(nodeID), StrengthenBy: want.StrengthenBy, Supports: supports}
			in := intent{path: rel, target: target, splice: insertAll(renderEntry(rendered)), expect: []expectedEntry{want}, create: c.create}
			if kind == "clm" {
				in.claimDelta = 1
			} else {
				in.supportDelta = 1
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
	read := readRegister(text)
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
		strengthens := e.pairs("strengthens")
		for _, p := range strengthens {
			if _, err := c.resolve(p.ID, "strengthens.id", true); err != nil {
				return nil, err
			}
		}
		status := e.str("status")
		if adopted := adoptExperiment(c.src, target, rel, status, strengthens); adopted != "" {
			c.adopt(e.Index, adopted, nil)
			continue
		}
		expID := c.mint("exp")
		c.ids = append(c.ids, expID)
		decl := experimentDecl{ExpID: expID, Status: status, Strengthens: strengthens}
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
// status and these strengthens pairs, or "".
func adoptExperiment(src *kb.Source, target, rel, status string, strengthens []pair) string {
	text, err := src.ReadText(target)
	if err != nil {
		return ""
	}
	nodes, err := kb.ParseExperimentLeaf(text, rel)
	if err != nil {
		return ""
	}
	for _, n := range nodes {
		if n.Status == status && strengthensEqual(n.Strengthens, strengthens) {
			return n.ID
		}
	}
	return ""
}

func strengthensEqual(got []kb.StrengthensPair, want []pair) bool {
	return slices.EqualFunc(got, want, func(g kb.StrengthensPair, w pair) bool { return g.ClaimID == w.ID && g.Strength == *w.Score })
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
			var wrong []string
			if n.Status != decl.Status {
				wrong = append(wrong, "status")
			}
			if !strengthensEqual(n.Strengthens, decl.Strengthens) {
				wrong = append(wrong, "strengthens")
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
		want := expectedEntry{NodeID: nodeID, Title: e.str("title"), Rigor: e.score("strength"), Rationale: e.str("rationale")}
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
			got, _ := readRegister(text).record(nodeID)
			differs := mismatchedFields(want, got)
			if i := slices.Index(differs, "rigor"); i >= 0 {
				differs[i] = "strength"
			}
			c.adopt(e.Index, nodeID, differs)
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

func planSetApplicability(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		nodeID, workID, fraction := e.str("id"), e.str("work"), e.score("applicability")
		rec, err := c.resolve(nodeID, "id", true)
		if err != nil {
			return nil, err
		}
		if _, err := c.resolve(workID, "work", true); err != nil {
			return nil, err
		}
		register, err := c.contained(rec.RegisterPath, "id")
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent{path: rec.RegisterPath, target: register,
			splice: replaceApplicability(nodeID, workID, fraction),
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, nodeID, rec.RegisterPath)
				if err != nil {
					return nil, err
				}
				matched := false
				for i := range cur.DependsOn {
					if cur.DependsOn[i].Target == workID {
						cur.DependsOn[i].Applicability, matched = fraction, true
					}
				}
				if !matched {
					return nil, opRefusal("work="+workID, "is not a dependency of "+nodeID+", so there is no bullet "+
						"carrying an applicability to rewrite. This op re-scores a pairing; it does not create one",
						"restore: add the edge with add-depends-on, or correct work, then re-run")
				}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// replaceApplicability rewrites the annotation on every bullet of nodeID
// naming workID; rewriting it to the value it carries is a no-op.
func replaceApplicability(nodeID, workID string, fraction *float64) func(string) (string, error) {
	annotation := "applicability " + formatScore(fraction)
	return func(document string) (string, error) {
		loc, err := locate(document, nodeID)
		if err != nil {
			return "", err
		}
		lines := kb.SplitLines(document)
		for i := loc.QualityStart; i < min(loc.QualityEnd, len(lines)); i++ {
			if kb.BulletHead(kb.TrimBulletLead(kb.Strip(lines[i]))) != workID {
				continue
			}
			m := kb.ApplicabilityRE.FindStringIndex(lines[i])
			if m == nil {
				return "", spliceFailed("%s's bullet for %s carries no (applicability …) annotation", nodeID, workID)
			}
			replaced := lines[i][:m[0]] + annotation + lines[i][m[1]:]
			document = spliceLines(document, i, i+1, []string{replaced})
			lines = kb.SplitLines(document)
		}
		return document, nil
	}
}

// edgeAddition is one entry's requested edges of one list and the subset it
// does not already hold, decided once from the baseline.
type edgeAddition struct {
	nodeID    string
	requested []dependsOnTarget
	added     []dependsOnTarget
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
		edge := readerTarget(a.nodeID, t)
		if held[edge] {
			continue
		}
		held[edge] = true
		a.added = append(a.added, t)
	}
}

// readerTarget is the id the reader keys this edge under once it is a
// bullet: the rendered bullet put through the reader that will read it.
func readerTarget(nodeID string, t dependsOnTarget) string {
	if edges := kb.ParseDependsOnBullet(renderDependsOnBullet(t), nodeID); len(edges) > 0 {
		return edges[0].Target
	}
	return t.Target
}

var (
	bulletRenderers = map[string]func(dependsOnTarget) string{
		"depends-on":       renderDependsOnBullet,
		"references":       renderReferencesBullet,
		kb.RelationDemoted: renderDemotedBullet,
	}
	// sectionAnchors are the lines a missing section is placed above, the
	// first present winning, so the reader's field order survives any order
	// of writing.
	sectionAnchors = map[string][]string{
		"depends-on":       {"- references:", "- " + kb.RelationDemoted + ":", "- solidity:"},
		"references":       {"- " + kb.RelationDemoted + ":", "- solidity:"},
		kb.RelationDemoted: {"- solidity:"},
	}
	// edgeSections is each list an entry's edges are added to, in field
	// order, with the edges of the entry's record it holds.
	edgeSections = []struct {
		name string
		held func(*expectedEntry) *[]expectedEdge
	}{
		{"depends-on", func(e *expectedEntry) *[]expectedEdge { return &e.DependsOn }},
		{"references", func(e *expectedEntry) *[]expectedEdge { return &e.References }},
		{kb.RelationDemoted, func(e *expectedEntry) *[]expectedEdge { return &e.Demoted }},
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
		limit := min(loc.QualityEnd, len(lines))
		first := func(prefix string) int {
			for i := loc.QualityStart; i < limit; i++ {
				if strings.HasPrefix(kb.Strip(lines[i]), prefix) {
					return i
				}
			}
			return -1
		}
		if head := first(header); head >= 0 {
			i := head + 1
			for i < limit && kb.Strip(lines[i]) != "" && !breakRE.MatchString(kb.Strip(lines[i])) {
				i++
			}
			return spliceLines(document, i, i, bullets), nil
		}
		for _, anchor := range sectionAnchors[section] {
			if at := first(anchor); at >= 0 {
				return spliceLines(document, at, at, append([]string{header}, bullets...)), nil
			}
		}
		return "", spliceFailed("%s has no %s line to anchor a %s section above", a.nodeID, strings.Join(sectionAnchors[section], " or "), section)
	}
}

func planAddDependsOn(c *opContext, entries []entry) ([]intent, error) {
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
			targets, err := c.dependsTargets(e.dependsOn(s.name))
			if err != nil {
				return nil, err
			}
			if len(targets) == 0 {
				continue
			}
			if s.name != "depends-on" && !isClaimID(nodeID) {
				return nil, opRefusal(nodeID, "carries a "+s.name+" list, and a reference is one claim of this corpus naming another",
					"restore: drop the "+s.name+" list, or name a claim entry as the id, then re-run")
			}
			sections = append(sections, i)
			additions[i] = &edgeAddition{nodeID: nodeID, requested: targets, added: targets}
		}
		if len(sections) == 0 {
			return nil, opRefusal(nodeID, "this op adds an entry's outgoing edges and neither list names one",
				"restore: supply a depends-on list, a references list, or both, then re-run")
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
						present[edge.Target] = true
					}
					a.narrow(present)
					for _, t := range a.added {
						*held = append(*held, expectedEdge{Target: t.Target, Context: t.Context, Applicability: t.Applicability, Origin: t.Origin})
					}
				}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// demotedResolution is one entry's changes to its demoted list: the targets
// whose demoted bullet goes, and of them the ones gaining a depends-on bullet.
type demotedResolution struct {
	register, target string
	drop             map[string]bool
	restore          []dependsOnTarget
}

// planResolveDemoted removes each named demoted edge, or rewrites it as a
// depends-on bullet where that closes no cycle of the authored premise graph,
// the batch's earlier restores included. A pair already as asked — no
// demoted edge to remove, or a depends edge and no demoted one to restore —
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
		depends := slices.ContainsFunc(cur.DependsOn, func(d kb.Edge) bool { return d.Relation == kb.RelationDepends && d.Target == target })
		r := byNode[nodeID]
		if demoted < 0 {
			if action == actionRemove || depends {
				continue
			}
			return nil, opRefusal("target="+target, fmt.Sprintf("%s carries no demoted edge to %s, so there is nothing to restore; "+
				"only the build writes a demoted edge", nodeID, target), "restore: add the edge with kbase add-depends-on, or correct target, then re-run")
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
					Remedy: "kbase resolve-demoted with action remove for this pair, or remove a depends edge on the named path first",
					Detail: fmt.Sprintf("restoring %s → %s as a depends edge would close the cycle %s", nodeID, target,
						strings.Join(append(path, target), " → "))}}
			}
			graph.Add(nodeID, target)
			done = resolvedRestore
			if !depends {
				d := cur.Demoted[demoted]
				t := dependsOnTarget{Target: target}
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
		splices := []func(string) (string, error){dropDemoted(nodeID, r.drop)}
		if len(r.restore) > 0 {
			splices = append(splices, addBullets(&edgeAddition{nodeID: nodeID, requested: r.restore, added: r.restore}, "depends-on"))
		}
		intents = append(intents, intent{path: r.register, target: r.target,
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
				cur, err := currentEntry(read, nodeID, r.register)
				if err != nil {
					return nil, err
				}
				cur.Demoted = slices.DeleteFunc(cur.Demoted, func(d expectedEdge) bool { return r.drop[d.Target] })
				for _, t := range r.restore {
					cur.DependsOn = append(cur.DependsOn, expectedEdge{Target: t.Target, Context: t.Context})
				}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// dropDemoted deletes the demoted bullets of nodeID's entry whose target is
// in targets, each with its continuation lines, and the list's header where
// no bullet is left.
func dropDemoted(nodeID string, targets map[string]bool) func(string) (string, error) {
	header := "- " + kb.RelationDemoted + ":"
	breakRE := foldBreakFor(nodeID)
	return func(document string) (string, error) {
		loc, err := locate(document, nodeID)
		if err != nil {
			return "", err
		}
		lines := kb.SplitLines(document)
		limit := min(loc.QualityEnd, len(lines))
		head := slices.IndexFunc(lines[loc.QualityStart:limit], func(l string) bool { return strings.HasPrefix(kb.Strip(l), header) })
		if head < 0 {
			return "", spliceFailed("%s has no %s list", nodeID, header)
		}
		head += loc.QualityStart
		end := head + 1
		for end < limit && !breakRE.MatchString(kb.Strip(lines[end])) && kb.Strip(lines[end]) != "---" {
			end++
		}
		for end > head+1 && kb.Strip(lines[end-1]) == "" {
			end--
		}
		var kept []string
		bullets, dropping := 0, false
		for _, l := range lines[head+1 : end] {
			if l != kb.LStrip(l) && strings.HasPrefix(kb.LStrip(l), "- ") {
				dropping = slices.ContainsFunc(kb.ClaimIDs(kb.BulletHead(kb.Strip(kb.TrimBulletLead(l)))), func(id string) bool { return targets[id] })
				if !dropping {
					bullets++
				}
			}
			if !dropping {
				kept = append(kept, l)
			}
		}
		if bullets == 0 {
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
		kb.SupportNodesKey, kb.FormatKey}
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
		expNodes, supNodes := e.experimentNodes(), e.supportNodes()
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
			for _, p := range n.Strengthens {
				if _, err := c.resolve(p.ID, "experiment-node.strengthens.id", true); err != nil {
					return nil, err
				}
			}
			redeclared = append(redeclared, n.ExpID)
		}
		for _, n := range supNodes {
			if _, err := c.resolve(n.SupID, "support-node.sup-id", true); err != nil {
				return nil, err
			}
			for _, p := range n.Supports {
				if _, err := c.resolve(p.ID, "support-node.supports.id", true); err != nil {
					return nil, err
				}
			}
			redeclared = append(redeclared, n.SupID)
		}
		intended := frontmatterValues{Kind: e.str("kind"), Claims: claims, Experiments: experiments, ExperimentNodes: expNodes, SupportNodes: supNodes}
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
		var pairs []map[string]any
		for _, p := range n.Strengthens {
			pairs = append(pairs, map[string]any{"id": p.ID, "strength": score(p.Score)})
		}
		if len(pairs) > 0 {
			e["strengthens"] = pairs
		}
		exps = append(exps, e)
	}
	if len(exps) > 0 {
		out["experiment-node"] = exps
	}
	var sups []map[string]any
	for _, n := range v.SupportNodes {
		e := map[string]any{"sup-id": n.SupID}
		var pairs []map[string]any
		for _, p := range n.Supports {
			pairs = append(pairs, map[string]any{"id": p.ID, "fraction": score(p.Score)})
		}
		if len(pairs) > 0 {
			e["supports"] = pairs
		}
		sups = append(sups, e)
	}
	if len(sups) > 0 {
		out["support-node"] = sups
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
		d := experimentDecl{ExpID: n.ID, Status: n.Status}
		for _, p := range n.Strengthens {
			s := p.Strength
			d.Strengthens = append(d.Strengthens, pair{ID: p.ClaimID, Score: &s})
		}
		v.ExperimentNodes = append(v.ExperimentNodes, d)
	}
	sups, err := kb.ParseSupportLeaf(text, rel)
	if err != nil {
		return nil, err
	}
	for _, n := range sups {
		d := supportDecl{SupID: n.ID}
		for _, p := range n.Supports {
			d.Supports = append(d.Supports, pair{ID: p.ClaimID, Score: fractionScore(p.Fraction)})
		}
		v.SupportNodes = append(v.SupportNodes, d)
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
				return a.ExpID == b.ExpID && a.Status == b.Status && pairsEqual(a.Strengthens, b.Strengthens)
			})},
			{"support-node", slices.EqualFunc(got.SupportNodes, want.SupportNodes, func(a, b supportDecl) bool {
				return a.SupID == b.SupID && pairsEqual(a.Supports, b.Supports)
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
			return []string{"claim-quality marker"}, nil
		}
		return nil, nil
	}
}

func planSetOnPointFraction(c *opContext, entries []entry) ([]intent, error) {
	var intents []intent
	for _, e := range entries {
		supID, claimID, fraction := e.str("id"), e.str("claim"), e.score("fraction")
		rec, err := c.resolve(supID, "id", true)
		if err != nil {
			return nil, err
		}
		if _, err := c.resolve(claimID, "claim", true); err != nil {
			return nil, err
		}
		line := renderSupportsPairLine(claimID, fraction)
		if rec.HostingLeaf != "" {
			target, err := c.contained(rec.HostingLeaf, "id")
			if err != nil {
				return nil, err
			}
			intents = append(intents, intent{path: rec.HostingLeaf, target: target,
				splice: replaceSupportsPair(supID, claimID, fraction),
				prove:  fractionProver(supID, claimID, fraction, rec.HostingLeaf), subject: rec.HostingLeaf + ":" + supID})
			continue
		}
		register, err := c.contained(rec.RegisterPath, "id")
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent{path: rec.RegisterPath, target: register,
			splice: replaceStagedPair(supID, claimID, line),
			expectCurrent: func(read registerRead) ([]expectedEntry, error) {
				cur, err := currentEntry(read, supID, rec.RegisterPath)
				if err != nil {
					return nil, err
				}
				at := slices.IndexFunc(cur.Supports, func(p pair) bool { return p.ID == claimID })
				if at < 0 {
					return nil, opRefusal("id="+supID, "authors no supports pair for "+claimID+" in either home — it is "+
						"declared in no document's frontmatter, and its register entry stages no such pair. This op re-scores "+
						"an edge that exists; it does not create one",
						"restore: author the pair — with insert-support-entry's supports value if "+supID+" is being created, "+
							"or with set-frontmatter once its hosting document exists — then re-run")
				}
				cur.Supports = slices.Clone(cur.Supports)
				cur.Supports[at] = pair{ID: claimID, Score: fraction}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// replaceSupportsPair rewrites one supports pair line inside its own entry
// of the document's support-nodes list; every other line is kept.
func replaceSupportsPair(supID, claimID string, fraction *float64) func(string) (string, error) {
	pairRE := kb.PyRE(`^(\s*-\s*)` + regexp.QuoteMeta(claimID) + `\s*:`)
	return func(document string) (string, error) {
		lines, start, end, ok := kb.FrontmatterKeySpan(document, kb.SupportNodesKey)
		if !ok {
			return "", spliceFailed("%s is not declared in this document's frontmatter", supID)
		}
		entry := slices.IndexFunc(lines[start:end], func(l string) bool {
			return strings.TrimPrefix(kb.Strip(l), "- ") == kb.SupIDKey+": "+supID
		})
		if entry < 0 {
			return "", spliceFailed("%s is not declared in this document's frontmatter", supID)
		}
		entry += start
		itemIndent := lines[entry][:len(lines[entry])-len(kb.LStrip(lines[entry]))]
		next := end
		for i := entry + 1; i < end; i++ {
			if strings.HasPrefix(lines[i], itemIndent+"- ") {
				next = i
				break
			}
		}
		pairLine, err := kb.RenderFrontmatterField(claimID, scoreNode(fraction))
		if err != nil {
			return "", err
		}
		for i := entry + 1; i < next; i++ {
			if m := pairRE.FindStringSubmatch(lines[i]); m != nil {
				replaced := m[1] + pairLine[0]
				return kb.EditFrontmatterLines(document, func(body []string) []string {
					body[i] = replaced
					return body
				}), nil
			}
		}
		return "", spliceFailed("%s declares no supports pair for %s", supID, claimID)
	}
}

// replaceStagedPair rewrites one staged supports pair inside the support's
// register entry; where the block ends is the reader's pair pattern's call.
func replaceStagedPair(supID, claimID, line string) func(string) (string, error) {
	return func(document string) (string, error) {
		loc, err := locate(document, supID)
		if err != nil {
			return "", err
		}
		lines := kb.SplitLines(document)
		limit := min(loc.QualityEnd, len(lines))
		head := -1
		for i := loc.QualityStart; i < limit; i++ {
			if kb.Strip(lines[i]) == "- supports:" {
				head = i
				break
			}
		}
		if head < 0 {
			return "", spliceFailed("%s stages no supports block in this register", supID)
		}
		for i := head + 1; i < limit; i++ {
			if p, ok := kb.ParseSupportPair(lines[i]); ok {
				if p.ClaimID == claimID {
					return spliceLines(document, i, i+1, []string{line}), nil
				}
				continue
			}
			if strings.HasPrefix(kb.Strip(lines[i]), "- ") {
				break
			}
		}
		return "", spliceFailed("%s stages no supports pair for %s", supID, claimID)
	}
}

func fractionProver(supID, claimID string, fraction *float64, rel string) func(string) ([]string, error) {
	return func(candidate string) ([]string, error) {
		nodes, err := kb.ParseSupportLeaf(candidate, rel)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			if n.ID != supID {
				continue
			}
			var got *kb.SupportPair
			for i := range n.Supports {
				if n.Supports[i].ClaimID == claimID {
					got = &n.Supports[i]
				}
			}
			if got == nil || !scoreEqual(fractionScore(got.Fraction), fraction) || (fraction == nil) != got.Fraction.Pending {
				return []string{"fraction"}, nil
			}
			return nil, nil
		}
		return []string{"sup-id"}, nil
	}
}
