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
// and a config.toml naming the key-file entry in each role's reference, the
// superseded provider key beside them.
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
	writeValues(t, dir, config.ConfigFileName, "provider = \"file\"\n\n[models]\ndecision = \"file/dec-1\"\ntext = \"file/text-1\"\n")
	return dir
}

// unsetInferenceEnv blanks the inference variables, so a fixture reports the
// configuration rather than whatever the host's environment supplies.
func unsetInferenceEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{config.EnvAPIBaseURL, config.EnvDecision, config.EnvText, config.EnvScoring, config.EnvAPIKeyFile} {
		t.Setenv(name, "")
	}
}

// runShowConfigVerb runs `kbase show-config` through the real root command from
// work and reads the document it wrote, with the verb's stderr.
func runShowConfigVerb(t *testing.T, work string, args ...string) (doc resultDoc, stdout, stderr string, code int) {
	t.Helper()
	t.Chdir(work)
	stdoutBuf, stderrBuf, err := executeRoot(t, append([]string{"show-config"}, args...)...)
	code = exitStatus(err, stdoutBuf, stderrBuf)
	if sub, _, ferr := rootCmd.Find(args); ferr == nil {
		if err := resetFlags(sub); err != nil {
			t.Fatal(err)
		}
	}
	stdout, stderr = stdoutBuf.String(), stderrBuf.String()
	return checkDocument(t, "show-config", stdout), stdout, stderr, code
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

// TestShowConfigReportsTheConfiguration: the document names the layer the flag
// chose with what it holds, every pool entry with the layer it came from, the
// kind of key it supplies and why it faulted, the merged role values, the
// selection, the mode, one record per role with its endpoint and each field's
// source, and one diagnostic per fault and per unresolved role — and no key
// material.
func TestShowConfigReportsTheConfiguration(t *testing.T) {
	unsetInferenceEnv(t)
	dir := configFixture(t)
	doc, stdout, stderr, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	layers := reportedLayers(t, doc)
	if len(layers) != 1 {
		t.Fatalf("layers = %v, want the one directory the flag named", layers)
	}
	if want := (map[string]any{"path": dir, "source": config.LayerSourceExplicit, "exists": true,
		"providersExists": true, "configExists": true, "harnessesExists": false}); !reflect.DeepEqual(layers[0], want) {
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
	if _, ok := cfg["provider"]; ok {
		t.Errorf("config carries the superseded provider key: %v", cfg)
	}
	if models[config.CategoryDecision] != "file/dec-1" || models[config.CategoryText] != "file/text-1" ||
		models[config.CategoryScoring] != nil {
		t.Errorf("config = %v, want each role's value as written and null for the unset one", cfg)
	}
	if env, _ := doc.Values["environment"].([]any); len(env) != 0 {
		t.Errorf("environment = %v, want the variables set — none here", doc.Values["environment"])
	}
	selection, _ := doc.Values["selection"].(map[string]any)
	if selection["provider"] != "file" || selection["rule"] != config.RuleSoleEntry {
		t.Errorf("selection = %v, want the pool's only usable entry — config.toml names none", selection)
	}
	if doc.Values["mode"] != string(config.ModeFile) {
		t.Errorf("mode = %v, want %q", doc.Values["mode"], config.ModeFile)
	}
	for _, want := range []struct{ role, model, provider string }{
		{"decision", "dec-1", "file"}, {"text", "text-1", "file"},
	} {
		got := roleOf(doc, want.role)
		if got == nil {
			t.Fatalf("no %s record in %v", want.role, doc.items("roles"))
		}
		if got["back-end"] != "provider" || got["provider"] != want.provider || got["provider-rule"] != config.RuleConfig ||
			got["provider-layer"] != config.LayerSourceExplicit || got["model"] != want.model ||
			got["base-url"] != "http://file.example/v1" {
			t.Errorf("%s = %v, want the entry its reference names", want.role, got)
		}
		sources, _ := got["sources"].(map[string]any)
		if sources["base-url"] != config.LayerSourceExplicit || sources["model"] != config.LayerSourceExplicit ||
			sources["key"] != config.LayerSourceExplicit {
			t.Errorf("%s sources = %v, want the one layer read for every field", want.role, sources)
		}
	}
	// The unset role resolves nothing, and says what it lacks.
	scoring := roleOf(doc, "scoring")
	if scoring["detail"] == nil || scoring["back-end"] != nil {
		t.Errorf("the scoring role = %v, want a detail and no back-end: nothing answered it", scoring)
	}

	diagnostics := doc.items("diagnostics")
	if len(diagnostics) != 3 {
		t.Fatalf("diagnostics = %v, want one per fault and one for the unset role", diagnostics)
	}
	for _, item := range diagnostics[:2] {
		if item["check"] != checkProvider || item["path"] != config.ProvidersPath(dir) || item["detail"] == "" {
			t.Errorf("diagnostic %v, want check %s, path the pool and the reason it gives", item, checkProvider)
		}
	}
	if last := diagnostics[2]; last["check"] != checkProvider || last["key"] != "models.scoring" || last["path"] != config.ConfigPath(dir) {
		t.Errorf("the role's diagnostic = %v, want models.scoring in the config file", last)
	}
	if strings.Contains(stdout, fileKey) {
		t.Errorf("the document carries the key %q:\n%s", fileKey, stdout)
	}
	// config.toml still states the superseded key, so the invocation says so —
	// once.
	if n := strings.Count(stderr, "superseded provider key"); n != 1 {
		t.Errorf("stderr = %q, want one superseded-key line", stderr)
	}
}

// TestShowConfigSaysNothingOfASupersededKeyItDidNotRead: an environment-mode
// invocation reads no file value, so it reports no superseded key.
func TestShowConfigSaysNothingOfASupersededKeyItDidNotRead(t *testing.T) {
	dir := configFixture(t)
	t.Setenv(config.EnvAPIBaseURL, "http://env.example/v1")
	t.Setenv(config.EnvDecision, "env-decision")
	t.Setenv(config.EnvText, "env-text")
	t.Setenv(config.EnvScoring, "env-scoring")
	t.Setenv(config.EnvAPIKeyFile, "")
	doc, stdout, stderr, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if strings.Contains(stderr, "superseded provider key") {
		t.Errorf("an environment-mode invocation said the file's key is superseded: %q", stderr)
	}
}

// TestShowConfigReportsAnAbsentConfiguration: with no configuration directory
// at all the verb still reports — done, the one layer it read present and
// empty, nothing selected, no role resolved and one diagnostic per role — and
// it creates no directory.
func TestShowConfigReportsAnAbsentConfiguration(t *testing.T) {
	unsetInferenceEnv(t)
	dir := filepath.Join(t.TempDir(), "absent")
	t.Setenv(config.EnvConfigDir, dir)
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir())
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	layers := reportedLayers(t, doc)
	if want := (map[string]any{"path": dir, "source": config.LayerSourceEnvironment,
		"exists": false, "providersExists": false, "configExists": false, "harnessesExists": false}); len(layers) != 1 || !reflect.DeepEqual(layers[0], want) {
		t.Errorf("layers = %v, want %v", layers, want)
	}
	if providers, _ := doc.Values["providers"].([]any); len(providers) != 0 {
		t.Errorf("providers = %v, want none", doc.Values["providers"])
	}
	if doc.Values["mode"] != string(config.ModeFile) {
		t.Errorf("mode = %v, want file: no inference variable is set", doc.Values["mode"])
	}
	if selection, _ := doc.Values["selection"].(map[string]any); selection["provider"] != nil || selection["detail"] == nil {
		t.Errorf("selection = %v, want the selection's own reason and no provider", selection)
	}
	for _, role := range config.Roles {
		if got := roleOf(doc, string(role)); got == nil || got["detail"] == nil {
			t.Errorf("role %s = %v, want it listed with what it lacks", role, got)
		}
	}
	if diagnostics := doc.items("diagnostics"); len(diagnostics) != len(config.Roles) {
		t.Fatalf("diagnostics = %v, want one per role", diagnostics)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("the verb created the configuration directory")
	}
}

// TestShowConfigNamesTheConfigFileALackBelongsTo: a role no layer sets is
// diagnosed against the config.toml of the layer this invocation writes into,
// by its path: a repository layer and a global one can each hold a config.toml,
// so the bare name would not say which.
func TestShowConfigNamesTheConfigFileALackBelongsTo(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, "[solo]\nbaseUrl = \"http://solo.example/v1\"\n")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if doc.Values["mode"] != string(config.ModeFile) {
		t.Errorf("mode = %v, want file", doc.Values["mode"])
	}
	diagnostics := doc.items("diagnostics")
	want := config.ConfigPath(dir)
	if len(diagnostics) != len(config.Roles) {
		t.Fatalf("diagnostics = %v, want one per role", diagnostics)
	}
	for i, role := range config.Roles {
		if diagnostics[i]["key"] != string(role) && diagnostics[i]["key"] != "models."+string(role) {
			t.Errorf("diagnostic %d = %v, want models.%s", i, diagnostics[i], role)
		}
		if diagnostics[i]["path"] != want {
			t.Errorf("diagnostic %d names %v, want %s", i, diagnostics[i]["path"], want)
		}
	}
}

// TestShowConfigReportsARolesProviderThePoolLacks: a role whose reference names
// an entry the pool does not hold is a provider item by that role's key, with
// the pool's entry names as the values that would work — the shape a run
// refuses with, reported here because this verb never refuses.
func TestShowConfigReportsARolesProviderThePoolLacks(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab.example/v1\"\n")
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"lab/d\"\ntext = \"lab/t\"\nscoring = \"elsewhere/clef\"\n")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	var scoring map[string]any
	for _, item := range doc.items("diagnostics") {
		if item["key"] == "models.scoring" {
			scoring = item
		}
	}
	if scoring == nil {
		t.Fatalf("no item names models.scoring: %v", doc.items("diagnostics"))
	}
	if scoring["check"] != checkProvider || !slices.Equal(anyStrings(scoring["allowed"]), []string{"lab"}) ||
		!strings.Contains(scoring["detail"].(string), `provider "elsewhere"`) {
		t.Errorf("the item = %v, want check %s, the pool's entries allowed and the provider named", scoring, checkProvider)
	}
	if role := roleOf(doc, "scoring"); role["detail"] == nil || role["provider"] != nil {
		t.Errorf("the scoring role = %v, want a detail and no provider", role)
	}
}

// TestShowConfigReportsAnEndpointTheEnvironmentSupplies: the environment
// supplies the endpoint and every role's model, so each role resolves with no
// pool entry behind it and nothing is diagnosed.
func TestShowConfigReportsAnEndpointTheEnvironmentSupplies(t *testing.T) {
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, "[a]\nbaseUrl = \"http://a.example/v1\"\n\n[b]\nbaseUrl = \"http://b.example/v1\"\n")
	unsetInferenceEnv(t)
	t.Setenv(config.EnvAPIBaseURL, "http://env.example/v1")
	t.Setenv(config.EnvDecision, "env-decision")
	t.Setenv(config.EnvText, "env-text")
	t.Setenv(config.EnvScoring, "env-scoring")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if doc.Values["mode"] != string(config.ModeEnvironment) {
		t.Errorf("mode = %v, want environment", doc.Values["mode"])
	}
	if selection, _ := doc.Values["selection"].(map[string]any); selection["provider"] != nil || selection["detail"] == nil {
		t.Errorf("selection = %v, want the reason no entry was selected", selection)
	}
	for _, want := range []struct{ role, model string }{
		{"decision", "env-decision"}, {"text", "env-text"}, {"scoring", "env-scoring"},
	} {
		got := roleOf(doc, want.role)
		if got == nil {
			t.Fatalf("no %s record", want.role)
		}
		if got["back-end"] != "environment" || got["model"] != want.model || got["base-url"] != "http://env.example/v1" {
			t.Errorf("%s = %v, want the environment's endpoint and the role's bare id", want.role, got)
		}
		if sources, _ := got["sources"].(map[string]any); sources["base-url"] != config.LayerSourceEnvironment ||
			sources["model"] != config.LayerSourceEnvironment || sources["key"] != nil {
			t.Errorf("%s sources = %v, want the environment for the endpoint and the model, null for the key", want.role, sources)
		}
		if _, ok := got["provider"]; ok {
			t.Errorf("%s names a pool entry: %v", want.role, got)
		}
	}
	env := anyStrings(doc.Values["environment"])
	if !slices.Equal(env, []string{config.EnvAPIBaseURL, config.EnvDecision, config.EnvScoring, config.EnvText}) {
		t.Errorf("environment = %v, want the four names set, sorted, and no value", env)
	}
	if diagnostics := doc.items("diagnostics"); len(diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none: nothing is missing", diagnostics)
	}
}

// TestShowConfigReportsAMixedModeAsADiagnostic: an environment variable beside
// a file's model is refused by the mode, and the report says so rather than
// stopping — mode null, every role undecided, and one item per side.
func TestShowConfigReportsAMixedModeAsADiagnostic(t *testing.T) {
	unsetInferenceEnv(t)
	dir := configFixture(t)
	t.Setenv(config.EnvText, "env-text")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if doc.Values["mode"] != nil {
		t.Errorf("mode = %v, want null where the sources are mixed", doc.Values["mode"])
	}
	for _, role := range config.Roles {
		if got := roleOf(doc, string(role)); got == nil || got["detail"] == nil {
			t.Errorf("role %s = %v, want it undecided with a detail", role, got)
		}
	}
	// The faults, plus one item per side of the mix: the variable, and each
	// [models] key that stands.
	items := doc.items("diagnostics")
	var keys []string
	for _, item := range items[2:] {
		if key, _ := item["key"].(string); key != "" {
			keys = append(keys, key)
		}
	}
	if want := []string{config.EnvText, "models.decision", "models.text"}; !slices.Equal(keys, want) {
		t.Errorf("the mix's items name %q, want %q", keys, want)
	}
}

// TestShowConfigDiagnosesAnUnreadableKeyFile: the report makes no request, so
// it learns an unreadable key file from the registry's preflight — a
// diagnostic naming the variable and the path, not a refusal.
func TestShowConfigDiagnosesAnUnreadableKeyFile(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "absent.txt")
	writeValues(t, dir, config.ProvidersFileName, "[solo]\nbaseUrl = \"http://solo.example/v1\"\napiKeyFile = \"key.txt\"\n")
	writeValues(t, dir, "key.txt", fileKey)
	if err := os.Chmod(filepath.Join(dir, "key.txt"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"solo/d\"\ntext = \"solo/t\"\nscoring = \"solo/s\"\n")
	t.Setenv(config.EnvAPIKeyFile, missing)
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	// One item: three roles dial with the one file, and the file is the fact.
	items := doc.items("diagnostics")
	if len(items) != 1 {
		t.Fatalf("diagnostics = %v, want the one key file named once", items)
	}
	if item := items[0]; item["check"] != checkProvider || item["key"] != config.EnvAPIKeyFile || item["path"] != missing {
		t.Errorf("diagnostic %v, want %s naming %s", item, config.EnvAPIKeyFile, missing)
	}
	for _, role := range config.Roles {
		if got := roleOf(doc, string(role)); got == nil || got["model"] == nil {
			t.Errorf("role %s = %v, want it resolved: the key file is the dial's business", role, got)
		}
	}
}

// TestShowConfigRefusesAConfigurationThatDoesNotLoad: a file failing the strict
// load refuses exactly as it does for every other subcommand, and a refused
// document carries no diagnostics.
func TestShowConfigRefusesAConfigurationThatDoesNotLoad(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	writeValues(t, dir, config.ConfigFileName, "provider = \"a\"\n\n[models]\nheavy = \"some-model\"\n")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
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
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"deciding/dec-1\"\ntext = \"deciding/text-1\"\nscoring = \"silent/score-1\"\n")

	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
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

// TestShowConfigListsHarnesses: the loaded profiles, in name order, each with
// the layer whose file supplied it and the command as written — a reader's only
// way to see what --harness could choose — and `[]` where no layer supplies one.
func TestShowConfigListsHarnesses(t *testing.T) {
	unsetInferenceEnv(t)
	dir := t.TempDir()
	writeValues(t, dir, config.HarnessesFileName,
		"[zeta]\ncommand = [\"z\", \"{prompt}\"]\n\n[alpha]\ncommand = [\"a\", \"--model\", \"{model}\"]\n")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d\n%s", doc.Outcome, code, stdout)
	}
	got, ok := doc.Values["harnesses"].([]any)
	if !ok || len(got) != 2 {
		t.Fatalf("harnesses = %v, want the two profiles", doc.Values["harnesses"])
	}
	want := []any{
		map[string]any{"name": "alpha", "layer": "explicit", "command": []any{"a", "--model", "{model}"}},
		map[string]any{"name": "zeta", "layer": "explicit", "command": []any{"z", "{prompt}"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("harnesses = %v, want %v in name order with each command as written", got, want)
	}
	if got := reportedLayers(t, doc)[0]["harnessesExists"]; got != true {
		t.Errorf("harnessesExists = %v, want the file it probes reported present", got)
	}

	empty, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", t.TempDir())
	if code != 0 || empty.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d\n%s", empty.Outcome, code, stdout)
	}
	if got, ok := empty.Values["harnesses"].([]any); !ok || len(got) != 0 {
		t.Errorf("harnesses with no file = %v, want []", empty.Values["harnesses"])
	}
}
