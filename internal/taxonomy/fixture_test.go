package taxonomy

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

// The synthetic corpus and the harness this package's tests drive the seam
// through: Coordinator → CallRunner → model.MockClient, with nothing stubbed
// between them. What is being tested is the seam's behaviour under the
// pipeline's own policies — one informed retry, then the unit fails, because
// there is no fallback — and a stub would be a second implementation of
// exactly the rules in question.
//
// The model side is a mock and the prompt text is a stub (see stubDefinition):
// the definition and its eval belong to the embedded-definitions burst, and the
// seam does not depend on the wording.

const (
	// stageName is the stage under test; also its lane and the directory its
	// artifact lands in, so a failure names one thing rather than three.
	stageName = "treeplan"
	// planUnit is the artifact the descent owes.
	planUnit = "treeplan/treeplan.json"
	// jobFrame is slot 1 for these runs.
	jobFrame = "Job: design a tree for the fixture corpus."
)

// testEffort stands in for the definition's declaration. Thinking is ON here,
// deliberately opposite to what the package declares (Effort): these tests
// assert that the descent passes its caller's value through, which a fixture
// agreeing with the default could not show.
var testEffort = model.DeclareEffort(model.RequestEffort{Thinking: true})

type secSpec struct {
	title string
	words int
}

type docSpec struct {
	path  string
	title string
	secs  []secSpec
}

// buildCorpus turns document specs into the pair the descent takes: the survey
// artifact and the source under custody. Every fixture goes through
// survey.Assemble, so offsets that do not tile fail here as loudly as an
// adapter's would in production.
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

// testBudgets is a small operating point: §9's arithmetic three orders of
// magnitude down, so a fixture that exercises a budget is a few hundred bytes.
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

// fixtureDocs is the corpus most of these tests run over: a folder of two
// documents beside a document at the root. It is the smallest shape that
// exercises all three container kinds and two levels of descent.
func fixtureDocs() []docSpec {
	return []docSpec{
		{path: "guide/one.md", title: "Document One", secs: []secSpec{
			{"Alpha", 40}, {"Beta", 40}, {"Gamma", 40}}},
		{path: "guide/two.md", title: "Document Two", secs: []secSpec{
			{"Delta", 40}, {"Epsilon", 40}}},
		{path: "intro.md", title: "Introduction", secs: []secSpec{
			{"Purpose", 40}, {"Audience", 40}}},
	}
}

// designerFor builds the descent under test over a corpus.
func designerFor(t *testing.T, lg log.Logger, budgets treeplan.Budgets, docs ...docSpec) *Designer {
	t.Helper()
	art, corpus := buildCorpus(t, docs...)
	params := treeplan.Params{Budgets: budgets}
	v, err := treeplan.NewVerifier(art, corpus, params)
	if err != nil {
		t.Fatalf("treeplan.NewVerifier: %v", err)
	}
	d, err := New(Design{
		Survey:   art,
		Verifier: v,
		Params:   params,
		Title:    "Fixture knowledge base",
		Scope:    "everything built from the fixture corpus",
		Unit:     planUnit,
		Effort:   testEffort,
	}, lg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

// runDescent runs the stage over a fresh job directory through the real
// coordinator, and returns the result with the directory it wrote into.
func runDescent(t *testing.T, d *Designer, client model.Client, lg log.Logger) (pipeline.JobResult, string, error) {
	t.Helper()
	dir := t.TempDir()
	res, err := runDescentIn(t, dir, d, client, lg)
	return res, dir, err
}

// runDescentIn is runDescent over a job directory the caller owns — a resume
// runs twice over one directory, which is the whole point of it.
func runDescentIn(t *testing.T, dir string, d *Designer, client model.Client, lg log.Logger) (pipeline.JobResult, error) {
	t.Helper()
	cfg := config.Config{Models: config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"}}
	work, err := pipeline.OpenTempWork(dir, lg)
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	coord := pipeline.NewCoordinator(work.ArtifactStore(),
		pipeline.NewCallRunner(client, cfg, lg), 1, lg)
	plan := pipeline.Plan{
		JobFrame: jobFrame,
		Stages: []*pipeline.StagePlan{d.StagePlan(stageName, []pipeline.Input{
			{Name: "corpus", Hash: pipeline.HashBytes([]byte("fixture"))},
		})},
	}
	return coord.Run(t.Context(), plan, pipeline.ModeResume)
}

// planIn reads the composed artifact back through the store, the way the
// stages that consume it will.
func planIn(t *testing.T, dir string) treeplan.TreePlan {
	t.Helper()
	work, err := pipeline.OpenTempWork(dir, log.Discard())
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	data, err := work.ArtifactStore().Get(planUnit)
	if err != nil {
		t.Fatalf("read %s: %v", planUnit, err)
	}
	plan, err := treeplan.ReadJSON(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("the composed artifact does not decode: %v", err)
	}
	return plan
}

// perCandidate is the reference answer: one group per entry, a `section` where
// the entry holds entries of its own and a `page` where it does not. It is what
// a model that agreed with the source's own organisation would say.
func perCandidate(a *ask) answer {
	var out answer
	for i, c := range a.cands {
		kind := kindPage
		if len(c.kids) > 0 {
			kind = kindSection
		}
		out.Groups = append(out.Groups, answerGroup{
			Title:   c.title,
			Scope:   "what a reader finds in " + c.title,
			Kind:    kind,
			Members: []int{i + 1},
		})
	}
	return out
}

// script renders one response per container call, in the order the descent
// asks them.
func script(d *Designer, answerFor func(*ask) answer) []model.Response {
	out := make([]model.Response, 0, len(d.asks))
	for _, a := range d.asks {
		out = append(out, model.Response{Content: mustJSON(answerFor(a)), FinishReason: "stop"})
	}
	return out
}

func mustJSON(a answer) string {
	data, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// indexPaths is every section node of a plan, in artifact order.
func indexPaths(plan treeplan.TreePlan) []string {
	var out []string
	for _, n := range plan.Nodes {
		if n.Kind != treeplan.KindLeaf {
			out = append(out, n.Path)
		}
	}
	return out
}
