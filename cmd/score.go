package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/model"
	"kbase/internal/modelclient"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
	"kbase/internal/statedir"
)

// scoreOptions is the score verb's input: the shared provider-reaching options,
// and the scope, the run records and the streams the verb adds.
type scoreOptions struct {
	providerOptions

	// Ctx is the run's context: a cancellation stops its walk at an item
	// boundary.
	Ctx context.Context

	// Pass is the pass a full run walks: one of query.Passes, or empty where
	// the invocation named none, which walks every pass.
	Pass string

	// Node is the --node flag: the one node whose rigor the run scores, ""
	// where it names none.
	Node string

	// Edge is the --edge flag as the command line gives it,
	// <source>,<target>,<relation>: the one edge whose bearing the run scores,
	// "" where it names none.
	Edge string

	// DryRun lists what a run would ask and asks nothing.
	DryRun bool

	// Backoff is the pause before each re-issue of an item's ask, its last
	// repeating; nil takes scoreBackoff. A test drives the full attempt count
	// through a short sequence rather than waiting out the real pauses.
	Backoff []time.Duration

	// ScoringModel is the --scoring-model flag: a reference overriding the
	// scoring role's model. It overrides a configured role and never
	// substitutes for one.
	ScoringModel string

	// Resume continues the repository's most recent score run.
	Resume bool

	// RescoreManual is the --rescore-manual flag: the run re-judges manually
	// scored values rather than skipping them.
	RescoreManual bool

	// StateDir is the state store the run's records live under.
	StateDir string

	// ConfigDir is the --config-dir flag, "" where it was not given: the
	// directory this invocation read, which the call continuing the run
	// carries so a resume reads the same one.
	ConfigDir string

	// ProjectDir is the --project-dir flag made absolute, "" where it was not
	// given; the call continuing the run carries it.
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

// scoreUsage is each usage refusal the invocation's scope arguments earn
// together, before anything is opened: a pass no run walks; resume beside an
// argument naming a scope, which the run it continues recorded; pass beside a
// named item, whose pass follows from its kind; node and edge together, a run
// naming one item; an edge that does not read as one; and a dry run resumed.
// It returns the edge the invocation names, nil where it names none.
func scoreUsage(opts scoreOptions) (*query.ScoringEdge, []toolresult.Item) {
	var items []toolresult.Item
	if opts.Pass != "" && !slices.Contains(query.Passes, opts.Pass) {
		items = append(items, toolresult.Item{Check: toolresult.CheckUsage, Key: argmark.Flag("pass"), Allowed: query.Passes,
			Detail: argmark.Flag("pass") + " names " + proseSeries(query.Passes, "or") + "; absent, a run walks " + query.PassAll})
	}
	if opts.Resume {
		for _, given := range []struct {
			name string
			set  bool
		}{{"pass", opts.Pass != ""}, {"node", opts.Node != ""}, {"edge", opts.Edge != ""}, {"rescore-manual", opts.RescoreManual}} {
			if given.set {
				items = append(items, toolresult.Item{Check: toolresult.CheckUsage, Key: argmark.Flag(given.name),
					Detail: argmark.Flag("resume") + " continues the most recent run with the scope it recorded, and takes no " +
						argmark.Flag(given.name) + " beside it",
					Remedy: "drop " + argmark.Flag(given.name)})
			}
		}
		if opts.DryRun {
			items = append(items, toolresult.Item{Check: toolresult.CheckUsage, Key: argmark.Flag("resume"),
				Detail: "a resume continues a recorded run and has no dry run",
				Remedy: "drop " + argmark.Flag("dry-run") + " to continue the most recent run, or drop " + argmark.Flag("resume") +
					" to preview a fresh run"})
		}
	}
	if opts.Pass != "" && (opts.Node != "" || opts.Edge != "") {
		items = append(items, toolresult.Item{Check: toolresult.CheckUsage, Key: argmark.Flag("pass"),
			Detail: "a named item's pass follows from its kind — a node's rigor, an edge's bearing — so " + argmark.Flag("pass") +
				" stands beside neither " + argmark.Flag("node") + " nor " + argmark.Flag("edge"),
			Remedy: "drop " + argmark.Flag("pass")})
	}
	if opts.Node != "" && opts.Edge != "" {
		items = append(items, toolresult.Item{Check: toolresult.CheckUsage, Key: argmark.Flag("edge"),
			Detail: "a run names one item, " + argmark.Flag("node") + " or " + argmark.Flag("edge"),
			Remedy: "drop one of them"})
	}
	var edge *query.ScoringEdge
	if opts.Edge != "" {
		parts := strings.Split(opts.Edge, ",")
		if len(parts) != 3 || slices.Contains(parts, "") {
			items = append(items, toolresult.Item{Check: toolresult.CheckUsage, Key: argmark.Flag("edge"),
				Detail: argmark.Flag("edge") + " names one edge by its source, target and relation, none of which holds a comma, not " +
					strconv.Quote(opts.Edge)})
		} else {
			edge = &query.ScoringEdge{Source: parts[0], Target: parts[1], Relation: parts[2]}
		}
	}
	return edge, items
}

// scorePass is the pass the invocation's run walks, a resume's aside: a named
// node's rigor, a named edge's bearing, else the --pass value, every pass where
// the invocation named none.
func scorePass(opts scoreOptions) string {
	switch {
	case opts.Node != "":
		return query.PassRigor
	case opts.Edge != "":
		return query.PassBearing
	case opts.Pass != "":
		return opts.Pass
	}
	return query.PassAll
}

// scoreRun is the verb's body: the KB, the run's ledger and the walk over its
// scope. Its result document is written beside the ledger before the run lets
// go of the store's holder lock, so a waiter reads the same document the verb
// reported without racing the lock's release.
func scoreRun(opts scoreOptions) (outcome string, fields []toolresult.Field) {
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	edge, usage := scoreUsage(opts)
	if usage != nil {
		return refused(nil, usage...)
	}
	concurrency, err := askConcurrency(opts.AskConcurrency, opts.Config.Asks.Concurrency)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
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

	// A resume continues the most recent run with the scope its header
	// recorded: its pass, and whether it re-judges manually scored values.
	pass, rescore, resumed := scorePass(opts), opts.RescoreManual, ""
	if opts.Resume {
		h, err := score.ResumeTarget(dir)
		if outcome, f, stop := refusalOrFailure(append(slices.Clone(fields), toolresult.Field{Key: "state-dir", Value: dir}), err); stop {
			return outcome, f
		}
		pass, rescore, resumed = h.Pass, h.RescoreManual, h.Run
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
	// A KB whose load fails is refused before the run takes its locks, naming
	// each fault.
	if outcome, f, stop := refuseLoadError(fields, src); stop {
		return outcome, f
	}
	// A KB in an older format is refused, dry run or not: only update-format
	// writes over it.
	if outcome, f, stop := refusalOrFailure(fields, index.PendingConversion(src)); stop {
		return outcome, f
	}

	run, err := score.RunID(dir, resumed, opts.DryRun)
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

	// The run's start refresh: the derived files are current before the walk,
	// and each landing's own derivation keeps them so. It holds the KB's write
	// lock as every write does, and releases it before the walk asks anything.
	// A dry run writes nothing.
	if !opts.DryRun {
		if outcome, f, stop := scoreRefresh(root, src, fields, opts); stop {
			return outcome, f
		}
	}
	inv := score.Invocation{KBRoot: root, Source: src, StateDir: dir, Run: run, Resume: opts.Resume,
		Pass: pass, Node: opts.Node, Edge: edge, Rescore: rescore, DryRun: opts.DryRun, Log: opts.Logger, Open: kbload.Open}
	if !opts.DryRun {
		// The registry is opened for the entry the scoring role named, and its
		// client is what every ask in this run sends through, each on a slot of
		// the run's one queue. The run's id is the key the registry gives every
		// client it opens, so every ask of this run carries it — a resumed run
		// continues the same id, and so sends the same key.
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
	fields = append(fields, toolresult.Field{Key: "counts", Value: score.CountsRecord(rep.Counts)})
	if opts.DryRun {
		fields = append(fields, toolresult.Field{Key: "ask", Value: rep.Ask}, toolresult.Field{Key: "skip", Value: rep.Skip})
	}
	fields = append(fields, toolresult.Field{Key: "scored", Value: rep.Scored})
	if rep.Unfinished {
		fields = append(fields, toolresult.Field{Key: "resume", Value: scoreResume(opts, dir)})
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
// opens for it — opened by the first item launched, so a run that asks
// nothing, its scope refused or holding no item, opens none. The --timeout
// deadline is set inside the item, around the call, so it counts from the
// call's start and never covers the wait for a token. Each ask's own text
// carries the instruction, so the system prompt is empty and nothing is said
// twice. A call the provider refused for its key fails with the remedy that
// stores one for ep's entry.
func scoringLaunch(opts scoreOptions, reg *modelclient.Registry, ep config.Endpoint, concurrency int) score.Launch {
	client := sync.OnceValue(func() model.Client { return reg.Get(ep) })
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
				res, err := slot.Chat(callCtx, client(), model.ChatRequest{Model: ep.Model, Prompt: prompt, Backoff: backoff})
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

// runKeys is the fields a setup refusal states before the KB is opened: the
// root this invocation found, the store it would use, and the pass it walks.
// The successful path states the same three around the migration record, which
// only a KB this run opened can carry.
func runKeys(fields []toolresult.Field, dir, pass string) []toolresult.Field {
	return append(slices.Clone(fields),
		toolresult.Field{Key: "state-dir", Value: dir}, toolresult.Field{Key: "pass", Value: pass})
}

// scoreRefresh is the run's refresh over src, its outcome and fields where it
// refuses or fails, and ok false where it wrote what it could. The refresh
// writes the KB, so it holds the KB's write lock; the run holds the operation
// lock itself, so no run can be holding it out. A lock it cannot take is the
// run's own answer. A machine with no Graphviz dot draws no sheet, and says
// so.
func scoreRefresh(root string, src *kb.Source, fields []toolresult.Field, opts scoreOptions) (outcome string, out []toolresult.Field, stop bool) {
	release, outcome, items, err := holdWriting(root, "re-run "+argmark.Call("score", nil), false)
	if err != nil {
		outcome, out = failed(fields, err)
		return outcome, out, true
	}
	if release == nil {
		return outcome, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: items}), true
	}
	defer release()
	_, _, noDot, err := index.RefreshReporting(src, opts.Logger)
	if noDot {
		sayNoDot(opts.Stderr, "score")
	}
	return refusalOrFailure(fields, err)
}

// scoreResume is the call that continues the run: a resume over the same
// store, reaching the model this invocation was told to reach. A scoring-time
// endpoint is named by --scoring-model, which states the provider and the
// model in one value; a --scoring-model the invocation carried is carried on,
// so the resumed run judges with the model the first one did — one issued
// without it would be a different judgement wearing the same run id.
// --config-dir is carried where it was set: a resumed run reads that directory
// and no other, so without it the same run could resolve another reference
// and ask a different judge under the same records. The scope is the run's
// header's, so no argument naming it is carried.
func scoreResume(opts scoreOptions, dir string) string {
	args := map[string]any{"resume": true, "state-dir": dir}
	for name, value := range map[string]string{"config-dir": opts.ConfigDir, "project-dir": opts.ProjectDir,
		"scoring-model": opts.ScoringModel} {
		if value != "" {
			args[name] = value
		}
	}
	return argmark.Call("score", args)
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
	scoreFlagNode         string
	scoreFlagEdge         string
	scoreFlagDryRun       bool
	scoreFlagScoringModel string
	scoreFlagResume       bool
	scoreFlagStateDir     string
	scoreFlagRescore      bool
	scoreFlagAskConc      int
)

// scoreReplay is the development replay's entry: given the invoked command, the
// verb its flags select in place of a run, or nil where they select none. It is
// nil in a build that carries no replay.
var scoreReplay func(cmd *cobra.Command) func(scoreOptions) (int, error)

// scoreLong is score's shared help body.
var scoreLong = `score asks the scoring model for the KB's rigor and bearing values, one item
per call, and prices the faults each reply names. When the walk ends — done,
cancelled or failed — the answered values land in one write, with the
manually-scored mark false, and the derivation follows it as it follows every
write. A KB whose load fails is refused, naming each fault with its file and
record. A KB in an older format is refused, ` + argmark.Flag("dry-run") + ` or not,
remedy ` + index.UpdateFormatRemedy + `, which saves the conversion.

A full run asks what is dirty: an item whose judged hash — the hash of its
text, a node's statement or the statements of an edge's two ends, with the
digest of the ask template that judges it — differs from the hash its record
carries — its text or the ask changed since it was judged. A value never
judged is dirty. The rigor pass covers claims, analyses
and experiments; the bearing pass covers ` + proseSeries(kb.BearingWalkRelations, "and") + `
edges. No pass reaches a work's rigor, judged by hand, an axiom's, 1.0 by
rule, or a founds edge's bearing. ` + argmark.Flag("pass") + ` names the pass a full run walks:
` + proseSeries(query.Passes, "or") + `, the default. ` + argmark.Flag("node") + ` names one claim, analysis or
experiment and ` + argmark.Flag("edge") + ` one ` + markedRelations() + ` edge, scored whether
dirty or not; a named item's pass follows from its kind, so ` + argmark.Flag("pass") + ` beside
either is refused. There is no item limit: a run asks every item of its
scope.

A value a person set is manually scored: a full run asks a dirty one nothing
and records it skipped, and naming one is refused. ` + argmark.Flag("rescore-manual") + ` re-judges
them: a full run then also asks every manually scored item of its pass, dirty
or not, and a named one is asked; the run's value replaces the person's and
lands unmarked, so later runs judge it whenever it is dirty, like any other.

` + argmark.Flag("dry-run") + ` lists under ask every node and edge the run would ask, counted in
counts, and under skip the dirty manually scored values it would skip; it asks
nothing, resolves no scoring role, writes no KB and cannot be resumed.

` + argmark.Flag("resume") + ` continues the repository's most recent score run with the scope it
recorded, keeping every value it landed — re-pricing, no model asked, any whose
recorded charges the running kbase prices differently — and re-asking the
items that failed or were not reached; it takes no ` + argmark.Flag("pass") + `, ` + argmark.Flag("node") + `,
` + argmark.Flag("edge") + ` or ` + argmark.Flag("rescore-manual") + ` beside it. It is refused where that run
finished or where a build has started since it: a fresh run then asks only
what is dirty. A run that failed an item or stopped short has a resume field,
in its result and in its status: the call that continues it. ` + argmark.Call("cancel", nil) + `
stops a run; the values already answered land and stay.

The scoring role is the KB's declaration of which model judges it:
models.scoring in config.toml. A run whose configuration names no scoring model
and whose arguments name none is refused, naming that key; nothing falls back
to the text model. ` + argmark.Flag("scoring-model") + ` names the role's
model for one run — a <provider>/<model> reference — supplying the role where
the configuration names none and overriding it where it does, and the call
continuing a run carries it.

Every attempt — the prompt sent, the reply and the value it priced to — is
recorded under the result's ledger, <state-dir>/score/<run>/.

A run holds the repository's operation lock and <state-dir> for its length, so
a second run over the KB — a build or a scoring run — is refused, naming the
holder and its pid.`

// scoreCLINote is what differs for score at the command line.
const scoreCLINote = `At the command line score runs in the foreground; SIGINT or SIGTERM (kbase
cancel) stops it resumably.`

var scoreCmd = mcp.Notes(&cobra.Command{
	Use:   "score",
	Short: "ask the scoring model for the KB's rigor and bearing values not yet judged against their current text, one item per call",
	Long:  scoreLong,
	Args:  cobra.NoArgs,
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
		code, err := verb(scoreOptions{providerOptions: opts, Ctx: ctx, Pass: scoreFlagPass, Node: scoreFlagNode, Edge: scoreFlagEdge,
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
}, runLaunchMCPNote, scoreCLINote)

func init() {
	// This verb bounds one item's call, not a catalogue request; it selects no
	// pool entry, so it carries no --provider — a scoring-time endpoint is
	// --scoring-model, which states the provider and the model together.
	registerTimeoutFlag(scoreCmd, &scoreFlagTimeout, "deadline for one item's call")
	scoreCmd.Flags().StringVar(&scoreFlagPass, "pass", "",
		"the pass a full run walks: "+proseSeries(query.Passes, "or")+" (default: "+query.PassAll+")")
	scoreCmd.Flags().StringVar(&scoreFlagNode, "node", "",
		"score this node's rigor, dirty or not: a claim, analysis or experiment id")
	scoreCmd.Flags().StringVar(&scoreFlagEdge, "edge", "",
		"score this edge's bearing, dirty or not: its source, target and relation")
	mcp.MappingFlag(scoreCmd.Flags().Lookup("edge"), "source", "target", "relation")
	scoreCmd.Flags().BoolVar(&scoreFlagDryRun, "dry-run", false,
		"list what a run would ask and the dirty manually scored values it would skip; ask nothing")
	scoreCmd.Flags().StringVar(&scoreFlagScoringModel, "scoring-model", "",
		"model that judges the KB, overriding models.scoring: <provider>/<model>")
	scoreCmd.Flags().BoolVar(&scoreFlagResume, "resume", false, "continue the most recent score run with the scope it recorded")
	scoreCmd.Flags().BoolVar(&scoreFlagRescore, "rescore-manual", false,
		"re-judge values a person set too: the run's value replaces each and clears its manually-scored mark; required to name a manually scored node or edge")
	scoreCmd.Flags().StringVar(&scoreFlagStateDir, "state-dir", "", runStateDirUsage)
	registerAskConcurrencyFlag(scoreCmd, &scoreFlagAskConc)
}
