package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Model tiers. The pipeline names a tier, never a model id: which gemma-4
// variant serves "heavy" is a deployment fact that belongs in config.toml,
// while "taxonomy design is heavy work" is a pipeline fact that belongs in
// the code. These constants are the single source of the tier names — the
// `toml` tags on ModelMap must mirror them (struct tags cannot reference
// constants).
const (
	// TierHeavy is the taxonomy / summaries / regeneration tier.
	TierHeavy = "heavy"
	// TierLight is the dissection / distillation / review tier.
	TierLight = "light"
)

// Config is the kbase settings file (config.toml) — the choices that draw
// from the providers.toml pool. providers.toml lists what is available;
// config.toml says which to use. It holds no credentials: every key lives
// on its providers.toml pool entry, which is what keeps config.toml safe
// to read and edit.
type Config struct {
	// Provider names the active providers.toml pool entry.
	Provider string `toml:"provider"`

	// Models maps the pipeline's tiers onto model ids at that provider.
	Models ModelMap `toml:"models"`

	// Dev holds the developer switches. The table is absent from an
	// ordinary config.toml, and every switch's off position is its zero
	// value, so absent and "turns nothing on" are the same state.
	Dev DevConfig `toml:"dev"`
}

// TreePlanMechanical is the only value [dev] tree_plan accepts. It names
// where the tree plan comes from rather than what it looks like, because
// that is the whole of the difference: the same verifier composes and
// checks both.
const TreePlanMechanical = "mechanical"

// DevConfig is the [dev] table: switches for diagnosing kbase itself.
//
// They are here rather than on the verbs as a deliberate concession to
// pragmatism (ruled 2026-08-14): `kbase build` is the delivered article and
// its flag set is the user's, so a switch that exists to serve kbase's own
// development does not get to sit in it. Each switch below states the
// upstream cause that makes it necessary — a switch that cannot name one is
// a flag looking for a home.
type DevConfig struct {
	// Telemetry enables local inference-timing diagnostics: the per-call
	// timing a provider reports alongside a response, which is a second
	// measurement channel for prompt-shape and prefill-vs-generation
	// questions. Off by default, and consumed by the logging facility —
	// the numbers land in the log, never in the user's output.
	Telemetry bool `toml:"telemetry"`

	// KeepTempWork keeps a SUCCESSFUL run's `<out>/temp-work/` tree instead
	// of deleting it (ARCHITECTURE.md §12). A failed or interrupted run keeps
	// it unconditionally, so this switch only ever answers "the run succeeded
	// and I still want to see what it did". Off by default: the intermediates
	// of a run that delivered are, by then, a copy of what it delivered plus
	// the proof machinery that got it there.
	KeepTempWork bool `toml:"keep_temp_work"`

	// BuildDate pins the date every delivered page's provenance receipt is
	// stamped with, as YYYY-MM-DD. Empty means today, UTC.
	//
	// Upstream cause: the receipt is DATED BY DESIGN (SPEC §7), so two
	// otherwise identical runs that straddle midnight deliver different
	// bytes. Byte-determinism is a property worth testing and a clock
	// cannot be tested against, so the date has to be pinnable from
	// somewhere; the only alternative — dropping the date — throws away a
	// fact the page is meant to carry.
	BuildDate string `toml:"build_date"`

	// TreePlan selects where stage 3's tree plan comes from. Empty is the
	// delivered behaviour: the taxonomy model designs it. TreePlanMechanical
	// takes the source's own file structure instead and dials no provider at
	// all — so a run under it makes no model call ANYWHERE, summaries
	// included, and its section pages carry no summary prose.
	//
	// Upstream cause: taxonomy design is a ruled no-fallback seam
	// (ARCHITECTURE §12), so the pipeline has no mechanical mode of its own
	// to fall into and a hermetic front-door test has nowhere to enter. This
	// is that entrance, and a run that takes it says so in run.json rather
	// than passing itself off as a build with a model in the loop.
	TreePlan string `toml:"tree_plan"`
}

// MechanicalTreePlan reports whether [dev] tree_plan selects the mechanical
// tree plan.
//
// An unrecognized value is a refusal rather than a fallback to either shape:
// the two differ in whether a provider is dialed at all, so guessing which
// one a typo meant is a guess about whether the corpus leaves the machine.
func (d DevConfig) MechanicalTreePlan() (bool, error) {
	switch v := strings.TrimSpace(d.TreePlan); v {
	case "":
		return false, nil
	case TreePlanMechanical:
		return true, nil
	default:
		return false, fmt.Errorf("config: [dev] tree_plan is %q; the only value is %q — omit the key for the designed tree plan",
			v, TreePlanMechanical)
	}
}

// ModelMap is the [models] table: the tier→model-id resolution, written by
// `kbase configure` or by hand. An unset tier is not defaulted — a guessed
// model id is a silent wrong answer, so callers fail loud instead.
type ModelMap struct {
	Heavy string `toml:"heavy"`
	Light string `toml:"light"`
}

// ModelFor returns the model id configured for a tier. The second return
// is false when the tier is unknown or unmapped, which callers must treat
// as "not configured" — never as license to substitute another tier's
// model.
func (c Config) ModelFor(tier string) (string, bool) {
	var id string
	switch tier {
	case TierHeavy:
		id = c.Models.Heavy
	case TierLight:
		id = c.Models.Light
	default:
		return "", false
	}
	return id, id != ""
}

// LoadConfig reads config.toml at path. A nonexistent file yields a zero
// Config and a nil error — a fresh install has no config.toml, and the
// caller decides which of the resulting gaps are fatal for the command it
// is running.
//
// No key material is resolved here: config.toml holds CHOICES, and every
// credential lives on its providers.toml pool entry, resolved by
// LoadProviders.
//
// The decode is strict: a key or table Config does not model refuses the
// load, naming the file and every offending key (see rejectUnknownKeys).
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if err := rejectUnknownKeys(path, md); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
