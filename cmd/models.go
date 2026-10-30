package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

// modelsOptions is the resolved input of the models verb: the shared
// provider-reaching options, the logger the registry reports a close failure
// to, plus the stream its result document goes to.
type modelsOptions struct {
	providerOptions

	Stdout io.Writer
	Logger log.Logger
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
	warnFaults(opts.Stderr, opts.Faults)
	name, rule, err := config.Select(opts.inputs(), opts.Provider)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	reg := modelclient.New(opts.NewClient, opts.Logger, "", 0)
	defer reg.CloseAll()
	entry := opts.Providers[name]
	fields := providerFields(name, rule, entry.Layer)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	ids, err := reg.Catalogue(callCtx, entry)
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		return refused(nil, refusal...)
	}
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

The provider is --provider, else the sole entry in providers.toml — with
several entries declared and none chosen, the command refuses, listing them,
rather than picking one. config.toml names no entry: each role names its own
entry in its own model reference. Pool entries that failed to load are
reported as warnings, and a summary line (provider, count, elapsed) is written
to stderr.`,
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
		opts.Provider = modelsFlagProvider
		opts.Timeout = modelsFlagTimeout
		code, err := runModels(cmd.Context(), modelsOptions{
			providerOptions: opts,
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
	registerProviderFlags(modelsCmd, &modelsFlagProvider, &modelsFlagTimeout, "query")
}
