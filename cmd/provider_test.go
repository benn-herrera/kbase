package main

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// closeLog counts the closes the clients a verb opened reported.
type closeLog struct{ n int }

// closer is a mock client that records its Close — the release the registry
// owes every client it opened.
type closer struct {
	*model.MockClient
	log *closeLog
}

func (c closer) Close() error { c.log.n++; return nil }

// cancellingCloser is the score run's client: it cancels the run as the second
// call arrives, and records its Close.
type cancellingCloser struct {
	*model.MockClient
	cancel context.CancelFunc
	calls  int
	log    *closeLog
}

func (c *cancellingCloser) Close() error { c.log.n++; return nil }

func (c *cancellingCloser) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	c.calls++
	if c.calls == 2 {
		c.cancel()
	}
	return c.MockClient.ConsultStream(ctx, req)
}

// TestRegistryOpensEachEndpointOnce: a client is opened on first use and
// reused — two roles on one pool entry share it and its connection pool, and
// an environment endpoint keys by its base URL — and closeAll releases every
// one it opened, once.
func TestRegistryOpensEachEndpointOnce(t *testing.T) {
	var log closeLog
	var opened int
	reg := newClients(func(model.Endpoint) model.Client {
		opened++
		return closer{model.NewScriptedMock(nil, nil), &log}
	}, nil)
	lab := config.Endpoint{Mode: config.ModeFile, Provider: "lab", BaseURL: "http://lab.example/v1"}
	first, err := reg.get(lab)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reg.get(lab)
	if err != nil {
		t.Fatal(err)
	}
	if opened != 1 || first != second {
		t.Errorf("the same entry opened %d client(s); want one, shared", opened)
	}
	if _, err := reg.get(config.Endpoint{Mode: config.ModeEnvironment, BaseURL: "http://env.example/v1"}); err != nil {
		t.Fatal(err)
	}
	if opened != 2 {
		t.Errorf("opened %d clients, want one per endpoint", opened)
	}
	reg.closeAll()
	if log.n != 2 {
		t.Errorf("closed %d clients, want every one opened", log.n)
	}
	reg.closeAll()
	if log.n != 2 {
		t.Errorf("a second close closed %d more", log.n-2)
	}
}

// TestRegistryPreflightReadsTheKeyFile: the key file is the registry's read,
// and the refusal it cannot read names the variable and the path. Nothing is
// opened.
func TestRegistryPreflightReadsTheKeyFile(t *testing.T) {
	var opened int
	reg := newClients(func(model.Endpoint) model.Client {
		opened++
		return model.NewScriptedMock(nil, nil)
	}, nil)
	_, err := reg.get(config.Endpoint{Provider: "lab", BaseURL: "http://lab.example/v1", APIKeyFile: "/absent/key.txt"})
	var refusal config.Refusal
	if !errors.As(err, &refusal) || refusal.Key != config.EnvAPIKeyFile || refusal.Path != "/absent/key.txt" {
		t.Fatalf("err = %v, want the key file's refusal", err)
	}
	if opened != 0 {
		t.Error("a refused key file opened a client")
	}
	if err := reg.preflight(config.Endpoint{Provider: "lab", APIKeyFile: "/absent/key.txt"}); err == nil {
		t.Error("the preflight read no key file")
	}
	if opened != 0 {
		t.Error("the preflight opened a client")
	}
}

// TestVerbsCloseEveryClientTheyOpen: every client a verb opens is closed
// before the verb returns, whatever the outcome — done, failed, refused and
// cancelled alike.
func TestVerbsCloseEveryClientTheyOpen(t *testing.T) {
	pool := func() config.Providers {
		return config.Providers{"solo": provider("solo", "http://solo.example/v1")}
	}
	var log closeLog
	newClient := func() newClientFunc {
		return func(model.Endpoint) model.Client { return closer{model.NewScriptedMock(nil, catalogue("m1")), &log} }
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
		unreachable := model.NewScriptedMock(nil, nil)
		unreachable.SetError(errors.New("http 503: service unavailable"))
		code, err := runModels(context.Background(), modelsOptions{
			providerOptions: providerOptions{Providers: pool(), Stderr: &stderr,
				NewClient: func(model.Endpoint) model.Client { return closer{unreachable, &log} }},
			Stdout: &stdout,
		})
		if err != nil || code != exitCodes[toolresult.Failed] {
			t.Fatalf("models: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("configure, refused after the dial", func(t *testing.T) {
		log.n = 0
		var stdout, stderr bytes.Buffer
		code, err := runConfigure(context.Background(), configureOptions{
			providerOptions: providerOptions{Providers: pool(), Stderr: &stderr, NewClient: newClient(), WriteDir: t.TempDir()},
			Stdout:          &stdout,
		})
		if err != nil || code != exitCodes[toolresult.Refused] {
			t.Fatalf("configure: exit %d, %v", code, err)
		}
		assertClosed(t, log.n, 1)
	})

	t.Run("score, cancelled", func(t *testing.T) {
		log.n = 0
		repo, state := scoreFixture(t), t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, Ctx: ctx}, func() model.Client {
			return &cancellingCloser{MockClient: scoring("0.11"), cancel: cancel, log: &log}
		})
		if got.doc.Outcome != toolresult.Cancelled {
			t.Fatalf("score: outcome %s\n%s", got.doc.Outcome, got.stdout)
		}
		assertClosed(t, log.n, 1)
	})
}

// assertClosed fails t unless n clients were closed — want them all.
func assertClosed(t *testing.T, n, want int) {
	t.Helper()
	if n != want {
		t.Errorf("clients closed = %d, want %d: every client the verb opened is released before it returns", n, want)
	}
}

// TestRefusalItems maps the configuration's refusals onto a document's items:
// one refusal is one item, several are several, and a failure is none.
func TestRefusalItems(t *testing.T) {
	one := config.Refusal{Key: "models.text", Path: "config.toml", Detail: "d"}
	few := config.Refusals{one, {Key: config.EnvText, Detail: "e"}}
	for _, tc := range []struct {
		name string
		err  error
		want []string
	}{
		{"one", one, []string{"models.text"}},
		{"several", few, []string{"models.text", config.EnvText}},
		{"a usage refusal keeps its check", config.Refusal{Check: checkUsage, Key: "--model-map"}, []string{"--model-map"}},
		{"a failure", errors.New("http 503"), nil},
		{"no error", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := refusalItems(tc.err)
			var keys []string
			for _, item := range items {
				keys = append(keys, item.Key)
				if item.Check == "" {
					t.Errorf("item %v carries no check", item)
				}
			}
			if !slices.Equal(keys, tc.want) {
				t.Errorf("keys = %q, want %q", keys, tc.want)
			}
		})
	}
}
