package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Model categories. The build names a category, never a model id: which model
// serves the decision category is a deployment fact that belongs in
// config.toml, while "the multichoice asks are decision-model calls" is a build
// fact that belongs in the code. These constants are the single source of the
// category names — a role is one of them typed as Role, and the `toml` tags on
// ModelMap must mirror them (struct tags cannot reference constants).
const (
	// CategoryDecision is the claim graph's multichoice asks' category: the
	// decision model that answers them.
	CategoryDecision = "decision"
	// CategoryText is the overview passage's category.
	CategoryText = "text"
	// CategoryScoring is kbase score's asks' category.
	CategoryScoring = "scoring"
)

// Config is the kbase settings file (config.toml) — the choices that draw
// from the providers.toml pool. providers.toml lists what is available;
// config.toml says which to use. It holds no credentials: every key belongs
// to a providers.toml pool entry, stored under its name or in its key file,
// which is what keeps config.toml safe to read and edit.
type Config struct {
	// Models maps the verb's roles onto the endpoint and model each asks.
	Models ModelMap `toml:"models"`

	// Asks is how the build puts its multichoice asks.
	Asks AsksConfig `toml:"asks"`

	// Sampling is how the text and scoring roles are asked.
	Sampling SamplingTable `toml:"sampling"`

	// Sources names the layer that supplied each merged model id — one of
	// LayerSourceRepository, LayerSourceGlobal, LayerSourceExplicit or
	// LayerSourceEnvironment. It is set by Load's merge, never read from a
	// file: one config.toml cannot say which layer it is.
	Sources Sources `toml:"-"`
}

// AsksConfig is the [asks] table.
type AsksConfig struct {
	// Concurrency is how many model calls one build or score has in flight
	// at once, across every role; nil, the key absent, leaves the default.
	Concurrency *int `toml:"concurrency"`
}

// keyConcurrency is AsksConfig's concurrency key by its full dotted path, as
// a message names it.
const keyConcurrency = "asks.concurrency"

// DefaultAskConcurrency is how many model calls an invocation has in flight
// at once where neither --ask-concurrency nor [asks] concurrency names a
// number.
const DefaultAskConcurrency = 4

// ModelMap is the [models] table: the role→model resolution, written by
// `kbase configure` or by hand. A value is a file-mode reference,
// `<provider>/<model>` — the pool entry that supplies the endpoint, then the
// model it serves. An unset role is not defaulted — a guessed model id is a
// silent wrong answer, so callers fail loud instead.
type ModelMap struct {
	Decision string `toml:"decision"`
	Text     string `toml:"text"`
	Scoring  string `toml:"scoring"`
}

// ModelFor returns the model id configured for a category. The second return
// is false when the category is unknown or unmapped, which callers must treat
// as "not configured" — never as license to substitute another category's
// model.
func (c Config) ModelFor(category string) (string, bool) {
	var id string
	switch category {
	case CategoryDecision:
		id = c.Models.Decision
	case CategoryText:
		id = c.Models.Text
	case CategoryScoring:
		id = c.Models.Scoring
	default:
		return "", false
	}
	return id, id != ""
}

// ModelLayer is the layer that supplied the model the role asks, "" where no
// layer did.
func (c Config) ModelLayer(role Role) string {
	switch role {
	case RoleDecision:
		return c.Sources.Decision
	case RoleText:
		return c.Sources.Text
	case RoleScoring:
		return c.Sources.Scoring
	}
	return ""
}

// The thinking levels a role may be asked at. The vocabulary is kbase's; each
// chat client maps a level onto what its protocol takes.
const (
	ThinkingOff    = "off"
	ThinkingLow    = "low"
	ThinkingMedium = "medium"
	ThinkingHigh   = "high"
)

// ThinkingLevels is every thinking level, in the order a message lists them.
var ThinkingLevels = []string{ThinkingOff, ThinkingLow, ThinkingMedium, ThinkingHigh}

// SamplingTable is the [sampling] table: one sub-table per chat role that is
// asked with sampling. The decision role has none, so a [sampling.decision]
// key fails the strict load as unmodeled.
type SamplingTable struct {
	Text    RoleSampling `toml:"text"`
	Scoring RoleSampling `toml:"scoring"`
}

// RoleSampling is one role's sampling. A field left unset is kbase's default —
// temperature 0, thinking off — and never the provider's.
type RoleSampling struct {
	// Temperature is nil where no layer states one.
	Temperature *float64 `toml:"temperature"`
	// Thinking is one of ThinkingLevels, or "" where no layer states one.
	Thinking string `toml:"thinking"`
}

// SamplingLayer is the layer that supplied the role's temperature and its
// thinking, "" for each that no layer did and for the decision role.
func (c Config) SamplingLayer(role Role) (temperature, thinking string) {
	switch role {
	case RoleText:
		return c.Sources.TextTemperature, c.Sources.TextThinking
	case RoleScoring:
		return c.Sources.ScoringTemperature, c.Sources.ScoringThinking
	}
	return "", ""
}

// samplingOf is the role's sampling as the files state it: the zero value for
// the decision role.
func (c Config) samplingOf(role Role) RoleSampling {
	switch role {
	case RoleText:
		return c.Sampling.Text
	case RoleScoring:
		return c.Sampling.Scoring
	}
	return RoleSampling{}
}

// Sources names the layer that supplied each model id the merge took: the
// configuration layer — explicit, environment, repository or global — the
// config.toml value came from. A role no layer supplied carries "". The
// sampling fields name the layer of each [sampling] value the same way.
type Sources struct {
	Decision, Text, Scoring string

	TextTemperature, TextThinking       string
	ScoringTemperature, ScoringThinking string
}

// LoadConfig reads config.toml at path. A nonexistent file yields a zero
// Config and a nil error — a fresh install has no config.toml, and the
// caller decides which of the resulting gaps are fatal for the command it
// is running.
//
// No key material is resolved here: config.toml holds CHOICES, and every
// credential belongs to a providers.toml pool entry, resolved by
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
	if n := cfg.Asks.Concurrency; n != nil && *n < 1 {
		return Config{}, fmt.Errorf("config: %s: %s is a whole number of model calls, at least 1", path, keyConcurrency)
	}
	for _, role := range []Role{RoleText, RoleScoring} {
		if level := cfg.samplingOf(role).Thinking; md.IsDefined("sampling", string(role), "thinking") && !slices.Contains(ThinkingLevels, level) {
			return Config{}, fmt.Errorf("config: %s: sampling.%s.thinking is one of %s", path, role, strings.Join(ThinkingLevels, ", "))
		}
	}
	return cfg, nil
}
