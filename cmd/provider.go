package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/kb"
	"kbase/internal/model"
	toolresult "kbase/internal/result"
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
	// Layers are the configuration directories this invocation read, in
	// precedence order, as config.Layers resolved them.
	Layers []config.Layer

	// RepoRoot is the repository root this invocation resolved, "" outside a
	// worktree. configure's --project is its one consumer: the layer it names
	// is chosen whether or not it is in the read stack above.
	RepoRoot string

	// WriteDir is the configuration directory this invocation writes into —
	// the layer config.WriteTarget names, and configure's --project may
	// replace. A verb naming a pool entry the pool lacks points at that
	// layer's providers.toml, the file the entry belongs in.
	WriteDir string

	// Providers and Faults are config.Load's two pool returns: the merged
	// pool and the entries that dropped out of it.
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
// on it. It reports the faulted entries, resolves the selection, builds the
// endpoint, and derives the request deadline — the whole preamble every
// provider-reaching verb runs before it has anything of its own to do. The
// entry's name, the rule that named it and the layer that supplies it are
// returned for the verb's result document.
//
// The deadline context is returned rather than applied internally because
// the call it bounds is the caller's: release cancels it and must run once
// the caller is done with the client.
//
// Stderr must be set; the verb checks that before it gets here.
func (o providerOptions) dial(ctx context.Context) (name string, rule providerRule, layer string, client model.Client, callCtx context.Context, release func(), err error) {
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

	// The selection rules are resolveProvider's, never a second copy here. A
	// dial needs the entry, not the categories, so a resolution that lacks a
	// category id is no fault of its own: only a selection that named no
	// usable entry stops it.
	r := resolveProvider(o, config.Overrides{})
	if r.Unselected != nil {
		return "", "", "", nil, nil, nil, r.Unselected
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
	return r.Provider, r.Rule, r.Layer, newClient(model.Endpoint{
		Name: r.Inference.Name, BaseURL: r.Inference.BaseURL, APIKey: r.Inference.APIKey,
		Decisions: decisionsSurface(r.Inference.DecisionsFlavor),
	}), callCtx, cancel, nil
}

// decisionsSurface is whether the decision surface a configured flavour names is
// the OpenAI Decisions shape. The flavour vocabulary is configuration's — it
// refuses a value it does not know at load — and the wire shapes are model's,
// so the one mapping between the two lives here.
func decisionsSurface(flavor string) bool { return flavor == config.FlavorDecisions }

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
// exactly one usable entry — that entry. The second return names the rule
// that answered, so a caller reporting the choice does not decide the
// precedence a second time.
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
//
// providersFile is the absolute providers.toml of the layer this invocation
// writes into — the file a user resolving either refusal must edit.
func selectProviderName(flag, configured string, providers config.Providers, providersFile string) (string, providerRule, error) {
	if name := strings.TrimSpace(flag); name != "" {
		return name, providerRuleFlag, nil
	}
	if name := strings.TrimSpace(configured); name != "" {
		return name, providerRuleConfig, nil
	}
	if len(providers) == 1 {
		for name := range providers {
			return name, providerRuleSoleEntry, nil
		}
	}
	if len(providers) == 0 {
		return "", "", itemError{toolresult.Item{Check: checkProvider, Path: providersFile,
			Detail: fmt.Sprintf("no usable provider in %s — declare one there", providersFile)}}
	}
	names := providerNames(providers)
	return "", "", itemError{toolresult.Item{Check: checkProvider, Key: "--provider", Allowed: names,
		Detail: fmt.Sprintf("no provider selected — pass --provider or set `provider` in %s (available: %s)",
			config.ConfigFileName, strings.Join(names, ", "))}}
}

// providerRule names which of selectProviderName's rules chose a pool entry.
type providerRule string

const (
	// providerRuleFlag is an explicit --provider.
	providerRuleFlag providerRule = "flag"
	// providerRuleConfig is config.toml's `provider`.
	providerRuleConfig providerRule = "config"
	// providerRuleSoleEntry is the pool's only usable entry.
	providerRuleSoleEntry providerRule = "sole-entry"
)

// resolution is where this invocation's inference resolves to and what chose
// it: the endpoint and category ids, the pool entry a selection rule named
// (and which rule, from which layer), why no entry was named, and what the
// configuration lacks when it does not resolve at all.
type resolution struct {
	Inference config.Inference
	// Provider and Rule are the pool entry the selection rules named and
	// the rule that named it; both are empty where none was selected.
	Provider string
	Rule     providerRule
	// Layer is the configuration layer that supplied the selected entry,
	// one of config.LayerSource*; empty where none was selected.
	Layer string
	// Unselected is why no pool entry was selected — the selection's own
	// refusal, or the name's absence from the pool — and nil where one was.
	Unselected error
	// Err is what the configuration lacks for inference to resolve, nil
	// when it resolves. A base URL the environment supplies resolves it
	// with no pool entry selected at all.
	Err error
}

// resolveProvider resolves where a verb's model calls go: the pool entry the
// selection rules name, the endpoint and category ids the layers above the
// files leave, and what the configuration lacks. It is the one resolution
// function behind buildProvider and the show-config verb.
func resolveProvider(p providerOptions, env config.Overrides) resolution {
	name, rule, unselected := selectProviderName(p.Provider, p.Config.Provider, p.Providers, p.providersFile())
	var r resolution
	var entry *config.Provider
	if unselected == nil {
		if e, ok := p.Providers[name]; ok {
			entry, r.Provider, r.Rule, r.Layer = &e, name, rule, e.Layer
		} else {
			unselected = unresolvedProviderError(name, p.Providers, p.Faults, p.providersFile())
		}
	}
	r.Unselected = unselected
	r.Inference, r.Err = config.ResolveInference(p.Config, entry, unselected, p.WriteDir, env)
	return r
}

// providerFields is an endpoint's provenance in the order every result
// document carries it: the pool entry the endpoint came from, the rule that
// named it and the configuration layer that supplied the entry. It is empty
// together where no pool entry supplied the endpoint — a base URL from the
// environment, a configure that dials nothing — so a document never names an
// entry without its rule and layer.
func providerFields(name string, rule providerRule, layer string) []toolresult.Field {
	if name == "" {
		return nil
	}
	return []toolresult.Field{
		{Key: "provider", Value: name},
		{Key: "provider-rule", Value: string(rule)},
		{Key: "provider-layer", Value: layer},
	}
}

// checkProvider names a refusal of the provider choice.
const checkProvider = "provider"

// itemError is wrong input to a verb — a provider choice naming no usable
// pool entry, a malformed flag — as the item its result refuses it with.
type itemError struct{ item toolresult.Item }

func (e itemError) Error() string { return e.item.Detail }

// refusedOrFailed is a verb stopped on err: refused where err is wrong
// input, failed otherwise.
func refusedOrFailed(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	var wrong itemError
	if errors.As(err, &wrong) {
		return refused(fields, wrong.item)
	}
	return failed(fields, err)
}

// providersFile is the providers.toml of the layer this invocation writes
// configuration into: what a refusal sends the user to when the pool lacks the
// entry they named. An options value built without a resolved layer — a unit
// test's — leaves the file name bare, which is the whole path there is.
func (o providerOptions) providersFile() string { return config.ProvidersPath(o.WriteDir) }

// unresolvedProviderError explains a name that is not in the pool,
// distinguishing a provider that FAULTED out of it from one that was never
// declared. Without the distinction a dropped provider reads exactly like a
// typo, and the user goes looking for the wrong mistake.
//
// providersFile is the absolute providers.toml of the layer this invocation
// writes into, and names where a declaration would go. A name that faulted
// keeps that file's own reason and gets no creation remedy: the file stands,
// and what is wrong is the entry in it.
func unresolvedProviderError(name string, providers config.Providers, faults []config.ProviderFault, providersFile string) error {
	item := toolresult.Item{Check: checkProvider, Path: providersFile, Key: "--provider", Allowed: providerNames(providers),
		Remedy: "kbase configure --create",
		Detail: fmt.Sprintf("provider %q not found in %s — a pool entry there supplies baseUrl and apiKeyFile", name, providersFile)}
	for _, f := range faults {
		// A warning fault belongs to a provider that loaded, so it can
		// never be the reason one is missing from the pool.
		if f.Name == name && !f.Warning {
			item.Detail = fmt.Sprintf("provider %q failed to load: %s", name, f.Reason)
			item.Remedy = ""
		}
	}
	return itemError{item}
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

// loadVerbContext is the file-system half of a provider-reaching verb's RunE:
// it resolves the configuration layers this invocation reads from workDir,
// loads both files of each, and returns the shared options prefilled from what
// it read — the layers, the repository root and the write target included, for
// a verb that reports them or writes into one. Flag values and verb-specific
// fields are the caller's to fill in — this helper is the part that is
// identical for every verb, and the only part that touches the process's home
// directory and the filesystem above workDir.
func loadVerbContext(workDir string) (providerOptions, error) {
	repoRoot, err := kb.RepositoryRoot(workDir)
	if err != nil {
		return providerOptions{}, err
	}
	layers, err := config.Layers(flagConfigDir, repoRoot)
	if err != nil {
		return providerOptions{}, err
	}
	opts, err := loadConfigContext(layers)
	if err != nil {
		return providerOptions{}, err
	}
	opts.RepoRoot = repoRoot
	opts.WriteDir = config.WriteTarget(layers, "")
	return opts, nil
}

// loadConfigContext loads both configuration files of every layer into the
// shared options a verb works from.
func loadConfigContext(layers []config.Layer) (providerOptions, error) {
	providers, faults, cfg, err := config.Load(layers)
	if err != nil {
		return providerOptions{}, err
	}
	return providerOptions{
		Layers:    layers,
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
	registerProviderFlag(cmd, provider, action)
	cmd.Flags().DurationVar(timeout, "timeout", defaultListTimeout,
		"deadline for the catalogue request")
}

// registerProviderFlag declares the --provider flag alone, for a verb that
// selects a pool entry without making a request.
func registerProviderFlag(cmd *cobra.Command, provider *string, action string) {
	cmd.Flags().StringVar(provider, "provider", "",
		"pool entry from "+config.ProvidersFileName+" to "+action+" (default: the provider named in "+config.ConfigFileName+", else the sole entry)")
}
