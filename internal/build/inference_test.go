package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/asks"
	"kbase/internal/asks/call"
	"kbase/internal/buildrecords"
	"kbase/internal/claimgraph"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/result"
)

// askedPaper's prose states one result in a paragraph of its own, recalls
// notation in another, and points at the theorem in a third; its lemma names
// the theorem whose proof rests on the lemma, so classifying both pairs as
// supported-by closes a ring.
const askedPaper = `\documentclass{article}
\newtheorem{theorem}{Theorem}
\newtheorem{lemma}{Lemma}
\title{A Small Paper}
\author{A. Author}
\begin{document}
\maketitle
\begin{abstract}
We prove a small theorem.
\end{abstract}
\section{Introduction}
We show that every widget is a gadget under mild conditions.

This section recalls the notation used throughout the paper.

The main result, Theorem~\ref{thm:main}, closes the argument in the next section.
\section{Results}
\begin{lemma}\label{lem:base}
Every widget is a gadget, as Theorem~\ref{thm:main} shows.
\end{lemma}
\begin{theorem}\label{thm:main}
Every gadget is a widget.
\end{theorem}
\begin{proof}
Apply Lemma~\ref{lem:base}.
\end{proof}
\end{document}
`

// answering is a provider answering each request from its state and question
// alone, so concurrent asks are answered alike whatever order they arrive in.
type answering struct {
	mu     sync.Mutex
	calls  int
	asked  map[string]int
	answer func(system, prompt string) string
}

func newAnswering(answer func(system, prompt string) string) *answering {
	return &answering{answer: answer, asked: map[string]int{}}
}

func (a *answering) Consult(context.Context, model.Request) (model.Response, error) {
	return model.Response{}, errors.New("the build asks over the stream")
}

func (a *answering) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	a.mu.Lock()
	a.calls++
	a.mu.Unlock()
	return a.reply(ctx, req)
}

func (a *answering) reply(ctx context.Context, req model.Request) (model.StreamReader, error) {
	reply := model.Response{Content: a.answer(req.Messages[0].Content, req.Messages[1].Content),
		Usage: model.Usage{PromptTokens: 100, CompletionTokens: 1}, UsageReported: true}
	return model.NewScriptedMock([]model.Response{reply}, nil).ConsultStream(ctx, req)
}

func (a *answering) ListModels(context.Context) ([]model.ModelInfo, error) { return nil, nil }

// SystemOne answers the letter asks: the state and the question's key name
// the ask, and the answer is a confidence over the offered letters keyed by
// their criteria — a noul question's true-letter taking the noul figure and
// the false letter the remainder. Each answer is one figure the script names
// per ask, so Decide returns the letter the script holds. A figure outside
// zero to one is a malformed answer, the seam's unreadable path.
func (a *answering) SystemOne(_ context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
	a.mu.Lock()
	a.calls++
	q := req.Questions[0]
	state, key := req.State, q.Key
	n := a.asked[key]
	a.asked[key] = n + 1
	a.mu.Unlock()
	var out model.DecisionAnswer
	switch q.Type {
	case "noul":
		out = model.DecisionAnswer{Key: key, Type: "noul", Noul: a.pTrue(state, key, n)}
	case "choice":
		// Every co-linearity ask is answered a step of its own, so no claim is
		// culled; every other choice is answered A.
		choice := "A"
		if q.Instructions == asks.ColinearAsks([]asks.ColinearItem{{}})[0].Question.Instructions {
			choice = asks.LetterDistinct
		}
		probabilities := map[string]float64{}
		for l := range q.Criteria {
			if l == choice {
				probabilities[l] = 0.9
			} else {
				probabilities[l] = 0.05
			}
		}
		out = model.DecisionAnswer{Key: key, Type: "choice", Choice: choice, Probabilities: probabilities}
	}
	return model.DecisionResponse{Answers: []model.DecisionAnswer{out}}, nil
}

// pTrue is the scripted P(true) of a noul ask, by the ask the state names —
// keyed on the slot values a state carries, never on template wording. The
// nth ask of an item takes the nth figure; a script that runs out keeps the
// last, so an unreadable item stays unreadable and defaults.
//
// A paragraph ask holds its paragraph's text twice, in the document and as
// the paragraph; the result-stating paragraph is a claim, the notation
// paragraph is unreadable once and then not a claim, the pointing paragraph
// is unreadable twice. An unmarked ask holds both claims' text; the lemma and
// the prose claim each point at the other and nothing else points.
func (a *answering) pTrue(state, key string, n int) float64 {
	asked := func(paragraph string) bool { return strings.Count(state, paragraph) == 2 }
	var figures []float64
	switch {
	case asked("We show that every widget"):
		figures = []float64{0.99}
	case asked("recalls the notation"):
		figures = []float64{2, 0.01}
	case asked("closes the argument"):
		figures = []float64{2, 2}
	case strings.Contains(state, "Every widget is a gadget, as") && strings.Contains(state, "under mild conditions"):
		figures = []float64{0.99}
	default:
		figures = []float64{0.01}
	}
	return figures[min(n, len(figures)-1)]
}

func (a *answering) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

// script answers the paper's overview asks: the README passage the overview
// call composes, keyed on the excerpt text a prompt carries, never on
// template wording.
func script(system, prompt string) string {
	const heading = "# Widgets and gadgets"
	if strings.Contains(prompt, heading) {
		return "A small paper showing that widgets and gadgets coincide."
	}
	return heading + "\nA small paper."
}

func (f fixture) asking(client model.Client) Options {
	opts := f.options()
	opts.NoInference = false
	opts.Provider = func() (Provider, error) { return Provider{Client: client, Letters: "decision", Overview: "text"}, nil }
	return opts
}

func TestBuildAsksThroughTheLetterSeam(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), askedPaper)
	provider := newAnswering(script)
	outcome, fields := f.build(t, f.asking(provider))
	if outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}

	record, ok, err := buildrecords.ReadNodePass(kb.OnDisk(filepath.Join(f.repo, kb.KBDir)))
	if err != nil || !ok {
		t.Fatalf("node-pass record: %t, %v", ok, err)
	}
	intro := record.Leaves["a-small-paper/introduction.md"]
	var judged []string
	for _, v := range intro.Verdicts {
		j := v.Verdict
		if v.Cause != nil {
			j += "/" + *v.Cause
		}
		judged = append(judged, j)
	}
	if want := []string{claimgraph.JudgementClaim, claimgraph.JudgementNotAClaim, claimgraph.JudgementDefaulted + "/" + claimgraph.CauseNoLetter}; intro.State != claimgraph.ReadLanded || !slices.Equal(judged, want) {
		t.Errorf("introduction = %s %q, want landed with verdicts %q", intro.State, judged, want)
	}
	const title = "We show that every widget is a gadget under mild conditions."
	if len(intro.Claims) != 1 || intro.Claims[0].Title != title {
		t.Errorf("planned claims = %+v, want one titled by its paragraph's sentence", intro.Claims)
	}
	files := snapshot(t, f.repo)
	if !strings.Contains(files["kb-root/a-small-paper/claim-quality.md"], title) {
		t.Error("the prose claim reached no register")
	}
	if !strings.Contains(files["kb-root/a-small-paper/introduction.md"], "under mild conditions. <!-- claim-quality: clm-") {
		t.Errorf("the prose claim's paragraph carries no marker:\n%s", files["kb-root/a-small-paper/introduction.md"])
	}

	unmarked, _, err := buildrecords.ReadUnmarked(kb.OnDisk(filepath.Join(f.repo, kb.KBDir)))
	if err != nil || unmarked.Planned == nil || len(*unmarked.Planned) != 2 || len(unmarked.Pairs) != 2 {
		t.Fatalf("unmarked record: %+v, %v; want two pairs planned and asked — the lemma and the prose claim are the one pair no reference or adjacency reaches, each in the other's paper and so in its proximity slots, and each source is planned apart", unmarked, err)
	}
	yeses := map[[2]string]bool{}
	for _, p := range unmarked.Pairs {
		if p.Letter != nil && *p.Letter == asks.LetterPoints {
			yeses[[2]string{p.Source, p.Target}] = true
		}
	}
	var yes buildrecords.CandidateEntry
	for _, p := range unmarked.Pairs {
		if strings.Contains(files["kb-root/a-small-paper/introduction.md"], p.Target) {
			yes = p
		}
	}
	if len(yeses) != 2 || yes.Target == "" || !yeses[[2]string{yes.Target, yes.Source}] {
		t.Errorf("unmarked pairs = %+v, want the lemma's to the prose claim and back, both answered A", unmarked.Pairs)
	}

	cull, _, err := buildrecords.ReadCull(kb.OnDisk(filepath.Join(f.repo, kb.KBDir)))
	if err != nil || len(cull.Entries) != 2 || len(cull.Culls) != 0 {
		t.Fatalf("cull record: %+v, %v; want two pass-1 asks — the unmarked link and the marked link with a prose end — and no cull", cull, err)
	}
	for _, e := range cull.Entries {
		if e.Verdict != buildrecords.VerdictDistinct || e.Outcome != claimgraph.ClassifyAnswered {
			t.Errorf("cull entry %+v, want answered distinct", e)
		}
	}

	classified, _, err := buildrecords.ReadClassification(kb.OnDisk(filepath.Join(f.repo, kb.KBDir)))
	if err != nil || len(classified.Candidates) == 0 {
		t.Fatalf("classification record: %+v, %v", classified, err)
	}
	var linked []buildrecords.CandidateEntry
	for _, c := range classified.Candidates {
		if c.Letter == nil || *c.Letter != asks.LetterSupportedBy || c.Outcome != claimgraph.ClassifyAnswered {
			t.Errorf("candidate %s -> %s = %+v, want answered A", c.Source, c.Target, c)
		}
		if yeses[[2]string{c.Source, c.Target}] {
			linked = append(linked, c)
		}
	}
	threeWay := []string{asks.LetterSupportedBy, asks.LetterInSupportOf, asks.LetterMention}
	if len(linked) != 1 || linked[0].Source != yes.Source || linked[0].Target != yes.Target || !slices.Equal(linked[0].Offered, threeWay) {
		t.Errorf("the lemma–prose link's candidates = %+v, want one, the lemma to the prose claim before it, offered %q", linked, threeWay)
	}
	if demoted := dependsField(t, fields, "stage-D-ring", "demoted"); len(demoted) != 4 {
		t.Errorf("demoted = %q, want all four depends edges: the lemma and the theorem each way, the lemma to the prose claim, and the prose claim to the theorem, every one on a cycle", demoted)
	}

	if readme := files["kb-root/README.md"]; !strings.Contains(readme, "A small paper showing that widgets and gadgets coincide.") {
		t.Errorf("README.md = %q, want the re-asked passage", readme)
	}

	_, status := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	fallbacks, _ := field(status, "recent-fallbacks").([]result.Item)
	if len(fallbacks) != 1 || fallbacks[0].Path != "a-small-paper/introduction.md" || fallbacks[0].Line == 0 {
		t.Errorf("recent fallbacks = %+v, want the pointing paragraph that took its draft", fallbacks)
	}
	events, _ := readProgress(f.state)
	var leaves []string
	for _, e := range events {
		if e.Event == eventUnit && e.Stage == "claims-discovered" && e.Unit != "discover.build" {
			leaves = append(leaves, e.Unit)
		}
	}
	if !slices.Contains(leaves, "a-small-paper/introduction.md") {
		t.Errorf("claims-discovered's units = %q, want one per leaf", leaves)
	}
	var groups []string
	for _, e := range events {
		if e.Event == eventUnit && e.Stage == "references-found" && e.Unit != "unmarked.build" {
			groups = append(groups, e.Unit)
		}
	}
	if len(groups) != 2 || !slices.Contains(groups, yes.Source) || !slices.Contains(groups, yes.Target) {
		t.Errorf("references-found's units = %q, want one per source group asked: the lemma's and the prose claim's", groups)
	}
	scratch := filepath.Join(f.state, scratchDir)
	for _, dir := range []string{call.CapturesDir, call.AnswersDir, asks.AskRecordsDir} {
		if entries, err := os.ReadDir(filepath.Join(scratch, dir)); err != nil || len(entries) == 0 {
			t.Errorf("%s/%s holds nothing: %v", scratchDir, dir, err)
		}
	}

	// A second build of the same paper lands the same KB modulo node ids.
	g := newFixture(t)
	writeFile(t, filepath.Join(g.repo, "paper.tex"), askedPaper)
	if outcome, fields := g.build(t, g.asking(newAnswering(script))); outcome != result.Done {
		t.Fatalf("rebuild = %s %v", outcome, fields)
	}
	sameModuloIDs(t, g.repo, f.repo)
}

// claimgraphFindings is the claim-graph findings the report of stage holds,
// the stage found among the result's stages.
func claimgraphFindings(t *testing.T, fields []result.Field, stage string) []map[string]any {
	t.Helper()
	stages, _ := field(fields, "stages").([]result.Record)
	for _, s := range stages {
		if s[0].Value != stage {
			continue
		}
		b, err := os.ReadFile(s[3].Value.(string))
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Stage string `yaml:"stage"`
			Rows  []struct {
				Claimgraph []map[string]any `yaml:"claimgraph"`
			} `yaml:"rows"`
		}
		if err := yaml.Unmarshal(b, &report); err != nil || report.Stage != stage {
			t.Fatalf("%s's report %s: %v", stage, s[3].Value, err)
		}
		var out []map[string]any
		for _, r := range report.Rows {
			out = append(out, r.Claimgraph...)
		}
		return out
	}
	t.Fatalf("no stage %s in %v", stage, fields)
	return nil
}

// dependsField is a list field of depends-attributed's finding named check.
func dependsField(t *testing.T, fields []result.Field, check, key string) []any {
	t.Helper()
	for _, finding := range claimgraphFindings(t, fields, "depends-attributed") {
		if finding["check"] == check {
			v, _ := finding[key].([]any)
			return v
		}
	}
	t.Fatalf("no %s %s in %v", check, key, fields)
	return nil
}

// settledRingPaper's two lemmas' proofs each cite the other, so containment
// directs both pairs and they close a two-cycle.
const settledRingPaper = "\\documentclass{article}\n\\newtheorem{lemma}{Lemma}\n\\title{A Ring}\n\\begin{document}\n\\maketitle\n" +
	"\\section{Results}\n" +
	"\\begin{lemma}\\label{lem:a}\nEvery widget is a gadget.\n\\end{lemma}\n\\begin{proof}\nBy Lemma~\\ref{lem:a2}.\n\\end{proof}\n" +
	"\\begin{lemma}\\label{lem:a2}\nEvery gadget is a widget.\n\\end{lemma}\n\\begin{proof}\nBy Lemma~\\ref{lem:a}.\n\\end{proof}\n" +
	"\\end{document}\n"

// TestNoInferenceCutsTheSettledRing: without inference the two settled pairs
// of a ring leave no depends edge between the lemmas, and each stands as a
// demoted row of its own source, origin cited.
func TestNoInferenceCutsTheSettledRing(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), settledRingPaper)
	if outcome, fields := f.build(t, f.options()); outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	st, err := kb.Discover(kb.OnDisk(filepath.Join(f.repo, kb.KBDir)), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	var ids, depends, demoted []string
	for _, e := range st.ClaimEntries {
		ids = append(ids, e.ID)
		for _, d := range e.DependsOn {
			if d.Relation == kb.RelationDepends && d.TargetKind == kb.NodeKindClaim {
				depends = append(depends, e.ID+" -> "+d.Target)
			}
		}
		for _, d := range e.Demoted {
			demoted = append(demoted, e.ID+" -> "+d.Target+" "+d.Origin)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("claim entries = %q, want the two lemmas", ids)
	}
	if len(depends) != 0 {
		t.Errorf("depends edges = %q, want none", depends)
	}
	slices.Sort(demoted)
	want := []string{ids[0] + " -> " + ids[1] + " " + kb.OriginCited, ids[1] + " -> " + ids[0] + " " + kb.OriginCited}
	slices.Sort(want)
	if !slices.Equal(demoted, want) {
		t.Errorf("demoted = %q, want %q", demoted, want)
	}
}

func TestACallThatNeverCompletesStopsTheStage(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), askedPaper)
	mock := model.NewScriptedMockPerConsult(nil)
	mock.SetError(errors.New("connection refused"))
	outcome, fields := f.build(t, f.asking(mock))
	if outcome != result.Failed {
		t.Fatalf("build = %s %v, want failed", outcome, fields)
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("claims-declared")) {
		t.Errorf("trail = %q, want every stage before the node pass and not the node pass", got)
	}
	stages := field(fields, "stages").([]result.Record)
	last := stages[len(stages)-1]
	if last[0].Value != "claims-discovered" || last[1].Value != nil {
		t.Fatalf("stopped in %v, commit %v", last[0].Value, last[1].Value)
	}
	findings := claimgraphFindings(t, fields, "claims-discovered")
	stop := findings[len(findings)-1]
	if detail, _ := stop["detail"].(string); stop["status"] != "FAIL" || stop["check"] != "inference-failed" || !strings.Contains(detail, "connection refused") {
		t.Errorf("the stage's last finding = %v, want inference-failed naming the cause", stop)
	}
	if failures, _ := field(fields, "failures").([]result.Item); len(failures) != 1 || failures[0].Check != "inference-failed" || !strings.Contains(failures[0].Detail, "connection refused") {
		t.Errorf("failures = %+v, want the stopping check named", failures)
	}
	if _, err := os.Stat(filepath.Join(f.repo, kb.KBDir, "README.md")); err == nil {
		t.Error("a build whose node pass stopped wrote the overview")
	}
}

// stopsAfter answers its first n requests as answer does and fails every
// one after, as a provider lost mid-stage. The check and the count are one
// step over both transports, so concurrent asks cannot both take the last
// answer.
type stopsAfter struct {
	answering
	n      int
	made   int
	failed bool
}

func (s *stopsAfter) step() bool {
	s.mu.Lock()
	over := s.made >= s.n
	if !over {
		s.made++
	}
	s.mu.Unlock()
	return over
}

func (s *stopsAfter) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	if s.step() {
		return nil, errors.New("connection refused")
	}
	return s.reply(ctx, req)
}

func (s *stopsAfter) SystemOne(ctx context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
	if s.step() {
		return model.DecisionResponse{}, errors.New("connection refused")
	}
	return s.answering.SystemOne(ctx, req)
}

// TestInterruptedAskingStageResumesFromTheAnswerCache: a build whose provider
// is lost inside classification — the node pass's and the unmarked asks
// answered, some source's group recorded and not every one — leaves the stage
// unrecorded and its writes uncommitted; the resume restores them, asks the
// provider only what the answer cache does not hold, and lands what the
// uninterrupted build lands.
//
// The node pass and the unmarked asks take 8 answers — among them the
// result-stating paragraph answered at once, the notation paragraph
// unreadable once and then not a claim, the pointing paragraph unreadable
// twice and defaulted, and the unmarked pairs, the lemma's to the prose claim
// and back, asked once each — the cull takes 2, one per link with a prose
// end, and classification asks three groups of one candidate each — the
// lemma's, the prose claim's and the theorem's, both unmarked yeses landing
// unasked — in ascending id. Twelve answers complete two groups and never the
// third.
func TestInterruptedAskingStageResumesFromTheAnswerCache(t *testing.T) {
	whole := newFixture(t)
	writeFile(t, filepath.Join(whole.repo, "paper.tex"), askedPaper)
	clean := newAnswering(script)
	if outcome, fields := whole.build(t, whole.asking(clean)); outcome != result.Done {
		t.Fatalf("uninterrupted build = %s %v", outcome, fields)
	}

	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), askedPaper)
	// n = 12: the thirteenth letter ask never arrives — the third classify
	// group of one candidate each; the call fails past SystemOne's own attempt
	// count and the stage stops unrecorded.
	lost := &stopsAfter{answering: *newAnswering(script), n: 12}
	if outcome, fields := f.build(t, f.asking(lost)); outcome != result.Failed {
		t.Fatalf("build losing its provider = %s %v, want failed", outcome, fields)
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("claims-culled")) {
		t.Fatalf("trail = %q, want dependency attribution unrecorded", got)
	}
	answers := filepath.Join(f.state, scratchDir, call.AnswersDir)
	cached, _ := os.ReadDir(answers)
	if len(cached) == 0 {
		t.Fatal("the interrupted stage cached no answer")
	}

	resumed := newAnswering(script)
	outcome, fields := f.build(t, f.asking(resumed))
	if outcome != result.Done {
		t.Fatalf("resume = %s %v", outcome, fields)
	}
	if restored, _ := field(fields, "restored").([]string); len(restored) == 0 {
		t.Error("the resume restored nothing of the interrupted stage")
	}
	if want := clean.count() - len(cached); resumed.count() != want {
		t.Errorf("the resume asked %d times, want %d: every ask but the %d the cache holds", resumed.count(), want, len(cached))
	}
	sameModuloIDs(t, f.repo, whole.repo)
}

func TestBibliographiesBesideTheVolumeRoots(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"z.bib", "a.bib", ".hidden.bib", "notes.txt", "two/m.bib"} {
		writeFile(t, filepath.Join(dir, name), "")
	}
	w := &walk{opts: Options{VolumeRoots: []string{filepath.Join(dir, "paper.tex")}}}
	got, err := w.bibliographies()
	if want := []string{filepath.Join(dir, "a.bib"), filepath.Join(dir, "z.bib")}; err != nil || !slices.Equal(got, want) {
		t.Errorf("bibliographies = %q, %v; want %q", got, err, want)
	}
	w.opts.VolumeRoots = []string{filepath.Join(dir, "two", "other.tex"), filepath.Join(dir, "paper.tex"), filepath.Join(dir, "third.tex")}
	got, err = w.bibliographies()
	if want := []string{filepath.Join(dir, "a.bib"), filepath.Join(dir, "two", "m.bib"), filepath.Join(dir, "z.bib")}; err != nil || !slices.Equal(got, want) {
		t.Errorf("bibliographies over three roots in two directories = %q, %v; want every one beside any root, once, sorted: %q", got, err, want)
	}
	w.opts.Bibliographies = []string{"given.bib"}
	if got, _ := w.bibliographies(); !slices.Equal(got, []string{"given.bib"}) {
		t.Errorf("given bibliographies = %q, want them alone", got)
	}
}
