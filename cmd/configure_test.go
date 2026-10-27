package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// Model ids the fixtures configure with — one per category, in the provider
// spellings the matcher must tolerate.
const (
	decisionID = "clef"
	textID     = "Qwen3.8-27B"
	scoringID  = "clef-scoring"
)

// ref is a category's model as a --model-map value: a reference to the solo
// pool entry the fixtures configure against.
func ref(id string) string { return "solo/" + id }

// refs is the three-category --model-map one value, at the solo entry.
func refs(decision, text, scoring string) []string {
	return []string{"decision=" + ref(decision) + ",text=" + ref(text) + ",scoring=" + ref(scoring)}
}

// configureResult is what one runConfigure invocation produced: its result
// document, stderr, the exit code, and the config directory it was pointed
// at.
type configureResult struct {
	doc    resultDoc
	stderr string
	dir    string
	code   int
}

// refusal is the detail of the result's one refusal, "" where it has none.
func (r configureResult) refusal(t *testing.T) map[string]any {
	t.Helper()
	items := r.doc.items("refusals")
	if r.doc.Outcome != "refused" || r.code != 1 || len(items) != 1 {
		t.Fatalf("outcome %s, exit %d, refusals %v: want one refusal", r.doc.Outcome, r.code, items)
	}
	return items[0]
}

// config reads back the config.toml the run was supposed to write. The
// second return is false when no file was written.
func (r configureResult) config(t *testing.T) (config.Config, bool) {
	t.Helper()
	path := config.ConfigPath(r.dir)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return config.Config{}, false
	} else if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg, true
}

// runConfig invokes runConfigure against a fresh temp configuration directory,
// substituting a client factory that serves ids from a MockClient — no
// test reaches the network, a real home, or the process streams.
func runConfig(t *testing.T, opts configureOptions, client *modeltest.MockClient) configureResult {
	t.Helper()
	if opts.WriteDir == "" {
		opts.WriteDir = t.TempDir()
	}
	var stdout, stderr bytes.Buffer
	opts.Stdout, opts.Stderr = &stdout, &stderr
	if opts.NewClient == nil {
		opts.NewClient = func(model.Endpoint) model.Client { return client.Client() }
	}
	code, err := runConfigure(context.Background(), opts)
	if err != nil {
		t.Fatalf("runConfigure: %v", err)
	}
	return configureResult{doc: checkDocument(t, "configure", stdout.String()), stderr: stderr.String(), dir: opts.WriteDir, code: code}
}

// soloPool is the single-entry pool every fixture configures against, so
// provider selection never needs a flag it is not the subject of.
func soloPool() config.Providers {
	return config.Providers{"solo": provider("solo", "http://provider.example/v1")}
}

// soloOpts is the baseline configure input: the single-entry pool and
// nothing else. Tests set the one field they are about on the result.
func soloOpts() configureOptions {
	return configureOptions{providerOptions: providerOptions{Providers: soloPool()}}
}

// anyStrings is a decoded YAML list of strings.
func anyStrings(v any) []string {
	list, _ := v.([]any)
	out := []string{}
	for _, e := range list {
		s, _ := e.(string)
		out = append(out, s)
	}
	return out
}

// TestRunConfigureReportsTheWrite: a dialed run that writes states the entry it
// configured, the rule and layer that named it, the three references and the
// file it wrote, and says the same in one line on stderr that never carries the
// key; the same run again, writing nothing, is unchanged.
func TestRunConfigureReportsTheWrite(t *testing.T) {
	opts := soloOpts()
	opts.ModelMap = refs(decisionID, textID, scoringID)
	got := runConfig(t, opts, modeltest.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)))

	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}
	models, _ := got.doc.Values["models"].(map[string]any)
	if got.doc.Values["provider"] != "solo" || models["decision"] != ref(decisionID) ||
		models["text"] != ref(textID) || models["scoring"] != ref(scoringID) ||
		!slices.Equal(anyStrings(got.doc.Values["written"]), []string{config.ConfigPath(got.dir)}) {
		t.Errorf("result = %v, want the provider, all three references and config.toml written", got.doc.Values)
	}
	if got.doc.Values["provider-rule"] != "sole-entry" || got.doc.Values["provider-layer"] != config.LayerSourceRepository {
		t.Errorf("result provenance = %v/%v, want the sole-entry rule and the layer that supplies the entry",
			got.doc.Values["provider-rule"], got.doc.Values["provider-layer"])
	}
	for _, want := range []string{"configured:",
		config.CategoryDecision + "=" + ref(decisionID), config.CategoryText + "=" + ref(textID),
		config.CategoryScoring + "=" + ref(scoringID), config.ConfigPath(got.dir)} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr: got %q, want it to report %q", got.stderr, want)
		}
	}
	if strings.Contains(got.stderr, testAPIKey) {
		t.Error("API key leaked into stderr")
	}
	again := runConfig(t, configureOptions{providerOptions: providerOptions{Providers: soloPool(), WriteDir: got.dir}, ModelMap: opts.ModelMap},
		modeltest.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)))
	if again.code != 0 || again.doc.Outcome != "unchanged" {
		t.Errorf("a second configure to the same values = %s, exit %d; want unchanged", again.doc.Outcome, again.code)
	}
}

// TestRunConfigureUnlistedMappedIDWarns: a mapped id the catalogue omits is
// a warning on stderr, not a failure — provider catalogues rotate, and a
// user pinning one the listing omits may know something the listing does
// not.
func TestRunConfigureUnlistedMappedIDWarns(t *testing.T) {
	opts := soloOpts()
	opts.ModelMap = []string{"decision=" + ref("clef-preview"), "text=" + ref(textID), "scoring=" + ref(scoringID)}
	got := runConfig(t, opts, modeltest.NewScriptedMock(nil, catalogue(textID, scoringID)))

	if got.code != 0 {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}
	if want := `decision category model "clef-preview" is not in provider "solo"'s catalogue — configuring it anyway`; !strings.Contains(got.stderr, want) {
		t.Errorf("stderr: got %q, want substring %q", got.stderr, want)
	}
}

// TestRunConfigureListFailure: a provider-side failure to list the catalogue is
// the run's failure, reported with the provider's own reason.
func TestRunConfigureListFailure(t *testing.T) {
	client := modeltest.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID))
	client.SetError(errors.New("http 503: service unavailable"))

	opts := soloOpts()
	opts.ModelMap = refs(decisionID, textID, scoringID)
	got := runConfig(t, opts, client)

	if failures := got.doc.items("failures"); got.code != 3 || len(failures) != 1 || !strings.Contains(failures[0]["detail"].(string), "503") {
		t.Fatalf("list failure: exit %d, failures %v; want it reported failed", got.code, failures)
	}
}

// TestRunConfigureWarnsBeforeTheDial: the pool's faults and the superseded
// provider key are on stderr before the first catalogue is requested, so a
// dial that hangs or fails leaves them said.
func TestRunConfigureWarnsBeforeTheDial(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts := soloOpts()
	opts.WriteDir = t.TempDir()
	opts.ModelMap = refs(decisionID, textID, scoringID)
	opts.Faults = []config.ProviderFault{{Name: "broken", Reason: "no base_url"}}
	opts.Config = config.Config{Provider: "solo"}
	opts.Stdout, opts.Stderr = &stdout, &stderr
	var atDial string
	opts.NewClient = func(model.Endpoint) model.Client {
		atDial = stderr.String()
		return modeltest.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)).Client()
	}
	if _, err := runConfigure(context.Background(), opts); err != nil {
		t.Fatalf("runConfigure: %v", err)
	}
	for _, want := range []string{`provider "broken" unavailable: no base_url`, "superseded provider key"} {
		if !strings.Contains(atDial, want) {
			t.Errorf("stderr at the dial = %q, want it to hold %q", atDial, want)
		}
	}
}

// TestRunConfigureBadModelMapSkipsNetwork: flag syntax is checked before
// anything is dialed, so a typo costs no round trip.
func TestRunConfigureBadModelMapSkipsNetwork(t *testing.T) {
	// The client seam is the network: a factory that fails the test is how
	// "no round trip" becomes an assertion rather than a claim.
	for _, value := range []string{"decision", "decision=" + decisionID} {
		opts := soloOpts()
		opts.ModelMap = []string{value}
		opts.NewClient = func(model.Endpoint) model.Client {
			t.Error("a malformed --model-map dialed the provider; flag syntax is checked first")
			return modeltest.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)).Client()
		}

		got := runConfig(t, opts, nil)
		if item := got.refusal(t); item["key"] != "--model-map" || item["check"] != "usage" {
			t.Errorf("--model-map %q = %v, want a usage refusal of --model-map", value, item)
		}
		if _, written := got.config(t); written {
			t.Errorf("config.toml written despite --model-map %q", value)
		}
	}
}

// TestRunConfigureCreateWritesBothFiles: --create with no --provider makes no
// request at all — the creating call dials nothing — and reports what it wrote
// with no provider, saying on stderr that the ids went unchecked.
func TestRunConfigureCreateWritesBothFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh")
	dialed := false
	opts := soloOpts()
	opts.WriteDir = dir
	opts.Create = true
	opts.ModelMap = refs(decisionID, textID, scoringID)
	opts.NewClient = func(model.Endpoint) model.Client {
		dialed = true
		return modeltest.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)).Client()
	}
	got := runConfig(t, opts, nil)

	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("configure --create: exit %d, %v", got.code, got.doc.Values)
	}
	if dialed {
		t.Error("--create with no --provider dialed a provider")
	}
	if got.doc.Values["provider"] != nil {
		t.Errorf("provider = %v, want null: no provider was named", got.doc.Values["provider"])
	}
	for _, key := range []string{"provider-rule", "provider-layer"} {
		if _, ok := got.doc.Values[key]; ok {
			t.Errorf("result = %v, want no %s: the creating call dialed nothing, so no pool entry supplied an endpoint", got.doc.Values, key)
		}
	}
	models, _ := got.doc.Values["models"].(map[string]any)
	if models[config.CategoryDecision] != ref(decisionID) || models[config.CategoryText] != ref(textID) ||
		models[config.CategoryScoring] != ref(scoringID) {
		t.Errorf("models = %v, want the three references the flag named", models)
	}
	if want := []string{config.ProvidersPath(dir), config.ConfigPath(dir)}; !slices.Equal(anyStrings(got.doc.Values["written"]), want) {
		t.Errorf("written = %v, want %v: the pool, then the choices", got.doc.Values["written"], want)
	}
	if !strings.Contains(got.stderr, "were not checked") {
		t.Errorf("stderr = %q, want the line saying the references were not checked", got.stderr)
	}
}

// TestConfigureTarget: --project names the repository's configuration directory
// whether or not it stands, over the directory the environment named; beside
// --config-dir it is refused, since the two name different directories; and
// outside a worktree it is refused with nothing to create.
func TestConfigureTarget(t *testing.T) {
	repo := t.TempDir()
	explicit := configureOptions{providerOptions: providerOptions{
		Layers: []config.Layer{{Path: "/somewhere", Source: config.LayerSourceExplicit}}, WriteDir: "/somewhere"}}
	environment := configureOptions{providerOptions: providerOptions{
		Layers: []config.Layer{{Path: "/elsewhere", Source: config.LayerSourceEnvironment}}, WriteDir: "/elsewhere"}}
	project := configureOptions{providerOptions: providerOptions{
		Layers: []config.Layer{{Path: "/elsewhere", Source: config.LayerSourceEnvironment}}, WriteDir: "/elsewhere", RepoRoot: repo},
		Project: true, WorkDir: filepath.Join(repo, "nested")}

	if got, items, err := configureTarget(environment); err != nil || items != nil || got != "/elsewhere" {
		t.Errorf("configureTarget without --project = %q, %v, %v; want the layer the invocation resolved", got, items, err)
	}
	if got, items, err := configureTarget(project); err != nil || items != nil || got != filepath.Join(repo, config.RepositoryDirName) {
		t.Errorf("configureTarget --project = %q, %v, %v; want the repository's directory over the environment's", got, items, err)
	}

	beside := project
	beside.providerOptions = explicit.providerOptions
	if _, items, err := configureTarget(beside); err != nil || len(items) != 1 ||
		items[0].Key != "--project" || items[0].Check != checkUsage {
		t.Errorf("--project beside --config-dir = %v, %v; want one usage refusal naming --project", items, err)
	}

	outside := project
	outside.RepoRoot = ""
	if _, items, err := configureTarget(outside); err != nil || len(items) != 1 || items[0].Check != checkWorktree ||
		items[0].Path != outside.WorkDir || items[0].Remedy != "git init" {
		t.Errorf("--project outside a worktree = %v, %v; want the worktree refusal naming the start directory", items, err)
	}
}
