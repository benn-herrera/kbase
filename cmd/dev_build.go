package main

import (
	"bytes"
	"context"
	"encoding/json"
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

// dev-build composes §10's plan table into one job and delivers a knowledge
// base out of a corpus. It has TWO shapes, and `--config-dir` selects between
// them.
//
// WITHOUT it, the job is the mechanical spine: every stage deterministic, no
// client dialed, the tree grouped by treeplan.SourceStructureProposal — the
// source's own file structure — and every index rendered with its framing and
// conclusions blocks absent, which O-1's grammar makes legal rather than
// broken. That is not an omission to be filled in later: it is the property
// that makes this verb a regression gate, running offline in seconds over a
// pinned corpus, where any drift in the mechanical half fails it.
//
// WITH it, stages 3 and 6 run LIVE against the configured provider: the
// container-descent fold designs the tree (internal/taxonomy, heavy tier,
// no-fallback seam) and the four level stages write the summaries
// (internal/summarize, heavy tier, no-fallback seam). Everything else is the
// same code on the same artifacts — which is the point of having built the
// spine first.
//
// Stage 4's own live seam (boundary refinement) is NOT wired here: it has its
// own verb (`dev-refine`) and its own place in the composing verb, and this
// verb's cut lists stay dissect.Split's mechanical output (ROADMAP).
const (
	devBuildVerb = "dev-build"

	// The two job frames — slot 1, job-constant (§7). A frame is the first
	// thing a model reads, so the mechanical one says what it is and is never
	// sent, while the live one describes the job the calls belong to.
	devBuildFrame     = "kbase dev-build: assembling a knowledge base with no model in the loop."
	devBuildLiveFrame = "kbase dev-build: turning a documentation corpus into a knowledge base — " +
		"a navigable tree of verbatim pages under sections that summarise them."

	// devBuildRecordName is the run record delivered beside the tree.
	devBuildRecordName = "run.json"

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

	// buildDateLayout is the provenance receipt's date format (SPEC §7).
	buildDateLayout = "2006-01-02"

	// devBuildTimeout bounds the WHOLE live job rather than one call: tens of
	// heavy-tier calls — one per container of the descent, one per section of
	// the tree — plus the transport policy's own retries underneath each of
	// them. It is generous because the failure it exists to catch is a
	// provider that has stopped answering, not one that is slow.
	devBuildTimeout = 2 * time.Hour
)

// devBuildOptions is the resolved input of the dev-build verb.
type devBuildOptions struct {
	// Root is the corpus directory to build from.
	Root string

	// Live is the provider pool and configuration, present exactly when
	// --config-dir was given. Nil is the mechanical build, and nil is the
	// honest value for it: wiring the pool into a run that makes no call would
	// invent a failure mode ("no provider selected") on a command that never
	// dials one.
	Live *providerOptions

	// Out is the OUTPUT directory: the delivered knowledge base and the run
	// record land here, and `<out>/temp-work/` holds everything transient
	// while the job runs (ARCHITECTURE.md §12). Nothing already in it is
	// touched.
	Out string

	// Budget is the per-leaf token budget (G-1) the tree plan is verified at
	// and the splitter fills toward.
	Budget int

	// KeepTempWork keeps `<out>/temp-work/` after a run that succeeded. A
	// failed or interrupted run keeps it whatever this says.
	KeepTempWork bool

	// BuildDate stamps the provenance receipt on every delivered page. It is
	// an option rather than a clock read inside the renderer so that the
	// rendered tree stays a pure function of its inputs (§8.1) and stage 9's
	// leaf re-derivation compares equal bytes; empty means today, UTC.
	BuildDate string

	// Stdout takes the verb's report, Stderr its failure detail, Logger the
	// pipeline's own diagnostics.
	Stdout io.Writer
	Stderr io.Writer
	Logger log.Logger
}

// devBuildResult is what a completed run produced — returned for the tests,
// which assert over it rather than over parsed stdout.
type devBuildResult struct {
	Plan   treeplan.TreePlan
	Report assemble.Report
	Job    pipeline.JobResult
	// Delivered is every path written under <out>, tree-relative.
	Delivered []string
}

// runDevBuild ingests a corpus and builds a knowledge base out of it.
//
// The order of the refusals is the same one every other verb keeps: everything
// refusable offline is refused before the job directory is opened, so a
// misconfigured run leaves nothing behind to clean up.
func runDevBuild(ctx context.Context, opts devBuildOptions) (devBuildResult, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, devBuildVerb); err != nil {
		return devBuildResult{}, err
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		return devBuildResult{}, fmt.Errorf("%s: a corpus directory is required", devBuildVerb)
	}
	out := strings.TrimSpace(opts.Out)
	if out == "" {
		return devBuildResult{}, fmt.Errorf(
			"%s: --out is required; this verb delivers a knowledge base and never uses a temporary directory", devBuildVerb)
	}
	if opts.Budget <= 0 {
		return devBuildResult{}, fmt.Errorf(
			"%s: --budget is %d; a leaf budget is a positive number of tokens", devBuildVerb, opts.Budget)
	}
	lg := opts.Logger
	if lg == nil {
		lg = log.Discard()
	}
	buildDate := strings.TrimSpace(opts.BuildDate)
	if buildDate == "" {
		buildDate = time.Now().UTC().Format(buildDateLayout)
	}
	if err := os.MkdirAll(out, pipeline.ArtifactDirMode); err != nil {
		return devBuildResult{}, fmt.Errorf("%s: create %s: %w", devBuildVerb, out, err)
	}

	// Ingest and survey run in process, at job setup: §10's table has ingest
	// contributing the corpus hash as a named input rather than an artifact,
	// and the survey artifact is needed by the tree-plan verifier and the
	// rebase map before any stage has run. The survey stage below writes the
	// same value to the store, which is what puts it in the chain.
	corpus, err := ingest.Walk(root, markdown.Extensions(), lg)
	if err != nil {
		return devBuildResult{}, err
	}
	est := tokens.Estimator{}
	art, err := markdown.Survey(corpus, est, lg)
	if err != nil {
		return devBuildResult{}, err
	}

	params := treeplan.DefaultParams()
	params.Est = est
	params.Budgets.LeafTokens = opts.Budget
	verifier, err := treeplan.NewVerifier(art, corpus, params)
	if err != nil {
		return devBuildResult{}, err
	}
	prov := distill.Provenance{CorpusHash: art.Corpus.ContentHash, BuildDate: buildDate}
	if err := prov.Validate(); err != nil {
		return devBuildResult{}, err
	}

	// The live half, dialed only after everything refusable offline has been
	// refused — a misconfigured run costs no tokens. The runner stays nil for
	// a mechanical build, and that is the honest value: every stage of that
	// plan is Produce tasks, so no call is ever built and a stub runner would
	// be a client this verb does not have standing where the thing it does not
	// use goes.
	var (
		runner *pipeline.CallRunner
		live   devBuildLive
		runCtx = ctx
	)
	if opts.Live != nil {
		id, ok := opts.Live.Config.ModelFor(config.TierHeavy)
		if !ok {
			return devBuildResult{}, fmt.Errorf(
				"%s: no model is configured for the %q tier in %s; taxonomy design and summaries are heavy-tier work",
				devBuildVerb, config.TierHeavy, config.ConfigFileName)
		}
		name, client, callCtx, release, err := opts.Live.dial(ctx)
		if err != nil {
			return devBuildResult{}, err
		}
		defer release()
		live = devBuildLive{
			Provider: name,
			BaseURL:  safeBaseURL(opts.Live.Providers[name].BaseURL),
			Model:    id,
			Tier:     config.TierHeavy,
			Thinking: taxonomy.Effort.Thinking,
		}
		runner, runCtx = pipeline.NewCallRunner(client, opts.Live.Config, lg), callCtx
		fmt.Fprintf(opts.Stdout, "provider: %s (%s)\nmodel: %s (%s tier)\n",
			live.Provider, live.BaseURL, live.Model, live.Tier)
	}

	work, err := pipeline.OpenTempWork(out, lg)
	if err != nil {
		return devBuildResult{}, err
	}
	store := work.ArtifactStore()
	job := &devBuildJob{
		opts: opts, out: out, corpus: corpus, art: art, params: params,
		verifier: verifier, prov: prov, est: est, store: store, lg: lg,
		live: opts.Live != nil,
		inputs: []pipeline.Input{
			{Name: corpusInput, Hash: art.Corpus.ContentHash},
			{Name: paramsInput, Hash: pipeline.HashBytes([]byte(fmt.Sprintf(
				"leafTokens=%d;buildDate=%s;version=%s", opts.Budget, buildDate, version.Current)))},
		},
	}
	if job.live {
		if err := job.wireModelStages(corpusTitle(opts.Root), corpusScope(opts.Root)); err != nil {
			return devBuildResult{}, err
		}
		live.TaxonomyCalls = job.designer.Calls()
		fmt.Fprintf(opts.Stdout, "taxonomy: %d container calls\n", live.TaxonomyCalls)
	}

	coord := pipeline.NewCoordinator(store, runner, pipeline.DefaultWorkers, lg)
	start := time.Now()
	res, err := coord.Run(runCtx, job.plan(), pipeline.ModeResume)
	elapsed := time.Since(start)
	// Every return from here down keeps temp-work: a failed or interrupted run
	// leaves its intermediates for a resume and for a human.
	if err != nil {
		return devBuildResult{}, err
	}
	for _, f := range res.Failures {
		fmt.Fprintf(opts.Stderr, "unit %s failed (%s): %v\n", f.Path, f.Kind, f.Err)
	}
	if !res.DeliveryReady() {
		return devBuildResult{}, fmt.Errorf("%s: the build did not complete: %d of %d units produced, %d failed",
			devBuildVerb, res.Produced, res.Units, len(res.Failures))
	}

	plan, err := job.readTreePlan()
	if err != nil {
		return devBuildResult{}, err
	}
	report, err := job.readReport()
	if err != nil {
		return devBuildResult{}, err
	}
	delivered, err := job.deliver(plan)
	if err != nil {
		return devBuildResult{}, err
	}

	summaries, err := job.summaryCount(plan)
	if err != nil {
		return devBuildResult{}, err
	}
	record := devBuildRun{
		Version:     version.Current,
		Corpus:      root,
		CorpusHash:  art.Corpus.ContentHash,
		BuildDate:   buildDate,
		Budgets:     plan.Budgets,
		Files:       art.Corpus.Files,
		Sections:    art.Corpus.Sections,
		Nodes:       len(plan.Nodes),
		Leaves:      report.Leaves,
		Indexes:     report.Indexes,
		Groups:      len(plan.Groups),
		SplitGroups: splitGroups(plan),
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
		live.PromptTokens = res.Usage.PromptTokens
		live.CachedTokens = res.Usage.CachedPromptTokens
		live.CompletionTokens = res.Usage.CompletionTokens
		live.ReasoningTokens = res.Usage.ReasoningTokens
		record.Live = &live
	}
	recordPath, err := writeDevBuildRecord(out, record)
	if err != nil {
		return devBuildResult{}, err
	}
	job.report(plan, report, res, elapsed, len(delivered), summaries, recordPath)

	if opts.KeepTempWork {
		fmt.Fprintf(opts.Stdout, "temp work kept: %s\n", work.Root())
		return devBuildResult{Plan: plan, Report: report, Job: res, Delivered: delivered}, nil
	}
	if err := work.Discard(); err != nil {
		return devBuildResult{}, err
	}
	return devBuildResult{Plan: plan, Report: report, Job: res, Delivered: delivered}, nil
}

// devBuildJob is the job's composition: everything the six stages share, in one
// value, so a stage description is a method rather than a closure over a dozen
// locals.
type devBuildJob struct {
	opts     devBuildOptions
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

	// live says whether stages 3 and 6 ask a model. The two model stages are
	// nil when it is false, and the plan below is then the mechanical spine.
	live       bool
	designer   *taxonomy.Designer
	summarizer *summarize.Summarizer
}

// wireModelStages builds the two no-fallback seams this verb can run: the
// container descent that designs the tree, and the level stages that summarise
// it.
//
// Both are built here, at the composition root, because that is where a stage
// is registered and therefore where its declared effort is threaded from
// (ARCHITECTURE.md §9, §12) — the values are the definitions' own
// (taxonomy.Effort, summarize.Effort), passed through untouched.
func (j *devBuildJob) wireModelStages(title, scope string) error {
	designer, err := taxonomy.New(taxonomy.Design{
		Survey:   j.art,
		Verifier: j.verifier,
		Params:   j.params,
		Title:    title,
		Scope:    scope,
		Unit:     treePlanUnit,
		Effort:   taxonomy.Effort,
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
	}, j.lg)
	if err != nil {
		return err
	}
	j.designer, j.summarizer = designer, summarizer
	return nil
}

// plan is §10's table: survey, tree plan, cuts, pages, summaries, assemble,
// verify — with stages 3 and 6 asking a model when this run is live and
// deriving their artifacts mechanically when it is not.
//
// Stage 4's live refinement is deliberately absent from both shapes: the cut
// lists here are dissect.Split's own output, which every consumer already
// treats as the always-valid fallback, and wiring the refinement fold into a
// job plan is its own item (ROADMAP).
func (j *devBuildJob) plan() pipeline.Plan {
	frame := devBuildFrame
	if j.live {
		frame = devBuildLiveFrame
	}
	stages := []*pipeline.StagePlan{
		j.surveyStage(), j.treePlanStage(), j.cutsStage(), j.leavesStage(),
	}
	stages = append(stages, j.summaryStages()...)
	stages = append(stages, j.assembleStage(), j.verifyStage())
	return pipeline.Plan{JobFrame: frame, Stages: stages}
}

// summaryStages is stage 6: one stage per index level, deepest first, and none
// at all when no model is in the loop.
func (j *devBuildJob) summaryStages() []*pipeline.StagePlan {
	if j.summarizer == nil {
		return nil
	}
	return j.summarizer.StagePlans(stageSummaries, func(unit string, upstreams []string) pipeline.OwedArtifact {
		return j.owed(unit, append([]string{treePlanUnit}, upstreams...)...)
	})
}

// owed describes one unit: the job's two derived inputs plus whatever store
// artifacts it consumed.
func (j *devBuildJob) owed(path string, upstreams ...string) pipeline.OwedArtifact {
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
func (j *devBuildJob) surveyStage() *pipeline.StagePlan {
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
				return nil, fmt.Errorf("%s: artifact is %T, not a survey artifact", devBuildVerb, artifact)
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
func (j *devBuildJob) treePlanStage() *pipeline.StagePlan {
	if j.designer != nil {
		return j.designer.StagePlan(stageTreePlan, j.inputs, surveyUnit)
	}
	return &pipeline.StagePlan{
		Name: stageTreePlan,
		Lanes: oneLane(stageTreePlan, pipeline.LaneTask{
			Owed:    j.owed(treePlanUnit, surveyUnit),
			Section: stageTreePlan,
			Produce: func() (any, error) {
				proposal, err := treeplan.SourceStructureProposal(j.art, corpusTitle(j.opts.Root), corpusScope(j.opts.Root))
				if err != nil {
					return nil, err
				}
				return j.verifier.Compose(proposal, nil)
			},
		}),
		// The same encoder the live stage uses: one artifact, one encoder,
		// whichever stage produced it.
		Encode: taxonomy.Encode,
	}
}

// cutsStage writes one cut list per SPLIT group — and none at all when no
// group splits, which is the ordinary outcome at the shipped budget over a
// corpus of documentation-sized sections.
//
// The list is dissect.Split's own output, which is always valid by
// construction. The live refiner replaces this stage without moving the
// artifact: same path, same encoder, same contract, boundaries adjudicated
// instead of taken.
func (j *devBuildJob) cutsStage() *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name:   stageCuts,
		Encode: dissect.EncodeCutList,
		Lanes: func() ([]pipeline.SerialLane, error) {
			plan, err := j.readTreePlan()
			if err != nil {
				return nil, err
			}
			var tasks []pipeline.LaneTask
			for _, g := range plan.Groups {
				if g.Parts == 1 {
					continue
				}
				unit := cutListPath(g.ID)
				tasks = append(tasks, pipeline.LaneTask{
					Owed:    j.owed(unit, treePlanUnit, surveyUnit),
					Section: stageCuts,
					Produce: j.cutProducer(g, unit),
				})
			}
			if len(tasks) == 0 {
				j.lg.Info("no group needs cutting", "stage", stageCuts, "groups", len(plan.Groups))
				return nil, nil
			}
			return []pipeline.SerialLane{{Domain: stageCuts, Tasks: tasks}}, nil
		},
	}
}

func (j *devBuildJob) cutProducer(g treeplan.SplitGroup, unit string) pipeline.Producer {
	return func() (any, error) {
		doc, ok := j.corpus.Doc(g.Source.File)
		if !ok {
			return nil, fmt.Errorf("%s: group %s draws on %s, which is not under custody",
				devBuildVerb, g.ID, g.Source.File)
		}
		f, ok := surveyFile(j.art, g.Source.File)
		if !ok {
			return nil, fmt.Errorf("%s: group %s draws on %s, which the survey does not describe",
				devBuildVerb, g.ID, g.Source.File)
		}
		cuts, err := dissect.Split(doc.Bytes, survey.Span{Start: g.Source.Start, End: g.Source.End}, f.Cuts,
			dissect.Params{Est: j.est, BudgetTokens: g.Budget})
		if err != nil {
			return nil, err
		}
		return dissect.CutList{Unit: unit, Cuts: cuts}, nil
	}
}

// leavesStage is stage 5: one Produce task per leaf, lanes per domain, units
// resolved from the tree plan ∪ the cut lists — ARCHITECTURE §12's lazy chain
// description, where stage 4's verified cut list defines stage 5's work.
func (j *devBuildJob) leavesStage() *pipeline.StagePlan {
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
// entry-point kinds, a byte-for-byte copy for the leaves, and §8.1's fixture
// manifest.
//
// Leaves are COPIED. Rebasing happened once, at stage 5, and this stage does
// not touch leaf bytes [MAD1: F-9] — which is what makes the delivered page
// and the artifact stage 9 re-derives against the same bytes by construction.
func (j *devBuildJob) assembleStage() *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name:   stageAssemble,
		Encode: encodeBytes,
		Lanes: func() ([]pipeline.SerialLane, error) {
			plan, err := j.readTreePlan()
			if err != nil {
				return nil, err
			}
			r, err := j.renderer(plan)
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
				// A section's page is the tree plan and its summary, so it
				// declares both: a regenerated summary invalidates the page it
				// is rendered into, for free.
				up := []string{treePlanUnit}
				if j.live {
					up = append(up, j.summarizer.Unit(n.Path))
				}
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

// verifyStage is stage 9: the nine gates over the assembled tree in the store,
// before any byte reaches <out> (I-5).
//
// It declares every delivered file as an upstream, which is the truthful
// statement of what it read — and what makes a re-run after one page changed
// redo the gates rather than reuse a verdict about a tree that no longer
// exists.
func (j *devBuildJob) verifyStage() *pipeline.StagePlan {
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

func (j *devBuildJob) verifyProducer() pipeline.Producer {
	return func() (any, error) {
		plan, cuts, err := j.readPlanAndCuts()
		if err != nil {
			return nil, err
		}
		r, err := j.renderer(plan)
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

// deliver copies the proven bytes out of the store into <out> proper — the
// mirror hierarchy read in the delivering direction, with the `tree/` prefix
// dropped because that prefix is the store's own namespace and not part of the
// knowledge base's shape.
func (j *devBuildJob) deliver(plan treeplan.TreePlan) ([]string, error) {
	paths := deliveredPaths(plan)
	for _, p := range paths {
		data, err := j.store.Get(treeDir + "/" + p)
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(j.out, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), pipeline.ArtifactDirMode); err != nil {
			return nil, fmt.Errorf("%s: create %s: %w", devBuildVerb, filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, pipeline.ArtifactFileMode); err != nil {
			return nil, fmt.Errorf("%s: write %s: %w", devBuildVerb, dst, err)
		}
	}
	return paths, nil
}

// report prints the human summary.
func (j *devBuildJob) report(plan treeplan.TreePlan, rep assemble.Report,
	res pipeline.JobResult, elapsed time.Duration, delivered, summaries int, recordPath string) {

	w := j.opts.Stdout
	t := j.art.Corpus
	fmt.Fprintf(w, "corpus: %s (%d files, %d sections, %d tokens, sha256 %s)\n",
		j.opts.Root, t.Files, t.Sections, t.Tokens, t.ContentHash)
	fmt.Fprintf(w, "budget: %d tokens per page\n", plan.Budgets.LeafTokens)
	fmt.Fprintf(w, "tree plan: %d nodes (%d pages, %d sections), %d groups, %d split\n",
		len(plan.Nodes), rep.Leaves, rep.Indexes, len(plan.Groups), splitGroups(plan))
	if j.live {
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
	fmt.Fprintf(w, "record: %s\n", recordPath)
}

// renderer builds stage 8's renderer over the tree plan and whatever summaries
// stage 6 wrote.
//
// One constructor for both call sites — the assemble stage and check 5's
// re-render — because check 5 compares the delivered bytes against a fresh
// render, and a renderer holding different summaries would be comparing two
// different pages and calling the difference a defect.
func (j *devBuildJob) renderer(plan treeplan.TreePlan) (*assemble.Renderer, error) {
	summaries, err := summarize.All(j.store, summariesDir, plan)
	if err != nil {
		return nil, err
	}
	return assemble.NewRenderer(plan, j.prov, summaries)
}

// summaryCount is how many sections a summary was written for — the live
// stage's own yield, reported beside the node counts.
func (j *devBuildJob) summaryCount(plan treeplan.TreePlan) (int, error) {
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
func (j *devBuildJob) readTreePlan() (treeplan.TreePlan, error) {
	data, err := j.store.Get(treePlanUnit)
	if err != nil {
		return treeplan.TreePlan{}, err
	}
	return treeplan.ReadJSON(bytes.NewReader(data))
}

// readPlanAndCuts reads the tree plan and every split group's cut list.
func (j *devBuildJob) readPlanAndCuts() (treeplan.TreePlan, map[string][]survey.Span, error) {
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
func (j *devBuildJob) readReport() (assemble.Report, error) {
	data, err := j.store.Get(verifyUnit)
	if err != nil {
		return assemble.Report{}, err
	}
	var rep assemble.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return assemble.Report{}, fmt.Errorf("%s: read the verify report: %w", devBuildVerb, err)
	}
	return rep, nil
}

// deliveredPaths is the whole delivered set — class A then class B — in a
// stable order (§8.1).
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

func splitGroups(plan treeplan.TreePlan) int {
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

// corpusTitle and corpusScope name the entry-point of a dev build. They are the
// corpus directory's own name, because the verb has nothing better: the real
// ones are the taxonomy stage's to write (§2.1), and inventing something that
// reads like a considered title would hide that.
func corpusTitle(root string) string {
	return filepath.Base(filepath.Clean(root)) + " knowledge base"
}

func corpusScope(root string) string {
	return "everything built from the " + filepath.Base(filepath.Clean(root)) + " corpus"
}

// encodeBytes is the assemble stage's encoder: a rendered page IS its bytes.
func encodeBytes(artifact any) ([]byte, error) {
	b, ok := artifact.([]byte)
	if !ok {
		return nil, fmt.Errorf("%s: artifact is %T, not rendered bytes", devBuildVerb, artifact)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("%s: a node rendered no bytes", devBuildVerb)
	}
	return b, nil
}

func renderProducer(r *assemble.Renderer, n treeplan.Node) pipeline.Producer {
	return func() (any, error) { return r.Node(n) }
}

func constProducer(data []byte) pipeline.Producer {
	return func() (any, error) { return data, nil }
}

func (j *devBuildJob) copyProducer(unit string) pipeline.Producer {
	return func() (any, error) { return j.store.Get(unit) }
}

// devBuildRun is the run record delivered beside the tree: what was built, from
// what, under which budgets, and what the gates said.
//
// It is a development record rather than a stamp — nothing reads it back — but
// it carries every identity a later reader would otherwise reconstruct from a
// log, and the nine verify results, because those ARE the result of the run.
type devBuildRun struct {
	Version     string                 `json:"version"`
	Corpus      string                 `json:"corpus"`
	CorpusHash  string                 `json:"corpusHash"`
	BuildDate   string                 `json:"buildDate"`
	Budgets     treeplan.Budgets       `json:"budgets"`
	Files       int                    `json:"sourceFiles"`
	Sections    int                    `json:"sourceSections"`
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
	Live *devBuildLive `json:"live,omitempty"`
}

// devBuildLive is what a live run dialed, asked and spent. It carries no
// credential: the API key is never in this process's output, only on the wire,
// and the base URL has any userinfo stripped (safeBaseURL).
type devBuildLive struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
	Tier     string `json:"tier"`
	// Thinking is the effort the two definitions declare. Both stages are
	// heavy-tier and both declare the same value today; the day they differ,
	// this becomes two fields rather than one lie.
	Thinking bool `json:"thinking"`
	// TaxonomyCalls is how many container calls the descent enumerated — the
	// stage's cost before it runs, and what an interrupted run re-spends.
	TaxonomyCalls    int `json:"taxonomyCalls"`
	PromptTokens     int `json:"promptTokens"`
	CachedTokens     int `json:"cachedTokens"`
	CompletionTokens int `json:"completionTokens"`
	ReasoningTokens  int `json:"reasoningTokens"`
}

func writeDevBuildRecord(dir string, rec devBuildRun) (string, error) {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("%s: encode %s: %w", devBuildVerb, devBuildRecordName, err)
	}
	path := filepath.Join(dir, devBuildRecordName)
	if err := os.WriteFile(path, append(data, '\n'), pipeline.ArtifactFileMode); err != nil {
		return "", fmt.Errorf("%s: write %s: %w", devBuildVerb, path, err)
	}
	return path, nil
}

// devBuildProvider resolves the live half of this verb, or nil for the
// mechanical build.
//
// The flag's PRESENCE is the switch, so this reads flagConfigDir directly
// rather than through resolveConfigDir: an absent flag here means "no model",
// not "find one somewhere else". Everything it loads is loaded before a client
// is dialed and before the job directory is opened, so a broken configuration
// costs nothing.
func devBuildProvider() (*providerOptions, error) {
	if strings.TrimSpace(flagConfigDir) == "" {
		return nil, nil
	}
	_, opts, err := loadVerbContext()
	if err != nil {
		return nil, err
	}
	opts.Timeout = devBuildTimeout
	return &opts, nil
}

var (
	devBuildFlagOut       string
	devBuildFlagBudget    int
	devBuildFlagKeep      bool
	devBuildFlagBuildDate string
)

var devBuildCmd = &cobra.Command{
	Use:   "dev-build <corpus-dir>",
	Short: "build a knowledge base from a corpus, with or without a model in the loop",
	Long: `Walk a Markdown corpus and assemble a complete, verified knowledge base out of
it: entry point, section indexes, verbatim pages, navigation links, the shipped
agent fixtures, and the nine verify gates that stand between the assembled tree
and the delivered one.

This is a development verb with two shapes, and --config-dir selects between
them.

WITHOUT --config-dir it runs the MECHANICAL half of the pipeline. No model, no
provider, no API key, no configuration directory — every stage is
deterministic, so it works offline and finishes in seconds. Two things the
shipped pipeline supplies are absent, and their absence is visible in the
output: the tree is grouped by the source's own file structure rather than by a
designed taxonomy, and the section pages carry no summary prose. Both are legal
shapes, and everything else — the byte derivation, the link rebasing, the page
grammar, the gates — is the shipped article.

WITH --config-dir the taxonomy and the summaries are written by the heavy-tier
model configured there: the tree is designed by a descent over the corpus's own
containers, and every section page carries the framing and conclusions a model
wrote for it. Both are essential seams — a unit that fails twice fails the job,
and nothing is delivered. The prompts behind both are marked stubs until the
definitions work lands, so a live run is evidence about the seam before it is
evidence about the prose. Page boundaries stay mechanical in both shapes:
wiring the boundary-refinement fold into this plan is separate work, and
'kbase dev-refine' is where that seam is exercised today.

--out is required and receives the knowledge base plus a run record. Nothing
already in it is touched: every intermediate lives under <out>/temp-work/,
which kbase creates and, on a successful run, removes. A failed or interrupted
run keeps it; --keep-temp-work keeps it after a successful one too.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --config-dir is what selects the live shape, and it is read as GIVEN
		// rather than resolved: this verb has a complete offline meaning, so
		// falling back to the environment or to ~/.config/kbase would put a
		// model in the loop of a run nobody asked to be live — and send the
		// corpus to a provider they did not name.
		live, err := devBuildProvider()
		if err != nil {
			return err
		}
		_, err = runDevBuild(cmd.Context(), devBuildOptions{
			Root:         args[0],
			Live:         live,
			Out:          devBuildFlagOut,
			Budget:       devBuildFlagBudget,
			KeepTempWork: devBuildFlagKeep,
			BuildDate:    devBuildFlagBuildDate,
			Stdout:       cmd.OutOrStdout(),
			Stderr:       cmd.ErrOrStderr(),
			Logger:       processLog.logger,
		})
		return err
	},
}

func init() {
	devBuildCmd.Flags().StringVar(&devBuildFlagOut, "out", "",
		"output directory for the knowledge base and "+devBuildRecordName+" (required; created if missing)")
	devBuildCmd.Flags().IntVar(&devBuildFlagBudget, "budget", treeplan.DefaultBudgets().LeafTokens,
		"per-page source token budget (G-1); pages over it are split")
	devBuildCmd.Flags().BoolVar(&devBuildFlagKeep, devRefineKeepFlag, false,
		"keep <out>/"+pipeline.TempWorkDirName+"/ after a run that succeeded (a failed run always keeps it)")
	devBuildCmd.Flags().StringVar(&devBuildFlagBuildDate, "build-date", "",
		"date stamped into every page's provenance receipt, YYYY-MM-DD (default: today, UTC)")
	rootCmd.AddCommand(devBuildCmd)
}
