package config

import (
	"path/filepath"
	"strings"
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

// TestLoadConfigReaderConcurrency: [asks] readerConcurrency is optional, and
// a value under 1 is refused naming the key and never the value.
func TestLoadConfigReaderConcurrency(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want *int
		ok   bool
	}{
		{"absent", "provider = \"reaper\"\n", nil, true},
		{"set", "[asks]\nreaderConcurrency = 2\n", new(2), true},
		{"zero", "[asks]\nreaderConcurrency = 0\n", nil, false},
		{"negative", "[asks]\nreaderConcurrency = -3\n", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, tc.body))
			if !tc.ok {
				if err == nil || !strings.Contains(err.Error(), keyReaderConcurrency) || strings.Contains(err.Error(), "-3") {
					t.Errorf("LoadConfig = %v, want a refusal naming %s and no value", err, keyReaderConcurrency)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if got := cfg.Asks.ReaderConcurrency; (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
				t.Errorf("readerConcurrency = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLoadConfigRefusesDev: there is no [dev] table; one fails strict load.
func TestLoadConfigRefusesDev(t *testing.T) {
	if _, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, "[dev]\ntelemetry = true\n")); err == nil || !strings.Contains(err.Error(), `"dev.telemetry"`) {
		t.Errorf("LoadConfig over [dev] = %v, want the key refused", err)
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

// TestLoadConfigUnknownKey: a key config.toml does not model decodes into
// nothing, so a typo silently costs the user the setting they thought they
// changed. The load refuses instead, naming the file and EVERY offending
// key — one refusal, not one per round of trial and error.
func TestLoadConfigUnknownKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []string
		// absent names keys that must NOT appear: the parent table of an
		// unknown key locates nothing the leaf does not.
		absent []string
	}{
		{
			name: "misspelled top-level key",
			body: "provdier = \"reaper\"\n",
			want: []string{`"provdier"`},
		},
		{
			name: "misspelled tier",
			body: "[models]\nheavy = \"" + heavyModel + "\"\nhevy = \"" + lightModel + "\"\n",
			want: []string{`"models.hevy"`},
		},
		{
			name: "misspelled table",
			body: "[modles]\nheavy = \"" + heavyModel + "\"\n",
			want: []string{`"modles.heavy"`},
			// The table itself is unknown too, but "modles.heavy" is
			// where the user's eye needs to land.
			absent: []string{`"modles"`},
		},
		{
			name: "several unknowns in one refusal",
			body: "provdier = \"reaper\"\n[models]\nhevy = \"x\"\n[dev]\ntelemitry = true\n",
			want: []string{`"provdier"`, `"models.hevy"`, `"dev.telemitry"`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), ConfigFileName, tc.body)
			_, err := LoadConfig(path)
			if err == nil {
				t.Fatal("an unknown key decoded into nothing and the load said so: got nil error")
			}
			msg := err.Error()
			for _, want := range append([]string{path, unknownKeyHint}, tc.want...) {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal %q does not name %s", msg, want)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(msg, absent) {
					t.Errorf("refusal %q names the parent table %s as well as its key", msg, absent)
				}
			}
		})
	}
}

// TestLoadConfigFixture: the committed fixture is what a hand-written
// config.toml looks like, so strict decoding must accept it. Only
// config.toml is read here — the fixture providers.toml names a key file,
// and no test reads a credential.
func TestLoadConfigFixture(t *testing.T) {
	path := filepath.Join("..", "..", "test_data", "fixtures", "config", ConfigFileName)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("the committed fixture no longer loads: %v", err)
	}
	if cfg.Provider == "" {
		t.Error("fixture config.toml names no provider")
	}
	for _, tier := range []string{TierHeavy, TierLight} {
		if _, ok := cfg.ModelFor(tier); !ok {
			t.Errorf("fixture config.toml leaves the %s tier unmapped", tier)
		}
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
