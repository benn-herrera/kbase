package write

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"kbase/internal/atomicfile"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
)

// status is what a batch of edits did.
type status int

const (
	statusWritten status = iota
	statusUnchanged
	statusRefused
	statusRetry
)

// Reasons a write did not happen, each raised at one point of the store's
// ladder.
const (
	reasonPathOutsideRoot  = "path-outside-root"
	reasonRegisterAbsent   = "register-absent"
	reasonDuplicateTarget  = "duplicate-target"
	reasonNotUTF8          = "not-utf8"
	reasonMalformed        = "malformed"
	reasonCensusMismatch   = "census-mismatch"
	reasonSpliceFailed     = "splice-failed"
	reasonReadbackMismatch = "readback-mismatch"
	reasonRecordCount      = "record-count"
	reasonContended        = "contended"
)

// storeError is a refusal raised anywhere in the store's ladder; contended
// marks the one that is a retry rather than a refusal.
type storeError struct {
	reason, subject, detail string
	contended               bool
}

func (e *storeError) Error() string { return e.subject + ": " + e.detail }

// spliceError is a splice that could not locate what it was asked to edit.
type spliceError struct{ msg string }

func (e spliceError) Error() string { return e.msg }

func spliceFailed(format string, args ...any) error { return spliceError{fmt.Sprintf(format, args...)} }

// interruptedError is a replace that failed after earlier targets of the
// batch were committed.
type interruptedError struct {
	written []string
	failed  string
	cause   error
}

func (e *interruptedError) Error() string {
	landed := "nothing"
	if len(e.written) > 0 {
		landed = strings.Join(e.written, ", ")
	}
	return fmt.Sprintf("%s could not be replaced (%v); this batch is interrupted, not refused — already committed: %s", e.failed, e.cause, landed)
}

type outcome struct {
	status                  status
	written                 []string
	reason, subject, detail string
}

// expectedEdge is one intended edge; its bearing is the judgement the edge
// carries (the zero bearing where the pairing carries none), origin a demoted
// target's. Source is the supporter — for a premise bullet the id it names —
// and Target the supported end.
type expectedEdge struct {
	Source, Target, Context string
	Bearing                 kb.Bearing
	Origin                  string
}

// expectedEntry is one record the written register must read back as.
type expectedEntry struct {
	NodeID, Title string
	Rigor         *float64
	Rationale     string
	Premises      []expectedEdge
	References    []expectedEdge
	Demoted       []expectedEdge
	StrengthenBy  []string
	Lifts         []pair
	Refutes       []pair
}

func edgeOf(e kb.Edge) expectedEdge {
	out := expectedEdge{Source: e.Source, Target: e.Target, Origin: e.Origin, Bearing: e.Bearing}
	if e.Context != nil {
		out.Context = *e.Context
	}
	return out
}

func edgesOf(edges []kb.Edge) []expectedEdge {
	var out []expectedEdge
	for _, e := range edges {
		out = append(out, edgeOf(e))
	}
	return out
}

// edit is one file's intent. splice takes the file's text and returns the
// candidate; expect is read only after the splice has run, so a splice may
// derive it from the baseline it received.
type edit struct {
	path                                 string
	splice                               func(string) (string, error)
	expect                               *[]expectedEntry
	claimDelta, analysisDelta, workDelta int
	create                               bool
}

// census is one register's markers per kind against the records each kind's
// parser returns, and the markers bound under a heading not their own.
type census struct {
	claimMarkers, claimRecords       int
	analysisMarkers, analysisRecords int
	workMarkers, workRecords         int
	misbound                         []string
}

func (c census) consistent() bool {
	return c.claimMarkers == c.claimRecords && c.analysisMarkers == c.analysisRecords &&
		c.workMarkers == c.workRecords && len(c.misbound) == 0
}

func (c census) describe() string {
	counts := fmt.Sprintf("claim markers %d vs records %d; analysis markers %d vs records %d; work markers %d vs records %d",
		c.claimMarkers, c.claimRecords, c.analysisMarkers, c.analysisRecords, c.workMarkers, c.workRecords)
	if len(c.misbound) == 0 {
		return counts
	}
	return counts + "; bound to a heading that is not their own: " + strings.Join(c.misbound, ", ")
}

// takeCensus counts over text as the readers read a file: newlines
// translated. A pair list the reader cannot take is an error, never a skip.
func takeCensus(text string) (census, error) {
	text = kb.TranslateNewlines(text)
	located := kb.LocateRegisterEntries(text)
	var c census
	for _, e := range located {
		switch e.Kind {
		case "clm":
			c.claimMarkers++
		case "ana":
			c.analysisMarkers++
		case kb.WorkPrefix:
			c.workMarkers++
		}
	}
	c.claimRecords = len(kb.ParseClaimEntries(text, "", nil, log.Discard()))
	_, analyses, err := kb.ParseAnalysisEntries(text, "", nil, log.Discard())
	if err != nil {
		return c, err
	}
	c.analysisRecords = len(analyses)
	c.workRecords = len(kb.ParseWorkEntries(text, ""))
	for _, e := range kb.MisBoundEntries(located) {
		c.misbound = append(c.misbound, e.NodeID)
	}
	return c, nil
}

// registerRead is every record of one register text as the readers return
// it.
type registerRead struct {
	claims   map[string]kb.ClaimEntry
	analyses map[string]kb.AnalysisEntry
	works    map[string]kb.ExternalWork
}

func readRegister(text string) (registerRead, error) {
	text = kb.TranslateNewlines(text)
	r := registerRead{claims: map[string]kb.ClaimEntry{}, analyses: map[string]kb.AnalysisEntry{}, works: map[string]kb.ExternalWork{}}
	for _, c := range kb.ParseClaimEntries(text, "", nil, log.Discard()) {
		r.claims[c.ID] = c
	}
	_, analyses, err := kb.ParseAnalysisEntries(text, "", nil, log.Discard())
	if err != nil {
		return r, err
	}
	r.analyses = analyses
	for _, w := range kb.ParseWorkEntries(text, "") {
		r.works[w.ID] = w
	}
	return r, nil
}

// pairsOf is edges as the pair list a values document or a declaration
// authored them: the other end and its bearing.
func pairsOf(edges []kb.Edge) []pair {
	var out []pair
	for _, e := range edges {
		var score *float64
		if !e.Bearing.Pending {
			v := e.Bearing.Value
			score = &v
		}
		out = append(out, pair{ID: e.Target, Score: score})
	}
	return out
}

// locate is the bound entry for nodeID with its Rigor span, or a splice
// failure.
func locate(document, nodeID string) (kb.EntryLocation, error) {
	for _, e := range kb.LocateEntries(document) {
		if e.NodeID == nodeID {
			if e.RigorStart < 0 {
				return e, spliceFailed("%s has no ### Rigor section", nodeID)
			}
			return e, nil
		}
	}
	return kb.EntryLocation{}, spliceFailed("no canonical entry for %s in this register", nodeID)
}

func lineTerminator(line string) string {
	body := kb.SplitLines(line)
	if len(body) == 0 {
		return line
	}
	return line[len(body[0]):]
}

// prevailingTerminator is the line break a line spliced in at start carries:
// the displaced line's, else the nearest one above, else \n.
func prevailingTerminator(parts []string, start int) string {
	if start < len(parts) {
		if t := lineTerminator(parts[start]); t != "" {
			return t
		}
	}
	for i := min(start, len(parts)) - 1; i >= 0; i-- {
		if t := lineTerminator(parts[i]); t != "" {
			return t
		}
	}
	return "\n"
}

// spliceLines replaces the line range [start, end) with lines, each emitted
// with the terminator of the line it displaces; every other line keeps its
// bytes, terminator included.
func spliceLines(document string, start, end int, lines []string) string {
	parts := kb.SplitLinesKeepEnds(document)
	if start < 0 || start > end || end > len(parts) {
		panic(fmt.Sprintf("write: line range [%d, %d) is outside a document of %d lines", start, end, len(parts)))
	}
	eol := prevailingTerminator(parts, start)
	head := slices.Clone(parts[:start])
	tail := parts[end:]
	emitted := make([]string, len(lines))
	for i, l := range lines {
		emitted[i] = l + eol
	}
	if len(head) > 0 && lineTerminator(head[len(head)-1]) == "" {
		head[len(head)-1] += eol
	}
	if len(tail) == 0 && len(emitted) > 0 && len(parts) > 0 && lineTerminator(parts[len(parts)-1]) == "" {
		last := emitted[len(emitted)-1]
		emitted[len(emitted)-1] = last[:len(last)-len(eol)]
	}
	return strings.Join(head, "") + strings.Join(emitted, "") + strings.Join(tail, "")
}

// insertEntry appends a rendered entry below every entry already in the
// register, a "---" rule between them, in the register's own terminator.
func insertEntry(document, rendered string) string {
	parts := kb.SplitLinesKeepEnds(document)
	for len(parts) > 0 && kb.Strip(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return rendered + "\n"
	}
	eol := prevailingTerminator(parts, len(parts)-1)
	last := kb.SplitLines(parts[len(parts)-1])[0]
	kept := strings.Join(parts[:len(parts)-1], "") + last + eol
	rule := eol + "---" + eol
	if kb.Strip(last) == "---" {
		rule = ""
	}
	return kept + rule + eol + strings.Join(kb.SplitLines(rendered), eol) + eol
}

var (
	claimFoldBreakRE    = kb.PyRE(`^- (` + strings.Join(kb.RigorFieldKeys, "|") + `):`)
	analysisFoldBreakRE = kb.PyRE(`^- (rigor|strength|rationale|` + kb.ListRestsOn + `|` + kb.LiftsKey + `|` + kb.RefutesKey + `):`)
	workFoldBreakRE     = kb.PyRE(`^- (rigor|rationale):`)
)

// atHeading is whether a stripped line opens a heading. An entry's authored
// fields end at one, so a scan that walks an entry's lines never treats what
// sits below a heading as part of the field above it.
func atHeading(s string) bool {
	return strings.HasPrefix(s, "## ") || strings.HasPrefix(s, "### ")
}

// foldBreakFor is the reader's fold-break key list for nodeID's entry kind.
func foldBreakFor(nodeID string) *regexp.Regexp {
	switch {
	case strings.HasPrefix(nodeID, kb.WorkPrefix+"-"):
		return workFoldBreakRE
	case strings.HasPrefix(nodeID, "ana-"):
		return analysisFoldBreakRE
	}
	return claimFoldBreakRE
}

var (
	singleLineFields = []string{"rigor"}
	foldedFields     = []string{"rationale"}
)

// replaceFieldLine replaces one authored field inside one entry's Rigor
// section, over the span the reader folds into it; every other line, derived
// ones included, keeps its bytes.
func replaceFieldLine(document, nodeID, fieldName, line string) (string, error) {
	single := slices.Contains(singleLineFields, fieldName)
	if !single && !slices.Contains(foldedFields, fieldName) {
		return "", spliceFailed("%q is outside the field-replacement vocabulary", fieldName)
	}
	e, err := locate(document, nodeID)
	if err != nil {
		return "", err
	}
	lines := kb.SplitLines(document)
	key := "- " + fieldName + ":"
	for i := e.RigorStart; i < min(e.RigorEnd, len(lines)); i++ {
		if !strings.HasPrefix(kb.Strip(lines[i]), key) {
			continue
		}
		indent := lines[i][:len(lines[i])-len(kb.LStrip(lines[i]))]
		end := i + 1
		if !single {
			breakRE := foldBreakFor(nodeID)
			for end < e.RigorEnd && end < len(lines) {
				s := kb.Strip(lines[end])
				if s == "" || atHeading(s) || breakRE.MatchString(s) {
					break
				}
				end++
			}
		}
		return spliceLines(document, i, end, []string{indent + line}), nil
	}
	return "", spliceFailed("%s has no - %s: line to replace", nodeID, fieldName)
}

// resolveTarget resolves a kb-root-relative path and refuses one that is a
// symbolic link or does not name a file inside kb-root.
func resolveTarget(root, rel string) (string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", &storeError{reason: reasonPathOutsideRoot, subject: strings.ReplaceAll(rel, "\x00", `\x00`),
			detail: "contains a NUL byte, which no path can hold"}
	}
	joined := filepath.Join(root, filepath.FromSlash(rel))
	if info, err := os.Lstat(joined); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", &kb.SymlinkError{Path: joined}
	}
	target := kb.ResolvePath(joined)
	if target == root {
		return "", &storeError{reason: reasonPathOutsideRoot, subject: rel,
			detail: fmt.Sprintf("resolves to the kb root %s itself, and a target must be a file inside it", root)}
	}
	if !kb.Within(root, target) {
		return "", &storeError{reason: reasonPathOutsideRoot, subject: rel,
			detail: fmt.Sprintf("resolves to %s which is outside %s", target, root)}
	}
	return target, nil
}

// readExact is a file's exact text as the KB reads it, refused when it is
// not UTF-8.
func readExact(src *kb.Source, path, subject string) (string, error) {
	data, err := src.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", &storeError{reason: reasonNotUTF8, subject: subject, detail: "is not valid UTF-8"}
	}
	return string(data), nil
}

// onDisk is the live file's bytes, nil where it does not exist.
func onDisk(src *kb.Source, path string) (*string, error) {
	data, err := src.DiskFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := string(data)
	return &s, nil
}

// stillMatches is whether the live file still holds what it held when it
// was read; a nil disk means it must still be absent.
func stillMatches(src *kb.Source, path string, disk *string) bool {
	now, err := onDisk(src, path)
	if err != nil {
		return false
	}
	return (now == nil) == (disk == nil) && (now == nil || *now == *disk)
}

type composed struct {
	edit      edit
	target    string
	subject   string
	baseline  *string
	disk      *string
	before    census
	candidate string
}

func compose(e edit, src *kb.Source, root string, claimed map[string]bool) (composed, error) {
	target, err := resolveTarget(root, e.path)
	if err != nil {
		return composed{}, err
	}
	rel, _ := filepath.Rel(root, target)
	subject := filepath.ToSlash(rel)
	if claimed[subject] {
		return composed{}, &storeError{reason: reasonDuplicateTarget, subject: subject,
			detail: "is named twice in one batch; both edits would splice from the same baseline"}
	}
	claimed[subject] = true
	c := composed{edit: e, target: target, subject: subject}
	disk, err := onDisk(src, target)
	if err != nil {
		return composed{}, err
	}
	c.disk = disk
	info, statErr := os.Stat(target)
	switch {
	case statErr == nil && info.Mode().IsRegular():
		text, err := readExact(src, target, subject)
		if err != nil {
			return composed{}, err
		}
		before, err := takeCensus(text)
		if err != nil {
			return composed{}, &storeError{reason: reasonMalformed, subject: subject,
				detail: "holds a pair list the readers cannot take, so nothing is written into it: " + err.Error()}
		}
		c.before = before
		if !stillMatches(src, target, disk) {
			return composed{}, &storeError{reason: reasonContended, subject: subject, contended: true,
				detail: "changed while it was being read, so its census describes no single version of it"}
		}
		if !c.before.consistent() {
			return composed{}, &storeError{reason: reasonCensusMismatch, subject: subject,
				detail: "is already losing entries and will not be written into: " + c.before.describe()}
		}
		c.baseline = &text
	case statErr == nil:
		return composed{}, &storeError{reason: reasonRegisterAbsent, subject: subject,
			detail: "exists but is not a file, so there is no register here to write into"}
	case e.create:
		if info, err := os.Stat(filepath.Dir(target)); err != nil || !info.IsDir() {
			return composed{}, &storeError{reason: reasonRegisterAbsent, subject: subject,
				detail: "cannot be created: its parent directory does not exist"}
		}
	default:
		return composed{}, &storeError{reason: reasonRegisterAbsent, subject: subject,
			detail: "does not exist, and creation was not asked for (creation is never implicit)"}
	}
	base := ""
	if c.baseline != nil {
		base = *c.baseline
	}
	candidate, err := e.splice(base)
	if err != nil {
		var se spliceError
		if errors.As(err, &se) {
			return composed{}, &storeError{reason: reasonSpliceFailed, subject: subject, detail: se.msg}
		}
		return composed{}, err
	}
	c.candidate = candidate
	return c, nil
}

// prove reads the candidate back through the production readers: its census
// must hold, its record counts must be the baseline's plus the intended
// deltas, and every expected record must read back field for field.
func prove(c composed) error {
	after, err := takeCensus(c.candidate)
	if err != nil {
		return &storeError{reason: reasonMalformed, subject: c.subject,
			detail: "the composed candidate holds a pair list the readers cannot take: " + err.Error()}
	}
	if !after.consistent() {
		return &storeError{reason: reasonRecordCount, subject: c.subject,
			detail: "the composed candidate loses a record, or binds one to the wrong heading: " + after.describe()}
	}
	for _, k := range []struct {
		kind      string
		want, got int
	}{
		{"claim", c.before.claimRecords + c.edit.claimDelta, after.claimRecords},
		{"analysis", c.before.analysisRecords + c.edit.analysisDelta, after.analysisRecords},
		{"work", c.before.workRecords + c.edit.workDelta, after.workRecords},
	} {
		if k.want != k.got {
			return &storeError{reason: reasonRecordCount, subject: c.subject,
				detail: fmt.Sprintf("%s record count is %d, expected %d (before plus the intended delta)", k.kind, k.got, k.want)}
		}
	}
	if c.edit.expect == nil || len(*c.edit.expect) == 0 {
		return nil
	}
	read, err := readRegister(c.candidate)
	if err != nil {
		return &storeError{reason: reasonMalformed, subject: c.subject,
			detail: "the composed candidate holds a pair list the readers cannot take: " + err.Error()}
	}
	for _, want := range *c.edit.expect {
		got, ok := read.record(want.NodeID)
		if !ok {
			return &storeError{reason: reasonReadbackMismatch, subject: c.subject + ":" + want.NodeID,
				detail: "the production parser returns no record for this id in the composed candidate"}
		}
		if wrong := mismatchedFields(want, got); len(wrong) > 0 {
			return &storeError{reason: reasonReadbackMismatch, subject: c.subject + ":" + want.NodeID,
				detail: "read back with a different " + strings.Join(wrong, ", ") + " than the values supplied"}
		}
	}
	return nil
}

// record is nodeID's record as an expectation of itself.
func (r registerRead) record(nodeID string) (expectedEntry, bool) {
	switch {
	case strings.HasPrefix(nodeID, kb.WorkPrefix+"-"):
		w, ok := r.works[nodeID]
		return expectedEntry{NodeID: nodeID, Title: w.Title, Rigor: w.Rigor, Rationale: w.Rationale}, ok
	case strings.HasPrefix(nodeID, "ana-"):
		a, ok := r.analyses[nodeID]
		return expectedEntry{NodeID: nodeID, Title: a.Title, Rigor: a.Rigor, Rationale: a.Rationale,
			Premises: edgesOf(a.Premises), Lifts: pairsOf(a.Lifts), Refutes: pairsOf(a.Refutes)}, ok
	}
	c, ok := r.claims[nodeID]
	e := expectedEntry{NodeID: nodeID, Title: c.Title, Rigor: c.Rigor, Rationale: c.Rationale,
		Premises: edgesOf(c.Premises), References: edgesOf(c.References), Demoted: edgesOf(c.Demoted),
		Refutes: pairsOf(c.Refutes)}
	for _, item := range c.StrengthenBy {
		e.StrengthenBy = append(e.StrengthenBy, item.Text)
	}
	return e, ok
}

var axiomExpectRE = regexp.MustCompile(`^Axiom (` + kb.AxiomLabel + `)$`)

// axiomEdgeID is an "Axiom <label>" token as the reader keys the edge it
// authors; any other token is itself.
func axiomEdgeID(token string) string {
	if m := axiomExpectRE.FindStringSubmatch(kb.Strip(token)); m != nil {
		return axiomIDPrefix + m[1]
	}
	return token
}

// normalized is an expectation in the form the readers return: prose
// collapsed, an axiom token in its edge spelling.
func normalized(e expectedEntry) expectedEntry {
	out := e
	out.Title, out.Rationale = collapse(e.Title), collapse(e.Rationale)
	norm := func(edges []expectedEdge) []expectedEdge {
		var res []expectedEdge
		for _, edge := range edges {
			res = append(res, expectedEdge{Source: axiomEdgeID(edge.Source), Target: axiomEdgeID(edge.Target),
				Context: collapse(edge.Context), Bearing: edge.Bearing, Origin: edge.Origin})
		}
		return res
	}
	out.Premises, out.References, out.Demoted = norm(e.Premises), norm(e.References), norm(e.Demoted)
	out.StrengthenBy = nil
	for _, s := range e.StrengthenBy {
		out.StrengthenBy = append(out.StrengthenBy, collapse(s))
	}
	return out
}

func scoreEqual(a, b *float64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func edgesEqual(a, b []expectedEdge) bool {
	return slices.EqualFunc(a, b, func(x, y expectedEdge) bool {
		return x.Source == y.Source && x.Target == y.Target && x.Context == y.Context && x.Bearing == y.Bearing && x.Origin == y.Origin
	})
}

func pairsEqual(a, b []pair) bool {
	return slices.EqualFunc(a, b, func(x, y pair) bool { return x.ID == y.ID && scoreEqual(x.Score, y.Score) })
}

// mismatchedFields names the compared fields on which got differs from want,
// as the values keys of a claim or analysis entry spell them.
func mismatchedFields(want, got expectedEntry) []string {
	w := normalized(want)
	var out []string
	for _, f := range []struct {
		name string
		same bool
	}{
		{"title", w.Title == got.Title},
		{"rigor", scoreEqual(w.Rigor, got.Rigor)},
		{"rationale", w.Rationale == got.Rationale},
		{kb.ListRestsOn, edgesEqual(w.Premises, got.Premises)},
		{"references", edgesEqual(w.References, got.References)},
		{kb.RelationDemoted, edgesEqual(w.Demoted, got.Demoted)},
		{"strengthen-by", slices.Equal(w.StrengthenBy, got.StrengthenBy)},
		{kb.LiftsKey, pairsEqual(w.Lifts, got.Lifts)},
		{kb.RefutesKey, pairsEqual(w.Refutes, got.Refutes)},
	} {
		if !f.same {
			out = append(out, f.name)
		}
	}
	return out
}

// applyEdits runs a batch all-or-nothing. Every edit is composed — read,
// census, splice — then proven, checked against a fresh read of the live
// file, and replaced. An edit whose candidate is its baseline is proven and
// not written.
func applyEdits(src *kb.Source, edits []edit) (outcome, error) {
	if len(edits) == 0 {
		return outcome{status: statusUnchanged}, nil
	}
	root := kb.ResolvePath(src.Root())
	claimed := map[string]bool{}
	var all []composed
	for _, e := range edits {
		c, err := compose(e, src, root, claimed)
		if err != nil {
			return storeOutcome(err)
		}
		all = append(all, c)
	}
	for _, c := range all {
		if err := atomicfile.Sweep(c.target); err != nil {
			return outcome{}, err
		}
	}
	for _, c := range all {
		if err := prove(c); err != nil {
			return storeOutcome(err)
		}
	}
	for _, c := range all {
		if !stillMatches(src, c.target, c.disk) {
			return storeOutcome(&storeError{reason: reasonContended, subject: c.subject, contended: true,
				detail: "changed between the read and the replace; a concurrent writer intervened"})
		}
	}
	entryPoint := src.KBPath(kb.EntryPointFile)
	var written []string
	for _, c := range all {
		if c.baseline != nil && *c.baseline == c.candidate {
			continue
		}
		// A migrated KB's entry point carries the stamp, so it lands only
		// with the save that rewrites the whole KB, last.
		if src.Migrated() && c.target == kb.ResolvePath(entryPoint) {
			src.Put(c.target, []byte(c.candidate))
		} else if err := index.WriteKBFile(src, c.target, []byte(c.candidate)); err != nil {
			return outcome{}, &interruptedError{written: written, failed: c.subject, cause: err}
		}
		written = append(written, c.subject)
	}
	if len(written) == 0 {
		return outcome{status: statusUnchanged}, nil
	}
	return outcome{status: statusWritten, written: written}, nil
}

// storeOutcome turns a store refusal into its outcome and passes any other
// error through.
func storeOutcome(err error) (outcome, error) {
	var se *storeError
	if !errors.As(err, &se) {
		return outcome{}, err
	}
	st := statusRefused
	if se.contended {
		st = statusRetry
	}
	return outcome{status: st, reason: se.reason, subject: se.subject, detail: se.detail}, nil
}
