package main

import (
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
)

// showConfigOptions is the resolved input of the show-config verb: the layers
// this invocation read with what each holds, the merged pool state loaded out
// of them, and the stream the document goes to. The verb makes no request and
// opens no client, so providerOptions' NewClient stays unset.
type showConfigOptions struct {
	providerOptions

	Stdout io.Writer
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
// the merged role values, the selection, one record per role, and the
// diagnostics. It carries no key material.
func showConfigFields(opts showConfigOptions) []toolresult.Field {
	// LoadProviders walks a map, so its faults arrive in no fixed order; a
	// report sorts them, so the same configuration always reads the same way.
	// The sort is stable: one name's faults keep the order Load gives them.
	faults := slices.Clone(opts.Faults)
	slices.SortStableFunc(faults, func(a, b config.ProviderFault) int {
		return strings.Compare(a.Name, b.Name)
	})
	facts := config.Report(opts.Config, opts.Providers, faults, opts.Layers, fileExists)
	s := opts.inputs()
	roles, roleItems := resolvedRoles(s)
	// The configuration's own diagnostics, then one per role that does not
	// resolve; an empty list where there is neither.
	diagnostics := append(append(make([]toolresult.Item, 0), facts.Diagnostics...), roleItems...)
	return []toolresult.Field{
		{Key: "layers", Value: layerRecords(facts.Layers)},
		{Key: "providers", Value: providerRecords(facts.Entries)},
		{Key: "config", Value: toolresult.Record{
			{Key: "models", Value: toolresult.Record{
				{Key: string(config.RoleDecision), Value: stringOrNull(facts.Models.Decision)},
				{Key: string(config.RoleText), Value: stringOrNull(facts.Models.Text)},
				{Key: string(config.RoleScoring), Value: stringOrNull(facts.Models.Scoring)},
			}},
			{Key: "sampling", Value: toolresult.Record{
				{Key: string(config.RoleText), Value: samplingRecord(facts.Sampling.Text)},
				{Key: string(config.RoleScoring), Value: samplingRecord(facts.Sampling.Scoring)},
			}},
		}},
		{Key: "selection", Value: selectionRecord(s, opts.Provider)},
		{Key: "roles", Value: roles},
		{Key: "diagnostics", Value: diagnostics},
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
			{Key: "providers-exists", Value: l.ProvidersExists},
			{Key: "config-exists", Value: l.ConfigExists},
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
		if e.Protocol != "" {
			record = append(record, toolresult.Field{Key: "api", Value: e.Protocol})
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
		if e.Upgrade != "" {
			record = append(record, toolresult.Field{Key: "upgrade", Value: e.Upgrade})
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

// temperatureOrNull is a temperature the report states as given, or null where
// no layer states one.
func temperatureOrNull(t *float64) any {
	if t == nil {
		return nil
	}
	return *t
}

// samplingRecord is one chat role's sampling as config.toml states it, null
// where unset.
func samplingRecord(s config.RoleSampling) toolresult.Record {
	return toolresult.Record{
		{Key: "temperature", Value: temperatureOrNull(s.Temperature)},
		{Key: "thinking", Value: stringOrNull(s.Thinking)},
	}
}

// selectionRecord is which pool entry the selection rules named and which rule
// named it — or, where none was named, the selection's own reason. The
// selection is what the catalogue verbs would dial, and names no role.
func selectionRecord(s config.Inputs, flag string) toolresult.Record {
	name, rule, err := config.Select(s, flag)
	if err != nil {
		return toolresult.Record{{Key: "detail", Value: err.Error()}}
	}
	return toolresult.Record{{Key: "provider", Value: name}, {Key: "rule", Value: rule}}
}

// resolvedRoles is one record per role, in role order: what answers it, the
// endpoint, the layer that supplied each field, and — where it does not
// resolve — why.
func resolvedRoles(s config.Inputs) ([]toolresult.Record, []toolresult.Item) {
	records := make([]toolresult.Record, 0, len(config.Roles))
	var items []toolresult.Item
	for _, role := range config.Roles {
		ep, err := config.Resolve(s, role, "")
		if err != nil {
			records = append(records, toolresult.Record{{Key: "role", Value: string(role)}, {Key: "detail", Value: err.Error()}})
			var refusal toolresult.Refusal
			if errors.As(err, &refusal) {
				items = append(items, refusal...)
			}
			continue
		}
		sources := toolresult.Record{
			{Key: "base-url", Value: stringOrNull(ep.BaseURLSource)},
			{Key: "model", Value: stringOrNull(ep.ModelSource)},
			{Key: "key", Value: stringOrNull(ep.KeySource)}}
		var sampling []toolresult.Field
		if ep.Role != config.RoleDecision {
			sources = append(sources,
				toolresult.Field{Key: "temperature", Value: stringOrNull(ep.TemperatureSource)},
				toolresult.Field{Key: "thinking", Value: stringOrNull(ep.ThinkingSource)})
			sampling = []toolresult.Field{
				{Key: "temperature", Value: temperatureOrNull(ep.Temperature)},
				{Key: "thinking", Value: stringOrNull(ep.Thinking)}}
		}
		record := roleRecord(ep)
		record = append(record, toolresult.Field{Key: "base-url", Value: ep.BaseURL},
			toolresult.Field{Key: "sources", Value: sources})
		record = append(record, sampling...)
		records = append(records, record)
	}
	return records, items
}

var showConfigFlagProvider string

var showConfigCmd = mcp.Notes(&cobra.Command{
	Use:   "show-config",
	Short: "report how the configuration resolves: the layers, the providers, the selection, the roles and the diagnostics",
	Long: `Report how this invocation's configuration resolves, as its result, and
change nothing.

Configuration is read from layers: <repository>/` + config.RepositoryDirName + ` over
~/.config/kbase; outside a worktree there is no repository layer. Where
--config-dir or KBASE_CONFIG_DIR names a directory, that one directory is read
alone. A provider name is read in lowercase, whatever case providers.toml
spells it in, and a higher layer's table for a provider replaces a lower
layer's table of the same name whole, while a config.toml value overrides only
the key it sets. The document names every layer read, in precedence order,
with whether its directory and each of its files stand; every provider the
merge produced with the layer it came from, the chat protocol and decision
surface it speaks, the kind of key it supplies and why any faulted; the merged
role values; which provider the selection rules name and which rule named it;
and, for each role, what answers it, the endpoint and the layer that supplied
each field — or what it lacks, with an item saying so.

Not every role resolves here and that is no refusal: an unresolved role is a
diagnostic. Diagnostics are not refusals at all — one call shows every gap at
once, and the outcome is done.

Keys never appear. A key is reported by the kind it is: store, kept by kbase
under the provider's name; file, read from the key file the provider's table
states, which is reported by its path; or none. A stored key outranks a key
file. A provider whose key comes from a file carries upgrade, the command that
moves the key into the store; a provider with both has a diagnostic saying its
key file is not read. A file in the store that does not open is a diagnostic,
and the provider falls back to the key file its table states, else to none.

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
}, configureMCPNote, "")

func init() {
	registerProviderFlag(showConfigCmd, &showConfigFlagProvider, "report")
}
