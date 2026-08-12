package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/config"
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
	err       error
}

// runDevRefineVerb invokes the verb with buffered streams and a client factory
// that records its endpoint and serves the given answer to every call.
func runDevRefineVerb(t *testing.T, opts devRefineOptions, answer string) devRefineResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var seen []model.Endpoint
	opts.Stdout, opts.Stderr = &stdout, &stderr
	client := model.NewScriptedMock([]model.Response{{Content: answer}}, nil)
	opts.NewClient = func(e model.Endpoint) model.Client {
		seen = append(seen, e)
		return client
	}
	err := runDevRefine(context.Background(), opts)
	return devRefineResult{stdout: stdout.String(), stderr: stderr.String(), endpoints: seen, err: err}
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
		"artifact: cuts/cutlist.txt",
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

	// The stage's artifact and its proof: the verb's real output, and what a
	// later stage would consume.
	for _, rel := range []string{"cuts/cutlist.txt", "cuts/cutlist.txt" + pipeline.StampSuffix} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("expected %s in the job directory: %v", rel, err)
		}
	}

	path := filepath.Join(out, devRefineRecordName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", devRefineRecordName, err)
	}
	if mode := info.Mode().Perm(); mode != artifactFileMode {
		t.Errorf("%s mode = %o, want %o", devRefineRecordName, mode, artifactFileMode)
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
	if rec.SectionsAfter != rec.Boundaries+1 {
		t.Errorf("record sections = %+v, want one more section than boundaries", rec)
	}
	if strings.Contains(string(raw), testAPIKey) {
		t.Error("API key leaked into the run record")
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
