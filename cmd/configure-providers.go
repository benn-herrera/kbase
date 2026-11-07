package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

// configureProvidersOptions is the resolved input of configure-providers: the
// shared provider-reaching options, the provider its arguments name, where it
// is written, the credentials store the written provider is reloaded over,
// and the stream its result goes to.
type configureProvidersOptions struct {
	providerOptions

	Name        string
	BaseURL     string
	API         string
	DecisionAPI string
	Remove      bool
	// Layer is the --layer flag, "" where it names none.
	Layer string

	// WorkDir is where the repository search started; the worktree refusal
	// names it.
	WorkDir string

	// StoredKeys is the credentials store as read: its keys, by the lowercase
	// name each is stored under, and each key file that did not open.
	StoredKeys config.StoredKeys

	Stdout io.Writer
	Logger log.Logger
}

func runConfigureProviders(ctx context.Context, opts configureProvidersOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "configure-providers"); err != nil {
		return 0, err
	}
	outcome, fields := configureProviders(ctx, opts)
	return emitResult("configure-providers", opts.Stdout, outcome, fields)
}

// configureProviders writes or removes the provider and, where the written
// provider has a key, dials it for its catalogue.
func configureProviders(ctx context.Context, opts configureProvidersOptions) (string, []toolresult.Field) {
	name := strings.ToLower(strings.TrimSpace(opts.Name))
	if items := providerArgumentRefusals(opts); items != nil {
		return refused(nil, items...)
	}
	if refusal := config.ProviderEntryRefusals(name, opts.BaseURL, opts.API, opts.DecisionAPI); refusal != nil {
		return refused(nil, refusal...)
	}
	dir, layer, items, err := configureLocation(opts.providerOptions, opts.Layer, opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if items != nil {
		return refused(nil, items...)
	}

	path := config.ProvidersPath(dir)
	fields := []toolresult.Field{{Key: "provider", Value: name}, {Key: "layer", Value: layer}}
	outcome, written, act := toolresult.Unchanged, []string{}, "written"
	if opts.Remove {
		removed, err := config.RemoveProvider(path, name)
		if err != nil {
			return failed(fields, err)
		}
		if !removed {
			return refused(nil, config.NoProvider(name, path)...)
		}
		outcome, written, act = toolresult.Done, []string{path}, "removed"
	} else {
		wrote, err := config.SetProvider(path, config.Provider{Name: name, BaseURL: opts.BaseURL, API: opts.API, DecisionAPI: opts.DecisionAPI})
		if err != nil {
			return failed(fields, err)
		}
		if wrote {
			outcome, written = toolresult.Done, []string{path}
		}
	}
	fields = append(fields, toolresult.Field{Key: "written", Value: written})

	stack, err := loadProvidersStack(dir, layer, opts.StoredKeys)
	if err != nil {
		return outcome, toolresult.WithWarnings(append(fields, toolresult.Field{Key: "next", Value: []string{fmt.Sprintf(
			"the provider is %s, but the configuration did not reload: %v; %s", act, err, argmark.Call("show-config", nil))}}),
			warningsField(nil))
	}
	warnings := warningsField(warnFaults(opts.Stderr, stack.Faults))
	if opts.Remove {
		if next := removalNext(opts, stack, name); next != nil {
			fields = append(fields, toolresult.Field{Key: "next", Value: next})
		}
		return outcome, toolresult.WithWarnings(fields, warnings)
	}
	return outcome, toolresult.WithWarnings(append(fields, dialWrittenProvider(ctx, opts, stack, name)...), warnings)
}

// loadProvidersStack loads, over stored, the configuration configure-providers
// writes into: the layer at dir, which source names, and the layers beneath it.
func loadProvidersStack(dir, source string, stored config.StoredKeys) (config.Loaded, error) {
	layers, err := config.WrittenStack(config.Layer{Path: dir, Source: source})
	if err != nil {
		return config.Loaded{}, err
	}
	return config.Load(layers, stored)
}

// providerArgumentRefusals refuses the arguments that do not go together:
// --remove beside a provider field, and no --base-url without it.
func providerArgumentRefusals(opts configureProvidersOptions) []toolresult.Item {
	if !opts.Remove {
		if opts.BaseURL == "" {
			return []toolresult.Item{{Check: toolresult.CheckUsage, Key: argmark.Flag("base-url"),
				Detail: fmt.Sprintf("%s is required to write a provider; %s removes one", argmark.Flag("base-url"), argmark.Flag("remove"))}}
		}
		return nil
	}
	var items []toolresult.Item
	for _, f := range []struct{ flag, value string }{
		{"base-url", opts.BaseURL}, {"api", opts.API}, {"decision-api", opts.DecisionAPI},
	} {
		if f.value != "" {
			items = append(items, toolresult.Item{Check: toolresult.CheckUsage, Key: argmark.Flag(f.flag),
				Detail: fmt.Sprintf("%s removes the provider, so %s sets nothing; pass one or the other", argmark.Flag("remove"), argmark.Flag(f.flag))})
		}
	}
	return items
}

// removalNext is what still names the provider name after its removal, where
// stack, the configuration written into as reloaded, no longer holds it: the
// configure-models call that replaces each role's reference to it, then the
// call that clears a key file stored under its name, whether or not it opens.
// It is nil where nothing names it.
func removalNext(opts configureProvidersOptions, stack config.Loaded, name string) []string {
	if _, held := stack.Providers[name]; held {
		return nil
	}
	var next []string
	for _, rr := range config.RolesNaming(stack.Cfg, name) {
		args := map[string]any{string(rr.Role): "<provider>/<model>"}
		if opts.Layer != "" {
			args["layer"] = opts.Layer
		}
		next = append(next, fmt.Sprintf("the %s role's reference %s names the removed provider; %s replaces it",
			rr.Role, rr.Reference, argmark.Call("configure-models", args)))
	}
	if _, stored := opts.StoredKeys.Keys[name]; stored {
		next = append(next, fmt.Sprintf("a key is stored for %s; %s (command line only) clears it", name, config.AuthRemoveCommand(name)))
	} else if slices.ContainsFunc(opts.StoredKeys.Faults, func(f config.StoreFault) bool { return f.Name == name }) {
		next = append(next, fmt.Sprintf("a key is stored for %s but does not open; %s (command line only) clears it", name, config.AuthRemoveCommand(name)))
	}
	return next
}

// dialWrittenProvider is the result's report of the written provider in stack,
// the configuration written into as reloaded: its catalogue, sorted, under
// models where it has a key and the dial answers; otherwise next, the steps
// that follow — to store a key and list the models; where the dial failed,
// the failure and the step after it; or, where the provider does not load, the
// remedy for its fault, which warnFaults has already reported.
func dialWrittenProvider(ctx context.Context, opts configureProvidersOptions, stack config.Loaded, name string) []toolresult.Field {
	modelsCall := argmark.Call("models", map[string]any{"provider": name})
	for _, fault := range stack.Faults {
		if fault.Name == name && !fault.Warning {
			return []toolresult.Field{{Key: "next", Value: []string{
				fmt.Sprintf("the provider is written, but does not load (%s); %s", fault.Reason, argmark.Call("show-config", nil))}}}
		}
	}
	entry := stack.Providers[name]
	if entry.APIKey.IsZero() {
		fmt.Fprintf(opts.Stderr, "configure-providers: no key is stored for %s; store one, then list its models\n", name)
		return []toolresult.Field{{Key: "next", Value: []string{config.AuthAddRemedy(name), modelsCall}}}
	}

	reg := modelclient.New(opts.NewClient, opts.Logger, "", 0)
	defer reg.CloseAll()
	callCtx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()
	start := time.Now()
	ids, err := reg.Catalogue(callCtx, entry)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "configure-providers: dialling %s failed: %v\n", name, err)
		return []toolresult.Field{{Key: "next", Value: []string{
			fmt.Sprintf("dialling %s failed: %v; the provider is written. A wrong URL: %s; a wrong key: %s (command line only); then %s",
				name, err, argmark.Call("configure-providers", map[string]any{"provider": name, "base-url": "<url>"}),
				config.AuthAddRemedy(name), modelsCall)}}}
	}
	slices.Sort(ids)
	fmt.Fprintf(opts.Stderr, "configure-providers ok: %s count=%d elapsed=%s\n", name, len(ids), time.Since(start).Round(time.Millisecond))
	return []toolresult.Field{{Key: "models", Value: nonNil(ids)}}
}

// configureLocationHelp is where a configure command writes, as both
// commands' help states it.
var configureLocationHelp = `It writes into the directory --config-dir or KBASE_CONFIG_DIR names, else
~/.config/kbase. ` + argmark.FlagValue("layer", config.LayerSourceRepository) + ` writes into <repository>/` + config.RepositoryDirName + ` instead (check: worktree;
remedy: git init); ` + argmark.FlagValue("layer", config.LayerSourceGlobal) + ` writes into ~/.config/kbase even where
KBASE_CONFIG_DIR is set. ` + argmark.Flag("layer") + ` with --config-dir is refused, and so is a layer
other than those two.`

// configureMCPNote is both configure commands' and show-config's MCP note: the
// location the server's own launch fixes.
const configureMCPNote = `--config-dir and KBASE_CONFIG_DIR are set where the server is launched; a
call cannot change them.`

var (
	configureProvidersFlagBaseURL     string
	configureProvidersFlagAPI         string
	configureProvidersFlagDecisionAPI string
	configureProvidersFlagRemove      bool
	configureProvidersFlagLayer       string
)

func configureProvidersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure-providers <provider>",
		Short: "add, edit or remove a provider in providers.toml",
		Long: `Add or edit the provider ` + argmark.Positional("provider") + ` in providers.toml, or remove it with
` + argmark.Flag("remove") + `, creating providers.toml where it is absent. A provider name,
` + config.ProviderNameChars + `,
is folded to lowercase and matches a provider whatever its case.

` + argmark.Flag("base-url") + ` sets the provider's base URL, an absolute http or https URL carrying
no credentials (a key is stored with kbase auth add). ` + argmark.Flag("api") + ` and
` + argmark.Flag("decision-api") + ` set its chat protocol and its decision API where given. Every
other field of the provider's table and every other byte of the file stand; a
provider already holding the values given is unchanged. ` + argmark.Flag("remove") + ` removes the
provider's table, and a provider with no table in the file is refused (check:
provider; remedy: ` + argmark.Call("show-config", nil) + `).

No API key passes through this command, and no argument names a key file.
Where the provider has a key — stored with kbase auth add, or read from a key
file its table names — the command dials the provider and gives its catalogue
as models, which vets the connection. Where it has none, next gives the steps
that follow: ` + config.AuthAddRemedy("<name>") + `, a command-line command
(` + config.KeyStoreNote + `), then
` + argmark.Call("models", map[string]any{"provider": "<name>"}) + `. A dial that fails still leaves the provider written,
outcome done. Where the dial fails, the written provider does not load, or a
removal leaves a role or a stored key naming a provider that no layer this call
writes into or reads beneath still holds, next gives the steps that follow.

` + configureLocationHelp + ` The result gives "provider"; "layer" (global or
repository, or explicit or environment where --config-dir or KBASE_CONFIG_DIR
chose it); "written", the files written; "models"
and "next" as above; and "warnings": one item per configuration fault in the
files this call writes into or reads beneath, as ` + argmark.Call("models", nil) + ` reports them, "path"
the file or directory it is against, "detail" what is wrong. The written
provider's own fault is both a "warnings" item and, with its remedy, a "next"
step. "warnings" is empty where there are none, and where the configuration
does not reload after the write ("next" then says so). It is carried on every
outcome from the write on; a refusal or a write that fails carries none.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			opts, err := resolveVerbLayers(wd)
			if err != nil {
				return err
			}
			storeDir, err := authStoreDir()
			if err != nil {
				return err
			}
			code, err := runConfigureProviders(cmd.Context(), configureProvidersOptions{
				providerOptions: opts,
				Name:            args[0],
				BaseURL:         configureProvidersFlagBaseURL,
				API:             configureProvidersFlagAPI,
				DecisionAPI:     configureProvidersFlagDecisionAPI,
				Remove:          configureProvidersFlagRemove,
				Layer:           configureProvidersFlagLayer,
				WorkDir:         wd,
				StoredKeys:      readStoredKeys(storeDir),
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
	cmd.Flags().StringVar(&configureProvidersFlagBaseURL, "base-url", "",
		"the provider's base URL, an absolute http or https URL; required unless "+argmark.Flag("remove"))
	mcp.Brief(cmd, "base-url", "the provider's base URL; required unless "+argmark.Flag("remove"))
	cmd.Flags().StringVar(&configureProvidersFlagAPI, "api", "",
		"the provider's chat protocol, "+strings.Join(config.ProviderAPIs, " or ")+" (unset: "+config.ProviderAPIAnthropic+")")
	mcp.EnumFlag(cmd.Flags().Lookup("api"), config.ProviderAPIs...)
	mcp.Brief(cmd, "api", "the provider's chat protocol")
	cmd.Flags().StringVar(&configureProvidersFlagDecisionAPI, "decision-api", "",
		"the decision API the provider's server speaks, "+strings.Join(config.DecisionAPIs, " or ")+" (unset: "+config.DecisionAPISystemOne+")")
	mcp.EnumFlag(cmd.Flags().Lookup("decision-api"), config.DecisionAPIs...)
	mcp.Brief(cmd, "decision-api", "the provider's decision API")
	cmd.Flags().BoolVar(&configureProvidersFlagRemove, "remove", false, "remove the provider instead of writing it")
	mcp.Brief(cmd, "remove", "remove the provider")
	registerLayerFlag(cmd, &configureProvidersFlagLayer)
	mcp.DescribePositional(cmd, "provider", "the provider to write or remove, by its name")
	return mcp.Notes(mcp.Brief(cmd, "provider", "the provider's name"), configureMCPNote, "")
}
