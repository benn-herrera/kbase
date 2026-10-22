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
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

const (
	// modelMapPairSep separates category assignments within one --model-map
	// value; modelMapAssign separates a category from its reference.
	modelMapPairSep = ","
	modelMapAssign  = "="

	// keyModelMap is the flag a reference the pool cannot resolve belongs to.
	keyModelMap = "--model-map"

	// modelMapSyntax is the flag spelling quoted back to the user in every
	// failure a manual assignment would resolve. A value is a reference, the
	// form config.toml's [models] values take.
	modelMapSyntax = keyModelMap + " " + config.CategoryDecision + "=<entry>/<id>" + modelMapPairSep +
		config.CategoryText + "=<entry>/<id>" + modelMapPairSep + config.CategoryScoring + "=<entry>/<id>"
)

// modelCategories is the order the model categories are resolved, reported, and
// written in: the roles' order, so the file, the verb and every document that
// reports a role state the same set in the same order.
var modelCategories = roleCategories()

// roleCategories is config.Roles as the [models] table spells each role.
func roleCategories() []string {
	out := make([]string, len(config.Roles))
	for i, role := range config.Roles {
		out[i] = string(role)
	}
	return out
}

// configureOptions is the resolved input of the configure verb: the shared
// provider-reaching options plus this verb's own flags and the stream its
// result goes to.
//
// The write target is the embedded WriteDir — the layer this invocation
// resolved, which --project may replace with the repository's. The embedded
// Config's [models] table is deliberately not consulted on the dial path:
// re-running configure is how a stale model map is replaced. The one value a
// dialed run reads from the target's file is the scoring reference it may
// leave standing, and it is read from that file rather than from the merge.
type configureOptions struct {
	providerOptions

	// ModelMap holds the raw --model-map values, one per occurrence of
	// the flag, each possibly comma-separated.
	ModelMap []string

	// Create creates the target layer's providers.toml and config.toml
	// where they are absent, and writes only the values the flags name.
	Create bool

	// Project names <repository>/.kbase as the write target, whether or not
	// it exists.
	Project bool

	// WorkDir is where the repository search started; the worktree refusal
	// names it.
	WorkDir string

	Stdout io.Writer

	// Logger takes the registry's close failures.
	Logger log.Logger
}

// runConfigure resolves a provider, discovers its model catalogue, assigns a
// reference to each model category, writes the result to config.toml, writes
// its result document and returns the exit code the outcome maps to.
//
// Every category gets a reference from --model-map or, for scoring alone, from
// the target file's own value: a category with neither is refused, listing what
// the provider offered, with nothing written — config.toml is what every later
// stage draws its models from, and nothing between the flags and the file picks
// a model on the user's behalf. --create with no --provider is the one
// exception, and it dials nothing to refuse against: it writes what the flags
// name, removes the superseded provider key, and says the references are
// unchecked.
func runConfigure(ctx context.Context, opts configureOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "configure"); err != nil {
		return 0, err
	}
	outcome, fields := configure(ctx, opts)
	return emitResult("configure", opts.Stdout, outcome, fields)
}

func configure(ctx context.Context, opts configureOptions) (string, []toolresult.Field) {
	// Flag syntax is checked before anything is dialed: a typo in
	// --model-map should not cost a network round trip to discover.
	manual, err := parseModelMap(opts.ModelMap)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	target, items, err := configureTarget(opts)
	if err != nil {
		return failed(nil, err)
	}
	if items != nil {
		return refused(nil, items...)
	}
	// The target layer is what the shared provider refusals name, so a
	// missing pool entry points at the file this run would write.
	opts.WriteDir = target

	if opts.Create && strings.TrimSpace(opts.Provider) == "" {
		return configureOffline(opts, manual)
	}
	return configureDialed(ctx, opts, manual)
}

// configureTarget resolves the directory this run writes into: the layer the
// invocation resolved, unless --project names the repository's — which is the
// target whether or not it stands. --project outside a worktree is refused,
// and so is --project beside --config-dir: the two name different
// directories, and honouring one would silently discard the other.
func configureTarget(opts configureOptions) (string, []toolresult.Item, error) {
	if !opts.Project {
		return opts.WriteDir, nil, nil
	}
	if len(opts.Layers) > 0 && opts.Layers[0].Source == config.LayerSourceExplicit {
		return "", []toolresult.Item{{Check: checkUsage, Key: "--project",
			Detail: "--project and --config-dir name different configuration directories; pass one"}}, nil
	}
	if opts.RepoRoot == "" {
		return "", []toolresult.Item{{Check: checkWorktree, Path: opts.WorkDir, Remedy: "git init",
			Detail: fmt.Sprintf("%s is not inside a git worktree; `git init` there is the operator's to run", opts.WorkDir)}}, nil
	}
	return config.WriteTarget(opts.Layers, opts.RepoRoot), nil, nil
}

// configureProviderFields is the `provider` field a configure run states: the
// entry it configured, with the rule and layer that named it, or null where
// the configuration names none — rule and layer absent then, as SPEC §7 has
// them.
func configureProviderFields(name, rule, layer string) []toolresult.Field {
	if fields := providerFields(name, rule, layer); fields != nil {
		return fields
	}
	return []toolresult.Field{{Key: "provider", Value: nil}}
}

// configureDialed is the ordinary path: every reference --model-map names is
// checked against its own provider's catalogue and the three references are
// written; an unassigned category is offered the selected provider's
// catalogue. A refusal or a failed listing writes nothing.
func configureDialed(ctx context.Context, opts configureOptions, manual map[string]config.Reference) (string, []toolresult.Field) {
	env := config.EnvOverrides(os.Getenv)
	warnFaults(opts.Stderr, opts.Faults)
	supersededProvider(opts.Stderr, opts.Config)
	s := opts.inputs(env)

	// The one value this run may leave standing: the target file's own
	// scoring reference, where --model-map names none. It is the file's, not
	// the merge's — a lower layer's value is not this file's to keep.
	fileCfg, err := config.LoadConfig(config.ConfigPath(opts.WriteDir))
	if err != nil {
		return failed(nil, err)
	}
	scoring, err := keepScoring(manual, config.ConfigPath(opts.WriteDir), fileCfg.Models.Scoring)
	if err != nil {
		return refusedOrFailed(nil, err)
	}

	name, rule, err := config.Select(s, opts.Provider)
	if err != nil {
		if opts.Provider != "" || len(manual) == 0 {
			return refusedOrFailed(nil, err)
		}
		// The references name their own providers, so the pool's size is no
		// reason to demand a selection no reference needs.
		name, rule = "", ""
	}

	// Every distinct provider the references name, the selected entry first:
	// one catalogue is what an unassigned category is offered, and each
	// reference's own is what its model part is warned against.
	targets := make([]string, 0, len(modelCategories)+1)
	addTarget := func(provider string) {
		if provider != "" && !slices.Contains(targets, provider) {
			targets = append(targets, provider)
		}
	}
	addTarget(name)
	for _, category := range modelCategories {
		if ref, given := manual[category]; given {
			addTarget(ref.Provider)
		}
	}

	reg := newClients(opts.NewClient, opts.Logger, "")
	defer reg.closeAll()
	fields := configureProviderFields(name, rule, s.Providers[name].Layer)
	catalogues := make(map[string][]string, len(targets))
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	for _, target := range targets {
		entry, ok := s.Providers[target]
		if !ok {
			return refusedOrFailed(fields, missingReferenceProvider(target, s))
		}
		client, err := reg.get(entryEndpoint(entry))
		if err != nil {
			return refusedOrFailed(fields, err)
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		ids, err := catalogueIDs(callCtx, client)
		cancel()
		if err != nil {
			return failed(fields, err)
		}
		catalogues[target] = ids
	}

	// A reference whose model part its provider does not list is a warning,
	// not a failure: catalogues rotate, and a user pinning a model the listing
	// omits may know something the listing does not.
	for _, category := range modelCategories {
		ref, given := manual[category]
		if given && !slices.Contains(catalogues[ref.Provider], ref.Model) {
			fmt.Fprintf(opts.Stderr,
				"configure: %s category model %q is not in provider %q's catalogue — configuring it anyway\n",
				category, ref.Model, ref.Provider)
		}
	}

	// A category with nothing to write is refused, one item per missing
	// category; the catalogue listing is the models to choose from.
	offered := name
	if offered == "" {
		offered = targets[0]
	}
	var unassigned []toolresult.Item
	for _, category := range modelCategories {
		if _, given := manual[category]; given {
			continue
		}
		if category == config.CategoryScoring && scoring != nil {
			continue
		}
		unassigned = append(unassigned, unassignedCategoryItem(category, offered, catalogues[offered]))
	}
	if unassigned != nil {
		return refused(fields, unassigned...)
	}

	models := config.ModelMap{
		Decision: manual[config.CategoryDecision].String(),
		Text:     manual[config.CategoryText].String(),
	}
	if scoring != nil {
		models.Scoring = scoring.String()
	}
	return writeConfig(opts, config.Config{Models: models}, fields, true)
}

// keepScoring is the scoring reference a dialed run writes: the flag's where
// it names one, the target file's own where the file holds one, and none where
// it holds none — which the run then refuses, naming the key. A file's bare id
// is refused rather than kept: writing it back would produce a file kbase
// score refuses.
func keepScoring(manual map[string]config.Reference, path, fileValue string) (*config.Reference, error) {
	if ref, given := manual[config.CategoryScoring]; given {
		return &ref, nil
	}
	if fileValue == "" {
		return nil, nil
	}
	ref, err := config.FileReference(fileValue)
	if err != nil {
		return nil, config.Refusal{Key: categoryKey(config.CategoryScoring), Path: path,
			Remedy: "kbase configure " + keyModelMap + " " + config.CategoryScoring + "=<entry>/<id>",
			Detail: fmt.Sprintf("%s names %q, and a model is named <provider>/<model> — the entry in %s that supplies the endpoint, then the model it serves",
				categoryKey(config.CategoryScoring), fileValue, config.ProvidersFileName)}
	}
	return &ref, nil
}

// missingReferenceProvider is a --model-map reference whose provider the pool
// does not hold: the flag is what named it, so the flag is what the refusal
// names, and the pool's entries are the names that would work. The pool's own
// refusal is reused, which keeps a faulted provider reading as faulted rather
// than as a typo.
func missingReferenceProvider(provider string, in config.Inputs) config.Refusal {
	_, _, err := config.Select(in, provider)
	var refusal config.Refusal
	if !errors.As(err, &refusal) {
		return config.Refusal{Check: config.CheckUsage, Key: keyModelMap,
			Detail: fmt.Sprintf("%s names provider %q, which is not in %s", keyModelMap, provider, config.ProvidersFileName)}
	}
	refusal.Key = keyModelMap
	refusal.Detail = fmt.Sprintf("%s names a provider the pool does not hold: %s", keyModelMap, refusal.Detail)
	return refusal
}

// configureOffline writes what the flags name and nothing else: no provider was
// named, so no request is made and no reference is checked against a catalogue.
// What the target layer's config.toml already holds — an untouched category, a
// comment — survives, because configure is an editor and --create is only
// permission to create the file it edits. The superseded provider key goes
// either way: the writer removes it.
func configureOffline(opts configureOptions, manual map[string]config.Reference) (string, []toolresult.Field) {
	cfg, err := config.LoadConfig(config.ConfigPath(opts.WriteDir))
	if err != nil {
		return failed(nil, err)
	}
	for _, category := range modelCategories {
		if ref, given := manual[category]; given {
			switch category {
			case config.CategoryDecision:
				cfg.Models.Decision = ref.String()
			case config.CategoryText:
				cfg.Models.Text = ref.String()
			case config.CategoryScoring:
				cfg.Models.Scoring = ref.String()
			}
		}
	}
	fmt.Fprintf(opts.Stderr, "configure: no provider named, so the references were not checked\n")
	// No entry was configured, and the superseded key this run removes is not
	// one: the field states what the run chose, as it does on the dialed path.
	fields := configureProviderFields("", "", "")
	return writeConfig(opts, cfg, fields, false)
}

// writeConfig writes cfg into the target layer and completes the result:
// the files written, the ids that now stand there, and — on the dialed path,
// where the ids were checked against a catalogue — the line a human reads
// instead of the document.
func writeConfig(opts configureOptions, cfg config.Config, fields []toolresult.Field, summary bool) (string, []toolresult.Field) {
	written, err := writeConfiguration(opts, cfg)
	fields = append(fields,
		toolresult.Field{Key: "models", Value: toolresult.Record{
			{Key: config.CategoryDecision, Value: stringOrNull(cfg.Models.Decision)},
			{Key: config.CategoryText, Value: stringOrNull(cfg.Models.Text)},
			{Key: config.CategoryScoring, Value: stringOrNull(cfg.Models.Scoring)}}},
		toolresult.Field{Key: "written", Value: nonNil(written)})
	if err != nil {
		return failed(fields, err)
	}
	if len(written) == 0 {
		return toolresult.Unchanged, fields
	}
	if summary {
		fmt.Fprintf(opts.Stderr, "configured: %s=%s %s=%s %s=%s (wrote %s)\n",
			config.CategoryDecision, cfg.Models.Decision,
			config.CategoryText, cfg.Models.Text,
			config.CategoryScoring, cfg.Models.Scoring,
			strings.Join(written, ", "))
	}
	return toolresult.Done, fields
}

// writeConfiguration creates the target layer's files where --create was given
// permission to and they are absent — providers.toml, then config.toml — and
// always sets cfg's values in config.toml. It reports the files it wrote, in the
// order it wrote them.
//
// Nothing is overwritten: the providers.toml template lands only where no file
// stands, and config.toml is edited line by line, so a directory a user has
// already filled in keeps everything this call was not asked to change.
func writeConfiguration(opts configureOptions, cfg config.Config) ([]string, error) {
	var written []string
	if opts.Create {
		providersFile := config.ProvidersPath(opts.WriteDir)
		created, err := config.CreateProvidersFile(providersFile)
		if err != nil {
			return written, err
		}
		if created {
			written = append(written, providersFile)
		}
	}
	configFile := config.ConfigPath(opts.WriteDir)
	wrote, err := config.UpdateConfig(configFile, cfg)
	if err != nil {
		return written, err
	}
	if wrote {
		written = append(written, configFile)
	}
	return written, nil
}

// parseModelMap turns the raw --model-map values into a category→reference
// map. Values accumulate across repeats of the flag and across comma-separated
// entries within one value, so `--model-map decision=a/d --model-map text=a/t`
// and `--model-map decision=a/d,text=a/t` are the same request. A value is a
// reference — the form config.toml's [models] values take — so no run of this
// verb writes a bare id. A partial map is syntactically legal here; the verb
// refuses a category it does not name, so every accepted run of the dialed
// path assigns what the file does not already hold.
func parseModelMap(values []string) (map[string]config.Reference, error) {
	out := make(map[string]config.Reference, len(modelCategories))
	for _, value := range values {
		for _, entry := range strings.Split(value, modelMapPairSep) {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			rawCategory, rawID, split := strings.Cut(entry, modelMapAssign)
			category := strings.ToLower(strings.TrimSpace(rawCategory))
			id := strings.TrimSpace(rawID)
			if !split || category == "" || id == "" {
				return nil, modelMapError(fmt.Sprintf("malformed --model-map entry %q — expected %s", entry, modelMapSyntax))
			}
			if !slices.Contains(modelCategories, category) {
				err := modelMapError(fmt.Sprintf("unknown category %q in --model-map (categories: %s)", category, strings.Join(modelCategories, ", ")))
				err.Allowed = modelCategories
				return nil, err
			}
			ref, err := config.FileReference(id)
			if err != nil {
				return nil, modelMapError(fmt.Sprintf("--model-map %s names %q, and a value is a reference: %s", category, id, modelMapSyntax))
			}
			if prev, dup := out[category]; dup && prev != ref {
				return nil, modelMapError(fmt.Sprintf("--model-map assigns the %s category twice (%q and %q)", category, prev.String(), ref.String()))
			}
			out[category] = ref
		}
	}
	return out, nil
}

// modelMapError is a --model-map value refused as usage.
func modelMapError(detail string) config.Refusal {
	return config.Refusal{Check: checkUsage, Key: "--model-map", Detail: detail}
}

// categoryKey is a category as config.toml's [models] table spells its key.
func categoryKey(category string) string { return "models." + category }

// unassignedCategoryItem refuses a category with no reference to write. The
// provider's catalogue is listed as the models to choose from — informational
// only, nothing in it is matched against the category.
func unassignedCategoryItem(category, provider string, ids []string) toolresult.Item {
	detail := fmt.Sprintf("no reference assigned to the %s category at provider %q; assign one explicitly, the provider offers: %s",
		category, provider, strings.Join(sortedCopy(ids), ", "))
	if len(ids) == 0 {
		detail = fmt.Sprintf("no reference assigned to the %s category at provider %q; assign one explicitly, it offered no models at all", category, provider)
	}
	return toolresult.Item{Check: checkDetect, Key: categoryKey(category), Allowed: sortedCopy(ids),
		Remedy: "kbase configure " + keyModelMap + " " + category + "=" + provider + "/<id>", Detail: detail}
}

// checkDetect names a category no --model-map assignment supplied.
const checkDetect = "detect"

func sortedCopy(values []string) []string {
	sorted := append([]string{}, values...)
	slices.Sort(sorted)
	return sorted
}

var (
	configureFlagProvider string
	configureFlagModelMap []string
	configureFlagTimeout  time.Duration
	configureFlagCreate   bool
	configureFlagProject  bool
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "assign a model reference to each category with --model-map and write config.toml, creating it with --create",
	Long: `Assign a model reference to each category — decision, text and scoring —
with --model-map, and write them to config.toml.

A value is a reference, <entry>/<id>: the providers.toml entry that supplies
the endpoint, then the model it serves. The entry is resolved as for
` + "`kbase models`" + ` — --provider, else the sole entry in providers.toml — and that
catalogue is what a category the flag does not name is offered. A reference
naming another entry is dialled and checked against that entry's own
catalogue. With --create and no --provider there is nothing to dial: no
request is made, no reference is checked against anything, and one line on
stderr says so. --timeout bounds one catalogue request, and is accepted and
unused when none is made.

On every path that dials, every category must have a reference to write.
A category the flag does not name is refused, naming models.<category> and
listing the models the provider offers as the choices, and nothing is written
— except scoring, which a run leaves standing where the target config.toml
already holds one. A model named in --model-map but missing from its entry's
catalogue is a warning, not a failure — provider catalogues rotate.

--create creates the target layer's providers.toml and config.toml where they
are absent, from an annotated template, and overwrites none: config.toml is
edited in place — the [models] decision, text and scoring keys are rewritten, a
superseded top-level provider key is removed, every other byte kept, and a file
already holding the values is left alone (outcome unchanged). A refused or
failed run writes nothing.

The target layer is the repository a --project flag names — <repository>/` + config.RepositoryDirName + `,
whether or not it exists, refused outside a git worktree — else the directory
--config-dir or KBASE_CONFIG_DIR names, else <repository>/` + config.RepositoryDirName + ` where that
directory already stands, else ~/.config/kbase. --config-dir with --project is
refused: they name different directories. Its result is one YAML document on
stdout.`,
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
		opts.Provider = configureFlagProvider
		opts.Timeout = configureFlagTimeout
		code, err := runConfigure(cmd.Context(), configureOptions{
			providerOptions: opts,
			ModelMap:        configureFlagModelMap,
			Create:          configureFlagCreate,
			Project:         configureFlagProject,
			WorkDir:         wd,
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
	registerProviderFlags(configureCmd, &configureFlagProvider, &configureFlagTimeout, "configure")
	configureCmd.Flags().BoolVar(&configureFlagCreate, "create", false,
		"create the target layer's "+config.ProvidersFileName+" and "+config.ConfigFileName+" where they are absent")
	configureCmd.Flags().BoolVar(&configureFlagProject, "project", false,
		"write into <repository>/"+config.RepositoryDirName+", whether or not it exists (refused outside a git worktree)")
	configureCmd.Flags().StringArrayVar(&configureFlagModelMap, "model-map", nil,
		"explicit category assignment, e.g. "+config.CategoryDecision+"=<entry>/<id>"+modelMapPairSep+config.CategoryText+"=<entry>/<id> (repeatable; every category needs one on a path that dials)")
	rootCmd.AddCommand(mcpBinding(configureCmd, mcpBound))
}
