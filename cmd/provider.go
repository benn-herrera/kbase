package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/auth"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/kb"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
	"kbase/internal/secret"
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
	// worktree. A configure command's --local is its one consumer: the layer
	// it names is chosen whether or not it is in the read stack above.
	RepoRoot string

	// WriteDir is the configuration directory written into where no location
	// flag names one — the layer config.WriteTarget names. A refusal naming a
	// pool entry the pool lacks points at that layer's providers.toml, the
	// file the entry belongs in.
	WriteDir string

	// Providers and Faults are the loaded pool and the entries that dropped
	// out of it, or warn — the credentials store's faults among them.
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

// warnFaults says what the pool's faults were, one line each on stderr, and
// returns them as warning items, one per fault, each naming the file it is
// against. A provider dropped from the pool is a warning, not a failure: the
// one being asked for may well have loaded. Fault reasons carry path and IO
// detail only, never key material.
func warnFaults(stderr io.Writer, faults []config.ProviderFault) []toolresult.Item {
	items := []toolresult.Item{}
	for _, f := range faults {
		var subject string
		switch {
		case f.Source == config.SourceStoreDir:
			// The credentials store's own fault: it names no provider.
			subject = "credentials store"
		case f.Warning:
			// The provider SURVIVED this one: it is in the pool and
			// selectable, so saying "unavailable" would send the user
			// looking for an endpoint that is not missing.
			subject = fmt.Sprintf("provider %q", f.Name)
		default:
			subject = fmt.Sprintf("provider %q unavailable", f.Name)
		}
		fmt.Fprintf(stderr, "%s: %s\n", subject, f.LocatedReason())
		items = append(items, toolresult.Item{Path: f.Path, Detail: subject + ": " + f.Reason})
	}
	return items
}

// warningsField is a result's warnings: the items given, an empty list where
// there are none.
func warningsField(items []toolresult.Item) toolresult.Field {
	if items == nil {
		items = []toolresult.Item{}
	}
	return toolresult.Field{Key: "warnings", Value: items}
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
	resolved, err := resolveVerbLayers(workDir)
	if err != nil {
		return providerOptions{}, err
	}
	opts, err := loadConfigContext(resolved.Layers)
	if err != nil {
		return providerOptions{}, err
	}
	opts.RepoRoot, opts.WriteDir = resolved.RepoRoot, resolved.WriteDir
	return opts, nil
}

// resolveVerbLayers is loadVerbContext without the load: the layers this
// invocation reads from workDir, the repository root and the write target,
// with stderr. The configure commands, which load only the stack they write
// into, start from it.
func resolveVerbLayers(workDir string) (providerOptions, error) {
	repoRoot, err := kb.RepositoryRoot(workDir)
	if err != nil {
		return providerOptions{}, err
	}
	layers, err := config.Layers(flagConfigDir, repoRoot)
	if err != nil {
		return providerOptions{}, err
	}
	return providerOptions{Layers: layers, RepoRoot: repoRoot, WriteDir: config.WriteTarget(layers), Stderr: os.Stderr}, nil
}

// loadConfigContext reads the credentials store and loads every configuration
// file of every layer over it into the shared options a verb works from.
func loadConfigContext(layers []config.Layer) (providerOptions, error) {
	dir, err := authStoreDir()
	if err != nil {
		return providerOptions{}, err
	}
	stored := readStoredKeys(dir)
	loaded, err := config.Load(layers, stored)
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

// readStoredKeys reads every key file in the credentials store at dir: each key
// that opens, by the name its file is stored under, and a fault for each file
// that does not. A store directory that does not stand holds no key; one that
// stands and does not list holds none either, and is a nameless fault against
// the directory, its reason the OS error's text without the path.
func readStoredKeys(dir string) config.StoredKeys {
	store := auth.NewStore(dir)
	names, err := store.List()
	if err != nil {
		reason := err.Error()
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			reason = pathErr.Err.Error()
		}
		return config.StoredKeys{Faults: []config.StoreFault{{Path: dir, Reason: reason}}}
	}
	stored := config.StoredKeys{Keys: make(map[string]secret.Key, len(names))}
	for _, name := range names {
		key, err := store.Read(name)
		if err != nil {
			stored.Faults = append(stored.Faults, storeFault(dir, name, err))
			continue
		}
		stored.Keys[name] = key
	}
	return stored
}

// storeFault is the fault of the key file stored under name, from the store's
// error: a *auth.FileError's path and what went wrong with the file, apart; any
// other error, which no one file caused, against the store's directory dir.
func storeFault(dir, name string, err error) config.StoreFault {
	var fileErr *auth.FileError
	if !errors.As(err, &fileErr) {
		return config.StoreFault{Name: name, Path: dir, Reason: err.Error()}
	}
	return config.StoreFault{Name: name, Path: fileErr.Path, Reason: fileErr.Err.Error(),
		DoesNotOpen: errors.Is(fileErr.Err, auth.ErrTruncated) || errors.Is(fileErr.Err, auth.ErrDoesNotOpen)}
}

// registerProviderFlags declares the --provider and --timeout flags a verb
// that selects a pool entry to dial carries. action completes "the provider
// to <action>" in the --provider help text ("query", "report").
func registerProviderFlags(cmd *cobra.Command, provider *string, timeout *time.Duration, action string) {
	registerProviderFlag(cmd, provider, action)
	registerTimeoutFlag(cmd, timeout, sharedTimeoutUsage)
}

// registerProviderFlag declares the --provider flag alone, for a verb that
// selects a pool entry without making a request.
func registerProviderFlag(cmd *cobra.Command, provider *string, action string) {
	cmd.Flags().StringVar(provider, "provider", "",
		"the provider to "+action+", named in any case (default: the only provider)")
}

// sharedTimeoutUsage is what --timeout bounds: one request, never a run.
const sharedTimeoutUsage = "deadline for one model request"

// registerTimeoutFlag declares --timeout alone, for a verb whose bound is not
// a catalogue request's.
func registerTimeoutFlag(cmd *cobra.Command, timeout *time.Duration, usage string) {
	cmd.Flags().DurationVar(timeout, "timeout", defaultListTimeout, usage)
}

// configureLocation is the directory a configure command writes into and the
// layer that names it: --global's ~/.config/kbase, --local's
// <repository>/.kbase, else WriteDir — the directory --config-dir or
// KBASE_CONFIG_DIR names, else the global one. The two flags together, either
// beside --config-dir, and --local outside a git worktree are refused.
func configureLocation(opts providerOptions, global, local bool, workDir string) (dir, layer string, items []toolresult.Item, err error) {
	top := opts.Layers[0].Source
	switch {
	case global && local:
		return "", "", []toolresult.Item{{Check: toolresult.CheckUsage, Key: argmark.Flag("local"),
			Detail: fmt.Sprintf("%s and %s name different configuration directories; pass one or the other", argmark.Flag("global"), argmark.Flag("local"))}}, nil
	case (global || local) && top == config.LayerSourceExplicit:
		flag := argmark.Flag("global")
		if local {
			flag = argmark.Flag("local")
		}
		return "", "", []toolresult.Item{{Check: toolresult.CheckUsage, Key: flag,
			Detail: flag + " and --config-dir name different configuration directories; pass one or the other"}}, nil
	case local && opts.RepoRoot == "":
		return "", "", []toolresult.Item{{Check: checkWorktree, Path: workDir, Remedy: "git init",
			Detail: fmt.Sprintf("%s is not inside a git worktree, which %s writes into; `git init` there is the operator's to run", workDir, argmark.Flag("local"))}}, nil
	case local:
		return config.RepositoryDir(opts.RepoRoot), config.LayerSourceRepository, nil, nil
	case global:
		dir, err := config.GlobalDir()
		return dir, config.LayerSourceGlobal, nil, err
	case top == config.LayerSourceExplicit || top == config.LayerSourceEnvironment:
		return opts.WriteDir, top, nil, nil
	}
	return opts.WriteDir, config.LayerSourceGlobal, nil, nil
}

// locationFlag is the location flag a configure call carried, by its name:
// "local", "global", or "" for neither.
func locationFlag(global, local bool) string {
	switch {
	case local:
		return "local"
	case global:
		return "global"
	}
	return ""
}

// loadWrittenStack loads, over the credentials store, the configuration a
// configure command writes into: the layer at dir, which source names, and
// the layers beneath it (config.WrittenStack). Its WriteDir is dir.
func loadWrittenStack(dir, source string) (providerOptions, error) {
	layers, err := config.WrittenStack(config.Layer{Path: dir, Source: source})
	if err != nil {
		return providerOptions{}, err
	}
	opts, err := loadConfigContext(layers)
	if err != nil {
		return providerOptions{}, err
	}
	opts.WriteDir = dir
	return opts, nil
}

// registerLocationFlags declares a configure command's --global and --local.
func registerLocationFlags(cmd *cobra.Command, global, local *bool) {
	cmd.Flags().BoolVar(global, "global", false,
		"write into the global configuration directory, ~/.config/kbase, the default where --config-dir and KBASE_CONFIG_DIR name none")
	cmd.Flags().BoolVar(local, "local", false,
		"write into the repository's configuration directory, <repository>/"+config.RepositoryDirName+" (refused outside a git worktree)")
}
