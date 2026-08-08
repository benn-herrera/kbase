package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
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

// configureResult is what one runConfigure invocation produced: stderr,
// the error, and the config directory it was pointed at.
type configureResult struct {
	stderr string
	dir    string
	err    error
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
	var stderr bytes.Buffer
	opts.Stderr = &stderr
	if opts.ConfigPath == "" {
		opts.ConfigPath = config.ConfigPath(dir)
	}
	opts.NewClient = func(model.Endpoint) model.Client { return client }
	err := runConfigure(context.Background(), opts)
	return configureResult{stderr: stderr.String(), dir: dir, err: err}
}

// soloPool is the single-entry pool every fixture configures against, so
// provider selection never needs a flag it is not the subject of.
func soloPool() config.Providers {
	return config.Providers{"solo": provider("solo", "http://provider.example/v1")}
}

// TestRunConfigureAutoDetect is the max-convenience default: one gemma-4
// dense and one MoE in the catalogue resolve both tiers with no flags, and
// the resolved ids land in config.toml.
func TestRunConfigureAutoDetect(t *testing.T) {
	got := runConfig(t, configureOptions{Providers: soloPool()},
		model.NewScriptedMock(nil, catalogue("text-embedding-3-large", denseID, moeID, "gemma-3-27b-it")))

	if got.err != nil {
		t.Fatalf("runConfigure: %v", got.err)
	}
	cfg, written := got.config(t)
	if !written {
		t.Fatal("no config.toml written on success")
	}
	want := config.Config{Provider: "solo", Models: config.ModelMap{Heavy: denseID, Light: moeID}}
	if cfg != want {
		t.Errorf("config.toml = %+v, want %+v", cfg, want)
	}
	if !strings.Contains(got.stderr, "configured: provider=solo heavy="+denseID+" light="+moeID+" (wrote ") {
		t.Errorf("stderr: got %q, want the summary line", got.stderr)
	}
	if strings.Contains(got.stderr, testAPIKey) {
		t.Error("API key leaked into stderr")
	}
}

// TestRunConfigureAmbiguous: two dense candidates is the case the
// appliance must refuse. The error names the tier, lists both, and shows
// the flag that settles it — and nothing is written.
func TestRunConfigureAmbiguous(t *testing.T) {
	const otherDense = "gemma-4-31b-instruct"
	got := runConfig(t, configureOptions{Providers: soloPool()},
		model.NewScriptedMock(nil, catalogue(denseID, otherDense, moeID)))

	if got.err == nil {
		t.Fatal("two dense candidates: got nil error, want an ambiguity failure")
	}
	msg := got.err.Error()
	for _, want := range []string{"heavy tier is ambiguous", denseID, otherDense, "--model-map heavy=<id>"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error: got %v, want substring %q", got.err, want)
		}
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite an ambiguity failure")
	}
}

// TestRunConfigureNoMatch: a provider with no gemma-4 at all fails and
// lists what it did offer — including family members whose tier went
// unrecognized, called out separately as the likeliest intended targets.
func TestRunConfigureNoMatch(t *testing.T) {
	const familyNoTier = "gemma-4-9b"
	got := runConfig(t, configureOptions{Providers: soloPool()},
		model.NewScriptedMock(nil, catalogue("text-embedding-3-large", familyNoTier, "gemma-3-27b-it")))

	if got.err == nil {
		t.Fatal("no gemma-4 tier models: got nil error, want a no-match failure")
	}
	msg := got.err.Error()
	for _, want := range []string{
		"no gemma-4 heavy-tier model detected",
		"gemma-4 models with no recognized tier:",
		familyNoTier,
		"text-embedding-3-large",
		"gemma-3-27b-it",
		modelMapSyntax,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error: got %v, want substring %q", got.err, want)
		}
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
			got := runConfig(t, configureOptions{Providers: soloPool(), ModelMap: tt.modelMap},
				model.NewScriptedMock(nil, catalogue(tt.ids...)))

			if got.err != nil {
				t.Fatalf("runConfigure: %v", got.err)
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
	var stderr bytes.Buffer
	err := runConfigure(context.Background(), configureOptions{
		Providers:  soloPool(),
		ConfigPath: config.ConfigPath(dir),
		Stderr:     &stderr,
		NewClient: func(model.Endpoint) model.Client {
			return model.NewScriptedMock(nil, catalogue(denseID, moeID))
		},
	})
	if err != nil {
		t.Fatalf("runConfigure: %v", err)
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

	var stderr bytes.Buffer
	err := runConfigure(context.Background(), configureOptions{
		Providers:  soloPool(),
		ConfigPath: path,
		Stderr:     &stderr,
		NewClient: func(model.Endpoint) model.Client {
			return model.NewScriptedMock(nil, catalogue(denseID, moeID))
		},
	})
	if err != nil {
		t.Fatalf("runConfigure: %v", err)
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

	got := runConfig(t, configureOptions{Providers: soloPool()}, client)

	if got.err == nil {
		t.Fatal("list failure: got nil error, want it propagated")
	}
	if _, written := got.config(t); written {
		t.Error("config.toml written despite a list failure")
	}
}

// TestRunConfigureBadModelMapSkipsNetwork: flag syntax is checked before
// anything is dialed, so a typo costs no round trip.
func TestRunConfigureBadModelMapSkipsNetwork(t *testing.T) {
	client := model.NewScriptedMock(nil, catalogue(denseID, moeID))
	client.RecordCalls = true

	got := runConfig(t, configureOptions{
		Providers: soloPool(),
		ModelMap:  []string{"heavy"},
	}, client)

	if got.err == nil {
		t.Fatal("malformed --model-map: got nil error, want a syntax failure")
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
