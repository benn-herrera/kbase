package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/result"
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

// TestSelect: the catalogue verbs' choice is the flag, else the pool's only
// entry — a role is never chosen this way.
func TestSelect(t *testing.T) {
	two := Providers{"a": {Name: "a"}, "b": {Name: "b"}}
	for _, tc := range []struct {
		name, flag string
		providers  Providers
		wantName   string
		wantRule   string
		wantErr    string
	}{
		{name: "the flag", flag: "a", providers: two, wantName: "a", wantRule: RuleFlag},
		{name: "the only entry", providers: Providers{"solo": {Name: "solo"}}, wantName: "solo", wantRule: RuleSoleEntry},
		{name: "several entries, none named", providers: two, wantErr: "no provider selected"},
		{name: "no entries", wantErr: "no usable provider"},
		{name: "a name the pool lacks", flag: "typo", providers: Providers{"a": {Name: "a"}}, wantErr: `provider "typo" not found`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			in := Inputs{Providers: tc.providers, WriteDir: dir}
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

// TestUnresolvedProviderRemedy: a name the pool lacks is refused naming the
// table to add, with configure --create as the remedy only where the
// providers.toml does not stand — a standing one it would never overwrite.
func TestUnresolvedProviderRemedy(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name       string
		files      []string
		wantRemedy string
	}{
		{name: "no providers.toml", wantRemedy: "kbase configure --create"},
		{name: "a providers.toml that stands", files: []string{ProvidersPath(dir)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := Inputs{Providers: Providers{"a": {Name: "a"}}, WriteDir: dir, ProvidersFiles: tc.files}
			_, _, err := Select(in, "typo")
			var refusal result.Refusal
			if !errors.As(err, &refusal) || len(refusal) != 1 {
				t.Fatalf("Select = %v, want one refusal", err)
			}
			want := fmt.Sprintf("provider %q not found in %s — add a [typo] table there with its baseUrl", "typo", ProvidersPath(dir))
			if item := refusal[0]; item.Detail != want || item.Remedy != tc.wantRemedy {
				t.Errorf("refusal = %+v, want detail %q and remedy %q", item, want, tc.wantRemedy)
			}
		})
	}
}

// TestReferenceGrammar: a value is provider/model, split at the FIRST slash,
// and a bare id is no reference.
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

// TestResolve: a role's endpoint is the entry its own reference names, with
// the model it names and the provenance its record states.
func TestResolve(t *testing.T) {
	in := scenario(t)
	ep, err := Resolve(in, RoleDecision, "")
	if err != nil {
		t.Fatal(err)
	}
	want := Endpoint{Role: RoleDecision, Provider: "lab", Model: "d3", Rule: RuleConfig,
		Layer: LayerSourceRepository, BaseURL: "https://lab.example/v1", Flavor: FlavorDecisions, Protocol: ProviderAPIAnthropic,
		APIKey: "lab-key", BaseURLSource: LayerSourceRepository, ModelSource: LayerSourceRepository}
	if ep != want {
		t.Errorf("= %+v, want %+v (key left out; len %d, want %d)", shownEndpoint(ep), shownEndpoint(want), len(ep.APIKey), len(want.APIKey))
	}
	// The flag overrides the configured model and says so: the entry is the
	// reference's, the model is the flag's, and the rule is what named it.
	ep, err = Resolve(in, RoleScoring, "lab/judge")
	if err != nil || ep.Provider != "lab" || ep.Model != "judge" || ep.Rule != RuleFlag {
		t.Errorf("= %s/%s under %q, %v; want lab/judge under the flag's rule", ep.Provider, ep.Model, ep.Rule, err)
	}
	// The flag supplies the role the files name no model for: the entry is the
	// flag's, no layer named the model, and the rule is the flag's.
	in.Cfg.Models.Scoring, in.Cfg.Sources.Scoring = "", ""
	ep, err = Resolve(in, RoleScoring, "lab/clef")
	if err != nil || ep.Provider != "lab" || ep.Model != "clef" || ep.Rule != RuleFlag || ep.ModelSource != "" {
		t.Errorf("= %s/%s under %q, model source %q, %v; want the flag to supply lab/clef under its own rule",
			ep.Provider, ep.Model, ep.Rule, ep.ModelSource, err)
	}
}

// shownEndpoint is ep with its key left out, for a failure message.
func shownEndpoint(ep Endpoint) Endpoint {
	ep.APIKey = ""
	return ep
}

// TestResolveKeySource: a role's key source names the store where the store
// supplied the entry's key, whatever the entry's apiKeyFile; the entry's layer
// where its key file did; and nothing where it has no key.
func TestResolveKeySource(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry Provider
		want  string
	}{
		{"a stored key over a key file", Provider{APIKeyFile: "lab.key", KeyStored: true}, string(KeyKindStore)},
		{"a key file", Provider{APIKeyFile: "lab.key"}, LayerSourceGlobal},
		{"no key", Provider{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := scenario(t)
			entry := tc.entry
			entry.Name, entry.BaseURL, entry.Layer = "lab", "https://lab.example/v1", LayerSourceGlobal
			in.Providers = Providers{"lab": entry}
			ep, err := Resolve(in, RoleDecision, "")
			if err != nil {
				t.Fatal(err)
			}
			if ep.KeySource != tc.want {
				t.Errorf("KeySource = %q, want %q", ep.KeySource, tc.want)
			}
		})
	}
}

// TestResolveRefusals: every way a role does not resolve, by its own key —
// never another role's endpoint.
func TestResolveRefusals(t *testing.T) {
	configFile, providersFile := "config.toml", "providers.toml"
	for _, tc := range []struct {
		name    string
		mutate  func(*Inputs)
		role    Role
		flag    string
		key     string
		check   string
		path    string
		allowed []string
		detail  string
	}{
		{name: "no value in the file, and no flag", role: RoleScoring,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "" },
			key:    "models.scoring", path: configFile, detail: "and --scoring-model names no model"},
		{name: "a bare id", role: RoleScoring,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "clef" },
			key:    "models.scoring", path: configFile, detail: "<provider>/<model>"},
		{name: "a provider the pool does not hold", role: RoleScoring,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "elsewhere/clef" },
			key:    "models.scoring", path: providersFile, allowed: []string{"lab"}},
		{name: "a provider that faulted out", role: RoleScoring,
			key: "models.scoring", path: providersFile, detail: `provider "cloud" failed to load: no baseUrl declared`},
		{name: "the flag is not a reference", role: RoleScoring, flag: "clef",
			key: keyScoringModel, check: result.CheckUsage, detail: "<provider>/<model>"},
		{name: "the flag names a provider the pool does not hold, for a role the files leave unset", role: RoleScoring,
			mutate: func(in *Inputs) { in.Cfg.Models.Scoring = "" },
			flag:   "elsewhere/clef", key: keyScoringModel, path: providersFile, allowed: []string{"lab"},
			detail: "--scoring-model names a provider the pool does not hold"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := scenario(t)
			if tc.mutate != nil {
				tc.mutate(&in)
			}
			_, err := Resolve(in, tc.role, tc.flag)
			refusal, ok := asRefusal(err)
			if !ok {
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
			wantCheck := tc.check
			if wantCheck == "" {
				wantCheck = CheckProvider
			}
			if refusal.Check != wantCheck {
				t.Errorf("check = %q, want %q", refusal.Check, wantCheck)
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
	_, err := ResolveFlag(in, RoleScoring, "", keyScoringModel, true)
	refusal, ok := asRefusal(err)
	if !ok {
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

// asRefusal returns the item of err where err is a refusal of exactly one.
func asRefusal(err error) (result.Item, bool) {
	one, ok := err.(result.Refusal)
	if !ok || len(one) != 1 {
		return result.Item{}, false
	}
	return one[0], true
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
			t.Errorf("%s: warning of %d bytes, want warning %t, one line naming no key", tc.name, len(got), tc.warns)
		}
	}
}

// samplingInputs is scenario with an openai entry "lab" and an anthropic entry
// "msg"; text asks lab and scoring asks msg. It states the sampling given, each
// field from the layer the sources name.
func samplingInputs(t *testing.T, sampling SamplingTable, sources Sources) Inputs {
	t.Helper()
	in := scenario(t)
	lab := in.Providers["lab"]
	lab.API = ProviderAPIOpenAI
	in.Providers["lab"] = lab
	in.Providers["msg"] = Provider{Name: "msg", BaseURL: "https://msg.example/v1", API: ProviderAPIAnthropic, Layer: LayerSourceRepository}
	in.Cfg.Models.Scoring, in.Cfg.Sources.Scoring = "msg/clef", LayerSourceRepository
	in.Cfg.Sampling = sampling
	in.Cfg.Sources.TextTemperature, in.Cfg.Sources.TextThinking = sources.TextTemperature, sources.TextThinking
	in.Cfg.Sources.ScoringTemperature, in.Cfg.Sources.ScoringThinking = sources.ScoringTemperature, sources.ScoringThinking
	return in
}

// TestResolveSamplingRefusals: a temperature outside the range the entry's
// chat protocol accepts refuses the role by the key that is wrong, in the write
// target's config.toml.
func TestResolveSamplingRefusals(t *testing.T) {
	fromRepo := Sources{TextTemperature: LayerSourceRepository, ScoringTemperature: LayerSourceRepository, ScoringThinking: LayerSourceRepository}
	for _, tc := range []struct {
		name     string
		role     Role
		sampling SamplingTable
		mutate   func(*Inputs)
		flag     string
		key      string
	}{
		{"openai above its range", RoleText, SamplingTable{Text: RoleSampling{Temperature: new(2.5)}}, nil, "", "sampling.text.temperature"},
		{"openai below zero", RoleText, SamplingTable{Text: RoleSampling{Temperature: new(-0.1)}}, nil, "", "sampling.text.temperature"},
		{"openai NaN", RoleText, SamplingTable{Text: RoleSampling{Temperature: new(math.NaN())}}, nil, "", "sampling.text.temperature"},
		{"anthropic above its range", RoleScoring, SamplingTable{Scoring: RoleSampling{Temperature: new(1.5)}}, nil, "", "sampling.scoring.temperature"},
		{"the model flag moves scoring onto anthropic", RoleScoring,
			SamplingTable{Scoring: RoleSampling{Temperature: new(1.5)}},
			func(in *Inputs) { in.Cfg.Models.Scoring = "lab/clef" }, "msg/other", "sampling.scoring.temperature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := samplingInputs(t, tc.sampling, fromRepo)
			if tc.mutate != nil {
				tc.mutate(&in)
			}
			_, err := Resolve(in, tc.role, tc.flag)
			refusal, ok := asRefusal(err)
			if !ok {
				t.Fatalf("err = %v, want one refusal", err)
			}
			if refusal.Key != tc.key || refusal.Check != CheckProvider || refusal.Path != ConfigPath(in.WriteDir) {
				t.Errorf("refusal = %+v, want %s in %s", refusal, tc.key, ConfigPath(in.WriteDir))
			}
			if !strings.Contains(refusal.Detail, tc.key) || !strings.Contains(refusal.Detail, LayerSourceRepository) || refusal.Remedy == "" {
				t.Errorf("detail %q, remedy %q: want the key and the layer named and a remedy", refusal.Detail, refusal.Remedy)
			}
		})
	}
}

// TestResolveSamplingNamesTheSupplyingLayer: the refusal's path is the write
// target's config.toml even where a lower layer states the value, and its
// detail and remedy name the layer that does.
func TestResolveSamplingNamesTheSupplyingLayer(t *testing.T) {
	in := samplingInputs(t, SamplingTable{Text: RoleSampling{Temperature: new(2.5)}}, Sources{TextTemperature: LayerSourceGlobal})
	_, err := Resolve(in, RoleText, "")
	refusal, ok := asRefusal(err)
	if !ok {
		t.Fatalf("err = %v, want one refusal", err)
	}
	if refusal.Path != ConfigPath(in.WriteDir) {
		t.Errorf("path = %q, want the write target's %q", refusal.Path, ConfigPath(in.WriteDir))
	}
	if !strings.Contains(refusal.Detail, "global layer") || !strings.Contains(refusal.Remedy, "global layer") {
		t.Errorf("detail %q, remedy %q: want the global layer named", refusal.Detail, refusal.Remedy)
	}
}

// TestResolveSamplingAccepts: the top of openai's range, and on anthropic
// thinking alone, a temperature beside thinking off and a temperature beside a
// thinking level, resolve and reach the endpoint with their layers.
func TestResolveSamplingAccepts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		role     Role
		sampling SamplingTable
	}{
		{"openai at 2", RoleText, SamplingTable{Text: RoleSampling{Temperature: new(2.0)}}},
		{"anthropic thinking, no temperature", RoleScoring, SamplingTable{Scoring: RoleSampling{Thinking: ThinkingLow}}},
		{"anthropic thinking off, temperature 0.4", RoleScoring,
			SamplingTable{Scoring: RoleSampling{Temperature: new(0.4), Thinking: ThinkingOff}}},
		{"anthropic thinking low, temperature 0.4", RoleScoring,
			SamplingTable{Scoring: RoleSampling{Temperature: new(0.4), Thinking: ThinkingLow}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := samplingInputs(t, tc.sampling, Sources{TextTemperature: LayerSourceGlobal,
				ScoringTemperature: LayerSourceGlobal, ScoringThinking: LayerSourceRepository})
			ep, err := Resolve(in, tc.role, "")
			if err != nil {
				t.Fatal(err)
			}
			want := tc.sampling.Text
			if tc.role == RoleScoring {
				want = tc.sampling.Scoring
			}
			if (ep.Temperature == nil) != (want.Temperature == nil) || ep.Temperature != nil && *ep.Temperature != *want.Temperature || ep.Thinking != want.Thinking {
				t.Errorf("endpoint carries %v, %q; want %v, %q", ep.Temperature, ep.Thinking, want.Temperature, want.Thinking)
			}
			if want.Temperature != nil && ep.TemperatureSource != LayerSourceGlobal || want.Thinking != "" && ep.ThinkingSource != LayerSourceRepository {
				t.Errorf("sources %q, %q, want the layers that stated the values", ep.TemperatureSource, ep.ThinkingSource)
			}
		})
	}
}

// TestResolveSamplingUnset: an unstated setting leaves the endpoint's fields
// unset, and the decision role carries none even where text states its own.
func TestResolveSamplingUnset(t *testing.T) {
	in := samplingInputs(t, SamplingTable{}, Sources{})
	ep, err := Resolve(in, RoleText, "")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Temperature != nil || ep.Thinking != "" || ep.TemperatureSource != "" || ep.ThinkingSource != "" {
		t.Errorf("endpoint = %+v, want no sampling", shownEndpoint(ep))
	}

	in = samplingInputs(t, SamplingTable{Text: RoleSampling{Temperature: new(0.5), Thinking: ThinkingHigh}}, Sources{
		TextTemperature: LayerSourceGlobal, TextThinking: LayerSourceGlobal})
	ep, err = Resolve(in, RoleDecision, "")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Temperature != nil || ep.Thinking != "" {
		t.Errorf("decision endpoint = %+v, want no sampling", shownEndpoint(ep))
	}
}
