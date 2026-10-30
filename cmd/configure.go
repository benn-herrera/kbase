package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

// configureOptions is the resolved input of the configure verb: the shared
// provider-reaching options plus this verb's own flags and the stream its
// result goes to.
//
// The write target is the embedded WriteDir — the layer this invocation
// resolved, which --project may replace with the repository's. The embedded
// Config's [models] table is deliberately not consulted on the dial path:
// re-running configure is how a stale model map is replaced. The one value a
// dialed run reads from the target's file is the scoring reference it may
// leave standing, and it is read from that file rather than from the merge.
type configureOptions struct {
	providerOptions

	// ModelMap holds the raw --model-map values, one per occurrence of
	// the flag, each possibly comma-separated.
	ModelMap []string

	// Create creates the target layer's providers.toml and config.toml
	// where they are absent, and writes only the values the flags name.
	Create bool

	// Project names <repository>/.kbase as the write target, whether or not
	// it exists.
	Project bool

	// WorkDir is where the repository search started; the worktree refusal
	// names it.
	WorkDir string

	Stdout io.Writer

	// Logger takes the registry's close failures.
	Logger log.Logger
}

// runConfigure runs config.Configure into the target layer, each catalogue it
// reads dialled under its own --timeout deadline, writes the run's stderr
// lines and its result document, and returns the exit code the outcome maps
// to.
func runConfigure(ctx context.Context, opts configureOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "configure"); err != nil {
		return 0, err
	}
	outcome, fields := configure(ctx, opts)
	return emitResult("configure", opts.Stdout, outcome, fields)
}

func configure(ctx context.Context, opts configureOptions) (string, []toolresult.Field) {
	// Flag syntax is checked before anything is dialed: a typo in
	// --model-map should not cost a network round trip to discover.
	manual, err := config.ParseModelMap(opts.ModelMap)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	target, items, err := configureTarget(opts)
	if err != nil {
		return failed(nil, err)
	}
	if items != nil {
		return refused(nil, items...)
	}
	// The target layer is what the shared provider refusals name, so a
	// missing pool entry points at the file this run would write.
	opts.WriteDir = target

	reg := modelclient.New(opts.NewClient, opts.Logger, "", 0)
	defer reg.CloseAll()
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	req := config.ConfigureRequest{
		Inputs:   opts.inputs(),
		Provider: opts.Provider,
		Models:   manual,
		Create:   opts.Create,
	}
	offline := req.Offline()
	if !offline {
		warnFaults(opts.Stderr, opts.Faults)
	}
	run, err := config.Configure(req, func(entry config.Provider) ([]string, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return reg.Catalogue(callCtx, entry)
	})

	for _, miss := range run.Misses {
		fmt.Fprintf(opts.Stderr,
			"configure: %s category model %q is not in provider %q's catalogue — configuring it anyway\n",
			miss.Category, miss.Reference.Model, miss.Reference.Provider)
	}
	if offline && run.Models != nil {
		fmt.Fprintf(opts.Stderr, "configure: no provider named, so the references were not checked\n")
	}

	// The provider field stands from the selection on, and on the offline
	// path from the write, where it states that no entry was configured.
	var fields []toolresult.Field
	if run.Selected || run.Models != nil {
		fields = configureProviderFields(run.Provider, run.Rule, run.Layer)
	}
	if run.Models != nil {
		fields = append(fields,
			toolresult.Field{Key: "models", Value: toolresult.Record{
				{Key: config.CategoryDecision, Value: stringOrNull(run.Models.Decision)},
				{Key: config.CategoryText, Value: stringOrNull(run.Models.Text)},
				{Key: config.CategoryScoring, Value: stringOrNull(run.Models.Scoring)}}},
			toolresult.Field{Key: "written", Value: nonNil(run.Written)})
	}
	if err != nil {
		return refusedOrFailed(fields, err)
	}
	if len(run.Written) == 0 {
		return toolresult.Unchanged, fields
	}
	// Where the references were checked against a catalogue, a human reads
	// this line instead of the document.
	if !offline {
		fmt.Fprintf(opts.Stderr, "configured: %s=%s %s=%s %s=%s (wrote %s)\n",
			config.CategoryDecision, run.Models.Decision,
			config.CategoryText, run.Models.Text,
			config.CategoryScoring, run.Models.Scoring,
			strings.Join(run.Written, ", "))
	}
	return toolresult.Done, fields
}

// configureTarget resolves the directory this run writes into: the layer the
// invocation resolved, unless --project names the repository's — which is the
// target whether or not it stands. --project outside a worktree is refused,
// and so is --project beside --config-dir: the two name different
// directories, and honouring one would silently discard the other.
func configureTarget(opts configureOptions) (string, []toolresult.Item, error) {
	if !opts.Project {
		return opts.WriteDir, nil, nil
	}
	if len(opts.Layers) > 0 && opts.Layers[0].Source == config.LayerSourceExplicit {
		return "", []toolresult.Item{{Check: checkUsage, Key: "--project",
			Detail: "--project and --config-dir name different configuration directories; pass one"}}, nil
	}
	if opts.RepoRoot == "" {
		return "", []toolresult.Item{{Check: checkWorktree, Path: opts.WorkDir, Remedy: "git init",
			Detail: fmt.Sprintf("%s is not inside a git worktree; `git init` there is the operator's to run", opts.WorkDir)}}, nil
	}
	return config.WriteTarget(opts.Layers, opts.RepoRoot), nil, nil
}

// configureProviderFields is the `provider` field a configure run states: the
// entry it configured, with the rule and layer that named it, or null where
// the configuration names none — rule and layer absent then, as SPEC §7 has
// them.
func configureProviderFields(name, rule, layer string) []toolresult.Field {
	if fields := providerFields(name, rule, layer); fields != nil {
		return fields
	}
	return []toolresult.Field{{Key: "provider", Value: nil}}
}

var (
	configureFlagProvider string
	configureFlagModelMap []string
	configureFlagTimeout  time.Duration
	configureFlagCreate   bool
	configureFlagProject  bool
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "assign a model reference to each category with --model-map and write config.toml, creating it with --create",
	Long: `Assign a model reference to each category — decision, text and scoring —
with --model-map, and write them to config.toml.

A value is a reference, <entry>/<id>: the providers.toml entry that supplies
the endpoint, then the model it serves. The entry is resolved as for
` + "`kbase models`" + ` — --provider, else the sole entry in providers.toml — and that
catalogue is what a category the flag does not name is offered. A reference
naming another entry is dialled and checked against that entry's own
catalogue. With --create and no --provider there is nothing to dial: no
request is made, no reference is checked against anything, and one line on
stderr says so. --timeout bounds one catalogue request, and is accepted and
unused when none is made.

On every path that dials, every category must have a reference to write.
A category the flag does not name is refused, naming models.<category> and
listing the models the provider offers as the choices, and nothing is written
— except scoring, which a run leaves standing where the target config.toml
already holds one. A model named in --model-map but missing from its entry's
catalogue is a warning, not a failure — provider catalogues rotate.

--create creates the target layer's providers.toml and config.toml where they
are absent, from an annotated template, and overwrites none: config.toml is
edited in place — the [models] decision, text and scoring keys are rewritten,
every other byte kept, and a file already holding the values is left alone
(outcome unchanged). A refused or failed run writes nothing.

The target layer is the repository a --project flag names — <repository>/` + config.RepositoryDirName + `,
whether or not it exists, refused outside a git worktree — else the directory
--config-dir or KBASE_CONFIG_DIR names, else <repository>/` + config.RepositoryDirName + ` where that
directory already stands, else ~/.config/kbase. --config-dir with --project is
refused: they name different directories. Its result is one YAML document on
stdout.`,
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
		opts.Provider = configureFlagProvider
		opts.Timeout = configureFlagTimeout
		code, err := runConfigure(cmd.Context(), configureOptions{
			providerOptions: opts,
			ModelMap:        configureFlagModelMap,
			Create:          configureFlagCreate,
			Project:         configureFlagProject,
			WorkDir:         wd,
			Stdout:          cmd.OutOrStdout(),
			Logger:          processLog.logger,
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
	registerProviderFlags(configureCmd, &configureFlagProvider, &configureFlagTimeout, "configure")
	configureCmd.Flags().BoolVar(&configureFlagCreate, "create", false,
		"create the target layer's "+config.ProvidersFileName+" and "+config.ConfigFileName+" where they are absent")
	configureCmd.Flags().BoolVar(&configureFlagProject, "project", false,
		"write into <repository>/"+config.RepositoryDirName+", whether or not it exists (refused outside a git worktree)")
	configureCmd.Flags().StringArrayVar(&configureFlagModelMap, "model-map", nil,
		"explicit category assignment, e.g. "+config.CategoryDecision+"=<entry>/<id>"+config.ModelMapPairSep+config.CategoryText+"=<entry>/<id> (repeatable; every category needs one on a path that dials)")
}
