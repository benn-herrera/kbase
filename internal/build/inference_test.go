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
	"kbase/internal/claimgraph"
	"kbase/internal/kb"
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

// answering is a provider answering each request from its prompt alone, so
// concurrent asks are answered alike whatever order they arrive in.
type answering struct {
	mu     sync.Mutex
	calls  int
	answer func(system, prompt string) string
}

func (a *answering) Consult(context.Context, model.Request) (model.Response, error) {
	return model.Response{}, errors.New("the build asks over the stream")
}

func (a *answering) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	a.mu.Lock()
	a.calls++
	a.mu.Unlock()
	reply := model.Response{Content: a.answer(req.Messages[0].Content, req.Messages[1].Content),
		Usage: model.Usage{PromptTokens: 100, CompletionTokens: 1}, UsageReported: true}
	return model.NewScriptedMock([]model.Response{reply}, nil).ConsultStream(ctx, req)
}

func (a *answering) ListModels(context.Context) ([]model.ModelInfo, error) { return nil, nil }

func (a *answering) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

// script answers the paper's asks: the result-stating paragraph is a claim,
// the notation paragraph is unreadable once and then not a claim, the
// pointing paragraph is unreadable twice; every candidate is supported-by;
// the overview passage holds a heading once.
func script(system, prompt string) string {
	reader, _ := asks.System(asks.ReaderSystem)
	if system != reader {
		if strings.Contains(prompt, "could not be used") {
			return "A small paper showing that widgets and gadgets coincide."
		}
		return "# Overview\nA small paper."
	}
	question := prompt[strings.LastIndex(prompt, "## The question"):]
	reask := strings.Contains(question, "could not be used")
	switch {
	case strings.Contains(question, "Does the source need the candidate"):
		return "A"
	case strings.Contains(question, "We show that every widget"):
		return "A"
	case strings.Contains(question, "recalls the notation"):
		if reask {
			return "B"
		}
		return "Probably not."
	case strings.Contains(question, "closes the argument"):
		return "It might."
	}
	return "B"
}

func (f fixture) asking(client model.Client) Options {
	opts := f.options()
	opts.NoInference = false
	opts.Provider = func() (Provider, error) { return Provider{Client: client, Letters: "light", Overview: "heavy"}, nil }
	return opts
}

func TestBuildAsksThroughTheLetterSeam(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), askedPaper)
	provider := &answering{answer: script}
	outcome, fields := f.build(t, f.asking(provider))
	if outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}

	record, ok, err := claimgraph.ReadNodePass(f.repo)
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

	classified, _, err := claimgraph.ReadClassification(f.repo)
	if err != nil || len(classified.Candidates) == 0 {
		t.Fatalf("classification record: %+v, %v", classified, err)
	}
	for _, c := range classified.Candidates {
		if c.Letter == nil || *c.Letter != asks.LetterSupportedBy || c.Outcome != claimgraph.ClassifyAnswered {
			t.Errorf("candidate %s -> %s = %+v, want answered A", c.Source, c.Target, c)
		}
	}
	if demoted := classifyField(t, fields, "demoted"); len(demoted) != 2 {
		t.Errorf("demoted = %q, want the lemma-theorem ring both ways", demoted)
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
	scratch := filepath.Join(f.state, scratchDir)
	for _, dir := range []string{call.CapturesDir, call.AnswersDir, asks.AskRecordsDir} {
		if entries, err := os.ReadDir(filepath.Join(scratch, dir)); err != nil || len(entries) == 0 {
			t.Errorf("%s/%s holds nothing: %v", scratchDir, dir, err)
		}
	}

	// A second build of the same paper lands the same KB modulo node ids.
	g := newFixture(t)
	writeFile(t, filepath.Join(g.repo, "paper.tex"), askedPaper)
	if outcome, fields := g.build(t, g.asking(&answering{answer: script})); outcome != result.Done {
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

// classifyField is a field of depends-attributed's stage-D-classify finding.
func classifyField(t *testing.T, fields []result.Field, key string) []any {
	t.Helper()
	for _, finding := range claimgraphFindings(t, fields, "depends-attributed") {
		if finding["check"] == "stage-D-classify" {
			v, _ := finding[key].([]any)
			return v
		}
	}
	t.Fatalf("no stage-D-classify %s in %v", key, fields)
	return nil
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
// one after, as a provider lost mid-stage.
type stopsAfter struct {
	answering
	n int
}

func (s *stopsAfter) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	s.mu.Lock()
	over := s.calls >= s.n
	s.mu.Unlock()
	if over {
		return nil, errors.New("connection refused")
	}
	return s.answering.ConsultStream(ctx, req)
}

// TestInterruptedAskingStageResumesFromTheAnswerCache: a build whose provider
// is lost inside classification — the node pass's asks answered, the first
// source's group recorded, the second's never — leaves the stage
// unrecorded and its writes uncommitted; the resume restores them, asks the
// provider only what the answer cache does not hold, and lands what the
// uninterrupted build lands.
func TestInterruptedAskingStageResumesFromTheAnswerCache(t *testing.T) {
	whole := newFixture(t)
	writeFile(t, filepath.Join(whole.repo, "paper.tex"), askedPaper)
	clean := &answering{answer: script}
	if outcome, fields := whole.build(t, whole.asking(clean)); outcome != result.Done {
		t.Fatalf("uninterrupted build = %s %v", outcome, fields)
	}

	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), askedPaper)
	lost := &stopsAfter{answering: answering{answer: script}, n: 7}
	if outcome, fields := f.build(t, f.asking(lost)); outcome != result.Failed {
		t.Fatalf("build losing its provider = %s %v, want failed", outcome, fields)
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("equations-minted")) {
		t.Fatalf("trail = %q, want dependency attribution unrecorded", got)
	}
	answers := filepath.Join(f.state, scratchDir, call.AnswersDir)
	cached, _ := os.ReadDir(answers)
	if len(cached) == 0 {
		t.Fatal("the interrupted stage cached no answer")
	}

	resumed := &answering{answer: script}
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

func TestBibliographiesBesideTheVolumeRoot(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"z.bib", "a.bib", ".hidden.bib", "notes.txt"} {
		writeFile(t, filepath.Join(dir, name), "")
	}
	w := &walk{opts: Options{VolumeRoot: filepath.Join(dir, "paper.tex")}}
	got, err := w.bibliographies()
	if want := []string{filepath.Join(dir, "a.bib"), filepath.Join(dir, "z.bib")}; err != nil || !slices.Equal(got, want) {
		t.Errorf("bibliographies = %q, %v; want %q", got, err, want)
	}
	w.opts.Bibliographies = []string{"given.bib"}
	if got, _ := w.bibliographies(); !slices.Equal(got, []string{"given.bib"}) {
		t.Errorf("given bibliographies = %q, want them alone", got)
	}
}
