package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Model categories. The build names a category, never a model id: which model
// serves the decision category is a deployment fact that belongs in
// config.toml, while "the letter asks are decision-model calls" is a build
// fact that belongs in the code. These constants are the single source of the
// category names — the `toml` tags on ModelMap must mirror them (struct tags
// cannot reference constants).
const (
	// CategoryDecision is the claim graph's letter asks' category: the
	// decision model that answers them.
	CategoryDecision = "decision"
	// CategoryText is the overview passage's category.
	CategoryText = "text"
)

// Config is the kbase settings file (config.toml) — the choices that draw
// from the providers.toml pool. providers.toml lists what is available;
// config.toml says which to use. It holds no credentials: every key lives
// on its providers.toml pool entry, which is what keeps config.toml safe
// to read and edit.
type Config struct {
	// Provider names the active providers.toml pool entry.
	Provider string `toml:"provider"`

	// Models maps the build's categories onto model ids at that provider.
	Models ModelMap `toml:"models"`

	// Asks is how the build puts its letter asks.
	Asks AsksConfig `toml:"asks"`

	// Sources names the layer that supplied each merged model id — one of
	// LayerSourceRepository, LayerSourceGlobal, LayerSourceExplicit or
	// LayerSourceEnvironment. It is set by Load's merge, never read from a
	// file: one config.toml cannot say which layer it is.
	Sources Sources `toml:"-"`
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

// ModelMap is the [models] table: the category→model-id resolution, written by
// `kbase configure` or by hand. An unset category is not defaulted — a guessed
// model id is a silent wrong answer, so callers fail loud instead.
type ModelMap struct {
	Decision string `toml:"decision"`
	Text     string `toml:"text"`
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
