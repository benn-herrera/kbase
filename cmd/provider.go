package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/model"
	toolresult "kbase/internal/result"
)

// defaultListTimeout bounds one request when --timeout is not given: one
// catalogue round-trip, or one item's call under score. A provider that cannot
// answer a small GET inside this window is unreachable, not busy.
const defaultListTimeout = 30 * time.Second

// newClientFunc constructs the model client for an endpoint. It is the seam
// that lets a verb be exercised without a network — production passes
// model.NewClient, tests pass a factory returning a model.MockClient.
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
	// replace. A refusal naming a pool entry the pool lacks points at that
	// layer's providers.toml, the file the entry belongs in.
	WriteDir string

	// Providers and Faults are the loaded pool and the entries that dropped
	// out of it.
	Providers config.Providers
	Faults    []config.ProviderFault

	// Config is the merged config.toml. A verb that resolves a role reads its
	// [models] table; the verb that selects a pool entry to dial reads none of
	// it.
	Config config.Config

	// Provider is the --provider flag, for the verbs that select a pool entry
	// to dial. It names no role: a role's endpoint is its own reference's.
	Provider string

	// Timeout is the --timeout flag; zero or negative means
	// defaultListTimeout.
	Timeout time.Duration

	// Stderr takes warnings, failure detail, and the verb's summary line —
	// everything that is not the verb's actual output.
	Stderr io.Writer

	// NewClient is the client seam; nil means model.NewClient.
	NewClient newClientFunc
}

// inputs is what a resolution decision reads: the loaded configuration this
// invocation works from, and the environment's strings.
func (o providerOptions) inputs(env config.Overrides) config.Inputs {
	return config.Inputs{Cfg: o.Config, Providers: o.Providers, Faults: o.Faults, WriteDir: o.WriteDir, Env: env}
}

// clients is the registry of the model clients one verb opens: at most one per
// endpoint, opened on first use and closed together before the verb returns.
// It is the only constructor of a model.Client in kbase, and it derives no
// deadline — each caller bounds its own call.
type clients struct {
	newClient func(model.Endpoint) model.Client
	// promptCacheKey is what every client the registry opens stamps on the
	// requests it sends, "" for none: a scoring run passes its run id, so
	// every ask of the run carries one key; every other verb names no run and
	// passes nothing.
	promptCacheKey string
	opened         map[string]model.Client
	// lg takes a close that failed. A close error is never raised: the verb
	// has usually written its result by then.
	lg log.Logger
}

// newClients is a registry over seam, nil taking model.NewClient — which builds
// the wire client the endpoint's protocol names — whose close failures go to lg
// and whose every client carries promptCacheKey.
func newClients(seam newClientFunc, lg log.Logger, promptCacheKey string) *clients {
	if seam == nil {
		seam = model.NewClient
	}
	if lg == nil {
		lg = log.Discard()
	}
	return &clients{newClient: seam, promptCacheKey: promptCacheKey, opened: map[string]model.Client{}, lg: lg}
}

// get is the client for ep, opened on first use: two roles naming one pool
// entry share a client and its connection pool, and an environment-mode
// endpoint shares by base URL, which is all it has.
func (c *clients) get(ep config.Endpoint) (model.Client, error) {
	key := ep.Provider
	if key == "" {
		key = "base:" + ep.BaseURL
	}
	if client, ok := c.opened[key]; ok {
		return client, nil
	}
	apiKey, err := c.key(ep)
	if err != nil {
		return nil, err
	}
	client := c.newClient(model.Endpoint{Name: ep.Provider, BaseURL: ep.BaseURL, APIKey: apiKey,
		Protocol: chatProtocol(ep.Protocol), Decisions: decisionsSurface(ep.Flavor), PromptCacheKey: c.promptCacheKey})
	c.opened[key] = client
	return client, nil
}

// preflight is the key-file read alone: the refusal a file KBASE_API_KEY_FILE
// names gives where it cannot be read, and nothing opened. It is what a verb
// that reports an endpoint without dialling it can still learn.
func (c *clients) preflight(ep config.Endpoint) error {
	_, err := c.key(ep)
	return err
}

// key is the key ep is dialled with: the file KBASE_API_KEY_FILE names where
// it names one, and otherwise the key the pool entry's own file supplied.
func (c *clients) key(ep config.Endpoint) (string, error) {
	if ep.APIKeyFile == "" {
		return ep.APIKey, nil
	}
	b, err := os.ReadFile(ep.APIKeyFile)
	if err != nil {
		return "", config.Refusal{Key: config.EnvAPIKeyFile, Path: ep.APIKeyFile,
			Detail: fmt.Sprintf("%s names %s, which cannot be read: %v", config.EnvAPIKeyFile, ep.APIKeyFile, err)}
	}
	return strings.TrimSpace(string(b)), nil
}

// closeAll closes every client the registry opened, whatever the verb's
// outcome. A client that is no io.Closer — a test's mock — has nothing to
// release.
func (c *clients) closeAll() {
	for key, client := range c.opened {
		closer, ok := client.(io.Closer)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			c.lg.Warn("a model client was not closed cleanly", "endpoint", key, "error", err)
		}
	}
	c.opened = map[string]model.Client{}
}

// entryEndpoint is a pool entry as the registry takes it, for a verb that dials
// the entry it selected to read that entry's catalogue: no role, no model, and
// nothing the registry does not read.
func entryEndpoint(entry config.Provider) config.Endpoint {
	return config.Endpoint{Provider: entry.Name, BaseURL: entry.BaseURL, APIKey: entry.APIKey,
		Protocol: entry.Protocol(), Flavor: entry.Flavor()}
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

// decisionsSurface is whether the decision surface a configured flavour names is
// the OpenAI Decisions shape. The flavour vocabulary is configuration's — it
// refuses a value it does not know at load — and the wire shapes are model's,
// so the one mapping between the two lives here.
func decisionsSurface(flavor string) bool { return flavor == config.FlavorDecisions }

// chatProtocol is the chat wire protocol a configured `api` names, in the model
// package's vocabulary. As decisionsSurface is for the decision surfaces: the
// configured value is configuration's — it refuses one it does not know at load
// — and the wire shapes are model's, so the one mapping between the two lives
// here.
func chatProtocol(api string) model.Protocol {
	if api == config.ProviderAPIAnthropic {
		return model.ProtocolAnthropic
	}
	return model.ProtocolOpenAI
}

// warnFaults says what the pool's faults were, one line each. A provider
// dropped from the pool is a warning, not a failure: the one being asked for
// may well have loaded. Fault reasons carry path and IO detail only, never key
// material.
func warnFaults(stderr io.Writer, faults []config.ProviderFault) {
	for _, f := range faults {
		if f.Warning {
			// The provider SURVIVED this one: it is in the pool and
			// selectable, so saying "unavailable" would send the user
			// looking for an endpoint that is not missing.
			fmt.Fprintf(stderr, "provider %q: %s\n", f.Name, f.Reason)
			continue
		}
		fmt.Fprintf(stderr, "provider %q unavailable: %s\n", f.Name, f.Reason)
	}
}

// supersededProvider is the one line a file-mode invocation writes where
// config.toml still states the provider key: it loads, and nothing resolves
// from it — a role names its own entry in its own reference — so the key is
// reported superseded rather than silently obeyed. A verb that resolves
// nothing, and an environment-mode invocation, read no file value and say
// nothing.
func supersededProvider(stderr io.Writer, cfg config.Config) {
	if cfg.Provider == "" {
		return
	}
	fmt.Fprintln(stderr, "config.toml states the superseded provider key; kbase does not use it — run kbase configure to rewrite the file")
}

// providerFields is a pool entry's provenance in the order a verb that selects
// one states it: the entry the endpoint came from, the rule that named it and
// the configuration layer that supplied the entry. It is empty together where
// no entry was selected, so a document never names an entry without its rule
// and layer.
func providerFields(name, rule, layer string) []toolresult.Field {
	if name == "" {
		return nil
	}
	return []toolresult.Field{
		{Key: "provider", Value: name},
		{Key: "provider-rule", Value: rule},
		{Key: "provider-layer", Value: layer},
	}
}

// roleRecord is one role's record in a result document, in §7.1's key order:
// the role, what answered it, where that endpoint came from, and the model
// this invocation asked for. The pool entry's name, rule and layer stand
// together or not at all — an environment endpoint has none of them — and
// back-end is what tells the cases apart, so a reader never infers the source
// from an absence.
func roleRecord(ep config.Endpoint) toolresult.Record {
	record := toolresult.Record{
		{Key: "role", Value: string(ep.Role)},
		{Key: "back-end", Value: backEndName(ep)},
	}
	record = append(record, providerFields(ep.Provider, ep.Rule, ep.Layer)...)
	return append(record, toolresult.Field{Key: "model", Value: stringOrNull(ep.Model)})
}

// backEndName is what answered ep, as a record states it: the environment the
// endpoint came from, or a pool entry's provider.
func backEndName(ep config.Endpoint) string {
	if ep.Mode == config.ModeEnvironment {
		return string(config.ModeEnvironment)
	}
	return string(config.BackEndProvider)
}

// rolesField is the roles record: one record per role, in role order.
func rolesField(records []toolresult.Record) toolresult.Field {
	return toolresult.Field{Key: "roles", Value: records}
}

// checkProvider names a refusal of the provider choice.
const checkProvider = "provider"

// refusedOrFailed is a verb stopped on err: refused where err is wrong input,
// failed otherwise.
func refusedOrFailed(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	if items := refusalItems(err); items != nil {
		return refused(fields, items...)
	}
	return failed(fields, err)
}

// refusalItems is err as the items it refuses with — a configuration refusal
// or several of them, or items already shaped as one — and nil where err is a
// failure. It is the one mapping from configuration's refusals to a result
// document's items.
func refusalItems(err error) []toolresult.Item {
	var shaped toolresult.Refusal
	if errors.As(err, &shaped) {
		return shaped
	}
	var several config.Refusals
	if errors.As(err, &several) {
		items := make([]toolresult.Item, 0, len(several))
		for _, r := range several {
			items = append(items, refusalItem(r))
		}
		return items
	}
	var one config.Refusal
	if errors.As(err, &one) {
		return []toolresult.Item{refusalItem(one)}
	}
	return nil
}

// refusalItem is one configuration refusal as a result document's item. A
// resolution refusal is about the provider choice unless it names its own
// check.
func refusalItem(r config.Refusal) toolresult.Item {
	check := r.Check
	if check == "" {
		check = checkProvider
	}
	return toolresult.Item{Check: check, Key: r.Key, Path: r.Path, Allowed: r.Allowed, Remedy: r.Remedy, Detail: r.Detail}
}

// loadVerbContext is the file-system half of a provider-reaching verb's RunE:
// it resolves the configuration layers this invocation reads from workDir,
// loads them, and returns the shared options prefilled from what it read — the
// layers, the repository root and the write target included, for a verb that
// reports them or writes into one. Flag values and verb-specific fields are
// the caller's to fill in — this helper is the part that is identical for
// every verb, and the only part that touches the process's home directory and
// the filesystem above workDir.
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

// loadConfigContext loads every configuration file of every layer into the
// shared options a verb works from.
func loadConfigContext(layers []config.Layer) (providerOptions, error) {
	loaded, err := config.Load(layers)
	if err != nil {
		return providerOptions{}, err
	}
	return providerOptions{
		Layers:    layers,
		Providers: loaded.Providers,
		Faults:    loaded.Faults,
		Config:    loaded.Cfg,
		Stderr:    os.Stderr,
	}, nil
}

// registerProviderFlags declares the --provider and --timeout flags a verb
// that selects a pool entry to dial carries. action completes "pool entry ...
// to <action>" in the --provider help text ("query", "configure", "report").
func registerProviderFlags(cmd *cobra.Command, provider *string, timeout *time.Duration, action string) {
	registerProviderFlag(cmd, provider, action)
	registerTimeoutFlag(cmd, timeout, sharedTimeoutUsage)
}

// registerProviderFlag declares the --provider flag alone, for a verb that
// selects a pool entry without making a request.
func registerProviderFlag(cmd *cobra.Command, provider *string, action string) {
	cmd.Flags().StringVar(provider, "provider", "",
		"pool entry from "+config.ProvidersFileName+" to "+action+" (default: the pool's only entry)")
}

// sharedTimeoutUsage is what --timeout bounds: one request, never a run.
const sharedTimeoutUsage = "deadline for one model request"

// registerTimeoutFlag declares --timeout alone, for a verb whose bound is not
// a catalogue request's.
func registerTimeoutFlag(cmd *cobra.Command, timeout *time.Duration, usage string) {
	cmd.Flags().DurationVar(timeout, "timeout", defaultListTimeout, usage)
}
