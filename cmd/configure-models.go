package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

// configureModelsOptions is the resolved input of configure-models: the shared
// provider-reaching options, each role's reference as given ("" for a role
// not given), where it is written, and the stream its result goes to.
type configureModelsOptions struct {
	providerOptions

	Models map[config.Role]string
	// Layer is the --layer flag, "" where it names none.
	Layer string

	// WorkDir is where the repository search started; the worktree refusal
	// names it.
	WorkDir string

	Stdout io.Writer
	Logger log.Logger
}

func runConfigureModels(ctx context.Context, opts configureModelsOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "configure-models"); err != nil {
		return 0, err
	}
	outcome, fields := configureModels(ctx, opts)
	return emitResult("configure-models", opts.Stdout, outcome, fields)
}

// configureModels writes the references given, each checked against its
// provider's catalogue under its own --timeout deadline, and against the pool
// of the stack it writes into: the layer written and the layers beneath it.
func configureModels(ctx context.Context, opts configureModelsOptions) (string, []toolresult.Field) {
	dir, layer, items, err := configureLocation(opts.providerOptions, opts.Layer, opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if items != nil {
		return refused(nil, items...)
	}
	stack, err := loadWrittenStack(dir, layer)
	if err != nil {
		return failed(nil, err)
	}
	faults := warnFaults(opts.Stderr, stack.Faults)
	reg := modelclient.New(opts.NewClient, opts.Logger, "", 0)
	defer reg.CloseAll()
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	run, err := config.ConfigureModels(stack.inputs(), opts.Layer, opts.Models, func(entry config.Provider) ([]string, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return reg.Catalogue(callCtx, entry)
	})
	warned := faults
	for _, miss := range run.Misses {
		detail := fmt.Sprintf("the %s role's model %q is not in provider %q's catalogue",
			miss.Role, miss.Reference.Model, miss.Reference.Provider)
		fmt.Fprintf(opts.Stderr, "configure-models: %s\n", detail)
		warned = append(warned, toolresult.Item{Key: argmark.Flag(string(miss.Role)), Detail: detail})
	}
	warnings := warningsField(warned)
	if err != nil {
		outcome, fields := refusedOrFailed(nil, err)
		return outcome, toolresult.WithWarnings(fields, warnings)
	}
	fields := []toolresult.Field{
		{Key: "models", Value: toolresult.Record{
			{Key: string(config.RoleDecision), Value: stringOrNull(run.Models.Decision)},
			{Key: string(config.RoleText), Value: stringOrNull(run.Models.Text)},
			{Key: string(config.RoleScoring), Value: stringOrNull(run.Models.Scoring)}}},
		{Key: "written", Value: nonNil(run.Written)},
	}
	outcome := toolresult.Done
	if len(run.Written) == 0 {
		outcome = toolresult.Unchanged
	}
	return outcome, toolresult.WithWarnings(fields, warnings)
}

var (
	configureModelsFlagDecision string
	configureModelsFlagText     string
	configureModelsFlagScoring  string
	configureModelsFlagTimeout  time.Duration
	configureModelsFlagLayer    string
)

func configureModelsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure-models",
		Short: "write a model reference into config.toml for each role given",
		Long: `Assign a model reference, <provider>/<model>, to each role given — ` + argmark.Flag(string(config.RoleDecision)) + `,
` + argmark.Flag(string(config.RoleText)) + ` and ` + argmark.Flag(string(config.RoleScoring)) + `, at least one — in config.toml's [models] table,
creating the file where it is absent. The roles not given stand as they are:
the lines of the roles given are rewritten, and every other byte of the file is
kept.

A reference names a provider, then the model it serves, split at the first /.
Its provider part is folded to lowercase and matches a provider whatever its
case. A value that is not a reference is refused, and so is a provider that no
providers.toml this call reads defines: the one in the directory it writes
into and, under ` + argmark.FlagValue("layer", config.LayerSourceRepository) + `, the global one (check: provider; path: the first of
these; remedy: ` + argmark.Call("configure-providers", map[string]any{"provider": "<provider>", "base-url": "<url>"}) + ` with this call's
` + argmark.Flag("layer") + `). A refusal writes nothing. Each provider named is
dialled once, ` + argmark.Flag("timeout") + ` bounding the request, and each reference is checked
against its catalogue: a model missing from its catalogue is a warning, not a
failure, since catalogues rotate. A catalogue that cannot be listed fails the
command, and nothing is written.

` + configureLocationHelp + ` The result gives "models", the decision, text and
scoring references the written config.toml holds (null for a role it does not
set); "written", the files written; and "warnings": one item per configuration
fault in the files this call reads, as ` + argmark.Call("models", nil) + ` reports them, "path" the file or
directory it is against, "detail" what is wrong; then one per model missing
from its catalogue, "key" the role (` + argmark.Flag(string(config.RoleDecision)) + `, ` + argmark.Flag(string(config.RoleText)) + ` or ` + argmark.Flag(string(config.RoleScoring)) + `), "detail" the
reference missing.
"warnings" is empty where there are none. It is carried on every outcome once
the configuration has loaded, refusals and failures included; a refusal of
` + argmark.Flag("layer") + `, or a configuration that does not load, carries none. A run
that writes nothing is unchanged.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			opts, err := resolveVerbLayers(wd)
			if err != nil {
				return err
			}
			opts.Timeout = configureModelsFlagTimeout
			code, err := runConfigureModels(cmd.Context(), configureModelsOptions{
				providerOptions: opts,
				Models: map[config.Role]string{config.RoleDecision: configureModelsFlagDecision,
					config.RoleText: configureModelsFlagText, config.RoleScoring: configureModelsFlagScoring},
				Layer:   configureModelsFlagLayer,
				WorkDir: wd,
				Stdout:  cmd.OutOrStdout(),
				Logger:  processLog.logger,
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
	for _, f := range []struct {
		value *string
		role  config.Role
	}{
		{&configureModelsFlagDecision, config.RoleDecision},
		{&configureModelsFlagText, config.RoleText},
		{&configureModelsFlagScoring, config.RoleScoring},
	} {
		cmd.Flags().StringVar(f.value, string(f.role), "", "the "+string(f.role)+" role's model reference, <provider>/<model>")
		mcp.Brief(cmd, string(f.role), string(f.role)+" role's model, provider/model")
	}
	registerTimeoutFlag(cmd, &configureModelsFlagTimeout, catalogueTimeoutUsage, catalogueTimeoutBrief)
	registerLayerFlag(cmd, &configureModelsFlagLayer)
	return mcp.Notes(cmd, configureMCPNote, "")
}
