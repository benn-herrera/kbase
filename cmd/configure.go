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
	// modelMapPairSep separates tier assignments within one --model-map
	// value; modelMapAssign separates a tier from its model id.
	modelMapPairSep = ","
	modelMapAssign  = "="

	// modelMapSyntax is the flag spelling quoted back to the user in every
	// failure a manual assignment would resolve.
	modelMapSyntax = "--model-map " + config.TierDecision + "=<id>" + modelMapPairSep + config.TierText + "=<id>"
)

// configureTiers is the order tiers are resolved, reported, and written
// in. config.TierDecision and config.TierText are the single source of the
// names; this is the single source of the order.
var configureTiers = []string{config.TierDecision, config.TierText}

// configureOptions is the resolved input of the configure verb: the shared
// provider-reaching options plus this verb's own flag and the path the
// result is written to.
//
// The embedded Config's [models] table is deliberately not consulted here —
// re-running configure is how a stale model map is replaced.
type configureOptions struct {
	providerOptions

	// ModelMap holds the raw --model-map values, one per occurrence of
	// the flag, each possibly comma-separated.
	ModelMap []string

	// ConfigPath is the config.toml the resolved choices are written to.
	ConfigPath string

	Stdout io.Writer
}

// runConfigure resolves a provider, discovers its model catalogue, assigns
// a model to each pipeline tier, writes the result to config.toml,
// writes its result document and returns the exit code the outcome maps to.
//
// Every tier is assigned from --model-map only: a tier the flag does not
// name is refused, listing what the provider offered, with nothing written —
// config.toml is what every later stage draws its models from, and nothing
// between the flag and the file picks a model on the user's behalf.
func runConfigure(ctx context.Context, opts configureOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "configure"); err != nil {
		return 0, err
	}
	if opts.ConfigPath == "" {
		return 0, fmt.Errorf("configure: ConfigPath is required")
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

	name, client, ctx, release, err := opts.dial(ctx)
	if err != nil {
		return refusedOrFailed(nil, err)
	}
	defer release()
	fields := []toolresult.Field{{Key: "provider", Value: name}}

	ids, err := catalogueIDs(ctx, client)
	if err != nil {
		return failed(fields, err)
	}

	// A manually assigned id the provider does not list is a warning, not
	// a failure: catalogues rotate, and a user pinning a model the listing
	// omits may know something the listing does not.
	for _, tier := range configureTiers {
		if id, given := manual[tier]; given && !slices.Contains(ids, id) {
			fmt.Fprintf(opts.Stderr,
				"configure: %s tier model %q is not in provider %q's catalogue — configuring it anyway\n",
				tier, id, name)
		}
	}

	// A tier --model-map does not name is refused, one item per missing
	// tier; the catalogue listing is the ids to choose from, named for the
	// choosing, never matched against.
	var unassigned []toolresult.Item
	for _, tier := range configureTiers {
		if _, given := manual[tier]; !given {
			unassigned = append(unassigned, unassignedTierItem(tier, name, ids))
		}
	}
	if unassigned != nil {
		return refused(fields, unassigned...)
	}

	cfg := config.Config{
		Provider: name,
		Models: config.ModelMap{
			Decision: manual[config.TierDecision],
			Text:     manual[config.TierText],
		},
	}
	fields = append(fields, toolresult.Field{Key: "models", Value: toolresult.Record{
		{Key: config.TierDecision, Value: cfg.Models.Decision}, {Key: config.TierText, Value: cfg.Models.Text}}})
	wrote, err := config.UpdateConfig(opts.ConfigPath, cfg)
	if err != nil {
		return failed(append(fields, toolresult.Field{Key: "written", Value: []string{}}), err)
	}
	if !wrote {
		return toolresult.Unchanged, append(fields, toolresult.Field{Key: "written", Value: []string{}})
	}
	fmt.Fprintf(opts.Stderr, "configured: provider=%s %s=%s %s=%s (wrote %s)\n",
		name,
		config.TierDecision, cfg.Models.Decision,
		config.TierText, cfg.Models.Text,
		opts.ConfigPath)
	return toolresult.Done, append(fields, toolresult.Field{Key: "written", Value: []string{opts.ConfigPath}})
}

// parseModelMap turns the raw --model-map values into a tier→id map.
// Values accumulate across repeats of the flag and across comma-separated
// entries within one value, so `--model-map decision=a --model-map text=b`
// and `--model-map decision=a,text=b` are the same request. A partial map is
// syntactically legal here; the verb refuses a tier it does not name, so
// every accepted run assigns both.
func parseModelMap(values []string) (map[string]string, error) {
	out := make(map[string]string, len(configureTiers))
	for _, value := range values {
		for _, entry := range strings.Split(value, modelMapPairSep) {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			rawTier, rawID, split := strings.Cut(entry, modelMapAssign)
			tier := strings.ToLower(strings.TrimSpace(rawTier))
			id := strings.TrimSpace(rawID)
			if !split || tier == "" || id == "" {
				return nil, modelMapError(fmt.Sprintf("malformed --model-map entry %q — expected %s", entry, modelMapSyntax))
			}
			if !slices.Contains(configureTiers, tier) {
				err := modelMapError(fmt.Sprintf("unknown tier %q in --model-map (tiers: %s)", tier, strings.Join(configureTiers, ", ")))
				err.item.Allowed = configureTiers
				return nil, err
			}
			if prev, dup := out[tier]; dup && prev != id {
				return nil, modelMapError(fmt.Sprintf("--model-map assigns the %s tier twice (%q and %q)", tier, prev, id))
			}
			out[tier] = id
		}
	}
	return out, nil
}

// modelMapError is a --model-map value refused as usage.
func modelMapError(detail string) itemError {
	return itemError{toolresult.Item{Check: checkUsage, Key: "--model-map", Detail: detail}}
}

// tierKey is a tier as config.toml's [models] table spells its key.
func tierKey(tier string) string { return "models." + tier }

// unassignedTierItem refuses a tier no --model-map assignment supplied. The
// provider's catalogue is listed as the ids to choose from — informational
// only, nothing in it is matched against the tier.
func unassignedTierItem(tier, provider string, ids []string) toolresult.Item {
	detail := fmt.Sprintf("no model assigned to the %s tier at provider %q; assign it explicitly, the provider offers: %s",
		tier, provider, strings.Join(sortedCopy(ids), ", "))
	if len(ids) == 0 {
		detail = fmt.Sprintf("no model assigned to the %s tier at provider %q; assign it explicitly, it offered no models at all", tier, provider)
	}
	return toolresult.Item{Check: checkDetect, Key: tierKey(tier), Allowed: sortedCopy(ids),
		Remedy: "kbase configure --model-map " + tier + "=<id>", Detail: detail}
}

// checkDetect names a tier no --model-map assignment supplied.
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
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "assign the decision and text models with --model-map and write config.toml",
	Long: `Assign a model id to each tier — decision and text — with --model-map,
and write them, with the provider, to config.toml.

The provider is resolved exactly as for ` + "`kbase models`" + `: --provider, else the
provider named in config.toml, else the sole entry in providers.toml.

Both tiers must be named: --model-map decision=<id>,text=<id>. Nothing is
guessed or matched between the tiers and the provider's catalogue — a tier
the flag does not name is refused, naming models.decision or models.text
and listing the ids the provider offers as the choices, and nothing is
written. A model named in --model-map but missing from the catalogue is a
warning, not a failure — provider catalogues rotate.

config.toml is edited in place: only provider and the [models] decision and
text keys are rewritten, every other byte is kept, and a file already
holding the values is left alone (outcome unchanged). Its result is one
YAML document on stdout.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, opts, err := loadVerbContext()
		if err != nil {
			return err
		}
		opts.Provider = configureFlagProvider
		opts.Timeout = configureFlagTimeout
		code, err := runConfigure(cmd.Context(), configureOptions{
			providerOptions: opts,
			ModelMap:        configureFlagModelMap,
			ConfigPath:      config.ConfigPath(dir),
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
	configureCmd.Flags().StringArrayVar(&configureFlagModelMap, "model-map", nil,
		"explicit tier assignment, e.g. "+config.TierDecision+"=<id>"+modelMapPairSep+config.TierText+"=<id> (repeatable; both tiers must be named)")
	rootCmd.AddCommand(mcpBinding(configureCmd, mcpExcluded))
}
