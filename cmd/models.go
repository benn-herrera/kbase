package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/spf13/cobra"

	toolresult "kbase/internal/result"
)

// modelsOptions is the resolved input of the models verb: the shared
// provider-reaching options plus the stream its result document goes to.
type modelsOptions struct {
	providerOptions

	Stdout io.Writer
}

// runModels selects a provider from the loaded pool, fetches its model
// catalogue, writes the result document — the ids sorted ascending — and
// returns the exit code its outcome maps to. Warnings and a one-line summary
// go to Stderr.
func runModels(ctx context.Context, opts modelsOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "models"); err != nil {
		return 0, err
	}
	outcome, fields := listModels(ctx, opts)
	return emitResult("models", opts.Stdout, outcome, fields)
}

func listModels(ctx context.Context, opts modelsOptions) (string, []toolresult.Field) {
	name, client, ctx, release, err := opts.dial(ctx)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	defer release()
	fields := []toolresult.Field{{Key: "provider", Value: name}}
	start := time.Now()
	ids, err := catalogueIDs(ctx, client)
	if err != nil {
		return failed(fields, err)
	}
	slices.Sort(ids)
	fmt.Fprintf(opts.Stderr, "models ok: %s count=%d elapsed=%s\n", name, len(ids), time.Since(start).Round(time.Millisecond))
	return toolresult.Done, append(fields, toolresult.Field{Key: "models", Value: nonNil(ids)})
}

var (
	modelsFlagProvider string
	modelsFlagTimeout  time.Duration
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "list the models a configured provider exposes",
	Long: `Query a provider's /models endpoint and write its model identifiers,
sorted ascending, as one YAML document on stdout.

The provider is --provider, else the provider named in config.toml, else
the sole entry in providers.toml — with several entries declared and none
chosen, the command refuses, listing them, rather than picking one. Pool
entries that failed to load are reported as warnings, and a summary line
(provider, count, elapsed) is written to stderr.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, opts, err := loadVerbContext()
		if err != nil {
			return err
		}
		opts.Provider = modelsFlagProvider
		opts.Timeout = modelsFlagTimeout
		code, err := runModels(cmd.Context(), modelsOptions{
			providerOptions: opts,
			Stdout:          cmd.OutOrStdout(),
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
	registerProviderFlags(modelsCmd, &modelsFlagProvider, &modelsFlagTimeout, "query")
	rootCmd.AddCommand(mcpBinding(modelsCmd, mcpExcluded))
}
