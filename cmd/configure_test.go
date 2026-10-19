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
)

// Model ids the fixtures configure with — one per category, in the provider
// spellings the matcher must tolerate.
const (
	decisionID = "clef"
	textID     = "Qwen3.8-27B"
	otherText  = "deepseek-v4.1"
)

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
func runConfig(t *testing.T, opts configureOptions, client *model.MockClient) configureResult {
	t.Helper()
	if opts.WriteDir == "" {
		opts.WriteDir = t.TempDir()
	}
	var stdout, stderr bytes.Buffer
	opts.Stdout, opts.Stderr = &stdout, &stderr
	if opts.NewClient == nil {
		opts.NewClient = func(model.Endpoint) model.Client { return client }
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

// TestRunConfigureRefusesUnassignedCategories: no category is guessed — a
// category --model-map does not name is refused, one item per missing category
// naming its models.<category> key, listing the provider's catalogue as the
// choices, and nothing is written.
func TestRunConfigureRefusesUnassignedCategories(t *testing.T) {
	ids := []string{"text-embedding-3-large", decisionID, textID, "mistral-small-3"}

	got := runConfig(t, soloOpts(), model.NewScriptedMock(nil, catalogue(ids...)))
	items := got.doc.items("refusals")
	if got.code != 1 || len(items) != 2 {
		t.Fatalf("no --model-map: exit %d, refusals %v: want one item per category", got.code, items)
	}
	for i, category := range modelCategories {
		if items[i]["key"] != "models."+category || !slices.Equal(anyStrings(items[i]["allowed"]), sortedStrings(ids)) ||
			items[i]["remedy"] != "kbase configure --model-map "+category+"=<id>" ||
			!strings.Contains(items[i]["detail"].(string), "no model assigned to the "+category+" category") {
			t.Errorf("refusal %d = %v, want models.%s offering the catalogue", i, items[i], category)
		}
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite unassigned categories")
	}

	opts := soloOpts()
	opts.ModelMap = []string{"decision=" + decisionID}
	partial := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(ids...)))
	item := partial.refusal(t)
	if item["key"] != "models.text" || !slices.Equal(anyStrings(item["allowed"]), sortedStrings(ids)) ||
		item["remedy"] != "kbase configure --model-map text=<id>" {
		t.Errorf("partial-map refusal = %v, want the missing text category with the catalogue allowed", item)
	}
	if _, written := partial.config(t); written {
		t.Error("config.toml written despite a partially assigned map")
	}
}

// sortedStrings is a decoded refusal's `allowed` list in the order it is
// written: sorted ascending.
func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	slices.Sort(out)
	return out
}

// TestRunConfigureModelMapAssignsBoth: a full map is the only accepted form.
// Both flag spellings — repeated and comma-separated — mean the same thing,
// and the resolved ids land in config.toml with the result naming each
// category.
func TestRunConfigureModelMapAssignsBoth(t *testing.T) {
	const pinned = "some-org/clef-flash-q4"

	tests := []struct {
		name     string
		modelMap []string
		ids      []string
	}{{
		name:     "one comma-separated value",
		modelMap: []string{"decision=" + pinned + ",text=" + textID},
		ids:      []string{decisionID, textID, otherText},
	}, {
		name:     "repeated flag",
		modelMap: []string{"decision=" + pinned, "text=" + textID},
		ids:      []string{decisionID, textID, otherText},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := soloOpts()
			opts.ModelMap = tt.modelMap
			got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(tt.ids...)))

			if got.code != 0 || got.doc.Outcome != "done" {
				t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
			}
			cfg, written := got.config(t)
			if !written {
				t.Fatal("no config.toml written on success")
			}
			want := config.Config{Provider: "solo", Models: config.ModelMap{Decision: pinned, Text: textID}}
			if cfg != want {
				t.Errorf("config.toml = %+v, want %+v", cfg, want)
			}
			models, _ := got.doc.Values["models"].(map[string]any)
			if got.doc.Values["provider"] != "solo" || models["decision"] != pinned || models["text"] != textID ||
				!slices.Equal(anyStrings(got.doc.Values["written"]), []string{config.ConfigPath(got.dir)}) {
				t.Errorf("result = %v, want the provider, both categories and config.toml written", got.doc.Values)
			}
			if got.doc.Values["provider-rule"] != "sole-entry" || got.doc.Values["provider-layer"] != config.LayerSourceRepository {
				t.Errorf("result provenance = %v/%v, want the sole-entry rule and the layer that supplies the entry",
					got.doc.Values["provider-rule"], got.doc.Values["provider-layer"])
			}
			for _, want := range []string{"configured:", "provider=solo",
				config.CategoryDecision + "=" + pinned, config.CategoryText + "=" + textID, config.ConfigPath(got.dir)} {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("stderr: got %q, want it to report %q", got.stderr, want)
				}
			}
			if strings.Contains(got.stderr, testAPIKey) {
				t.Error("API key leaked into stderr")
			}
			again := runConfig(t, configureOptions{providerOptions: providerOptions{Providers: soloPool(), WriteDir: got.dir}, ModelMap: tt.modelMap},
				model.NewScriptedMock(nil, catalogue(tt.ids...)))
			if again.code != 0 || again.doc.Outcome != "unchanged" || len(anyStrings(again.doc.Values["written"])) != 0 {
				t.Errorf("a second configure to the same values = %s, written %v; want unchanged with nothing written", again.doc.Outcome, again.doc.Values["written"])
			}
		})
	}
}

// TestRunConfigureUnlistedMappedIDWarns: a mapped id the catalogue omits is
// a warning on stderr, not a failure — provider catalogues rotate, and a
// user pinning one the listing omits may know something the listing does
// not.
func TestRunConfigureUnlistedMappedIDWarns(t *testing.T) {
	opts := soloOpts()
	opts.ModelMap = []string{"decision=clef-preview", "text=" + textID}
	got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(textID)))

	if got.code != 0 {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}
	if want := `decision category model "clef-preview" is not in provider "solo"'s catalogue — configuring it anyway`; !strings.Contains(got.stderr, want) {
		t.Errorf("stderr: got %q, want substring %q", got.stderr, want)
	}
}

// TestRunConfigureCreatesConfigDir: the first configure of a fresh install
// has no config directory to write into and must make one.
func TestRunConfigureCreatesConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "kbase")
	opts := soloOpts()
	opts.WriteDir = dir
	opts.ModelMap = []string{"decision=" + decisionID, "text=" + textID}
	if got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(decisionID, textID))); got.code != 0 {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}
	if _, err := os.Stat(config.ConfigPath(dir)); err != nil {
		t.Fatalf("config.toml in a created directory: %v", err)
	}
}

// TestRunConfigureKeepsHandEdits: config.toml is a file users annotate, and
// `kbase configure` is the command that rewrites it. It must land the
// assigned ids without disturbing anything around them.
func TestRunConfigureKeepsHandEdits(t *testing.T) {
	dir := t.TempDir()
	path := config.ConfigPath(dir)
	const note = "# the endpoint this box is allowed to reach"
	if err := os.WriteFile(path, []byte(note+"\nprovider = \"solo\"\n"), 0o600); err != nil {
		t.Fatalf("seed config.toml: %v", err)
	}

	opts := soloOpts()
	opts.WriteDir = dir
	opts.ModelMap = []string{"decision=" + decisionID, "text=" + textID}
	if got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(decisionID, textID))); got.code != 0 {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(body), note) {
		t.Errorf("configure ate a hand-written comment:\n%s", body)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Models.Decision != decisionID || cfg.Models.Text != textID {
		t.Errorf("models = %+v, want decision=%q text=%q", cfg.Models, decisionID, textID)
	}
}

// TestRunConfigureListFailure: a provider-side failure writes nothing. The
// previous configuration outliving a failed run is the whole reason the
// write is the last step.
func TestRunConfigureListFailure(t *testing.T) {
	client := model.NewScriptedMock(nil, catalogue(decisionID, textID))
	client.SetError(errors.New("http 503: service unavailable"))

	opts := soloOpts()
	opts.ModelMap = []string{"decision=" + decisionID, "text=" + textID}
	got := runConfig(t, opts, client)

	if failures := got.doc.items("failures"); got.code != 3 || len(failures) != 1 || !strings.Contains(failures[0]["detail"].(string), "503") {
		t.Fatalf("list failure: exit %d, failures %v; want it reported failed", got.code, failures)
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite a list failure")
	}
}

// TestRunConfigureBadModelMapSkipsNetwork: flag syntax is checked before
// anything is dialed, so a typo costs no round trip.
func TestRunConfigureBadModelMapSkipsNetwork(t *testing.T) {
	// The client seam is the network: a factory that fails the test is how
	// "no round trip" becomes an assertion rather than a claim.
	opts := soloOpts()
	opts.ModelMap = []string{"decision"}
	opts.NewClient = func(model.Endpoint) model.Client {
		t.Error("a malformed --model-map dialed the provider; flag syntax is checked first")
		return model.NewScriptedMock(nil, catalogue(decisionID, textID))
	}

	got := runConfig(t, opts, nil)
	if item := got.refusal(t); item["key"] != "--model-map" || item["check"] != "usage" {
		t.Errorf("refusal = %v, want a usage refusal of --model-map", item)
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite a malformed --model-map")
	}
}

// TestParseModelMap covers the flag grammar, including the forms that must
// be rejected rather than half-understood.
func TestParseModelMap(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    map[string]string
		wantErr string
	}{{
		name:   "empty",
		values: nil,
		want:   map[string]string{},
	}, {
		name:   "comma-separated pair",
		values: []string{"decision=d,text=t"},
		want:   map[string]string{config.CategoryDecision: "d", config.CategoryText: "t"},
	}, {
		name:   "repeated flag accumulates",
		values: []string{"decision=d", "text=t"},
		want:   map[string]string{config.CategoryDecision: "d", config.CategoryText: "t"},
	}, {
		name:   "surrounding space and category case are tolerated",
		values: []string{" DECISION = d , text=t "},
		want:   map[string]string{config.CategoryDecision: "d", config.CategoryText: "t"},
	}, {
		name:   "repeating one category with the same id is not a conflict",
		values: []string{"decision=d", "decision=d"},
		want:   map[string]string{config.CategoryDecision: "d"},
	}, {
		name:    "no assignment",
		values:  []string{"decision"},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "missing id",
		values:  []string{"decision="},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "unknown category",
		values:  []string{"medium=m"},
		wantErr: `unknown category "medium"`,
	}, {
		name:    "conflicting assignments to one category",
		values:  []string{"decision=a,decision=b"},
		wantErr: "assigns the decision category twice",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseModelMap(tt.values)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got %v", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error: got %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseModelMap: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("map = %v, want %v", got, tt.want)
			}
			for category, id := range tt.want {
				if got[category] != id {
					t.Errorf("map[%s] = %q, want %q", category, got[category], id)
				}
			}
		})
	}
}

// TestRunConfigureCreateWritesBothFiles: --create with no --provider makes no
// request at all — the creating call dials nothing — and lands both files in a
// directory that did not exist, reporting what it wrote and saying on stderr
// that the ids went unchecked.
func TestRunConfigureCreateWritesBothFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh")
	dialed := false
	opts := soloOpts()
	opts.WriteDir = dir
	opts.Create = true
	opts.ModelMap = []string{"decision=" + decisionID, "text=" + textID}
	opts.NewClient = func(model.Endpoint) model.Client {
		dialed = true
		return model.NewScriptedMock(nil, catalogue(decisionID, textID))
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
	if models[config.CategoryDecision] != decisionID || models[config.CategoryText] != textID {
		t.Errorf("models = %v, want the ids the flag named", models)
	}
	if want := []string{config.ProvidersPath(dir), config.ConfigPath(dir)}; !slices.Equal(anyStrings(got.doc.Values["written"]), want) {
		t.Errorf("written = %v, want %v", got.doc.Values["written"], want)
	}
	if !strings.Contains(got.stderr, "were not checked") {
		t.Errorf("stderr = %q, want the line saying the ids were not checked", got.stderr)
	}
	pool, faults, err := config.LoadProviders(config.ProvidersPath(dir))
	if err != nil || len(faults) != 0 || len(pool) != 1 {
		t.Fatalf("the created pool = %v, faults %v, err %v; want one usable entry", pool, faults, err)
	}
	cfg, written := got.config(t)
	if !written || cfg.Provider != "" || cfg.Models != (config.ModelMap{Decision: decisionID, Text: textID}) {
		t.Errorf("config.toml = %+v (%t), want the ids and no provider", cfg, written)
	}
}

// TestRunConfigureCreateIsIdempotent: a second identical call finds every file
// holding what it would write, and says so rather than rewriting them.
func TestRunConfigureCreateIsIdempotent(t *testing.T) {
	opts := soloOpts()
	opts.WriteDir = t.TempDir()
	opts.Create = true
	opts.ModelMap = []string{"decision=" + decisionID, "text=" + textID}

	if first := runConfig(t, opts, nil); first.code != 0 || first.doc.Outcome != "done" {
		t.Fatalf("first configure --create: exit %d, %v", first.code, first.doc.Values)
	}
	again := runConfig(t, opts, nil)
	if again.code != 0 || again.doc.Outcome != "unchanged" || len(anyStrings(again.doc.Values["written"])) != 0 {
		t.Errorf("a second configure --create = %s, written %v; want unchanged with nothing written",
			again.doc.Outcome, again.doc.Values["written"])
	}
}

// TestRunConfigureCreateDialedRefusalWritesNothing: creation is the last step,
// so a --provider the pool does not hold refuses against the target layer's own
// providers.toml — the file the entry belongs in — and creates neither file.
func TestRunConfigureCreateDialedRefusalWritesNothing(t *testing.T) {
	dir := t.TempDir()
	opts := soloOpts()
	opts.WriteDir = dir
	opts.Create = true
	opts.Provider = "typo"
	opts.ModelMap = []string{"decision=" + decisionID, "text=" + textID}

	got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(decisionID, textID)))
	item := got.refusal(t)
	if item["check"] != checkProvider || item["key"] != "--provider" || item["path"] != config.ProvidersPath(dir) ||
		item["remedy"] != "kbase configure --create" {
		t.Errorf("refusal = %v, want --provider refused against %s with the creation remedy", item, config.ProvidersPath(dir))
	}
	for _, path := range []string{config.ProvidersPath(dir), config.ConfigPath(dir)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s written despite the refusal", path)
		}
	}
}

// TestRunConfigureCreateKeepsWhatStandsThere: --create is permission to create,
// not to replace. A config.toml holding a provider, an unmentioned category and
// a hand-written comment keeps all three, and a standing providers.toml is left
// exactly as it was.
func TestRunConfigureCreateKeepsWhatStandsThere(t *testing.T) {
	dir := t.TempDir()
	const note = "# this box reaches the lab"
	pool := "[solo]\nbaseUrl = \"http://kept.example/v1\"\n"
	if err := os.WriteFile(config.ConfigPath(dir), []byte(note+"\nprovider = \"solo\"\n\n[models]\ndecision = \"kept\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ProvidersPath(dir), []byte(pool), 0o600); err != nil {
		t.Fatal(err)
	}

	opts := soloOpts()
	opts.WriteDir = dir
	opts.Create = true
	opts.ModelMap = []string{"text=" + textID}
	got := runConfig(t, opts, nil)
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("configure --create: exit %d, %v", got.code, got.doc.Values)
	}
	if got.doc.Values["provider"] != "solo" {
		t.Errorf("provider = %v, want the provider config.toml already names", got.doc.Values["provider"])
	}
	cfg, _ := got.config(t)
	if cfg.Provider != "solo" || cfg.Models.Decision != "kept" || cfg.Models.Text != textID {
		t.Errorf("config.toml = %+v, want the standing provider and decision kept and text set", cfg)
	}
	body, err := os.ReadFile(config.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), note) {
		t.Errorf("configure ate a hand-written comment:\n%s", body)
	}
	if kept, err := os.ReadFile(config.ProvidersPath(dir)); err != nil || string(kept) != pool {
		t.Errorf("providers.toml = %q (%v), want it untouched", kept, err)
	}
	if written := anyStrings(got.doc.Values["written"]); !slices.Equal(written, []string{config.ConfigPath(dir)}) {
		t.Errorf("written = %v, want only config.toml: providers.toml already stood", written)
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
