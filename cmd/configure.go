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
	"kbase/internal/detect"
	toolresult "kbase/internal/result"
)

const (
	// modelMapPairSep separates tier assignments within one --model-map
	// value; modelMapAssign separates a tier from its model id.
	modelMapPairSep = ","
	modelMapAssign  = "="

	// modelMapSyntax is the flag spelling quoted back to the user in every
	// failure a manual assignment would resolve. Auto-detection declining
	// to guess is only helpful if the way past it is on screen.
	modelMapSyntax = "--model-map " + config.TierHeavy + "=<id>" + modelMapPairSep + config.TierLight + "=<id>"
)

// configureTiers is the order tiers are resolved, reported, and written
// in. config.TierHeavy and config.TierLight are the single source of the
// names; this is the single source of the order.
var configureTiers = []string{config.TierHeavy, config.TierLight}

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
// a gemma-4 model to each pipeline tier, writes the result to config.toml,
// writes its result document and returns the exit code the outcome maps to.
//
// A tier is assigned from --model-map when given, otherwise by
// auto-detection — and only when detection finds exactly one candidate for
// it. Zero candidates or several is refused, listing what the provider
// offered, with nothing written: config.toml is what every later stage draws
// its models from, so a guessed entry there is a wrong answer that never
// announces itself.
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

	res := detect.Classify(ids)
	resolved := make(map[string]string, len(configureTiers))
	var undetected []toolresult.Item
	for _, tier := range configureTiers {
		if id, given := manual[tier]; given {
			resolved[tier] = id
			continue
		}
		candidates := tierCandidates(res, tier)
		switch len(candidates) {
		case 1:
			resolved[tier] = candidates[0]
		case 0:
			undetected = append(undetected, noCandidateItem(tier, name, res, ids))
		default:
			undetected = append(undetected, ambiguousTierItem(tier, name, candidates))
		}
	}
	if undetected != nil {
		return refused(fields, undetected...)
	}

	cfg := config.Config{
		Provider: name,
		Models: config.ModelMap{
			Heavy: resolved[config.TierHeavy],
			Light: resolved[config.TierLight],
		},
	}
	fields = append(fields, toolresult.Field{Key: "models", Value: toolresult.Record{
		{Key: config.TierHeavy, Value: cfg.Models.Heavy}, {Key: config.TierLight, Value: cfg.Models.Light}}})
	wrote, err := config.UpdateConfig(opts.ConfigPath, cfg)
	if err != nil {
		return failed(append(fields, toolresult.Field{Key: "written", Value: []string{}}), err)
	}
	if !wrote {
		return toolresult.Unchanged, append(fields, toolresult.Field{Key: "written", Value: []string{}})
	}
	fmt.Fprintf(opts.Stderr, "configured: provider=%s %s=%s %s=%s (wrote %s)\n",
		name,
		config.TierHeavy, cfg.Models.Heavy,
		config.TierLight, cfg.Models.Light,
		opts.ConfigPath)
	return toolresult.Done, append(fields, toolresult.Field{Key: "written", Value: []string{opts.ConfigPath}})
}

// parseModelMap turns the raw --model-map values into a tier→id map.
// Values accumulate across repeats of the flag and across comma-separated
// entries within one value, so `--model-map heavy=a --model-map light=b`
// and `--model-map heavy=a,light=b` are the same request. A partial map is
// legal: tiers it does not name are auto-detected.
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

// tierCandidates returns the detected candidates for a tier. An unknown
// tier has none, which surfaces as the same fail-loud no-match a provider
// with no gemma-4 models produces.
func tierCandidates(res detect.Result, tier string) []string {
	switch tier {
	case config.TierHeavy:
		return res.Heavy
	case config.TierLight:
		return res.Light
	default:
		return nil
	}
}

// modelMapError is a --model-map value refused as usage.
func modelMapError(detail string) itemError {
	return itemError{toolresult.Item{Check: checkUsage, Key: "--model-map", Detail: detail}}
}

// tierKey is a tier as config.toml's [models] table spells its key.
func tierKey(tier string) string { return "models." + tier }

// noCandidateItem refuses a tier auto-detection could not fill, listing what
// the provider offered as the ids to choose from: the gemma-4 models whose
// tier went unrecognized where there are any — the likeliest thing the user
// meant — else the whole catalogue.
func noCandidateItem(tier, provider string, res detect.Result, ids []string) toolresult.Item {
	offered := res.UnclassifiedFamily
	if len(offered) == 0 {
		offered = ids
	}
	detail := fmt.Sprintf("no gemma-4 %s-tier model detected at provider %q", tier, provider)
	if len(ids) == 0 {
		detail += "; it offered no models at all"
	}
	return toolresult.Item{Check: checkDetect, Key: tierKey(tier), Allowed: sortedCopy(offered),
		Remedy: "kbase configure --model-map " + tier + "=<id>", Detail: detail}
}

// ambiguousTierItem refuses a tier with several detected candidates. kbase
// does not break the tie — picking one would be a silent best guess — so it
// names them and asks.
func ambiguousTierItem(tier, provider string, candidates []string) toolresult.Item {
	return toolresult.Item{Check: checkDetect, Key: tierKey(tier), Allowed: sortedCopy(candidates),
		Remedy: "kbase configure --model-map " + tier + "=<id>",
		Detail: fmt.Sprintf("the %s tier is ambiguous at provider %q: %d gemma-4 candidates", tier, provider, len(candidates))}
}

// checkDetect names a tier detection could not settle.
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
	Short: "detect the provider's gemma-4 models and write config.toml",
	Long: `Query a provider's model catalogue, assign a model id to each tier —
heavy and light — and write them, with the provider, to config.toml.

The provider is resolved exactly as for ` + "`kbase models`" + `: --provider, else the
provider named in config.toml, else the sole entry in providers.toml.

Tiers are filled from --model-map where given and by detecting the
provider's gemma-4 models otherwise — and only where detection finds
exactly one candidate for the tier. A tier with none, or several, is
refused, naming models.heavy or models.light and listing the ids to choose
from, and nothing is written: a guessed model mapping is a wrong answer
that never announces itself. A model named in --model-map but missing from
the catalogue is a warning, not a failure — provider catalogues rotate.

config.toml is edited in place: only provider and the [models] heavy and
light keys are rewritten, every other byte is kept, and a file already
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
		"explicit tier assignment, e.g. "+config.TierHeavy+"=<id>"+modelMapPairSep+config.TierLight+"=<id> (repeatable; unnamed tiers are auto-detected)")
	rootCmd.AddCommand(mcpBinding(configureCmd, mcpExcluded))
}
