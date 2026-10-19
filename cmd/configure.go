package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	toolresult "kbase/internal/result"
)

const (
	// modelMapPairSep separates category assignments within one --model-map
	// value; modelMapAssign separates a category from its model id.
	modelMapPairSep = ","
	modelMapAssign  = "="

	// modelMapSyntax is the flag spelling quoted back to the user in every
	// failure a manual assignment would resolve.
	modelMapSyntax = "--model-map " + config.CategoryDecision + "=<id>" + modelMapPairSep + config.CategoryText + "=<id>"
)

// modelCategories is the order the model categories are resolved, reported, and
// written in. config.CategoryDecision and config.CategoryText are the single
// source of the names; this is the single source of the order.
var modelCategories = []string{config.CategoryDecision, config.CategoryText}

// configureOptions is the resolved input of the configure verb: the shared
// provider-reaching options plus this verb's own flags and the stream its
// result goes to.
//
// The write target is the embedded WriteDir — the layer this invocation
// resolved, which --project may replace with the repository's. The embedded
// Config's [models] table is deliberately not consulted on the dial path:
// re-running configure is how a stale model map is replaced.
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
}

// runConfigure resolves a provider, discovers its model catalogue, assigns
// a model to each model category, writes the result to config.toml,
// writes its result document and returns the exit code the outcome maps to.
//
// Every category is assigned from --model-map only: a category the flag does
// not name is refused, listing what the provider offered, with nothing
// written — config.toml is what every later stage draws its models from, and
// nothing between the flag and the file picks a model on the user's behalf.
// --create with no --provider is the one exception, and it dials nothing to
// refuse against: it writes what the flags name and says the ids are unchecked.
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

// configureDialed is the ordinary path: select a pool entry, dial it, list its
// catalogue, and write both categories' ids — a refusal or a failed listing
// writes nothing.
func configureDialed(ctx context.Context, opts configureOptions, manual map[string]string) (string, []toolresult.Field) {
	name, rule, layer, client, ctx, release, err := opts.dial(ctx)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	defer release()
	fields := providerFields(name, rule, layer)

	ids, err := catalogueIDs(ctx, client)
	if err != nil {
		return failed(fields, err)
	}

	// A manually assigned id the provider does not list is a warning, not
	// a failure: catalogues rotate, and a user pinning a model the listing
	// omits may know something the listing does not.
	for _, category := range modelCategories {
		if id, given := manual[category]; given && !slices.Contains(ids, id) {
			fmt.Fprintf(opts.Stderr,
				"configure: %s category model %q is not in provider %q's catalogue — configuring it anyway\n",
				category, id, name)
		}
	}

	// A category --model-map does not name is refused, one item per missing
	// category; the catalogue listing is the ids to choose from, named for the
	// choosing, never matched against.
	var unassigned []toolresult.Item
	for _, category := range modelCategories {
		if _, given := manual[category]; !given {
			unassigned = append(unassigned, unassignedCategoryItem(category, name, ids))
		}
	}
	if unassigned != nil {
		return refused(fields, unassigned...)
	}

	cfg := config.Config{
		Provider: name,
		Models: config.ModelMap{
			Decision: manual[config.CategoryDecision],
			Text:     manual[config.CategoryText],
		},
	}
	return writeConfig(opts, cfg, fields, true)
}

// configureOffline writes what the flags name and nothing else: no provider was
// named, so no request is made and no id is checked against a catalogue. What
// the target layer's config.toml already holds — a provider choice, an
// untouched category, a comment — survives, because configure is an editor and
// --create is only permission to create the file it edits.
func configureOffline(opts configureOptions, manual map[string]string) (string, []toolresult.Field) {
	cfg, err := config.LoadConfig(config.ConfigPath(opts.WriteDir))
	if err != nil {
		return failed(nil, err)
	}
	if id, given := manual[config.CategoryDecision]; given {
		cfg.Models.Decision = id
	}
	if id, given := manual[config.CategoryText]; given {
		cfg.Models.Text = id
	}
	fmt.Fprintf(opts.Stderr, "configure: no provider named, so the model ids were not checked\n")
	fields := []toolresult.Field{{Key: "provider", Value: stringOrNull(cfg.Provider)}}
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
			{Key: config.CategoryText, Value: stringOrNull(cfg.Models.Text)}}},
		toolresult.Field{Key: "written", Value: nonNil(written)})
	if err != nil {
		return failed(fields, err)
	}
	if len(written) == 0 {
		return toolresult.Unchanged, fields
	}
	if summary {
		fmt.Fprintf(opts.Stderr, "configured: provider=%s %s=%s %s=%s (wrote %s)\n",
			cfg.Provider,
			config.CategoryDecision, cfg.Models.Decision,
			config.CategoryText, cfg.Models.Text,
			strings.Join(written, ", "))
	}
	return toolresult.Done, fields
}

// writeConfiguration creates the target layer's files where --create was given
// permission to and they are absent, and always sets cfg's values in
// config.toml. It reports the files it wrote, in the order it wrote them.
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

// parseModelMap turns the raw --model-map values into a category→id map.
// Values accumulate across repeats of the flag and across comma-separated
// entries within one value, so `--model-map decision=a --model-map text=b`
// and `--model-map decision=a,text=b` are the same request. A partial map is
// syntactically legal here; the verb refuses a category it does not name, so
// every accepted run of the dialed path assigns both.
func parseModelMap(values []string) (map[string]string, error) {
	out := make(map[string]string, len(modelCategories))
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
				err.item.Allowed = modelCategories
				return nil, err
			}
			if prev, dup := out[category]; dup && prev != id {
				return nil, modelMapError(fmt.Sprintf("--model-map assigns the %s category twice (%q and %q)", category, prev, id))
			}
			out[category] = id
		}
	}
	return out, nil
}

// modelMapError is a --model-map value refused as usage.
func modelMapError(detail string) itemError {
	return itemError{toolresult.Item{Check: checkUsage, Key: "--model-map", Detail: detail}}
}

// categoryKey is a category as config.toml's [models] table spells its key.
func categoryKey(category string) string { return "models." + category }

// unassignedCategoryItem refuses a category no --model-map assignment supplied.
// The provider's catalogue is listed as the ids to choose from — informational
// only, nothing in it is matched against the category.
func unassignedCategoryItem(category, provider string, ids []string) toolresult.Item {
	detail := fmt.Sprintf("no model assigned to the %s category at provider %q; assign it explicitly, the provider offers: %s",
		category, provider, strings.Join(sortedCopy(ids), ", "))
	if len(ids) == 0 {
		detail = fmt.Sprintf("no model assigned to the %s category at provider %q; assign it explicitly, it offered no models at all", category, provider)
	}
	return toolresult.Item{Check: checkDetect, Key: categoryKey(category), Allowed: sortedCopy(ids),
		Remedy: "kbase configure --model-map " + category + "=<id>", Detail: detail}
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
	Short: "assign the decision and text models with --model-map and write config.toml, creating it with --create",
	Long: `Assign a model id to each category — decision and text — with --model-map,
and write them, with the provider, to config.toml.

The provider is resolved exactly as for ` + "`kbase models`" + `: --provider, else the
provider named in config.toml, else the sole entry in providers.toml. With
--create and no --provider there is nothing to dial: no request is made, no id
is checked against anything, and one line on stderr says so. --timeout bounds
that request, and is accepted and unused when none is made.

On every path that dials, both categories must be named: --model-map
decision=<id>,text=<id>. Nothing is guessed or matched between the categories
and the provider's catalogue — a category the flag does not name is refused,
naming models.decision or models.text and listing the ids the provider offers
as the choices, and nothing is written. A model named in --model-map but
missing from the catalogue is a warning, not a failure — provider catalogues
rotate.

--create creates the target layer's providers.toml and config.toml where they
are absent, from annotated templates, and overwrites neither: config.toml is
edited in place — only provider and the [models] decision and text keys are
rewritten, every other byte kept, and a file already holding the values is
left alone (outcome unchanged). A refused or failed run writes nothing.

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
		"explicit category assignment, e.g. "+config.CategoryDecision+"=<id>"+modelMapPairSep+config.CategoryText+"=<id> (repeatable; both categories must be named on a path that dials)")
	rootCmd.AddCommand(mcpBinding(configureCmd, mcpBound))
}
