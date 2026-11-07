package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/model"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

type buildOptions struct {
	VolumeRoots    []string
	Bibliographies []string
	// Resume is the --resume flag: continue the trail with the inputs it
	// records.
	Resume      bool
	Through     string
	StateDir    string
	NoInference bool
	// ConfigDir is the --config-dir flag made absolute, "" where it was not
	// given.
	ConfigDir string
	// ProjectDir is the --project-dir flag made absolute, "" where it was not
	// given.
	ProjectDir string
	// AskConcurrency is the --ask-concurrency flag, nil where it was not
	// given; ConfiguredConcurrency is [asks] concurrency as the loaded
	// configuration states it, nil where it states none.
	AskConcurrency, ConfiguredConcurrency *int
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	// Faults are the loaded configuration's faults, the result's warnings.
	Faults []config.ProviderFault
	// Provider builds where the build's model calls go, or what the
	// configuration lacks for them, over the registry the verb opened and
	// closes when the build returns.
	Provider func(reg *modelclient.Registry) (build.Provider, error)
	// NewClient is the client seam the registry opens its clients through;
	// nil means model.NewClient.
	NewClient modelclient.NewClientFunc
	Stdout    io.Writer
	Stderr    io.Writer
	Logger    log.Logger
}

// runBuild runs the build, writes its result, and returns the exit
// code the outcome maps to. The configuration's faults are the result's
// warnings and lines on Stderr.
func runBuild(ctx context.Context, opts buildOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "build"); err != nil {
		return 0, err
	}
	warnings := warningsField(warnFaults(opts.Stderr, opts.Faults))
	outcome, fields := buildRun(ctx, opts)
	if err := toolresult.Emit(resultOut(opts.Stdout), outcome, toolresult.WithWarnings(fields, warnings)...); err != nil {
		return 0, fmt.Errorf("build: writing the result: %w", err)
	}
	return toolresult.ExitCode(outcome), nil
}

// buildRun is the verb's body: the build's one queue of the invocation's
// model calls in flight, the registry its clients are opened through, and the
// walk.
func buildRun(ctx context.Context, opts buildOptions) (string, []toolresult.Field) {
	reg, provider, err := buildModels(opts)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	defer reg.CloseAll()
	named := 0
	if opts.AskConcurrency != nil {
		named = *opts.AskConcurrency
	}
	return build.Run(ctx, build.Options{
		VolumeRoots: opts.VolumeRoots, Bibliographies: opts.Bibliographies, Resume: opts.Resume, Through: opts.Through,
		StateDir: opts.StateDir, NoInference: opts.NoInference, ConfigDir: opts.ConfigDir, ProjectDir: opts.ProjectDir,
		AskConcurrency: named,
		WorkDir:        opts.WorkDir, Provider: provider, Logger: opts.Logger,
	})
}

// buildModels is where an invocation's build or replay sends its model calls:
// the registry its clients are opened through, which the caller closes, and
// the Provider seam over it, every role's calls on one queue as wide as the
// ask concurrency.
func buildModels(opts buildOptions) (*modelclient.Registry, func() (build.Provider, error), error) {
	concurrency, err := askConcurrency(opts.AskConcurrency, opts.ConfiguredConcurrency)
	if err != nil {
		return nil, nil, err
	}
	reg := modelclient.New(opts.NewClient, opts.Logger, "", concurrency)
	return reg, providerFor(opts, reg, model.NewQueue(concurrency)), nil
}

// providerFor is the build's Provider seam over the registry the verb opened,
// every role's calls on queue. A run that names no Provider — a caller with
// nothing to hand it — resolves none, and the walk's launch refuses a model
// call it cannot make.
func providerFor(opts buildOptions, reg *modelclient.Registry, queue *model.Queue) func() (build.Provider, error) {
	if opts.Provider == nil {
		return nil
	}
	return func() (build.Provider, error) {
		p, err := opts.Provider(reg)
		p.Queue = queue
		return p, err
	}
}

// refused is a refusal of every item, after fields. A configuration or
// provider item leaves here with show-config in its remedy, after the
// clearing text the item carries; show-config reports its own without it.
func refused(fields []toolresult.Field, items ...toolresult.Item) (string, []toolresult.Field) {
	out := make([]toolresult.Item, len(items))
	for i, it := range items {
		if it.Check == config.CheckProvider {
			it.Remedy = config.ShowConfigRemedy(it.Remedy)
		}
		out[i] = it
	}
	return toolresult.Refused, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: out})
}

// checkError names a failure kbase has no narrower class for.
const checkError = buildstore.CheckError

// failed is a failure of err, after fields, carrying the remedy err names
// where the provider refused a call for its key.
func failed(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	item := toolresult.Item{Check: checkError, Detail: err.Error()}
	var refused modelclient.KeyRefusedError
	if errors.As(err, &refused) {
		item.Remedy = refused.Remedy
		item.Detail = config.WithRefusedKeyNote(item.Detail)
	}
	return toolresult.Failed, append(fields, toolresult.Field{Key: toolresult.FailuresKey, Value: []toolresult.Item{item}})
}

// exitCode carries a non-zero exit out of a verb that has already written
// its result.
type exitCode int

func (c exitCode) Error() string { return fmt.Sprintf("exit %d", int(c)) }

var (
	flagBibliographies []string
	flagResume         bool
	flagThrough        string
	flagStateDir       string
	flagNoInference    bool
	flagAskConcurrency int
)

// buildProvider is where the build's model calls go: each role resolved from
// the loaded configuration, its own entry dialled through reg — the multichoice
// asks on the decision role, the overview passage on the text role — or what
// it lacks. A key bound for an http:// endpoint off the local network is said on stderr,
// and the build proceeds.
func buildProvider(p providerOptions, stderr io.Writer) func(reg *modelclient.Registry) (build.Provider, error) {
	return func(reg *modelclient.Registry) (build.Provider, error) {
		s := p.inputs()
		var records []toolresult.Record
		warned := map[string]bool{}
		role := func(r config.Role) (model.Client, string, string, error) {
			ep, err := config.Resolve(s, r, "")
			if err != nil {
				return nil, "", "", err
			}
			// One line per endpoint, however many roles reach it.
			if !warned[ep.BaseURL] {
				warned[ep.BaseURL] = warnCleartextKey(stderr, ep)
			}
			records = append(records, roleRecord(ep))
			return reg.Get(ep), ep.Provider, ep.Model, nil
		}
		var (
			provider build.Provider
			err      error
		)
		if provider.MultichoiceClient, provider.MultichoiceProvider, provider.Multichoice, err = role(config.RoleDecision); err != nil {
			return build.Provider{}, err
		}
		if provider.OverviewClient, provider.OverviewProvider, provider.Overview, err = role(config.RoleText); err != nil {
			return build.Provider{}, err
		}
		provider.Roles = records
		return provider, nil
	}
}

// buildReplay is the build's development replay entry: given the invoked
// command and its positionals, the verb its flags select in place of a build,
// or nil where they select none. It is nil in a build that carries no replay.
var buildReplay func(cmd *cobra.Command, args []string) func(context.Context, buildOptions) (int, error)

// buildLong is build's shared help body.
var buildLong = `build converts LaTeX volume roots — each a paper's top .tex file, never one
another includes — into a KB at kb-root/, in the git worktree root: one
tree, each volume its own directory, entry-point.md listing every volume in
path order. A volume root is the file 00README.json's toplevel entry names,
else the sole file containing \documentclass. A relative ` + argmark.Positional("volume-root") + ` or
` + argmark.Flag("bibliography") + ` resolves against the project root, and the build orders each
list by full path, whatever order it was given in. With no ` + argmark.Flag("bibliography") + `,
every .bib beside any volume root is offered to every volume, in sorted order.
Where two nodes of one kind would share a title, the build suffixes the later
" (2)", " (3)" and so on: every title is unique within its kind, as every
write requires.

It walks the build's stages in order through the stage ` + argmark.Flag("through") + ` names.
Each stage's boundary is a commit, its subject "kb-build: <stage> | <display>",
scoped to kb-root/, the build's records in kb-root/.build/ included; the
commit records the inputs, and the start boundary also records started-at, the
commit the build began from. The trail is the kb-build: commits on the current branch.

A fresh build takes volume roots and is refused where kb-root/ exists in any
state, a skeleton-only one and one an interrupted build left included.
` + argmark.Flag("resume") + ` takes no volume roots and no ` + argmark.Flag("bibliography") + `, and continues the trail with
the inputs it records. It is refused where kb-root/ does not exist (run a
fresh build) and where kb-root/ stands with nothing to resume; removing
kb-root/ is how to start over: kbase does not inspect half-built state. ` + argmark.Call("build-trail", nil) + ` lists each
boundary's commit with the command that rewinds to it — a reset, then a resume
— and started-at's, the reset alone. A finished ` + argmark.Flag("no-inference") + ` build never
runs its dropped steps — a step is a part of a stage's work — and a resume
over it changes nothing: to build with inference, run started-at's command
from ` + argmark.Call("build-trail", nil) + `, then a fresh build.

` + argmark.Flag("no-inference") + ` drops each step that would spend inference; every stage
is still walked and committed. Without it, the claim graph's multichoice
asks go to the endpoint [models] decision names and the overview passage to
the one [models] text names — each value a provider/model reference, the
provider that supplies the endpoint then the model it serves — and a build
missing either is refused.

Build state lives in ` + argmark.Flag("state-dir") + `, by default ` + stateStoreDefault + `.
One repository carries one run at a time, a build or a score run: a second is
refused, naming the holder's verb and pid. A run's own result names each stage
walked, its boundary commit and its report, and the call that resumes the
build where it did not finish.

` + configWarningsHelp("the run's own result")

// runLaunchMCPNote is what differs for build and score over MCP.
const runLaunchMCPNote = `The call returns once the run has started, as a detached process that carries
on without the server. Its result gives "pid", "state-dir" and "wait", a shell
command line that blocks until the run ends; no tool waits. Its "warnings" is
the configuration as read at launch; the run's own result carries the run's
own. The status tool follows the run and the cancel tool stops it.`

// buildMCPNote is what differs for build over MCP.
var buildMCPNote = runLaunchMCPNote + `

` + argmark.Flag("through") + ` takes a stage id alone; the command line also takes a stage's display name.`

// buildCLINote is what differs for build at the command line.
const buildCLINote = `build runs in the foreground, and its result is the run's own; SIGINT or
SIGTERM (kbase cancel) stops it resumably.`

var buildCmd = mcp.Notes(&cobra.Command{
	Use:   "build [<volume-root>...]",
	Short: "build the KB from LaTeX volume roots into kb-root/ at the git worktree root",
	Long:  buildLong,
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := invocationDir(cmd)
		if err != nil {
			return err
		}
		// The configuration directory is where the flag was typed, the one
		// config.Layers reads, so the resume command names it absolute.
		configDir, err := absIfSet(flagConfigDir)
		if err != nil {
			return err
		}
		projectDir, err := absIfSet(flagProjectDir)
		if err != nil {
			return err
		}
		providers, err := loadVerbContext(wd)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		verb := runBuild
		if buildReplay != nil {
			if replay := buildReplay(cmd, args); replay != nil {
				verb = replay
			}
		}
		code, err := verb(ctx, buildOptions{
			VolumeRoots:           args,
			Bibliographies:        flagBibliographies,
			Resume:                flagResume,
			Through:               flagThrough,
			StateDir:              flagStateDir,
			NoInference:           flagNoInference,
			ConfigDir:             configDir,
			ProjectDir:            projectDir,
			AskConcurrency:        givenAskConcurrency(cmd, flagAskConcurrency),
			ConfiguredConcurrency: providers.Config.Asks.Concurrency,
			WorkDir:               wd,
			Faults:                providers.Faults,
			Provider:              buildProvider(providers, cmd.ErrOrStderr()),
			Stdout:                cmd.OutOrStdout(),
			Stderr:                cmd.ErrOrStderr(),
			Logger:                processLog.logger,
		})
		if err != nil {
			return err
		}
		if code != 0 {
			return exitCode(code)
		}
		return nil
	},
}, buildMCPNote, buildCLINote)

func init() {
	buildCmd.Flags().StringArrayVar(&flagBibliographies, "bibliography", nil,
		"a .bib citations resolve against, one per file, read in path order (default: every .bib beside any volume root, sorted)")
	mcp.Brief(buildCmd, "bibliography", ".bib files citations resolve against")
	buildCmd.Flags().BoolVar(&flagResume, "resume", false, "continue the build the commit trail records, with the inputs it records")
	mcp.Brief(buildCmd, "resume", "continue the recorded build; omit "+argmark.Positional("volume-root"))
	buildCmd.Flags().StringVar(&flagThrough, "through", "", "the last stage to walk, by id — "+
		proseSeries(buildstore.StageIDs(), "or")+" — or by display name (default: every stage)")
	mcp.EnumFlag(buildCmd.Flags().Lookup("through"), buildstore.StageIDs()...)
	mcp.Brief(buildCmd, "through", "last stage to run")
	registerStateDirFlag(buildCmd, &flagStateDir, runStateDirUsage, stateDirBrief)
	buildCmd.Flags().BoolVar(&flagNoInference, "no-inference", false, "skip each step that would spend inference; every stage is still walked")
	mcp.Brief(buildCmd, "no-inference", "skip every inference step")
	registerAskConcurrencyFlag(buildCmd, &flagAskConcurrency)
	mcp.DescribePositional(buildCmd, "volume-root", "each paper's top .tex file, a relative path resolving against the project root; required unless "+
		argmark.Flag("resume")+", refused with it")
	mcp.Brief(buildCmd, "volume-root", "each paper's top .tex, project-relative; omit with "+argmark.Flag("resume"))
}
