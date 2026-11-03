package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/modelclient"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
	"kbase/internal/secret"
)

// closeLog counts the closes the clients a verb opened reported.
type closeLog struct{ n int }

// closer is a mock client that records its Close — the release the registry
// owes every client it opened.
type closer struct {
	model.Client
	log *closeLog
}

func (c closer) Close() error { c.log.n++; return nil }

// TestVerbsCloseEveryClientTheyOpen: every client a verb opens is closed
// before the verb returns, whatever the outcome — done, failed, refused and
// cancelled alike.
func TestVerbsCloseEveryClientTheyOpen(t *testing.T) {
	pool := func() config.Providers {
		return config.Providers{"solo": provider("solo", "http://solo.example/v1")}
	}
	var log closeLog
	newClient := func() modelclient.NewClientFunc {
		return func(model.Endpoint) model.Client {
			return closer{modeltest.NewScriptedMock(nil, catalogue("m1")).Client(), &log}
		}
	}

	t.Run("models, done", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		code, err := runModels(context.Background(), modelsOptions{
			providerOptions: providerOptions{Providers: pool(), Stderr: &stderr, NewClient: newClient()},
			Stdout:          &stdout,
		})
		if err != nil || code != 0 {
			t.Fatalf("models: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("models, failed", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		unreachable := modeltest.NewScriptedMock(nil, nil)
		unreachable.SetError(errors.New("http 503: service unavailable"))
		code, err := runModels(context.Background(), modelsOptions{
			providerOptions: providerOptions{Providers: pool(), Stderr: &stderr,
				NewClient: func(model.Endpoint) model.Client { return closer{unreachable.Client(), &log} }},
			Stdout: &stdout,
		})
		if err != nil || code != toolresult.ExitCode(toolresult.Failed) {
			t.Fatalf("models: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("configure-models, done", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		dir := t.TempDir()
		writeConfig(t, dir, config.ProvidersFileName, "[solo]\nbaseUrl = \"http://solo.example/v1\"\n")
		opts := namedLayer(dir)
		opts.Stderr, opts.NewClient = &stderr, newClient()
		code, err := runConfigureModels(context.Background(), configureModelsOptions{
			providerOptions: opts,
			Models:          map[config.Role]string{config.RoleText: "solo/m1"},
			Stdout:          &stdout,
		})
		if err != nil || code != 0 {
			t.Fatalf("configure-models: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("configure-providers, dialled", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		opts := namedLayer(t.TempDir())
		opts.Stderr, opts.NewClient = &stderr, newClient()
		code, err := runConfigureProviders(context.Background(), configureProvidersOptions{
			providerOptions: opts, Name: "solo", BaseURL: "http://solo.example/v1",
			StoredKeys: config.StoredKeys{Keys: map[string]secret.Key{"solo": secret.NewKey(testAPIKey)}},
			Stdout:     &stdout,
		})
		if err != nil || code != 0 {
			t.Fatalf("configure-providers: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("score, cancelled", func(t *testing.T) {
		log.n = 0
		repo, state := scoreFixture(t), t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Ctx: ctx}, func() model.Client {
			return closer{(&cancelOnSecond{MockClient: scriptedScoring("0.11"), cancel: cancel}).Client(), &log}
		})
		if got.doc.Outcome != toolresult.Cancelled {
			t.Fatalf("score: outcome %s\n%s", got.doc.Outcome, got.stdout)
		}
		assertClosed(t, log.n, 1)
	})
}

// TestShowConfigDropsACredentialedBaseURL: a providers.toml entry whose
// baseUrl carries credentials is dropped as it loads, so show-config lists the
// provider with no base-url, and no part of the URL reaches its output.
func TestShowConfigDropsACredentialedBaseURL(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"https://u:k@host/v1\"\n")
	doc, stdout, stderr, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	providers := doc.items("providers")
	if code != 0 || len(providers) != 1 || providers[0]["name"] != "lab" {
		t.Fatalf("exit %d, providers %v; want lab listed", code, providers)
	}
	if _, ok := providers[0]["base-url"]; ok {
		t.Error("the dropped provider carries a base-url")
	}
	if strings.Contains(stdout+stderr, "u:k@") {
		t.Error("the base URL's credentials reached the output")
	}
}

// assertClosed fails t unless n clients were closed — want them all.
func assertClosed(t *testing.T, n, want int) {
	t.Helper()
	if n != want {
		t.Errorf("clients closed = %d, want %d: every client the verb opened is released before it returns", n, want)
	}
}

// TestRefusedOrFailed: an error is a verb's stop — a refusal is refused with its
// items, a live run's retry is retry, and anything else is failed.
func TestRefusedOrFailed(t *testing.T) {
	items := []toolresult.Item{{Check: config.CheckProvider, Key: "models.text", Detail: "d"}}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"a refusal", toolresult.Refusal(items), toolresult.Refused},
		{"a wrapped refusal", fmt.Errorf("resolve: %w", toolresult.Refusal(items)), toolresult.Refused},
		{"a live run", buildstore.Retry(items), toolresult.Retry},
		{"a failure", errors.New("http 503"), toolresult.Failed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if outcome, _ := refusedOrFailed(nil, tc.err); outcome != tc.want {
				t.Errorf("outcome = %q, want %q", outcome, tc.want)
			}
		})
	}
}
