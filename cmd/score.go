package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/modelclient"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
	"kbase/internal/statedir"
)

// scoreOptions is the score verb's input: the shared provider-reaching options,
// and the set, the run records and the streams the verb adds.
type scoreOptions struct {
	providerOptions

	// Ctx is the run's context: a cancellation stops its walk at an item
	// boundary.
	Ctx context.Context

	// Pass is the set to walk: one of query's pass names, or empty where the
	// invocation named none, which walks both. scoreWalkSet resolves it.
	Pass string

	// Limit bounds the items walked; 0 walks every one.
	Limit int

	// DryRun lists the set and asks nothing.
	DryRun bool

	// Backoff is the pause before each re-issue of an item's ask, its last
	// repeating; nil takes scoreBackoff. A test drives the full attempt count
	// through a short sequence rather than waiting out the real pauses.
	Backoff []time.Duration

	// ScoringModel is the --scoring-model flag: a reference overriding the
	// scoring role's model. It overrides a configured role and never
	// substitutes for one.
	ScoringModel string

	// Resume is the run to continue; "" starts a fresh one.
	Resume string

	// RescoreManual is the --rescore-manual flag: the pass whose manually-scored
	// values this invocation re-judges, both, or score.RescoreNone; "" is none.
	RescoreManual string

	// StateDir is the state store the run's records live under.
	StateDir string

	// ConfigDir is the --config-dir flag, "" where it was not given: the
	// directory this invocation read, which the command continuing the run
	// carries so a resume reads the same one.
	ConfigDir string

	// ProjectDir is the --project-dir flag made absolute, "" where it was not
	// given; the command continuing the run carries it.
	ProjectDir string

	// AskConcurrency is the --ask-concurrency flag, nil where it was not
	// given.
	AskConcurrency *int

	// WorkDir is where the search for the repository root starts.
	WorkDir string

	Stdout io.Writer
	Logger log.Logger
}

// scoreBackoff is the pause before each re-issue of an item's ask — the
// sequence the build's overview call pauses by, and for its reason: a 429 or a
// 5xx re-issued at once is three attempts inside one outage.
var scoreBackoff = []time.Duration{5 * time.Second, 30 * time.Second}

// callTimeout is one ask's deadline: the --timeout value, or the shared default
// where it was zero or negative.
func (o scoreOptions) callTimeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return defaultListTimeout
}

// runScore runs one score run, writes its result document, and returns the exit
// code the outcome maps to.
func runScore(opts scoreOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "score"); err != nil {
		return 0, err
	}
	outcome, fields := scoreRun(opts)
	return emitResult("score", opts.Stdout, outcome, fields)
}

// scoreRun is the verb's body: the KB, the run's ledger and the walk over the
// scoring set. Its result document is written beside the ledger before the run
// lets go of the store's holder lock, so a waiter reads the same document the
// verb reported without racing the lock's release.
func scoreRun(opts scoreOptions) (outcome string, fields []toolresult.Field) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	passes := []string{query.PassRigor, query.PassBearing, query.PassBoth}
	rescore := opts.RescoreManual
	if rescore == "" {
		rescore = score.RescoreNone
	}
	switch {
	case opts.Pass != "" && !slices.Contains(passes, opts.Pass):
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: "--pass", Allowed: passes,
			Detail: "--pass names " + strings.Join(passes, ", ") + "; absent, a run walks both"})
	case !slices.Contains(score.RescoreManual, rescore):
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: "--rescore-manual", Allowed: score.RescoreManual,
			Detail: "--rescore-manual names " + strings.Join(score.RescoreManual, ", ") + "; absent, a run re-judges no manually-scored value"})
	case opts.Limit < 0:
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: "--limit", Detail: "--limit counts items and cannot be negative"})
	case opts.Resume != "" && !runIDRE.MatchString(opts.Resume):
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: "--resume",
			Detail: fmt.Sprintf("--resume names a real run's id (%s-<%d characters of a-z0-9>), not a path: %s",
				score.RunKindReal, kb.HashLen, opts.Resume)})
	case opts.DryRun && opts.Resume != "":
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: "--resume",
			Detail: "a dry run asks nothing and records nothing to resume: drop --resume, or drop --dry-run to continue run " + opts.Resume})
	}
	concurrency, err := askConcurrency(opts.AskConcurrency, opts.Config.Asks.Concurrency)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	// The set the run's walk and its records both take: both where the flag
	// named none, so a default run is one run over both sets.
	pass := scoreWalkSet(opts.Pass)
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	fields = []toolresult.Field{{Key: "kb-root", Value: root}}
	dir, err := statedir.Dir(opts.StateDir, root)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}

	// The scoring role, resolved before the KB is opened: a configuration that
	// names no model for it refuses before the run's start refresh, with the
	// run's own keys and nothing written. A dry run asks nothing, so it
	// resolves no role and refuses nothing for one (§8).
	var ep config.Endpoint
	if !opts.DryRun {
		if ep, err = scoringRole(opts); err != nil {
			return refusedOrFailed(runKeys(fields, dir, pass), err)
		}
	}

	src, refusals, err := openKB(root)
	if err != nil {
		return failed(fields, err)
	}
	if refusals != nil {
		return refused(fields, refusals...)
	}
	fields = withMigration(fields, src, "score", opts.Stderr)
	fields = append(fields, toolresult.Field{Key: "state-dir", Value: dir}, toolresult.Field{Key: "pass", Value: pass})
	// A degraded KB is refused before the start refresh, naming each fault: the
	// refresh is the remedy, and a run would judge a graph the load resolved
	// rather than the one the records hold.
	if outcome, f, stop := refuseDegraded(fields, src); stop {
		return outcome, f
	}

	run, err := score.RunID(dir, opts.Resume, opts.DryRun)
	if err != nil {
		return failed(fields, err)
	}
	// The run's locks, in the order a build takes them: the KB's operation
	// lock, so a second run over the repository — a build or a scoring run — is
	// turned away at once, then the state store's holder lock, whose record
	// names this run for status and cancel.
	release, err := build.Hold(root, dir, buildstore.KindScore, run, time.Now())
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	defer func() {
		if err := release(); err != nil {
			opts.Logger.Error("the run's locks were released and their files not removed", "error", err)
		}
	}()

	// The run's start refresh: the set is enumerated from an index that is
	// current, and on a KB in an older metadata format this is the write that
	// saves the conversion. It holds the KB's write lock as every write does,
	// and releases it before the walk asks anything. A dry run writes nothing,
	// so it derives the index from the sources as they stand.
	if !opts.DryRun {
		if outcome, f, stop := scoreRefresh(root, src, fields, opts); stop {
			return outcome, f
		}
	}
	inv := score.Invocation{KBRoot: root, Source: src, StateDir: dir, Run: run, Resume: opts.Resume != "",
		Pass: pass, Rescore: rescore, Limit: opts.Limit, DryRun: opts.DryRun, Log: opts.Logger}
	if !opts.DryRun {
		// The registry is opened for the entry the scoring role named, and its
		// client is what every ask in this run sends through, each on a slot of
		// the run's one queue. The run's id is the key the registry gives every
		// client it opens, so every ask of this run carries it — a resumed run
		// re-derives the same id from its ledger, and so sends the same key.
		reg := modelclient.New(opts.NewClient, opts.Logger, run, concurrency)
		defer reg.CloseAll()
		inv.BackEnd, inv.Provider, inv.Model = string(config.BackEndProvider), ep.Provider, ep.Model
		inv.Launch = scoringLaunch(opts, reg, ep, concurrency)
	}
	// The document is written beside the ledger once its fields name one, before
	// the run lets go of its locks.
	defer func() { persistScoreResult(opts.Logger, fields, outcome) }()
	rep, err := score.Invoke(ctx, inv)
	// The document's roles record names the scoring role's endpoint where the
	// run reached it.
	if rep.Asked {
		fields = append(fields, rolesField([]toolresult.Record{roleRecord(ep)}))
	}
	if rep.Ledger != "" {
		fields = append(fields, toolresult.Field{Key: "run", Value: rep.Run}, toolresult.Field{Key: "ledger", Value: rep.Ledger})
	}
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	fields = append(fields, toolresult.Field{Key: "counts", Value: score.CountsRecord(rep.Counts)},
		toolresult.Field{Key: "scored", Value: rep.Scored})
	// The command that continues the run: before the end refresh and the record
	// the outcome closes on, so a stop the refresh itself takes still says how to
	// go on.
	if rep.Unfinished {
		fields = append(fields, toolresult.Field{Key: "resume", Value: scoreResume(opts, run, dir)})
	}
	// One refresh after the last write, whether the run finished, stopped at
	// its limit or was cancelled: the index is what the next reader reads. The
	// writes landed outside the source the run loaded, so the refresh publishes
	// them from the KB as it now stands. It takes the KB's write lock like every
	// write.
	if rep.Wrote {
		published, refusals, err := openKB(root)
		if outcome, f, stop := refusalOrFailure(fields, err); stop {
			return outcome, f
		}
		if refusals != nil {
			return refused(fields, refusals...)
		}
		if outcome, f, stop := scoreRefresh(root, published, fields, opts); stop {
			return outcome, f
		}
	}
	switch rep.Outcome {
	case toolresult.Failed:
		fields = append(fields, toolresult.Field{Key: toolresult.FailuresKey, Value: rep.Failures})
	case toolresult.Cancelled:
		fields = append(fields, toolresult.Field{Key: toolresult.CancelledKey, Value: rep.Stopped})
	}
	return rep.Outcome, fields
}

// scoringRole is the scoring role this invocation's settings resolve:
// --scoring-model supplying or overriding the model, nothing falling back to
// another role.
func scoringRole(opts scoreOptions) (config.Endpoint, error) {
	ep, err := config.Resolve(opts.inputs(), config.RoleScoring, opts.ScoringModel)
	if err != nil {
		return config.Endpoint{}, err
	}
	warnFaults(opts.Stderr, opts.Faults)
	return ep, nil
}

// scoringLaunch is the launch every scoring call goes through: each item
// started on the invocation's one queue of concurrency tokens, its Ask one
// tool-less chat call on ep's model through the item's slot and the client reg
// opens for it. The --timeout deadline is set inside the item, around the
// call, so it counts from the call's start and never covers the wait for a
// token. Each ask's own text carries the instruction, so the system prompt is
// empty and nothing is said twice. A call the provider refused for its key
// fails with the remedy that stores one for ep's entry.
func scoringLaunch(opts scoreOptions, reg *modelclient.Registry, ep config.Endpoint, concurrency int) score.Launch {
	client := reg.Get(ep)
	// A key bound for a non-loopback http:// endpoint is said on stderr once
	// and the call proceeds, as `build` says it.
	warnCleartextKey(opts.Stderr, ep)
	timeout := opts.callTimeout()
	backoff := opts.Backoff
	if backoff == nil {
		backoff = scoreBackoff
	}
	queue := model.NewQueue(concurrency)
	return func(ctx context.Context, wg *sync.WaitGroup, item func(score.Ask) func()) error {
		return queue.Go(ctx, wg, func(slot *model.Slot) func() {
			return item(func(ctx context.Context, prompt string) (score.Reply, error) {
				callCtx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				res, err := slot.Chat(callCtx, client, model.ChatRequest{Model: ep.Model, Prompt: prompt, Backoff: backoff})
				var refused modelclient.KeyRefusedError
				if err = modelclient.KeyRefused(err, ep.Provider); errors.As(err, &refused) {
					return score.Reply{}, score.RemedyError{Err: err, Remedy: refused.Remedy, Note: config.RefusedKeyNote}
				}
				if err != nil {
					return score.Reply{}, err
				}
				return score.Reply{Text: res.Final.Reply, Truncated: res.Final.FinishReason == model.FinishLength}, nil
			})
		})
	}
}

// runIDBody is the body of an id kb.MintID mints: the id body's length of its
// alphabet, which holds no regular-expression metacharacter.
var runIDBody = `[` + kb.HashAlphabet + `]{` + strconv.Itoa(kb.HashLen) + `}`

// runIDRE is the shape of a real run's id — kb.MintID's own, the kind run then
// the id body; a dry run's dry- id is not one, having nothing to resume.
// --resume takes one of these and never a path: the value is
// joined into the runs directory, and a directory under it is removed
// recursively, so a name that reaches further than a run's own directory
// reaches it too. TestScoreResumeRefusesAPath holds this to what MintID mints.
var runIDRE = regexp.MustCompile(`^` + score.RunKindReal + `-` + runIDBody + `$`)

// scoreWalkSet is the set a run walks: the --pass value, or both where the flag
// named none. A default run and one naming both are the same run, its header
// and its walk both taking this.
func scoreWalkSet(pass string) string {
	if pass == "" {
		return query.PassBoth
	}
	return pass
}

// runKeys is the fields a setup refusal states before the KB is opened: the
// root this invocation found, the store it would use, and the set it walks. The
// successful path states the same three around the migration record, which only
// a KB this run opened can carry.
func runKeys(fields []toolresult.Field, dir, pass string) []toolresult.Field {
	return append(slices.Clone(fields),
		toolresult.Field{Key: "state-dir", Value: dir}, toolresult.Field{Key: "pass", Value: pass})
}

// scoreRefresh is the run's refresh over src, its outcome and fields where it
// refuses or fails, and ok false where it wrote what it could. The refresh
// writes the KB, so it holds the KB's write lock; the run holds the operation
// lock itself, so no run can be holding it out. A lock it cannot take is the
// run's own answer, and the document's resume field carries the command that
// continues the run. A machine with no Graphviz dot draws no sheet, and says
// so.
func scoreRefresh(root string, src *kb.Source, fields []toolresult.Field, opts scoreOptions) (outcome string, out []toolresult.Field, stop bool) {
	release, outcome, items, err := holdWriting(root, "re-run kbase score", false)
	if err != nil {
		outcome, out = failed(fields, err)
		return outcome, out, true
	}
	if release == nil {
		return outcome, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: items}), true
	}
	defer release()
	if err := restamp(src); err != nil {
		outcome, out = failed(fields, err)
		return outcome, out, true
	}
	_, _, noDot, err := index.RefreshReporting(src, opts.Logger)
	if noDot {
		sayNoDot(opts.Stderr, "score")
	}
	return refusalOrFailure(fields, err)
}

// scoreResume is the command that continues the run: the set its header records
// over the same ledger, reaching the model this invocation was told to reach. A
// scoring-time endpoint is named by --scoring-model, which states the provider
// and the model in one value; there is no second flag for half of it. A
// --scoring-model the invocation carried is carried on, so the resumed run
// judges with the model the first one did — one issued without it would be a
// different judgement wearing the same run id.
//
// --pass is carried only where this invocation gave it: a run that walked both
// by default is continued the same way without the flag, its header being what
// says which set it walks.
//
// --config-dir is carried where it was set: a resumed run reads that directory
// and no other, so without it the same run id could resolve another reference
// and ask a different judge under the same records.
//
// --rescore-manual is carried where it names a pass: run.yaml does not record
// it, so the continuing run re-judges the manually-scored values this one did
// only if the command says so.
func scoreResume(opts scoreOptions, run, dir string) string {
	command := []string{"kbase", "score"}
	if opts.Pass != "" {
		command = append(command, "--pass", opts.Pass)
	}
	command = append(command, "--resume", run, "--state-dir", dir)
	if opts.ConfigDir != "" {
		command = append(command, "--config-dir", opts.ConfigDir)
	}
	if opts.ProjectDir != "" {
		command = append(command, "--project-dir", opts.ProjectDir)
	}
	if opts.ScoringModel != "" {
		command = append(command, "--scoring-model", opts.ScoringModel)
	}
	if opts.RescoreManual != "" && opts.RescoreManual != score.RescoreNone {
		command = append(command, "--rescore-manual", opts.RescoreManual)
	}
	return buildstore.ShellJoin(command)
}

// refusalOrFailure is err as a verb's stop: a refusal's own items where err is
// one, a retry's where a live run holds what the verb needs, a failure
// otherwise. stop is false where err is nil.
func refusalOrFailure(fields []toolresult.Field, err error) (outcome string, out []toolresult.Field, stop bool) {
	if err == nil {
		return "", nil, false
	}
	outcome, out = refusedOrFailed(fields, err)
	return outcome, out, true
}

var (
	scoreFlagTimeout      time.Duration
	scoreFlagPass         string
	scoreFlagLimit        int
	scoreFlagDryRun       bool
	scoreFlagScoringModel string
	scoreFlagResume       string
	scoreFlagStateDir     string
	scoreFlagRescore      string
	scoreFlagAskConc      int
)

// scoreReplay is the development replay's entry: given the invoked command, the
// verb its flags select in place of a run, or nil where they select none. It is
// nil in a build that carries no replay.
var scoreReplay func(cmd *cobra.Command) func(scoreOptions) (int, error)

var scoreCmd = &cobra.Command{
	Use:   "score",
	Short: "ask the scoring model for the KB's rigor and bearing values, one item per call",
	Long: `score walks the KB's scoring set, asks the configured scoring model for the
faults one item's text commits, prices the charge lines it answers with, and
lands the value through the write API: set-rigor for a node, set-bearing for an
edge, never a record or a leaf edit of its own. One refresh opens the run — the
index the set is read from is current, and a KB in an older metadata format is
converted and saved by it — and one more follows the last write. A degraded KB
is refused before that refresh, naming each fault; kbase refresh is the remedy.
Under --dry-run, which runs no refresh, a KB in an older metadata format is
refused; kbase refresh saves the conversion.

The scoring role is the KB's declaration of which model judges it: models.scoring
in config.toml. A run whose configuration names no scoring model and whose flags
name none is refused, naming that key, before the KB is opened; nothing falls
back to the text model. --scoring-model names the role's model for one run — a
<provider>/<model> reference — supplying the role where the configuration names
none and overriding it where it does, and is carried by the command that
continues a run. --dry-run lists the set and
resolves no role, so a missing one does not refuse it.

The passes are rigor — every claim, analysis and experiment, by id — and
bearing — every supports, lifts and refutes edge, by relation, then source,
then target. A run walks both unless --pass names one, over the node set and
then the edge set. A text edit moves a node's rigor and the bearing of every
edge that touches it, so a run scoped to one pass leaves the other stale against
the same edit: scope it where there is a reason — one edited subtree, or
isolating a pass that is failing. A work's rigor is the one value neither pass
asks: it is the work's own standing, judged by hand with the document in hand,
and set-rigor is what writes it. An axiom's is 1.0 by rule. A founds edge's
bearing is the other: the work or axiom it rests on lies outside the corpus, so
no pass reaches it either, and set-bearing writes it.

Every attempt is recorded under <state-dir>/score/<run>/: run.yaml names the
run, the set it walks and the digest of the ask its judgements answer,
ledger.jsonl holds a line per attempt with the charges a reply recorded, the
reply itself where it parsed, the value it prices to, the model and the prompt's
digest, captures.jsonl holds each call made — the closed text, the prompt sent
and the reply returned, a failed attempt's included — and items.jsonl lists the
set. A run's id is run-<id>, a --dry-run's dry-<id>; the newest ` + strconv.Itoa(buildstore.KeptRuns) + ` runs of each
kind are kept, and a run's start removes the older ones of its kind whole.
--dry-run writes the run's records of the set, asks nothing, writes no KB and
cannot be resumed. --resume continues a real run,
leaving every item it already landed and re-asking the ones that failed or were
not reached; a fresh run asks everything again. A landed value whose
recorded charges the running kbase prices differently is re-priced by the
resume, no model asked, and the move recorded in repriced.yaml. --limit stops
after that many items, and the result carries the command that continues the
run.

A value a person supplied — through set-rigor, set-bearing or an
insert-*-entry op that gives a rigor or bearing — is marked manually scored; the
run's own landings are not. A marked value is skipped: the run asks nothing
about it, lands nothing on it and records it skipped, unless --rescore-manual
names its pass or both. A skip is the person's judgement standing, not a
failure: no resume command is issued for it. --rescore-manual rigor, bearing or
both re-judges that pass's marked values: the run's value replaces the person's
and lands unmarked, so every later run re-judges it as well. The flag holds for
one invocation and is carried by the command that continues a run; a resume
issued without it leaves a skipped item skipped, and re-asks it only where its
own --rescore-manual names that item's pass.

A run holds the repository's operation lock and <state-dir> for its length, so
a second run over the KB — a build or a scoring run — is refused, naming the
holder and its pid. SIGINT or SIGTERM stops it at an item boundary. Its result
is one YAML document on stdout.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := invocationDir(cmd)
		if err != nil {
			return err
		}
		projectDir, err := absIfSet(flagProjectDir)
		if err != nil {
			return err
		}
		configDir, err := absIfSet(flagConfigDir)
		if err != nil {
			return err
		}
		opts, err := loadVerbContext(wd)
		if err != nil {
			return err
		}
		opts.Timeout = scoreFlagTimeout
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		verb := runScore
		if scoreReplay != nil {
			if replay := scoreReplay(cmd); replay != nil {
				verb = replay
			}
		}
		code, err := verb(scoreOptions{providerOptions: opts, Ctx: ctx, Pass: scoreFlagPass, Limit: scoreFlagLimit,
			DryRun: scoreFlagDryRun, ScoringModel: scoreFlagScoringModel,
			Resume: scoreFlagResume, RescoreManual: scoreFlagRescore, StateDir: scoreFlagStateDir, ConfigDir: configDir, ProjectDir: projectDir,
			AskConcurrency: givenAskConcurrency(cmd, scoreFlagAskConc), WorkDir: wd,
			Stdout: cmd.OutOrStdout(), Logger: processLog.logger})
		if err != nil {
			return err
		}
		if code != 0 {
			return exitCode(code)
		}
		return nil
	},
}

func init() {
	// This verb bounds one item's call, not a catalogue request; it selects no
	// pool entry, so it carries no --provider — a scoring-time endpoint is
	// --scoring-model, which states the provider and the model together.
	registerTimeoutFlag(scoreCmd, &scoreFlagTimeout, "deadline for one item's call")
	scoreCmd.Flags().StringVar(&scoreFlagPass, "pass", "",
		"the set to walk: "+query.PassRigor+" for a node's own value, "+query.PassBearing+" for the value on a supports, lifts or refutes edge; absent or "+query.PassBoth+" walks the node set and then the edge set, because one edit moves a node's rigor and the bearings of the edges that touch it")
	scoreCmd.Flags().IntVar(&scoreFlagLimit, "limit", 0, "stop after this many items (default: every item)")
	scoreCmd.Flags().BoolVar(&scoreFlagDryRun, "dry-run", false, "list the set and ask nothing")
	scoreCmd.Flags().StringVar(&scoreFlagScoringModel, "scoring-model", "",
		"model that judges the KB, overriding models.scoring: <provider>/<model>")
	scoreCmd.Flags().StringVar(&scoreFlagResume, "resume", "", "continue the run with this id")
	scoreCmd.Flags().StringVar(&scoreFlagRescore, "rescore-manual", score.RescoreNone,
		"re-judge values marked manually scored: none, rigor, bearing or both")
	scoreCmd.Flags().StringVar(&scoreFlagStateDir, "state-dir", "", "the state store (default: $XDG_STATE_HOME/kbase/<key>)")
	registerAskConcurrencyFlag(scoreCmd, &scoreFlagAskConc)
}
