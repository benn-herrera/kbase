package main

import (
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

// showConfigOptions is the resolved input of the show-config verb: the layers
// this invocation read with what each holds, the merged pool state loaded out
// of them, the environment's strings, and the stream the document goes to. The
// verb makes no request and opens no client, so providerOptions' NewClient
// stays unset.
type showConfigOptions struct {
	providerOptions

	// Env is the environment's inference settings.
	Env config.Overrides

	Stdout io.Writer

	// Logger takes the registry's close failures; the registry here never
	// opens a client, so only a preflight can reach it.
	Logger log.Logger
}

// runShowConfig reports how this invocation's configuration resolves and writes
// its result document. It writes no file, reads no KB and makes no request, so
// its outcome is done whenever configuration loads; a file that fails the
// strict load refuses before the verb runs, as it does for every subcommand.
func runShowConfig(opts showConfigOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "show-config"); err != nil {
		return 0, err
	}
	return emitResult("show-config", opts.Stdout, toolresult.Done, showConfigFields(opts))
}

// showConfigFields is the report: every layer read and what it holds, every
// pool entry the merge produced with the layer it came from and every fault,
// the merged role values, the inference variable names the process supplies,
// the selection, the mode, one record per role, and the diagnostics. It carries
// no key material and no variable value.
func showConfigFields(opts showConfigOptions) []toolresult.Field {
	// LoadProviders walks a map, so its faults arrive in no fixed order; a
	// report sorts them, so the same configuration always reads the same way.
	faults := slices.Clone(opts.Faults)
	slices.SortFunc(faults, func(a, b config.ProviderFault) int {
		return strings.Compare(a.Name, b.Name)
	})
	facts := config.Report(opts.Config, opts.Providers, faults, opts.Layers, fileExists, os.Getenv)
	s := opts.inputs(opts.Env)
	mode, modeErr := config.ModeOf(s)
	if mode == config.ModeFile {
		// The report resolves every role from the files in file mode, so a
		// config.toml still stating the superseded key is said once on stderr,
		// as it is by any other verb that resolves from it.
		supersededProvider(opts.Stderr, opts.Config)
	}
	reg := newClients(opts.NewClient, opts.Logger)
	roles, roleItems := resolvedRoles(reg, s, mode, modeErr)
	return []toolresult.Field{
		{Key: "layers", Value: layerRecords(facts.Layers)},
		{Key: "providers", Value: providerRecords(facts.Entries)},
		{Key: "config", Value: toolresult.Record{
			{Key: "models", Value: toolresult.Record{
				{Key: config.CategoryDecision, Value: stringOrNull(facts.Models.Decision)},
				{Key: config.CategoryText, Value: stringOrNull(facts.Models.Text)},
				{Key: config.CategoryScoring, Value: stringOrNull(facts.Models.Scoring)},
			}},
		}},
		{Key: "environment", Value: facts.Environment},
		{Key: "selection", Value: selectionRecord(s, opts.Provider)},
		{Key: "mode", Value: modeRecord(mode, modeErr)},
		{Key: "roles", Value: roles},
		{Key: "diagnostics", Value: diagnostics(faults, modeErr, roleItems)},
	}
}

// layerRecords is the configuration directories the report read, in precedence
// order: each directory, the source that named it, and whether it and each of
// its files stand.
func layerRecords(layers []config.Layer) []toolresult.Record {
	records := make([]toolresult.Record, 0, len(layers))
	for _, l := range layers {
		records = append(records, toolresult.Record{
			{Key: "path", Value: l.Path},
			{Key: "source", Value: l.Source},
			{Key: "exists", Value: l.Exists},
			{Key: "providersExists", Value: l.ProvidersExists},
			{Key: "configExists", Value: l.ConfigExists},
		})
	}
	return records
}

// fileExists reports whether path names a file; config.Report's existence
// seam.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// providerRecords is the pool as the report lists it: one mapping per usable
// entry and per provider a dropping fault removed, in name order. A dropped
// provider has no entry to describe, so its mapping carries no base-url and
// no key.
func providerRecords(entries []config.Entry) []toolresult.Record {
	records := make([]toolresult.Record, 0, len(entries))
	for _, e := range entries {
		record := toolresult.Record{{Key: "name", Value: e.Name}}
		if e.BaseURL != "" {
			record = append(record, toolresult.Field{Key: "base-url", Value: e.BaseURL})
		}
		if e.DecisionsFlavor != "" {
			record = append(record, toolresult.Field{Key: "decisions-flavor", Value: e.DecisionsFlavor})
		}
		if e.Key != "" {
			record = append(record, toolresult.Field{Key: "key", Value: string(e.Key)})
		}
		if e.KeyFile != "" {
			record = append(record, toolresult.Field{Key: "key-file", Value: e.KeyFile})
		}
		if e.Layer != "" {
			record = append(record, toolresult.Field{Key: "layer", Value: e.Layer})
		}
		record = append(record, toolresult.Field{Key: "usable", Value: e.Usable})
		if e.Fault != "" {
			record = append(record, toolresult.Field{Key: "fault", Value: e.Fault})
		}
		records = append(records, record)
	}
	return records
}

// stringOrNull is a value the report states as given, or null where it is
// empty: config.toml leaving a role unset and a field having no source are
// both stated that way, and neither is the same as a value named "".
func stringOrNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// selectionRecord is which pool entry the selection rules named and which rule
// named it — or, where none was named, the selection's own reason, so an
// environment endpoint reads as an endpoint with no pool entry behind it
// rather than as a fourth rule. An environment-mode invocation may name an
// entry here and resolve no role to it: the selection is what the catalogue
// verbs would dial.
func selectionRecord(s config.Inputs, flag string) toolresult.Record {
	name, rule, err := config.Select(s, flag)
	if err != nil {
		return toolresult.Record{{Key: "detail", Value: err.Error()}}
	}
	return toolresult.Record{{Key: "provider", Value: name}, {Key: "rule", Value: rule}}
}

// modeRecord names which source supplies inference — `file` or `environment`
// — or null where the sources are mixed or the environment is partial, the
// reason being each role's own detail and the diagnostics' items.
func modeRecord(mode config.Mode, err error) any {
	if err != nil {
		return nil
	}
	return string(mode)
}

// resolvedRoles is one record per role, in role order: what answers it, the
// endpoint, the layer that supplied each field, and — where it does not
// resolve — why. A role nothing answers carries no back-end: nothing answered
// it. The key-file preflight is the registry's read, so the report learns of an
// unreadable key without opening a client or making a request.
func resolvedRoles(reg *clients, s config.Inputs, mode config.Mode, modeErr error) ([]toolresult.Record, []toolresult.Item) {
	records := make([]toolresult.Record, 0, len(config.Roles))
	var items []toolresult.Item
	// One preflight per key file: the file is the fact, however many roles
	// read it.
	preflighted := map[string]bool{}
	for _, role := range config.Roles {
		if modeErr != nil {
			records = append(records, toolresult.Record{
				{Key: "role", Value: string(role)},
				{Key: "detail", Value: "no role resolves while the inference mode is undecided: " + modeErr.Error()}})
			continue
		}
		ep, err := config.Resolve(s, mode, role, "")
		if err != nil {
			records = append(records, toolresult.Record{{Key: "role", Value: string(role)}, {Key: "detail", Value: err.Error()}})
			items = append(items, refusalItems(err)...)
			continue
		}
		record := roleRecord(ep, config.BackEndProvider)
		record = append(record, toolresult.Field{Key: "base-url", Value: ep.BaseURL},
			toolresult.Field{Key: "sources", Value: toolresult.Record{
				{Key: "base-url", Value: stringOrNull(ep.BaseURLSource)},
				{Key: "model", Value: stringOrNull(ep.ModelSource)},
				{Key: "key", Value: stringOrNull(ep.KeySource)}}})
		records = append(records, record)
		if ep.APIKeyFile != "" && !preflighted[ep.APIKeyFile] {
			preflighted[ep.APIKeyFile] = true
			if err := reg.preflight(ep); err != nil {
				items = append(items, refusalItems(err)...)
			}
		}
	}
	return records, items
}

// diagnostics is every fault the pool reports — one item per dropped provider
// and one per warning, each naming the providers.toml that carries it —
// followed by the mode's refusals where the sources are mixed or partial, and
// one per role that does not resolve. Each is the error that produced it:
// nothing here composes a sentence of its own.
func diagnostics(faults []config.ProviderFault, modeErr error, roleItems []toolresult.Item) []toolresult.Item {
	items := make([]toolresult.Item, 0, len(faults)+len(roleItems)+1)
	for _, f := range faults {
		items = append(items, toolresult.Item{Check: checkProvider, Path: f.Path, Detail: f.Reason})
	}
	if modeErr != nil {
		items = append(items, refusalItems(modeErr)...)
	}
	return append(items, roleItems...)
}

var showConfigFlagProvider string

var showConfigCmd = &cobra.Command{
	Use:   "show-config",
	Short: "report how the configuration resolves: the layers, the provider pool, the selection, the roles and the diagnostics",
	Long: `Report how this invocation's configuration resolves, as one YAML document
on stdout, and change nothing.

Configuration is read from layers: <repository>/` + config.RepositoryDirName + ` over
~/.config/kbase, or the one directory --config-dir or KBASE_CONFIG_DIR names,
which isolates the run from any repository's own. The document names every
layer read, in precedence order, with whether its directory and each of its
files stand; every pool entry the merge produced with the layer it came from,
the kind of key it supplies and why any faulted; the merged role values; the
names of the inference variables this process sets; which pool entry the
selection rules name and which rule named it; which source supplies inference
and, for each role, what answers it, the endpoint and the layer that supplied
each field — or what it lacks, with an item saying so.

Not every role resolves here and that is no refusal: an unresolved role is a
diagnostic, and so is a mixed or partial environment. Diagnostics are not
refusals at all — one call shows every gap at once, and the outcome is done.
Keys and variable values never appear: a key is reported by the kind it is
(file or none) and a key file by the path its entry states, never its content.

It reads no KB and makes no network request.`,
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
		opts.Provider = showConfigFlagProvider
		opts.Stderr = cmd.ErrOrStderr()
		code, err := runShowConfig(showConfigOptions{
			providerOptions: opts,
			Env:             config.EnvOverrides(os.Getenv),
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
	registerProviderFlag(showConfigCmd, &showConfigFlagProvider, "report")
	rootCmd.AddCommand(mcpBinding(readOnly(showConfigCmd), mcpBound))
}
