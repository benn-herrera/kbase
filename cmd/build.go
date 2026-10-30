package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/log"
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

// runBuild runs the build, writes its result document, and returns the exit
// code the outcome maps to.
func runBuild(ctx context.Context, opts buildOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "build"); err != nil {
		return 0, err
	}
	outcome, fields := buildRun(ctx, opts)
	if err := toolresult.Emit(opts.Stdout, outcome, fields...); err != nil {
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

// refused is a refusal of every item, after fields.
func refused(fields []toolresult.Field, items ...toolresult.Item) (string, []toolresult.Field) {
	return toolresult.Refused, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: items})
}

// checkError names a failure kbase has no narrower class for.
const checkError = buildstore.CheckError

// failed is a failure of err, after fields.
func failed(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	return toolresult.Failed, append(fields, toolresult.Field{Key: toolresult.FailuresKey,
		Value: []toolresult.Item{{Check: checkError, Detail: err.Error()}}})
}

// exitCode carries a non-zero exit out of a verb that has already written
// its result document.
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
// it lacks. A key bound for a non-loopback http:// endpoint is said on stderr,
// and the build proceeds.
func buildProvider(p providerOptions, env config.Overrides, stderr io.Writer) func(reg *modelclient.Registry) (build.Provider, error) {
	return func(reg *modelclient.Registry) (build.Provider, error) {
		s := p.inputs(env)
		mode, err := config.ModeOf(s)
		if err != nil {
			return build.Provider{}, err
		}
		if mode == config.ModeFile {
			// An environment-mode invocation reads no file value, so it says
			// nothing of the keys the files state.
			supersededProvider(stderr, p.Config)
			supersededReaderConcurrency(stderr, p.Config)
		}
		warnFaults(stderr, p.Faults)
		var records []toolresult.Record
		warned := map[string]bool{}
		role := func(r config.Role) (model.Client, string, error) {
			ep, err := config.Resolve(s, mode, r, "")
			if err != nil {
				return nil, "", err
			}
			// One line per endpoint, however many roles reach it: the key is
			// what the run sends, and the registry is what read it.
			if !warned[ep.BaseURL] {
				warned[ep.BaseURL] = warnCleartextKey(stderr, reg, ep)
			}
			client, err := reg.Get(ep)
			if err != nil {
				return nil, "", err
			}
			records = append(records, roleRecord(ep))
			return client, ep.Model, nil
		}
		var provider build.Provider
		if provider.MultichoiceClient, provider.Multichoice, err = role(config.RoleDecision); err != nil {
			return build.Provider{}, err
		}
		if provider.OverviewClient, provider.Overview, err = role(config.RoleText); err != nil {
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

var buildCmd = &cobra.Command{
	Use:   "build [<volume-root>...]",
	Short: "build the KB from LaTeX volume roots into <git root>/kb-root",
	Long: `build converts LaTeX volume roots — each a paper's top .tex file, never one
another includes — into a KB, at kb-root/ beside the repository's .git: one
tree, each volume its own directory, entry-point.md listing every volume in
path order. A relative volume root or --bibliography resolves against the
project root, and the build orders each list by full path, whatever order it
was given in. It walks the build's stages in order through the
stage --through names (an id or a display name; every stage when absent).
Each stage's boundary is a commit in the repository, its subject
"kb-build: <stage> | <display>", scoped to the paths the build owns, and
records the inputs; the start boundary also records started-at, the commit the
build began from. A build is given volume roots only over an empty trail;
--resume takes none and continues from the last boundary the commit trail
holds, with the inputs it records. A build resumes from any recorded boundary,
not only the last: status names each stage's commit, so reset the repository
to one and run build --resume; a build over other inputs starts over from
started-at. The rewind-build MCP tool and the kbase://build-trail resource
print the commands for either.
--no-inference drops every row that spends inference and walks the rest;
without it, the claim graph's multichoice asks go to the endpoint [models] decision
names and the overview passage to the one [models] text names — each value a
provider/model reference, the providers.toml entry that supplies the endpoint
then the model it serves. KBASE_API_BASE_URL, KBASE_DECISION and KBASE_TEXT
configure the build from the environment with no configuration directory at
all: the environment supplies the whole inference configuration or nothing, so
a variable set beside a config.toml model is refused, and KBASE_API_KEY_FILE —
no mode, a key — replaces the key of whichever endpoint is in force. With no
--bibliography,
every .bib beside any volume root is offered to every volume, in sorted order.
Build state — the holder lock, progress.jsonl, the document graph's records, each
stage's report under reports/, each call's capture and the answer cache under
scratch/ — lives in --state-dir, by default $XDG_STATE_HOME/kbase/<key>.
SIGINT or SIGTERM (kbase cancel) stops it resumably. Its result is one YAML
document on stdout, naming each stage walked, its boundary commit and its
report, and the command that resumes the build where it did not finish.`,
	Args: cobra.ArbitraryArgs,
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
			Provider:              buildProvider(providers, config.EnvOverrides(os.Getenv), cmd.ErrOrStderr()),
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
}

func init() {
	buildCmd.Flags().StringArrayVar(&flagBibliographies, "bibliography", nil,
		"a .bib citations resolve against; repeat per file, the files read in path order (default: every .bib beside any volume root, sorted)")
	buildCmd.Flags().BoolVar(&flagResume, "resume", false, "continue the build the commit trail records, with the inputs it records")
	buildCmd.Flags().StringVar(&flagThrough, "through", "", "the last stage to walk, by id or display name (default: every stage)")
	buildCmd.Flags().StringVar(&flagStateDir, "state-dir", "", "the build's state store (default: $XDG_STATE_HOME/kbase/<key>)")
	buildCmd.Flags().BoolVar(&flagNoInference, "no-inference", false, "drop every row that spends inference and walk the rest")
	registerAskConcurrencyFlag(buildCmd, &flagAskConcurrency)
}
