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

// Model ids the fixtures configure with — one per tier, in the provider
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

// runConfig invokes runConfigure against a fresh temp config directory,
// substituting a client factory that serves ids from a MockClient — no
// test reaches the network, a real home, or the process streams.
func runConfig(t *testing.T, opts configureOptions, client *model.MockClient) configureResult {
	t.Helper()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	opts.Stdout, opts.Stderr = &stdout, &stderr
	if opts.ConfigPath == "" {
		opts.ConfigPath = config.ConfigPath(dir)
	} else {
		dir = filepath.Dir(opts.ConfigPath)
	}
	if opts.NewClient == nil {
		opts.NewClient = func(model.Endpoint) model.Client { return client }
	}
	code, err := runConfigure(context.Background(), opts)
	if err != nil {
		t.Fatalf("runConfigure: %v", err)
	}
	return configureResult{doc: checkDocument(t, "configure", stdout.String()), stderr: stderr.String(), dir: dir, code: code}
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

// TestRunConfigureRefusesUnassignedTiers: no tier is guessed — a tier
// --model-map does not name is refused, one item per missing tier naming its
// models.<tier> key, listing the provider's catalogue as the choices, and
// nothing is written.
func TestRunConfigureRefusesUnassignedTiers(t *testing.T) {
	ids := []string{"text-embedding-3-large", decisionID, textID, "mistral-small-3"}

	got := runConfig(t, soloOpts(), model.NewScriptedMock(nil, catalogue(ids...)))
	items := got.doc.items("refusals")
	if got.code != 1 || len(items) != 2 {
		t.Fatalf("no --model-map: exit %d, refusals %v: want one item per tier", got.code, items)
	}
	for i, tier := range configureTiers {
		if items[i]["key"] != "models."+tier || !slices.Equal(anyStrings(items[i]["allowed"]), sortedStrings(ids)) ||
			items[i]["remedy"] != "kbase configure --model-map "+tier+"=<id>" ||
			!strings.Contains(items[i]["detail"].(string), "no model assigned to the "+tier+" tier") {
			t.Errorf("refusal %d = %v, want models.%s offering the catalogue", i, items[i], tier)
		}
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite unassigned tiers")
	}

	opts := soloOpts()
	opts.ModelMap = []string{"decision=" + decisionID}
	partial := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(ids...)))
	item := partial.refusal(t)
	if item["key"] != "models.text" || !slices.Equal(anyStrings(item["allowed"]), sortedStrings(ids)) ||
		item["remedy"] != "kbase configure --model-map text=<id>" {
		t.Errorf("partial-map refusal = %v, want the missing text tier with the catalogue allowed", item)
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
// and the resolved ids land in config.toml with the result naming each tier.
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
				t.Errorf("result = %v, want the provider, both tiers and config.toml written", got.doc.Values)
			}
			for _, want := range []string{"configured:", "provider=solo",
				config.TierDecision + "=" + pinned, config.TierText + "=" + textID, config.ConfigPath(got.dir)} {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("stderr: got %q, want it to report %q", got.stderr, want)
				}
			}
			if strings.Contains(got.stderr, testAPIKey) {
				t.Error("API key leaked into stderr")
			}
			again := runConfig(t, configureOptions{providerOptions: providerOptions{Providers: soloPool()}, ConfigPath: config.ConfigPath(got.dir), ModelMap: tt.modelMap},
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
	if want := `decision tier model "clef-preview" is not in provider "solo"'s catalogue — configuring it anyway`; !strings.Contains(got.stderr, want) {
		t.Errorf("stderr: got %q, want substring %q", got.stderr, want)
	}
}

// TestRunConfigureCreatesConfigDir: the first configure of a fresh install
// has no config directory to write into and must make one.
func TestRunConfigureCreatesConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "kbase")
	opts := soloOpts()
	opts.ConfigPath = config.ConfigPath(dir)
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
	opts.ConfigPath = path
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
		want:   map[string]string{config.TierDecision: "d", config.TierText: "t"},
	}, {
		name:   "repeated flag accumulates",
		values: []string{"decision=d", "text=t"},
		want:   map[string]string{config.TierDecision: "d", config.TierText: "t"},
	}, {
		name:   "surrounding space and tier case are tolerated",
		values: []string{" DECISION = d , text=t "},
		want:   map[string]string{config.TierDecision: "d", config.TierText: "t"},
	}, {
		name:   "repeating one tier with the same id is not a conflict",
		values: []string{"decision=d", "decision=d"},
		want:   map[string]string{config.TierDecision: "d"},
	}, {
		name:    "no assignment",
		values:  []string{"decision"},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "missing id",
		values:  []string{"decision="},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "unknown tier",
		values:  []string{"medium=m"},
		wantErr: `unknown tier "medium"`,
	}, {
		name:    "conflicting assignments to one tier",
		values:  []string{"decision=a,decision=b"},
		wantErr: "assigns the decision tier twice",
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
			for tier, id := range tt.want {
				if got[tier] != id {
					t.Errorf("map[%s] = %q, want %q", tier, got[tier], id)
				}
			}
		})
	}
}
