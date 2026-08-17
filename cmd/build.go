package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/assemble"
	"kbase/internal/config"
	"kbase/internal/dissect"
	"kbase/internal/distill"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/pipeline"
	"kbase/internal/summarize"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
	"kbase/internal/taxonomy"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
	"kbase/internal/version"
)

// build composes §10's plan table into one job and delivers a knowledge base
// out of a corpus. It is THE verb: what a user runs, and what an integration
// test runs, because those have to be the same thing.
//
// Stages 3 and 6 are model work against the configured provider: the
// container-descent fold designs the tree (internal/taxonomy, heavy tier,
// no-fallback seam) and the level stages write the summaries
// (internal/summarize, heavy tier, no-fallback seam). Configuration is
// therefore required in the ordinary path — an unconfigured tier refuses
// before the job directory is opened rather than failing halfway through it.
//
// `[dev] tree_plan = "mechanical"` (config.TreePlanMechanical) is the one way
// out of that, and it exists for a stated reason rather than as a mode: with
// taxonomy a ruled no-fallback seam, a hermetic end-to-end run of the
// delivered binary has no other entrance. Under it the tree is grouped by
// treeplan.SourceStructureProposal — the source's own file structure — no
// provider is dialed at all, and every index renders with its framing and
// conclusions blocks absent, which O-1's grammar makes legal rather than
// broken. The run record says the run took that route.
//
// Stage 4 is model work too, on the other tier and on the other kind of seam:
// a LIVE run's cut lists are the boundary-refinement fold's (internal/dissect,
// light tier, fallback-backed), so the light tier is required exactly as the
// heavy one is. Under the mechanical switch the cuts stage stays
// dissect.Split's own output, which is what "no model call anywhere" means.
const (
	buildVerb = "build"

	// The two job frames — slot 1, job-constant (§7). A frame is the first
	// thing a model reads, so the mechanical one says what it is and is never
	// sent, while the live one describes the job the calls belong to.
	buildFrame     = "kbase build: assembling a knowledge base with no model in the loop."
	buildLiveFrame = "kbase build: turning a documentation corpus into a knowledge base — " +
		"a navigable tree of verbatim pages under sections that summarise them."

	// buildRecordName is the run record, written at the temp-work ROOT — the
	// temp mirror of the delivered tree, so the mirror's root is where the
	// delivered tree's root would have held it (ruled 2026-08-15). It is a
	// development record and nothing reads it back, and dev-grade records live
	// in the dev mirror: `--out` holds only what a consumer of the knowledge
	// base uses. The consequence is deliberate — a run without
	// --keep-temp-work keeps no record, because §12's teardown takes it with
	// the rest of temp-work.
	buildRecordName = "run.json"

	// The store layout — §10's plan table, which is the composing verb's to
	// declare. Every stage below writes at one of these prefixes and every
	// consumer reads from them, so the layout is stated once, here, and the
	// packages that produce artifacts take their prefix as a parameter.
	surveyUnit   = "survey/survey.json"
	treePlanUnit = "treeplan/treeplan.json"
	cutsDir      = "cuts"
	leavesDir    = "leaves"
	summariesDir = "summaries"
	treeDir      = "tree"
	verifyUnit   = "verify/report.json"

	// cutListName is the file a group's cut list lands in, under
	// `cuts/{group}/`. It is the same name stage 4's refiner writes, because
	// the refiner replaces this stage's mechanical list without moving it.
	cutListName = "cutlist.txt"

	// Stage names. They name the lane, the store directory and the log
	// records, so a failure names one thing rather than three.
	stageSurvey    = "survey"
	stageTreePlan  = "treeplan"
	stageCuts      = "cuts"
	stageLeaves    = "leaves"
	stageSummaries = "summaries"
	stageAssemble  = "assemble"
	stageVerify    = "verify"

	// corpusInput and paramsInput name the two derived hashes every unit of
	// this job is stamped against: the identity of the source bytes, and the
	// operating point they were built under.
	corpusInput = "corpus"
	paramsInput = "params"

	// buildDateLayout is the provenance receipt's date format (SPEC §4.6).
	buildDateLayout = "2006-01-02"

	// kbTitlePrefix opens the knowledge base's own title. A KB is named for the
	// documentation it was built from — the thing a reader already knows the
	// name of — and says that it is the kbase rendering of it rather than the
	// documentation itself.
	kbTitlePrefix = "KBase for "

	// treePlanModel and treePlanMechanical are what the run record says about
	// where the tree plan came from. It is recorded on every run, not just the
	// dev one: "this tree was designed by a model" is the more load-bearing of
	// the two statements, and a field that appears only in the odd case reads
	// as an anomaly rather than as provenance.
	treePlanModel      = "model"
	treePlanMechanical = config.TreePlanMechanical + " (dev)"

	// keepTempWorkFlag turns off the successful run's temp-work teardown for
	// one run, over whatever `[dev] keep_temp_work` says.
	keepTempWorkFlag = "keep-temp-work"

	// buildTimeout bounds the WHOLE live job rather than one call: tens of
	// heavy-tier calls — one per container of the descent, one per section of
	// the tree — plus the transport policy's own retries underneath each of
	// them. It is generous because the failure it exists to catch is a
	// provider that has stopped answering, not one that is slow.
	buildTimeout = 2 * time.Hour
)

// buildOptions is the resolved input of the build verb.
type buildOptions struct {
	// Root is the corpus directory to build from.
	Root string

	// Title is the original doc set's title as the user states it (--title):
	// "Rojo v7 Documentation", not the directory that happens to hold it. Empty
	// falls back to the corpus directory's base name, which is the only name
	// the verb has when nobody gave it one (resolveTitle).
	Title string

	// Live is the provider pool and configuration. Nil selects the mechanical
	// tree plan — `[dev] tree_plan` — and nil is the honest value for it:
	// wiring the pool into a run that makes no call would invent a failure
	// mode ("no provider selected") on a job that never dials one.
	Live *providerOptions

	// Out is the OUTPUT directory: the delivered knowledge base lands here,
	// and `<out>/temp-work/` holds everything transient while the job runs —
	// the stage artifacts, their stamps, the job lock and the run record
	// (ARCHITECTURE.md §12). Nothing already in it is touched.
	Out string

	// Annexes are the corpus-relative prefixes declared by --annex:
	// territories deliberately not distilled (§2.8). They are a property of
	// the corpus, so they are the invocation's to state — and configuration's
	// alone, since a model that can place material out of scope can hide its
	// own failures (O-12).
	Annexes []string

	// KeepTempWork keeps `<out>/temp-work/` after a run that succeeded. A
	// failed or interrupted run keeps it whatever this says.
	KeepTempWork bool

	// BuildDate stamps the provenance receipt on every delivered page. It is
	// an option rather than a clock read inside the renderer so that the
	// rendered tree stays a pure function of its inputs (§5) and stage 9's
	// leaf re-derivation compares equal bytes; empty means today, UTC.
	BuildDate string

	// Stdout takes the verb's report, Stderr its failure detail, Logger the
	// pipeline's own diagnostics.
	Stdout io.Writer
	Stderr io.Writer
	Logger log.Logger
}

// buildResult is what a completed run produced — returned for the tests,
// which assert over it rather than over parsed stdout.
type buildResult struct {
	Plan   treeplan.TreePlan
	Report assemble.Report
	Job    pipeline.JobResult
	// Delivered is every path written under <out>, tree-relative.
	Delivered []string
}

// runBuild ingests a corpus and builds a knowledge base out of it.
//
// The order of the refusals is the same one every other verb keeps: everything
// refusable offline is refused before the job directory is opened, so a
// misconfigured run leaves nothing behind to clean up.
func runBuild(ctx context.Context, opts buildOptions) (buildResult, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, buildVerb); err != nil {
		return buildResult{}, err
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		return buildResult{}, fmt.Errorf("%s: a corpus directory is required", buildVerb)
	}
	out := strings.TrimSpace(opts.Out)
	if out == "" {
		return buildResult{}, fmt.Errorf(
			"%s: --out is required; this verb delivers a knowledge base and never uses a temporary directory", buildVerb)
	}
	annexes := declaredAnnexes(opts.Annexes)
	title := resolveTitle(opts.Title, root)
	lg := opts.Logger
	if lg == nil {
		lg = log.Discard()
	}
	buildDate, err := resolveBuildDate(opts.BuildDate)
	if err != nil {
		return buildResult{}, err
	}
	// The delivery precondition, refused before the corpus is read and before
	// the output directory is so much as created (§3.1) [MAD2: B-7].
	if err := checkOutIsClear(out); err != nil {
		return buildResult{}, err
	}
	if err := os.MkdirAll(out, pipeline.CreateDirMode); err != nil {
		return buildResult{}, fmt.Errorf("%s: create %s: %w", buildVerb, out, err)
	}

	// Ingest and survey run in process, at job setup: §10's table has ingest
	// contributing the corpus hash as a named input rather than an artifact,
	// and the survey artifact is needed by the tree-plan verifier and the
	// rebase map before any stage has run. The survey stage below writes the
	// same value to the store, which is what puts it in the chain.
	corpus, err := ingest.Walk(root, markdown.Extensions(), lg)
	if err != nil {
		return buildResult{}, err
	}
	est := tokens.Estimator{}
	art, err := markdown.Survey(corpus, est, lg)
	if err != nil {
		return buildResult{}, err
	}

	// The budgets are the shipped ones (ARCHITECTURE §9). They are not a flag:
	// every one of them is a calibrated constant with a guarantee hanging off
	// it — G-1 bounds stage 4's and stage 5's per-call input, G-2 bounds stage
	// 6's — so a number a user could pass is a number that could quietly
	// invalidate a guarantee the pipeline states elsewhere.
	params := treeplan.DefaultParams()
	params.Est = est
	verifier, err := treeplan.NewVerifier(art, corpus, params)
	if err != nil {
		return buildResult{}, err
	}
	prov := distill.Provenance{CorpusHash: art.Corpus.ContentHash, BuildDate: buildDate}
	if err := prov.Validate(); err != nil {
		return buildResult{}, err
	}
	// The declared annexes are checked against the survey HERE, at job setup,
	// and not where Compose checks them again at the end of the taxonomy fold:
	// a prefix that names nothing must cost no calls (§2.8).
	// The refusal is wrapped with the flag's name rather than restated here:
	// treeplan owns what a legal prefix is, and the user needs to know which
	// input to fix.
	if err := verifier.CheckAnnexes(annexes); err != nil {
		return buildResult{}, fmt.Errorf("%s: --annex: %w", buildVerb, err)
	}

	// The live half, dialed only after everything refusable offline has been
	// refused — a misconfigured run costs no tokens. The runner stays nil for
	// a mechanical build, and that is the honest value: every stage of that
	// plan is Produce tasks, so no call is ever built and a stub runner would
	// be a client this verb does not have standing where the thing it does not
	// use goes.
	var (
		runner *pipeline.CallRunner
		live   buildLive
		runCtx = ctx
	)
	if opts.Live != nil {
		// Both tiers, both required, both refused here — before the job
		// directory is opened. A live build asks the heavy tier to design the
		// tree and write the summaries and the light tier to adjudicate every
		// page boundary; a tier that is unmapped would abort a worker three
		// stages in, having already spent the other tier's tokens.
		id, ok := opts.Live.Config.ModelFor(config.TierHeavy)
		if !ok {
			return buildResult{}, fmt.Errorf(
				"%s: no model is configured for the %q tier in %s; taxonomy design and summaries are heavy-tier work",
				buildVerb, config.TierHeavy, config.ConfigFileName)
		}
		lightID, ok := opts.Live.Config.ModelFor(config.TierLight)
		if !ok {
			return buildResult{}, fmt.Errorf(
				"%s: no model is configured for the %q tier in %s; page-boundary refinement is light-tier work",
				buildVerb, config.TierLight, config.ConfigFileName)
		}
		name, client, callCtx, release, err := opts.Live.dial(ctx)
		if err != nil {
			return buildResult{}, err
		}
		defer release()
		live = buildLive{
			Provider:      name,
			BaseURL:       safeBaseURL(opts.Live.Providers[name].BaseURL),
			Model:         id,
			Tier:          config.TierHeavy,
			LightModel:    lightID,
			Thinking:      taxonomy.Effort.Thinking,
			RetryThinking: taxonomy.Retry.Effort.Thinking,
		}
		runner, runCtx = pipeline.NewCallRunner(client, opts.Live.Config, lg), callCtx
		fmt.Fprintf(opts.Stdout, "provider: %s (%s)\nmodel: %s (%s tier), %s (%s tier)\n",
			live.Provider, live.BaseURL, live.Model, live.Tier, live.LightModel, config.TierLight)
	}

	work, err := pipeline.OpenTempWork(out, lg)
	if err != nil {
		return buildResult{}, err
	}
	store := work.ArtifactStore()
	job := &buildJob{
		opts: opts, out: out, corpus: corpus, art: art, params: params,
		verifier: verifier, prov: prov, est: est, store: store, lg: lg,
		annexes: annexes, title: title, live: opts.Live != nil,
		inputs: []pipeline.Input{
			{Name: corpusInput, Hash: art.Corpus.ContentHash},
			// The operating point every unit is stamped against. The annex
			// prefixes are in it because changing them changes the tree
			// (§3.1), and the tree-plan source is in it because the two
			// shapes produce different tree plans from the same corpus at the
			// same budgets — without it, switching modes over one --out would
			// reuse the other mode's artifact and call it fresh. The resolved
			// title is in it for the same reason: it names the entry-point in
			// the tree plan, and a resume that did not see it change would
			// deliver the old name out of a proven-fresh artifact.
			{Name: paramsInput, Hash: pipeline.HashBytes([]byte(fmt.Sprintf(
				"leafTokens=%d;buildDate=%s;treePlan=%s;annexes=%s;title=%s;version=%s",
				params.Budgets.LeafTokens, buildDate, treePlanSource(opts.Live != nil),
				strings.Join(annexPrefixList(annexes), ","), title, version.Current)))},
		},
	}
	if job.live {
		if err := job.wireModelStages(corpusTitle(title), corpusScope(title)); err != nil {
			return buildResult{}, err
		}
		live.TaxonomyCalls = job.designer.Calls()
		fmt.Fprintf(opts.Stdout, "taxonomy: %d container calls\n", live.TaxonomyCalls)
	}

	jobPlan, err := job.plan()
	if err != nil {
		return buildResult{}, err
	}
	coord := pipeline.NewCoordinator(store, runner, pipeline.DefaultWorkers, lg)
	start := time.Now()
	res, err := coord.Run(runCtx, jobPlan, pipeline.ModeResume)
	elapsed := time.Since(start)
	// Every return from here down keeps temp-work: a failed or interrupted run
	// leaves its intermediates for a resume and for a human.
	if err != nil {
		return buildResult{}, err
	}
	for _, f := range res.Failures {
		fmt.Fprintf(opts.Stderr, "unit %s failed (%s): %v\n", f.Path, f.Kind, f.Err)
	}
	if !res.DeliveryReady() {
		return buildResult{}, fmt.Errorf("%s: the build did not complete: %d of %d units produced, %d failed",
			buildVerb, res.Produced, res.Units, len(res.Failures))
	}

	plan, err := job.readTreePlan()
	if err != nil {
		return buildResult{}, err
	}
	report, err := job.readReport()
	if err != nil {
		return buildResult{}, err
	}
	delivered, err := job.deliver(plan)
	if err != nil {
		return buildResult{}, err
	}

	summaries, err := job.summaryCount(plan)
	if err != nil {
		return buildResult{}, err
	}
	record := buildRun{
		Version:     version.Current,
		Corpus:      root,
		Title:       title,
		CorpusHash:  art.Corpus.ContentHash,
		BuildDate:   buildDate,
		TreePlan:    treePlanSource(job.live),
		Annexes:     annexPrefixList(annexes),
		Budgets:     plan.Budgets,
		Files:       art.Corpus.Files,
		Sections:    art.Corpus.Sections,
		Excluded:    len(corpus.Excluded),
		Exclusions:  corpus.Excluded,
		Links:       art.Corpus.Links,
		Nodes:       len(plan.Nodes),
		Leaves:      report.Leaves,
		Indexes:     report.Indexes,
		Groups:      len(plan.Groups),
		SplitGroups: splitGroupCount(plan),
		Summaries:   summaries,
		Delivered:   len(delivered),
		Units:       res.Units,
		Produced:    res.Produced,
		Reused:      res.Reused,
		ElapsedMS:   elapsed.Milliseconds(),
		Verify:      report.Checks,
	}
	if job.live {
		// Only a live run has a model, a tier and a token bill; a mechanical
		// one would report zeroes, which read as "it cost nothing" rather than
		// as "nothing was asked".
		if err := job.readRefinement(); err != nil {
			return buildResult{}, err
		}
		live.BoundariesAdjudicated = job.refined.Boundaries
		live.BoundariesMoved = job.refined.Moved
		live.BoundariesFellBack = job.refined.Fallbacks
		live.BoundaryRejections = job.refined.Rejections
		live.CallsSkipped = res.Skipped
		live.PromptTokens = res.Usage.PromptTokens
		live.CachedTokens = res.Usage.CachedPromptTokens
		live.CompletionTokens = res.Usage.CompletionTokens
		live.ReasoningTokens = res.Usage.ReasoningTokens
		record.Live = &live
	}
	recordPath, err := writeBuildRecord(work.Root(), record)
	if err != nil {
		return buildResult{}, err
	}
	// Reported only when temp-work survives: the teardown below is about to
	// take the record with it, and a path this run is deleting would already be
	// wrong by the time anyone read the line.
	if !opts.KeepTempWork {
		recordPath = ""
	}
	job.report(plan, report, res, elapsed, len(delivered), summaries, recordPath)

	if opts.KeepTempWork {
		fmt.Fprintf(opts.Stdout, "temp work kept: %s\n", work.Root())
		return buildResult{Plan: plan, Report: report, Job: res, Delivered: delivered}, nil
	}
	if err := work.Discard(); err != nil {
		return buildResult{}, err
	}
	return buildResult{Plan: plan, Report: report, Job: res, Delivered: delivered}, nil
}

// buildJob is the job's composition: everything the six stages share, in one
// value, so a stage description is a method rather than a closure over a dozen
// locals.
type buildJob struct {
	opts     buildOptions
	out      string
	corpus   ingest.Corpus
	art      survey.Artifact
	params   treeplan.Params
	verifier *treeplan.Verifier
	prov     distill.Provenance
	est      tokens.Estimator
	store    *pipeline.ArtifactStore
	inputs   []pipeline.Input
	lg       log.Logger

	// annexes are the declared territories, validated against the survey at
	// job setup. They reach the artifact through whichever stage 3 shape runs
	// — the descent carries them, and the mechanical proposal hands them to
	// the same Compose.
	annexes []treeplan.Annex

	// title is the resolved doc-set title (resolveTitle), from which both
	// stage-3 shapes name the entry-point. Resolved once, at job setup, so the
	// two shapes cannot name one corpus two ways.
	title string

	// live says whether stages 3, 4 and 6 ask a model. The two model stages
	// built here are nil when it is false, and the plan below is then the
	// mechanical spine.
	live       bool
	designer   *taxonomy.Designer
	summarizer *summarize.Summarizer

	// folds are stage 4's per-group refiners, kept from the moment the stage
	// resolved its work so the run record can report what they did. Written
	// once, when the coordinator describes the cuts stage, and read once the
	// job has finished — never concurrently with the workers that drive them.
	// refined is that reading (readRefinement).
	folds   []*dissect.Refiner
	refined dissect.Stats
}

// wireModelStages builds the two no-fallback seams this verb can run: the
// container descent that designs the tree, and the level stages that summarise
// it.
//
// Both are built here, at the composition root, because that is where a stage
// is registered and therefore where its declared effort and retry policy are
// threaded from (ARCHITECTURE.md §9, §12) — the values are the definitions' own
// (taxonomy.Effort/taxonomy.Retry, summarize.Effort/summarize.Retry), passed
// through untouched.
func (j *buildJob) wireModelStages(title, scope string) error {
	designer, err := taxonomy.New(taxonomy.Design{
		Survey:   j.art,
		Verifier: j.verifier,
		Params:   j.params,
		Title:    title,
		Scope:    scope,
		Annexes:  j.annexes,
		Unit:     treePlanUnit,
		Effort:   taxonomy.Effort,
		Retry:    taxonomy.Retry,
	}, j.lg)
	if err != nil {
		return err
	}
	summarizer, err := summarize.New(summarize.Job{
		TreePlan:     j.readTreePlan,
		Store:        j.store,
		LeavesDir:    leavesDir,
		SummariesDir: summariesDir,
		Params:       j.params,
		Effort:       summarize.Effort,
		Retry:        summarize.Retry,
	}, j.lg)
	if err != nil {
		return err
	}
	j.designer, j.summarizer = designer, summarizer
	return nil
}

// plan is §10's table: survey, tree plan, cuts, pages, summaries, assemble,
// verify — with stages 3, 4 and 6 asking a model when this run is live and
// deriving their artifacts mechanically when it is not.
func (j *buildJob) plan() (pipeline.Plan, error) {
	frame := buildFrame
	if j.live {
		frame = buildLiveFrame
	}
	cuts, err := j.cutsStage()
	if err != nil {
		return pipeline.Plan{}, err
	}
	stages := []*pipeline.StagePlan{
		j.surveyStage(), j.treePlanStage(), cuts, j.leavesStage(),
	}
	stages = append(stages, j.summaryStages()...)
	stages = append(stages, j.assembleStage(), j.verifyStage())
	return pipeline.Plan{JobFrame: frame, Stages: stages}, nil
}

// summaryStages is stage 6: one stage per index level, deepest first, and none
// at all when no model is in the loop.
func (j *buildJob) summaryStages() []*pipeline.StagePlan {
	if j.summarizer == nil {
		return nil
	}
	return j.summarizer.StagePlans(stageSummaries, func(unit string, upstreams []string) pipeline.OwedArtifact {
		return j.owed(unit, append([]string{treePlanUnit}, upstreams...)...)
	})
}

// owed describes one unit: the job's two derived inputs plus whatever store
// artifacts it consumed.
func (j *buildJob) owed(path string, upstreams ...string) pipeline.OwedArtifact {
	return pipeline.OwedArtifact{Path: path, Inputs: j.inputs, Upstreams: upstreams}
}

// oneLane is the stage shape for a stage with a single unit.
func oneLane(name string, task pipeline.LaneTask) pipeline.LaneResolver {
	return func() ([]pipeline.SerialLane, error) {
		return []pipeline.SerialLane{{Domain: name, Tasks: []pipeline.LaneTask{task}}}, nil
	}
}

// surveyStage writes the survey artifact the rest of the chain is stamped
// against. Its producer hands over the value computed at job setup rather than
// re-surveying: one survey per run, and the artifact is the record of it.
func (j *buildJob) surveyStage() *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name: stageSurvey,
		Lanes: oneLane(stageSurvey, pipeline.LaneTask{
			Owed:    j.owed(surveyUnit),
			Section: stageSurvey,
			Produce: func() (any, error) { return j.art, nil },
		}),
		Encode: func(artifact any) ([]byte, error) {
			a, ok := artifact.(survey.Artifact)
			if !ok {
				return nil, fmt.Errorf("%s: artifact is %T, not a survey artifact", buildVerb, artifact)
			}
			var buf bytes.Buffer
			if err := a.WriteJSON(&buf); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		},
	}
}

// treePlanStage is stage 3: the container descent when a model is in the loop,
// and the dev/baseline proposal when one is not.
//
// The two shapes share everything but the proposal. The verifier is the
// shipped one either way — §2.7's operators, §2.4's split expansion, the namer
// and every §3.3 post-condition run over both — so what the mechanical shape
// skips is the model that would have chosen the grouping, not any of the
// checking that grouping has to survive.
func (j *buildJob) treePlanStage() *pipeline.StagePlan {
	if j.designer != nil {
		return j.designer.StagePlan(stageTreePlan, j.inputs, surveyUnit)
	}
	return &pipeline.StagePlan{
		Name: stageTreePlan,
		Lanes: oneLane(stageTreePlan, pipeline.LaneTask{
			Owed:    j.owed(treePlanUnit, surveyUnit),
			Section: stageTreePlan,
			Produce: func() (any, error) {
				proposal, err := treeplan.SourceStructureProposal(
					j.art, corpusTitle(j.title), corpusScope(j.title), j.annexes)
				if err != nil {
					return nil, err
				}
				return j.verifier.Compose(proposal, j.annexes)
			},
		}),
		// The same encoder the live stage uses: one artifact, one encoder,
		// whichever stage produced it.
		Encode: taxonomy.Encode,
	}
}

// cutsStage is stage 4: one cut list per SPLIT group — and none at all when no
// group splits, which is the ordinary outcome at the shipped budget over a
// corpus of documentation-sized sections.
//
// Two shapes, one artifact. A LIVE run adjudicates every interior boundary
// against the light tier (§5's serial fold, one lane per group), and a
// mechanical one writes dissect.Split's own output, which is always valid by
// construction and is what the fold falls back to boundary by boundary. The
// refined stage does not move the artifact: same path, same encoder, same
// contract, boundaries adjudicated instead of taken — which is why the stages
// downstream of it cannot tell the two shapes apart.
func (j *buildJob) cutsStage() (*pipeline.StagePlan, error) {
	if !j.live {
		return j.mechanicalCutsStage(), nil
	}
	return dissect.StagePlan(stageCuts, dissect.CutJob{
		// The refinement definition's own declaration, passed through
		// untouched — this is the site that registers the stage, and both
		// halves of the ask are its to state (ARCHITECTURE.md §9, §12).
		Effort: dissect.Effort,
		Retry:  dissect.Retry,
		Folds:  j.cutFolds,
		Owed: func(unit string) pipeline.OwedArtifact {
			return j.owed(unit, treePlanUnit, surveyUnit)
		},
	}, j.lg)
}

func (j *buildJob) mechanicalCutsStage() *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name:   stageCuts,
		Encode: dissect.EncodeCutList,
		Lanes: func() ([]pipeline.SerialLane, error) {
			groups, err := j.splitGroups()
			if err != nil {
				return nil, err
			}
			var tasks []pipeline.LaneTask
			for _, g := range groups {
				unit := cutListPath(g.ID)
				tasks = append(tasks, pipeline.LaneTask{
					Owed:    j.owed(unit, treePlanUnit, surveyUnit),
					Section: stageCuts,
					Produce: j.cutProducer(g, unit),
				})
			}
			if len(tasks) == 0 {
				j.lg.Info("no group needs cutting", "stage", stageCuts)
				return nil, nil
			}
			return []pipeline.SerialLane{{Domain: stageCuts, Tasks: tasks}}, nil
		},
	}
}

func (j *buildJob) cutProducer(g treeplan.SplitGroup, unit string) pipeline.Producer {
	return func() (any, error) {
		src, err := j.groupSource(g)
		if err != nil {
			return nil, err
		}
		cuts, err := dissect.Split(src.bytes, src.span, src.cands, src.params)
		if err != nil {
			return nil, err
		}
		return dissect.CutList{Unit: unit, Cuts: cuts}, nil
	}
}

// cutFolds is the live stage's lazy half: one Refiner per split group, built
// when the coordinator reaches stage 4 — the first moment the tree plan that
// says which groups split is on disk (§12's lazy chain).
//
// The folds are kept, because they are the only place the run's refinement
// outcome exists: the fold logs each boundary as it adjudicates it, and the
// run record needs the totals (see refinement).
func (j *buildJob) cutFolds() ([]*dissect.Refiner, error) {
	groups, err := j.splitGroups()
	if err != nil {
		return nil, err
	}
	folds := make([]*dissect.Refiner, 0, len(groups))
	for _, g := range groups {
		src, err := j.groupSource(g)
		if err != nil {
			return nil, err
		}
		mech, err := dissect.Split(src.bytes, src.span, src.cands, src.params)
		if err != nil {
			return nil, err
		}
		// The unit DIRECTORY, not the cut list's path: the fold names its own
		// artifact inside it, so the refined list lands exactly where the
		// mechanical one would have.
		r, err := dissect.NewRefiner(src.bytes, src.span, src.cands, mech,
			cutsDir+"/"+g.ID, src.params, dissect.Effort, dissect.Retry, j.lg)
		if err != nil {
			return nil, err
		}
		folds = append(folds, r)
	}
	j.folds = folds
	return folds, nil
}

// splitGroups is the tree plan's groups that need cutting at all — the ones
// stage 3 sized into more than one page. Both cuts shapes read it, so "which
// groups does stage 4 work on" is one answer rather than two that could drift.
func (j *buildJob) splitGroups() ([]treeplan.SplitGroup, error) {
	plan, err := j.readTreePlan()
	if err != nil {
		return nil, err
	}
	var out []treeplan.SplitGroup
	for _, g := range plan.Groups {
		if g.Parts > 1 {
			out = append(out, g)
		}
	}
	return out, nil
}

// groupSource is everything stage 4 needs about one split group: the custody
// bytes, the span the group covers, the cut candidates the adapter enumerated
// inside the document, and the budget the group was sized to.
//
// One resolver for both shapes. The mechanical producer and the fold's seed
// must be the same material or the fold would be improving on a list nothing
// else would have produced.
type groupSource struct {
	bytes  []byte
	span   survey.Span
	cands  []survey.CutCandidate
	params dissect.Params
}

func (j *buildJob) groupSource(g treeplan.SplitGroup) (groupSource, error) {
	doc, ok := j.corpus.Doc(g.Source.File)
	if !ok {
		return groupSource{}, fmt.Errorf("%s: group %s draws on %s, which is not under custody",
			buildVerb, g.ID, g.Source.File)
	}
	f, ok := surveyFile(j.art, g.Source.File)
	if !ok {
		return groupSource{}, fmt.Errorf("%s: group %s draws on %s, which the survey does not describe",
			buildVerb, g.ID, g.Source.File)
	}
	return groupSource{
		bytes:  doc.Bytes,
		span:   survey.Span{Start: g.Source.Start, End: g.Source.End},
		cands:  f.Cuts,
		params: dissect.Params{Est: j.est, BudgetTokens: g.Budget},
	}, nil
}

// readRefinement collects what the fold did, summed over every group: the
// boundaries it adjudicated, how many it moved, and how many kept the
// mechanical cut because no answer verified.
//
// Read ONCE, after the job, which is the only moment it is readable:
// Refiner.Stats refuses while a fold is running, and the coordinator's return
// is the point at which every worker has exited. It is then read from the
// field by both the record and the console report, so the artifact and the
// terminal cannot state different numbers.
//
// A run with no split group has no fold and reports zeroes — the honest
// reading of "there was no boundary for anyone to choose", not of "the stage
// did not run".
func (j *buildJob) readRefinement() error {
	var total dissect.Stats
	for _, r := range j.folds {
		s, err := r.Stats()
		if err != nil {
			return fmt.Errorf("%s: reading the boundary fold's outcome: %w", buildVerb, err)
		}
		total.Boundaries += s.Boundaries
		total.Moved += s.Moved
		total.Fallbacks += s.Fallbacks
		total.Rejections += s.Rejections
	}
	j.refined = total
	return nil
}

// leavesStage is stage 5: one Produce task per leaf, lanes per domain, units
// resolved from the tree plan ∪ the cut lists — ARCHITECTURE §12's lazy chain
// description, where stage 4's verified cut list defines stage 5's work.
func (j *buildJob) leavesStage() *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name:   stageLeaves,
		Encode: distill.Encode,
		Lanes: func() ([]pipeline.SerialLane, error) {
			plan, cuts, err := j.readPlanAndCuts()
			if err != nil {
				return nil, err
			}
			d, err := distill.New(plan, j.art, j.corpus, cuts, j.prov)
			if err != nil {
				return nil, err
			}
			return d.Lanes(leavesDir, func(n treeplan.Node) pipeline.OwedArtifact {
				up := []string{treePlanUnit, surveyUnit}
				if g, ok := plan.SplitGroup(n.SplitGroup); ok && g.Parts > 1 {
					up = append(up, cutListPath(g.ID))
				}
				return j.owed(leavesDir+"/"+n.Path, up...)
			}), nil
		},
	}
}

// assembleStage is stage 8: §4's grammar over the tree plan for the index and
// entry-point kinds, a byte-for-byte copy for the leaves, and §4.7's fixture
// manifest.
//
// Leaves are COPIED. Rebasing happened once, at stage 5, and this stage does
// not touch leaf bytes [MAD1: F-9] — which is what makes the delivered page
// and the artifact stage 9 re-derives against the same bytes by construction.
func (j *buildJob) assembleStage() *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name:   stageAssemble,
		Encode: encodeBytes,
		Lanes: func() ([]pipeline.SerialLane, error) {
			plan, cuts, err := j.readPlanAndCuts()
			if err != nil {
				return nil, err
			}
			r, err := j.renderer(plan, cuts)
			if err != nil {
				return nil, err
			}
			var tasks []pipeline.LaneTask
			for _, n := range plan.Nodes {
				if n.Kind == treeplan.KindLeaf {
					tasks = append(tasks, pipeline.LaneTask{
						Owed:    j.owed(treeDir+"/"+n.Path, leavesDir+"/"+n.Path),
						Section: stageAssemble,
						Produce: j.copyProducer(leavesDir + "/" + n.Path),
					})
					continue
				}
				// A section's page is the tree plan, its summary and the cut
				// lists of any split family it lists, so it declares all
				// three: a regenerated summary or a re-adjudicated boundary
				// invalidates the page it is rendered into, for free.
				up := []string{treePlanUnit}
				if j.live {
					up = append(up, j.summarizer.Unit(n.Path))
				}
				up = append(up, splitChildCuts(plan, n.Path)...)
				tasks = append(tasks, pipeline.LaneTask{
					Owed:    j.owed(treeDir+"/"+n.Path, up...),
					Section: stageAssemble,
					Produce: renderProducer(r, n),
				})
			}
			for _, f := range r.Fixtures() {
				tasks = append(tasks, pipeline.LaneTask{
					Owed:    j.owed(treeDir+"/"+f.Path, treePlanUnit),
					Section: stageAssemble,
					Produce: constProducer(f.Data),
				})
			}
			return []pipeline.SerialLane{{Domain: stageAssemble, Tasks: tasks}}, nil
		},
	}
}

// verifyStage is stage 9: the ten gates over the assembled tree in the store,
// before any byte reaches <out> (I-5).
//
// It declares every delivered file as an upstream, which is the truthful
// statement of what it read — and what makes a re-run after one page changed
// redo the gates rather than reuse a verdict about a tree that no longer
// exists.
func (j *buildJob) verifyStage() *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name:   stageVerify,
		Encode: assemble.EncodeReport,
		Lanes: func() ([]pipeline.SerialLane, error) {
			plan, err := j.readTreePlan()
			if err != nil {
				return nil, err
			}
			up := []string{treePlanUnit, surveyUnit}
			for _, p := range deliveredPaths(plan) {
				up = append(up, treeDir+"/"+p)
			}
			return []pipeline.SerialLane{{Domain: stageVerify, Tasks: []pipeline.LaneTask{{
				Owed:    j.owed(verifyUnit, up...),
				Section: stageVerify,
				Produce: j.verifyProducer(),
			}}}}, nil
		},
	}
}

func (j *buildJob) verifyProducer() pipeline.Producer {
	return func() (any, error) {
		plan, cuts, err := j.readPlanAndCuts()
		if err != nil {
			return nil, err
		}
		r, err := j.renderer(plan, cuts)
		if err != nil {
			return nil, err
		}
		// A FRESH distiller over the corpus bytes: check 7 proves the
		// delivered page is a re-derivation of the source, so it must not read
		// the leaves artifact it is checking.
		d, err := distill.New(plan, j.art, j.corpus, cuts, j.prov)
		if err != nil {
			return nil, err
		}
		leaves := map[string]distill.Leaf{}
		for _, n := range d.Leaves() {
			l, err := d.Render(n)
			if err != nil {
				return nil, err
			}
			leaves[n.Path] = l
		}
		files := map[string][]byte{}
		for _, p := range deliveredPaths(plan) {
			data, err := j.store.Get(treeDir + "/" + p)
			if err != nil {
				return nil, err
			}
			files[p] = data
		}
		return assemble.Verify(assemble.Verification{
			Plan: plan, Renderer: r, Files: files, Leaves: leaves,
			Verifier: j.verifier, Cuts: cuts, Est: j.est,
		})
	}
}

// checkOutIsClear is the delivery precondition (SPEC §3.1, §1.5): `--out`
// receives the delivered tree and only the delivered tree, so a directory that
// already holds something is refused BEFORE the corpus is read — every path it
// holds named, nothing written, exit 1 [MAD2: B-7].
//
// It is the write-agents refusal one directory up, and for the same reason: the
// delivered set is decided by a tree plan that does not exist yet, so the only
// honest all-or-nothing check available at job setup is over the directory
// rather than over the file list. Iterative update of a delivered knowledge
// base is not a feature of this appliance, so there is nothing this refusal
// costs a user that a `rm -r` does not restore.
//
// The one exception is the run this `--out` is already in the middle of
// (interruptedJob). A killed run can leave a half-copied delivery behind — that
// residue is its own to overwrite, and refusing it would make the resume §5
// guarantees impossible to reach.
func checkOutIsClear(out string) error {
	entries, err := os.ReadDir(out)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("%s: --out %s: %w", buildVerb, out, err)
	}
	present := make([]string, 0, len(entries))
	for _, e := range entries {
		// The transient half is kbase's own and is not "already there": it is
		// either this job's resume material or a kept run's evidence, and the
		// store's own sweep and teardown are what govern it.
		if e.Name() == pipeline.TempWorkDirName {
			continue
		}
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		present = append(present, name)
	}
	if len(present) == 0 {
		return nil
	}
	resuming, why := interruptedJob(out)
	if resuming {
		return nil
	}
	return fmt.Errorf("%s: refusing to deliver into a directory that is not empty; it holds:\n%s\n"+
		"nothing was written; %s — kbase delivers a whole knowledge base or nothing and never "+
		"updates one in place, so delete %s (or name an empty --out) and build again",
		buildVerb, indentedList(present), why, out)
}

// interruptedJob reports whether `<out>` is the middle of a build this run may
// legitimately finish, and — when it is not — the clause the refusal quotes.
//
// The predicate is the temp-work tree's, never the delivered files': temp-work
// survives exactly the runs that did NOT finish (a successful run tears it
// down, §3.1), and the run record inside it is written only once a delivery has
// completed (§3.8). So "temp-work/ present and no run record in it" is
// precisely "an interrupted build left this here" — which is the state §5's
// resume reads, and the only state whose delivered residue belongs to this run.
//
// A `--keep-temp-work` run that SUCCEEDED leaves both, and that is a rerun and
// not a resume: it refuses like any other. Only a definitive absence counts as
// resume state, so a temp-work tree kbase cannot read refuses rather than
// licensing an overwrite.
func interruptedJob(out string) (bool, string) {
	work := filepath.Join(out, pipeline.TempWorkDirName)
	if _, err := os.Stat(work); err != nil {
		return false, fmt.Sprintf("there is no %s/ here, so no interrupted build owns them",
			pipeline.TempWorkDirName)
	}
	if _, err := os.Stat(filepath.Join(work, buildRecordName)); errors.Is(err, os.ErrNotExist) {
		return true, ""
	}
	return false, fmt.Sprintf("the %s/ here holds %s, so the build that wrote these finished — "+
		"this is a rerun, not the resume of an interrupted one",
		pipeline.TempWorkDirName, buildRecordName)
}

// deliver copies the proven bytes out of the store into <out> proper — the
// mirror hierarchy read in the delivering direction, with the `tree/` prefix
// dropped because that prefix is the store's own namespace and not part of the
// knowledge base's shape.
func (j *buildJob) deliver(plan treeplan.TreePlan) ([]string, error) {
	paths := deliveredPaths(plan)
	for _, p := range paths {
		data, err := j.store.Get(treeDir + "/" + p)
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(j.out, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), pipeline.CreateDirMode); err != nil {
			return nil, fmt.Errorf("%s: create %s: %w", buildVerb, filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, pipeline.CreateFileMode); err != nil {
			return nil, fmt.Errorf("%s: write %s: %w", buildVerb, dst, err)
		}
	}
	return paths, nil
}

// report prints the human summary. recordPath is empty when the run record did
// not survive the run — it lives in temp-work, which an ordinary run tears
// down.
func (j *buildJob) report(plan treeplan.TreePlan, rep assemble.Report,
	res pipeline.JobResult, elapsed time.Duration, delivered, summaries int, recordPath string) {

	w := j.opts.Stdout
	t := j.art.Corpus
	fmt.Fprintf(w, "corpus: %s (%d files, %d sections, %d tokens, sha256 %s)\n",
		j.opts.Root, t.Files, t.Sections, t.Tokens, t.ContentHash)
	if n := t.Links.Unresolved; n > 0 {
		// The one line the exemption gets. Guarantee 1 does not fail on these
		// — a destination naming a document this corpus does not hold is a
		// defect in the corpus, and refusing delivery over it would make an
		// unbuildable KB out of someone else's typo (§4.5 rule 3). What it may
		// not be is silent: the pages ship carrying the destination exactly as
		// written, and the operator is the only one who can go and look.
		fmt.Fprintf(j.opts.Stderr,
			"links: %s; %d destination(s) name no document in the corpus and are delivered verbatim, exempt from guarantee 1\n",
			linkCensus(t.Links), n)
	}
	fmt.Fprintf(w, "budget: %d tokens per page\n", plan.Budgets.LeafTokens)
	fmt.Fprintf(w, "tree plan: %d nodes (%d pages, %d sections), %d groups, %d split\n",
		len(plan.Nodes), rep.Leaves, rep.Indexes, len(plan.Groups), splitGroupCount(plan))
	if j.live {
		fmt.Fprintf(w, "boundaries: %d adjudicated, %d moved, %d fell back to the mechanical cut\n",
			j.refined.Boundaries, j.refined.Moved, j.refined.Fallbacks)
		fmt.Fprintf(w, "summaries: %d of %d sections\n", summaries, rep.Indexes)
		fmt.Fprintf(w, "usage: prompt=%d cached=%d completion=%d reasoning=%d\n",
			res.Usage.PromptTokens, res.Usage.CachedPromptTokens,
			res.Usage.CompletionTokens, res.Usage.ReasoningTokens)
	}
	fmt.Fprintln(w, "verify:")
	for _, c := range rep.Checks {
		mark := "ok  "
		if !c.OK {
			mark = "FAIL"
		}
		fmt.Fprintf(w, "  %s %d %s\n", mark, c.Number, c.Name)
	}
	fmt.Fprintf(w, "units: %d produced, %d reused, %d failed\n", res.Produced, res.Reused, len(res.Failures))
	fmt.Fprintf(w, "elapsed: %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(w, "delivered: %d files to %s\n", delivered, j.out)
	if recordPath != "" {
		fmt.Fprintf(w, "record: %s\n", recordPath)
	}
}

// renderer builds stage 8's renderer over the tree plan and whatever summaries
// stage 6 wrote.
//
// One constructor for both call sites — the assemble stage and check 5's
// re-render — because check 5 compares the delivered bytes against a fresh
// render, and a renderer holding different summaries would be comparing two
// different pages and calling the difference a defect.
func (j *buildJob) renderer(plan treeplan.TreePlan, cuts map[string][]survey.Span) (*assemble.Renderer, error) {
	summaries, err := summarize.All(j.store, summariesDir, plan)
	if err != nil {
		return nil, err
	}
	// The per-part descriptors an index's bullets carry are stage 5's, derived
	// from the same (tree plan, cut list, survey) triple the pages are: a
	// distiller is how this verb asks for them, here as at stage 9, so the
	// rendered index and its re-derivation read one implementation.
	d, err := distill.New(plan, j.art, j.corpus, cuts, j.prov)
	if err != nil {
		return nil, err
	}
	descriptors, err := d.Descriptors()
	if err != nil {
		return nil, err
	}
	return assemble.NewRenderer(plan, j.prov, summaries, descriptors)
}

// summaryCount is how many sections a summary was written for — the live
// stage's own yield, reported beside the node counts.
func (j *buildJob) summaryCount(plan treeplan.TreePlan) (int, error) {
	summaries, err := summarize.All(j.store, summariesDir, plan)
	if err != nil {
		return 0, err
	}
	return len(summaries), nil
}

// readTreePlan reads the composed tree plan back out of the store — the lazy
// chain's own idiom: a stage reads what its upstream left, rather than a value
// the verb happened to keep, so a resumed run and a fresh one read the same
// bytes.
func (j *buildJob) readTreePlan() (treeplan.TreePlan, error) {
	data, err := j.store.Get(treePlanUnit)
	if err != nil {
		return treeplan.TreePlan{}, err
	}
	return treeplan.ReadJSON(bytes.NewReader(data))
}

// readPlanAndCuts reads the tree plan and every split group's cut list.
func (j *buildJob) readPlanAndCuts() (treeplan.TreePlan, map[string][]survey.Span, error) {
	plan, err := j.readTreePlan()
	if err != nil {
		return treeplan.TreePlan{}, nil, err
	}
	cuts := map[string][]survey.Span{}
	for _, g := range plan.Groups {
		if g.Parts == 1 {
			continue
		}
		data, err := j.store.Get(cutListPath(g.ID))
		if err != nil {
			return treeplan.TreePlan{}, nil, err
		}
		list, err := dissect.DecodeCutList(data)
		if err != nil {
			return treeplan.TreePlan{}, nil, err
		}
		cuts[g.ID] = list
	}
	return plan, cuts, nil
}

// readReport reads stage 9's artifact back for the run record.
func (j *buildJob) readReport() (assemble.Report, error) {
	data, err := j.store.Get(verifyUnit)
	if err != nil {
		return assemble.Report{}, err
	}
	var rep assemble.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return assemble.Report{}, fmt.Errorf("%s: read the verify report: %w", buildVerb, err)
	}
	return rep, nil
}

// deliveredPaths is the whole delivered set — class A then class B — in a
// stable order (§4.7).
func deliveredPaths(plan treeplan.TreePlan) []string {
	out := make([]string, 0, len(plan.Nodes)+len(assemble.Manifest()))
	for _, n := range plan.Nodes {
		out = append(out, n.Path)
	}
	out = append(out, assemble.Manifest()...)
	sort.Strings(out)
	return out
}

func cutListPath(group string) string { return cutsDir + "/" + group + "/" + cutListName }

// splitChildCuts is the cut lists one index reads through its own down-links:
// a bullet naming a part of a split group carries that part's post-cut
// descriptor, which stage 4 decided. A page that renders a value declares the
// artifact the value came from, or a re-adjudicated boundary would leave a
// stale bullet behind on a resumed run.
func splitChildCuts(plan treeplan.TreePlan, parent string) []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range plan.Nodes {
		if n.Kind != treeplan.KindLeaf || n.Parent != parent {
			continue
		}
		g, ok := plan.SplitGroup(n.SplitGroup)
		if !ok || g.Parts == 1 || seen[g.ID] {
			continue
		}
		seen[g.ID] = true
		out = append(out, cutListPath(g.ID))
	}
	return out
}

func splitGroupCount(plan treeplan.TreePlan) int {
	n := 0
	for _, g := range plan.Groups {
		if g.Parts > 1 {
			n++
		}
	}
	return n
}

func surveyFile(art survey.Artifact, path string) (survey.File, bool) {
	for _, f := range art.Files {
		if f.Path == path {
			return f, true
		}
	}
	return survey.File{}, false
}

// resolveTitle is the doc set's title: --title as the user stated it, else the
// corpus directory's base name.
//
// The fallback is the raw material the old title was built from, and it is a
// fallback rather than the rule because a directory name is what the corpus was
// checked out into, not what the documentation is called.
func resolveTitle(flag, root string) string {
	if t := strings.TrimSpace(flag); t != "" {
		return t
	}
	return filepath.Base(filepath.Clean(root))
}

// corpusTitle and corpusScope name the entry-point, from the one resolved title
// (resolveTitle). Both stage-3 shapes call them, so a live tree and a mechanical
// one over the same corpus are named the same way.
func corpusTitle(title string) string { return kbTitlePrefix + title }

func corpusScope(title string) string {
	return "everything built from the " + title + " corpus"
}

// encodeBytes is the assemble stage's encoder: a rendered page IS its bytes.
func encodeBytes(artifact any) ([]byte, error) {
	b, ok := artifact.([]byte)
	if !ok {
		return nil, fmt.Errorf("%s: artifact is %T, not rendered bytes", buildVerb, artifact)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("%s: a node rendered no bytes", buildVerb)
	}
	return b, nil
}

func renderProducer(r *assemble.Renderer, n treeplan.Node) pipeline.Producer {
	return func() (any, error) { return r.Node(n) }
}

func constProducer(data []byte) pipeline.Producer {
	return func() (any, error) { return data, nil }
}

func (j *buildJob) copyProducer(unit string) pipeline.Producer {
	return func() (any, error) { return j.store.Get(unit) }
}

// buildRun is the run record written at the temp-work root: what was built,
// from what, under which budgets, and what the gates said.
//
// It is a development record rather than a stamp — nothing reads it back — and
// that is why it sits in the dev mirror rather than in the delivered tree. It
// carries every identity a later reader would otherwise reconstruct from a
// log, and the ten verify results, because those ARE the result of the run.
type buildRun struct {
	Version string `json:"version"`
	Corpus  string `json:"corpus"`
	// Title is the resolved doc-set title the knowledge base is named for —
	// --title, or the corpus directory's base name. It is recorded on every run
	// because it is an output-affecting input: it names the entry-point, and it
	// is stamped into the parameter digest that governs resume.
	Title      string `json:"title"`
	CorpusHash string `json:"corpusHash"`
	BuildDate  string `json:"buildDate"`

	// TreePlan is where stage 3's artifact came from — treePlanModel, or
	// treePlanMechanical for a run under the [dev] switch. It is stated on
	// every run: a KB whose organisation nobody designed is the one thing a
	// later reader must not have to infer from an absence.
	TreePlan string `json:"treePlan"`

	// Annexes are the declared prefixes, in the order they were declared.
	// Absent when there are none — an empty list would read as a decision to
	// annex nothing, which is not what a run without the flag made.
	Annexes []string `json:"annexes,omitempty"`

	Budgets  treeplan.Budgets `json:"budgets"`
	Files    int              `json:"sourceFiles"`
	Sections int              `json:"sourceSections"`

	// Excluded and Exclusions are the corpus DENOMINATOR: what the corpus root
	// held that the ingest walk did not take, and why (ingest.Exclusion's reason
	// classes). Without them `sourceFiles` is a count of what kbase chose to
	// look at, and a format class the walk ignores — a corpus's `.mdx` files
	// are the live case — leaves no trace anywhere [MAD2: B-3]. Disclosure is
	// this record and nothing else (ruled 2026-08-17): the knowledge base is
	// the documentation set's, not a report on the walk. The total is stated
	// beside the list because the denominator is the load-bearing half; the
	// list is omitted when a corpus root holds nothing but documents.
	Excluded   int                `json:"sourceExcluded"`
	Exclusions []ingest.Exclusion `json:"exclusions,omitempty"`

	// Links is the corpus's link census (§3.2's roll-up), recorded because it
	// is the measurement guarantee 1's exemption is taken against. A
	// destination the survey could not resolve is delivered verbatim and
	// exempted from that gate (§4.5 rule 3) — which is right, since the defect
	// is in someone else's corpus, and is only defensible while somebody can
	// see how often it happened. `internal` is the other half of the same
	// reading: a corpus with a working link graph that resolved none of it is
	// a resolver failure, and without this line it looks exactly like a corpus
	// with no cross-references.
	Links survey.LinkTotals `json:"links"`

	Nodes       int                    `json:"nodes"`
	Leaves      int                    `json:"pages"`
	Indexes     int                    `json:"sections"`
	Groups      int                    `json:"groups"`
	SplitGroups int                    `json:"splitGroups"`
	Summaries   int                    `json:"summaries"`
	Delivered   int                    `json:"deliveredFiles"`
	Units       int                    `json:"units"`
	Produced    int                    `json:"unitsProduced"`
	Reused      int                    `json:"unitsReused"`
	ElapsedMS   int64                  `json:"elapsedMs"`
	Verify      []assemble.CheckResult `json:"verify"`

	// Live is present exactly when a model was in the loop. Absent rather than
	// zeroed: a mechanical run reporting `"promptTokens": 0` reads as "it cost
	// nothing" where the truth is "nothing was asked".
	Live *buildLive `json:"live,omitempty"`
}

// buildLive is what a live run dialed, asked and spent. It carries no
// credential: the API key is never in this process's output, only on the wire,
// and the base URL has any userinfo stripped (safeBaseURL).
type buildLive struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	// Model and Tier are the HEAVY tier's: the model that designed the tree
	// and wrote the summaries. LightModel is the one that adjudicated the page
	// boundaries — the other tier a live run requires, and a different model
	// id whenever the two tiers are mapped to different variants (§10).
	Model      string `json:"model"`
	Tier       string `json:"tier"`
	LightModel string `json:"lightModel"`
	// Thinking and RetryThinking are the efforts the two HEAVY definitions
	// declare — the first attempt's and the informed retry's
	// (ARCHITECTURE.md §9). Taxonomy and summaries declare the same pair
	// today, so one pair is the truth about both; refinement declares its own
	// (dissect.Effort/dissect.Retry, off/off) and is not folded in here,
	// because two definitions averaged into one row is a row that is true of
	// neither.
	Thinking      bool `json:"thinking"`
	RetryThinking bool `json:"retryThinking"`
	// TaxonomyCalls is how many container calls the descent ENUMERATED — the
	// stage's cost before it runs, and what an interrupted run re-spends.
	// CallsSkipped is how many calls of the whole job were not made because the
	// stage that planned them found it no longer needed the answer (an absorbed
	// container, §3.2), so the two together are what the run actually asked.
	TaxonomyCalls int `json:"taxonomyCalls"`
	CallsSkipped  int `json:"callsSkipped"`

	// What stage 4's fold did, summed over every split group. Zero across the
	// board is the CORRECT reading of a corpus whose groups all fit one page:
	// there was no boundary for anyone to choose, so the stage planned no call
	// — not a stage that failed to run. BoundariesFellBack is the degradation
	// count: a boundary whose answers did not verify keeps the mechanical cut,
	// which costs quality and never correctness (§3 monotone safety).
	BoundariesAdjudicated int `json:"boundariesAdjudicated"`
	BoundariesMoved       int `json:"boundariesMoved"`
	BoundariesFellBack    int `json:"boundariesFellBack"`
	BoundaryRejections    int `json:"boundaryRejections"`

	PromptTokens     int `json:"promptTokens"`
	CachedTokens     int `json:"cachedTokens"`
	CompletionTokens int `json:"completionTokens"`
	ReasoningTokens  int `json:"reasoningTokens"`
}

// writeBuildRecord writes the record into the temp-work root. The moment
// matters: it is after the coordinator returned, and therefore after the store
// swept the job directory. The record is not a chain unit, so a sweep that ran
// after this write would delete it as unaccounted-for.
func writeBuildRecord(dir string, rec buildRun) (string, error) {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("%s: encode %s: %w", buildVerb, buildRecordName, err)
	}
	path := filepath.Join(dir, buildRecordName)
	if err := os.WriteFile(path, append(data, '\n'), pipeline.CreateFileMode); err != nil {
		return "", fmt.Errorf("%s: write %s: %w", buildVerb, path, err)
	}
	return path, nil
}

// declaredAnnexes turns the --annex values into annex declarations.
//
// Only the prefix is set. §2.8 gives the convention text to the stage-3
// verifier, which authors it from the survey so stage 9 can re-derive it; a
// string carried in from a flag is the one field it could not. The flag order
// is preserved, because it is the order the entry-point's annex lookup renders
// in, and everything else a prefix has to satisfy is treeplan's to say.
func declaredAnnexes(flags []string) []treeplan.Annex {
	if len(flags) == 0 {
		return nil
	}
	out := make([]treeplan.Annex, 0, len(flags))
	for _, f := range flags {
		out = append(out, treeplan.Annex{Prefix: strings.TrimSpace(f)})
	}
	return out
}

// annexPrefixList is the declared prefixes, for the run record and the
// parameter digest.
func annexPrefixList(annexes []treeplan.Annex) []string {
	if len(annexes) == 0 {
		return nil
	}
	out := make([]string, 0, len(annexes))
	for _, a := range annexes {
		out = append(out, a.Prefix)
	}
	return out
}

// treePlanSource names where stage 3's artifact came from, for the record.
func treePlanSource(live bool) string {
	if live {
		return treePlanModel
	}
	return treePlanMechanical
}

// resolveBuildDate is the date every provenance receipt is stamped with: the
// pinned [dev] build_date, else today, UTC.
//
// A malformed pin is refused rather than passed through. The renderer treats
// the date as an opaque string, so an unparsed value would land in the footer
// of every delivered page and be noticed only by whoever read one.
func resolveBuildDate(pinned string) (string, error) {
	date := strings.TrimSpace(pinned)
	if date == "" {
		return time.Now().UTC().Format(buildDateLayout), nil
	}
	if _, err := time.Parse(buildDateLayout, date); err != nil {
		return "", fmt.Errorf("%s: [dev] build_date is %q; the format is %s",
			buildVerb, pinned, buildDateLayout)
	}
	return date, nil
}

var (
	buildFlagOut     string
	buildFlagTitle   string
	buildFlagKeep    bool
	buildFlagAnnexes []string
)

var buildCmd = &cobra.Command{
	Use:   "build <corpus-dir>",
	Short: "build a knowledge base from a documentation corpus",
	Long: `Walk a Markdown corpus and assemble a complete, verified knowledge base out of
it: entry point, section indexes, verbatim pages, navigation links, the shipped
agent fixtures, and the ten verify gates that stand between the assembled tree
and the delivered one.

kbase is a model-driven appliance, so this needs a configured provider, and
both tiers. The tree is designed by a descent over the corpus's own containers
and every section page carries the framing and conclusions a model wrote for
it — both heavy-tier, and both essential seams: a unit that fails twice fails
the job and nothing is delivered. Page boundaries inside an oversized section
are adjudicated by the light tier against the mechanical splitter's proposal,
which is a refinement seam: a boundary whose answers do not verify keeps the
mechanical cut, so that half costs quality and never correctness.
Configuration is resolved the way every verb resolves it (--config-dir, else
$KBASE_CONFIG_DIR, else ~/.config/kbase); an unconfigured tier refuses before
the corpus is read, so a misconfigured run costs nothing.

--out is required and receives the knowledge base and nothing else — so a --out
that already holds files refuses before the corpus is read, naming every one and
writing nothing: kbase delivers a whole knowledge base or nothing and never
updates one in place, so rebuilding means deleting the old tree first. A build
that was interrupted is the one exception; the next run of the same --out
finishes it. Everything
else the run produces lives under <out>/temp-work/, which kbase creates and, on
a successful run, removes: every intermediate, and the run record naming the
corpus and its hash, the budgets, the counts and each gate's verdict. A failed
or interrupted run keeps it; --keep-temp-work (or [dev] keep_temp_work) keeps
it after a successful one too — and a successful run without either keeps no
run record, the record being a development record rather than something the
knowledge base's readers use.

--title is the original doc set's title as you would state it ("Rojo v7
Documentation"); the knowledge base is named "KBase for <title>" and that name
is its entry-point heading, the up-links pointing at it and the start link every
shipped fixture carries. Without it the corpus directory's base name is used,
which is the checkout's name rather than the documentation's.

--annex is repeatable and is the only way a corpus territory becomes an annex:
the material under the prefix is not distilled, not enumerated by the taxonomy
descent and not required to be covered, and the entry point gains a lookup
section saying where it lives and how it is laid out. A prefix naming no
surveyed document refuses at job setup rather than excluding nothing quietly.
It is a flag and not a model decision on purpose — a model that can place
material out of scope can hide its own failures.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, opts, err := loadVerbContext()
		if err != nil {
			return err
		}
		opts.Timeout = buildTimeout

		// The one switch that decides whether this run dials at all. It is
		// read before anything else so a typo in it fails here rather than
		// resolving to whichever shape the code happened to fall into.
		mechanical, err := opts.Config.Dev.MechanicalTreePlan()
		if err != nil {
			return err
		}
		live := &opts
		if mechanical {
			live = nil
		}
		_, err = runBuild(cmd.Context(), buildOptions{
			Root:    args[0],
			Title:   buildFlagTitle,
			Live:    live,
			Out:     buildFlagOut,
			Annexes: buildFlagAnnexes,
			// The flag turns keeping ON and cannot turn it off: the only
			// reason to insist on deletion is disk, and that remedy is one
			// `rm -r` away, while the reason to keep is a run whose evidence
			// someone wants and cannot get back.
			KeepTempWork: buildFlagKeep || opts.Config.Dev.KeepTempWork,
			BuildDate:    opts.Config.Dev.BuildDate,
			Stdout:       cmd.OutOrStdout(),
			Stderr:       cmd.ErrOrStderr(),
			Logger:       processLog.logger,
		})
		return err
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildFlagOut, "out", "",
		"output directory for the knowledge base (required; created if missing)")
	buildCmd.Flags().StringVar(&buildFlagTitle, "title", "",
		"the documentation set's own title; the knowledge base is named \""+
			kbTitlePrefix+"<title>\" (default: the corpus directory's name)")
	buildCmd.Flags().StringArrayVar(&buildFlagAnnexes, "annex", nil,
		"corpus-relative prefix to leave undistilled and list in the entry point's annex lookup (repeatable)")
	buildCmd.Flags().BoolVar(&buildFlagKeep, keepTempWorkFlag, false,
		"keep <out>/"+pipeline.TempWorkDirName+"/, and with it "+buildRecordName+
			", after a run that succeeded (a failed run always keeps it)")
	rootCmd.AddCommand(buildCmd)
}
