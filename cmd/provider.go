package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/model"
)

// defaultListTimeout bounds one catalogue round-trip when --timeout is not
// given. Listing a catalogue is a small GET: a provider that cannot answer
// it inside this window is unreachable, not busy.
const defaultListTimeout = 30 * time.Second

// newClientFunc constructs the model client for an endpoint. It is the seam
// that lets a verb be exercised without a network — production passes
// model.NewHTTPClient, tests pass a factory returning a model.MockClient.
type newClientFunc func(model.Endpoint) model.Client

// providerOptions is the input every provider-reaching verb shares: the
// already-loaded pool state, the two flags that decide WHICH provider is
// reached and for HOW LONG, and the process resources a verb is otherwise
// not entitled to name for itself. Verbs embed it and add their own fields,
// so the shared half is described once, here.
type providerOptions struct {
	// Providers and Faults are LoadProviders' two returns: the usable pool
	// and the entries that dropped out of it.
	Providers config.Providers
	Faults    []config.ProviderFault

	// Config supplies the fallback provider choice when Provider is unset.
	// No other part of it is consulted on this path — a verb that reads
	// more (configure deliberately ignores [models]) says so itself.
	Config config.Config

	// Provider is the --provider flag; empty selects by the rules in
	// selectProviderName.
	Provider string

	// Timeout is the --timeout flag; zero or negative means
	// defaultListTimeout.
	Timeout time.Duration

	// Stderr takes warnings, failure detail, and the verb's summary line —
	// everything that is not the verb's actual output.
	Stderr io.Writer

	// NewClient is the client seam; nil means model.NewHTTPClient.
	NewClient newClientFunc
}

// dial resolves which pool entry this invocation talks to and opens a client
// on it. It reports the faulted entries, applies the selection rules, builds
// the endpoint, and derives the request deadline — the whole preamble every
// provider-reaching verb runs before it has anything of its own to do.
//
// The deadline context is returned rather than applied internally because
// the call it bounds is the caller's: release cancels it and must run once
// the caller is done with the client.
//
// Stderr must be set; the verb checks that before it gets here.
func (o providerOptions) dial(ctx context.Context) (name string, client model.Client, callCtx context.Context, release func(), err error) {
	// A provider dropped from the pool is a warning, not a failure: the one
	// being asked for may well have loaded. Fault reasons carry path and IO
	// detail only, never key material. Like the errors below, they name no
	// verb — see selectProviderName.
	for _, f := range o.Faults {
		if f.Warning {
			// The provider SURVIVED this one: it is in the pool and
			// selectable, so saying "unavailable" would send the user
			// looking for an endpoint that is not missing.
			fmt.Fprintf(o.Stderr, "provider %q: %s\n", f.Name, f.Reason)
			continue
		}
		fmt.Fprintf(o.Stderr, "provider %q unavailable: %s\n", f.Name, f.Reason)
	}

	name, err = selectProviderName(o.Provider, o.Config.Provider, o.Providers)
	if err != nil {
		return "", nil, nil, nil, err
	}
	p, ok := o.Providers[name]
	if !ok {
		return "", nil, nil, nil, unresolvedProviderError(name, o.Faults)
	}

	newClient := o.NewClient
	if newClient == nil {
		// Wrapped rather than referenced directly so the seam does not
		// depend on NewHTTPClient's exact return type.
		newClient = func(e model.Endpoint) model.Client { return model.NewHTTPClient(e) }
	}

	timeout := o.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	return name, newClient(model.Endpoint{Name: name, BaseURL: p.BaseURL, APIKey: p.APIKey}), callCtx, cancel, nil
}

// safeBaseURL is a provider's base URL with any userinfo removed.
//
// `https://user:pass@host/v1` is a legal providers.toml value, and run.json is
// by design an artifact an operator shares as evidence — a credential in it
// would travel with the run report. The API key never appears there (it lives
// on the Endpoint and goes on the wire), so this closes the one remaining way
// a secret could reach the record or the console. An unparseable value is
// passed through: this process already dialed it, and nothing here could
// establish which part of a non-URL is a credential.
func safeBaseURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

// catalogueIDs fetches the provider's catalogue and returns the model ids in
// the order the provider listed them. Callers that show them to a human sort
// first; callers that classify them do not care.
func catalogueIDs(ctx context.Context, client model.Client) ([]string, error) {
	infos, err := client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	ids := make([]string, 0, len(infos))
	for _, mi := range infos {
		ids = append(ids, mi.ID)
	}
	return ids, nil
}

// selectProviderName resolves WHICH pool entry to reach: the --provider
// flag, else config.toml's `provider`, else — only when the pool holds
// exactly one usable entry — that entry.
//
// There is deliberately no conventional default name. With several
// endpoints declared and none chosen, any pick would send the request, and
// the corpus behind it, somewhere the user did not name; the command fails
// and lists the candidates instead.
//
// This, unresolvedProviderError, and dial's fault warnings are shared by
// every verb that reaches a provider, so none of them names a verb: a
// message reading "models: ..." under `kbase configure` sends the user
// looking at the wrong command. Verb-specific diagnostics keep their prefix.
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
		// A warning fault belongs to a provider that loaded, so it can
		// never be the reason one is missing from the pool.
		if f.Name == name && !f.Warning {
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

// loadVerbContext is the file-system half of a provider-reaching verb's
// RunE: it resolves the configuration directory and loads both files out of
// it, returning the directory (verbs that write need it) and the shared
// options prefilled from what it read. Flag values and verb-specific fields
// are the caller's to fill in — this helper is the part that is identical
// for every verb, and the only part that touches the process.
func loadVerbContext() (string, providerOptions, error) {
	dir, err := resolveConfigDir()
	if err != nil {
		return "", providerOptions{}, err
	}
	providers, faults, err := config.LoadProviders(config.ProvidersPath(dir))
	if err != nil {
		return "", providerOptions{}, err
	}
	cfg, err := config.LoadConfig(config.ConfigPath(dir))
	if err != nil {
		return "", providerOptions{}, err
	}
	return dir, providerOptions{
		Providers: providers,
		Faults:    faults,
		Config:    cfg,
		Stderr:    os.Stderr,
	}, nil
}

// registerProviderFlags declares the --provider and --timeout flags every
// provider-reaching verb carries. action completes "pool entry ... to
// <action>" in the --provider help text ("query", "configure").
func registerProviderFlags(cmd *cobra.Command, provider *string, timeout *time.Duration, action string) {
	cmd.Flags().StringVar(provider, "provider", "",
		"pool entry from "+config.ProvidersFileName+" to "+action+" (default: the provider named in "+config.ConfigFileName+", else the sole entry)")
	cmd.Flags().DurationVar(timeout, "timeout", defaultListTimeout,
		"deadline for the catalogue request")
}
