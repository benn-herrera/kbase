package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/model"
)

// defaultListTimeout bounds one `kbase models` round-trip when --timeout is
// not given. Listing a catalogue is a small GET: a provider that cannot
// answer it inside this window is unreachable, not busy.
const defaultListTimeout = 30 * time.Second

// newClientFunc constructs the model client for an endpoint. It is the seam
// that lets the verb be exercised without a network — production passes
// model.NewHTTPClient, tests pass a factory returning a model.MockClient.
type newClientFunc func(model.Endpoint) model.Client

// modelsOptions is the resolved input of the models verb: the already-loaded
// config-file state, the flag values, and the process resources the verb is
// otherwise not entitled to name for itself.
type modelsOptions struct {
	// Providers and Faults are LoadProviders' two returns: the usable pool
	// and the entries that dropped out of it.
	Providers config.Providers
	Faults    []config.ProviderFault

	// Config supplies the fallback provider choice when --provider is unset.
	Config config.Config

	// Provider is the --provider flag; empty selects by the rules in
	// selectProviderName.
	Provider string

	// Timeout is the --timeout flag; zero or negative means
	// defaultListTimeout.
	Timeout time.Duration

	Stdout io.Writer // model ids, one per line, sorted ascending
	Stderr io.Writer // warnings and the summary line

	// NewClient is the client seam; nil means model.NewHTTPClient.
	NewClient newClientFunc
}

// runModels selects a provider from the loaded pool, fetches its model
// catalogue, and writes the ids to Stdout sorted ascending, one per line.
// Warnings and a one-line summary go to Stderr, keeping stdout a clean list
// a pipeline can consume.
func runModels(ctx context.Context, opts modelsOptions) error {
	if opts.Stdout == nil || opts.Stderr == nil {
		return fmt.Errorf("models: Stdout and Stderr are required")
	}

	// A provider dropped from the pool is a warning, not a failure: the one
	// being asked for may well have loaded. Fault reasons carry path and IO
	// detail only, never key material.
	for _, f := range opts.Faults {
		fmt.Fprintf(opts.Stderr, "models: provider %q unavailable: %s\n", f.Name, f.Reason)
	}

	name, err := selectProviderName(opts.Provider, opts.Config.Provider, opts.Providers)
	if err != nil {
		return err
	}
	p, ok := opts.Providers[name]
	if !ok {
		return unresolvedProviderError(name, opts.Faults)
	}

	newClient := opts.NewClient
	if newClient == nil {
		newClient = model.NewHTTPClient
	}
	client := newClient(model.Endpoint{Name: name, BaseURL: p.BaseURL, APIKey: p.APIKey})

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	infos, err := client.ListModels(ctx)
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("models: list: %w", err)
	}

	ids := make([]string, 0, len(infos))
	for _, mi := range infos {
		ids = append(ids, mi.ID)
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

// selectProviderName resolves WHICH pool entry to query: the --provider
// flag, else config.toml's `provider`, else — only when the pool holds
// exactly one usable entry — that entry.
//
// There is deliberately no conventional default name. With several
// endpoints declared and none chosen, any pick would send the request, and
// the corpus behind it, somewhere the user did not name; the command fails
// and lists the candidates instead.
//
// This and unresolvedProviderError are shared by every verb that reaches a
// provider, so their errors name no verb: a message reading "models: ..."
// under `kbase configure` sends the user looking at the wrong command.
func selectProviderName(flag, configured string, providers config.Providers) (string, error) {
	if name := strings.TrimSpace(flag); name != "" {
		return name, nil
	}
	if name := strings.TrimSpace(configured); name != "" {
		return name, nil
	}
	if len(providers) == 1 {
		for name := range providers {
			return name, nil
		}
	}
	if len(providers) == 0 {
		return "", fmt.Errorf("no usable provider in %s — declare one there",
			config.ProvidersFileName)
	}
	return "", fmt.Errorf("no provider selected — pass --provider or set `provider` in %s (available: %s)",
		config.ConfigFileName, strings.Join(providerNames(providers), ", "))
}

// unresolvedProviderError explains a name that is not in the pool,
// distinguishing a provider that FAULTED out of it from one that was never
// declared. Without the distinction a dropped provider reads exactly like a
// typo, and the user goes looking for the wrong mistake.
func unresolvedProviderError(name string, faults []config.ProviderFault) error {
	for _, f := range faults {
		if f.Name == name {
			return fmt.Errorf("provider %q failed to load: %s", name, f.Reason)
		}
	}
	return fmt.Errorf("provider %q not found in %s", name, config.ProvidersFileName)
}

// providerNames returns the pool's entry names, sorted — a stable list for
// error messages the user is expected to choose from.
func providerNames(providers config.Providers) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
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
		dir, err := resolveConfigDir()
		if err != nil {
			return err
		}
		providers, faults, err := config.LoadProviders(config.ProvidersPath(dir))
		if err != nil {
			return err
		}
		cfg, err := config.LoadConfig(config.ConfigPath(dir))
		if err != nil {
			return err
		}
		return runModels(cmd.Context(), modelsOptions{
			Providers: providers,
			Faults:    faults,
			Config:    cfg,
			Provider:  modelsFlagProvider,
			Timeout:   modelsFlagTimeout,
			Stdout:    os.Stdout,
			Stderr:    os.Stderr,
		})
	},
}

func init() {
	modelsCmd.Flags().StringVar(&modelsFlagProvider, "provider", "",
		"pool entry from "+config.ProvidersFileName+" to query (default: the provider named in "+config.ConfigFileName+", else the sole entry)")
	modelsCmd.Flags().DurationVar(&modelsFlagTimeout, "timeout", defaultListTimeout,
		"deadline for the catalogue request")
	rootCmd.AddCommand(modelsCmd)
}
