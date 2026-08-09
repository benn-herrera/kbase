package config

import (
	"path/filepath"
	"testing"
)

const (
	heavyModel = "gemma-4-31b-it"
	lightModel = "gemma-4-26b-a4b-it"
)

func TestLoadConfig(t *testing.T) {
	path := writeFile(t, t.TempDir(), ConfigFileName, `
provider = "reaper"

[models]
heavy = "`+heavyModel+`"
light = "`+lightModel+`"
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Provider != "reaper" {
		t.Errorf("Provider = %q, want reaper", cfg.Provider)
	}
	if cfg.Models.Heavy != heavyModel || cfg.Models.Light != lightModel {
		t.Errorf("Models = %+v, want %q/%q", cfg.Models, heavyModel, lightModel)
	}
}

// TestLoadConfigDevSwitches: [dev] is optional and every switch is off
// until the file says otherwise. An absent table and an explicit `false`
// must be the same state — a diagnostic that turns itself on because a
// user never wrote the table down is a diagnostic nobody asked for.
func TestLoadConfigDevSwitches(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"absent table", "provider = \"reaper\"\n", false},
		{"empty table", "[dev]\n", false},
		{"explicitly off", "[dev]\ntelemetry = false\n", false},
		{"on", "[dev]\ntelemetry = true\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, tc.body))
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.Dev.Telemetry != tc.want {
				t.Errorf("Dev.Telemetry = %v, want %v", cfg.Dev.Telemetry, tc.want)
			}
		})
	}
}

// TestLoadConfigMissing: a fresh install has no config.toml. That is the
// unconfigured state, not a failure — the caller decides which gaps are
// fatal for the command it is running.
func TestLoadConfigMissing(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), ConfigFileName))
	if err != nil {
		t.Fatalf("LoadConfig missing: %v", err)
	}
	if cfg != (Config{}) {
		t.Errorf("missing config should yield zero Config, got %+v", cfg)
	}
}

func TestLoadConfigMalformed(t *testing.T) {
	path := writeFile(t, t.TempDir(), ConfigFileName, "[models\nbroken")
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error from malformed config.toml, got nil")
	}
}

// TestModelFor: an unmapped or unknown tier reports not-configured. The
// caller must fail loud there; substituting the other tier's model is a
// silent wrong answer.
func TestModelFor(t *testing.T) {
	full := Config{Models: ModelMap{Heavy: heavyModel, Light: lightModel}}
	heavyOnly := Config{Models: ModelMap{Heavy: heavyModel}}

	for _, tc := range []struct {
		name   string
		cfg    Config
		tier   string
		want   string
		wantOK bool
	}{
		{"heavy mapped", full, TierHeavy, heavyModel, true},
		{"light mapped", full, TierLight, lightModel, true},
		{"light unmapped", heavyOnly, TierLight, "", false},
		{"zero config", Config{}, TierHeavy, "", false},
		{"unknown tier", full, "medium", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.cfg.ModelFor(tc.tier)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ModelFor(%q) = (%q, %v), want (%q, %v)", tc.tier, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
