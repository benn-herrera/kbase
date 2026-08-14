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
	"kbase/internal/dissect"
	"kbase/internal/distill"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/pipeline"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
	"kbase/internal/version"
)

// dev-build is the MECHANICAL SPINE of §10's plan table, run end to end: the
// stages that need no model, composed into one job, delivering a walkable
// knowledge base.
//
// It is a development verb and the shape of its tree says so. Stage 3's
// taxonomy is a no-fallback seam (O-1) and it does not exist yet, so the tree
// plan here is treeplan.SourceStructureProposal — the source's own file
// structure, one domain per document, one page per top-level section. Stage 6's
// summaries do not exist either, so every index renders with its framing and
// conclusions blocks absent, which O-1's grammar makes legal rather than
// broken. What IS the shipped article is everything mechanical: the byte
// derivation, the rebase map, §4's grammar, the fixture manifest, the nine
// verify gates and the delivery.
//
// No model, no client, no configuration directory. That is not an omission to
// be filled in later — it is the property that makes this verb a regression
// gate: it runs offline, in seconds, over a pinned corpus, and any drift in the
// mechanical half of the pipeline fails it.
const (
	devBuildVerb = "dev-build"

	// devBuildFrame is slot 1 for this job. It is job-constant and it exists
	// only because pipeline.Plan requires one; no call is ever made with it.
	devBuildFrame = "kbase dev-build: assembling a knowledge base with no model in the loop."

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
	treeDir      = "tree"
	verifyUnit   = "verify/report.json"

	// cutListName is the file a group's cut list lands in, under
	// `cuts/{group}/`. It is the same name stage 4's refiner writes, because
	// the refiner replaces this stage's mechanical list without moving it.
	cutListName = "cutlist.txt"

	// Stage names. They name the lane, the store directory and the log
	// records, so a failure names one thing rather than three.
	stageSurvey   = "survey"
	stageTreePlan = "treeplan"
	stageCuts     = "cuts"
	stageLeaves   = "leaves"
	stageAssemble = "assemble"
	stageVerify   = "verify"

	// corpusInput and paramsInput name the two derived hashes every unit of
	// this job is stamped against: the identity of the source bytes, and the
	// operating point they were built under.
	corpusInput = "corpus"
	paramsInput = "params"

	// buildDateLayout is the provenance receipt's date format (SPEC §7).
	buildDateLayout = "2006-01-02"
)

// devBuildOptions is the resolved input of the dev-build verb.
//
// It carries no providerOptions and no configuration: every stage it runs is
// deterministic, so wiring it to the provider pool would invent a failure mode
// — "no provider selected" on a command that never calls one.
type devBuildOptions struct {
	// Root is the corpus directory to build from.
	Root string

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

	work, err := pipeline.OpenTempWork(out, lg)
	if err != nil {
		return devBuildResult{}, err
	}
	store := work.ArtifactStore()
	job := &devBuildJob{
		opts: opts, out: out, corpus: corpus, art: art, params: params,
		verifier: verifier, prov: prov, est: est, store: store, lg: lg,
		inputs: []pipeline.Input{
			{Name: corpusInput, Hash: art.Corpus.ContentHash},
			{Name: paramsInput, Hash: pipeline.HashBytes([]byte(fmt.Sprintf(
				"leafTokens=%d;buildDate=%s;version=%s", opts.Budget, buildDate, version.Current)))},
		},
	}

	// The runner is nil, and that is the honest value: every stage of this
	// plan is Produce tasks, so no call is ever built and no client is ever
	// dialed. A stub runner would be a client this verb does not have,
	// standing where the thing it does not use goes.
	coord := pipeline.NewCoordinator(store, nil, pipeline.DefaultWorkers, lg)
	start := time.Now()
	res, err := coord.Run(ctx, job.plan(), pipeline.ModeResume)
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
		Delivered:   len(delivered),
		Units:       res.Units,
		Produced:    res.Produced,
		Reused:      res.Reused,
		ElapsedMS:   elapsed.Milliseconds(),
		Verify:      report.Checks,
	}
	recordPath, err := writeDevBuildRecord(out, record)
	if err != nil {
		return devBuildResult{}, err
	}
	job.report(plan, report, res, elapsed, len(delivered), recordPath)

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
}

// plan is §10's table with the model stages left out: survey, tree plan, cuts,
// leaves, assemble, verify.
func (j *devBuildJob) plan() pipeline.Plan {
	return pipeline.Plan{
		JobFrame: devBuildFrame,
		Stages: []*pipeline.StagePlan{
			j.surveyStage(), j.treePlanStage(), j.cutsStage(),
			j.leavesStage(), j.assembleStage(), j.verifyStage(),
		},
	}
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

// treePlanStage composes the dev/baseline tree plan and holds it to every
// stage-3 post-condition.
//
// The proposal is the source's own structure and the verifier is the shipped
// one: what this stage skips is the model that would have chosen the grouping,
// not any of the checking that grouping has to survive.
func (j *devBuildJob) treePlanStage() *pipeline.StagePlan {
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
		Encode: func(artifact any) ([]byte, error) {
			p, ok := artifact.(treeplan.TreePlan)
			if !ok {
				return nil, fmt.Errorf("%s: artifact is %T, not a tree plan", devBuildVerb, artifact)
			}
			var buf bytes.Buffer
			if err := p.WriteJSON(&buf); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		},
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
			r, err := assemble.NewRenderer(plan, j.prov)
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
				tasks = append(tasks, pipeline.LaneTask{
					Owed:    j.owed(treeDir+"/"+n.Path, treePlanUnit),
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
		r, err := assemble.NewRenderer(plan, j.prov)
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
	res pipeline.JobResult, elapsed time.Duration, delivered int, recordPath string) {

	w := j.opts.Stdout
	t := j.art.Corpus
	fmt.Fprintf(w, "corpus: %s (%d files, %d sections, %d tokens, sha256 %s)\n",
		j.opts.Root, t.Files, t.Sections, t.Tokens, t.ContentHash)
	fmt.Fprintf(w, "budget: %d tokens per page\n", plan.Budgets.LeafTokens)
	fmt.Fprintf(w, "tree plan: %d nodes (%d pages, %d sections), %d groups, %d split\n",
		len(plan.Nodes), rep.Leaves, rep.Indexes, len(plan.Groups), splitGroups(plan))
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
	Delivered   int                    `json:"deliveredFiles"`
	Units       int                    `json:"units"`
	Produced    int                    `json:"unitsProduced"`
	Reused      int                    `json:"unitsReused"`
	ElapsedMS   int64                  `json:"elapsedMs"`
	Verify      []assemble.CheckResult `json:"verify"`
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

var (
	devBuildFlagOut       string
	devBuildFlagBudget    int
	devBuildFlagKeep      bool
	devBuildFlagBuildDate string
)

var devBuildCmd = &cobra.Command{
	Use:   "dev-build <corpus-dir>",
	Short: "build a knowledge base from a corpus with no model in the loop",
	Long: `Walk a Markdown corpus and assemble a complete, verified knowledge base out of
it: entry point, section indexes, verbatim pages, navigation links, the shipped
agent fixtures, and the nine verify gates that stand between the assembled tree
and the delivered one.

This is a development verb, and it runs the MECHANICAL half of the pipeline. No
model, no provider, no API key, no configuration directory — every stage it
runs is deterministic, so it works offline and finishes in seconds. Two things
the shipped pipeline supplies are therefore absent, and their absence is
visible in the output: the tree is grouped by the source's own file structure
rather than by a designed taxonomy, and the index pages carry no summary
prose. Both are legal shapes, and everything else — the byte derivation, the
link rebasing, the page grammar, the gates — is the shipped article.

--out is required and receives the knowledge base plus a run record. Nothing
already in it is touched: every intermediate lives under <out>/temp-work/,
which kbase creates and, on a successful run, removes. A failed or interrupted
run keeps it; --keep-temp-work keeps it after a successful one too.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := runDevBuild(cmd.Context(), devBuildOptions{
			Root:         args[0],
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
