package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/dissect"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
	"kbase/internal/version"
)

// dev-refine runs stage 4's boundary fold over ONE document against a REAL
// provider. It is a development verb: the first place the orchestrator, the
// refinement seam and a live model meet, and the only place today where they
// do — the taxonomy stage that will eventually say what the spans are does not
// exist yet, so this verb supplies the one span it can name without inventing
// anything: the whole file (ARCHITECTURE.md §5, ROADMAP).
//
// What it measures is PLUMBING, not quality. The refinement definition is a
// marked stub (dissect.stubDefinition) pending the embedded-definitions burst,
// so the questions this asks are not the questions the shipped pipeline will
// ask. What a run answers is whether the light tier returns a bare menu
// number, how often the informed retry and the mechanical fallback fire, and
// what real latency and cached_tokens look like through the whole path.
//
// Everything here is composition: no policy, no verification and no prompt
// text is decided in this file. That is the same rule every other verb in
// cmd/ follows, and it matters more here because a shortcut taken at this
// seam would be a second implementation of a rule internal/dissect and
// internal/pipeline already own exactly one of.
const (
	// devRefineVerb prefixes this verb's own diagnostics — the ones that name
	// a flag or a file rather than something the pipeline reported.
	devRefineVerb = "dev-refine"

	// devRefineBudget is the default --budget. It is deliberately small: the
	// point of a smoke run is a document that yields several real boundaries,
	// and the per-section budget the pipeline will actually use (tens of
	// thousands of tokens) would leave a doc-sized file in one piece with
	// nothing to adjudicate.
	devRefineBudget = 512

	// devRefineStage names the stage, its one domain stream, and the store
	// directory its composed cut list lands in — one name, so a failure names
	// one thing rather than three.
	devRefineStage = "cuts"

	// devRefineFrame is slot 1 for this job: process framing, job-stable
	// (ARCHITECTURE.md §7). One verb, one document, one frame.
	devRefineFrame = "kbase dev-refine: adjudicating the section boundaries of one document."

	// devRefineSourceInput names the source identity in the composed cut
	// list's stamp. The stage appends its own parameter digest beside it
	// (dissect.Refiner.StagePlan), which is what makes a re-run at a different
	// budget a different question rather than the same one.
	devRefineSourceInput = "corpus"

	// devRefineRecordName is the run record this verb leaves in --out: what
	// was dialed, what was asked, and what came back. It is written AFTER the
	// job, because a completed run sweeps every file the chain does not
	// account for (pipeline.Store.sweep) and this is not one of them.
	devRefineRecordName = "run.json"

	// devRefineTimeout bounds the WHOLE fold rather than one call: n boundary
	// calls at light-tier latency, plus the transport policy's own retries
	// underneath each of them. It is generous because the failure it exists to
	// catch is a provider that has stopped answering, not one that is slow.
	devRefineTimeout = 30 * time.Minute

	// devRefineThinkingFlag is the effort override's flag name. It is a
	// constant because the tri-state reads it back — a flag whose default
	// means "unset" has to be asked whether it was given, and the name is
	// then in two places.
	devRefineThinkingFlag = "thinking"

	// outDirMode is the mode --out is created with when it does not exist.
	// The directory holds a derived view of the user's document, so it gets
	// the same owner-only posture the store gives the artifacts inside it.
	outDirMode = 0o700
)

// devRefineEffort is the boundary-refinement definition's DECLARED effort —
// the registration site's statement of how hard this exact ask is worth
// asking, threaded from here through dissect.NewRefiner to the wire
// (ARCHITECTURE.md §9, §12). It lives in cmd because this is where the
// refinement stage is registered today; when the taxonomy stage wires the real
// job plan, the declaration moves with the registration.
//
// Thinking is OFF. The menu choice is the narrowest ask in the pipeline — pick
// one of at most seven numbered positions, in a window the model is already
// shown — and the live A/B of 2026-08-12 measured what reasoning bought on it:
// a byte-identical cut list for 20,924 completion tokens instead of 4, two
// minutes instead of three seconds, and one MORE compliance rejection (a
// reasoning run answered boundary 2 with something other than a bare number).
// It bought nothing and cost everything. The off value is stated rather than
// defaulted so that a later ask which IS worth reasoning over has to say so on
// its own line.
var devRefineEffort = model.DeclareEffort(model.Effort{Thinking: false})

// devRefineOptions is the resolved input of the dev-refine verb: the shared
// provider-reaching options, the one document under work, and where its
// artifacts go.
type devRefineOptions struct {
	providerOptions

	// File is the Markdown document to adjudicate.
	File string

	// Out is the job directory: the store's root, and where the run record
	// lands. Required — there is no temporary directory anywhere in this
	// verb, because the artifacts and stamps ARE the smoke test's evidence.
	Out string

	// Budget is the per-section token budget the mechanical splitter fills
	// toward, and half of what identifies the cut list it produces.
	Budget int

	// Thinking is the --thinking override, tri-state: nil takes the
	// definition's own declaration (devRefineEffort), non-nil replaces it.
	// A verb does not get to decide how hard a definition asks — this is an
	// experiment switch, and it exists so the two configurations can be run
	// against the same document and compared.
	Thinking *bool

	// Stdout takes the verb's report: what was dialed, and the composed cut
	// list read back for a human.
	Stdout io.Writer

	// Logger takes the pipeline's own diagnostics — per-boundary outcomes,
	// usage, the composed-write record. They are a different channel from
	// Stdout and land at info.
	Logger log.Logger
}

// runDevRefine ingests one document, splits it mechanically, and runs the
// refinement fold over the result against a real provider.
//
// The order of the refusals is the point: everything that can be refused
// offline is refused before a client is dialed, so a misconfigured run costs
// no tokens. The one exception is the seam's own failures, which are the
// pipeline's to report — this verb surfaces them exactly as it receives them,
// because a friendlier wrapper around "the model's refinement did not verify"
// is a wrapper around the measurement.
func runDevRefine(ctx context.Context, opts devRefineOptions) error {
	if err := requireStreams(opts.Stdout, opts.Stderr, devRefineVerb); err != nil {
		return err
	}
	file := strings.TrimSpace(opts.File)
	if file == "" {
		return fmt.Errorf("%s: a document to refine is required", devRefineVerb)
	}
	out := strings.TrimSpace(opts.Out)
	if out == "" {
		return fmt.Errorf("%s: --out is required; this verb writes artifacts and stamps and never uses a temporary directory", devRefineVerb)
	}
	if opts.Budget <= 0 {
		return fmt.Errorf("%s: --budget is %d; a section budget is a positive number of tokens", devRefineVerb, opts.Budget)
	}
	lg := opts.Logger
	if lg == nil {
		lg = log.Discard()
	}

	// Refinement is light-tier work (ARCHITECTURE.md §10). The runner resolves
	// the tier itself on every call, so this check is not what makes the run
	// correct — it is what makes an unmapped tier a refusal before the ingest
	// rather than a worker abort after it.
	modelID, ok := opts.Config.ModelFor(config.TierLight)
	if !ok {
		return fmt.Errorf("%s: no model is configured for the %q tier in %s; refinement is light-tier work",
			devRefineVerb, config.TierLight, config.ConfigFileName)
	}
	if err := os.MkdirAll(out, outDirMode); err != nil {
		return fmt.Errorf("%s: create %s: %w", devRefineVerb, out, err)
	}

	corpus, err := ingestOne(file)
	if err != nil {
		return err
	}
	est := tokens.Estimator{}
	artifact, err := markdown.Survey(corpus, est, lg)
	if err != nil {
		return err
	}
	// One document in, one file out — Survey returns one File per corpus unit,
	// in corpus order, and survey.Assemble has already refused any other shape.
	doc := artifact.Files[0]
	src := corpus.Units[0].Bytes
	span := survey.Range{Start: 0, End: len(src)}
	params := dissect.Params{Est: est, BudgetTokens: opts.Budget}

	mech, err := dissect.Split(src, span, doc.Cuts, params)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Stdout, "source: %s (%d bytes, %d tokens, sha256 %s)\n",
		file, doc.Bytes, doc.Tokens, doc.SHA256)
	fmt.Fprintf(opts.Stdout, "budget: %d tokens\n", opts.Budget)
	fmt.Fprintf(opts.Stdout, "mechanical split: %d sections, %d interior boundaries\n", len(mech), len(mech)-1)
	if len(mech) < 2 {
		// A one-section span is a valid outcome, not a failure: the document
		// fits the budget, so there is no boundary anyone chose and nothing for
		// a model to improve on.
		fmt.Fprintln(opts.Stdout, "no boundaries to refine")
		return nil
	}

	effort, overridden := devRefineAsk(opts.Thinking)
	refiner, err := dissect.NewRefiner(src, span, doc.Cuts, mech, devRefineStage, params, effort, lg)
	if err != nil {
		return err
	}
	// Read BEFORE the fold runs: Boundaries is a view of live stage state, so
	// this is the mechanical list and the same call after the run is the
	// composed one. The difference between them is what the report is about.
	before := refiner.Boundaries()

	name, client, callCtx, release, err := opts.dial(ctx)
	if err != nil {
		return err
	}
	defer release()
	fmt.Fprintf(opts.Stdout, "provider: %s (%s)\nmodel: %s (%s tier)\n",
		name, opts.Providers[name].BaseURL, modelID, config.TierLight)
	fmt.Fprintf(opts.Stdout, "thinking: %t (%s)\n", effort.Thinking, effortSource(overridden))

	plan := pipeline.Plan{
		SystemFrame: devRefineFrame,
		Stages: []*pipeline.StagePlan{refiner.StagePlan(devRefineStage, []pipeline.Input{
			{Name: devRefineSourceInput, Hash: artifact.Corpus.ContentHash},
		})},
	}
	coord := pipeline.NewCoordinator(
		pipeline.NewStore(out, lg),
		pipeline.NewCallRunner(client, opts.Config, lg),
		1, lg)

	// ModeFresh, always. Resume would reuse a proven cut list and make no call
	// at all, which for a verb whose entire purpose is live traffic is a
	// successful-looking run that measured nothing.
	start := time.Now()
	res, err := coord.Run(callCtx, plan, pipeline.ModeFresh)
	elapsed := time.Since(start)
	if err != nil {
		return err
	}
	for _, f := range res.Failures {
		fmt.Fprintf(opts.Stderr, "unit %s failed (%s): %v\n", f.Path, f.Kind, f.Err)
	}
	if !res.EmitReady() {
		return fmt.Errorf("%s: the refinement job did not complete: %d of %d units produced, %d failed",
			devRefineVerb, res.Produced, res.Units, len(res.Failures))
	}

	after := refiner.Boundaries()
	cuts := composedCuts(span, after)
	unit := after[len(after)-1].Unit
	data, err := pipeline.NewStore(out, lg).Get(unit)
	if err != nil {
		return err
	}

	moved := reportBoundaries(opts.Stdout, src, est, before, after)
	reportSections(opts.Stdout, src, est, cuts)
	fmt.Fprintf(opts.Stdout, "outcome: %d boundaries, %d moved, %d kept, %d fell back to the mechanical cut\n",
		len(after), moved, len(after)-moved, res.Degraded)
	fmt.Fprintf(opts.Stdout, "usage: prompt=%d cached=%d completion=%d\n",
		res.Usage.PromptTokens, res.Usage.CachedPromptTokens, res.Usage.CompletionTokens)
	fmt.Fprintf(opts.Stdout, "elapsed: %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(opts.Stdout, "artifact: %s (%d bytes, %d lines) + %s\n",
		unit, len(data), bytes.Count(data, []byte("\n")), unit+pipeline.StampSuffix)

	record := devRefineRun{
		Version:          version.Current,
		Provider:         name,
		BaseURL:          opts.Providers[name].BaseURL,
		Model:            modelID,
		Tier:             config.TierLight,
		Thinking:         effort.Thinking,
		ThinkingOverride: overridden,
		Source:           file,
		SourceSHA256:     doc.SHA256,
		BudgetTokens:     opts.Budget,
		SectionsBefore:   len(mech),
		SectionsAfter:    len(cuts),
		Boundaries:       len(after),
		BoundariesMoved:  moved,
		BoundariesKept:   len(after) - moved,
		Fallbacks:        res.Degraded,
		PromptTokens:     res.Usage.PromptTokens,
		CachedTokens:     res.Usage.CachedPromptTokens,
		CompletionTokens: res.Usage.CompletionTokens,
		ElapsedMS:        elapsed.Milliseconds(),
		Artifact:         unit,
	}
	path, err := writeRunRecord(out, record)
	if err != nil {
		return err
	}
	fmt.Fprintf(opts.Stdout, "record: %s\n", path)
	return nil
}

// devRefineAsk resolves the effort this run asks with, and reports whether the
// declaration was overridden.
//
// Both halves are reported because both are evidence: a run.json saying
// thinking was on means one thing if that is what the definition declares and
// another if an operator asked for it, and a reader looking at two runs of the
// same document has no other way to tell which one was the experiment.
func devRefineAsk(override *bool) (model.Effort, bool) {
	if override == nil {
		return devRefineEffort, false
	}
	return model.DeclareEffort(model.Effort{Thinking: *override}), true
}

// effortSource labels where the effective effort came from, for the report.
func effortSource(overridden bool) string {
	if overridden {
		return "--thinking override"
	}
	return "declared"
}

// ingestOne takes custody of the ONE document named on the command line.
//
// ingest.Walk is the corpus path and takes a directory; this verb is handed a
// file, and walking its parent would ingest and survey every sibling document
// to refine one of them. ingest.New is the same custody with the walk left out
// — it is the constructor Walk itself ends in, so the bytes are the file's
// exactly and the content hash is derived the one way it is ever derived.
func ingestOne(path string) (ingest.Corpus, error) {
	if !strings.EqualFold(filepath.Ext(path), markdown.Ext) {
		return ingest.Corpus{}, fmt.Errorf("%s: %s is not a %s document; this verb surveys through the Markdown adapter",
			devRefineVerb, path, markdown.Ext)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ingest.Corpus{}, fmt.Errorf("%s: read %s: %w", devRefineVerb, path, err)
	}
	// The corpus id is the base name: a one-document corpus has no tree for a
	// path to be relative to, and an absolute path in a derived artifact is
	// machine state (see ingest.Corpus).
	return ingest.New([]ingest.Unit{{Path: filepath.Base(path), Bytes: b}})
}

// composedCuts rebuilds the section list from the fold's final boundaries.
//
// The stage's own composed list is on disk as bytes and there is no exported
// decoder for it, so this derives the same list from the positions the fold
// arrived at — the values the artifact was encoded from. The verb reads the
// artifact back as well, and reports its size and line count beside these
// sections, so the two readings are visible together rather than one standing
// in for the other.
func composedCuts(span survey.Range, bs []dissect.Boundary) []survey.Range {
	cuts := make([]survey.Range, 0, len(bs)+1)
	start := span.Start
	for _, b := range bs {
		cuts = append(cuts, survey.Range{Start: start, End: b.Cut})
		start = b.Cut
	}
	return append(cuts, survey.Range{Start: start, End: span.End})
}

// reportBoundaries prints one line per boundary and returns how many moved.
//
// Moved-or-kept is the whole of what a caller outside the stage can say about
// an individual boundary: which ones fell back to the mechanical cut, and
// which were rejected once and retried, are the fold's own records and land in
// the log at info (see dissect.Refiner.accept/reject/baseline). A fallback
// keeps the incumbent, so it reads as "kept" here — the stage-level fallback
// count is reported separately, and the log is what attributes it.
//
// Byte offsets are shown deliberately. The no-raw-offsets rule is about what
// the MODEL is shown (§5); this is an operator reading a cut list, and an
// offset is the only thing that locates one.
func reportBoundaries(w io.Writer, src []byte, est tokens.Estimator, before, after []dissect.Boundary) int {
	fmt.Fprintln(w, "boundaries:")
	moved := 0
	for i, b := range after {
		was := before[i].Cut
		if b.Cut == was {
			fmt.Fprintf(w, "  %3d kept  at %d\n", b.Index, b.Cut)
			continue
		}
		moved++
		direction := "forward"
		lo, hi := was, b.Cut
		if hi < lo {
			direction, lo, hi = "back", b.Cut, was
		}
		fmt.Fprintf(w, "  %3d moved %d -> %d (%s %d bytes, %d tokens)\n",
			b.Index, was, b.Cut, direction, hi-lo, est.EstimateBytes(src[lo:hi]))
	}
	return moved
}

// reportSections prints the composed cut list: one line per section, its byte
// range and its size.
func reportSections(w io.Writer, src []byte, est tokens.Estimator, cuts []survey.Range) {
	fmt.Fprintln(w, "sections:")
	for i, c := range cuts {
		fmt.Fprintf(w, "  %3d [%d,%d) %d bytes, %d tokens\n",
			i+1, c.Start, c.End, c.End-c.Start, est.EstimateBytes(src[c.Start:c.End]))
	}
}

// devRefineRun is the run record left in --out: what was dialed, what was
// asked, and what came back.
//
// It is a development record rather than a stamp — nothing reads it back, so
// it carries no schema version and its field order proves nothing. What it
// does carry is every identity a later reader would otherwise have to
// reconstruct from a log: the model and endpoint, the document and its digest,
// the effort the calls were made at, and the parameters the cut list beside it
// was adjudicated under. It carries no credential: the API key is never in
// this process's output, only on the wire.
//
// The effort is two fields rather than one because "thinking was on" and "an
// operator asked for thinking" are different facts, and an A/B of the same
// document is exactly the reading where confusing them loses the experiment.
//
// Rejections — a boundary whose first answer failed verification and was
// retried — are absent because no caller-visible value carries them:
// pipeline.JobResult reports fallbacks (Degraded) and not semantic attempts,
// and the fold's own counters are unexported. They are in the log, at info,
// one record per boundary.
type devRefineRun struct {
	Version          string `json:"version"`
	Provider         string `json:"provider"`
	BaseURL          string `json:"baseUrl"`
	Model            string `json:"model"`
	Tier             string `json:"tier"`
	Thinking         bool   `json:"thinking"`
	ThinkingOverride bool   `json:"thinkingOverridden"`
	Source           string `json:"source"`
	SourceSHA256     string `json:"sourceSha256"`
	BudgetTokens     int    `json:"budgetTokens"`
	SectionsBefore   int    `json:"sectionsBefore"`
	SectionsAfter    int    `json:"sectionsAfter"`
	Boundaries       int    `json:"boundaries"`
	BoundariesMoved  int    `json:"boundariesMoved"`
	BoundariesKept   int    `json:"boundariesKept"`
	Fallbacks        int    `json:"boundariesFellBack"`
	PromptTokens     int    `json:"promptTokens"`
	CachedTokens     int    `json:"cachedTokens"`
	CompletionTokens int    `json:"completionTokens"`
	ElapsedMS        int64  `json:"elapsedMs"`
	Artifact         string `json:"artifact"`
}

// writeRunRecord writes the run record into the job directory and returns its
// path. Owner-only, like everything else under a job directory.
func writeRunRecord(dir string, rec devRefineRun) (string, error) {
	path := filepath.Join(dir, devRefineRecordName)
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("%s: encode %s: %w", devRefineVerb, devRefineRecordName, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), artifactFileMode); err != nil {
		return "", fmt.Errorf("%s: write %s: %w", devRefineVerb, path, err)
	}
	return path, nil
}

// requireConfigDirFlag refuses the implicit configuration directory.
//
// Every other verb resolves one (resolveConfigDir: the flag, else the
// environment, else ~/.config/kbase). This one does not: it exists to be
// pointed at a development fixture, and a run that silently fell back to the
// user's own configuration would reach a provider they did not name from a
// command whose whole point is which provider it reached.
func requireConfigDirFlag(verb string) error {
	if strings.TrimSpace(flagConfigDir) == "" {
		return fmt.Errorf("%s: --config-dir is required; this verb reads no default configuration directory", verb)
	}
	return nil
}

var (
	devRefineFlagOut      string
	devRefineFlagBudget   int
	devRefineFlagThinking bool
)

var devRefineCmd = &cobra.Command{
	Use:   "dev-refine <file.md>",
	Short: "run stage 4's boundary fold over one document against a real provider",
	Long: `Ingest one Markdown document, split it mechanically at the given budget, and
run the boundary-refinement fold over the result — with a real client, a real
model and the real orchestrator.

This is a development verb. It measures the plumbing rather than the output:
whether the light tier answers a boundary question with a bare menu number,
how often the informed retry and the mechanical fallback fire, and what the
whole path costs in latency and tokens. The refinement prompt is a marked stub
until the embedded-definitions work lands, so the cut list it produces is
evidence about the seam and not about the document.

--config-dir is required and has no default: a smoke run must reach the
provider it was pointed at. --out is required too, and is kept — the artifact,
its stamp and the run record are what the run leaves behind to read. The
per-boundary outcomes are logged at info.

--thinking overrides the effort the refinement definition declares for its
calls; left off, the definition's own declaration stands. It exists so the two
configurations can be run against the same document and compared. The
effective value and whether it was overridden are printed and recorded in the
run record.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireConfigDirFlag(devRefineVerb); err != nil {
			return err
		}
		_, opts, err := loadVerbContext()
		if err != nil {
			return err
		}
		opts.Timeout = devRefineTimeout
		// Tri-state: the flag's VALUE means nothing unless it was given, so
		// "unset" is the pointer being nil and not a third bool value. That
		// keeps the definition's declaration the default in the one way that
		// cannot be confused with an operator asking for thinking off.
		var thinking *bool
		if cmd.Flags().Changed(devRefineThinkingFlag) {
			thinking = &devRefineFlagThinking
		}
		return runDevRefine(cmd.Context(), devRefineOptions{
			providerOptions: opts,
			File:            args[0],
			Out:             devRefineFlagOut,
			Budget:          devRefineFlagBudget,
			Thinking:        thinking,
			Stdout:          os.Stdout,
			Logger:          processLog.logger,
		})
	},
}

func init() {
	devRefineCmd.Flags().StringVar(&devRefineFlagOut, "out", "",
		"job directory for the composed cut list, its stamp and "+devRefineRecordName+" (required; created if missing)")
	devRefineCmd.Flags().IntVar(&devRefineFlagBudget, "budget", devRefineBudget,
		"per-section token budget the mechanical splitter fills toward")
	devRefineCmd.Flags().BoolVar(&devRefineFlagThinking, devRefineThinkingFlag, devRefineEffort.Thinking,
		"override the refinement definition's declared thinking mode (default: what it declares)")
	rootCmd.AddCommand(devRefineCmd)
}
