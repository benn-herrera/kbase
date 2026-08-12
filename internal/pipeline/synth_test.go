package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/prompt"
)

// A synthetic two-stage pipeline: mock transport, invented roles, invented
// verifiers, no real prompts and no real stage implementations. What is under
// test is the ORCHESTRATION — phases, seams, retries, resume — and a real
// stage would only add content whose correctness is a different question.

// stubClient is the pipeline's test transport: one function decides what each
// consult returns, and every prompt is recorded.
//
// model.MockClient covers a scripted single response or a fixed queue, and
// neither shape fits here. The retry tests need the SAME call to fail and then
// succeed, which a sticky SetError cannot express; the concurrent harness needs
// a response derived from the request rather than from a position in a queue
// that several workers race to advance. The streaming half still goes through
// MockClient's reader, so the drain path under test is the shipped one.
type stubClient struct {
	mu      sync.Mutex
	calls   int
	prompts []string
	respond func(n int, req model.Request) (model.Response, error)
}

func (c *stubClient) Consult(ctx context.Context, req model.Request) (model.Response, error) {
	if err := ctx.Err(); err != nil {
		return model.Response{}, err
	}
	c.mu.Lock()
	c.calls++
	n := c.calls
	if len(req.Messages) > 0 {
		c.prompts = append(c.prompts, req.Messages[0].Content)
	}
	c.mu.Unlock()
	return c.respond(n, req)
}

func (c *stubClient) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	resp, err := c.Consult(ctx, req)
	if err != nil {
		return nil, err
	}
	return model.NewScriptedMock([]model.Response{resp}, nil).ConsultStream(ctx, req)
}

func (c *stubClient) ListModels(ctx context.Context) ([]model.ModelInfo, error) {
	return nil, nil
}

func (c *stubClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *stubClient) recorded() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.prompts...)
}

// echoStub answers every call with a digest of the prompt it was sent. That
// makes each unit's artifact a function of that unit's prompt — so a resumed
// run reproduces byte-identical output only if it rebuilt the same prompts, and
// two units can never be confused for one another.
func echoStub() *stubClient {
	return &stubClient{respond: func(_ int, req model.Request) (model.Response, error) {
		sum := sha256.Sum256([]byte(req.Messages[0].Content))
		return model.Response{
			Content:      synthAccept + " " + hex.EncodeToString(sum[:8]),
			FinishReason: "stop",
			Usage:        model.Usage{PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110, CachedPromptTokens: 40},
		}, nil
	}}
}

// synthAccept is the token the synthetic verifier requires. It stands in for a
// real mechanical post-condition (a cut list that tiles, a schema that parses):
// cheap to check, impossible to satisfy by accident.
const synthAccept = "OK"

// synthArtifact is what the synthetic verifier produces — a typed value, not
// the model's text, so the "raw model text never escapes the runner" claim has
// something to be true of.
type synthArtifact struct{ Text string }

func synthVerify(_, response string) (any, error) {
	if !strings.HasPrefix(response, synthAccept+" ") {
		return nil, fmt.Errorf("response does not start with %q", synthAccept)
	}
	return synthArtifact{Text: response}, nil
}

func synthEncode(artifact any) ([]byte, error) {
	a, ok := artifact.(synthArtifact)
	if !ok {
		return nil, fmt.Errorf("artifact is %T, not a synthArtifact", artifact)
	}
	return []byte(a.Text + "\n"), nil
}

// synthBaseline is the mechanical result a refinement seam falls back to.
const synthBaselineText = "MECHANICAL BASELINE"

func synthBaseline(string) any { return synthArtifact{Text: synthBaselineText} }

func synthDef(t *testing.T, body string) prompt.Definition {
	t.Helper()
	def, err := prompt.ParseDefinition(body)
	if err != nil {
		t.Fatalf("ParseDefinition: %v", err)
	}
	return def
}

// synthThinking and synthNoThinking are the two declared efforts the synthetic
// roles ask with. They differ so a test can tell which role's declaration
// reached the wire; that a heavy-tier role thinks and a light-tier one does not
// is this fixture's convention, not a rule the pipeline knows.
var (
	synthThinking   = model.DeclareEffort(model.Effort{Thinking: true})
	synthNoThinking = model.DeclareEffort(model.Effort{Thinking: false})
)

// essentialRole has no baseline: a failure fails the unit.
func essentialRole(t *testing.T) Role {
	t.Helper()
	return Role{
		Def:    synthDef(t, "# Surveyor\n\n## CRITICAL\n\nEmit only the inventory.\n"),
		Tier:   config.TierHeavy,
		Effort: synthThinking,
		Verify: synthVerify,
		Encode: synthEncode,
	}
}

// refinementRole has one: a failure keeps the mechanical result and degrades.
func refinementRole(t *testing.T) Role {
	t.Helper()
	r := essentialRole(t)
	r.Def = synthDef(t, "# Refiner\n\n## CRITICAL\n\nChoose from the listed candidates only.\n")
	r.Tier = config.TierLight
	r.Effort = synthNoThinking
	r.Baseline = synthBaseline
	return r
}

// synthFrame is the job-constant slot 1 every synthetic plan renders. It is a
// job-level value now, so the stage specs below carry only slot 3.
const synthFrame = "Job: survey the synthetic corpus."

func synthSpec(taskDef string) prompt.StageSpec {
	return prompt.StageSpec{TaskDef: taskDef}
}

// synthJobFrame is the canonical job frame the runner-level tests build their
// agents against, since they construct an Agent without going through Run.
func synthJobFrame(t *testing.T) jobFrame {
	t.Helper()
	frame, err := newJobFrame(synthFrame)
	if err != nil {
		t.Fatalf("newJobFrame: %v", err)
	}
	return frame
}

// staticStreams is the StreamResolver a stage whose work is known up front
// uses. Most stages are this; the dynamic resolution exists for the ones that
// are not (see synthDerivedPlan).
func staticStreams(streams ...DomainStream) StreamResolver {
	return func() ([]DomainStream, error) { return streams, nil }
}

// synthConfig maps both tiers, since a role that names an unmapped tier is a
// configuration failure and not the thing under test.
func synthConfig() config.Config {
	return config.Config{Models: config.ModelMap{Heavy: "gemma-4-31b", Light: "gemma-4-26b-a4b"}}
}

// synthConfigWithout drops one tier's mapping, for the case where a role names
// a tier the deployment never configured.
func synthConfigWithout(t *testing.T, tier string) config.Config {
	t.Helper()
	cfg := synthConfig()
	switch tier {
	case config.TierHeavy:
		cfg.Models.Heavy = ""
	case config.TierLight:
		cfg.Models.Light = ""
	default:
		t.Fatalf("unknown tier %q", tier)
	}
	return cfg
}

// corpusInput is the source-identity hash every synthetic unit derives from.
var corpusInput = Input{Name: "corpus", Hash: HashBytes([]byte("synthetic corpus"))}

// synthTask builds one task. The content is unique per path, so every unit's
// prompt — and therefore every unit's artifact — is its own.
func synthTask(path, section string, upstreams ...string) Task {
	return Task{
		Unit: Unit{Path: path, Inputs: []Input{corpusInput}, Upstreams: upstreams},

		Section:    section,
		SectionRef: "Cross-file listing for " + section + ":\n- one.md\n- two.md",
		Input: ConstInput(prompt.CallInput{
			StatusLines:        []string{"Section: " + section, "Unit: " + path},
			Content:            "Source span for " + path + ": the quick brown fox.",
			RefB:               "Prior unit ended at " + path,
			AcceptanceCriteria: []string{"- answer with " + synthAccept},
		}),
	}
}

// The fold stage's names. Its stream is the multi-call-artifact shape, so its
// three tasks share one unit directory and only the last of them is a unit.
const (
	synthFoldStage   = "fold"
	synthFoldSection = "span"
	synthFoldUnit    = synthFoldStage + "/cutlist.txt"
	// synthFoldCalls is how many calls that stream makes for its one unit.
	synthFoldCalls = 3
)

// synthFoldPaths are the fold stream's tasks in order: two boundary calls
// that write nothing, then the one that carries the composed artifact.
var synthFoldPaths = []string{
	synthFoldStage + "/0001.boundary",
	synthFoldStage + "/0002.boundary",
	synthFoldUnit,
}

// synthFoldPlan is the multi-call-artifact shape (Task.CallOnly) as a stage:
// [CallOnly, CallOnly, producing] over ONE piece of stage state, which is
// stage 4's boundary fold seen from the orchestration side.
//
// The state is a closure the stage's verifier folds every response into, and
// every task's prompt renders it. So the producing task's prompt — and
// therefore the artifact echoStub digests out of it — is a function of ALL
// THREE calls. Byte-identical output after a kill then means the resumed run
// really re-made every call of the stream, which is the property
// filterStreams owes and which no per-unit comparison could show.
func synthFoldPlan(t *testing.T) *StagePlan {
	t.Helper()
	role := refinementRole(t)
	inner := role.Verify
	folded := ""
	role.Verify = func(unit, response string) (any, error) {
		artifact, err := inner(unit, response)
		if err != nil {
			return nil, err
		}
		folded = HashBytes([]byte(folded + response))[:8]
		return artifact, nil
	}

	tasks := make([]Task, 0, len(synthFoldPaths))
	for i, path := range synthFoldPaths {
		task := synthTask(path, synthFoldSection)
		task.CallOnly = i < len(synthFoldPaths)-1
		task.Input = func() prompt.CallInput {
			in := synthTask(path, synthFoldSection).Input()
			in.Content = fmt.Sprintf("%s [fold state %s]", in.Content, folded)
			return in
		}
		tasks = append(tasks, task)
	}
	return &StagePlan{
		Name:    synthFoldStage,
		Role:    role,
		Spec:    synthSpec("Task: adjudicate each boundary of the span."),
		Streams: staticStreams(DomainStream{Domain: synthFoldStage, Tasks: tasks}),
	}
}

// synthPlan is the three-stage fixture: an essential stage of two units in one
// domain; a fold stage whose three calls produce one artifact; then a
// refinement stage of four units across two domains — one of which spans two
// sections, so the section-transition phase is crossed both within a stream
// and at its start.
//
// The fold sits in the middle rather than at the end, which is where a real
// one sits (§4) and which keeps the last stage the one a resume test poisons.
func synthPlan(t *testing.T) Plan {
	t.Helper()
	return Plan{
		SystemFrame: synthFrame,
		Stages: []*StagePlan{
			{
				Name: "survey",
				Role: essentialRole(t),
				Spec: synthSpec("Task: inventory each domain."),
				Streams: staticStreams(DomainStream{
					Domain: "corpus",
					Tasks: []Task{
						synthTask("survey/a.json", "all"),
						synthTask("survey/b.json", "all"),
					},
				}),
			},
			synthFoldPlan(t),
			{
				Name: "leaves",
				Role: refinementRole(t),
				Spec: synthSpec("Task: refine each leaf boundary."),
				Streams: staticStreams(
					DomainStream{Domain: "d1", Tasks: []Task{
						synthTask("leaves/d1/one.md", "s1", "survey/a.json"),
						synthTask("leaves/d1/two.md", "s2", "survey/a.json"),
					}},
					DomainStream{Domain: "d2", Tasks: []Task{
						synthTask("leaves/d2/one.md", "s1", "survey/b.json"),
						synthTask("leaves/d2/two.md", "s1", "survey/b.json"),
					}},
				),
			},
		},
	}
}

// synthUnits is how many units synthPlan describes, and synthCalls how many
// model calls one complete run of it makes. They differ because the fold
// spends synthFoldCalls calls on one unit.
const (
	synthUnits = 7
	synthCalls = synthUnits + synthFoldCalls - 1
)

// synthDerivedPlan is the dynamic chain made concrete: stage 2's units do not
// exist until stage 1 has run, because their PATHS are derived from the bytes
// stage 1 wrote.
//
// This is the shape stages 4–9 have (ARCHITECTURE.md §4): the skeleton stage 3
// emits defines stage 4's leaves, and stage 4's verified cut list defines stage
// 5's. Nothing in the plan could name those units at job setup, which is why
// the resolver runs when the stage is reached.
//
// The derivation reads the artifact, so a resolver that ran before stage 1 —
// or a resume that resolved it from a store the stage had not repopulated —
// fails loudly rather than quietly producing an empty stage.
func synthDerivedPlan(t *testing.T, dir string) Plan {
	t.Helper()
	const upstream = "survey/a.json"
	return Plan{
		SystemFrame: synthFrame,
		Stages: []*StagePlan{
			{
				Name: "survey",
				Role: essentialRole(t),
				Spec: synthSpec("Task: inventory each domain."),
				Streams: staticStreams(DomainStream{
					Domain: "corpus",
					Tasks:  []Task{synthTask(upstream, "all")},
				}),
			},
			{
				Name: "leaves",
				Role: refinementRole(t),
				Spec: synthSpec("Task: refine each leaf boundary."),
				Streams: func() ([]DomainStream, error) {
					data, err := os.ReadFile(filepath.Join(storeRoot(dir), filepath.FromSlash(upstream)))
					if err != nil {
						return nil, fmt.Errorf("derive leaves from %s: %w", upstream, err)
					}
					// One leaf per derived name, named for the upstream's
					// content: the same upstream bytes give the same unit
					// paths, and different bytes give different ones, so a
					// resume that reused stage 1 and a run that rebuilt it
					// agree only if stage 1 really is the same artifact.
					id := HashBytes(data)[:8]
					var tasks []Task
					for _, n := range []string{"one", "two"} {
						tasks = append(tasks, synthTask("leaves/"+id+"/"+n+".md", "s1", upstream))
					}
					return []DomainStream{{Domain: id, Tasks: tasks}}, nil
				},
			},
		},
	}
}

// synthDerivedUnits is how many units synthDerivedPlan describes once both
// stages have resolved.
const synthDerivedUnits = 3

// newSynthCoordinator wires a coordinator over an OUTPUT dir and a client.
//
// dir is what an operator would pass as --out, and the store is rooted where
// a real run roots it: the temp-work tree kbase creates inside it. Every
// helper below reads the same place through storeRoot, so no test can
// accidentally assert against a layout production does not use.
func newSynthCoordinator(t *testing.T, dir string, client model.Client, workers int, lg log.Logger) *Coordinator {
	t.Helper()
	work, err := OpenTempWork(dir, lg)
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	return NewCoordinator(work.Store(), NewCallRunner(client, synthConfig(), lg), workers, lg)
}

// storeRoot is where a run under the output dir keeps its artifacts, stamps
// and lockfile.
func storeRoot(dir string) string { return filepath.Join(dir, TempWorkDirName) }

// synthStore opens the store of an output dir the way a test that inspects
// one does — same root the coordinator wrote through.
func synthStore(t *testing.T, dir string, lg log.Logger) *Store {
	t.Helper()
	work, err := OpenTempWork(dir, lg)
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	return work.Store()
}

// storeState reads an output directory's store back as a path→bytes map — the
// thing two runs must agree on byte for byte. Paths are relative to the store
// root, so they read as the chain's own unit paths.
//
// Two kinds of file are excluded, and for the same reason: they are not the
// job's output. The lockfile is process bookkeeping, and a temp file is the
// residue a killed write leaves behind (with a random name, so it could not
// compare equal anyway). A resumed run is required to produce the same
// ARTIFACTS as an uninterrupted one, not the same litter.
func storeState(t *testing.T, dir string) map[string]string {
	t.Helper()
	root := storeRoot(dir)
	state := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		name := d.Name()
		if name == LockFileName || strings.Contains(name, tempSuffix) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		state[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("read job dir: %v", err)
	}
	return state
}

// assertSameState reports every difference rather than the first, because the
// useful question after a resume is "what did it get wrong", not "did it".
func assertSameState(t *testing.T, want, got map[string]string, context string) {
	t.Helper()
	for path, wantData := range want {
		gotData, ok := got[path]
		switch {
		case !ok:
			t.Errorf("%s: %s is missing", context, path)
		case gotData != wantData:
			t.Errorf("%s: %s differs:\n want %q\n got  %q", context, path, wantData, gotData)
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("%s: %s is not in the uninterrupted run's output", context, path)
		}
	}
}
