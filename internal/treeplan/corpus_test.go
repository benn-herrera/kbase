package treeplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
)

// The property over real material. The synthetic trees next door are built to
// hit the shapes the operators decide between; this one is built to hit the
// shapes nobody chose — a corpus written by technical writers who never heard
// of this verifier, at the budgets the appliance actually ships with.
//
// It drives the Markdown adapter to get there, which is why internal/survey's
// import policy names this package: a property about a real tree plan needs a
// real survey, and a survey comes from an adapter. What crosses the seam is
// still only neutral types — sections, ranges and cut candidates — and no
// parser node reaches this package.

// rojoDocs and omlxDocs are the corpora the justfile pins and fetches
// (prep-test-integration-rojo, prep-test-integration-omlx), at the paths those
// recipes guarantee — oMLX is a monorepo fetched sparse, so its documents are
// the docs/ subdirectory of the clone, not the clone. Each corpus SKIPS
// independently when it is absent rather than fetching anything: a unit-test
// run must not reach the network, and the integration recipe is where fetching
// belongs.
const (
	rojoDocs = "test_data/transient/rojo.space/docs"
	omlxDocs = "test_data/transient/omlx/docs"
)

// pinnedCorpus is one corpus these properties run over: where its documents
// live, the recipe that puts them there, what the mechanical proposal calls the
// knowledge base built from it, and where its evidence lands. The properties
// are written once and swept over this table, so a second corpus is a row here
// and nothing else.
type pinnedCorpus struct {
	name string
	docs string
	// prep is the bare recipe name; the skip message names `just <prep>` so a
	// reader of a skipped run knows which fetch to run.
	prep string
	// title and scope are the entry-point identity SourceStructureProposal
	// stamps on the mechanical plan — corpus material, not test material.
	title string
	scope string
	// evidence is where this corpus leaves its observational evidence, per
	// AGENTS.md's testing rule: a measurement worth taking is worth being able
	// to look at afterwards. It is under test_data/transient/ because it is
	// test output — generated, gitignored, and rewritten from scratch every run.
	evidence string
}

// corpora is the swept corpora. Rojo is a small hand-written docs site; oMLX is
// an ML runtime's docs/ tree out of a monorepo — different writers, different
// structural habits, the same claims.
func corpora() []pinnedCorpus {
	return []pinnedCorpus{
		{
			name:     "rojo",
			docs:     rojoDocs,
			prep:     "prep-test-integration-rojo",
			title:    "Rojo Documentation",
			scope:    "the pinned Rojo docs corpus",
			evidence: "test_data/transient/treeplan-rojo",
		},
		{
			name:     "omlx",
			docs:     omlxDocs,
			prep:     "prep-test-integration-omlx",
			title:    "oMLX Documentation",
			scope:    "the pinned oMLX docs corpus",
			evidence: "test_data/transient/treeplan-omlx",
		},
	}
}

// TestTreePlanOverRealCorpus composes the mechanical tree plan a descent would
// produce over each pinned corpus if it grouped by file — one domain per
// document, one page per top-level section — and holds it to every composed
// post-condition at the shipped budgets.
//
// Grouping by file is not a claim about what the taxonomy stage will do. It is
// the plan that exercises the most machinery with no model in the loop:
// every surveyed section is covered, so the tiling check is live; every span
// is a real one, so the splitter runs over real material; and the fan-out cap
// bites on whichever documents have more top-level sections than the cap.
//
// One subtest per corpus, named for it, so an integration recipe can pin one
// (`-run 'TestTreePlanOverRealCorpus/omlx'`) and so an absent corpus skips only
// its own subtest.
func TestTreePlanOverRealCorpus(t *testing.T) {
	for _, c := range corpora() {
		t.Run(c.name, func(t *testing.T) { treePlanOverCorpus(t, c) })
	}
}

func treePlanOverCorpus(t *testing.T, c pinnedCorpus) {
	art, corpus := realArtifact(t, c)
	v, err := NewVerifier(art, corpus, DefaultParams(), log.Discard())
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	plan := planByFile(t, art, c)

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose over the pinned corpus: %v", err)
	}
	if err := v.Check(s); err != nil {
		t.Fatalf("the composed tree plan does not re-verify: %v", err)
	}

	n := countsOf(s)
	t.Logf("%s tree plan at the shipped budgets: %d nodes (%d leaves, %d indexes), "+
		"%d groups of which %d split into %d parts; %d source files, %d surveyed sections",
		c.name, n.Nodes, n.Leaves, n.Indexes, n.Groups, n.SplitGroups, n.SplitParts,
		art.Corpus.Files, art.Corpus.Sections)

	assertWithinCaps(t, s, DefaultBudgets())
	assertGroupsMatchTheSplitter(t, v, s)

	// Determinism over real material: the same corpus and the same parameters
	// produce the same tree, node for node.
	again, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("second Compose: %v", err)
	}
	for i := range s.Nodes {
		if s.Nodes[i] != again.Nodes[i] {
			t.Fatalf("node %d differs between runs:\n %+v\n %+v", i, s.Nodes[i], again.Nodes[i])
		}
	}
}

// TestTreePlanOverRealCorpusUnderStress is the same plan at leaf budgets small
// enough that a pinned corpus's own sections have to be split.
//
// The shipped budget leaves every Rojo section under one leaf, so the split
// path never runs over real material there. Here it does, against real cut
// candidates — headings, fences and paragraph starts a technical writer put
// where they wanted them, not where a fixture put them.
//
// The claim is the same shape as the dissector's own corpus property: Compose
// either produces a tree plan its own Check accepts, or refuses with a
// Rejection naming what the model would have to do differently. There is no
// third outcome — in particular, no defect, which would mean the verifier
// disagreed with itself over real bytes.
//
// The subtests nest corpus over budget, so a recipe pinning
// `-run 'TestTreePlanOverRealCorpus/omlx'` gets that corpus's whole sweep.
func TestTreePlanOverRealCorpusUnderStress(t *testing.T) {
	for _, c := range corpora() {
		t.Run(c.name, func(t *testing.T) { stressOverCorpus(t, c) })
	}
}

func stressOverCorpus(t *testing.T, c pinnedCorpus) {
	art, corpus := realArtifact(t, c)
	plan := planByFile(t, art, c)

	for _, budget := range stressLeafBudgets() {
		t.Run(fmt.Sprintf("leafTokens=%d", budget), func(t *testing.T) {
			v, s, out := sweepAt(t, art, corpus, plan, budget)
			if out.Rejection != nil {
				t.Logf("refused at leafTokens=%d: %s", budget, out.Rejection.Detail)
				return
			}
			if err := v.Check(s); err != nil {
				t.Fatalf("Compose produced a tree plan its own Check rejects: %v", err)
			}
			n := out.Counts
			t.Logf("leafTokens=%d: %d nodes (%d leaves, %d indexes), %d groups, %d split into %d parts",
				budget, n.Nodes, n.Leaves, n.Indexes, n.Groups, n.SplitGroups, n.SplitParts)
			assertWithinCaps(t, s, s.Budgets)
			assertGroupsMatchTheSplitter(t, v, s)
		})
	}
}

// stressLeafBudgets is the swept operating points: two well under any real
// section, one that bites on the larger ones, one just under the shipped
// budget. Both the stress property and the evidence emission read this list,
// so a point added to the sweep is a point that appears in both.
func stressLeafBudgets() []int { return []int{64, 200, 800, 2000} }

// sweepAt composes the plan at one leaf budget and classifies the outcome the
// way §3.3 says there are only two of: a tree plan, or a Rejection naming what
// the model would have to do differently. A DefectError is neither, and fails
// the test here rather than being recorded as evidence — the verifier
// disagreeing with itself over real bytes is not an observation, it is a bug.
//
// It is the single place both corpus sweeps ask that question: the stress
// property asserts over what it returns, the evidence emission records it.
func sweepAt(t *testing.T, art survey.Artifact, corpus ingest.Corpus, plan TreeProposal, leafTokens int) (*Verifier, TreePlan, sweepOutcome) {
	t.Helper()
	p := DefaultParams()
	p.Budgets.LeafTokens = leafTokens
	v, err := NewVerifier(art, corpus, p, log.Discard())
	if err != nil {
		t.Fatalf("NewVerifier at leafTokens=%d: %v", leafTokens, err)
	}

	s, err := v.Compose(plan, nil)
	if err == nil {
		c := countsOf(s)
		return v, s, sweepOutcome{LeafTokens: leafTokens, Outcome: "accepted", Counts: &c}
	}

	var defect DefectError
	if errors.As(err, &defect) {
		t.Fatalf("the verifier reported its own defect over real material: %v", err)
	}
	r, ok := AsRejection(err)
	if !ok {
		t.Fatalf("neither a tree plan nor a rejection: %T: %v", err, err)
	}
	rej := &rejectionEvidence{Kind: "structural", Note: r.Note(), Detail: err.Error()}
	var starved StarvedRejection
	if errors.As(err, &starved) {
		rej.Kind = "starved"
		rej.Span = &starvedSpan{
			Subject: starved.Subject,
			File:    starved.File,
			Start:   starved.Starved.Span.Start,
			End:     starved.Starved.Span.End,
			Tokens:  starved.Starved.Tokens,
			Budget:  starved.Starved.Budget,
		}
	}
	return v, TreePlan{}, sweepOutcome{LeafTokens: leafTokens, Outcome: "rejected", Rejection: rej}
}

// TestTreePlanOverRealCorpusEvidence writes down what the two properties above
// measured.
//
// The properties assert; this records. AGENTS.md's rule is that results which
// cannot be examined are not results, and the numbers in those tests' t.Logf
// lines are exactly the kind that get quoted once in a report and are never
// checkable again. Here the composed tree and the sweep land on disk under
// evidenceDir, in a shape a diff can be taken over.
//
// It re-composes rather than reading what the properties built: a test that
// depended on another test having run first is an ordering constraint go test
// does not promise, and the composition is milliseconds.
func TestTreePlanOverRealCorpusEvidence(t *testing.T) {
	for _, c := range corpora() {
		t.Run(c.name, func(t *testing.T) { evidenceOverCorpus(t, c) })
	}
}

func evidenceOverCorpus(t *testing.T, c pinnedCorpus) {
	art, corpus := realArtifact(t, c)
	plan := planByFile(t, art, c)

	v, err := NewVerifier(art, corpus, DefaultParams(), log.Discard())
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose over the pinned corpus: %v", err)
	}

	dir := evidencePath(t, c)
	var buf bytes.Buffer
	if err := s.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	writeEvidence(t, dir, "treeplan.json", buf.Bytes())

	ev := corpusEvidence{
		Corpus:  art.Corpus,
		Budgets: s.Budgets,
		Counts:  countsOf(s),
		Domains: domainsOf(s),
	}
	for _, budget := range stressLeafBudgets() {
		_, _, out := sweepAt(t, art, corpus, plan, budget)
		ev.Sweep = append(ev.Sweep, out)
	}
	writeEvidence(t, dir, "stats.json", encodeEvidence(t, ev))
}

// corpusEvidence is the stats file, and its field order is the file's field
// order. Nothing on this path is a map: an artifact that reorders itself
// between runs cannot be diffed, and a diff is the whole reason to keep it.
type corpusEvidence struct {
	Corpus  survey.Totals  `json:"corpus"`
	Budgets Budgets        `json:"budgets"`
	Counts  counts         `json:"counts"`
	Domains []domainFanOut `json:"domains"`
	Sweep   []sweepOutcome `json:"budgetSweep"`
}

// counts is the composed tree in six numbers.
type counts struct {
	Nodes   int `json:"nodes"`
	Leaves  int `json:"leaves"`
	Indexes int `json:"indexes"`
	Groups  int `json:"groups"`
	// SplitGroups is how many groups the splitter cut into more than one
	// leaf; SplitParts is how many leaves those groups became. Both, because
	// one group in twenty parts and twenty groups in two are the same
	// SplitParts and very different trees.
	SplitGroups int `json:"splitGroups"`
	SplitParts  int `json:"splitParts"`
}

// domainFanOut is one domain index's share of the tree: what the fan-out cap
// is actually up against on this corpus.
type domainFanOut struct {
	Path     string `json:"path"`
	Title    string `json:"title"`
	Children int    `json:"children"`
	Leaves   int    `json:"leaves"`
}

// sweepOutcome is one operating point of the budget sweep: §3.3's two legal
// outcomes and nothing else.
type sweepOutcome struct {
	LeafTokens int                `json:"leafTokens"`
	Outcome    string             `json:"outcome"`
	Counts     *counts            `json:"counts,omitempty"`
	Rejection  *rejectionEvidence `json:"rejection,omitempty"`
}

// rejectionEvidence is a refusal as evidence: both renderings, because the
// difference between them is a design claim (errors.go) and a file that
// carried only one could not show it.
type rejectionEvidence struct {
	Kind   string       `json:"kind"`
	Note   string       `json:"note"`
	Detail string       `json:"detail"`
	Span   *starvedSpan `json:"span,omitempty"`
}

// starvedSpan is the material a starved refusal names — which is exactly what
// a reader of this file wants next: WHICH bytes of the corpus were too big.
type starvedSpan struct {
	Subject string `json:"subject"`
	File    string `json:"file"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Tokens  int    `json:"tokens"`
	Budget  int    `json:"budget"`
}

// countsOf counts a composed tree. leafCount and indexCount are the fixture
// helpers the synthetic tests already ask with; asking differently here would
// be a second definition of "leaf".
func countsOf(s TreePlan) counts {
	c := counts{
		Nodes:   len(s.Nodes),
		Leaves:  leafCount(s),
		Indexes: indexCount(s),
		Groups:  len(s.Groups),
	}
	for _, g := range s.Groups {
		if g.Parts > 1 {
			c.SplitGroups++
			c.SplitParts += g.Parts
		}
	}
	return c
}

// domainsOf is the entry-point's children with their subtree sizes, in tree
// order — which is the artifact's own node order, so the slice is
// deterministic without a sort.
func domainsOf(s TreePlan) []domainFanOut {
	var root string
	for _, n := range s.Nodes {
		if n.Kind == KindEntryPoint {
			root = n.Path
			break
		}
	}
	children := map[string][]Node{}
	for _, n := range s.Nodes {
		children[n.Parent] = append(children[n.Parent], n)
	}
	var leavesUnder func(path string) int
	leavesUnder = func(path string) int {
		n := 0
		for _, c := range children[path] {
			if c.Kind == KindLeaf {
				n++
			}
			n += leavesUnder(c.Path)
		}
		return n
	}
	out := make([]domainFanOut, 0, len(children[root]))
	for _, d := range children[root] {
		out = append(out, domainFanOut{
			Path:     d.Path,
			Title:    d.Title,
			Children: len(children[d.Path]),
			Leaves:   leavesUnder(d.Path),
		})
	}
	return out
}

// encodeEvidence renders the stats with the artifact's own JSON discipline:
// HTML escaping off because titles come from documents, indented because a
// human reads this, and one trailing newline because it is a text file.
func encodeEvidence(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatalf("encode evidence: %v", err)
	}
	return buf.Bytes()
}

// evidencePath is the corpus's evidence directory resolved against the module
// root, since the test binary runs in the package directory.
func evidencePath(t *testing.T, c pinnedCorpus) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), filepath.FromSlash(c.evidence))
}

// writeEvidence fails the test when the evidence cannot be written. A test
// that quietly skipped its own record would leave the same empty directory as
// a test that never ran — which is the failure mode the rule exists to close.
func writeEvidence(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("make the evidence directory: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("evidence: %s (%d bytes)", path, len(data))
}

// planByFile is the mechanical grouping every corpus test composes, and it is
// the shipped dev/baseline helper rather than a test-local copy of it: the
// property is about the tree `kbase build` actually produces, so it has to be
// composed from the same proposal that verb composes.
func planByFile(t *testing.T, art survey.Artifact, c pinnedCorpus) TreeProposal {
	t.Helper()
	plan, err := SourceStructureProposal(art, c.title, c.scope, nil)
	if err != nil {
		t.Fatalf("SourceStructureProposal: %v", err)
	}
	return plan
}

// realArtifact surveys one pinned corpus, or skips.
func realArtifact(t *testing.T, c pinnedCorpus) (survey.Artifact, ingest.Corpus) {
	t.Helper()
	root := moduleRoot(t)
	docs := filepath.Join(root, filepath.FromSlash(c.docs))
	if _, err := os.Stat(docs); err != nil {
		t.Skipf("the pinned %s corpus is not present (%s); run `just %s`", c.name, c.docs, c.prep)
	}
	corpus, err := ingest.Walk(docs, markdown.Extensions(), log.Discard())
	if err != nil {
		t.Fatalf("ingest.Walk: %v", err)
	}
	art, err := markdown.Survey(corpus, tokens.Estimator{}, log.Discard())
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	return art, corpus
}

// moduleRoot walks up to the directory holding go.mod, so the corpus path is
// resolved against the module rather than against whatever directory the test
// binary was started in.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package under test")
		}
		dir = parent
	}
}
