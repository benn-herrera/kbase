package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"
)

// modelsOptions is the resolved input of the models verb: the shared
// provider-reaching options plus the stream the catalogue itself goes to.
type modelsOptions struct {
	providerOptions

	Stdout io.Writer // model ids, one per line, sorted ascending
}

// runModels selects a provider from the loaded pool, fetches its model
// catalogue, and writes the ids to Stdout sorted ascending, one per line.
// Warnings and a one-line summary go to Stderr, keeping stdout a clean list
// a pipeline can consume.
func runModels(ctx context.Context, opts modelsOptions) error {
	if err := requireStreams(opts.Stdout, opts.Stderr, "models"); err != nil {
		return err
	}

	name, client, ctx, release, err := opts.dial(ctx)
	if err != nil {
		return err
	}
	defer release()

	start := time.Now()
	ids, err := catalogueIDs(ctx, client)
	elapsed := time.Since(start)
	if err != nil {
		return err
	}

	slices.Sort(ids)
	for _, id := range ids {
		if _, err := fmt.Fprintln(opts.Stdout, id); err != nil {
			return fmt.Errorf("models: write stdout: %w", err)
		}
	}
	fmt.Fprintf(opts.Stderr, "models ok: %s count=%d elapsed=%s\n",
		name, len(ids), elapsed.Round(time.Millisecond))
	return nil
}

var (
	modelsFlagProvider string
	modelsFlagTimeout  time.Duration
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "list the models a configured provider exposes",
	Long: `Query a provider's /models endpoint and print one model identifier per
line, sorted ascending, to stdout.

The provider is --provider, else the provider named in config.toml, else
the sole entry in providers.toml — with several entries declared and none
chosen, the command lists them and fails rather than picking one. Pool
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
		return runModels(cmd.Context(), modelsOptions{
			providerOptions: opts,
			Stdout:          os.Stdout,
		})
	},
}

func init() {
	registerProviderFlags(modelsCmd, &modelsFlagProvider, &modelsFlagTimeout, "query")
	rootCmd.AddCommand(modelsCmd)
}
