package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// keyFile stages a key file holding key and returns its path.
func keyFile(t *testing.T, key string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(p, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestEnvOverrides: the environment reads each variable into its field,
// trimmed.
func TestEnvOverrides(t *testing.T) {
	getenv := func(name string) string {
		switch name {
		case EnvAPIBaseURL:
			return " http://env.example/v1 "
		case EnvDecision:
			return " clef "
		case EnvText:
			return "Qwen3.8-27B"
		case EnvScoring:
			return " scoring-id "
		case EnvAPIKeyFile:
			return "/keys/reaper.txt"
		default:
			return ""
		}
	}
	want := Overrides{BaseURL: "http://env.example/v1", Decision: "clef", Text: "Qwen3.8-27B",
		Scoring: "scoring-id", APIKeyFile: "/keys/reaper.txt"}
	if got := EnvOverrides(getenv); got != want {
		t.Errorf("EnvOverrides = %+v, want %+v", got, want)
	}
	if got := EnvOverrides(func(string) string { return "" }); got != (Overrides{}) {
		t.Errorf("EnvOverrides with an empty environment = %+v, want the zero layer", got)
	}
}

// TestModeOf: the environment either supplies the whole inference
// configuration or nothing. With no inference variable set the files do; with
// the base URL and no file value the environment does; and every other
// combination is a refusal listing every side the user has to decide between.
func TestModeOf(t *testing.T) {
	files := Config{Models: ModelMap{Decision: "p/d", Text: "p/t"}}
	scoringOnly := Config{Models: ModelMap{Scoring: "p/s"}}
	for _, tc := range []struct {
		name string
		cfg  Config
		env  Overrides
		want Mode
		keys []string // the refusal's keys, in order, where it refuses
	}{
		{name: "the files alone", cfg: files, want: ModeFile},
		{name: "no configuration at all", want: ModeFile},
		{name: "the environment alone", env: Overrides{BaseURL: "http://env/v1", Decision: "d", Text: "t"}, want: ModeEnvironment},
		{name: "the key file is no mode", env: Overrides{APIKeyFile: "/keys/k"}, want: ModeFile},
		{name: "the key file beside a file value", cfg: files, env: Overrides{APIKeyFile: "/keys/k"}, want: ModeFile},
		{name: "a role variable with no base URL", env: Overrides{Text: "t"},
			keys: []string{EnvText, EnvAPIBaseURL}},
		{name: "the base URL alone beside a file value", cfg: files, env: Overrides{BaseURL: "http://env/v1"},
			keys: []string{EnvAPIBaseURL, "models.decision", "models.text"}},
		{name: "a role variable beside a file value", cfg: files, env: Overrides{BaseURL: "http://env/v1", Text: "t"},
			keys: []string{EnvAPIBaseURL, EnvText, "models.decision", "models.text"}},
		{name: "the scoring variable is an inference variable", cfg: scoringOnly, env: Overrides{BaseURL: "http://env/v1", Scoring: "s"},
			keys: []string{EnvAPIBaseURL, EnvScoring, "models.scoring"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mode, err := ModeOf(Inputs{Cfg: tc.cfg, Env: tc.env, WriteDir: dir})
			if tc.keys == nil {
				if err != nil || mode != tc.want {
					t.Fatalf("= %q, %v; want %q", mode, err, tc.want)
				}
				return
			}
			var refusals Refusals
			if !asRefusals(err, &refusals) {
				t.Fatalf("err = %v, want refusals", err)
			}
			var keys []string
			for _, r := range refusals {
				keys = append(keys, r.Key)
				if strings.HasPrefix(r.Key, "models.") && r.Path != ConfigPath(dir) {
					t.Errorf("%s names no config.toml: path %q", r.Key, r.Path)
				}
			}
			if strings.Join(keys, ",") != strings.Join(tc.keys, ",") {
				t.Errorf("keys = %q, want %q", keys, tc.keys)
			}
		})
	}
}

// asRefusals reports whether err is the aggregate, keeping its items.
func asRefusals(err error, out *Refusals) bool {
	several, ok := err.(Refusals)
	if ok {
		*out = several
	}
	return ok
}

// TestSelect: the catalogue verbs' choice is the flag, else the pool's only
// entry — a role is never chosen this way, and config.toml's provider key is
// never a fallback.
func TestSelect(t *testing.T) {
	two := Providers{"a": {Name: "a"}, "b": {Name: "b"}}
	for _, tc := range []struct {
		name, flag string
		providers  Providers
		configured string
		wantName   string
		wantRule   string
		wantErr    string
	}{
		{name: "the flag", flag: "a", providers: two, configured: "b", wantName: "a", wantRule: RuleFlag},
		{name: "the only entry", providers: Providers{"solo": {Name: "solo"}}, wantName: "solo", wantRule: RuleSoleEntry},
		{name: "several entries, none named", providers: two, configured: "b", wantErr: "no provider selected"},
		{name: "no entries", wantErr: "no usable provider"},
		{name: "a name the pool lacks", flag: "typo", providers: Providers{"a": {Name: "a"}}, wantErr: `provider "typo" not found`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			in := Inputs{Providers: tc.providers, Cfg: Config{Provider: tc.configured}, WriteDir: dir}
			name, rule, err := Select(in, tc.flag)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || name != tc.wantName || rule != tc.wantRule {
				t.Errorf("= (%q, %q), %v; want (%q, %q)", name, rule, err, tc.wantName, tc.wantRule)
			}
		})
	}
}

// TestSelectNeverReadsTheProviderKey: a config.toml naming one entry of two
// selects nothing — the key is inert, so the verb refuses and lists both.
func TestSelectNeverReadsTheProviderKey(t *testing.T) {
	in := Inputs{Cfg: Config{Provider: "a"}, Providers: Providers{"a": {Name: "a"}, "b": {Name: "b"}}}
	if _, _, err := Select(in, ""); err == nil {
		t.Fatal("the superseded provider key selected an entry")
	}
}

// TestReferenceGrammar: one grammar per mode. A file value is
// provider/model, split at the FIRST slash; an environment value is a bare id
// of the environment's endpoint, which no parser reads.
func TestReferenceGrammar(t *testing.T) {
	for _, tc := range []struct {
		value    string
		provider string
		model    string
		ok       bool
	}{
		{value: "openrouter/anthropic/x", provider: "openrouter", model: "anthropic/x", ok: true},
		{value: " lab/d3 ", provider: "lab", model: "d3", ok: true},
		{value: "d3"},
		{value: "/d3"},
		{value: "lab/"},
		{value: " lab / d3 ", provider: "lab", model: "d3", ok: true},
	} {
		ref, err := ParseReference(tc.value)
		if (err == nil) != tc.ok || ref.Provider != tc.provider || ref.Model != tc.model {
			t.Errorf("ParseReference(%q) = %+v, %v; want %q/%q ok %t", tc.value, ref, err, tc.provider, tc.model, tc.ok)
		}
	}
	if _, err := FileReference("bare"); err == nil {
		t.Error("FileReference accepted a bare id")
	}
}

// scenario is the configuration Resolve is exercised against: one pool entry
// named by a reference, one faulted out of the pool, and the file's values in
// place.
func scenario(t *testing.T) Inputs {
	t.Helper()
	dir := t.TempDir()
	return Inputs{
		Cfg: Config{Models: ModelMap{Decision: "lab/d3", Text: "lab/qwen", Scoring: "cloud/clef"},
			Sources: Sources{Decision: LayerSourceRepository, Text: LayerSourceGlobal, Scoring: LayerSourceRepository}},
		Providers: Providers{"lab": {Name: "lab", BaseURL: "https://lab.example/v1", APIKey: "lab-key",
			DecisionsFlavor: FlavorDecisions, Layer: LayerSourceRepository}},
		Faults:   []ProviderFault{{Name: "cloud", Path: "/cfg/providers.toml", Reason: "no baseUrl declared"}},
		WriteDir: dir,
	}
}

// TestResolveFileMode: a role's endpoint is the entry its own reference names,
// with the model it names and the provenance its record states.
func TestResolveFileMode(t *testing.T) {
	in := scenario(t)
	ep, err := Resolve(in, ModeFile, RoleDecision, "")
	if err != nil {
		t.Fatal(err)
	}
	want := Endpoint{Role: RoleDecision, Mode: ModeFile, Provider: "lab", Model: "d3", Rule: RuleConfig,
		Layer: LayerSourceRepository, BaseURL: "https://lab.example/v1", Flavor: FlavorDecisions, Protocol: ProviderAPIOpenAI,
		APIKey: "lab-key", BaseURLSource: LayerSourceRepository, ModelSource: LayerSourceRepository}
	if ep != want {
		t.Errorf("= %+v, want %+v", ep, want)
	}
	// The key file is no mode: it replaces the key of whichever endpoint is in
	// force, and the dial reads it.
	in.Env = Overrides{APIKeyFile: "~/keys/lab.txt"}
	ep, err = Resolve(in, ModeFile, RoleText, "")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if ep.APIKeyFile != filepath.Join(home, "keys", "lab.txt") || ep.KeySource != LayerSourceEnvironment || ep.Model != "qwen" {
		t.Errorf("= %+v, want the key file expanded and named environment", ep)
	}
	// The flag overrides the configured model and says so: the entry is the
	// reference's, the model is the flag's, and the rule is what named it.
	ep, err = Resolve(in, ModeFile, RoleScoring, "lab/judge")
	if err != nil || ep.Provider != "lab" || ep.Model != "judge" || ep.Rule != RuleFlag {
		t.Errorf("= %+v, %v; want lab/judge under the flag's rule", ep, err)
	}
	// The flag supplies the role the files name no model for: the entry is the
	// flag's, no layer named the model, and the rule is the flag's.
	in.Cfg.Models.Scoring, in.Cfg.Sources.Scoring = "", ""
	ep, err = Resolve(in, ModeFile, RoleScoring, "lab/clef")
	if err != nil || ep.Provider != "lab" || ep.Model != "clef" || ep.Rule != RuleFlag || ep.ModelSource != "" {
		t.Errorf("= %+v, %v; want the flag to supply lab/clef under its own rule", ep, err)
	}
}

// TestResolveEnvironmentMode: the environment's endpoint, its role's bare id,
// and no pool entry behind it.
func TestResolveEnvironmentMode(t *testing.T) {
	in := Inputs{Env: Overrides{BaseURL: "https://env.example/v1", Decision: "d3", Text: "qwen", Scoring: "clef"}}
	ep, err := Resolve(in, ModeEnvironment, RoleText, "")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Provider != "" || ep.Layer != "" || ep.Model != "qwen" || ep.BaseURL != "https://env.example/v1" ||
		ep.Mode != ModeEnvironment || ep.BaseURLSource != LayerSourceEnvironment || ep.ModelSource != LayerSourceEnvironment || ep.KeySource != "" {
		t.Errorf("= %+v, want the environment endpoint and the role's bare id", ep)
	}
	// A value bearing a slash is the endpoint's model, sent verbatim: nothing
	// parses it against a provider the environment does not name.
	ep, err = Resolve(in, ModeEnvironment, RoleText, "org/model")
	if err != nil || ep.Model != "org/model" || ep.ModelSource != "" {
		t.Errorf("= %+v, %v; want the flag verbatim", ep, err)
	}
}

// TestResolveRefusals: every way a role does not resolve, by its own key —
// never another role's endpoint, never the superseded provider key.
func TestResolveRefusals(t *testing.T) {
	configFile, providersFile := "config.toml", "providers.toml"
	for _, tc := range []struct {
		name    string
		mutate  func(*Inputs)
		role    Role
		mode    Mode
		flag    string
		key     string
		check   string
		path    string
		allowed []string
		detail  string
	}{
		{name: "no value in the file, and no flag", role: RoleScoring, mode: ModeFile,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "" },
			key:    "models.scoring", path: configFile, detail: "and --scoring-model names no model"},
		{name: "a bare id", role: RoleScoring, mode: ModeFile,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "clef" },
			key:    "models.scoring", path: configFile, detail: "<provider>/<model>"},
		{name: "a provider the pool does not hold", role: RoleScoring, mode: ModeFile,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "elsewhere/clef" },
			key:    "models.scoring", path: providersFile, allowed: []string{"lab"}},
		{name: "a provider that faulted out", role: RoleScoring, mode: ModeFile,
			key: "models.scoring", path: providersFile, detail: `provider "cloud" failed to load: no baseUrl declared`},
		{name: "the environment names no model for the role", role: RoleScoring, mode: ModeEnvironment,
			mutate: func(in *Inputs) { in.Env = Overrides{BaseURL: "https://env/v1"} },
			key:    EnvScoring},
		{name: "the flag is not a reference", role: RoleScoring, mode: ModeFile, flag: "clef",
			key: keyScoringModel, check: CheckUsage, detail: "<provider>/<model>"},
		{name: "the flag names a provider the pool does not hold, for a role the files leave unset", role: RoleScoring, mode: ModeFile,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "" },
			flag:   "elsewhere/clef", key: keyScoringModel, path: providersFile, allowed: []string{"lab"},
			detail: "--scoring-model names a provider the pool does not hold"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := scenario(t)
			if tc.mutate != nil {
				tc.mutate(&in)
			}
			_, err := Resolve(in, tc.mode, tc.role, tc.flag)
			var refusal Refusal
			if !asRefusal(err, &refusal) {
				t.Fatalf("err = %v, want one refusal", err)
			}
			if refusal.Key != tc.key || (tc.path != "" && strings.TrimPrefix(refusal.Path, in.WriteDir+"/") != tc.path) {
				t.Errorf("named %s in %s, want %s in %s", refusal.Key, refusal.Path, tc.key, tc.path)
			}
			if tc.allowed != nil && strings.Join(refusal.Allowed, ",") != strings.Join(tc.allowed, ",") {
				t.Errorf("allowed = %q, want %q", refusal.Allowed, tc.allowed)
			}
			if tc.detail != "" && !strings.Contains(refusal.Detail, tc.detail) {
				t.Errorf("detail = %q, want %q", refusal.Detail, tc.detail)
			}
			if refusal.Check != tc.check {
				t.Errorf("check = %q, want %q", refusal.Check, tc.check)
			}
		})
	}
}

// TestResolveDefaultRefusalNamesTheRoleFirst: a verb whose role is its own
// default — nothing named a selection — is refused by the role's key as the
// first remedy and by the flag as the other, and never as though the flag had
// been passed.
func TestResolveDefaultRefusalNamesTheRoleFirst(t *testing.T) {
	in := scenario(t)
	in.Cfg.Models.Scoring = ""
	_, err := ResolveFlag(in, ModeFile, RoleScoring, "", keyScoringModel, true)
	var refusal Refusal
	if !asRefusal(err, &refusal) {
		t.Fatalf("err = %v, want one refusal", err)
	}
	if refusal.Key != "models.scoring" || refusal.Path != ConfigPath(in.WriteDir) || refusal.Remedy != configureRemedy {
		t.Errorf("named %s in %s, remedy %q; want the role's key and file", refusal.Key, refusal.Path, refusal.Remedy)
	}
	for _, want := range []string{"sets no models.scoring", "nothing named a selection",
		"the scoring role is the default", keyScoringModel} {
		if !strings.Contains(refusal.Detail, want) {
			t.Errorf("detail = %q, want %q", refusal.Detail, want)
		}
	}
	if strings.Contains(refusal.Detail, "names no model") {
		t.Errorf("detail = %q reads as though the flag had been passed", refusal.Detail)
	}
}

// asRefusal reports whether err is exactly one refusal.
func asRefusal(err error, out *Refusal) bool {
	one, ok := err.(Refusal)
	if ok {
		*out = one
	}
	return ok
}

func TestCleartextWarning(t *testing.T) {
	for _, tc := range []struct {
		name, url, key string
		warns          bool
	}{
		{"no key", "http://reaper.local:4000/v1", "", false},
		{"loopback by name", "http://localhost:4000/v1", "k", false},
		{"loopback by address", "http://127.0.0.1:4000/v1", "k", false},
		{"loopback v6", "http://[::1]:4000/v1", "k", false},
		{"https", "https://reaper.local/v1", "k", false},
		{"non-loopback with key", "http://reaper.local:4000/v1", "secret-key", true},
	} {
		got := CleartextWarning(tc.url, tc.key)
		if (got != "") != tc.warns || strings.Contains(got, "secret-key") || strings.Contains(got, "\n") {
			t.Errorf("%s: %q, want warning %t, one line naming no key", tc.name, got, tc.warns)
		}
	}
}
