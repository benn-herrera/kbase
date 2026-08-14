package summarize

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/survey"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// The harness: a real tree plan over a synthetic corpus, real page artifacts in
// a real store, and the level stages driven through the REAL coordinator —
// Coordinator → CallRunner → model.MockClient. What is being tested is the
// level chain and the no-fallback seam under the pipeline's own policies, and a
// stub orchestrator would be a second implementation of the rules in question.

const (
	stageName    = "summaries"
	leavesDir    = "leaves"
	summariesDir = "summaries"
	jobFrame     = "Job: summarise the fixture knowledge base."
)

var testEffort = model.DeclareEffort(model.RequestEffort{Thinking: false})

// testBudgets is §9's arithmetic scaled down, so a fixture that exercises G-2
// is a few hundred bytes rather than a megabyte.
func testBudgets() treeplan.Budgets {
	return treeplan.Budgets{
		LeafTokens:         400,
		SummaryInputTokens: 4000,
		SummaryTokens:      50,
		EntryPointTokens:   300,
		DepthCap:           4,
		FanOutCap:          6,
		CandidateCap:       10,
	}
}

// scene is one job's worth of state: the tree the summaries are over, the store
// their inputs live in, and the summarizer under test.
type scene struct {
	plan  treeplan.TreePlan
	store *pipeline.ArtifactStore
	sum   *Summarizer
	dir   string
}

// newScene builds the tree plan from the source's own structure — two
// documents, so the tree is an entry-point over two sections over their pages —
// and writes every page artifact the summaries will read.
func newScene(t *testing.T, lg log.Logger, body func(treeplan.Node) string) *scene {
	t.Helper()
	art, corpus := buildCorpus(t,
		docSpec{path: "one.md", title: "Document One", secs: []secSpec{{"Alpha", 40}, {"Beta", 40}}},
		docSpec{path: "two.md", title: "Document Two", secs: []secSpec{{"Gamma", 40}, {"Delta", 40}}},
	)
	params := treeplan.Params{Budgets: testBudgets()}
	v, err := treeplan.NewVerifier(art, corpus, params)
	if err != nil {
		t.Fatalf("treeplan.NewVerifier: %v", err)
	}
	proposal, err := treeplan.SourceStructureProposal(art, "Fixture knowledge base", "the fixture corpus")
	if err != nil {
		t.Fatalf("SourceStructureProposal: %v", err)
	}
	plan, err := v.Compose(proposal, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	dir := t.TempDir()
	work, err := pipeline.OpenTempWork(dir, lg)
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	store := work.ArtifactStore()
	for _, n := range plan.Nodes {
		if n.Kind != treeplan.KindLeaf {
			continue
		}
		if err := store.Put(leavesDir+"/"+n.Path, []byte(body(n)), nil); err != nil {
			t.Fatalf("write the page %s: %v", n.Path, err)
		}
	}

	s, err := New(Job{
		TreePlan:     func() (treeplan.TreePlan, error) { return plan, nil },
		Store:        store,
		LeavesDir:    leavesDir,
		SummariesDir: summariesDir,
		Params:       params,
		Effort:       testEffort,
	}, lg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &scene{plan: plan, store: store, sum: s, dir: dir}
}

// pageBody is the default page artifact: enough text to be recognisable in a
// prompt, tagged with the node it came from.
func pageBody(n treeplan.Node) string {
	return fmt.Sprintf("# %s\n\nThe page body of %s. %s\n", n.Title, n.Title, filler("body", 20))
}

// run drives the level stages through the coordinator, one worker, so the call
// order is the level order and a test may read the calls in it.
func (sc *scene) run(t *testing.T, client model.Client, lg log.Logger) (pipeline.JobResult, error) {
	t.Helper()
	cfg := config.Config{Models: config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"}}
	coord := pipeline.NewCoordinator(sc.store, pipeline.NewCallRunner(client, cfg, lg), 1, lg)
	plan := pipeline.Plan{
		JobFrame: jobFrame,
		Stages: sc.sum.StagePlans(stageName, func(unit string, upstreams []string) pipeline.OwedArtifact {
			return pipeline.OwedArtifact{
				Path:      unit,
				Inputs:    []pipeline.Input{{Name: "corpus", Hash: pipeline.HashBytes([]byte("fixture"))}},
				Upstreams: upstreams,
			}
		}),
	}
	return coord.Run(t.Context(), plan, pipeline.ModeResume)
}

// summaryIn reads one node's artifact back through the store — the way stage 8
// and the level above both get at it.
func (sc *scene) summaryIn(t *testing.T, nodePath string) Summary {
	t.Helper()
	s, ok, err := Read(sc.store, summariesDir, nodePath)
	if err != nil {
		t.Fatalf("read the summary of %s: %v", nodePath, err)
	}
	if !ok {
		t.Fatalf("no summary was written for %s", nodePath)
	}
	return s
}

// indexNodes is every node a summary is owed for, in tree-plan order.
func (sc *scene) indexNodes() []treeplan.Node {
	var out []treeplan.Node
	for _, n := range sc.plan.Nodes {
		if n.Kind != treeplan.KindLeaf {
			out = append(out, n)
		}
	}
	return out
}

// answer renders one scripted summary response.
func answer(framing, heading, conclusions string) model.Response {
	data, err := json.Marshal(Summary{Framing: framing, ConclusionsHeading: heading, Conclusions: conclusions})
	if err != nil {
		panic(err)
	}
	return model.Response{Content: string(data), FinishReason: "stop"}
}

// --- the synthetic corpus, the same shape every package here builds.

type secSpec struct {
	title string
	words int
}

type docSpec struct {
	path  string
	title string
	secs  []secSpec
}

func buildCorpus(t *testing.T, docs ...docSpec) (survey.Artifact, ingest.Corpus) {
	t.Helper()
	units := make([]ingest.SourceDoc, 0, len(docs))
	files := make([]survey.File, 0, len(docs))
	for _, d := range docs {
		u, f := buildDoc(d)
		units = append(units, u)
		files = append(files, f)
	}
	corpus, err := ingest.New(units)
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	sorted := make([]survey.File, 0, len(files))
	for _, u := range corpus.Docs {
		for _, f := range files {
			if f.Path == u.Path {
				sorted = append(sorted, f)
			}
		}
	}
	art, err := survey.Assemble(corpus, sorted, log.Discard())
	if err != nil {
		t.Fatalf("survey.Assemble: %v", err)
	}
	return art, corpus
}

func buildDoc(d docSpec) (ingest.SourceDoc, survey.File) {
	var sb strings.Builder
	var sections []survey.Section
	var cands []survey.CutCandidate
	est := tokens.Estimator{}

	for i, s := range d.secs {
		start := sb.Len()
		if i > 0 {
			cands = append(cands, survey.CutCandidate{Offset: start, Kind: survey.CutHeading})
		}
		fmt.Fprintf(&sb, "# %s\n\n", s.title)
		cands = append(cands, survey.CutCandidate{Offset: sb.Len(), Kind: survey.CutParagraph})
		sb.WriteString(filler(strings.ToLower(s.title[:1]), s.words))
		sb.WriteString("\n\n")
		sections = append(sections, survey.Section{
			Level:  1,
			Title:  s.title,
			Start:  start,
			End:    sb.Len(),
			Tokens: est.Estimate(sb.String()[start:sb.Len()]),
			Gist:   "gist of " + s.title,
		})
	}
	src := []byte(sb.String())
	return ingest.SourceDoc{Path: d.path, Bytes: src}, survey.File{
		Path:     d.path,
		Bytes:    len(src),
		Tokens:   est.EstimateBytes(src),
		Title:    d.title,
		Sections: sections,
		Cuts:     cands,
	}
}

func filler(tag string, n int) string {
	parts := make([]string, 0, n)
	for i := range n {
		parts = append(parts, fmt.Sprintf("%s%02d", tag, i%100))
	}
	return strings.Join(parts, " ")
}
