package write

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
)

// RenderCitation is the one op that reads instead of writing.
const RenderCitation = "render-citation"

// opSpec is one write op: its planner, and whether it may create a register.
type opSpec struct {
	plan    func(*opContext, []entry) ([]intent, error)
	creates bool
	inserts bool
}

var writeOps = map[string]opSpec{
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

// Ops is every write op's name, sorted.
func Ops() []string {
	names := make([]string, 0, len(writeOps))
	for name := range writeOps {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// CreatesRegister is whether op takes --create.
func CreatesRegister(op string) bool { return writeOps[op].creates }

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
// there. Refreshed is what the trailing refresh wrote, nil where none ran.
type Result struct {
	Outcome   string
	Written   []string
	IDs       []string
	Minted    []string
	Adopted   []Adoption
	Refreshed []string
	Citations []string
	Refusals  []result.Item
	Failures  []result.Item
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
type opRefusal struct{ name, detail, restore string }

func (e *opRefusal) Error() string { return e.name + ": " + e.detail }

func refusedResult(r result.Item) Result {
	return Result{Outcome: result.Refused, Refusals: []result.Item{r}}
}

func (e *opRefusal) refusal() result.Item {
	return result.Item{Key: e.name, Remedy: remedy(e.restore), Detail: e.detail}
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

// Run executes one op over the values in opts.
func Run(op string, opts Options) Result {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	if info, err := os.Stat(opts.KBRoot); err != nil || !info.IsDir() {
		return failedResult(opts.KBRoot + " is not a directory, so no KB can be resolved under it")
	}
	root := resolvePath(opts.KBRoot)
	entries, refusals := parseValues(opts.Values, op)
	if refusals != nil {
		return Result{Outcome: result.Refused, Refusals: refusals}
	}
	if op == RenderCitation {
		return renderCitations(root, entries)
	}
	spec, ok := writeOps[op]
	if !ok {
		return failedResult("unknown op " + op)
	}
	ctx, err := openStore(root)
	if err != nil {
		return errorResult(err)
	}
	ctx.create = opts.Create && spec.creates
	intents, err := spec.plan(ctx, entries)
	if err != nil {
		return errorResult(err)
	}
	out, err := applyEdits(root, batch(intents))
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
			Remedy: "re-run kbase " + op + " with these values unchanged — never re-author values that were already right",
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
	refreshed, err := index.Refresh(root, opts.Logger)
	slices.Sort(refreshed)
	res.Refreshed = append([]string{}, slices.Compact(refreshed)...)
	if err != nil {
		res.Outcome = result.Failed
		res.Failures = []result.Item{{Check: checkWrite, Detail: "the write landed and the trailing refresh did not: " + err.Error()}}
		return res
	}
	if len(res.Refreshed) > 0 {
		res.Outcome = result.Done
	}
	return res
}

func errorResult(err error) Result {
	var or *opRefusal
	var rb *readbackFailed
	var mf kb.MalformedError
	switch {
	case errors.As(err, &or):
		return refusedResult(or.refusal())
	case errors.As(err, &rb):
		return refusedResult(result.Item{Key: rb.subject, Remedy: "this is a renderer defect, not a value defect — report it; nothing on disk changed",
			Detail: "read back from the composed candidate with a different " + strings.Join(rb.mismatched, ", ") +
				" than the values supplied, so the live file was never written"})
	case errors.As(err, &mf):
		return refusedResult(result.Item{Key: "kb-frontmatter", Remedy: "repair the named document's frontmatter, then re-run", Detail: mf.Msg})
	}
	return failedResult(err.Error())
}

// opContext is one invocation's view of the authored store. create is the
// --create acknowledgement, on an op that admits it.
type opContext struct {
	root    string
	create  bool
	known   map[string]kb.IDRecord
	ids     []string
	minted  []string
	adopted []Adoption
	titles  map[string]string
	read    map[string]bool
}

// openStore reads the authored-id inventory, refusing a KB that keys one id
// from two register entries: the inventory keeps the first and would report
// the collision as clean.
func openStore(root string) (*opContext, error) {
	known, duplicates, err := kb.AuthoredIDs(root)
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
		return nil, &opRefusal{id, fmt.Sprintf("is keyed by %d canonical register entries (%s); every later reading "+
			"would be taken over the inventory that keeps only the first", len(duplicates[id]), strings.Join(duplicates[id], ", ")),
			fmt.Sprintf("restore: delete the duplicate '<!-- id: %s -->' entry from all but one register, then re-run this op unchanged", id)}
	}
	return &opContext{root: root, known: known, titles: map[string]string{}, read: map[string]bool{}}, nil
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
		text, err := kb.ReadText(filepath.Join(c.root, filepath.FromSlash(rec.RegisterPath)))
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
		return "", &opRefusal{key, fmt.Sprintf("%q %s. Paths are relative to kb-root/, not to the repository root", rel, se.detail),
			"restore: correct " + key + " to a kb-root-relative path inside the KB and re-run"}
	}
	return target, err
}

// resolve is an id-valued field's reference closure. A clm-, sup- or work-
// id must have its register entry; an exp- id lives in its hosting document.
func (c *opContext) resolve(nodeID, key string, needsRegister bool) (kb.IDRecord, error) {
	rec, ok := c.known[nodeID]
	if !ok {
		return rec, &opRefusal{key + "=" + nodeID, "does not resolve against the authored-id inventory. Every id-valued " +
			"field must name a node that already exists at write time",
			fmt.Sprintf("restore: create %s's node with the insert op for its kind, or correct %s, then re-run", nodeID, key)}
	}
	if needsRegister && rec.RegisterPath == "" {
		return rec, &opRefusal{key + "=" + nodeID, "is authored in document frontmatter but has no canonical register " +
			"entry, so it is not a node any consumer can read a title, a rigor or a rationale from",
			fmt.Sprintf("restore: insert %s's register entry, or correct %s, then re-run", nodeID, key)}
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
		out = append(out, dependsOnTarget{Target: v.ID, Title: title, Context: v.Context, Applicability: v.Applicability})
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
		return e, &opRefusal{"id=" + nodeID, "has no canonical entry in " + register + " that the production parser returns",
			"restore: correct id, or repair " + nodeID + "'s entry, then re-run"}
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
			if adopted, differs := adoptRegisterEntry(target, kind, want); adopted != "" {
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
func adoptRegisterEntry(target, kind string, want expectedEntry) (string, []string) {
	text, err := kb.ReadText(target)
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
		if adopted := adoptExperiment(target, rel, status, strengthens); adopted != "" {
			c.adopt(e.Index, adopted, nil)
			continue
		}
		expID := c.mint("exp")
		c.ids = append(c.ids, expID)
		decl := experimentDecl{ExpID: expID, Status: status, Strengthens: strengthens}
		block := kb.SplitLines(renderFrontmatterBlock(frontmatterValues{Kind: kb.DocumentLeaf, ExperimentNodes: []experimentDecl{decl}}))
		var added []string
		for _, line := range block[1 : len(block)-1] {
			if !strings.HasPrefix(line, "kind:") {
				added = append(added, line)
			}
		}
		intents = append(intents, intent{path: rel, target: target, splice: appendToBlock(added),
			prove: experimentProver(decl, rel), subject: rel + ":" + expID})
	}
	return intents, nil
}

// adoptExperiment is an experiment the document already hosts with this
// status and these strengthens pairs, or "".
func adoptExperiment(target, rel, status string, strengthens []pair) string {
	text, err := kb.ReadText(target)
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
			return nil, &opRefusal{"key=" + key, fmt.Sprintf("is named twice in this batch (%s). A work's id is derived from its "+
				"citation key, so a second entry would be a second node for one work", nodeID),
				"restore: drop one of the two entries, then re-run"}
		}
		if held, ok := c.known[nodeID]; ok && held.RegisterPath != "" {
			text, err := kb.ReadText(filepath.Join(c.root, filepath.FromSlash(held.RegisterPath)))
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
					return nil, &opRefusal{"work=" + workID, "is not a dependency of " + nodeID + ", so there is no bullet " +
						"carrying an applicability to rewrite. This op re-scores a pairing; it does not create one",
						"restore: add the edge with add-depends-on, or correct work, then re-run"}
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
		"depends-on": renderDependsOnBullet,
		"references": renderReferencesBullet,
	}
	// sectionAnchors are the lines a missing section is placed above, the
	// first present winning, so the reader's field order survives either
	// order of writing.
	sectionAnchors = map[string][]string{
		"depends-on": {"- references:", "- solidity:"},
		"references": {"- solidity:"},
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
		depends, err := c.dependsTargets(e.dependsOn("depends-on"))
		if err != nil {
			return nil, err
		}
		referenced, err := c.dependsTargets(e.dependsOn("references"))
		if err != nil {
			return nil, err
		}
		if len(depends) == 0 && len(referenced) == 0 {
			return nil, &opRefusal{nodeID, "this op adds an entry's outgoing edges and neither list names one",
				"restore: supply a depends-on list, a references list, or both, then re-run"}
		}
		if len(referenced) > 0 && !isClaimID(nodeID) {
			return nil, &opRefusal{nodeID, "carries a references list, and a reference is one claim of this corpus naming another",
				"restore: drop the references list, or name a claim entry as the id, then re-run"}
		}
		var sections []string
		additions := map[string]*edgeAddition{}
		for _, s := range []struct {
			name    string
			targets []dependsOnTarget
		}{{"depends-on", depends}, {"references", referenced}} {
			if len(s.targets) > 0 {
				sections = append(sections, s.name)
				additions[s.name] = &edgeAddition{nodeID: nodeID, requested: s.targets, added: s.targets}
			}
		}
		var splices []func(string) (string, error)
		for _, s := range sections {
			splices = append(splices, addBullets(additions[s], s))
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
				for _, s := range sections {
					a := additions[s]
					held := cur.DependsOn
					if s == "references" {
						held = cur.References
					}
					present := map[string]bool{}
					for _, edge := range held {
						present[edge.Target] = true
					}
					a.narrow(present)
					var added []expectedEdge
					for _, t := range a.added {
						added = append(added, expectedEdge{Target: t.Target, Context: t.Context, Applicability: t.Applicability})
					}
					if s == "references" {
						cur.References = append(cur.References, added...)
					} else {
						cur.DependsOn = append(cur.DependsOn, added...)
					}
				}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// knownFrontmatterKeys is every key a frontmatter block this API composes
// may carry; derivedFrontmatterFields are refresh's roll-ups, carried over
// verbatim at refresh's own anchors.
var (
	knownFrontmatterKeys = []string{"kind", "path-stable", "claims", "no-claim", "experiments", "exp-id", "status",
		"strengthens", "sup-id", "supports"}
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
			splice: replaceBlock(renderFrontmatterBlock(intended), rel, redeclared),
			prove:  frontmatterProver(intended, rel), subject: rel})
	}
	return intents, nil
}

// replaceBlock replaces a document's whole frontmatter block, or places a
// first one, carrying refresh's roll-ups over. What the replace would lose
// is read off the document the splice receives.
func replaceBlock(block, rel string, redeclared []string) func(string) (string, error) {
	return func(document string) (string, error) {
		existing, err := existingFrontmatter(document, rel, redeclared)
		if err != nil {
			return "", err
		}
		var candidate string
		if m := kb.FrontmatterRE.FindStringIndex(document); m == nil {
			candidate = insertBlock(document, block)
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
		return candidate, nil
	}
}

// insertBlock places a first frontmatter block below the up-link line, else
// at the top, adding a separating blank line only where there is none.
func insertBlock(document, block string) string {
	lines := kb.SplitLines(document)
	at := 0
	for i, line := range lines {
		if kb.Strip(line) == "" {
			continue
		}
		at = i
		if strings.HasPrefix(kb.LStrip(line), "[") {
			at = i + 1
		}
		break
	}
	added := kb.SplitLines(block)
	if at > 0 && kb.Strip(lines[at-1]) != "" {
		added = append([]string{""}, added...)
	}
	if at < len(lines) && kb.Strip(lines[at]) != "" {
		added = append(added, "")
	}
	return spliceLines(document, at, at, added)
}

// existingFrontmatter is the document's current fields, refusing a replace
// that would drop a key this API cannot render or a hosted node declaration
// the values do not restate.
func existingFrontmatter(text, rel string, redeclared []string) (kb.Frontmatter, error) {
	fields := kb.ParseFrontmatter(text)
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
		return nil, &opRefusal{rel, fmt.Sprintf("carries frontmatter key(s) %v that this API cannot render, so replacing the block would drop them", unknown),
			fmt.Sprintf("restore: remove %q from the block by hand, or leave this document's frontmatter alone, then re-run", unknown[0])}
	}
	m := kb.FindFrontmatter(text, 0)
	var dropped []string
	for _, id := range kb.DeclaredNodeIDs(text[m[2]:m[3]]) {
		if !slices.Contains(redeclared, id) && !slices.Contains(dropped, id) {
			dropped = append(dropped, id)
		}
	}
	slices.Sort(dropped)
	if len(dropped) > 0 {
		return nil, &opRefusal{rel, fmt.Sprintf("declares hosted node(s) %v that these values do not restate, and replacing the block would destroy the declaration rather than edit it", dropped),
			fmt.Sprintf("restore: restate %s's block in the values, or leave this document's frontmatter alone, then re-run", dropped[0])}
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
	fields := kb.ParseFrontmatter(text)
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
	var err error
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
			return []string{"kb-frontmatter"}, nil
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

// appendToBlock appends declaration lines inside the existing frontmatter
// block; the delimiters come back from wrapFrontmatter.
func appendToBlock(added []string) func(string) (string, error) {
	return func(document string) (string, error) {
		m := kb.FrontmatterRE.FindStringSubmatchIndex(document)
		if m == nil {
			return "", spliceFailed("the document has no kb-frontmatter block to declare a node in")
		}
		lines := append(kb.SplitLines(document[m[2]:m[3]]), added...)
		return document[:m[0]] + wrapFrontmatter(strings.Join(lines, "\n")) + document[m[1]:], nil
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
		if !kb.IsFile(target) {
			return nil, &opRefusal{rel, "does not exist. A marker is placed beside the prose it anchors, and this API never composes a document body",
				"restore: correct document, or author the document first, then re-run"}
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
		claims, err := kb.ParseFrontmatter(document).ListOrEmpty("claims")
		if err != nil {
			return "", kb.MalformedError{Msg: rel + ": claims: " + err.Error()}
		}
		if !slices.Contains(claims, nodeID) {
			return "", &opRefusal{"id=" + nodeID, "is not in " + rel + "'s own claims: list, so the reader would harvest the " +
				"marker and drop it — markers are intersected with the document's claim membership",
				"restore: add " + nodeID + " to " + rel + "'s claims: with set-frontmatter, then re-run"}
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
				splice: replaceSupportsPair(supID, claimID, line),
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
					return nil, &opRefusal{"id=" + supID, "authors no supports pair for " + claimID + " in either home — it is " +
						"declared in no document's frontmatter, and its register entry stages no such pair. This op re-scores " +
						"an edge that exists; it does not create one",
						"restore: author the pair — with insert-support-entry's supports value if " + supID + " is being created, " +
							"or with set-frontmatter once its hosting document exists — then re-run"}
				}
				cur.Supports = slices.Clone(cur.Supports)
				cur.Supports[at] = pair{ID: claimID, Score: fraction}
				return []expectedEntry{cur}, nil
			}})
	}
	return intents, nil
}

// replaceSupportsPair rewrites one supports pair line inside its own sup-id
// block of the document's frontmatter.
func replaceSupportsPair(supID, claimID, line string) func(string) (string, error) {
	pairRE := kb.PyRE(`^(\s*-?\s*)` + regexp.QuoteMeta(claimID) + `\s*:`)
	return func(document string) (string, error) {
		m := kb.FrontmatterRE.FindStringSubmatchIndex(document)
		if m == nil {
			return "", spliceFailed("the document has no kb-frontmatter block")
		}
		block := kb.SplitLines(document[m[2]:m[3]])
		start := slices.IndexFunc(block, func(l string) bool { return kb.Strip(l) == "sup-id: "+supID })
		if start < 0 {
			return "", spliceFailed("%s is not declared in this document's frontmatter", supID)
		}
		end := len(block)
		for i := start + 1; i < len(block); i++ {
			if strings.HasPrefix(kb.Strip(block[i]), "sup-id:") {
				end = i
				break
			}
		}
		for i := start + 1; i < end; i++ {
			if pairRE.MatchString(block[i]) {
				block[i] = line
				return document[:m[0]] + wrapFrontmatter(strings.Join(block, "\n")) + document[m[1]:], nil
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
