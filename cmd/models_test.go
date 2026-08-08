package main

import (
	"bytes"
	"context"
	"errors"
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

// result is what one runModels invocation produced: both streams, the
// endpoints the client factory was asked to build, and the error.
type result struct {
	stdout    string
	stderr    string
	endpoints []model.Endpoint
	err       error
}

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
	err := runModels(context.Background(), opts)
	return result{stdout: stdout.String(), stderr: stderr.String(), endpoints: seen, err: err}
}

// TestRunModelsSortedOutput is the happy path: ids land on stdout in
// ascending order regardless of the order the provider returned them, the
// endpoint is built from the pool entry, and the summary goes to stderr.
func TestRunModelsSortedOutput(t *testing.T) {
	got := run(t, modelsOptions{
		Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
		Provider:  "solo",
	}, model.NewScriptedMock(nil, catalogue("zeta", "alpha", "mu")))

	if got.err != nil {
		t.Fatalf("runModels: %v", got.err)
	}
	if want := "alpha\nmu\nzeta\n"; got.stdout != want {
		t.Errorf("stdout: got %q, want %q", got.stdout, want)
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
// stdout is empty and the summary reports count=0.
func TestRunModelsEmptyCatalogue(t *testing.T) {
	got := run(t, modelsOptions{
		Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
		Provider:  "solo",
	}, model.NewScriptedMock(nil, nil))

	if got.err != nil {
		t.Fatalf("runModels: %v", got.err)
	}
	if got.stdout != "" {
		t.Errorf("stdout: got %q, want empty", got.stdout)
	}
	if want := "models ok: solo count=0 elapsed="; !strings.HasPrefix(got.stderr, want) {
		t.Errorf("stderr: got %q, want prefix %q", got.stderr, want)
	}
}

// TestRunModelsFaultsAreWarnings: an entry dropped from the pool must be
// named on stderr — not vanish silently — while a healthy provider still
// lists.
func TestRunModelsFaultsAreWarnings(t *testing.T) {
	got := run(t, modelsOptions{
		Providers: config.Providers{"good": provider("good", "http://provider.example/v1")},
		Faults:    []config.ProviderFault{{Name: "bad", Reason: "read apiKeyFile: open missing.key: no such file or directory"}},
		Provider:  "good",
	}, model.NewScriptedMock(nil, catalogue("m1")))

	if got.err != nil {
		t.Fatalf("runModels: %v", got.err)
	}
	if !strings.Contains(got.stderr, `models: provider "bad" unavailable: read apiKeyFile:`) {
		t.Errorf("stderr should warn about the faulted provider; got %q", got.stderr)
	}
	if got.stdout != "m1\n" {
		t.Errorf("stdout: got %q, want %q", got.stdout, "m1\n")
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
		opts:       modelsOptions{Providers: pool("alpha", "beta"), Config: config.Config{Provider: "beta"}, Provider: "alpha"},
		wantChosen: "alpha",
	}, {
		name:       "config chosen when flag empty",
		opts:       modelsOptions{Providers: pool("alpha", "beta"), Config: config.Config{Provider: "beta"}},
		wantChosen: "beta",
	}, {
		name:       "sole entry needs no choice",
		opts:       modelsOptions{Providers: pool("alpha")},
		wantChosen: "alpha",
	}, {
		name:    "several entries, none chosen",
		opts:    modelsOptions{Providers: pool("beta", "alpha")},
		wantErr: "no provider selected",
	}, {
		name:    "empty pool",
		opts:    modelsOptions{Providers: config.Providers{}},
		wantErr: "no usable provider",
	}, {
		name: "named provider faulted out",
		opts: modelsOptions{
			Providers: pool("good"),
			Faults:    []config.ProviderFault{{Name: "bad", Reason: "no baseUrl declared"}},
			Provider:  "bad",
		},
		wantErr: `provider "bad" failed to load: no baseUrl declared`,
	}, {
		name:    "named provider never declared",
		opts:    modelsOptions{Providers: pool("good"), Provider: "typo"},
		wantErr: `provider "typo" not found`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, tt.opts, model.NewScriptedMock(nil, catalogue("m1")))
			if tt.wantErr != "" {
				if got.err == nil {
					t.Fatalf("expected error containing %q, got nil (stderr %q)", tt.wantErr, got.stderr)
				}
				if !strings.Contains(got.err.Error(), tt.wantErr) {
					t.Errorf("error: got %v, want substring %q", got.err, tt.wantErr)
				}
				if len(got.endpoints) != 0 {
					t.Errorf("no client should be constructed when selection fails; got %d", len(got.endpoints))
				}
				return
			}
			if got.err != nil {
				t.Fatalf("runModels: %v", got.err)
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
	got := run(t, modelsOptions{
		Providers: config.Providers{
			"beta":  provider("beta", "http://beta.example/v1"),
			"alpha": provider("alpha", "http://alpha.example/v1"),
		},
	}, model.NewScriptedMock(nil, nil))

	if got.err == nil {
		t.Fatal("expected a selection error, got nil")
	}
	if want := "available: alpha, beta"; !strings.Contains(got.err.Error(), want) {
		t.Errorf("error: got %v, want substring %q", got.err, want)
	}
}

// TestRunModelsListErrorPropagates: a provider-side failure is returned, not
// swallowed into an empty-but-successful listing.
func TestRunModelsListErrorPropagates(t *testing.T) {
	wantErr := errors.New("http 503: service unavailable")
	client := model.NewScriptedMock(nil, catalogue("m1"))
	client.SetError(wantErr)

	got := run(t, modelsOptions{
		Providers: config.Providers{"solo": provider("solo", "http://provider.example/v1")},
		Provider:  "solo",
	}, client)

	if !errors.Is(got.err, wantErr) {
		t.Fatalf("error: got %v, want it to wrap %v", got.err, wantErr)
	}
	if got.stdout != "" {
		t.Errorf("stdout: got %q, want empty on failure", got.stdout)
	}
	if strings.Contains(got.stderr, "models ok:") {
		t.Errorf("stderr must not report success after a list failure; got %q", got.stderr)
	}
}
