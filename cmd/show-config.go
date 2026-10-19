package main

import (
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	toolresult "kbase/internal/result"
)

// showConfigOptions is the resolved input of the show-config verb: the layers
// this invocation read with what each holds, the merged pool state loaded out
// of them, the environment's layer over the files, and the stream the document
// goes to. The verb makes no request, so providerOptions' Timeout and
// NewClient stay unset.
type showConfigOptions struct {
	providerOptions

	// Env is the environment's layer over the two files.
	Env config.Overrides

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
// the merged provider and categories, the inference variable names the process
// supplies, the selection, the resolved endpoint and each resolved field's
// source, and the diagnostics. It carries no key material and no variable
// value.
func showConfigFields(opts showConfigOptions) []toolresult.Field {
	// LoadProviders walks a map, so its faults arrive in no fixed order; a
	// report sorts them, so the same configuration always reads the same way.
	faults := slices.Clone(opts.Faults)
	slices.SortFunc(faults, func(a, b config.ProviderFault) int {
		return strings.Compare(a.Name, b.Name)
	})
	facts := config.Report(opts.Config, opts.Providers, faults, opts.Layers, fileExists, os.Getenv)
	r := resolveProvider(opts.providerOptions, opts.Env)
	return []toolresult.Field{
		{Key: "layers", Value: layerRecords(facts.Layers)},
		{Key: "providers", Value: providerRecords(facts.Entries)},
		{Key: "config", Value: toolresult.Record{
			{Key: "provider", Value: facts.Provider},
			{Key: "models", Value: toolresult.Record{
				{Key: config.CategoryDecision, Value: stringOrNull(facts.Models.Decision)},
				{Key: config.CategoryText, Value: stringOrNull(facts.Models.Text)},
			}},
		}},
		{Key: "environment", Value: facts.Environment},
		{Key: "selection", Value: selectionRecord(r)},
		{Key: "inference", Value: inferenceRecord(r)},
		{Key: "diagnostics", Value: diagnostics(faults, r)},
	}
}

// layerRecords is the configuration directories the report read, in precedence
// order: each directory, the source that named it, and whether it and each of
// its two files stand.
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
// empty: config.toml leaving a category unset and a field having no source are
// both stated that way, and neither is the same as a value named "".
func stringOrNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// selectionRecord is which pool entry the selection rules named and which rule
// named it — or, where none was named, the selection's own error, so a base
// URL the environment supplies reads as an endpoint with no pool entry behind
// it rather than as a fourth rule.
func selectionRecord(r resolution) toolresult.Record {
	if r.Provider == "" {
		return toolresult.Record{{Key: "detail", Value: r.Unselected.Error()}}
	}
	return toolresult.Record{{Key: "provider", Value: r.Provider}, {Key: "rule", Value: string(r.Rule)}}
}

// inferenceRecord is the endpoint inference resolves to, or null where it does
// not resolve. It never carries a key, and each field's source is null where
// the field itself is unset.
func inferenceRecord(r resolution) any {
	if r.Err != nil {
		return nil
	}
	return toolresult.Record{
		{Key: "name", Value: r.Inference.Name},
		{Key: "base-url", Value: r.Inference.BaseURL},
		{Key: "decision", Value: r.Inference.Decision},
		{Key: "text", Value: r.Inference.Text},
		{Key: "sources", Value: toolresult.Record{
			{Key: "base-url", Value: stringOrNull(r.Inference.Sources.BaseURL)},
			{Key: "decision", Value: stringOrNull(r.Inference.Sources.Decision)},
			{Key: "text", Value: stringOrNull(r.Inference.Sources.Text)},
			{Key: "key", Value: stringOrNull(r.Inference.Sources.Key)},
		}},
	}
}

// diagnostics is every fault the pool reports — one item per dropped provider
// and one per warning, each naming the providers.toml that carries it —
// followed by the resolution failure's own item where inference does not
// resolve. Each is the error that produced it: nothing here composes a sentence
// of its own.
func diagnostics(faults []config.ProviderFault, r resolution) []toolresult.Item {
	items := make([]toolresult.Item, 0, len(faults)+1)
	for _, f := range faults {
		items = append(items, toolresult.Item{Check: checkProvider, Path: f.Path, Detail: f.Reason})
	}
	if r.Err != nil {
		item := toolresult.Item{Check: checkProvider, Detail: r.Err.Error()}
		var lack config.LackError
		if errors.As(r.Err, &lack) {
			item.Key, item.Path = lack.Named()
		}
		items = append(items, item)
	}
	return items
}

var showConfigFlagProvider string

var showConfigCmd = &cobra.Command{
	Use:   "show-config",
	Short: "report how the configuration resolves: the layers, the provider pool, the selection and the diagnostics",
	Long: `Report how this invocation's configuration resolves, as one YAML document
on stdout, and change nothing.

Configuration is read from layers: <repository>/` + config.RepositoryDirName + ` over
~/.config/kbase, or the one directory --config-dir or KBASE_CONFIG_DIR names,
which isolates the run from any repository's own. The document names every
layer read, in precedence order, with whether its directory and each of its two
files stand; every pool entry the merge produced with the layer it came from,
the kind of key it supplies and why any faulted; the merged provider and
category ids; the names of the inference variables this process sets; which
pool entry the selection rules name and which rule named it; and the endpoint
and models inference resolves to with the layer that supplied each — or null,
with an item saying what the configuration lacks.

Diagnostics are not refusals: a faulted pool entry and an unresolved
configuration both leave the outcome done, so one call shows every gap at once.
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
		code, err := runShowConfig(showConfigOptions{
			providerOptions: opts,
			Env:             config.EnvOverrides(os.Getenv),
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
}

func init() {
	registerProviderFlag(showConfigCmd, &showConfigFlagProvider, "report")
	rootCmd.AddCommand(mcpBinding(readOnly(showConfigCmd), mcpBound))
}
