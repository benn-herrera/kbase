package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Model tiers. The build names a tier, never a model id: which model serves
// "heavy" is a deployment fact that belongs in config.toml, while "the
// overview passage is heavy work" is a build fact that belongs in the code.
// These constants are the single source of the tier names — the `toml` tags
// on ModelMap must mirror them (struct tags cannot reference constants).
const (
	// TierHeavy is the overview passage's tier.
	TierHeavy = "heavy"
	// TierLight is the claim graph's letter asks' tier.
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

	// Models maps the build's tiers onto model ids at that provider.
	Models ModelMap `toml:"models"`

	// Asks is how the build puts its letter asks.
	Asks AsksConfig `toml:"asks"`
}

// AsksConfig is the [asks] table.
type AsksConfig struct {
	// ReaderConcurrency is how many asks of one group are in flight once its
	// first has returned; nil, the key absent, leaves the build's default.
	ReaderConcurrency *int `toml:"readerConcurrency"`
}

// keyReaderConcurrency is AsksConfig's key by its full dotted path, as a
// message names it.
const keyReaderConcurrency = "asks.readerConcurrency"

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
	if n := cfg.Asks.ReaderConcurrency; n != nil && *n < 1 {
		return Config{}, fmt.Errorf("config: %s: %s is a whole number of asks, at least 1", path, keyReaderConcurrency)
	}
	return cfg, nil
}
