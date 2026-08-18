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

// Stage 9 (§4.8): the ten gates, run over the assembled tree IN THE STORE,
// before any byte reaches <out> (I-5).
//
// Every check names the file class it applies to [MAD1: F-3]. Without that
// scoping checks 2, 4, 5 and 10 fail by construction on every green run,
// because stage 8 ships fixtures that are not nodes and carry no up-link and
// no Location.
//
// Any failure refuses delivery and names the node. There is no
// deliver-with-warnings mode.
const (
	// ReportSchema versions the verify report. Something reads it back — the
	// composing verb's run record and a human looking at a failed job — so it
	// says what shape it is.
	ReportSchema = "kbase.verify/1"
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
	Schema     string `json:"schema"`
	CorpusHash string `json:"corpusHash"`
	Files      int    `json:"files"`
	ClassA     int    `json:"classA"`
	ClassB     int    `json:"classB"`
	Leaves     int    `json:"leaves"`
	Indexes    int    `json:"indexes"`
	// ExemptedDestinations is how many DISTINCT destinations guarantee 1
	// exempts across the whole delivered tree — the set check 1 skips, counted
	// where the set is: every leaf's Unresolved list, which is what the rebase
	// map did not land [ARCH F4].
	//
	// It is not the survey's `links.unresolved` census and does not replace it.
	// That number classifies the corpus's own link graph; this one is the
	// exemption itself, which is strictly the wider idea — a destination inside
	// an annex, or into a file no node was drawn from (§4.5 rule 3), is exempt
	// and was never unresolved. Recording only the census measured the wrong
	// seam: B-1's "the exemption is MEASURED, never silent" needs the number
	// the gate uses. Distinct rather than total occurrences, because a broken
	// destination repeated on forty pages is one thing to go and fix.
	ExemptedDestinations int           `json:"exemptedDestinations"`
	Checks               []CheckResult `json:"checks"`
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

// Verify runs all ten gates and returns the report plus the first refusal.
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
	rep.ExemptedDestinations = exemptedDestinations(v.Leaves)

	rep.Checks = []CheckResult{
		result(1, "every referenced file exists", v.checkLinksResolve(classA, classB)),
		result(2, "one up-link under the frontmatter block, and every navigation link in it labelled with the node it points at, on every class-A node but the entry-point", v.checkNavLinks(classA)),
		result(3, "the entry-point exists and its domains exist", v.checkEntryPoint()),
		result(4, "every page is reachable from the entry-point, and every fixture links to it", v.checkReachable(classA, classB)),
		result(5, "the delivered set is the tree plan's nodes plus the fixture manifest", v.checkConformance(classA, classB, stray)),
		result(6, "coverage and tiling: every surveyed section is on exactly one page or in an annex", v.checkCoverage()),
		result(7, "leaf fidelity: every page re-derives byte-for-byte from source", v.checkLeafFidelity(classA)),
		result(8, "structural caps: depth, fan-out, entry-point ceiling", v.checkCaps()),
		result(9, "grammar conformance, per node kind and per fixture template", v.checkGrammar(classA, classB)),
		result(10, "every class-A page declares its own delivered path in its frontmatter", v.checkLocation(classA)),
	}
	for _, c := range rep.Checks {
		if !c.OK {
			return rep, fmt.Errorf("assemble: verify gate %d (%s) refuses delivery: %s", c.Number, c.Name, c.Detail)
		}
	}
	return rep, nil
}

// exemptedDestinations counts the distinct destinations guarantee 1 lets
// through, over the same per-leaf lists checkLinksResolve reads.
//
// One counter here rather than a second walk in the composing verb: the gate
// and the number that measures it read one set, so a leaf whose exemption the
// gate honours cannot be missing from the count.
func exemptedDestinations(leaves map[string]distill.Leaf) int {
	seen := map[string]bool{}
	for _, l := range leaves {
		for _, u := range l.Unresolved {
			seen[u] = true
		}
	}
	return len(seen)
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
// failing here would be failing on a defect in someone else's corpus (§4.5 rule 3).
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

// checkNavLinks is check 2: every class-A document except the entry-point has
// exactly one up-link, it is the first line under the frontmatter block, and
// every link in the navigation block it opens — the up-link and a split part's
// continuation edges — is labelled with the node its target resolves to. Class
// B carries none, by grammar.
//
// The agreement half is the point of the label carrying a root-relative path
// (ruled 2026-08-15): two projections of one tree-plan fact, checked against
// each other over the delivered bytes. The renderer cannot disagree with
// itself — a copy, a rebase or a hand edit can, and this is where that shows.
// Continuation edges are the same two projections of the same artifact
// [MAD2: B-4], so they are checked by this loop rather than by a second one
// that would have to be kept in agreement with it.
//
// The COUNT is taken from that same block and from nowhere else [MAD2: B-11,
// ARCH F3]. A whole-page count is a second predicate with a wider scope inside
// one gate, and the width is the defect: a leaf body is verbatim source (§4.2
// item 4), so a corpus documenting this grammar carries an unfenced `[↑ …](…)`
// line as CONTENT, and counting it refused the whole delivery with no remedy
// anyone could apply. Nothing is weakened by dropping it — the block itself is
// still checked entirely, and a page that grew a second up-link below the block
// fails gate 5's byte-for-byte re-render if it is an index and gate 7's leaf
// fidelity if it is a leaf.
func (v Verification) checkNavLinks(classA []string) error {
	entry := v.Renderer.EntryPointPath()
	for _, p := range classA {
		_, body, _ := distill.ParseFrontmatter(v.Files[p])
		block := distill.NavBlock(body)
		n := 0
		for _, l := range block {
			if l.Marker == distill.UpMarker {
				n++
			}
		}
		if p == entry {
			if n != 0 {
				return fmt.Errorf("the entry-point %s carries an up-link", p)
			}
			if len(block) != 0 {
				return fmt.Errorf("the entry-point %s opens with a navigation link", p)
			}
			continue
		}
		if n != 1 {
			return fmt.Errorf("%s carries %d up-links in its navigation block; a class-A node has exactly one", p, n)
		}
		if len(block) == 0 || block[0].Marker != distill.UpMarker {
			return fmt.Errorf("%s does not open with its up-link", p)
		}
		for _, l := range block {
			if got := path.Clean(path.Join(path.Dir(p), l.Target)); got != l.Label {
				return fmt.Errorf("%s: the %s link is labelled %q and points at %q, which is %s; "+
					"the label and the target must name one node", p, l.Marker, l.Label, l.Target, got)
			}
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
// Class A: every page is REACHABLE from the entry-point by following delivered
// links — a traversal, not an in-degree count [MAD2: B-9]. The distinction is
// the whole guarantee: the artifact's own CLAUDE.md tells every consumer to
// "follow the tree from the entry-point instead of grepping the file set", so a
// page no descent from the entry-point reaches does not exist for the reader
// however many pages link to it. A severed island whose members link to each
// other passes an in-degree test and fails this one.
//
// Self-links are excluded by construction rather than by a test: a page's own
// links are only read once the traversal has already reached it, so no page can
// make itself reachable.
//
// It is strictly stronger than the in-degree check it replaces. Reachability
// from the entry-point implies an inbound edge from another page for every page
// but the entry-point itself, and the entry-point's own inbound edges are the
// class-B half below — every fixture links to it, with no member exempt.
//
// Class B inverts — fixtures are landing sites, not destinations — so every
// fixture links TO the entry-point.
func (v Verification) checkReachable(classA, classB []string) error {
	entry := v.Renderer.EntryPointPath()
	inClassA := make(map[string]bool, len(classA))
	for _, p := range classA {
		inClassA[p] = true
	}

	reached := map[string]bool{entry: true}
	for frontier := []string{entry}; len(frontier) > 0; {
		p := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		body := v.Files[p]
		for _, d := range distill.Destinations(body) {
			text := d.Text(body)
			if distill.IsExternal(text) {
				continue
			}
			target := path.Clean(path.Join(path.Dir(p), strings.SplitN(text, "#", 2)[0]))
			if !inClassA[target] || reached[target] {
				continue
			}
			reached[target] = true
			frontier = append(frontier, target)
		}
	}
	for _, p := range classA {
		if !reached[p] {
			return fmt.Errorf("%s is reachable from no descent of %s", p, entry)
		}
	}

	for _, p := range classB {
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
// including the entry-point's mandatory contract, definitions-pointer and
// annex blocks [MAD1: F-5] and the provenance footer on all three kinds
// [MAD1: F-12] — and each class-B file matches its own fixture template rather
// than §4.
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

// checkLocation is check 10: every class-A page opens with a frontmatter block
// declaring its own delivered path (SPEC §4.9). Class B carries none — a
// fixture sits at the tree root and its name is the whole address.
//
// It is a free integrity tripwire rather than a restatement of the render: the
// path compared against is the key the page was DELIVERED under, so a page
// copied to the wrong node, or rebased against the wrong parent, fails here
// even when every link in it still resolves.
//
// The read is the forgiving one (distill.ParseFrontmatter): fields this build does
// not know are skipped, so a later kbase pass may add one. What is refused is
// an absent block, an absent Location, and a Location that names another page.
func (v Verification) checkLocation(classA []string) error {
	for _, p := range classA {
		loc, _, ok := distill.ParseFrontmatter(v.Files[p])
		if !ok {
			return fmt.Errorf("%s does not open with a frontmatter block", p)
		}
		if loc == "" {
			return fmt.Errorf("%s carries a frontmatter block that declares no Location", p)
		}
		if loc != p {
			return fmt.Errorf("%s declares Location %q; a page states the path it was delivered under", p, loc)
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
	want := []string{"# ", "\n" + domainsHeading + "\n", "\n" + usingHeading + "\n", contractText, definitionsPointer}
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
