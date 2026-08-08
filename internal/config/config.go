package config

import (
	"errors"
	"fmt"
	"os"

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
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return cfg, nil
}
