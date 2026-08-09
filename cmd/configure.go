package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/detect"
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

	// listIndent prefixes each id in a multi-line failure listing.
	listIndent = "  "
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
}

// runConfigure resolves a provider, discovers its model catalogue, assigns
// a gemma-4 model to each pipeline tier, and writes the result to
// config.toml.
//
// A tier is assigned from --model-map when given, otherwise by
// auto-detection — and only when detection finds exactly one candidate for
// it. Zero candidates or several is a failure that lists what the provider
// offered and exits without writing anything: config.toml is what every
// later stage draws its models from, so a guessed entry there is a wrong
// answer that never announces itself (SPEC.md §1).
func runConfigure(ctx context.Context, opts configureOptions) error {
	if opts.Stderr == nil {
		return fmt.Errorf("configure: Stderr is required")
	}
	if opts.ConfigPath == "" {
		return fmt.Errorf("configure: ConfigPath is required")
	}

	// Flag syntax is checked before anything is dialed: a typo in
	// --model-map should not cost a network round trip to discover.
	manual, err := parseModelMap(opts.ModelMap)
	if err != nil {
		return err
	}

	name, client, ctx, release, err := opts.dial(ctx)
	if err != nil {
		return err
	}
	defer release()

	ids, err := catalogueIDs(ctx, client)
	if err != nil {
		return err
	}

	// A manually assigned id the provider does not list is a warning, not
	// a failure: catalogues rotate, and a user pinning a model the listing
	// omits may know something the listing does not (SPEC.md §1).
	for _, tier := range configureTiers {
		if id, given := manual[tier]; given && !slices.Contains(ids, id) {
			fmt.Fprintf(opts.Stderr,
				"configure: %s tier model %q is not in provider %q's catalogue — configuring it anyway\n",
				tier, id, name)
		}
	}

	res := detect.Classify(ids)
	resolved := make(map[string]string, len(configureTiers))
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
			return noCandidateError(tier, name, res, ids)
		default:
			return ambiguousTierError(tier, name, candidates)
		}
	}

	cfg := config.Config{
		Provider: name,
		Models: config.ModelMap{
			Heavy: resolved[config.TierHeavy],
			Light: resolved[config.TierLight],
		},
	}
	if err := config.UpdateConfig(opts.ConfigPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(opts.Stderr, "configured: provider=%s %s=%s %s=%s (wrote %s)\n",
		name,
		config.TierHeavy, cfg.Models.Heavy,
		config.TierLight, cfg.Models.Light,
		opts.ConfigPath)
	return nil
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
				return nil, fmt.Errorf("configure: malformed --model-map entry %q — expected %s", entry, modelMapSyntax)
			}
			if !slices.Contains(configureTiers, tier) {
				return nil, fmt.Errorf("configure: unknown tier %q in --model-map (tiers: %s)",
					tier, strings.Join(configureTiers, ", "))
			}
			if prev, dup := out[tier]; dup && prev != id {
				return nil, fmt.Errorf("configure: --model-map assigns the %s tier twice (%q and %q)", tier, prev, id)
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

// noCandidateError reports a tier auto-detection could not fill, and lists
// what the provider actually offered — first any gemma-4 models whose tier
// went unrecognized (the likeliest thing the user meant), then the whole
// catalogue. Listing is the entire point: "no gemma-4 detected" without
// the candidates leaves the user with nothing to type next.
func noCandidateError(tier, provider string, res detect.Result, ids []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "configure: no gemma-4 %s-tier model detected at provider %q", tier, provider)
	if len(res.UnclassifiedFamily) > 0 {
		fmt.Fprintf(&b, "\ngemma-4 models with no recognized tier:\n%s", indentedList(res.UnclassifiedFamily))
	}
	if len(ids) > 0 {
		fmt.Fprintf(&b, "\nmodels offered by %q:\n%s", provider, indentedList(ids))
	} else {
		fmt.Fprintf(&b, "\nprovider %q offered no models at all", provider)
	}
	fmt.Fprintf(&b, "\nassign the tiers explicitly and rerun: %s", modelMapSyntax)
	return fmt.Errorf("%s", b.String())
}

// ambiguousTierError reports a tier with several detected candidates. The
// appliance does not break the tie — picking one would be the silent
// best-guess SPEC.md §1 rules out — so it names them and asks.
func ambiguousTierError(tier, provider string, candidates []string) error {
	return fmt.Errorf("configure: %s tier is ambiguous at provider %q — %d gemma-4 candidates:\n%s\nassign one explicitly and rerun: --model-map %s=<id>",
		tier, provider, len(candidates), indentedList(candidates), tier)
}

// indentedList renders ids one per indented line, sorted ascending — the
// same stable ordering `kbase models` prints, so the two verbs' output can
// be read against each other.
func indentedList(ids []string) string {
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	return listIndent + strings.Join(sorted, "\n"+listIndent)
}

var (
	configureFlagProvider string
	configureFlagModelMap []string
	configureFlagTimeout  time.Duration
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "detect the provider's gemma-4 models and write config.toml",
	Long: `Query a provider's model catalogue, assign a gemma-4 model to each
pipeline tier, and write the result to config.toml.

The provider is resolved exactly as for ` + "`kbase models`" + `: --provider, else the
provider named in config.toml, else the sole entry in providers.toml.

Tiers are filled from --model-map where given and by auto-detection
otherwise — and only where detection finds exactly one gemma-4 candidate
for the tier. If a tier has none, or several, the command lists what the
provider offered and exits without writing anything: a guessed model
mapping is a wrong answer that never announces itself. A model named in
--model-map but missing from the catalogue is a warning, not a failure —
provider catalogues rotate.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, opts, err := loadVerbContext()
		if err != nil {
			return err
		}
		opts.Provider = configureFlagProvider
		opts.Timeout = configureFlagTimeout
		return runConfigure(cmd.Context(), configureOptions{
			providerOptions: opts,
			ModelMap:        configureFlagModelMap,
			ConfigPath:      config.ConfigPath(dir),
		})
	},
}

func init() {
	registerProviderFlags(configureCmd, &configureFlagProvider, &configureFlagTimeout, "configure")
	configureCmd.Flags().StringArrayVar(&configureFlagModelMap, "model-map", nil,
		"explicit tier assignment, e.g. "+config.TierHeavy+"=<id>"+modelMapPairSep+config.TierLight+"=<id> (repeatable; unnamed tiers are auto-detected)")
	rootCmd.AddCommand(configureCmd)
}
