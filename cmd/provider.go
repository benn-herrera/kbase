package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/kb"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

// defaultListTimeout bounds one request when --timeout is not given: one
// catalogue round-trip, or one item's call under score. A provider that cannot
// answer a small GET inside this window is unreachable, not busy.
const defaultListTimeout = 30 * time.Second

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
	NewClient modelclient.NewClientFunc
}

// inputs is what a resolution decision reads: the loaded configuration this
// invocation works from.
func (o providerOptions) inputs() config.Inputs {
	return config.Inputs{Cfg: o.Config, Providers: o.Providers, Faults: o.Faults, WriteDir: o.WriteDir}
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

// warnCleartextKey writes config.CleartextWarning's line for ep, judged on the
// key ep is dialled with, and reports whether it wrote one.
func warnCleartextKey(stderr io.Writer, ep config.Endpoint) bool {
	line := config.CleartextWarning(ep.BaseURL, ep.APIKey)
	if line == "" {
		return false
	}
	fmt.Fprintln(stderr, line)
	return true
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
// the role, the pool entry that answered it with the rule that named the entry
// and the layer that supplied it, and the model this invocation asked for.
func roleRecord(ep config.Endpoint) toolresult.Record {
	record := toolresult.Record{{Key: "role", Value: string(ep.Role)}}
	record = append(record, providerFields(ep.Provider, ep.Rule, ep.Layer)...)
	return append(record, toolresult.Field{Key: "model", Value: stringOrNull(ep.Model)})
}

// rolesField is the roles record: one record per role, in role order.
func rolesField(records []toolresult.Record) toolresult.Field {
	return toolresult.Field{Key: "roles", Value: records}
}

// refusedOrFailed is the one mapping from an error to a result document: a
// refusal is refused with its items, a retry-class error is retry, and
// anything else failed.
func refusedOrFailed(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	var (
		refusal toolresult.Refusal
		live    buildstore.Retry
	)
	switch {
	case errors.As(err, &refusal):
		return refused(fields, refusal...)
	case errors.As(err, &live):
		return toolresult.Retry, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: []toolresult.Item(live)})
	}
	return failed(fields, err)
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
