package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	toolresult "kbase/internal/result"
)

// The key material the fixture configuration carries. No test lets it reach the
// document.
const fileKey = "sk-file-MUST-NOT-APPEAR"

// configFixture stages a configuration directory with both files: a pool whose
// entries cover both faults the report distinguishes — one that drops for want
// of a base URL, one supplying its key from a file any local account can read —
// and a config.toml naming the key-file entry with both categories set.
func configFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, `[broken]

[file]
baseUrl = "http://file.example/v1"
apiKeyFile = "key.txt"
`)
	keyFile := writeValues(t, dir, "key.txt", fileKey+"\n")
	if err := os.Chmod(keyFile, 0o644); err != nil {
		t.Fatal(err)
	}
	writeValues(t, dir, config.ConfigFileName, "provider = \"file\"\n\n[models]\ndecision = \"dec-1\"\ntext = \"text-1\"\n")
	return dir
}

// unsetInferenceEnv blanks the four inference variables, so a fixture reports
// the configuration rather than whatever the host's environment supplies.
func unsetInferenceEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{config.EnvAPIBaseURL, config.EnvDecision, config.EnvText, config.EnvAPIKeyFile} {
		t.Setenv(name, "")
	}
}

// runShowConfigVerb runs `kbase show-config` through the real root command from
// work and reads the document it wrote.
func runShowConfigVerb(t *testing.T, work string, args ...string) (doc resultDoc, stdout string, code int) {
	t.Helper()
	stdout, code = kbase(t, work, append([]string{"show-config"}, args...)...)
	return checkDocument(t, "show-config", stdout), stdout, code
}

// reportedLayers is the report's layers, in precedence order, each as the
// mapping the document carries.
func reportedLayers(t *testing.T, doc resultDoc) []map[string]any {
	t.Helper()
	layers := doc.items("layers")
	if len(layers) == 0 {
		t.Fatalf("the document reports no layers: %v", doc.Values)
	}
	return layers
}

// TestSelectProviderNameRules: each of the three rules answers in its own
// turn — the flag, then config.toml's provider, then the pool's sole entry —
// and names which one did; with several entries and neither named, none does.
func TestSelectProviderNameRules(t *testing.T) {
	two := config.Providers{"a": provider("a", "http://a.example/v1"), "b": provider("b", "http://b.example/v1")}
	one := config.Providers{"solo": provider("solo", "http://solo.example/v1")}
	providersFile := config.ProvidersPath("/cfg")
	for _, tc := range []struct {
		name             string
		flag, configured string
		providers        config.Providers
		wantName         string
		wantRule         providerRule
		wantErr          bool
	}{
		{name: "flag", flag: "a", configured: "b", providers: two, wantName: "a", wantRule: providerRuleFlag},
		{name: "flag over the sole entry", flag: "solo", providers: one, wantName: "solo", wantRule: providerRuleFlag},
		{name: "config", configured: "b", providers: two, wantName: "b", wantRule: providerRuleConfig},
		{name: "config over the sole entry", flag: "  ", configured: "solo", providers: one, wantName: "solo", wantRule: providerRuleConfig},
		{name: "sole entry", providers: one, wantName: "solo", wantRule: providerRuleSoleEntry},
		{name: "two entries, none named", providers: two, wantErr: true},
		{name: "no entries", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name, rule, err := selectProviderName(tc.flag, tc.configured, tc.providers, providersFile)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %t", err, tc.wantErr)
			}
			if name != tc.wantName || rule != tc.wantRule {
				t.Errorf("(name, rule) = (%q, %q), want (%q, %q)", name, rule, tc.wantName, tc.wantRule)
			}
		})
	}
}

// TestShowConfigReportsTheConfiguration: the document names the layer the flag
// chose with what it holds, every pool entry with the layer it came from, the
// kind of key it supplies and why it faulted, the merged choices, the selection
// and its rule, the resolved endpoint, and one diagnostic per fault — and no
// key material.
func TestShowConfigReportsTheConfiguration(t *testing.T) {
	unsetInferenceEnv(t)
	dir := configFixture(t)
	doc, stdout, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	layers := reportedLayers(t, doc)
	if len(layers) != 1 {
		t.Fatalf("layers = %v, want the one directory the flag named", layers)
	}
	if want := (map[string]any{"path": dir, "source": config.LayerSourceExplicit,
		"exists": true, "providersExists": true, "configExists": true}); !reflect.DeepEqual(layers[0], want) {
		t.Errorf("layers[0] = %v, want %v", layers[0], want)
	}

	var names []string
	entries := map[string]map[string]any{}
	for _, p := range doc.items("providers") {
		name, _ := p["name"].(string)
		names = append(names, name)
		entries[name] = p
	}
	if !slices.Equal(names, []string{"broken", "file"}) {
		t.Errorf("providers %v, want every entry in name order", names)
	}
	if broken := entries["broken"]; broken["usable"] != false || broken["fault"] == nil ||
		broken["layer"] != config.LayerSourceExplicit {
		t.Errorf("the dropped entry = %v, want usable false, the reason it dropped and its layer", broken)
	} else if _, ok := broken["base-url"]; ok {
		t.Errorf("the dropped entry carries base-url: %v", broken)
	} else if _, ok := broken["key"]; ok {
		t.Errorf("the dropped entry carries key: %v", broken)
	}
	if file := entries["file"]; file["key"] != string(config.KeyKindFile) || file["key-file"] != "key.txt" ||
		file["usable"] != true || file["fault"] == nil || file["layer"] != config.LayerSourceExplicit {
		t.Errorf("the key-file entry = %v, want key file, its path, usable, the permission warning and its layer", file)
	}

	cfg, _ := doc.Values["config"].(map[string]any)
	models, _ := cfg["models"].(map[string]any)
	if cfg["provider"] != "file" || models[config.CategoryDecision] != "dec-1" || models[config.CategoryText] != "text-1" {
		t.Errorf("config = %v, want the merged choices", cfg)
	}
	if env, _ := doc.Values["environment"].([]any); len(env) != 0 {
		t.Errorf("environment = %v, want the variables set — none here", doc.Values["environment"])
	}
	selection, _ := doc.Values["selection"].(map[string]any)
	if selection["provider"] != "file" || selection["rule"] != string(providerRuleConfig) {
		t.Errorf("selection = %v, want the provider config.toml names and that rule", selection)
	}
	inference, _ := doc.Values["inference"].(map[string]any)
	if inference["name"] != "file" || inference["base-url"] != "http://file.example/v1" ||
		inference["decision"] != "dec-1" || inference["text"] != "text-1" {
		t.Errorf("inference = %v, want the resolved endpoint and category ids", inference)
	}
	sources, _ := inference["sources"].(map[string]any)
	if sources["base-url"] != config.LayerSourceExplicit || sources["key"] != config.LayerSourceExplicit ||
		sources["decision"] != config.LayerSourceExplicit || sources["text"] != config.LayerSourceExplicit {
		t.Errorf("inference sources = %v, want the one layer read for every field", sources)
	}

	diagnostics := doc.items("diagnostics")
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %v, want one per fault: the drop and the permission warning", diagnostics)
	}
	for _, item := range diagnostics {
		if item["check"] != checkProvider || item["path"] != config.ProvidersPath(dir) || item["detail"] == "" {
			t.Errorf("diagnostic %v, want check %s, path the pool and the reason it gives", item, checkProvider)
		}
	}
	if strings.Contains(stdout, fileKey) {
		t.Errorf("the document carries the key %q:\n%s", fileKey, stdout)
	}
}

// TestShowConfigReportsAnAbsentConfiguration: with no configuration directory
// at all the verb still reports — done, the one layer it read present and empty,
// nothing selected, no endpoint resolved, and one diagnostic naming the pool's
// baseUrl — and it creates no directory.
func TestShowConfigReportsAnAbsentConfiguration(t *testing.T) {
	unsetInferenceEnv(t)
	dir := filepath.Join(t.TempDir(), "absent")
	t.Setenv(config.EnvConfigDir, dir)
	doc, stdout, code := runShowConfigVerb(t, t.TempDir())
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	layers := reportedLayers(t, doc)
	if want := (map[string]any{"path": dir, "source": config.LayerSourceEnvironment,
		"exists": false, "providersExists": false, "configExists": false}); len(layers) != 1 || !reflect.DeepEqual(layers[0], want) {
		t.Errorf("layers = %v, want %v", layers, want)
	}
	if providers, _ := doc.Values["providers"].([]any); len(providers) != 0 {
		t.Errorf("providers = %v, want none", doc.Values["providers"])
	}
	if cfg, _ := doc.Values["config"].(map[string]any); cfg["provider"] != "" {
		t.Errorf("config = %v, want no provider named", cfg)
	}
	if doc.Values["inference"] != nil {
		t.Errorf("inference = %v, want null where nothing resolves", doc.Values["inference"])
	}
	if selection, _ := doc.Values["selection"].(map[string]any); selection["provider"] != nil || selection["detail"] == nil {
		t.Errorf("selection = %v, want the selection's own reason and no provider", selection)
	}
	diagnostics := doc.items("diagnostics")
	if want := filepath.Join(dir, config.ProvidersFileName); len(diagnostics) != 1 || diagnostics[0]["key"] != "baseUrl" || diagnostics[0]["path"] != want {
		t.Fatalf("diagnostics = %v, want one naming baseUrl in %s", diagnostics, want)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("the verb created the configuration directory")
	}
}

// TestShowConfigNamesTheConfigFileALackBelongsTo: a category no layer sets is
// diagnosed against the config.toml of the layer this invocation writes into,
// by its path: a repository layer and a global one can each hold a config.toml,
// so the bare name would not say which.
func TestShowConfigNamesTheConfigFileALackBelongsTo(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, "[solo]\nbaseUrl = \"http://solo.example/v1\"\n")
	doc, stdout, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if doc.Values["inference"] != nil {
		t.Errorf("inference = %v, want null: no category resolves", doc.Values["inference"])
	}
	diagnostics := doc.items("diagnostics")
	want := filepath.Join(dir, config.ConfigFileName)
	if len(diagnostics) != 1 || diagnostics[0]["key"] != "models.decision" || diagnostics[0]["path"] != want {
		t.Fatalf("diagnostics = %v, want one naming models.decision in %s", diagnostics, want)
	}
}

// TestShowConfigReportsAnEndpointTheEnvironmentSupplies: a base URL the
// environment supplies resolves inference with no pool entry selected — the
// selection still says why it named none, and nothing is diagnosed, because
// nothing is missing.
func TestShowConfigReportsAnEndpointTheEnvironmentSupplies(t *testing.T) {
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, "[a]\nbaseUrl = \"http://a.example/v1\"\n\n[b]\nbaseUrl = \"http://b.example/v1\"\n")
	t.Setenv(config.EnvAPIBaseURL, "http://env.example/v1")
	t.Setenv(config.EnvDecision, "env-decision")
	t.Setenv(config.EnvText, "env-text")
	t.Setenv(config.EnvAPIKeyFile, "")
	doc, stdout, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if selection, _ := doc.Values["selection"].(map[string]any); selection["provider"] != nil || selection["detail"] == nil {
		t.Errorf("selection = %v, want the reason no entry was selected", selection)
	}
	if inference, _ := doc.Values["inference"].(map[string]any); inference["name"] != "environment" || inference["base-url"] != "http://env.example/v1" {
		t.Errorf("inference = %v, want the endpoint the environment names", inference)
	} else if sources, _ := inference["sources"].(map[string]any); sources["base-url"] != config.LayerSourceEnvironment ||
		sources["decision"] != config.LayerSourceEnvironment || sources["text"] != config.LayerSourceEnvironment || sources["key"] != nil {
		t.Errorf("inference sources = %v, want the environment for the three variables set and null for the key", sources)
	}
	env := anyStrings(doc.Values["environment"])
	if !slices.Equal(env, []string{config.EnvAPIBaseURL, config.EnvDecision, config.EnvText}) {
		t.Errorf("environment = %v, want the three names set, sorted, and no value", env)
	}
	if diagnostics := doc.items("diagnostics"); len(diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none: nothing is missing", diagnostics)
	}
}

// TestShowConfigRefusesAConfigurationThatDoesNotLoad: a file failing the strict
// load refuses exactly as it does for every other subcommand, and a refused
// document carries no diagnostics.
func TestShowConfigRefusesAConfigurationThatDoesNotLoad(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	writeValues(t, dir, config.ConfigFileName, "provider = \"a\"\n\n[models]\nheavy = \"some-model\"\n")
	doc, stdout, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != exitCodes[toolresult.Refused] || doc.Outcome != toolresult.Refused {
		t.Fatalf("outcome %s, exit %d; want refused, %d\n%s", doc.Outcome, code, exitCodes[toolresult.Refused], stdout)
	}
	if _, ok := doc.Values["diagnostics"]; ok {
		t.Errorf("a refused document carries diagnostics:\n%s", stdout)
	}
	if items := doc.items(toolresult.RefusalsKey); len(items) != 1 || items[0]["check"] != checkUsage {
		t.Errorf("refusals = %v, want the load's one usage item", items)
	}
}

// TestShowConfigReportsTheDecisionsFlavor: every usable entry states which
// decision surface it speaks — the systemone default where the entry is
// silent, the value where it names one — and a dropped provider, which has no
// entry to describe, states none.
func TestShowConfigReportsTheDecisionsFlavor(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, `[silent]
baseUrl = "http://silent.example/v1"

[deciding]
baseUrl = "http://deciding.example/v1"
decisionsFlavor = "decisions"

[broken]
`)
	writeValues(t, dir, config.ConfigFileName, "provider = \"deciding\"\n\n[models]\ndecision = \"dec-1\"\ntext = \"text-1\"\n")

	doc, stdout, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	entries := map[string]map[string]any{}
	for _, p := range doc.items("providers") {
		name, _ := p["name"].(string)
		entries[name] = p
	}
	if got := entries["silent"]["decisions-flavor"]; got != config.FlavorSystemOne {
		t.Errorf("an entry silent on the flavour reports %v, want %s", got, config.FlavorSystemOne)
	}
	if got := entries["deciding"]["decisions-flavor"]; got != config.FlavorDecisions {
		t.Errorf("an entry naming decisions reports %v, want %s", got, config.FlavorDecisions)
	}
	if got, ok := entries["broken"]["decisions-flavor"]; ok {
		t.Errorf("the dropped entry states a decision surface (%v): %v", got, entries["broken"])
	}
}
