package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/dissect"
	"kbase/internal/model"
	"kbase/internal/pipeline"
)

// The dev-refine verb tested as WIRING: that the composition root assembles
// ingest → survey → split → refiner → coordinator correctly, and that its
// refusals happen before anything is dialed. What the seam then does with a
// response — accept, reject and retry, fall back — is internal/dissect's own
// suite, driven through the same real orchestrator; asserting it again here
// would be a second copy of those expectations with a CLI in front of it.

// devRefineBody is one section's worth of filler: comfortably over the
// 64-token minimum and under a small budget, so each heading below is a cut
// the splitter can actually take.
const devRefineBody = "The sync mechanism reconciles the project file against the live tree. " +
	"Every property the file names is applied in order, and every property it does not name is left alone. " +
	"A reconciliation that cannot be applied is reported rather than partially performed, " +
	"because a half-applied tree is harder to diagnose than one that never moved. "

// devRefineDoc is a document with three headed sections, each around a hundred
// tokens — so a small budget yields two interior boundaries with real
// candidates inside their windows.
func devRefineDoc() string {
	var sb strings.Builder
	for _, title := range []string{"# Syncing", "# Building", "# Serving"} {
		sb.WriteString(title + "\n\n" + devRefineBody + "\n\n" + devRefineBody + "\n\n")
	}
	return sb.String()
}

// devRefineFile writes a document into a temporary directory and returns its
// path.
func devRefineFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// devRefineResult is what one runDevRefine invocation produced.
type devRefineResult struct {
	stdout    string
	stderr    string
	endpoints []model.Endpoint
	calls     []model.MockCall
	err       error
}

// runDevRefineVerb invokes the verb with buffered streams and a client factory
// that records its endpoint and serves the given answer to every call. The
// mock records its calls, so a test can read back what actually went on the
// wire — the effort included.
func runDevRefineVerb(t *testing.T, opts devRefineOptions, answer string) devRefineResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var seen []model.Endpoint
	opts.Stdout, opts.Stderr = &stdout, &stderr
	client := model.NewScriptedMock([]model.Response{{Content: answer}}, nil)
	client.RecordCalls = true
	opts.NewClient = func(e model.Endpoint) model.Client {
		seen = append(seen, e)
		return client
	}
	err := runDevRefine(context.Background(), opts)
	return devRefineResult{
		stdout: stdout.String(), stderr: stderr.String(),
		endpoints: seen, calls: client.Calls(), err: err,
	}
}

// devRefineOpts is the wired-up options value the tests vary from: one usable
// provider, a light tier mapped, and a document with boundaries in it.
func devRefineOpts(t *testing.T, file, out string) devRefineOptions {
	t.Helper()
	return devRefineOptions{
		providerOptions: providerOptions{
			Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
			Config:    config.Config{Provider: "solo", Models: config.ModelMap{Light: "gemma-4-26b-a4b"}},
		},
		File:   file,
		Out:    out,
		Budget: 128,
	}
}

// TestRunDevRefineWiring is the happy path: the fold runs, the composed cut
// list and its stamp land in --out beside the run record, and the report names
// what was dialed.
func TestRunDevRefineWiring(t *testing.T) {
	file := devRefineFile(t, "sync.md", devRefineDoc())
	out := filepath.Join(t.TempDir(), "job")

	got := runDevRefineVerb(t, devRefineOpts(t, file, out), "1")
	if got.err != nil {
		t.Fatalf("runDevRefine: %v (stderr %q)", got.err, got.stderr)
	}
	for _, want := range []string{
		"budget: 128 tokens",
		"provider: solo (http://provider.example/v1)",
		"model: gemma-4-26b-a4b (light tier)",
		"boundaries:",
		"sections:",
		"outcome:",
		"artifact: " + filepath.Join(out, "cuts", "cutlist.txt"),
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout %q missing %q", got.stdout, want)
		}
	}
	if len(got.endpoints) != 1 || got.endpoints[0].APIKey != testAPIKey {
		t.Fatalf("client constructions: got %+v, want one built from the pool entry", got.endpoints)
	}
	if strings.Contains(got.stdout, testAPIKey) || strings.Contains(got.stderr, testAPIKey) {
		t.Error("API key leaked into an output stream")
	}

	// The delivered artifact: the composed cut list, in --out proper, and
	// decodable by the one decoder a later stage would use.
	data, err := os.ReadFile(filepath.Join(out, "cuts", "cutlist.txt"))
	if err != nil {
		t.Fatalf("the composed cut list was not delivered: %v", err)
	}
	if _, err := dissect.DecodeCutList(data); err != nil {
		t.Errorf("the delivered cut list does not decode: %v", err)
	}
	// And the scratch tree is gone: a run that succeeded and delivered has
	// nothing left to keep (ARCHITECTURE.md §12).
	if _, err := os.Stat(filepath.Join(out, pipeline.TempWorkDirName)); !os.IsNotExist(err) {
		t.Errorf("%s survived a successful run (err = %v)", pipeline.TempWorkDirName, err)
	}

	path := filepath.Join(out, devRefineRecordName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", devRefineRecordName, err)
	}
	if mode := info.Mode().Perm(); mode != pipeline.ArtifactFileMode {
		t.Errorf("%s mode = %o, want %o", devRefineRecordName, mode, pipeline.ArtifactFileMode)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", devRefineRecordName, err)
	}
	var rec devRefineRun
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("%s is not valid JSON: %v", devRefineRecordName, err)
	}
	if rec.Model != "gemma-4-26b-a4b" || rec.Tier != config.TierLight || rec.Provider != "solo" {
		t.Errorf("record identity = %+v, want the dialed provider, model and tier", rec)
	}
	if rec.BudgetTokens != 128 || rec.Source != file || rec.SourceSHA256 == "" {
		t.Errorf("record inputs = %+v, want the budget and the document it was run over", rec)
	}
	if rec.Boundaries != rec.BoundariesMoved+rec.BoundariesKept || rec.Boundaries < 1 {
		t.Errorf("record outcomes = %+v, want every boundary accounted for as moved or kept", rec)
	}
	if rec.Sections != rec.Boundaries+1 {
		t.Errorf("record sections = %+v, want one more section than boundaries", rec)
	}
	if strings.Contains(string(raw), testAPIKey) {
		t.Error("API key leaked into the run record")
	}
}

// TestRunDevRefineEffortDeclarationAndOverride: the effort every call asks
// with is the definition's declaration unless --thinking replaces it, and the
// effective value is both printed and recorded.
//
// All three places are checked together on purpose. The stdout line and the
// run record are what an operator reads afterwards, and the wire is what
// actually happened; a run whose report and whose request disagree is worse
// than one that reported nothing, because the A/B it exists for would be
// comparing the labels rather than the runs.
func TestRunDevRefineEffortDeclarationAndOverride(t *testing.T) {
	on, off := true, false
	for _, tc := range []struct {
		name       string
		override   *bool
		want       bool
		overridden bool
		wantLine   string
	}{
		{"the definition's declaration", nil, devRefineEffort.Thinking, false,
			fmt.Sprintf("thinking: %t (declared)", devRefineEffort.Thinking)},
		{"overridden on", &on, true, true, "thinking: true (--thinking override)"},
		{"overridden off", &off, false, true, "thinking: false (--thinking override)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := devRefineFile(t, "sync.md", devRefineDoc())
			out := filepath.Join(t.TempDir(), "job")
			opts := devRefineOpts(t, file, out)
			opts.Thinking = tc.override

			got := runDevRefineVerb(t, opts, "1")
			if got.err != nil {
				t.Fatalf("runDevRefine: %v (stderr %q)", got.err, got.stderr)
			}
			if !strings.Contains(got.stdout, tc.wantLine) {
				t.Errorf("stdout %q missing %q", got.stdout, tc.wantLine)
			}

			if len(got.calls) == 0 {
				t.Fatal("no call reached the client; there is no effort to check")
			}
			for i, call := range got.calls {
				for _, key := range []string{"thinking", "enable_thinking"} {
					if v, ok := call.Request.ChatTemplateKwargs[key]; !ok || v != tc.want {
						t.Errorf("call %d: %s = %v (present %t), want %t", i, key, v, ok, tc.want)
					}
				}
			}

			raw, err := os.ReadFile(filepath.Join(out, devRefineRecordName))
			if err != nil {
				t.Fatalf("read %s: %v", devRefineRecordName, err)
			}
			var rec devRefineRun
			if err := json.Unmarshal(raw, &rec); err != nil {
				t.Fatalf("%s is not valid JSON: %v", devRefineRecordName, err)
			}
			if rec.Thinking != tc.want || rec.ThinkingOverride != tc.overridden {
				t.Errorf("record effort = {thinking %t, overridden %t}, want {%t, %t}",
					rec.Thinking, rec.ThinkingOverride, tc.want, tc.overridden)
			}
		})
	}
}

// TestRunDevRefineNoBoundaries: a document the budget already fits is a valid
// smoke run with nothing to adjudicate. It says so, succeeds, and never dials
// — spending a client construction on a stage with no calls in it would be
// reporting a provider that was never asked anything.
func TestRunDevRefineNoBoundaries(t *testing.T) {
	file := devRefineFile(t, "small.md", "# Small\n\n"+devRefineBody+"\n")
	out := filepath.Join(t.TempDir(), "job")

	opts := devRefineOpts(t, file, out)
	opts.Budget = 4096
	got := runDevRefineVerb(t, opts, "1")

	if got.err != nil {
		t.Fatalf("runDevRefine: %v", got.err)
	}
	if !strings.Contains(got.stdout, "no boundaries to refine") {
		t.Errorf("stdout %q should say there is nothing to adjudicate", got.stdout)
	}
	if len(got.endpoints) != 0 {
		t.Errorf("a run with no boundaries must not dial; got %d client constructions", len(got.endpoints))
	}
	if _, err := os.Stat(filepath.Join(out, devRefineRecordName)); err == nil {
		t.Errorf("a run that made no call should leave no %s", devRefineRecordName)
	}
}

// TestRunDevRefineRefusals: every refusal this verb owns happens before a
// client is built. A run that dials and then discovers it cannot proceed has
// already put the corpus on someone's wire.
func TestRunDevRefineRefusals(t *testing.T) {
	file := devRefineFile(t, "sync.md", devRefineDoc())

	for _, tc := range []struct {
		name string
		mut  func(*devRefineOptions)
		want string
	}{{
		name: "no output directory",
		mut:  func(o *devRefineOptions) { o.Out = "  " },
		want: "--out is required",
	}, {
		name: "no document",
		mut:  func(o *devRefineOptions) { o.File = "" },
		want: "a document to refine is required",
	}, {
		name: "budget of nothing",
		mut:  func(o *devRefineOptions) { o.Budget = 0 },
		want: "a section budget is a positive number of tokens",
	}, {
		name: "light tier unmapped",
		mut:  func(o *devRefineOptions) { o.Config.Models = config.ModelMap{Heavy: "gemma-4-31b"} },
		want: `no model is configured for the "light" tier`,
	}, {
		name: "not a markdown document",
		mut:  func(o *devRefineOptions) { o.File = devRefineFile(t, "notes.txt", devRefineDoc()) },
		want: "is not a .md document",
	}, {
		name: "document does not exist",
		mut:  func(o *devRefineOptions) { o.File = filepath.Join(t.TempDir(), "gone.md") },
		want: "read",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			opts := devRefineOpts(t, file, filepath.Join(t.TempDir(), "job"))
			tc.mut(&opts)
			got := runDevRefineVerb(t, opts, "1")
			if got.err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.want)
			}
			if !strings.Contains(got.err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", got.err, tc.want)
			}
			if len(got.endpoints) != 0 {
				t.Errorf("a refused run must not dial; got %d client constructions", len(got.endpoints))
			}
		})
	}
}

// TestRunDevRefineLeavesTheOutputDirectoryAlone is the case the temp-work
// rooting exists for (ARCHITECTURE.md §12).
//
// A completed run sweeps every file its store's root holds that the chain does
// not account for. What keeps that away from an operator's files is not a
// check but a place: the store is rooted in a directory kbase created one
// level below --out. So `kbase dev-refine notes/sync.md --out notes` — which
// is the first thing anyone will type — leaves notes/ exactly as it found it.
func TestRunDevRefineLeavesTheOutputDirectoryAlone(t *testing.T) {
	file := devRefineFile(t, "sync.md", devRefineDoc())
	out := t.TempDir()

	const foreign = "notes.md"
	want := "the operator's own document, which this run has no business deleting\n"
	if err := os.WriteFile(filepath.Join(out, foreign), []byte(want), 0o600); err != nil {
		t.Fatalf("write the operator's file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "chapter"), 0o700); err != nil {
		t.Fatalf("make the operator's directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "chapter", "deeper.md"), []byte(want), 0o600); err != nil {
		t.Fatalf("write the operator's nested file: %v", err)
	}

	got := runDevRefineVerb(t, devRefineOpts(t, file, out), "1")
	if got.err != nil {
		t.Fatalf("runDevRefine: %v (stderr %q)", got.err, got.stderr)
	}

	for _, rel := range []string{foreign, filepath.Join("chapter", "deeper.md")} {
		data, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Errorf("%s did not survive the run: %v", rel, err)
			continue
		}
		if string(data) != want {
			t.Errorf("%s = %q, want it untouched", rel, data)
		}
	}
	// And the run still delivered what it owed, and cleaned up after itself.
	for _, rel := range []string{devRefineRecordName, filepath.Join("cuts", "cutlist.txt")} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("%s was not delivered: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, pipeline.TempWorkDirName)); !os.IsNotExist(err) {
		t.Errorf("%s survived a successful run (err = %v)", pipeline.TempWorkDirName, err)
	}
}

// TestRunDevRefineKeepsTempWorkWhenAsked: the one switch over the teardown,
// and what it keeps is the resume machinery a delivered artifact does not
// carry — the stamp above all.
func TestRunDevRefineKeepsTempWorkWhenAsked(t *testing.T) {
	file := devRefineFile(t, "sync.md", devRefineDoc())
	out := filepath.Join(t.TempDir(), "job")

	opts := devRefineOpts(t, file, out)
	opts.KeepTempWork = true
	got := runDevRefineVerb(t, opts, "1")
	if got.err != nil {
		t.Fatalf("runDevRefine: %v (stderr %q)", got.err, got.stderr)
	}

	work := filepath.Join(out, pipeline.TempWorkDirName)
	for _, rel := range []string{
		filepath.Join("cuts", "cutlist.txt"),
		filepath.Join("cuts", "cutlist.txt") + pipeline.StampSuffix,
	} {
		if _, err := os.Stat(filepath.Join(work, rel)); err != nil {
			t.Errorf("%s was not kept: %v", rel, err)
		}
	}
	if !strings.Contains(got.stdout, "temp work kept: "+work) {
		t.Errorf("stdout %q does not say where the kept scratch tree is", got.stdout)
	}
}

// TestRunDevRefineKeepsTempWorkAfterAnInterruptedRun: the asymmetry is the
// rule. A run that did not deliver keeps its intermediates unconditionally —
// they are what a resume reads, and litter around a broken job is evidence.
func TestRunDevRefineKeepsTempWorkAfterAnInterruptedRun(t *testing.T) {
	file := devRefineFile(t, "sync.md", devRefineDoc())
	out := filepath.Join(t.TempDir(), "job")

	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderr bytes.Buffer
	opts := devRefineOpts(t, file, out)
	opts.Stdout, opts.Stderr = &stdout, &stderr
	opts.NewClient = func(model.Endpoint) model.Client { return cancellingClient{cancel: cancel} }

	if err := runDevRefine(ctx, opts); err == nil {
		t.Fatal("a cancelled run must return an error, not a report")
	}
	if _, err := os.Stat(filepath.Join(out, pipeline.TempWorkDirName)); err != nil {
		t.Errorf("an interrupted run threw away its intermediates: %v", err)
	}
	// Nothing was delivered: delivery happens only after the job succeeded.
	if _, err := os.Stat(filepath.Join(out, devRefineRecordName)); !os.IsNotExist(err) {
		t.Errorf("a run that did not finish left a run record (err = %v)", err)
	}
}

// cancellingClient stops the run from inside its first call — the shape of a
// Ctrl-C, which is the interruption a development verb actually meets.
type cancellingClient struct{ cancel context.CancelFunc }

func (c cancellingClient) Consult(ctx context.Context, _ model.Request) (model.Response, error) {
	c.cancel()
	return model.Response{}, context.Canceled
}

func (c cancellingClient) ConsultStream(ctx context.Context, _ model.Request) (model.StreamReader, error) {
	c.cancel()
	return nil, context.Canceled
}

func (c cancellingClient) ListModels(context.Context) ([]model.ModelInfo, error) { return nil, nil }

// TestDevRefineThinkingIsTriState pins the half of the override that lives
// entirely in cobra, through the flag the command really registers.
//
// `--thinking=false` against a declaration of false must still read as an
// OVERRIDE. The value cannot say so — it is the declaration's own — so the
// fact lives entirely in whether the flag was GIVEN, and a plumbing that read
// the value would look correct on every row but this one. This is the row an
// A/B of one document turns on.
func TestDevRefineThinkingIsTriState(t *testing.T) {
	flags := devRefineCmd.Flags()
	t.Cleanup(func() {
		flags.Lookup(devRefineThinkingFlag).Changed = false
		devRefineFlagThinking = devRefineEffort.Thinking
	})

	if got := devRefineThinking(flags); got != nil {
		t.Fatalf("an unset flag reads as %t, want nil: the definition's declaration stands", *got)
	}
	if effort, overridden := devRefineAsk(devRefineThinking(flags)); overridden || effort != devRefineEffort {
		t.Errorf("unset = {%+v, overridden %t}, want the declaration untouched", effort, overridden)
	}

	// The declared value, given explicitly — the row that separates "given"
	// from "equal to the default".
	declared := fmt.Sprint(devRefineEffort.Thinking)
	if err := flags.Parse([]string{"--" + devRefineThinkingFlag + "=" + declared}); err != nil {
		t.Fatalf("parse --%s=%s: %v", devRefineThinkingFlag, declared, err)
	}
	got := devRefineThinking(flags)
	if got == nil {
		t.Fatal("a flag that was given must read as an override, whatever its value")
	}
	if *got != devRefineEffort.Thinking {
		t.Errorf("the flag reads %t, want the %t it was given", *got, devRefineEffort.Thinking)
	}
	effort, overridden := devRefineAsk(got)
	if !overridden {
		t.Error("--thinking=<the declared value> is still an override; a run record that said otherwise would lose the experiment")
	}
	if !effort.Declared() || effort.Thinking != devRefineEffort.Thinking {
		t.Errorf("effort = %+v, want a declared %t", effort, devRefineEffort.Thinking)
	}
}

// TestSafeBaseURL: run.json is by design an artifact an operator shares as
// evidence, and a base URL with userinfo in it is a legal providers.toml
// value. The key never reaches the record; this is the other way a credential
// could have.
func TestSafeBaseURL(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"no userinfo", "https://host/v1", "https://host/v1"},
		{"a user and a password", "https://user:secret@host/v1", "https://host/v1"},
		{"a user alone", "https://user@host/v1", "https://host/v1"},
		{"not a URL at all", "://nonsense", "://nonsense"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeBaseURL(tc.in); got != tc.want {
				t.Errorf("safeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.Contains(safeBaseURL(tc.in), "secret") {
				t.Error("a credential survived into the recorded base URL")
			}
		})
	}
}

// TestRequireConfigDirFlag: dev-refine has no implicit configuration
// directory. The refusal names the flag, because naming the flag is the whole
// remedy.
func TestRequireConfigDirFlag(t *testing.T) {
	saved := flagConfigDir
	t.Cleanup(func() { flagConfigDir = saved })

	flagConfigDir = "   "
	err := requireConfigDirFlag(devRefineVerb)
	if err == nil {
		t.Fatal("an unset --config-dir must be refused, not resolved from the environment or the home directory")
	}
	if !strings.Contains(err.Error(), "--config-dir is required") {
		t.Errorf("error = %v, want it to name the flag", err)
	}

	flagConfigDir = t.TempDir()
	if err := requireConfigDirFlag(devRefineVerb); err != nil {
		t.Errorf("a given --config-dir must satisfy the guard; got %v", err)
	}
}
