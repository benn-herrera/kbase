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

// TestLoadConfigBuildSwitches: the two `kbase build` switches are declared,
// so strict decoding accepts them — the point of declaring them is that a
// config.toml carrying one is not refused as a typo.
func TestLoadConfigBuildSwitches(t *testing.T) {
	cfg, err := LoadConfig(writeFile(t, t.TempDir(), ConfigFileName,
		"[dev]\nbuild_date = \"2026-01-01\"\ntree_plan = \""+TreePlanMechanical+"\"\n"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Dev.BuildDate != "2026-01-01" {
		t.Errorf("Dev.BuildDate = %q, want 2026-01-01", cfg.Dev.BuildDate)
	}
	mechanical, err := cfg.Dev.MechanicalTreePlan()
	if err != nil || !mechanical {
		t.Errorf("MechanicalTreePlan() = (%v, %v), want (true, nil)", mechanical, err)
	}
}

// TestMechanicalTreePlan: the two shapes differ in whether a provider is
// dialed, so a value that is neither is refused rather than resolved to
// either one.
func TestMechanicalTreePlan(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{"absent", "", false, false},
		{"mechanical", TreePlanMechanical, true, false},
		{"padded", "  " + TreePlanMechanical + "  ", true, false},
		{"misspelled", "mechnical", false, true},
		{"the other shape spelled out", "model", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DevConfig{TreePlan: tc.value}.MechanicalTreePlan()
			if (err != nil) != tc.wantErr {
				t.Fatalf("MechanicalTreePlan(%q) error = %v, want error: %v", tc.value, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("MechanicalTreePlan(%q) = %v, want %v", tc.value, got, tc.want)
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

// TestLoadConfigMechanicalFixture: the fixture the hermetic build recipes
// point `--config-dir` at. It is the only configuration those runs read, so
// if strict decoding ever stops accepting it, four integration recipes stop
// with it — and they would say so in shell rather than here.
func TestLoadConfigMechanicalFixture(t *testing.T) {
	path := filepath.Join("..", "..", "test_data", "fixtures", "config-mechanical", ConfigFileName)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("the committed mechanical fixture no longer loads: %v", err)
	}
	mechanical, err := cfg.Dev.MechanicalTreePlan()
	if err != nil {
		t.Fatalf("the fixture's tree_plan: %v", err)
	}
	if !mechanical {
		t.Error("the mechanical fixture does not select the mechanical tree plan")
	}
	if cfg.Dev.BuildDate == "" {
		t.Error("the mechanical fixture pins no build date, so its recipes cannot assert determinism")
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
