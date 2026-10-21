package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/build"
	"kbase/internal/config"
	"kbase/internal/filelock"
	"kbase/internal/harness"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
	"kbase/internal/statedir"
	"kbase/internal/write"
)

// scoreOptions is the score verb's input: the shared provider-reaching options,
// and the pass, the run records and the streams the verb adds.
type scoreOptions struct {
	providerOptions

	// Ctx is the run's context: a cancellation stops its walk at an item
	// boundary.
	Ctx context.Context

	// Pass is which pass to walk, one of query's two pass names.
	Pass string

	// Limit bounds the items walked; 0 walks every one.
	Limit int

	// DryRun lists the set and asks nothing.
	DryRun bool

	// ScoringModel is the --scoring-model flag: a reference overriding the
	// scoring role's model in file mode, a bare id under the environment's
	// endpoint, the role's value whole under --harness. It overrides
	// a configured role and never substitutes for one.
	ScoringModel string

	// Harness is the --harness flag: which back end asks each item, which is
	// the harness back end and the end of every provider question this run
	// would otherwise ask. `opencode` is the one value it takes.
	Harness string

	// Resume is the run to continue; "" starts a fresh one.
	Resume string

	// StateDir is the state store the run's records live under.
	StateDir string

	// ConfigDir is the --config-dir flag, "" where it was not given: the
	// directory this invocation read, which the command continuing the run
	// carries so a resume reads the same one.
	ConfigDir string

	// WorkDir is where the search for the repository root starts.
	WorkDir string

	Stdout io.Writer
	Logger log.Logger
}

// callTimeout is one ask's deadline: the --timeout value, or the shared default
// where it was zero or negative.
func (o scoreOptions) callTimeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return defaultListTimeout
}

// runScore runs one score pass, writes its result document, and returns the
// exit code the outcome maps to.
func runScore(opts scoreOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "score"); err != nil {
		return 0, err
	}
	outcome, fields := scoreRun(opts)
	return emitResult("score", opts.Stdout, outcome, fields)
}

// scoreRun is the verb's body: the KB, the run's ledger and the walk over the
// scoring set.
func scoreRun(opts scoreOptions) (string, []toolresult.Field) {
	passes := []string{query.PassRigor, query.PassBearing}
	switch {
	case !slices.Contains(passes, opts.Pass):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--pass", Allowed: passes,
			Detail: "--pass is required and names one pass: " + strings.Join(passes, " or ")})
	case opts.Harness != "" && opts.Harness != opencodeHarness:
		return refused(nil, toolresult.Item{Check: config.CheckHarness, Key: "--harness", Allowed: []string{opencodeHarness},
			Detail: fmt.Sprintf("--harness names %q, and kbase runs no harness profile: its one back end is %q, which kbase composes itself",
				opts.Harness, opencodeHarness)})
	case opts.Limit < 0:
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--limit", Detail: "--limit counts items and cannot be negative"})
	case opts.Resume != "" && !runIDRE.MatchString(opts.Resume):
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--resume",
			Detail: fmt.Sprintf("--resume names a run id kbase minted (run-<%d characters of a-z0-9>), not a path: %s",
				kb.HashLen, opts.Resume)})
	case opts.DryRun && opts.Resume != "":
		return refused(nil, toolresult.Item{Check: checkUsage, Key: "--resume",
			Detail: "a dry run asks nothing and records nothing to resume: drop --resume, or drop --dry-run to continue run " + opts.Resume})
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
	fields := []toolresult.Field{{Key: "kb-root", Value: root}}
	dir, err := statedir.Dir(opts.StateDir, root)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}

	// Which back end asks, decided once with the role, before the KB is opened:
	// a configuration that names no model for the scoring role, a command that
	// cannot start or a flag a command cannot carry refuses before the run's
	// start refresh, with the run's own keys and nothing written. A dry run asks
	// nothing, so it resolves no role and refuses nothing for one; a --harness
	// value kbase ships no back end for is refused whatever the run does, ahead
	// of the KB (§8).
	var (
		ep   config.Endpoint
		hcmd *harness.Command
		// itemEnv is the environment each item's process is started with, nil
		// for the one kbase runs under: a back end that composes one composes
		// it per item, from the directory the item runs in.
		itemEnv func(dir string) []string
		// override is the argv the back end override named, nil where it named
		// none: the command a run executes that kbase did not compose.
		override []string
		mode     config.Mode
	)
	env := config.EnvOverrides(os.Getenv)
	if !opts.DryRun {
		s := opts.inputs(env)
		if opts.Harness != "" {
			// A harness chooses no endpoint, so it consults no mode: a base URL
			// or a role variable set beside it is neither a mixed mode nor a
			// refusal (§4.5), and the scoring role resolves from the file's
			// reference or the environment's id alone.
			override = backEndCommand(os.Getenv)
			ep, hcmd, itemEnv, err = opencodeSetup(ctx, s, opts, override)
			if err != nil {
				return refusedOrFailed(runKeys(fields, dir, opts.Pass), err)
			}
			if override != nil {
				announceBackEndCommand(opts.Stderr, override)
			}
		} else {
			if mode, err = config.ModeOf(s); err != nil {
				return refusedOrFailed(runKeys(fields, dir, opts.Pass), err)
			}
			if mode == config.ModeFile {
				// An environment-mode invocation reads no file value, so it
				// says nothing of the key the files state.
				supersededProvider(opts.Stderr, opts.Config)
			}
			if ep, err = config.Resolve(s, mode, config.RoleScoring, opts.ScoringModel); err != nil {
				return refusedOrFailed(runKeys(fields, dir, opts.Pass), err)
			}
			warnFaults(opts.Stderr, opts.Faults)
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
	fields = append(fields, toolresult.Field{Key: "state-dir", Value: dir}, toolresult.Field{Key: "pass", Value: opts.Pass})

	if err := os.MkdirAll(score.RunsDir(dir), 0o777); err != nil {
		return failed(fields, err)
	}
	run := opts.Resume
	if run == "" {
		run = kb.MintID("run", func(id string) bool {
			_, err := os.Stat(filepath.Join(score.RunsDir(dir), id))
			return err == nil
		})
	}
	// A run asks its questions over minutes, so a second run is turned away at
	// once rather than held through a wait the first will outlast. The lock is
	// the runs' own directory: it stands before any run does, and it names the
	// place a reader inspecting the contention looks.
	lock, err := filelock.Acquire(score.RunsDir(dir), 0)
	if errors.Is(err, filelock.ErrHeld) {
		return toolresult.Retry, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: []toolresult.Item{{
			Check: checkLock, Path: score.RunsDir(dir), Remedy: "re-run kbase score when the run holding it has ended",
			Detail: "another score run holds " + score.RunsDir(dir) + "; nothing was asked or written"}}})
	}
	if err != nil {
		return failed(fields, err)
	}
	defer lock.Release()

	// The run's start refresh: the set is enumerated from an index that is
	// current, and on a KB in an older metadata format this is the write that
	// saves the conversion. It holds the KB's write lock as every write does,
	// and releases it before the walk asks anything. A dry run writes nothing,
	// so it reads the index as it stands and refuses one the load cannot answer.
	if !opts.DryRun {
		if outcome, f, stop := scoreRefresh(root, src, fields, opts); stop {
			return outcome, f
		}
	}
	ix, err := query.Load(src)
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	items, err := query.ScoringSet(ix, opts.Pass)
	if err != nil {
		return failed(fields, err)
	}

	if opts.DryRun {
		l, err := score.Create(dir, score.RunHeader{Run: run, KBRoot: root, Pass: opts.Pass, Started: scoreStamp()})
		if err != nil {
			return failed(fields, err)
		}
		defer l.Close()
		if err := l.WriteItems(items); err != nil {
			return failed(fields, err)
		}
		fields = append(fields, toolresult.Field{Key: "run", Value: run}, toolresult.Field{Key: "ledger", Value: l.Dir()})
		res, err := (&score.Runner{DryRun: true, Items: items}).Run(ctx)
		if err != nil {
			return failed(fields, err)
		}
		return res.Outcome, append(fields, scoreCounted(res)...)
	}

	backEnd := config.BackEndProvider
	if hcmd != nil {
		backEnd = config.BackEndHarness
	}
	// A provider endpoint is dialled and a harness command is not: the registry
	// is opened for the back end that has an endpoint, and a harness run opens
	// no client at all.
	reg := newClients(opts.NewClient, opts.Logger)
	defer reg.closeAll()
	var client model.Client
	if hcmd == nil {
		if client, err = reg.get(ep); err != nil {
			return refusedOrFailed(fields, err)
		}
		// A key bound for a non-loopback http:// endpoint is said on stderr once
		// and the run proceeds, as `build` says it: the key is what this run
		// sends, and the registry is what read it. A harness sends no key.
		if key, err := reg.key(ep); err == nil {
			if line := config.CleartextWarning(ep.BaseURL, key); line != "" {
				fmt.Fprintln(opts.Stderr, line)
			}
		}
	}
	modelID := ep.Model
	fields = append(fields, rolesField([]toolresult.Record{roleRecord(ep, backEnd)}))
	// The run's record, opened below and needed above: a harness item runs in
	// the run's own working directory, so the ask reads the run's path once the
	// ledger stands.
	var l *score.Ledger

	timeout := opts.callTimeout()
	// The ask is one tool-less chat call on the role's model, or one process on
	// the back end's command. Each ask's own text carries the instruction, so
	// the system prompt is empty and nothing is said twice.
	ask := func(ctx context.Context, prompt string) (string, error) {
		if hcmd != nil {
			item, err := hcmd.Item(prompt)
			if err != nil {
				return "", err
			}
			work := filepath.Join(l.Dir(), harnessDirName)
			if itemEnv != nil {
				item = item.WithEnv(itemEnv(work))
			}
			return harnessAsk(ctx, item, work, timeout)
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		attempt, err := model.Chat(callCtx, client, modelID, "", prompt, nil, nil)
		if err != nil {
			return "", err
		}
		// A reply cut off at the completion cap loses the lines it did not
		// reach, and where the cut falls between lines the rest would parse as a
		// clean under-charge. It fails the item for what it is: the charge list
		// is incomplete, and an incomplete answer is never a judgement.
		if attempt.FinishReason == model.FinishLength {
			return "", errors.New("the reply reached the completion cap before it finished, so its charge list is incomplete")
		}
		return attempt.Reply, nil
	}
	header := score.RunHeader{Run: run, KBRoot: root, Pass: opts.Pass, BackEnd: backEndName(ep, backEnd),
		Provider: ep.Provider, Model: modelID, Started: scoreStamp()}
	if hcmd != nil {
		header.Command = hcmd.Argv()
		if override != nil {
			header.CommandOverride = backEndCommandEnv
		}
	}
	// One write seam for both: the values the walk lands and the values a resume
	// re-prices under a changed table take the same path, the KB's lock held as
	// every write holds it.
	land := func(it query.Item, value kb.Bearing) error { return landScore(root, it, value, opts.Logger) }
	if opts.Resume != "" {
		l, err = score.Resume(dir, header, land)
	} else {
		l, err = score.Create(dir, header)
	}
	if outcome, f, stop := refusalOrFailure(fields, err); stop {
		return outcome, f
	}
	defer l.Close()
	fields = append(fields, toolresult.Field{Key: "run", Value: run}, toolresult.Field{Key: "ledger", Value: l.Dir()})

	res, err := (&score.Runner{Source: src, Index: ix, Items: items, Pass: opts.Pass, Ask: ask,
		Write: land, Ledger: l, Model: modelID, Limit: opts.Limit, Log: opts.Logger}).Run(ctx)
	if err != nil {
		return failed(fields, err)
	}
	fields = append(fields, scoreCounted(res)...)
	// The command that continues the run, wherever an item is not done: before
	// the end refresh and the record the outcome closes on, so a stop the
	// refresh itself takes still says how to go on.
	if res.Counts.Done < res.Counts.Total {
		fields = append(fields, toolresult.Field{Key: "resume", Value: scoreResume(opts, run, dir, mode, env)})
	}
	// One refresh after the last write, whether the run finished, stopped at
	// its limit or was cancelled: the index is what the next reader reads. The
	// writes landed outside the source the run loaded, so the refresh publishes
	// them from the KB as it now stands. It takes the KB's write lock like every
	// write. A resume that re-priced a value wrote as well, whether or not its
	// walk landed one.
	if len(res.Written) > 0 || l.Repriced() > 0 {
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
	switch res.Outcome {
	case toolresult.Failed:
		fields = append(fields, toolresult.Field{Key: toolresult.FailuresKey, Value: scoreFailures(res.Walked, l)})
	case toolresult.Cancelled:
		fields = append(fields, toolresult.Field{Key: toolresult.CancelledKey, Value: scoreStopped(res.Walked, l)})
	}
	return res.Outcome, fields
}

// harnessAsk is the harness back end's Ask: one item, one process, in the run's
// own working directory — the same directory for every item, emptied before
// each so nothing one command wrote is visible to the next. The deadline is the
// run's per-item one, so a SIGINT cancels the child as it cancels a provider
// call.
func harnessAsk(ctx context.Context, item harness.Command, dir string, timeout time.Duration) (string, error) {
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return item.Ask(callCtx, dir)
}

// harnessDirName is the run's working directory for the commands it runs.
const harnessDirName = "harness"

// runIDRE is the shape of the run ids kbase mints — kb.MintID's own, the kind
// then the id body's length of its alphabet, which holds no regular-expression
// metacharacter. --resume takes one of these and never a path: the value is
// joined into the runs directory, and a directory under it is removed
// recursively, so a name that reaches further than a run's own directory
// reaches it too. TestScoreResumeRefusesAPath holds this to what MintID mints.
var runIDRE = regexp.MustCompile(`^run-[` + kb.HashAlphabet + `]{` + strconv.Itoa(kb.HashLen) + `}$`)

// runKeys is the fields a setup refusal states before the KB is opened: the
// root this invocation found, the store it would use, and the pass it was
// asked for. The successful path states the same three around the migration
// record, which only a KB this run opened can carry.
func runKeys(fields []toolresult.Field, dir, pass string) []toolresult.Field {
	return append(slices.Clone(fields),
		toolresult.Field{Key: "state-dir", Value: dir}, toolresult.Field{Key: "pass", Value: pass})
}

// scoreStopped is the record a cancelled run closes on: the walk's first item
// that carries no attempt — the one the run stopped before — named as the
// document names items, and an empty record where every item of the walk
// carries one.
func scoreStopped(walked []query.Item, l *score.Ledger) toolresult.Record {
	for _, it := range walked {
		if l.Status(it) == "" {
			return toolresult.Record{{Key: "item", Value: score.ItemName(it)}}
		}
	}
	return toolresult.Record{}
}

// scoreFailures is what a failed run closes on: one item per item of the walk
// the ledger records failed, its name and the reason its line recorded in the
// detail, in the walk's order. The reason is the line's reply as a summary —
// bounded and stripped of control characters — where the line itself holds it
// whole.
func scoreFailures(walked []query.Item, l *score.Ledger) []toolresult.Item {
	out := []toolresult.Item{}
	for _, it := range walked {
		e, ok := l.Last(it)
		if !ok || e.Status != score.StatusFailed {
			continue
		}
		out = append(out, toolresult.Item{Check: checkError, Detail: score.ItemName(it) + ": " + score.ReplySummary(e.Reply)})
	}
	return out
}

// scoreRefresh is the run's refresh over src, its outcome and fields where it
// refuses or fails, and ok false where it wrote what it could. The refresh
// writes the KB, so it holds the KB's write lock; a lock it cannot take is the
// run's own answer, and the document's resume field carries the command that
// continues the run. A machine with no Graphviz dot draws no sheet, and says
// so.
func scoreRefresh(root string, src *kb.Source, fields []toolresult.Field, opts scoreOptions) (outcome string, out []toolresult.Field, stop bool) {
	release, outcome, items, err := holdKB(root, "re-run kbase score")
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

// scoreCounted is what a run reports of its items: the counts, and the items
// whose value landed, in the order it landed them.
func scoreCounted(res score.Result) []toolresult.Field {
	scored := make([]string, 0, len(res.Written))
	for _, it := range res.Written {
		scored = append(scored, score.ItemName(it))
	}
	return []toolresult.Field{
		{Key: "counts", Value: toolresult.Record{
			{Key: "total", Value: res.Counts.Total},
			{Key: "done", Value: res.Counts.Done},
			{Key: "failed", Value: res.Counts.Failed},
			{Key: "remaining", Value: res.Counts.Remaining}}},
		{Key: "scored", Value: scored},
	}
}

// scoreResume is the command that continues the run: the same pass over the
// same ledger, reaching the model this invocation was told to reach on the back
// end it was told to use. A scoring-time endpoint is named by --scoring-model,
// which states the provider and the model in one value; there is no second flag
// for half of it. A --scoring-model and a --harness the invocation carried are
// carried on, so the resumed run judges with the model and the command the
// first one did — one issued without its harness would be a different judgement
// wearing the same run id.
//
// The mode is part of the command too. An environment-mode run has no flag that
// names its source, so the command carries the mode's settings as assignments
// before the program: a shell with none of them set asks the same endpoint with
// the same models rather than falling to the files or refusing. They are
// printed from the values this invocation read, and no key material is among
// them — the key is a file, named by its path.
//
// --config-dir is carried where it was set: a resumed run reads that directory
// and no other, so without it the same run id could resolve another reference
// and ask a different judge under the same records.
func scoreResume(opts scoreOptions, run, dir string, mode config.Mode, env config.Overrides) string {
	command := []string{"kbase", "score", "--pass", opts.Pass, "--resume", run, "--state-dir", dir}
	if opts.ConfigDir != "" {
		command = append(command, "--config-dir", opts.ConfigDir)
	}
	if opts.ScoringModel != "" {
		command = append(command, "--scoring-model", opts.ScoringModel)
	}
	if opts.Harness != "" {
		command = append(command, "--harness", opts.Harness)
	}
	if mode == config.ModeEnvironment {
		command = append(environmentAssignments(env), command...)
	}
	return build.ShellJoin(command)
}

// environmentAssignments is the environment mode's settings as assignments: the
// endpoint and the role variables, and the key file where it was set.
func environmentAssignments(env config.Overrides) []string {
	var out []string
	for _, setting := range []struct{ name, value string }{
		{config.EnvAPIBaseURL, env.BaseURL},
		{config.EnvDecision, env.Decision},
		{config.EnvText, env.Text},
		{config.EnvScoring, env.Scoring},
		{config.EnvAPIKeyFile, env.APIKeyFile},
	} {
		if setting.value != "" {
			out = append(out, setting.name+"="+setting.value)
		}
	}
	return out
}

// scoreStamp is a time as the run's own records spell it: RFC 3339 in UTC.
func scoreStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// landScore lands one answer on one item through the write API — a node's rigor
// through set-rigor, an edge's bearing through set-bearing — and lands nothing
// else: the run refreshes once when its writes are in. The write holds the KB's
// write lock, as every writer of the KB does, and a lock it cannot take is the
// item's failure. A value the KB already holds is no failure; a refusal or a
// failure is the item's, and the run goes on.
func landScore(root string, it query.Item, value kb.Bearing, lg log.Logger) error {
	op, entry := "set-rigor", map[string]any{}
	switch {
	case it.Node != nil:
		entry["id"], entry["rigor"] = it.Node.ID, scoreValue(value)
	case it.Edge != nil:
		op = "set-bearing"
		entry["id"], entry["target"], entry["relation"] = it.Edge.Source, it.Edge.Target, it.Edge.Relation
		entry["bearing"] = scoreValue(value)
	default:
		return errors.New("the item names neither a node nor an edge")
	}
	values, err := json.Marshal(map[string]any{"entry": []map[string]any{entry}})
	if err != nil {
		return err
	}
	release, outcome, items, err := holdKB(root, write.RetryRemedy(op))
	if err != nil {
		return fmt.Errorf("the KB's write lock: %w", err)
	}
	if release == nil {
		if len(items) == 0 {
			return errors.New("the KB's write lock was not taken: the write ended " + outcome)
		}
		return toolresult.Items(items)
	}
	defer release()
	res := write.Run(op, write.Options{KBRoot: root, Values: values, NoRefresh: true, Logger: lg})
	switch res.Outcome {
	case toolresult.Done, toolresult.Unchanged:
		return nil
	}
	parts := make([]string, 0, len(res.Refusals)+len(res.Failures))
	for _, item := range res.Refusals {
		parts = append(parts, item.Detail)
	}
	for _, item := range res.Failures {
		parts = append(parts, item.Detail)
	}
	if len(parts) == 0 {
		parts = append(parts, "the write ended "+res.Outcome)
	}
	return errors.New(strings.Join(parts, "; "))
}

// scoreValue is a bearing as a write op reads it back: the number itself, or
// the pending literal. A number travels as one, never as its spelling.
func scoreValue(v kb.Bearing) any {
	if v.Pending {
		return kb.PendingLiteral
	}
	return v.Value
}

// refusalOrFailure is err as a verb's stop: a refusal's own items where err is
// one, a failure otherwise. stop is false where err is nil.
func refusalOrFailure(fields []toolresult.Field, err error) (outcome string, out []toolresult.Field, stop bool) {
	if err == nil {
		return "", nil, false
	}
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		outcome, out = refused(fields, refusal...)
		return outcome, out, true
	}
	outcome, out = failed(fields, err)
	return outcome, out, true
}

var (
	scoreFlagTimeout      time.Duration
	scoreFlagPass         string
	scoreFlagLimit        int
	scoreFlagDryRun       bool
	scoreFlagScoringModel string
	scoreFlagHarness      string
	scoreFlagResume       string
	scoreFlagStateDir     string
)

var scoreCmd = &cobra.Command{
	Use:   "score",
	Short: "ask the scoring model for the KB's rigor and bearing values, one item per call",
	Long: `score walks the KB's scoring set, asks the configured scoring model for the
faults one item's text commits, prices the charge lines it answers with, and
lands the value through the write API: set-rigor for a node, set-bearing for an
edge, never a register or a leaf edit of its own. One refresh opens the run — the
index the set is read from is current, and a KB in an older metadata format is
converted and saved by it — and one more follows the last write.

The scoring role is the KB's declaration of which model judges it: models.scoring
in config.toml, or KBASE_SCORING with the environment's endpoint. A run whose
mode names no scoring model is refused, naming that key, before the KB is
opened; nothing falls back to the text model. --scoring-model overrides a
configured role for one run — a <provider>/<model> reference in file mode, a
bare id under the environment's endpoint — and is carried by the command that
continues a run. --dry-run lists the set, resolves no role and refuses nothing
for one.

--harness NAME runs one process per item instead of asking an endpoint. kbase
composes the command, and ` + "`opencode`" + ` is the only name it takes: the
built-in opencode back end, run as one opencode v2 process per item with the
prompt on its stdin, its stdout the reply and its stderr kept for a failure's
reason. No endpoint is chosen and the mode is not consulted, so a base URL and
role variables may be set beside it. Any other name is refused before the KB is
opened: kbase runs no command a user supplies. --dry-run asks nothing, so it
makes no difference there.

The passes are rigor — every claim, analysis and experiment, by id — and
bearing — every scoring edge, by relation, then source, then target. A work's
rigor is the one value neither pass asks: it is the work's own standing, judged
by hand with the document in hand, and set-rigor is what writes it. An axiom's
is 1.0 by rule.

Every attempt is recorded under <state-dir>/score/<run>/: run.yaml names the
run and the digest of the ask its judgements answer, ledger.jsonl holds a line
per attempt with the charges a reply recorded, the reply itself where it parsed,
the value it prices to, the model and the prompt's digest, and items.jsonl lists
the set. --dry-run writes those records of the set, asks nothing, writes no KB
and cannot be resumed. --resume continues a run, skipping every item it already
landed and re-asking the ones it did not; a fresh run asks everything again.
--limit stops after that many items, and the result carries the command that
continues the run.

A run holds <state-dir>/score for its length, so a second run is turned away
with retry. SIGINT or SIGTERM stops it at an item boundary. Its result is one
YAML document on stdout.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := invocationDir(cmd)
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
		code, err := runScore(scoreOptions{providerOptions: opts, Ctx: ctx, Pass: scoreFlagPass, Limit: scoreFlagLimit,
			DryRun: scoreFlagDryRun, ScoringModel: scoreFlagScoringModel, Harness: scoreFlagHarness,
			Resume: scoreFlagResume, StateDir: scoreFlagStateDir, ConfigDir: flagConfigDir, WorkDir: wd,
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
		"the pass to walk (required): "+query.PassRigor+" for a node's own value, "+query.PassBearing+" for an edge's")
	scoreCmd.Flags().IntVar(&scoreFlagLimit, "limit", 0, "stop after this many items (default: every item)")
	scoreCmd.Flags().BoolVar(&scoreFlagDryRun, "dry-run", false, "list the set and ask nothing")
	scoreCmd.Flags().StringVar(&scoreFlagScoringModel, "scoring-model", "",
		"model that judges the KB, overriding models.scoring: <provider>/<model> in file mode, a bare id under the environment's endpoint, the role's value whole under --harness")
	scoreCmd.Flags().StringVar(&scoreFlagHarness, "harness", "",
		"back end that asks each item, one process per item: "+opencodeHarness+" (no difference under --dry-run, which asks nothing)")
	scoreCmd.Flags().StringVar(&scoreFlagResume, "resume", "", "continue the run with this id")
	scoreCmd.Flags().StringVar(&scoreFlagStateDir, "state-dir", "", "the state store (default: $XDG_STATE_HOME/kbase/<key>)")
	rootCmd.AddCommand(mcpBinding(scoreCmd, mcpExcluded))
}
