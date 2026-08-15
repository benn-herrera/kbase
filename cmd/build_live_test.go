package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/treeplan"
)

// The verb's LIVE shape, driven by a mock model through the same composition
// the binary runs: the taxonomy descent designs the tree, the level stages
// write the summaries, and stage 8 renders their conclusions into the section
// pages.
//
// It is hermetic — the model is a scripted mock behind providerOptions'
// NewClient seam — and it is the only place the two new stages meet the
// mechanical spine before a real provider does.

// liveCorpus is shaped so every container call presents exactly TWO entries:
// two documents at the root, two top-level sections in each. That is what lets
// one scripted answer be a legal answer to every container without the test
// knowing the descent's internals.
const (
	liveDocOne = `# Alpha

The alpha section of document one.

# Beta

The beta section of document one.
`
	liveDocTwo = `# Gamma

The gamma section of document two.

# Delta

The delta section of document two.
`
)

func writeLiveCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{"one.md": liveDocOne, "two.md": liveDocTwo} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// liveScript is the scripted conversation: three container answers, then a
// summary for every section node. The taxonomy calls are one serial lane and
// come first, in order; the summaries follow, and are identical because their
// lanes fan out across workers and the order between two domains is not one.
func liveScript(t *testing.T) []model.Response {
	t.Helper()
	group := func(title string) string {
		data, err := json.Marshal(map[string]any{"groups": []map[string]any{{
			"title": title, "scope": "what a reader finds in " + title,
			"kind": "section", "members": []int{1, 2},
		}}})
		if err != nil {
			t.Fatalf("encode the grouping answer: %v", err)
		}
		return string(data)
	}
	summary, err := json.Marshal(map[string]string{
		"framing":            "This section holds the material below it.",
		"conclusionsHeading": "What it settles",
		"conclusions":        "Alpha and Beta are documented here.",
	})
	if err != nil {
		t.Fatalf("encode the summary answer: %v", err)
	}

	out := []model.Response{
		{Content: group("The Corpus"), FinishReason: "stop"},
		{Content: group("Document One"), FinishReason: "stop"},
		{Content: group("Document Two"), FinishReason: "stop"},
	}
	// Four section nodes get a summary: the entry-point, the domain the root's
	// group created, and the two the documents' groups created.
	for range 4 {
		out = append(out, model.Response{Content: string(summary), FinishReason: "stop"})
	}
	return out
}

func TestBuildLiveWritesTaxonomyAndSummaries(t *testing.T) {
	root := writeLiveCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	client := model.NewScriptedMockPerConsult(liveScript(t))

	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root: root,
		Out:  out,
		Live: &providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config: config.Config{
				Provider: "solo",
				Models:   config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"},
			},
			NewClient: func(model.Endpoint) model.Client { return client },
			Stderr:    &stderr,
		},
		BuildDate: pinnedBuildDate,
		Stdout:    &stdout,
		Stderr:    &stderr,
		Logger:    log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the live build did not deliver:\n%s", stdout.String())
	}

	// The tree is the model's, not the source's: the descent's own titles are
	// on the nodes, and no mechanical proposal produces them.
	for _, title := range []string{"The Corpus", "Document One", "Document Two"} {
		found := false
		for _, n := range res.Plan.Nodes {
			if n.Title == title {
				found = true
			}
		}
		if !found {
			t.Errorf("the tree plan holds no node the descent titled %q", title)
		}
	}

	// The conclusions block reached the delivered page, under the model's own
	// heading and above the down-link list.
	var section string
	for _, n := range res.Plan.Nodes {
		if n.Kind == treeplan.KindIndex {
			section = n.Path
			break
		}
	}
	page, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(section)))
	if err != nil {
		t.Fatalf("read the delivered section %s: %v", section, err)
	}
	body := string(page)
	for _, want := range []string{
		"This section holds the material below it.",
		"## What it settles",
		"Alpha and Beta are documented here.",
		"## Derivations and Detail",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the delivered section is missing %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "## What it settles") > strings.Index(body, "## Derivations and Detail") {
		t.Errorf("the conclusions block renders below the down-link list:\n%s", body)
	}

	// The run record carries what a live run dialed, asked and spent — and no
	// credential.
	data, err := os.ReadFile(filepath.Join(out, buildRecordName))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live == nil {
		t.Fatal("the run record carries no live block")
	}
	if rec.Live.Model != "gemma-4-31b" || rec.Live.Tier != config.TierHeavy {
		t.Errorf("live = %+v, want the heavy tier's configured model", rec.Live)
	}
	if rec.Live.TaxonomyCalls != 3 {
		t.Errorf("%d taxonomy calls recorded, want one per container", rec.Live.TaxonomyCalls)
	}
	if rec.Summaries != 4 {
		t.Errorf("%d summaries, want one per section node", rec.Summaries)
	}
	if strings.Contains(string(data), testAPIKey) {
		t.Error("the run record carries the API key")
	}
}

// The refined cuts stage, end to end.
//
// Rojo has no split group at the shipped budget and neither does the corpus
// above, which is the ordinary outcome over documentation-sized sections — so
// the only way to exercise stage 4's live shape is a corpus built to have
// some. writeSplitCorpus is that: two documents whose first section is
// comfortably over the leaf budget, so stage 3 sizes both their groups into
// parts and stage 4 has real interior boundaries to adjudicate in two lanes.

// splitPara is one paragraph of the oversized section: enough bytes that a
// handful of them cross the leaf budget, and a paragraph break after each, so
// the splitter has candidates to choose between and the menu has entries
// either side of every incumbent.
const splitPara = "The reconciler walks the project file and the live tree together, comparing " +
	"each declared property against the one the tree currently holds and applying only " +
	"the differences. A property the file does not name is left exactly as it was, which " +
	"is what makes a partial project file a legal one. Where a difference cannot be " +
	"applied the whole reconciliation is reported and abandoned rather than half-performed, " +
	"because a half-applied tree is harder to diagnose than one that never moved at all. "

// splitDoc is a document whose first top-level section runs well past the leaf
// budget, followed by a small one — so the group covering it is sized into
// several parts and the boundaries between them are stage 4's work.
func splitDoc(first, second string, paras int) string {
	var sb strings.Builder
	sb.WriteString("# " + first + "\n\n")
	for range paras {
		sb.WriteString(splitPara + "\n\n")
	}
	sb.WriteString("# " + second + "\n\nThe " + second + " section, which is short.\n")
	return sb.String()
}

// writeSplitCorpus writes TWO oversized documents, so the job has two split
// groups and stage 4 has two folds running in two lanes. One would exercise
// the fold; two are what exercise the stage — a response has to reach the fold
// that owns its span and no other.
func writeSplitCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "corpus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the corpus directory: %v", err)
	}
	for name, body := range map[string]string{
		"one.md": splitDoc("Alpha", "Beta", 100),
		"two.md": splitDoc("Gamma", "Delta", 60),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// stageFake answers each stage's question by RECOGNISING it rather than by
// counting calls.
//
// A live build asks three different things — group these entries, choose this
// boundary, summarise this section — and the summary stages fan their lanes
// out across workers, so a positional script would encode a call order the
// pipeline never promised. Recognition is the one thing the mock fabric
// cannot do (it walks a queue or re-serves one slot); everything below it,
// the SSE chunking ConsultDrained reads through included, is model's own.
type stageFake struct {
	grouping string
	boundary string
	summary  string

	mu sync.Mutex
	// models is the model id each kind of call went out under — the proof
	// that refinement resolved the LIGHT tier while the other two resolved
	// the heavy one.
	models map[string]string
	counts map[string]int
}

func newStageFake(grouping, boundary, summary string) *stageFake {
	return &stageFake{
		grouping: grouping, boundary: boundary, summary: summary,
		models: map[string]string{}, counts: map[string]int{},
	}
}

// answer classifies one request and returns what to say back. The markers are
// each stage's own rendered material: the refinement menu, and the summary
// definition's instruction.
func (f *stageFake) answer(req model.Request) (string, string) {
	var sb strings.Builder
	for _, m := range req.Messages {
		sb.WriteString(m.Content)
	}
	prompt := sb.String()
	switch {
	case strings.Contains(prompt, "Candidate positions:"):
		return stageCuts, f.boundary
	case strings.Contains(prompt, "Write that section's own summary"):
		return stageSummaries, f.summary
	default:
		return stageTreePlan, f.grouping
	}
}

func (f *stageFake) Consult(ctx context.Context, req model.Request) (model.Response, error) {
	if err := ctx.Err(); err != nil {
		return model.Response{}, err
	}
	kind, content := f.answer(req)
	f.mu.Lock()
	f.models[kind] = req.Model
	f.counts[kind]++
	f.mu.Unlock()
	return model.Response{Content: content, FinishReason: "stop"}, nil
}

func (f *stageFake) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	resp, err := f.Consult(ctx, req)
	if err != nil {
		return nil, err
	}
	// The real chunking, so the drain this pipeline actually performs is the
	// one under test rather than a second implementation of it.
	return model.NewScriptedMock([]model.Response{resp}, nil).ConsultStream(ctx, model.Request{})
}

func (f *stageFake) ListModels(context.Context) ([]model.ModelInfo, error) {
	return []model.ModelInfo{}, nil
}

func (f *stageFake) seen(kind string) (string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.models[kind], f.counts[kind]
}

// buildSplit runs a live build over the split corpus with the given boundary
// answer and returns the result and the run record.
func buildSplit(t *testing.T, boundary string) (buildResult, buildRun, string) {
	t.Helper()
	root := writeSplitCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	group, err := json.Marshal(map[string]any{"groups": []map[string]any{{
		"title": "A Section", "scope": "what a reader finds here",
		"kind": "section", "members": []int{1, 2},
	}}})
	if err != nil {
		t.Fatalf("encode the grouping answer: %v", err)
	}
	summary, err := json.Marshal(map[string]string{
		"framing":            "This section holds the material below it.",
		"conclusionsHeading": "What it settles",
		"conclusions":        "The reconciler is documented here.",
	})
	if err != nil {
		t.Fatalf("encode the summary answer: %v", err)
	}
	client := newStageFake(string(group), boundary, string(summary))

	var stdout, stderr bytes.Buffer
	res, err := runBuild(context.Background(), buildOptions{
		Root: root,
		Out:  out,
		Live: &providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config: config.Config{
				Provider: "solo",
				Models:   config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"},
			},
			NewClient: func(model.Endpoint) model.Client { return client },
			Stderr:    &stderr,
		},
		BuildDate: pinnedBuildDate,
		Stdout:    &stdout,
		Stderr:    &stderr,
		Logger:    log.Discard(),
	})
	if err != nil {
		t.Fatalf("runBuild: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the live build did not deliver:\n%s", stdout.String())
	}

	// The light tier answered the boundary calls and the heavy one everything
	// else. Nothing else in this test could tell a tier mix-up from a working
	// build: both models are the same mock.
	if id, calls := client.seen(stageCuts); id != "gemma-4-26b-a4b" || calls == 0 {
		t.Errorf("the boundary calls went to %q over %d calls, want the light tier's model", id, calls)
	}
	if id, _ := client.seen(stageTreePlan); id != "gemma-4-31b" {
		t.Errorf("the taxonomy calls went to %q, want the heavy tier's model", id)
	}

	data, err := os.ReadFile(filepath.Join(out, buildRecordName))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live == nil {
		t.Fatal("the run record carries no live block")
	}
	// Two split groups and more than one boundary in the job: a single-fold,
	// single-boundary run would pass every assertion below while proving
	// nothing about the routing between folds or the serial scan inside one.
	if rec.SplitGroups < 2 || rec.Live.BoundariesAdjudicated < 2 {
		t.Fatalf("the corpus produced %d split groups and %d boundaries; the stage under test needs several of each",
			rec.SplitGroups, rec.Live.BoundariesAdjudicated)
	}
	return res, rec, stdout.String()
}

// TestBuildLiveRefinesCuts: the model's choice is what the delivered pages are
// cut at.
//
// Answering "1" takes the earliest candidate the menu offers, which is behind
// the mechanical cut in every window with a candidate to spare — so a run that
// moved every boundary is a run whose answers were mapped back to offsets,
// verified against the working list, and composed, rather than one that fell
// through to the fallback and looked the same from outside.
func TestBuildLiveRefinesCuts(t *testing.T) {
	res, rec, stdout := buildSplit(t, "1")

	if rec.Live.BoundariesFellBack != 0 || rec.Live.BoundaryRejections != 0 {
		t.Errorf("live = %+v, want every boundary adjudicated cleanly:\n%s", rec.Live, stdout)
	}
	if rec.Live.BoundariesMoved != rec.Live.BoundariesAdjudicated {
		t.Errorf("%d of %d boundaries moved; the model's choice was not taken",
			rec.Live.BoundariesMoved, rec.Live.BoundariesAdjudicated)
	}
	if rec.Live.LightModel != "gemma-4-26b-a4b" {
		t.Errorf("lightModel = %q, want the configured light tier", rec.Live.LightModel)
	}

	// The split groups' parts are on disk as separate pages, and each fold
	// adjudicated one boundary fewer than its group has parts.
	split := map[string]bool{}
	for _, g := range res.Plan.Groups {
		if g.Parts > 1 {
			split[g.ID] = true
		}
	}
	parts := 0
	for _, n := range res.Plan.Nodes {
		if n.Kind == treeplan.KindLeaf && split[n.SplitGroup] {
			parts++
		}
	}
	if want := rec.Live.BoundariesAdjudicated + rec.SplitGroups; parts != want {
		t.Errorf("%d split-group pages against %d boundaries over %d groups, want %d",
			parts, rec.Live.BoundariesAdjudicated, rec.SplitGroups, want)
	}
}

// TestBuildLiveFallsBackOnRefusedBoundaries: refinement is a fallback-backed
// seam, so a model that never answers the question costs quality and never the
// delivery.
//
// Every boundary here is answered with prose rather than a menu number, which
// parseChoice rejects; the informed retry gets the same, and the boundary then
// keeps the mechanical cut. The gates still pass and the tree is still
// delivered — which is the whole claim ARCHITECTURE §3's monotone-safety rule
// makes about this seam.
func TestBuildLiveFallsBackOnRefusedBoundaries(t *testing.T) {
	_, rec, stdout := buildSplit(t, "I would put the boundary a little later")

	if rec.Live.BoundariesFellBack != rec.Live.BoundariesAdjudicated {
		t.Errorf("%d of %d boundaries fell back; an answer that is not a menu number cannot be taken:\n%s",
			rec.Live.BoundariesFellBack, rec.Live.BoundariesAdjudicated, stdout)
	}
	if rec.Live.BoundariesMoved != 0 {
		t.Errorf("%d boundaries moved on answers nothing verified", rec.Live.BoundariesMoved)
	}
	// Two model attempts per boundary, both rejected: the first and the one
	// informed retry (pipeline's declared policy, dissect.Retry).
	if want := 2 * rec.Live.BoundariesAdjudicated; rec.Live.BoundaryRejections != want {
		t.Errorf("%d rejections over %d boundaries, want %d — one per attempt",
			rec.Live.BoundaryRejections, rec.Live.BoundariesAdjudicated, want)
	}
}

// TestBuildRefusesAnUnmappedTier: a live build needs BOTH tiers, and says
// which one is missing before it reads the corpus.
//
// Both are essential to a delivered tree in different ways — the heavy tier
// designs it and summarises it, the light tier decides where its pages are cut
// — so an unmapped one is refused at the same moment for the same reason: a
// misconfigured run must cost no tokens, and half a build is not a build. The
// client factory fails the test if it is ever reached, which is what makes
// "before anything is dialed" an assertion rather than a claim.
func TestBuildRefusesAnUnmappedTier(t *testing.T) {
	for _, tc := range []struct {
		name   string
		models config.ModelMap
		want   string
	}{
		{"no light tier", config.ModelMap{Heavy: "gemma-4-31b"}, config.TierLight},
		{"no heavy tier", config.ModelMap{Light: "gemma-4-26b-a4b"}, config.TierHeavy},
		{"neither tier", config.ModelMap{}, config.TierHeavy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, err := runBuild(context.Background(), buildOptions{
				Root: writeLiveCorpus(t),
				Out:  filepath.Join(t.TempDir(), "kb"),
				Live: &providerOptions{
					Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
					Config:    config.Config{Provider: "solo", Models: tc.models},
					NewClient: func(model.Endpoint) model.Client {
						t.Error("a build with an unmapped tier dialed a provider")
						return model.NewScriptedMock(nil, nil)
					},
					Stderr: &stderr,
				},
				BuildDate: pinnedBuildDate,
				Stdout:    &stdout,
				Stderr:    &stderr,
				Logger:    log.Discard(),
			})
			if err == nil {
				t.Fatal("a live build with an unmapped tier must refuse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to name the %s tier", err, tc.want)
			}
		})
	}
}

// TestBuildWithoutConfigDirMakesNoCall is the other half of the switch: the
// mechanical shape must stay mechanical. A client that fails on every call
// proves it — the build succeeds because nothing ever asks it anything.
func TestBuildWithoutConfigDirMakesNoCall(t *testing.T) {
	root := writeLiveCorpus(t)
	out := filepath.Join(t.TempDir(), "kb")
	res, stdout := build(t, root, out)
	if !res.Report.Passed() || !res.Job.DeliveryReady() {
		t.Fatalf("the mechanical build did not deliver:\n%s", stdout)
	}
	data, err := os.ReadFile(filepath.Join(out, buildRecordName))
	if err != nil {
		t.Fatalf("read the run record: %v", err)
	}
	var rec buildRun
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode the run record: %v", err)
	}
	if rec.Live != nil {
		t.Errorf("a run with no model in the loop reported %+v", rec.Live)
	}
	if rec.Summaries != 0 {
		t.Errorf("%d summaries were written with no model in the loop", rec.Summaries)
	}
}
