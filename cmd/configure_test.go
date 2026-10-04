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
	denseID = "google/gemma-4-31b-it"
	moeID   = "gemma4:31b-a4b"
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

// TestRunConfigureAutoDetect is the max-convenience default: one gemma-4
// dense and one MoE in the catalogue resolve both tiers with no flags, and
// the resolved ids land in config.toml.
func TestRunConfigureAutoDetect(t *testing.T) {
	got := runConfig(t, soloOpts(),
		model.NewScriptedMock(nil, catalogue("text-embedding-3-large", denseID, moeID, "gemma-3-27b-it")))

	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}
	cfg, written := got.config(t)
	if !written {
		t.Fatal("no config.toml written on success")
	}
	want := config.Config{Provider: "solo", Models: config.ModelMap{Heavy: denseID, Light: moeID}}
	if cfg != want {
		t.Errorf("config.toml = %+v, want %+v", cfg, want)
	}
	models, _ := got.doc.Values["models"].(map[string]any)
	if got.doc.Values["provider"] != "solo" || models["heavy"] != denseID || models["light"] != moeID ||
		!slices.Equal(anyStrings(got.doc.Values["written"]), []string{config.ConfigPath(got.dir)}) {
		t.Errorf("result = %v, want the provider, both tiers and config.toml written", got.doc.Values)
	}
	again := runConfig(t, configureOptions{providerOptions: providerOptions{Providers: soloPool()}, ConfigPath: config.ConfigPath(got.dir)},
		model.NewScriptedMock(nil, catalogue(denseID, moeID)))
	if again.code != 0 || again.doc.Outcome != "unchanged" || len(anyStrings(again.doc.Values["written"])) != 0 {
		t.Errorf("a second configure to the same values = %s, written %v; want unchanged with nothing written", again.doc.Outcome, again.doc.Values["written"])
	}
	for _, want := range []string{"configured:", "provider=solo",
		config.TierHeavy + "=" + denseID, config.TierLight + "=" + moeID, config.ConfigPath(got.dir)} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr: got %q, want it to report %q", got.stderr, want)
		}
	}
	if strings.Contains(got.stderr, testAPIKey) {
		t.Error("API key leaked into stderr")
	}
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

// TestRunConfigureAmbiguous: two dense candidates is the case the
// appliance must refuse. The refusal names the tier's key, lists both, and
// gives the flag that settles it — and nothing is written.
func TestRunConfigureAmbiguous(t *testing.T) {
	const otherDense = "gemma-4-31b-instruct"
	got := runConfig(t, soloOpts(),
		model.NewScriptedMock(nil, catalogue(denseID, otherDense, moeID)))

	item := got.refusal(t)
	if item["key"] != "models.heavy" || !slices.Equal(anyStrings(item["allowed"]), []string{otherDense, denseID}) ||
		item["remedy"] != "kbase configure --model-map heavy=<id>" || !strings.Contains(item["detail"].(string), "heavy tier is ambiguous") {
		t.Errorf("refusal = %v, want models.heavy with both candidates allowed", item)
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite an ambiguity failure")
	}
}

// TestRunConfigureNoMatch: a provider with no gemma-4 model of a tier is
// refused, offering the family members whose tier went unrecognized as the
// likeliest intended targets, or the whole catalogue where there are none.
func TestRunConfigureNoMatch(t *testing.T) {
	const familyNoTier = "gemma-4-9b"
	got := runConfig(t, soloOpts(),
		model.NewScriptedMock(nil, catalogue("text-embedding-3-large", familyNoTier, "gemma-3-27b-it")))

	items := got.doc.items("refusals")
	if got.code != 1 || len(items) != 2 {
		t.Fatalf("exit %d, refusals %v: want one per undetected tier", got.code, items)
	}
	for i, tier := range configureTiers {
		if items[i]["key"] != "models."+tier || !slices.Equal(anyStrings(items[i]["allowed"]), []string{familyNoTier}) ||
			!strings.Contains(items[i]["detail"].(string), "no gemma-4 "+tier+"-tier model detected") {
			t.Errorf("refusal %d = %v, want models.%s offering %s", i, items[i], tier, familyNoTier)
		}
	}
	none := runConfig(t, soloOpts(), model.NewScriptedMock(nil, catalogue("text-embedding-3-large", "gemma-3-27b-it")))
	if items := none.doc.items("refusals"); len(items) != 2 || !slices.Equal(anyStrings(items[0]["allowed"]), []string{"gemma-3-27b-it", "text-embedding-3-large"}) {
		t.Errorf("refusals = %v, want the whole catalogue offered where no gemma-4 model is", items)
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite a no-match failure")
	}
}

// TestRunConfigureModelMapOverrides: an explicit assignment wins over
// detection, and a partial map leaves the tiers it does not name to be
// detected. Both forms of the flag — repeated and comma-separated — mean
// the same thing.
func TestRunConfigureModelMapOverrides(t *testing.T) {
	const pinned = "some-org/gemma-4-31b-it-q4"

	tests := []struct {
		name       string
		modelMap   []string
		ids        []string
		wantHeavy  string
		wantLight  string
		wantStderr string // substring expected on stderr, "" for none
	}{{
		name:      "full map beats an ambiguous catalogue",
		modelMap:  []string{"heavy=" + pinned + ",light=" + moeID},
		ids:       []string{denseID, pinned, moeID},
		wantHeavy: pinned,
		wantLight: moeID,
	}, {
		name:      "repeated flag is the same as one comma-separated value",
		modelMap:  []string{"heavy=" + pinned, "light=" + moeID},
		ids:       []string{denseID, pinned, moeID},
		wantHeavy: pinned,
		wantLight: moeID,
	}, {
		name:      "partial map: light is still auto-detected",
		modelMap:  []string{"heavy=" + pinned},
		ids:       []string{pinned, moeID},
		wantHeavy: pinned,
		wantLight: moeID,
	}, {
		name:       "a mapped id the catalogue omits warns and proceeds",
		modelMap:   []string{"heavy=gemma-4-31b-preview"},
		ids:        []string{moeID},
		wantHeavy:  "gemma-4-31b-preview",
		wantLight:  moeID,
		wantStderr: `heavy tier model "gemma-4-31b-preview" is not in provider "solo"'s catalogue`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := soloOpts()
			opts.ModelMap = tt.modelMap
			got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(tt.ids...)))

			if got.code != 0 {
				t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
			}
			cfg, written := got.config(t)
			if !written {
				t.Fatal("no config.toml written on success")
			}
			if cfg.Models.Heavy != tt.wantHeavy || cfg.Models.Light != tt.wantLight {
				t.Errorf("models = %+v, want heavy=%q light=%q", cfg.Models, tt.wantHeavy, tt.wantLight)
			}
			if tt.wantStderr != "" && !strings.Contains(got.stderr, tt.wantStderr) {
				t.Errorf("stderr: got %q, want substring %q", got.stderr, tt.wantStderr)
			}
		})
	}
}

// TestRunConfigureCreatesConfigDir: the first configure of a fresh install
// has no config directory to write into and must make one.
func TestRunConfigureCreatesConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "kbase")
	opts := soloOpts()
	opts.ConfigPath = config.ConfigPath(dir)
	if got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(denseID, moeID))); got.code != 0 {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}
	if _, err := os.Stat(config.ConfigPath(dir)); err != nil {
		t.Fatalf("config.toml in a created directory: %v", err)
	}
}

// TestRunConfigureKeepsHandEdits: config.toml is a file users annotate, and
// `kbase configure` is the command that rewrites it. It must land the
// detected ids without disturbing anything around them.
func TestRunConfigureKeepsHandEdits(t *testing.T) {
	dir := t.TempDir()
	path := config.ConfigPath(dir)
	const note = "# the endpoint this box is allowed to reach"
	if err := os.WriteFile(path, []byte(note+"\nprovider = \"solo\"\n"), 0o600); err != nil {
		t.Fatalf("seed config.toml: %v", err)
	}

	opts := soloOpts()
	opts.ConfigPath = path
	if got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(denseID, moeID))); got.code != 0 {
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
	if cfg.Models.Heavy != denseID || cfg.Models.Light != moeID {
		t.Errorf("models = %+v, want heavy=%q light=%q", cfg.Models, denseID, moeID)
	}
}

// TestRunConfigureListFailure: a provider-side failure writes nothing. The
// previous configuration outliving a failed run is the whole reason the
// write is the last step.
func TestRunConfigureListFailure(t *testing.T) {
	client := model.NewScriptedMock(nil, catalogue(denseID, moeID))
	client.SetError(errors.New("http 503: service unavailable"))

	got := runConfig(t, soloOpts(), client)

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
	opts.ModelMap = []string{"heavy"}
	opts.NewClient = func(model.Endpoint) model.Client {
		t.Error("a malformed --model-map dialed the provider; flag syntax is checked first")
		return model.NewScriptedMock(nil, catalogue(denseID, moeID))
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
		values: []string{"heavy=h,light=l"},
		want:   map[string]string{config.TierHeavy: "h", config.TierLight: "l"},
	}, {
		name:   "repeated flag accumulates",
		values: []string{"heavy=h", "light=l"},
		want:   map[string]string{config.TierHeavy: "h", config.TierLight: "l"},
	}, {
		name:   "surrounding space and tier case are tolerated",
		values: []string{" HEAVY = h , light=l "},
		want:   map[string]string{config.TierHeavy: "h", config.TierLight: "l"},
	}, {
		name:   "repeating one tier with the same id is not a conflict",
		values: []string{"heavy=h", "heavy=h"},
		want:   map[string]string{config.TierHeavy: "h"},
	}, {
		name:    "no assignment",
		values:  []string{"heavy"},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "missing id",
		values:  []string{"heavy="},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "unknown tier",
		values:  []string{"medium=m"},
		wantErr: `unknown tier "medium"`,
	}, {
		name:    "conflicting assignments to one tier",
		values:  []string{"heavy=a,heavy=b"},
		wantErr: "assigns the heavy tier twice",
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
