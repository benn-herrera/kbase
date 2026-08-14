package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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
// fallback-backed seam and a live model meet, and the only place today where they
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

	// devRefineStage names the stage, its one serial lane, and the store
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
	// was dialed, what was asked, and what came back. It is one of this verb's
	// two DELIVERED artifacts (the composed cut list is the other), so it
	// lands in the output directory proper rather than in the scratch tree —
	// which is also why the sweep can no longer reach it.
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

	// devRefineKeepFlag turns off the successful run's temp-work teardown for
	// one run, over whatever `[dev] keep_temp_work` says.
	devRefineKeepFlag = "keep-temp-work"
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
var devRefineEffort = model.DeclareEffort(model.RequestEffort{Thinking: false})

// devRefineOptions is the resolved input of the dev-refine verb: the shared
// provider-reaching options, the one document under work, and where its
// artifacts go.
type devRefineOptions struct {
	providerOptions

	// File is the Markdown document to adjudicate.
	File string

	// Out is the OUTPUT directory: where this run's delivered artifacts land
	// — the composed cut list and the run record — and, while it runs, where
	// its `temp-work/` scratch tree sits (ARCHITECTURE.md §12). Required, and
	// never a system temporary directory, because the run's output IS the
	// smoke test's evidence.
	//
	// Nothing already in it is touched. Everything kbase deletes it deleted
	// out of a directory it created one level down.
	Out string

	// KeepTempWork keeps `<out>/temp-work/` after a run that succeeded — the
	// stamps, the scratch copy of the cut list, and the rest of the resume
	// machinery. It is the --keep-temp-work flag ORed with `[dev]
	// keep_temp_work`; a failed or interrupted run keeps the tree whatever
	// either says.
	KeepTempWork bool

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
		return fmt.Errorf("%s: --out is required; this verb delivers a cut list and a run record and never uses a temporary directory", devRefineVerb)
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
	// The output directory, created if missing — but nothing transient goes
	// in it. The scratch tree is opened later, once there is work to do.
	if err := os.MkdirAll(out, pipeline.CreateDirMode); err != nil {
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
	src := corpus.Docs[0].Bytes
	span := survey.Span{Start: 0, End: len(src)}
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
	before, err := refiner.Boundaries()
	if err != nil {
		return err
	}

	name, client, callCtx, release, err := opts.dial(ctx)
	if err != nil {
		return err
	}
	defer release()
	baseURL := safeBaseURL(opts.Providers[name].BaseURL)
	fmt.Fprintf(opts.Stdout, "provider: %s (%s)\nmodel: %s (%s tier)\n",
		name, baseURL, modelID, config.TierLight)
	fmt.Fprintf(opts.Stdout, "thinking: %t (%s)\n", effort.Thinking, effortSource(overridden))

	plan := pipeline.Plan{
		JobFrame: devRefineFrame,
		Stages: []*pipeline.StagePlan{refiner.StagePlan(devRefineStage, []pipeline.Input{
			{Name: devRefineSourceInput, Hash: artifact.Corpus.ContentHash},
		})},
	}
	// Everything the job writes goes under <out>/temp-work/ — stamps, the
	// scratch cut list, the lock — and the delivered artifacts are copied out
	// below. That is what makes `--out .` survivable: the sweep at the end of
	// a completed run operates inside a directory this call created.
	work, err := pipeline.OpenTempWork(out, lg)
	if err != nil {
		return err
	}
	store := work.ArtifactStore()
	coord := pipeline.NewCoordinator(store, pipeline.NewCallRunner(client, opts.Config, lg), 1, lg)

	// ModeFresh, always. Resume would reuse a proven cut list and make no call
	// at all, which for a verb whose entire purpose is live traffic is a
	// successful-looking run that measured nothing.
	start := time.Now()
	res, err := coord.Run(callCtx, plan, pipeline.ModeFresh)
	elapsed := time.Since(start)
	// Every early return from here down keeps temp-work: a run that failed or
	// was interrupted leaves its intermediates for a resume and for a human
	// (ARCHITECTURE.md §12).
	if err != nil {
		return err
	}
	for _, f := range res.Failures {
		fmt.Fprintf(opts.Stderr, "unit %s failed (%s): %v\n", f.Path, f.Kind, f.Err)
	}
	if !res.DeliveryReady() {
		return fmt.Errorf("%s: the refinement job did not complete: %d of %d units produced, %d failed",
			devRefineVerb, res.Produced, res.Units, len(res.Failures))
	}

	after, err := refiner.Boundaries()
	if err != nil {
		return err
	}
	unit := after[len(after)-1].Unit
	data, err := store.Get(unit)
	if err != nil {
		return err
	}
	// The artifact's own bytes decoded, not the fold's boundaries re-derived:
	// one decoder for this format, and what is reported is what was written.
	cuts, err := dissect.DecodeCutList(data)
	if err != nil {
		return err
	}
	artifactPath, err := deliver(out, unit, data)
	if err != nil {
		return err
	}

	moved := reportBoundaries(opts.Stdout, src, est, before, after)
	reportSections(opts.Stdout, src, est, cuts)
	fmt.Fprintf(opts.Stdout, "outcome: %d boundaries, %d moved, %d kept, %d fell back to the mechanical cut\n",
		len(after), moved, len(after)-moved, res.FallbackCount)
	fmt.Fprintf(opts.Stdout, "usage: prompt=%d cached=%d completion=%d\n",
		res.Usage.PromptTokens, res.Usage.CachedPromptTokens, res.Usage.CompletionTokens)
	fmt.Fprintf(opts.Stdout, "elapsed: %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(opts.Stdout, "artifact: %s (%d bytes, %d lines)\n",
		artifactPath, len(data), bytes.Count(data, []byte("\n")))

	record := devRefineRun{
		Version:          version.Current,
		Provider:         name,
		BaseURL:          baseURL,
		Model:            modelID,
		Tier:             config.TierLight,
		Thinking:         effort.Thinking,
		ThinkingOverride: overridden,
		Source:           file,
		UploadSHA256:     doc.SHA256,
		BudgetTokens:     opts.Budget,
		Sections:         len(cuts),
		Boundaries:       len(after),
		BoundariesMoved:  moved,
		BoundariesKept:   len(after) - moved,
		Fallbacks:        res.FallbackCount,
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

	// The run succeeded and both delivered artifacts are out. The scratch tree
	// goes unless someone asked to keep it.
	if opts.KeepTempWork {
		fmt.Fprintf(opts.Stdout, "temp work kept: %s\n", work.Root())
		return nil
	}
	return work.Discard()
}

// safeBaseURL is a provider's base URL with any userinfo removed.
//
// `https://user:pass@host/v1` is a legal providers.toml value, and run.json is
// by design an artifact an operator shares as evidence — a credential in it
// would travel with the run report. The API key never appears here (it lives
// on the Endpoint and goes on the wire), so this closes the one remaining way
// a secret could reach the record or the console. An unparseable value is
// passed through: this process already dialed it, and nothing here could
// establish which part of a non-URL is a credential.
func safeBaseURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

// deliver writes one of the job's artifacts into the output directory proper,
// at the same relative path it had in the scratch tree — the mirror
// hierarchy read in the delivering direction (ARCHITECTURE.md §12).
//
// The bytes are the store's, verbatim: this is a copy out, not a second
// rendering of the artifact, so the delivered file and the one the stamp
// proves are the same bytes.
func deliver(out, rel string, data []byte) (string, error) {
	path := filepath.Join(out, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), pipeline.CreateDirMode); err != nil {
		return "", fmt.Errorf("%s: create %s: %w", devRefineVerb, filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, pipeline.CreateFileMode); err != nil {
		return "", fmt.Errorf("%s: write %s: %w", devRefineVerb, path, err)
	}
	return path, nil
}

// devRefineAsk resolves the effort this run asks with, and reports whether the
// declaration was overridden.
//
// Both halves are reported because both are evidence: a run.json saying
// thinking was on means one thing if that is what the definition declares and
// another if an operator asked for it, and a reader looking at two runs of the
// same document has no other way to tell which one was the experiment.
func devRefineAsk(override *bool) (model.RequestEffort, bool) {
	if override == nil {
		return devRefineEffort, false
	}
	return model.DeclareEffort(model.RequestEffort{Thinking: *override}), true
}

// devRefineThinking reads the tri-state --thinking override off a command's
// flags.
//
// The flag's VALUE means nothing unless it was GIVEN, so "unset" is the
// pointer being nil rather than a third bool value: a flag whose default is
// the definition's own declaration cannot distinguish "left alone" from
// "asked for that value" any other way. `--thinking=false` against a
// declaration of false is an override and has to record as one, which is the
// row an A/B of one document turns on and the only row a plumbing that read
// the value alone would get wrong.
//
// It is a function rather than four lines in RunE so that the reading is
// testable without a process, a network and a configuration directory.
func devRefineThinking(flags *pflag.FlagSet) *bool {
	if !flags.Changed(devRefineThinkingFlag) {
		return nil
	}
	return &devRefineFlagThinking
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
	return ingest.New([]ingest.SourceDoc{{Path: filepath.Base(path), Bytes: b}})
}

// reportBoundaries prints one line per boundary and returns how many moved.
//
// Moved-or-kept is the whole of what a caller outside the stage can say about
// an individual boundary: which ones fell back to the mechanical cut, and
// which were rejected once and retried, are the fold's own records and land in
// the log at info (see dissect.Refiner.accept/reject/fallback). A fallback
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
func reportSections(w io.Writer, src []byte, est tokens.Estimator, cuts []survey.Span) {
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
// pipeline.JobResult reports fallbacks (FallbackCount) and not model attempts,
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
	UploadSHA256     string `json:"sourceSha256"`
	BudgetTokens     int    `json:"budgetTokens"`
	// Sections is one field rather than a before/after pair: the fold
	// re-tiles a span and never re-sizes it, which is the same property
	// dissect.Refiner's stage-constant reference buffer depends on, so the
	// two numbers were equal by construction and a reader could only ever
	// have compared them to itself.
	Sections         int    `json:"sections"`
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

// writeRunRecord writes the run record into the output directory and returns
// its path. Owner-only, like everything else kbase writes there.
func writeRunRecord(dir string, rec devRefineRun) (string, error) {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("%s: encode %s: %w", devRefineVerb, devRefineRecordName, err)
	}
	return deliver(dir, devRefineRecordName, append(data, '\n'))
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
	devRefineFlagKeep     bool
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
provider it was pointed at. --out is required too, and is where the run
delivers what it produced: the composed cut list and a run record. Nothing
already in that directory is touched — every intermediate the job writes lives
under <out>/temp-work/, which kbase creates and, on a successful run, removes.
A failed or interrupted run keeps it; --keep-temp-work (or [dev]
keep_temp_work) keeps it after a successful one too. The per-boundary outcomes
are logged at info.

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
		return runDevRefine(cmd.Context(), devRefineOptions{
			providerOptions: opts,
			File:            args[0],
			Out:             devRefineFlagOut,
			Budget:          devRefineFlagBudget,
			Thinking:        devRefineThinking(cmd.Flags()),
			// The flag turns keeping ON and cannot turn it off: the only
			// reason to insist on deletion is disk, and that remedy is one
			// `rm -r` away, while the reason to keep is a run whose evidence
			// someone wants and cannot get back.
			KeepTempWork: devRefineFlagKeep || opts.Config.Dev.KeepTempWork,
			Stdout:       os.Stdout,
			Logger:       processLog.logger,
		})
	},
}

func init() {
	devRefineCmd.Flags().StringVar(&devRefineFlagOut, "out", "",
		"output directory for the composed cut list and "+devRefineRecordName+" (required; created if missing)")
	devRefineCmd.Flags().BoolVar(&devRefineFlagKeep, devRefineKeepFlag, false,
		"keep <out>/"+pipeline.TempWorkDirName+"/ after a run that succeeded (a failed run always keeps it)")
	devRefineCmd.Flags().IntVar(&devRefineFlagBudget, "budget", devRefineBudget,
		"per-section token budget the mechanical splitter fills toward")
	devRefineCmd.Flags().BoolVar(&devRefineFlagThinking, devRefineThinkingFlag, devRefineEffort.Thinking,
		"override the refinement definition's declared thinking mode (default: what it declares)")
	rootCmd.AddCommand(devRefineCmd)
}
