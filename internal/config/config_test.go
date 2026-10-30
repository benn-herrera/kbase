package config

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	decisionModel = "clef"
	textModel     = "Qwen3.8-27B"
	scoringModel  = "clef-scoring"
)

func TestLoadConfig(t *testing.T) {
	path := writeFile(t, t.TempDir(), ConfigFileName, `
[models]
decision = "`+decisionModel+`"
text = "`+textModel+`"
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Models.Decision != decisionModel || cfg.Models.Text != textModel {
		t.Errorf("Models = %+v, want %q/%q", cfg.Models, decisionModel, textModel)
	}
}

// TestLoadConfigConcurrency: [asks] concurrency is optional, and a value
// under 1 is refused naming asks.concurrency and never the value.
func TestLoadConfigConcurrency(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want *int
		ok   bool
	}{
		{"absent", "[models]\n", nil, true},
		{"set", "[asks]\nconcurrency = 3\n", new(3), true},
		{"zero", "[asks]\nconcurrency = 0\n", nil, false},
		{"negative", "[asks]\nconcurrency = -3\n", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, tc.body))
			if !tc.ok {
				if err == nil || !strings.Contains(err.Error(), "asks.concurrency") || strings.Contains(err.Error(), "-3") {
					t.Errorf("LoadConfig = %v, want a refusal naming asks.concurrency and no value", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if got := cfg.Asks.Concurrency; (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
				t.Errorf("concurrency = %v, want %v", got, tc.want)
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
			name: "the removed provider key",
			body: "provider = \"x\"\n",
			want: []string{`"provider"`},
		},
		{
			name: "the removed readerConcurrency key",
			body: "[asks]\nreaderConcurrency = 2\n",
			want: []string{`"asks.readerConcurrency"`},
		},
		{
			name: "misspelled category",
			body: "[models]\ndecision = \"" + decisionModel + "\"\ndecisoin = \"" + textModel + "\"\n",
			want: []string{`"models.decisoin"`},
		},
		{
			name: "misspelled table",
			body: "[modles]\ndecision = \"" + decisionModel + "\"\n",
			want: []string{`"modles.decision"`},
			// The table itself is unknown too, but "modles.heavy" is
			// where the user's eye needs to land.
			absent: []string{`"modles"`},
		},
		{
			name: "several unknowns in one refusal",
			body: "provdier = \"reaper\"\n[models]\ndecisoin = \"x\"\n[dev]\ntelemitry = true\n",
			want: []string{`"provdier"`, `"models.decisoin"`, `"dev.telemitry"`},
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

func TestLoadConfigMalformed(t *testing.T) {
	path := writeFile(t, t.TempDir(), ConfigFileName, "[models\nbroken")
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error from malformed config.toml, got nil")
	}
}

// TestModelFor: an unmapped or unknown category reports not-configured. The
// caller must fail loud there; substituting the other category's model is a
// silent wrong answer.
func TestModelFor(t *testing.T) {
	full := Config{Models: ModelMap{Decision: decisionModel, Text: textModel}}
	decisionOnly := Config{Models: ModelMap{Decision: decisionModel}}

	for _, tc := range []struct {
		name     string
		cfg      Config
		category string
		want     string
		wantOK   bool
	}{
		{"decision mapped", full, CategoryDecision, decisionModel, true},
		{"text mapped", full, CategoryText, textModel, true},
		{"text unmapped", decisionOnly, CategoryText, "", false},
		{"zero config", Config{}, CategoryDecision, "", false},
		{"unknown category", full, "medium", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.cfg.ModelFor(tc.category)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ModelFor(%q) = (%q, %v), want (%q, %v)", tc.category, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestLoadConfigEveryThinkingLevel: each level of the vocabulary loads.
func TestLoadConfigEveryThinkingLevel(t *testing.T) {
	for _, level := range ThinkingLevels {
		t.Run(level, func(t *testing.T) {
			cfg, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, "[sampling.text]\nthinking = \""+level+"\"\n"))
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if got := cfg.Sampling.Text.Thinking; got != level {
				t.Errorf("thinking = %q, want %q", got, level)
			}
		})
	}
}

// TestLoadConfigSampling: a thinking value outside the vocabulary and a
// temperature that is no number are refused at load, a role the schema does not
// model fails strict load, and an integer temperature loads as the number it
// is.
func TestLoadConfigSampling(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []string // substrings of the refusal; none means it loads
	}{
		{"unknown level", "[sampling.text]\nthinking = \"max\"\n",
			[]string{"sampling.text.thinking", "off, low, medium, high"}},
		{"unknown level on scoring", "[sampling.scoring]\nthinking = \"Low\"\n",
			[]string{"sampling.scoring.thinking", "off, low, medium, high"}},
		{"the decision role takes no sampling", "[sampling.decision]\ntemperature = 0.5\n",
			[]string{"sampling.decision.temperature", "unknown key"}},
		{"temperature is no number", "[sampling.text]\ntemperature = \"hot\"\n", []string{"parse", ConfigFileName}},
		{"an explicit empty level", "[sampling.text]\nthinking = \"\"\n",
			[]string{"sampling.text.thinking", "off, low, medium, high"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, tc.body))
			if tc.want == nil {
				if err != nil {
					t.Fatalf("LoadConfig: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("LoadConfig accepted it")
			}
			for _, s := range tc.want {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("error %q lacks %q", err, s)
				}
			}
		})
	}

	if _, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, "[sampling.text]\nthinking = \"max\"\n")); err == nil || strings.Contains(err.Error(), `"max"`) {
		t.Errorf("error %v, want the bad value not echoed", err)
	}

	cfg, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName, "[sampling.scoring]\ntemperature = 1\n"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := cfg.Sampling.Scoring.Temperature; got == nil || *got != 1.0 {
		t.Errorf("temperature = %v, want 1.0 from the integer 1", got)
	}
	if cfg.Sampling.Text.Temperature != nil || cfg.Sampling.Text.Thinking != "" {
		t.Errorf("text sampling = %+v, want it unset", cfg.Sampling.Text)
	}
}
