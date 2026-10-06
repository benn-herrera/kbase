package main

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
)

// testAPIKey is the credential every fixture provider carries. Tests assert
// it never reaches an output stream or an error string.
const testAPIKey = "sk-MUST-NOT-APPEAR-0001"

// provider builds a pool entry as LoadProviders would return it — Name
// populated from the table header, APIKey already resolved.
func provider(name, baseURL string) config.Provider {
	return config.Provider{Name: name, BaseURL: baseURL, APIKey: testAPIKey}
}

// catalogue builds the ModelInfo slice a mock client serves from ListModels.
func catalogue(ids ...string) []model.ModelInfo {
	infos := make([]model.ModelInfo, 0, len(ids))
	for _, id := range ids {
		infos = append(infos, model.ModelInfo{ID: id})
	}
	return infos
}

// result is what one runModels invocation produced: its result document,
// stdout as written, stderr, the exit code, and the endpoints the client
// factory was asked to build.
type result struct {
	doc       resultDoc
	stdout    string
	stderr    string
	code      int
	endpoints []model.Endpoint
}

// ids is the result's model ids.
func (r result) ids() []string { return anyStrings(r.doc.Values["models"]) }

// run invokes runModels with the given options, substituting a client
// factory that records its endpoint and serves ids from a MockClient. Any
// Stdout/Stderr/NewClient already set on opts is overwritten — the point of
// the helper is that no test reaches the network or the process streams.
func run(t *testing.T, opts modelsOptions, client *model.MockClient) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var seen []model.Endpoint
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	opts.NewClient = func(e model.Endpoint) model.Client {
		seen = append(seen, e)
		return client
	}
	code, err := runModels(context.Background(), opts)
	if err != nil {
		t.Fatalf("runModels: %v", err)
	}
	return result{doc: checkDocument(t, "models", stdout.String()), stdout: stdout.String(), stderr: stderr.String(), code: code, endpoints: seen}
}

// TestVerbStreamGuard: the guard the verbs share still names the verb that
// tripped it. A shared helper that reported a generic message would send the
// reader looking through the wrong command's call sites.
func TestVerbStreamGuard(t *testing.T) {
	for _, tc := range []struct {
		verb string
		call func() error
	}{
		{"models", func() error {
			_, err := runModels(context.Background(), modelsOptions{})
			return err
		}},
		{"configure", func() error {
			_, err := runConfigure(context.Background(), configureOptions{ConfigPath: "config.toml"})
			return err
		}},
		{"build", func() error {
			_, err := runBuild(context.Background(), buildOptions{VolumeRoots: []string{t.TempDir()}})
			return err
		}},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("a verb handed no streams must refuse before doing any work")
			}
			if !strings.HasPrefix(err.Error(), tc.verb+":") {
				t.Errorf("error = %v, want it prefixed with the verb that tripped the guard", err)
			}
		})
	}
}

// TestRunModelsSortedOutput is the happy path: ids land on stdout in
// ascending order regardless of the order the provider returned them, the
// endpoint is built from the pool entry, and the summary goes to stderr.
func TestRunModelsSortedOutput(t *testing.T) {
	got := run(t, modelsOptions{providerOptions: providerOptions{
		Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
		Provider:  "solo",
	}}, model.NewScriptedMock(nil, catalogue("zeta", "alpha", "mu")))

	if got.code != 0 || got.doc.Values["provider"] != "solo" {
		t.Fatalf("models: exit %d, %v", got.code, got.doc.Values)
	}
	if want := []string{"alpha", "mu", "zeta"}; !slices.Equal(got.ids(), want) {
		t.Errorf("models: got %q, want %q", got.ids(), want)
	}
	if want := "models ok: solo count=3 elapsed="; !strings.HasPrefix(got.stderr, want) {
		t.Errorf("stderr: got %q, want prefix %q", got.stderr, want)
	}
	if len(got.endpoints) != 1 {
		t.Fatalf("client constructions: got %d, want 1", len(got.endpoints))
	}
	e := got.endpoints[0]
	if e.Name != "solo" || e.BaseURL != "http://provider.example/v1" || e.APIKey != testAPIKey {
		t.Errorf("endpoint: got %+v, want name/baseURL/key from the pool entry", model.Endpoint{Name: e.Name, BaseURL: e.BaseURL})
	}
	if strings.Contains(got.stdout, testAPIKey) || strings.Contains(got.stderr, testAPIKey) {
		t.Error("API key leaked into an output stream")
	}
}

// TestRunModelsEmptyCatalogue: a provider serving nothing is not an error —
// the list is empty and the summary reports count=0.
func TestRunModelsEmptyCatalogue(t *testing.T) {
	got := run(t, modelsOptions{providerOptions: providerOptions{
		Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
		Provider:  "solo",
	}}, model.NewScriptedMock(nil, nil))

	if got.code != 0 || len(got.ids()) != 0 {
		t.Errorf("models: exit %d, ids %q; want done and none", got.code, got.ids())
	}
	if want := "models ok: solo count=0 elapsed="; !strings.HasPrefix(got.stderr, want) {
		t.Errorf("stderr: got %q, want prefix %q", got.stderr, want)
	}
}

// TestRunModelsFaultsAreWarnings: an entry dropped from the pool must be
// named on stderr — not vanish silently — while a healthy provider still
// lists.
func TestRunModelsFaultsAreWarnings(t *testing.T) {
	got := run(t, modelsOptions{providerOptions: providerOptions{
		Providers: config.Providers{"good": provider("good", "http://provider.example/v1")},
		Faults:    []config.ProviderFault{{Name: "bad", Reason: "read apiKeyFile: open missing.key: no such file or directory"}},
		Provider:  "good",
	}}, model.NewScriptedMock(nil, catalogue("m1")))

	if !strings.Contains(got.stderr, `provider "bad" unavailable: read apiKeyFile:`) {
		t.Errorf("stderr should warn about the faulted provider; got %q", got.stderr)
	}
	if got.code != 0 || !slices.Equal(got.ids(), []string{"m1"}) {
		t.Errorf("models: exit %d, ids %q; want m1", got.code, got.ids())
	}
}

// TestRunModelsWarningFaultKeepsProvider: a fault the provider SURVIVED —
// an exposed key file, say — must be reported without calling the provider
// unavailable. It is still in the pool, still selected, and still listed.
func TestRunModelsWarningFaultKeepsProvider(t *testing.T) {
	const reason = "key file is readable by group or other"
	got := run(t, modelsOptions{providerOptions: providerOptions{
		Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
		Faults:    []config.ProviderFault{{Name: "solo", Reason: reason, Warning: true}},
	}}, model.NewScriptedMock(nil, catalogue("m1")))

	if !strings.Contains(got.stderr, `provider "solo": `+reason) {
		t.Errorf("stderr should carry the warning; got %q", got.stderr)
	}
	if strings.Contains(got.stderr, "unavailable") {
		t.Errorf("a survived fault must not be reported as unavailable; got %q", got.stderr)
	}
	if got.code != 0 || !slices.Equal(got.ids(), []string{"m1"}) {
		t.Errorf("models: exit %d, ids %q; want m1", got.code, got.ids())
	}
}

// TestRunModelsProviderSelection covers the selection ladder and its
// failures. Selection is observed through the summary line, which names the
// provider that was actually queried.
func TestRunModelsProviderSelection(t *testing.T) {
	pool := func(names ...string) config.Providers {
		p := config.Providers{}
		for _, n := range names {
			p[n] = provider(n, "http://"+n+".example/v1")
		}
		return p
	}

	tests := []struct {
		name       string
		opts       modelsOptions
		wantChosen string // provider expected in the summary line
		wantErr    string // substring of the expected error
	}{{
		name:       "flag beats config",
		opts:       modelsOptions{providerOptions: providerOptions{Providers: pool("alpha", "beta"), Config: config.Config{Provider: "beta"}, Provider: "alpha"}},
		wantChosen: "alpha",
	}, {
		name:       "config chosen when flag empty",
		opts:       modelsOptions{providerOptions: providerOptions{Providers: pool("alpha", "beta"), Config: config.Config{Provider: "beta"}}},
		wantChosen: "beta",
	}, {
		name:       "sole entry needs no choice",
		opts:       modelsOptions{providerOptions: providerOptions{Providers: pool("alpha")}},
		wantChosen: "alpha",
	}, {
		name:    "several entries, none chosen",
		opts:    modelsOptions{providerOptions: providerOptions{Providers: pool("beta", "alpha")}},
		wantErr: "no provider selected",
	}, {
		name:    "empty pool",
		opts:    modelsOptions{providerOptions: providerOptions{Providers: config.Providers{}}},
		wantErr: "no usable provider",
	}, {
		name: "named provider faulted out",
		opts: modelsOptions{providerOptions: providerOptions{
			Providers: pool("good"),
			Faults:    []config.ProviderFault{{Name: "bad", Reason: "no baseUrl declared"}},
			Provider:  "bad",
		}},
		wantErr: `provider "bad" failed to load: no baseUrl declared`,
	}, {
		name:    "named provider never declared",
		opts:    modelsOptions{providerOptions: providerOptions{Providers: pool("good"), Provider: "typo"}},
		wantErr: `provider "typo" not found`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, tt.opts, model.NewScriptedMock(nil, catalogue("m1")))
			if tt.wantErr != "" {
				items := got.doc.items("refusals")
				if got.code != 1 || len(items) != 1 {
					t.Fatalf("expected one refusal containing %q, got exit %d, %v (stderr %q)", tt.wantErr, got.code, items, got.stderr)
				}
				if detail, _ := items[0]["detail"].(string); !strings.Contains(detail, tt.wantErr) || items[0]["check"] != "provider" {
					t.Errorf("refusal: got %v, want a provider refusal containing %q", items[0], tt.wantErr)
				}
				if len(got.endpoints) != 0 {
					t.Errorf("no client should be constructed when selection fails; got %d", len(got.endpoints))
				}
				return
			}
			if got.code != 0 || got.doc.Values["provider"] != tt.wantChosen {
				t.Fatalf("models: exit %d, %v", got.code, got.doc.Values)
			}
			if want := "models ok: " + tt.wantChosen + " "; !strings.Contains(got.stderr, want) {
				t.Errorf("stderr: got %q, want substring %q", got.stderr, want)
			}
		})
	}
}

// TestRunModelsSelectionErrorListsCandidates: the failure with several
// declared providers must name them, since resolving it means choosing one.
func TestRunModelsSelectionErrorListsCandidates(t *testing.T) {
	got := run(t, modelsOptions{providerOptions: providerOptions{
		Providers: config.Providers{
			"beta":  provider("beta", "http://beta.example/v1"),
			"alpha": provider("alpha", "http://alpha.example/v1"),
		},
	}}, model.NewScriptedMock(nil, nil))

	items := got.doc.items("refusals")
	if got.code != 1 || len(items) != 1 {
		t.Fatalf("expected one selection refusal, got exit %d, %v", got.code, items)
	}
	if items[0]["key"] != "--provider" || !slices.Equal(anyStrings(items[0]["allowed"]), []string{"alpha", "beta"}) {
		t.Errorf("refusal: got %v, want --provider with every entry allowed", items[0])
	}
}

// TestRunModelsListErrorPropagates: a provider-side failure is reported
// failed, not swallowed into an empty-but-successful listing.
func TestRunModelsListErrorPropagates(t *testing.T) {
	client := model.NewScriptedMock(nil, catalogue("m1"))
	client.SetError(errors.New("http 503: service unavailable"))

	got := run(t, modelsOptions{providerOptions: providerOptions{
		Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
		Provider:  "solo",
	}}, client)

	if failures := got.doc.items("failures"); got.code != 3 || len(failures) != 1 || !strings.Contains(failures[0]["detail"].(string), "503") {
		t.Fatalf("list failure: exit %d, failures %v; want it reported failed", got.code, failures)
	}
	if _, listed := got.doc.Values["models"]; listed {
		t.Errorf("a failed listing reported models: %v", got.doc.Values)
	}
	if strings.Contains(got.stderr, "models ok:") {
		t.Errorf("stderr must not report success after a list failure; got %q", got.stderr)
	}
}
