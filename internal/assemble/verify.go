package assemble

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strings"

	"kbase/internal/distill"
	"kbase/internal/survey"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// Stage 9 (§9): the nine gates, run over the assembled tree IN THE STORE,
// before any byte reaches <out> (I-5).
//
// Every check names the file class it applies to [MAD1: F-3]. Without that
// scoping checks 2, 4 and 5 fail by construction on every green run, because
// stage 8 ships fixtures that are not nodes and carry no up-link.
//
// Any failure refuses delivery and names the node. There is no
// deliver-with-warnings mode.
const (
	// ReportSchema versions the verify report. Something reads it back — the
	// composing verb's run record and a human looking at a failed job — so it
	// says what shape it is.
	ReportSchema = "kbase.verify/1"

	// upLinkPrefix opens the up-link line. Checks 2 and 9 count and locate
	// up-links with it; check 5 compares the whole line.
	upLinkPrefix = "[↑ "
)

// Verification is everything the gates read.
//
// The distiller and the tree-plan verifier are the two upstream objects the
// checks RE-DERIVE from rather than trust: check 7 re-slices every leaf out of
// the corpus through the distiller, and check 6 asks the tree-plan verifier
// its own coverage question. Both are the packages that own those statements,
// asked here rather than restated here.
type Verification struct {
	Plan     treeplan.TreePlan
	Renderer *Renderer
	// Files is the delivered tree: tree-relative slash-separated path to
	// bytes, both classes.
	Files map[string][]byte
	// Leaves is stage 5's rendering of every leaf, which check 1 reads for its
	// unresolved-destination exemptions and check 7 compares bytes against.
	Leaves map[string]distill.Leaf
	// Verifier answers check 6 over the tree plan and the corpus behind it.
	Verifier *treeplan.Verifier
	// Cuts holds one entry per split group, for check 7's tiling proof.
	Cuts map[string][]survey.Span
	// Est measures the entry-point against its ceiling (check 8).
	Est tokens.Estimator
}

// CheckResult is one gate's outcome.
type CheckResult struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	// Detail names the node or file a failure is about, and is empty on a
	// pass. It is what "refuses delivery and names the node" means in the
	// artifact rather than only in the error.
	Detail string `json:"detail,omitempty"`
}

// Report is stage 9's artifact.
type Report struct {
	Schema     string        `json:"schema"`
	CorpusHash string        `json:"corpusHash"`
	Files      int           `json:"files"`
	ClassA     int           `json:"classA"`
	ClassB     int           `json:"classB"`
	Leaves     int           `json:"leaves"`
	Indexes    int           `json:"indexes"`
	Checks     []CheckResult `json:"checks"`
}

// Passed reports whether every gate passed.
func (r Report) Passed() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return false
		}
	}
	return true
}

// WriteJSON writes the report with the artifact discipline every other derived
// file here uses: escaping off, indented, one trailing newline.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("assemble: write verify report: %w", err)
	}
	return nil
}

// EncodeReport renders the report for the store — the verify stage's
// pipeline.StagePlan.Encode.
func EncodeReport(artifact any) ([]byte, error) {
	r, ok := artifact.(Report)
	if !ok {
		return nil, fmt.Errorf("assemble: artifact is %T, not a Report", artifact)
	}
	var buf bytes.Buffer
	if err := r.WriteJSON(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Verify runs all nine gates and returns the report plus the first refusal.
//
// Every check runs even after one has failed: a report that stopped at the
// first failure would make the second one a surprise on the next run, and the
// whole set costs milliseconds. The error is the first failure in check order,
// which is the one whose cause is furthest upstream.
func Verify(v Verification) (Report, error) {
	rep := Report{Schema: ReportSchema, CorpusHash: v.Plan.CorpusHash, Files: len(v.Files)}
	for _, n := range v.Plan.Nodes {
		switch n.Kind {
		case treeplan.KindLeaf:
			rep.Leaves++
		default:
			rep.Indexes++
		}
	}
	classA, classB, stray := classify(v.Plan, v.Files)
	rep.ClassA, rep.ClassB = len(classA), len(classB)

	rep.Checks = []CheckResult{
		result(1, "every referenced file exists", v.checkLinksResolve(classA, classB)),
		result(2, "one up-link, at line 1, on every class-A node but the entry-point", v.checkUpLinks(classA)),
		result(3, "the entry-point exists and its domains exist", v.checkEntryPoint()),
		result(4, "no unreachable documents, per class", v.checkReachable(classA, classB)),
		result(5, "the delivered set is the tree plan's nodes plus the fixture manifest", v.checkConformance(classA, classB, stray)),
		result(6, "coverage and tiling: every surveyed section is on exactly one page or in an annex", v.checkCoverage()),
		result(7, "leaf fidelity: every page re-derives byte-for-byte from source", v.checkLeafFidelity(classA)),
		result(8, "structural caps: depth, fan-out, entry-point ceiling", v.checkCaps()),
		result(9, "grammar conformance, per node kind and per fixture template", v.checkGrammar(classA, classB)),
	}
	for _, c := range rep.Checks {
		if !c.OK {
			return rep, fmt.Errorf("assemble: verify gate %d (%s) refuses delivery: %s", c.Number, c.Name, c.Detail)
		}
	}
	return rep, nil
}

func result(n int, name string, err error) CheckResult {
	if err != nil {
		return CheckResult{Number: n, Name: name, OK: false, Detail: err.Error()}
	}
	return CheckResult{Number: n, Name: name, OK: true}
}

// classify splits the delivered set into the two classes and whatever belongs
// to neither. Sorted, so a failure reads the same way on every run.
func classify(plan treeplan.TreePlan, files map[string][]byte) (classA, classB, stray []string) {
	nodes := map[string]bool{}
	for _, n := range plan.Nodes {
		nodes[n.Path] = true
	}
	fixtures := map[string]bool{}
	for _, f := range Manifest() {
		fixtures[f] = true
	}
	for p := range files {
		switch {
		case nodes[p]:
			classA = append(classA, p)
		case fixtures[p]:
			classB = append(classB, p)
		default:
			stray = append(stray, p)
		}
	}
	sort.Strings(classA)
	sort.Strings(classB)
	sort.Strings(stray)
	return classA, classB, stray
}

// checkLinksResolve is check 1: every file referenced in any link exists —
// ALL delivered files, both classes.
//
// A destination is exempt when it points off the corpus (a scheme, a bare
// fragment) or when stage 5 inventoried it as unresolvable: a source link
// whose target is LinkUnresolved or sits in an annex stays verbatim, and
// failing here would be failing on a defect in someone else's corpus (§4.4).
func (v Verification) checkLinksResolve(classA, classB []string) error {
	for _, p := range append(slices.Clone(classA), classB...) {
		exempt := map[string]bool{}
		if l, ok := v.Leaves[p]; ok {
			for _, u := range l.Unresolved {
				exempt[u] = true
			}
		}
		body := v.Files[p]
		for _, d := range distill.Destinations(body) {
			text := d.Text(body)
			if distill.IsExternal(text) || exempt[text] {
				continue
			}
			target := path.Clean(path.Join(path.Dir(p), strings.SplitN(text, "#", 2)[0]))
			if _, ok := v.Files[target]; !ok {
				return fmt.Errorf("%s links to %q, which the tree does not deliver", p, text)
			}
		}
	}
	return nil
}

// checkUpLinks is check 2: every class-A document except the entry-point has
// exactly one up-link, at line 1. Class B carries none, by grammar.
func (v Verification) checkUpLinks(classA []string) error {
	entry := v.Renderer.EntryPointPath()
	for _, p := range classA {
		lines := strings.Split(string(v.Files[p]), "\n")
		n := 0
		for _, l := range lines {
			if strings.HasPrefix(l, upLinkPrefix) {
				n++
			}
		}
		if p == entry {
			if n != 0 {
				return fmt.Errorf("the entry-point %s carries an up-link", p)
			}
			continue
		}
		if n != 1 {
			return fmt.Errorf("%s carries %d up-links; a class-A node has exactly one", p, n)
		}
		if !strings.HasPrefix(lines[0], upLinkPrefix) {
			return fmt.Errorf("%s does not open with its up-link", p)
		}
	}
	return nil
}

// checkEntryPoint is check 3: it exists, and every domain it references does.
func (v Verification) checkEntryPoint() error {
	entry := v.Renderer.EntryPointPath()
	if entry == "" {
		return fmt.Errorf("the tree plan holds no entry-point node")
	}
	if _, ok := v.Files[entry]; !ok {
		return fmt.Errorf("the entry-point %s was not delivered", entry)
	}
	for _, c := range v.Renderer.Children(entry) {
		if _, ok := v.Files[c.Path]; !ok {
			return fmt.Errorf("the entry-point names the domain %s, which was not delivered", c.Path)
		}
	}
	return nil
}

// checkReachable is check 4, stated per class.
//
// Class A: every file is linked from at least one other class-A file. Class B
// inverts — fixtures are landing sites, not destinations — so each root member
// links TO the entry-point, and `.agents/` members are covered by the
// directory-level pointer the entry-point and AGENTS.md both carry. That
// pointer discharges reachability for `.agents/*` and for nothing else.
func (v Verification) checkReachable(classA, classB []string) error {
	linked := map[string]bool{}
	for _, p := range classA {
		body := v.Files[p]
		for _, d := range distill.Destinations(body) {
			text := d.Text(body)
			if distill.IsExternal(text) {
				continue
			}
			linked[path.Clean(path.Join(path.Dir(p), strings.SplitN(text, "#", 2)[0]))] = true
		}
	}
	for _, p := range classA {
		if !linked[p] {
			return fmt.Errorf("%s is linked from no other page", p)
		}
	}
	entry := v.Renderer.EntryPointPath()
	for _, p := range classB {
		if InAgentsDir(p) {
			continue
		}
		if !bytes.Contains(v.Files[p], []byte("("+entry+")")) {
			return fmt.Errorf("the fixture %s does not link to %s", p, entry)
		}
	}
	return nil
}

// checkConformance is check 5: the delivered set equals the tree plan's nodes
// plus the manifest, and every class-A up-link and down-link matches the tree
// plan character-for-character.
//
// The second half is stated as a re-render: an index and the entry-point are
// pure functions of the tree plan (§8), so re-rendering one and comparing
// bytes says everything a line-by-line comparison of its links would, and says
// it about the whole file. Leaves are excluded here and proven by check 7,
// which re-derives them from source instead.
func (v Verification) checkConformance(classA, classB, stray []string) error {
	if len(stray) > 0 {
		return fmt.Errorf("%s is neither a tree-plan node nor a fixture; nothing else may appear under the tree", stray[0])
	}
	have := map[string]bool{}
	for _, p := range append(slices.Clone(classA), classB...) {
		have[p] = true
	}
	for _, n := range v.Plan.Nodes {
		if !have[n.Path] {
			return fmt.Errorf("the tree plan holds the node %s, which was not delivered", n.Path)
		}
	}
	for _, f := range Manifest() {
		if !have[f] {
			return fmt.Errorf("the fixture manifest names %s, which was not delivered", f)
		}
	}
	for _, n := range v.Plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			continue
		}
		want, err := v.Renderer.Node(n)
		if err != nil {
			return fmt.Errorf("%s: %w", n.Path, err)
		}
		if !bytes.Equal(want, v.Files[n.Path]) {
			return fmt.Errorf("%s is not what the tree plan renders to", n.Path)
		}
	}
	return nil
}

// checkCoverage is check 6, asked of the package that owns the statement.
//
// treeplan.Verifier.Check IS the definition of a valid tree plan — partition,
// caps, G-1, G-2 and the tiling of every surveyed section against the corpus
// under custody. Restating any of it here would be a second rule with nothing
// keeping the two in agreement, which is the failure mode this design spends
// most of its rules avoiding.
func (v Verification) checkCoverage() error {
	if v.Verifier == nil {
		return fmt.Errorf("no tree-plan verifier was supplied; coverage cannot be re-derived")
	}
	return v.Verifier.Check(v.Plan)
}

// checkLeafFidelity is check 7: for every leaf, re-derive template(source
// slice, rebase map) and compare to the delivered file.
//
// The re-derivation runs the same renderer stage 5 ran, over the CORPUS BYTES
// rather than over the leaves artifact — so what it proves is that the
// delivered page is a fresh function of (source, tree plan, cut list, link
// graph) and not a stale or corrupted copy of one. It does not prove the
// renderer itself is right; nothing self-checking could, and the renderer's
// own tests are where that lives.
//
// For a multi-part group it additionally proves the cut list tiles the group's
// span exactly and numbers SplitGroup.Parts [MAD1: F-2].
func (v Verification) checkLeafFidelity(classA []string) error {
	for _, g := range v.Plan.Groups {
		if g.Parts == 1 {
			continue
		}
		cuts := v.Cuts[g.ID]
		if len(cuts) != g.Parts {
			return fmt.Errorf("group %s claims %d parts and its cut list holds %d sections",
				g.ID, g.Parts, len(cuts))
		}
		if err := tilesSpan(survey.Span{Start: g.Source.Start, End: g.Source.End}, cuts); err != nil {
			return fmt.Errorf("group %s: %w", g.ID, err)
		}
	}
	for _, p := range classA {
		want, ok := v.Leaves[p]
		if !ok {
			continue
		}
		if !bytes.Equal(want.Data, v.Files[p]) {
			return fmt.Errorf("%s does not match its re-derivation from source", p)
		}
	}
	for p := range v.Leaves {
		if _, ok := v.Files[p]; !ok {
			return fmt.Errorf("the page %s re-derives from source but was not delivered", p)
		}
	}
	return nil
}

// checkCaps is check 8: depth ≤ the index-level cap, fan-out ≤ its cap, and
// the entry-point within its token ceiling. Class A only.
//
// The first two are computed from the tree plan and the third from the
// RENDERED bytes, which is the whole reason the ceiling is enforced here and
// not at stage 3: stage 3 has no page to measure.
func (v Verification) checkCaps() error {
	level := map[string]int{}
	fanOut := map[string]int{}
	for _, n := range v.Plan.Nodes {
		if n.Parent == "" {
			level[n.Path] = 1
			continue
		}
		level[n.Path] = level[n.Parent] + 1
		fanOut[n.Parent]++
		if n.Kind != treeplan.KindLeaf && level[n.Path] > v.Plan.Budgets.DepthCap {
			return fmt.Errorf("%s sits at index level %d, past the cap of %d",
				n.Path, level[n.Path], v.Plan.Budgets.DepthCap)
		}
	}
	for _, n := range v.Plan.Nodes {
		if fanOut[n.Path] > v.Plan.Budgets.FanOutCap {
			return fmt.Errorf("%s holds %d entries, past the fan-out cap of %d",
				n.Path, fanOut[n.Path], v.Plan.Budgets.FanOutCap)
		}
	}
	entry := v.Renderer.EntryPointPath()
	if got := v.Est.EstimateBytes(v.Files[entry]); got > v.Plan.Budgets.EntryPointTokens {
		return fmt.Errorf("%s is about %d tokens, past its ceiling of %d; the entry-point is refused, never truncated",
			entry, got, v.Plan.Budgets.EntryPointTokens)
	}
	return nil
}

// checkGrammar is check 9: each class-A node kind matches §4's section order —
// including the entry-point's mandatory contract, `.agents/` and annex blocks
// [MAD1: F-5] and the provenance footer on all three kinds [MAD1: F-12] — and
// each class-B file matches its own fixture template rather than §4.
func (v Verification) checkGrammar(classA, classB []string) error {
	byPath := map[string]treeplan.Node{}
	for _, n := range v.Plan.Nodes {
		byPath[n.Path] = n
	}
	for _, p := range classA {
		body := v.Files[p]
		if !distill.HasFooter(body) {
			return fmt.Errorf("%s does not end in its provenance receipt", p)
		}
		text := string(body)
		switch byPath[p].Kind {
		case treeplan.KindLeaf:
			if strings.Contains(text, derivationsHeading) {
				return fmt.Errorf("%s is a page carrying an index's down-link list", p)
			}
		case treeplan.KindIndex:
			if !strings.Contains(text, "\n"+derivationsHeading+"\n") {
				return fmt.Errorf("%s is a section with no %q list", p, derivationsHeading)
			}
		case treeplan.KindEntryPoint:
			if err := entryPointGrammar(text, len(v.Plan.Annexes) > 0); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
		}
	}
	fixtures := map[string][]byte{}
	for _, f := range v.Renderer.Fixtures() {
		fixtures[f.Path] = f.Data
	}
	for _, p := range classB {
		if !bytes.Equal(fixtures[p], v.Files[p]) {
			return fmt.Errorf("the fixture %s is not what its template renders to", p)
		}
	}
	return nil
}

// tilesSpan proves a cut list covers a group's span exactly once: it starts
// where the span starts, each section begins where the last ended, and the
// last ends where the span ends.
//
// Stage 4's own verifier proved this of the list it composed; check 7 proves
// it again of the list on disk, because the artifact crossed a run boundary
// and the reader cannot ask the writer what it meant.
func tilesSpan(span survey.Span, cuts []survey.Span) error {
	if len(cuts) == 0 {
		return fmt.Errorf("the cut list holds no sections")
	}
	at := span.Start
	for i, c := range cuts {
		if c.Start != at {
			return fmt.Errorf("section %d starts at %d, want %d", i+1, c.Start, at)
		}
		if c.End <= c.Start {
			return fmt.Errorf("section %d spans [%d,%d)", i+1, c.Start, c.End)
		}
		at = c.End
	}
	if at != span.End {
		return fmt.Errorf("the sections end at %d, the span ends at %d", at, span.End)
	}
	return nil
}

// entryPointGrammar checks §4.3's mandatory blocks and their fixed order.
func entryPointGrammar(text string, annexed bool) error {
	want := []string{"# ", "\n" + domainsHeading + "\n", "\n" + usingHeading + "\n", contractText, agentsPointer}
	if annexed {
		want = append(want, "\n"+annexHeading+"\n")
	}
	at := 0
	for _, w := range want {
		i := strings.Index(text[at:], w)
		if i < 0 {
			return fmt.Errorf("is missing %q, or carries it out of §4.3's order", strings.TrimSpace(w))
		}
		at += i + len(w)
	}
	if !annexed && strings.Contains(text, annexHeading) {
		return fmt.Errorf("carries an annex lookup section with no annexes declared")
	}
	return nil
}
